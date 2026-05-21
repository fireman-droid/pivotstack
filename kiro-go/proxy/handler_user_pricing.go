package proxy

import (
	"kiro-api-proxy/config"
	"net/http"
	"strconv"
)

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
			"time":          r.Time,
			"timestamp":     r.Timestamp,
			"type":          r.Type,
			"code":          r.Code,
			"amountUSD":     r.AmountUSD,
			"amountCNY":     r.AmountCNY,
			"balanceBefore": r.BalanceBefore,
			"balanceAfter":  r.BalanceAfter,
			"giftBefore":    r.GiftBefore,
			"giftAfter":     r.GiftAfter,
			"note":          r.Note,
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
		"active":                true,
		"name":                  promo.Name,
		"proPoolPriceUSD":       promo.ProPoolPriceUSD,
		"freePoolPriceUSD":      promo.FreePoolPriceUSD,
		"minMonthlyRechargeCNY": promo.MinMonthlyRechargeCNY,
		"minRecentCalls":        promo.MinRecentCalls,
		"recentCallsDays":       promo.RecentCallsDays,
		"startTs":               promo.StartTs,
		"endTs":                 promo.EndTs,
		"excludedReason":        excludedReason,
		"you": map[string]interface{}{
			"eligible":           eligible,
			"monthlyRechargeCNY": monthCNY,
			"recentCalls":        recentCalls,
			"whitelisted":        whitelisted,
		},
	})
}
