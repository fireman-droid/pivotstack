package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-api-proxy/config"
)

func newAPITestHandlerWithCache(t *testing.T, cache *providerCache) *Handler {
	t.Helper()
	h := tokenTestHandler()
	h.newapiManager = NewNewAPIManager(h)
	if cache != nil {
		h.newapiManager.caches.Store("apijing", cache)
	}
	return h
}

func newAPITestUpstreamChannel(t *testing.T, baseURL string) *NewAPIRuntimeChannel {
	t.Helper()
	keyEnc, err := config.EncryptSecret("sk-upstream")
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
		Markup:          1,
		SeriesID:        "gpt",
		Enabled:         true,
	}, config.NewAPIProvider{
		ID:                    "apijing",
		BaseURL:               baseURL,
		QuotaPerUnitDollar:    1000,
		YuanPerUpstreamDollar: 1,
		Enabled:               true,
	})
}

func TestHandleNewAPIChannelRequestEndToEnd(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	var sawAuth atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer sk-upstream" {
			sawAuth.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"chatcmpl-newapi",
			"object":"chat.completion",
			"model":"gpt-5.5",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}
		}`)
	}))
	defer upstream.Close()

	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	h := newAPITestHandlerWithCache(t, newAPITestPricingCache(1, 1, 1))
	ch := newAPITestUpstreamChannel(t, upstream.URL)
	rawBody := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  newAPITestModel,
		EstimatedInput: 100,
		MaxOutput:      100,
		RawBody:        rawBody,
	}, &UserContext{KeyID: keyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !sawAuth.Load() {
		t.Fatal("upstream Authorization header missing")
	}
	if !strings.Contains(rr.Body.String(), "chatcmpl-newapi") {
		t.Fatalf("upstream body not forwarded: %s", rr.Body.String())
	}
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 97)
	tokenTestAssertFloat(t, gift, 0)

	log := tokenTestLastCallLog(t, h)
	if log.Status != "success" {
		t.Fatalf("log.Status = %q", log.Status)
	}
	if log.BillingMode != "newapi" || log.BillingStatus != "estimated" {
		t.Fatalf("unexpected billing log fields: %#v", log)
	}
	if log.ChannelID != "apijing:tok-1" || log.ChannelType != "newapi" {
		t.Fatalf("unexpected channel log fields: %#v", log)
	}
	if log.InputTokens != 100 || log.OutputTokens != 50 {
		t.Fatalf("unexpected usage log fields: %#v", log)
	}
	tokenTestAssertFloat(t, log.CostUSD, 3)
}

func TestHandleNewAPIChannelRequestProviderNotSynced(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	h := newAPITestHandlerWithCache(t, nil)
	ch := newAPITestUpstreamChannel(t, "https://apijing.test")
	rawBody := []byte(`{"model":"gpt-5.5"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  newAPITestModel,
		EstimatedInput: 100,
		MaxOutput:      100,
		RawBody:        rawBody,
	}, &UserContext{KeyID: keyID})

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "channel temporarily unavailable") {
		t.Fatalf("body = %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "provider") || strings.Contains(rr.Body.String(), "synced") {
		t.Fatalf("admin-state leak in error body: %s", rr.Body.String())
	}
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 100)
	tokenTestAssertFloat(t, gift, 0)
}

func TestHandleNewAPIChannelRequestUpstreamErrorRefunds(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"limited"}`)
	}))
	defer upstream.Close()

	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	h := newAPITestHandlerWithCache(t, newAPITestPricingCache(1, 1, 1))
	ch := newAPITestUpstreamChannel(t, upstream.URL)
	rawBody := []byte(`{"model":"gpt-5.5"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  newAPITestModel,
		EstimatedInput: 100,
		MaxOutput:      100,
		RawBody:        rawBody,
	}, &UserContext{KeyID: keyID})

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	paid, gift := tokenTestBalances(t, keyID)
	tokenTestAssertFloat(t, paid, 100)
	tokenTestAssertFloat(t, gift, 0)

	log := tokenTestLastCallLog(t, h)
	if log.Status == "success" {
		t.Fatalf("unexpected success log: %#v", log)
	}
	if log.BillingMode != "newapi" || log.ChannelID != "apijing:tok-1" {
		t.Fatalf("unexpected error log: %#v", log)
	}
}

// TestHandleNewAPIChannelRequestStreamUsageFallback 验证流式上游不返回 usage 时
// 回退到估算 token + UpstreamCredits 用 EstQuota 兜底，避免 UI 看到 0 token / 0 quota。
func TestHandleNewAPIChannelRequestStreamUsageFallback(t *testing.T) {
	newAPITestConfig(t)
	withPivotStackDollarsPerYuan(t, 20)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-stream\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	keyID := tokenTestAddKey(t, "credit", 100, 0, 0)
	h := newAPITestHandlerWithCache(t, newAPITestPricingCache(1, 1, 1))
	ch := newAPITestUpstreamChannel(t, upstream.URL)
	rawBody := []byte(`{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hi"}],"max_tokens":100}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(rawBody)))
	rr := httptest.NewRecorder()

	h.handleChannelRequest(rr, req, ch, &channelDispatch{
		Protocol:       ProtocolOpenAI,
		OriginalModel:  newAPITestModel,
		Stream:         true,
		EstimatedInput: 10,
		MaxOutput:      100,
		RawBody:        rawBody,
	}, &UserContext{KeyID: keyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "chatcmpl-stream") {
		t.Fatalf("stream body not forwarded: %s", rr.Body.String())
	}
	log := tokenTestLastCallLog(t, h)
	if log.Status != "success" || !log.Stream {
		t.Fatalf("log unexpected: %#v", log)
	}
	if !log.UsageEstimated {
		t.Fatal("stream without usage should fall back to estimated=true")
	}
	if log.InputTokens != 10 || log.OutputTokens != 100 {
		t.Fatalf("usage fallback wrong: in=%d out=%d", log.InputTokens, log.OutputTokens)
	}
	// UpstreamCredits 应被 Phase 4a 用 EstQuota 兜底，非 0
	if log.UpstreamCredits == 0 {
		t.Fatalf("UpstreamCredits should be populated with EstQuota, got 0")
	}
}
