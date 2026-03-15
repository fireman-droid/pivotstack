package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
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
	switch {
	case path == "/user/api/me" && r.Method == "GET":
		h.handleUserMe(w, keyInfo)
	case path == "/user/api/usage" && r.Method == "GET":
		h.handleUserUsage(w, keyInfo)
	case path == "/user/api/logs" && r.Method == "GET":
		h.handleUserLogs(w, r, keyInfo)
	case path == "/user/api/redeem" && r.Method == "POST":
		h.handleUserRedeem(w, r, keyInfo)
	case path == "/user/api/pricing" && r.Method == "GET":
		h.handleUserPricing(w)
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
		"id":        info.ID,
		"tier":      info.Tier,
		"plan":      info.Plan,
		"balance":   info.Balance,
		"credits":   info.Credits,
		"expiresAt": info.ExpiresAt,
		"enabled":   info.Enabled,
		"requests":  info.Requests,
		"tokens":    info.Tokens,
		"models":    info.Models,
		"createdAt": info.CreatedAt,
		"lastUsed":  info.LastUsed,
		"note":      info.Note,
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
		totalCredits += log.Credits

		model := log.ActualModel
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
		ms["credits"] = ms["credits"].(float64) + log.Credits
	}

	writeJSON(w, 200, map[string]interface{}{
		"totalRequests":     count,
		"totalInputTokens":  totalInput,
		"totalOutputTokens": totalOutput,
		"totalCredits":      totalCredits,
		"models":            modelStats,
	})
}

// GET /user/api/logs - request logs for this key only
func (h *Handler) handleUserLogs(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	h.callLogsMu.RLock()
	defer h.callLogsMu.RUnlock()

	var logs []CallLog
	limit := 50 // default limit

	// Collect logs for this key (most recent first)
	for i := len(h.callLogs) - 1; i >= 0 && len(logs) < limit; i-- {
		log := h.callLogs[i]
		if log.ApiKeyID == info.ID {
			// Sanitize: don't expose account details to user
			log.Account = ""
			logs = append(logs, log)
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
	})
}

// POST /user/api/redeem - redeem activation code
func (h *Handler) handleUserRedeem(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
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
	expiresAtBefore := info.ExpiresAt

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

	// Find the code amount for receipt
	var amount float64
	codes := config.GetActivationCodes()
	for _, ac := range codes {
		if ac.Code == req.Code {
			amount = ac.Amount
			break
		}
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
	writeJSON(w, 200, pricing)
}

// writeJSON sends a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
