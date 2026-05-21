package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

func writeUserAuthLocked(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeJSONStatus(w, http.StatusLocked, map[string]interface{}{
		"error":      "too many login failures, try later",
		"locked":     true,
		"retryAfter": retryAfter,
	})
}

// User auth：用户名/密码登录与注册。
//
// 这两个 endpoint 是**未鉴权**的，跟其他 /user/api/* 不同（其他都要 Bearer）。
// 入口在 handler_serve.go 里专门旁路（不经 resolveUserKey）。

// POST /user/api/login  {email, password}
func (h *Handler) handleUserPasswordLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ip := requestIP(r)
	if locked, retryAfter := userLoginLimiter.IsLocked(ip); locked {
		writeUserAuthLocked(w, int(retryAfter.Seconds()))
		return
	}
	var in struct {
		Email    string `json:"email"`
		Username string `json:"username"` // 兼容前端把 email 填到 username 字段
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	email := in.Email
	if email == "" {
		email = in.Username
	}
	if email == "" || in.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	u, key, err := users.LoginByPassword(email, in.Password)
	if err != nil {
		if locked, retryAfter := userLoginLimiter.RecordFailure(ip); locked {
			writeUserAuthLocked(w, int(retryAfter.Seconds()))
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	userLoginLimiter.RecordSuccess(ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"apiKey":  key.Key,
		"user": map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"username": u.Username,
		},
	})
}

// POST /user/api/register  {email, password, activationCode?}
func (h *Handler) handleUserRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in users.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	u, key, err := users.Register(in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, users.ErrEmailAlreadyRegistered) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"apiKey":  key.Key,
		"user": map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"username": u.Username,
		},
	})
}

// POST /user/api/bind-account
// Headers: Bearer <api-key>
// Body: { email, password }
// Response: 200 / 400 / 409
func (h *Handler) handleUserBindAccount(w http.ResponseWriter, r *http.Request, key *config.ApiKeyInfo) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if key == nil || key.ID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing api key"})
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username,omitempty"` // v7: 可选 override，空则从 key.Note 派生
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if in.Email == "" || in.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}
	u, err := users.BindKeyToNewUser(key.ID, in.Email, in.Password, in.Username)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, users.ErrKeyAlreadyBound) || errors.Is(err, users.ErrEmailAlreadyRegistered) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("user_bind_account", u.Email, fmt.Sprintf("keyId=%s userId=%s", key.ID, u.ID))
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "userId": u.ID, "email": u.Email})
}
