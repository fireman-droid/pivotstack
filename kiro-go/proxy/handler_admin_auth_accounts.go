package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// apiRefreshAccount 刷新账户信息
func (h *Handler) apiRefreshAccount(w http.ResponseWriter, _ *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}
	refreshTokenIfNeeded := func() error {
		if account.RefreshToken == "" {
			return nil
		}
		newAccessToken, newRefreshToken, newExpiresAt, err := auth.RefreshToken(account)
		if err != nil {
			return err
		}
		account.AccessToken = newAccessToken
		if newRefreshToken != "" {
			account.RefreshToken = newRefreshToken
		}
		account.ExpiresAt = newExpiresAt
		config.UpdateAccountToken(id, newAccessToken, newRefreshToken, newExpiresAt)
		h.pool.UpdateToken(id, newAccessToken, newRefreshToken, newExpiresAt)
		return nil
	}
	if account.ExpiresAt > 0 && time.Now().Unix() > account.ExpiresAt-300 {
		if err := refreshTokenIfNeeded(); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed: " + err.Error()})
			return
		}
	}
	info, err := RefreshAccountInfo(account)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "TEMPORARILY_SUSPENDED") || strings.Contains(errMsg, "Account suspended") {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Account status updated"})
			return
		}
		if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "expired") {
			if refreshErr := refreshTokenIfNeeded(); refreshErr == nil {
				info, err = RefreshAccountInfo(account)
				if err != nil {
					if strings.Contains(err.Error(), "TEMPORARILY_SUSPENDED") || strings.Contains(err.Error(), "Account suspended") {
						json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Account status updated"})
						return
					}
				}
			}
		}
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	if err := config.UpdateAccountInfo(id, *info); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "info": info})
}

// apiGetAccountFull 获取单个账号完整信息
func (h *Handler) apiGetAccountFull(w http.ResponseWriter, _ *http.Request, id string) {
	accounts := config.GetAccounts()
	poolAccounts := h.pool.GetAllAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}
	var stats config.Account
	for _, a := range poolAccounts {
		if a.ID == id {
			stats = a
			break
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": account.ID, "email": account.Email, "userId": account.UserId, "nickname": account.Nickname,
		"accessToken": account.AccessToken, "refreshToken": account.RefreshToken,
		"clientId": account.ClientID, "clientSecret": account.ClientSecret,
		"authMethod": account.AuthMethod, "provider": account.Provider, "region": account.Region,
		"expiresAt": account.ExpiresAt, "machineId": account.MachineId,
		"enabled": account.Enabled, "banStatus": account.BanStatus, "banReason": account.BanReason, "banTime": account.BanTime,
		"subscriptionType": account.SubscriptionType, "subscriptionTitle": account.SubscriptionTitle, "daysRemaining": account.DaysRemaining,
		"usageCurrent": account.UsageCurrent, "usageLimit": account.UsageLimit, "usagePercent": account.UsagePercent,
		"nextResetDate": account.NextResetDate, "lastRefresh": account.LastRefresh,
		"trialUsageCurrent": account.TrialUsageCurrent, "trialUsageLimit": account.TrialUsageLimit,
		"trialUsagePercent": account.TrialUsagePercent, "trialStatus": account.TrialStatus, "trialExpiresAt": account.TrialExpiresAt,
		"requestCount": stats.RequestCount, "errorCount": stats.ErrorCount,
		"totalTokens": stats.TotalTokens, "totalCredits": stats.TotalCredits, "lastUsed": stats.LastUsed,
		"weight": account.Weight,
	})
}

// apiGetAccountModels 获取账户可用模型
func (h *Handler) apiGetAccountModels(w http.ResponseWriter, _ *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}
	models, err := ListAvailableModels(account)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "models": models})
}
