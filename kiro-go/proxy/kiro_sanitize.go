package proxy

import (
	"fmt"
	"strings"
)

// 这些限制由 Kiro/CodeWhisperer 上游强制；超出会触发 400 "Improperly formed request"。
// 数值参考 kiro-gateway/kiro/config.py 与 converters_core.py。
const (
	kiroToolNameMaxLen = 64
	kiroToolDescMaxLen = 10000
	kiroEmptyContent   = "(empty)"
)

// NormalizeKiroPayload 在 KiroPayload 发送给上游前做一次"防坑"清理。
//
// 修复客户端可能发来的不规范请求，避免上游返回模糊的 400 "Improperly formed request":
//   - 工具 inputSchema 缺/无效字段（required:[]、properties:null、type:missing、
//     additionalProperties 非 bool/object）
//   - 工具名 > 64 字符
//   - 工具描述 > 10000 字符
//   - tool_use 没有匹配 tool_result（孤儿）→ 移除
//   - tool_result 没有匹配 tool_use（孤儿）→ 转文本
//   - tool_results 前缺 assistant with toolUses → 转文本
//   - history 第一条不是 user → prepend 合成 user
//   - history 内连续同 role → 中间塞合成消息保持交替
//   - 没声明 tools 但 message 里有 tool 内容 → 全部转文本
//
// 函数就地修改 payload，幂等（多次调用结果相同）。规则参考 kiro-gateway
// converters_core.py 同名函数；该文件之前只在 Python gateway 一侧生效，本函数
// 把同等保护落到 Go 端，供 8990 prod 直连模式使用。
func NormalizeKiroPayload(payload *KiroPayload) {
	if payload == nil {
		return
	}
	cs := &payload.ConversationState
	cur := &cs.CurrentMessage.UserInputMessage

	// === 工具清理 ===
	var toolDocs string
	hasTools := false
	if cur.UserInputMessageContext != nil && len(cur.UserInputMessageContext.Tools) > 0 {
		ctx := cur.UserInputMessageContext
		ctx.Tools, toolDocs = processToolsLongDescriptions(ctx.Tools)
		ctx.Tools = filterToolsByNameLen(ctx.Tools)
		for i := range ctx.Tools {
			ctx.Tools[i].ToolSpecification.InputSchema.JSON = sanitizeJSONSchema(ctx.Tools[i].ToolSpecification.InputSchema.JSON)
		}
		hasTools = len(ctx.Tools) > 0
	}
	if toolDocs != "" {
		cur.Content = toolDocs + "\n\n" + cur.Content
	}

	// === 消息序列清理 ===
	if !hasTools {
		// 没声明工具时，把所有 tool 内容转文本
		cs.History = stripAllToolContentInHistory(cs.History)
		if cur.UserInputMessageContext != nil && len(cur.UserInputMessageContext.ToolResults) > 0 {
			cur.Content = appendToolResultsToText(cur.Content, cur.UserInputMessageContext.ToolResults)
			cur.UserInputMessageContext.ToolResults = nil
		}
	} else {
		// 有工具：配对清理 + ensure assistant before tool_results
		cs.History = validateAndCleanToolPairingHistory(cs.History, cur)
		cs.History = ensureAssistantBeforeToolResultsHistory(cs.History, cur)
	}

	// 合并相邻同 role（兜底，前面 ClaudeToKiro 已经做过一次）
	cs.History = mergeAdjacentHistoryMessages(cs.History)

	// 首条必须 user
	cs.History = ensureFirstHistoryIsUser(cs.History, cur.ModelID)

	// 交替 user/assistant
	cs.History = ensureAlternatingHistory(cs.History, cur.ModelID)

	// history 末尾如果是 user，current 又是 user → 中间塞合成 assistant
	if len(cs.History) > 0 && cs.History[len(cs.History)-1].UserInputMessage != nil {
		cs.History = append(cs.History, KiroHistoryMessage{
			AssistantResponseMessage: &KiroAssistantResponseMessage{Content: kiroEmptyContent},
		})
	}

	// current message 内容兜底（Kiro API 要求 Content 非空）
	if strings.TrimSpace(cur.Content) == "" {
		cur.Content = kiroEmptyContent
	}
}

// ============== 工具相关 ==============

// processToolsLongDescriptions 把超长 description 移到 system 提示文档片段，原 description 替换成 reference。
// 返回 (清理后工具, 拼到 system/current 的文档块)。
func processToolsLongDescriptions(tools []KiroToolWrapper) ([]KiroToolWrapper, string) {
	if len(tools) == 0 {
		return tools, ""
	}
	var docs []string
	out := make([]KiroToolWrapper, 0, len(tools))
	for _, t := range tools {
		desc := t.ToolSpecification.Description
		if len(desc) <= kiroToolDescMaxLen {
			out = append(out, t)
			continue
		}
		docs = append(docs, fmt.Sprintf("## Tool: %s\n\n%s", t.ToolSpecification.Name, desc))
		t.ToolSpecification.Description = fmt.Sprintf("[Full documentation in system prompt under '## Tool: %s']", t.ToolSpecification.Name)
		out = append(out, t)
	}
	if len(docs) == 0 {
		return out, ""
	}
	return out, "\n\n---\n# Tool Documentation\nThe following tools have detailed documentation that couldn't fit in the tool definition.\n\n" +
		strings.Join(docs, "\n\n---\n\n")
}

// filterToolsByNameLen 名字 > 64 字符直接丢弃（截断会重名碰撞，丢弃最安全）。
func filterToolsByNameLen(tools []KiroToolWrapper) []KiroToolWrapper {
	out := make([]KiroToolWrapper, 0, len(tools))
	for _, t := range tools {
		if len(t.ToolSpecification.Name) > kiroToolNameMaxLen {
			continue
		}
		out = append(out, t)
	}
	return out
}

// sanitizeJSONSchema 递归清理 JSON Schema 让它满足 Kiro 上游约束。
// 修复点（这些字段任何一个不规范都会触发上游 400）：
//   - type 必须是非空字符串，缺则补 "object"
//   - properties 必须是 object，null 时补 {}
//   - required 必须是非空字符串数组，空数组/null 时移除字段
//   - additionalProperties 必须是 bool 或 object
//
// 实现参考 kiro-gateway converters_core.py:373 sanitize_json_schema。
func sanitizeJSONSchema(schema interface{}) interface{} {
	minimal := func() map[string]interface{} {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": true,
		}
	}

	if schema == nil {
		return minimal()
	}
	m, ok := schema.(map[string]interface{})
	if !ok {
		return minimal()
	}

	out := make(map[string]interface{}, len(m))

	for key, value := range m {
		switch key {
		case "type":
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				out[key] = s
			} else {
				out[key] = "object"
			}
		case "properties":
			if pm, ok := value.(map[string]interface{}); ok {
				cleaned := make(map[string]interface{}, len(pm))
				for pn, pv := range pm {
					if pvm, ok := pv.(map[string]interface{}); ok {
						cleaned[pn] = sanitizeJSONSchema(pvm)
					} else {
						cleaned[pn] = pv
					}
				}
				out[key] = cleaned
			} else {
				out[key] = map[string]interface{}{}
			}
		case "required":
			if arr, ok := value.([]interface{}); ok {
				strs := make([]interface{}, 0, len(arr))
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						strs = append(strs, s)
					}
				}
				if len(strs) > 0 {
					out[key] = strs
				}
			}
		case "additionalProperties":
			switch value.(type) {
			case bool, map[string]interface{}:
				out[key] = value
			default:
				out[key] = true
			}
		default:
			switch v := value.(type) {
			case map[string]interface{}:
				out[key] = sanitizeJSONSchema(v)
			case []interface{}:
				arr := make([]interface{}, 0, len(v))
				for _, item := range v {
					if im, ok := item.(map[string]interface{}); ok {
						arr = append(arr, sanitizeJSONSchema(im))
					} else {
						arr = append(arr, item)
					}
				}
				out[key] = arr
			default:
				out[key] = value
			}
		}
	}

	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}
	if t, ok := out["type"].(string); ok && t == "object" {
		if _, ok := out["properties"]; !ok {
			out["properties"] = map[string]interface{}{}
		}
	}

	return out
}

// ============== 消息序列相关 ==============

// stripAllToolContentInHistory 没声明 tools 时，把 history 里所有 toolUses/toolResults 转文本。
// Kiro API 在 toolResults 存在但 tools 为空时会 reject。
func stripAllToolContentInHistory(history []KiroHistoryMessage) []KiroHistoryMessage {
	if len(history) == 0 {
		return history
	}
	out := make([]KiroHistoryMessage, 0, len(history))
	for _, h := range history {
		nh := h
		if h.AssistantResponseMessage != nil && len(h.AssistantResponseMessage.ToolUses) > 0 {
			ar := *h.AssistantResponseMessage
			ar.Content = appendToolUsesToText(ar.Content, ar.ToolUses)
			ar.ToolUses = nil
			nh.AssistantResponseMessage = &ar
		}
		if h.UserInputMessage != nil && h.UserInputMessage.UserInputMessageContext != nil &&
			len(h.UserInputMessage.UserInputMessageContext.ToolResults) > 0 {
			u := *h.UserInputMessage
			ctx := *u.UserInputMessageContext
			u.Content = appendToolResultsToText(u.Content, ctx.ToolResults)
			ctx.ToolResults = nil
			u.UserInputMessageContext = &ctx
			nh.UserInputMessage = &u
		}
		out = append(out, nh)
	}
	return out
}

func appendToolUsesToText(existing string, uses []KiroToolUse) string {
	if len(uses) == 0 {
		return existing
	}
	var b strings.Builder
	b.WriteString("[Tool Calls]")
	for _, u := range uses {
		fmt.Fprintf(&b, "\n- name: %s, id: %s, input: %v", u.Name, u.ToolUseID, u.Input)
	}
	if existing == "" {
		return b.String()
	}
	return existing + "\n\n" + b.String()
}

func appendToolResultsToText(existing string, results []KiroToolResult) string {
	if len(results) == 0 {
		return existing
	}
	var b strings.Builder
	b.WriteString("[Tool Results]")
	for _, r := range results {
		fmt.Fprintf(&b, "\n- tool_use_id: %s", r.ToolUseID)
		for _, c := range r.Content {
			if c.Text != "" {
				fmt.Fprintf(&b, "\n  %s", c.Text)
			}
		}
	}
	if existing == "" {
		return b.String()
	}
	return existing + "\n\n" + b.String()
}

// validateAndCleanToolPairingHistory 移除孤儿 tool_use（无匹配 tool_result）；
// 把孤儿 tool_result（无匹配 tool_use）转文本。current 也参与 result 端检查。
func validateAndCleanToolPairingHistory(history []KiroHistoryMessage, current *KiroUserInputMessage) []KiroHistoryMessage {
	useIDs := map[string]bool{}
	resultIDs := map[string]bool{}

	for _, h := range history {
		if h.AssistantResponseMessage != nil {
			for _, u := range h.AssistantResponseMessage.ToolUses {
				if u.ToolUseID != "" {
					useIDs[u.ToolUseID] = true
				}
			}
		}
		if h.UserInputMessage != nil && h.UserInputMessage.UserInputMessageContext != nil {
			for _, r := range h.UserInputMessage.UserInputMessageContext.ToolResults {
				if r.ToolUseID != "" {
					resultIDs[r.ToolUseID] = true
				}
			}
		}
	}
	if current != nil && current.UserInputMessageContext != nil {
		for _, r := range current.UserInputMessageContext.ToolResults {
			if r.ToolUseID != "" {
				resultIDs[r.ToolUseID] = true
			}
		}
	}

	orphanedUses := map[string]bool{}
	for id := range useIDs {
		if !resultIDs[id] {
			orphanedUses[id] = true
		}
	}
	orphanedResults := map[string]bool{}
	for id := range resultIDs {
		if !useIDs[id] {
			orphanedResults[id] = true
		}
	}

	if len(orphanedUses) == 0 && len(orphanedResults) == 0 {
		return history
	}

	out := make([]KiroHistoryMessage, 0, len(history))
	for _, h := range history {
		nh := h
		if h.AssistantResponseMessage != nil && len(h.AssistantResponseMessage.ToolUses) > 0 && len(orphanedUses) > 0 {
			ar := *h.AssistantResponseMessage
			cleaned := make([]KiroToolUse, 0, len(ar.ToolUses))
			for _, u := range ar.ToolUses {
				if !orphanedUses[u.ToolUseID] {
					cleaned = append(cleaned, u)
				}
			}
			if len(cleaned) != len(ar.ToolUses) {
				ar.ToolUses = cleaned
				nh.AssistantResponseMessage = &ar
			}
		}
		if h.UserInputMessage != nil && h.UserInputMessage.UserInputMessageContext != nil && len(orphanedResults) > 0 {
			tr := h.UserInputMessage.UserInputMessageContext.ToolResults
			if len(tr) > 0 {
				var orphan, valid []KiroToolResult
				for _, r := range tr {
					if orphanedResults[r.ToolUseID] {
						orphan = append(orphan, r)
					} else {
						valid = append(valid, r)
					}
				}
				if len(orphan) > 0 {
					u := *h.UserInputMessage
					ctx := *u.UserInputMessageContext
					u.Content = appendToolResultsToText(u.Content, orphan)
					ctx.ToolResults = valid
					u.UserInputMessageContext = &ctx
					nh.UserInputMessage = &u
				}
			}
		}
		out = append(out, nh)
	}

	if current != nil && current.UserInputMessageContext != nil && len(orphanedResults) > 0 {
		tr := current.UserInputMessageContext.ToolResults
		if len(tr) > 0 {
			var orphan, valid []KiroToolResult
			for _, r := range tr {
				if orphanedResults[r.ToolUseID] {
					orphan = append(orphan, r)
				} else {
					valid = append(valid, r)
				}
			}
			if len(orphan) > 0 {
				current.Content = appendToolResultsToText(current.Content, orphan)
				current.UserInputMessageContext.ToolResults = valid
			}
		}
	}

	return out
}

// ensureAssistantBeforeToolResultsHistory 把没有前置 assistant(with toolUses) 的 tool_results 转文本。
func ensureAssistantBeforeToolResultsHistory(history []KiroHistoryMessage, current *KiroUserInputMessage) []KiroHistoryMessage {
	out := make([]KiroHistoryMessage, 0, len(history))
	for _, h := range history {
		if h.UserInputMessage != nil &&
			h.UserInputMessage.UserInputMessageContext != nil &&
			len(h.UserInputMessage.UserInputMessageContext.ToolResults) > 0 {
			hasPrev := false
			if len(out) > 0 {
				last := out[len(out)-1]
				if last.AssistantResponseMessage != nil && len(last.AssistantResponseMessage.ToolUses) > 0 {
					hasPrev = true
				}
			}
			if !hasPrev {
				u := *h.UserInputMessage
				ctx := *u.UserInputMessageContext
				u.Content = appendToolResultsToText(u.Content, ctx.ToolResults)
				ctx.ToolResults = nil
				u.UserInputMessageContext = &ctx
				nh := h
				nh.UserInputMessage = &u
				out = append(out, nh)
				continue
			}
		}
		out = append(out, h)
	}

	if current != nil && current.UserInputMessageContext != nil &&
		len(current.UserInputMessageContext.ToolResults) > 0 {
		hasPrev := false
		if len(out) > 0 {
			last := out[len(out)-1]
			if last.AssistantResponseMessage != nil && len(last.AssistantResponseMessage.ToolUses) > 0 {
				hasPrev = true
			}
		}
		if !hasPrev {
			current.Content = appendToolResultsToText(current.Content, current.UserInputMessageContext.ToolResults)
			current.UserInputMessageContext.ToolResults = nil
		}
	}

	return out
}

// mergeAdjacentHistoryMessages 合并相邻同 role 消息（防止前面清理后产生新的相邻）。
func mergeAdjacentHistoryMessages(history []KiroHistoryMessage) []KiroHistoryMessage {
	if len(history) <= 1 {
		return history
	}
	out := make([]KiroHistoryMessage, 0, len(history))
	for _, h := range history {
		if len(out) == 0 {
			out = append(out, h)
			continue
		}
		last := &out[len(out)-1]
		merged := false
		if last.UserInputMessage != nil && h.UserInputMessage != nil {
			merged = true
			last.UserInputMessage.Content = mergeText(last.UserInputMessage.Content, h.UserInputMessage.Content)
			if len(h.UserInputMessage.Images) > 0 {
				last.UserInputMessage.Images = append(last.UserInputMessage.Images, h.UserInputMessage.Images...)
			}
			if h.UserInputMessage.UserInputMessageContext != nil &&
				len(h.UserInputMessage.UserInputMessageContext.ToolResults) > 0 {
				if last.UserInputMessage.UserInputMessageContext == nil {
					last.UserInputMessage.UserInputMessageContext = &UserInputMessageContext{}
				}
				last.UserInputMessage.UserInputMessageContext.ToolResults = append(
					last.UserInputMessage.UserInputMessageContext.ToolResults,
					h.UserInputMessage.UserInputMessageContext.ToolResults...)
			}
		} else if last.AssistantResponseMessage != nil && h.AssistantResponseMessage != nil {
			merged = true
			last.AssistantResponseMessage.Content = mergeText(last.AssistantResponseMessage.Content, h.AssistantResponseMessage.Content)
			if len(h.AssistantResponseMessage.ToolUses) > 0 {
				last.AssistantResponseMessage.ToolUses = append(
					last.AssistantResponseMessage.ToolUses,
					h.AssistantResponseMessage.ToolUses...)
			}
		}
		if !merged {
			out = append(out, h)
		}
	}
	return out
}

func mergeText(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n" + b
}

// ensureFirstHistoryIsUser 首条不是 user 时 prepend 一条合成 user 消息。
func ensureFirstHistoryIsUser(history []KiroHistoryMessage, modelID string) []KiroHistoryMessage {
	if len(history) == 0 {
		return history
	}
	if history[0].UserInputMessage != nil {
		return history
	}
	if modelID == "" {
		for _, h := range history {
			if h.UserInputMessage != nil && h.UserInputMessage.ModelID != "" {
				modelID = h.UserInputMessage.ModelID
				break
			}
		}
	}
	synth := KiroHistoryMessage{
		UserInputMessage: &KiroUserInputMessage{
			Content: kiroEmptyContent,
			ModelID: modelID,
			Origin:  "AI_EDITOR",
		},
	}
	return append([]KiroHistoryMessage{synth}, history...)
}

// ensureAlternatingHistory 连续相同 role 之间塞合成消息保持交替。
func ensureAlternatingHistory(history []KiroHistoryMessage, modelID string) []KiroHistoryMessage {
	if len(history) < 2 {
		return history
	}
	out := []KiroHistoryMessage{history[0]}
	for i := 1; i < len(history); i++ {
		prev := out[len(out)-1]
		cur := history[i]
		sameRole := false
		if prev.UserInputMessage != nil && cur.UserInputMessage != nil {
			sameRole = true
		} else if prev.AssistantResponseMessage != nil && cur.AssistantResponseMessage != nil {
			sameRole = true
		}
		if sameRole {
			if cur.UserInputMessage != nil {
				out = append(out, KiroHistoryMessage{
					AssistantResponseMessage: &KiroAssistantResponseMessage{Content: kiroEmptyContent},
				})
			} else {
				out = append(out, KiroHistoryMessage{
					UserInputMessage: &KiroUserInputMessage{
						Content: kiroEmptyContent,
						ModelID: modelID,
						Origin:  "AI_EDITOR",
					},
				})
			}
		}
		out = append(out, cur)
	}
	return out
}
