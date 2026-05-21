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

	body := req.RawBody
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

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai channel %s: upstream call failed: %w", c.id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai channel %s: upstream HTTP %d: %s", c.id, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	result := &ChannelResult{
		ChannelID:   c.id,
		ChannelType: "openai",
		ActualModel: req.OriginalModel,
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
	var payload struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err == nil {
		result.InputTokens = payload.Usage.PromptTokens
		result.OutputTokens = payload.Usage.CompletionTokens
		if payload.Model != "" {
			result.ActualModel = payload.Model
		}
		if len(payload.Choices) > 0 {
			result.StopReason = payload.Choices[0].FinishReason
		}
	}
	if result.InputTokens == 0 && result.OutputTokens == 0 {
		result.UsageEstimated = true
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(bodyBytes)
	return result, nil
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
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data != "" && data != "[DONE]" {
				var chunk struct {
					Model string `json:"model"`
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
					} `json:"usage"`
					Choices []struct {
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
				}
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					if chunk.Usage != nil {
						result.InputTokens = chunk.Usage.PromptTokens
						result.OutputTokens = chunk.Usage.CompletionTokens
					}
					if chunk.Model != "" {
						result.ActualModel = chunk.Model
					}
					if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
						result.StopReason = chunk.Choices[0].FinishReason
					}
				}
			}
		}
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
