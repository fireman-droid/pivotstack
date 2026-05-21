package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/login" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		// new-api 通过 Set-Cookie 设服务端 session（body 不返 access_token）
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "fake-session-value", HttpOnly: true})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"id":           123,
				"username":     "u",
				"display_name": "u",
				"group":        "default",
				"role":         1,
				"status":       1,
			},
		})
	}))
	defer srv.Close()

	got, err := NewNewAPIClient().Login(context.Background(), srv.URL, "u", "p")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got.AccessToken != "fake-session-value" || got.UserID != 123 {
		t.Fatalf("Login() = %+v", got)
	}
	if got.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("ExpiresAt should be in the future, got %d", got.ExpiresAt)
	}
}

func TestLoginRejectedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "bad credentials"})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().Login(context.Background(), srv.URL, "u", "bad")
	if err == nil || !strings.Contains(err.Error(), "bad credentials") {
		t.Fatalf("expected login rejection, got %v", err)
	}
}

func TestLoginNetworkErrorWraps(t *testing.T) {
	_, err := NewNewAPIClient().Login(context.Background(), "http://127.0.0.1:1", "u", "p")
	if err == nil || !strings.Contains(err.Error(), "newapi login") {
		t.Fatalf("expected wrapped network error, got %v", err)
	}
}

func TestFetchPricingPublicNoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("New-Api-User") != "" {
			t.Fatalf("pricing should not send auth headers")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []map[string]any{{
				"model_name":    "gpt-5.5",
				"model_ratio":   2.5,
				"enable_groups": []string{"vip"},
			}},
		})
	}))
	defer srv.Close()

	got, err := NewNewAPIClient().FetchPricing(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchPricing() error = %v", err)
	}
	if len(got) != 1 || got[0].ModelName != "gpt-5.5" || got[0].EnableGroups[0] != "vip" {
		t.Fatalf("FetchPricing() = %+v", got)
	}
}

func TestFetchGroupsRequiresAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Fatalf("New-Api-User = %q", got)
		}
		// new-api 用 cookie session 鉴权，不是 Authorization Bearer
		ck, err := r.Cookie("session")
		if err != nil || ck.Value != "access" {
			t.Fatalf("session cookie = %v, err=%v", ck, err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"vip": map[string]any{"desc": "VIP", "ratio": 0.12},
			},
		})
	}))
	defer srv.Close()

	got, err := NewNewAPIClient().FetchGroups(context.Background(), srv.URL, "access", 42)
	if err != nil {
		t.Fatalf("FetchGroups() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "vip" || got[0].Ratio != 0.12 {
		t.Fatalf("FetchGroups() = %+v", got)
	}
}

func TestFetchTokensPagination(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		page := r.URL.Query().Get("p")
		count := 100
		if page == "3" {
			count = 8
		}
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = map[string]any{"id": i + 1, "name": "tok", "key": "sk-x", "group": "vip", "status": 1}
		}
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
	}))
	defer srv.Close()

	got, err := NewNewAPIClient().FetchAllTokens(context.Background(), srv.URL, "access", 1)
	if err != nil {
		t.Fatalf("FetchAllTokens() error = %v", err)
	}
	if len(got) != 208 {
		t.Fatalf("len(tokens) = %d", len(got))
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestCreateTokenReturnsFullKey(t *testing.T) {
	fullKey := "TFKzX7zx6t5z4nexOwYCoNwo3IQE3bdgrRbd3BYWlyUKa7qh"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Fatalf("New-Api-User = %q", got)
		}
		ck, err := r.Cookie("session")
		if err != nil || ck.Value != "access" {
			t.Fatalf("session cookie = %v, err=%v", ck, err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"id":  908,
				"key": fullKey,
			},
		})
	}))
	defer srv.Close()

	got, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name:           "pivotstack",
		Group:          "vip",
		UnlimitedQuota: true,
		ExpiredTime:    -1,
	})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if got.ID != 908 || got.Key != fullKey {
		t.Fatalf("CreateToken() = %+v", got)
	}
}

func TestCreateTokenRejectsMaskedKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"id":  908,
				"key": "OK7f**********b8QK",
			},
		})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name:        "pivotstack",
		Group:       "vip",
		ExpiredTime: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "masked") {
		t.Fatalf("expected masked key error, got %v", err)
	}
}

func TestCreateTokenSendsCorrectBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": 77, "key": "TFKzX7zx6t5z4nexOwYCoNwo3IQE3bdgrRbd3BYWlyUKa7qh"},
		})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name:               "tok",
		Group:              "vip",
		Models:             []string{"gpt-5.5", "claude-sonnet-4.5"},
		UnlimitedQuota:     false,
		RemainQuota:        12345,
		ExpiredTime:        -1,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		CrossGroupRetry:    true,
		AllowIPs:           "",
	})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	assertBodyValue(t, body, "name", "tok")
	assertBodyValue(t, body, "group", "vip")
	assertBodyValue(t, body, "models", "gpt-5.5,claude-sonnet-4.5")
	assertBodyValue(t, body, "unlimited_quota", false)
	assertBodyValue(t, body, "remain_quota", float64(12345))
	assertBodyValue(t, body, "expired_time", float64(-1))
	assertBodyValue(t, body, "model_limits_enabled", false)
	assertBodyValue(t, body, "model_limits", "")
	assertBodyValue(t, body, "cross_group_retry", true)
	assertBodyValue(t, body, "allow_ips", "")
}

func TestDeleteTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/908" || r.Method != http.MethodDelete {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Fatalf("New-Api-User = %q", got)
		}
		ck, err := r.Cookie("session")
		if err != nil || ck.Value != "access" {
			t.Fatalf("session cookie = %v, err=%v", ck, err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	if err := NewNewAPIClient().DeleteToken(context.Background(), srv.URL, "access", 42, 908); err != nil {
		t.Fatalf("DeleteToken() error = %v", err)
	}
}

func TestDeleteTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "not found"})
	}))
	defer srv.Close()

	err := NewNewAPIClient().DeleteToken(context.Background(), srv.URL, "access", 42, 908)
	if err == nil {
		t.Fatal("expected DeleteToken error")
	}
}

func assertBodyValue(t *testing.T, body map[string]any, key string, want any) {
	t.Helper()
	if got := body[key]; got != want {
		t.Fatalf("body[%q] = %#v, want %#v", key, got, want)
	}
}

// 补 codex audit 要求的边界 case 测试 — Stage 3 完整覆盖。

func TestCreateTokenRejectsSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"message": "group not found",
		})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name: "x", Group: "missing", ExpiredTime: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "group not found") {
		t.Fatalf("expected success=false error, got %v", err)
	}
}

func TestCreateTokenRejectsZeroID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": 0, "key": "TFKzX7zx6t5z4nexOwYCoNwo3IQE3bdgrRbd3BYWlyUKa7qh"},
		})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name: "x", Group: "vip", ExpiredTime: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid id") {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}

func TestCreateTokenRejectsEmptyKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": 1, "key": ""},
		})
	}))
	defer srv.Close()

	_, err := NewNewAPIClient().CreateToken(context.Background(), srv.URL, "access", 42, NewAPICreateTokenRequest{
		Name: "x", Group: "vip", ExpiredTime: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "empty key") {
		t.Fatalf("expected empty key error, got %v", err)
	}
}

func TestDeleteTokenRejectsSuccessFalseHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 上游 HTTP 200 但 success=false（token 不存在等场景）
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"message": "token not found",
		})
	}))
	defer srv.Close()

	err := NewNewAPIClient().DeleteToken(context.Background(), srv.URL, "access", 42, 999)
	if err == nil || !strings.Contains(err.Error(), "token not found") {
		t.Fatalf("expected success=false error, got %v", err)
	}
}
