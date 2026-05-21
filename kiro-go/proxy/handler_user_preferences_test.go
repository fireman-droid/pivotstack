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

func seedPreferenceTestConfig(t *testing.T) {
	t.Helper()
	newAPITestConfig(t)
	if err := config.UpdateSeries([]config.Series{
		{ID: "gpt", Name: "GPT", DefaultChannelID: "default", ModelPatterns: []string{"gpt-"}},
		{ID: "claude", Name: "Claude", DefaultChannelID: "claude-default", ModelPatterns: []string{"claude-"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "apijing:tok-1", ProviderID: "apijing", Alias: "特价 GPT", SeriesID: "gpt", Enabled: true},
		{ID: "apijing:tok-2", ProviderID: "apijing", Alias: "Claude", SeriesID: "claude", Enabled: true},
		{ID: "apijing:tok-disabled", ProviderID: "apijing", Alias: "Disabled", SeriesID: "gpt", Enabled: false},
		{ID: "apijing:tok-deleted", ProviderID: "apijing", Alias: "Deleted", SeriesID: "gpt", Enabled: true, DeletedAt: 123},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddApiKey(config.ApiKeyInfo{
		ID:                "key-1",
		Key:               "sk-user",
		Enabled:           true,
		Plan:              "credit",
		Balance:           10,
		SeriesPreferences: map[string]string{"gpt": "apijing:tok-1"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUserPreferencesGetReturnsCurrentPreferencesAndAvailableSeries(t *testing.T) {
	seedPreferenceTestConfig(t)
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/user/api/preferences", nil)
	req.Header.Set("Authorization", "Bearer sk-user")
	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp userPreferencesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ChannelPreferences["gpt"] != "apijing:tok-1" {
		t.Fatalf("channelPreferences = %#v", resp.ChannelPreferences)
	}
	if len(resp.AvailableSeries) != 2 {
		t.Fatalf("availableSeries = %#v", resp.AvailableSeries)
	}
	// defaultChannelId 必须按 series 配置回填，前端用它标 "系统默认"
	for _, s := range resp.AvailableSeries {
		switch s.ID {
		case "gpt":
			if s.DefaultChannelID != "default" {
				t.Fatalf("gpt defaultChannelId = %q", s.DefaultChannelID)
			}
		case "claude":
			if s.DefaultChannelID != "claude-default" {
				t.Fatalf("claude defaultChannelId = %q", s.DefaultChannelID)
			}
		}
	}
	body := rr.Body.String()
	if strings.Contains(body, "tok-disabled") || strings.Contains(body, "tok-deleted") || strings.Contains(body, "upstreamKey") {
		t.Fatalf("private or unavailable channel leaked: %s", body)
	}
}

func TestUserPreferencesPutValidMappingUpdatesApiKey(t *testing.T) {
	seedPreferenceTestConfig(t)
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/user/api/preferences", bytes.NewBufferString(`{"channelPreferences":{"claude":"apijing:tok-2"}}`))
	req.Header.Set("Authorization", "Bearer sk-user")
	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	info := config.FindApiKeyByID("key-1")
	if info == nil {
		t.Fatal("key missing")
	}
	if len(info.SeriesPreferences) != 1 || info.SeriesPreferences["claude"] != "apijing:tok-2" {
		t.Fatalf("preferences not updated: %#v", info.SeriesPreferences)
	}
}

func TestUserPreferencesPutDisabledOrDeletedChannelReturns400(t *testing.T) {
	seedPreferenceTestConfig(t)
	h := &Handler{}
	for _, channelID := range []string{"apijing:tok-disabled", "apijing:tok-deleted"} {
		req := httptest.NewRequest(http.MethodPut, "/user/api/preferences", bytes.NewBufferString(`{"channelPreferences":{"gpt":"`+channelID+`"}}`))
		req.Header.Set("Authorization", "Bearer sk-user")
		rr := httptest.NewRecorder()
		h.handleUserAPI(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s", channelID, rr.Code, rr.Body.String())
		}
	}
}

func TestUserPreferencesPutSeriesMismatchReturns400(t *testing.T) {
	seedPreferenceTestConfig(t)
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/user/api/preferences", bytes.NewBufferString(`{"channelPreferences":{"gpt":"apijing:tok-2"}}`))
	req.Header.Set("Authorization", "Bearer sk-user")
	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUserPreferencesPutEmptyStringClearsSeriesPreference(t *testing.T) {
	seedPreferenceTestConfig(t)
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/user/api/preferences", bytes.NewBufferString(`{"channelPreferences":{"gpt":""}}`))
	req.Header.Set("Authorization", "Bearer sk-user")
	rr := httptest.NewRecorder()
	h.handleUserAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	info := config.FindApiKeyByID("key-1")
	if info == nil {
		t.Fatal("key missing")
	}
	if len(info.SeriesPreferences) != 0 {
		t.Fatalf("preferences not cleared: %#v", info.SeriesPreferences)
	}
}
