package proxy

import (
	"context"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
)

// Protocol 标识请求协议（影响响应格式与翻译）。
type Protocol string

const (
	ProtocolClaude Protocol = "claude"
	ProtocolOpenAI Protocol = "openai"
)

// ChannelRequest 是渠道执行所需的最小输入集。
// 渠道层不感知计费，billing 由 handler 层负责。
type ChannelRequest struct {
	Protocol      Protocol
	OriginalModel string
	Stream        bool
	RawBody       []byte
	RequestID     string
	UserContext   *UserContext
}

// ChannelResult 是渠道执行的产出，供 billing 层使用。
// UpstreamCredits 仅 Kiro 渠道有值（用于成本/利润追踪），外部渠道恒为 0。
type ChannelResult struct {
	ChannelID       string
	ChannelType     string
	ActualModel     string
	Account         string
	InputTokens     int
	OutputTokens    int
	UpstreamCredits float64
	UsageEstimated  bool
	StopReason      string
	BillingModel    string
	Subscription    string
	RequestID       string
	DurationMs      int64
	PayloadKB       int
}

// Channel 是上游 API 渠道抽象。
type Channel interface {
	ID() string
	Type() string
	Supports(model string) bool
	SupportsProtocol(p Protocol) bool
	Execute(ctx context.Context, w http.ResponseWriter, req ChannelRequest) (*ChannelResult, error)
}

// ChannelRouter 按 model 路由到合适的渠道。
// channels 为空时 HasChannels() 返回 false，handler 走 legacy Kiro 路径。
type ChannelRouter struct {
	channels []Channel
}

// NewChannelRouter 从 config.Channels 构建路由器。
// 禁用的渠道被跳过；未知类型被忽略（不阻断启动）。
func NewChannelRouter(cfgChannels []config.ChannelConfig, exec KiroExecutor) *ChannelRouter {
	r := &ChannelRouter{}
	for _, c := range cfgChannels {
		if !c.Enabled {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(c.Type)) {
		case "kiro":
			r.channels = append(r.channels, newKiroChannel(c, exec))
		case "openai":
			r.channels = append(r.channels, newOpenAIChannel(c))
		}
	}
	return r
}

// Resolve 返回支持指定 model 的第一个渠道。
// P1-P4 阶段不允许多渠道支持同一 model（admin 接口需做去重校验）。
func (r *ChannelRouter) Resolve(model string) (Channel, bool) {
	if r == nil {
		return nil, false
	}
	for _, c := range r.channels {
		if c.Supports(model) {
			return c, true
		}
	}
	return nil, false
}

// HasChannels 决定 handler 是否走渠道分发。空 = legacy fallback。
func (r *ChannelRouter) HasChannels() bool {
	return r != nil && len(r.channels) > 0
}

// Channels 返回当前已注册的渠道副本（admin 列表/调试用）。
func (r *ChannelRouter) Channels() []Channel {
	if r == nil {
		return nil
	}
	out := make([]Channel, len(r.channels))
	copy(out, r.channels)
	return out
}

// channelSupportsModel 在渠道配置的 Models 列表里查 model（归一化匹配）。
func channelSupportsModel(models []string, model string) bool {
	if model == "" || len(models) == 0 {
		return false
	}
	target := normalizeChannelModelKey(model)
	for _, m := range models {
		if normalizeChannelModelKey(m) == target {
			return true
		}
	}
	return false
}

// normalizeChannelModelKey 把 model 名标准化：小写、'-' ↔ '.' 互换、剥离 thinking 后缀。
func normalizeChannelModelKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "-thinking")
	s = strings.TrimSuffix(s, "-think")
	return strings.ReplaceAll(s, "-", ".")
}
