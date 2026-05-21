package proxy

import (
	"errors"
	"strings"
	"testing"

	"kiro-api-proxy/config"
)

const newAPITestModel = "gpt-5.5"

func newAPITestPricingCache(groupRatio, modelRatio, completionRatio float64) *providerCache {
	return &providerCache{
		Groups: []config.NewAPIGroup{{Name: "vip", Ratio: groupRatio}},
		Models: []config.NewAPIModel{{
			ModelName:       newAPITestModel,
			ModelRatio:      modelRatio,
			CompletionRatio: completionRatio,
			EnableGroups:    []string{"vip"},
		}},
	}
}

func newAPITestRuntimeChannel(t *testing.T, markup, quotaPerUnit, yuanPerUpstream float64) *NewAPIRuntimeChannel {
	t.Helper()
	keyEnc, err := config.EncryptSecret("sk-test-upstream")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	return newNewAPIRuntimeChannel(config.NewAPIChannel{
		ID:              "apijing:tok-1",
		ProviderID:      "apijing",
		Alias:           "特价 GPT",
		UpstreamTokenID: 1,
		UpstreamKeyEnc:  keyEnc,
		GroupName:       "vip",
		Models:          []string{newAPITestModel},
		Markup:          markup,
		SeriesID:        "gpt",
		Enabled:         true,
	}, config.NewAPIProvider{
		ID:                    "apijing",
		BaseURL:               "https://apijing.test",
		QuotaPerUnitDollar:    quotaPerUnit,
		YuanPerUpstreamDollar: yuanPerUpstream,
		Enabled:               true,
	})
}

func withPivotStackDollarsPerYuan(t *testing.T, value float64) {
	t.Helper()
	old := config.GetPivotStackDollarsPerYuan()
	if err := config.UpdatePivotStackDollarsPerYuan(value, false); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan(%v): %v", value, err)
	}
	t.Cleanup(func() {
		_ = config.UpdatePivotStackDollarsPerYuan(old, false)
	})
}

func TestEstimateNewAPIQuotaHappyPath(t *testing.T) {
	cache := newAPITestPricingCache(0.5, 2, 4)
	got, err := EstimateNewAPIQuota(cache, newAPITestModel, "vip", 100, 200)
	if err != nil {
		t.Fatalf("EstimateNewAPIQuota: %v", err)
	}
	if got != 900 {
		t.Fatalf("quota = %d, want 900", got)
	}
}

func TestEstimateNewAPIQuotaMissingModelReturnsError(t *testing.T) {
	cache := newAPITestPricingCache(1, 1, 1)
	got, err := EstimateNewAPIQuota(cache, "missing-model", "vip", 100, 200)
	if !errors.Is(err, ErrSellPriceMissing) {
		t.Fatalf("expected ErrSellPriceMissing, got quota=%d err=%v", got, err)
	}
	if !strings.Contains(err.Error(), "not in upstream pricing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEstimateNewAPIQuotaMissingGroupDefaultsToOne(t *testing.T) {
	cache := newAPITestPricingCache(0.25, 1, 2)
	got, err := EstimateNewAPIQuota(cache, newAPITestModel, "missing-group", 10, 10)
	if err != nil {
		t.Fatalf("EstimateNewAPIQuota: %v", err)
	}
	if got != 30 {
		t.Fatalf("quota = %d, want 30", got)
	}
}

func TestPreAuthorizeNewAPIRequestSnapshotsAllUnits(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 2, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}
	tokenTestAssertFloat(t, res.Markup, 2)
	tokenTestAssertFloat(t, res.QuotaPerUnitDollar, 1000)
	tokenTestAssertFloat(t, res.YuanPerUpstreamDollar, 1)
	tokenTestAssertFloat(t, res.PivotStackDollarsPerYuanSnap, 20)
	tokenTestAssertFloat(t, res.CompletionRatioSnap, 1)
	tokenTestAssertFloat(t, res.ModelRatioSnap, 1)
	tokenTestAssertFloat(t, res.GroupRatioSnap, 1)
	tokenTestAssertFloat(t, res.PrePaidUSD, 4)

	// 改全局 PSDPY，reconcile 应该用 snapshot 不受影响
	if err := config.UpdatePivotStackDollarsPerYuan(10, false); err != nil {
		t.Fatalf("UpdatePivotStackDollarsPerYuan: %v", err)
	}
	paid, gift, err := ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 100})
	if err != nil {
		t.Fatalf("ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 4)
	tokenTestAssertFloat(t, gift, 0)
	afterPaid, _ := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 96)
}

func TestPreAuthorizeNewAPIRequestInsufficientBalance(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)
	beforePaid, beforeGift := tokenTestBalances(t, keyID)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 2, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
	if res != nil {
		t.Fatalf("expected nil reservation, got %#v", res)
	}
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestReconcileNewAPIRequestUnderestimateDeductsDelta(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	paid, gift, err := ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 100, OutputTokens: 100})
	if err != nil {
		t.Fatalf("ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 4)
	tokenTestAssertFloat(t, gift, 0)
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 6)
	tokenTestAssertFloat(t, afterGift, 0)
}

func TestReconcileNewAPIRequestOverestimateRefundsDelta(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 10, 2, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 100)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	paid, gift, err := ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 100})
	if err != nil {
		t.Fatalf("ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 2)
	tokenTestAssertFloat(t, gift, 0)
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 8)
	tokenTestAssertFloat(t, afterGift, 2)
}

func TestReconcileNewAPIRequestInsufficientBalanceForUnderestimate(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 3, 0, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	paid, gift, err := ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 100, OutputTokens: 400})
	if err != nil {
		t.Fatalf("ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 3)
	tokenTestAssertFloat(t, gift, 0)
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 0)
	tokenTestAssertFloat(t, afterGift, 0)
	info := config.FindApiKeyByID(keyID)
	if info == nil {
		t.Fatal("missing key")
	}
	tokenTestAssertFloat(t, info.DebtUSD, 0)
}

func TestRefundNewAPIReservationRestoresFull(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 10, 2, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 100)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	RefundNewAPIReservation(res)
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 10)
	tokenTestAssertFloat(t, afterGift, 2)
}

// TestPreAuthorizeNewAPIRequestZeroQuotaPerUnitDollarFailsClosed 防白嫖：
// provider 单位字段未配 / sync 失败导致 0 时必须拒绝，不能 silent 免费放行。
func TestPreAuthorizeNewAPIRequestZeroQuotaPerUnitDollarFailsClosed(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	beforePaid, beforeGift := tokenTestBalances(t, keyID)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 0, 1) // QuotaPerUnitDollar = 0

	_, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if !errors.Is(err, ErrSellPriceMissing) {
		t.Fatalf("expected ErrSellPriceMissing, got %v", err)
	}
	if !strings.Contains(err.Error(), "QuotaPerUnitDollar not configured") {
		t.Fatalf("error message lacks context: %v", err)
	}
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestPreAuthorizeNewAPIRequestZeroYuanPerUpstreamDollarFailsClosed(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 0) // YuanPerUpstreamDollar = 0

	_, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if !errors.Is(err, ErrSellPriceMissing) {
		t.Fatalf("expected ErrSellPriceMissing, got %v", err)
	}
}

// TestRefundNewAPIReservationIsIdempotent 防御 caller 误重复调用：第二次 refund 应该 noop。
func TestRefundNewAPIReservationIsIdempotent(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 10, 2, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 100)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	RefundNewAPIReservation(res)
	RefundNewAPIReservation(res) // 第二次应该是 noop
	RefundNewAPIReservation(res) // 第三次也是
	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 10) // 原始余额，没被多退
	tokenTestAssertFloat(t, afterGift, 2)
}
