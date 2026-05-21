package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
)

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
