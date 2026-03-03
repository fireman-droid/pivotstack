package proxy

import (
	"encoding/json"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"os"
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
		Enabled: true, MachineId: config.GenerateMachineId(),
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
		Enabled: true, MachineId: config.GenerateMachineId(),
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
			Enabled: true, MachineId: config.GenerateMachineId(),
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.RefreshToken == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "refreshToken is required"})
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
	tempAccount := &config.Account{
		RefreshToken: req.RefreshToken, ClientID: req.ClientID,
		ClientSecret: req.ClientSecret, AuthMethod: req.AuthMethod, Region: req.Region,
	}
	newAccessToken, newRefreshToken, newExpiresAt, err := auth.RefreshToken(tempAccount)
	if err != nil {
		if req.AccessToken != "" {
			accessToken = req.AccessToken
			expiresAt = time.Now().Unix() + 300
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
	email, _, _ := auth.GetUserInfo(accessToken)
	account := config.Account{
		ID: auth.GenerateAccountID(), Email: email,
		AccessToken: accessToken, RefreshToken: req.RefreshToken,
		ClientID: req.ClientID, ClientSecret: req.ClientSecret,
		AuthMethod: req.AuthMethod, Provider: req.Provider, Region: req.Region,
		ExpiresAt: expiresAt, Enabled: true, MachineId: config.GenerateMachineId(),
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

// ==================== 静态文件服务 ====================

func (h *Handler) serveAdminPage(w http.ResponseWriter, r *http.Request) {
	// 优先从 web/dist/ (Vue build) 提供，否则回退到 web/index.html (原始单文件)
	if fileExists("web/dist/index.html") {
		http.ServeFile(w, r, "web/dist/index.html")
	} else {
		http.ServeFile(w, r, "web/index.html")
	}
}

func (h *Handler) serveStaticFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/")
	// 优先从 dist 目录提供
	distPath := "web/dist/" + path
	if fileExists(distPath) {
		http.ServeFile(w, r, distPath)
		return
	}
	// 对于 Vue SPA，所有非静态资源路由回 index.html
	if fileExists("web/dist/index.html") && !strings.Contains(path, ".") {
		http.ServeFile(w, r, "web/dist/index.html")
		return
	}
	http.ServeFile(w, r, "web/"+path)
}

// serveDistFile 从 web/dist/ 目录提供 Vue 构建的静态资源
func (h *Handler) serveDistFile(w http.ResponseWriter, r *http.Request) {
	filePath := "web/dist" + r.URL.Path
	if fileExists(filePath) {
		http.ServeFile(w, r, filePath)
	} else {
		http.NotFound(w, r)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
