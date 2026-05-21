package proxy

import (
	"encoding/json"
	"net/http"
	"strings"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// handleUserAPI routes /user/api/* requests.
func (h *Handler) handleUserAPI(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 未鉴权旁路：登录 + 注册（密码 = email/password）
	switch path {
	case "/user/api/login":
		h.handleUserPasswordLogin(w, r)
		return
	case "/user/api/register":
		h.handleUserRegister(w, r)
		return
	}

	// Authenticate using API key as Bearer token
	keyInfo := h.resolveUserKey(r)
	if keyInfo == nil {
		writeJSON(w, 401, map[string]string{"error": "invalid or missing api key"})
		return
	}

	// reseller 子路由优先：/user/api/reseller/* 全部走 reseller handler
	if strings.HasPrefix(path, "/user/api/reseller/") {
		h.handleResellerAPI(w, r)
		return
	}
	// notifications 子路由：/user/api/notifications[/...] 全部走 notif handler
	if path == "/user/api/notifications" || strings.HasPrefix(path, "/user/api/notifications/") {
		h.handleUserNotifications(w, r, keyInfo)
		return
	}
	switch {
	case path == "/user/api/me" && r.Method == "GET":
		h.handleUserMe(w, keyInfo)
	case path == "/user/api/bind-account" && r.Method == "POST":
		h.handleUserBindAccount(w, r, keyInfo)
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
	case path == "/user/api/preferences" && r.Method == "GET":
		h.handleUserPreferences(w, keyInfo)
	case path == "/user/api/preferences" && r.Method == "PUT":
		h.handleUserUpdatePreferences(w, r, keyInfo)
	case path == "/user/api/leaderboard" && r.Method == "GET":
		h.handleUserLeaderboard(w, r, keyInfo)
	// v7: user-side ApiKey CRUD（ownership 严格校验）
	case path == "/user/api/channel-options" && r.Method == "GET":
		h.handleUserChannelOptions(w, r, keyInfo)
	case path == "/user/api/keys" && r.Method == "GET":
		h.handleUserListKeys(w, keyInfo)
	case path == "/user/api/keys" && r.Method == "POST":
		h.handleUserCreateKey(w, r, keyInfo)
	case strings.HasPrefix(path, "/user/api/keys/") && r.Method == "PATCH":
		h.handleUserPatchKey(w, r, keyInfo, strings.TrimPrefix(path, "/user/api/keys/"))
	case strings.HasPrefix(path, "/user/api/keys/") && r.Method == "DELETE":
		h.handleUserDeleteKey(w, r, keyInfo, strings.TrimPrefix(path, "/user/api/keys/"))
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

// resolveUserKey extracts API key from Bearer token and returns ApiKeyInfo.
// v8: also rejects bound users that have been Disabled by admin.
// Returned ApiKeyInfo has wallet fields overlayed from user wallet (for bound non-child keys).
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
	info := config.FindApiKey(key)
	if info == nil || !info.Enabled {
		return nil
	}
	if u, ok := users.Default().FindByApiKeyID(info.ID); ok && u.Disabled {
		return nil
	}
	return users.OverlayWalletOnKey(info)
}

// writeJSON sends a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
