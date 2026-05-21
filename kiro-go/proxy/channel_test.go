package proxy

import (
	"testing"

	"kiro-api-proxy/config"
)

func routerTestChannel(id, typ string, enabled bool, models ...string) config.ChannelConfig {
	return config.ChannelConfig{
		ID:      id,
		Type:    typ,
		Enabled: enabled,
		BaseURL: "http://example.test/v1",
		Models:  models,
	}
}

func TestResolveSeriesDefaultAllowsDuplicateModels(t *testing.T) {
	model := "gpt-5.5"
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "ch-a", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{
			routerTestChannel("ch-a", "openai", true, model),
			routerTestChannel("ch-b", "openai", true, model),
		},
		nil,
	)

	ch, ok := r.Resolve(model, &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if ch.ID() != "ch-a" {
		t.Fatalf("channel = %q, want ch-a", ch.ID())
	}
}

func TestResolveLegacyFlatWhenNoSeries(t *testing.T) {
	r := NewChannelRouter(nil, []config.ChannelConfig{
		routerTestChannel("ch-a", "openai", true, "claude-sonnet-4.6"),
		routerTestChannel("ch-b", "openai", true, "gpt-5.5"),
	}, nil)

	ch, ok := r.Resolve("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if ch.ID() != "ch-b" {
		t.Fatalf("channel = %q, want ch-b", ch.ID())
	}
}

func TestResolveNoSeriesMatch(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "claude", Name: "Claude", DefaultChannelID: "ch-a", ModelPatterns: []string{"claude-"}}},
		[]config.ChannelConfig{routerTestChannel("ch-a", "openai", true, "claude-sonnet-4.6")},
		nil,
	)

	if ch, ok := r.Resolve("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI}); ok || ch != nil {
		t.Fatalf("Resolve = (%v, %v), want nil,false", ch, ok)
	}
}

func TestResolveDefaultDisabled(t *testing.T) {
	model := "gpt-5.5"
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "ch-a", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{
			routerTestChannel("ch-a", "openai", false, model),
			routerTestChannel("ch-b", "openai", true, model),
		},
		nil,
	)

	if ch, ok := r.Resolve(model, &ResolveHint{Protocol: ProtocolOpenAI}); ok || ch != nil {
		t.Fatalf("Resolve = (%v, %v), want nil,false", ch, ok)
	}
}

func TestResolveDefaultMissingFromRouter(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "missing", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("ch-a", "openai", true, "gpt-5.5")},
		nil,
	)

	if ch, ok := r.Resolve("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI}); ok || ch != nil {
		t.Fatalf("Resolve = (%v, %v), want nil,false", ch, ok)
	}
}

func TestResolveProtocolMismatch(t *testing.T) {
	model := "gpt-5.5"
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "ch-openai", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("ch-openai", "openai", true, model)},
		nil,
	)

	if ch, ok := r.Resolve(model, &ResolveHint{Protocol: ProtocolClaude}); ok || ch != nil {
		t.Fatalf("Resolve = (%v, %v), want nil,false", ch, ok)
	}
}

func TestResolveExplicitChannelIDBypassesSeries(t *testing.T) {
	model := "gpt-5.5"
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "ch-b", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{
			routerTestChannel("ch-a", "openai", true, model),
			routerTestChannel("ch-b", "openai", true, model),
		},
		nil,
	)

	ch, ok := r.Resolve(model, &ResolveHint{Protocol: ProtocolOpenAI, ChannelID: "ch-a"})
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if ch.ID() != "ch-a" {
		t.Fatalf("channel = %q, want ch-a", ch.ID())
	}
}

func TestResolveNilRouter(t *testing.T) {
	var r *ChannelRouter
	if ch, ok := r.Resolve("gpt-5.5", &ResolveHint{Protocol: ProtocolOpenAI}); ok || ch != nil {
		t.Fatalf("Resolve = (%v, %v), want nil,false", ch, ok)
	}
}

func TestResolveHintNilFallsBackToIdentifySeries(t *testing.T) {
	model := "gpt-5.5"
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", DefaultChannelID: "ch-a", ModelPatterns: []string{"gpt-"}}},
		[]config.ChannelConfig{routerTestChannel("ch-a", "openai", true, model)},
		nil,
	)

	ch, ok := r.Resolve(model, nil)
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if ch.ID() != "ch-a" {
		t.Fatalf("channel = %q, want ch-a", ch.ID())
	}
}

func TestIdentifySeriesPrefix(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "claude", Name: "Claude", ModelPatterns: []string{"claude-"}}},
		nil,
		nil,
	)

	if got := r.identifySeries("claude-3.5-sonnet"); got != "claude" {
		t.Fatalf("identifySeries = %q, want claude", got)
	}
}

func TestIdentifySeriesRegex(t *testing.T) {
	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", ModelPatterns: []string{"re:^gpt-.+$"}}},
		nil,
		nil,
	)

	if got := r.identifySeries("gpt-5.5"); got != "gpt" {
		t.Fatalf("identifySeries = %q, want gpt", got)
	}
}

func TestIdentifySeriesInvalidRegexSilentlySkipped(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("identifySeries panicked: %v", r)
		}
	}()

	r := NewChannelRouter(
		[]config.Series{{ID: "gpt", Name: "GPT", ModelPatterns: []string{"re:[invalid", "gpt-"}}},
		nil,
		nil,
	)

	if got := r.identifySeries("gpt-5.5"); got != "gpt" {
		t.Fatalf("identifySeries = %q, want gpt", got)
	}
}

func TestChannelByIDReturnsCorrectChannel(t *testing.T) {
	r := NewChannelRouter(nil, []config.ChannelConfig{
		routerTestChannel("ch-a", "openai", true, "model-a"),
		routerTestChannel("ch-b", "openai", true, "model-b"),
	}, nil)

	ch, ok := r.ChannelByID("ch-b")
	if !ok {
		t.Fatal("ChannelByID returned false")
	}
	if ch.ID() != "ch-b" {
		t.Fatalf("channel = %q, want ch-b", ch.ID())
	}
}

func TestLegacyFlatTrueWhenSeriesEmpty(t *testing.T) {
	r := NewChannelRouter(nil, []config.ChannelConfig{
		routerTestChannel("ch-a", "openai", true, "gpt-5.5"),
	}, nil)

	if !r.LegacyFlat() {
		t.Fatal("LegacyFlat = false, want true")
	}
}
