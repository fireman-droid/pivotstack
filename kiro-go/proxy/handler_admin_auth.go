package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) apiStartIamSso(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartUrl string `json:"startUrl"`
		Region   string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.StartUrl == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "startUrl is required"})
		return
	}
	sessionID, authorizeUrl, expiresIn, err := auth.StartIamSsoLogin(req.StartUrl, req.Region)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId": sessionID, "authorizeUrl": authorizeUrl, "expiresIn": expiresIn,
	})
}

func (h *Handler) apiCompleteIamSso(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID   string `json:"sessionId"`
		CallbackUrl string `json:"callbackUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	accessToken, refreshToken, clientID, clientSecret, region, expiresIn, err := auth.CompleteIamSsoLogin(req.SessionID, req.CallbackUrl)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	email, _, _ := auth.GetUserInfo(accessToken)
	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		AccessToken: accessToken, RefreshToken: refreshToken,
		ClientID: clientID, ClientSecret: clientSecret,
		AuthMethod: "idc", Region: region,
		ExpiresAt: time.Now().Unix() + int64(expiresIn),
		Enabled:   true, MachineId: config.GenerateMachineId(),
	}
	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "account": map[string]interface{}{"id": account.ID, "email": account.Email},
	})
}

func (h *Handler) apiStartBuilderIdLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Region string `json:"region"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	session, err := auth.StartBuilderIdLogin(req.Region)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId": session.ID, "userCode": session.UserCode,
		"verificationUri": session.VerificationUri, "interval": session.Interval,
	})
}

func (h *Handler) apiPollBuilderIdAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	accessToken, refreshToken, clientID, clientSecret, region, expiresIn, status, err := auth.PollBuilderIdAuth(req.SessionID)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	if status == "pending" || status == "slow_down" {
		interval := 5
		if session := auth.GetBuilderIdSession(req.SessionID); session != nil {
			interval = session.Interval
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "completed": false, "status": status, "interval": interval,
		})
		return
	}
	email, _, _ := auth.GetUserInfo(accessToken)
	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		AccessToken: accessToken, RefreshToken: refreshToken,
		ClientID: clientID, ClientSecret: clientSecret,
		AuthMethod: "idc", Provider: "BuilderId", Region: region,
		ExpiresAt: time.Now().Unix() + int64(expiresIn),
		Enabled:   true, MachineId: config.GenerateMachineId(),
	}
	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "completed": true,
		"account": map[string]interface{}{"id": account.ID, "email": account.Email},
	})
}

func (h *Handler) apiImportSsoToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BearerToken string `json:"bearerToken"`
		Region      string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.BearerToken == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "bearerToken is required"})
		return
	}
	tokens := strings.Split(strings.TrimSpace(req.BearerToken), "\n")
	var imported []map[string]interface{}
	var errors []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		accessToken, refreshToken, clientID, clientSecret, expiresIn, err := auth.ImportFromSsoToken(token, req.Region)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		email, _, _ := auth.GetUserInfo(accessToken)
		account := config.Account{
			ID: auth.GenerateAccountID(), Email: email,
			AccessToken: accessToken, RefreshToken: refreshToken,
			ClientID: clientID, ClientSecret: clientSecret,
			AuthMethod: "idc", Region: req.Region,
			ExpiresAt: time.Now().Unix() + int64(expiresIn),
			Enabled:   true, MachineId: config.GenerateMachineId(),
		}
		if err := config.AddAccount(account); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		imported = append(imported, map[string]interface{}{"id": account.ID, "email": account.Email})
	}
	h.pool.Reload()
	if len(imported) == 0 && len(errors) > 0 {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": strings.Join(errors, "; ")})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "accounts": imported, "errors": errors})
}

func (h *Handler) apiImportCredentials(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		AuthMethod   string `json:"authMethod"`
		Provider     string `json:"provider"`
		Region       string `json:"region"`
		// 额外字段：从 kiro-account-manager 导入
		Email      string                 `json:"email"`
		UserId     string                 `json:"userId"`
		ProfileArn string                 `json:"profileArn"`
		MachineId  string                 `json:"machineId"`
		UsageData  map[string]interface{} `json:"usageData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.RefreshToken == "" && req.AccessToken == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "refreshToken or accessToken is required"})
		return
	}
	if req.Region == "" {
		req.Region = "us-east-1"
	}
	if req.AuthMethod == "" {
		if req.ClientID != "" {
			req.AuthMethod = "idc"
		} else {
			req.AuthMethod = "social"
		}
	}
	switch strings.ToLower(req.AuthMethod) {
	case "idc", "builderid", "enterprise":
		req.AuthMethod = "idc"
	case "social", "google", "github":
		req.AuthMethod = "social"
	default:
		if req.ClientID != "" && req.ClientSecret != "" {
			req.AuthMethod = "idc"
		} else {
			req.AuthMethod = "social"
		}
	}

	var accessToken string
	var expiresAt int64
	refreshFailed := false

	if req.RefreshToken != "" {
		tempAccount := &config.Account{
			RefreshToken: req.RefreshToken, ClientID: req.ClientID,
			ClientSecret: req.ClientSecret, AuthMethod: req.AuthMethod, Region: req.Region,
		}
		newAccessToken, newRefreshToken, newExpiresAt, err := auth.RefreshToken(tempAccount)
		if err != nil {
			refreshFailed = true
			if req.AccessToken != "" {
				accessToken = req.AccessToken
				expiresAt = time.Now().Unix() + 3600 // 1小时有效期
			} else {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed: " + err.Error()})
				return
			}
		} else {
			accessToken = newAccessToken
			if newRefreshToken != "" {
				req.RefreshToken = newRefreshToken
			}
			expiresAt = newExpiresAt
		}
	} else {
		accessToken = req.AccessToken
		expiresAt = time.Now().Unix() + 3600
		refreshFailed = true
	}

	// 获取 email：优先使用请求中自带的
	email := req.Email
	if email == "" {
		email, _, _ = auth.GetUserInfo(accessToken)
	}

	// 使用请求中的 machineId 或生成新的
	machineId := req.MachineId
	if machineId == "" {
		machineId = config.GenerateMachineId()
	}

	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		UserId:      req.UserId,
		AccessToken: accessToken, RefreshToken: req.RefreshToken,
		ClientID: req.ClientID, ClientSecret: req.ClientSecret,
		AuthMethod: req.AuthMethod, Provider: req.Provider, Region: req.Region,
		ExpiresAt: expiresAt, Enabled: true, MachineId: machineId,
		Weight: 1, // 默认权重 1
	}

	// 从 usageData 中解析配额信息（kiro-account-manager 导出格式）
	if req.UsageData != nil {
		parseUsageData(&account, req.UsageData)
		account.LastRefresh = time.Now().Unix()
	}

	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()

	// 异步刷新配额（仅在没有 usageData 且刷新成功时）
	if req.UsageData == nil && !refreshFailed {
		credAccountID := account.ID
		go func() {
			accounts := config.GetAccounts()
			for i := range accounts {
				if accounts[i].ID == credAccountID {
					info, err := RefreshAccountInfo(&accounts[i])
					if err == nil {
						config.UpdateAccountInfo(credAccountID, *info)
					}
					return
				}
			}
		}()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "account": map[string]interface{}{"id": account.ID, "email": account.Email},
	})
}

// parseUsageData 解析 kiro-account-manager 导出的 usageData 字段
func parseUsageData(account *config.Account, data map[string]interface{}) {
	// 解析 subscriptionInfo
	if subInfo, ok := data["subscriptionInfo"].(map[string]interface{}); ok {
		if t, ok := subInfo["type"].(string); ok {
			// Q_DEVELOPER_STANDALONE_FREE -> FREE, Q_DEVELOPER_STANDALONE_PRO -> PRO
			switch {
			case strings.Contains(t, "FREE"):
				account.SubscriptionType = "FREE"
			case strings.Contains(t, "PRO_PLUS"):
				account.SubscriptionType = "PRO_PLUS"
			case strings.Contains(t, "PRO"):
				account.SubscriptionType = "PRO"
			default:
				account.SubscriptionType = t
			}
		}
		if title, ok := subInfo["subscriptionTitle"].(string); ok {
			account.SubscriptionTitle = title
		}
	}

	// 解析 daysUntilReset
	if days, ok := data["daysUntilReset"].(float64); ok {
		account.DaysRemaining = int(days)
	}

	// 解析 nextDateReset
	if resetTs, ok := data["nextDateReset"].(float64); ok {
		t := time.Unix(int64(resetTs), 0)
		account.NextResetDate = t.Format("2006-01-02")
	}

	// 解析 usageBreakdownList
	if breakdowns, ok := data["usageBreakdownList"].([]interface{}); ok && len(breakdowns) > 0 {
		if bd, ok := breakdowns[0].(map[string]interface{}); ok {
			// 主额度
			if usage, ok := bd["currentUsage"].(float64); ok {
				account.UsageCurrent = usage
			}
			if limit, ok := bd["usageLimit"].(float64); ok {
				account.UsageLimit = limit
				if limit > 0 {
					account.UsagePercent = account.UsageCurrent / limit
				}
			}

			// 试用额度
			if trial, ok := bd["freeTrialInfo"].(map[string]interface{}); ok {
				if usage, ok := trial["currentUsage"].(float64); ok {
					account.TrialUsageCurrent = usage
				}
				if limit, ok := trial["usageLimit"].(float64); ok {
					account.TrialUsageLimit = limit
					if limit > 0 {
						account.TrialUsagePercent = account.TrialUsageCurrent / limit
					}
				}
				if status, ok := trial["freeTrialStatus"].(string); ok {
					account.TrialStatus = status
				}
				if expiry, ok := trial["freeTrialExpiry"].(float64); ok {
					account.TrialExpiresAt = int64(expiry)
				}
			}
		}
	}
}

// apiRefreshAccount 刷新账户信息
func (h *Handler) apiRefreshAccount(w http.ResponseWriter, r *http.Request, id string) {
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
func (h *Handler) apiGetAccountFull(w http.ResponseWriter, r *http.Request, id string) {
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
func (h *Handler) apiGetAccountModels(w http.ResponseWriter, r *http.Request, id string) {
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
