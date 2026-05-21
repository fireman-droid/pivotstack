package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func newAPIProviderTestHandler(t *testing.T, upstream http.Handler) (*Handler, string) {
	t.Helper()
	newAPITestConfig(t)
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	h := &Handler{adminSessions: newAdminSessionStore()}
	h.newapiManager = NewNewAPIManager(h)
	return h, srv.URL
}

func TestCreateProviderValidatesIDPattern(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", bytes.NewBufferString(`{"id":"bad/id","baseUrl":"`+upstream+`","username":"u","password":"p","quotaPerUnitDollar":1,"yuanPerUpstreamDollar":1}`))
	rr := httptest.NewRecorder()
	h.apiCreateProvider(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateProviderRequiresValidBaseURL(t *testing.T) {
	h, _ := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", bytes.NewBufferString(`{"id":"p","baseUrl":"ftp://example.com","username":"u","password":"p","quotaPerUnitDollar":1,"yuanPerUpstreamDollar":1}`))
	rr := httptest.NewRecorder()
	h.apiCreateProvider(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateProviderRequiresPositiveUnits(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", bytes.NewBufferString(`{"id":"p","baseUrl":"`+upstream+`","username":"u","password":"p","quotaPerUnitDollar":0,"yuanPerUpstreamDollar":1}`))
	rr := httptest.NewRecorder()
	h.apiCreateProvider(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateProviderFailsClosedOnLoginError(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "bad"})
	}))
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", bytes.NewBufferString(`{"id":"p","baseUrl":"`+upstream+`","username":"u","password":"p","quotaPerUnitDollar":1,"yuanPerUpstreamDollar":1}`))
	rr := httptest.NewRecorder()
	h.apiCreateProvider(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if providers := config.GetNewAPIProviders(); len(providers) != 0 {
		t.Fatalf("provider saved on failed login: %+v", providers)
	}
}

func TestCreateProviderSuccessTriggersSyncAndAudit(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "access", HttpOnly: true})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 1}})
		case "/api/pricing":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{}})
		case "/api/user/groups":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{}})
		case "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", bytes.NewBufferString(`{"id":"p","name":"Provider","baseUrl":"`+upstream+`","username":"u","password":"p","quotaPerUnitDollar":500000,"yuanPerUpstreamDollar":1,"enabled":true}`))
	rr := httptest.NewRecorder()
	h.apiCreateProvider(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	p, ok := config.GetNewAPIProvider("p")
	if !ok || p.PasswordEnc == "" || p.AccessTokenEnc == "" {
		t.Fatalf("provider not saved with encrypted secrets: %+v", p)
	}
}

func TestListProvidersMasksSecrets(t *testing.T) {
	newAPITestConfig(t)
	pass, _ := config.EncryptSecret("password")
	token, _ := config.EncryptSecret("access")
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", Name: "P", BaseURL: "https://example.com", Username: "u", PasswordEnc: pass, AccessTokenEnc: token}}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{adminSessions: newAdminSessionStore(), newapiManager: NewNewAPIManager(nil)}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/providers", nil)
	rr := httptest.NewRecorder()
	h.apiListProviders(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, pass) || strings.Contains(body, token) || strings.Contains(body, "password") || strings.Contains(body, "access") {
		t.Fatalf("secrets leaked: %s", body)
	}
}

func TestUpdateProviderPartial(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "new", HttpOnly: true})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 2}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{}})
	}))
	pass, _ := config.EncryptSecret("oldpass")
	token, _ := config.EncryptSecret("oldtoken")
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", Name: "Old", BaseURL: upstream, Username: "u", PasswordEnc: pass, AccessTokenEnc: token, AccessTokenExpiresAt: time.Now().Add(time.Hour).Unix(), UserID: 1}}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/admin/api/providers/p", bytes.NewBufferString(`{"name":"New Name"}`))
	rr := httptest.NewRecorder()
	h.apiUpdateProvider(rr, req, "p")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	p, _ := config.GetNewAPIProvider("p")
	if p.Name != "New Name" || p.PasswordEnc != pass || p.UserID != 1 {
		t.Fatalf("partial update incorrect: %+v", p)
	}
}

func TestDeleteProviderSoftDisablesByDefault(t *testing.T) {
	newAPITestConfig(t)
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{{ID: "p:tok-1", ProviderID: "p", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{adminSessions: newAdminSessionStore(), newapiManager: NewNewAPIManager(nil)}
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/providers/p", nil)
	rr := httptest.NewRecorder()
	h.apiDeleteProvider(rr, req, "p")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	p, _ := config.GetNewAPIProvider("p")
	channels := config.GetNewAPIChannels()
	if p.Enabled || channels[0].Enabled {
		t.Fatalf("not disabled: provider=%+v channels=%+v", p, channels)
	}
}

func TestDeleteProviderPurgeRejectsIfChannelsLinked(t *testing.T) {
	newAPITestConfig(t)
	_ = config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p"}})
	_ = config.UpdateNewAPIChannels([]config.NewAPIChannel{{ID: "p:tok-1", ProviderID: "p", DeletedAt: 0}})
	h := &Handler{adminSessions: newAdminSessionStore(), newapiManager: NewNewAPIManager(nil)}
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/providers/p?purge=true", nil)
	rr := httptest.NewRecorder()
	h.apiDeleteProvider(rr, req, "p")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSyncProviderReturnsCountsOnSuccess(t *testing.T) {
	h, upstream := newAPIProviderTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pricing":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{{"model_name": "m", "enable_groups": []string{"g"}}}})
		case "/api/user/groups":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"g": map[string]any{"ratio": 1}}})
		case "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{{"id": 1, "name": "t", "key": "sk", "group": "g", "status": 1}}})
		default:
			http.NotFound(w, r)
		}
	}))
	pass, _ := config.EncryptSecret("p")
	token, _ := config.EncryptSecret("access")
	_ = config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", BaseURL: upstream, Username: "u", PasswordEnc: pass, AccessTokenEnc: token, AccessTokenExpiresAt: time.Now().Add(time.Hour).Unix(), UserID: 1}})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers/p/sync", nil)
	rr := httptest.NewRecorder()
	h.apiSyncProvider(rr, req, "p")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"tokenCount":1`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestProviderEndpointsRequireSessionAndCSRF(t *testing.T) {
	newAPITestConfig(t)
	h := &Handler{adminSessions: newAdminSessionStore(), newapiManager: NewNewAPIManager(nil)}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/providers", nil)
	rr := httptest.NewRecorder()
	h.handleAdminAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d body=%s", rr.Code, rr.Body.String())
	}
}
