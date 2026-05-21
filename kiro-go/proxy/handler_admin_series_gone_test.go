package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// v6 stage 7: /series + /providers/{id}/migrate-manual-channels 删除，返 410。
// 保留一个版本周期让老前端能识别为已废弃而不是 404。

func seriesGoneRouter(t *testing.T) func(path, method string) (int, string) {
	t.Helper()
	h := tokenTestHandler()
	return func(path, method string) (int, string) {
		req := httptest.NewRequest(method, "/admin/api"+path, nil)
		rr := httptest.NewRecorder()
		h.routeAdminAPI(path, rr, req)
		return rr.Code, rr.Body.String()
	}
}

func TestAdminSeriesEndpointsReturn410(t *testing.T) {
	call := seriesGoneRouter(t)
	cases := []struct {
		path, method string
	}{
		{"/series", http.MethodGet},
		{"/series", http.MethodPost},
		{"/series/whatever", http.MethodPut},
		{"/series/whatever", http.MethodDelete},
	}
	for _, c := range cases {
		code, body := call(c.path, c.method)
		if code != http.StatusGone {
			t.Fatalf("%s %s: status=%d body=%s", c.method, c.path, code, body)
		}
		if !strings.Contains(body, "removed in v6") {
			t.Fatalf("%s %s: body missing deprecation hint: %s", c.method, c.path, body)
		}
	}
}

func TestAdminMigrateManualChannelsReturns410(t *testing.T) {
	call := seriesGoneRouter(t)
	code, body := call("/providers/apijing/migrate-manual-channels", http.MethodPost)
	if code != http.StatusGone {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if !strings.Contains(body, "removed in v6") {
		t.Fatalf("body missing deprecation hint: %s", body)
	}
}
