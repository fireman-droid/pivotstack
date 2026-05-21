package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// v6 stage 8：legacy admin URL 301 表测试。
// 验证：方法过滤、query string 保留、所有 16 条 mapping 命中、未命中路径返回 false。

func TestAdminLegacyRedirectsCoverPlanMapping(t *testing.T) {
	cases := map[string]string{
		"/insights":        "/overview?tab=trend",
		"/leaderboard":     "/overview?tab=rank",
		"/series":          "/channels",
		"/providers":       "/channels/newapi",
		"/newapi-channels": "/channels/newapi",
		"/reconcile":       "/channels/reconcile",
		"/apikeys":         "/billing/keys",
		"/codes":           "/billing/codes",
		"/pricing":         "/billing/pricing",
		"/system-unit":     "/billing/unit",
		"/logs":            "/ops/call-logs",
		"/api":             "/ops/api-docs",
		"/settings":        "/system/settings",
		"/stealth":         "/system/experimental?tab=flags",
		"/accounts":        "/channels/direct?type=kiro",
	}
	for src, want := range cases {
		req := httptest.NewRequest(http.MethodGet, src, nil)
		rr := httptest.NewRecorder()
		if !tryLegacyAdminRedirect(rr, req) {
			t.Fatalf("%s: expected redirect", src)
		}
		if rr.Code != http.StatusMovedPermanently {
			t.Fatalf("%s: status=%d, want 301", src, rr.Code)
		}
		if got := rr.Header().Get("Location"); got != want {
			t.Fatalf("%s: Location=%q, want %q", src, got, want)
		}
	}
}

func TestAdminLegacyRedirectsPreservesQuery(t *testing.T) {
	cases := []struct {
		path    string
		raw     string
		wantLoc string
	}{
		{"/apikeys", "id=123", "/billing/keys?id=123"},
		{"/pricing", "model=gpt-4&tab=models", "/billing/pricing?model=gpt-4&tab=models"},
		{"/stealth", "feature=foo", "/system/experimental?tab=flags&feature=foo"},
		{"/accounts", "search=acc-001", "/channels/direct?type=kiro&search=acc-001"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path+"?"+c.raw, nil)
		rr := httptest.NewRecorder()
		if !tryLegacyAdminRedirect(rr, req) {
			t.Fatalf("%s?%s: expected redirect", c.path, c.raw)
		}
		if got := rr.Header().Get("Location"); got != c.wantLoc {
			t.Fatalf("%s?%s: Location=%q, want %q", c.path, c.raw, got, c.wantLoc)
		}
	}
}

func TestAdminLegacyRedirectsHEADAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/logs", nil)
	rr := httptest.NewRecorder()
	if !tryLegacyAdminRedirect(rr, req) {
		t.Fatal("HEAD: expected redirect")
	}
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("HEAD status=%d", rr.Code)
	}
}

func TestAdminLegacyRedirectsRejectsNonGetHead(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/logs", nil)
		rr := httptest.NewRecorder()
		if tryLegacyAdminRedirect(rr, req) {
			t.Fatalf("%s: should not redirect non-GET/HEAD methods", method)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected no write but got status=%d", method, rr.Code)
		}
	}
}

func TestAdminLegacyRedirectsSkipsUnknown(t *testing.T) {
	for _, path := range []string{"/", "/overview", "/billing/keys", "/login", "/random"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		if tryLegacyAdminRedirect(rr, req) {
			t.Fatalf("%s: should NOT be in redirect map", path)
		}
	}
}

// 集成 smoke test：直接打 Handler.ServeHTTP 验证 legacy 路径会进 redirect
// 而 unknown 路径走 SPA fallback。不打 /admin/api/* /v1/* 这些路径，
// 因为 tokenTestHandler 是裸 Handler，没初始化 admin/channel 依赖会 panic。
func TestAdminLegacyRedirectsServeHTTPPrecedence(t *testing.T) {
	h := tokenTestHandler()
	t.Run("legacy redirected", func(t *testing.T) {
		for _, path := range []string{"/insights", "/apikeys", "/api"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusMovedPermanently {
				t.Fatalf("%s: status=%d, want 301", path, rr.Code)
			}
			if rr.Header().Get("Location") == "" {
				t.Fatalf("%s: missing Location header", path)
			}
		}
	})
	t.Run("unknown not redirected", func(t *testing.T) {
		for _, path := range []string{"/overview", "/login", "/random"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code == http.StatusMovedPermanently {
				t.Fatalf("%s: unexpected 301 → %q", path, rr.Header().Get("Location"))
			}
		}
	})
}

func TestAdminLegacyRedirectsCount(t *testing.T) {
	// 15 条 = 原 plan 16 条减去 /channels（v6 后 /channels 自身是 Groups 总览，禁止 redirect）。
	if got := len(legacyAdminRedirects); got != 15 {
		t.Fatalf("expected 15 redirect entries, got %d", got)
	}
}
