package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"kiro-api-proxy/config"
)

const userPreferencesBodyLimit = 64 << 10

// userPreferencesResponse v6 升级：
//   - ChannelPreferences (主路径): groupID → runtime channel id
//   - SeriesPreferences (deprecated): seriesID → channelID — 兼容 v5 老 ApiKey
//   - AvailableGroups (v6): user 视角的分组列表 + 每组下可挑的 channels
//   - AvailableSeries (legacy): 兼容已有前端
type userPreferencesResponse struct {
	ChannelPreferences map[string]string      `json:"channelPreferences"`
	SeriesPreferences  map[string]string      `json:"seriesPreferences,omitempty"`
	AvailableGroups    []userPreferenceGroup  `json:"availableGroups"`
	AvailableSeries    []userPreferenceSeries `json:"availableSeries"`
}

type userPreferenceGroup struct {
	ID                      string                  `json:"id"`
	Name                    string                  `json:"name"`
	Description             string                  `json:"description,omitempty"`
	DefaultRuntimeChannelID string                  `json:"defaultRuntimeChannelId,omitempty"`
	Channels                []userPreferenceChannel `json:"channels"`
}

type userPreferenceSeries struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	DefaultChannelID string                  `json:"defaultChannelId,omitempty"`
	Channels         []userPreferenceChannel `json:"channels"`
}

type userPreferenceChannel struct {
	ID      string `json:"id"`       // 注：v6 group 里这是 runtime channel id（direct: 前缀；NewAPI 原 id）
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
	Billing string `json:"billing,omitempty"` // v6: 给 user 看的卖价摘要
}

func (h *Handler) handleUserPreferences(w http.ResponseWriter, info *config.ApiKeyInfo) {
	writeJSON(w, http.StatusOK, buildUserPreferencesResponse(info))
}

// buildUserPreferencesResponse 决定 ChannelPreferences 字段填什么：
//   - v6（ChannelGroups 非空）：填 info.ChannelPreferences（真正的 group→runtime channel id 偏好）
//   - v5 兼容：填 info.SeriesPreferences（旧前端用 channelPreferences 字段当 series 偏好用）
//
// 同时把另一侧也带回去（SeriesPreferences 字段总是返回 info.SeriesPreferences 让 v5 路径仍可用）。
func buildUserPreferencesResponse(info *config.ApiKeyInfo) userPreferencesResponse {
	resp := userPreferencesResponse{
		SeriesPreferences: copyUserPreferenceMap(info.SeriesPreferences),
		AvailableGroups:   buildAvailablePreferenceGroups(),
		AvailableSeries:   buildAvailablePreferenceSeries(),
	}
	if len(resp.AvailableGroups) > 0 {
		resp.ChannelPreferences = copyUserPreferenceMap(info.ChannelPreferences)
	} else {
		resp.ChannelPreferences = copyUserPreferenceMap(info.SeriesPreferences)
	}
	return resp
}

func (h *Handler) handleUserUpdatePreferences(w http.ResponseWriter, r *http.Request, info *config.ApiKeyInfo) {
	r.Body = http.MaxBytesReader(w, r.Body, userPreferencesBodyLimit)
	var req struct {
		ChannelPreferences map[string]string `json:"channelPreferences"`
		SeriesPreferences  map[string]string `json:"seriesPreferences"` // legacy 兼容（前端旧版用）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// v6：ChannelGroups 非空时 channelPreferences 走新逻辑（groupID → runtime channel id）。
	// 老前端传 channelPreferences 但 ChannelGroups 为空（项目还在 v5 模式）→ 当 series 偏好处理。
	groups := config.GetActiveChannelGroups()
	useV6 := len(groups) > 0
	if useV6 {
		next := make(map[string]string, len(req.ChannelPreferences))
		for groupID, runtimeID := range req.ChannelPreferences {
			groupID = strings.TrimSpace(groupID)
			runtimeID = strings.TrimSpace(runtimeID)
			if groupID == "" || runtimeID == "" {
				continue
			}
			next[groupID] = runtimeID
		}
		if err := config.SetApiKeyChannelPreferences(info.ID, next); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	} else {
		// v5 兼容路径：channelPreferences 当 seriesPreferences 用（旧前端语义）
		src := req.ChannelPreferences
		if len(src) == 0 {
			src = req.SeriesPreferences
		}
		next := make(map[string]string, len(src))
		for seriesID, channelID := range src {
			seriesID = strings.TrimSpace(seriesID)
			channelID = strings.TrimSpace(channelID)
			if seriesID == "" || channelID == "" {
				continue
			}
			next[seriesID] = channelID
		}
		if err := config.SetApiKeySeriesPreferences(info.ID, next); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	updated := config.FindApiKeyByID(info.ID)
	if updated == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reload preferences"})
		return
	}
	writeJSON(w, http.StatusOK, buildUserPreferencesResponse(updated))
}

// buildAvailablePreferenceGroups v6：user 视角的分组 + 每组下可挑的 channels（含计费摘要）。
func buildAvailablePreferenceGroups() []userPreferenceGroup {
	groups := config.GetActiveChannelGroups()
	newAPIByID := indexNewAPIChannels(config.GetNewAPIChannels())
	directByID := indexDirectChannels(config.GetDirectChannels())
	out := make([]userPreferenceGroup, 0, len(groups))
	for _, g := range groups {
		channels := make([]userPreferenceChannel, 0, len(g.Channels))
		for _, ref := range g.Channels {
			rid := config.RuntimeChannelIDFor(ref)
			entry := userPreferenceChannel{ID: rid}
			switch strings.ToLower(strings.TrimSpace(ref.SourceType)) {
			case "newapi":
				if ch, ok := newAPIByID[ref.ChannelID]; ok && ch.DeletedAt == 0 && ch.Enabled {
					entry.Alias = maskedChannelAlias(ch)
					entry.Enabled = true
					entry.Billing = fmt.Sprintf("×%.2f markup", ch.Markup)
				} else {
					continue // 跳过已删/已禁用的 channel
				}
			case "direct":
				if ch, ok := directByID[ref.ChannelID]; ok && ch.DeletedAt == 0 && ch.Enabled {
					entry.Alias = ch.Alias
					entry.Enabled = true
					entry.Billing = directGroupBilling(ch.SellPrice)
				} else {
					continue
				}
			default:
				continue
			}
			channels = append(channels, entry)
		}
		sort.Slice(channels, func(i, j int) bool { return channels[i].Alias < channels[j].Alias })
		out = append(out, userPreferenceGroup{
			ID:                      g.ID,
			Name:                    g.Name,
			Description:             g.Description,
			DefaultRuntimeChannelID: g.DefaultRuntimeChannelID,
			Channels:                channels,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		gi := groups[i]
		gj := groups[j]
		if gi.SortOrder != gj.SortOrder {
			return gi.SortOrder < gj.SortOrder
		}
		return gi.Name < gj.Name
	})
	return out
}

// buildAvailablePreferenceSeries（v5 legacy）保留兼容旧前端。
func buildAvailablePreferenceSeries() []userPreferenceSeries {
	series := config.GetSeries()
	channels := config.GetNewAPIChannels()
	bySeries := make(map[string][]userPreferenceChannel)
	for _, ch := range channels {
		if ch.SeriesID == "" || !ch.Enabled || ch.DeletedAt > 0 {
			continue
		}
		bySeries[ch.SeriesID] = append(bySeries[ch.SeriesID], userPreferenceChannel{
			ID:      ch.ID,
			Alias:   maskedChannelAlias(ch),
			Enabled: ch.Enabled,
		})
	}

	out := make([]userPreferenceSeries, 0, len(series))
	for _, s := range series {
		ch := bySeries[s.ID]
		sort.Slice(ch, func(i, j int) bool {
			if ch[i].Alias != ch[j].Alias {
				return ch[i].Alias < ch[j].Alias
			}
			return ch[i].ID < ch[j].ID
		})
		out = append(out, userPreferenceSeries{
			ID:               s.ID,
			Name:             s.Name,
			DefaultChannelID: s.DefaultChannelID,
			Channels:         ch,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func maskedChannelAlias(ch config.NewAPIChannel) string {
	if alias := strings.TrimSpace(ch.Alias); alias != "" {
		return alias
	}
	if ch.UpstreamTokenID > 0 {
		return fmt.Sprintf("Channel #%d", ch.UpstreamTokenID)
	}
	return "Channel"
}

func copyUserPreferenceMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}
