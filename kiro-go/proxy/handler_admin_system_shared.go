package proxy

import (
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"sync/atomic"
	"time"
)

func (h *Handler) apiGetStatus(w http.ResponseWriter, _ *http.Request) {
	proPool := h.pool.TierStats("pro")
	freePool := h.pool.TierStats("free")
	proRemaining := (proPool.UsageLimit - proPool.UsageCurrent) + (proPool.TrialLimit - proPool.TrialCurrent)
	freeRemaining := (freePool.UsageLimit - freePool.UsageCurrent) + (freePool.TrialLimit - freePool.TrialCurrent)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accounts": h.pool.Count(), "available": h.pool.AvailableCount(),
		"totalRequests": h.totalRequests, "successRequests": h.successRequests,
		"failedRequests": h.failedRequests, "totalTokens": h.totalTokens,
		"totalCredits": h.totalCredits, "uptime": time.Now().Unix() - h.startTime,
		"freePool":       freePool,
		"proPool":        proPool,
		"prediction":     h.creditPredictor.Predict(proRemaining + freeRemaining),
		"proPrediction":  h.proCreditPredictor.Predict(proRemaining),
		"freePrediction": h.freeCreditPredictor.Predict(freeRemaining),
	})
}

func (h *Handler) apiGetSettings(w http.ResponseWriter, _ *http.Request) {
	runtimeHost, runtimePort := config.GetRuntimeHTTPAddress()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKey": config.GetApiKey(), "requireApiKey": config.IsApiKeyRequired(),
		"port": config.GetPort(), "host": config.GetHost(),
		"runtimeHost": runtimeHost, "runtimePort": runtimePort,
		"profitIncludeGift": config.GetProfitIncludeGift(),
	})
}

func (h *Handler) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ApiKey        string  `json:"apiKey"`
		RequireApiKey bool    `json:"requireApiKey"`
		Password      *string `json:"password,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	// 改密走专用 endpoint，settings 不再接受 password 字段（防 admin 通过 /settings 改密码绕过 旧密码校验）
	if req.Password != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "use POST /admin/api/password to change admin password"})
		return
	}
	if err := config.UpdateSettings(req.ApiKey, req.RequireApiKey); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetAdminStats(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalRequests": atomic.LoadInt64(&h.totalRequests), "successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests": atomic.LoadInt64(&h.failedRequests), "totalTokens": atomic.LoadInt64(&h.totalTokens),
		"totalCredits": h.getCredits(), "uptime": time.Now().Unix() - h.startTime,
	})
}

func (h *Handler) apiResetStats(w http.ResponseWriter, _ *http.Request) {
	atomic.StoreInt64(&h.totalRequests, 0)
	atomic.StoreInt64(&h.successRequests, 0)
	atomic.StoreInt64(&h.failedRequests, 0)
	atomic.StoreInt64(&h.totalTokens, 0)
	h.creditsMu.Lock()
	h.totalCredits = 0
	h.creditsMu.Unlock()
	config.UpdateStats(0, 0, 0, 0, 0)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGenerateMachineId(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"machineId": config.GenerateMachineId()})
}

func (h *Handler) apiGetThinkingConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := config.GetThinkingConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suffix": cfg.Suffix, "openaiFormat": cfg.OpenAIFormat, "claudeFormat": cfg.ClaudeFormat,
	})
}

func (h *Handler) apiUpdateThinkingConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Suffix       string `json:"suffix"`
		OpenAIFormat string `json:"openaiFormat"`
		ClaudeFormat string `json:"claudeFormat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	validFormats := map[string]bool{"reasoning_content": true, "thinking": true, "think": true}
	if req.OpenAIFormat != "" && !validFormats[req.OpenAIFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid openaiFormat"})
		return
	}
	if req.ClaudeFormat != "" && !validFormats[req.ClaudeFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claudeFormat"})
		return
	}
	if err := config.UpdateThinkingConfig(req.Suffix, req.OpenAIFormat, req.ClaudeFormat); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetEndpointConfig(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"preferredEndpoint": config.GetPreferredEndpoint()})
}

func (h *Handler) apiUpdateEndpointConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PreferredEndpoint string `json:"preferredEndpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	valid := map[string]bool{"auto": true, "codewhisperer": true, "amazonq": true}
	if !valid[req.PreferredEndpoint] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid endpoint"})
		return
	}
	if err := config.UpdatePreferredEndpoint(req.PreferredEndpoint); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetConcurrency(w http.ResponseWriter, _ *http.Request) {
	perKey, perFree, perPro := config.GetConcurrencyConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maxConcurrentPerKey":       perKey,
		"maxInFlightPerAccountFree": perFree,
		"maxInFlightPerAccountPro":  perPro,
		"timedKeyRPM":               config.GetTimedKeyRPM(), // 0 = 走老兜底 200/min
	})
}

func (h *Handler) apiUpdateConcurrency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxConcurrentPerKey       int  `json:"maxConcurrentPerKey"`
		MaxInFlightPerAccount     int  `json:"maxInFlightPerAccount"`     // Legacy: sets both free and pro
		MaxInFlightPerAccountFree int  `json:"maxInFlightPerAccountFree"` // Per FREE account
		MaxInFlightPerAccountPro  int  `json:"maxInFlightPerAccountPro"`  // Per PRO account
		TimedKeyRPM               *int `json:"timedKeyRPM,omitempty"`     // 天卡 key 全局 RPM；nil=不改，0=禁用走老兜底，>0=新阈值
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.MaxConcurrentPerKey < 1 || req.MaxConcurrentPerKey > 200 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxConcurrentPerKey must be 1-200"})
		return
	}
	perFree := req.MaxInFlightPerAccountFree
	perPro := req.MaxInFlightPerAccountPro
	if req.MaxInFlightPerAccount > 0 {
		if perFree == 0 {
			perFree = req.MaxInFlightPerAccount
		}
		if perPro == 0 {
			perPro = req.MaxInFlightPerAccount
		}
	}
	if perFree > 0 && (perFree < 1 || perFree > 500) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxInFlightPerAccountFree must be 1-500"})
		return
	}
	if perPro > 0 && (perPro < 1 || perPro > 500) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "maxInFlightPerAccountPro must be 1-500"})
		return
	}
	if err := config.UpdateConcurrencyConfig(req.MaxConcurrentPerKey, perFree, perPro); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if req.TimedKeyRPM != nil {
		v := *req.TimedKeyRPM
		if v < 0 || v > 10000 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "timedKeyRPM must be 0-10000"})
			return
		}
		if err := config.UpdateTimedKeyRPM(v); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetVersion(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"version": config.Version})
}
