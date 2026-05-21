package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

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
		id, isNew, err := config.AddOrUpdateAccount(account)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		imported = append(imported, map[string]interface{}{"id": id, "email": account.Email, "isNew": isNew})
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

	id, isNew, err := config.AddOrUpdateAccount(account)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	account.ID = id
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
		"success": true, "isNew": isNew,
		"account": map[string]interface{}{"id": account.ID, "email": account.Email},
	})
}

// apiImportCredentialsBatch 批量导入账号（SSE 实时进度）
func (h *Handler) apiImportCredentialsBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Accounts    []json.RawMessage `json:"accounts"`
		Concurrency int               `json:"concurrency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}
	if len(req.Accounts) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "accounts array is empty"})
		return
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 20
	}
	if req.Concurrency > 100 {
		req.Concurrency = 100
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(500)
		fmt.Fprintf(w, "data: {\"error\":\"streaming not supported\"}\n\n")
		return
	}

	var writeMu sync.Mutex
	sendEvent := func(event string, data interface{}) {
		writeMu.Lock()
		defer writeMu.Unlock()
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
		flusher.Flush()
	}

	type credItem struct {
		AccessToken  string                 `json:"accessToken"`
		RefreshToken string                 `json:"refreshToken"`
		ClientID     string                 `json:"clientId"`
		ClientSecret string                 `json:"clientSecret"`
		AuthMethod   string                 `json:"authMethod"`
		Provider     string                 `json:"provider"`
		Region       string                 `json:"region"`
		Email        string                 `json:"email"`
		UserId       string                 `json:"userId"`
		MachineId    string                 `json:"machineId"`
		UsageData    map[string]interface{} `json:"usageData"`
	}

	type batchResult struct {
		Success bool
		Account config.Account
		Email   string
		Error   string
	}

	total := len(req.Accounts)
	sendEvent("start", map[string]int{"total": total})

	startTime := time.Now()
	results := make([]batchResult, total)
	var wg sync.WaitGroup
	sem := make(chan struct{}, req.Concurrency)

	// 原子计数器
	var doneCount int64
	var okCount int64
	var failCount int64

	for i, raw := range req.Accounts {
		var item credItem
		if err := json.Unmarshal(raw, &item); err != nil {
			results[i] = batchResult{Error: "JSON 解析失败: " + err.Error()}
			atomic.AddInt64(&doneCount, 1)
			atomic.AddInt64(&failCount, 1)
			sendEvent("progress", map[string]interface{}{
				"done": atomic.LoadInt64(&doneCount), "total": total,
				"ok": atomic.LoadInt64(&okCount), "fail": atomic.LoadInt64(&failCount),
			})
			continue
		}
		if item.Region == "" {
			item.Region = "us-east-1"
		}
		if item.AuthMethod == "" {
			if item.ClientID != "" {
				item.AuthMethod = "idc"
			} else {
				item.AuthMethod = "social"
			}
		}
		switch strings.ToLower(item.AuthMethod) {
		case "idc", "builderid", "enterprise":
			item.AuthMethod = "idc"
		case "social", "google", "github":
			item.AuthMethod = "social"
		default:
			if item.ClientID != "" && item.ClientSecret != "" {
				item.AuthMethod = "idc"
			} else {
				item.AuthMethod = "social"
			}
		}
		if item.RefreshToken == "" && item.AccessToken == "" {
			results[i] = batchResult{Error: "缺少 token", Email: item.Email}
			atomic.AddInt64(&doneCount, 1)
			atomic.AddInt64(&failCount, 1)
			sendEvent("progress", map[string]interface{}{
				"done": atomic.LoadInt64(&doneCount), "total": total,
				"ok": atomic.LoadInt64(&okCount), "fail": atomic.LoadInt64(&failCount),
			})
			continue
		}

		wg.Add(1)
		go func(idx int, it credItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := &results[idx]
			result.Email = it.Email

			var accessToken string
			var expiresAt int64
			refreshToken := it.RefreshToken

			if it.RefreshToken != "" {
				tempAccount := &config.Account{
					RefreshToken: it.RefreshToken, ClientID: it.ClientID,
					ClientSecret: it.ClientSecret, AuthMethod: it.AuthMethod, Region: it.Region,
				}
				newAT, newRT, newExp, err := auth.RefreshToken(tempAccount)
				if err != nil {
					if it.AccessToken != "" {
						accessToken = it.AccessToken
						expiresAt = time.Now().Unix() + 3600
					} else {
						result.Error = "Token 刷新失败: " + err.Error()
						atomic.AddInt64(&doneCount, 1)
						atomic.AddInt64(&failCount, 1)
						sendEvent("progress", map[string]interface{}{
							"done": atomic.LoadInt64(&doneCount), "total": total,
							"ok": atomic.LoadInt64(&okCount), "fail": atomic.LoadInt64(&failCount),
						})
						return
					}
				} else {
					accessToken = newAT
					if newRT != "" {
						refreshToken = newRT
					}
					expiresAt = newExp
				}
			} else {
				accessToken = it.AccessToken
				expiresAt = time.Now().Unix() + 3600
			}

			email := it.Email
			if email == "" {
				email, _, _ = auth.GetUserInfo(accessToken)
			}
			result.Email = email

			machineId := it.MachineId
			if machineId == "" {
				machineId = config.GenerateMachineId()
			}

			account := config.Account{
				ID: auth.GenerateAccountID(), Email: email,
				UserId:      it.UserId,
				AccessToken: accessToken, RefreshToken: refreshToken,
				ClientID: it.ClientID, ClientSecret: it.ClientSecret,
				AuthMethod: it.AuthMethod, Provider: it.Provider, Region: it.Region,
				ExpiresAt: expiresAt, Enabled: true, MachineId: machineId,
				Weight: 1,
			}

			if it.UsageData != nil {
				parseUsageData(&account, it.UsageData)
				account.LastRefresh = time.Now().Unix()
			}

			result.Success = true
			result.Account = account
			atomic.AddInt64(&doneCount, 1)
			atomic.AddInt64(&okCount, 1)
			sendEvent("progress", map[string]interface{}{
				"done": atomic.LoadInt64(&doneCount), "total": total,
				"ok": atomic.LoadInt64(&okCount), "fail": atomic.LoadInt64(&failCount),
			})
		}(i, item)
	}

	wg.Wait()

	// 批量写入
	var successAccounts []config.Account
	var imported []map[string]string
	var failed []map[string]string

	for _, r := range results {
		if r.Success {
			successAccounts = append(successAccounts, r.Account)
			imported = append(imported, map[string]string{
				"email": r.Email, "id": r.Account.ID,
			})
		} else if r.Error != "" {
			failed = append(failed, map[string]string{
				"email": r.Email, "error": r.Error,
			})
		}
	}

	if len(successAccounts) > 0 {
		config.ImportAccounts(successAccounts)
	}
	h.pool.Reload()

	elapsed := time.Since(startTime)
	sendEvent("done", map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("导入完成: %d 成功, %d 失败, 耗时 %.1f 秒", len(imported), len(failed), elapsed.Seconds()),
		"imported":    len(imported),
		"failed":      len(failed),
		"elapsed_sec": elapsed.Seconds(),
		"details": map[string]interface{}{
			"imported": imported,
			"failed":   failed,
		},
	})
}
