package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ==================== 管理 API ====================
//
// 鉴权模型（v2 之后）：
//   - URL 显式拒绝 ?password=（旧版漏洞，无条件 401）
//   - POST /login            → apiAdminLogin（不要 session；走 IP 速率限制）
//   - GET  /sse/*            → 一次性 SSE token 验证（5min TTL，用过即焚）
//   - 其余                    → 必须带 admin_session cookie；unsafe method 还要 X-CSRF-Token
//
// 旧的「明文密码 header / cookie / query」三路兜底已全部移除。
func (h *Handler) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/api")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.URL.Query().Has("password") {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "password query is forbidden"})
		return
	}

	// /login 不要求 session，但要过 IP 速率限制
	if path == "/login" && r.Method == http.MethodPost {
		h.apiAdminLogin(w, r)
		return
	}

	// SSE 流：走一次性 token（/sse/token 自身仍走 session 分支）
	if strings.HasPrefix(path, "/sse/") && path != "/sse/token" {
		r2, ok := h.requireSSEToken(w, r, path)
		if !ok {
			return
		}
		h.routeAdminAPI(path, w, r2)
		return
	}

	sess, ok := h.requireAdminSession(w, r)
	if !ok {
		return
	}

	if isUnsafeMethod(r.Method) && !h.validateAdminCSRF(r, sess) {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "CSRF token required"})
		return
	}

	switch {
	case path == "/session" && r.Method == http.MethodGet:
		h.apiAdminSession(w, r, sess)
		return
	case path == "/logout" && r.Method == http.MethodPost:
		h.apiAdminLogout(w, r, sess)
		return
	case path == "/password" && r.Method == http.MethodPost:
		h.apiChangeAdminPassword(w, r, sess)
		return
	case path == "/sse/token" && r.Method == http.MethodPost:
		h.apiCreateSSEToken(w, r, sess)
		return
	}

	h.routeAdminAPI(path, w, r)
}

// routeAdminAPI 路由所有已鉴权的常规 admin endpoint。
// 调用方负责确保鉴权 + CSRF 已通过。
//
// 实现：按域链式调度。每个 routeAdmin<Domain> 返回 true 表示本域已处理；
// 全部返回 false → 404。这样主调度 ≤20 行、每个子函数 ≤80 行，
// 符合 v6 plan §0 函数体硬约束。
func (h *Handler) routeAdminAPI(path string, w http.ResponseWriter, r *http.Request) {
	if h.routeAdminAccountsAndAuth(path, w, r) {
		return
	}
	if h.routeAdminSystem(path, w, r) {
		return
	}
	if h.routeAdminApiKeysAndCodes(path, w, r) {
		return
	}
	if h.routeAdminPricingAndProfit(path, w, r) {
		return
	}
	if h.routeAdminProviders(path, w, r) {
		return
	}
	if h.routeAdminNewAPIChannels(path, w, r) {
		return
	}
	if h.routeAdminDirectChannels(path, w, r) {
		return
	}
	if h.routeAdminGroups(path, w, r) {
		return
	}
	if h.routeAdminLegacyChannels(path, w, r) {
		return
	}
	if h.routeAdminOpsAndInsights(path, w, r) {
		return
	}
	if h.routeAdminNotifications(path, w, r) {
		return
	}
	if h.routeAdminUsers(path, w, r) {
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
}

// /accounts/* + /auth/*：kiro 账号池 + SSO/凭证导入。
func (h *Handler) routeAdminAccountsAndAuth(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/accounts" && r.Method == "GET":
		h.apiGetAccounts(w, r)
	case path == "/accounts" && r.Method == "POST":
		h.apiAddAccount(w, r)
	case path == "/accounts/batch" && r.Method == "POST":
		h.apiBatchAccounts(w, r)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/refresh") && r.Method == "POST":
		h.apiRefreshAccount(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/refresh"))
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/models") && r.Method == "GET":
		h.apiGetAccountModels(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/models"))
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/full") && r.Method == "GET":
		h.apiGetAccountFull(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/full"))
	case strings.HasPrefix(path, "/accounts/") && r.Method == "DELETE":
		h.apiDeleteAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case strings.HasPrefix(path, "/accounts/") && r.Method == "PUT":
		h.apiUpdateAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case path == "/auth/iam-sso/start" && r.Method == "POST":
		h.apiStartIamSso(w, r)
	case path == "/auth/iam-sso/complete" && r.Method == "POST":
		h.apiCompleteIamSso(w, r)
	case path == "/auth/builderid/start" && r.Method == "POST":
		h.apiStartBuilderIdLogin(w, r)
	case path == "/auth/builderid/poll" && r.Method == "POST":
		h.apiPollBuilderIdAuth(w, r)
	case path == "/auth/sso-token" && r.Method == "POST":
		h.apiImportSsoToken(w, r)
	case path == "/auth/credentials" && r.Method == "POST":
		h.apiImportCredentials(w, r)
	case path == "/auth/credentials/batch" && r.Method == "POST":
		h.apiImportCredentialsBatch(w, r)
	default:
		return false
	}
	return true
}

// /status /settings /stats /thinking /endpoint /concurrency /version /export /import /generate-machine-id /system/unit-config：
// 系统配置 + 全局状态。
func (h *Handler) routeAdminSystem(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/status" && r.Method == "GET":
		h.apiGetStatus(w, r)
	case path == "/settings" && r.Method == "GET":
		h.apiGetSettings(w, r)
	case path == "/settings" && r.Method == "POST":
		h.apiUpdateSettings(w, r)
	case path == "/stats" && r.Method == "GET":
		h.apiGetAdminStats(w, r)
	case path == "/stats/reset" && r.Method == "POST":
		h.apiResetStats(w, r)
	case path == "/generate-machine-id" && r.Method == "GET":
		h.apiGenerateMachineId(w, r)
	case path == "/thinking" && r.Method == "GET":
		h.apiGetThinkingConfig(w, r)
	case path == "/thinking" && r.Method == "POST":
		h.apiUpdateThinkingConfig(w, r)
	case path == "/endpoint" && r.Method == "GET":
		h.apiGetEndpointConfig(w, r)
	case path == "/endpoint" && r.Method == "POST":
		h.apiUpdateEndpointConfig(w, r)
	case path == "/concurrency" && r.Method == "GET":
		h.apiGetConcurrency(w, r)
	case path == "/concurrency" && r.Method == "POST":
		h.apiUpdateConcurrency(w, r)
	case path == "/version" && r.Method == "GET":
		h.apiGetVersion(w, r)
	case path == "/export" && r.Method == "POST":
		h.apiExportAccounts(w, r)
	case path == "/import/db" && r.Method == "POST":
		h.apiImportFromDB(w, r)
	case path == "/import/db-status" && r.Method == "GET":
		h.apiGetDBStatus(w, r)
	case path == "/system/unit-config" && r.Method == "GET":
		h.apiGetSystemUnitConfig(w, r)
	case path == "/system/unit-config" && r.Method == "POST":
		h.apiPostSystemUnitConfig(w, r)
	default:
		return false
	}
	return true
}

// /apikeys/* + /codes/* + /recharges + /apikeys/clear-gift：销售域 sk-key 管理 + 激活码 + 充值流水。
// 注意：/apikeys/{id}/logs 和 /apikeys/{id}/recharges 必须先于通用 /apikeys/{id} 匹配。
func (h *Handler) routeAdminApiKeysAndCodes(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/apikeys" && r.Method == "GET":
		h.apiGetApiKeys(w, r)
	case path == "/apikeys" && r.Method == "POST":
		h.apiCreateApiKey(w, r)
	case path == "/apikeys/clear-gift" && r.Method == "POST":
		h.apiClearAllGift(w, r)
	case strings.HasPrefix(path, "/apikeys/") && strings.HasSuffix(path, "/logs") && r.Method == "GET":
		h.apiGetApiKeyLogs(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/apikeys/"), "/logs"))
	case strings.HasPrefix(path, "/apikeys/") && strings.HasSuffix(path, "/recharges") && r.Method == "GET":
		h.apiGetApiKeyRecharges(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/apikeys/"), "/recharges"))
	case strings.HasPrefix(path, "/apikeys/") && r.Method == "PUT":
		h.apiUpdateApiKey(w, r, strings.TrimPrefix(path, "/apikeys/"))
	case strings.HasPrefix(path, "/apikeys/") && r.Method == "DELETE":
		h.apiDeleteApiKey(w, r, strings.TrimPrefix(path, "/apikeys/"))
	case path == "/codes" && r.Method == "GET":
		h.apiGetCodes(w, r)
	case path == "/codes" && r.Method == "POST":
		h.apiCreateCodes(w, r)
	case path == "/codes/cleanup" && r.Method == "POST":
		h.apiCleanupCodes(w, r)
	case strings.HasPrefix(path, "/codes/") && r.Method == "DELETE":
		h.apiDeleteCode(w, r, strings.TrimPrefix(path, "/codes/"))
	case path == "/recharges" && r.Method == "GET":
		h.apiAdminRecharges(w, r)
	default:
		return false
	}
	return true
}

// /sell-prices /stealth /promotion：渠道售价 / 隐身 / 推广。
// v9: /pricing /pricing-analysis /profit /profit-include-gift /cost-entry 全部下线，
// 由 OPS /business-board 经营看板替代（基于 token 计费 + 渠道粒度成本聚合）。
func (h *Handler) routeAdminPricingAndProfit(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/sell-prices" && r.Method == "GET":
		h.apiGetSellPrices(w, r)
	case path == "/sell-prices" && r.Method == "PUT":
		h.apiUpdateSellPrices(w, r)
	case path == "/stealth" && r.Method == "GET":
		h.apiGetStealth(w, r)
	case path == "/stealth" && r.Method == "PUT":
		h.apiUpdateStealth(w, r)
	case path == "/promotion" && r.Method == "GET":
		h.apiGetPromotion(w, r)
	case path == "/promotion" && r.Method == "PUT":
		h.apiUpdatePromotion(w, r)
	case path == "/promotion/whitelist" && r.Method == "POST":
		h.apiAddPromotionWhitelist(w, r)
	case strings.HasPrefix(path, "/promotion/whitelist/") && r.Method == "DELETE":
		h.apiRemovePromotionWhitelist(w, r, strings.TrimPrefix(path, "/promotion/whitelist/"))
	default:
		return false
	}
	return true
}

// /providers/*：NewAPI 上游实例 (v5)。
// 注意：sync / metadata / migrate-manual-channels 子路径必须先于通用 /providers/{id} 匹配。
func (h *Handler) routeAdminProviders(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/providers" && r.Method == "GET":
		h.apiListProviders(w, r)
	case path == "/providers" && r.Method == "POST":
		h.apiCreateProvider(w, r)
	case strings.HasPrefix(path, "/providers/") && strings.HasSuffix(path, "/sync") && r.Method == "POST":
		h.apiSyncProvider(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/providers/"), "/sync")))
	case strings.HasPrefix(path, "/providers/") && strings.HasSuffix(path, "/metadata") && r.Method == "GET":
		h.apiGetProviderMetadata(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/providers/"), "/metadata")))
	case strings.HasPrefix(path, "/providers/") && strings.HasSuffix(path, "/migrate-manual-channels") && r.Method == "POST":
		// v6: masked-key migration 已废弃；走 POST /admin/api/newapi/channels 重新创建。
		writeAdminJSONError(w, http.StatusGone, "migrate-manual-channels endpoint removed in v6; create channels through POST /admin/api/newapi/channels")
	case strings.HasPrefix(path, "/providers/") && r.Method == "GET":
		h.apiGetProvider(w, r, decodePathID(strings.TrimPrefix(path, "/providers/")))
	case strings.HasPrefix(path, "/providers/") && r.Method == "PUT":
		h.apiUpdateProvider(w, r, decodePathID(strings.TrimPrefix(path, "/providers/")))
	case strings.HasPrefix(path, "/providers/") && r.Method == "DELETE":
		h.apiDeleteProvider(w, r, decodePathID(strings.TrimPrefix(path, "/providers/")))
	default:
		return false
	}
	return true
}

// /newapi/channels/* + /newapi/reconcile-status/*：v5/v6 物化渠道 + 对账。
// 注意：health-check 子路径必须先于通用 /newapi/channels/{id} 匹配。
// v6：删除了 /upstream-key PATCH 路由（masked-key path 已废弃，改走 POST 创建）。
func (h *Handler) routeAdminNewAPIChannels(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/newapi/channels" && r.Method == "GET":
		h.apiListNewAPIChannels(w, r)
	case path == "/newapi/channels" && r.Method == "POST":
		h.apiCreateNewAPIChannel(w, r)
	case path == "/newapi/reconcile-status" && r.Method == "GET":
		h.apiGetNewAPIReconcileStatus(w, r)
	case strings.HasPrefix(path, "/newapi/reconcile-status/retry/") && r.Method == "POST":
		h.apiRetryNewAPIReconcile(w, r, decodePathID(strings.TrimPrefix(path, "/newapi/reconcile-status/retry/")))
	case strings.HasPrefix(path, "/newapi/channels/") && strings.HasSuffix(path, "/health-check") && r.Method == "POST":
		h.apiHealthCheckNewAPIChannel(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/newapi/channels/"), "/health-check")))
	case strings.HasPrefix(path, "/newapi/channels/") && r.Method == "GET":
		h.apiGetNewAPIChannel(w, r, decodePathID(strings.TrimPrefix(path, "/newapi/channels/")))
	case strings.HasPrefix(path, "/newapi/channels/") && r.Method == "PATCH":
		h.apiPatchNewAPIChannel(w, r, decodePathID(strings.TrimPrefix(path, "/newapi/channels/")))
	case strings.HasPrefix(path, "/newapi/channels/") && r.Method == "DELETE":
		h.apiDeleteNewAPIChannel(w, r, decodePathID(strings.TrimPrefix(path, "/newapi/channels/")))
	default:
		return false
	}
	return true
}

// /direct-channels/*：v6 自营直连渠道（替换 v4 /channels 的对外路径）。
// 注意：health-check 子路径必须先于通用 PATCH/DELETE 匹配。
func (h *Handler) routeAdminDirectChannels(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/direct-channels" && r.Method == "GET":
		h.apiListDirectChannels(w, r)
	case path == "/direct-channels" && r.Method == "POST":
		h.apiCreateDirectChannel(w, r)
	case strings.HasPrefix(path, "/direct-channels/") && strings.HasSuffix(path, "/health-check") && r.Method == "POST":
		h.apiHealthCheckDirectChannel(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/direct-channels/"), "/health-check")))
	case strings.HasPrefix(path, "/direct-channels/") && r.Method == "GET":
		h.apiGetDirectChannel(w, r, decodePathID(strings.TrimPrefix(path, "/direct-channels/")))
	case strings.HasPrefix(path, "/direct-channels/") && r.Method == "PATCH":
		h.apiPatchDirectChannel(w, r, decodePathID(strings.TrimPrefix(path, "/direct-channels/")))
	case strings.HasPrefix(path, "/direct-channels/") && r.Method == "DELETE":
		h.apiDeleteDirectChannel(w, r, decodePathID(strings.TrimPrefix(path, "/direct-channels/")))
	default:
		return false
	}
	return true
}

// /channels/* + /series/*：v4 扁平渠道 + (deprecated) Series。
// 注意：health-check / test 子路径必须先于通用 PUT/DELETE 匹配。
func (h *Handler) routeAdminLegacyChannels(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/channels" && r.Method == "GET":
		h.apiListChannels(w, r)
	case path == "/channels" && r.Method == "POST":
		h.apiCreateChannel(w, r)
	case strings.HasPrefix(path, "/channels/") && strings.HasSuffix(path, "/health-check") && r.Method == "POST":
		h.apiHealthCheckChannel(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/channels/"), "/health-check")))
	case strings.HasPrefix(path, "/channels/") && strings.HasSuffix(path, "/test") && r.Method == "POST":
		h.apiTestChannel(w, r, decodePathID(strings.TrimSuffix(strings.TrimPrefix(path, "/channels/"), "/test")))
	case strings.HasPrefix(path, "/channels/") && r.Method == "PUT":
		h.apiUpdateChannel(w, r, decodePathID(strings.TrimPrefix(path, "/channels/")))
	case strings.HasPrefix(path, "/channels/") && r.Method == "DELETE":
		h.apiDeleteChannel(w, r, decodePathID(strings.TrimPrefix(path, "/channels/")))
	case path == "/series" || strings.HasPrefix(path, "/series/"):
		// v6: Series 概念已废弃，对外分组改用 alias（NewAPIChannel + DirectChannel 各自唯一）。
		// 保留一个版本周期返回 410，老前端可显式提示用户刷新。
		writeAdminJSONError(w, http.StatusGone, "/series endpoints removed in v6; use /admin/api/groups for the aggregate overview")
	default:
		return false
	}
	return true
}

// /logs /sse/* /abuse /inactive-keys /leaderboard /insights /business-board：运营 + 实时流 + 风控 + 洞察。
func (h *Handler) routeAdminOpsAndInsights(path string, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case path == "/business-board" && r.Method == "GET":
		h.apiBusinessBoard(w, r)
	case path == "/logs" && r.Method == "GET":
		h.apiGetLogs(w, r)
	case path == "/logs" && r.Method == "DELETE":
		h.apiClearLogs(w, r)
	case path == "/sse/logs" && r.Method == "GET":
		h.handleSSELogs(w, r)
	case path == "/sse/stats" && r.Method == "GET":
		h.handleSSEStats(w, r)
	case path == "/abuse" && r.Method == "GET":
		h.apiGetAbuse(w, r)
	case strings.HasPrefix(path, "/abuse/") && strings.HasSuffix(path, "/clear") && r.Method == "POST":
		h.apiClearAbuse(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/abuse/"), "/clear"))
	case path == "/inactive-keys" && r.Method == "GET":
		h.apiInactiveKeys(w, r)
	case path == "/leaderboard" && r.Method == "GET":
		h.apiAdminLeaderboard(w, r)
	case path == "/leaderboard/config" && r.Method == "GET":
		h.apiGetLeaderboardConfig(w, r)
	case path == "/leaderboard/config" && r.Method == "PUT":
		h.apiUpdateLeaderboardConfig(w, r)
	case path == "/insights/funnel" && r.Method == "GET":
		h.apiInsightsFunnel(w, r)
	case path == "/insights/whales" && r.Method == "GET":
		h.apiInsightsWhales(w, r)
	case path == "/insights/freeloaders" && r.Method == "GET":
		h.apiInsightsFreeloaders(w, r)
	case path == "/insights/daily" && r.Method == "GET":
		h.apiInsightsDaily(w, r)
	default:
		return false
	}
	return true
}
