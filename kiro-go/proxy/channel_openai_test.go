package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func openAITestRawBody(model string) []byte {
	return []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"max_tokens":8,"stream":false}`, model))
}

func openAITestChannel(baseURL string, mutate func(*config.ChannelConfig)) *OpenAIChannel {
	cfg := config.ChannelConfig{
		ID:      "ch-openai-test",
		Type:    "openai",
		BaseURL: baseURL,
		APIKey:  "real-key",
		Enabled: true,
		Models:  []string{"gpt-5.5"},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return newOpenAIChannel(cfg)
}

func TestOpenAIChannelExecutePassesThroughUpstream4xx(t *testing.T) {
	body := `{"error":{"message":"rate limited"}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("X-RateLimit-Reset", "123")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, body)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	})

	var up *UpstreamHTTPError
	if !errors.As(err, &up) {
		t.Fatalf("expected UpstreamHTTPError, got %T %v", err, err)
	}
	if up.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want 429", up.StatusCode)
	}
	if string(up.Body) != body {
		t.Fatalf("Body = %q, want %q", string(up.Body), body)
	}
	if up.Chargeable {
		t.Fatal("Chargeable = true, want false")
	}
	if up.Header.Get("X-RateLimit-Reset") != "123" {
		t.Fatalf("header not cloned: %#v", up.Header)
	}
}

func TestOpenAIChannelExecutePassesThroughUpstream5xx(t *testing.T) {
	body := `{"error":{"message":"temporarily unavailable"}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, body)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	})

	var up *UpstreamHTTPError
	if !errors.As(err, &up) {
		t.Fatalf("expected UpstreamHTTPError, got %T %v", err, err)
	}
	if up.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", up.StatusCode)
	}
	if string(up.Body) != body {
		t.Fatalf("Body = %q, want %q", string(up.Body), body)
	}
}

func TestOpenAIChannelAppliesModelAliasInRequestBody(t *testing.T) {
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotModel = payload.Model
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"gpt5.5-final","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", func(cfg *config.ChannelConfig) {
		cfg.ModelAliases = map[string]string{"gpt-5.5": "gpt5.5-final"}
	})
	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotModel != "gpt5.5-final" {
		t.Fatalf("upstream model = %q, want gpt5.5-final", gotModel)
	}
}

func TestOpenAIChannelKeepsPublicModelInResult(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"gpt5.5-final","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", func(cfg *config.ChannelConfig) {
		cfg.ModelAliases = map[string]string{"gpt-5.5": "gpt5.5-final"}
	})
	result, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ActualModel != "gpt-5.5" {
		t.Fatalf("ActualModel = %q, want public model gpt-5.5", result.ActualModel)
	}
}

func TestOpenAIChannelExtraHeadersInjected(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Custom")
		fmt.Fprint(w, `{"model":"gpt-5.5","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", func(cfg *config.ChannelConfig) {
		cfg.ExtraHeaders = map[string]string{"X-Custom": "foo"}
	})
	if _, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "foo" {
		t.Fatalf("X-Custom = %q, want foo", got)
	}
}

func TestOpenAIChannelExtraHeadersDenylistBlocksAuthorization(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"model":"gpt-5.5","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", func(cfg *config.ChannelConfig) {
		cfg.APIKey = "real-key"
		cfg.ExtraHeaders = map[string]string{"Authorization": "Bearer evil"}
	})
	if _, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotAuth != "Bearer real-key" {
		t.Fatalf("Authorization = %q, want Bearer real-key", gotAuth)
	}
}

func TestOpenAIChannelExtraHeadersDenylistBlocksHost(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		fmt.Fprint(w, `{"model":"gpt-5.5","choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer upstream.Close()
	serverURL := upstream.URL

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	ch := openAITestChannel(upstream.URL+"/v1", func(cfg *config.ChannelConfig) {
		cfg.ExtraHeaders = map[string]string{"Host": "evil.example"}
	})
	if _, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       openAITestRawBody("gpt-5.5"),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotHost == "evil.example" {
		t.Fatal("Host header was overridden by ExtraHeaders")
	}
	if gotHost != parsed.Host {
		t.Fatalf("Host = %q, want %q", gotHost, parsed.Host)
	}
}

func TestOpenAIChannelHealthCheckSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"gpt5.5-final"}]}`)
		case "/v1/chat/completions":
			fmt.Fprint(w, `{"id":"ok"}`)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	res := ch.HealthCheck(context.Background(), "")
	if !res.Success || !res.ModelsOK || !res.ChatOK {
		t.Fatalf("unexpected health result: %#v", res)
	}
}

func TestOpenAIChannelHealthCheckModelsFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"bad key"}`)
		case "/v1/chat/completions":
			fmt.Fprint(w, `{"id":"ok"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	res := ch.HealthCheck(context.Background(), "")
	if res.Success {
		t.Fatalf("Success = true, want false: %#v", res)
	}
	if res.ModelsOK {
		t.Fatalf("ModelsOK = true, want false: %#v", res)
	}
	if !res.ChatOK {
		t.Fatalf("ChatOK = false, want true: %#v", res)
	}
}

func TestOpenAIChannelHealthCheckChatFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"gpt-5.5"}]}`)
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	res := ch.HealthCheck(context.Background(), "")
	if res.Success {
		t.Fatalf("Success = true, want false: %#v", res)
	}
	if !res.ModelsOK {
		t.Fatalf("ModelsOK = false, want true: %#v", res)
	}
	if res.ChatOK {
		t.Fatalf("ChatOK = true, want false: %#v", res)
	}
}

func TestOpenAIChannelHealthCheckTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer upstream.Close()

	ch := openAITestChannel(upstream.URL+"/v1", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	res := ch.HealthCheck(ctx, "")
	elapsed := time.Since(start)

	if res.Success {
		t.Fatalf("Success = true, want false: %#v", res)
	}
	if !strings.Contains(res.Error, "deadline exceeded") {
		t.Fatalf("Error = %q, want deadline exceeded", res.Error)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout path took %s, want under 2s", elapsed)
	}
}
