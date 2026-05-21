package config

import (
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func resetTestConfig(t *testing.T, c *Config) {
	t.Helper()
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if c == nil {
		c = &Config{}
	}
	if c.Password == "" {
		c.Password = "test-admin-password-hash"
	}
	cfg = c
	cfgPath = filepath.Join(t.TempDir(), "config.json")
	passwordEnvOverride = false
}

func assertFloatEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}

func TestNewAPIProviderRoundTrip(t *testing.T) {
	want := NewAPIProvider{
		ID:                    "apijing",
		Name:                  "API Jing",
		BaseURL:               "https://apijing.com",
		Username:              "owner",
		PasswordEnc:           "v1:gcm:password",
		AccessTokenEnc:        "v1:gcm:token",
		AccessTokenExpiresAt:  1778825532,
		UserID:                123,
		QuotaPerUnitDollar:    500000,
		YuanPerUpstreamDollar: 1.0,
		LastSyncAt:            1778825600,
		LastSyncError:         "previous error",
		SyncIntervalSec:       3600,
		Enabled:               true,
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got NewAPIProvider
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roundtrip mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestNewAPIChannelDeepCopyIsolation(t *testing.T) {
	resetTestConfig(t, &Config{})
	in := []NewAPIChannel{{
		ID:                "apijing:tok-908",
		ProviderID:        "apijing",
		Alias:             "特价 GPT",
		UpstreamTokenID:   908,
		UpstreamKeyEnc:    "v1:gcm:key",
		UpstreamTokenName: "huo",
		GroupName:         "codex全网最低价格",
		Models:            []string{"gpt-5.5", "gpt-5.5-codex"},
		Markup:            2,
		SeriesID:          "gpt",
		Enabled:           true,
		RemainQuota:       -158,
		UnlimitedQuota:    true,
		Status:            1,
		LastSeenAt:        1778825600,
	}}
	if err := UpdateNewAPIChannels(in); err != nil {
		t.Fatalf("UpdateNewAPIChannels: %v", err)
	}
	in[0].Models[0] = "mutated-input"

	got := GetNewAPIChannels()
	got[0].Alias = "mutated"
	got[0].Models[0] = "mutated-return"

	again := GetNewAPIChannels()
	if again[0].Alias != "特价 GPT" {
		t.Fatalf("Alias mutated through returned copy: %q", again[0].Alias)
	}
	if again[0].Models[0] != "gpt-5.5" {
		t.Fatalf("Models mutated through copy: %q", again[0].Models[0])
	}
}

func TestApiKeyInfoSeriesPreferencesDeepCopy(t *testing.T) {
	resetTestConfig(t, &Config{
		ApiKeys: []ApiKeyInfo{{
			ID:                "key-1",
			Key:               "sk-test",
			Enabled:           true,
			Models:            map[string]int64{"gpt-5.5": 1},
			SeriesPreferences: map[string]string{"gpt": "apijing:tok-908"},
		}},
	})

	keys := GetAllApiKeys()
	keys[0].SeriesPreferences["gpt"] = "mutated"
	keys[0].Models["gpt-5.5"] = 99

	got := FindApiKeyByID("key-1")
	if got == nil {
		t.Fatal("FindApiKeyByID returned nil")
	}
	if got.SeriesPreferences["gpt"] != "apijing:tok-908" {
		t.Fatalf("SeriesPreferences mutated through returned copy: %q", got.SeriesPreferences["gpt"])
	}
	if got.Models["gpt-5.5"] != 1 {
		t.Fatalf("Models mutated through returned copy: %d", got.Models["gpt-5.5"])
	}
}

func TestPivotStackDollarsPerYuanDefaultWhenZero(t *testing.T) {
	resetTestConfig(t, &Config{})
	assertFloatEqual(t, GetPivotStackDollarsPerYuan(), DefaultPivotStackDollarsPerYuan)
}

func TestPivotStackDollarsPerYuanReadWriteRoundTrip(t *testing.T) {
	resetTestConfig(t, &Config{})
	if err := UpdatePivotStackDollarsPerYuan(12.5, false); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan: %v", err)
	}
	assertFloatEqual(t, GetPivotStackDollarsPerYuan(), 12.5)
}

func TestUpdatePivotStackDollarsPerYuanRejectsNonPositive(t *testing.T) {
	resetTestConfig(t, &Config{})
	for _, v := range []float64{0, -1} {
		if err := UpdatePivotStackDollarsPerYuan(v, false); err == nil {
			t.Fatalf("UpdatePivotStackDollarsPerYuan(%v) expected error", v)
		}
	}
}

func TestUpdatePivotStackDollarsPerYuanRebalanceMaintainsRealYuan(t *testing.T) {
	resetTestConfig(t, &Config{
		PivotStackDollarsPerYuan: 20,
		ApiKeys: []ApiKeyInfo{{
			ID:          "key-1",
			Key:         "sk-test",
			Balance:     100,
			GiftBalance: 20,
		}},
	})
	if err := UpdatePivotStackDollarsPerYuan(10, true); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan: %v", err)
	}
	got := FindApiKeyByID("key-1")
	if got == nil {
		t.Fatal("FindApiKeyByID returned nil")
	}
	assertFloatEqual(t, got.Balance, 50)
	assertFloatEqual(t, got.GiftBalance, 10)
	assertFloatEqual(t, got.Balance/GetPivotStackDollarsPerYuan(), 5)
	assertFloatEqual(t, got.GiftBalance/GetPivotStackDollarsPerYuan(), 1)
}

func TestUpdatePivotStackDollarsPerYuanWithoutRebalanceLeavesBalance(t *testing.T) {
	resetTestConfig(t, &Config{
		PivotStackDollarsPerYuan: 20,
		ApiKeys: []ApiKeyInfo{{
			ID:          "key-1",
			Key:         "sk-test",
			Balance:     100,
			GiftBalance: 20,
		}},
	})
	if err := UpdatePivotStackDollarsPerYuan(10, false); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan: %v", err)
	}
	got := FindApiKeyByID("key-1")
	if got == nil {
		t.Fatal("FindApiKeyByID returned nil")
	}
	assertFloatEqual(t, got.Balance, 100)
	assertFloatEqual(t, got.GiftBalance, 20)
}

func TestSetApiKeySeriesPreferencesValidation(t *testing.T) {
	resetTestConfig(t, &Config{
		ApiKeys: []ApiKeyInfo{{
			ID:      "key-1",
			Key:     "sk-test",
			Enabled: true,
		}},
		NewAPIChannels: []NewAPIChannel{
			{ID: "apijing:tok-1", ProviderID: "apijing", SeriesID: "gpt", Enabled: true},
			{ID: "apijing:tok-2", ProviderID: "apijing", SeriesID: "claude", Enabled: true},
			{ID: "apijing:tok-3", ProviderID: "apijing", SeriesID: "gpt", Enabled: false},
			{ID: "apijing:tok-4", ProviderID: "apijing", SeriesID: "gpt", Enabled: true, DeletedAt: 123},
		},
	})

	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": "apijing:tok-1"}); err != nil {
		t.Fatalf("SetApiKeySeriesPreferences valid: %v", err)
	}
	got := FindApiKeyByID("key-1")
	if got == nil {
		t.Fatal("FindApiKeyByID returned nil")
	}
	if got.SeriesPreferences["gpt"] != "apijing:tok-1" {
		t.Fatalf("preference = %q", got.SeriesPreferences["gpt"])
	}

	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": "missing"}); err == nil {
		t.Fatal("expected missing channel error")
	}
	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": "apijing:tok-2"}); err == nil {
		t.Fatal("expected series mismatch error")
	}
	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": "apijing:tok-3"}); err == nil {
		t.Fatal("expected disabled channel error")
	}
	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": "apijing:tok-4"}); err == nil {
		t.Fatal("expected deleted channel error")
	}

	if err := SetApiKeySeriesPreferences("key-1", map[string]string{"gpt": ""}); err != nil {
		t.Fatalf("SetApiKeySeriesPreferences clear: %v", err)
	}
	got = FindApiKeyByID("key-1")
	if got == nil {
		t.Fatal("FindApiKeyByID returned nil after clear")
	}
	if len(got.SeriesPreferences) != 0 {
		t.Fatalf("preferences not cleared: %#v", got.SeriesPreferences)
	}
}
