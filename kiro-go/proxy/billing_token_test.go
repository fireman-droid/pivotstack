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
	"sync/atomic"
	"testing"
	"time"
)

const (
	tokenTestModel = "claude-sonnet-4.6"
	tokenTestEps   = 1e-9
)

var tokenTestSeq uint64

func tokenTestID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), atomic.AddUint64(&tokenTestSeq, 1))
}

func tokenTestPrice(in, out float64) config.ModelSellPrice {
	return config.ModelSellPrice{InputPerM: in, OutputPerM: out}
}

func tokenTestUsageCost(price config.ModelSellPrice, usage TokenUsage) float64 {
	return float64(usage.InputTokens)*price.InputPerM/1_000_000.0 +
		float64(usage.OutputTokens)*price.OutputPerM/1_000_000.0
}

func tokenTestSetPricingAndChannels(t *testing.T, sell map[string]config.ModelSellPrice, channels []config.ChannelConfig) {
	t.Helper()

	oldPricing := config.GetPricing()
	oldChannels := config.GetChannels()
	t.Cleanup(func() {
		if err := config.UpdateChannels(oldChannels); err != nil {
			t.Errorf("restore channels: %v", err)
		}
		if err := config.UpdatePricing(oldPricing); err != nil {
			t.Errorf("restore pricing: %v", err)
		}
	})

	if err := config.UpdatePricing(config.PricingConfig{
		DefaultProPriceUSD:  0.20,
		DefaultFreePriceUSD: 0.04,
		SellPrices:          sell,
	}); err != nil {
		t.Fatalf("UpdatePricing: %v", err)
	}
	if err := config.UpdateChannels(channels); err != nil {
		t.Fatalf("UpdateChannels: %v", err)
	}
}

func tokenTestAddKey(t *testing.T, plan string, paid, gift float64, expiresAt int64) string {
	t.Helper()

	id := tokenTestID("test-token-key")
	if err := config.AddApiKey(config.ApiKeyInfo{
		ID:          id,
		Key:         "sk-test-" + id,
		Plan:        plan,
		Enabled:     true,
		ExpiresAt:   expiresAt,
		Balance:     paid,
		GiftBalance: gift,
		CreatedAt:   time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	t.Cleanup(func() {
		if err := config.DeleteApiKey(id); err != nil {
			t.Errorf("DeleteApiKey(%s): %v", id, err)
		}
	})
	return id
}

func tokenTestBalances(t *testing.T, keyID string) (float64, float64) {
	t.Helper()

	info := config.FindApiKeyByID(keyID)
	if info == nil {
		t.Fatalf("key %q not found", keyID)
	}
	return info.Balance, info.GiftBalance
}

func tokenTestAssertFloat(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > tokenTestEps {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}

func tokenTestHandler() *Handler {
	return &Handler{
		startTime:      time.Now().Unix(),
		apiKeyStats:    make(map[string]*ApiKeyStats),
		logSubscribers: make(map[chan CallLog]bool),
	}
}

func tokenTestLastCallLog(t *testing.T, h *Handler) CallLog {
	t.Helper()

	h.callLogsMu.RLock()
	defer h.callLogsMu.RUnlock()
	if len(h.callLogs) == 0 {
		t.Fatal("expected at least one call log")
	}
	return h.callLogs[len(h.callLogs)-1]
}

type tokenTestMockChannel struct {
	id       string
	typ      string
	models   []string
	execute  func(context.Context, http.ResponseWriter, ChannelRequest) (*ChannelResult, error)
	protocol Protocol
}

func (m *tokenTestMockChannel) ID() string { return m.id }

func (m *tokenTestMockChannel) Type() string {
	if m.typ == "" {
		return "openai"
	}
	return m.typ
}

func (m *tokenTestMockChannel) Supports(model string) bool {
	if len(m.models) == 0 {
		return true
	}
	return channelSupportsModel(m.models, model)
}

func (m *tokenTestMockChannel) SupportsProtocol(p Protocol) bool {
	if m.protocol == "" {
		return p == ProtocolOpenAI
	}
	return p == m.protocol
}

func (m *tokenTestMockChannel) Execute(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error) {
	return m.execute(ctx, w, req)
}

func TestTokenCostForChannel(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		model     string
		usage     TokenUsage
		sell      map[string]config.ModelSellPrice
		channels  []config.ChannelConfig
		want      float64
		wantErr   error
	}{
		{
			name:  "found global price",
			model: tokenTestModel,
			usage: TokenUsage{InputTokens: 500_000, OutputTokens: 250_000},
			sell: map[string]config.ModelSellPrice{
				tokenTestModel: tokenTestPrice(2, 4),
			},
			want: 2.0,
		},
		{
			name:    "missing price",
			model:   "missing-model",
			usage:   TokenUsage{InputTokens: 1, OutputTokens: 1},
			sell:    map[string]config.ModelSellPrice{},
			wantErr: ErrSellPriceMissing,
		},
		{
			name:      "channel price overrides global",
			channelID: "ch-override",
			model:     tokenTestModel,
			usage:     TokenUsage{InputTokens: 100_000, OutputTokens: 100_000},
			sell: map[string]config.ModelSellPrice{
				tokenTestModel: tokenTestPrice(2, 4),
			},
			channels: []config.ChannelConfig{
				{
					ID:      "ch-override",
					Type:    "openai",
					Enabled: true,
					Models:  []string{tokenTestModel},
					ModelPrices: map[string]config.ModelSellPrice{
						tokenTestModel: tokenTestPrice(10, 20),
					},
				},
			},
			want: 3.0,
		},
		{
			// v4 严格化：channel 指定后不再 fallback 全局，缺价 fail closed。
			// 旧 v3 行为是 fallback global (want: 6.0)；v4 改成 ErrSellPriceMissing
			// 避免同 model 不同渠道被收一样的钱。
			name:      "v4 channel missing model: fail closed, no global fallback",
			channelID: "ch-fallback",
			model:     tokenTestModel,
			usage:     TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
			sell: map[string]config.ModelSellPrice{
				tokenTestModel: tokenTestPrice(2, 4),
			},
			channels: []config.ChannelConfig{
				{
					ID:      "ch-fallback",
					Type:    "openai",
					Enabled: true,
					Models:  []string{tokenTestModel},
					ModelPrices: map[string]config.ModelSellPrice{
						"other-model": tokenTestPrice(99, 99),
					},
				},
			},
			wantErr: ErrSellPriceMissing,
		},
		{
			name:  "zero tokens",
			model: tokenTestModel,
			usage: TokenUsage{},
			sell: map[string]config.ModelSellPrice{
				tokenTestModel: tokenTestPrice(2, 4),
			},
			want: 0,
		},
		{
			name:  "very large token counts",
			model: tokenTestModel,
			usage: TokenUsage{InputTokens: 1_500_000_000, OutputTokens: 500_000_000},
			sell: map[string]config.ModelSellPrice{
				tokenTestModel: tokenTestPrice(1.25, 2.5),
			},
			want: 3125.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenTestSetPricingAndChannels(t, tt.sell, tt.channels)

			got, err := TokenCostForChannel(tt.channelID, tt.model, tt.usage)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("TokenCostForChannel: %v", err)
			}
			tokenTestAssertFloat(t, got, tt.want)
		})
	}
}

func TestPreAuthorizeTokensForChannel_NoKey(t *testing.T) {
	t.Run("empty keyID returns nil reservation", func(t *testing.T) {
		res, err := PreAuthorizeTokensForChannel("", "ch-any", tokenTestModel, TokenUsage{InputTokens: 1, OutputTokens: 1})
		if err != nil {
			t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
		}
		if res != nil {
			t.Fatalf("expected nil reservation, got %#v", res)
		}
	})

	t.Run("non-existent keyID returns nil reservation", func(t *testing.T) {
		res, err := PreAuthorizeTokensForChannel("missing-key", "ch-any", tokenTestModel, TokenUsage{InputTokens: 1, OutputTokens: 1})
		if err != nil {
			t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
		}
		if res != nil {
			t.Fatalf("expected nil reservation, got %#v", res)
		}
	})
}

func TestPreAuthorizeTokensForChannel_FreeDayCard(t *testing.T) {
	price := tokenTestPrice(3, 7)
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)

	keyID := tokenTestAddKey(t, "timed", 5, 2, time.Now().Add(time.Hour).Unix())
	beforePaid, beforeGift := tokenTestBalances(t, keyID)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000})
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}
	if res.Action != "free" {
		t.Fatalf("Action = %q, want free", res.Action)
	}
	tokenTestAssertFloat(t, res.PrePaidUSD, 0)
	tokenTestAssertFloat(t, res.PreGiftUSD, 0)
	tokenTestAssertFloat(t, res.InputPerM, price.InputPerM)
	tokenTestAssertFloat(t, res.OutputPerM, price.OutputPerM)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestPreAuthorizeTokensForChannel_DeductPath(t *testing.T) {
	price := tokenTestPrice(2, 4)
	est := TokenUsage{InputTokens: 1_000_000, OutputTokens: 500_000}
	wantCost := tokenTestUsageCost(price, est)

	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)

	keyID := tokenTestAddKey(t, "credit", 10, 1, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, est)
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}
	if res.Action != "deduct" {
		t.Fatalf("Action = %q, want deduct", res.Action)
	}
	tokenTestAssertFloat(t, res.PrePaidUSD, wantCost)
	tokenTestAssertFloat(t, res.PreGiftUSD, 0)
	tokenTestAssertFloat(t, res.InputPerM, price.InputPerM)
	tokenTestAssertFloat(t, res.OutputPerM, price.OutputPerM)

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10-wantCost)
	tokenTestAssertFloat(t, gift, 1)
}

func TestPreAuthorizeTokensForChannel_InsufficientBalance(t *testing.T) {
	price := tokenTestPrice(2, 4)
	est := TokenUsage{InputTokens: 1_000_000, OutputTokens: 500_000}

	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{
		tokenTestModel: price,
	}, nil)

	keyID := tokenTestAddKey(t, "credit", 1, 0, 0)
	beforePaid, beforeGift := tokenTestBalances(t, keyID)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, est)
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Fatalf("expected insufficient balance error, got %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil reservation, got %#v", res)
	}

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestPreAuthorizeTokensForChannel_MissingPrice(t *testing.T) {
	tokenTestSetPricingAndChannels(t, map[string]config.ModelSellPrice{}, nil)

	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	beforePaid, beforeGift := tokenTestBalances(t, keyID)

	res, err := PreAuthorizeTokensForChannel(keyID, "", tokenTestModel, TokenUsage{InputTokens: 1, OutputTokens: 1})
	if !errors.Is(err, ErrSellPriceMissing) {
		t.Fatalf("expected ErrSellPriceMissing, got %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil reservation, got %#v", res)
	}

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestPreAuthorizeTokensForChannel_ChannelSpecificPriceSnapshot(t *testing.T) {
	globalPrice := tokenTestPrice(2, 4)
	channelPrice := tokenTestPrice(10, 20)
	est := TokenUsage{InputTokens: 100_000, OutputTokens: 100_000}
	wantCost := tokenTestUsageCost(channelPrice, est)

	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{tokenTestModel: globalPrice},
		[]config.ChannelConfig{
			{
				ID:      "ch-priced",
				Type:    "openai",
				Enabled: true,
				Models:  []string{tokenTestModel},
				ModelPrices: map[string]config.ModelSellPrice{
					tokenTestModel: channelPrice,
				},
			},
		},
	)

	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)

	res, err := PreAuthorizeTokensForChannel(keyID, "ch-priced", tokenTestModel, est)
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}
	tokenTestAssertFloat(t, res.InputPerM, channelPrice.InputPerM)
	tokenTestAssertFloat(t, res.OutputPerM, channelPrice.OutputPerM)
	tokenTestAssertFloat(t, res.PrePaidUSD, wantCost)

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10-wantCost)
	tokenTestAssertFloat(t, gift, 0)
}

func TestReconcileTokenUsage_NoOps(t *testing.T) {
	t.Run("nil reservation", func(t *testing.T) {
		paid, gift, err := ReconcileTokenUsage(nil, TokenUsage{InputTokens: 1, OutputTokens: 1})
		if err != nil {
			t.Fatalf("ReconcileTokenUsage: %v", err)
		}
		tokenTestAssertFloat(t, paid, 0)
		tokenTestAssertFloat(t, gift, 0)
	})

	t.Run("empty keyID", func(t *testing.T) {
		paid, gift, err := ReconcileTokenUsage(&TokenReservation{Action: "deduct"}, TokenUsage{InputTokens: 1, OutputTokens: 1})
		if err != nil {
			t.Fatalf("ReconcileTokenUsage: %v", err)
		}
		tokenTestAssertFloat(t, paid, 0)
		tokenTestAssertFloat(t, gift, 0)
	})

	t.Run("free action", func(t *testing.T) {
		keyID := tokenTestAddKey(t, "credit", 3, 4, 0)
		beforePaid, beforeGift := tokenTestBalances(t, keyID)

		paid, gift, err := ReconcileTokenUsage(&TokenReservation{
			KeyID:      keyID,
			Action:     "free",
			PrePaidUSD: 9,
			PreGiftUSD: 8,
			InputPerM:  100,
			OutputPerM: 100,
		}, TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000})
		if err != nil {
			t.Fatalf("ReconcileTokenUsage: %v", err)
		}
		tokenTestAssertFloat(t, paid, 0)
		tokenTestAssertFloat(t, gift, 0)

		afterPaid, afterGift := tokenTestBalances(t, keyID)
		tokenTestAssertFloat(t, afterPaid, beforePaid)
		tokenTestAssertFloat(t, afterGift, beforeGift)
	})
}

func TestReconcileTokenUsage_ExactMatch(t *testing.T) {
	keyID := tokenTestAddKey(t, "credit", 9, 8, 0)
	beforePaid, beforeGift := tokenTestBalances(t, keyID)

	paid, gift, err := ReconcileTokenUsage(&TokenReservation{
		KeyID:      keyID,
		Action:     "deduct",
		InputPerM:  10,
		OutputPerM: 20,
		PrePaidUSD: 1.2,
		PreGiftUSD: 0.3,
	}, TokenUsage{InputTokens: 100_000, OutputTokens: 25_000})
	if err != nil {
		t.Fatalf("ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 1.2)
	tokenTestAssertFloat(t, gift, 0.3)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, beforePaid)
	tokenTestAssertFloat(t, afterGift, beforeGift)
}

func TestReconcileTokenUsage_UnderpaidDeductsAdditionalBalance(t *testing.T) {
	keyID := tokenTestAddKey(t, "credit", 10, 5, 0)

	paid, gift, err := ReconcileTokenUsage(&TokenReservation{
		KeyID:      keyID,
		Action:     "deduct",
		InputPerM:  30,
		OutputPerM: 0,
		PrePaidUSD: 1,
		PreGiftUSD: 0,
	}, TokenUsage{InputTokens: 100_000})
	if err != nil {
		t.Fatalf("ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 3)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 8)
	tokenTestAssertFloat(t, afterGift, 5)
}

func TestReconcileTokenUsage_OverpaidRefundsGiftFirstThenPaid(t *testing.T) {
	keyID := tokenTestAddKey(t, "credit", 5, 5, 0)

	paid, gift, err := ReconcileTokenUsage(&TokenReservation{
		KeyID:      keyID,
		Action:     "deduct",
		InputPerM:  15,
		OutputPerM: 0,
		PrePaidUSD: 2,
		PreGiftUSD: 1,
	}, TokenUsage{InputTokens: 100_000})
	if err != nil {
		t.Fatalf("ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, 1.5)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, 5.5)
	tokenTestAssertFloat(t, afterGift, 6)
}

func TestReconcileTokenUsage_UsesPreAuthPriceSnapshot(t *testing.T) {
	initialBalance := 1000.0
	priceAtPreAuth := tokenTestPrice(10, 20)
	priceAfterAdminChange := tokenTestPrice(1000, 2000)
	usage := TokenUsage{InputTokens: 100_000, OutputTokens: 100_000}
	wantCost := tokenTestUsageCost(priceAtPreAuth, usage)

	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{tokenTestModel: tokenTestPrice(1, 1)},
		[]config.ChannelConfig{
			{
				ID:      "ch-snapshot",
				Type:    "openai",
				Enabled: true,
				Models:  []string{tokenTestModel},
				ModelPrices: map[string]config.ModelSellPrice{
					tokenTestModel: priceAtPreAuth,
				},
			},
		},
	)

	keyID := tokenTestAddKey(t, "credit", initialBalance, 0, 0)
	res, err := PreAuthorizeTokensForChannel(keyID, "ch-snapshot", tokenTestModel, usage)
	if err != nil {
		t.Fatalf("PreAuthorizeTokensForChannel: %v", err)
	}
	tokenTestAssertFloat(t, res.PrePaidUSD, wantCost)

	if err := config.UpdateChannels([]config.ChannelConfig{
		{
			ID:      "ch-snapshot",
			Type:    "openai",
			Enabled: true,
			Models:  []string{tokenTestModel},
			ModelPrices: map[string]config.ModelSellPrice{
				tokenTestModel: priceAfterAdminChange,
			},
		},
	}); err != nil {
		t.Fatalf("UpdateChannels changed price: %v", err)
	}

	paid, gift, err := ReconcileTokenUsage(res, usage)
	if err != nil {
		t.Fatalf("ReconcileTokenUsage: %v", err)
	}
	tokenTestAssertFloat(t, paid, wantCost)
	tokenTestAssertFloat(t, gift, 0)

	afterPaid, afterGift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, afterPaid, initialBalance-wantCost)
	tokenTestAssertFloat(t, afterGift, 0)
}

func TestRefundTokenReservation(t *testing.T) {
	t.Run("nil reservation", func(t *testing.T) {
		RefundTokenReservation(nil)
	})

	t.Run("empty keyID", func(t *testing.T) {
		RefundTokenReservation(&TokenReservation{PrePaidUSD: 1, PreGiftUSD: 1})
	})

	t.Run("zero amounts", func(t *testing.T) {
		keyID := tokenTestAddKey(t, "credit", 1, 2, 0)
		beforePaid, beforeGift := tokenTestBalances(t, keyID)

		RefundTokenReservation(&TokenReservation{KeyID: keyID})

		afterPaid, afterGift := tokenTestBalances(t, keyID)
		tokenTestAssertFloat(t, afterPaid, beforePaid)
		tokenTestAssertFloat(t, afterGift, beforeGift)
	})

	t.Run("normal refund", func(t *testing.T) {
		keyID := tokenTestAddKey(t, "credit", 1, 2, 0)

		RefundTokenReservation(&TokenReservation{
			KeyID:      keyID,
			PrePaidUSD: 3,
			PreGiftUSD: 4,
		})

		afterPaid, afterGift := tokenTestBalances(t, keyID)
		tokenTestAssertFloat(t, afterPaid, 4)
		tokenTestAssertFloat(t, afterGift, 6)
	})
}

func TestKiroExecError(t *testing.T) {
	t.Run("Error and Unwrap", func(t *testing.T) {
		base := errors.New("upstream failed")
		execErr := &KiroExecError{Err: base, Retryable: true, ResponseStarted: false, PayloadKB: 12}

		if got := execErr.Error(); got != base.Error() {
			t.Fatalf("Error() = %q, want %q", got, base.Error())
		}
		if got := execErr.Unwrap(); got != base {
			t.Fatalf("Unwrap() = %v, want %v", got, base)
		}
	})

	t.Run("errors.As extracts KiroExecError from wrapped error", func(t *testing.T) {
		base := errors.New("upstream failed")
		execErr := &KiroExecError{Err: base, ResponseStarted: true}
		wrapped := fmt.Errorf("outer: %w", execErr)

		var got *KiroExecError
		if !errors.As(wrapped, &got) {
			t.Fatal("errors.As returned false")
		}
		if got != execErr {
			t.Fatalf("errors.As extracted %#v, want %#v", got, execErr)
		}
	})

	t.Run("errors.As returns false for non-KiroExecError", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", errors.New("plain error"))

		var got *KiroExecError
		if errors.As(wrapped, &got) {
			t.Fatalf("errors.As returned true with %#v", got)
		}
	})
}

func TestHandleChannelRequest_OpenAIChannelPreAuthExecuteReconcile(t *testing.T) {
	price := tokenTestPrice(100, 200)
	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{tokenTestModel: tokenTestPrice(1, 1)},
		[]config.ChannelConfig{
			{
				ID:      "ch-openai-success",
				Type:    "openai",
				BaseURL: "unused",
				APIKey:  "upstream-key",
				Enabled: true,
				Models:  []string{tokenTestModel},
				ModelPrices: map[string]config.ModelSellPrice{
					tokenTestModel: price,
				},
			},
		},
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected upstream path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-key" {
			t.Errorf("Authorization = %q, want Bearer upstream-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"model":"claude-sonnet-4.6",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":500,"completion_tokens":250,"total_tokens":750}
		}`)
	}))
	defer upstream.Close()

	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	uc := &UserContext{KeyID: keyID}
	h := tokenTestHandler()
	ch := newOpenAIChannel(config.ChannelConfig{
		ID:      "ch-openai-success",
		Type:    "openai",
		BaseURL: upstream.URL,
		APIKey:  "upstream-key",
		Enabled: true,
		Models:  []string{tokenTestModel},
	})

	rawBody := []byte(`{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"hi"}],"max_tokens":1000}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  tokenTestModel,
		EstimatedInput: 1000,
		MaxOutput:      1000,
		RawBody:        rawBody,
	}, uc)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "chatcmpl-test") {
		t.Fatalf("expected upstream response body, got %s", rr.Body.String())
	}

	wantActualCost := tokenTestUsageCost(price, TokenUsage{InputTokens: 500, OutputTokens: 250})
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10-wantActualCost)
	tokenTestAssertFloat(t, gift, 0)
	tokenTestAssertFloat(t, uc.ActualPaidUSD, wantActualCost)
	tokenTestAssertFloat(t, uc.ActualGiftUSD, 0)

	if got := atomic.LoadInt64(&h.totalRequests); got != 1 {
		t.Fatalf("totalRequests = %d, want 1", got)
	}
	if got := atomic.LoadInt64(&h.successRequests); got != 1 {
		t.Fatalf("successRequests = %d, want 1", got)
	}

	log := tokenTestLastCallLog(t, h)
	if log.Status != "success" {
		t.Fatalf("log.Status = %q, want success", log.Status)
	}
	if log.ChannelID != "ch-openai-success" || log.ChannelType != "openai" {
		t.Fatalf("unexpected channel log fields: %#v", log)
	}
	if log.BillingMode != "token" || log.BillingStatus != "paid" {
		t.Fatalf("unexpected billing log fields: %#v", log)
	}
	if log.InputTokens != 500 || log.OutputTokens != 250 {
		t.Fatalf("unexpected token log fields: %#v", log)
	}
	tokenTestAssertFloat(t, log.CostUSD, wantActualCost)
}

func TestHandleChannelRequest_OpenAIChannelExecErrorRefundsPreBody(t *testing.T) {
	price := tokenTestPrice(100, 200)
	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{},
		[]config.ChannelConfig{
			{
				ID:      "ch-openai-fail",
				Type:    "openai",
				Enabled: true,
				Models:  []string{tokenTestModel},
				ModelPrices: map[string]config.ModelSellPrice{
					tokenTestModel: price,
				},
			},
		},
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream exploded", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	h := tokenTestHandler()
	ch := newOpenAIChannel(config.ChannelConfig{
		ID:      "ch-openai-fail",
		Type:    "openai",
		BaseURL: upstream.URL,
		Enabled: true,
		Models:  []string{tokenTestModel},
	})

	rawBody := []byte(`{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  tokenTestModel,
		EstimatedInput: 1000,
		MaxOutput:      1000,
		RawBody:        rawBody,
	}, &UserContext{KeyID: keyID})

	// v4 语义变化：上游 4xx/5xx 通过 UpstreamHTTPError 透传原状态码给客户端，
	// 不再被一律封装成 502 upstream_error。v3 期望 502，v4 期望上游真实 500。
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10)
	tokenTestAssertFloat(t, gift, 0)

	if got := atomic.LoadInt64(&h.totalRequests); got != 1 {
		t.Fatalf("totalRequests = %d, want 1", got)
	}
	if got := atomic.LoadInt64(&h.failedRequests); got != 1 {
		t.Fatalf("failedRequests = %d, want 1", got)
	}

	log := tokenTestLastCallLog(t, h)
	if log.Status != "error" {
		t.Fatalf("log.Status = %q, want error", log.Status)
	}
	if !strings.Contains(log.Error, "upstream HTTP 500") {
		t.Fatalf("log.Error = %q, want upstream HTTP 500", log.Error)
	}
	if log.ChannelID != "ch-openai-fail" || log.BillingMode != "token" {
		t.Fatalf("unexpected error log fields: %#v", log)
	}
}

func TestHandleChannelRequest_MidStreamKiroExecErrorDoesNotRefund(t *testing.T) {
	price := tokenTestPrice(100, 200)
	est := TokenUsage{InputTokens: 1000, OutputTokens: 1000}
	preAuthCost := tokenTestUsageCost(price, est)

	tokenTestSetPricingAndChannels(t,
		map[string]config.ModelSellPrice{},
		[]config.ChannelConfig{
			{
				ID:      "ch-stream-fail",
				Type:    "openai",
				Enabled: true,
				Models:  []string{tokenTestModel},
				ModelPrices: map[string]config.ModelSellPrice{
					tokenTestModel: price,
				},
			},
		},
	)

	keyID := tokenTestAddKey(t, "credit", 10, 0, 0)
	h := tokenTestHandler()
	ch := &tokenTestMockChannel{
		id:     "ch-stream-fail",
		typ:    "openai",
		models: []string{tokenTestModel},
		execute: func(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "data: partial\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return nil, &KiroExecError{
				Err:             errors.New("stream failed after response started"),
				ResponseStarted: true,
			}
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  tokenTestModel,
		Stream:         true,
		EstimatedInput: est.InputTokens,
		MaxOutput:      est.OutputTokens,
		RawBody:        []byte(`{}`),
	}, &UserContext{KeyID: keyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 10-preAuthCost)
	tokenTestAssertFloat(t, gift, 0)

	if got := atomic.LoadInt64(&h.totalRequests); got != 1 {
		t.Fatalf("totalRequests = %d, want 1", got)
	}
	if got := atomic.LoadInt64(&h.failedRequests); got != 1 {
		t.Fatalf("failedRequests = %d, want 1", got)
	}

	log := tokenTestLastCallLog(t, h)
	if log.Status != "error" {
		t.Fatalf("log.Status = %q, want error", log.Status)
	}
	if !strings.Contains(log.Error, "stream failed after response started") {
		t.Fatalf("log.Error = %q", log.Error)
	}
	if log.ChannelID != "ch-stream-fail" || log.BillingMode != "token" {
		t.Fatalf("unexpected error log fields: %#v", log)
	}
}
