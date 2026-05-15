package proxy

import (
	"bufio"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ==================== 管理 API ====================
//
// 鉴权模型（v2 之后）：
//   - URL 显式拒绝 ?password=（旧版漏洞，无条件 401）
//   - POST /login            → apiAdminLogin（不要 session；走 IP 速率限制）
//   - GET  /sse/*            → 一次性 SSE token 验证（5min TTL，用过即焚）
//   - 其余                    → 必须带 admin_session cookie；unsafe method 还要 X-CSRF-Token
//
// 旧的「明文密码 header / cookie / query」三路兜底已全部移除。
func (h *Handler) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/api")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.URL.Query().Has("password") {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "password query is forbidden"})
		return
	}

	// /login 不要求 session，但要过 IP 速率限制
	if path == "/login" && r.Method == http.MethodPost {
		h.apiAdminLogin(w, r)
		return
	}

	// SSE 流：走一次性 token（/sse/token 自身仍走 session 分支）
	if strings.HasPrefix(path, "/sse/") && path != "/sse/token" {
		r2, ok := h.requireSSEToken(w, r, path)
		if !ok {
			return
		}
		h.routeAdminAPI(path, w, r2)
		return
	}

	sess, ok := h.requireAdminSession(w, r)
	if !ok {
		return
	}

	if isUnsafeMethod(r.Method) && !h.validateAdminCSRF(r, sess) {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "CSRF token required"})
		return
	}

	switch {
	case path == "/session" && r.Method == http.MethodGet:
		h.apiAdminSession(w, r, sess)
		return
	case path == "/logout" && r.Method == http.MethodPost:
		h.apiAdminLogout(w, r, sess)
		return
	case path == "/password" && r.Method == http.MethodPost:
		h.apiChangeAdminPassword(w, r, sess)
		return
	case path == "/sse/token" && r.Method == http.MethodPost:
		h.apiCreateSSEToken(w, r, sess)
		return
	}

	h.routeAdminAPI(path, w, r)
}

// routeAdminAPI 路由所有已鉴权的常规 admin endpoint。
// 调用方负责确保鉴权 + CSRF 已通过。
func (h *Handler) routeAdminAPI(path string, w http.ResponseWriter, r *http.Request) {
	switch {
	case path == "/accounts" && r.Method == "GET":
		h.apiGetAccounts(w, r)
	case path == "/accounts" && r.Method == "POST":
		h.apiAddAccount(w, r)
	case path == "/accounts/batch" && r.Method == "POST":
		h.apiBatchAccounts(w, r)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/refresh") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/refresh")
		h.apiRefreshAccount(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/models") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/models")
		h.apiGetAccountModels(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/full") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/full")
		h.apiGetAccountFull(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && r.Method == "DELETE":
		h.apiDeleteAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case strings.HasPrefix(path, "/accounts/") && r.Method == "PUT":
		h.apiUpdateAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case path == "/auth/iam-sso/start" && r.Method == "POST":
		h.apiStartIamSso(w, r)
	case path == "/auth/iam-sso/complete" && r.Method == "POST":
		h.apiCompleteIamSso(w, r)
	case path == "/auth/builderid/start" && r.Method == "POST":
		h.apiStartBuilderIdLogin(w, r)
	case path == "/auth/builderid/poll" && r.Method == "POST":
		h.apiPollBuilderIdAuth(w, r)
	case path == "/auth/sso-token" && r.Method == "POST":
		h.apiImportSsoToken(w, r)
	case path == "/auth/credentials" && r.Method == "POST":
		h.apiImportCredentials(w, r)
	case path == "/auth/credentials/batch" && r.Method == "POST":
		h.apiImportCredentialsBatch(w, r)
	case path == "/status" && r.Method == "GET":
		h.apiGetStatus(w, r)
	case path == "/settings" && r.Method == "GET":
		h.apiGetSettings(w, r)
	case path == "/settings" && r.Method == "POST":
		h.apiUpdateSettings(w, r)
	case path == "/stats" && r.Method == "GET":
		h.apiGetAdminStats(w, r)
	case path == "/stats/reset" && r.Method == "POST":
		h.apiResetStats(w, r)
	case path == "/generate-machine-id" && r.Method == "GET":
		h.apiGenerateMachineId(w, r)
	case path == "/thinking" && r.Method == "GET":
		h.apiGetThinkingConfig(w, r)
	case path == "/thinking" && r.Method == "POST":
		h.apiUpdateThinkingConfig(w, r)
	case path == "/endpoint" && r.Method == "GET":
		h.apiGetEndpointConfig(w, r)
	case path == "/endpoint" && r.Method == "POST":
		h.apiUpdateEndpointConfig(w, r)
	case path == "/concurrency" && r.Method == "GET":
		h.apiGetConcurrency(w, r)
	case path == "/concurrency" && r.Method == "POST":
		h.apiUpdateConcurrency(w, r)
	case path == "/version" && r.Method == "GET":
		h.apiGetVersion(w, r)
	case path == "/export" && r.Method == "POST":
		h.apiExportAccounts(w, r)
	case path == "/import/db" && r.Method == "POST":
		h.apiImportFromDB(w, r)
	case path == "/import/db-status" && r.Method == "GET":
		h.apiGetDBStatus(w, r)
	case path == "/apikeys" && r.Method == "GET":
		h.apiGetApiKeys(w, r)
	case path == "/apikeys" && r.Method == "POST":
		h.apiCreateApiKey(w, r)
	case strings.HasPrefix(path, "/apikeys/") && strings.HasSuffix(path, "/logs") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/apikeys/"), "/logs")
		h.apiGetApiKeyLogs(w, r, id)
	case strings.HasPrefix(path, "/apikeys/") && r.Method == "PUT":
		h.apiUpdateApiKey(w, r, strings.TrimPrefix(path, "/apikeys/"))
	case strings.HasPrefix(path, "/apikeys/") && r.Method == "DELETE":
		h.apiDeleteApiKey(w, r, strings.TrimPrefix(path, "/apikeys/"))
	case path == "/logs" && r.Method == "GET":
		h.apiGetLogs(w, r)
	case path == "/logs" && r.Method == "DELETE":
		h.apiClearLogs(w, r)
	case path == "/sse/logs" && r.Method == "GET":
		h.handleSSELogs(w, r)
	case path == "/sse/stats" && r.Method == "GET":
		h.handleSSEStats(w, r)
	case path == "/pricing-analysis" && r.Method == "GET":
		h.apiPricingAnalysis(w, r)

	// ==================== Channels & Sell Prices (v3) ====================
	case path == "/channels" && r.Method == "GET":
		h.apiListChannels(w, r)
	case path == "/channels" && r.Method == "POST":
		h.apiCreateChannel(w, r)
	case strings.HasPrefix(path, "/channels/") && strings.HasSuffix(path, "/test") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/channels/"), "/test")
		h.apiTestChannel(w, r, id)
	case strings.HasPrefix(path, "/channels/") && r.Method == "PUT":
		h.apiUpdateChannel(w, r, strings.TrimPrefix(path, "/channels/"))
	case strings.HasPrefix(path, "/channels/") && r.Method == "DELETE":
		h.apiDeleteChannel(w, r, strings.TrimPrefix(path, "/channels/"))
	case path == "/sell-prices" && r.Method == "GET":
		h.apiGetSellPrices(w, r)
	case path == "/sell-prices" && r.Method == "PUT":
		h.apiUpdateSellPrices(w, r)

	// ==================== Billing Management ====================
	case path == "/pricing" && r.Method == "GET":
		h.apiGetPricing(w, r)
	case path == "/pricing" && r.Method == "PUT":
		h.apiUpdatePricing(w, r)
	case path == "/stealth" && r.Method == "GET":
		h.apiGetStealth(w, r)
	case path == "/stealth" && r.Method == "PUT":
		h.apiUpdateStealth(w, r)
	case path == "/profit" && r.Method == "GET":
		h.apiGetProfit(w, r)
	case path == "/profit-include-gift" && r.Method == "POST":
		h.apiUpdateProfitIncludeGift(w, r)
	case path == "/cost-entry" && r.Method == "POST":
		h.apiAddCostEntry(w, r)
	case path == "/cost-entry" && r.Method == "DELETE":
		h.apiRemoveCostEntry(w, r)
	case path == "/codes" && r.Method == "GET":
		h.apiGetCodes(w, r)
	case path == "/codes" && r.Method == "POST":
		h.apiCreateCodes(w, r)
	case path == "/codes/cleanup" && r.Method == "POST":
		h.apiCleanupCodes(w, r)
	case strings.HasPrefix(path, "/codes/") && r.Method == "DELETE":
		code := strings.TrimPrefix(path, "/codes/")
		h.apiDeleteCode(w, r, code)
	case path == "/abuse" && r.Method == "GET":
		h.apiGetAbuse(w, r)
	case strings.HasPrefix(path, "/abuse/") && strings.HasSuffix(path, "/clear") && r.Method == "POST":
		keyID := strings.TrimSuffix(strings.TrimPrefix(path, "/abuse/"), "/clear")
		h.apiClearAbuse(w, r, keyID)

	// ==================== Engagement / Insights ====================
	case path == "/inactive-keys" && r.Method == "GET":
		h.apiInactiveKeys(w, r)
	case path == "/leaderboard" && r.Method == "GET":
		h.apiAdminLeaderboard(w, r)
	case path == "/leaderboard/config" && r.Method == "GET":
		h.apiGetLeaderboardConfig(w, r)
	case path == "/leaderboard/config" && r.Method == "PUT":
		h.apiUpdateLeaderboardConfig(w, r)
	case path == "/apikeys/clear-gift" && r.Method == "POST":
		h.apiClearAllGift(w, r)

	// ==================== Promotion / Recharge / Insights (新增) ====================
	case path == "/promotion" && r.Method == "GET":
		h.apiGetPromotion(w, r)
	case path == "/promotion" && r.Method == "PUT":
		h.apiUpdatePromotion(w, r)
	case path == "/promotion/whitelist" && r.Method == "POST":
		h.apiAddPromotionWhitelist(w, r)
	case strings.HasPrefix(path, "/promotion/whitelist/") && r.Method == "DELETE":
		kid := strings.TrimPrefix(path, "/promotion/whitelist/")
		h.apiRemovePromotionWhitelist(w, r, kid)
	case path == "/recharges" && r.Method == "GET":
		h.apiGetRecharges(w, r)
	case strings.HasPrefix(path, "/apikeys/") && strings.HasSuffix(path, "/recharges") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/apikeys/"), "/recharges")
		h.apiGetApiKeyRecharges(w, r, id)
	case path == "/insights/funnel" && r.Method == "GET":
		h.apiInsightsFunnel(w, r)
	case path == "/insights/whales" && r.Method == "GET":
		h.apiInsightsWhales(w, r)
	case path == "/insights/freeloaders" && r.Method == "GET":
		h.apiInsightsFreeloaders(w, r)
	case path == "/insights/daily" && r.Method == "GET":
		h.apiInsightsDaily(w, r)

	default:
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
	}
}

// ==================== 新版鉴权 endpoint ====================

// POST /admin/api/login
// Body: { "password": "..." }
// 200: { success, csrfToken, expiresAt }  + Set-Cookie admin_session
// 401: { error, remainingAttempts }
// 423: { error, locked, retryAfter }
func (h *Handler) apiAdminLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	ip := clientIP(r)
	if locked, retryAfter := h.adminSessions.limiter.IsLocked(ip); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		writeJSONStatus(w, http.StatusLocked, map[string]interface{}{
			"error":      "too many login failures, try later",
			"locked":     true,
			"retryAfter": int(retryAfter.Seconds()),
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if !config.VerifyAdminPassword(req.Password) {
		locked, retryAfter := h.adminSessions.limiter.RecordFailure(ip)
		if locked {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSONStatus(w, http.StatusLocked, map[string]interface{}{
				"error":      "too many login failures, try later",
				"locked":     true,
				"retryAfter": int(retryAfter.Seconds()),
			})
			return
		}
		writeJSONStatus(w, http.StatusUnauthorized, map[string]interface{}{
			"error":             "invalid password",
			"remainingAttempts": h.adminSessions.limiter.RemainingAttempts(ip),
		})
		return
	}

	h.adminSessions.limiter.RecordSuccess(ip)
	sess, err := h.adminSessions.Create(w, r)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"csrfToken": sess.CSRFToken,
		"expiresAt": sess.ExpiresAt.Unix(),
	})
}

// GET /admin/api/session - SPA 刷新页面时拿 csrfToken
func (h *Handler) apiAdminSession(w http.ResponseWriter, _ *http.Request, sess *adminSession) {
	writeJSONStatus(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"csrfToken": sess.CSRFToken,
		"expiresAt": sess.ExpiresAt.Unix(),
	})
}

// POST /admin/api/logout
func (h *Handler) apiAdminLogout(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	h.adminSessions.Invalidate(sess.TokenHash)
	h.adminSessions.ClearCookie(w, r)
	writeJSONStatus(w, http.StatusOK, map[string]bool{"success": true})
}

// POST /admin/api/password
// Body: { "oldPassword": "...", "newPassword": "...", "confirmPassword": "..." }
// 成功后所有 session 失效（踢出所有设备），客户端要重新登录
func (h *Handler) apiChangeAdminPassword(w http.ResponseWriter, r *http.Request, _ *adminSession) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	if config.IsPasswordEnvOverride() {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "password managed by ADMIN_PASSWORD env"})
		return
	}

	var req struct {
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "passwords do not match"})
		return
	}
	if len(req.NewPassword) < 12 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "password too short (min 12 chars)"})
		return
	}
	if err := config.ChangeAdminPassword(req.OldPassword, req.NewPassword); err != nil {
		// 旧密码错 → 401；hash/写盘失败 → 500（错误分类便于排障 + 前端正确提示）
		if errors.Is(err, config.ErrInvalidOldPassword) {
			writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		} else {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}

	h.adminSessions.InvalidateAll()
	h.adminSessions.ClearCookie(w, r)
	writeJSONStatus(w, http.StatusOK, map[string]bool{"success": true})
}

// POST /admin/api/sse/token
// Body: { "stream": "stats" | "logs" }
// 返回一次性 token（5min TTL），客户端拼到 EventSource URL：?sse_token=...
func (h *Handler) apiCreateSSEToken(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var req struct {
		Stream string `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.Stream != "stats" && req.Stream != "logs" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "stream must be stats or logs"})
		return
	}
	token, err := h.adminSessions.NewSSEToken(sess.TokenHash, req.Stream, adminSSETokenTTL)
	if err != nil {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) apiGetAccounts(w http.ResponseWriter, _ *http.Request) {
	accounts := config.GetAccounts()
	poolAccounts := h.pool.GetAllAccounts()
	statsMap := make(map[string]config.Account)
	for _, a := range poolAccounts {
		statsMap[a.ID] = a
	}
	result := make([]map[string]interface{}, len(accounts))
	for i, a := range accounts {
		stats := statsMap[a.ID]
		result[i] = map[string]interface{}{
			"id": a.ID, "email": a.Email, "userId": a.UserId, "nickname": a.Nickname,
			"authMethod": a.AuthMethod, "provider": a.Provider, "region": a.Region,
			"enabled": a.Enabled, "allowOverQuota": a.AllowOverQuota, "banStatus": a.BanStatus, "banReason": a.BanReason, "banTime": a.BanTime,
			"expiresAt": a.ExpiresAt, "hasToken": a.AccessToken != "", "machineId": a.MachineId, "weight": a.Weight,
			"subscriptionType": a.SubscriptionType, "subscriptionTitle": a.SubscriptionTitle, "daysRemaining": a.DaysRemaining,
			"usageCurrent": a.UsageCurrent, "usageLimit": a.UsageLimit, "usagePercent": a.UsagePercent,
			"nextResetDate": a.NextResetDate, "lastRefresh": a.LastRefresh,
			"trialUsageCurrent": a.TrialUsageCurrent, "trialUsageLimit": a.TrialUsageLimit,
			"trialUsagePercent": a.TrialUsagePercent, "trialStatus": a.TrialStatus, "trialExpiresAt": a.TrialExpiresAt,
			"requestCount": stats.RequestCount, "errorCount": stats.ErrorCount,
			"totalTokens": stats.TotalTokens, "totalCredits": stats.TotalCredits, "lastUsed": stats.LastUsed,
			"inFlight": h.pool.GetInFlight(a.ID),
		}
	}
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) apiAddAccount(w http.ResponseWriter, r *http.Request) {
	var account config.Account
	if err := json.NewDecoder(r.Body).Decode(&account); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if account.ID == "" {
		account.ID = auth.GenerateAccountID()
	}
	if account.Region == "" {
		account.Region = "us-east-1"
	}
	if account.MachineId == "" {
		account.MachineId = config.GenerateMachineId()
	}
	if account.Enabled == false && account.AccessToken == "" && account.RefreshToken == "" {
		account.Enabled = true
	}
	// Normalize authMethod casing
	switch strings.ToLower(account.AuthMethod) {
	case "idc", "builderid", "enterprise":
		account.AuthMethod = "idc"
	case "social", "google", "github":
		account.AuthMethod = "social"
	}
	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	// Async: populate real quota data immediately after adding
	accountID := account.ID
	go func() {
		accounts := config.GetAccounts()
		for i := range accounts {
			if accounts[i].ID == accountID {
				info, err := RefreshAccountInfo(&accounts[i])
				if err == nil {
					config.UpdateAccountInfo(accountID, *info)
				}
				return
			}
		}
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": account.ID})
}

func (h *Handler) apiDeleteAccount(w http.ResponseWriter, _ *http.Request, id string) {
	// 先获取账号 email，用于重置远程数据库状态
	var email string
	accounts := config.GetAccounts()
	for _, a := range accounts {
		if a.ID == id {
			email = a.Email
			break
		}
	}
	if err := config.DeleteAccount(id); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	// 异步重置远程数据库中该账号的 card_status，让它可以被重新导入
	if email != "" {
		go resetDBCardStatus([]string{email})
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiUpdateAccount(w http.ResponseWriter, r *http.Request, id string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	accounts := config.GetAccounts()
	var existing *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			existing = &accounts[i]
			break
		}
	}
	if existing == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}
	if v, ok := updates["enabled"].(bool); ok {
		existing.Enabled = v
	}
	if v, ok := updates["allowOverQuota"].(bool); ok {
		existing.AllowOverQuota = v
	}
	if v, ok := updates["nickname"].(string); ok {
		existing.Nickname = v
	}
	if v, ok := updates["machineId"].(string); ok {
		existing.MachineId = v
	}
	if v, ok := updates["weight"].(float64); ok {
		existing.Weight = int(v)
	}
	if err := config.UpdateAccount(id, *existing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// apiBatchAccounts 批量操作账号（启用/禁用/刷新/删除/设权重/导出）
func (h *Handler) apiBatchAccounts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
		Weight int      `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(req.IDs) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "No account IDs provided"})
		return
	}

	switch req.Action {
	case "enable", "disable":
		enabled := req.Action == "enable"
		accounts := config.GetAccounts()
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		for _, a := range accounts {
			if idSet[a.ID] {
				a.Enabled = enabled
				if enabled && a.BanStatus != "" && a.BanStatus != "ACTIVE" {
					a.BanStatus = "ACTIVE"
					a.BanReason = ""
					a.BanTime = 0
				}
				config.UpdateAccount(a.ID, a)
			}
		}
		h.pool.Reload()
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "count": len(req.IDs)})

	case "refresh":
		successCount := 0
		failCount := 0
		for _, id := range req.IDs {
			accounts := config.GetAccounts()
			var account *config.Account
			for i := range accounts {
				if accounts[i].ID == id {
					account = &accounts[i]
					break
				}
			}
			if account == nil {
				failCount++
				continue
			}
			if account.RefreshToken != "" {
				if newAccess, newRefresh, newExpires, err := auth.RefreshToken(account); err == nil {
					account.AccessToken = newAccess
					if newRefresh != "" {
						account.RefreshToken = newRefresh
					}
					account.ExpiresAt = newExpires
					config.UpdateAccountToken(id, newAccess, newRefresh, newExpires)
					h.pool.UpdateToken(id, newAccess, newRefresh, newExpires)
				}
			}
			info, err := RefreshAccountInfo(account)
			if err != nil {
				failCount++
				continue
			}
			config.UpdateAccountInfo(id, *info)
			successCount++
		}
		h.pool.Reload()
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "refreshed": successCount, "failed": failCount})

	case "delete":
		successCount := 0
		failCount := 0
		// 收集要删除账号的 email，用于重置远程数据库状态
		accounts := config.GetAccounts()
		emailMap := make(map[string]string)
		for _, a := range accounts {
			emailMap[a.ID] = a.Email
		}
		var deletedEmails []string
		for _, id := range req.IDs {
			if err := config.DeleteAccount(id); err != nil {
				failCount++
			} else {
				successCount++
				if e := emailMap[id]; e != "" {
					deletedEmails = append(deletedEmails, e)
				}
			}
		}
		h.pool.Reload()
		// 异步重置远程数据库中这些账号的 card_status
		if len(deletedEmails) > 0 {
			go resetDBCardStatus(deletedEmails)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "deleted": successCount, "failed": failCount})

	case "setWeight":
		accounts := config.GetAccounts()
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		count := 0
		for _, a := range accounts {
			if idSet[a.ID] {
				a.Weight = req.Weight
				config.UpdateAccount(a.ID, a)
				count++
			}
		}
		h.pool.Reload()
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "count": count})

	case "export":
		accounts := config.GetAccounts()
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		var exported []map[string]interface{}
		for _, a := range accounts {
			if idSet[a.ID] {
				exported = append(exported, map[string]interface{}{
					"id": a.ID, "email": a.Email, "accessToken": a.AccessToken,
					"refreshToken": a.RefreshToken, "clientId": a.ClientID, "clientSecret": a.ClientSecret,
					"authMethod": a.AuthMethod, "provider": a.Provider, "region": a.Region,
				})
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "accounts": exported})

	default:
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid action: " + req.Action})
	}
}

func (h *Handler) apiGetStatus(w http.ResponseWriter, _ *http.Request) {
	proPool := h.pool.TierStats("pro")
	freePool := h.pool.TierStats("free")
	proRemaining := (proPool.UsageLimit - proPool.UsageCurrent) + (proPool.TrialLimit - proPool.TrialCurrent)
	freeRemaining := (freePool.UsageLimit - freePool.UsageCurrent) + (freePool.TrialLimit - freePool.TrialCurrent)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accounts": h.pool.Count(), "available": h.pool.AvailableCount(),
		"totalRequests": h.totalRequests, "successRequests": h.successRequests,
		"failedRequests": h.failedRequests, "totalTokens": h.totalTokens,
		"totalCredits": h.totalCredits, "uptime": time.Now().Unix() - h.startTime,
		"freePool":       freePool,
		"proPool":        proPool,
		"prediction":     h.creditPredictor.Predict(proRemaining + freeRemaining),
		"proPrediction":  h.proCreditPredictor.Predict(proRemaining),
		"freePrediction": h.freeCreditPredictor.Predict(freeRemaining),
	})
}

func (h *Handler) apiGetSettings(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKey": config.GetApiKey(), "requireApiKey": config.IsApiKeyRequired(),
		"port": config.GetPort(), "host": config.GetHost(),
		"profitIncludeGift": config.GetProfitIncludeGift(),
	})
}

// POST /admin/api/profit-include-gift — 持久化"利润计算是否计入赠送"开关
func (h *Handler) apiUpdateProfitIncludeGift(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value bool `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateProfitIncludeGift(req.Value); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ApiKey        string  `json:"apiKey"`
		RequireApiKey bool    `json:"requireApiKey"`
		Password      *string `json:"password,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	// 改密走专用 endpoint，settings 不再接受 password 字段（防 admin 通过 /settings 改密码绕过 旧密码校验）
	if req.Password != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "use POST /admin/api/password to change admin password"})
		return
	}
	if err := config.UpdateSettings(req.ApiKey, req.RequireApiKey); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetAdminStats(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalRequests": atomic.LoadInt64(&h.totalRequests), "successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests": atomic.LoadInt64(&h.failedRequests), "totalTokens": atomic.LoadInt64(&h.totalTokens),
		"totalCredits": h.getCredits(), "uptime": time.Now().Unix() - h.startTime,
	})
}

func (h *Handler) apiResetStats(w http.ResponseWriter, _ *http.Request) {
	atomic.StoreInt64(&h.totalRequests, 0)
	atomic.StoreInt64(&h.successRequests, 0)
	atomic.StoreInt64(&h.failedRequests, 0)
	atomic.StoreInt64(&h.totalTokens, 0)
	h.creditsMu.Lock()
	h.totalCredits = 0
	h.creditsMu.Unlock()
	config.UpdateStats(0, 0, 0, 0, 0)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGenerateMachineId(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"machineId": config.GenerateMachineId()})
}

func (h *Handler) apiGetLogs(w http.ResponseWriter, r *http.Request) {
	h.callLogsMu.RLock()
	logs := make([]CallLog, len(h.callLogs))
	copy(logs, h.callLogs)
	h.callLogsMu.RUnlock()

	// get query params
	statusFilter := r.URL.Query().Get("status")
	keyFilter := r.URL.Query().Get("key")
	searchQuery := strings.ToLower(r.URL.Query().Get("search"))

	// filter logs
	var filteredLogs []CallLog
	for _, l := range logs {
		isErr := l.Error != "" || l.Status == "error"
		if statusFilter == "error" && !isErr {
			continue
		}
		if statusFilter == "success" && isErr {
			continue
		}
		if keyFilter != "" && keyFilter != "all" && l.ApiKeyID != keyFilter {
			continue
		}

		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(l.ActualModel), searchQuery) &&
				!strings.Contains(strings.ToLower(l.OriginalModel), searchQuery) &&
				!strings.Contains(strings.ToLower(l.Account), searchQuery) &&
				!strings.Contains(strings.ToLower(l.Error), searchQuery) &&
				!strings.Contains(strings.ToLower(l.RequestID), searchQuery) &&
				!strings.Contains(strings.ToLower(l.StopReason), searchQuery) {
				continue
			}
		}
		filteredLogs = append(filteredLogs, l)
	}

	// reverse so newest first
	for i, j := 0, len(filteredLogs)-1; i < j; i, j = i+1, j-1 {
		filteredLogs[i], filteredLogs[j] = filteredLogs[j], filteredLogs[i]
	}

	total := len(filteredLogs)

	// pagination: ?page=1&limit=50
	page := 1
	limit := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  filteredLogs[start:end],
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) apiClearLogs(w http.ResponseWriter, _ *http.Request) {
	h.callLogsMu.Lock()
	h.callLogs = nil
	h.callLogsMu.Unlock()
	// Also truncate the on-disk log file
	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	os.Truncate(logPath, 0)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) apiGetThinkingConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := config.GetThinkingConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suffix": cfg.Suffix, "openaiFormat": cfg.OpenAIFormat, "claudeFormat": cfg.ClaudeFormat,
	})
}

func (h *Handler) apiUpdateThinkingConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Suffix       string `json:"suffix"`
		OpenAIFormat string `json:"openaiFormat"`
		ClaudeFormat string `json:"claudeFormat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	validFormats := map[string]bool{"reasoning_content": true, "thinking": true, "think": true}
	if req.OpenAIFormat != "" && !validFormats[req.OpenAIFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid openaiFormat"})
		return
	}
	if req.ClaudeFormat != "" && !validFormats[req.ClaudeFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claudeFormat"})
		return
	}
	if err := config.UpdateThinkingConfig(req.Suffix, req.OpenAIFormat, req.ClaudeFormat); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetEndpointConfig(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"preferredEndpoint": config.GetPreferredEndpoint()})
}

func (h *Handler) apiUpdateEndpointConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PreferredEndpoint string `json:"preferredEndpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	valid := map[string]bool{"auto": true, "codewhisperer": true, "amazonq": true}
	if !valid[req.PreferredEndpoint] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid endpoint"})
		return
	}
	if err := config.UpdatePreferredEndpoint(req.PreferredEndpoint); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetConcurrency(w http.ResponseWriter, _ *http.Request) {
	perKey, perFree, perPro := config.GetConcurrencyConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maxConcurrentPerKey":       perKey,
		"maxInFlightPerAccountFree": perFree,
		"maxInFlightPerAccountPro":  perPro,
		"timedKeyRPM":               config.GetTimedKeyRPM(), // 0 = 走老兜底 200/min
	})
}

func (h *Handler) apiUpdateConcurrency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxConcurrentPerKey       int  `json:"maxConcurrentPerKey"`
		MaxInFlightPerAccount     int  `json:"maxInFlightPerAccount"`     // Legacy: sets both free and pro
		MaxInFlightPerAccountFree int  `json:"maxInFlightPerAccountFree"` // Per FREE account
		MaxInFlightPerAccountPro  int  `json:"maxInFlightPerAccountPro"`  // Per PRO account
		TimedKeyRPM               *int `json:"timedKeyRPM,omitempty"`     // 天卡 key 全局 RPM；nil=不改，0=禁用走老兜底，>0=新阈值
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.MaxConcurrentPerKey < 1 || req.MaxConcurrentPerKey > 200 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxConcurrentPerKey must be 1-200"})
		return
	}
	// Backward compat: if old unified field is set but new fields aren't, apply to both
	perFree := req.MaxInFlightPerAccountFree
	perPro := req.MaxInFlightPerAccountPro
	if req.MaxInFlightPerAccount > 0 {
		if perFree == 0 {
			perFree = req.MaxInFlightPerAccount
		}
		if perPro == 0 {
			perPro = req.MaxInFlightPerAccount
		}
	}
	if perFree > 0 && (perFree < 1 || perFree > 500) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxInFlightPerAccountFree must be 1-500"})
		return
	}
	if perPro > 0 && (perPro < 1 || perPro > 500) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxInFlightPerAccountPro must be 1-500"})
		return
	}
	if err := config.UpdateConcurrencyConfig(req.MaxConcurrentPerKey, perFree, perPro); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	// 单独处理 timedKeyRPM（指针：nil 表示不改）
	if req.TimedKeyRPM != nil {
		v := *req.TimedKeyRPM
		if v < 0 || v > 10000 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "timedKeyRPM must be 0-10000"})
			return
		}
		if err := config.UpdateTimedKeyRPM(v); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetVersion(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"version": config.Version})
}

// apiPricingAnalysis 定价分析 API — 为未来 AI 提供所有定价决策数据
// GET /admin/api/pricing-analysis
func (h *Handler) apiPricingAnalysis(w http.ResponseWriter, _ *http.Request) {
	h.callLogsMu.RLock()
	logs := make([]CallLog, len(h.callLogs))
	copy(logs, h.callLogs)
	h.callLogsMu.RUnlock()

	// 按模型统计
	type ModelStats struct {
		Requests      int     `json:"requests"`
		TotalCredits  float64 `json:"totalCredits"`
		TotalTokens   int     `json:"totalTokens"`
		AvgCredits    float64 `json:"avgCredits"`    // 平均每次 credit
		AvgTokens     int     `json:"avgTokens"`     // 平均每次 token
		CreditPerKTok float64 `json:"creditPerKTok"` // 每 1K token 的 credit 成本
		Errors        int     `json:"errors"`
	}
	modelMap := make(map[string]*ModelStats)

	var totalReqs, totalErrors int
	var totalCreditsAll float64
	var totalTokensAll int
	var firstTs, lastTs int64

	for _, log := range logs {
		totalReqs++
		if log.Timestamp > 0 {
			if firstTs == 0 || log.Timestamp < firstTs {
				firstTs = log.Timestamp
			}
			if log.Timestamp > lastTs {
				lastTs = log.Timestamp
			}
		}

		model := log.ActualModel
		if model == "" {
			model = log.OriginalModel
		}
		if _, ok := modelMap[model]; !ok {
			modelMap[model] = &ModelStats{}
		}
		ms := modelMap[model]
		ms.Requests++
		if log.Status == "error" {
			ms.Errors++
			totalErrors++
			continue
		}
		// v3 token 模式下 Credits=0 但 UpstreamCredits 保留了上游真实计费 — 用它做成本统计
		costCredits := log.Credits
		if log.BillingMode == "token" && log.UpstreamCredits > 0 {
			costCredits = log.UpstreamCredits
		}
		ms.TotalCredits += costCredits
		ms.TotalTokens += log.TotalTokens
		totalCreditsAll += costCredits
		totalTokensAll += log.TotalTokens
	}

	// 计算模型平均值
	for _, ms := range modelMap {
		successReqs := ms.Requests - ms.Errors
		if successReqs > 0 {
			ms.AvgCredits = ms.TotalCredits / float64(successReqs)
			ms.AvgTokens = ms.TotalTokens / successReqs
			if ms.TotalTokens > 0 {
				ms.CreditPerKTok = (ms.TotalCredits / float64(ms.TotalTokens)) * 1000
			}
		}
	}

	// 时间跨度
	var spanHours float64
	if lastTs > firstTs {
		spanHours = float64(lastTs-firstTs) / 3600.0
	}

	// 池数据
	proPool := h.pool.TierStats("pro")
	freePool := h.pool.TierStats("free")
	proUsed := proPool.UsageCurrent + proPool.TrialCurrent
	proTotal := proPool.UsageLimit + proPool.TrialLimit
	freeUsed := freePool.UsageCurrent + freePool.TrialCurrent
	freeTotal := freePool.UsageLimit + freePool.TrialLimit
	remaining := (proTotal - proUsed) + (freeTotal - freeUsed)

	prediction := h.creditPredictor.Predict(remaining)
	proPrediction := h.proCreditPredictor.Predict(proTotal - proUsed)
	freePrediction := h.freeCreditPredictor.Predict(freeTotal - freeUsed)

	// 成本计算：按池子区分成本
	pricing := config.GetPricing()
	var proCreditsAll, freeCreditsAll float64
	for _, log := range logs {
		if log.Status == "error" {
			continue
		}
		// 同上：token 模式用 UpstreamCredits 做成本统计
		costCredits := log.Credits
		if log.BillingMode == "token" && log.UpstreamCredits > 0 {
			costCredits = log.UpstreamCredits
		}
		pool := ResolveModelPool(log.ActualModel)
		if pool == "pro" {
			proCreditsAll += costCredits
		} else {
			freeCreditsAll += costCredits
		}
	}
	totalCostCNY := proCreditsAll*pricing.ProCostPerCredit() + freeCreditsAll*pricing.FreeCostPerCredit()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"summary": map[string]interface{}{
			"totalRequests":   totalReqs,
			"successRequests": totalReqs - totalErrors,
			"errorRequests":   totalErrors,
			"totalCredits":    totalCreditsAll,
			"totalTokens":     totalTokensAll,
			"spanHours":       spanHours,
			"avgCreditsPerReq": func() float64 {
				if totalReqs-totalErrors > 0 {
					return totalCreditsAll / float64(totalReqs-totalErrors)
				}
				return 0
			}(),
			"avgTokensPerReq": func() int {
				if totalReqs-totalErrors > 0 {
					return totalTokensAll / (totalReqs - totalErrors)
				}
				return 0
			}(),
			"creditsPerHour": func() float64 {
				if spanHours > 0 {
					return totalCreditsAll / spanHours
				}
				return 0
			}(),
			"totalCostCNY":     totalCostCNY,
			"costPerCreditCNY": map[string]float64{"pro": pricing.ProCostPerCredit(), "free": pricing.FreeCostPerCredit()},
		},
		"modelBreakdown": modelMap,
		"poolStatus": map[string]interface{}{
			"pro":  map[string]interface{}{"used": proUsed, "total": proTotal, "remaining": proTotal - proUsed, "accounts": proPool.Total},
			"free": map[string]interface{}{"used": freeUsed, "total": freeTotal, "remaining": freeTotal - freeUsed, "accounts": freePool.Total},
		},
		"prediction":     prediction,
		"proPrediction":  proPrediction,
		"freePrediction": freePrediction,
		"pricingHints": map[string]string{
			"costFormula":    "用户面板扣费 = 起步价 + (Token费 × 模型倍率)",
			"revenueFormula": "真实收入 = 用户面板扣费 × 0.2 元/刀",
			"costFormulaCNY": "真实成本 = Credit消耗 × 0.04 元/Credit",
			"breakEvenPanel": "一个号(1500 Credit)生命周期面板消耗 ≥ $300 = 保本",
			"profitPanel":    "面板消耗 ≥ $800 = 暴利，可降倍率抢客源",
			"avgTokenHint":   "平均Token<1K=聊天党,提高起步价; >15K=程序员,降起步价提倍率",
		},
	})
}

// ==================== API Key 管理 ====================

func (h *Handler) apiGetApiKeys(w http.ResponseWriter, _ *http.Request) {
	keys := config.GetAllApiKeys()
	// 合并内存中的实时统计
	h.apiKeyStatsMu.RLock()
	for i, k := range keys {
		if stats, ok := h.apiKeyStats[k.ID]; ok {
			keys[i].LastUsed = stats.LastUsed
			keys[i].Requests = stats.Requests
			keys[i].Errors = stats.Errors
			keys[i].Tokens = stats.Tokens
			keys[i].Credits = stats.Credits
			if stats.Models != nil {
				keys[i].Models = make(map[string]int64, len(stats.Models))
				for m, c := range stats.Models {
					keys[i].Models[m] = c
				}
			}
		}
	}
	h.apiKeyStatsMu.RUnlock()
	json.NewEncoder(w).Encode(keys)
}

func (h *Handler) apiCreateApiKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	key := config.ApiKeyInfo{
		ID:        config.GenerateMachineId(),
		Key:       config.GenerateApiKeyString(),
		Enabled:   true,
		Note:      req.Note,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(key); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(key)
}

func (h *Handler) apiUpdateApiKey(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Plan             *string  `json:"plan"`
		ExpiresAt        *int64   `json:"expiresAt"`
		Enabled          *bool    `json:"enabled"`
		Balance          *float64 `json:"balance"`
		GiftBalance      *float64 `json:"giftBalance"`
		Note             *string  `json:"note"`
		// 代理设置（不再有 ResellerDiscount —— 杠杆由 admin 出卡时手动定面值）
		IsReseller   *bool `json:"isReseller"`
		MaxChildKeys *int  `json:"maxChildKeys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	keys := config.GetAllApiKeys()
	var existing *config.ApiKeyInfo
	for i := range keys {
		if keys[i].ID == id {
			existing = &keys[i]
			break
		}
	}
	if existing == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "API key not found"})
		return
	}
	// 记录变更前状态（用于审计 + recharge_records 流水）
	beforeBalance := existing.Balance
	beforeGift := existing.GiftBalance
	beforeExpiresAt := existing.ExpiresAt

	if req.Plan != nil {
		existing.Plan = *req.Plan
	}
	if req.ExpiresAt != nil {
		existing.ExpiresAt = *req.ExpiresAt
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Balance != nil {
		existing.Balance = *req.Balance
	}
	if req.GiftBalance != nil {
		existing.GiftBalance = *req.GiftBalance
	}
	if req.Note != nil {
		existing.Note = *req.Note
	}
	// 代理设置：子 key 不允许开代理（防套娃）
	if req.IsReseller != nil {
		if existing.ParentKeyID != "" && *req.IsReseller {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "child key cannot become reseller"})
			return
		}
		existing.IsReseller = *req.IsReseller
		// 关闭代理时清空相关字段（保留 SoldToChildren 作为历史统计）
		if !existing.IsReseller {
			existing.MaxChildKeys = 0
			existing.ResellerDiscount = 0 // 历史字段：清零，新版不再使用
		}
	}
	if req.MaxChildKeys != nil && existing.IsReseller {
		if *req.MaxChildKeys < 0 {
			existing.MaxChildKeys = 0
		} else {
			existing.MaxChildKeys = *req.MaxChildKeys
		}
	}
	if err := config.UpdateApiKey(id, *existing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 审计 + 充值流水（如果 balance/gift/expiresAt 有变化）
	operator := operatorFromRequest(r)
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)

	// balance 变化 → 写流水
	if req.Balance != nil && existing.Balance != beforeBalance {
		delta := existing.Balance - beforeBalance
		appendRechargeRecord(RechargeRecord{
			Time: now.In(cst).Format("01-02 15:04:05"), Timestamp: now.Unix(),
			KeyID: existing.ID, KeyNote: existing.Note,
			Type: "admin_adjust", AmountUSD: delta, AmountCNY: delta * config.CNYPerUSDFace,
			BalanceBefore: beforeBalance, BalanceAfter: existing.Balance,
			GiftBefore: beforeGift, GiftAfter: existing.GiftBalance,
			Operator: operator, Note: "admin balance adjust",
		})
		AuditLog("apikey_balance_adjust", operator,
			fmt.Sprintf("keyID=%s before=$%.4f after=$%.4f delta=$%.4f", existing.ID, beforeBalance, existing.Balance, delta))
	}
	// gift 变化 → 写流水
	if req.GiftBalance != nil && existing.GiftBalance != beforeGift {
		delta := existing.GiftBalance - beforeGift
		appendRechargeRecord(RechargeRecord{
			Time: now.In(cst).Format("01-02 15:04:05"), Timestamp: now.Unix(),
			KeyID: existing.ID, KeyNote: existing.Note,
			Type: "admin_gift", AmountUSD: delta, AmountCNY: 0, // 赠送不算 CNY 充值
			BalanceBefore: beforeBalance, BalanceAfter: existing.Balance,
			GiftBefore: beforeGift, GiftAfter: existing.GiftBalance,
			Operator: operator, Note: "admin gift adjust",
		})
		AuditLog("apikey_gift_adjust", operator,
			fmt.Sprintf("keyID=%s before=$%.4f after=$%.4f delta=$%.4f", existing.ID, beforeGift, existing.GiftBalance, delta))
	}
	// ExpiresAt 变化 → audit（不写充值流水，但留痕方便排查"天卡消失"）
	if req.ExpiresAt != nil && existing.ExpiresAt != beforeExpiresAt {
		AuditLog("apikey_expires_change", operator,
			fmt.Sprintf("keyID=%s before=%d after=%d delta=%d", existing.ID, beforeExpiresAt, existing.ExpiresAt, existing.ExpiresAt-beforeExpiresAt))
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiDeleteApiKey(w http.ResponseWriter, _ *http.Request, id string) {
	if err := config.DeleteApiKey(id); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.apiKeyStatsMu.Lock()
	delete(h.apiKeyStats, id)
	h.apiKeyStatsMu.Unlock()
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetApiKeyLogs(w http.ResponseWriter, _ *http.Request, keyID string) {
	h.callLogsMu.RLock()
	var filtered []CallLog
	for _, log := range h.callLogs {
		if log.ApiKeyID == keyID {
			filtered = append(filtered, log)
		}
	}
	h.callLogsMu.RUnlock()
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": filtered})
}

// ==================== Billing Admin APIs ====================

// GET /admin/api/pricing
func (h *Handler) apiGetPricing(w http.ResponseWriter, _ *http.Request) {
	pricing := config.GetPricing()
	supported := SupportedModels()
	out := struct {
		config.PricingConfig
		SupportedModels map[string][]string  `json:"supportedModels"`
		Preview         []pricingPreviewRow  `json:"preview"` // v2: 给前端表格用的扁平视图
		Promotion       *promotionPreviewOut `json:"promotionPreview,omitempty"`
	}{
		PricingConfig:   pricing,
		SupportedModels: supported,
		Preview:         buildPricingPreview(pricing, supported),
		Promotion:       buildPromotionPreview(pricing, supported),
	}
	json.NewEncoder(w).Encode(out)
}

// pricingPreviewRow 给前端表格的一行：每个 model 一条。
type pricingPreviewRow struct {
	Model              string  `json:"model"`
	Pool               string  `json:"pool"`
	PriceUSD           float64 `json:"priceUSD"`           // 实际生效（ModelPrices 命中或 Default 兜底）
	IsDefault          bool    `json:"isDefault"`          // true = 走的是 DefaultProPriceUSD/DefaultFreePriceUSD
	PriceCNYPerCredit  float64 `json:"priceCNYPerCredit"`  // priceUSD × CNYPerUSDFace（0.05）
	CostCNYPerCredit   float64 `json:"costCNYPerCredit"`   // 采购成本（成本端，不是定价）
	CostIsFallback     bool    `json:"costIsFallback"`     // true = 没有真实成本数据（CostEntries 空），cost 是 hardcoded 兜底；前端应显示 "—" 而非真实利润率
	MarginPercent      float64 `json:"marginPercent"`      // (revenue - cost) / revenue × 100；CostIsFallback=true 时无意义
	LegacyPriceUSD     float64 `json:"legacyPriceUSD"`     // shadow：旧公式 v1 算的价（前端可比对）
	LegacyEqualsActual bool    `json:"legacyEqualsActual"` // 新旧是否一致（迁移正确）
}

func buildPricingPreview(p config.PricingConfig, supported map[string][]string) []pricingPreviewRow {
	rows := make([]pricingPreviewRow, 0)
	seen := map[string]bool{}

	emit := func(model, pool string) {
		if seen[model] {
			return
		}
		seen[model] = true
		actual := ModelPriceUSD(model)
		fromMap := lookupModelPrice(p.ModelPrices, model) > 0
		legacy := LegacyModelPriceUSD(model)
		var costPer float64
		var costIsFallback bool
		if pool == "pro" {
			costPer = p.ProCostPerCredit()
			costIsFallback = len(p.ProCostEntries) == 0 &&
				!(p.ProAccountCredits > 0 && p.ProAccountPriceCNY > 0) &&
				p.PurchasePriceCNY <= 0
		} else {
			costPer = p.FreeCostPerCredit()
			costIsFallback = len(p.FreeCostEntries) == 0 &&
				!(p.FreeAccountBatchCount > 0 && p.FreeAccountCredits > 0 && p.FreeAccountBatchPrice > 0)
		}
		revenuePer := actual * config.CNYPerUSDFace
		margin := 0.0
		if revenuePer > 0 {
			margin = (revenuePer - costPer) / revenuePer * 100
		}
		rows = append(rows, pricingPreviewRow{
			Model:              model,
			Pool:               pool,
			PriceUSD:           actual,
			IsDefault:          !fromMap,
			PriceCNYPerCredit:  revenuePer,
			CostCNYPerCredit:   costPer,
			CostIsFallback:     costIsFallback,
			MarginPercent:      margin,
			LegacyPriceUSD:     legacy,
			LegacyEqualsActual: math.Abs(actual-legacy) < 0.0001,
		})
	}

	// 1. supportedModels 里的 model 先列
	for _, m := range supported["pro"] {
		emit(m, "pro")
	}
	for _, m := range supported["free"] {
		emit(m, "free")
	}
	// 2. ModelPrices 里有但 supportedModels 没的（admin 自定义的）后追加
	for m := range p.ModelPrices {
		emit(m, ResolveModelPool(m))
	}
	return rows
}

// promotionPreviewOut 在活动 tab 给前端用的"原价 → 活动价 → 折扣"对照
type promotionPreviewOut struct {
	Enabled bool                  `json:"enabled"`
	Rows    []promotionPreviewRow `json:"rows"`
}

type promotionPreviewRow struct {
	Model           string  `json:"model"`
	Pool            string  `json:"pool"`
	OriginalUSD     float64 `json:"originalUSD"`
	PromoUSD        float64 `json:"promoUSD"`
	IsPromoDefault  bool    `json:"isPromoDefault"` // true = 活动期走的是 promo.DefaultPro/FreePriceUSD
	DiscountPercent float64 `json:"discountPercent"`
}

func buildPromotionPreview(p config.PricingConfig, supported map[string][]string) *promotionPreviewOut {
	promo := config.GetPromotion()
	if promo == nil {
		return nil
	}
	out := &promotionPreviewOut{Enabled: promo.Enabled}
	allModels := []string{}
	for _, m := range supported["pro"] {
		allModels = append(allModels, m)
	}
	for _, m := range supported["free"] {
		allModels = append(allModels, m)
	}
	for m := range p.ModelPrices {
		// dedup
		dup := false
		for _, x := range allModels {
			if normalizeModelKey(strings.ToLower(x)) == normalizeModelKey(strings.ToLower(m)) {
				dup = true
				break
			}
		}
		if !dup {
			allModels = append(allModels, m)
		}
	}
	for _, m := range allModels {
		pool := ResolveModelPool(m)
		original := ModelPriceUSD(m)
		// 活动期单价：lookup promo.ModelPrices → 否则 promo.DefaultPro/FreePriceUSD → 否则原价
		promoPrice := lookupModelPrice(promo.ModelPrices, m)
		isDefault := false
		if promoPrice <= 0 {
			isDefault = true
			if pool == "pro" {
				promoPrice = promo.DefaultProPriceUSD
			} else {
				promoPrice = promo.DefaultFreePriceUSD
			}
		}
		if promoPrice <= 0 {
			promoPrice = original
			isDefault = false
		}
		discount := 0.0
		if original > 0 {
			discount = (1 - promoPrice/original) * 100
		}
		out.Rows = append(out.Rows, promotionPreviewRow{
			Model:           m,
			Pool:            pool,
			OriginalUSD:     original,
			PromoUSD:        promoPrice,
			IsPromoDefault:  isDefault,
			DiscountPercent: discount,
		})
	}
	return out
}

// PUT /admin/api/pricing
func (h *Handler) apiUpdatePricing(w http.ResponseWriter, r *http.Request) {
	var pricing config.PricingConfig
	if err := json.NewDecoder(r.Body).Decode(&pricing); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	// v1→v2 兜底：admin 用旧 Pricing UI（只发 ProPoolPriceUSD/FreePoolPriceUSD/ModelMultipliers，
	// 不发 ModelPrices 或 Default*）时，自动算出 ModelPrices 让计费立刻生效。
	// 判定"v1 UI"：ModelPrices 空 + DefaultPro/FreePriceUSD 都为 0。
	if len(pricing.ModelPrices) == 0 && pricing.DefaultProPriceUSD == 0 && pricing.DefaultFreePriceUSD == 0 {
		config.MigratePricingToModelLevel(&pricing)
	}
	if err := config.UpdatePricing(pricing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("pricing_update", "admin",
		fmt.Sprintf("modelPrices=%d entries defaultPro=$%.4f defaultFree=$%.4f freePool=$%.4f proPool=$%.4f purchaseCNY=¥%.4f",
			len(pricing.ModelPrices), pricing.DefaultProPriceUSD, pricing.DefaultFreePriceUSD,
			pricing.FreePoolPriceUSD, pricing.ProPoolPriceUSD, pricing.PurchasePriceCNY))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/stealth
func (h *Handler) apiGetStealth(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(config.GetStealth())
}

// PUT /admin/api/stealth
func (h *Handler) apiUpdateStealth(w http.ResponseWriter, r *http.Request) {
	var s config.StealthConfig
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateStealth(s); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("stealth_update", "admin", fmt.Sprintf("enabled=%v opus=%.2f sonnet=%.2f opusTarget=%s sonnetTarget=%s",
		s.Enabled, s.OpusFakeRatio, s.SonnetFakeRatio, s.OpusFakeTarget, s.SonnetFakeTarget))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/profit?period=this_month|last_month|7d|30d|all|custom&from=&to=&include_gift=true|false
//
// 真现金口径：
//   revenue = 兑换流水里 type=code_redeem*（实际收款） + (可选) admin_gift（赠送总额）
//   cost    = period 内新增的 CostEntry.CostCNY 之和
//
// 查询参数：
//   period       — 时间窗口预设；custom 时配合 from/to。默认 this_month
//   from / to    — unix seconds，仅 period=custom 时生效
//   include_gift — true|false；不传时回退 Settings.ProfitIncludeGift
func (h *Handler) apiGetProfit(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "this_month"
	}
	from, to := resolveProfitPeriod(period, r.URL.Query().Get("from"), r.URL.Query().Get("to"))

	includeGiftQ := r.URL.Query().Get("include_gift")
	includeGift := config.GetProfitIncludeGift() // 不传时回退持久化偏好
	if includeGiftQ == "true" {
		includeGift = true
	} else if includeGiftQ == "false" {
		includeGift = false
	}

	balanceCNY, timeCardsCNY, giftCNY := aggregateRechargeRevenue(from, to)

	pricing := config.GetPricing()
	var proCost, freeCost float64
	for _, e := range pricing.ProCostEntries {
		if entryInWindow(e.CreatedAt, from, to) {
			proCost += e.CostCNY
		}
	}
	for _, e := range pricing.FreeCostEntries {
		if entryInWindow(e.CreatedAt, from, to) {
			freeCost += e.CostCNY
		}
	}

	revenueCNY := balanceCNY + timeCardsCNY
	if includeGift {
		revenueCNY += giftCNY
	}
	costCNY := proCost + freeCost
	profitCNY := revenueCNY - costCNY
	margin := 0.0
	if revenueCNY > 0 {
		margin = profitCNY / revenueCNY * 100
	}

	resp := map[string]interface{}{
		"period":        period,
		"from":          from,
		"to":            to,
		"include_gift":  includeGift,
		"revenue_cny":   revenueCNY,
		"revenue_breakdown": map[string]float64{
			"balance_cards": balanceCNY,
			"time_cards":    timeCardsCNY,
			"gift":          map[bool]float64{true: giftCNY, false: 0}[includeGift],
		},
		"cost_cny": costCNY,
		"cost_breakdown": map[string]float64{
			"pro":  proCost,
			"free": freeCost,
		},
		"profit_cny":     profitCNY,
		"margin_percent": margin,
	}
	writeJSON(w, 200, resp)
}

// resolveProfitPeriod 把 period 字符串转成 [from, to] 秒时间戳（CST 边界）。
// custom 模式读 fromStr/toStr。其它模式以 CST 自然月/日为界。
func resolveProfitPeriod(period, fromStr, toStr string) (int64, int64) {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	switch period {
	case "all":
		return 0, math.MaxInt32 // 足够远未来
	case "custom":
		from, _ := strconv.ParseInt(fromStr, 10, 64)
		to, _ := strconv.ParseInt(toStr, 10, 64)
		if to <= 0 {
			to = math.MaxInt32
		}
		return from, to
	case "7d":
		return now.Add(-7 * 24 * time.Hour).Unix(), now.Unix()
	case "30d":
		return now.Add(-30 * 24 * time.Hour).Unix(), now.Unix()
	case "last_month":
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst)
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
		return lastMonthStart.Unix(), thisMonthStart.Unix() - 1
	case "this_month":
		fallthrough
	default:
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst)
		return thisMonthStart.Unix(), now.Unix()
	}
}

// entryInWindow 判断 CostEntry 时间戳是否在 [from, to]。
// 老 entry CreatedAt=0 时归到"all-time"桶 — 即视为永远在窗口内（避免历史数据丢失）。
func entryInWindow(createdAt, from, to int64) bool {
	if createdAt == 0 {
		return true
	}
	return createdAt >= from && createdAt <= to
}

// aggregateRechargeRevenue 扫 recharge_records.jsonl，按 type 分桶累加 amountCNY。
// 仅返回 [from, to] 内的记录。
func aggregateRechargeRevenue(from, to int64) (balanceCNY, timeCardsCNY, giftCNY float64) {
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return 0, 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var rec RechargeRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.Timestamp < from || rec.Timestamp > to {
			continue
		}
		switch rec.Type {
		case "code_redeem":
			balanceCNY += rec.AmountCNY
		case "code_redeem_days":
			timeCardsCNY += rec.AmountCNY
		case "admin_gift":
			giftCNY += rec.AmountCNY
		}
	}
	return
}

// POST /admin/api/cost-entry
func (h *Handler) apiAddCostEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool  string           `json:"pool"` // "pro" or "free"
		Entry config.CostEntry `json:"entry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Pool != "pro" && req.Pool != "free" {
		writeJSON(w, 400, map[string]string{"error": "pool must be 'pro' or 'free'"})
		return
	}
	if req.Entry.Count <= 0 || req.Entry.CostCNY <= 0 {
		writeJSON(w, 400, map[string]string{"error": "count and costCNY must be > 0"})
		return
	}
	if req.Pool == "pro" && req.Entry.Credits <= 0 {
		writeJSON(w, 400, map[string]string{"error": "credits must be > 0 for PRO"})
		return
	}
	if err := config.AddCostEntry(req.Pool, req.Entry); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("cost_entry_add", "admin", fmt.Sprintf("pool=%s count=%d cost=¥%.2f", req.Pool, req.Entry.Count, req.Entry.CostCNY))
	writeJSON(w, 200, map[string]bool{"success": true})
}

// DELETE /admin/api/cost-entry
func (h *Handler) apiRemoveCostEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool string `json:"pool"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := config.RemoveCostEntry(req.Pool, req.ID); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("cost_entry_remove", "admin", fmt.Sprintf("pool=%s id=%s", req.Pool, req.ID))
	writeJSON(w, 200, map[string]bool{"success": true})
}

func (h *Handler) apiGetCodes(w http.ResponseWriter, _ *http.Request) {
	codes := config.GetActivationCodes()
	if codes == nil {
		codes = []config.ActivationCode{}
	}

	// Filter out used codes
	activeCodes := []config.ActivationCode{}
	for _, c := range codes {
		if !c.Used {
			activeCodes = append(activeCodes, c)
		}
	}

	json.NewEncoder(w).Encode(activeCodes)
}

// POST /admin/api/codes/cleanup - physically remove all used codes from data store
func (h *Handler) apiCleanupCodes(w http.ResponseWriter, _ *http.Request) {
	cleaned := config.CleanupUsedCodes()
	AuditLog("codes_cleanup", "admin", fmt.Sprintf("Removed %d used codes", cleaned))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"cleaned": cleaned,
	})
}

// POST /admin/api/codes - batch create activation codes
func (h *Handler) apiCreateCodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string  `json:"type"`         // "balance" | "days" | "time"
		Amount       float64 `json:"amount"`       // CNY (for balance) or days/seconds (for days/time)
		Tier         string  `json:"tier"`         // "free" | "pro" (only for type=days/time)
		Count        int     `json:"count"`        // how many codes to generate
		Note         string  `json:"note"`
		SalePriceCNY float64 `json:"salePriceCNY"` // 仅 days/time：admin 卖给客户的实际价格（¥），用于利润计算
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.Type != "balance" && req.Type != "days" && req.Type != "time" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "type must be 'balance', 'days', or 'time'"})
		return
	}
	if req.Amount <= 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "amount must be positive"})
		return
	}
	if req.Count <= 0 || req.Count > 100 {
		req.Count = 1
	}

	var codes []string
	now := time.Now().Unix()
	for i := 0; i < req.Count; i++ {
		code := generateActivationCode()
		ac := config.ActivationCode{
			Code:      code,
			Type:      req.Type,
			Amount:    req.Amount,
			CreatedAt: now,
			Note:      req.Note,
		}
		if (req.Type == "days" || req.Type == "time") && (req.Tier == "free" || req.Tier == "pro") {
			ac.Tier = req.Tier
		}
		// 仅天卡需要"售价"字段；balance 类型 amount 本身就是 CNY 售价，不重复填
		if (req.Type == "days" || req.Type == "time") && req.SalePriceCNY > 0 {
			ac.SalePriceCNY = req.SalePriceCNY
		}
		if err := config.AddActivationCode(ac); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		codes = append(codes, code)
	}

	AuditLog("codes_create", "admin", fmt.Sprintf("type=%s amount=%.2f count=%d note=%s", req.Type, req.Amount, len(codes), req.Note))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"codes":   codes,
		"count":   len(codes),
	})
}

// DELETE /admin/api/codes/:code
func (h *Handler) apiDeleteCode(w http.ResponseWriter, _ *http.Request, code string) {
	if err := config.DeleteActivationCode(code); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("code_delete", "admin", fmt.Sprintf("code=%s", code))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/abuse
func (h *Handler) apiGetAbuse(w http.ResponseWriter, _ *http.Request) {
	flagged := GetFlaggedKeys()
	if flagged == nil {
		flagged = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(flagged)
}

// POST /admin/api/abuse/:keyId/clear
func (h *Handler) apiClearAbuse(w http.ResponseWriter, _ *http.Request, keyID string) {
	ClearFlag(keyID)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ==================== Engagement / Insights handlers ====================

// apiInactiveKeys returns API keys idle for >= ?days (default 30).
// daysIdle is computed from LastUsed; if LastUsed=0 (never used), CreatedAt is the anchor.
func (h *Handler) apiInactiveKeys(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 3650 {
			days = v
		}
	}
	keys := config.GetAllApiKeys()
	// merge in-memory stats
	h.apiKeyStatsMu.RLock()
	for i := range keys {
		if stats, ok := h.apiKeyStats[keys[i].ID]; ok {
			keys[i].LastUsed = stats.LastUsed
			keys[i].Requests = stats.Requests
		}
	}
	h.apiKeyStatsMu.RUnlock()

	now := time.Now().Unix()
	type inactiveKey struct {
		ID          string  `json:"id"`
		Note        string  `json:"note"`
		LastUsed    int64   `json:"lastUsed"`
		CreatedAt   int64   `json:"createdAt"`
		DaysIdle    int     `json:"daysIdle"`
		NeverUsed   bool    `json:"neverUsed"`
		Balance     float64 `json:"balance"`
		GiftBalance float64 `json:"giftBalance"`
		Requests    int64   `json:"requests"`
		Enabled     bool    `json:"enabled"`
	}
	out := make([]inactiveKey, 0)
	for _, k := range keys {
		anchor := k.LastUsed
		never := false
		if anchor == 0 {
			anchor = k.CreatedAt
			never = true
		}
		if anchor == 0 {
			continue
		}
		idle := int((now - anchor) / 86400)
		if idle < days {
			continue
		}
		out = append(out, inactiveKey{
			ID:          k.ID,
			Note:        k.Note,
			LastUsed:    k.LastUsed,
			CreatedAt:   k.CreatedAt,
			DaysIdle:    idle,
			NeverUsed:   never,
			Balance:     k.Balance,
			GiftBalance: k.GiftBalance,
			Requests:    k.Requests,
			Enabled:     k.Enabled,
		})
	}
	// sort by daysIdle DESC
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DaysIdle > out[i].DaysIdle {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"days":  days,
		"count": len(out),
		"keys":  out,
	})
}

// GET /admin/api/leaderboard/config
func (h *Handler) apiGetLeaderboardConfig(w http.ResponseWriter, _ *http.Request) {
	enabled, fakeUsers := config.GetLeaderboardConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":   enabled,
		"fakeUsers": fakeUsers,
	})
}

// PUT /admin/api/leaderboard/config
func (h *Handler) apiUpdateLeaderboardConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled   bool `json:"enabled"`
		FakeUsers int  `json:"fakeUsers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateLeaderboardConfig(req.Enabled, req.FakeUsers); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("leaderboard_config_update", "admin", fmt.Sprintf("enabled=%v fakeUsers=%d", req.Enabled, req.FakeUsers))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /admin/api/apikeys/clear-gift
// Body must contain {"confirm": true}. Zeros GiftBalance on every key (does NOT touch Balance / TotalGifted).
func (h *Handler) apiClearAllGift(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if !req.Confirm {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "confirm=true is required"})
		return
	}
	count, total := config.ClearAllGiftBalances()
	AuditLog("clear_all_gift", "admin", fmt.Sprintf("cleared=%d totalGiftCleared=$%.4f", count, total))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"cleared":          count,
		"totalGiftCleared": total,
	})
}

// generateActivationCode creates a code like KIRO-XXXX-XXXX-XXXX
func generateActivationCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1
	seg := func() string {
		b := make([]byte, 4)
		randBytes := make([]byte, 4)
		crand.Read(randBytes)
		for i := range b {
			b[i] = chars[int(randBytes[i])%len(chars)]
		}
		return string(b)
	}
	return "KIRO-" + seg() + "-" + seg() + "-" + seg()
}
