package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// GET /admin/api/insights/whales?metric=credits|recharge|requests&limit=20&days=30
// metric 默认 credits（消耗），可选 recharge（充值额）/ requests（调用次数）
func (h *Handler) apiInsightsWhales(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "credits"
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	days := 30
	if d, _ := strconv.Atoi(r.URL.Query().Get("days")); d > 0 {
		days = d
	}
	cutoff := time.Now().Unix() - int64(days*86400)

	type row struct {
		KeyID    string  `json:"keyId"`
		Note     string  `json:"note"`
		Calls    int     `json:"calls"`
		Credits  float64 `json:"credits"`
		Recharge float64 `json:"rechargeCNY"` // 期间充值（按时间窗）
	}
	stat := map[string]*row{}

	apiKeys := config.GetAllApiKeys()
	keyNote := map[string]string{}
	for _, k := range apiKeys {
		keyNote[k.ID] = k.Note
	}

	// 从 call_logs.jsonl 算 calls/credits（窗口内）
	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	if f, err := os.Open(logPath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var entry CallLog
			if json.Unmarshal(scanner.Bytes(), &entry) != nil {
				continue
			}
			if entry.Timestamp < cutoff || entry.ApiKeyID == "" {
				continue
			}
			if _, ok := stat[entry.ApiKeyID]; !ok {
				stat[entry.ApiKeyID] = &row{KeyID: entry.ApiKeyID, Note: keyNote[entry.ApiKeyID]}
			}
			s := stat[entry.ApiKeyID]
			s.Calls++
			// v3 token 模式 Credits=0 但 UpstreamCredits>0；活跃度排名兜底用 UpstreamCredits
			if entry.Credits > 0 {
				s.Credits += entry.Credits
			} else if entry.UpstreamCredits > 0 {
				s.Credits += entry.UpstreamCredits
			}
		}
		f.Close()
	}

	// 从 recharge_records.jsonl 算 recharge（窗口内）
	rechargePath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	if f, err := os.Open(rechargePath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var rec RechargeRecord
			if json.Unmarshal(scanner.Bytes(), &rec) != nil {
				continue
			}
			if rec.Timestamp < cutoff || rec.KeyID == "" {
				continue
			}
			if _, ok := stat[rec.KeyID]; !ok {
				stat[rec.KeyID] = &row{KeyID: rec.KeyID, Note: keyNote[rec.KeyID]}
			}
			if rec.AmountCNY > 0 {
				stat[rec.KeyID].Recharge += rec.AmountCNY
			}
		}
		f.Close()
	}

	rows := make([]*row, 0, len(stat))
	for _, v := range stat {
		rows = append(rows, v)
	}
	sort.Slice(rows, func(i, j int) bool {
		switch metric {
		case "requests":
			return rows[i].Calls > rows[j].Calls
		case "recharge":
			return rows[i].Recharge > rows[j].Recharge
		default:
			return rows[i].Credits > rows[j].Credits
		}
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}

	writeJSON(w, 200, map[string]interface{}{
		"metric":  metric,
		"days":    days,
		"updated": time.Now().Unix(),
		"rows":    rows,
	})
}
