package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"kiro-api-proxy/config"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 这些测试覆盖代理（reseller）功能的所有关键不变量：
//   - 创建子 key 的上限/余额校验
//   - TransferBalance 的横向越权防御 + 原子性
//   - RefundChildBalance 删除子 key 时退款
//   - 激活码兑换时的 reseller 折扣放大（普通用户不受影响）
//   - handleUserMe 不暴露 parentKeyId（透明性守护）
//
// TestMain 在 billing_test.go 已经 init config，这里直接复用。

// ============== 测试基础设施 ==============

// makeReseller 创建一个 reseller key 并加入 cfg，返回 *ApiKeyInfo（只读）+ cleanup
func makeReseller(t *testing.T, balance, discount float64, maxChildren int) (*config.ApiKeyInfo, func()) {
	t.Helper()
	id := "test-reseller-" + randomHex(t, 4)
	k := config.ApiKeyInfo{
		ID:               id,
		Key:              "sk-test-" + randomHex(t, 8),
		Plan:             "credit",
		Enabled:          true,
		IsReseller:       true,
		MaxChildKeys:     maxChildren,
		ResellerDiscount: discount,
		Balance:          balance,
		CreatedAt:        time.Now().Unix(),
	}
	if err := config.AddApiKey(k); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	info := config.FindApiKeyByID(id)
	if info == nil {
		t.Fatalf("just-created key not found")
	}
	return info, func() { _ = config.DeleteApiKey(id) }
}

// makeNormalKey 创建普通用户 key（非 reseller，无 parent）
func makeNormalKey(t *testing.T, balance float64) (*config.ApiKeyInfo, func()) {
	t.Helper()
	id := "test-normal-" + randomHex(t, 4)
	k := config.ApiKeyInfo{
		ID:        id,
		Key:       "sk-test-" + randomHex(t, 8),
		Plan:      "credit",
		Enabled:   true,
		Balance:   balance,
		CreatedAt: time.Now().Unix(),
	}
	if err := config.AddApiKey(k); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	info := config.FindApiKeyByID(id)
	return info, func() { _ = config.DeleteApiKey(id) }
}

// makeChild 创建子 key（ParentKeyID 设置好）
func makeChild(t *testing.T, parentID string, balance float64) (*config.ApiKeyInfo, func()) {
	t.Helper()
	id := "test-child-" + randomHex(t, 4)
	k := config.ApiKeyInfo{
		ID:          id,
		Key:         "sk-test-child-" + randomHex(t, 8),
		Plan:        "credit",
		Enabled:     true,
		ParentKeyID: parentID,
		Balance:     balance,
		CreatedAt:   time.Now().Unix(),
	}
	if err := config.AddApiKey(k); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	info := config.FindApiKeyByID(id)
	return info, func() { _ = config.DeleteApiKey(id) }
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

// ============== TransferBalance ==============

func TestTransferBalance_NormalFlow(t *testing.T) {
	parent, cleanupP := makeReseller(t, 100.0, 0, 0)
	defer cleanupP()
	child, cleanupC := makeChild(t, parent.ID, 0)
	defer cleanupC()

	if err := config.TransferBalance(parent.ID, child.ID, 5.0); err != nil {
		t.Fatalf("TransferBalance: %v", err)
	}
	parentAfter := config.FindApiKeyByID(parent.ID)
	childAfter := config.FindApiKeyByID(child.ID)
	if math.Abs(parentAfter.Balance-95.0) > 1e-6 {
		t.Errorf("parent.Balance: got %.4f, want 95.0", parentAfter.Balance)
	}
	if math.Abs(parentAfter.SoldToChildren-5.0) > 1e-6 {
		t.Errorf("parent.SoldToChildren: got %.4f, want 5.0", parentAfter.SoldToChildren)
	}
	if math.Abs(childAfter.Balance-5.0) > 1e-6 {
		t.Errorf("child.Balance: got %.4f, want 5.0", childAfter.Balance)
	}
	if math.Abs(childAfter.TotalRecharged-5.0) > 1e-6 {
		t.Errorf("child.TotalRecharged: got %.4f, want 5.0", childAfter.TotalRecharged)
	}
}

func TestTransferBalance_NotMyChild(t *testing.T) {
	rA, cleanA := makeReseller(t, 100.0, 0, 0)
	defer cleanA()
	rB, cleanB := makeReseller(t, 100.0, 0, 0)
	defer cleanB()
	// child 属于 A
	childA, cleanC := makeChild(t, rA.ID, 0)
	defer cleanC()

	// reseller B 试图给 A 的子 key 转账 → 拒绝
	err := config.TransferBalance(rB.ID, childA.ID, 5.0)
	if err == nil {
		t.Fatal("expected error 'not your child key', got nil")
	}
	if !strings.Contains(err.Error(), "not your child key") {
		t.Errorf("expected 'not your child key', got: %v", err)
	}
	// B 余额不变，A 的子 key 余额不变
	rbAfter := config.FindApiKeyByID(rB.ID)
	if math.Abs(rbAfter.Balance-100.0) > 1e-6 {
		t.Errorf("B.Balance changed unexpectedly: %.4f", rbAfter.Balance)
	}
	cAfter := config.FindApiKeyByID(childA.ID)
	if math.Abs(cAfter.Balance) > 1e-6 {
		t.Errorf("childA.Balance changed unexpectedly: %.4f", cAfter.Balance)
	}
}

func TestTransferBalance_InsufficientFunds(t *testing.T) {
	parent, cleanP := makeReseller(t, 3.0, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()

	err := config.TransferBalance(parent.ID, child.ID, 5.0)
	if err == nil || !strings.Contains(err.Error(), "insufficient") {
		t.Errorf("expected 'insufficient', got: %v", err)
	}
	// 余额不变
	pAfter := config.FindApiKeyByID(parent.ID)
	if math.Abs(pAfter.Balance-3.0) > 1e-6 {
		t.Errorf("parent.Balance changed: %.4f", pAfter.Balance)
	}
	cAfter := config.FindApiKeyByID(child.ID)
	if math.Abs(cAfter.Balance) > 1e-6 {
		t.Errorf("child.Balance changed: %.4f", cAfter.Balance)
	}
}

// v3.3 起 TransferBalance 支持负数（child → parent 反向扣回），
// 但要求 child 余额足够；零金额仍被拒。
func TestTransferBalance_BoundaryAmounts(t *testing.T) {
	parent, cleanP := makeReseller(t, 100, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0) // child 余额 = 0
	defer cleanC()

	// case 1: amount = 0 被拒
	if err := config.TransferBalance(parent.ID, child.ID, 0); err == nil {
		t.Error("expected error for zero amount")
	}

	// case 2: 负数扣回但 child 余额 = 0 → 余额不足拒绝
	if err := config.TransferBalance(parent.ID, child.ID, -5.0); err == nil {
		t.Error("expected error for recall when child balance is 0")
	}
}

// v3.3：双向转账完整 round trip：先充入再扣回。
func TestTransferBalance_BidirectionalRoundTrip(t *testing.T) {
	parent, cleanP := makeReseller(t, 100, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()

	// 1) parent → child: +30
	if err := config.TransferBalance(parent.ID, child.ID, 30); err != nil {
		t.Fatalf("transfer +30 failed: %v", err)
	}
	pAfter1 := config.FindApiKeyByID(parent.ID)
	cAfter1 := config.FindApiKeyByID(child.ID)
	if math.Abs(pAfter1.Balance-70) > 1e-4 || math.Abs(cAfter1.Balance-30) > 1e-4 {
		t.Fatalf("after +30: parent=%.2f child=%.2f (want 70/30)", pAfter1.Balance, cAfter1.Balance)
	}
	if math.Abs(pAfter1.SoldToChildren-30) > 1e-4 {
		t.Errorf("SoldToChildren after +30: got %.2f want 30", pAfter1.SoldToChildren)
	}

	// 2) child → parent: -10（扣回 10）
	if err := config.TransferBalance(parent.ID, child.ID, -10); err != nil {
		t.Fatalf("recall -10 failed: %v", err)
	}
	pAfter2 := config.FindApiKeyByID(parent.ID)
	cAfter2 := config.FindApiKeyByID(child.ID)
	if math.Abs(pAfter2.Balance-80) > 1e-4 || math.Abs(cAfter2.Balance-20) > 1e-4 {
		t.Fatalf("after -10: parent=%.2f child=%.2f (want 80/20)", pAfter2.Balance, cAfter2.Balance)
	}
	// SoldToChildren 修正：30 - 10 = 20
	if math.Abs(pAfter2.SoldToChildren-20) > 1e-4 {
		t.Errorf("SoldToChildren after -10: got %.2f want 20 (修正历史)", pAfter2.SoldToChildren)
	}

	// 3) 全扣回：-20
	if err := config.TransferBalance(parent.ID, child.ID, -20); err != nil {
		t.Fatalf("full recall -20 failed: %v", err)
	}
	pAfter3 := config.FindApiKeyByID(parent.ID)
	cAfter3 := config.FindApiKeyByID(child.ID)
	if math.Abs(pAfter3.Balance-100) > 1e-4 || math.Abs(cAfter3.Balance) > 1e-4 {
		t.Fatalf("after full recall: parent=%.2f child=%.2f (want 100/0)", pAfter3.Balance, cAfter3.Balance)
	}

	// 4) 试图扣回更多（child 余额为 0）→ 拒
	if err := config.TransferBalance(parent.ID, child.ID, -1); err == nil {
		t.Error("expected error when child balance insufficient for further recall")
	}
}

// ============== RefundChildBalance ==============

func TestRefundChildBalance_NormalFlow(t *testing.T) {
	parent, cleanP := makeReseller(t, 50.0, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()

	// 先转账 $10
	_ = config.TransferBalance(parent.ID, child.ID, 10.0)
	// 此时 parent.Balance=40, parent.SoldToChildren=10, child.Balance=10
	// child 消耗了 $3
	cInfo := config.FindApiKeyByID(child.ID)
	cInfo.Balance = 7
	_ = config.UpdateApiKey(child.ID, *cInfo)

	refund, err := config.RefundChildBalance(child.ID)
	if err != nil {
		t.Fatalf("RefundChildBalance: %v", err)
	}
	if math.Abs(refund-7.0) > 1e-6 {
		t.Errorf("refund: got %.4f, want 7.0", refund)
	}
	pAfter := config.FindApiKeyByID(parent.ID)
	// parent.Balance 应该 = 40 + 7 = 47
	if math.Abs(pAfter.Balance-47.0) > 1e-6 {
		t.Errorf("parent.Balance: got %.4f, want 47.0", pAfter.Balance)
	}
	// SoldToChildren = 10 - 7 = 3（修正"已销售"避免负数）
	if math.Abs(pAfter.SoldToChildren-3.0) > 1e-6 {
		t.Errorf("parent.SoldToChildren: got %.4f, want 3.0", pAfter.SoldToChildren)
	}
	cAfter := config.FindApiKeyByID(child.ID)
	if math.Abs(cAfter.Balance) > 1e-6 || math.Abs(cAfter.GiftBalance) > 1e-6 {
		t.Errorf("child balance not zeroed: balance=%.4f gift=%.4f", cAfter.Balance, cAfter.GiftBalance)
	}
}

// ============== Reseller Activation Code（无杠杆，与普通用户一致）==============
//
// 历史背景：
//   - v1/v2 设计有 ResellerDiscount 字段，兑换时按 1/discount 放大 balance（杠杆）。
//   - v3 移除杠杆 —— 让利由 admin 出激活码时手算面值（如客户付 ¥200，admin 出 ¥285 卡）。
//   - 系统层不再做任何自动折扣，reseller 与普通用户兑换激活码语义完全一致。

func TestRedeemActivationCode_ResellerNoLeverage(t *testing.T) {
	// 即使 reseller 上仍有历史 ResellerDiscount=0.5 字段，也不再生效
	reseller, cleanR := makeReseller(t, 0, 0.5, 0)
	defer cleanR()

	code := "TEST-RESELLER-" + randomHex(t, 8)
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "balance", Amount: 10.0, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	if _, err := config.RedeemActivationCode(code, reseller.ID); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}

	rAfter := config.FindApiKeyByID(reseller.ID)
	// v3：reseller 无杠杆，跟普通用户一样 10¥ → $200 face value
	expected := config.VirtualUSDFromCNY(10.0)
	if math.Abs(rAfter.Balance-expected) > 1e-4 {
		t.Errorf("reseller.Balance: got %.4f, want %.4f (v3 无杠杆，杠杆字段被忽略)", rAfter.Balance, expected)
	}
}

func TestRedeemActivationCode_NormalUserNoDiscount(t *testing.T) {
	user, cleanU := makeNormalKey(t, 0)
	defer cleanU()

	code := "TEST-NORMAL-" + randomHex(t, 8)
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "balance", Amount: 10.0, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	if _, err := config.RedeemActivationCode(code, user.ID); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}

	uAfter := config.FindApiKeyByID(user.ID)
	// 普通用户无折扣：10/0.05 = 200
	expected := config.VirtualUSDFromCNY(10.0)
	if math.Abs(uAfter.Balance-expected) > 1e-4 {
		t.Errorf("user.Balance: got %.4f, want %.4f (no discount)", uAfter.Balance, expected)
	}
}

func TestRedeemActivationCode_ResellerWithDiscountZero(t *testing.T) {
	// ResellerDiscount=0 视作无折扣（边界 case）
	reseller, cleanR := makeReseller(t, 0, 0, 0)
	defer cleanR()

	code := "TEST-RES0-" + randomHex(t, 8)
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "balance", Amount: 10.0, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	if _, err := config.RedeemActivationCode(code, reseller.ID); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}

	rAfter := config.FindApiKeyByID(reseller.ID)
	expected := config.VirtualUSDFromCNY(10.0)
	if math.Abs(rAfter.Balance-expected) > 1e-4 {
		t.Errorf("reseller.Balance with discount=0: got %.4f, want %.4f (no discount)", rAfter.Balance, expected)
	}
}

// ============== 天卡（days 类型激活码）回归 ==============

// TestRedeemActivationCode_DaysAddsCorrectSeconds 守卫一个曾出现过的 BUG：
// days 卡 amount=N 应该让 ExpiresAt 增加 N*86400 秒；曾错写成 +N（30 天卡只加 30 秒）。
func TestRedeemActivationCode_DaysAddsCorrectSeconds(t *testing.T) {
	user, cleanU := makeNormalKey(t, 0)
	defer cleanU()

	code := "TEST-DAYS-" + randomHex(t, 8)
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "days", Amount: 30, Tier: "pro",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	before := time.Now().Unix()
	if _, err := config.RedeemActivationCode(code, user.ID); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}
	after := config.FindApiKeyByID(user.ID)

	expected := before + 30*86400
	// 允许 5 秒误差（兑换中花的时间）
	if math.Abs(float64(after.ExpiresAt-expected)) > 5 {
		t.Errorf("ExpiresAt: got %d (≈ +%d sec), want ≈%d (+30 days = +%d sec)",
			after.ExpiresAt, after.ExpiresAt-before, expected, 30*86400)
	}
	if after.Plan != "timed" && after.Plan != "hybrid" {
		t.Errorf("Plan: got %q, want timed or hybrid", after.Plan)
	}
	if after.Tier != "pro" {
		t.Errorf("Tier: got %q, want pro", after.Tier)
	}
}

// TestRedeemActivationCode_TimeUsesSecondsDirectly 守卫另一个曾出现过的 BUG：
// type=time 卡的 amount 单位是"秒"（前端 CodeManagement.vue 已把天/时/分折算成秒），
// 不能再 ×86400。曾 days/time 共用一段代码导致 1 天卡（amount=86400 秒）变成 86400 天。
func TestRedeemActivationCode_TimeUsesSecondsDirectly(t *testing.T) {
	user, cleanU := makeNormalKey(t, 0)
	defer cleanU()

	code := "TEST-TIME-" + randomHex(t, 8)
	// 1 天 = 86400 秒，前端会以 amount=86400 提交
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "time", Amount: 86400, Tier: "pro",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	before := time.Now().Unix()
	if _, err := config.RedeemActivationCode(code, user.ID); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}
	after := config.FindApiKeyByID(user.ID)

	expected := before + 86400 // 86400 秒，不是 86400×86400
	if math.Abs(float64(after.ExpiresAt-expected)) > 5 {
		t.Errorf("ExpiresAt: got %d (≈ +%d sec ≈ %.2f days), want ≈%d (+1 day)",
			after.ExpiresAt, after.ExpiresAt-before, float64(after.ExpiresAt-before)/86400, expected)
	}
}

// TestRedeemActivationCode_DaysStacksOnExistingExpiry 守卫续期场景：
// 已有未过期 ExpiresAt 的 key 再兑天卡，应在原 ExpiresAt 基础上累加，不是从 now 重置。
func TestRedeemActivationCode_DaysStacksOnExistingExpiry(t *testing.T) {
	user, cleanU := makeNormalKey(t, 0)
	defer cleanU()

	// 先给 key 设一个 10 天后到期
	id := user.ID
	beforeKey := config.FindApiKeyByID(id)
	beforeKey.ExpiresAt = time.Now().Unix() + 10*86400
	beforeKey.Plan = "timed"
	if err := config.UpdateApiKey(id, *beforeKey); err != nil {
		t.Fatalf("UpdateApiKey: %v", err)
	}
	beforeKey = config.FindApiKeyByID(id)
	originalExp := beforeKey.ExpiresAt

	code := "TEST-DAYS-STACK-" + randomHex(t, 8)
	if err := config.AddActivationCode(config.ActivationCode{
		Code: code, Type: "days", Amount: 7, Tier: "pro",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	if _, err := config.RedeemActivationCode(code, id); err != nil {
		t.Fatalf("RedeemActivationCode: %v", err)
	}
	after := config.FindApiKeyByID(id)

	expected := originalExp + 7*86400
	if math.Abs(float64(after.ExpiresAt-expected)) > 5 {
		t.Errorf("ExpiresAt stack: got %d, want %d (+7 days from existing %d)",
			after.ExpiresAt, expected, originalExp)
	}
}

// ============== handleUserMe 透明性 ==============

func TestUserMeDoesNotExposeParentKeyID(t *testing.T) {
	parent, cleanP := makeReseller(t, 50, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 5.0)
	defer cleanC()

	h := &Handler{}
	w := httptest.NewRecorder()
	h.handleUserMe(w, child)

	body := w.Body.Bytes()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	if _, exists := resp["parentKeyId"]; exists {
		t.Errorf("CRITICAL: parentKeyId leaked in /user/me response: %v", resp)
	}
	if _, exists := resp["isReseller"]; exists {
		// 子 key 不是 reseller，所以不应该有这个字段
		t.Errorf("isReseller should not appear for non-reseller (got: %v)", resp["isReseller"])
	}
}

func TestUserMeExposesIsResellerForReseller(t *testing.T) {
	reseller, cleanR := makeReseller(t, 100, 0.5, 50)
	defer cleanR()

	h := &Handler{}
	w := httptest.NewRecorder()
	h.handleUserMe(w, reseller)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, _ := resp["isReseller"].(bool); !v {
		t.Errorf("expected isReseller=true for reseller, got: %v", resp["isReseller"])
	}
	if _, exists := resp["maxChildKeys"]; !exists {
		t.Errorf("expected maxChildKeys field for reseller")
	}
	// 即便 reseller 本人，也不暴露 parentKeyId
	if _, exists := resp["parentKeyId"]; exists {
		t.Errorf("CRITICAL: parentKeyId leaked")
	}
}

// ============== GetChildKeys ==============

func TestGetChildKeys_FilterByParent(t *testing.T) {
	pA, cleanA := makeReseller(t, 100, 0, 0)
	defer cleanA()
	pB, cleanB := makeReseller(t, 100, 0, 0)
	defer cleanB()
	c1, clean1 := makeChild(t, pA.ID, 0)
	defer clean1()
	_, clean2 := makeChild(t, pA.ID, 0)
	defer clean2()
	cB, clean3 := makeChild(t, pB.ID, 0)
	defer clean3()

	gotA := config.GetChildKeys(pA.ID)
	if len(gotA) != 2 {
		t.Errorf("pA children: got %d, want 2", len(gotA))
	}
	gotB := config.GetChildKeys(pB.ID)
	if len(gotB) != 1 {
		t.Errorf("pB children: got %d, want 1", len(gotB))
	}
	// 确保 cB 不在 pA 的子列表里
	for _, c := range gotA {
		if c.ID == cB.ID {
			t.Errorf("cross-tenant leak: cB in pA's children")
		}
	}
	// 确保 c1 在 pA 的子列表里
	found := false
	for _, c := range gotA {
		if c.ID == c1.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("c1 missing from pA's children")
	}
}

// ============== IsResellerKey / IsChildKey helper ==============

func TestApiKeyInfo_RoleHelpers(t *testing.T) {
	cases := []struct {
		name           string
		isReseller     bool
		parentKeyID    string
		wantIsReseller bool
		wantIsChild    bool
	}{
		{"normal user", false, "", false, false},
		{"reseller", true, "", true, false},
		{"child key", false, "parent-id", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &config.ApiKeyInfo{IsReseller: tc.isReseller, ParentKeyID: tc.parentKeyID}
			if info.IsResellerKey() != tc.wantIsReseller {
				t.Errorf("IsResellerKey: got %v, want %v", info.IsResellerKey(), tc.wantIsReseller)
			}
			if info.IsChildKey() != tc.wantIsChild {
				t.Errorf("IsChildKey: got %v, want %v", info.IsChildKey(), tc.wantIsChild)
			}
		})
	}
}

// ============== 子 key 不参与活动 ==============

func TestPromotion_ChildKeyExcludedFromWhitelist(t *testing.T) {
	parent, cleanP := makeReseller(t, 100, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()

	// 起一个活动：把子 key 加白名单 + 给它定一个活动价
	restorePricing := testSetPricing(config.PricingConfig{
		ModelPrices: map[string]float64{"claude-opus-4.6": 0.20},
		DefaultProPriceUSD: 0.20, DefaultFreePriceUSD: 0.04,
	}, defaultSupportedModels())
	defer restorePricing()
	now := time.Now().Unix()
	promo := &config.PromotionConfig{
		Enabled: true, Name: "test",
		ModelPrices: map[string]float64{"claude-opus-4.6": 0.05},
		DefaultProPriceUSD: 0.05, DefaultFreePriceUSD: 0.005,
		Whitelist: []string{child.ID}, // 子 key 在白名单
		StartTs: now - 3600, EndTs: now + 3600,
	}
	if err := config.UpdatePromotion(promo, "test"); err != nil {
		t.Fatalf("UpdatePromotion: %v", err)
	}
	defer config.UpdatePromotion(nil, "test")

	// 子 key 即便在白名单也不该享受活动价
	got := ModelPriceUSDForKey(child.ID, "claude-opus-4.6")
	if math.Abs(got-0.20) > 1e-6 {
		t.Errorf("child key in promotion whitelist: got price %.4f, want 0.20 (standard, not 0.05 promo)", got)
	}

	// 反例：reseller 自己（非子 key）在白名单也不享活动价 — 已享 ResellerDiscount，
	// 再叠活动 = 双重套利，所以同样排除。普通直购用户才是活动唯一受益者。
	promo2 := &config.PromotionConfig{
		Enabled: true, Name: "test2",
		ModelPrices: map[string]float64{"claude-opus-4.6": 0.05},
		DefaultProPriceUSD: 0.05, DefaultFreePriceUSD: 0.005,
		Whitelist: []string{parent.ID},
		StartTs: now - 3600, EndTs: now + 3600,
	}
	_ = config.UpdatePromotion(promo2, "test")
	gotParent := ModelPriceUSDForKey(parent.ID, "claude-opus-4.6")
	if math.Abs(gotParent-0.20) > 1e-6 {
		t.Errorf("reseller in whitelist should NOT get promo (anti-arbitrage): got %.4f, want 0.20", gotParent)
	}
}

func TestKeyEligibleForPromotion_ChildKeyAlwaysFalse(t *testing.T) {
	parent, cleanP := makeReseller(t, 100, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()
	// 普通用户作为对照
	user, cleanU := makeNormalKey(t, 100)
	defer cleanU()

	now := time.Now().Unix()
	promo := &config.PromotionConfig{
		Enabled: true, Name: "test",
		Whitelist: []string{child.ID, parent.ID, user.ID}, // 三个都在白名单
		StartTs:   now - 3600, EndTs: now + 3600,
	}
	_ = config.UpdatePromotion(promo, "test")
	defer config.UpdatePromotion(nil, "test")

	// 子 key 被排除（防 reseller 套利）
	if keyEligibleForPromotion(promo, child.ID) {
		t.Error("child key should NOT be eligible even if whitelisted")
	}
	// reseller 也被排除（已享折扣进货，不参与活动）
	if keyEligibleForPromotion(promo, parent.ID) {
		t.Error("reseller should NOT be eligible (already discounted, anti-arbitrage)")
	}
	// 普通用户白名单 → 享受活动
	if !keyEligibleForPromotion(promo, user.ID) {
		t.Error("normal user in whitelist should be eligible")
	}
}

// ============== TransferBalance + Refund 端到端 ==============

func TestTransferAndRefund_RoundTrip(t *testing.T) {
	parent, cleanP := makeReseller(t, 100, 0, 0)
	defer cleanP()
	child, cleanC := makeChild(t, parent.ID, 0)
	defer cleanC()

	// 转 30
	if err := config.TransferBalance(parent.ID, child.ID, 30); err != nil {
		t.Fatalf("transfer 1: %v", err)
	}
	// 再转 20
	if err := config.TransferBalance(parent.ID, child.ID, 20); err != nil {
		t.Fatalf("transfer 2: %v", err)
	}
	// child 用了 15
	cInfo := config.FindApiKeyByID(child.ID)
	cInfo.Balance = 35
	_ = config.UpdateApiKey(child.ID, *cInfo)
	// 退款 35
	refund, err := config.RefundChildBalance(child.ID)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if math.Abs(refund-35) > 1e-6 {
		t.Errorf("refund: got %.4f, want 35", refund)
	}
	pAfter := config.FindApiKeyByID(parent.ID)
	// parent.Balance: 100 - 50 + 35 = 85
	if math.Abs(pAfter.Balance-85) > 1e-6 {
		t.Errorf("parent.Balance: got %.4f, want 85", pAfter.Balance)
	}
	// SoldToChildren: 50 - 35 = 15
	if math.Abs(pAfter.SoldToChildren-15) > 1e-6 {
		t.Errorf("parent.SoldToChildren: got %.4f, want 15", pAfter.SoldToChildren)
	}
}
