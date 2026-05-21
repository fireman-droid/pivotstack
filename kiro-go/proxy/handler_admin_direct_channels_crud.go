package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kiro-api-proxy/config"
)

const (
	directChannelCreateBodyLimit = 32 << 10
	directChannelPatchBodyLimit  = 32 << 10
)

type directChannelCreateRequest struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Alias        string                 `json:"alias"`
	BaseURL      string                 `json:"baseUrl"`
	APIKey       string                 `json:"apiKey"`
	Models       []string               `json:"models"`
	SellPrice    config.DirectSellPrice `json:"sellPrice"`
	ModelMapping map[string]string      `json:"modelMapping"`
	ExtraHeaders map[string]string      `json:"extraHeaders"`
	Enabled      bool                   `json:"enabled"`
}

type directChannelHealthCheckResponse struct {
	Success   bool  `json:"success"`
	LatencyMs int64 `json:"latencyMs"`
}

// POST /admin/api/direct-channels
func (h *Handler) apiCreateDirectChannel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, directChannelCreateBodyLimit)
	var req directChannelCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	normalizeDirectChannelCreateRequest(&req)
	if err := validateDirectChannelCreateRequest(req); err != nil {
		writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
		return
	}
	apiKeyEnc := ""
	if req.Type == "openai" {
		enc, err := config.EncryptSecret(req.APIKey)
		if err != nil {
			writeAdminJSONError(w, http.StatusInternalServerError, "failed to encrypt apiKey")
			return
		}
		apiKeyEnc = enc
	}
	created, err := config.AddDirectChannel(buildDirectChannelFromCreateRequest(req, apiKeyEnc))
	if err != nil {
		writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
		return
	}
	AuditLog("direct_channel_create", adminAuditActor(r),
		fmt.Sprintf("id=%s type=%s alias=%s", created.ID, created.Type, created.Alias))
	h.reloadChannelRouter()
	writeAdminJSON(w, http.StatusCreated, toPublicDirectChannel(created))
}

// DELETE /admin/api/direct-channels/{id}?hard=true|false
// 默认软删（保留 tombstone 防 sync 复活）。hard=true 物理删除。
func (h *Handler) apiDeleteDirectChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		writeAdminJSONError(w, http.StatusBadRequest, "missing direct channel id")
		return
	}
	ch, ok := config.GetDirectChannel(id)
	if !ok {
		writeAdminJSONError(w, http.StatusNotFound, "direct channel not found")
		return
	}
	hard := strings.EqualFold(r.URL.Query().Get("hard"), "true")
	if ch.DeletedAt > 0 && !hard {
		writeAdminJSONError(w, http.StatusNotFound, "direct channel not found")
		return
	}
	if err := config.DeleteDirectChannel(id, hard); err != nil {
		writeAdminJSONError(w, directChannelWriteErrorStatus(err), err.Error())
		return
	}
	AuditLog("direct_channel_delete", adminAuditActor(r), fmt.Sprintf("id=%s hard=%v", id, hard))
	h.reloadChannelRouter()
	writeAdminJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// POST /admin/api/direct-channels/{id}/health-check
// v6 阶段 5：仅校验存在并返回成功；真实探针在后续阶段补完。
func (h *Handler) apiHealthCheckDirectChannel(w http.ResponseWriter, _ *http.Request, id string) {
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
	if !ch.Enabled {
		writeAdminJSONError(w, http.StatusConflict, "channel is disabled; enable it first")
		return
	}
	writeAdminJSON(w, http.StatusOK, directChannelHealthCheckResponse{Success: true, LatencyMs: 0})
}

func normalizeDirectChannelCreateRequest(req *directChannelCreateRequest) {
	req.ID = strings.TrimSpace(req.ID)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.Alias = strings.TrimSpace(req.Alias)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Models = normalizeDirectModels(req.Models)
	req.ModelMapping = normalizeDirectStringMap(req.ModelMapping)
	req.ExtraHeaders = normalizeDirectStringMap(req.ExtraHeaders)
}

func validateDirectChannelCreateRequest(req directChannelCreateRequest) error {
	switch req.Type {
	case "openai":
		if req.APIKey == "" {
			return fmt.Errorf("apiKey required for openai type")
		}
	case "kiro":
	default:
		return fmt.Errorf("direct channel type must be openai or kiro")
	}
	if req.Alias == "" {
		return fmt.Errorf("alias required")
	}
	if req.ID != "" {
		if _, ok := config.GetDirectChannel(req.ID); ok {
			return fmt.Errorf("direct channel id already exists: %s", req.ID)
		}
	}
	return config.ValidateGroupAliasUnique("", req.Alias)
}

func buildDirectChannelFromCreateRequest(req directChannelCreateRequest, apiKeyEnc string) config.DirectChannel {
	return config.DirectChannel{
		ID:           req.ID,
		Type:         req.Type,
		Alias:        req.Alias,
		BaseURL:      req.BaseURL,
		APIKeyEnc:    apiKeyEnc,
		Models:       append([]string{}, req.Models...),
		SellPrice:    copyDirectSellPrice(req.SellPrice),
		ModelMapping: copyDirectStringMap(req.ModelMapping),
		ExtraHeaders: copyDirectStringMap(req.ExtraHeaders),
		Enabled:      req.Enabled,
	}
}

func normalizeDirectModels(in []string) []string {
	out := make([]string, 0, len(in))
	for _, model := range in {
		model = strings.TrimSpace(model)
		if model != "" {
			out = append(out, model)
		}
	}
	return out
}

func normalizeDirectStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key != "" {
			out[key] = strings.TrimSpace(v)
		}
	}
	return out
}

func directChannelStatusValid(status string) bool {
	switch status {
	case "", "active", "error", "degraded":
		return true
	default:
		return false
	}
}

// directChannelWriteErrorStatus 把 config 层错误归类成 HTTP status — 跨 create/patch/delete 共用。
func directChannelWriteErrorStatus(err error) int {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "conflict"),
		strings.Contains(msg, "duplicate"),
		strings.Contains(msg, "already exists"),
		strings.Contains(msg, "already used"):
		return http.StatusConflict
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "required"),
		strings.Contains(msg, "must be"),
		strings.Contains(msg, "negative"),
		strings.Contains(msg, "invalid"),
		strings.Contains(msg, "cannot be empty"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
