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

const newAPIChannelCreateBodyLimit = 32 << 10

type newAPIChannelCreateRequest struct {
	ProviderID          string   `json:"providerId"`
	Alias               string   `json:"alias"`
	Group               string   `json:"group"`
	Models              []string `json:"models"`
	Markup              float64  `json:"markup"`
	RemainQuota         int64    `json:"remainQuota"`
	UnlimitedQuota      bool     `json:"unlimitedQuota"`
	ExpiredTime         int64    `json:"expiredTime"`
	ModelLimitsEnabled  bool     `json:"modelLimitsEnabled"`
	ModelLimits         string   `json:"modelLimits"`
	CrossGroupRetry     bool     `json:"crossGroupRetry"`
	AllowIPs            string   `json:"allowIPs"`
}

// GET /admin/api/newapi/channels/{id}
func (h *Handler) apiGetNewAPIChannel(w http.ResponseWriter, _ *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing channel id")
		return
	}
	ch, ok := config.GetNewAPIChannel(id)
	if !ok || ch.DeletedAt > 0 {
		writeAdminJSONError(w, http.StatusNotFound, "newapi channel not found")
		return
	}
	writeAdminJSON(w, http.StatusOK, toPublicNewAPIChannel(ch))
}

// POST /admin/api/newapi/channels
func (h *Handler) apiCreateNewAPIChannel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, newAPIChannelCreateBodyLimit)
	var req newAPIChannelCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	normalizeNewAPIChannelCreateRequest(&req)
	if err := validateNewAPIChannelCreateRequest(req); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "conflicts") {
			code = http.StatusConflict
		}
		writeAdminJSONError(w, code, err.Error())
		return
	}

	provider, sessionCookie, status, err := h.newAPIChannelProviderSession(r.Context(), req.ProviderID, true)
	if err != nil {
		writeAdminJSONError(w, status, err.Error())
		return
	}
	createReq := newAPIChannelCreateTokenRequest(req)
	token, err := h.createUpstreamNewAPIToken(r.Context(), provider, sessionCookie, createReq)
	if err != nil {
		writeAdminJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	keyEnc, err := config.EncryptSecret("sk-" + token.Key)
	if err != nil {
		h.cleanupCreatedNewAPIToken(provider, sessionCookie, token.ID, err)
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt upstream key")
		return
	}
	channel := buildNewAPIChannelFromCreate(provider.ID, req, token, keyEnc, time.Now().Unix())
	if err := config.AddNewAPIChannel(channel); err != nil {
		h.cleanupCreatedNewAPIToken(provider, sessionCookie, token.ID, err)
		// codex audit warning #1: alias/id conflict 应返 409 而非 500
		msg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(msg, "already used by") || strings.Contains(msg, "already exists") {
			status = http.StatusConflict
		}
		writeAdminJSONError(w, status, msg)
		return
	}
	AuditLog("newapi_channel_create", adminAuditActor(r), fmt.Sprintf("id=%s alias=%s", channel.ID, channel.Alias))
	h.reloadChannelRouter()
	writeAdminJSON(w, http.StatusCreated, toPublicNewAPIChannel(channel))
}

// DELETE /admin/api/newapi/channels/{id}?deleteUpstream=true|false
func (h *Handler) apiDeleteNewAPIChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing channel id")
		return
	}
	ch, ok := config.GetNewAPIChannel(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "newapi channel not found")
		return
	}
	deleteUpstream := strings.EqualFold(r.URL.Query().Get("deleteUpstream"), "true")
	// 即使 ch.DeletedAt > 0（本地已软删），仍允许"再次删除"以触发上游 token 清理 ——
	// 上游 token 在 apijing 上还活着会持续占配额并污染 sync 列表，必须能从这里清掉。
	upstreamDeletedOK := false
	if deleteUpstream {
		if err := h.deleteUpstreamNewAPIToken(r.Context(), ch); err != nil {
			fmt.Printf("[newapi] WARN: delete upstream token failed channel=%s tokenID=%d error=%v\n", id, ch.UpstreamTokenID, err)
		} else {
			upstreamDeletedOK = true
		}
	}
	if ch.DeletedAt == 0 {
		if err := config.SoftDeleteNewAPIChannel(id); err != nil {
			writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	AuditLog("newapi_channel_delete", adminAuditActor(r), fmt.Sprintf("id=%s deleteUpstream=%t alreadySoftDeleted=%t", id, deleteUpstream, ch.DeletedAt > 0))
	h.reloadChannelRouter()
	// 上游 token 已删 → 同步刷新 metadata cache，让前端 reload 立刻看到不包含已删 token 的列表
	if upstreamDeletedOK {
		syncCtx, cancel := context.WithTimeout(r.Context(), newAPISyncTimeout)
		if err := h.ensureNewAPIManager().SyncProviderMetadata(syncCtx, ch.ProviderID); err != nil {
			fmt.Printf("[newapi] WARN: post-delete sync failed provider=%s error=%v\n", ch.ProviderID, err)
		}
		cancel()
	}
	writeAdminJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func normalizeNewAPIChannelCreateRequest(req *newAPIChannelCreateRequest) {
	req.ProviderID = strings.TrimSpace(req.ProviderID)
	req.Alias = strings.TrimSpace(req.Alias)
	req.Group = strings.TrimSpace(req.Group)
	req.ModelLimits = strings.TrimSpace(req.ModelLimits)
	req.AllowIPs = strings.TrimSpace(req.AllowIPs)
	models := make([]string, 0, len(req.Models))
	for _, model := range req.Models {
		model = strings.TrimSpace(model)
		if model != "" {
			models = append(models, model)
		}
	}
	req.Models = models
	if req.ExpiredTime == 0 {
		req.ExpiredTime = -1
	}
}

func validateNewAPIChannelCreateRequest(req newAPIChannelCreateRequest) error {
	if req.ProviderID == "" {
		return fmt.Errorf("providerId required")
	}
	if req.Alias == "" {
		return fmt.Errorf("alias required")
	}
	if req.Markup <= 0 {
		return fmt.Errorf("markup must be > 0")
	}
	return config.ValidateGroupAliasUnique("", req.Alias)
}

func newAPIChannelCreateTokenRequest(req newAPIChannelCreateRequest) NewAPICreateTokenRequest {
	return NewAPICreateTokenRequest{
		Name:               req.Alias,
		Group:              req.Group,
		Models:             append([]string(nil), req.Models...),
		UnlimitedQuota:     req.UnlimitedQuota,
		RemainQuota:        req.RemainQuota,
		ExpiredTime:        req.ExpiredTime,
		ModelLimitsEnabled: req.ModelLimitsEnabled,
		ModelLimits:        req.ModelLimits,
		CrossGroupRetry:    req.CrossGroupRetry,
		AllowIPs:           req.AllowIPs,
	}
}

func (h *Handler) newAPIChannelProviderSession(ctx context.Context, providerID string, requireEnabled bool) (config.NewAPIProvider, string, int, error) {
	p, ok := config.GetNewAPIProvider(providerID)
	if !ok {
		return config.NewAPIProvider{}, "", http.StatusNotFound, fmt.Errorf("provider not found")
	}
	if requireEnabled && !p.Enabled {
		return config.NewAPIProvider{}, "", http.StatusConflict, fmt.Errorf("provider is disabled")
	}
	if p.AccessTokenEnc == "" || p.AccessTokenExpiresAt <= time.Now().Add(newAPILoginRefreshSkew).Unix() {
		loginCtx, cancel := context.WithTimeout(ctx, newAPISyncTimeout)
		err := h.ensureNewAPIManager().EnsureLogin(loginCtx, providerID, false)
		cancel()
		if err != nil {
			// codex audit warning #2: upstream login fail 用 502 BadGateway，admin auth 已经过
			// 用 401 会让客户端误判 admin session 失效；改 502 表示上游不可达。
			return config.NewAPIProvider{}, "", http.StatusBadGateway, fmt.Errorf("upstream provider login failed; re-login provider: %w", err)
		}
		p, ok = config.GetNewAPIProvider(providerID)
		if !ok {
			return config.NewAPIProvider{}, "", http.StatusNotFound, fmt.Errorf("provider not found")
		}
	}
	sessionCookie, err := config.DecryptSecret(p.AccessTokenEnc)
	if err != nil {
		return config.NewAPIProvider{}, "", http.StatusInternalServerError, fmt.Errorf("failed to decrypt provider session")
	}
	if strings.TrimSpace(sessionCookie) == "" {
		// 424 FailedDependency: provider 配置存在但凭据缺失 — 跟"上游不可达"语义区分开
		return config.NewAPIProvider{}, "", http.StatusFailedDependency, fmt.Errorf("provider session unavailable; re-login provider")
	}
	return p, sessionCookie, http.StatusOK, nil
}

func (h *Handler) createUpstreamNewAPIToken(ctx context.Context, p config.NewAPIProvider, sessionCookie string, req NewAPICreateTokenRequest) (*NewAPICreatedToken, error) {
	createCtx, cancel := context.WithTimeout(ctx, newAPISyncTimeout)
	defer cancel()
	return h.ensureNewAPIManager().client.CreateToken(createCtx, p.BaseURL, sessionCookie, p.UserID, req)
}

func buildNewAPIChannelFromCreate(providerID string, req newAPIChannelCreateRequest, token *NewAPICreatedToken, keyEnc string, now int64) config.NewAPIChannel {
	return config.NewAPIChannel{
		ID:                fmt.Sprintf("%s:tok-%d", providerID, token.ID),
		ProviderID:        providerID,
		Alias:             req.Alias,
		UpstreamTokenID:   token.ID,
		UpstreamTokenName: req.Alias,
		UpstreamKeyEnc:    keyEnc,
		GroupName:         req.Group,
		Models:            append([]string(nil), req.Models...),
		Markup:            req.Markup,
		Enabled:           true,
		RemainQuota:       req.RemainQuota,
		UnlimitedQuota:    req.UnlimitedQuota,
		Status:            1,
		CreateMode:        "pivotstack",
		CreatedAt:         now,
		UpdatedAt:         now,
		LastSeenAt:        now,
	}
}

func (h *Handler) cleanupCreatedNewAPIToken(p config.NewAPIProvider, sessionCookie string, tokenID int, cause error) {
	if tokenID <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), newAPISyncTimeout)
	defer cancel()
	err := h.ensureNewAPIManager().client.DeleteToken(ctx, p.BaseURL, sessionCookie, p.UserID, tokenID)
	if err != nil {
		fmt.Printf("[newapi] WARN: rollback delete token failed provider=%s tokenID=%d cause=%v deleteError=%v\n", p.ID, tokenID, cause, err)
	}
}

func (h *Handler) deleteUpstreamNewAPIToken(ctx context.Context, ch config.NewAPIChannel) error {
	if ch.UpstreamTokenID <= 0 {
		return fmt.Errorf("channel %s has no upstream token id", ch.ID)
	}
	p, sessionCookie, _, err := h.newAPIChannelProviderSession(ctx, ch.ProviderID, false)
	if err != nil {
		return err
	}
	deleteCtx, cancel := context.WithTimeout(ctx, newAPISyncTimeout)
	defer cancel()
	return h.ensureNewAPIManager().client.DeleteToken(deleteCtx, p.BaseURL, sessionCookie, p.UserID, ch.UpstreamTokenID)
}
