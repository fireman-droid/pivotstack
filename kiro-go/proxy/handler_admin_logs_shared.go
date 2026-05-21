package proxy

import (
	"encoding/json"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (h *Handler) apiGetLogs(w http.ResponseWriter, r *http.Request) {
	h.callLogsMu.RLock()
	logs := make([]CallLog, len(h.callLogs))
	copy(logs, h.callLogs)
	h.callLogsMu.RUnlock()

	// get query params
	statusFilter := r.URL.Query().Get("status")
	keyFilter := r.URL.Query().Get("key")
	searchQuery := strings.ToLower(r.URL.Query().Get("search"))

	// filter logs
	var filteredLogs []CallLog
	for _, l := range logs {
		isErr := l.Error != "" || l.Status == "error"
		if statusFilter == "error" && !isErr {
			continue
		}
		if statusFilter == "success" && isErr {
			continue
		}
		if keyFilter != "" && keyFilter != "all" && l.ApiKeyID != keyFilter {
			continue
		}

		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(l.ActualModel), searchQuery) &&
				!strings.Contains(strings.ToLower(l.OriginalModel), searchQuery) &&
				!strings.Contains(strings.ToLower(l.Account), searchQuery) &&
				!strings.Contains(strings.ToLower(l.Error), searchQuery) &&
				!strings.Contains(strings.ToLower(l.RequestID), searchQuery) &&
				!strings.Contains(strings.ToLower(l.StopReason), searchQuery) {
				continue
			}
		}
		filteredLogs = append(filteredLogs, l)
	}

	// reverse so newest first
	for i, j := 0, len(filteredLogs)-1; i < j; i, j = i+1, j-1 {
		filteredLogs[i], filteredLogs[j] = filteredLogs[j], filteredLogs[i]
	}

	total := len(filteredLogs)

	// pagination: ?page=1&limit=50
	page := 1
	limit := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  filteredLogs[start:end],
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) apiClearLogs(w http.ResponseWriter, _ *http.Request) {
	h.callLogsMu.Lock()
	h.callLogs = nil
	h.callLogsMu.Unlock()
	// Also truncate the on-disk log file
	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	os.Truncate(logPath, 0)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
