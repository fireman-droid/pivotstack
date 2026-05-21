package proxy

import (
	"net/http"
	"strings"
	"testing"
)

func TestIsUnsafeUpstreamHeader(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Authorization", true},
		{"Content-Length", true},
		{"Content-Encoding", true},
		{"Host", true},
		{"Connection", true},
		{"Transfer-Encoding", true},
		{"Upgrade", true},
		{"Te", true},
		{"Trailer", true},
		{"Proxy-Connection", true},
		{"Proxy-Authorization", true},
		{"authorization", true},
		{" X-Custom ", false},
		{"Content-Type", false},
		{"Retry-After", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnsafeUpstreamHeader(tt.name); got != tt.want {
				t.Fatalf("isUnsafeUpstreamHeader(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestApplyExtraHeadersBlocksDenylist(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer real")
	applyExtraHeaders(h, map[string]string{
		"Authorization": "Bearer evil",
		"X-Custom":      "foo",
	})

	if got := h.Get("Authorization"); got != "Bearer real" {
		t.Fatalf("Authorization = %q, want Bearer real", got)
	}
	if got := h.Get("X-Custom"); got != "foo" {
		t.Fatalf("X-Custom = %q, want foo", got)
	}
}

func TestApplyExtraHeadersTrimsAndIgnoresEmpty(t *testing.T) {
	h := http.Header{}
	applyExtraHeaders(h, map[string]string{
		" X-Trace ": " abc ",
		"X-Empty":   "   ",
		"   ":       "value",
	})

	if got := h.Get("X-Trace"); got != "abc" {
		t.Fatalf("X-Trace = %q, want abc", got)
	}
	if got := h.Get("X-Empty"); got != "" {
		t.Fatalf("X-Empty = %q, want empty", got)
	}
}

func TestApplyExtraHeadersCanonicalizesHeaderName(t *testing.T) {
	h := http.Header{}
	applyExtraHeaders(h, map[string]string{"x-custom": "foo"})

	if _, ok := h["X-Custom"]; !ok {
		t.Fatalf("expected canonical X-Custom key, got %#v", h)
	}
	if got := h.Get("X-Custom"); got != "foo" {
		t.Fatalf("X-Custom = %q, want foo", got)
	}
}

func TestCopySafeHeadersSkipsSetCookie(t *testing.T) {
	src := http.Header{}
	src.Add("Set-Cookie", "sid=upstream")
	src.Add("X-Trace", "abc")
	src.Add("Content-Encoding", "gzip")

	dst := http.Header{}
	copySafeHeaders(dst, src)

	if got := dst.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie copied: %#v", got)
	}
	if got := dst.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := dst.Get("X-Trace"); got != "abc" {
		t.Fatalf("X-Trace = %q, want abc", got)
	}
}

func TestUpstreamHTTPErrorErrorMethodTruncatesLongBody(t *testing.T) {
	body := strings.Repeat("a", 300)
	err := (&UpstreamHTTPError{
		StatusCode: http.StatusTooManyRequests,
		Body:       []byte(body),
	}).Error()

	wantPreview := strings.Repeat("a", 256) + "..."
	if !strings.Contains(err, "upstream HTTP 429: ") {
		t.Fatalf("missing status prefix: %q", err)
	}
	if !strings.Contains(err, wantPreview) {
		t.Fatalf("error did not contain truncated preview: %q", err)
	}
	if strings.Contains(err, strings.Repeat("a", 257)) {
		t.Fatalf("error was not truncated: %q", err)
	}
}
