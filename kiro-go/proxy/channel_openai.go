package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// OpenAIChannel 调用外部 OpenAI 兼容 API。
// APIKey 仅用于 Authorization header，绝不暴露给响应、日志、错误信息。
type OpenAIChannel struct {
	id     string
	cfg    config.ChannelConfig
	client *http.Client
}

func newOpenAIChannel(cfg config.ChannelConfig) *OpenAIChannel {
	return &OpenAIChannel{
		id:     cfg.ID,
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (c *OpenAIChannel) ID() string                       { return c.id }
func (c *OpenAIChannel) Type() string                     { return "openai" }
func (c *OpenAIChannel) SupportsProtocol(p Protocol) bool { return p == ProtocolOpenAI }

func (c *OpenAIChannel) Supports(model string) bool {
	return channelSupportsModel(c.cfg.Models, model)
}

func (c *OpenAIChannel) Execute(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error) {
	if !c.SupportsProtocol(req.Protocol) {
		return nil, fmt.Errorf("openai channel %s: unsupported protocol %q", c.id, req.Protocol)
	}

	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("openai channel %s: base URL not configured", c.id)
	}
	endpoint := base + "/chat/completions"

	// v4: model 别名映射（client 看到的 model 名 → 上游真实 model 名）
	upstreamModel := c.upstreamModel(req.OriginalModel)
	body := req.RawBody
	if upstreamModel != "" && upstreamModel != req.OriginalModel {
		body = rewriteOpenAIRequestModel(body, upstreamModel)
	}
	if req.Stream {
		body = ensureIncludeUsage(body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	// v4: admin 配的 extra header 注入（denylist 保护 Authorization 等关键头不被覆盖）
	applyExtraHeaders(httpReq.Header, c.cfg.ExtraHeaders)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai channel %s: upstream call failed: %w", c.id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// v4: 用结构化 UpstreamHTTPError 让 handler 层透传上游原状态码+body
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &UpstreamHTTPError{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       bodyBytes,
			Chargeable: false,
		}
	}

	result := &ChannelResult{
		ChannelID:   c.id,
		ChannelType: "openai",
		ActualModel: req.OriginalModel, // 永远用 public model 名，不暴露 upstream 别名
		Account:     c.id,
	}

	if req.Stream {
		return c.handleStream(w, resp, result)
	}
	return c.handleNonStream(w, resp, result)
}

func (c *OpenAIChannel) handleNonStream(w http.ResponseWriter, resp *http.Response, result *ChannelResult) (*ChannelResult, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	parseOpenAIChatBody(bodyBytes, result)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(bodyBytes)
	return result, nil
}

// parseOpenAIChatBody 把 OpenAI 非流式响应里的 usage + finish_reason 填到 result。
// 永远不覆盖 ActualModel（保留 caller 设置的 public model 名，避免 alias 穿帮）。
func parseOpenAIChatBody(body []byte, result *ChannelResult) {
	if result == nil {
		return
	}
	var payload struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		result.InputTokens = payload.Usage.PromptTokens
		result.OutputTokens = payload.Usage.CompletionTokens
		if len(payload.Choices) > 0 {
			result.StopReason = payload.Choices[0].FinishReason
		}
	}
	if result.InputTokens == 0 && result.OutputTokens == 0 {
		result.UsageEstimated = true
	}
}

func (c *OpenAIChannel) handleStream(w http.ResponseWriter, resp *http.Response, result *ChannelResult) (*ChannelResult, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return result, err
		}
		parseOpenAIStreamLine(line, result)
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if result.InputTokens == 0 && result.OutputTokens == 0 {
		result.UsageEstimated = true
	}
	return result, nil
}

// parseOpenAIStreamLine 解析单行 SSE 数据，把 usage / finish_reason 写到 result。
// 非 data: 行、心跳、[DONE] 标记都安全 noop。
func parseOpenAIStreamLine(line string, result *ChannelResult) {
	if result == nil || !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}
	var chunk struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return
	}
	if chunk.Usage != nil {
		result.InputTokens = chunk.Usage.PromptTokens
		result.OutputTokens = chunk.Usage.CompletionTokens
	}
	if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
		result.StopReason = chunk.Choices[0].FinishReason
	}
}

// ensureIncludeUsage 在 streaming 请求里强制 stream_options.include_usage=true，
// 保证 final chunk 携带 usage。失败时回退原 body（外部渠道可能不支持，
// 由 handleStream 的兜底估算逻辑处理）。
func ensureIncludeUsage(raw []byte) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	so, _ := m["stream_options"].(map[string]interface{})
	if so == nil {
		so = map[string]interface{}{}
	}
	so["include_usage"] = true
	m["stream_options"] = so
	patched, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return patched
}

// upstreamModel 把客户端看到的 public model 名映射到上游真实 model 名（如果配了 ModelAliases）。
// 没配 alias 时透传 public name。
func (c *OpenAIChannel) upstreamModel(public string) string {
	if len(c.cfg.ModelAliases) == 0 {
		return public
	}
	if v, ok := c.cfg.ModelAliases[public]; ok {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	// 归一化匹配（小写、'-/.'互换）
	target := normalizeChannelModelKey(public)
	for k, v := range c.cfg.ModelAliases {
		if normalizeChannelModelKey(k) == target {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return public
}

// rewriteOpenAIRequestModel 在请求 body 里把 "model" 字段替换为 upstream 真实名。
// JSON parse 失败时返回原 body（让上游报错，不静默掉）。
func rewriteOpenAIRequestModel(raw []byte, upstreamModel string) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	m["model"] = upstreamModel
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// HealthCheckResult 是渠道健康检查的结构化产出。
type HealthCheckResult struct {
	Success       bool     `json:"success"`
	ChannelID     string   `json:"channelId"`
	ChannelType   string   `json:"channelType"`
	ModelsOK      bool     `json:"modelsOk"`
	ChatOK        bool     `json:"chatOk"`
	LatencyMs     int64    `json:"latencyMs"`
	Models        []string `json:"models,omitempty"`        // 上游 /v1/models 返回的列表
	ModelTested   string   `json:"modelTested,omitempty"`   // 实际探测用的 public model 名
	UpstreamModel string   `json:"upstreamModel,omitempty"` // alias 映射后的上游名
	Error         string   `json:"error,omitempty"`
}

// HealthCheck 对 OpenAI 渠道做双探针：GET /v1/models + 1-token chat probe。
// 调用方应该在 admin handler 层加 per-channel cooldown（防 burn 上游配额）。
//
// model 为空时使用 c.cfg.Models[0] 作为探测目标；都为空时跳过 chat probe。
func (c *OpenAIChannel) HealthCheck(ctx context.Context, model string) *HealthCheckResult {
	start := time.Now()
	res := &HealthCheckResult{
		ChannelID:   c.id,
		ChannelType: c.Type(),
	}
	models, modelsErr := c.fetchModels(ctx)
	res.ModelsOK = modelsErr == nil
	res.Models = models

	testModel := strings.TrimSpace(model)
	if testModel == "" && len(c.cfg.Models) > 0 {
		testModel = c.cfg.Models[0]
	}
	res.ModelTested = testModel
	if testModel != "" {
		res.UpstreamModel = c.upstreamModel(testModel)
	}

	var chatErr error
	if testModel != "" {
		chatErr = c.probeChat(ctx, res.UpstreamModel)
	}
	res.ChatOK = chatErr == nil
	res.Success = res.ModelsOK && res.ChatOK && testModel != ""
	res.LatencyMs = time.Since(start).Milliseconds()
	res.Error = joinHealthErrors(modelsErr, chatErr)
	return res
}

func (c *OpenAIChannel) fetchModels(ctx context.Context) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	applyExtraHeaders(req.Header, c.cfg.ExtraHeaders)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyPreview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyPreview)))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

func (c *OpenAIChannel) probeChat(ctx context.Context, upstreamModel string) error {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return fmt.Errorf("base URL not configured")
	}
	probeBody := map[string]interface{}{
		"model":      upstreamModel,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
		"stream":     false,
	}
	raw, _ := json.Marshal(probeBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	applyExtraHeaders(req.Header, c.cfg.ExtraHeaders)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(preview)))
	}
	return nil
}

func joinHealthErrors(errs ...error) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			parts = append(parts, e.Error())
		}
	}
	return strings.Join(parts, "; ")
}
