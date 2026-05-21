package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kiro-api-proxy/config"
)

// publicNewAPIChannel 是 v5 NewAPIChannel 的对外视图，刻意省略 UpstreamKeyEnc。
type publicNewAPIChannel struct {
	ID                string   `json:"id"`
	ProviderID        string   `json:"providerId"`
	Alias             string   `json:"alias"`
	UpstreamTokenID   int      `json:"upstreamTokenId"`
	UpstreamTokenName string   `json:"upstreamTokenName,omitempty"`
	GroupName         string   `json:"groupName"`
	Models            []string `json:"models"`
	Markup            float64  `json:"markup"`
	SeriesID          string   `json:"seriesId,omitempty"`
	CreateMode        string   `json:"createMode,omitempty"`
	CreatedAt         int64    `json:"createdAt,omitempty"`
	UpdatedAt         int64    `json:"updatedAt,omitempty"`
	Enabled           bool     `json:"enabled"`
	RemainQuota       int64    `json:"remainQuota"`
	UnlimitedQuota    bool     `json:"unlimitedQuota"`
	Status            int      `json:"status"`
	LastSeenAt        int64    `json:"lastSeenAt,omitempty"`
	DeletedAt         int64    `json:"deletedAt,omitempty"`
}

func toPublicNewAPIChannel(c config.NewAPIChannel) publicNewAPIChannel {
	models := make([]string, len(c.Models))
	copy(models, c.Models)
	return publicNewAPIChannel{
		ID:                c.ID,
		ProviderID:        c.ProviderID,
		Alias:             c.Alias,
		UpstreamTokenID:   c.UpstreamTokenID,
		UpstreamTokenName: c.UpstreamTokenName,
		GroupName:         c.GroupName,
		Models:            models,
		Markup:            c.Markup,
		SeriesID:          c.SeriesID,
		CreateMode:        c.CreateMode,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		Enabled:           c.Enabled,
		RemainQuota:       c.RemainQuota,
		UnlimitedQuota:    c.UnlimitedQuota,
		Status:            c.Status,
		LastSeenAt:        c.LastSeenAt,
		DeletedAt:         c.DeletedAt,
	}
}

// newAPIChannelPatchRequest 只允许 admin 改 4 个字段；其余由同步流程覆盖。
// 用指针类型区分 "未提供" vs "想清空"。
type newAPIChannelPatchRequest struct {
	Alias    *string  `json:"alias"`
	Markup   *float64 `json:"markup"`
	SeriesID *string  `json:"seriesId"`
	Enabled  *bool    `json:"enabled"`
}

// GET /admin/api/newapi/channels
func (h *Handler) apiListNewAPIChannels(w http.ResponseWriter, _ *http.Request) {
	channels := config.GetNewAPIChannels()
	out := make([]publicNewAPIChannel, 0, len(channels))
	for _, c := range channels {
		out = append(out, toPublicNewAPIChannel(c))
	}
	writeAdminJSON(w, http.StatusOK, out)
}

// PATCH /admin/api/newapi/channels/{id}
func (h *Handler) apiPatchNewAPIChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing channel id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var req newAPIChannelPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	channels := config.GetNewAPIChannels()
	idx := -1
	for i, c := range channels {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeAdminJSONError(w, http.StatusNotFound, fmt.Sprintf("channel %q not found", id))
		return
	}
	// codex audit warning #3: 已软删的 channel 不可再 PATCH
	if channels[idx].DeletedAt > 0 {
		writeAdminJSONError(w, http.StatusConflict, fmt.Sprintf("channel %q is deleted", id))
		return
	}

	// 校验 SeriesID 必须存在（空字符串允许 = 解除 series 绑定）
	if req.SeriesID != nil {
		next := strings.TrimSpace(*req.SeriesID)
		if next != "" {
			ok := false
			for _, s := range config.GetSeries() {
				if s.ID == next {
					ok = true
					break
				}
			}
			if !ok {
				writeAdminJSONError(w, http.StatusBadRequest, fmt.Sprintf("series %q not found", next))
				return
			}
		}
	}

	// Markup 必须为正数（0 或负数会导致 silent free，参见 Phase 4a fail-closed 校验）
	if req.Markup != nil && *req.Markup <= 0 {
		writeAdminJSONError(w, http.StatusBadRequest, "markup must be > 0 (zero or negative would silently zero out billing)")
		return
	}

	// Alias 改名：必须非空且跨 NewAPIChannel + DirectChannel 全局唯一（v6）
	if req.Alias != nil {
		nextAlias := strings.TrimSpace(*req.Alias)
		if nextAlias == "" {
			writeAdminJSONError(w, http.StatusBadRequest, "alias cannot be empty")
			return
		}
		if err := config.ValidateGroupAliasUnique(id, nextAlias); err != nil {
			writeAdminJSONError(w, http.StatusConflict, err.Error())
			return
		}
	}

	patched := channels[idx]
	if req.Alias != nil {
		patched.Alias = strings.TrimSpace(*req.Alias)
	}
	if req.Markup != nil {
		patched.Markup = *req.Markup
	}
	if req.SeriesID != nil {
		patched.SeriesID = strings.TrimSpace(*req.SeriesID)
	}
	if req.Enabled != nil {
		patched.Enabled = *req.Enabled
	}
	channels[idx] = patched

	if err := config.UpdateNewAPIChannels(channels); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// markup / enabled / seriesId 都影响路由 + 计费，必须 reload 让在飞后续请求看到新值。
	h.reloadChannelRouter()

	writeAdminJSON(w, http.StatusOK, toPublicNewAPIChannel(patched))
}

// POST /admin/api/newapi/channels/{id}/health-check
//
// 同 v4 health-check 端点：双探针 + per-channel 30s 冷却。
// 复用 handler_admin_health 里的 allowChannelHealthCheck（v5 channel ID 形如 "apijing:tok-908"
// 跟 v4 channel ID 命名空间不冲突，共享冷却 map 是安全的）。
func (h *Handler) apiHealthCheckNewAPIChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "channel id required")
		return
	}

	// 找到 channel 配置
	var target *config.NewAPIChannel
	for _, c := range config.GetNewAPIChannels() {
		if c.ID == id {
			c2 := c
			target = &c2
			break
		}
	}
	if target == nil {
		writeAdminJSONError(w, http.StatusNotFound, "newapi channel not found")
		return
	}
	if !target.Enabled || target.DeletedAt > 0 {
		writeAdminJSONError(w, http.StatusConflict, "channel is disabled or deleted; enable it first")
		return
	}

	// 找到 provider
	provider, ok := config.GetNewAPIProvider(target.ProviderID)
	if !ok {
		writeAdminJSONError(w, http.StatusFailedDependency, fmt.Sprintf("provider %q not found", target.ProviderID))
		return
	}

	// 现在才扣 rate-limit 配额（前面的校验失败不应消耗冷却名额）
	if allowed, retryAfterSec := allowChannelHealthCheck(id); !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSec))
		writeAdminJSONError(w, http.StatusTooManyRequests,
			fmt.Sprintf("health check rate limited; retry in %ds", retryAfterSec))
		return
	}

	ch := newNewAPIRuntimeChannel(*target, provider)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result := ch.HealthCheck(ctx, "")
	AuditLog("newapi_channel_health_check", adminAuditActor(r),
		fmt.Sprintf("id=%s success=%v latencyMs=%d modelsOk=%v chatOk=%v",
			id, result.Success, result.LatencyMs, result.ModelsOK, result.ChatOK))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
