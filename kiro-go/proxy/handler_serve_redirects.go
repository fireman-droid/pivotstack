package proxy

import (
	"net/http"
)

// legacyAdminRedirects 老 admin URL 301 表。
// 仅命中 exact-path + GET/HEAD。其它方法穿透到 SPA fallback。
// query string 保留（追加在新 path 后），方便深链书签：例如 /apikeys?id=xxx → /billing/keys?id=xxx。
//
// 所有新 view 已落地，全表解封。`/channels` 故意保留不重定向——它现在是 Groups 总览本体。
var legacyAdminRedirects = map[string]string{
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
	// "/channels":     不重定向——/channels 自身就是 Groups 总览
}

// tryLegacyAdminRedirect 命中 redirect map 时写 301 并返回 true，否则 false。
// 调用方应放在 SPA fallback 之前。
func tryLegacyAdminRedirect(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	target, ok := legacyAdminRedirects[r.URL.Path]
	if !ok {
		return false
	}
	target = appendOriginalQuery(target, r.URL.RawQuery)
	http.Redirect(w, r, target, http.StatusMovedPermanently)
	return true
}

// appendOriginalQuery 把请求里原始 query string 追加到 redirect target。
// target 里已经带 query（"?tab=trend"）时用 "&" 拼接；没有时用 "?"。空 query 直接返回 target。
func appendOriginalQuery(target, rawQuery string) string {
	if rawQuery == "" {
		return target
	}
	sep := "?"
	for i := 0; i < len(target); i++ {
		if target[i] == '?' {
			sep = "&"
			break
		}
	}
	return target + sep + rawQuery
}
