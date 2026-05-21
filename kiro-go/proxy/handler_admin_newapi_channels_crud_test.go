package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func newAPIChannelCRUDTestConfig(t *testing.T) {
	t.Helper()
	newAPITestConfig(t)
	oldDirect := config.GetDirectChannels()
	t.Cleanup(func() {
		_ = config.UpdateDirectChannels(oldDirect)
	})
	if err := config.UpdateDirectChannels(nil); err != nil {
		t.Fatalf("UpdateDirectChannels(nil): %v", err)
	}
}

func newAPIChannelCRUDUpstream(t *testing.T, tokenID int, key string) (*httptest.Server, *int32, *int32, *map[string]any) {
	t.Helper()
	var createCount int32
	var deleteCount int32
	var createBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Errorf("New-Api-User = %q, want 42", got)
		}
		if ck, err := r.Cookie("session"); err != nil || ck.Value != "session-cookie" {
			t.Errorf("session cookie = %v %v, want session-cookie", ck, err)
		}
		switch {
		case r.URL.Path == "/api/token/" && r.Method == http.MethodPost:
			atomic.AddInt32(&createCount, 1)
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"id":  tokenID,
					"key": key,
				},
			})
		case r.URL.Path == fmt.Sprintf("/api/token/%d", tokenID) && r.Method == http.MethodDelete:
			atomic.AddInt32(&deleteCount, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Errorf("unexpected upstream request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return srv, &createCount, &deleteCount, &createBody
}

func seedNewAPIChannelCRUDProvider(t *testing.T, baseURL string) *Handler {
	t.Helper()
	sessionEnc, err := config.EncryptSecret("session-cookie")
	if err != nil {
		t.Fatalf("EncryptSecret(session): %v", err)
	}
	err = config.UpdateNewAPIProviders([]config.NewAPIProvider{{
		ID:                    "apijing",
		Name:                  "apijing",
		BaseURL:               baseURL,
		Username:              "admin",
		AccessTokenEnc:        sessionEnc,
		AccessTokenExpiresAt:  time.Now().Add(time.Hour).Unix(),
		UserID:                42,
		QuotaPerUnitDollar:    1000,
		YuanPerUpstreamDollar: 1,
		Enabled:               true,
	}})
	if err != nil {
		t.Fatalf("UpdateNewAPIProviders: %v", err)
	}
	h := tokenTestHandler()
	h.newapiManager = NewNewAPIManager(h)
	return h
}

func newAPIChannelCreatePayload(alias string) *bytes.Buffer {
	return bytes.NewBufferString(fmt.Sprintf(`{
		"providerId":"apijing",
		"alias":%q,
		"group":"vip",
		"models":["gpt-5.5","claude-sonnet-4.6"],
		"markup":2.5,
		"remainQuota":12345,
		"unlimitedQuota":false,
		"expiredTime":-1,
		"modelLimitsEnabled":false,
		"modelLimits":"",
		"crossGroupRetry":true,
		"allowIPs":"127.0.0.1"
	}`, alias))
}

func TestCreateNewAPIChannelHappyPath(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, createCount, deleteCount, createBody := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels", newAPIChannelCreatePayload("GPT Premium"))
	rr := httptest.NewRecorder()
	h.apiCreateNewAPIChannel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(createCount) != 1 || atomic.LoadInt32(deleteCount) != 0 {
		t.Fatalf("create/delete counts = %d/%d", atomic.LoadInt32(createCount), atomic.LoadInt32(deleteCount))
	}
	if got := (*createBody)["models"]; got != "gpt-5.5,claude-sonnet-4.6" {
		t.Fatalf("models body = %#v", got)
	}
	ch, ok := config.GetNewAPIChannel("apijing:tok-908")
	if !ok {
		t.Fatal("created channel not found")
	}
	key, err := config.DecryptSecret(ch.UpstreamKeyEnc)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if key != "sk-FULL" {
		t.Fatalf("stored key = %q, want sk-FULL", key)
	}
	if ch.Alias != "GPT Premium" || ch.CreateMode != "pivotstack" || !ch.Enabled {
		t.Fatalf("created channel fields wrong: %+v", ch)
	}
}

func TestCreateNewAPIChannelRejectsDuplicateAlias(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, createCount, _, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-1", ProviderID: "apijing", Alias: "Taken", UpstreamTokenID: 1, Markup: 2, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels", newAPIChannelCreatePayload(" taken "))
	rr := httptest.NewRecorder()
	h.apiCreateNewAPIChannel(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(createCount) != 0 {
		t.Fatalf("upstream create should not be called")
	}
}

func TestCreateNewAPIChannelRollbackOnSaveFailure(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, createCount, deleteCount, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-908", ProviderID: "apijing", Alias: "Existing", UpstreamTokenID: 908, Markup: 2, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels", newAPIChannelCreatePayload("Fresh Alias"))
	rr := httptest.NewRecorder()
	h.apiCreateNewAPIChannel(rr, req)
	// codex stage 4 audit warning #1: ID conflict 应返 409（不是 500）
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(createCount) != 1 || atomic.LoadInt32(deleteCount) != 1 {
		t.Fatalf("rollback create/delete counts = %d/%d", atomic.LoadInt32(createCount), atomic.LoadInt32(deleteCount))
	}
}

func TestDeleteNewAPIChannelSoftDelete(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, _, deleteCount, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-908", ProviderID: "apijing", Alias: "Delete Me", UpstreamTokenID: 908, Markup: 2, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/newapi/channels/apijing:tok-908", nil)
	rr := httptest.NewRecorder()
	h.apiDeleteNewAPIChannel(rr, req, "apijing:tok-908")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(deleteCount) != 0 {
		t.Fatalf("upstream delete should not be called")
	}
	ch, _ := config.GetNewAPIChannel("apijing:tok-908")
	if ch.Enabled || ch.DeletedAt == 0 {
		t.Fatalf("channel not soft deleted: %+v", ch)
	}
}

func TestDeleteNewAPIChannelHardWithUpstream(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, _, deleteCount, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-908", ProviderID: "apijing", Alias: "Delete Me", UpstreamTokenID: 908, Markup: 2, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/newapi/channels/apijing:tok-908?deleteUpstream=true", nil)
	rr := httptest.NewRecorder()
	h.apiDeleteNewAPIChannel(rr, req, "apijing:tok-908")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(deleteCount) != 1 {
		t.Fatalf("upstream delete count = %d, want 1", atomic.LoadInt32(deleteCount))
	}
	ch, _ := config.GetNewAPIChannel("apijing:tok-908")
	if ch.Enabled || ch.DeletedAt == 0 {
		t.Fatalf("channel not soft deleted: %+v", ch)
	}
}

func TestGetNewAPIChannelExcludesDeleted(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, _, _, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{
		ID: "apijing:tok-908", ProviderID: "apijing", Alias: "Deleted", UpstreamTokenID: 908, Markup: 2, DeletedAt: time.Now().Unix(),
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/newapi/channels/apijing:tok-908", nil)
	rr := httptest.NewRecorder()
	h.apiGetNewAPIChannel(rr, req, "apijing:tok-908")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateNewAPIChannelRejectsDisabledProvider(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, createCount, _, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)
	providers := config.GetNewAPIProviders()
	providers[0].Enabled = false
	if err := config.UpdateNewAPIProviders(providers); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels", newAPIChannelCreatePayload("Disabled Provider"))
	rr := httptest.NewRecorder()
	h.apiCreateNewAPIChannel(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(createCount) != 0 {
		t.Fatalf("upstream create should not be called")
	}
}

func TestCreateNewAPIChannelRejectsBlankAlias(t *testing.T) {
	newAPIChannelCRUDTestConfig(t)
	srv, createCount, _, _ := newAPIChannelCRUDUpstream(t, 908, "FULL")
	defer srv.Close()
	h := seedNewAPIChannelCRUDProvider(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels", newAPIChannelCreatePayload("   "))
	rr := httptest.NewRecorder()
	h.apiCreateNewAPIChannel(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if atomic.LoadInt32(createCount) != 0 || !strings.Contains(rr.Body.String(), "alias") {
		t.Fatalf("unexpected blank alias response/count: body=%s count=%d", rr.Body.String(), atomic.LoadInt32(createCount))
	}
}
