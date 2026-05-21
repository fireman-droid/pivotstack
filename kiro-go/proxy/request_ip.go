package proxy

import (
	"net"
	"net/http"
	"strings"
)

// requestIP 返回 socket 层 client IP，不读 X-Forwarded-For / CF-Connecting-IP。
// 攻击者可伪造这些 header 绕过基于 IP 的限流/abuse 标记，故默认只信任 RemoteAddr。
// 如果未来部署在可信反代（nginx/cloudflare）后面，需要走显式 trusted_proxies allowlist。
func requestIP(r *http.Request) string {
	return clientIP(r)
}

// clientIP 仅暴露 socket 层 IP。其他 IP 相关 helper 必须走 requestIP / clientIP，
// 不允许直接访问 r.RemoteAddr 或 X-Forwarded-For。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
