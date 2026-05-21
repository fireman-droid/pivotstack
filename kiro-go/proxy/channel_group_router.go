package proxy

import (
	"log"
	"regexp"
	"strings"

	"kiro-api-proxy/config"
)

// ChannelGroup 路由专用文件（v6）。
// 把 router 里跟 ChannelGroup 相关的 struct + 编译 + 路由分支抽出来，
// 让 channel.go 不超过 500 行硬约束。

// compiledChannelGroup 是 ChannelGroup 的预编译形式（matchers 已编译）。
type compiledChannelGroup struct {
	cfg      config.ChannelGroup
	matchers []modelMatcher
}

// compileChannelGroup 优先使用 ModelPatterns 手动配置；
// 若 ModelPatterns 为空 且 Channels 非空，则从挂载渠道的 SupportedModels 派生 exact matchers
// （v7 fallback：避免 admin 漏填 patterns 导致 group 路由静默失败）。
// 返回 mode：
//   - "manual"  : 至少有一条非空 ModelPatterns
//   - "auto"    : ModelPatterns 空，从渠道派生
//   - "invalid" : 两者都空（调用方应跳过并日志）
func compileChannelGroup(g config.ChannelGroup, byID map[string]Channel) (compiledChannelGroup, string) {
	cg := compiledChannelGroup{cfg: g}
	hasManual := false
	for _, p := range g.ModelPatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		hasManual = true
		if strings.HasPrefix(p, "re:") {
			re, err := regexp.Compile(strings.TrimPrefix(p, "re:"))
			if err != nil {
				continue
			}
			cg.matchers = append(cg.matchers, modelMatcher{regex: re})
			continue
		}
		cg.matchers = append(cg.matchers, modelMatcher{prefix: p})
	}
	if hasManual {
		return cg, "manual"
	}
	if len(g.Channels) > 0 {
		seen := map[string]bool{}
		for _, ref := range g.Channels {
			rid := config.RuntimeChannelIDFor(ref)
			ch, ok := byID[rid]
			if !ok {
				continue
			}
			for _, model := range supportedModelsOf(ch) {
				key := normalizeChannelModelKey(model)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				cg.matchers = append(cg.matchers, modelMatcher{exact: model})
			}
		}
		if len(cg.matchers) > 0 {
			return cg, "auto"
		}
	}
	return cg, "invalid"
}

// supportedModelsOf 取 channel 声明支持的模型清单（用于 group auto-derive）。
// 用类型断言而非 interface 方法：保持 Channel interface 稳定。
// 不识别的类型返回 nil（auto-derive 时跳过，不影响路由）。
func supportedModelsOf(ch Channel) []string {
	switch c := ch.(type) {
	case *NewAPIRuntimeChannel:
		return append([]string(nil), c.cfg.Models...)
	case *OpenAIChannel:
		return append([]string(nil), c.cfg.Models...)
	case *KiroChannel:
		return append([]string(nil), c.cfg.Models...)
	}
	return nil
}

// compileAndRegisterChannelGroups 编译并注册 ChannelGroup 列表到 router。
// 跳过软删 / 禁用 / invalid（两端空 + 无渠道派生）项；启动期日志每个 group 的 matcher 模式。
func compileAndRegisterChannelGroups(r *ChannelRouter, groups []config.ChannelGroup) {
	for _, g := range groups {
		if g.DeletedAt != 0 || !g.Enabled {
			continue
		}
		cg, mode := compileChannelGroup(g, r.byID)
		if mode == "invalid" {
			log.Printf("[router] group id=%s SKIPPED reason=no_matcher (modelPatterns empty AND no channels derive)", g.ID)
			continue
		}
		r.channelGroups = append(r.channelGroups, cg)
		r.groupByID[g.ID] = cg
		log.Printf("[router] group id=%s matcher=%s matchers=%d channels=%d default=%s",
			g.ID, mode, len(cg.matchers), len(g.Channels),
			strings.TrimSpace(g.DefaultRuntimeChannelID))
	}
}

// resolveViaGroup v6：用 ChannelGroup + user ChannelPreferences 路由。
// 步骤：
//  1. groupID 来源：hint.GroupID > identifyGroup(model)（ModelPatterns 匹配）；都没 → 返回 false
//  2. group enabled + 未删（NewChannelRouter 已过滤）→ 进入
//  3. user ChannelPreferences[groupID] 命中 → 校验该 runtime channel 仍是 group 成员且 enabled + 协议 + supports model → 用之
//  4. 否则 group.DefaultRuntimeChannelID 校验 + 用之；空则挑第一个可用成员
//  5. 都不通 → 返回 false（让外层 fallback 到 series）
func (r *ChannelRouter) resolveViaGroup(model string, protocol Protocol, hint *ResolveHint) (*ResolveResult, bool) {
	groupID := ""
	if hint != nil {
		groupID = strings.TrimSpace(hint.GroupID)
	}
	if groupID == "" {
		groupID = r.identifyGroup(model)
	}
	if groupID == "" {
		return nil, false
	}
	g, ok := r.groupByID[groupID]
	if !ok {
		return nil, false
	}
	// user 偏好优先
	if hint != nil && len(hint.ChannelPreferences) > 0 {
		if pref := strings.TrimSpace(hint.ChannelPreferences[groupID]); pref != "" {
			if isGroupMember(g.cfg, pref) {
				if ch, ok := r.byID[pref]; ok {
					if (protocol == "" || ch.SupportsProtocol(protocol)) && ch.Supports(model) {
						return &ResolveResult{Channel: ch, GroupID: groupID, ResolvedBy: "group_pref"}, true
					}
				}
			}
		}
	}
	// group 默认
	defaultID := strings.TrimSpace(g.cfg.DefaultRuntimeChannelID)
	if defaultID == "" {
		// 没显式默认，挑第一个 enabled 成员
		for _, ref := range g.cfg.Channels {
			rid := config.RuntimeChannelIDFor(ref)
			if ch, ok := r.byID[rid]; ok {
				if (protocol == "" || ch.SupportsProtocol(protocol)) && ch.Supports(model) {
					return &ResolveResult{Channel: ch, GroupID: groupID, ResolvedBy: "group_default"}, true
				}
			}
		}
		return nil, false
	}
	ch, ok := r.byID[defaultID]
	if !ok {
		return nil, false
	}
	if protocol != "" && !ch.SupportsProtocol(protocol) {
		return nil, false
	}
	if !ch.Supports(model) {
		return nil, false
	}
	return &ResolveResult{Channel: ch, GroupID: groupID, ResolvedBy: "group_default"}, true
}

// identifyGroup 用 ModelPatterns 找出 model 对应的 ChannelGroup ID。
// 第一个命中胜出（admin 应保证 patterns 不歧义；NewChannelRouter 已按 sortOrder 排好）。
// 未命中返回空。
func (r *ChannelRouter) identifyGroup(model string) string {
	normalized := normalizeChannelModelKey(model)
	for _, g := range r.channelGroups {
		for _, m := range g.matchers {
			if m.match(model, normalized) {
				return g.cfg.ID
			}
		}
	}
	return ""
}

// isGroupMember 检查 runtime channel id 是不是 group.Channels 的成员。
func isGroupMember(g config.ChannelGroup, runtimeID string) bool {
	for _, ref := range g.Channels {
		if config.RuntimeChannelIDFor(ref) == runtimeID {
			return true
		}
	}
	return false
}
