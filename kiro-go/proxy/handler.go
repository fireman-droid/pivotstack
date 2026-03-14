package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"kiro-api-proxy/pool"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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
	// SSE 实时日志订阅
	logSubscribers   map[chan CallLog]bool
	logSubscribersMu sync.RWMutex
	// Credit 消耗预测
	creditPredictor *CreditPredictor
}

// CallLog 单次调用记录（结构化日志）
type CallLog struct {
	Time            string  `json:"time"`
	Timestamp       int64   `json:"timestamp"`
	APIType         string  `json:"api_type"`
	OriginalModel   string  `json:"original_model"`
	ActualModel     string  `json:"actual_model"`
	Account         string  `json:"account"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Credits         float64 `json:"credits,omitempty"`
	Stream          bool    `json:"stream"`
	Error           string  `json:"error,omitempty"`
	PayloadKB       int     `json:"payload_kb,omitempty"`
	Status          string  `json:"status"`
	Attempt         int     `json:"attempt,omitempty"`
	Subscription    string  `json:"subscription,omitempty"`
	RequestSummary  string  `json:"request_summary,omitempty"`
	ResponseSummary string  `json:"response_summary,omitempty"`
}

const maxCallLogs = 500

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
		pool:            pool.GetPool(),
		totalRequests:   int64(totalReq),
		successRequests: int64(successReq),
		failedRequests:  int64(failedReq),
		totalTokens:     int64(totalTokens),
		totalCredits:    totalCredits,
		startTime:       time.Now().Unix(),
		stopRefresh:     make(chan struct{}),
		stopStatsSaver:  make(chan struct{}),
		logSubscribers:  make(map[chan CallLog]bool),
		creditPredictor: newCreditPredictor(200, 0.3),
	}
	// 从磁盘恢复历史日志和 CreditPredictor
	h.loadLogsFromDisk()
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

// validateApiKey 验证 API Key
func (h *Handler) validateApiKey(r *http.Request) bool {
	if !config.IsApiKeyRequired() {
		return true
	}

	expectedKey := config.GetApiKey()
	if expectedKey == "" {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-Api-Key")

	var providedKey string
	if strings.HasPrefix(authHeader, "Bearer ") {
		providedKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else if apiKeyHeader != "" {
		providedKey = apiKeyHeader
	}

	return providedKey == expectedKey
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
		if !h.validateApiKey(r) {
			h.sendClaudeError(w, 401, "authentication_error", "Invalid or missing API key")
			return
		}
		h.handleClaudeMessages(w, r)
	case path == "/v1/messages/count_tokens" || path == "/messages/count_tokens":
		if !h.validateApiKey(r) {
			h.sendClaudeError(w, 401, "authentication_error", "Invalid or missing API key")
			return
		}
		h.handleCountTokens(w, r)
	case path == "/v1/chat/completions" || path == "/chat/completions":
		if !h.validateApiKey(r) {
			h.sendOpenAIError(w, 401, "authentication_error", "Invalid or missing API key")
			return
		}
		h.handleOpenAIChat(w, r)
	case path == "/v1/models" || path == "/models":
		h.handleModels(w, r)
	case path == "/api/event_logging/batch":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"status":"ok"}`))

	case strings.HasPrefix(path, "/admin/api/sse/"):
		h.handleAdminAPI(w, r) // SSE endpoints handled inside admin API router

	case strings.HasPrefix(path, "/admin/api/"):
		h.handleAdminAPI(w, r)

	case path == "/admin" || path == "/admin/" || strings.HasPrefix(path, "/admin/"):
		// Serve Vue frontend (SPA fallback)
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))

	case strings.HasPrefix(path, "/assets/"):
		// Serve Vue static assets (JS, CSS)
		distDir := "web-vue/dist"
		if _, err := os.Stat(distDir); os.IsNotExist(err) {
			distDir = "/app/web-vue/dist"
		}
		http.StripPrefix("/", http.FileServer(http.Dir(distDir))).ServeHTTP(w, r)

	case path == "/health" || path == "/":
		h.handleHealth(w, r)

	case path == "/v1/stats":
		if !h.validateApiKey(r) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing API key"})
			return
		}
		h.handleStats(w, r)

	default:
		http.Error(w, "Not Found", 404)
	}
}

// ensureValidToken 确保 token 有效，过期前 10 分钟自动刷新
func (h *Handler) ensureValidToken(account *config.Account) error {
	if account.ExpiresAt == 0 || time.Now().Unix() < account.ExpiresAt-600 {
		return nil
	}

	fmt.Printf("[ensureValidToken] Refreshing token for %s (expiresAt=%d, now=%d)\n",
		account.Email, account.ExpiresAt, time.Now().Unix())

	accessToken, refreshToken, expiresAt, err := auth.RefreshToken(account)
	if err != nil {
		fmt.Printf("[ensureValidToken] Token refresh FAILED for %s: %v\n", account.Email, err)
		return err
	}

	h.pool.UpdateToken(account.ID, accessToken, refreshToken, expiresAt)
	account.AccessToken = accessToken
	if refreshToken != "" {
		account.RefreshToken = refreshToken
	}
	account.ExpiresAt = expiresAt

	config.UpdateAccountToken(account.ID, accessToken, refreshToken, expiresAt)
	fmt.Printf("[ensureValidToken] Token refreshed OK for %s, new expiresAt=%d\n", account.Email, expiresAt)

	return nil
}
