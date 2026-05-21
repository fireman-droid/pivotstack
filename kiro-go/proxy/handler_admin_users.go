package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kiro-api-proxy/users"
)

// Admin 用户体系管理：用户列表 + 注册策略开关。
//
// 路由前缀（已在 routeAdminUsers 注册）：
//   GET    /admin/api/users
//   GET    /admin/api/users/policy
//   PUT    /admin/api/users/policy
//   POST   /admin/api/users/:id/disable
//   POST   /admin/api/users/:id/enable

func (h *Handler) routeAdminUsers(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/users" && r.Method == http.MethodGet:
		h.apiListUsers(w, r)
	case path == "/users/policy" && r.Method == http.MethodGet:
		h.apiGetUserPolicy(w, r)
	case path == "/users/policy" && r.Method == http.MethodPut:
		h.apiSetUserPolicy(w, r)
	case strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/disable") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/disable")
		h.apiToggleUser(w, r, id, true)
	case strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/enable") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/enable")
		h.apiToggleUser(w, r, id, false)
	default:
		return false
	}
	return true
}

// ───────────── users ─────────────

func (h *Handler) apiListUsers(w http.ResponseWriter, r *http.Request) {
	list := users.Default().ListUsers()
	// 屏蔽 passwordHash 防 leak
	out := make([]map[string]any, 0, len(list))
	for _, u := range list {
		out = append(out, map[string]any{
			"id":            u.ID,
			"email":         u.Email,
			"username":      u.Username,
			"apiKeyIds":     u.ApiKeyIDs,
			"defaultKeyId":  u.DefaultKeyID,
			"invitedBy":     u.InvitedBy,
			"inviterUserId": u.InviterUserID,
			"createdAt":     u.CreatedAt,
			"lastLoginAt":   u.LastLoginAt,
			"disabled":      u.Disabled,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "total": len(out)})
}

func (h *Handler) apiToggleUser(w http.ResponseWriter, r *http.Request, id string, disabled bool) {
	if err := users.Default().UpdateUser(id, func(u *users.User) { u.Disabled = disabled }); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	action := "user_enable"
	if disabled {
		action = "user_disable"
	}
	AuditLog(action, adminAuditActor(r), fmt.Sprintf("id=%s", id))
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ───────────── policy ─────────────

func (h *Handler) apiGetUserPolicy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"allowSelfRegister":     users.AllowSelfRegister,
		"requireActivationCode": users.RequireActivationCode,
	})
}

func (h *Handler) apiSetUserPolicy(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AllowSelfRegister     *bool `json:"allowSelfRegister"`
		RequireActivationCode *bool `json:"requireActivationCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if in.AllowSelfRegister != nil {
		users.AllowSelfRegister = *in.AllowSelfRegister
	}
	if in.RequireActivationCode != nil {
		users.RequireActivationCode = *in.RequireActivationCode
	}
	AuditLog("user_policy_update", adminAuditActor(r),
		fmt.Sprintf("allow=%v activation=%v", users.AllowSelfRegister, users.RequireActivationCode))
	writeJSON(w, http.StatusOK, map[string]bool{
		"allowSelfRegister":     users.AllowSelfRegister,
		"requireActivationCode": users.RequireActivationCode,
	})
}
