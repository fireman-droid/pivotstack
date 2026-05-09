package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"kiro-api-proxy/pool"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func init() {
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".json", "application/json")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".woff", "font/woff")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".ttf", "font/ttf")
	mime.AddExtensionType(".png", "image/png")
	mime.AddExtensionType(".jpg", "image/jpeg")
	mime.AddExtensionType(".ico", "image/x-icon")
}

// Handler HTTP 处理器
type Handler struct {
	pool *pool.AccountPool
	// 运行时统计 (使用原子操作)
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalTokens     int64
	totalCredits    float64 // float64 需要用锁保护
	creditsMu       sync.RWMutex
	startTime       int64
	stopRefresh     chan struct{}
	stopStatsSaver  chan struct{}
	// 模型缓存
	cachedModels    []ModelInfo
	modelsCacheMu   sync.RWMutex
	modelsCacheTime int64
	// 调用日志
	callLogs   []CallLog
	callLogsMu sync.RWMutex
	// API Key 统计 (内存缓存)
	apiKeyStats   map[string]*ApiKeyStats
	apiKeyStatsMu sync.RWMutex
	// SSE 实时日志订阅
	logSubscribers   map[chan CallLog]bool
	logSubscribersMu sync.RWMutex
	// Credit 消耗预测
	creditPredictor     *CreditPredictor
	proCreditPredictor  *CreditPredictor
	freeCreditPredictor *CreditPredictor
}

type contextKeyType string

const userCtxKey contextKeyType = "userCtx"

// UserContext holds per-request API key context.
type UserContext struct {
	KeyID         string
	KeyTier       string // "free" | "pro"
	ActualPaidUSD float64
	ActualGiftUSD float64
}

func withUserContext(r *http.Request, uc *UserContext) *http.Request {
	if uc == nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), userCtxKey, uc))
}

func getUserContext(ctx context.Context) *UserContext {
	if v, ok := ctx.Value(userCtxKey).(*UserContext); ok {
		return v
	}
	return nil
}

// CallLog 单次调用记录（结构化日志）
type CallLog struct {
	Time            string  `json:"time"`
	Timestamp       int64   `json:"timestamp"`
	RequestID       string  `json:"request_id,omitempty"`
	APIType         string  `json:"api_type"`
	OriginalModel   string  `json:"original_model"`
	ActualModel     string  `json:"actual_model"`
	Account         string  `json:"account"`
	ApiKeyID        string  `json:"api_key_id,omitempty"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Credits         float64 `json:"credits,omitempty"`           // 计费 credits（掺水后；若未掺水 = 上游原值）
	UpstreamCredits float64 `json:"upstream_credits,omitempty"`  // 上游原始 credits（掺水前；admin 审计用，用户端清零）
	// PaidCredits/GiftedCredits/CostUSD 故意不带 omitempty：
	// 即使 0 也要显式落盘，否则纯 gift 余额扣费的请求（paid=0）会让对账误判为"未扣费"。
	// 注意 CostUSD 在 handler_stats 中被覆盖为 paidCostUSD（仅营收，不含 gift 部分）。
	PaidCredits   float64 `json:"paid_credits"`
	GiftedCredits float64 `json:"gifted_credits"`
	CostUSD       float64 `json:"cost_usd"`
	// CostUSDLegacy 是按 v1 旧公式（PoolPriceUSD × ModelMultiplier）算的 shadow 值。
	// 部署 v2 后 24 小时观察期内 grep 看 cost_usd 跟 cost_usd_legacy 是否始终相等，
	// 不等说明迁移有偏差，立即回滚。观察期后下个迭代清理这个字段。
	CostUSDLegacy   float64 `json:"cost_usd_legacy,omitempty"`
	PriceModel      string  `json:"price_model,omitempty"` // 计费用 model（含 stealth originalModel 信息）
	Stream          bool    `json:"stream"`
	Error           string  `json:"error,omitempty"`
	PayloadKB       int     `json:"payload_kb,omitempty"`
	Status          string  `json:"status"`
	StopReason      string  `json:"stop_reason,omitempty"`
	DurationMs      int64   `json:"duration_ms,omitempty"`
	Attempt         int     `json:"attempt,omitempty"`
	Subscription    string  `json:"subscription,omitempty"`
	RequestSummary  string  `json:"request_summary,omitempty"`
	ResponseSummary string  `json:"response_summary,omitempty"`
}

const maxCallLogs = 5000

// RechargeRecord 充值/赠送/调整流水（金额关键，每条立即落盘）
// 写入路径：data/recharge_records.jsonl
type RechargeRecord struct {
	Time          string  `json:"time"`           // "MM-DD HH:MM:SS" CST
	Timestamp     int64   `json:"timestamp"`      // unix seconds
	KeyID         string  `json:"key_id"`         // ApiKey UUID
	KeyNote       string  `json:"key_note,omitempty"`
	Type          string  `json:"type"`           // "code_redeem" | "code_redeem_days" | "admin_balance" | "admin_gift" | "admin_adjust"
	Code          string  `json:"code,omitempty"` // 仅 code_redeem 类型有
	AmountUSD     float64 `json:"amount_usd"`     // USD face value 增量（带正负号）
	AmountCNY     float64 `json:"amount_cny"`     // ¥ 等价（cny = usd × 0.05）
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	GiftBefore    float64 `json:"gift_before"`
	GiftAfter     float64 `json:"gift_after"`
	Operator      string  `json:"operator"` // "user"（自助兑换）| "admin"
	Note          string  `json:"note,omitempty"`
	IP            string  `json:"ip,omitempty"`
}

type thinkingStreamSource int

const (
	thinkingSourceUnknown thinkingStreamSource = iota
	thinkingSourceReasoningEvent
	thinkingSourceTagBlock
)

func allowReasoningSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceTagBlock {
		return false
	}
	*source = thinkingSourceReasoningEvent
	return true
}

func allowTagSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceReasoningEvent {
		return false
	}
	if *source == thinkingSourceUnknown {
		*source = thinkingSourceTagBlock
	}
	return *source == thinkingSourceTagBlock
}

func NewHandler() *Handler {
	totalReq, successReq, failedReq, totalTokens, totalCredits := config.GetStats()
	h := &Handler{
		pool:                pool.GetPool(),
		totalRequests:       int64(totalReq),
		successRequests:     int64(successReq),
		failedRequests:      int64(failedReq),
		totalTokens:         int64(totalTokens),
		totalCredits:        totalCredits,
		startTime:           time.Now().Unix(),
		stopRefresh:         make(chan struct{}),
		stopStatsSaver:      make(chan struct{}),
		apiKeyStats:         make(map[string]*ApiKeyStats),
		logSubscribers:      make(map[chan CallLog]bool),
		creditPredictor:     newCreditPredictor(200, 0.3),
		proCreditPredictor:  newCreditPredictor(200, 0.3),
		freeCreditPredictor: newCreditPredictor(200, 0.3),
	}
	// 从磁盘恢复历史日志和 CreditPredictor
	h.loadLogsFromDisk()
	// 启动日志自动清理（每6小时清理超过7天的）
	h.startLogCleanupTicker()
	// 启动后台刷新
	go h.backgroundRefresh()
	// 启动后台统计保存 (每5分钟批量写入)
	go h.backgroundStatsSaver()
	return h
}

// backgroundRefresh 后台定时刷新账户信息
func (h *Handler) backgroundRefresh() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	time.Sleep(5 * time.Second)
	h.refreshAllAccounts()
	h.refreshModelsCache()

	for {
		select {
		case <-ticker.C:
			h.refreshAllAccounts()
			h.refreshModelsCache()
		case <-h.stopRefresh:
			return
		}
	}
}

// refreshAllAccounts 刷新所有账户信息
func (h *Handler) refreshAllAccounts() {
	accounts := config.GetAccounts()
	refreshed := 0
	failed := 0
	skipped := 0

	for i := range accounts {
		account := &accounts[i]
		if !account.Enabled || (account.AccessToken == "" && account.RefreshToken == "") {
			fmt.Printf("[BackgroundRefresh] SKIP %s: enabled=%v, atLen=%d, rtLen=%d\n",
				account.Email, account.Enabled, len(account.AccessToken), len(account.RefreshToken))
			skipped++
			continue
		}

		// 主动刷新 token：过期前 30 分钟或已过期
		needsRefresh := false
		if account.ExpiresAt == 0 {
			needsRefresh = true // ExpiresAt 未设置，强制刷新
		} else if time.Now().Unix() > account.ExpiresAt-1800 {
			needsRefresh = true // 30 分钟内过期或已过期
		}

		if needsRefresh {
			fmt.Printf("[BackgroundRefresh] Refreshing token for %s (expiresAt=%d, now=%d)...\n",
				account.Email, account.ExpiresAt, time.Now().Unix())

			// 最多重试 2 次
			var lastErr error
			for retry := 0; retry < 2; retry++ {
				newAccessToken, newRefreshToken, newExpiresAt, err := auth.RefreshToken(account)
				if err != nil {
					lastErr = err
					fmt.Printf("[BackgroundRefresh] Retry %d failed for %s: %v\n", retry+1, account.Email, err)
					time.Sleep(3 * time.Second)
					continue
				}
				account.AccessToken = newAccessToken
				if newRefreshToken != "" {
					account.RefreshToken = newRefreshToken
				}
				account.ExpiresAt = newExpiresAt
				config.UpdateAccountToken(account.ID, newAccessToken, newRefreshToken, newExpiresAt)
				h.pool.UpdateToken(account.ID, newAccessToken, newRefreshToken, newExpiresAt)
				fmt.Printf("[BackgroundRefresh] Token refreshed OK for %s, new expiresAt=%d (in %dm)\n",
					account.Email, newExpiresAt, (newExpiresAt-time.Now().Unix())/60)
				lastErr = nil
				refreshed++
				break
			}
			if lastErr != nil {
				fmt.Printf("[BackgroundRefresh] Token refresh FAILED for %s after retries: %v\n", account.Email, lastErr)
				failed++
				continue
			}
		}

		info, err := RefreshAccountInfo(account)
		if err != nil {
			fmt.Printf("[BackgroundRefresh] Failed to refresh info for %s: %v\n", account.Email, err)
			continue
		}

		config.UpdateAccountInfo(account.ID, *info)
	}
	h.pool.Reload()
	fmt.Printf("[BackgroundRefresh] Done: %d refreshed, %d failed, %d skipped, %d total\n",
		refreshed, failed, skipped, len(accounts))
}

// resolveApiKey resolves the API key from the request and returns a UserContext.
// When API key validation is disabled, still tries to extract KeyID for log association.
func (h *Handler) resolveApiKey(r *http.Request) (*UserContext, error) {
	authHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-Api-Key")

	var providedKey string
	if strings.HasPrefix(authHeader, "Bearer ") {
		providedKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else if apiKeyHeader != "" {
		providedKey = apiKeyHeader
	}

	if !config.IsApiKeyRequired() {
		// API key validation disabled, but still try to associate logs with user
		uc := &UserContext{KeyTier: "pro"}
		if providedKey != "" {
			if info := config.FindApiKey(providedKey); info != nil {
				uc.KeyID = info.ID
				uc.KeyTier = info.Tier
			}
		}
		return uc, nil
	}

	if providedKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	info := config.FindApiKey(providedKey)
	if info == nil {
		return nil, fmt.Errorf("invalid api key")
	}

	// Unified plan validation (timed / credit / hybrid)
	if errType, err := config.ValidateKeyAccess(info); err != nil {
		return nil, fmt.Errorf("%s: %s", errType, err.Error())
	}

	return &UserContext{KeyID: info.ID, KeyTier: info.Tier}, nil
}

// ServeHTTP 路由分发
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 生成 request_id
	requestID := generateRequestID()
	r.Header.Set("X-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)

	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, anthropic-version, anthropic-beta, x-api-key, x-stainless-os, x-stainless-lang, x-stainless-package-version, x-stainless-runtime, x-stainless-runtime-version, x-stainless-arch")
	w.Header().Set("Access-Control-Expose-Headers", "x-request-id, x-ratelimit-limit-requests, x-ratelimit-limit-tokens, x-ratelimit-remaining-requests, x-ratelimit-remaining-tokens, x-ratelimit-reset-requests, x-ratelimit-reset-tokens")

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}

	switch {
	case path == "/v1/messages" || path == "/messages" || path == "/anthropic/v1/messages":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendClaudeError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleClaudeMessages(w, withUserContext(r, uc))
	case path == "/v1/messages/count_tokens" || path == "/messages/count_tokens":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendClaudeError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleCountTokens(w, withUserContext(r, uc))
	case path == "/v1/chat/completions" || path == "/chat/completions":
		uc, err := h.resolveApiKey(r)
		if err != nil {
			h.sendOpenAIError(w, 401, "authentication_error", err.Error())
			return
		}
		h.handleOpenAIChat(w, withUserContext(r, uc))
	case path == "/v1/models" || path == "/models":
		h.handleModels(w, r)
	case path == "/api/event_logging/batch":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"status":"ok"}`))

	case strings.HasPrefix(path, "/admin/api/sse/"):
		h.handleAdminAPI(w, r) // SSE endpoints handled inside admin API router

	case strings.HasPrefix(path, "/admin/api/"):
		h.handleAdminAPI(w, r)

	case strings.HasPrefix(path, "/user/api/"):
		h.handleUserAPI(w, r)

	case path == "/admin" || path == "/admin/":
		// 老 URL 兼容：直接 redirect 到根，让前端 history router 接管
		http.Redirect(w, r, "/", http.StatusMovedPermanently)

	case strings.HasPrefix(path, "/assets/"):
		// Serve Vue static assets (JS, CSS)
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.StripPrefix("/", http.FileServer(http.Dir(distDir))).ServeHTTP(w, r)

	case path == "/health":
		h.handleHealth(w, r)

	case path == "/v1/stats":
		if _, err := h.resolveApiKey(r); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing API key"})
			return
		}
		h.handleStats(w, r)

	default:
		// SPA fallback：任何非 API 非 assets 路径都返回 index.html，
		// 让前端 history mode router 接管（/login / /dashboard / /user/dashboard 等）。
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	}
}

// tokenRefreshLeadSec 是 token 过期前提前刷新的窗口（秒）。
//
// 实测 AWS 偶尔在 expiresAt 之前 ~10 分钟就让 token 失效（上游返回 400 INVALID_MODEL_ID
// 而非 401），所以必须提前刷新。30 分钟窗口给足缓冲，单 token 寿命通常 1 小时，仍可正常轮换。
const tokenRefreshLeadSec int64 = 1800

// ensureValidToken 确保 token 有效，过期前 30 分钟自动刷新
func (h *Handler) ensureValidToken(account *config.Account) error {
	return h.refreshAccountToken(account, false)
}

// forceRefreshToken 无视到期时间强制刷新（用于上游返回 INVALID_MODEL_ID 等"token 看着没过期但其实已废"的情形）
func (h *Handler) forceRefreshToken(account *config.Account) error {
	return h.refreshAccountToken(account, true)
}

func (h *Handler) refreshAccountToken(account *config.Account, force bool) error {
	if !force && (account.ExpiresAt == 0 || time.Now().Unix() < account.ExpiresAt-tokenRefreshLeadSec) {
		return nil
	}

	tag := "ensureValidToken"
	if force {
		tag = "forceRefreshToken"
	}
	fmt.Printf("[%s] Refreshing token for %s (expiresAt=%d, now=%d)\n",
		tag, account.Email, account.ExpiresAt, time.Now().Unix())

	accessToken, refreshToken, expiresAt, err := auth.RefreshToken(account)
	if err != nil {
		fmt.Printf("[%s] Token refresh FAILED for %s: %v\n", tag, account.Email, err)
		return err
	}

	h.pool.UpdateToken(account.ID, accessToken, refreshToken, expiresAt)
	account.AccessToken = accessToken
	if refreshToken != "" {
		account.RefreshToken = refreshToken
	}
	account.ExpiresAt = expiresAt

	config.UpdateAccountToken(account.ID, accessToken, refreshToken, expiresAt)
	fmt.Printf("[%s] Token refreshed OK for %s, new expiresAt=%d\n", tag, account.Email, expiresAt)

	return nil
}
