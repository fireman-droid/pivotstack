package proxy

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) apiGetCodes(w http.ResponseWriter, r *http.Request) {
	codes := config.GetActivationCodes()
	if codes == nil {
		codes = []config.ActivationCode{}
	}

	// ?used=true 返回已使用激活码（默认或 used=false 返回未使用）。
	wantUsed := r.URL.Query().Get("used") == "true"
	result := []config.ActivationCode{}
	for _, c := range codes {
		if c.Used == wantUsed {
			result = append(result, c)
		}
	}

	json.NewEncoder(w).Encode(result)
}

// POST /admin/api/codes/cleanup - physically remove all used codes from data store
func (h *Handler) apiCleanupCodes(w http.ResponseWriter, _ *http.Request) {
	cleaned := config.CleanupUsedCodes()
	AuditLog("codes_cleanup", "admin", fmt.Sprintf("Removed %d used codes", cleaned))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"cleaned": cleaned,
	})
}

// POST /admin/api/codes - batch create activation codes
func (h *Handler) apiCreateCodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string  `json:"type"`         // "balance" | "days" | "time"
		Amount       float64 `json:"amount"`       // CNY (for balance) or days/seconds (for days/time)
		Tier         string  `json:"tier"`         // "free" | "pro" (only for type=days/time)
		Count        int     `json:"count"`        // how many codes to generate
		Note         string  `json:"note"`
		SalePriceCNY float64 `json:"salePriceCNY"` // 仅 days/time：admin 卖给客户的实际价格（¥），用于利润计算
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.Type != "balance" && req.Type != "days" && req.Type != "time" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "type must be 'balance', 'days', or 'time'"})
		return
	}
	if req.Amount <= 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "amount must be positive"})
		return
	}
	if req.Count <= 0 || req.Count > 100 {
		req.Count = 1
	}

	var codes []string
	now := time.Now().Unix()
	for i := 0; i < req.Count; i++ {
		code := generateActivationCode()
		ac := config.ActivationCode{
			Code:      code,
			Type:      req.Type,
			Amount:    req.Amount,
			CreatedAt: now,
			Note:      req.Note,
		}
		if (req.Type == "days" || req.Type == "time") && (req.Tier == "free" || req.Tier == "pro") {
			ac.Tier = req.Tier
		}
		// 仅天卡需要"售价"字段；balance 类型 amount 本身就是 CNY 售价，不重复填
		if (req.Type == "days" || req.Type == "time") && req.SalePriceCNY > 0 {
			ac.SalePriceCNY = req.SalePriceCNY
		}
		if err := config.AddActivationCode(ac); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		codes = append(codes, code)
	}

	AuditLog("codes_create", "admin", fmt.Sprintf("type=%s amount=%.2f count=%d note=%s", req.Type, req.Amount, len(codes), req.Note))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"codes":   codes,
		"count":   len(codes),
	})
}

// DELETE /admin/api/codes/:code
func (h *Handler) apiDeleteCode(w http.ResponseWriter, _ *http.Request, code string) {
	if err := config.DeleteActivationCode(code); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("code_delete", "admin", fmt.Sprintf("code=%s", code))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /admin/api/abuse
func (h *Handler) apiGetAbuse(w http.ResponseWriter, _ *http.Request) {
	flagged := GetFlaggedKeys()
	if flagged == nil {
		flagged = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(flagged)
}

// POST /admin/api/abuse/:keyId/clear
func (h *Handler) apiClearAbuse(w http.ResponseWriter, _ *http.Request, keyID string) {
	ClearFlag(keyID)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ==================== Engagement / Insights handlers ====================

// apiInactiveKeys returns API keys idle for >= ?days (default 30).
// daysIdle is computed from LastUsed; if LastUsed=0 (never used), CreatedAt is the anchor.
func (h *Handler) apiInactiveKeys(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 3650 {
			days = v
		}
	}
	keys := config.GetAllApiKeys()
	// merge in-memory stats
	h.apiKeyStatsMu.RLock()
	for i := range keys {
		if stats, ok := h.apiKeyStats[keys[i].ID]; ok {
			keys[i].LastUsed = stats.LastUsed
			keys[i].Requests = stats.Requests
		}
	}
	h.apiKeyStatsMu.RUnlock()

	now := time.Now().Unix()
	type inactiveKey struct {
		ID          string  `json:"id"`
		Note        string  `json:"note"`
		LastUsed    int64   `json:"lastUsed"`
		CreatedAt   int64   `json:"createdAt"`
		DaysIdle    int     `json:"daysIdle"`
		NeverUsed   bool    `json:"neverUsed"`
		Balance     float64 `json:"balance"`
		GiftBalance float64 `json:"giftBalance"`
		Requests    int64   `json:"requests"`
		Enabled     bool    `json:"enabled"`
	}
	out := make([]inactiveKey, 0)
	for _, k := range keys {
		anchor := k.LastUsed
		never := false
		if anchor == 0 {
			anchor = k.CreatedAt
			never = true
		}
		if anchor == 0 {
			continue
		}
		idle := int((now - anchor) / 86400)
		if idle < days {
			continue
		}
		out = append(out, inactiveKey{
			ID:          k.ID,
			Note:        k.Note,
			LastUsed:    k.LastUsed,
			CreatedAt:   k.CreatedAt,
			DaysIdle:    idle,
			NeverUsed:   never,
			Balance:     k.Balance,
			GiftBalance: k.GiftBalance,
			Requests:    k.Requests,
			Enabled:     k.Enabled,
		})
	}
	// sort by daysIdle DESC
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DaysIdle > out[i].DaysIdle {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"days":  days,
		"count": len(out),
		"keys":  out,
	})
}

// GET /admin/api/leaderboard/config
func (h *Handler) apiGetLeaderboardConfig(w http.ResponseWriter, _ *http.Request) {
	enabled, fakeUsers := config.GetLeaderboardConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":   enabled,
		"fakeUsers": fakeUsers,
	})
}

// PUT /admin/api/leaderboard/config
func (h *Handler) apiUpdateLeaderboardConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled   bool `json:"enabled"`
		FakeUsers int  `json:"fakeUsers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if err := config.UpdateLeaderboardConfig(req.Enabled, req.FakeUsers); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	AuditLog("leaderboard_config_update", "admin", fmt.Sprintf("enabled=%v fakeUsers=%d", req.Enabled, req.FakeUsers))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /admin/api/apikeys/clear-gift
// Body must contain {"confirm": true}. Zeros GiftBalance on every key (does NOT touch Balance / TotalGifted).
func (h *Handler) apiClearAllGift(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if !req.Confirm {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "confirm=true is required"})
		return
	}
	count, total := config.ClearAllGiftBalances()
	AuditLog("clear_all_gift", "admin", fmt.Sprintf("cleared=%d totalGiftCleared=$%.4f", count, total))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"cleared":          count,
		"totalGiftCleared": total,
	})
}

// generateActivationCode creates a code like KIRO-XXXX-XXXX-XXXX
func generateActivationCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1
	seg := func() string {
		b := make([]byte, 4)
		randBytes := make([]byte, 4)
		crand.Read(randBytes)
		for i := range b {
			b[i] = chars[int(randBytes[i])%len(chars)]
		}
		return string(b)
	}
	return "KIRO-" + seg() + "-" + seg() + "-" + seg()
}
