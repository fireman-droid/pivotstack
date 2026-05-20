package db

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPricingConfigSingleton_UpdateGetVersionBump(t *testing.T) {
	ctx := testDB(t)
	var before map[string]any
	beforeVersion, _, err := GetPricingConfig(ctx, &before)
	if err != nil && err != ErrNotFound {
		t.Fatal(err)
	}
	token := testID("pricing")
	if err := UpdatePricingConfig(ctx, map[string]any{"token": token}, beforeVersion+1, "tester"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	version, updatedAt, err := GetPricingConfig(ctx, &got)
	if err != nil {
		t.Fatal(err)
	}
	if version <= beforeVersion || updatedAt.IsZero() || got["token"] != token {
		t.Fatalf("pricing config not updated: version=%d before=%d payload=%+v", version, beforeVersion, got)
	}
}

func TestStealthConfigSingleton_RoundTrip(t *testing.T) {
	ctx := testDB(t)
	token := testID("stealth")
	if err := UpdateStealthConfig(ctx, map[string]any{"mode": token}, "tester"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	updatedAt, err := GetStealthConfig(ctx, &got)
	if err != nil {
		t.Fatal(err)
	}
	if updatedAt.IsZero() || got["mode"] != token {
		t.Fatalf("stealth config not updated: %+v", got)
	}
}

func TestPromotionConfigSingleton_Window(t *testing.T) {
	ctx := testDB(t)
	start := time.Now().UTC().Add(-time.Hour)
	end := time.Now().UTC().Add(time.Hour)
	token := testID("promo")
	if err := UpdatePromotionConfig(ctx, map[string]any{"name": token}, true, &start, &end, "tester"); err != nil {
		t.Fatal(err)
	}
	got, err := GetPromotionConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.StartAt == nil || got.EndAt == nil || got.UpdatedBy != "tester" || payload["name"] != token {
		t.Fatalf("promotion config not updated: %+v payload=%+v", got, payload)
	}
}
