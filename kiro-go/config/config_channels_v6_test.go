package config

import (
	"strings"
	"testing"
)

func TestMigrateConfigToV6_DisablesMaskedNewAPIKey(t *testing.T) {
	c := &Config{
		NewAPIChannels: []NewAPIChannel{{
			ID:      "apijing:tok-1",
			Alias:   "masked",
			Enabled: true,
			Status:  1,
		}},
	}
	resetTestConfig(t, c)
	enc, err := EncryptSecret("sk-****masked")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	c.NewAPIChannels[0].UpstreamKeyEnc = enc

	changed, warnings := MigrateConfigToV6(c)
	if !changed {
		t.Fatal("expected migration to change config")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "masked") {
		t.Fatalf("expected masked warning, got %#v", warnings)
	}
	got := c.NewAPIChannels[0]
	if got.Enabled || got.Status != 0 {
		t.Fatalf("channel not disabled: enabled=%v status=%d", got.Enabled, got.Status)
	}
	if got.CreateMode != "legacy_import" {
		t.Fatalf("CreateMode = %q", got.CreateMode)
	}
	if c.SchemaVersion != 6 || c.LastV6MigrationAt == 0 {
		t.Fatalf("schema fields not updated: version=%d at=%d", c.SchemaVersion, c.LastV6MigrationAt)
	}
}

func TestMigrateConfigToV6_IsIdempotent(t *testing.T) {
	c := &Config{}
	resetTestConfig(t, c)

	changed, warnings := MigrateConfigToV6(c)
	if !changed {
		t.Fatal("first migration should change config")
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}

	changed, warnings = MigrateConfigToV6(c)
	if changed {
		t.Fatal("second migration should be a no-op")
	}
	if len(warnings) != 0 {
		t.Fatalf("second migration warnings: %#v", warnings)
	}
}

func TestMigrateConfigToV6_PreservesValidKey(t *testing.T) {
	c := &Config{
		NewAPIChannels: []NewAPIChannel{{
			ID:      "apijing:tok-2",
			Alias:   "valid",
			Enabled: true,
			Status:  1,
		}},
	}
	resetTestConfig(t, c)
	enc, err := EncryptSecret("sk-valid-token")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	c.NewAPIChannels[0].UpstreamKeyEnc = enc

	_, warnings := MigrateConfigToV6(c)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	got := c.NewAPIChannels[0]
	if !got.Enabled || got.Status != 1 {
		t.Fatalf("valid channel changed: enabled=%v status=%d", got.Enabled, got.Status)
	}
	if got.CreateMode != "legacy_import" {
		t.Fatalf("CreateMode = %q", got.CreateMode)
	}
}

func TestAliasUniqueAcrossTypes(t *testing.T) {
	resetTestConfig(t, &Config{
		NewAPIChannels: []NewAPIChannel{{
			ID:      "apijing:tok-1",
			Alias:   "Shared",
			Enabled: true,
		}},
	})

	if err := ValidateGroupAliasUnique("", " shared "); err == nil {
		t.Fatal("expected new-api alias conflict")
	}
	if _, err := AddDirectChannel(DirectChannel{
		Type:    "openai",
		Alias:   "SHARED",
		BaseURL: "https://example.test/v1",
		Enabled: true,
	}); err == nil {
		t.Fatal("expected AddDirectChannel alias conflict")
	}
}

func TestAliasUniqueAllowsSameAfterDeletion(t *testing.T) {
	resetTestConfig(t, &Config{
		NewAPIChannels: []NewAPIChannel{{
			ID:        "apijing:tok-1",
			Alias:     "Shared",
			DeletedAt: 1,
		}},
		DirectChannels: []DirectChannel{{
			ID:        "direct-1",
			Type:      "openai",
			Alias:     "Shared",
			DeletedAt: 1,
		}},
	})

	if err := ValidateGroupAliasUnique("", " shared "); err != nil {
		t.Fatalf("deleted aliases should be ignored: %v", err)
	}
}

func TestDirectChannelCRUDRoundtrip(t *testing.T) {
	resetTestConfig(t, &Config{})

	created, err := AddDirectChannel(DirectChannel{
		Type:    "openai",
		Alias:   " Primary ",
		BaseURL: "https://upstream.example/v1",
		Models:  []string{"gpt-5.5"},
		SellPrice: DirectSellPrice{
			Default: DirectSellPriceRow{InputPerM: 1, OutputPerM: 2},
			Models: map[string]DirectSellPriceRow{
				"gpt-5.5": {InputPerM: 1.5, OutputPerM: 2.5},
			},
		},
		ModelMapping: map[string]string{"gpt-5.5": "gpt-upstream"},
		ExtraHeaders: map[string]string{"X-Test": "yes"},
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("AddDirectChannel: %v", err)
	}
	if created.ID == "" || created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("timestamps/id not set: %#v", created)
	}
	if created.Alias != "Primary" || created.Type != "openai" {
		t.Fatalf("normalization failed: %#v", created)
	}

	got, ok := GetDirectChannel(created.ID)
	if !ok {
		t.Fatal("GetDirectChannel returned false")
	}
	got.Models[0] = "mutated"
	got.ModelMapping["gpt-5.5"] = "mutated"
	got.ExtraHeaders["X-Test"] = "mutated"
	got.SellPrice.Models["gpt-5.5"] = DirectSellPriceRow{InputPerM: 99}

	again, _ := GetDirectChannel(created.ID)
	if again.Models[0] != "gpt-5.5" || again.ModelMapping["gpt-5.5"] != "gpt-upstream" {
		t.Fatalf("copy isolation failed: %#v", again)
	}
	if again.ExtraHeaders["X-Test"] != "yes" || again.SellPrice.Models["gpt-5.5"].InputPerM != 1.5 {
		t.Fatalf("map copy isolation failed: %#v", again)
	}

	if err := SetDirectChannelAPIKey(created.ID, "v1:gcm:key"); err != nil {
		t.Fatalf("SetDirectChannelAPIKey: %v", err)
	}
	updated, err := UpdateDirectChannel(created.ID, DirectChannel{
		Type:    "kiro",
		Alias:   "Kiro Direct",
		BaseURL: "https://kiro.example",
		Models:  []string{"claude-sonnet-4.5"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateDirectChannel: %v", err)
	}
	if updated.Type != "kiro" || updated.Alias != "Kiro Direct" {
		t.Fatalf("unexpected updated channel: %#v", updated)
	}

	if err := DeleteDirectChannel(created.ID, false); err != nil {
		t.Fatalf("soft DeleteDirectChannel: %v", err)
	}
	deleted, _ := GetDirectChannel(created.ID)
	if deleted.DeletedAt == 0 || deleted.Enabled {
		t.Fatalf("soft delete failed: %#v", deleted)
	}
	if err := DeleteDirectChannel(created.ID, true); err != nil {
		t.Fatalf("hard DeleteDirectChannel: %v", err)
	}
	if _, ok := GetDirectChannel(created.ID); ok {
		t.Fatal("hard deleted channel still exists")
	}
}
