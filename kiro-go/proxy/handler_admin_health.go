package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"kiro-api-proxy/config"
)

// healthChecker 是支持自检的 channel 必须实现的接口。
// 目前 OpenAIChannel 实现了；Kiro channel 可后续实现（探测上游 token 有效性）。
type healthChecker interface {
	HealthCheck(ctx context.Context, model string) *HealthCheckResult
}

// channelHealthCooldown 防止 admin 一直点 health-check burn 上游配额。
// 同一个 channel 30 秒内只允许一次。
const channelHealthCooldown = 30 * time.Second

// admin 触发 health check 的速率限制状态（per-channel）。
var (
	healthCheckMu      sync.Mutex
	healthCheckLastRun = map[string]time.Time{}
)

// allowChannelHealthCheck 返回是否允许立即对该 channel 跑 health-check。
// 不允许时返回剩余冷却秒数。
func allowChannelHealthCheck(channelID string) (bool, int64) {
	healthCheckMu.Lock()
	defer healthCheckMu.Unlock()
	last, ok := healthCheckLastRun[channelID]
	if !ok {
		healthCheckLastRun[channelID] = time.Now()
		return true, 0
	}
	elapsed := time.Since(last)
	if elapsed >= channelHealthCooldown {
		healthCheckLastRun[channelID] = time.Now()
		return true, 0
	}
	return false, int64((channelHealthCooldown - elapsed).Seconds()) + 1
}

// POST /admin/api/channels/{id}/health-check
//
// v4 端点：探测渠道连通性 + 模型可用性。
// 流程：
//  1. per-channel 30s 冷却（防 burn 上游配额）
//  2. 找到 channel 配置（在路由器里）
//  3. 走 healthChecker 接口（OpenAI 实现：/v1/models + 1-token chat probe）
//  4. AuditLog 记录
//  5. 返回 HealthCheckResult JSON
func (h *Handler) apiHealthCheckChannel(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "channel id required")
		return
	}

	// codex audit 修复（Warning E）：先验证 channel 存在和可用，再扣 rate-limit 配额。
	// 避免 admin 误点不存在的 channel 也消耗 30s 冷却名额。
	channels := config.GetChannels()
	var cfgCh *config.ChannelConfig
	for i := range channels {
		if channels[i].ID == id {
			cfgCh = &channels[i]
			break
		}
	}
	if cfgCh == nil {
		writeAdminJSONError(w, http.StatusNotFound, "channel not found")
		return
	}

	router := h.currentChannelRouter()
	if router == nil {
		writeAdminJSONError(w, http.StatusServiceUnavailable, "channel router not initialized")
		return
	}
	ch, ok := router.ChannelByID(id)
	if !ok {
		writeAdminJSONError(w, http.StatusConflict, "channel is disabled or not loaded; enable it first")
		return
	}

	hc, ok := ch.(healthChecker)
	if !ok {
		writeAdminJSONError(w, http.StatusNotImplemented,
			fmt.Sprintf("channel type %q does not support health check", ch.Type()))
		return
	}

	// 现在才扣 rate-limit 配额（健康检查会真访问上游）
	if allowed, retryAfterSec := allowChannelHealthCheck(id); !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSec))
		writeAdminJSONError(w, http.StatusTooManyRequests,
			fmt.Sprintf("health check rate limited; retry in %ds", retryAfterSec))
		return
	}

	// 15s 整体超时（fetchModels 和 probeChat 内部 client 还各有 15s timeout 兜底）
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result := hc.HealthCheck(ctx, "")
	AuditLog("channel_health_check", adminAuditActor(r),
		fmt.Sprintf("id=%s success=%v latencyMs=%d modelsOk=%v chatOk=%v",
			id, result.Success, result.LatencyMs, result.ModelsOK, result.ChatOK))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
