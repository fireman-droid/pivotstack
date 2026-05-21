package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-api-proxy/config"
)

func TestNewAPIChannelExecuteSuccessNonStream(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, err := config.EncryptSecret("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	var sawAuth, sawBody atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer sk-secret" {
			sawAuth.Store(true)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"model":"gpt-5.5"`) {
			sawBody.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","model":"upstream","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":4,"completion_tokens":7}}`))
	}))
	defer srv.Close()

	ch := newNewAPIRuntimeChannel(config.NewAPIChannel{
		ID:             "apijing:tok-1",
		ProviderID:     "apijing",
		UpstreamKeyEnc: keyEnc,
		Models:         []string{"gpt-5.5"},
	}, config.NewAPIProvider{ID: "apijing", BaseURL: srv.URL, Enabled: true})

	rr := httptest.NewRecorder()
	result, err := ch.Execute(context.Background(), rr, ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !sawAuth.Load() {
		t.Fatal("Authorization header was not forwarded")
	}
	if !sawBody.Load() {
		t.Fatal("request body was not forwarded")
	}
	if result.ChannelID != "apijing:tok-1" || result.ChannelType != "newapi" || result.Account != "apijing" {
		t.Fatalf("bad result identity: %+v", result)
	}
	if result.ActualModel != "gpt-5.5" || result.InputTokens != 4 || result.OutputTokens != 7 || result.StopReason != "stop" || result.UsageEstimated {
		t.Fatalf("bad result usage: %+v", result)
	}
	if !strings.Contains(rr.Body.String(), `"content":"ok"`) {
		t.Fatalf("response body not forwarded: %s", rr.Body.String())
	}
}

func TestNewAPIChannelExecuteUpstreamError(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-secret")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "rate")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"limited"}`))
	}))
	defer srv.Close()

	ch := newNewAPIRuntimeChannel(config.NewAPIChannel{
		ID:             "apijing:tok-1",
		ProviderID:     "apijing",
		UpstreamKeyEnc: keyEnc,
		Models:         []string{"gpt-5.5"},
	}, config.NewAPIProvider{ID: "apijing", BaseURL: srv.URL})

	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       []byte(`{"model":"gpt-5.5"}`),
	})
	var up *UpstreamHTTPError
	if !errors.As(err, &up) {
		t.Fatalf("expected UpstreamHTTPError, got %T %v", err, err)
	}
	if up.StatusCode != http.StatusTooManyRequests || up.Chargeable {
		t.Fatalf("bad upstream error: %+v", up)
	}
	if string(up.Body) != `{"error":"limited"}` {
		t.Fatalf("body = %s", string(up.Body))
	}
}

func TestNewAPIChannelExecuteStream(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-secret")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	ch := newNewAPIRuntimeChannel(config.NewAPIChannel{
		ID:             "apijing:tok-1",
		ProviderID:     "apijing",
		UpstreamKeyEnc: keyEnc,
		Models:         []string{"gpt-5.5"},
	}, config.NewAPIProvider{ID: "apijing", BaseURL: srv.URL})

	rr := httptest.NewRecorder()
	result, err := ch.Execute(context.Background(), rr, ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		Stream:        true,
		RawBody:       []byte(`{"model":"gpt-5.5","stream":true}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.InputTokens != 3 || result.OutputTokens != 5 || result.StopReason != "stop" || result.UsageEstimated {
		t.Fatalf("bad stream result: %+v", result)
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]") {
		t.Fatalf("stream not forwarded: %s", rr.Body.String())
	}
}

func TestNewAPIChannelExecuteClaudeProtocolRejected(t *testing.T) {
	newAPITestConfig(t)
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
	}))
	defer srv.Close()
	keyEnc, _ := config.EncryptSecret("sk-secret")
	ch := newNewAPIRuntimeChannel(config.NewAPIChannel{ID: "c", UpstreamKeyEnc: keyEnc}, config.NewAPIProvider{ID: "p", BaseURL: srv.URL})

	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{Protocol: ProtocolClaude})
	if err == nil {
		t.Fatal("expected unsupported protocol error")
	}
	if called.Load() {
		t.Fatal("upstream was called for unsupported protocol")
	}
}

// TestNewAPIChannelExecuteRedactsSecretInUpstreamError 防御坏 / 恶意上游
// 把 Authorization 原样 echo 回响应 body，导致客户端/日志拿到真实 sk-* key。
func TestNewAPIChannelExecuteRedactsSecretInUpstreamError(t *testing.T) {
	newAPITestConfig(t)
	keyEnc, _ := config.EncryptSecret("sk-secret")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized","auth":"Bearer sk-secret","echo":"sk-secret"}`))
	}))
	defer srv.Close()
	ch := newNewAPIRuntimeChannel(config.NewAPIChannel{ID: "c", UpstreamKeyEnc: keyEnc, Models: []string{"gpt-5.5"}}, config.NewAPIProvider{ID: "p", BaseURL: srv.URL})
	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       []byte(`{"model":"gpt-5.5"}`),
	})
	var up *UpstreamHTTPError
	if !errors.As(err, &up) {
		t.Fatalf("expected UpstreamHTTPError, got %T %v", err, err)
	}
	if strings.Contains(string(up.Body), "sk-secret") {
		t.Fatalf("upstream key leaked into body: %s", string(up.Body))
	}
	if !strings.Contains(string(up.Body), "[REDACTED]") {
		t.Fatalf("redaction marker missing in body: %s", string(up.Body))
	}
}

// TestNewAPIChannelExecuteDecryptError 上游 key 加密 blob 损坏时返回错误且消息不含 blob。
func TestNewAPIChannelExecuteDecryptError(t *testing.T) {
	newAPITestConfig(t)
	ch := newNewAPIRuntimeChannel(
		config.NewAPIChannel{ID: "c", UpstreamKeyEnc: "v1:gcm:not-valid-base64!@#"},
		config.NewAPIProvider{ID: "p", BaseURL: "http://127.0.0.1:1"},
	)
	_, err := ch.Execute(context.Background(), httptest.NewRecorder(), ChannelRequest{
		Protocol:      ProtocolOpenAI,
		OriginalModel: "gpt-5.5",
		RawBody:       []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected decrypt error")
	}
	if strings.Contains(err.Error(), "not-valid-base64") {
		t.Fatalf("decrypt error leaked encrypted blob: %v", err)
	}
}
