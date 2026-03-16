package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleCountTokens Token 计数（Claude Code 会调用）
func (h *Handler) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Invalid JSON")
		return
	}

	estimatedTokens := estimateClaudeRequestInputTokens(&req)
	if estimatedTokens < 1 {
		estimatedTokens = 1
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]int{"input_tokens": estimatedTokens})
}

// handleClaudeMessages Claude API 处理
func (h *Handler) handleClaudeMessages(w http.ResponseWriter, r *http.Request) {
	h.handleClaudeMessagesInternal(w, r)
}

func (h *Handler) handleClaudeMessagesInternal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// 限制请求体大小为 100MB
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	// 读取请求
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	// 可选的请求体调试日志（通过环境变量 DEBUG_REQUESTS=true 启用）
	if os.Getenv("DEBUG_REQUESTS") == "true" {
		fmt.Printf("[DEBUG] Claude API Request Body: %s\n", string(body))
	}

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Invalid JSON: "+err.Error())
		return
	}

	// 请求内故障转移：最多尝试 3 个账号
	maxRetries := 3
	var lastErr error

	// 用户层 tier 决策（必须在 GetNextByTier 之前）
	uc := getUserContext(r.Context())
	var keyID string
	if uc != nil {
		keyID = uc.KeyID
	}

	// Abuse prevention: check rate/concurrency limits
	if keyID != "" {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		allowed, reason := OnRequestStart(keyID, strings.TrimSpace(ip))
		if !allowed {
			h.sendClaudeError(w, 429, "rate_limit_error", "Request blocked: "+reason)
			return
		}
		defer OnRequestEnd(keyID)
	}

	// Model pool routing: determined by model, not by user tier
	// Credit users can use any model; day card restrictions handled by ValidateKeyAccessForModel
	pool := ResolveModelPool(req.Model)
	if keyID != "" {
		info := config.FindApiKeyByID(keyID)
		if info != nil {
			_, err := config.ValidateKeyAccessForModel(info, pool)
			if err != nil {
				h.sendClaudeError(w, 403, "forbidden", err.Error())
				return
			}
		}
	}

	// Billing: PreAuthorize – lock estimated cost before calling upstream
	var preChargedPaid, preChargedGift float64
	if keyID != "" {
		estimatedInput := estimateClaudeRequestInputTokens(&req)
		maxTokens := 4096
		if req.MaxTokens > 0 {
			maxTokens = req.MaxTokens
		}
		var preErr error
		preChargedPaid, preChargedGift, preErr = PreAuthorize(keyID, req.Model, maxTokens, estimatedInput)
		if preErr != nil {
			h.sendClaudeError(w, 402, "insufficient_balance", preErr.Error())
			return
		}
	}

	tier := DeterminePoolTier(req.Model)

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 从对应号池获取账号
		account := h.pool.GetNextByTier(tier)
		if account == nil {
			RefundPreAuth(keyID, preChargedPaid, preChargedGift) // refund on no account
			h.sendClaudeError(w, 503, "api_error", fmt.Sprintf("No available accounts in %s pool", tier))
			return
		}

		// 检查并刷新 token
		if err := h.ensureValidToken(account); err != nil {
			h.pool.RecordError(account.ID, false)
			h.pool.ReleaseAccount(account.ID)
			lastErr = fmt.Errorf("token refresh failed: %v", err)
			continue
		}

		// 模型验证：根据订阅类型严格校验
		thinkingCfg := config.GetThinkingConfig()
		originalModel := req.Model
		mappedModel, validateErr := ValidateAndMapModel(req.Model, account.SubscriptionType)
		if validateErr != nil {
			h.pool.ReleaseAccount(account.ID)
			RefundPreAuth(keyID, preChargedPaid, preChargedGift)
			h.sendClaudeError(w, 400, "invalid_request_error", validateErr.Error())
			return
		}
		req.Model = mappedModel
		actualModel, thinking := ParseModelAndThinking(req.Model, thinkingCfg.Suffix)
		req.Model = actualModel
		estimatedInputTokens := estimateClaudeRequestInputTokens(&req)

		if originalModel != actualModel {
			fmt.Printf("[Request] Claude API | %s → %s | account: %s | stream: %v | attempt: %d\n", originalModel, actualModel, account.Email, req.Stream, attempt+1)
		} else {
			fmt.Printf("[Request] Claude API | model: %s | account: %s | stream: %v | attempt: %d\n", actualModel, account.Email, req.Stream, attempt+1)
		}

		// 转换请求
		kiroPayload := ClaudeToKiro(&req, thinking)

		// 流式或非流式 (pass preChargedUSD for billing reconciliation)
		if req.Stream {
			h.handleClaudeStream(w, account, kiroPayload, req.Model, originalModel, thinking, estimatedInputTokens, uc, preChargedPaid, preChargedGift)
		} else {
			h.handleClaudeNonStream(w, account, kiroPayload, req.Model, originalModel, thinking, estimatedInputTokens, uc, preChargedPaid, preChargedGift)
		}
		return // 成功或已处理错误，退出循环
	}

	// 所有重试都失败 – refund pre-auth
	RefundPreAuth(keyID, preChargedPaid, preChargedGift)
	h.recordFailure()
	errMsg := "All accounts failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	h.sendClaudeError(w, 503, "api_error", errMsg)
}

// handleClaudeStream Claude 流式响应
func (h *Handler) handleClaudeStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, model, originalModel string, thinking bool, estimatedInputTokens int, uc *UserContext, preChargedPaid float64, preChargedGift float64) {
	requestStart := time.Now()
	requestID := genRequestID()
	fmt.Printf("[req-%s] → Claude Stream | %s → %s | account: %s | input≈%dK | thinking=%v\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000, thinking)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.sendClaudeError(w, 500, "api_error", "Streaming not supported")
		return
	}

	// 获取 thinking 输出格式配置
	thinkingFormat := config.GetThinkingConfig().ClaudeFormat

	msgID := "msg_" + uuid.New().String()
	var inputTokens, outputTokens int
	var credits float64
	var toolUses []KiroToolUse
	var nextContentIndex int
	var rawContentBuilder strings.Builder
	var rawThinkingBuilder strings.Builder
	activeBlockIndex := -1
	activeBlockType := ""
	startInputTokens := estimatedInputTokens

	closeActiveBlock := func() {
		if activeBlockIndex < 0 {
			return
		}
		h.sendSSE(w, flusher, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": activeBlockIndex,
		})
		activeBlockIndex = -1
		activeBlockType = ""
	}

	startContentBlock := func(blockType string) {
		if activeBlockType == blockType {
			return
		}
		closeActiveBlock()

		idx := nextContentIndex
		nextContentIndex++

		if blockType == "thinking" {
			h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": idx,
				"content_block": map[string]string{
					"type":     "thinking",
					"thinking": "",
				},
			})
		} else {
			h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": idx,
				"content_block": map[string]string{
					"type": "text",
					"text": "",
				},
			})
		}

		activeBlockIndex = idx
		activeBlockType = blockType
	}

	// Thinking 标签解析状态
	var textBuffer string
	var inThinkingBlock bool
	var dropTagThinking bool
	var thinkingSource thinkingStreamSource

	// 发送文本的辅助函数
	// thinkingState: 0=普通内容, 1=thinking开始, 2=thinking中间, 3=thinking结束
	sendText := func(text string, thinkingState int) {
		if thinkingState == 0 {
			// 普通内容
			if text == "" {
				return
			}
			startContentBlock("text")
			h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": activeBlockIndex,
				"delta": map[string]string{"type": "text_delta", "text": text},
			})
			return
		}

		if !thinking {
			return
		}

		switch thinkingFormat {
		case "think":
			var outputText string
			switch thinkingState {
			case 1:
				outputText = "<think>" + text
			case 2:
				outputText = text
			case 3:
				outputText = text + "</think>"
			}
			if outputText == "" {
				return
			}
			startContentBlock("text")
			h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": activeBlockIndex,
				"delta": map[string]string{"type": "text_delta", "text": outputText},
			})
		case "reasoning_content":
			if text == "" {
				return
			}
			startContentBlock("text")
			h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": activeBlockIndex,
				"delta": map[string]string{"type": "text_delta", "text": text},
			})
		default:
			if thinkingState == 3 && text == "" {
				if activeBlockType == "thinking" {
					closeActiveBlock()
				}
				return
			}
			if text != "" {
				startContentBlock("thinking")
				h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": activeBlockIndex,
					"delta": map[string]string{"type": "thinking_delta", "thinking": text},
				})
			}
			if thinkingState == 3 && activeBlockType == "thinking" {
				closeActiveBlock()
			}
		}
	}

	// 处理文本，解析 <thinking> 标签
	var thinkingStarted bool
	var eventThinkingOpen bool

	processClaudeText := func(text string, isThinking bool, forceFlush bool) {
		if isThinking && !thinking {
			return
		}

		// 如果是 reasoningContentEvent，直接输出
		if isThinking {
			if !allowReasoningSource(&thinkingSource) {
				return
			}
			if !thinkingStarted {
				sendText(text, 1)
				thinkingStarted = true
				eventThinkingOpen = true
			} else {
				sendText(text, 2)
			}
			return
		}

		if eventThinkingOpen {
			sendText("", 3)
			eventThinkingOpen = false
			thinkingStarted = false
		}

		textBuffer += text

		for {
			if !inThinkingBlock {
				thinkingStart := strings.Index(textBuffer, "<thinking>")
				if thinkingStart != -1 {
					if thinkingStart > 0 {
						sendText(textBuffer[:thinkingStart], 0)
					}
					textBuffer = textBuffer[thinkingStart+10:]
					inThinkingBlock = true
					dropTagThinking = !allowTagSource(&thinkingSource)
					thinkingStarted = false
				} else if forceFlush || len([]rune(textBuffer)) > 50 {
					// 使用 rune 切片来正确处理 Unicode 字符
					runes := []rune(textBuffer)
					safeLen := len(runes)
					if !forceFlush {
						safeLen = max(0, len(runes)-15)
					}
					if safeLen > 0 {
						sendText(string(runes[:safeLen]), 0)
						textBuffer = string(runes[safeLen:])
					}
					break
				} else {
					break
				}
			} else {
				thinkingEnd := strings.Index(textBuffer, "</thinking>")
				if thinkingEnd != -1 {
					content := textBuffer[:thinkingEnd]
					if !dropTagThinking {
						if !thinkingStarted {
							sendText(content, 1)
							sendText("", 3)
						} else {
							sendText(content, 3)
						}
					}
					textBuffer = textBuffer[thinkingEnd+11:]
					inThinkingBlock = false
					dropTagThinking = false
					thinkingStarted = false
				} else if forceFlush {
					if textBuffer != "" {
						if !dropTagThinking {
							if !thinkingStarted {
								sendText(textBuffer, 1)
								sendText("", 3)
							} else {
								sendText(textBuffer, 3)
							}
						}
						textBuffer = ""
					}
					inThinkingBlock = false
					dropTagThinking = false
					thinkingStarted = false
					break
				} else {
					// 流式输出 thinking 块内的内容
					runes := []rune(textBuffer)
					if len(runes) > 20 {
						safeLen := len(runes) - 15
						if safeLen > 0 {
							if !dropTagThinking {
								if !thinkingStarted {
									sendText(string(runes[:safeLen]), 1)
									thinkingStarted = true
								} else {
									sendText(string(runes[:safeLen]), 2)
								}
							}
							textBuffer = string(runes[safeLen:])
						}
					}
					break
				}
			}
		}
	}

	// 发送 message_start
	h.sendSSE(w, flusher, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"content":       []interface{}{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  startInputTokens,
				"output_tokens": 0,
			},
		},
	})

	callback := &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			if text == "" {
				return
			}
			if isThinking {
				rawThinkingBuilder.WriteString(text)
			} else {
				rawContentBuilder.WriteString(text)
			}
			processClaudeText(text, isThinking, false)
		},
		OnToolUse: func(tu KiroToolUse) {
			tu.Name = RestoreToolName(tu.Name)
			// 先刷新缓冲区
			processClaudeText("", false, true)
			rawContentBuilder.WriteString(tu.Name)
			if b, err := json.Marshal(tu.Input); err == nil {
				rawContentBuilder.Write(b)
			}

			toolUses = append(toolUses, tu)
			closeActiveBlock()

			idx := nextContentIndex
			nextContentIndex++

			h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": idx,
				"content_block": map[string]interface{}{
					"type":  "tool_use",
					"id":    tu.ToolUseID,
					"name":  tu.Name,
					"input": map[string]interface{}{},
				},
			})

			inputJSON, _ := json.Marshal(tu.Input)
			h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": idx,
				"delta": map[string]interface{}{
					"type":         "input_json_delta",
					"partial_json": string(inputJSON),
				},
			})

			h.sendSSE(w, flusher, "content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": idx,
			})
		},
		OnComplete: func(inTok, outTok int) {
			inputTokens = inTok
			outputTokens = outTok
		},
		OnError: func(err error) {
			h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota"))
		},
		OnCredits: func(c float64) {
			credits = c
		},
	}

	upstreamErr, err := CallKiroAPI(account, payload, callback)
	if err != nil {
		h.recordFailure()
		h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota"))
		payloadKB := 0
		if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
			payloadKB = len(payloadBytes) / 1024
			fmt.Printf("[ERROR] Claude Stream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] Claude Stream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		h.addCallLogErrorWithKey("Claude", originalModel, model, account.Email, true, err.Error(), payloadKB, uc)
		if upstreamErr != nil {
			WriteClaudeStreamError(w, upstreamErr.ToAppError(""))
		} else {
			h.sendSSE(w, flusher, "error", map[string]interface{}{
				"type":  "error",
				"error": map[string]string{"type": "api_error", "message": err.Error()},
			})
		}
		return
	}
	processClaudeText("", false, true)
	if eventThinkingOpen {
		sendText("", 3)
		eventThinkingOpen = false
	}
	closeActiveBlock()

	inputTokens = estimatedInputTokens
	outputContent, extractedReasoning := extractThinkingFromContent(rawContentBuilder.String())
	thinkingOutput := rawThinkingBuilder.String()
	if thinking && thinkingOutput == "" && extractedReasoning != "" {
		thinkingOutput = extractedReasoning
	}
	if !thinking {
		thinkingOutput = ""
	}
	outputTokens = estimateClaudeOutputTokens(outputContent, thinkingOutput, toolUses)

	h.recordSuccess(inputTokens, outputTokens, credits)
	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	// Billing: Reconcile pre-authorized amount with actual credits
	if uc != nil && uc.KeyID != "" && (preChargedPaid > 0 || preChargedGift > 0) {
		actualCredits := credits
		if actualCredits <= 0 {
			// Kiro API didn't return credits – fallback to estimation
			actualCredits = EstimateCredits(outputTokens, inputTokens)
		}
		actualPaid, actualGift := Reconcile(uc.KeyID, model, actualCredits, preChargedPaid, preChargedGift)
		uc.ActualPaidUSD = actualPaid
		uc.ActualGiftUSD = actualGift
	}

	// 发送 message_delta
	stopReason := "end_turn"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}

	durationMs := time.Since(requestStart).Milliseconds()
	h.addCallLogWithKey("Claude", originalModel, model, account.Email, account.SubscriptionType, inputTokens, outputTokens, true, credits, "", "", stopReason, requestID, durationMs, uc)
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, stopReason, credits, durationMs)

	h.sendSSE(w, flusher, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
		"usage": map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	})

	h.sendSSE(w, flusher, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
}

// handleClaudeNonStream Claude 非流式响应
func (h *Handler) handleClaudeNonStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, model, originalModel string, thinking bool, estimatedInputTokens int, uc *UserContext, preChargedPaid float64, preChargedGift float64) {
	requestStart := time.Now()
	requestID := genRequestID()
	fmt.Printf("[req-%s] → Claude NonStream | %s → %s | account: %s | input≈%dK\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000)

	var content string
	var thinkingContent string
	var toolUses []KiroToolUse
	var inputTokens, outputTokens int
	var credits float64

	callback := &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			if isThinking {
				thinkingContent += text
			} else {
				content += text
			}
		},
		OnToolUse: func(tu KiroToolUse) {
			tu.Name = RestoreToolName(tu.Name)
			toolUses = append(toolUses, tu)
		},
		OnComplete: func(inTok, outTok int) {
			inputTokens = inTok
			outputTokens = outTok
		},
		OnError: func(err error) {
			h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429"))
		},
		OnCredits: func(c float64) {
			credits = c
		},
	}

	upstreamErr, err := CallKiroAPI(account, payload, callback)
	if err != nil {
		h.recordFailure()
		h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429"))
		// Billing: refund pre-auth on API failure
		if uc != nil {
			RefundPreAuth(uc.KeyID, preChargedPaid, preChargedGift)
		}
		payloadKB := 0
		if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
			payloadKB = len(payloadBytes) / 1024
			fmt.Printf("[ERROR] Claude NonStream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] Claude NonStream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		h.addCallLogErrorWithKey("Claude", originalModel, model, account.Email, false, err.Error(), payloadKB, uc)
		if upstreamErr != nil {
			appErr := upstreamErr.ToAppError("")
			WriteClaudeError(w, appErr, upstreamErr.StatusCode)
		} else {
			h.sendClaudeError(w, 500, "api_error", err.Error())
		}
		return
	}

	// 合并 thinking 内容（如果有 reasoningContentEvent 的内容）
	thinkingFormat := config.GetThinkingConfig().ClaudeFormat
	finalContent, extractedReasoning := extractThinkingFromContent(content)
	if thinking && thinkingContent == "" && extractedReasoning != "" {
		thinkingContent = extractedReasoning
	}
	if !thinking {
		thinkingContent = ""
	}

	inputTokens = estimatedInputTokens
	outputTokens = estimateClaudeOutputTokens(finalContent, thinkingContent, toolUses)

	h.recordSuccess(inputTokens, outputTokens, credits)
	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	// Billing: Reconcile pre-authorized amount with actual credits
	if uc != nil && uc.KeyID != "" && (preChargedPaid > 0 || preChargedGift > 0) {
		actualCredits := credits
		if actualCredits <= 0 {
			actualCredits = EstimateCredits(outputTokens, inputTokens)
		}
		actualPaid, actualGift := Reconcile(uc.KeyID, model, actualCredits, preChargedPaid, preChargedGift)
		uc.ActualPaidUSD = actualPaid
		uc.ActualGiftUSD = actualGift
	}

	stopReason := "end_turn"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}
	durationMs := time.Since(requestStart).Milliseconds()
	h.addCallLogWithKey("Claude", originalModel, model, account.Email, account.SubscriptionType, inputTokens, outputTokens, false, credits, "", "", stopReason, requestID, durationMs, uc)
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, stopReason, credits, durationMs)

	if thinking && thinkingContent != "" {
		switch thinkingFormat {
		case "think":
			finalContent = "<think>" + thinkingContent + "</think>" + finalContent
			thinkingContent = ""
		case "reasoning_content":
			finalContent = thinkingContent + finalContent // Claude 格式不支持 reasoning_content，直接拼接
			thinkingContent = ""
		default:
		}
	}

	resp := KiroToClaudeResponse(finalContent, thinkingContent, toolUses, inputTokens, outputTokens, model)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) sendClaudeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}
