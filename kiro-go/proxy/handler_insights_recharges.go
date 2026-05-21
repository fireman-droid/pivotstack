package proxy

import (
	"net/http"
	"strconv"
)

// GET /admin/api/apikeys/{id}/recharges
func (h *Handler) apiGetApiKeyRecharges(w http.ResponseWriter, r *http.Request, keyID string) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	records, total := readRechargeRecords(keyID, page, limit)
	writeJSON(w, 200, map[string]interface{}{
		"records": records,
		"total":   total,
	})
}
