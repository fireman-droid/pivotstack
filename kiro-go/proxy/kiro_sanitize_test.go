package proxy

import (
	"strings"
	"testing"
)

// ============== Schema sanitization ==============

func TestSanitizeJSONSchema_RequiredEmptyArrayDropped(t *testing.T) {
	in := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"x": map[string]interface{}{"type": "string"}},
		"required":   []interface{}{},
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	if _, has := out["required"]; has {
		t.Errorf("empty required should be dropped, got: %v", out["required"])
	}
}

func TestSanitizeJSONSchema_RequiredNullDropped(t *testing.T) {
	in := map[string]interface{}{
		"type":     "object",
		"required": nil,
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	if _, has := out["required"]; has {
		t.Errorf("null required should be dropped")
	}
}

func TestSanitizeJSONSchema_PropertiesNullBecomesEmptyObject(t *testing.T) {
	in := map[string]interface{}{
		"type":       "object",
		"properties": nil,
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	props, ok := out["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties should become empty map, got: %T", out["properties"])
	}
	if len(props) != 0 {
		t.Errorf("expected empty properties, got: %v", props)
	}
}

func TestSanitizeJSONSchema_TypeMissingFilledAsObject(t *testing.T) {
	in := map[string]interface{}{
		"properties": map[string]interface{}{},
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	if out["type"] != "object" {
		t.Errorf("missing type should default to 'object', got: %v", out["type"])
	}
}

func TestSanitizeJSONSchema_AdditionalPropertiesInvalidNormalized(t *testing.T) {
	in := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": "yes", // invalid: should be bool or object
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	if out["additionalProperties"] != true {
		t.Errorf("invalid additionalProperties should default to true, got: %v", out["additionalProperties"])
	}
}

func TestSanitizeJSONSchema_RecursiveOnNestedProperties(t *testing.T) {
	in := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"nested": map[string]interface{}{
				"type":     "object",
				"required": []interface{}{}, // should be dropped at nested level
			},
		},
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	props := out["properties"].(map[string]interface{})
	nested := props["nested"].(map[string]interface{})
	if _, has := nested["required"]; has {
		t.Errorf("nested empty required should be dropped, got: %v", nested["required"])
	}
}

func TestSanitizeJSONSchema_NilInputReturnsMinimalSchema(t *testing.T) {
	out := sanitizeJSONSchema(nil).(map[string]interface{})
	if out["type"] != "object" {
		t.Errorf("nil → minimal schema with type=object, got: %v", out)
	}
}

func TestSanitizeJSONSchema_KeepsValidFieldsUnchanged(t *testing.T) {
	in := map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{"x": map[string]interface{}{"type": "string"}},
		"required":             []interface{}{"x"},
		"additionalProperties": false,
	}
	out := sanitizeJSONSchema(in).(map[string]interface{})
	req, _ := out["required"].([]interface{})
	if len(req) != 1 || req[0] != "x" {
		t.Errorf("valid required should be preserved, got: %v", req)
	}
	if out["additionalProperties"] != false {
		t.Errorf("additionalProperties=false should be preserved, got: %v", out["additionalProperties"])
	}
}

// ============== Tool description / name limits ==============

func TestProcessToolsLongDescriptions_MovesToSystemDoc(t *testing.T) {
	long := strings.Repeat("a", kiroToolDescMaxLen+1)
	tools := []KiroToolWrapper{newTestTool("search", long, nil)}
	out, doc := processToolsLongDescriptions(tools)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool kept, got %d", len(out))
	}
	if !strings.Contains(out[0].ToolSpecification.Description, "Full documentation") {
		t.Errorf("description should be replaced by reference, got: %q", out[0].ToolSpecification.Description)
	}
	if !strings.Contains(doc, "## Tool: search") {
		t.Errorf("system doc should contain tool name header, got: %q", doc)
	}
	if !strings.Contains(doc, long[:50]) {
		t.Errorf("system doc should embed full description")
	}
}

func TestProcessToolsLongDescriptions_ShortKeptAsIs(t *testing.T) {
	tools := []KiroToolWrapper{newTestTool("ok", "short desc", nil)}
	out, doc := processToolsLongDescriptions(tools)
	if doc != "" {
		t.Errorf("no doc expected for short desc, got: %q", doc)
	}
	if out[0].ToolSpecification.Description != "short desc" {
		t.Errorf("short desc should be preserved")
	}
}

func TestFilterToolsByNameLen_DropsOverLong(t *testing.T) {
	long := strings.Repeat("x", kiroToolNameMaxLen+1)
	tools := []KiroToolWrapper{
		newTestTool("ok", "d", nil),
		newTestTool(long, "d", nil),
		newTestTool("ok2", "d", nil),
	}
	out := filterToolsByNameLen(tools)
	if len(out) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(out))
	}
	for _, tw := range out {
		if len(tw.ToolSpecification.Name) > kiroToolNameMaxLen {
			t.Errorf("over-long name leaked through: %s", tw.ToolSpecification.Name)
		}
	}
}

// ============== Tool pairing ==============

func TestValidateAndCleanToolPairing_OrphanToolUseRemoved(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "first", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{
			Content: "calling",
			ToolUses: []KiroToolUse{
				{ToolUseID: "u1", Name: "search"},
				{ToolUseID: "u2", Name: "search"}, // orphan: no matching result
			},
		}},
		{UserInputMessage: &KiroUserInputMessage{
			Content: "results",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				ToolResults: []KiroToolResult{{ToolUseID: "u1", Content: []KiroResultContent{{Text: "ok"}}, Status: "success"}},
			},
		}},
	}
	out := validateAndCleanToolPairingHistory(history, nil)
	uses := out[1].AssistantResponseMessage.ToolUses
	if len(uses) != 1 || uses[0].ToolUseID != "u1" {
		t.Errorf("orphan tool_use u2 should be removed, got: %v", uses)
	}
}

func TestValidateAndCleanToolPairing_OrphanToolResultToText(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "first", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{
			Content:  "calling",
			ToolUses: []KiroToolUse{{ToolUseID: "u1", Name: "search"}},
		}},
		{UserInputMessage: &KiroUserInputMessage{
			Content: "results",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				ToolResults: []KiroToolResult{
					{ToolUseID: "u1", Content: []KiroResultContent{{Text: "ok"}}, Status: "success"},
					{ToolUseID: "ghost", Content: []KiroResultContent{{Text: "phantom"}}, Status: "success"},
				},
			},
		}},
	}
	out := validateAndCleanToolPairingHistory(history, nil)
	last := out[2].UserInputMessage
	if len(last.UserInputMessageContext.ToolResults) != 1 || last.UserInputMessageContext.ToolResults[0].ToolUseID != "u1" {
		t.Errorf("orphan tool_result ghost should be removed, got: %v", last.UserInputMessageContext.ToolResults)
	}
	if !strings.Contains(last.Content, "phantom") {
		t.Errorf("orphan result text should be appended to content, got: %q", last.Content)
	}
}

// ============== Ensure assistant before tool_results ==============

func TestEnsureAssistantBeforeToolResults_ConvertsOrphanedUserToText(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{
			Content: "what",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				ToolResults: []KiroToolResult{{ToolUseID: "u1", Content: []KiroResultContent{{Text: "result text"}}, Status: "success"}},
			},
		}},
	}
	out := ensureAssistantBeforeToolResultsHistory(history, nil)
	if len(out[0].UserInputMessage.UserInputMessageContext.ToolResults) != 0 {
		t.Errorf("toolResults should be cleared when no preceding assistant")
	}
	if !strings.Contains(out[0].UserInputMessage.Content, "result text") {
		t.Errorf("toolResults should be converted to text in content, got: %q", out[0].UserInputMessage.Content)
	}
}

func TestEnsureAssistantBeforeToolResults_CurrentMessageHandled(t *testing.T) {
	current := &KiroUserInputMessage{
		Content: "ask",
		Origin:  "AI_EDITOR",
		UserInputMessageContext: &UserInputMessageContext{
			ToolResults: []KiroToolResult{{ToolUseID: "u1", Content: []KiroResultContent{{Text: "phantom"}}, Status: "success"}},
		},
	}
	out := ensureAssistantBeforeToolResultsHistory(nil, current)
	if len(out) != 0 {
		t.Errorf("nil history should stay empty")
	}
	if len(current.UserInputMessageContext.ToolResults) != 0 {
		t.Errorf("current toolResults should be cleared when no preceding assistant")
	}
	if !strings.Contains(current.Content, "phantom") {
		t.Errorf("current content should embed orphan results, got: %q", current.Content)
	}
}

// ============== Strip all tool content (no tools defined) ==============

func TestStripAllToolContent_RemovesAndConvertsToText(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "hi", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{
			Content:  "calling",
			ToolUses: []KiroToolUse{{ToolUseID: "u1", Name: "search", Input: map[string]interface{}{"q": "v"}}},
		}},
		{UserInputMessage: &KiroUserInputMessage{
			Content: "out",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				ToolResults: []KiroToolResult{{ToolUseID: "u1", Content: []KiroResultContent{{Text: "data"}}, Status: "success"}},
			},
		}},
	}
	out := stripAllToolContentInHistory(history)
	if len(out[1].AssistantResponseMessage.ToolUses) != 0 {
		t.Errorf("toolUses should be stripped")
	}
	if !strings.Contains(out[1].AssistantResponseMessage.Content, "search") {
		t.Errorf("toolUses should be converted to text content, got: %q", out[1].AssistantResponseMessage.Content)
	}
	if len(out[2].UserInputMessage.UserInputMessageContext.ToolResults) != 0 {
		t.Errorf("toolResults should be stripped")
	}
	if !strings.Contains(out[2].UserInputMessage.Content, "data") {
		t.Errorf("toolResults should be converted to text, got: %q", out[2].UserInputMessage.Content)
	}
}

// ============== First message must be user ==============

func TestEnsureFirstHistoryIsUser_PrependsWhenAssistantFirst(t *testing.T) {
	history := []KiroHistoryMessage{
		{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "hello"}},
	}
	out := ensureFirstHistoryIsUser(history, "claude-opus-4.6")
	if len(out) != 2 {
		t.Fatalf("expected 2 messages after prepend, got %d", len(out))
	}
	if out[0].UserInputMessage == nil {
		t.Errorf("first message should be user")
	}
	if out[0].UserInputMessage.ModelID != "claude-opus-4.6" {
		t.Errorf("synthetic user should inherit modelID, got %s", out[0].UserInputMessage.ModelID)
	}
}

func TestEnsureFirstHistoryIsUser_NoOpWhenAlreadyUser(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "hi", Origin: "AI_EDITOR"}},
	}
	out := ensureFirstHistoryIsUser(history, "x")
	if len(out) != 1 {
		t.Errorf("should not prepend when first is already user")
	}
}

// ============== Alternating roles ==============

func TestEnsureAlternatingHistory_InsertsSyntheticAssistantBetweenUsers(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "1", Origin: "AI_EDITOR"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "2", Origin: "AI_EDITOR"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "3", Origin: "AI_EDITOR"}},
	}
	out := ensureAlternatingHistory(history, "")
	if len(out) != 5 {
		t.Fatalf("expected 5 messages (3 user + 2 synthetic assistant), got %d", len(out))
	}
	if out[1].AssistantResponseMessage == nil || out[1].AssistantResponseMessage.Content != kiroEmptyContent {
		t.Errorf("expected synthetic assistant at idx 1, got: %+v", out[1])
	}
	if out[3].AssistantResponseMessage == nil {
		t.Errorf("expected synthetic assistant at idx 3, got: %+v", out[3])
	}
}

func TestEnsureAlternatingHistory_NoOpWhenAlreadyAlternating(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "1", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "2"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "3", Origin: "AI_EDITOR"}},
	}
	out := ensureAlternatingHistory(history, "")
	if len(out) != 3 {
		t.Errorf("alternating history should be unchanged, got len %d", len(out))
	}
}

// ============== Merge adjacent ==============

func TestMergeAdjacentHistoryMessages_MergesUsers(t *testing.T) {
	history := []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "A", Origin: "AI_EDITOR"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "B", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "C"}},
	}
	out := mergeAdjacentHistoryMessages(history)
	if len(out) != 2 {
		t.Fatalf("expected merge to 2 messages, got %d", len(out))
	}
	if !strings.Contains(out[0].UserInputMessage.Content, "A") || !strings.Contains(out[0].UserInputMessage.Content, "B") {
		t.Errorf("merged content should contain both, got: %q", out[0].UserInputMessage.Content)
	}
}

// ============== End-to-end via NormalizeKiroPayload ==============

func TestNormalizeKiroPayload_FixesInvalidToolSchema(t *testing.T) {
	payload := &KiroPayload{}
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: "hello",
		ModelID: "claude-opus-4.6",
		Origin:  "AI_EDITOR",
		UserInputMessageContext: &UserInputMessageContext{
			Tools: []KiroToolWrapper{newTestTool("search", "do search", map[string]interface{}{
				"type":     "object",
				"required": []interface{}{}, // invalid empty required
			})},
		},
	}

	NormalizeKiroPayload(payload)

	tool := payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools[0]
	schema := tool.ToolSpecification.InputSchema.JSON.(map[string]interface{})
	if _, has := schema["required"]; has {
		t.Errorf("invalid required should be removed by Normalize, got: %v", schema["required"])
	}
}

func TestNormalizeKiroPayload_StripsToolContentWhenNoTools(t *testing.T) {
	payload := &KiroPayload{}
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: "current",
		ModelID: "x",
		Origin:  "AI_EDITOR",
	}
	payload.ConversationState.History = []KiroHistoryMessage{
		{UserInputMessage: &KiroUserInputMessage{Content: "hi", Origin: "AI_EDITOR"}},
		{AssistantResponseMessage: &KiroAssistantResponseMessage{
			Content:  "x",
			ToolUses: []KiroToolUse{{ToolUseID: "u1", Name: "search"}},
		}},
		{UserInputMessage: &KiroUserInputMessage{
			Content: "y",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				ToolResults: []KiroToolResult{{ToolUseID: "u1", Content: []KiroResultContent{{Text: "data"}}, Status: "success"}},
			},
		}},
	}

	NormalizeKiroPayload(payload)

	for i, h := range payload.ConversationState.History {
		if h.AssistantResponseMessage != nil && len(h.AssistantResponseMessage.ToolUses) > 0 {
			t.Errorf("history[%d] still has toolUses after strip", i)
		}
		if h.UserInputMessage != nil && h.UserInputMessage.UserInputMessageContext != nil &&
			len(h.UserInputMessage.UserInputMessageContext.ToolResults) > 0 {
			t.Errorf("history[%d] still has toolResults after strip", i)
		}
	}
}

func TestNormalizeKiroPayload_EnforcesAlternationAndUserStart(t *testing.T) {
	payload := &KiroPayload{}
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: "current",
		ModelID: "claude-opus-4.6",
		Origin:  "AI_EDITOR",
	}
	// History 故意是：[assistant, user, user]（首条不是 user 且有连续 user）
	payload.ConversationState.History = []KiroHistoryMessage{
		{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "first"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "u1", Origin: "AI_EDITOR"}},
		{UserInputMessage: &KiroUserInputMessage{Content: "u2", Origin: "AI_EDITOR"}},
	}

	NormalizeKiroPayload(payload)

	h := payload.ConversationState.History
	if len(h) == 0 || h[0].UserInputMessage == nil {
		t.Fatalf("first message should be user, got: %+v", h)
	}
	for i := 1; i < len(h); i++ {
		prevUser := h[i-1].UserInputMessage != nil
		curUser := h[i].UserInputMessage != nil
		if prevUser && curUser {
			t.Errorf("found consecutive user messages at idx %d-%d", i-1, i)
		}
	}
	// history 末尾必须不是 user（防止跟 current user 形成连续 user）
	if h[len(h)-1].UserInputMessage != nil {
		t.Errorf("history must not end with user (current is user), got tail: %+v", h[len(h)-1])
	}
}

func TestNormalizeKiroPayload_EmptyContentFallsBackToEmpty(t *testing.T) {
	payload := &KiroPayload{}
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: "   ", // whitespace only
		ModelID: "x",
		Origin:  "AI_EDITOR",
	}
	NormalizeKiroPayload(payload)
	got := payload.ConversationState.CurrentMessage.UserInputMessage.Content
	if got != kiroEmptyContent {
		t.Errorf("empty content should become %q, got %q", kiroEmptyContent, got)
	}
}

func TestNormalizeKiroPayload_Idempotent(t *testing.T) {
	build := func() *KiroPayload {
		p := &KiroPayload{}
		p.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
			Content: "hello",
			ModelID: "claude-opus-4.6",
			Origin:  "AI_EDITOR",
			UserInputMessageContext: &UserInputMessageContext{
				Tools: []KiroToolWrapper{newTestTool("search", "do search", map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{"q": map[string]interface{}{"type": "string"}},
					"required":   []interface{}{}, // bad
				})},
			},
		}
		p.ConversationState.History = []KiroHistoryMessage{
			{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "x"}},
			{UserInputMessage: &KiroUserInputMessage{Content: "u", Origin: "AI_EDITOR"}},
			{UserInputMessage: &KiroUserInputMessage{Content: "u2", Origin: "AI_EDITOR"}},
		}
		return p
	}

	p1 := build()
	NormalizeKiroPayload(p1)
	p2 := build()
	NormalizeKiroPayload(p2)
	NormalizeKiroPayload(p2) // 第二次应该是 no-op

	if len(p1.ConversationState.History) != len(p2.ConversationState.History) {
		t.Errorf("not idempotent: history len differs %d vs %d",
			len(p1.ConversationState.History), len(p2.ConversationState.History))
	}
}

// ============== End-to-end via ClaudeToKiro ==============

func TestClaudeToKiro_E2E_BadMCPSchemaFixedAutomatically(t *testing.T) {
	// 模拟 Cursor / Cline 等 MCP 客户端发来的不规范 schema
	req := &ClaudeRequest{
		Model:     "claude-opus-4.6",
		MaxTokens: 100,
		Messages: []ClaudeMessage{{Role: "user", Content: "hello"}},
		Tools: []ClaudeTool{{
			Name:        "list_files",
			Description: "list files",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []interface{}{}, // 触发 Improperly formed
			},
		}},
	}

	payload := ClaudeToKiro(req, false)
	if payload == nil {
		t.Fatal("payload nil")
	}
	tools := payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	schema := tools[0].ToolSpecification.InputSchema.JSON.(map[string]interface{})
	if _, has := schema["required"]; has {
		t.Errorf("E2E: invalid required should be cleaned by ClaudeToKiro, got: %v", schema["required"])
	}
}

// ============== Helpers ==============

func newTestTool(name, desc string, schema map[string]interface{}) KiroToolWrapper {
	var w KiroToolWrapper
	w.ToolSpecification.Name = name
	w.ToolSpecification.Description = desc
	w.ToolSpecification.InputSchema = InputSchema{JSON: schema}
	return w
}
