package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GET /admin/api/channels — 返回所有渠道，APIKey 自动 mask。
func (h *Handler) apiListChannels(w http.ResponseWriter, _ *http.Request) {
	channels := config.GetChannels()
	for i := range channels {
		channels[i] = publicChannel(channels[i])
	}
	json.NewEncoder(w).Encode(channels)
}

// POST /admin/api/channels — 新建渠道。
func (h *Handler) apiCreateChannel(w http.ResponseWriter, r *http.Request) {
	var ch config.ChannelConfig
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	ch = normalizeChannel(ch)

	channels := config.GetChannels()
	for _, existing := range channels {
		if existing.ID == ch.ID {
			writeAdminJSONError(w, http.StatusConflict, "channel id already exists")
			return
		}
	}
	channels = append(channels, ch)
	if err := validateChannelsConfig(channels); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.UpdateChannels(channels); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.reloadChannelRouter()
	AuditLog("channel_create", "admin", fmt.Sprintf("id=%s type=%s models=%d enabled=%v", ch.ID, ch.Type, len(ch.Models), ch.Enabled))
	json.NewEncoder(w).Encode(publicChannel(ch))
}

// PUT /admin/api/channels/{id} — 修改渠道。
// APIKey 留空或全是掩码字符时保留旧值（避免前端 mask 后回写时清空）。
func (h *Handler) apiUpdateChannel(w http.ResponseWriter, r *http.Request, id string) {
	id = strings.TrimSpace(id)
	var incoming config.ChannelConfig
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	channels := config.GetChannels()
	found := -1
	for i := range channels {
		if channels[i].ID == id {
			found = i
			break
		}
	}
	if found < 0 {
		writeAdminJSONError(w, http.StatusNotFound, "channel not found")
		return
	}

	incoming.ID = id
	if isMaskedOrEmpty(incoming.APIKey) {
		incoming.APIKey = channels[found].APIKey
	}
	incoming = normalizeChannel(incoming)
	// codex audit Warning D: 禁用 channel 之前先验证没有 series 把它作为默认渠道
	if channels[found].Enabled && !incoming.Enabled {
		if err := validateChannelNotDefaultForSeries(id, config.GetSeries()); err != nil {
			writeAdminJSONError(w, http.StatusConflict, err.Error())
			return
		}
	}
	channels[found] = incoming
	if err := validateChannelsConfig(channels); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.UpdateChannels(channels); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.reloadChannelRouter()
	AuditLog("channel_update", "admin", fmt.Sprintf("id=%s type=%s models=%d enabled=%v", incoming.ID, incoming.Type, len(incoming.Models), incoming.Enabled))
	json.NewEncoder(w).Encode(publicChannel(incoming))
}

// DELETE /admin/api/channels/{id}
func (h *Handler) apiDeleteChannel(w http.ResponseWriter, _ *http.Request, id string) {
	id = strings.TrimSpace(id)
	// codex audit Warning D: 删除前先验证没有 series 把它作为默认渠道
	if err := validateChannelNotDefaultForSeries(id, config.GetSeries()); err != nil {
		writeAdminJSONError(w, http.StatusConflict, err.Error())
		return
	}
	channels := config.GetChannels()
	next := make([]config.ChannelConfig, 0, len(channels))
	deleted := false
	for _, ch := range channels {
		if ch.ID == id {
			deleted = true
			continue
		}
		next = append(next, ch)
	}
	if !deleted {
		writeAdminJSONError(w, http.StatusNotFound, "channel not found")
		return
	}
	if err := config.UpdateChannels(next); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.reloadChannelRouter()
	AuditLog("channel_delete", "admin", fmt.Sprintf("id=%s", id))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /admin/api/channels/{id}/test — 测试连通性（外部渠道发 /v1/models 探测）。
func (h *Handler) apiTestChannel(w http.ResponseWriter, _ *http.Request, id string) {
	id = strings.TrimSpace(id)
	var ch *config.ChannelConfig
	channels := config.GetChannels()
	for i := range channels {
		if channels[i].ID == id {
			ch = &channels[i]
			break
		}
	}
	if ch == nil {
		writeAdminJSONError(w, http.StatusNotFound, "channel not found")
		return
	}
	if strings.EqualFold(ch.Type, "kiro") {
		// Kiro 渠道不能探测外部 URL，直接报告配置好的 models
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"type":    "kiro",
			"models":  ch.Models,
		})
		return
	}

	endpoint, err := channelModelsEndpoint(ch.BaseURL)
	if err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(ch.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ch.APIKey))
	}

	start := time.Now()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		writeAdminJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		writeAdminJSONError(w, http.StatusBadGateway, fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
		return
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	var models []string
	if err := json.Unmarshal(body, &parsed); err == nil {
		for _, m := range parsed.Data {
			if strings.TrimSpace(m.ID) != "" {
				models = append(models, m.ID)
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"latencyMs": latencyMs,
		"models":    models,
	})
}

// GET /admin/api/sell-prices — 返回 pricing.SellPrices。
func (h *Handler) apiGetSellPrices(w http.ResponseWriter, _ *http.Request) {
	pricing := config.GetPricing()
	if pricing.SellPrices == nil {
		pricing.SellPrices = map[string]config.ModelSellPrice{}
	}
	json.NewEncoder(w).Encode(pricing.SellPrices)
}

// PUT /admin/api/sell-prices — 全量替换 SellPrices。
func (h *Handler) apiUpdateSellPrices(w http.ResponseWriter, r *http.Request) {
	var input map[string]config.ModelSellPrice
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAdminJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	normalized := make(map[string]config.ModelSellPrice, len(input))
	for model, price := range input {
		key := strings.ToLower(strings.TrimSpace(model))
		if key == "" {
			writeAdminJSONError(w, http.StatusBadRequest, "model name cannot be empty")
			return
		}
		if price.InputPerM < 0 || price.OutputPerM < 0 {
			writeAdminJSONError(w, http.StatusBadRequest, fmt.Sprintf("negative sell price for %s", key))
			return
		}
		if price.InputPerM == 0 && price.OutputPerM == 0 {
			writeAdminJSONError(w, http.StatusBadRequest, fmt.Sprintf("sell price for %s cannot be all zero", key))
			return
		}
		normalized[key] = price
	}
	pricing := config.GetPricing()
	pricing.SellPrices = normalized
	if err := config.UpdatePricing(pricing); err != nil {
		writeAdminJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	AuditLog("sell_prices_update", "admin", fmt.Sprintf("entries=%d", len(normalized)))
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// publicChannel 返回带 mask APIKey 的渠道副本（响应给前端用）。
func publicChannel(ch config.ChannelConfig) config.ChannelConfig {
	ch.APIKey = maskAPIKey(ch.APIKey)
	return ch
}

// normalizeChannel 修正/去重 ChannelConfig 字段。
func normalizeChannel(ch config.ChannelConfig) config.ChannelConfig {
	ch.ID = strings.TrimSpace(ch.ID)
	ch.Type = strings.ToLower(strings.TrimSpace(ch.Type))
	ch.BaseURL = strings.TrimRight(strings.TrimSpace(ch.BaseURL), "/")
	ch.APIKey = strings.TrimSpace(ch.APIKey)
	models := make([]string, 0, len(ch.Models))
	seen := map[string]struct{}{}
	for _, m := range ch.Models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		key := normalizeChannelModelKey(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, m)
	}
	ch.Models = models

	// normalize ModelPrices：key 小写、剔除全 0/负数项、剔除模型不在 Models 列表的孤儿项
	if len(ch.ModelPrices) > 0 {
		modelLookup := map[string]struct{}{}
		for _, m := range models {
			modelLookup[normalizeChannelModelKey(m)] = struct{}{}
		}
		clean := make(map[string]config.ModelSellPrice, len(ch.ModelPrices))
		for k, v := range ch.ModelPrices {
			key := strings.ToLower(strings.TrimSpace(k))
			if key == "" {
				continue
			}
			if v.InputPerM < 0 || v.OutputPerM < 0 {
				continue
			}
			if v.InputPerM == 0 && v.OutputPerM == 0 {
				continue // 全 0 等于没配
			}
			// 只保留实际在 Models 列表里的（容忍 -/. 互换）
			if _, ok := modelLookup[normalizeChannelModelKey(key)]; !ok {
				continue
			}
			clean[key] = v
		}
		if len(clean) > 0 {
			ch.ModelPrices = clean
		} else {
			ch.ModelPrices = nil
		}
	}
	return ch
}

// validateChannelNotDefaultForSeries 阻止删除/禁用作为某个 series default 的渠道。
// 用于 channel DELETE 和 PUT(enabled=false) 前置校验，避免悬空 default。
// 调用方应该在 409 时提示用户先去 Series 页面解绑或换默认。
func validateChannelNotDefaultForSeries(channelID string, series []config.Series) error {
	for _, s := range series {
		if s.DefaultChannelID == channelID {
			return fmt.Errorf("channel %q is the default for series %q; unbind in Series page first", channelID, s.ID)
		}
	}
	return nil
}

// validateChannelsConfig 校验渠道列表整体合法。
//
// v4 双模式：
//   - legacy flat 模式（config.Series=[]）：保留 v3 严格语义 — 同 model 在 enabled 渠道间不能重复（单渠道独占）
//   - v4 series 模式（config.Series!=[]）：允许 duplicate model 跨渠道（不同渠道可服务同 model + 不同价格）；
//     但 ch.SeriesID 必须引用真实存在的 Series.ID。
//
// 调用方应该使用 config.GetSeries() 拿当前 series 列表后传入。
func validateChannelsConfig(channels []config.ChannelConfig) error {
	return validateChannelsConfigWithSeries(channels, config.GetSeries())
}

// validateChannelsConfigWithSeries 显式接受 series 列表（避免测试和调用方的全局状态依赖）。
func validateChannelsConfigWithSeries(channels []config.ChannelConfig, series []config.Series) error {
	seriesMode := len(series) > 0
	seriesIDs := map[string]struct{}{}
	for _, s := range series {
		seriesIDs[s.ID] = struct{}{}
	}

	ids := map[string]struct{}{}
	modelOwners := map[string]string{} // 仅 legacy flat 模式用：同 model 不可重复 enabled
	for _, ch := range channels {
		if ch.ID == "" {
			return fmt.Errorf("channel id is required")
		}
		if _, ok := ids[ch.ID]; ok {
			return fmt.Errorf("duplicate channel id %q", ch.ID)
		}
		ids[ch.ID] = struct{}{}
		switch ch.Type {
		case "kiro", "openai":
		default:
			return fmt.Errorf("unsupported channel type %q", ch.Type)
		}
		if len(ch.Models) == 0 {
			return fmt.Errorf("channel %q must include at least one model", ch.ID)
		}
		if ch.Type == "openai" {
			if _, err := validateBaseURL(ch.BaseURL); err != nil {
				return fmt.Errorf("channel %q: %w", ch.ID, err)
			}
		}
		// v4: channel.SeriesID 必须引用真实 series（如果非空）
		if seriesMode && ch.SeriesID != "" {
			if _, ok := seriesIDs[ch.SeriesID]; !ok {
				return fmt.Errorf("channel %q references unknown series %q", ch.ID, ch.SeriesID)
			}
		}
		if !ch.Enabled {
			continue
		}
		// 同 model 唯一约束：仅 legacy flat 模式启用
		if !seriesMode {
			for _, model := range ch.Models {
				key := normalizeChannelModelKey(model)
				if owner, ok := modelOwners[key]; ok {
					return fmt.Errorf("model %q is already served by channel %q", model, owner)
				}
				modelOwners[key] = ch.ID
			}
		}
	}
	return nil
}

// validateBaseURL 检查 OpenAI 兼容渠道的 baseUrl 合法性。
func validateBaseURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("baseUrl is required")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid baseUrl")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("baseUrl scheme must be http or https")
	}
	return u, nil
}

// channelModelsEndpoint 根据 baseUrl 拼出 /models 探测路径，兼容是否带 /v1。
func channelModelsEndpoint(base string) (string, error) {
	u, err := validateBaseURL(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(u.String(), "/")
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/models", nil
	}
	return endpoint + "/v1/models", nil
}

// maskAPIKey 把 API key mask 成 "sk-***...xxxx"（最后 4 位可见）。
func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 7 {
		return "***"
	}
	last4 := key
	if len(key) > 4 {
		last4 = key[len(key)-4:]
	}
	if strings.HasPrefix(key, "sk-") {
		return "sk-***..." + last4
	}
	return "***..." + last4
}

// isMaskedOrEmpty 判断字符串是空 / 全是 mask 占位符（admin 没改 APIKey 时前端会回传 mask 值）。
func isMaskedOrEmpty(key string) bool {
	key = strings.TrimSpace(key)
	return key == "" || strings.Contains(key, "***") || strings.Contains(key, "...")
}

// writeAdminJSONError 写一个 JSON 错误响应。
func writeAdminJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// writeAdminJSON 写带 status code 的 JSON 响应（v5 新；现有 writeAdminJSONError 只处理错误）。
func writeAdminJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
