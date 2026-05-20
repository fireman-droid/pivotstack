package db

import (
	"errors"
	"testing"
)

func TestSettingsRepository_GetSetRoundTrip(t *testing.T) {
	ctx := testDB(t)
	key := testID("setting")
	if err := SetSetting(ctx, key, map[string]any{"enabled": true}, "tester"); err != nil {
		t.Fatal(err)
	}
	got, err := GetSetting(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != key || got.UpdatedBy != "tester" || len(got.Value) == 0 {
		t.Fatalf("unexpected setting: %+v", got)
	}
}

func TestSettingsRepository_JSONWrappers(t *testing.T) {
	ctx := testDB(t)
	key := testID("setting")
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := SetSettingJSON(ctx, key, payload{Name: "alpha", Count: 2}, "tester"); err != nil {
		t.Fatal(err)
	}
	var got payload
	if err := GetSettingJSON(ctx, key, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "alpha" || got.Count != 2 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSettingsRepository_Delete(t *testing.T) {
	ctx := testDB(t)
	key := testID("setting")
	if err := SetSetting(ctx, key, map[string]any{"v": 1}, "tester"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSetting(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSetting(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSettingsRepository_Upsert(t *testing.T) {
	ctx := testDB(t)
	key := testID("setting")
	if err := SetSetting(ctx, key, map[string]any{"v": 1}, "one"); err != nil {
		t.Fatal(err)
	}
	if err := SetSetting(ctx, key, map[string]any{"v": 2}, "two"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := GetSettingJSON(ctx, key, &got); err != nil {
		t.Fatal(err)
	}
	if got["v"].(float64) != 2 {
		t.Fatalf("upsert did not replace value: %+v", got)
	}
	rows, err := ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Key == key && row.UpdatedBy == "two" {
			found = true
		}
	}
	if !found {
		t.Fatal("upserted setting not listed")
	}
}
