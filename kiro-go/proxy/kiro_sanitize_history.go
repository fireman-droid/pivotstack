package proxy

import (
	"fmt"
	"strings"
)

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
