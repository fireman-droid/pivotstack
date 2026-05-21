package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

// newAPITestConfig 利用 billing_test.go TestMain 已经初始化的 cfg；
// 只 push/pop NewAPIProviders + NewAPIChannels，不重置 cfgPath（避免污染其他测试）。
func newAPITestConfig(t *testing.T) {
	t.Helper()
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "test-secret-key")

	oldProviders := config.GetNewAPIProviders()
	oldChannels := config.GetNewAPIChannels()
	t.Cleanup(func() {
		_ = config.UpdateNewAPIProviders(oldProviders)
		_ = config.UpdateNewAPIChannels(oldChannels)
	})
	if err := config.UpdateNewAPIProviders(nil); err != nil {
		t.Fatalf("UpdateNewAPIProviders(nil) error = %v", err)
	}
	if err := config.UpdateNewAPIChannels(nil); err != nil {
		t.Fatalf("UpdateNewAPIChannels(nil) error = %v", err)
	}
}

func TestMaterializeNewAPIChannelsPreservesAdminFields(t *testing.T) {
	newAPITestConfig(t)
	m := NewNewAPIManager(nil)
	p := config.NewAPIProvider{ID: "apijing"}
	existing := []config.NewAPIChannel{{
		ID: "apijing:tok-908", ProviderID: "apijing", Alias: "特价 GPT", Markup: 3.5, SeriesID: "gpt", Enabled: false,
	}}
	got := m.materializeNewAPIChannels(existing, p, []config.NewAPIModel{{
		ModelName: "gpt-5.5", EnableGroups: []string{"vip"},
	}}, nil, []config.NewAPIToken{{
		ID: 908, Name: "upstream", Key: "sk-a", Group: "vip", Status: 1,
	}})
	if len(got) != 1 {
		t.Fatalf("len(channels) = %d", len(got))
	}
	ch := got[0]
	if ch.Alias != "特价 GPT" || ch.Markup != 3.5 || ch.SeriesID != "gpt" || ch.Enabled {
		t.Fatalf("admin fields not preserved: %+v", ch)
	}
	if ch.GroupName != "vip" || ch.UpstreamTokenName != "upstream" || len(ch.Models) != 1 || ch.Models[0] != "gpt-5.5" {
		t.Fatalf("synced fields incorrect: %+v", ch)
	}
}

func TestMaterializeNewAPIChannelsSoftDeletesDisappearedTokens(t *testing.T) {
	newAPITestConfig(t)
	m := NewNewAPIManager(nil)
	got := m.materializeNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-1", ProviderID: "apijing", Enabled: true,
	}}, config.NewAPIProvider{ID: "apijing"}, nil, nil, nil)
	if len(got) != 1 {
		t.Fatalf("len(channels) = %d", len(got))
	}
	if got[0].Enabled || got[0].DeletedAt == 0 {
		t.Fatalf("expected soft-delete, got %+v", got[0])
	}
}

func TestMaterializeNewAPIChannelsIntersectsModels(t *testing.T) {
	newAPITestConfig(t)
	m := NewNewAPIManager(nil)
	got := m.materializeNewAPIChannels(nil, config.NewAPIProvider{ID: "p"}, []config.NewAPIModel{
		{ModelName: "a", EnableGroups: []string{"vip"}},
		{ModelName: "b", EnableGroups: []string{"other"}},
		{ModelName: "c", EnableGroups: []string{"vip", "other"}},
	}, nil, []config.NewAPIToken{{ID: 1, Name: "t", Key: "sk", Group: "vip", Status: 1}})
	if len(got) != 1 || len(got[0].Models) != 2 || got[0].Models[0] != "a" || got[0].Models[1] != "c" {
		t.Fatalf("models intersection = %+v", got)
	}
}

func TestMaterializeNewAPIChannelsAssignsDefaultsForNewTokens(t *testing.T) {
	newAPITestConfig(t)
	m := NewNewAPIManager(nil)
	got := m.materializeNewAPIChannels(nil, config.NewAPIProvider{ID: "p"}, nil, nil, []config.NewAPIToken{{ID: 1, Name: "tok", Key: "sk", Group: "vip", Status: 1}})
	if len(got) != 1 {
		t.Fatalf("len(channels) = %d", len(got))
	}
	if got[0].Alias != "tok" || got[0].Markup != 2.0 || !got[0].Enabled || got[0].ID != "p:tok-1" {
		t.Fatalf("defaults incorrect: %+v", got[0])
	}
}

func TestSyncProviderMetadataAtomicSwap(t *testing.T) {
	newAPITestConfig(t)
	pass, _ := config.EncryptSecret("p")
	token, _ := config.EncryptSecret("old-token")
	var failGroups bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pricing":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{{"model_name": "ok", "enable_groups": []string{"vip"}}}})
		case "/api/user/groups":
			if failGroups {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "groups down"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"vip": map[string]any{"ratio": 1}}})
		case "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{{"id": 1, "name": "tok", "key": "sk", "group": "vip", "status": 1}}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{
		ID: "p", BaseURL: srv.URL, Username: "u", PasswordEnc: pass, AccessTokenEnc: token, AccessTokenExpiresAt: time.Now().Add(time.Hour).Unix(), UserID: 1, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	m := NewNewAPIManager(nil)
	failGroups = true
	if err := m.SyncProviderMetadata(context.Background(), "p"); err == nil {
		t.Fatal("expected sync failure")
	}
	if _, ok := m.Cache("p"); ok {
		t.Fatal("cache swapped on failed sync")
	}
	p, _ := config.GetNewAPIProvider("p")
	if p.LastSyncError == "" {
		t.Fatal("LastSyncError not set")
	}

	failGroups = false
	if err := m.SyncProviderMetadata(context.Background(), "p"); err != nil {
		t.Fatalf("SyncProviderMetadata() error = %v", err)
	}
	cache, ok := m.Cache("p")
	if !ok {
		t.Fatal("cache missing after success")
	}
	_, _, tokens, _ := snapshotProviderCache(cache)
	if len(tokens) != 1 {
		t.Fatalf("tokens = %+v", tokens)
	}
}

func TestEnsureLoginSkipsWhenTokenFresh(t *testing.T) {
	newAPITestConfig(t)
	pass, _ := config.EncryptSecret("p")
	token, _ := config.EncryptSecret("fresh")
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{
		ID: "p", BaseURL: srv.URL, Username: "u", PasswordEnc: pass, AccessTokenEnc: token, AccessTokenExpiresAt: time.Now().Add(time.Hour).Unix(), UserID: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := NewNewAPIManager(nil).EnsureLogin(context.Background(), "p", false); err != nil {
		t.Fatalf("EnsureLogin() error = %v", err)
	}
	if called {
		t.Fatal("fresh token should skip login")
	}
}

func TestEnsureLoginForceTriggersRelogin(t *testing.T) {
	newAPITestConfig(t)
	pass, _ := config.EncryptSecret("p")
	token, _ := config.EncryptSecret("old")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "new-session", HttpOnly: true})
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 9}})
	}))
	defer srv.Close()
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{
		ID: "p", BaseURL: srv.URL, Username: "u", PasswordEnc: pass, AccessTokenEnc: token, AccessTokenExpiresAt: time.Now().Add(time.Hour).Unix(), UserID: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := NewNewAPIManager(nil).EnsureLogin(context.Background(), "p", true); err != nil {
		t.Fatalf("EnsureLogin() error = %v", err)
	}
	p, _ := config.GetNewAPIProvider("p")
	if p.UserID != 9 {
		t.Fatalf("provider not updated: %+v", p)
	}
}
