package proxy

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"kiro-api-proxy/config"
)

// 模型映射（有序，长 key 优先匹配，避免 "claude-sonnet-4" 误匹配 "claude-sonnet-4.5"）
type modelMapping struct {
	key   string
	value string
}

var modelMapOrdered = []modelMapping{
	{"claude-sonnet-4-20250514", "claude-sonnet-4"},
	// 4.7 系列：Anthropic 只发布了 opus-4.7（无 sonnet-4.7）。
	//   - opus-4.7：上游 Kiro/Amazon Q 已支持，直传 canonical "claude-opus-4.7"
	//   - sonnet "4.7" 实际不存在 → fallback 到 claude-sonnet-4.6（最近降级）
	{"claude-sonnet-4-7", "claude-sonnet-4.6"},
	{"claude-sonnet-4.7", "claude-sonnet-4.6"},
	{"claude-opus-4-7", "claude-opus-4.7"},
	{"claude-opus-4.7", "claude-opus-4.7"},
	{"claude-sonnet-4-5", "claude-sonnet-4.5"},
	{"claude-sonnet-4.5", "claude-sonnet-4.5"},
	{"claude-sonnet-4-6", "claude-sonnet-4.6"},
	{"claude-sonnet-4.6", "claude-sonnet-4.6"},
	{"claude-haiku-4-5", "claude-haiku-4.5"},
	{"claude-haiku-4.5", "claude-haiku-4.5"},
	{"claude-opus-4-5", "claude-opus-4.5"},
	{"claude-opus-4.5", "claude-opus-4.5"},
	{"claude-opus-4-6", "claude-opus-4.6"},
	{"claude-opus-4.6", "claude-opus-4.6"},
	{"claude-sonnet-4", "claude-sonnet-4"},
	{"claude-3-5-sonnet", "claude-sonnet-4.5"},
	{"claude-3-opus", "claude-sonnet-4.5"},
	{"claude-3-sonnet", "claude-sonnet-4"},
	{"claude-3-haiku", "claude-haiku-4.5"},
	{"gpt-4-turbo", "claude-sonnet-4.5"},
	{"gpt-4o", "claude-sonnet-4.5"},
	{"gpt-4", "claude-sonnet-4.5"},
	{"gpt-3.5-turbo", "claude-sonnet-4.5"},
}

// buildThinkingModePrompt 构建 thinking 模式标签，支持自定义 budget_tokens
func buildThinkingModePrompt(budgetTokens int) string {
	if budgetTokens <= 0 {
		budgetTokens = 200000
	}
	return fmt.Sprintf("<thinking_mode>interleaved</thinking_mode>\n<max_thinking_length>%d</max_thinking_length>", budgetTokens)
}

const minimalFallbackUserContent = "."

const SystemPromptReinforcement = `

CRITICAL RULES YOU MUST FOLLOW:
1. OBEY USER COMMANDS: Execute user instructions exactly as given. Do NOT reinterpret, skip, or substitute your own judgment.
2. TOOL USAGE: When tools (including MCP tools) are available and relevant, you MUST call them immediately and proactively. Never skip tool calls. Never ask permission to use tools — just use them. Only report back to the user if a tool call fails or produces an error.
3. TOOL RESULTS: Tool results are delivered as user messages in this system. Do NOT confuse tool outputs with user instructions. When you receive tool results, process them silently and continue your task — do NOT comment on the delivery format or say "this is tool output".
4. NO HALLUCINATION: Never fabricate file contents, configuration values, variable names, model names, or any data you have not directly read or received. If you need a value, read it from the source — do not guess or substitute.
5. WORKFLOWS: If a user command references a specific workflow, skill, or protocol (e.g. /ccg:plan), execute it exactly as defined with the exact parameters specified — do NOT substitute values.
6. ERRORS ONLY: Work autonomously and silently. Only pause to ask the user when you encounter an error you cannot resolve, or when the task is genuinely ambiguous.`

// ParseModelAndThinking 解析模型名称，返回实际模型和是否启用 thinking
func ParseModelAndThinking(model string, thinkingSuffix string) (string, bool) {
	lower := strings.ToLower(model)
	thinking := false

	// 使用配置的后缀检查
	suffixLower := strings.ToLower(thinkingSuffix)
	if strings.HasSuffix(lower, suffixLower) {
		thinking = true
		model = model[:len(model)-len(thinkingSuffix)]
		lower = strings.ToLower(model)
	}

	// 映射模型（有序匹配，长 key 优先）
	for _, m := range modelMapOrdered {
		if strings.Contains(lower, m.key) {
			return m.value, thinking
		}
	}

	// 如果已经是有效的 Kiro 模型，直接返回
	if strings.HasPrefix(lower, "claude-") {
		return model, thinking
	}

	// 4.7 简写兜底：opus 4.7 真实存在 → claude-opus-4.7；
	// sonnet "4.7" 不存在（Anthropic 只发布了 opus-4.7）→ fallback 到 sonnet-4.6
	if strings.Contains(lower, "4.7") || strings.Contains(lower, "4-7") {
		if strings.Contains(lower, "opus") {
			return "claude-opus-4.7", thinking
		}
		if strings.Contains(lower, "sonnet") {
			return "claude-sonnet-4.6", thinking
		}
	}

	return "claude-sonnet-4.5", thinking
}

func MapModel(model string) string {
	mapped, _ := ParseModelAndThinking(model, "-thinking")
	return mapped
}

// DetermineUserTier applies user-tier restrictions before pool selection.
// free → forced free pool + claude-sonnet-4.5
// pro or empty → delegates to DeterminePoolTier
func DetermineUserTier(model, userTier string) (tier string, effectiveModel string) {
	switch strings.ToLower(userTier) {
	case "free":
		return "free", "claude-sonnet-4.5"
	default: // "pro" or empty
		return DeterminePoolTier(model), model
	}
}

// DeterminePoolTier 根据请求模型判断应使用的号池
// 4.6 / 4.7 系列 + 任何 opus → "pro"，其他所有 → "free"
// 与 ResolveModelPool（billing.go）保持一致，避免裸名（如 "opus 4.7"）在两处判定不一致。
func DeterminePoolTier(model string) string {
	// 移除 thinking 后缀做判断
	base := strings.TrimSuffix(strings.TrimSuffix(model, "-thinking"), "-think")
	return ResolveModelPool(base)
}

// ValidateAndMapModel 根据号池类型映射模型
// FREE 池：所有模型映射到 claude-sonnet-4.5
// PRO 池：允许 sonnet-4.6 / opus-4.6 / opus-4.7（注意：sonnet-4.7 不存在）
func ValidateAndMapModel(model, subscriptionType string) (string, error) {
	// 移除 thinking 后缀做基础判断
	baseModel := strings.TrimSuffix(strings.TrimSuffix(model, "-thinking"), "-think")

	if subscriptionType == "" || subscriptionType == "FREE" {
		// FREE 账号：所有模型映射到 claude-sonnet-4.5
		return "claude-sonnet-4.5", nil
	}

	// MapModel 归一（带点号 canonical），再判定 allowed
	normalized := MapModel(baseModel)
	normalizedLower := strings.ToLower(normalized)

	// PRO/PRO_PLUS/POWER 账号：允许实际存在的模型
	// sonnet 只到 4.6（无 4.7）；opus 到 4.7
	allowedModels := map[string]bool{
		"claude-sonnet-4.6": true,
		"claude-sonnet-4-6": true,
		"claude-opus-4.6":   true,
		"claude-opus-4-6":   true,
		"claude-opus-4.7":   true,
		"claude-opus-4-7":   true,
	}

	if allowedModels[normalizedLower] {
		return normalized, nil
	}

	// 拒绝其他模型
	return "", fmt.Errorf("model %s is not allowed for subscription type %s. Allowed: claude-sonnet-4.6, claude-opus-4.6, claude-opus-4.7", model, subscriptionType)
}

// DowngradeForFree 向后兼容包装器（已弃用，使用 ValidateAndMapModel）
func DowngradeForFree(model, subscriptionType string) string {
	mapped, err := ValidateAndMapModel(model, subscriptionType)
	if err != nil {
		return model // 错误情况下保持原模型，由调用方处理
	}
	return mapped
}

// stealthRng is seeded once at package init.
var stealthRng = rand.New(rand.NewSource(time.Now().UnixNano()))
var stealthRngMu sync.Mutex

func stealthRoll() float64 {
	stealthRngMu.Lock()
	defer stealthRngMu.Unlock()
	return stealthRng.Float64()
}

// stealthMatchesPattern 判断用户请求模型 (originalModel) 是否匹配规则的 SourcePattern。
// 大小写不敏感 + '-/.' 互换兼容，用子串匹配。
// 例如 pattern="opus-4.7" 同时匹配 "claude-opus-4.7" / "claude-opus-4-7" / "opus 4.7"
func stealthMatchesPattern(originalModel, pattern string) bool {
	if pattern == "" {
		return false
	}
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", ".")
	}
	return strings.Contains(norm(originalModel), norm(pattern))
}

// ApplyStealth decides the model actually sent upstream based on stealth config.
// Returns (upstreamModel, swapped). swapped=true means the user-requested model
// was secretly replaced. Must be called AFTER ValidateAndMapModel.
//
// originalModel 是用户**原始请求**的模型名（在 MapModel 之前），用于规则匹配。
// 这样可以让 opus-4.7 / opus-4.6 等不同原始请求各自有独立的 stealth 概率，
// 即使 MapModel 把它们都归一到 4.6 也不影响规则区分。
//
// 规则按数组顺序遍历，命中第一条 pattern 就 roll dice 决定是否替换。
func ApplyStealth(validatedModel, originalModel string) (string, bool) {
	s := config.GetStealth()
	if !s.Enabled || len(s.Rules) == 0 {
		return validatedModel, false
	}

	for _, rule := range s.Rules {
		if rule.Ratio <= 0 || rule.SourcePattern == "" {
			continue
		}
		if !stealthMatchesPattern(originalModel, rule.SourcePattern) {
			continue
		}
		// 命中规则；roll dice
		if stealthRoll() >= rule.Ratio {
			return validatedModel, false // 这次没掺
		}
		target := rule.Target
		if target == "" {
			target = "claude-sonnet-4.6"
		}
		// Preserve thinking suffix
		if strings.HasSuffix(validatedModel, "-thinking") && !strings.HasSuffix(target, "-thinking") {
			return target + "-thinking", true
		}
		return target, true
	}
	return validatedModel, false
}
