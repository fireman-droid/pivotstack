package proxy

import (
	"bufio"
	"encoding/json"
	"kiro-api-proxy/config"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type adminRechargesSummary struct {
	TodayCNY      float64 `json:"todayCNY"`
	MonthCNY      float64 `json:"monthCNY"`
	AvgCNY        float64 `json:"avgCNY"`
	ReturningRate float64 `json:"returningRate"`
}

// adminRechargeRow 嵌入原 RechargeRecord 并补充 ¥ 单位字段。
// balance_before/after 内部是虚拟 $（user.Balance 同一口径），
// 前端不应直接当 ¥ 显示，所以这里换算好 *_cny 返回。
type adminRechargeRow struct {
	RechargeRecord
	BalanceBeforeCNY *float64 `json:"balance_before_cny,omitempty"`
	BalanceAfterCNY  *float64 `json:"balance_after_cny,omitempty"`
}

func wrapAdminRechargeRows(records []RechargeRecord) []adminRechargeRow {
	psdpy := config.GetPivotStackDollarsPerYuan()
	if psdpy <= 0 {
		psdpy = config.DefaultPivotStackDollarsPerYuan
	}
	out := make([]adminRechargeRow, len(records))
	for i, rec := range records {
		before := rec.BalanceBefore / psdpy
		after := rec.BalanceAfter / psdpy
		out[i] = adminRechargeRow{
			RechargeRecord:   rec,
			BalanceBeforeCNY: &before,
			BalanceAfterCNY:  &after,
		}
	}
	return out
}

// GET /admin/api/recharges?limit=200&offset=0&type=&search=&from=&to=
// 全平台 RechargeRecord 流水 + 汇总指标。
func (h *Handler) apiAdminRecharges(w http.ResponseWriter, r *http.Request) {
	records, err := readAllAdminRechargeRecords()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Timestamp == records[j].Timestamp {
			return records[i].Time > records[j].Time
		}
		return records[i].Timestamp > records[j].Timestamp
	})

	summary := summarizeAdminRecharges(records)
	filtered := filterAdminRecharges(records, adminRechargeFilterFromRequest(r))
	total := len(filtered)

	limit, offset := adminRechargePagination(r)
	if offset >= total {
		filtered = []RechargeRecord{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		filtered = filtered[offset:end]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"records": wrapAdminRechargeRows(filtered),
		"total":   total,
		"summary": summary,
	})
}

func readAllAdminRechargeRecords() ([]RechargeRecord, error) {
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []RechargeRecord{}, nil
		}
		return nil, err
	}
	defer f.Close()

	records := make([]RechargeRecord, 0, 256)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var rec RechargeRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

type adminRechargeFilter struct {
	typ    string
	search string
	from   int64
	to     int64
}

func adminRechargeFilterFromRequest(r *http.Request) adminRechargeFilter {
	q := r.URL.Query()
	return adminRechargeFilter{
		typ:    strings.TrimSpace(q.Get("type")),
		search: strings.ToLower(strings.TrimSpace(q.Get("search"))),
		from:   parseAdminRechargeUnix(q.Get("from")),
		to:     parseAdminRechargeUnix(q.Get("to")),
	}
}

func parseAdminRechargeUnix(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func filterAdminRecharges(records []RechargeRecord, f adminRechargeFilter) []RechargeRecord {
	if f.typ == "" && f.search == "" && f.from == 0 && f.to == 0 {
		out := make([]RechargeRecord, len(records))
		copy(out, records)
		return out
	}
	out := make([]RechargeRecord, 0, len(records))
	for _, rec := range records {
		if f.typ != "" && rec.Type != f.typ {
			continue
		}
		if f.from > 0 && rec.Timestamp < f.from {
			continue
		}
		if f.to > 0 && rec.Timestamp > f.to {
			continue
		}
		if f.search != "" && !adminRechargeMatchesSearch(rec, f.search) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func adminRechargeMatchesSearch(rec RechargeRecord, search string) bool {
	return strings.Contains(strings.ToLower(rec.KeyNote), search) ||
		strings.Contains(strings.ToLower(rec.Code), search) ||
		strings.Contains(strings.ToLower(rec.Note), search)
}

func adminRechargePagination(r *http.Request) (limit, offset int) {
	limit = 200
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 1000 {
		limit = 1000
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return limit, offset
}

func summarizeAdminRecharges(records []RechargeRecord) adminRechargesSummary {
	cst := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cst)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cst).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Unix()
	nowUnix := now.Unix()

	var summary adminRechargesSummary
	var avgSum float64
	rechargeCounts := make(map[string]int)
	for _, rec := range records {
		avgSum += rec.AmountCNY
		if !isAdminRevenueRechargeType(rec.Type) {
			continue
		}
		amount := math.Abs(rec.AmountCNY)
		if rec.Timestamp >= todayStart && rec.Timestamp <= nowUnix {
			summary.TodayCNY += amount
		}
		if rec.Timestamp >= monthStart && rec.Timestamp <= nowUnix {
			summary.MonthCNY += amount
		}
		if rec.KeyID != "" {
			rechargeCounts[rec.KeyID]++
		}
	}
	if len(records) > 0 {
		summary.AvgCNY = avgSum / float64(len(records))
	}
	totalRechargeKeys := len(rechargeCounts)
	if totalRechargeKeys == 0 {
		return summary
	}
	returningKeys := 0
	for _, count := range rechargeCounts {
		if count >= 2 {
			returningKeys++
		}
	}
	summary.ReturningRate = float64(returningKeys) / float64(totalRechargeKeys)
	return summary
}

func isAdminRevenueRechargeType(typ string) bool {
	return typ == "code_redeem" || typ == "admin_balance"
}
