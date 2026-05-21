package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

// UpstreamHTTPError 是上游 HTTP 4xx/5xx 的结构化错误。
//
// channel_openai.go 遇到上游错误码时返回此类型而不是 fmt.Errorf，
// handler_channel.go 用 errors.As 解出后把**上游原状态码 + body** 透传给客户端，
// 避免上游 401/429/400 被一律转换成 502 让客户端无法识别真正问题。
//
// Chargeable=false 表示这是上游错误，调用方不应该向用户扣费（refund 预扣）。
type UpstreamHTTPError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Chargeable bool
}

func (e *UpstreamHTTPError) Error() string {
	if e == nil {
		return "upstream error (nil)"
	}
	bodyPreview := strings.TrimSpace(string(e.Body))
	if len(bodyPreview) > 256 {
		bodyPreview = bodyPreview[:256] + "..."
	}
	return fmt.Sprintf("upstream HTTP %d: %s", e.StatusCode, bodyPreview)
}

// isUnsafeUpstreamHeader 判断是否禁止从上游/admin 配置原样透传/注入的头。
// 这些头会被 Go 的 http 客户端自动管理或泄露我们后端的细节。
//
// 适用于两个场景：
//  1. channel.ExtraHeaders（admin 配的）注入上游请求时：不能覆盖 Authorization/Content-Length 等
//  2. UpstreamHTTPError 透传上游响应头给客户端时：不能透传 Connection/Transfer-Encoding 等
func isUnsafeUpstreamHeader(name string) bool {
	switch http.CanonicalHeaderKey(strings.TrimSpace(name)) {
	case "Authorization",
		"Content-Length",
		"Content-Encoding", // 上游已 gzip 解过，我们再写一次 body 不能再带这个
		"Host",
		"Connection",
		"Transfer-Encoding",
		"Upgrade",
		"Te",
		"Trailer",
		"Proxy-Connection",
		"Proxy-Authorization":
		return true
	}
	return false
}

// applyExtraHeaders 把 admin 配置的额外 header 注入到目标 http.Header 上（带 denylist 保护）。
// 空值/键被跳过；被 denylist 阻挡的头被忽略，避免覆盖我们自己设的 Authorization。
func applyExtraHeaders(dst http.Header, extra map[string]string) {
	if dst == nil || len(extra) == 0 {
		return
	}
	for k, v := range extra {
		name := http.CanonicalHeaderKey(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if name == "" || val == "" || isUnsafeUpstreamHeader(name) {
			continue
		}
		dst.Set(name, val)
	}
}

// copySafeHeaders 把上游响应的安全头透传到客户端。
// 跳过 denylist + Set-Cookie（避免上游 cookie 串到我们域名）。
func copySafeHeaders(dst, src http.Header) {
	if dst == nil || src == nil {
		return
	}
	for k, vs := range src {
		name := http.CanonicalHeaderKey(k)
		if isUnsafeUpstreamHeader(name) || name == "Set-Cookie" {
			continue
		}
		for _, v := range vs {
			dst.Add(name, v)
		}
	}
}
