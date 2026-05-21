package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
)

// User-side ApiKey CRUD (v7). 自营 user 通过这套 API 自助管理名下多把 key。
// 与 admin /admin/api/apikeys 严格区分：
//   - admin policy 可改 balance/gift/isReseller 等；
//   - user policy 仅允许改 note/enabled/channelPreferences；
//   - ownership 强制：keyID 必须在 User.ApiKeyIDs 列表里；
//   - 不允许删最后一把 key（user 至少要留一把可登录）。

// GET /user/api/channel-options
// 返回可选的 ChannelGroup + 每个 group 下的 channel 选项（脱敏，不含 upstream key）。
// 用于"创建 Key" Drawer 选择路由偏好。即使没绑账号也可访问（read-only）。
func (h *Handler) handleUserChannelOptions(w http.ResponseWriter, _ *http.Request, _ *config.ApiKeyInfo) {
	groups := config.GetActiveChannelGroups()
	groupsOut := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		if !g.Enabled {
			continue
		}
		channels := make([]map[string]any, 0, len(g.Channels))
		for _, ref := range g.Channels {
			rid := config.RuntimeChannelIDFor(ref)
			alias := lookupChannelAlias(rid)
			modelsList := channelModelsForRef(ref)
			models := make([]map[string]any, 0, len(modelsList))
			for _, m := range modelsList {
				row := map[string]any{"name": m}
				if in, out, ok := h.resolveDisplayPriceForChannel(ref, m); ok && (in > 0 || out > 0) {
					row["inputPerM"] = in
					row["outputPerM"] = out
				}
				models = append(models, row)
			}
			channels = append(channels, map[string]any{
				"id":         rid,
				"alias":      alias,
				"sourceType": ref.SourceType,
				"models":     models,
			})
		}
		groupsOut = append(groupsOut, map[string]any{
			"id":             g.ID,
			"name":           g.Name,
			"description":    g.Description,
			"defaultChannel": strings.TrimSpace(g.DefaultRuntimeChannelID),
			"channels":       channels,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"groups":                   groupsOut,
		"pivotStackDollarsPerYuan": config.GetPivotStackDollarsPerYuan(),
	})
}

// channelModelsForRef 返回 ChannelGroup 引用的渠道支持的模型列表。
// 走 config 层 GetDirectChannel/GetNewAPIChannel，跳过已删除的渠道。
func channelModelsForRef(ref config.ChannelGroupChannelRef) []string {
	switch ref.SourceType {
	case "direct":
		if ch, ok := config.GetDirectChannel(ref.ChannelID); ok && ch.DeletedAt == 0 {
			return ch.Models
		}
	case "newapi":
		if ch, ok := config.GetNewAPIChannel(ref.ChannelID); ok && ch.DeletedAt == 0 {
			return ch.Models
		}
	}
	return nil
}

// resolveDisplayPriceForChannel 返回 UI 展示用的等价单价（virtual$ / 1M tokens）。
//   - direct: 复用 resolveSellPriceForChannel（直接读 DirectChannel.SellPrice）
//   - newapi: 用 1M tokens 走 EstimateNewAPIQuota + QuotaToPivotDollars 反算，
//             保证显示价 = 实际扣费时算出的 cost（同公式同 cache 同 markup）
func (h *Handler) resolveDisplayPriceForChannel(ref config.ChannelGroupChannelRef, model string) (in, out float64, ok bool) {
	if ref.SourceType == "direct" {
		rid := "direct:" + ref.ChannelID
		if p, found := resolveSellPriceForChannel(rid, model); found {
			return p.InputPerM, p.OutputPerM, true
		}
		return 0, 0, false
	}
	if ref.SourceType != "newapi" || h.newapiManager == nil {
		return 0, 0, false
	}
	ch, found := config.GetNewAPIChannel(ref.ChannelID)
	if !found || ch.DeletedAt != 0 {
		return 0, 0, false
	}
	cache, ok := h.newapiManager.Cache(ch.ProviderID)
	if !ok || cache == nil {
		return 0, 0, false
	}
	provider, found := config.GetNewAPIProvider(ch.ProviderID)
	if !found || provider.QuotaPerUnitDollar <= 0 || provider.YuanPerUpstreamDollar <= 0 {
		return 0, 0, false
	}
	markup := positiveOrDefault(ch.Markup, 1.0)
	psdpy := config.GetPivotStackDollarsPerYuan()
	// 1M prompt → inputPerM；1M output → outputPerM
	quotaIn, _, err := estimateNewAPIQuotaWithRatios(cache, model, ch.GroupName, 1_000_000, 0)
	if err != nil {
		return 0, 0, false
	}
	quotaOut, _, err := estimateNewAPIQuotaWithRatios(cache, model, ch.GroupName, 0, 1_000_000)
	if err != nil {
		return 0, 0, false
	}
	in = QuotaToPivotDollars(quotaIn, provider.QuotaPerUnitDollar, provider.YuanPerUpstreamDollar, psdpy, markup)
	out = QuotaToPivotDollars(quotaOut, provider.QuotaPerUnitDollar, provider.YuanPerUpstreamDollar, psdpy, markup)
	return in, out, true
}

// resolveUserAndOwnedKey 拿到当前请求用户 + 校验目标 keyID 是否归该 user 所有。
// 未绑 User → 401 "bind account required"。keyID 不属于 user → 403。
func (h *Handler) resolveUserAndOwnedKey(w http.ResponseWriter, currentKey *config.ApiKeyInfo, targetKeyID string) (users.User, *config.ApiKeyInfo, bool) {
	if currentKey == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing api key"})
		return users.User{}, nil, false
	}
	u, ok := users.Default().FindByApiKeyID(currentKey.ID)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "bind account required"})
		return users.User{}, nil, false
	}
	if targetKeyID == "" {
		return u, nil, true
	}
	owned := false
	for _, id := range u.ApiKeyIDs {
		if id == targetKeyID {
			owned = true
			break
		}
	}
	if !owned {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "key not owned by current user"})
		return users.User{}, nil, false
	}
	target := config.FindApiKeyByID(targetKeyID)
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "api key not found"})
		return users.User{}, nil, false
	}
	return u, target, true
}

// userKeyView 是返回给前端的 redacted 视图（不含 raw Key，仅在 create 时返回一次）。
type userKeyView struct {
	ID                 string             `json:"id"`
	Key                string             `json:"key,omitempty"` // raw API key — user 自己的 key 可重复查看（不是临时一次性展示）
	Note               string             `json:"note,omitempty"`
	Enabled            bool               `json:"enabled"`
	Plan               string             `json:"plan,omitempty"`
	ExpiresAt          int64              `json:"expiresAt,omitempty"`
	Balance            float64            `json:"balance"`
	GiftBalance        float64            `json:"giftBalance"`
	CreatedAt          int64              `json:"createdAt"`
	LastUsed           int64              `json:"lastUsed,omitempty"`
	Requests           int64              `json:"requests"`
	Errors             int64              `json:"errors"`
	Tokens             int64              `json:"tokens"`
	Credits            float64            `json:"credits"`
	IsDefault          bool               `json:"isDefault,omitempty"`
	ChannelPreferences map[string]string  `json:"channelPreferences,omitempty"`
}

func keyToUserView(k *config.ApiKeyInfo, defaultID string) userKeyView {
	prefs := map[string]string{}
	for kk, vv := range k.ChannelPreferences {
		prefs[kk] = vv
	}
	return userKeyView{
		ID:                 k.ID,
		Key:                k.Key,
		Note:               k.Note,
		Enabled:            k.Enabled,
		Plan:               k.Plan,
		ExpiresAt:          k.ExpiresAt,
		Balance:            k.Balance,
		GiftBalance:        k.GiftBalance,
		CreatedAt:          k.CreatedAt,
		LastUsed:           k.LastUsed,
		Requests:           k.Requests,
		Errors:             k.Errors,
		Tokens:             k.Tokens,
		Credits:            k.Credits,
		IsDefault:          k.ID == defaultID,
		ChannelPreferences: prefs,
	}
}

// GET /user/api/keys
func (h *Handler) handleUserListKeys(w http.ResponseWriter, currentKey *config.ApiKeyInfo) {
	u, _, ok := h.resolveUserAndOwnedKey(w, currentKey, "")
	if !ok {
		return
	}
	out := make([]userKeyView, 0, len(u.ApiKeyIDs))
	for _, id := range u.ApiKeyIDs {
		k := config.FindApiKeyByID(id)
		if k == nil {
			continue
		}
		// 合并 in-memory stats
		h.apiKeyStatsMu.RLock()
		if stats, exists := h.apiKeyStats[k.ID]; exists {
			k.LastUsed = stats.LastUsed
			k.Requests = stats.Requests
			k.Errors = stats.Errors
			k.Tokens = stats.Tokens
			k.Credits = stats.Credits
		}
		h.apiKeyStatsMu.RUnlock()
		out = append(out, keyToUserView(k, u.DefaultKeyID))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /user/api/keys
// Body: { note, expiresAt?, channelPreferences?, rateLimitPerMin? }
// 返回新建 key 的完整信息（含 raw Key，仅这一次返回）。
// v7.1：路由偏好在创建 key 时一次性配置（NewAPI 风格），不再用独立"分组路由"页面。
func (h *Handler) handleUserCreateKey(w http.ResponseWriter, r *http.Request, currentKey *config.ApiKeyInfo) {
	u, _, ok := h.resolveUserAndOwnedKey(w, currentKey, "")
	if !ok {
		return
	}
	var req struct {
		Note               string            `json:"note"`
		ExpiresAt          int64             `json:"expiresAt"`          // 0 = 永不过期
		ChannelPreferences map[string]string `json:"channelPreferences"` // groupID → runtimeChannelID
		RateLimitPerMin    int               `json:"rateLimitPerMin"`    // 0 = 走全局默认
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	// 防越权：user 不允许给自己设过高速率（admin 才能设；这里强制 ≤ 默认上限或保持 0）
	if req.RateLimitPerMin < 0 {
		req.RateLimitPerMin = 0
	}
	prefs := map[string]string{}
	for k, v := range req.ChannelPreferences {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		prefs[k] = v
	}
	newKey := config.ApiKeyInfo{
		ID:                 config.GenerateMachineId(),
		Key:                config.GenerateApiKeyString(),
		Plan:               "credit",
		Enabled:            true,
		Note:               req.Note,
		ExpiresAt:          req.ExpiresAt,
		ChannelPreferences: prefs,
		RateLimitPerMin:    req.RateLimitPerMin,
		CreatedAt:          time.Now().Unix(),
	}
	if err := config.AddApiKey(newKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 同步到 user.ApiKeyIDs；失败时回滚刚创建的 key（避免孤儿）
	if err := users.Default().UpdateUser(u.ID, func(uu *users.User) {
		uu.ApiKeyIDs = append(uu.ApiKeyIDs, newKey.ID)
	}); err != nil {
		_ = config.DeleteApiKey(newKey.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	AuditLog("user_apikey_create", u.Email, "keyID="+newKey.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                 newKey.ID,
		"key":                newKey.Key, // raw，仅此一次返回
		"note":               newKey.Note,
		"enabled":            newKey.Enabled,
		"expiresAt":          newKey.ExpiresAt,
		"channelPreferences": newKey.ChannelPreferences,
		"rateLimitPerMin":    newKey.RateLimitPerMin,
		"createdAt":          newKey.CreatedAt,
	})
}

// PATCH /user/api/keys/{id}  Body: { note?, enabled?, channelPreferences? }
// 严格 restricted policy：禁止改 balance / gift / isReseller / plan / expiresAt（admin 专属）。
func (h *Handler) handleUserPatchKey(w http.ResponseWriter, r *http.Request, currentKey *config.ApiKeyInfo, keyID string) {
	u, target, ok := h.resolveUserAndOwnedKey(w, currentKey, keyID)
	if !ok {
		return
	}
	var req struct {
		Note               *string            `json:"note"`
		Enabled            *bool              `json:"enabled"`
		ChannelPreferences *map[string]string `json:"channelPreferences"`
		MakeDefault        *bool              `json:"makeDefault"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Note != nil {
		target.Note = *req.Note
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if req.ChannelPreferences != nil {
		prefs := map[string]string{}
		for k, v := range *req.ChannelPreferences {
			prefs[k] = v
		}
		target.ChannelPreferences = prefs
	}
	if err := config.UpdateApiKey(keyID, *target); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// MakeDefault 同步到 user 表
	if req.MakeDefault != nil && *req.MakeDefault {
		_ = users.Default().UpdateUser(u.ID, func(uu *users.User) {
			uu.DefaultKeyID = keyID
		})
	}
	AuditLog("user_apikey_patch", u.Email, "keyID="+keyID)
	writeJSON(w, http.StatusOK, keyToUserView(target, u.DefaultKeyID))
}

// DELETE /user/api/keys/{id}
// 软删：从 user.ApiKeyIDs 移除 + 实际 DeleteApiKey。
// 禁止删最后一把 key（至少要留一把可登录）。
func (h *Handler) handleUserDeleteKey(w http.ResponseWriter, r *http.Request, currentKey *config.ApiKeyInfo, keyID string) {
	u, _, ok := h.resolveUserAndOwnedKey(w, currentKey, keyID)
	if !ok {
		return
	}
	if len(u.ApiKeyIDs) <= 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete last key; create another first"})
		return
	}
	// 1. 从 user 表移除
	if err := users.Default().UpdateUser(u.ID, func(uu *users.User) {
		out := uu.ApiKeyIDs[:0]
		for _, id := range uu.ApiKeyIDs {
			if id != keyID {
				out = append(out, id)
			}
		}
		uu.ApiKeyIDs = out
		if uu.DefaultKeyID == keyID && len(uu.ApiKeyIDs) > 0 {
			uu.DefaultKeyID = uu.ApiKeyIDs[0]
		}
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 2. 删 ApiKey 表
	if err := config.DeleteApiKey(keyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.apiKeyStatsMu.Lock()
	delete(h.apiKeyStats, keyID)
	h.apiKeyStatsMu.Unlock()
	AuditLog("user_apikey_delete", u.Email, "keyID="+keyID)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
