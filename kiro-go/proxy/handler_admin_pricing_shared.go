package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
)

// v9: pricing-center 已下线（apiGetPricing / apiUpdatePricing / buildPricingPreview
// / buildPromotionPreview 全部移除）。看板分析归 OPS /business-board；
// 文件保留是因为 /stealth handlers 仍在 routeAdminPricingAndProfit 里挂着。

// GET /admin/api/stealth
func (h *Handler) apiGetStealth(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(config.GetStealth())
}

// PUT /admin/api/stealth
func (h *Handler) apiUpdateStealth(w http.ResponseWriter, r *http.Request) {
	var s config.StealthConfig
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateStealth(s); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("stealth_update", "admin", fmt.Sprintf("enabled=%v opus=%.2f sonnet=%.2f opusTarget=%s sonnetTarget=%s",
		s.Enabled, s.OpusFakeRatio, s.SonnetFakeRatio, s.OpusFakeTarget, s.SonnetFakeTarget))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
