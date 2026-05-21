package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// decodePathID URL-decode admin path 段并 trim 空格；用于 /providers/{id}、
// /newapi/channels/{id}、/direct-channels/{id} 等末段提取。
// 失败时返回原值（让下层 validate 报具体错误）。
func decodePathID(seg string) string {
	if dec, err := url.PathUnescape(seg); err == nil {
		return strings.TrimSpace(dec)
	}
	return strings.TrimSpace(seg)
}

// adminIDPattern 校验 admin 创建的 entity ID（provider / 自定义 channel ID 等）。
// 从前 series/migrate 模块共用同一 regex；v6 删除 series 后保留这个通用形状给 provider 用。
var adminIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// adminAuditActor 从 admin 请求里提取 audit log 的 actor 标识。
// 优先用 sha256(admin_session cookie)[:8]（不可逆，防止日志泄漏完整 session token），
// fallback 用 RemoteAddr 的 host 部分（net.SplitHostPort 处理 IPv6/带端口场景）。
func adminAuditActor(r *http.Request) string {
	if r == nil {
		return "admin"
	}
	if c, err := r.Cookie("admin_session"); err == nil {
		if token := strings.TrimSpace(c.Value); token != "" {
			sum := sha256.Sum256([]byte(token))
			return "admin:" + hex.EncodeToString(sum[:4])
		}
	}
	if host := remoteHost(r.RemoteAddr); host != "" {
		return "admin@" + host
	}
	return "admin"
}

func remoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
