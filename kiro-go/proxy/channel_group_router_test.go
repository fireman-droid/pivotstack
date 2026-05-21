package proxy

import (
	"testing"

	"kiro-api-proxy/config"
)

// ChannelGroup 路由单测：覆盖 group_pref / group_default / fallback to series 三条路径。

func TestResolveDetailedChannelGroupDefault(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "claude", Name: "Claude", DefaultChannelID: "fallback", ModelPatterns: []string{"claude-"}}},
		[]config.ChannelConfig{routerTestChannel("fallback", "openai", true, "claude-opus-4-6")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", Models: []string{"claude-opus-4-6"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
		WithChannelGroups([]config.ChannelGroup{{
			ID:      "claude-group", Name: "Claude 分组", Enabled: true,
			ModelPatterns:           []string{"claude-"},
			DefaultRuntimeChannelID: "apijing:tok-1",
			Channels:                []config.ChannelGroupChannelRef{{SourceType: "newapi", ChannelID: "apijing:tok-1"}},
		}}),
	)
	rr, ok := r.ResolveDetailed("claude-opus-4-6", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "apijing:tok-1" || rr.ResolvedBy != "group_default" || rr.GroupID != "claude-group" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedChannelGroupUserPreferenceWins(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "claude", Name: "Claude", DefaultChannelID: "fallback", ModelPatterns: []string{"claude-"}}},
		[]config.ChannelConfig{routerTestChannel("fallback", "openai", true, "claude-opus-4-6")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{
			{ID: "apijing:tok-1", ProviderID: "apijing", Models: []string{"claude-opus-4-6"}, Enabled: true},
			{ID: "apijing:tok-2", ProviderID: "apijing", Models: []string{"claude-opus-4-6"}, Enabled: true},
		}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
		WithChannelGroups([]config.ChannelGroup{{
			ID:      "claude-group", Name: "Claude 分组", Enabled: true,
			ModelPatterns:           []string{"claude-"},
			DefaultRuntimeChannelID: "apijing:tok-1",
			Channels: []config.ChannelGroupChannelRef{
				{SourceType: "newapi", ChannelID: "apijing:tok-1"},
				{SourceType: "newapi", ChannelID: "apijing:tok-2"},
			},
		}}),
	)
	// user 偏好走 tok-2，期望命中 tok-2 而非默认 tok-1
	rr, ok := r.ResolveDetailed("claude-opus-4-6", &ResolveHint{
		Protocol:           ProtocolOpenAI,
		ChannelPreferences: map[string]string{"claude-group": "apijing:tok-2"},
	})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "apijing:tok-2" || rr.ResolvedBy != "group_pref" || rr.GroupID != "claude-group" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedChannelGroupFallsBackToSeriesWhenNoMatch(t *testing.T) {
	// model "gpt-5" 没有对应 group 但有对应 series → 路由器应 fallback 到 series
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-5")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", Models: []string{"claude-opus-4-6"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
		WithChannelGroups([]config.ChannelGroup{{
			ID: "claude-group", Name: "Claude", Enabled: true,
			ModelPatterns:           []string{"claude-"},
			DefaultRuntimeChannelID: "apijing:tok-1",
			Channels:                []config.ChannelGroupChannelRef{{SourceType: "newapi", ChannelID: "apijing:tok-1"}},
		}}),
	)
	rr, ok := r.ResolveDetailed("gpt-5", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "default" || rr.ResolvedBy != "series_default" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedChannelGroupSoftDeletedExcluded(t *testing.T) {
	// 软删的 group 不参与路由
	r := NewChannelRouter(
		[]config.Series{{ID: "claude", Name: "Claude", DefaultChannelID: "fallback", ModelPatterns: []string{"claude-"}}},
		[]config.ChannelConfig{routerTestChannel("fallback", "openai", true, "claude-opus-4-6")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", Models: []string{"claude-opus-4-6"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
		WithChannelGroups([]config.ChannelGroup{{
			ID: "claude-group", Name: "Claude", Enabled: true, DeletedAt: 99999,
			ModelPatterns:           []string{"claude-"},
			DefaultRuntimeChannelID: "apijing:tok-1",
			Channels:                []config.ChannelGroupChannelRef{{SourceType: "newapi", ChannelID: "apijing:tok-1"}},
		}}),
	)
	rr, ok := r.ResolveDetailed("claude-opus-4-6", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	// 软删的 group 被忽略，走 series 默认
	if rr.ResolvedBy != "series_default" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedChannelGroupExplicitGroupIDHint(t *testing.T) {
	// hint.GroupID 显式覆盖 model 推断
	r := NewChannelRouter(
		nil,
		nil, nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", Models: []string{"anything"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
		WithChannelGroups([]config.ChannelGroup{{
			ID: "my-group", Name: "My", Enabled: true,
			// 没设 ModelPatterns，必须靠 hint.GroupID 显式指定
			DefaultRuntimeChannelID: "apijing:tok-1",
			Channels:                []config.ChannelGroupChannelRef{{SourceType: "newapi", ChannelID: "apijing:tok-1"}},
		}}),
	)
	rr, ok := r.ResolveDetailed("anything", &ResolveHint{Protocol: ProtocolOpenAI, GroupID: "my-group"})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.GroupID != "my-group" || rr.ResolvedBy != "group_default" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}
