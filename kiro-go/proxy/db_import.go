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

// apiImportFromDB 从远程数据库导入未激活普通账号
func (h *Handler) apiImportFromDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	// 解析请求参数
	var req struct {
		Min   int  `json:"min"`   // 最少保持多少个可用账号
		Limit int  `json:"limit"` // 最多导入多少个
		Force bool `json:"force"` // 强制导入（不检查当前数量）
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

	// 逐个导入
	var imported []map[string]string
	var failed []map[string]string

	for _, a := range accounts {
		// 确定 authMethod
		authMethod := "idc"
		if a.ClientID == "" {
			authMethod = "social"
		}

		// 尝试刷新 token
		tempAccount := &config.Account{
			RefreshToken: a.RefreshToken,
			ClientID:     a.ClientID,
			ClientSecret: a.ClientSecret,
			AuthMethod:   authMethod,
			Region:       a.Region,
		}

		newAccessToken, newRefreshToken, expiresAt, err := auth.RefreshToken(tempAccount)
		if err != nil {
			failed = append(failed, map[string]string{
				"email": a.Email,
				"error": "Token 刷新失败: " + err.Error(),
			})
			continue
		}

		refreshToken := a.RefreshToken
		if newRefreshToken != "" {
			refreshToken = newRefreshToken
		}

		// 获取用户邮箱
		email, _, _ := auth.GetUserInfo(newAccessToken)
		if email == "" {
			email = a.Email
		}

		// 创建账号
		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        email,
			AccessToken:  newAccessToken,
			RefreshToken: refreshToken,
			ClientID:     a.ClientID,
			ClientSecret: a.ClientSecret,
			AuthMethod:   authMethod,
			Provider:     a.Provider,
			Region:       a.Region,
			ExpiresAt:    expiresAt,
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}

		if err := config.AddAccount(account); err != nil {
			failed = append(failed, map[string]string{
				"email": a.Email,
				"error": err.Error(),
			})
			continue
		}

		// 标记为已激活
		_, markErr := db.Exec("UPDATE kiro_accounts SET card_status = 'activated' WHERE id = ?", a.ID)
		if markErr != nil {
			log.Printf("[DB Import] 标记已激活失败 %s: %v", a.Email, markErr)
		}

		imported = append(imported, map[string]string{
			"email": email,
			"id":    account.ID,
		})
	}

	// 重载账号池
	h.pool.Reload()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("导入完成: %d 成功, %d 失败", len(imported), len(failed)),
		"current":   currentAvailable + len(imported),
		"total":     currentTotal + len(imported),
		"imported":  len(imported),
		"failed":    len(failed),
		"available": len(accounts),
		"details": map[string]interface{}{
			"imported": imported,
			"failed":   failed,
		},
	})
}

// apiGetDBStatus 查询远程数据库中可用账号数
func (h *Handler) apiGetDBStatus(w http.ResponseWriter, r *http.Request) {
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
