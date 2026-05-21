package proxy

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

// 模型 API 跨域 headers（IDE 客户端如 Cursor/Claude Code 直连必须）。
const modelCORSHeaders = "Content-Type, Authorization, X-Api-Key, X-Pivotstack-Channel, X-CSRF-Token, " +
	"anthropic-version, anthropic-beta, x-api-key, x-stainless-os, x-stainless-lang, " +
	"x-stainless-package-version, x-stainless-runtime, x-stainless-runtime-version, x-stainless-arch"

// applyCORS 按路径分级设置 CORS：
//   - /v1/* /chat/completions /messages → 任意 Origin（IDE 跨域必须）
//   - /admin/api/* /user/api/* → 默认同源；ENV PIVOTSTACK_ALLOWED_ORIGINS 配 allowlist 才放行
//   - 其他静态资源 → 不写 CORS（同源由浏览器默认守门）
func applyCORS(w http.ResponseWriter, r *http.Request, path string) {
	if isModelCORSPath(path) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", modelCORSHeaders)
		w.Header().Set("Access-Control-Expose-Headers",
			"x-request-id, x-ratelimit-limit-requests, x-ratelimit-limit-tokens, "+
				"x-ratelimit-remaining-requests, x-ratelimit-remaining-tokens, "+
				"x-ratelimit-reset-requests, x-ratelimit-reset-tokens")
		return
	}
	if !isRestrictedCORSPath(path) {
		return
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && isAllowedRestrictedOrigin(r, origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	// admin/user portal 跨域支持 Bearer / X-Api-Key（同源/allowlist origin 已经守门，header 放行不放大风险）
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, X-CSRF-Token")
}

func isModelCORSPath(path string) bool {
	switch {
	case strings.HasPrefix(path, "/v1/"):
		return true
	case path == "/chat/completions":
		return true
	case path == "/messages" || path == "/messages/count_tokens":
		return true
	case path == "/anthropic/v1/messages":
		return true
	default:
		return false
	}
}

func isRestrictedCORSPath(path string) bool {
	return strings.HasPrefix(path, "/admin/api/") || strings.HasPrefix(path, "/user/api/")
}

func isAllowedRestrictedOrigin(r *http.Request, origin string) bool {
	if isSameOrigin(r, origin) {
		return true
	}
	allowed := os.Getenv("PIVOTSTACK_ALLOWED_ORIGINS")
	if allowed == "" {
		return false
	}
	target := normalizeOrigin(origin)
	if target == "" {
		return false
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if normalizeOrigin(candidate) == target {
			return true
		}
	}
	return false
}

func isSameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}
