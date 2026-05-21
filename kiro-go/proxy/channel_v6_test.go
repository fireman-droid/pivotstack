package proxy

import (
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func encryptDirectTestKey(t *testing.T) string {
	t.Helper()
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "test-secret-key")
	enc, err := config.EncryptSecret("test-key-1234")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	return enc
}

func directOpenAITestChannel(t *testing.T, id string, enabled bool, models ...string) config.DirectChannel {
	t.Helper()
	return config.DirectChannel{
		ID:        id,
		Type:      "openai",
		Alias:     id,
		BaseURL:   "https://direct.example.test/v1",
		APIKeyEnc: encryptDirectTestKey(t),
		Models:    models,
		Enabled:   enabled,
	}
}

func TestRouterLegacyFlatRegistersNewAPIChannels(t *testing.T) {
	r := NewChannelRouter(
		nil,
		nil,
		nil,
		WithNewAPIChannels([]config.NewAPIChannel{{
			ID:              "apijing:tok-9",
			ProviderID:      "apijing",
			Alias:           "GPT",
			UpstreamTokenID: 9,
			GroupName:       "vip",
			Models:          []string{"gpt-test"},
			Enabled:         true,
		}}),
		WithNewAPIProviders([]config.NewAPIProvider{{
			ID:      "apijing",
			BaseURL: "https://apijing.example.test",
			Enabled: true,
		}}),
	)

	ch, ok := r.Resolve("gpt-test", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if ch.ID() != "apijing:tok-9" {
		t.Fatalf("channel ID = %q", ch.ID())
	}
}

func TestRouterLegacyFlatRegistersDirectOpenAIChannel(t *testing.T) {
	r := NewChannelRouter(
		nil,
		nil,
		nil,
		WithDirectChannels([]config.DirectChannel{
			directOpenAITestChannel(t, "direct-openai", true, "gpt-direct"),
		}),
	)

	ch, ok := r.Resolve("gpt-direct", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	oa, ok := ch.(*OpenAIChannel)
	if !ok {
		t.Fatalf("channel type = %T", ch)
	}
	if oa.ID() != "direct:direct-openai" || oa.cfg.APIKey != "test-key-1234" {
		t.Fatalf("channel not adapted correctly: id=%q apiKey=%q", oa.ID(), oa.cfg.APIKey)
	}
}

func TestRouterLegacyFlatRegistersDirectKiroChannel(t *testing.T) {
	r := NewChannelRouter(
		nil,
		nil,
		nil,
		WithDirectChannels([]config.DirectChannel{{
			ID:      "direct-kiro",
			Type:    "kiro",
			Alias:   "Kiro",
			Models:  []string{"claude-direct"},
			Enabled: true,
		}}),
	)

	ch, ok := r.Resolve("claude-direct", &ResolveHint{Protocol: ProtocolClaude})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if _, ok := ch.(*KiroChannel); !ok {
		t.Fatalf("channel type = %T", ch)
	}
	if ch.ID() != "direct:direct-kiro" {
		t.Fatalf("channel ID = %q", ch.ID())
	}
}

func TestRouterSeriesModeStillRegistersDirectChannels(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("default", "openai", true, "gpt-series")},
		nil,
		WithDirectChannels([]config.DirectChannel{
			directOpenAITestChannel(t, "direct-openai", true, "gpt-direct"),
		}),
	)

	rr, ok := r.ResolveDetailed("gpt-series", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("series default ResolveDetailed returned false")
	}
	if rr.Channel.ID() != "default" || rr.ResolvedBy != "series_default" {
		t.Fatalf("series default result = %+v", rr)
	}

	ch, ok := r.Resolve("gpt-direct", &ResolveHint{Protocol: ProtocolOpenAI, ChannelID: "direct:direct-openai"})
	if !ok {
		t.Fatal("direct explicit Resolve returned false")
	}
	if ch.ID() != "direct:direct-openai" {
		t.Fatalf("direct channel ID = %q", ch.ID())
	}
}

func TestRouterSkipsDirectChannelWithBadAPIKey(t *testing.T) {
	r := NewChannelRouter(nil, nil, nil, WithDirectChannels([]config.DirectChannel{{
		ID: "bad-key", Type: "openai", Alias: "Bad", BaseURL: "https://bad.example.test/v1",
		APIKeyEnc: "not-real-cipher", Models: []string{"gpt-bad"}, Enabled: true,
	}}))

	if _, ok := r.ChannelByID("direct:bad-key"); ok {
		t.Fatal("bad key channel should not be registered")
	}
}

func TestRouterSkipsDisabledDirectChannel(t *testing.T) {
	r := NewChannelRouter(nil, nil, nil, WithDirectChannels([]config.DirectChannel{
		directOpenAITestChannel(t, "disabled", false, "gpt-disabled"),
	}))

	if _, ok := r.ChannelByID("direct:disabled"); ok {
		t.Fatal("disabled channel should not be registered")
	}
}

func TestRouterSkipsOpenAIDirectChannelWithoutBaseURL(t *testing.T) {
	r := NewChannelRouter(nil, nil, nil, WithDirectChannels([]config.DirectChannel{{
		ID: "no-base", Type: "openai", Alias: "NoBase",
		APIKeyEnc: encryptDirectTestKey(t), Models: []string{"gpt-x"}, Enabled: true,
	}}))

	if _, ok := r.ChannelByID("direct:no-base"); ok {
		t.Fatal("openai DirectChannel without BaseURL must not register")
	}
}

func TestRouterSkipsDeletedDirectChannel(t *testing.T) {
	ch := directOpenAITestChannel(t, "deleted", true, "gpt-deleted")
	ch.DeletedAt = time.Now().Unix()
	r := NewChannelRouter(nil, nil, nil, WithDirectChannels([]config.DirectChannel{ch}))

	if _, ok := r.ChannelByID("direct:deleted"); ok {
		t.Fatal("deleted channel should not be registered")
	}
}
