package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"kiro-api-proxy/config"
)

// adminGroupView 是 v5 channel 聚合视图（每条 channel 一行）。
// v6 把它降级为「GroupDrawer 候选 channel 池」，路径迁到 /admin/api/groups/channels。
type adminGroupView struct {
	Alias        string  `json:"alias"`
	SourceType   string  `json:"sourceType"`
	SourceDetail string  `json:"sourceDetail"`
	Billing      string  `json:"billing"`
	Status       string  `json:"status"`
	ChannelID    string  `json:"channelId"`
	RuntimeID    string  `json:"runtimeId"`
	Route        string  `json:"route"`
	Markup       float64 `json:"markup,omitempty"` // NewAPI markup 数字（前端做精确卖价算）
}

func (h *Handler) apiListGroupCandidateChannels(w http.ResponseWriter, _ *http.Request) {
	views := buildAdminCandidateChannelViews()
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].Alias < views[j].Alias
	})
	writeAdminJSON(w, http.StatusOK, views)
}

func buildAdminCandidateChannelViews() []adminGroupView {
	newAPIChannels := config.GetNewAPIChannels()
	directChannels := config.GetDirectChannels()
	out := make([]adminGroupView, 0, len(newAPIChannels)+len(directChannels))
	for _, ch := range newAPIChannels {
		if ch.DeletedAt == 0 {
			out = append(out, newAPIAdminGroupView(ch))
		}
	}
	for _, ch := range directChannels {
		if ch.DeletedAt == 0 {
			out = append(out, directAdminGroupView(ch))
		}
	}
	return out
}

func newAPIAdminGroupView(ch config.NewAPIChannel) adminGroupView {
	return adminGroupView{
		Alias:        ch.Alias,
		SourceType:   "newapi",
		SourceDetail: newAPIGroupSourceDetail(ch),
		Billing:      fmt.Sprintf("×%.2f markup", ch.Markup),
		Status:       adminGroupStatus(ch.Enabled),
		ChannelID:    ch.ID,
		RuntimeID:    ch.ID,
		Route:        fmt.Sprintf("/channels/newapi/%s/channels/%s", ch.ProviderID, ch.ID),
		Markup:       ch.Markup,
	}
}

func directAdminGroupView(ch config.DirectChannel) adminGroupView {
	return adminGroupView{
		Alias:        ch.Alias,
		SourceType:   "direct",
		SourceDetail: directGroupSourceDetail(ch),
		Billing:      directGroupBilling(ch.SellPrice),
		Status:       adminGroupStatus(ch.Enabled),
		ChannelID:    ch.ID,
		RuntimeID:    "direct:" + ch.ID,
		Route:        fmt.Sprintf("/channels/direct/%s", ch.ID),
	}
}

func newAPIGroupSourceDetail(ch config.NewAPIChannel) string {
	return fmt.Sprintf("%s / tok-%d / %s", providerGroupName(ch.ProviderID), ch.UpstreamTokenID, ch.GroupName)
}

func providerGroupName(providerID string) string {
	provider, ok := config.GetNewAPIProvider(providerID)
	if !ok {
		return providerID
	}
	name := strings.TrimSpace(provider.Name)
	if name == "" {
		return providerID
	}
	return name
}

func directGroupSourceDetail(ch config.DirectChannel) string {
	typ := strings.ToLower(strings.TrimSpace(ch.Type))
	if typ == "kiro" {
		return "kiro 账号池"
	}
	base := directBaseURLLabel(ch.BaseURL)
	if typ == "" {
		typ = "direct"
	}
	if base == "" {
		return typ
	}
	return fmt.Sprintf("%s / %s", typ, base)
}

func directBaseURLLabel(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return baseURL
}

func directGroupBilling(price config.DirectSellPrice) string {
	row := price.Default
	if row.InputPerM == 0 && row.OutputPerM == 0 {
		return "未设置定价"
	}
	return fmt.Sprintf("¥%.4f/Mtok in / ¥%.4f/Mtok out", row.InputPerM, row.OutputPerM)
}

func adminGroupStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// === group view 构造（groupView 在 handler_admin_groups.go 定义）===

func makeGroupView(g config.ChannelGroup, newAPIByID map[string]config.NewAPIChannel, directByID map[string]config.DirectChannel) groupView {
	entries := make([]groupChannelEntry, 0, len(g.Channels))
	enabledCount := 0
	for _, ref := range g.Channels {
		entry := groupChannelEntry{
			RuntimeID:  config.RuntimeChannelIDFor(ref),
			SourceType: ref.SourceType,
			ChannelID:  ref.ChannelID,
		}
		switch strings.ToLower(strings.TrimSpace(ref.SourceType)) {
		case "newapi":
			if ch, ok := newAPIByID[ref.ChannelID]; ok && ch.DeletedAt == 0 {
				entry.Alias = ch.Alias
				entry.Enabled = ch.Enabled
				entry.SourceDetail = newAPIGroupSourceDetail(ch)
				entry.Billing = fmt.Sprintf("×%.2f markup", ch.Markup)
				if ch.Enabled {
					enabledCount++
				}
			} else {
				entry.Missing = true
				entry.Alias = "(missing) " + ref.ChannelID
			}
		case "direct":
			if ch, ok := directByID[ref.ChannelID]; ok && ch.DeletedAt == 0 {
				entry.Alias = ch.Alias
				entry.Enabled = ch.Enabled
				entry.SourceDetail = directGroupSourceDetail(ch)
				entry.Billing = directGroupBilling(ch.SellPrice)
				if ch.Enabled {
					enabledCount++
				}
			} else {
				entry.Missing = true
				entry.Alias = "(missing) " + ref.ChannelID
			}
		default:
			entry.Missing = true
			entry.Alias = "(unknown source) " + ref.ChannelID
		}
		entries = append(entries, entry)
	}
	return groupView{
		ID:                      g.ID,
		Name:                    g.Name,
		Description:             g.Description,
		Enabled:                 g.Enabled,
		ModelPatterns:           g.ModelPatterns,
		DefaultRuntimeChannelID: g.DefaultRuntimeChannelID,
		SortOrder:               g.SortOrder,
		CreatedAt:               g.CreatedAt,
		UpdatedAt:               g.UpdatedAt,
		Channels:                entries,
		ChannelCount:            len(entries),
		EnabledChannelCount:     enabledCount,
	}
}

func indexNewAPIChannels(channels []config.NewAPIChannel) map[string]config.NewAPIChannel {
	out := make(map[string]config.NewAPIChannel, len(channels))
	for _, ch := range channels {
		out[ch.ID] = ch
	}
	return out
}

func indexDirectChannels(channels []config.DirectChannel) map[string]config.DirectChannel {
	out := make(map[string]config.DirectChannel, len(channels))
	for _, ch := range channels {
		out[ch.ID] = ch
	}
	return out
}

func trimStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
