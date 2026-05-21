package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kiro-api-proxy/config"
)

// newAPIRuntimeHTTPClient 是所有 NewAPIRuntimeChannel 共享的 HTTP client，
// 10 分钟超时覆盖长 chat completions（含 reasoning）的最坏情况。
var newAPIRuntimeHTTPClient = &http.Client{Timeout: 10 * time.Minute}

// NewAPIRuntimeChannel 由上游 new-api sk-* token 物化出来的 PivotStack 渠道。
// cfg + provider 均为请求级快照，避免锁外读写共享配置。
type NewAPIRuntimeChannel struct {
	cfg      config.NewAPIChannel
	provider config.NewAPIProvider
	client   *http.Client
}

func newNewAPIRuntimeChannel(cfg config.NewAPIChannel, p config.NewAPIProvider) *NewAPIRuntimeChannel {
	if len(cfg.Models) > 0 {
		cfg.Models = append([]string(nil), cfg.Models...)
	}
	return &NewAPIRuntimeChannel{cfg: cfg, provider: p, client: newAPIRuntimeHTTPClient}
}

func (c *NewAPIRuntimeChannel) ID() string                       { return c.cfg.ID }
func (c *NewAPIRuntimeChannel) Type() string                     { return "newapi" }
func (c *NewAPIRuntimeChannel) SupportsProtocol(p Protocol) bool { return p == ProtocolOpenAI }
func (c *NewAPIRuntimeChannel) ProviderID() string               { return c.cfg.ProviderID }
func (c *NewAPIRuntimeChannel) Markup() float64                  { return c.cfg.Markup }
func (c *NewAPIRuntimeChannel) QuotaPerUnitDollar() float64      { return c.provider.QuotaPerUnitDollar }
func (c *NewAPIRuntimeChannel) YuanPerUpstreamDollar() float64   { return c.provider.YuanPerUpstreamDollar }
func (c *NewAPIRuntimeChannel) UpstreamTokenID() int             { return c.cfg.UpstreamTokenID }
func (c *NewAPIRuntimeChannel) GroupName() string                { return c.cfg.GroupName }

func (c *NewAPIRuntimeChannel) Supports(model string) bool {
	return channelSupportsModel(c.cfg.Models, model)
}

func (c *NewAPIRuntimeChannel) Execute(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error) {
	start := time.Now()
	if !c.SupportsProtocol(req.Protocol) {
		return nil, fmt.Errorf("newapi channel %s: unsupported protocol %q", c.cfg.ID, req.Protocol)
	}
	base := strings.TrimRight(strings.TrimSpace(c.provider.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("newapi channel %s: provider base url not configured", c.cfg.ID)
	}
	upstreamKey, err := config.DecryptSecret(c.cfg.UpstreamKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("newapi channel %s: decrypt upstream key: %w", c.cfg.ID, err)
	}

	body := req.RawBody
	if req.Stream {
		body = ensureIncludeUsage(body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+upstreamKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	client := c.client
	if client == nil {
		client = newAPIRuntimeHTTPClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("newapi channel %s: upstream call failed: %w", c.cfg.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		// 防御：坏 / 恶意上游可能把 Authorization 原样 echo 进响应 body，
		// 而 handler_channel.go 会 _w.Write(up.Body)_ 透传给客户端 + 日志记录 execErr.Error()，
		// 导致用户的真实 sk-* key 泄露给调用方/日志。强制 redact 之后再装箱。
		bodyBytes = redactUpstreamSecret(bodyBytes, upstreamKey)
		return nil, &UpstreamHTTPError{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       bodyBytes,
			Chargeable: false,
		}
	}

	result := &ChannelResult{
		ChannelID:   c.cfg.ID,
		ChannelType: "newapi",
		ActualModel: req.OriginalModel,
		Account:     c.provider.ID,
		RequestID:   req.RequestID,
		PayloadKB:   len(req.RawBody) / 1024,
	}
	if req.Stream {
		result, err = c.handleStream(w, resp, result)
	} else {
		result, err = c.handleNonStream(w, resp, result)
	}
	if result != nil {
		result.DurationMs = time.Since(start).Milliseconds()
	}
	return result, err
}

func (c *NewAPIRuntimeChannel) handleNonStream(w http.ResponseWriter, resp *http.Response, result *ChannelResult) (*ChannelResult, error) {
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

// HealthCheck v5 渠道双探针：解密 sk-* → GET /v1/models + 1-token chat probe。
// admin 配完上游后用来确认连通性 + 模型可用性，不真扣余额。
// 调用方应在 admin handler 层加 per-channel 30s 冷却（避免 burn 上游配额）。
//
// model 为空时：自动取 cfg.Models[0] 作为探测目标；都为空时跳过 chat probe。
func (c *NewAPIRuntimeChannel) HealthCheck(ctx context.Context, model string) *HealthCheckResult {
	start := time.Now()
	res := &HealthCheckResult{
		ChannelID:   c.cfg.ID,
		ChannelType: c.Type(),
	}
	base := strings.TrimRight(strings.TrimSpace(c.provider.BaseURL), "/")
	if base == "" {
		res.Error = "provider base url not configured"
		res.LatencyMs = time.Since(start).Milliseconds()
		return res
	}
	upstreamKey, err := config.DecryptSecret(c.cfg.UpstreamKeyEnc)
	if err != nil {
		res.Error = fmt.Sprintf("decrypt upstream key: %v", err)
		res.LatencyMs = time.Since(start).Milliseconds()
		return res
	}

	models, modelsErr := c.fetchHealthModels(ctx, base, upstreamKey)
	res.ModelsOK = modelsErr == nil
	res.Models = models

	testModel := strings.TrimSpace(model)
	if testModel == "" && len(c.cfg.Models) > 0 {
		testModel = c.cfg.Models[0]
	}
	res.ModelTested = testModel
	res.UpstreamModel = testModel // v5 不做 alias 映射，public name 直接当上游 model

	var chatErr error
	if testModel != "" {
		chatErr = c.probeHealthChat(ctx, base, upstreamKey, testModel)
	}
	res.ChatOK = chatErr == nil
	res.Success = res.ModelsOK && res.ChatOK && testModel != ""
	res.LatencyMs = time.Since(start).Milliseconds()
	res.Error = joinHealthErrors(modelsErr, chatErr)
	return res
}

func (c *NewAPIRuntimeChannel) fetchHealthModels(ctx context.Context, base, upstreamKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+upstreamKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(redactPreview(preview, upstreamKey)))
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

func (c *NewAPIRuntimeChannel) probeHealthChat(ctx context.Context, base, upstreamKey, upstreamModel string) error {
	probeBody := map[string]interface{}{
		"model":      upstreamModel,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
		"stream":     false,
	}
	raw, _ := json.Marshal(probeBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+upstreamKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(redactPreview(preview, upstreamKey)))
	}
	return nil
}

// redactPreview 防止上游 echo Authorization header 时把 sk-* 通过 health-check error message 泄露给 admin。
func redactPreview(body []byte, secret string) string {
	if secret == "" {
		return string(body)
	}
	return strings.ReplaceAll(string(body), secret, "[REDACTED]")
}

func (c *NewAPIRuntimeChannel) handleStream(w http.ResponseWriter, resp *http.Response, result *ChannelResult) (*ChannelResult, error) {
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

// redactUpstreamSecret 在 secret 非空时把 body 内出现的密钥替换为 [REDACTED]。
// 只对短期解密快照做替换，不写日志，调用方确保 secret != ""（否则 noop）。
func redactUpstreamSecret(body []byte, secret string) []byte {
	if len(body) == 0 || secret == "" {
		return body
	}
	return bytes.ReplaceAll(body, []byte(secret), []byte("[REDACTED]"))
}
