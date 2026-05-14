package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ==================== Promotion endpoints ====================

// GET /admin/api/promotion
func (h *Handler) apiGetPromotion(w http.ResponseWriter, _ *http.Request) {
	promo := config.GetPromotion()
	if promo == nil {
		writeJSON(w, 200, map[string]interface{}{"enabled": false})
		return
	}
	writeJSON(w, 200, promo)
}

// PUT /admin/api/promotion
func (h *Handler) apiUpdatePromotion(w http.ResponseWriter, r *http.Request) {
	var p config.PromotionConfig
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	// v1→v2 兜底：admin 用旧 UI 只发 ProPoolPriceUSD/FreePoolPriceUSD（不发 ModelPrices/Default*），
	// 自动映射到 v2 兜底字段，让计费立刻生效。
	if len(p.ModelPrices) == 0 && p.DefaultProPriceUSD == 0 && p.DefaultFreePriceUSD == 0 {
		config.MigratePromotionToModelLevel(&p)
	}
	operator := operatorFromRequest(r)
	if err := config.UpdatePromotion(&p, operator); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("promotion_update", operator,
		fmt.Sprintf("enabled=%v name=%q modelPrices=%d defaultPro=$%.4f defaultFree=$%.4f minRecharge=¥%.0f minCalls=%d days=%d whitelist=%d",
			p.Enabled, p.Name, len(p.ModelPrices), p.DefaultProPriceUSD, p.DefaultFreePriceUSD,
			p.MinMonthlyRechargeCNY, p.MinRecentCalls, p.RecentCallsDays, len(p.Whitelist)))
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /admin/api/promotion/whitelist  body: {"keyID": "..."}
func (h *Handler) apiAddPromotionWhitelist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		KeyID string `json:"keyID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		writeJSON(w, 400, map[string]string{"error": "keyID required"})
		return
	}
	operator := operatorFromRequest(r)
	if err := config.AddPromotionWhitelist(req.KeyID, operator); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("promotion_whitelist_add", operator, fmt.Sprintf("keyID=%s", req.KeyID))
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// DELETE /admin/api/promotion/whitelist/{keyID}
func (h *Handler) apiRemovePromotionWhitelist(w http.ResponseWriter, r *http.Request, keyID string) {
	operator := operatorFromRequest(r)
	if err := config.RemovePromotionWhitelist(keyID, operator); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("promotion_whitelist_remove", operator, fmt.Sprintf("keyID=%s", keyID))
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// ==================== Recharge endpoints ====================

// GET /admin/api/recharges?key_id=xxx&page=1&limit=50
func (h *Handler) apiGetRecharges(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("key_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	records, total := readRechargeRecords(keyID, page, limit)
	writeJSON(w, 200, map[string]interface{}{
		"records": records,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// GET /admin/api/apikeys/{id}/recharges
func (h *Handler) apiGetApiKeyRecharges(w http.ResponseWriter, r *http.Request, keyID string) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	records, total := readRechargeRecords(keyID, page, limit)
	writeJSON(w, 200, map[string]interface{}{
		"records": records,
		"total":   total,
	})
}

// ==================== Insights endpoints ====================

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

// GET /admin/api/insights/freeloaders?since=ts&min_calls=5
// since: 活动开始 timestamp（unix sec），不传则用 promotion.StartTs
// 算法：since 之后的"活动期"调用 vs since 之前 7 天"平时"调用
// 对每个 key 输出 score（白嫖度评分 0-10）
func (h *Handler) apiInsightsFreeloaders(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if since == 0 {
		if promo := config.GetPromotion(); promo != nil && promo.StartTs > 0 {
			since = promo.StartTs
		}
	}
	if since == 0 {
		// 默认用 24 小时前
		since = time.Now().Unix() - 86400
	}
	minCalls := 5
	if v, _ := strconv.Atoi(r.URL.Query().Get("min_calls")); v > 0 {
		minCalls = v
	}
	normalStart := since - 7*86400 // 平时窗口：since 之前 7 天

	type stat struct {
		Normal  int     // 平时调用次数
		Active  int     // 活动期调用次数
		Credits float64 // 活动期消耗 credits
		CostUSD float64 // 活动期实际付费 USD（享受了活动价的）
	}
	keyStats := map[string]*stat{}

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"rows": []interface{}{}})
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry CallLog
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.ApiKeyID == "" {
			continue
		}
		if _, ok := keyStats[entry.ApiKeyID]; !ok {
			keyStats[entry.ApiKeyID] = &stat{}
		}
		s := keyStats[entry.ApiKeyID]
		if entry.Timestamp >= since {
			s.Active++
			s.Credits += entry.Credits
			s.CostUSD += entry.CostUSD
		} else if entry.Timestamp >= normalStart {
			s.Normal++
		}
	}

	apiKeys := config.GetAllApiKeys()
	keyInfoMap := map[string]*config.ApiKeyInfo{}
	for i := range apiKeys {
		keyInfoMap[apiKeys[i].ID] = &apiKeys[i]
	}

	const proPriceOriginal = 1.4
	const cnyPerUSD = config.CNYPerUSDFace
	type row struct {
		KeyID       string  `json:"keyId"`
		Note        string  `json:"note"`
		Score       int     `json:"score"`
		RechargeCNY float64 `json:"rechargeCNY"`
		Normal      int     `json:"normal"`
		Active      int     `json:"active"`
		Surge       float64 `json:"surge"`
		ActivePaidCNY float64 `json:"activePaidCNY"`
		SavedCNY    float64 `json:"savedCNY"`
	}
	rows := []row{}
	for kid, s := range keyStats {
		if s.Active < minCalls {
			continue
		}
		info := keyInfoMap[kid]
		var rch float64
		var note string
		if info != nil {
			rch = info.TotalRecharged * cnyPerUSD
			note = info.Note
		}
		surge := float64(s.Active)
		if s.Normal > 0 {
			surge = float64(s.Active) / float64(s.Normal)
		}
		// 评分（参考 chat 里 Python 脚本逻辑）
		score := 0
		if rch < 30 {
			score += 3
		} else if rch < 50 {
			score += 2
		} else if rch < 100 {
			score += 1
		}
		if s.Normal == 0 {
			score += 3
		} else if s.Normal < 10 {
			score += 2
		} else if s.Normal < 30 {
			score += 1
		}
		if s.Active > 50 {
			score += 2
		} else if s.Active > 20 {
			score += 1
		}
		if s.Normal > 0 {
			if surge > 10 {
				score += 2
			} else if surge > 3 {
				score += 1
			}
		}
		origUSD := s.Credits * proPriceOriginal
		savedCNY := (origUSD - s.CostUSD) * cnyPerUSD
		rows = append(rows, row{
			KeyID:         kid,
			Note:          note,
			Score:         score,
			RechargeCNY:   rch,
			Normal:        s.Normal,
			Active:        s.Active,
			Surge:         surge,
			ActivePaidCNY: s.CostUSD * cnyPerUSD,
			SavedCNY:      savedCNY,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].SavedCNY > rows[j].SavedCNY
	})

	writeJSON(w, 200, map[string]interface{}{
		"since":    since,
		"normalStart": normalStart,
		"rows":     rows,
		"updated":  time.Now().Unix(),
	})
}

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

	const cnyPerUSD = config.CNYPerUSDFace
	writeJSON(w, 200, map[string]interface{}{
		"date":             dateStr,
		"dayStart":         dayStart,
		"dayEnd":           dayEnd,
		"calls":            calls,
		"errors":           errors,
		"uniqueKeys":       len(uniqueKeys),
		"credits":          sumCredits,
		"upstreamCredits":  sumUpstream,
		"paidCredits":      sumPaidCredits,
		"giftedCredits":    sumGiftedCredits,
		"costUSD":          sumCostUSD,           // 实收营收
		"costCNY":          sumCostUSD * cnyPerUSD,
		"rechargeCNY":      dayRechargeCNY,
		"rechargeUSD":      dayRechargeUSD,
		"rechargersCount":  len(rechargers),
	})
}

// operatorFromRequest 提取操作人标识（用于 audit log）
func operatorFromRequest(r *http.Request) string {
	if u := r.Header.Get("X-Admin-User"); u != "" {
		return u
	}
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}
	return "admin@" + ip
}
