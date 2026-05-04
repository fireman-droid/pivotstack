package proxy

import (
	"crypto/sha1"
	"encoding/hex"
	"kiro-api-proxy/config"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LeaderEntry represents a single ranked row.
// IsFake/RealID/Note are admin-only fields and MUST be stripped before user-facing serialization.
type LeaderEntry struct {
	Rank   int     `json:"rank"`
	Alias  string  `json:"alias"`
	Value  float64 `json:"value"`
	IsYou  bool    `json:"isYou,omitempty"`
	IsFake bool    `json:"isFake,omitempty"`
	RealID string  `json:"realId,omitempty"` // admin only
	Note   string  `json:"note,omitempty"`   // admin only
}

type LeaderResp struct {
	Metric  string        `json:"metric"`
	Updated int64         `json:"updated"`
	Top     []LeaderEntry `json:"top"`
	You     *LeaderEntry  `json:"you,omitempty"`
	Total   int           `json:"total,omitempty"`
}

func validMetric(m string) string {
	switch m {
	case "requests", "credits", "tokens":
		return m
	}
	return ""
}

func metricValue(k *config.ApiKeyInfo, metric string) float64 {
	switch metric {
	case "requests":
		return float64(k.Requests)
	case "credits":
		return k.Credits
	case "tokens":
		return float64(k.Tokens)
	}
	return 0
}

// maskAlias produces a user-safe alias from note/id.
// note "玄武宗" id "abc...xyz" → "玄武宗***A1F"
func maskAlias(note, id string) string {
	prefix := strings.TrimSpace(note)
	if prefix != "" {
		runes := []rune(prefix)
		if len(runes) > 4 {
			prefix = string(runes[:4])
		}
	} else {
		prefix = "玄武"
	}
	sum := sha1.Sum([]byte(id))
	suffix := strings.ToUpper(hex.EncodeToString(sum[:])[:3])
	return prefix + "***" + suffix
}

// buildRealEntries returns sorted real entries (desc by metric).
func (h *Handler) buildRealEntries(metric string) []LeaderEntry {
	keys := config.GetAllApiKeys()
	h.apiKeyStatsMu.RLock()
	for i := range keys {
		if stats, ok := h.apiKeyStats[keys[i].ID]; ok {
			keys[i].LastUsed = stats.LastUsed
			keys[i].Requests = stats.Requests
			keys[i].Errors = stats.Errors
			keys[i].Tokens = stats.Tokens
			keys[i].Credits = stats.Credits
		}
	}
	h.apiKeyStatsMu.RUnlock()

	out := make([]LeaderEntry, 0, len(keys))
	for i := range keys {
		k := &keys[i]
		v := metricValue(k, metric)
		if v <= 0 {
			continue
		}
		out = append(out, LeaderEntry{
			Value:  v,
			Alias:  maskAlias(k.Note, k.ID),
			RealID: k.ID,
			Note:   k.Note,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Value > out[j].Value
	})
	return out
}

// apiAdminLeaderboard handles GET /admin/api/leaderboard?metric=requests|credits|tokens
// Admin sees full real ranking + injected fakes (with isFake/realId/note flags visible).
func (h *Handler) apiAdminLeaderboard(w http.ResponseWriter, r *http.Request) {
	metric := validMetric(r.URL.Query().Get("metric"))
	if metric == "" {
		metric = "requests"
	}

	real := h.buildRealEntries(metric)
	_, fakeN := config.GetLeaderboardConfig()

	combined := real
	if fakeN > 0 && len(real) > 0 {
		seed := time.Now().UTC().Format("2006-01-02") + "|" + metric
		fakes := generateFakes(metric, real[0].Value, fakeN, seed)
		combined = append(combined, fakes...)
		sort.SliceStable(combined, func(i, j int) bool {
			return combined[i].Value > combined[j].Value
		})
	}
	for i := range combined {
		combined[i].Rank = i + 1
	}
	writeJSON(w, 200, LeaderResp{
		Metric:  metric,
		Updated: time.Now().Unix(),
		Top:     combined,
		Total:   len(real),
	})
}

// handleUserLeaderboard handles GET /user/api/leaderboard?metric=...
// Anti-leak: strips IsFake/RealID/Note; You.Rank is computed against REAL keys only.
func (h *Handler) handleUserLeaderboard(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	enabled, fakeN := config.GetLeaderboardConfig()
	if !enabled {
		writeJSON(w, 404, map[string]string{"error": "leaderboard not available"})
		return
	}
	metric := validMetric(r.URL.Query().Get("metric"))
	if metric == "" {
		metric = "requests"
	}

	real := h.buildRealEntries(metric)

	var youReal *LeaderEntry
	for i := range real {
		if real[i].RealID == info.ID {
			youReal = &LeaderEntry{
				Rank:  i + 1,
				Alias: "你",
				Value: real[i].Value,
				IsYou: true,
			}
			break
		}
	}

	combined := append([]LeaderEntry(nil), real...)
	if fakeN > 0 && len(real) > 0 {
		seed := time.Now().UTC().Format("2006-01-02") + "|" + metric
		fakes := generateFakes(metric, real[0].Value, fakeN, seed)
		combined = append(combined, fakes...)
	}
	sort.SliceStable(combined, func(i, j int) bool {
		return combined[i].Value > combined[j].Value
	})

	const topLimit = 20
	if len(combined) > topLimit {
		combined = combined[:topLimit]
	}

	out := make([]LeaderEntry, len(combined))
	for i, e := range combined {
		entry := LeaderEntry{
			Rank:  i + 1,
			Alias: e.Alias,
			Value: e.Value,
		}
		if e.RealID != "" && e.RealID == info.ID {
			entry.Alias = "你"
			entry.IsYou = true
		}
		out[i] = entry
	}

	writeJSON(w, 200, LeaderResp{
		Metric:  metric,
		Updated: time.Now().Unix(),
		Top:     out,
		You:     youReal,
		Total:   len(real),
	})
}
