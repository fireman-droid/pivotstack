package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kiro-api-proxy/config"
)

const adminTestPassword = "admin-test-password"

// seedUnitConfigTest 准备一个 admin 密码 hash + 几个 user balances，
// 供 unit-config 端点测试使用。复用 newAPITestConfig 的 cfg reset 行为，
// 但额外灌入 admin 密码（PivotStackUnitChange 校验需要）。
func seedUnitConfigTest(t *testing.T) *Handler {
	t.Helper()
	newAPITestConfig(t)
	// 隔离全局 cfg.ApiKeys —— 其他测试遗留的 keys 会污染 UsersAffected 计数。
	preexistingKeys := config.GetAllApiKeys()
	for _, k := range preexistingKeys {
		_ = config.DeleteApiKey(k.ID)
	}
	t.Cleanup(func() {
		current := config.GetAllApiKeys()
		for _, k := range current {
			_ = config.DeleteApiKey(k.ID)
		}
		for _, k := range preexistingKeys {
			_ = config.AddApiKey(k)
		}
	})

	if err := config.SetPassword(adminTestPassword); err != nil {
		t.Fatal(err)
	}
	if err := config.AddApiKey(config.ApiKeyInfo{ID: "k1", Key: "sk-1", Enabled: true, Plan: "credit", Balance: 100, GiftBalance: 20}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddApiKey(config.ApiKeyInfo{ID: "k2", Key: "sk-2", Enabled: true, Plan: "credit", Balance: 50, GiftBalance: 0}); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdatePivotStackDollarsPerYuan(20, false); err != nil {
		t.Fatal(err)
	}
	return tokenTestHandler()
}

func TestGetSystemUnitConfigReturnsCurrentValuesAndUserTotals(t *testing.T) {
	h := seedUnitConfigTest(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/system/unit-config", nil)
	rr := httptest.NewRecorder()
	h.apiGetSystemUnitConfig(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp unitConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.PivotStackDollarsPerYuan != 20 {
		t.Fatalf("current value = %v", resp.PivotStackDollarsPerYuan)
	}
	if resp.UsersCount != 2 || resp.UsersTotalPaid != 150 || resp.UsersTotalGift != 20 {
		t.Fatalf("user totals wrong: %+v", resp)
	}
}

func TestPostSystemUnitConfigChangesValueWithoutRebalance(t *testing.T) {
	h := seedUnitConfigTest(t)
	body := bytes.NewBufferString(`{"newValue":10,"rebalanceUserBalances":false,"adminPassword":"` + adminTestPassword + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/system/unit-config", body)
	rr := httptest.NewRecorder()
	h.apiPostSystemUnitConfig(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp unitConfigChangeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldValue != 20 || resp.NewValue != 10 || resp.Rebalanced {
		t.Fatalf("response unexpected: %+v", resp)
	}
	if resp.UsersAffected != 0 || resp.PaidBalanceDiff != 0 || resp.GiftBalanceDiff != 0 {
		t.Fatalf("no-rebalance should not touch user balances: %+v", resp)
	}
	if config.GetPivotStackDollarsPerYuan() != 10 {
		t.Fatal("global value not changed")
	}
	got := config.FindApiKeyByID("k1")
	if got == nil || got.Balance != 100 || got.GiftBalance != 20 {
		t.Fatalf("user balance unexpectedly changed: %+v", got)
	}
}

func TestPostSystemUnitConfigRebalanceMaintainsRealYuan(t *testing.T) {
	h := seedUnitConfigTest(t)
	body := bytes.NewBufferString(`{"newValue":10,"rebalanceUserBalances":true,"adminPassword":"` + adminTestPassword + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/system/unit-config", body)
	rr := httptest.NewRecorder()
	h.apiPostSystemUnitConfig(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp unitConfigChangeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.UsersAffected != 2 {
		t.Fatalf("usersAffected = %d", resp.UsersAffected)
	}
	// factor = 10/20 = 0.5，所有 paid 减半 / gift 减半，真¥ 不变
	got := config.FindApiKeyByID("k1")
	if got == nil || got.Balance != 50 || got.GiftBalance != 10 {
		t.Fatalf("k1 balance not rebalanced: %+v", got)
	}
	// diff = sum(after) - sum(before) = (50+25 - 100-50) + (10+0 - 20-0) = -75 / -10
	if resp.PaidBalanceDiff != -75 {
		t.Fatalf("paidBalanceDiff = %v, want -75", resp.PaidBalanceDiff)
	}
	if resp.GiftBalanceDiff != -10 {
		t.Fatalf("giftBalanceDiff = %v, want -10", resp.GiftBalanceDiff)
	}
}

func TestPostSystemUnitConfigRequiresAdminPassword(t *testing.T) {
	h := seedUnitConfigTest(t)
	body := bytes.NewBufferString(`{"newValue":10,"rebalanceUserBalances":false}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/system/unit-config", body)
	rr := httptest.NewRecorder()
	h.apiPostSystemUnitConfig(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if config.GetPivotStackDollarsPerYuan() != 20 {
		t.Fatal("global value should not change on auth failure")
	}
}

func TestPostSystemUnitConfigRejectsWrongPassword(t *testing.T) {
	h := seedUnitConfigTest(t)
	body := bytes.NewBufferString(`{"newValue":10,"rebalanceUserBalances":false,"adminPassword":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/system/unit-config", body)
	rr := httptest.NewRecorder()
	h.apiPostSystemUnitConfig(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPostSystemUnitConfigRejectsZeroOrNegative(t *testing.T) {
	h := seedUnitConfigTest(t)
	for _, payload := range []string{`{"newValue":0,"adminPassword":"` + adminTestPassword + `"}`, `{"newValue":-1,"adminPassword":"` + adminTestPassword + `"}`} {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/system/unit-config", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		h.apiPostSystemUnitConfig(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("payload %s status = %d body=%s", payload, rr.Code, rr.Body.String())
		}
	}
}
