package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kiro-api-proxy/config"
)

func seedNewAPIChannelsAdminTest(t *testing.T) *Handler {
	t.Helper()
	newAPITestConfig(t)
	if err := config.UpdateSeries([]config.Series{
		{ID: "gpt", Name: "GPT", DefaultChannelID: "apijing:tok-1", ModelPatterns: []string{"gpt-"}},
		{ID: "claude", Name: "Claude", DefaultChannelID: "apijing:tok-2", ModelPatterns: []string{"claude-"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "apijing:tok-1", ProviderID: "apijing", Alias: "GPT 渠道", UpstreamTokenID: 1, UpstreamKeyEnc: "encrypted-secret-XYZ", GroupName: "vip", Models: []string{"gpt-5.5"}, Markup: 2, SeriesID: "gpt", Enabled: true},
		{ID: "apijing:tok-2", ProviderID: "apijing", Alias: "Claude 渠道", UpstreamTokenID: 2, UpstreamKeyEnc: "encrypted-secret-CCC", GroupName: "vip", Models: []string{"claude-sonnet-4.6"}, Markup: 1.5, SeriesID: "claude", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	return tokenTestHandler()
}

func TestListNewAPIChannelsMasksUpstreamKey(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/newapi/channels", nil)
	rr := httptest.NewRecorder()
	h.apiListNewAPIChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "encrypted-secret") || strings.Contains(rr.Body.String(), "upstreamKey") {
		t.Fatalf("upstream key leaked: %s", rr.Body.String())
	}
	var out []publicNewAPIChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("channels count = %d", len(out))
	}
}

func TestPatchNewAPIChannelAliasMarkupSeriesEnabled(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	body := bytes.NewBufferString(`{"alias":"新别名","markup":3.5,"seriesId":"claude","enabled":false}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/newapi/channels/apijing:tok-1", body)
	rr := httptest.NewRecorder()
	h.apiPatchNewAPIChannel(rr, req, "apijing:tok-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var got publicNewAPIChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Alias != "新别名" || got.Markup != 3.5 || got.SeriesID != "claude" || got.Enabled {
		t.Fatalf("patched fields wrong: %+v", got)
	}
	// 持久化校验
	all := config.GetNewAPIChannels()
	for _, c := range all {
		if c.ID == "apijing:tok-1" {
			if c.Alias != "新别名" || c.Markup != 3.5 || c.SeriesID != "claude" || c.Enabled {
				t.Fatalf("config not persisted: %+v", c)
			}
			// 同步流程的字段不变
			if c.UpstreamTokenID != 1 || c.GroupName != "vip" {
				t.Fatalf("synced fields tampered: %+v", c)
			}
			return
		}
	}
	t.Fatal("channel disappeared")
}

func TestPatchNewAPIChannelInvalidSeriesReturns400(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	body := bytes.NewBufferString(`{"seriesId":"nonexistent"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/newapi/channels/apijing:tok-1", body)
	rr := httptest.NewRecorder()
	h.apiPatchNewAPIChannel(rr, req, "apijing:tok-1")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPatchNewAPIChannelClearSeriesAllowed(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	body := bytes.NewBufferString(`{"seriesId":""}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/newapi/channels/apijing:tok-1", body)
	rr := httptest.NewRecorder()
	h.apiPatchNewAPIChannel(rr, req, "apijing:tok-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPatchNewAPIChannelMarkupZeroOrNegativeReturns400(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	for _, payload := range []string{`{"markup":0}`, `{"markup":-1.5}`} {
		req := httptest.NewRequest(http.MethodPatch, "/admin/api/newapi/channels/apijing:tok-1", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		h.apiPatchNewAPIChannel(rr, req, "apijing:tok-1")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("payload %s status = %d body=%s", payload, rr.Code, rr.Body.String())
		}
	}
}

func TestPatchNewAPIChannelNotFound(t *testing.T) {
	h := seedNewAPIChannelsAdminTest(t)
	body := bytes.NewBufferString(`{"alias":"foo"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/newapi/channels/missing", body)
	rr := httptest.NewRecorder()
	h.apiPatchNewAPIChannel(rr, req, "missing")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHealthCheckNewAPIChannelSuccess(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-good")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-good" {
			t.Errorf("missing or wrong Authorization: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.5"}]}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"x","model":"gpt-5.5","choices":[{"finish_reason":"stop"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{
		{ID: "apijing", BaseURL: srv.URL, Enabled: true, QuotaPerUnitDollar: 1000, YuanPerUpstreamDollar: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "apijing:tok-1", ProviderID: "apijing", UpstreamKeyEnc: keyEnc, Models: []string{"gpt-5.5"}, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	h := tokenTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels/apijing:tok-1/health-check", nil)
	rr := httptest.NewRecorder()
	h.apiHealthCheckNewAPIChannel(rr, req, "apijing:tok-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var result HealthCheckResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Success || !result.ModelsOK || !result.ChatOK {
		t.Fatalf("unexpected health result: %+v", result)
	}
	if result.ModelTested != "gpt-5.5" {
		t.Fatalf("wrong test model: %+v", result)
	}
}

func TestHealthCheckNewAPIChannelDisabledReturns409(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-x")
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", BaseURL: "https://example.test", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-1", ProviderID: "p", UpstreamKeyEnc: keyEnc, Models: []string{"gpt-5.5"}, Enabled: false},
	}); err != nil {
		t.Fatal(err)
	}
	h := tokenTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels/p:tok-1/health-check", nil)
	rr := httptest.NewRecorder()
	h.apiHealthCheckNewAPIChannel(rr, req, "p:tok-1")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHealthCheckNewAPIChannelUpstreamErrorRedactsSecretInBody(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-leak")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 恶意上游把 Authorization echo 进 body
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"got Bearer sk-leak"}`))
	}))
	defer srv.Close()
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "p", BaseURL: srv.URL, Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-2", ProviderID: "p", UpstreamKeyEnc: keyEnc, Models: []string{"gpt-5.5"}, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	h := tokenTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/newapi/channels/p:tok-2/health-check", nil)
	rr := httptest.NewRecorder()
	h.apiHealthCheckNewAPIChannel(rr, req, "p:tok-2")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "sk-leak") {
		t.Fatalf("upstream secret leaked: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "[REDACTED]") {
		t.Fatalf("redaction marker missing: %s", rr.Body.String())
	}
}
