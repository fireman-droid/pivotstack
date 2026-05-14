package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
)

// KiroExecutor 是 Kiro 渠道的窄接口，由 Handler 实现。
// 避免渠道层反向持有 *Handler，保持解耦 + 易于测试。
type KiroExecutor interface {
	executeKiroChat(ctx context.Context, w http.ResponseWriter,
		req *OpenAIRequest, body []byte, uc *UserContext, requestID string,
	) (*ChannelResult, error)
	executeKiroClaude(ctx context.Context, w http.ResponseWriter,
		req *ClaudeRequest, body []byte, uc *UserContext, requestID string,
	) (*ChannelResult, error)
}

// KiroChannel 包装 Kiro 执行流。渠道层只负责协议分发，不直接持有账号池。
type KiroChannel struct {
	id   string
	cfg  config.ChannelConfig
	exec KiroExecutor
}

func newKiroChannel(cfg config.ChannelConfig, exec KiroExecutor) *KiroChannel {
	id := cfg.ID
	if id == "" {
		id = "kiro-default"
	}
	return &KiroChannel{id: id, cfg: cfg, exec: exec}
}

func (c *KiroChannel) ID() string                       { return c.id }
func (c *KiroChannel) Type() string                     { return "kiro" }
func (c *KiroChannel) SupportsProtocol(p Protocol) bool { return p == ProtocolOpenAI || p == ProtocolClaude }

func (c *KiroChannel) Supports(model string) bool {
	return channelSupportsModel(c.cfg.Models, model)
}

func (c *KiroChannel) Execute(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error) {
	if c.exec == nil {
		return nil, fmt.Errorf("KiroChannel.Execute: executor not configured")
	}
	var (
		result *ChannelResult
		err    error
	)
	switch req.Protocol {
	case ProtocolOpenAI:
		var oaReq OpenAIRequest
		if uerr := json.Unmarshal(req.RawBody, &oaReq); uerr != nil {
			return nil, fmt.Errorf("invalid openai request body: %w", uerr)
		}
		result, err = c.exec.executeKiroChat(ctx, w, &oaReq, req.RawBody, req.UserContext, req.RequestID)
	case ProtocolClaude:
		var clReq ClaudeRequest
		if uerr := json.Unmarshal(req.RawBody, &clReq); uerr != nil {
			return nil, fmt.Errorf("invalid claude request body: %w", uerr)
		}
		result, err = c.exec.executeKiroClaude(ctx, w, &clReq, req.RawBody, req.UserContext, req.RequestID)
	default:
		return nil, fmt.Errorf("KiroChannel: unsupported protocol %q", req.Protocol)
	}
	if result != nil {
		result.ChannelID = c.id
		result.ChannelType = c.Type()
	}
	return result, err
}
