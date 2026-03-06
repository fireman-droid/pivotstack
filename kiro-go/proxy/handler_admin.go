package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
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
	case path == "/logs" && r.Method == "GET":
		h.apiGetLogs(w, r)
	case path == "/logs" && r.Method == "DELETE":
		h.apiClearLogs(w, r)
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accounts": h.pool.Count(), "available": h.pool.AvailableCount(),
		"totalRequests": h.totalRequests, "successRequests": h.successRequests,
		"failedRequests": h.failedRequests, "totalTokens": h.totalTokens,
		"totalCredits": h.totalCredits, "uptime": time.Now().Unix() - h.startTime,
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
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": logs})
}

func (h *Handler) apiClearLogs(w http.ResponseWriter, r *http.Request) {
	h.callLogsMu.Lock()
	h.callLogs = nil
	h.callLogsMu.Unlock()
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
