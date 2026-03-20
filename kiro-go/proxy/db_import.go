package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/auth"
	"kiro-api-proxy/config"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// dbImportConfig 数据库导入配置
type dbImportConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

func getDBImportConfig() *dbImportConfig {
	port := 3306
	if p := os.Getenv("IMPORT_DB_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	return &dbImportConfig{
		Host:     getEnvOrDefault("IMPORT_DB_HOST", "115.191.35.73"),
		Port:     port,
		User:     getEnvOrDefault("IMPORT_DB_USER", "root"),
		Password: getEnvOrDefault("IMPORT_DB_PASSWORD", "Lin20050201"),
		Database: getEnvOrDefault("IMPORT_DB_NAME", "kiro_db"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// dbAccount 从远程数据库查到的账号
type dbAccount struct {
	ID           string
	Email        string
	RefreshToken string
	ClientID     string
	ClientSecret string
	Region       string
	Provider     string
}

// importResult 单个账号导入结果
type importResult struct {
	Success bool
	Account config.Account
	DbID    string // 远端数据库 ID，用于标记已激活
	Email   string
	Error   string
}

// apiImportFromDB 从远程数据库导入未激活普通账号（并发版本）
func (h *Handler) apiImportFromDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	// 解析请求参数
	var req struct {
		Min         int  `json:"min"`         // 最少保持多少个可用账号
		Limit       int  `json:"limit"`       // 最多导入多少个
		Force       bool `json:"force"`       // 强制导入（不检查当前数量）
		Concurrency int  `json:"concurrency"` // 并发数，默认 10
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 允许空 body
		req.Min = 3
		req.Limit = 5
	}
	if req.Min <= 0 {
		req.Min = 3
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 10
	}
	if req.Concurrency > 50 {
		req.Concurrency = 50
	}

	// 检查当前可用账号数
	currentAvailable := h.pool.AvailableCount()
	currentTotal := h.pool.Count()

	need := req.Min - currentAvailable
	if !req.Force && need <= 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"message":   "账号充足，无需补充",
			"current":   currentAvailable,
			"total":     currentTotal,
			"imported":  0,
			"failed":    0,
			"available": 0,
		})
		return
	}
	if !req.Force && need > req.Limit {
		need = req.Limit
	}
	if req.Force {
		need = req.Limit
	}

	// 连接远程数据库
	dbCfg := getDBImportConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&timeout=10s",
		dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "数据库连接失败: " + err.Error()})
		return
	}
	defer db.Close()
	db.SetConnMaxLifetime(30 * time.Second)

	if err := db.Ping(); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "数据库连接失败: " + err.Error()})
		return
	}

	// 查询未激活普通账号
	rows, err := db.Query(`
		SELECT id, email, refresh_token, client_id, client_secret, region, provider
		FROM kiro_accounts
		WHERE pool = 'normal'
		  AND card_status = 'unactivated'
		  AND status = 'active'
		  AND refresh_token IS NOT NULL
		  AND refresh_token != ''
		ORDER BY created_at DESC
		LIMIT ?
	`, need)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	var accounts []dbAccount
	for rows.Next() {
		var a dbAccount
		var clientID, clientSecret, region, provider sql.NullString
		if err := rows.Scan(&a.ID, &a.Email, &a.RefreshToken, &clientID, &clientSecret, &region, &provider); err != nil {
			continue
		}
		a.ClientID = clientID.String
		a.ClientSecret = clientSecret.String
		a.Region = region.String
		a.Provider = provider.String
		if a.Region == "" {
			a.Region = "us-east-1"
		}
		if a.Provider == "" {
			a.Provider = "BuilderId"
		}
		accounts = append(accounts, a)
	}

	if len(accounts) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"message":   "数据库中没有可用的未激活普通账号",
			"current":   currentAvailable,
			"total":     currentTotal,
			"imported":  0,
			"failed":    0,
			"available": 0,
		})
		return
	}

	startTime := time.Now()
	log.Printf("[DB Import] 开始并发导入 %d 个账号，并发数: %d", len(accounts), req.Concurrency)

	// ========== 并发刷新 Token + 获取用户信息 ==========
	results := make([]importResult, len(accounts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, req.Concurrency) // 并发信号量

	for i, a := range accounts {
		wg.Add(1)
		go func(idx int, acc dbAccount) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			result := &results[idx]
			result.DbID = acc.ID
			result.Email = acc.Email

			// 确定 authMethod
			authMethod := "idc"
			if acc.ClientID == "" {
				authMethod = "social"
			}

			// 尝试刷新 token
			tempAccount := &config.Account{
				RefreshToken: acc.RefreshToken,
				ClientID:     acc.ClientID,
				ClientSecret: acc.ClientSecret,
				AuthMethod:   authMethod,
				Region:       acc.Region,
			}

			newAccessToken, newRefreshToken, expiresAt, err := auth.RefreshToken(tempAccount)
			if err != nil {
				result.Error = "Token 刷新失败: " + err.Error()
				return
			}

			refreshToken := acc.RefreshToken
			if newRefreshToken != "" {
				refreshToken = newRefreshToken
			}

			// 获取用户邮箱
			email, _, _ := auth.GetUserInfo(newAccessToken)
			if email == "" {
				email = acc.Email
			}
			result.Email = email

			// 构建账号对象（暂不写入，等批量写入）
			result.Success = true
			result.Account = config.Account{
				ID:           auth.GenerateAccountID(),
				Email:        email,
				AccessToken:  newAccessToken,
				RefreshToken: refreshToken,
				ClientID:     acc.ClientID,
				ClientSecret: acc.ClientSecret,
				AuthMethod:   authMethod,
				Provider:     acc.Provider,
				Region:       acc.Region,
				ExpiresAt:    expiresAt,
				Enabled:      true,
				MachineId:    config.GenerateMachineId(),
			}
		}(i, a)
	}

	wg.Wait() // 等待所有并发刷新完成

	// ========== 批量写入配置（一次性写 JSON） ==========
	var successAccounts []config.Account
	var imported []map[string]string
	var failed []map[string]string
	var successDbIDs []string

	for _, r := range results {
		if r.Success {
			successAccounts = append(successAccounts, r.Account)
			imported = append(imported, map[string]string{
				"email": r.Email,
				"id":    r.Account.ID,
			})
			successDbIDs = append(successDbIDs, r.DbID)
		} else {
			failed = append(failed, map[string]string{
				"email": r.Email,
				"error": r.Error,
			})
		}
	}

	// 批量导入（只写一次 JSON 文件）
	if len(successAccounts) > 0 {
		importedCount, skippedCount, err := config.ImportAccounts(successAccounts)
		if err != nil {
			log.Printf("[DB Import] 批量写入失败: %v", err)
		} else {
			log.Printf("[DB Import] 批量写入完成: %d 导入, %d 跳过", importedCount, skippedCount)
		}
	}

	// 批量标记数据库已激活
	if len(successDbIDs) > 0 {
		placeholders := make([]string, len(successDbIDs))
		args := make([]interface{}, len(successDbIDs))
		for i, id := range successDbIDs {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("UPDATE kiro_accounts SET card_status = 'activated' WHERE id IN (%s)", strings.Join(placeholders, ","))
		if _, markErr := db.Exec(query, args...); markErr != nil {
			log.Printf("[DB Import] 批量标记已激活失败: %v", markErr)
		}
	}

	// 重载账号池
	h.pool.Reload()

	elapsed := time.Since(startTime)
	log.Printf("[DB Import] 导入完成: %d 成功, %d 失败, 耗时 %.1f 秒", len(imported), len(failed), elapsed.Seconds())

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("导入完成: %d 成功, %d 失败, 耗时 %.1f 秒", len(imported), len(failed), elapsed.Seconds()),
		"current":     currentAvailable + len(imported),
		"total":       currentTotal + len(imported),
		"imported":    len(imported),
		"failed":      len(failed),
		"available":   len(accounts),
		"elapsed_sec": elapsed.Seconds(),
		"details": map[string]interface{}{
			"imported": imported,
			"failed":   failed,
		},
	})
}

// apiGetDBStatus 查询远程数据库中可用账号数
func (h *Handler) apiGetDBStatus(w http.ResponseWriter, _ *http.Request) {
	dbCfg := getDBImportConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&timeout=10s",
		dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer db.Close()

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM kiro_accounts
		WHERE pool = 'normal'
		  AND card_status = 'unactivated'
		  AND status = 'active'
		  AND refresh_token IS NOT NULL
		  AND refresh_token != ''
	`).Scan(&count)

	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 拼接数据库地址（隐藏密码）
	dbAddr := fmt.Sprintf("%s@%s:%d/%s", dbCfg.User, dbCfg.Host, dbCfg.Port, dbCfg.Database)
	dbAddr = strings.Replace(dbAddr, dbCfg.Password, "***", -1)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"available": count,
		"database":  dbAddr,
	})
}

// resetDBCardStatus 异步重置远程数据库中指定 email 账号的 card_status 为 unactivated
// 这样删除本地账号后，该账号可以被重新从数据库导入
func resetDBCardStatus(emails []string) {
	if len(emails) == 0 {
		return
	}

	dbCfg := getDBImportConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&timeout=10s",
		dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("[DB Reset] 连接数据库失败: %v", err)
		return
	}
	defer db.Close()

	placeholders := make([]string, len(emails))
	args := make([]interface{}, len(emails))
	for i, e := range emails {
		placeholders[i] = "?"
		args[i] = e
	}

	query := fmt.Sprintf(
		"UPDATE kiro_accounts SET card_status = 'unactivated' WHERE email IN (%s)",
		strings.Join(placeholders, ","),
	)

	result, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("[DB Reset] 重置 card_status 失败: %v", err)
		return
	}

	affected, _ := result.RowsAffected()
	log.Printf("[DB Reset] 已重置 %d 个账号的 card_status 为 unactivated (emails: %v)", affected, emails)
}
