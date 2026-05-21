package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kiro-api-proxy/config"
)

// === v6 ChannelGroup CRUD endpoints ===
//
// 路由：
//   GET    /groups            → 列出所有 ChannelGroup（含成员明细）
//   POST   /groups            → 新建 group
//   GET    /groups/channels   → 候选 channel 池（admin 在 GroupDrawer 里挑成员用，在 _view.go 实现）
//   GET    /groups/:id        → 单条 + 成员明细
//   PATCH  /groups/:id        → 改元数据
//   PUT    /groups/:id/channels → 替换整个成员列表 + 默认 runtime channel
//   DELETE /groups/:id        → 软删 + 清掉所有 ApiKey 上指向该 group 的偏好

const channelGroupBodyLimit = 64 << 10

// groupView 是 admin 前端展示用的 ChannelGroup 视图（成员展开为 channel 明细）。
type groupView struct {
	ID                      string              `json:"id"`
	Name                    string              `json:"name"`
	Description             string              `json:"description,omitempty"`
	Enabled                 bool                `json:"enabled"`
	ModelPatterns           []string            `json:"modelPatterns,omitempty"`
	DefaultRuntimeChannelID string              `json:"defaultRuntimeChannelId,omitempty"`
	SortOrder               int                 `json:"sortOrder,omitempty"`
	CreatedAt               int64               `json:"createdAt,omitempty"`
	UpdatedAt               int64               `json:"updatedAt,omitempty"`
	Channels                []groupChannelEntry `json:"channels"`
	ChannelCount            int                 `json:"channelCount"`
	EnabledChannelCount     int                 `json:"enabledChannelCount"`
}

// groupChannelEntry 是 group 内每条成员渠道的明细，用于前端在分组详情页直接展示。
type groupChannelEntry struct {
	RuntimeID    string `json:"runtimeId"`             // newapi 原 id 或 "direct:<id>"
	SourceType   string `json:"sourceType"`            // "newapi" | "direct"
	ChannelID    string `json:"channelId"`             // 配置层 id（不含 direct: 前缀）
	Alias        string `json:"alias"`
	SourceDetail string `json:"sourceDetail,omitempty"`
	Billing      string `json:"billing,omitempty"`
	Enabled      bool   `json:"enabled"`
	Missing      bool   `json:"missing,omitempty"` // 引用的 channel 已被删
}

// groupCreateRequest / groupUpdateRequest 用于 POST / PATCH。
type groupCreateRequest struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Enabled       bool     `json:"enabled"`
	ModelPatterns []string `json:"modelPatterns"`
	SortOrder     int      `json:"sortOrder"`
}

type groupUpdateRequest struct {
	Name          *string   `json:"name"`
	Description   *string   `json:"description"`
	Enabled       *bool     `json:"enabled"`
	ModelPatterns *[]string `json:"modelPatterns"`
	SortOrder     *int      `json:"sortOrder"`
}

type groupMembersRequest struct {
	Channels         []config.ChannelGroupChannelRef `json:"channels"`
	DefaultChannelID string                          `json:"defaultRuntimeChannelId"`
}

// === 路由 ===

func (h *Handler) routeAdminGroups(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/groups" && r.Method == http.MethodGet:
		h.apiListChannelGroups(w, r)
		return true
	case path == "/groups" && r.Method == http.MethodPost:
		h.apiCreateChannelGroup(w, r)
		return true
	case path == "/groups/channels" && r.Method == http.MethodGet:
		h.apiListGroupCandidateChannels(w, r)
		return true
	case strings.HasPrefix(path, "/groups/") && strings.HasSuffix(path, "/channels") && r.Method == http.MethodPut:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/groups/"), "/channels")
		h.apiReplaceChannelGroupMembers(w, r, decodePathID(id))
		return true
	case strings.HasPrefix(path, "/groups/") && r.Method == http.MethodGet:
		h.apiGetChannelGroup(w, r, decodePathID(strings.TrimPrefix(path, "/groups/")))
		return true
	case strings.HasPrefix(path, "/groups/") && r.Method == http.MethodPatch:
		h.apiUpdateChannelGroup(w, r, decodePathID(strings.TrimPrefix(path, "/groups/")))
		return true
	case strings.HasPrefix(path, "/groups/") && r.Method == http.MethodDelete:
		h.apiDeleteChannelGroup(w, r, decodePathID(strings.TrimPrefix(path, "/groups/")))
		return true
	}
	return false
}

// === handlers ===

func (h *Handler) apiListChannelGroups(w http.ResponseWriter, _ *http.Request) {
	groups := config.GetActiveChannelGroups()
	sortChannelGroups(groups)
	newAPIByID := indexNewAPIChannels(config.GetNewAPIChannels())
	directByID := indexDirectChannels(config.GetDirectChannels())
	out := make([]groupView, 0, len(groups))
	for _, g := range groups {
		out = append(out, makeGroupView(g, newAPIByID, directByID))
	}
	writeAdminJSON(w, http.StatusOK, out)
}

func (h *Handler) apiCreateChannelGroup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, channelGroupBodyLimit)
	var req groupCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	group := config.ChannelGroup{
		ID:            req.ID,
		Name:          req.Name,
		Description:   strings.TrimSpace(req.Description),
		Enabled:       req.Enabled,
		ModelPatterns: trimStringSlice(req.ModelPatterns),
		SortOrder:     req.SortOrder,
	}
	if err := config.AddChannelGroup(group); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeAdminJSONError(w, http.StatusConflict, err.Error())
		} else {
			writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	AuditLog("channel_group_create", adminAuditActor(r), fmt.Sprintf("id=%s name=%s", group.ID, group.Name))
	created, _ := config.GetChannelGroup(group.ID)
	view := makeGroupView(created, indexNewAPIChannels(config.GetNewAPIChannels()), indexDirectChannels(config.GetDirectChannels()))
	writeAdminJSON(w, http.StatusCreated, view)
}

func (h *Handler) apiGetChannelGroup(w http.ResponseWriter, _ *http.Request, id string) {
	group, ok := config.GetChannelGroup(id)
	if !ok || group.DeletedAt != 0 {
		writeAdminJSONError(w, http.StatusNotFound, "channel group not found")
		return
	}
	view := makeGroupView(group, indexNewAPIChannels(config.GetNewAPIChannels()), indexDirectChannels(config.GetDirectChannels()))
	writeAdminJSON(w, http.StatusOK, view)
}

func (h *Handler) apiUpdateChannelGroup(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, channelGroupBodyLimit)
	var req groupUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := config.UpdateChannelGroupByID(id, func(g *config.ChannelGroup) {
		if req.Name != nil {
			if name := strings.TrimSpace(*req.Name); name != "" {
				g.Name = name
			}
		}
		if req.Description != nil {
			g.Description = strings.TrimSpace(*req.Description)
		}
		if req.Enabled != nil {
			g.Enabled = *req.Enabled
		}
		if req.ModelPatterns != nil {
			g.ModelPatterns = trimStringSlice(*req.ModelPatterns)
		}
		if req.SortOrder != nil {
			g.SortOrder = *req.SortOrder
		}
	}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeAdminJSONError(w, http.StatusNotFound, err.Error())
		} else {
			writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	AuditLog("channel_group_update", adminAuditActor(r), fmt.Sprintf("id=%s", id))
	updated, _ := config.GetChannelGroup(id)
	view := makeGroupView(updated, indexNewAPIChannels(config.GetNewAPIChannels()), indexDirectChannels(config.GetDirectChannels()))
	writeAdminJSON(w, http.StatusOK, view)
}

func (h *Handler) apiReplaceChannelGroupMembers(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, channelGroupBodyLimit)
	var req groupMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	newAPIByID := indexNewAPIChannels(config.GetNewAPIChannels())
	directByID := indexDirectChannels(config.GetDirectChannels())
	if errMsg := validateGroupMembers(req.Channels, newAPIByID, directByID); errMsg != "" {
		writeAdminJSONError(w, http.StatusBadRequest, errMsg)
		return
	}
	defaultID := strings.TrimSpace(req.DefaultChannelID)
	if defaultID != "" {
		found := false
		for _, ref := range req.Channels {
			if config.RuntimeChannelIDFor(ref) == defaultID {
				found = true
				break
			}
		}
		if !found {
			writeAdminJSONError(w, http.StatusBadRequest, "defaultRuntimeChannelId is not a member of the new channel list")
			return
		}
	}
	if err := config.ReplaceChannelGroupMembers(id, req.Channels, defaultID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeAdminJSONError(w, http.StatusNotFound, err.Error())
		} else {
			writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	AuditLog("channel_group_members_update", adminAuditActor(r), fmt.Sprintf("id=%s count=%d default=%s", id, len(req.Channels), defaultID))
	updated, _ := config.GetChannelGroup(id)
	view := makeGroupView(updated, newAPIByID, directByID)
	writeAdminJSON(w, http.StatusOK, view)
}

func (h *Handler) apiDeleteChannelGroup(w http.ResponseWriter, r *http.Request, id string) {
	if err := config.SoftDeleteChannelGroup(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeAdminJSONError(w, http.StatusNotFound, err.Error())
		} else {
			writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := config.PruneApiKeyChannelPreferences([]string{id}); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "deleted group but failed to prune preferences: "+err.Error())
		return
	}
	AuditLog("channel_group_delete", adminAuditActor(r), fmt.Sprintf("id=%s", id))
	writeAdminJSON(w, http.StatusOK, map[string]any{"success": true})
}

// === helpers (sort + member validation) ===

func sortChannelGroups(groups []config.ChannelGroup) {
	// 按 sortOrder 升序，相同 sortOrder 按 name 字典序
	for i := 1; i < len(groups); i++ {
		j := i
		for j > 0 {
			a, b := groups[j-1], groups[j]
			if a.SortOrder < b.SortOrder || (a.SortOrder == b.SortOrder && a.Name <= b.Name) {
				break
			}
			groups[j-1], groups[j] = b, a
			j--
		}
	}
}

func validateGroupMembers(refs []config.ChannelGroupChannelRef, newAPIByID map[string]config.NewAPIChannel, directByID map[string]config.DirectChannel) string {
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		typ := strings.ToLower(strings.TrimSpace(ref.SourceType))
		if typ != "newapi" && typ != "direct" {
			return fmt.Sprintf("invalid sourceType %q", ref.SourceType)
		}
		cid := strings.TrimSpace(ref.ChannelID)
		if cid == "" {
			return "channelId is required"
		}
		switch typ {
		case "newapi":
			if ch, ok := newAPIByID[cid]; !ok || ch.DeletedAt > 0 {
				return fmt.Sprintf("newapi channel %q not found", cid)
			}
		case "direct":
			if ch, ok := directByID[cid]; !ok || ch.DeletedAt > 0 {
				return fmt.Sprintf("direct channel %q not found", cid)
			}
		}
		key := typ + ":" + cid
		if _, dup := seen[key]; dup {
			return fmt.Sprintf("duplicate channel member %s", key)
		}
		seen[key] = struct{}{}
	}
	return ""
}
