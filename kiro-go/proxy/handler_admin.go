package proxy

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ==================== 管理 API ====================

func (h *Handler) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		cookie, _ := r.Cookie("admin_password")
		if cookie != nil {
			password = cookie.Value
		}
	}
	// SSE EventSource 不支持自定义 Header/Cookie，支持 query 参数
	if password == "" {
		password = r.URL.Query().Get("password")
	}

	if password != config.GetPassword() {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/api")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

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

	// ==================== Billing Management ====================
	case path == "/pricing" && r.Method == "GET":
		h.apiGetPricing(w, r)
	case path == "/pricing" && r.Method == "PUT":
		h.apiUpdatePricing(w, r)
	case path == "/profit" && r.Method == "GET":
		h.apiGetProfit(w, r)
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

	default:
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
	}
}

func (h *Handler) apiGetAccounts(w http.ResponseWriter, r *http.Request) {
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
			"enabled": a.Enabled, "banStatus": a.BanStatus, "banReason": a.BanReason, "banTime": a.BanTime,
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

func (h *Handler) apiDeleteAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := config.DeleteAccount(id); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
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
		for _, id := range req.IDs {
			if err := config.DeleteAccount(id); err != nil {
				failCount++
			} else {
				successCount++
			}
		}
		h.pool.Reload()
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

func (h *Handler) apiGetStatus(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKey": config.GetApiKey(), "requireApiKey": config.IsApiKeyRequired(),
		"port": config.GetPort(), "host": config.GetHost(),
	})
}

func (h *Handler) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ApiKey        string `json:"apiKey"`
		RequireApiKey bool   `json:"requireApiKey"`
		Password      string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateSettings(req.ApiKey, req.RequireApiKey, req.Password); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetAdminStats(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalRequests": atomic.LoadInt64(&h.totalRequests), "successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests": atomic.LoadInt64(&h.failedRequests), "totalTokens": atomic.LoadInt64(&h.totalTokens),
		"totalCredits": h.getCredits(), "uptime": time.Now().Unix() - h.startTime,
	})
}

func (h *Handler) apiResetStats(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) apiGenerateMachineId(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) apiClearLogs(w http.ResponseWriter, r *http.Request) {
	h.callLogsMu.Lock()
	h.callLogs = nil
	h.callLogsMu.Unlock()
	// Also truncate the on-disk log file
	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	os.Truncate(logPath, 0)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) apiGetThinkingConfig(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) apiGetEndpointConfig(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) apiGetVersion(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"version": config.Version})
}

// apiPricingAnalysis 定价分析 API — 为未来 AI 提供所有定价决策数据
// GET /admin/api/pricing-analysis
func (h *Handler) apiPricingAnalysis(w http.ResponseWriter, r *http.Request) {
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
		ms.TotalCredits += log.Credits
		ms.TotalTokens += log.TotalTokens
		totalCreditsAll += log.Credits
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
		pool := ResolveModelPool(log.ActualModel)
		if pool == "pro" {
			proCreditsAll += log.Credits
		} else {
			freeCreditsAll += log.Credits
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

func (h *Handler) apiGetApiKeys(w http.ResponseWriter, r *http.Request) {
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
		Plan      *string  `json:"plan"`
		ExpiresAt *int64   `json:"expiresAt"`
		Enabled   *bool    `json:"enabled"`
		Balance   *float64 `json:"balance"`
		Note      *string  `json:"note"`
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
	if req.Note != nil {
		existing.Note = *req.Note
	}
	if err := config.UpdateApiKey(id, *existing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiDeleteApiKey(w http.ResponseWriter, r *http.Request, id string) {
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

func (h *Handler) apiGetApiKeyLogs(w http.ResponseWriter, r *http.Request, keyID string) {
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
func (h *Handler) apiGetPricing(w http.ResponseWriter, r *http.Request) {
	pricing := config.GetPricing()
	json.NewEncoder(w).Encode(pricing)
}

// PUT /admin/api/pricing
func (h *Handler) apiUpdatePricing(w http.ResponseWriter, r *http.Request) {
	var pricing config.PricingConfig
	if err := json.NewDecoder(r.Body).Decode(&pricing); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdatePricing(pricing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("pricing_update", "admin", fmt.Sprintf("freePool=$%.2f proPool=$%.2f purchaseCNY=¥%.4f", pricing.FreePoolPriceUSD, pricing.ProPoolPriceUSD, pricing.PurchasePriceCNY))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/profit
func (h *Handler) apiGetProfit(w http.ResponseWriter, r *http.Request) {
	h.callLogsMu.RLock()
	var totalUSDConsumed float64
	var proCreditConsumed float64
	var freeCreditConsumed float64
	for _, log := range h.callLogs {
		pool := ResolveModelPool(log.ActualModel)
		costUSD := CreditsToCostUSD(log.Credits, pool)
		totalUSDConsumed += costUSD
		if pool == "pro" {
			proCreditConsumed += log.Credits
		} else {
			freeCreditConsumed += log.Credits
		}
	}
	h.callLogsMu.RUnlock()

	profit := CalcAdminProfit(totalUSDConsumed, proCreditConsumed, freeCreditConsumed)
	writeJSON(w, 200, profit)
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

func (h *Handler) apiGetCodes(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) apiCleanupCodes(w http.ResponseWriter, r *http.Request) {
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
		Type   string  `json:"type"`   // "balance" | "days"
		Amount float64 `json:"amount"` // USD face value (for balance) or days
		Tier   string  `json:"tier"`   // "free" | "pro" (only for type=days)
		Count  int     `json:"count"`  // how many codes to generate
		Note   string  `json:"note"`
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
func (h *Handler) apiDeleteCode(w http.ResponseWriter, r *http.Request, code string) {
	if err := config.DeleteActivationCode(code); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("code_delete", "admin", fmt.Sprintf("code=%s", code))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/abuse
func (h *Handler) apiGetAbuse(w http.ResponseWriter, r *http.Request) {
	flagged := GetFlaggedKeys()
	if flagged == nil {
		flagged = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(flagged)
}

// POST /admin/api/abuse/:keyId/clear
func (h *Handler) apiClearAbuse(w http.ResponseWriter, r *http.Request, keyID string) {
	ClearFlag(keyID)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
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
