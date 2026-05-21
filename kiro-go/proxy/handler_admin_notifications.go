package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"kiro-api-proxy/notif"
)

// ───────────── Routing entry ─────────────

// routeAdminNotifications 处理 /notifications 域。返回 true 表示本域吃了请求。
func (h *Handler) routeAdminNotifications(path string, w http.ResponseWriter, r *http.Request) bool {
	if path == "/notifications" {
		switch r.Method {
		case http.MethodGet:
			h.apiListNotifications(w, r)
		case http.MethodPost:
			h.apiCreateNotification(w, r)
		default:
			writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return true
	}
	if strings.HasPrefix(path, "/notifications/") {
		rest := strings.TrimPrefix(path, "/notifications/")
		if rest == "" {
			return false
		}
		// /notifications/:id/stats
		if strings.HasSuffix(rest, "/stats") && r.Method == http.MethodGet {
			id := strings.TrimSuffix(rest, "/stats")
			h.apiNotificationStats(w, r, id)
			return true
		}
		// /notifications/:id
		switch r.Method {
		case http.MethodGet:
			h.apiGetNotification(w, r, rest)
		case http.MethodPut:
			h.apiUpdateNotification(w, r, rest)
		case http.MethodDelete:
			h.apiDeleteNotification(w, r, rest)
		default:
			writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return true
	}
	return false
}

// ───────────── Handlers ─────────────

// GET /admin/api/notifications?status=&limit=&offset=
func (h *Handler) apiListNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	res := notif.ListForAdmin(q.Get("status"), limit, offset)
	writeJSON(w, http.StatusOK, res)
}

// GET /admin/api/notifications/:id
func (h *Handler) apiGetNotification(w http.ResponseWriter, r *http.Request, id string) {
	n, ok := notif.Default().GetNotification(id)
	if !ok || n.DeletedAt != 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// POST /admin/api/notifications
func (h *Handler) apiCreateNotification(w http.ResponseWriter, r *http.Request) {
	var in notif.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	actor := adminAuditActor(r)
	n, err := notif.Create(in, actor)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("notification_create", actor, fmt.Sprintf("id=%s status=%s level=%s target=%s",
		n.ID, n.Status, n.Level, n.TargetType))
	writeJSON(w, http.StatusCreated, n)
}

// PUT /admin/api/notifications/:id
func (h *Handler) apiUpdateNotification(w http.ResponseWriter, r *http.Request, id string) {
	var in notif.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	actor := adminAuditActor(r)
	n, err := notif.Update(id, in, actor)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("notification_update", actor, fmt.Sprintf("id=%s status=%s", n.ID, n.Status))
	writeJSON(w, http.StatusOK, n)
}

// DELETE /admin/api/notifications/:id
func (h *Handler) apiDeleteNotification(w http.ResponseWriter, r *http.Request, id string) {
	actor := adminAuditActor(r)
	if err := notif.Delete(id, actor); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("notification_delete", actor, fmt.Sprintf("id=%s", id))
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GET /admin/api/notifications/:id/stats
func (h *Handler) apiNotificationStats(w http.ResponseWriter, r *http.Request, id string) {
	st, err := notif.GetStats(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}
