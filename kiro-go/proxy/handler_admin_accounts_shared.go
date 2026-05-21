package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
)

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
