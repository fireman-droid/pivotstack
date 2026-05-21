package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GET /admin/api/insights/funnel
// 返回活跃度漏斗：注册 → 启用 → 有余额 → 周活 → 日活 → 小时活 → 在线 5min
func (h *Handler) apiInsightsFunnel(w http.ResponseWriter, _ *http.Request) {
	apiKeys := config.GetAllApiKeys()
	totalKeys := len(apiKeys)
	enabled := 0
	withBalance := 0
	for _, k := range apiKeys {
		if k.Enabled {
			enabled++
		}
		if k.Enabled && (k.Balance+k.GiftBalance) > 0.1 {
			withBalance++
		}
	}

	// 从 call_logs.jsonl 文件读取（不是 in-memory 5000 条上限），保证 7 天数据完整（Bug #5）
	now := time.Now().Unix()
	cutoff5m := now - 5*60
	cutoff1h := now - 3600
	cutoff24h := now - 86400
	cutoff7d := now - 7*86400

	online5m := map[string]bool{}
	hour := map[string]bool{}
	day := map[string]bool{}
	week := map[string]bool{}

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	if f, err := os.Open(logPath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var entry struct {
				Timestamp int64  `json:"timestamp"`
				ApiKeyID  string `json:"api_key_id"`
			}
			if json.Unmarshal(scanner.Bytes(), &entry) != nil {
				continue
			}
			if entry.ApiKeyID == "" || entry.Timestamp < cutoff7d {
				continue
			}
			week[entry.ApiKeyID] = true
			if entry.Timestamp >= cutoff24h {
				day[entry.ApiKeyID] = true
			}
			if entry.Timestamp >= cutoff1h {
				hour[entry.ApiKeyID] = true
			}
			if entry.Timestamp >= cutoff5m {
				online5m[entry.ApiKeyID] = true
			}
		}
		f.Close()
	}

	writeJSON(w, 200, map[string]interface{}{
		"totalKeys":   totalKeys,
		"enabled":     enabled,
		"withBalance": withBalance,
		"weekActive":  len(week),
		"dayActive":   len(day),
		"hourActive":  len(hour),
		"online5m":    len(online5m),
		"updatedAt":   now,
	})
}
