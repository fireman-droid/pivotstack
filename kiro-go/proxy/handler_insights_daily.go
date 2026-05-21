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

// GET /admin/api/insights/daily?date=YYYY-MM-DD
// 当日总账：调用 / 独立 keys / 总 credits / 总 cost / 总充值额
func (h *Handler) apiInsightsDaily(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	loc := time.FixedZone("CST", 8*3600)
	now := time.Now().In(loc)
	var dayStart, dayEnd int64
	if dateStr == "" {
		// 今天
		y, m, d := now.Date()
		t := time.Date(y, m, d, 0, 0, 0, 0, loc)
		dayStart = t.Unix()
		dayEnd = t.Add(24 * time.Hour).Unix()
	} else {
		t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid date format, want YYYY-MM-DD"})
			return
		}
		dayStart = t.Unix()
		dayEnd = t.Add(24 * time.Hour).Unix()
	}

	calls := 0
	errors := 0
	uniqueKeys := map[string]bool{}
	var sumCredits, sumCostUSD, sumPaidCredits, sumGiftedCredits, sumUpstream float64

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	if f, err := os.Open(logPath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var entry CallLog
			if json.Unmarshal(scanner.Bytes(), &entry) != nil {
				continue
			}
			if entry.Timestamp < dayStart || entry.Timestamp >= dayEnd {
				continue
			}
			calls++
			if entry.Status == "error" {
				errors++
			}
			if entry.ApiKeyID != "" {
				uniqueKeys[entry.ApiKeyID] = true
			}
			sumCredits += entry.Credits
			sumCostUSD += entry.CostUSD
			sumPaidCredits += entry.PaidCredits
			sumGiftedCredits += entry.GiftedCredits
			sumUpstream += entry.UpstreamCredits
		}
		f.Close()
	}

	// 当日充值
	var dayRechargeCNY, dayRechargeUSD float64
	rechargers := map[string]bool{}
	rechargePath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	if f, err := os.Open(rechargePath); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var rec RechargeRecord
			if json.Unmarshal(scanner.Bytes(), &rec) != nil {
				continue
			}
			if rec.Timestamp < dayStart || rec.Timestamp >= dayEnd {
				continue
			}
			if rec.AmountCNY > 0 {
				dayRechargeCNY += rec.AmountCNY
				dayRechargeUSD += rec.AmountUSD
				rechargers[rec.KeyID] = true
			}
		}
		f.Close()
	}

	writeJSON(w, 200, map[string]interface{}{
		"date":            dateStr,
		"dayStart":        dayStart,
		"dayEnd":          dayEnd,
		"calls":           calls,
		"errors":          errors,
		"uniqueKeys":      len(uniqueKeys),
		"credits":         sumCredits,
		"upstreamCredits": sumUpstream,
		"paidCredits":     sumPaidCredits,
		"giftedCredits":   sumGiftedCredits,
		"costUSD":         sumCostUSD, // 实收营收
		"costCNY":         config.CNYFromVirtualUSD(sumCostUSD),
		"rechargeCNY":     dayRechargeCNY,
		"rechargeUSD":     dayRechargeUSD,
		"rechargersCount": len(rechargers),
	})
}
