package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kiro-api-proxy/config"
)

// channelGroupTestSetup 重置 NewAPI / Direct / ChannelGroup 配置，返回可用的 handler。
func channelGroupTestSetup(t *testing.T) *Handler {
	t.Helper()
	oldGroups := config.GetChannelGroups()
	oldNewAPI := config.GetNewAPIChannels()
	oldDirect := config.GetDirectChannels()
	t.Cleanup(func() {
		_ = config.UpdateChannelGroups(oldGroups)
		_ = config.UpdateNewAPIChannels(oldNewAPI)
		_ = config.UpdateDirectChannels(oldDirect)
	})
	_ = config.UpdateChannelGroups(nil)
	_ = config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-1", ProviderID: "p", Alias: "NewAPI 1", UpstreamTokenID: 1, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 2, Enabled: true},
		{ID: "p:tok-2", ProviderID: "p", Alias: "NewAPI 2", UpstreamTokenID: 2, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.5, Enabled: true},
	})
	_ = config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "d-1", Type: "openai", Alias: "Direct 1", APIKeyEnc: "stub", Enabled: true},
	})
	return tokenTestHandler()
}

func dispatchAdminGroups(t *testing.T, h *Handler, method, path string, body any) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		buf = bytes.NewBuffer(raw)
	}
	var req *http.Request
	if buf != nil {
		req = httptest.NewRequest(method, path, buf)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	// path 去掉 /admin/api 前缀给 routeAdminGroups
	innerPath := path[len("/admin/api"):]
	handled := h.routeAdminGroups(innerPath, rr, req)
	return rr, handled
}

func TestChannelGroupCreateAndList(t *testing.T) {
	h := channelGroupTestSetup(t)
	rr, handled := dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{
		"id":            "claude",
		"name":          "Claude 分组",
		"enabled":       true,
		"modelPatterns": []string{"claude-"},
	})
	if !handled {
		t.Fatal("route not handled")
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body.String())
	}
	listRR, _ := dispatchAdminGroups(t, h, http.MethodGet, "/admin/api/groups", nil)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRR.Code, listRR.Body.String())
	}
	var listed []groupView
	if err := json.Unmarshal(listRR.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "claude" {
		t.Fatalf("listed groups = %+v", listed)
	}
}

func TestChannelGroupCreateDuplicate(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "First", "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Second", "modelPatterns": []string{"claude-"}})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestChannelGroupUpdateMetadata(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPatch, "/admin/api/groups/claude", map[string]any{
		"name":        "Claude Pro",
		"description": "for pro users",
		"enabled":     false,
		"sortOrder":   3,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rr.Code, rr.Body.String())
	}
	var got groupView
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Name != "Claude Pro" || got.Description != "for pro users" || got.Enabled || got.SortOrder != 3 {
		t.Fatalf("patched group wrong: %+v", got)
	}
}

func TestChannelGroupReplaceMembers(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPut, "/admin/api/groups/claude/channels", map[string]any{
		"channels": []map[string]string{
			{"sourceType": "newapi", "channelId": "p:tok-1"},
			{"sourceType": "direct", "channelId": "d-1"},
		},
		"defaultRuntimeChannelId": "p:tok-1",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put members status = %d body=%s", rr.Code, rr.Body.String())
	}
	var got groupView
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.ChannelCount != 2 {
		t.Fatalf("channel count = %d, want 2", got.ChannelCount)
	}
	if got.DefaultRuntimeChannelID != "p:tok-1" {
		t.Fatalf("default channel = %q", got.DefaultRuntimeChannelID)
	}
	// runtime ID 转换：direct: 前缀
	wantRuntime := map[string]string{"p:tok-1": "p:tok-1", "d-1": "direct:d-1"}
	for _, ch := range got.Channels {
		if want := wantRuntime[ch.ChannelID]; ch.RuntimeID != want {
			t.Fatalf("runtime id for %s = %q, want %q", ch.ChannelID, ch.RuntimeID, want)
		}
	}
}

func TestChannelGroupReplaceMembersRejectMissingChannel(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPut, "/admin/api/groups/claude/channels", map[string]any{
		"channels": []map[string]string{
			{"sourceType": "newapi", "channelId": "p:tok-NOPE"},
		},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestChannelGroupReplaceMembersRejectDuplicate(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPut, "/admin/api/groups/claude/channels", map[string]any{
		"channels": []map[string]string{
			{"sourceType": "newapi", "channelId": "p:tok-1"},
			{"sourceType": "newapi", "channelId": "p:tok-1"},
		},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for dup, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestChannelGroupReplaceMembersDefaultMustBeMember(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodPut, "/admin/api/groups/claude/channels", map[string]any{
		"channels": []map[string]string{
			{"sourceType": "newapi", "channelId": "p:tok-1"},
		},
		"defaultRuntimeChannelId": "p:tok-2",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-member default, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestChannelGroupSoftDelete(t *testing.T) {
	h := channelGroupTestSetup(t)
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	rr, _ := dispatchAdminGroups(t, h, http.MethodDelete, "/admin/api/groups/claude", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rr.Code, rr.Body.String())
	}
	listRR, _ := dispatchAdminGroups(t, h, http.MethodGet, "/admin/api/groups", nil)
	var listed []groupView
	_ = json.Unmarshal(listRR.Body.Bytes(), &listed)
	if len(listed) != 0 {
		t.Fatalf("expected no active groups after delete, got %d", len(listed))
	}
	// 软删后仍能拿到（用于审计）
	g, ok := config.GetChannelGroup("claude")
	if !ok || g.DeletedAt == 0 {
		t.Fatalf("soft delete record missing or not marked: %+v", g)
	}
}

func TestChannelGroupSoftDeletePrunesPreferences(t *testing.T) {
	h := channelGroupTestSetup(t)
	// 准备一个有 ChannelPreferences 的 ApiKey
	dispatchAdminGroups(t, h, http.MethodPost, "/admin/api/groups", map[string]any{"id": "claude", "name": "Claude", "enabled": true, "modelPatterns": []string{"claude-"}})
	dispatchAdminGroups(t, h, http.MethodPut, "/admin/api/groups/claude/channels", map[string]any{
		"channels": []map[string]string{{"sourceType": "newapi", "channelId": "p:tok-1"}},
	})
	if err := config.AddApiKey(config.ApiKeyInfo{
		ID:                 "k1",
		ChannelPreferences: map[string]string{"claude": "p:tok-1", "other": "x"},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.DeleteApiKey("k1") })
	dispatchAdminGroups(t, h, http.MethodDelete, "/admin/api/groups/claude", nil)
	key := config.FindApiKeyByID("k1")
	if key == nil {
		t.Fatal("api key gone")
	}
	if _, ok := key.ChannelPreferences["claude"]; ok {
		t.Fatalf("claude preference not pruned: %+v", key.ChannelPreferences)
	}
	if key.ChannelPreferences["other"] != "x" {
		t.Fatalf("unrelated preference dropped: %+v", key.ChannelPreferences)
	}
}
