package proxy

import (
	"strings"
	"testing"
)

func TestExtractOpenAIMessageTextStructured(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "alpha"},
		map[string]interface{}{"type": "input_text", "text": "beta"},
	}

	if got := extractOpenAIMessageText(content); got != "alphabeta" {
		t.Fatalf("expected concatenated structured text, got %q", got)
	}

	nested := map[string]interface{}{
		"content": []interface{}{map[string]interface{}{"type": "text", "text": "nested"}},
	}
	if got := extractOpenAIMessageText(nested); got != "nested" {
		t.Fatalf("expected nested content extraction, got %q", got)
	}
}

func TestOpenAIToKiroPreservesStructuredAssistantAndToolContent(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{
				Role: "system",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "system-a"},
					map[string]interface{}{"type": "text", "text": "system-b"},
				},
			},
			{Role: "user", Content: "first-question"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "assistant-structured"},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "tool-result-structured"},
				},
			},
		},
	}

	payload := OpenAIToKiro(req, false)

	// history: system-user + system-assistant-ack + user(first-question) + assistant(assistant-structured) = 4
	if len(payload.ConversationState.History) != 4 {
		t.Fatalf("expected 4 history items, got %d", len(payload.ConversationState.History))
	}

	// history[0]: system prompt as user message
	sysUser := payload.ConversationState.History[0].UserInputMessage
	if sysUser == nil {
		t.Fatalf("expected first history item to be system prompt user message")
	}
	if !strings.Contains(sysUser.Content, "system-a") ||
		!strings.Contains(sysUser.Content, "system-b") {
		t.Fatalf("expected system prompt content, got %q", sysUser.Content)
	}

	// history[1]: assistant ack
	sysAck := payload.ConversationState.History[1].AssistantResponseMessage
	if sysAck == nil || sysAck.Content != "I will follow these instructions." {
		t.Fatalf("expected assistant ack for system prompt")
	}

	// history[2]: first user question
	firstUser := payload.ConversationState.History[2].UserInputMessage
	if firstUser == nil {
		t.Fatalf("expected third history item to be user message")
	}
	if !strings.Contains(firstUser.Content, "first-question") {
		t.Fatalf("expected first-question in user content, got %q", firstUser.Content)
	}

	// history[3]: assistant response
	historyAssistant := payload.ConversationState.History[3].AssistantResponseMessage
	if historyAssistant == nil {
		t.Fatalf("expected fourth history item to be assistant message")
	}
	if historyAssistant.Content != "assistant-structured" {
		t.Fatalf("expected assistant structured content to be preserved, got %q", historyAssistant.Content)
	}

	cur := payload.ConversationState.CurrentMessage.UserInputMessage
	if cur.Content != "tool-result-structured" {
		t.Fatalf("expected tool-result continuation content, got %q", cur.Content)
	}
	if cur.UserInputMessageContext == nil || len(cur.UserInputMessageContext.ToolResults) != 1 {
		t.Fatalf("expected one tool result in current context")
	}
	gotToolText := cur.UserInputMessageContext.ToolResults[0].Content[0].Text
	if gotToolText != "tool-result-structured" {
		t.Fatalf("expected structured tool result text, got %q", gotToolText)
	}
}

func TestOpenAIToKiroAssistantMapContentInHistory(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "u1"},
			{Role: "assistant", Content: map[string]interface{}{"type": "text", "text": "assistant-map"}},
			{Role: "user", Content: "u2"},
		},
	}

	payload := OpenAIToKiro(req, false)

	if len(payload.ConversationState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(payload.ConversationState.History))
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected second history entry to be assistant")
	}
	if assistant.Content != "assistant-map" {
		t.Fatalf("expected assistant map content preserved, got %q", assistant.Content)
	}
}

func TestOpenAIToKiroAssistantToolCallsDoNotInjectPlaceholder(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find weather"},
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "get_weather", Arguments: "{}"},
				}},
			},
			{Role: "user", Content: "continue"},
		},
	}

	payload := OpenAIToKiro(req, false)
	if len(payload.ConversationState.History) < 2 {
		t.Fatalf("expected history with assistant tool call")
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected assistant history entry")
	}
	if assistant.Content != "" {
		t.Fatalf("expected empty assistant content for tool-call-only turn, got %q", assistant.Content)
	}
}

func TestOpenAIConversationIDStableFromAnchor(t *testing.T) {
	baseMessages := []OpenAIMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Build calculator"},
		{Role: "assistant", Content: "Sure"},
		{Role: "user", Content: "Continue"},
	}

	reqA := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: baseMessages}
	reqB := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: append(baseMessages, OpenAIMessage{Role: "assistant", Content: "Next step"})}

	payloadA := OpenAIToKiro(reqA, false)
	payloadB := OpenAIToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}

func TestMergeConsecutiveMessages(t *testing.T) {
	// 无连续消息 — 不变
	msgs := []ClaudeMessage{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
	}
	merged := mergeConsecutiveMessages(msgs)
	if len(merged) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(merged))
	}

	// 连续 user 消息 — 合并
	msgs = []ClaudeMessage{
		{Role: "user", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "user", Content: "c"},
	}
	merged = mergeConsecutiveMessages(msgs)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(merged))
	}
	if merged[0].Role != "user" {
		t.Fatalf("expected role user, got %s", merged[0].Role)
	}
	blocks, ok := merged[0].Content.([]interface{})
	if !ok {
		t.Fatalf("expected merged content to be []interface{}")
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 content blocks, got %d", len(blocks))
	}

	// 混合场景：user user assistant user user
	msgs = []ClaudeMessage{
		{Role: "user", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "c"},
		{Role: "user", Content: "d"},
		{Role: "user", Content: "e"},
	}
	merged = mergeConsecutiveMessages(msgs)
	if len(merged) != 3 {
		t.Fatalf("expected 3 messages after merge, got %d", len(merged))
	}
	if merged[0].Role != "user" || merged[1].Role != "assistant" || merged[2].Role != "user" {
		t.Fatalf("unexpected roles: %s %s %s", merged[0].Role, merged[1].Role, merged[2].Role)
	}

	// content blocks 格式也能正确合并
	msgs = []ClaudeMessage{
		{Role: "user", Content: []interface{}{map[string]interface{}{"type": "text", "text": "x"}}},
		{Role: "user", Content: "y"},
	}
	merged = mergeConsecutiveMessages(msgs)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(merged))
	}
	blocks, ok = merged[0].Content.([]interface{})
	if !ok || len(blocks) != 2 {
		t.Fatalf("expected 2 content blocks from mixed formats, got %v", merged[0].Content)
	}
}

func TestClaudeToKiroConsecutiveUserMessages(t *testing.T) {
	// 这是 OpenClaw 触发 400 "Improperly formed request" 的场景：
	// 连续的 user 消息，没有 assistant 消息交替
	req := &ClaudeRequest{
		Model: "claude-sonnet-4.5",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "msg1"},
			{Role: "user", Content: "msg2"},
			{Role: "user", Content: "msg3"},
		},
	}

	payload := ClaudeToKiro(req, false)

	// history 不应该有连续的 UserInputMessage
	for i := 1; i < len(payload.ConversationState.History); i++ {
		prev := payload.ConversationState.History[i-1]
		cur := payload.ConversationState.History[i]
		if prev.UserInputMessage != nil && cur.UserInputMessage != nil {
			t.Fatalf("history[%d] and history[%d] are both user messages — should have been merged", i-1, i)
		}
	}

	// currentMessage 应该包含合并后的内容
	cur := payload.ConversationState.CurrentMessage.UserInputMessage
	if !strings.Contains(cur.Content, "msg1") || !strings.Contains(cur.Content, "msg2") || !strings.Contains(cur.Content, "msg3") {
		t.Fatalf("expected all message contents in current message, got %q", cur.Content)
	}
}

func TestClaudeConversationIDStableFromAnchor(t *testing.T) {
	reqA := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	reqB := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "ok"},
			{Role: "user", Content: "next"},
		},
	}

	payloadA := ClaudeToKiro(reqA, false)
	payloadB := ClaudeToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}
