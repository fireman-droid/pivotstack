package proxy

import (
	"encoding/json"
	"errors"
	"kiro-api-proxy/config"
	"net/http"
	"strconv"
)

// ==================== 新版鉴权 endpoint ====================

// POST /admin/api/login
// Body: { "password": "..." }
// 200: { success, csrfToken, expiresAt }  + Set-Cookie admin_session
// 401: { error, remainingAttempts }
// 423: { error, locked, retryAfter }
func (h *Handler) apiAdminLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	ip := clientIP(r)
	if locked, retryAfter := h.adminSessions.limiter.IsLocked(ip); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		writeJSONStatus(w, http.StatusLocked, map[string]interface{}{
			"error":      "too many login failures, try later",
			"locked":     true,
			"retryAfter": int(retryAfter.Seconds()),
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if !config.VerifyAdminPassword(req.Password) {
		locked, retryAfter := h.adminSessions.limiter.RecordFailure(ip)
		if locked {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSONStatus(w, http.StatusLocked, map[string]interface{}{
				"error":      "too many login failures, try later",
				"locked":     true,
				"retryAfter": int(retryAfter.Seconds()),
			})
			return
		}
		writeJSONStatus(w, http.StatusUnauthorized, map[string]interface{}{
			"error":             "invalid password",
			"remainingAttempts": h.adminSessions.limiter.RemainingAttempts(ip),
		})
		return
	}

	h.adminSessions.limiter.RecordSuccess(ip)
	sess, err := h.adminSessions.Create(w, r)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"csrfToken": sess.CSRFToken,
		"expiresAt": sess.ExpiresAt.Unix(),
	})
}

// GET /admin/api/session - SPA 刷新页面时拿 csrfToken
func (h *Handler) apiAdminSession(w http.ResponseWriter, _ *http.Request, sess *adminSession) {
	writeJSONStatus(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"csrfToken": sess.CSRFToken,
		"expiresAt": sess.ExpiresAt.Unix(),
	})
}

// POST /admin/api/logout
func (h *Handler) apiAdminLogout(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	h.adminSessions.Invalidate(sess.TokenHash)
	h.adminSessions.ClearCookie(w, r)
	writeJSONStatus(w, http.StatusOK, map[string]bool{"success": true})
}

// POST /admin/api/password
// Body: { "oldPassword": "...", "newPassword": "...", "confirmPassword": "..." }
// 成功后所有 session 失效（踢出所有设备），客户端要重新登录
func (h *Handler) apiChangeAdminPassword(w http.ResponseWriter, r *http.Request, _ *adminSession) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	if config.IsPasswordEnvOverride() {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "password managed by ADMIN_PASSWORD env"})
		return
	}

	var req struct {
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "passwords do not match"})
		return
	}
	if err := config.ValidateAdminPasswordStrength(req.NewPassword); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := config.ChangeAdminPassword(req.OldPassword, req.NewPassword); err != nil {
		// 旧密码错 → 401；hash/写盘失败 → 500（错误分类便于排障 + 前端正确提示）
		if errors.Is(err, config.ErrInvalidOldPassword) {
			writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		} else {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}

	h.adminSessions.InvalidateAll()
	h.adminSessions.ClearCookie(w, r)
	writeJSONStatus(w, http.StatusOK, map[string]bool{"success": true})
}

// POST /admin/api/sse/token
// Body: { "stream": "stats" | "logs" }
// 返回一次性 token（5min TTL），客户端拼到 EventSource URL：?sse_token=...
func (h *Handler) apiCreateSSEToken(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var req struct {
		Stream string `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.Stream != "stats" && req.Stream != "logs" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "stream must be stats or logs"})
		return
	}
	token, err := h.adminSessions.NewSSEToken(sess.TokenHash, req.Stream, adminSSETokenTTL)
	if err != nil {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]string{"token": token})
}
