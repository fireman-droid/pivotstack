package proxy

import (
	"encoding/json"
	"strings"
)

// ==================== OpenAI -> Kiro 转换 ====================

// mergeConsecutiveOpenAIMessages 合并连续的同角色 user/assistant 消息。
// Kiro/AmazonQ API 严格要求 user/assistant 交替；OpenAI 客户端（含 Claude Code 走 OpenAI 兼容接口）
// 在长对话/多轮 tool_calls 情况下可能产生连续同 role，触发 HTTP 400 "Improperly formed request"。
//
// 行为：
//   - 只合并 user/user 与 assistant/assistant 相邻同 role
//   - tool 角色不合并（每条 tool message 必须保留其独立的 tool_call_id 配对）
//   - 合并时 content 统一转为 [{type:"text",text:...}/multimodal] blocks 数组，下游 extract* 能正确解析
//   - 合并时 tool_calls 数组按顺序拼接
//
// 历史背景：commit 2ed8536 为 Claude API 路径实现了 mergeConsecutiveMessages（针对 ClaudeMessage），
// 但 OpenAI 路径漏改 —— 本函数补齐 OpenAI 路径同样问题。
func mergeConsecutiveOpenAIMessages(messages []OpenAIMessage) []OpenAIMessage {
	if len(messages) <= 1 {
		return messages
	}
	result := make([]OpenAIMessage, 0, len(messages))
	i := 0
	for i < len(messages) {
		cur := messages[i]
		// 只合并 user / assistant；tool / 其它 role 直接保留
		if cur.Role != "user" && cur.Role != "assistant" {
			result = append(result, cur)
			i++
			continue
		}
		// 与下一条 role 不同（或已是最后一条）→ 不需要合并
		if i == len(messages)-1 || messages[i+1].Role != cur.Role {
			result = append(result, cur)
			i++
			continue
		}
		// 连续同 role → 合并
		role := cur.Role
		var mergedBlocks []interface{}
		var mergedToolCalls []ToolCall
		for i < len(messages) && messages[i].Role == role {
			mergedBlocks = append(mergedBlocks, openaiContentToBlocks(messages[i].Content)...)
			if len(messages[i].ToolCalls) > 0 {
				mergedToolCalls = append(mergedToolCalls, messages[i].ToolCalls...)
			}
			i++
		}
		merged := OpenAIMessage{
			Role:    role,
			Content: mergedBlocks,
		}
		if len(mergedToolCalls) > 0 {
			merged.ToolCalls = mergedToolCalls
		}
		result = append(result, merged)
	}
	return result
}

// openaiContentToBlocks 将 OpenAI content 字段（可能是 string / []block / nil）统一转为 blocks 数组。
// 用于 mergeConsecutiveOpenAIMessages 合并不同消息的 content 时统一容器形态。
func openaiContentToBlocks(content interface{}) []interface{} {
	if content == nil {
		return nil
	}
	if s, ok := content.(string); ok {
		if s == "" {
			return nil
		}
		return []interface{}{map[string]interface{}{
			"type": "text",
			"text": s,
		}}
	}
	if blocks, ok := content.([]interface{}); ok {
		return blocks
	}
	// 其它结构（map 等）尝试 marshal 成字符串塞进 text block，避免数据丢失
	if raw, err := json.Marshal(content); err == nil {
		return []interface{}{map[string]interface{}{
			"type": "text",
			"text": string(raw),
		}}
	}
	return nil
}

func OpenAIToKiro(req *OpenAIRequest, thinking bool) *KiroPayload {
	modelID := MapModel(req.Model)
	origin := "AI_EDITOR"

	// 提取系统提示
	var systemPrompt string
	var nonSystemMessages []OpenAIMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if s := extractOpenAIMessageText(msg.Content); s != "" {
				systemPrompt += s + "\n"
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}
	systemPrompt = strings.TrimRight(systemPrompt, "\n")

	// 合并连续的同 role user/assistant 消息（Kiro 严格交替要求）
	nonSystemMessages = mergeConsecutiveOpenAIMessages(nonSystemMessages)

	// 如果启用 thinking 模式，注入 thinking 提示
	if thinking {
		thinkingPrompt := buildThinkingModePrompt(0) // OpenAI 格式暂不支持自定义 budget
		if systemPrompt != "" {
			systemPrompt = thinkingPrompt + "\n\n" + systemPrompt
		} else {
			systemPrompt = thinkingPrompt
		}
	}

	// 构建历史消息
	history := make([]KiroHistoryMessage, 0)
	var currentContent string
	var currentImages []KiroImage
	var currentToolResults []KiroToolResult

	// 系统提示词作为 history 的第一轮对话（user + assistant 确认）
	if systemPrompt != "" {
		history = append(history, KiroHistoryMessage{
			UserInputMessage: &KiroUserInputMessage{
				Content: systemPrompt + SystemPromptReinforcement,
				ModelID: modelID,
				Origin:  origin,
			},
		})
		history = append(history, KiroHistoryMessage{
			AssistantResponseMessage: &KiroAssistantResponseMessage{
				Content: "I will strictly follow all instructions above. I will actively call tools (including MCP tools) whenever relevant, and execute user commands exactly as specified without deviation.",
			},
		})
	}

	for i, msg := range nonSystemMessages {
		isLast := i == len(nonSystemMessages)-1

		switch msg.Role {
		case "user":
			content, images := extractOpenAIUserContent(msg.Content)
			content = normalizeUserContent(content, len(images) > 0)

			if isLast {
				currentContent = content
				currentImages = images
			} else {
				history = append(history, KiroHistoryMessage{
					UserInputMessage: &KiroUserInputMessage{
						Content: content,
						ModelID: modelID,
						Origin:  origin,
						Images:  images,
					},
				})
			}

		case "assistant":
			content := extractOpenAIMessageText(msg.Content)

			var toolUses []KiroToolUse
			for _, tc := range msg.ToolCalls {
				var input map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				if input == nil {
					input = make(map[string]interface{})
				}
				toolUses = append(toolUses, KiroToolUse{
					ToolUseID: tc.ID,
					Name:      tc.Function.Name,
					Input:     input,
				})
			}

			history = append(history, KiroHistoryMessage{
				AssistantResponseMessage: &KiroAssistantResponseMessage{
					Content:  content,
					ToolUses: toolUses,
				},
			})

		case "tool":
			content := extractOpenAIMessageText(msg.Content)
			currentToolResults = append(currentToolResults, KiroToolResult{
				ToolUseID: msg.ToolCallID,
				Content:   []KiroResultContent{{Text: content}},
				Status:    "success",
			})

			// 检查下一条是否还是 tool
			nextIdx := i + 1
			if nextIdx >= len(nonSystemMessages) || nonSystemMessages[nextIdx].Role != "tool" {
				if !isLast {
					history = append(history, KiroHistoryMessage{
						UserInputMessage: &KiroUserInputMessage{
							Content: "Process the tool results above and continue.",
							ModelID: modelID,
							Origin:  origin,
							UserInputMessageContext: &UserInputMessageContext{
								ToolResults: currentToolResults,
							},
						},
					})
					currentToolResults = nil
				}
			}
		}
	}

	// 构建最终内容（不再嵌入系统提示词）
	finalContent := currentContent
	if finalContent == "" {
		if len(currentImages) > 0 {
			finalContent = normalizeUserContent("", true)
		} else if len(currentToolResults) > 0 {
			finalContent = "Process the tool results above and continue."
		} else {
			finalContent = minimalFallbackUserContent
		}
	}

	// 转换工具
	kiroTools := convertOpenAITools(req.Tools)

	// 构建 payload
	payload := &KiroPayload{}
	payload.ConversationState.ChatTriggerType = "MANUAL"
	payload.ConversationState.ConversationID = buildConversationID(modelID, systemPrompt, firstOpenAIConversationAnchor(nonSystemMessages))
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: finalContent,
		ModelID: modelID,
		Origin:  origin,
		Images:  currentImages,
	}

	if len(kiroTools) > 0 || len(currentToolResults) > 0 {
		payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext = &UserInputMessageContext{
			Tools:       kiroTools,
			ToolResults: currentToolResults,
		}
	}

	if len(history) > 0 {
		payload.ConversationState.History = history
	}

	if req.MaxTokens > 0 || req.Temperature > 0 || req.TopP > 0 {
		payload.InferenceConfig = &InferenceConfig{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
		}
	}

	NormalizeKiroPayload(payload)
	return payload
}
