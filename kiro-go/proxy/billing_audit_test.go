package proxy

import (
	"context"
	"errors"
	"fmt"
	"kiro-api-proxy/config"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBillingAudit_GiftRemainderSplitDoesNotOverdraft(t *testing.T) {
	price := tokenTestPrice(3, 0)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 1, 5, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	tokenTestAssertFloat(t, res.PrePaidUSD, 1)
	tokenTestAssertFloat(t, res.PreGiftUSD, 2)

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 0)
	tokenTestAssertFloat(t, gift, 3)
}

func TestBillingAudit_ConcurrentTokenReservationsCannotOverspend(t *testing.T) {
	price := tokenTestPrice(1, 0)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)

	var wg sync.WaitGroup
	var success int64
	var insufficient int64
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
			if err == nil && res != nil {
				atomic.AddInt64(&success, 1)
				return
			}
			if err != nil && strings.Contains(err.Error(), "insufficient balance") {
				atomic.AddInt64(&insufficient, 1)
				return
			}
			t.Errorf("unexpected reservation result: res=%#v err=%v", res, err)
		}()
	}
	wg.Wait()

	if success != 10 {
		t.Fatalf("success reservations = %d, want 10", success)
	}
	if insufficient != 15 {
		t.Fatalf("insufficient reservations = %d, want 15", insufficient)
	}
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 0)
	tokenTestAssertFloat(t, gift, 0)
}

func TestBillingAudit_TokenUnderpaidInsufficientBalanceDoesNotOverdraft(t *testing.T) {
	price := tokenTestPrice(1, 0)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	paid, gift, err := ReconcileTokenUsage(res, TokenUsage{InputTokens: 3_000_000})
	if err != nil {
		t.Fatalf("ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 1)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 0)
	tokenTestAssertFloat(t, afterGift, 0)
}

func TestBillingAudit_TokenRefundReservationIsIdempotent(t *testing.T) {
	price := tokenTestPrice(2, 0)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	RefundTokenReservation(res)
	RefundTokenReservation(res)

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10)
	tokenTestAssertFloat(t, gift, 0)
}

func TestBillingAudit_TokenReconcileSameReservationIsIdempotent(t *testing.T) {
	price := tokenTestPrice(3, 0)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	paid, gift, err := ReconcileTokenUsage(res, TokenUsage{InputTokens: 2_000_000})
	if err != nil {
		t.Fatalf("first ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 6)
	tokenTestAssertFloat(t, gift, 0)

	paid, gift, err = ReconcileTokenUsage(res, TokenUsage{InputTokens: 2_000_000})
	if err != nil {
		t.Fatalf("second ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 6)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 4)
	tokenTestAssertFloat(t, afterGift, 0)
}

func TestBillingAudit_NewAPIReconcileSameReservationIsIdempotent(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	cache := newAPITestPricingCache(1, 1, 1)
	ch := newAPITestRuntimeChannel(t, 1, 1000, 1)

	res, err := PreAuthorizeNewAPIRequest(keyID, ch, cache, newAPITestModel, 100, 0)
	if err != nil {
		t.Fatalf("PreAuthorizeNewAPIRequest: %v", err)
	}
	paid, gift, err := ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 200})
	if err != nil {
		t.Fatalf("first ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 4)
	tokenTestAssertFloat(t, gift, 0)

	paid, gift, err = ReconcileNewAPIRequest(res, TokenUsage{InputTokens: 200})
	if err != nil {
		t.Fatalf("second ReconcileNewAPIRequest: %v", err)
	}
	tokenTestAssertFloat(t, paid, 4)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 6)
	tokenTestAssertFloat(t, afterGift, 0)
}

func TestBillingAudit_ChannelPanicAfterPreauthRefundsReservation(t *testing.T) {
	price := tokenTestPrice(2, 0)
	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{},
		[]config.ChannelConfig{{
			ID:      "panic-channel",
			Type:    "openai",
			Enabled: true,
			Models:  []string{tokenTestModel},
			ModelPrices: map[string]config.ModelSellPrice{
				tokenTestModel: price,
			},
		}},
	)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	h := tokenTestHandler()
	ch := &tokenTestMockChannel{
		id:     "panic-channel",
		typ:    "openai",
		models: []string{tokenTestModel},
		execute: func(context.Context, http.ResponseWriter, ChannelRequest) (*ChannelResult, error) {
			panic("upstream adapter panic")
		},
	}

	func() {
		defer func() {
			_ = recover()
		}()
		h.handleChannelRequest(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)),
			ch,
			&channelDispatch{
				Protocol:       ProtocolOpenAI,
				OriginalModel:  tokenTestModel,
				EstimatedInput: 1_000_000,
				MaxOutput:      0,
				RawBody:        []byte(`{}`),
			},
			&UserContext{KeyID: keyID},
		)
	}()

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10)
	tokenTestAssertFloat(t, gift, 0)
}

func TestBillingAudit_NegativeTokenPriceCannotIncreaseBalance(t *testing.T) {
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: tokenTestPrice(-1, 0),
	}, nil)
	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000})
	if err == nil {
		t.Fatalf("expected negative price to fail closed, got reservation %#v", res)
	}

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10)
	tokenTestAssertFloat(t, gift, 0)
}

func TestBillingAudit_NewAPISmallQuotaCumulativeDriftStaysBounded(t *testing.T) {
	const calls = 10_000
	const perCall = 1
	want := float64(calls*perCall) / 500_000 * 1 * 20 * 1

	var got float64
	for i := 0; i < calls; i++ {
		got += QuotaToPivotDollars(perCall, 500_000, 1, 20, 1)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("cumulative quota drift = %.12f, want %.12f", got, want)
	}
}

func TestBillingAudit_BillingAmountMixedModesIgnoresUpstreamQuota(t *testing.T) {
	tests := []struct {
		name string
		log  CallLog
		want float64
	}{
		{
			name: "token uses charged amount over paid-only cost",
			log:  CallLog{BillingMode: "token", ChargedUSD: 0.25, CostUSD: 0.10, Credits: 9, UpstreamCredits: 500_000},
			want: 0.25,
		},
		{
			name: "old token log falls back to cost usd and never upstream quota",
			log:  CallLog{BillingMode: "token", CostUSD: 0.10, UpstreamCredits: 500_000},
			want: 0.10,
		},
		{
			name: "newapi without booked cost ignores upstream quota",
			log:  CallLog{BillingMode: "newapi", UpstreamCredits: 500_000},
			want: 0,
		},
		{
			name: "legacy without usd fields falls back to credits",
			log:  CallLog{BillingMode: "legacy_credits", Credits: 3, UpstreamCredits: 500_000},
			want: 3,
		},
		{
			name: "empty billing mode keeps historical legacy credits fallback",
			log:  CallLog{Credits: 2, UpstreamCredits: 500_000},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenTestAssertFloat(t, billingAmount(tt.log), tt.want)
		})
	}
}

func TestBillingAudit_LegacyCallLogChargedUSDUsesPaidPlusGiftUSD(t *testing.T) {
	h := tokenTestHandler()
	uc := &UserContext{
		KeyID:         tokenTestAddKey(t, "credit", 0, 0, 0),
		ActualPaidUSD: 1.25,
		ActualGiftUSD: 0.75,
	}

	h.addCallLogWithKey(
		"Claude",
		tokenTestModel,
		tokenTestModel,
		"acct",
		"PRO",
		100,
		200,
		false,
		10,
		10,
		"",
		"",
		"stop",
		"req-legacy-charged",
		123,
		uc,
	)

	log := tokenTestLastCallLog(t, h)
	tokenTestAssertFloat(t, log.CostUSD, 1.25)
	tokenTestAssertFloat(t, log.ChargedUSD, 2.0)
}

func TestBillingAudit_ConcurrentRedeemActivationCodeSingleUse(t *testing.T) {
	keyID := tokenTestAddKey(t, "credit", 0, 0, 0)
	code := tokenTestID("AUDIT-CODE")
	if err := config.AddActivationCode(config.ActivationCode{
		Code:   code,
		Type:   "balance",
		Amount: 10,
	}); err != nil {
		t.Fatalf("AddActivationCode: %v", err)
	}
	t.Cleanup(func() {
		_ = config.DeleteActivationCode(code)
	})

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := config.RedeemActivationCode(code, keyID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	var successes, failures int
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, fmt.Errorf("activation code not found")) {
			failures++
			continue
		}
		failures++
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("successes=%d failures=%d, want 1/1", successes, failures)
	}

	paid, _ := tokenTestBalances(t, keyID)
	if paid <= 0 {
		t.Fatalf("paid balance after redemption = %v, want > 0", paid)
	}
	for _, ac := range config.GetActivationCodes() {
		if ac.Code == code {
			t.Fatalf("activation code %q still exists after redemption", code)
		}
	}
}
