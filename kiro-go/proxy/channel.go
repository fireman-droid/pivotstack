package proxy

import (
	"context"
	"kiro-api-proxy/config"
	"net/http"
	"regexp"
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

// ResolveHint 是 Resolve 的附加路由提示（v4 新增）。
//   - Protocol: 必须，调用方约束渠道协议兼容性
//   - SeriesID: admin/test override；为空时由 identifySeries(model) 自动推断
//   - ChannelID: 显式指定渠道（X-Pivotstack-Channel header / 调试场景），优先级最高，绕过 series 路由
//   - UserPreferences: 用户面板保存的 per-series 渠道偏好快照（seriesID → channelID）
type ResolveHint struct {
	Protocol Protocol
	// v5 兼容：series 路径
	SeriesID        string
	UserPreferences map[string]string // seriesID → channelID (deprecated, v5 兜底)
	// v6 新增：ChannelGroup 路径（admin 自定义分组 + user 选具体 channel）
	GroupID            string
	ChannelPreferences map[string]string // groupID → runtime channel id
	// 显式渠道（X-Pivotstack-Channel header），优先级最高
	ChannelID string
}

// ResolveResult 携带 ResolveDetailed 的完整路由决策，便于调用方区分错误语义。
type ResolveResult struct {
	Channel    Channel
	SeriesID   string
	GroupID    string
	ResolvedBy string // "explicit" | "group_pref" | "group_default" | "user_pref" | "series_default" | "legacy_flat"
}

// ChannelRouter v4/v6 路由器。
//
// legacyFlat=true（Series=[] 时）→ 走 v3 flat 路由：遍历 channels 找第一个支持 model 的；
// legacyFlat=false（v4 series 模式）→ model → series → DefaultChannelID 路由。
// v6：ChannelGroups 非空时优先走分组路由（explicit > group > legacy > series 兜底）。
type ChannelRouter struct {
	channels      []Channel
	byID          map[string]Channel
	series        []compiledSeries
	seriesByID    map[string]compiledSeries
	channelGroups []compiledChannelGroup
	groupByID     map[string]compiledChannelGroup
	legacyFlat    bool
}

// compiledChannelGroup / compileChannelGroup 在 channel_group_router.go 实现。

// compiledSeries 是 Series 的预编译形式（matcher 已编译，避免每次请求重做正则）。
type compiledSeries struct {
	cfg      config.Series
	matchers []modelMatcher
}

// modelMatcher 匹配策略：exact / prefix / regex。
// exact 匹配走归一化后等值（ChannelGroup auto-derive 自渠道 SupportedModels 用）；
// prefix 匹配走归一化（小写、'-/.' 互换、去 thinking 后缀）；
// regex 走原文本（让用户自己控制）。
type modelMatcher struct {
	prefix string
	exact  string
	regex  *regexp.Regexp
}

func (m modelMatcher) match(rawModel, normalizedModel string) bool {
	if m.regex != nil {
		return m.regex.MatchString(rawModel)
	}
	if m.exact != "" {
		return normalizeChannelModelKey(m.exact) == normalizedModel
	}
	if m.prefix != "" {
		// 把 prefix 也归一化后再 HasPrefix 检查
		return strings.HasPrefix(normalizedModel, normalizeChannelModelKey(m.prefix))
	}
	return false
}

// compileSeries 把 config.Series 编译成 compiledSeries（提前 regex.Compile）。
// 错误的 pattern 静默跳过（避免一个错配置打挂整个 router）。
func compileSeries(s config.Series) compiledSeries {
	cs := compiledSeries{cfg: s}
	for _, p := range s.ModelPatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "re:") {
			re, err := regexp.Compile(strings.TrimPrefix(p, "re:"))
			if err != nil {
				continue
			}
			cs.matchers = append(cs.matchers, modelMatcher{regex: re})
			continue
		}
		cs.matchers = append(cs.matchers, modelMatcher{prefix: p})
	}
	return cs
}

// RouterOption 用 functional options 把 v5 NewAPI 渠道注入 router，
// 旧的 3-arg 调用点（测试 + legacy）保持源码兼容，不需要改签名传 nil。
type RouterOption func(*routerOptions)

type routerOptions struct {
	directChannels  []config.DirectChannel
	newAPIChannels  []config.NewAPIChannel
	newAPIProviders []config.NewAPIProvider
	channelGroups   []config.ChannelGroup
}

// WithDirectChannels 注入 v6 自营直连渠道。openai 类型在注册时 decrypt APIKeyEnc；
// kiro 类型借用 KiroExecutor，账号池仍走全局 accounts 配置。
func WithDirectChannels(channels []config.DirectChannel) RouterOption {
	return func(o *routerOptions) { o.directChannels = channels }
}

// WithNewAPIChannels 注入由上游同步物化出来的 v5 channel 列表。
func WithNewAPIChannels(channels []config.NewAPIChannel) RouterOption {
	return func(o *routerOptions) { o.newAPIChannels = channels }
}

// WithNewAPIProviders 注入 v5 provider 元数据（提供 BaseURL 等运行时上下文）。
func WithNewAPIProviders(providers []config.NewAPIProvider) RouterOption {
	return func(o *routerOptions) { o.newAPIProviders = providers }
}

// WithChannelGroups 注入 v6 ChannelGroup（admin 自定义分组）。
// 空 = 走 series/legacy 兜底路径；非空 = 路由优先级 explicit > group > legacy > series。
func WithChannelGroups(groups []config.ChannelGroup) RouterOption {
	return func(o *routerOptions) { o.channelGroups = groups }
}

// NewChannelRouter v4/v6：同时接收 legacy series/channels、v5 NewAPI 物化渠道和 v6 DirectChannel。
//
// series 为空时启用 legacyFlat 模式（行为完全等价 v3）。
// v6 起 NewAPIChannel / DirectChannel 在 legacyFlat 与 series 模式下都注册：
// 前者从 admin POST /admin/api/newapi/channels 物化而来；
// 后者是 admin 直营 openai / kiro 渠道。
// 禁用、已删、APIKey 解密失败的渠道单独跳过，不阻断启动。
func NewChannelRouter(seriesCfg []config.Series, channelsCfg []config.ChannelConfig, exec KiroExecutor, opts ...RouterOption) *ChannelRouter {
	var ro routerOptions
	for _, opt := range opts {
		opt(&ro)
	}

	r := &ChannelRouter{
		byID:       map[string]Channel{},
		seriesByID: map[string]compiledSeries{},
		groupByID:  map[string]compiledChannelGroup{},
		legacyFlat: len(seriesCfg) == 0,
	}

	for _, s := range seriesCfg {
		cs := compileSeries(s)
		r.series = append(r.series, cs)
		r.seriesByID[s.ID] = cs
	}

	// 必须先注册所有 channel（kiro / openai / newapi / direct），
	// 再编译 ChannelGroup —— group 在 ModelPatterns 为空时会从挂载的
	// runtime channel 派生 SupportedModels（exact matcher），依赖 r.byID 已就绪。
	for _, c := range channelsCfg {
		if !c.Enabled {
			continue
		}
		var ch Channel
		switch strings.ToLower(strings.TrimSpace(c.Type)) {
		case "kiro":
			ch = newKiroChannel(c, exec)
		case "openai":
			ch = newOpenAIChannel(c)
		default:
			continue
		}
		r.channels = append(r.channels, ch)
		r.byID[ch.ID()] = ch
	}

	registerNewAPIChannels(r, ro.newAPIChannels, ro.newAPIProviders)
	registerDirectChannels(r, ro.directChannels, exec)

	// v6 ChannelGroup：跳过软删 / 禁用项；剩余按 manual patterns 或 auto-derive 编译。
	compileAndRegisterChannelGroups(r, ro.channelGroups)

	return r
}

// Resolve 是 ResolveDetailed 的薄 wrapper，返回值简化为 (channel, ok)。
// 保留以维持已有调用点的语义；新代码若需要区分路由来源，请直接调 ResolveDetailed。
func (r *ChannelRouter) Resolve(model string, hint *ResolveHint) (Channel, bool) {
	rr, ok := r.ResolveDetailed(model, hint)
	if !ok {
		return nil, false
	}
	return rr.Channel, true
}

// ResolveDetailed v5 路由查找，返回决策来源便于上层做精确错误响应。
//
// 优先级：
//  1. hint.ChannelID != "" → 显式渠道（X-Pivotstack-Channel header / 调试），绕过 series。失败不 fallback。
//  2. legacyFlat=true → v3 flat：遍历找第一个支持 model 的（ResolvedBy="legacy_flat"）
//  3. v4/v5 series 模式：
//     a. seriesID = hint.SeriesID > identifySeries(model)；为空时整体返回 false
//     b. 若 hint.UserPreferences[seriesID] 命中且 channel 通过协议/model/series 校验 → ResolvedBy="user_pref"
//     c. 否则取 series.DefaultChannelID + 校验 → ResolvedBy="series_default"
//     d. 不 fallback 到任意支持 model 的非 default channel
func (r *ChannelRouter) ResolveDetailed(model string, hint *ResolveHint) (*ResolveResult, bool) {
	if r == nil {
		return nil, false
	}

	var protocol Protocol
	if hint != nil {
		protocol = hint.Protocol
	}

	if hint != nil && strings.TrimSpace(hint.ChannelID) != "" {
		ch, ok := r.resolveExplicit(model, protocol, strings.TrimSpace(hint.ChannelID))
		if !ok {
			return nil, false
		}
		return &ResolveResult{Channel: ch, SeriesID: r.resultSeriesID(model, hint), ResolvedBy: "explicit"}, true
	}

	// v6 ChannelGroup 路由：admin 自定义分组的命中优先于 series。
	// 仅当 ChannelGroups 非空（admin 已经迁移到 v6 模式）时启用；空时跳过让 series 兜底。
	if len(r.channelGroups) > 0 {
		if rr, ok := r.resolveViaGroup(model, protocol, hint); ok {
			return rr, true
		}
		// group 路由未命中（model 没对应 group 或 group 内 channel 全失效）→ 继续走 series 兜底
	}

	if r.legacyFlat {
		ch, ok := r.resolveLegacyFlat(model, protocol)
		if !ok {
			return nil, false
		}
		return &ResolveResult{Channel: ch, ResolvedBy: "legacy_flat"}, true
	}

	seriesID := ""
	if hint != nil {
		seriesID = strings.TrimSpace(hint.SeriesID)
	}
	if seriesID == "" {
		seriesID = r.identifySeries(model)
	}
	if seriesID == "" {
		return nil, false
	}

	if hint != nil {
		if preferred := strings.TrimSpace(hint.UserPreferences[seriesID]); preferred != "" {
			if ch, ok := r.byID[preferred]; ok {
				protocolOK := protocol == "" || ch.SupportsProtocol(protocol)
				if protocolOK && ch.Supports(model) && channelAllowedForSeries(ch, seriesID) {
					return &ResolveResult{Channel: ch, SeriesID: seriesID, ResolvedBy: "user_pref"}, true
				}
			}
		}
	}

	s, ok := r.seriesByID[seriesID]
	if !ok || s.cfg.DefaultChannelID == "" {
		return nil, false
	}

	ch, ok := r.byID[s.cfg.DefaultChannelID]
	if !ok {
		return nil, false // 默认渠道不存在或已禁用
	}
	if protocol != "" && !ch.SupportsProtocol(protocol) {
		return nil, false
	}
	if !ch.Supports(model) {
		return nil, false // Phase 1 不 fallback
	}
	if !channelAllowedForSeries(ch, seriesID) {
		return nil, false
	}
	return &ResolveResult{Channel: ch, SeriesID: seriesID, ResolvedBy: "series_default"}, true
}

func (r *ChannelRouter) resultSeriesID(model string, hint *ResolveHint) string {
	if hint != nil && strings.TrimSpace(hint.SeriesID) != "" {
		return strings.TrimSpace(hint.SeriesID)
	}
	return r.identifySeries(model)
}

// channelAllowedForSeries 对所有带 SeriesID 的运行时渠道做 series 绑定校验。
// SeriesID 为空表示 legacy/未绑定渠道 → 通过（向后兼容 v3 flat + 早期 v4 配置）。
// 防御场景：persisted config 或外部导入的 user preferences 把 manual 渠道指给错的 series，
// 应在路由层兜底拒绝（SetApiKeySeriesPreferences 只覆盖 user API 入口）。
func channelAllowedForSeries(ch Channel, seriesID string) bool {
	switch c := ch.(type) {
	case *NewAPIRuntimeChannel:
		return c.cfg.SeriesID == "" || c.cfg.SeriesID == seriesID
	case *OpenAIChannel:
		return c.cfg.SeriesID == "" || c.cfg.SeriesID == seriesID
	case *KiroChannel:
		return c.cfg.SeriesID == "" || c.cfg.SeriesID == seriesID
	default:
		return true
	}
}

// resolveExplicit 用 channelID 直接查找（不做 series 推断）。
func (r *ChannelRouter) resolveExplicit(model string, protocol Protocol, channelID string) (Channel, bool) {
	ch, ok := r.byID[channelID]
	if !ok {
		return nil, false
	}
	if protocol != "" && !ch.SupportsProtocol(protocol) {
		return nil, false
	}
	if model != "" && !ch.Supports(model) {
		return nil, false
	}
	return ch, true
}

// resolveLegacyFlat 完全等价 v3 行为：遍历找第一个支持 model 的 channel。
func (r *ChannelRouter) resolveLegacyFlat(model string, protocol Protocol) (Channel, bool) {
	for _, c := range r.channels {
		if protocol != "" && !c.SupportsProtocol(protocol) {
			continue
		}
		if c.Supports(model) {
			return c, true
		}
	}
	return nil, false
}

// identifySeries 根据 model 名遍历 series 的 matchers 找到所属 series。
// 第一个匹配的 series 胜出（admin 应避免歧义 patterns）。
// 未匹配返回空字符串。
//
// prefix 比对走归一化（避免 "claude-" 模式漏掉 "claude.3.5" 这种 '.'/'-' 互换写法
// 或 "opus-4.6-thinking" 这种 thinking 后缀）；regex 比对走原文本由用户控制。
func (r *ChannelRouter) identifySeries(model string) string {
	normalized := normalizeChannelModelKey(model)
	for _, s := range r.series {
		for _, m := range s.matchers {
			if m.match(model, normalized) {
				return s.cfg.ID
			}
		}
	}
	return ""
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

// ChannelByID 按 ID 查找渠道（admin health-check 用）。
func (r *ChannelRouter) ChannelByID(id string) (Channel, bool) {
	if r == nil {
		return nil, false
	}
	ch, ok := r.byID[id]
	return ch, ok
}

// LegacyFlat 返回当前是否处于 legacy flat 模式（admin UI 显示用）。
func (r *ChannelRouter) LegacyFlat() bool {
	return r != nil && r.legacyFlat
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
