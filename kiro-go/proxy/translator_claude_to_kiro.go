package proxy

import (
	"strconv"
	"strings"
	"sync"
)

// ==================== Claude -> Kiro 转换 ====================

// mergeConsecutiveMessages 合并连续的同角色消息。
// Kiro/AmazonQ API 要求严格的 user/assistant 交替，不接受连续的同角色消息。
// Anthropic 原生 API 允许这种格式，所以 OpenClaw/Claude Code 等客户端可能会发出这种请求。
func mergeConsecutiveMessages(messages []ClaudeMessage) []ClaudeMessage {
	if len(messages) <= 1 {
		return messages
	}

	result := make([]ClaudeMessage, 0, len(messages))
	i := 0
	for i < len(messages) {
		// 如果下一条消息角色不同，或者已经是最后一条，直接保留
		if i == len(messages)-1 || messages[i].Role != messages[i+1].Role {
			result = append(result, messages[i])
			i++
			continue
		}

		// 发现连续的同角色消息 — 合并
		role := messages[i].Role
		var mergedBlocks []interface{}
		for i < len(messages) && messages[i].Role == role {
			mergedBlocks = append(mergedBlocks, contentToBlocks(messages[i].Content)...)
			i++
		}

		result = append(result, ClaudeMessage{
			Role:    role,
			Content: mergedBlocks,
		})
	}

	return result
}

// contentToBlocks 将消息内容统一转为 []interface{} 的 content blocks 格式。
func contentToBlocks(content interface{}) []interface{} {
	if content == nil {
		return nil
	}
	if s, ok := content.(string); ok {
		return []interface{}{map[string]interface{}{
			"type": "text",
			"text": s,
		}}
	}
	if blocks, ok := content.([]interface{}); ok {
		return blocks
	}
	return nil
}

const maxToolDescLen = 10237

func ClaudeToKiro(req *ClaudeRequest, thinking bool) *KiroPayload {
	modelID := MapModel(req.Model)
	origin := "AI_EDITOR"

	// 提取系统提示
	systemPrompt := extractSystemPrompt(req.System)

	// 如果启用 thinking 模式，注入 thinking 提示（支持自定义 budget_tokens）
	if thinking {
		budgetTokens := 0
		if req.Thinking != nil && req.Thinking.BudgetTokens > 0 {
			budgetTokens = req.Thinking.BudgetTokens
		}
		thinkingPrompt := buildThinkingModePrompt(budgetTokens)
		if systemPrompt != "" {
			systemPrompt = thinkingPrompt + "\n\n" + systemPrompt
		} else {
			systemPrompt = thinkingPrompt
		}
	}

	// 合并连续的同角色消息（Kiro API 要求严格的 user/assistant 交替）
	messages := mergeConsecutiveMessages(req.Messages)

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

	for i, msg := range messages {
		isLast := i == len(messages)-1

		switch msg.Role {
		case "user":
			content, images, toolResults := extractClaudeUserContent(msg.Content)
			content = normalizeUserContent(content, len(images) > 0)

			if isLast {
				currentContent = content
				currentImages = images
				currentToolResults = toolResults
			} else {
				userMsg := KiroUserInputMessage{
					Content: content,
					ModelID: modelID,
					Origin:  origin,
				}
				if len(images) > 0 {
					userMsg.Images = images
				}
				if len(toolResults) > 0 {
					userMsg.UserInputMessageContext = &UserInputMessageContext{
						ToolResults: toolResults,
					}
				}
				history = append(history, KiroHistoryMessage{
					UserInputMessage: &userMsg,
				})
			}
		case "assistant":
			content, toolUses := extractClaudeAssistantContent(msg.Content)
			history = append(history, KiroHistoryMessage{
				AssistantResponseMessage: &KiroAssistantResponseMessage{
					Content:  content,
					ToolUses: toolUses,
				},
			})
		}
	}

	// 确保 history 以 user 开始（仅在没有系统提示词且首条是 assistant 时）
	if len(history) > 0 && history[0].AssistantResponseMessage != nil {
		history = append([]KiroHistoryMessage{{
			UserInputMessage: &KiroUserInputMessage{
				Content: "Begin conversation",
				ModelID: modelID,
				Origin:  origin,
			},
		}}, history...)
	}

	// 构建最终内容（不再嵌入系统提示词）
	finalContent := ""
	if currentContent != "" {
		finalContent = currentContent
	} else if len(currentImages) > 0 {
		finalContent = normalizeUserContent("", true)
	} else if len(currentToolResults) > 0 {
		// Don't duplicate tool result text into Content — it's already in ToolResults.
		// Use a short continuation prompt so Kiro API accepts the message (Content must be non-empty).
		finalContent = "Process the tool results above and continue."
	} else {
		finalContent = minimalFallbackUserContent
	}

	// 转换工具
	kiroTools := convertClaudeTools(req.Tools)

	// 构建 payload
	payload := &KiroPayload{}
	payload.ConversationState.ChatTriggerType = "MANUAL"
	payload.ConversationState.ConversationID = buildConversationID(modelID, systemPrompt, firstClaudeConversationAnchor(req.Messages))
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

func extractSystemPrompt(system interface{}) string {
	if system == nil {
		return ""
	}
	if s, ok := system.(string); ok {
		return s
	}
	if blocks, ok := system.([]interface{}); ok {
		var parts []string
		for _, b := range blocks {
			if block, ok := b.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func extractClaudeUserContent(content interface{}) (string, []KiroImage, []KiroToolResult) {
	var text string
	var images []KiroImage
	var toolResults []KiroToolResult
	docCount := 0

	if s, ok := content.(string); ok {
		return s, nil, nil
	}

	if blocks, ok := content.([]interface{}); ok {
		for _, b := range blocks {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := block["type"].(string)
			switch blockType {
			case "text", "input_text":
				if t, ok := block["text"].(string); ok {
					text += t
				}
			case "image", "image_url", "input_image":
				if img := extractImageFromClaudeBlock(block); img != nil {
					images = append(images, *img)
				}
			case "document", "input_file", "file":
				if docCount >= docMaxPerRequest {
					text += `<document error="超过单消息文档数上限 ` + strconv.Itoa(docMaxPerRequest) + ` 个，已忽略后续附件"/>` + "\n"
					docCount++
					continue
				}
				if doc := extractDocFromClaudeBlock(block); doc != nil {
					text += formatDocBlock(doc) + "\n"
					docCount++
				}
			case "tool_result":
				toolUseID, _ := block["tool_use_id"].(string)
				resultContent := extractToolResultContent(block["content"])
				toolResults = append(toolResults, KiroToolResult{
					ToolUseID: toolUseID,
					Content:   []KiroResultContent{{Text: resultContent}},
					Status:    "success",
				})
			}
		}
	}

	return text, images, toolResults
}

func extractImageFromClaudeBlock(block map[string]interface{}) *KiroImage {
	if source, ok := block["source"].(map[string]interface{}); ok {
		if data, ok := source["data"].(string); ok {
			if img := parseDataURL(data); img != nil {
				return img
			}
			mediaType, _ := source["media_type"].(string)
			if mediaType == "" {
				mediaType, _ = source["mediaType"].(string)
			}
			if mediaType == "" {
				mediaType, _ = source["mime_type"].(string)
			}
			format := strings.TrimPrefix(strings.ToLower(mediaType), "image/")
			if img := parseBase64Image(data, format); img != nil {
				return img
			}
		}
		if url, ok := source["url"].(string); ok {
			if img := parseDataURL(url); img != nil {
				return img
			}
		}
	}

	if img := extractImageFromOpenAIPart(block); img != nil {
		return img
	}

	if data, ok := block["data"].(string); ok {
		if img := parseDataURL(data); img != nil {
			return img
		}
	}

	return nil
}

func extractToolResultContent(content interface{}) string {
	if s, ok := content.(string); ok {
		return s
	}
	if blocks, ok := content.([]interface{}); ok {
		var parts []string
		for _, b := range blocks {
			if block, ok := b.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func extractClaudeAssistantContent(content interface{}) (string, []KiroToolUse) {
	var text string
	var toolUses []KiroToolUse

	if s, ok := content.(string); ok {
		return s, nil
	}

	if blocks, ok := content.([]interface{}); ok {
		for _, b := range blocks {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				if t, ok := block["text"].(string); ok {
					text += t
				}
			case "tool_use":
				id, _ := block["id"].(string)
				name, _ := block["name"].(string)
				input, _ := block["input"].(map[string]interface{})
				if input == nil {
					input = make(map[string]interface{})
				}
				toolUses = append(toolUses, KiroToolUse{
					ToolUseID: id,
					Name:      name,
					Input:     input,
				})
			}
		}
	}

	return text, toolUses
}

// toolNameMapping 存储 shortened → original 的工具名映射（线程安全）
var (
	toolNameMap   = make(map[string]string)
	toolNameMapMu sync.RWMutex
)

// RestoreToolName 将截断后的工具名还原为原始名称
func RestoreToolName(shortened string) string {
	toolNameMapMu.RLock()
	defer toolNameMapMu.RUnlock()
	if original, ok := toolNameMap[shortened]; ok {
		return original
	}
	return shortened
}

func convertClaudeTools(tools []ClaudeTool) []KiroToolWrapper {
	if len(tools) == 0 {
		return nil
	}

	result := make([]KiroToolWrapper, len(tools))
	for i, tool := range tools {
		desc := tool.Description
		if len(desc) > maxToolDescLen {
			desc = desc[:maxToolDescLen] + "..."
		}
		shortened := shortenToolName(tool.Name)
		// 注册映射（仅在名字被改变时）
		if shortened != tool.Name {
			toolNameMapMu.Lock()
			toolNameMap[shortened] = tool.Name
			toolNameMapMu.Unlock()
		}
		result[i] = KiroToolWrapper{}
		result[i].ToolSpecification.Name = shortened
		result[i].ToolSpecification.Description = desc
		result[i].ToolSpecification.InputSchema = InputSchema{JSON: tool.InputSchema}
	}
	return result
}

func shortenToolName(name string) string {
	if len(name) <= 64 {
		return name
	}
	// MCP tools: mcp__server__tool -> mcp__tool
	if strings.HasPrefix(name, "mcp__") {
		lastIdx := strings.LastIndex(name, "__")
		if lastIdx > 5 {
			shortened := "mcp__" + name[lastIdx+2:]
			if len(shortened) <= 64 {
				return shortened
			}
		}
	}
	return name[:64]
}
