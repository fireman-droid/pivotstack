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

func directChannelAdminTestConfig(t *testing.T) *Handler {
	t.Helper()
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "test-secret-key")
	oldDirect := config.GetDirectChannels()
	oldNewAPI := config.GetNewAPIChannels()
	t.Cleanup(func() {
		_ = config.UpdateNewAPIChannels(oldNewAPI)
		_ = config.UpdateDirectChannels(oldDirect)
	})
	if err := config.UpdateNewAPIChannels(nil); err != nil {
		t.Fatalf("UpdateNewAPIChannels(nil): %v", err)
	}
	if err := config.UpdateDirectChannels(nil); err != nil {
		t.Fatalf("UpdateDirectChannels(nil): %v", err)
	}
	return tokenTestHandler()
}

func seedDirectChannels(t *testing.T, channels ...config.DirectChannel) {
	t.Helper()
	if err := config.UpdateDirectChannels(channels); err != nil {
		t.Fatalf("UpdateDirectChannels: %v", err)
	}
}

func TestCreateDirectChannelEncryptsKey(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	body := bytes.NewBufferString(`{"id":"direct-primary","type":"openai","alias":"Primary","baseUrl":"https://upstream.example/v1","apiKey":"test-key-1234","models":["gpt-test"],"enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/direct-channels", body)
	rr := httptest.NewRecorder()

	h.apiCreateDirectChannel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "test-key-1234") || strings.Contains(rr.Body.String(), "apiKeyEnc") {
		t.Fatalf("secret leaked in response: %s", rr.Body.String())
	}
	var out publicDirectChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.HasAPIKey {
		t.Fatalf("hasAPIKey = false: %+v", out)
	}
	ch, ok := config.GetDirectChannel(out.ID)
	if !ok {
		t.Fatal("created channel not found")
	}
	plain, err := config.DecryptSecret(ch.APIKeyEnc)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if plain != "test-key-1234" {
		t.Fatalf("stored key = %q", plain)
	}
}

func TestCreateDirectChannelDuplicateAliasReturns409(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	seedDirectChannels(t, config.DirectChannel{ID: "direct-existing", Type: "openai", Alias: "Taken", APIKeyEnc: "stub", Enabled: true})
	body := bytes.NewBufferString(`{"id":"direct-dup","type":"openai","alias":" taken ","apiKey":"test-key-1234","enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/direct-channels", body)
	rr := httptest.NewRecorder()

	h.apiCreateDirectChannel(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateDirectChannelKiroTypeIgnoresApiKey(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	body := bytes.NewBufferString(`{"id":"direct-kiro","type":"kiro","alias":"Kiro Pool","apiKey":"test-key-1234","enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/direct-channels", body)
	rr := httptest.NewRecorder()

	h.apiCreateDirectChannel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out publicDirectChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	ch, ok := config.GetDirectChannel(out.ID)
	if !ok {
		t.Fatal("created channel not found")
	}
	if ch.APIKeyEnc != "" || out.HasAPIKey {
		t.Fatalf("kiro api key should be ignored: stored=%q public=%+v", ch.APIKeyEnc, out)
	}
}

func TestPatchDirectChannelPreservesEncryptedKeyOnEmptyPatch(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	enc, err := config.EncryptSecret("test-key-1234")
	if err != nil {
		t.Fatal(err)
	}
	seedDirectChannels(t, config.DirectChannel{ID: "direct-one", Type: "openai", Alias: "Primary", APIKeyEnc: enc, Enabled: true})
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/direct-channels/direct-one", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()

	h.apiPatchDirectChannel(rr, req, "direct-one")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	ch, _ := config.GetDirectChannel("direct-one")
	if ch.APIKeyEnc != enc || !ch.Enabled {
		t.Fatalf("patch did not preserve fields: %+v", ch)
	}
}

func TestPatchDirectChannelClearsAPIKey(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	enc, err := config.EncryptSecret("test-key-1234")
	if err != nil {
		t.Fatal(err)
	}
	seedDirectChannels(t, config.DirectChannel{
		ID: "direct-clear", Type: "openai", Alias: "Clear Me", APIKeyEnc: enc, Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/direct-channels/direct-clear", bytes.NewBufferString(`{"apiKey":""}`))
	rr := httptest.NewRecorder()

	h.apiPatchDirectChannel(rr, req, "direct-clear")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out publicDirectChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	ch, _ := config.GetDirectChannel("direct-clear")
	if ch.APIKeyEnc != "" || out.HasAPIKey {
		t.Fatalf("api key not cleared: stored=%q public=%+v", ch.APIKeyEnc, out)
	}
}

func TestPatchDirectChannelOnDeletedReturns409(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	seedDirectChannels(t, config.DirectChannel{ID: "direct-deleted", Type: "openai", Alias: "Deleted", APIKeyEnc: "stub", Enabled: false, DeletedAt: time.Now().Unix()})
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/direct-channels/direct-deleted", bytes.NewBufferString(`{"alias":"New"}`))
	rr := httptest.NewRecorder()

	h.apiPatchDirectChannel(rr, req, "direct-deleted")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPatchDirectChannelAliasConflict(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	seedDirectChannels(t,
		config.DirectChannel{ID: "direct-one", Type: "openai", Alias: "One", APIKeyEnc: "stub", Enabled: true},
		config.DirectChannel{ID: "direct-two", Type: "openai", Alias: "Two", APIKeyEnc: "stub", Enabled: true},
	)
	req := httptest.NewRequest(http.MethodPatch, "/admin/api/direct-channels/direct-one", bytes.NewBufferString(`{"alias":" two "}`))
	rr := httptest.NewRecorder()

	h.apiPatchDirectChannel(rr, req, "direct-one")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeleteDirectChannelSoftDelete(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	seedDirectChannels(t, config.DirectChannel{ID: "direct-delete", Type: "openai", Alias: "Delete Me", APIKeyEnc: "stub", Enabled: true})
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/direct-channels/direct-delete", nil)
	rr := httptest.NewRecorder()

	h.apiDeleteDirectChannel(rr, req, "direct-delete")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	ch, _ := config.GetDirectChannel("direct-delete")
	if ch.Enabled || ch.DeletedAt == 0 {
		t.Fatalf("channel not soft deleted: %+v", ch)
	}
}

func TestGetDirectChannelExcludesDeleted(t *testing.T) {
	h := directChannelAdminTestConfig(t)
	seedDirectChannels(t, config.DirectChannel{ID: "direct-deleted", Type: "openai", Alias: "Deleted", APIKeyEnc: "stub", Enabled: false, DeletedAt: time.Now().Unix()})
	req := httptest.NewRequest(http.MethodGet, "/admin/api/direct-channels/direct-deleted", nil)
	rr := httptest.NewRecorder()

	h.apiGetDirectChannel(rr, req, "direct-deleted")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPublicDirectChannelMasksAPIKey(t *testing.T) {
	out := toPublicDirectChannel(config.DirectChannel{
		ID:        "direct-mask",
		Type:      "openai",
		Alias:     "Masked",
		APIKeyEnc: "encrypted-secret-XYZ",
		Enabled:   true,
	})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if !out.HasAPIKey {
		t.Fatalf("hasAPIKey = false: %+v", out)
	}
	if strings.Contains(string(raw), "encrypted-secret") || strings.Contains(string(raw), "apiKeyEnc") {
		t.Fatalf("secret leaked: %s", string(raw))
	}
}
