package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kiro-api-proxy/config"
)

// publicDirectChannel 是 DirectChannel 对外视图，刻意省略 APIKeyEnc。
type publicDirectChannel struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Alias        string                 `json:"alias"`
	BaseURL      string                 `json:"baseUrl"`
	HasAPIKey    bool                   `json:"hasAPIKey"`
	Models       []string               `json:"models"`
	SellPrice    config.DirectSellPrice `json:"sellPrice"`
	ModelMapping map[string]string      `json:"modelMapping"`
	ExtraHeaders map[string]string      `json:"extraHeaders"`
	Enabled      bool                   `json:"enabled"`
	Status       string                 `json:"status"`
	CreatedAt    int64                  `json:"createdAt"`
	UpdatedAt    int64                  `json:"updatedAt"`
	DeletedAt    int64                  `json:"deletedAt"`
}

// directChannelPatchRequest 用指针字段区分 "未提供" vs "想清空"。
type directChannelPatchRequest struct {
	Alias        *string                 `json:"alias"`
	BaseURL      *string                 `json:"baseUrl"`
	APIKey       *string                 `json:"apiKey"`
	Models       *[]string               `json:"models"`
	SellPrice    *config.DirectSellPrice `json:"sellPrice"`
	ModelMapping *map[string]string      `json:"modelMapping"`
	ExtraHeaders *map[string]string      `json:"extraHeaders"`
	Enabled      *bool                   `json:"enabled"`
	Status       *string                 `json:"status"`
}

func toPublicDirectChannel(c config.DirectChannel) publicDirectChannel {
	return publicDirectChannel{
		ID:           c.ID,
		Type:         c.Type,
		Alias:        c.Alias,
		BaseURL:      c.BaseURL,
		HasAPIKey:    strings.TrimSpace(c.APIKeyEnc) != "",
		Models:       append([]string{}, c.Models...),
		SellPrice:    copyDirectSellPrice(c.SellPrice),
		ModelMapping: copyDirectStringMap(c.ModelMapping),
		ExtraHeaders: copyDirectStringMap(c.ExtraHeaders),
		Enabled:      c.Enabled,
		Status:       c.Status,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		DeletedAt:    c.DeletedAt,
	}
}

// GET /admin/api/direct-channels
func (h *Handler) apiListDirectChannels(w http.ResponseWriter, _ *http.Request) {
	channels := config.GetDirectChannels()
	out := make([]publicDirectChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.DeletedAt > 0 {
			continue
		}
		out = append(out, toPublicDirectChannel(ch))
	}
	writeAdminJSON(w, http.StatusOK, out)
}

// GET /admin/api/direct-channels/{id}
func (h *Handler) apiGetDirectChannel(w http.ResponseWriter, _ *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing direct channel id")
		return
	}
	ch, ok := config.GetDirectChannel(id)
	if !ok || ch.DeletedAt > 0 {
		writeAdminJSONError(w, http.StatusNotFound, "direct channel not found")
		return
	}
	writeAdminJSON(w, http.StatusOK, toPublicDirectChannel(ch))
}

// PATCH /admin/api/direct-channels/{id}
func (h *Handler) apiPatchDirectChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing direct channel id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, directChannelPatchBodyLimit)
	var req directChannelPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ch, ok := config.GetDirectChannel(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "direct channel not found")
		return
	}
	if ch.DeletedAt > 0 {
		writeAdminJSONError(w, http.StatusConflict, fmt.Sprintf("direct channel %q is deleted", id))
		return
	}
	normalizeDirectChannelPatchRequest(&req)
	if err := validateDirectChannelPatchRequest(id, req); err != nil {
		writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
		return
	}
	apiKeyEnc, clearAPIKey, err := directChannelPatchAPIKey(req.APIKey)
	if err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt apiKey")
		return
	}
	updated, err := config.UpdateDirectChannel(id, buildDirectChannelPatch(ch, req, apiKeyEnc))
	if err != nil {
		writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
		return
	}
	if clearAPIKey {
		if err := config.SetDirectChannelAPIKey(id, ""); err != nil {
			writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
			return
		}
		updated.APIKeyEnc = ""
	}
	AuditLog("direct_channel_update", adminAuditActor(r),
		fmt.Sprintf("id=%s alias=%s enabled=%v", updated.ID, updated.Alias, updated.Enabled))
	h.reloadChannelRouter()
	writeAdminJSON(w, http.StatusOK, toPublicDirectChannel(updated))
}

func normalizeDirectChannelPatchRequest(req *directChannelPatchRequest) {
	if req.Alias != nil {
		*req.Alias = strings.TrimSpace(*req.Alias)
	}
	if req.BaseURL != nil {
		*req.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.APIKey != nil {
		*req.APIKey = strings.TrimSpace(*req.APIKey)
	}
	if req.Status != nil {
		*req.Status = strings.TrimSpace(*req.Status)
	}
	if req.Models != nil {
		models := normalizeDirectModels(*req.Models)
		req.Models = &models
	}
	if req.ModelMapping != nil {
		modelMapping := normalizeDirectStringMap(*req.ModelMapping)
		req.ModelMapping = &modelMapping
	}
	if req.ExtraHeaders != nil {
		extraHeaders := normalizeDirectStringMap(*req.ExtraHeaders)
		req.ExtraHeaders = &extraHeaders
	}
}

func validateDirectChannelPatchRequest(id string, req directChannelPatchRequest) error {
	if req.Alias != nil {
		if *req.Alias == "" {
			return fmt.Errorf("alias cannot be empty")
		}
		if err := config.ValidateGroupAliasUnique(id, *req.Alias); err != nil {
			return err
		}
	}
	if req.Status != nil && !directChannelStatusValid(*req.Status) {
		return fmt.Errorf("status must be active, error, or degraded")
	}
	return nil
}

// directChannelPatchAPIKey 把 PATCH 请求中的 apiKey 字段映射到加密结果 + 是否清空标志：
//   - nil → 未提供，返回 ("", false, nil)
//   - "" → 显式清空，返回 ("", true, nil)，由 PATCH handler 之后调用 SetDirectChannelAPIKey("") 完成
//   - 其它 → 加密，返回 (encrypted, false, nil)
func directChannelPatchAPIKey(apiKey *string) (string, bool, error) {
	if apiKey == nil {
		return "", false, nil
	}
	if *apiKey == "" {
		return "", true, nil
	}
	enc, err := config.EncryptSecret(*apiKey)
	if err != nil {
		return "", false, err
	}
	return enc, false, nil
}

// buildDirectChannelPatch 把 PATCH 请求构造成 config.UpdateDirectChannel 接受的 merge-patch。
// existing.Type 必须带回去（UpdateDirectChannel merge 不接受空 Type → validateDirectChannelShape 会 fail）。
// existing.Enabled 作为默认（UpdateDirectChannel.mergeDirectChannelPatch 总是覆盖 Enabled）。
func buildDirectChannelPatch(existing config.DirectChannel, req directChannelPatchRequest, apiKeyEnc string) config.DirectChannel {
	patch := config.DirectChannel{
		Type:    existing.Type,
		Enabled: existing.Enabled,
	}
	if req.Alias != nil {
		patch.Alias = *req.Alias
	}
	if req.BaseURL != nil {
		patch.BaseURL = *req.BaseURL
	}
	if apiKeyEnc != "" {
		patch.APIKeyEnc = apiKeyEnc
	}
	if req.Models != nil {
		patch.Models = append([]string{}, (*req.Models)...)
	}
	if req.SellPrice != nil {
		patch.SellPrice = copyDirectSellPrice(*req.SellPrice)
	}
	if req.ModelMapping != nil {
		patch.ModelMapping = copyDirectStringMap(*req.ModelMapping)
	}
	if req.ExtraHeaders != nil {
		patch.ExtraHeaders = copyDirectStringMap(*req.ExtraHeaders)
	}
	if req.Enabled != nil {
		patch.Enabled = *req.Enabled
	}
	if req.Status != nil {
		patch.Status = *req.Status
	}
	return patch
}

func copyDirectStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyDirectSellPrice(in config.DirectSellPrice) config.DirectSellPrice {
	out := config.DirectSellPrice{Default: in.Default}
	if in.Models != nil {
		out.Models = make(map[string]config.DirectSellPriceRow, len(in.Models))
		for k, v := range in.Models {
			out.Models[k] = v
		}
	}
	return out
}
