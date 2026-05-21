package proxy

import (
	"kiro-api-proxy/config"
	"net/http"
	"strconv"
	"time"
)

// GET /user/api/logs - request logs for this key (paginated)
//
// 默认只返回当天 (CST 0:00 ~ 24:00)。
// 支持 ?date=YYYY-MM-DD 查指定日期；?date=all 查全部历史（向后兼容）。
func (h *Handler) handleUserLogs(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	h.callLogsMu.RLock()
	defer h.callLogsMu.RUnlock()

	// 时间窗口
	loc := time.Now().Location()
	var startTs, endTs int64
	dateParam := r.URL.Query().Get("date")
	selectedDate := ""
	if dateParam != "all" {
		var anchor time.Time
		if dateParam != "" {
			if t, err := time.ParseInLocation("2006-01-02", dateParam, loc); err == nil {
				anchor = t
			}
		}
		if anchor.IsZero() {
			now := time.Now().In(loc)
			anchor = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		}
		startTs = anchor.Unix()
		endTs = anchor.AddDate(0, 0, 1).Unix()
		selectedDate = anchor.Format("2006-01-02")
	}

	// Collect logs for this key (most recent first)
	var allLogs []CallLog
	for i := len(h.callLogs) - 1; i >= 0; i-- {
		log := h.callLogs[i]
		if log.ApiKeyID != info.ID {
			continue
		}
		if endTs > 0 && (log.Timestamp < startTs || log.Timestamp >= endTs) {
			continue
		}
		log.Account = ""                    // Sanitize: don't expose account details to user
		log.ActualModel = log.OriginalModel // Sanitize: hide upstream model from user
		log.UpstreamCredits = 0             // Sanitize: hide raw upstream credits（反穿帮）
		allLogs = append(allLogs, log)
	}

	total := len(allLogs)

	// Pagination: ?page=1&limit=50
	page := 1
	limit := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	writeJSON(w, 200, map[string]interface{}{
		"logs":  allLogs[start:end],
		"total": total,
		"page":  page,
		"limit": limit,
		"date":  selectedDate, // "" 表示 ?date=all 全量
	})
}

// GET /user/api/activity?days=7 - 最近 N 天每日调用统计 + 当前促销资格状态
//
// 用于用户端"近 7 天活跃度"图表。
func (h *Handler) handleUserActivity(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 1 && v <= 30 {
			days = v
		}
	}

	daily := recentDailyStats(info.ID, days)
	var totalCalls, totalErrors int
	var totalTokens int64
	var totalCost float64
	for _, d := range daily {
		totalCalls += d.Calls
		totalErrors += d.Errors
		totalTokens += d.Tokens
		totalCost += d.CostUSD
	}

	// 当前促销活动状态 + 用户自身资格
	promo := config.GetPromotion()
	promoInfo := map[string]interface{}{"active": false}
	if promo != nil && promo.Enabled && promotionInTimeWindow(promo) {
		monthCNY := monthlyRechargeSumCNY(info.ID)
		recentCalls := recentCallCount(info.ID, promo.RecentCallsDays)
		whitelisted := isInPromotionWhitelist(promo, info.ID)
		eligible := promotionEligible(promo, info.ID, monthCNY, recentCalls)
		promoInfo = map[string]interface{}{
			"active":                true,
			"name":                  promo.Name,
			"eligible":              eligible,
			"whitelisted":           whitelisted,
			"minMonthlyRechargeCNY": promo.MinMonthlyRechargeCNY,
			"monthlyRechargeCNY":    monthCNY,
			"minRecentCalls":        promo.MinRecentCalls,
			"recentCallsDays":       promo.RecentCallsDays,
			"recentCalls":           recentCalls,
			"proPoolPriceUSD":       promo.ProPoolPriceUSD,
			"freePoolPriceUSD":      promo.FreePoolPriceUSD,
			"endTs":                 promo.EndTs,
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"days":         days,
		"daily":        daily,
		"totalCalls":   totalCalls,
		"totalErrors":  totalErrors,
		"totalTokens":  totalTokens,
		"totalCostUSD": totalCost,
		"promotion":    promoInfo,
	})
}
