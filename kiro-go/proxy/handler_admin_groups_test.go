package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func groupAdminTestConfig(t *testing.T) *Handler {
	t.Helper()
	oldProviders := config.GetNewAPIProviders()
	oldNewAPI := config.GetNewAPIChannels()
	oldDirect := config.GetDirectChannels()
	t.Cleanup(func() {
		_ = config.UpdateNewAPIChannels(nil)
		_ = config.UpdateDirectChannels(nil)
		_ = config.UpdateNewAPIProviders(oldProviders)
		_ = config.UpdateNewAPIChannels(oldNewAPI)
		_ = config.UpdateDirectChannels(oldDirect)
	})
	if err := config.UpdateNewAPIChannels(nil); err != nil {
		t.Fatalf("UpdateNewAPIChannels(nil): %v", err)
	}
	if err := config.UpdateDirectChannels(nil); err != nil {
		t.Fatalf("UpdateDirectChannels(nil): %v", err)
	}
	if err := config.UpdateNewAPIProviders(nil); err != nil {
		t.Fatalf("UpdateNewAPIProviders(nil): %v", err)
	}
	return tokenTestHandler()
}

func getAdminGroups(t *testing.T, h *Handler) []adminGroupView {
	t.Helper()
	// v6: 旧的 /groups 返回 channel 聚合视图迁到 /groups/channels；这里测试的就是候选 channel 池。
	req := httptest.NewRequest(http.MethodGet, "/admin/api/groups/channels", nil)
	rr := httptest.NewRecorder()
	h.apiListGroupCandidateChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out []adminGroupView
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func groupsByAlias(groups []adminGroupView) map[string]adminGroupView {
	out := make(map[string]adminGroupView, len(groups))
	for _, group := range groups {
		out[group.Alias] = group
	}
	return out
}

func TestGroupsAggregateMixesNewAPIAndDirect(t *testing.T) {
	h := groupAdminTestConfig(t)
	if err := config.UpdateNewAPIProviders([]config.NewAPIProvider{{ID: "apijing", Name: "Apijing"}}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "apijing:tok-9", ProviderID: "apijing", Alias: "Zulu NewAPI", UpstreamTokenID: 9, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.2, Enabled: true},
		{ID: "apijing:tok-2", ProviderID: "apijing", Alias: "Alpha NewAPI", UpstreamTokenID: 2, UpstreamKeyEnc: "stub", GroupName: "basic", Markup: 1.5, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "direct-bravo", Type: "openai", Alias: "Bravo Direct", APIKeyEnc: "stub", BaseURL: "https://openai.example.test/v1", Enabled: true},
		{ID: "direct-charlie", Type: "kiro", Alias: "Charlie Direct", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}

	groups := getAdminGroups(t, h)
	if len(groups) != 4 {
		t.Fatalf("groups count = %d", len(groups))
	}
	wantAliases := []string{"Alpha NewAPI", "Bravo Direct", "Charlie Direct", "Zulu NewAPI"}
	for i, want := range wantAliases {
		if groups[i].Alias != want {
			t.Fatalf("alias[%d] = %q, want %q; groups=%+v", i, groups[i].Alias, want, groups)
		}
	}
	byAlias := groupsByAlias(groups)
	if byAlias["Alpha NewAPI"].Route != "/channels/newapi/apijing/channels/apijing:tok-2" {
		t.Fatalf("newapi route wrong: %+v", byAlias["Alpha NewAPI"])
	}
	if byAlias["Bravo Direct"].Route != "/channels/direct/direct-bravo" {
		t.Fatalf("direct route wrong: %+v", byAlias["Bravo Direct"])
	}
}

func TestGroupsAggregateExcludesDeleted(t *testing.T) {
	h := groupAdminTestConfig(t)
	now := time.Now().Unix()
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-1", ProviderID: "p", Alias: "Active NewAPI", UpstreamTokenID: 1, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.1, Enabled: true},
		{ID: "p:tok-2", ProviderID: "p", Alias: "Deleted NewAPI", UpstreamTokenID: 2, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.1, DeletedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "direct-active", Type: "openai", Alias: "Active Direct", APIKeyEnc: "stub", Enabled: true},
		{ID: "direct-deleted", Type: "openai", Alias: "Deleted Direct", APIKeyEnc: "stub", DeletedAt: now},
	}); err != nil {
		t.Fatal(err)
	}

	groups := getAdminGroups(t, h)
	byAlias := groupsByAlias(groups)
	if len(groups) != 2 {
		t.Fatalf("groups count = %d groups=%+v", len(groups), groups)
	}
	if _, ok := byAlias["Deleted NewAPI"]; ok {
		t.Fatalf("deleted newapi appeared: %+v", groups)
	}
	if _, ok := byAlias["Deleted Direct"]; ok {
		t.Fatalf("deleted direct appeared: %+v", groups)
	}
}

func TestGroupsAggregateNewAPIProviderFallback(t *testing.T) {
	h := groupAdminTestConfig(t)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "missing:tok-908", ProviderID: "missing", Alias: "Fallback", UpstreamTokenID: 908, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.2, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}

	groups := getAdminGroups(t, h)
	if len(groups) != 1 {
		t.Fatalf("groups count = %d", len(groups))
	}
	if groups[0].SourceDetail != "missing / tok-908 / vip" {
		t.Fatalf("SourceDetail = %q", groups[0].SourceDetail)
	}
}

func TestGroupsAggregateDirectKiroAndOpenAILabels(t *testing.T) {
	h := groupAdminTestConfig(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "direct-openai", Type: "openai", Alias: "OpenAI Host", APIKeyEnc: "stub", BaseURL: "https://api.example.test/v1", Enabled: true},
		{ID: "direct-openai-empty", Type: "openai", Alias: "OpenAI Empty", APIKeyEnc: "stub", Enabled: true},
		{ID: "direct-kiro", Type: "kiro", Alias: "Kiro Pool", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}

	byAlias := groupsByAlias(getAdminGroups(t, h))
	if byAlias["OpenAI Host"].SourceDetail != "openai / api.example.test" {
		t.Fatalf("openai host detail wrong: %+v", byAlias["OpenAI Host"])
	}
	if byAlias["OpenAI Empty"].SourceDetail != "openai" {
		t.Fatalf("openai empty detail wrong: %+v", byAlias["OpenAI Empty"])
	}
	if byAlias["Kiro Pool"].SourceDetail != "kiro 账号池" {
		t.Fatalf("kiro detail wrong: %+v", byAlias["Kiro Pool"])
	}
}

func TestGroupsAggregateBillingFormat(t *testing.T) {
	h := groupAdminTestConfig(t)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-1", ProviderID: "p", Alias: "Markup", UpstreamTokenID: 1, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.2, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{
			ID: "direct-priced", Type: "openai", Alias: "Priced", APIKeyEnc: "stub", Enabled: true,
			SellPrice: config.DirectSellPrice{Default: config.DirectSellPriceRow{InputPerM: 0.123456, OutputPerM: 9.876543}},
		},
		{ID: "direct-zero", Type: "openai", Alias: "Zero", APIKeyEnc: "stub", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}

	byAlias := groupsByAlias(getAdminGroups(t, h))
	if byAlias["Markup"].Billing != "×1.20 markup" {
		t.Fatalf("markup billing = %q", byAlias["Markup"].Billing)
	}
	if byAlias["Priced"].Billing != "¥0.1235/Mtok in / ¥9.8765/Mtok out" {
		t.Fatalf("direct billing = %q", byAlias["Priced"].Billing)
	}
	if byAlias["Zero"].Billing != "未设置定价" {
		t.Fatalf("zero billing = %q", byAlias["Zero"].Billing)
	}
}

func TestGroupsAggregateStatusEnabledDisabled(t *testing.T) {
	h := groupAdminTestConfig(t)
	if err := config.UpdateNewAPIChannels([]config.NewAPIChannel{
		{ID: "p:tok-1", ProviderID: "p", Alias: "Enabled NewAPI", UpstreamTokenID: 1, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.1, Enabled: true},
		{ID: "p:tok-2", ProviderID: "p", Alias: "Disabled NewAPI", UpstreamTokenID: 2, UpstreamKeyEnc: "stub", GroupName: "vip", Markup: 1.1, Enabled: false},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "direct-enabled", Type: "openai", Alias: "Enabled Direct", APIKeyEnc: "stub", Enabled: true},
		{ID: "direct-disabled", Type: "openai", Alias: "Disabled Direct", APIKeyEnc: "stub", Enabled: false},
	}); err != nil {
		t.Fatal(err)
	}

	byAlias := groupsByAlias(getAdminGroups(t, h))
	if byAlias["Enabled NewAPI"].Status != "enabled" || byAlias["Enabled Direct"].Status != "enabled" {
		t.Fatalf("enabled statuses wrong: %+v", byAlias)
	}
	if byAlias["Disabled NewAPI"].Status != "disabled" || byAlias["Disabled Direct"].Status != "disabled" {
		t.Fatalf("disabled statuses wrong: %+v", byAlias)
	}
}
