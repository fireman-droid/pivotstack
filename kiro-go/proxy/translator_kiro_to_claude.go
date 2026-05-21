package proxy

import "github.com/google/uuid"

// ==================== Kiro -> Claude 转换 ====================

func KiroToClaudeResponse(content, thinkingContent string, toolUses []KiroToolUse, inputTokens, outputTokens int, model string) *ClaudeResponse {
	blocks := make([]ClaudeContentBlock, 0)

	if thinkingContent != "" {
		blocks = append(blocks, ClaudeContentBlock{
			Type:     "thinking",
			Thinking: thinkingContent,
		})
	}

	if content != "" {
		blocks = append(blocks, ClaudeContentBlock{
			Type: "text",
			Text: content,
		})
	}

	for _, tu := range toolUses {
		blocks = append(blocks, ClaudeContentBlock{
			Type:  "tool_use",
			ID:    tu.ToolUseID,
			Name:  tu.Name,
			Input: tu.Input,
		})
	}

	stopReason := "end_turn"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}

	return &ClaudeResponse{
		ID:         "msg_" + uuid.New().String(),
		Type:       "message",
		Role:       "assistant",
		Content:    blocks,
		Model:      model,
		StopReason: stopReason,
		Usage: ClaudeUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
}
