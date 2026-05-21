package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// handleUserAPI routes /user/api/* requests.
func (h *Handler) handleUserAPI(w http.ResponseWriter, r *http.Request) {
	// Authenticate using API key as Bearer token
	keyInfo := h.resolveUserKey(r)
	if keyInfo == nil {
		writeJSON(w, 401, map[string]string{"error": "invalid or missing api key"})
		return
	}

	path := r.URL.Path
	// reseller 子路由优先：/user/api/reseller/* 全部走 reseller handler
	if strings.HasPrefix(path, "/user/api/reseller/") {
		h.handleResellerAPI(w, r)
		return
	}
	switch {
	case path == "/user/api/me" && r.Method == "GET":
		h.handleUserMe(w, keyInfo)
	case path == "/user/api/usage" && r.Method == "GET":
		h.handleUserUsage(w, keyInfo)
	case path == "/user/api/logs" && r.Method == "GET":
		h.handleUserLogs(w, r, keyInfo)
	case path == "/user/api/activity" && r.Method == "GET":
		h.handleUserActivity(w, r, keyInfo)
	case path == "/user/api/redeem" && r.Method == "POST":
		h.handleUserRedeem(w, r, keyInfo)
	case path == "/user/api/recharges" && r.Method == "GET":
		h.handleUserRecharges(w, r, keyInfo)
	case path == "/user/api/pricing" && r.Method == "GET":
		h.handleUserPricing(w)
	case path == "/user/api/promotion" && r.Method == "GET":
		h.handleUserPromotion(w, keyInfo)
	case path == "/user/api/leaderboard" && r.Method == "GET":
		h.handleUserLeaderboard(w, r, keyInfo)
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

// resolveUserKey extracts API key from Bearer token and returns ApiKeyInfo.
func (h *Handler) resolveUserKey(r *http.Request) *config.ApiKeyInfo {
	authHeader := r.Header.Get("Authorization")
	var key string
	if strings.HasPrefix(authHeader, "Bearer ") {
		key = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if key == "" {
		key = r.Header.Get("X-Api-Key")
	}
	if key == "" {
		return nil
	}
	return config.FindApiKey(key)
}

// GET /user/api/me
func (h *Handler) handleUserMe(w http.ResponseWriter, info *config.ApiKeyInfo) {
	resp := map[string]interface{}{
		"id":             info.ID,
		"tier":           info.Tier,
		"plan":           info.Plan,
		"balance":        info.Balance,
		"giftBalance":    info.GiftBalance,
		"totalBalance":   info.Balance + info.GiftBalance,
		"totalRecharged": info.TotalRecharged,
		"totalGifted":    info.TotalGifted,
		"credits":        info.Credits,
		"expiresAt":      info.ExpiresAt,
		"enabled":        info.Enabled,
		"requests":       info.Requests,
		"tokens":         info.Tokens,
		"models":         info.Models,
		"createdAt":      info.CreatedAt,
		"lastUsed":       info.LastUsed,
		"note":           info.Note,
	}

	// 代理身份：仅 reseller 自己能看到（让前端导航 v-if 显示"代理管理"菜单）。
	// 注意：永远不返回 parentKeyId 具体值 —— 子 key 不应知道自己被哪个 reseller 代理。
	if info.IsReseller {
		resp["isReseller"] = true
		resp["maxChildKeys"] = info.MaxChildKeys
		resp["resellerDiscount"] = info.ResellerDiscount
		resp["soldToChildren"] = info.SoldToChildren
	}
	// 子 key 标记：让前端隐藏充值/活动 UI，并显示"请联系服务商"提示
	if info.ParentKeyID != "" {
		resp["isChildKey"] = true
	}

	// 天卡速率上限：仅当 key 处于"按时长收费"活跃期时返回，让用户面板能展示。
	// 过期 / 纯 credit 用户不返回此字段（避免误导：以为还有限速）。
	if isTimedActive(info) {
		resp["rateLimitPerMin"] = getEffectiveRPM(info)
	}

	// Check access validity
	errType, err := config.ValidateKeyAccess(info)
	if err != nil {
		resp["status"] = errType
		resp["statusMessage"] = err.Error()
	} else {
		resp["status"] = "active"
	}

	// Time remaining for timed/hybrid plans
	if info.ExpiresAt > 0 {
		remaining := info.ExpiresAt - time.Now().Unix()
		if remaining > 0 {
			resp["daysRemaining"] = remaining / 86400
		} else {
			resp["daysRemaining"] = 0
		}
	}

	writeJSON(w, 200, resp)
}

// GET /user/api/usage - usage stats grouped by model
func (h *Handler) handleUserUsage(w http.ResponseWriter, info *config.ApiKeyInfo) {
	h.callLogsMu.RLock()
	defer h.callLogsMu.RUnlock()

	modelStats := make(map[string]map[string]interface{})
	var totalInput, totalOutput int
	var totalCredits float64
	count := 0

	for _, log := range h.callLogs {
		if log.ApiKeyID != info.ID || log.Status == "error" {
			continue
		}
		count++
		totalInput += log.InputTokens
		totalOutput += log.OutputTokens
		// v3 token 模式 Credits=0 但 UpstreamCredits>0；用 UpstreamCredits 兜底，
		// 否则 user dashboard 显示 0 credits 但实际已扣费
		if log.Credits > 0 {
			totalCredits += log.Credits
		} else if log.UpstreamCredits > 0 {
			totalCredits += log.UpstreamCredits
		}

		model := log.OriginalModel
		if _, ok := modelStats[model]; !ok {
			modelStats[model] = map[string]interface{}{
				"requests":     0,
				"inputTokens":  0,
				"outputTokens": 0,
				"credits":      0.0,
			}
		}
		ms := modelStats[model]
		ms["requests"] = ms["requests"].(int) + 1
		ms["inputTokens"] = ms["inputTokens"].(int) + log.InputTokens
		ms["outputTokens"] = ms["outputTokens"].(int) + log.OutputTokens
		// 同上 token 模式兜底
		credPerCall := log.Credits
		if credPerCall == 0 && log.UpstreamCredits > 0 {
			credPerCall = log.UpstreamCredits
		}
		ms["credits"] = ms["credits"].(float64) + credPerCall
	}

	writeJSON(w, 200, map[string]interface{}{
		"totalRequests":     count,
		"totalInputTokens":  totalInput,
		"totalOutputTokens": totalOutput,
		"totalCredits":      totalCredits,
		"models":            modelStats,
	})
}

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

// POST /user/api/redeem - redeem activation code
func (h *Handler) handleUserRedeem(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	// 子 key 不允许兑激活码：钱由所属 reseller 转账下发，
	// 否则 child 兑的钱会在 reseller 删 child 时回流（参见 RefundChildBalance），造成资金错位。
	if info != nil && info.ParentKeyID != "" {
		writeJSON(w, 403, map[string]string{"error": "子 Key 不能兑换激活码，请联系您的服务商充值"})
		return
	}

	// IP rate limiting for brute force prevention
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}
	if allowed, reason := CheckRedeemRateLimit(ip); !allowed {
		writeJSON(w, 429, map[string]string{"error": reason})
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Code == "" {
		writeJSON(w, 400, map[string]string{"error": "code is required"})
		return
	}

	// Capture before state for receipt
	balanceBefore := info.Balance
	giftBefore := info.GiftBalance
	expiresAtBefore := info.ExpiresAt

	// 在兑换前先记下激活码金额（兑换后激活码会被删除）
	var codeAmountInput float64  // 兑换码原始 amount（balance 类型为 ¥CNY，days 类型为天数）
	var codeSalePriceCNY float64 // 仅 days/time 类型：admin 设的销售价格（¥），写入流水作 revenue 来源
	{
		codes := config.GetActivationCodes()
		for _, ac := range codes {
			if ac.Code == req.Code {
				codeAmountInput = ac.Amount
				codeSalePriceCNY = ac.SalePriceCNY
				break
			}
		}
	}

	codeType, err := config.RedeemActivationCode(req.Code, info.ID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	// Fetch updated key info
	updated := config.FindApiKeyByID(info.ID)
	if updated == nil {
		writeJSON(w, 500, map[string]string{"error": "failed to fetch updated info"})
		return
	}

	fmt.Printf("[Redeem] key=%s code=%s type=%s balance=¥%.2f expiresAt=%d\n",
		info.ID[:8], req.Code, codeType, updated.Balance, updated.ExpiresAt)

	// 写充值流水（金额关键，立即落盘）
	{
		now := time.Now()
		cst := time.FixedZone("CST", 8*3600)
		recType := "code_redeem"
		var amountUSD, amountCNY float64
		switch codeType {
		case "balance":
			// codeAmountInput 是 CNY，转 USD face
			amountCNY = codeAmountInput
			amountUSD = codeAmountInput / config.CNYPerUSDFace
		case "days", "time":
			recType = "code_redeem_days"
			// 天卡兑换收入 = ac.SalePriceCNY（admin 创建天卡时填的售价）。
			// 老天卡 / 白送 → 0，不计入 revenue（向前兼容）。
			amountCNY = codeSalePriceCNY
			amountUSD = amountCNY / config.CNYPerUSDFace
		}
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		appendRechargeRecord(RechargeRecord{
			Time:          now.In(cst).Format("01-02 15:04:05"),
			Timestamp:     now.Unix(),
			KeyID:         info.ID,
			KeyNote:       updated.Note,
			Type:          recType,
			Code:          req.Code,
			AmountUSD:     amountUSD,
			AmountCNY:     amountCNY,
			BalanceBefore: balanceBefore,
			BalanceAfter:  updated.Balance,
			GiftBefore:    giftBefore,
			GiftAfter:     updated.GiftBalance,
			Operator:      "user",
			Note:          fmt.Sprintf("self-redeem %s", codeType),
			IP:            ip,
		})
	}

	// Find the code amount for receipt (convert CNY → face-value USD)
	var amount float64
	switch codeType {
	case "balance":
		amount = codeAmountInput / config.CNYPerUSDFace // ¥ → $ face value
	case "days", "time":
		amount = codeAmountInput // days: keep as-is
	}

	writeJSON(w, 200, map[string]interface{}{
		"type":            codeType,
		"amount":          amount,
		"balance":         updated.Balance,
		"balanceBefore":   balanceBefore,
		"balanceAfter":    updated.Balance,
		"expiresAt":       updated.ExpiresAt,
		"expiresAtBefore": expiresAtBefore,
	})
}

// GET /user/api/pricing - public pricing info
func (h *Handler) handleUserPricing(w http.ResponseWriter) {
	pricing := config.GetPricing()
	out := struct {
		config.PricingConfig
		SupportedModels map[string][]string `json:"supportedModels"`
	}{
		PricingConfig:   pricing,
		SupportedModels: SupportedModels(),
	}
	writeJSON(w, 200, out)
}

// GET /user/api/recharges - 当前 key 的充值历史（分页）
func (h *Handler) handleUserRecharges(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
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
	records, total := readRechargeRecords(info.ID, page, limit)
	// 用户端隐藏 IP（隐私）
	out := make([]map[string]interface{}, len(records))
	for i, r := range records {
		out[i] = map[string]interface{}{
			"time":           r.Time,
			"timestamp":      r.Timestamp,
			"type":           r.Type,
			"code":           r.Code,
			"amountUSD":      r.AmountUSD,
			"amountCNY":      r.AmountCNY,
			"balanceBefore":  r.BalanceBefore,
			"balanceAfter":   r.BalanceAfter,
			"giftBefore":     r.GiftBefore,
			"giftAfter":      r.GiftAfter,
			"note":           r.Note,
		}
	}
	writeJSON(w, 200, map[string]interface{}{
		"records": out,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// GET /user/api/promotion - 当前活动状态 + 自己的资格
func (h *Handler) handleUserPromotion(w http.ResponseWriter, info *config.ApiKeyInfo) {
	promo := config.GetPromotion()
	if promo == nil || !promo.Enabled || !promotionInTimeWindow(promo) {
		writeJSON(w, 200, map[string]interface{}{
			"active": false,
		})
		return
	}
	// 计算用户自身资格
	monthCNY := monthlyRechargeSumCNY(info.ID)
	recentCalls := recentCallCount(info.ID, promo.RecentCallsDays)
	whitelisted := isInPromotionWhitelist(promo, info.ID)
	eligible := promotionEligible(promo, info.ID, monthCNY, recentCalls)
	// 子 key 与 reseller 都不参与活动（防止双重套利）。给前端一个原因字段方便展示。
	excludedReason := ""
	if info.ParentKeyID != "" {
		excludedReason = "child_key"
		eligible = false
	} else if info.IsReseller {
		excludedReason = "reseller"
		eligible = false
	}

	writeJSON(w, 200, map[string]interface{}{
		"active":              true,
		"name":                promo.Name,
		"proPoolPriceUSD":     promo.ProPoolPriceUSD,
		"freePoolPriceUSD":    promo.FreePoolPriceUSD,
		"minMonthlyRechargeCNY": promo.MinMonthlyRechargeCNY,
		"minRecentCalls":      promo.MinRecentCalls,
		"recentCallsDays":     promo.RecentCallsDays,
		"startTs":             promo.StartTs,
		"endTs":               promo.EndTs,
		"excludedReason":      excludedReason,
		"you": map[string]interface{}{
			"eligible":         eligible,
			"monthlyRechargeCNY": monthCNY,
			"recentCalls":      recentCalls,
			"whitelisted":      whitelisted,
		},
	})
}

// writeJSON sends a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
