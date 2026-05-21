package proxy

import (
	"testing"

	"kiro-api-proxy/config"
)

func TestResolveDetailedExplicitChannelHintWins(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{
			routerTestChannel("default", "openai", true, "gpt-5.5"),
			routerTestChannel("explicit", "openai", true, "gpt-5.5"),
		},
		nil,
	)
	rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI, ChannelID: "explicit"})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "explicit" || rr.ResolvedBy != "explicit" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedUserPrefBeatsSeriesDefault(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-5.5")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", SeriesID: "gpt", Models: []string{"gpt-5.5"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
	)
	rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{
		Protocol:        ProtocolOpenAI,
		UserPreferences: map[string]string{"gpt": "apijing:tok-1"},
	})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "apijing:tok-1" || rr.ResolvedBy != "user_pref" || rr.SeriesID != "gpt" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedUserPrefMismatchedSeriesFallsBackToDefault(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-5.5")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", SeriesID: "claude", Models: []string{"gpt-5.5"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
	)
	rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{
		Protocol:        ProtocolOpenAI,
		UserPreferences: map[string]string{"gpt": "apijing:tok-1"},
	})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "default" || rr.ResolvedBy != "series_default" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedLegacyFlatPath(t *testing.T) {
	r := NewChannelRouter(nil, []config.ChannelConfig{
		routerTestChannel("a", "openai", true, "claude-sonnet-4.6"),
		routerTestChannel("b", "openai", true, "gpt-5.5"),
	}, nil)
	rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "b" || rr.ResolvedBy != "legacy_flat" || rr.SeriesID != "" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedUserPrefUnsupportedModelFallsBackToDefault(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-5.5")},
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{ID: "apijing:tok-1", ProviderID: "apijing", SeriesID: "gpt", Models: []string{"gpt-4o"}, Enabled: true}}),
		WithNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", BaseURL: "https://apijing.test", Enabled: true}}),
	)
	rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{
		Protocol:        ProtocolOpenAI,
		UserPreferences: map[string]string{"gpt": "apijing:tok-1"},
	})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "default" || rr.ResolvedBy != "series_default" {
		t.Fatalf("ResolveDetailed = %+v", rr)
	}
}

func TestResolveDetailedDefaultUnsupportedModelReturnsFalse(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-4o")},
		nil,
	)
	if rr, ok := r.ResolveDetailed("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI}); ok || rr != nil {
		t.Fatalf("ResolveDetailed = (%+v, %v), want nil,false", rr, ok)
	}
}

// TestResolveDetailedManualChannelSeriesMismatchFallsBack 防御 stale config：
// 手动渠道显式标了 SeriesID="gpt" 但被外部导入的 user prefs 关联到 series "claude"，
// router 必须拒绝该映射并回 series_default，而不是无脑信任 persisted preferences。
func TestResolveDetailedManualChannelSeriesMismatchFallsBack(t *testing.T) {
	gptChannel := routerTestChannel("gpt-channel", "openai", true, "claude-sonnet-4.6")
	gptChannel.SeriesID = "gpt"
	claudeDefault := routerTestChannel("claude-default", "openai", true, "claude-sonnet-4.6")
	r := NewChannelRouter(
		[]config.Series{
			{ID: "gpt", Name: "GPT", DefaultChannelID: "gpt-channel", ModelPatterns: []string{"gpt-"}},
			{ID: "claude", Name: "Claude", DefaultChannelID: "claude-default", ModelPatterns: []string{"claude-"}},
		},
		[]config.ChannelConfig{gptChannel, claudeDefault},
		nil,
	)
	rr, ok := r.ResolveDetailed("claude-sonnet-4.6", &ResolveHint{
		Protocol:        ProtocolOpenAI,
		UserPreferences: map[string]string{"claude": "gpt-channel"},
	})
	if !ok {
		t.Fatal("ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "claude-default" || rr.ResolvedBy != "series_default" {
		t.Fatalf("ResolveDetailed = %+v; want claude-default/series_default", rr)
	}
}
