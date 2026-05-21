package proxy

import (
	"net/http"
	"strconv"
	"strings"

	"kiro-api-proxy/config"
	"kiro-api-proxy/notif"
)

// handleUserNotifications dispatches /user/api/notifications[/...].
//
//	GET    /user/api/notifications              → list + unreadCount
//	POST   /user/api/notifications/read-all     → 一键全部已读
//	POST   /user/api/notifications/:id/read     → 单条已读
//	POST   /user/api/notifications/:id/dismiss  → 单条隐藏（要 dismissible）
func (h *Handler) handleUserNotifications(w http.ResponseWriter, r *http.Request, key *config.ApiKeyInfo) {
	path := strings.TrimPrefix(r.URL.Path, "/user/api/notifications")

	if path == "" || path == "/" {
		if r.Method != http.MethodGet {
			writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 10
		}
		writeJSON(w, http.StatusOK, notif.ListForUser(*key, limit, false))
		return
	}

	if path == "/read-all" && r.Method == http.MethodPost {
		n, err := notif.MarkAllUserRead(*key)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "marked": n})
		return
	}

	rest := strings.TrimPrefix(path, "/")
	switch {
	case strings.HasSuffix(rest, "/read") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(rest, "/read")
		ts, err := notif.MarkUserRead(id, *key, false)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "readAt": ts})
	case strings.HasSuffix(rest, "/dismiss") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(rest, "/dismiss")
		ts, err := notif.MarkUserRead(id, *key, true)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "dismissedAt": ts})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}
