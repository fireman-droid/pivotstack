package proxy

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// handleOpenAIStream OpenAI 流式响应。返回 ChannelResult（成功）或 KiroExecError（错误）。
// 此函数**不**做计费、不写日志、不更新 success/failure stats — 由 caller 负责。
func (h *Handler) handleOpenAIStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, upstreamModel, originalModel, billingModel string, stealthSwapped bool, thinking bool, estimatedInputTokens int, requestID string) (*ChannelResult, *KiroExecError) {
	requestStart := time.Now()
	if requestID == "" {
		requestID = genRequestID()
	}
	model := upstreamModel
	payloadKB := 0
	if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
		payloadKB = len(payloadBytes) / 1024
	}
	fmt.Printf("[req-%s] → OpenAI Stream | %s → %s | account: %s | input≈%dK | thinking=%v\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000, thinking)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("streaming not supported")
		h.sendOpenAIError(w, 500, "server_error", "Streaming not supported")
		return nil, &KiroExecError{Err: err, Retryable: false, ResponseStarted: false, PayloadKB: payloadKB}
	}

	// 获取 thinking 输出格式配置
	thinkingFormat := config.GetThinkingConfig().OpenAIFormat

	chatID := "chatcmpl-" + uuid.New().String()
	var toolCalls []ToolCall
	var toolCallIndex int
	var inputTokens, outputTokens int
	var credits float64
	var rawContentBuilder strings.Builder
	var rawReasoningBuilder strings.Builder
	headersSent := false

	// Thinking 标签解析状态
	var textBuffer string
	var inThinkingBlock bool
	var dropTagThinking bool
	var thinkingSource thinkingStreamSource

	// 发送 chunk 的辅助函数
	// thinkingState: 0=普通内容, 1=thinking开始, 2=thinking中间, 3=thinking结束
	sendChunk := func(content string, thinkingState int) {
		if content == "" && thinkingState == 2 {
			return
		}
		headersSent = true

		var chunk map[string]interface{}

		if thinkingState > 0 {
			if !thinking {
				return
			}
			// thinking 内容
			switch thinkingFormat {
			case "thinking":
				// 流式输出标签
				var text string
				switch thinkingState {
				case 1: // 开始
					text = "<thinking>" + content
				case 2: // 中间
					text = content
				case 3: // 结束
					text = content + "</thinking>"
				}
				if text == "" {
					return
				}
				chunk = map[string]interface{}{
					"id":      chatID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   originalModel,
					"choices": []map[string]interface{}{{
						"index":         0,
						"delta":         map[string]string{"content": text},
						"finish_reason": nil,
					}},
				}
			case "think":
				var text string
				switch thinkingState {
				case 1:
					text = "<think>" + content
				case 2:
					text = content
				case 3:
					text = content + "</think>"
				}
				if text == "" {
					return
				}
				chunk = map[string]interface{}{
					"id":      chatID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   originalModel,
					"choices": []map[string]interface{}{{
						"index":         0,
						"delta":         map[string]string{"content": text},
						"finish_reason": nil,
					}},
				}
			default: // "reasoning_content"
				if content == "" {
					return
				}
				chunk = map[string]interface{}{
					"id":      chatID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   originalModel,
					"choices": []map[string]interface{}{{
						"index":         0,
						"delta":         map[string]string{"reasoning_content": content},
						"finish_reason": nil,
					}},
				}
			}
		} else {
			// 普通内容
			if content == "" {
				return
			}
			chunk = map[string]interface{}{
				"id":      chatID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   originalModel,
				"choices": []map[string]interface{}{{
					"index":         0,
					"delta":         map[string]string{"content": content},
					"finish_reason": nil,
				}},
			}
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}

	// 处理文本，解析 <thinking> 标签
	// thinkingStarted 用于跟踪是否已发送开始标签
	var thinkingStarted bool
	var eventThinkingOpen bool

	processText := func(text string, isThinking bool, forceFlush bool) {
		if isThinking && !thinking {
			return
		}

		// 如果是 reasoningContentEvent，直接输出
		if isThinking {
			if !allowReasoningSource(&thinkingSource) {
				return
			}
			if !thinkingStarted {
				sendChunk(text, 1) // 开始
				thinkingStarted = true
				eventThinkingOpen = true
			} else {
				sendChunk(text, 2) // 中间
			}
			return
		}

		if eventThinkingOpen {
			sendChunk("", 3)
			eventThinkingOpen = false
			thinkingStarted = false
		}

		textBuffer += text

		for {
			if !inThinkingBlock {
				// 查找 <thinking> 开始标签
				thinkingStart := strings.Index(textBuffer, "<thinking>")
				if thinkingStart != -1 {
					// 输出 thinking 标签之前的内容
					if thinkingStart > 0 {
						sendChunk(textBuffer[:thinkingStart], 0)
					}
					textBuffer = textBuffer[thinkingStart+10:] // 移除 <thinking>
					inThinkingBlock = true
					dropTagThinking = !allowTagSource(&thinkingSource)
					thinkingStarted = false // 重置，准备发送新的开始标签
				} else if forceFlush || len([]rune(textBuffer)) > 50 {
					// 没有找到标签，安全输出（保留可能的部分标签）
					runes := []rune(textBuffer)
					safeLen := len(runes)
					if !forceFlush {
						safeLen = max(0, len(runes)-15)
					}
					if safeLen > 0 {
						sendChunk(string(runes[:safeLen]), 0)
						textBuffer = string(runes[safeLen:])
					}
					break
				} else {
					break
				}
			} else {
				// 在 thinking 块内，查找 </thinking> 结束标签
				thinkingEnd := strings.Index(textBuffer, "</thinking>")
				if thinkingEnd != -1 {
					// 输出 thinking 内容
					content := textBuffer[:thinkingEnd]
					if !dropTagThinking {
						if !thinkingStarted {
							// 一次性输出完整内容（开始+内容+结束）
							sendChunk(content, 1) // 开始
							sendChunk("", 3)      // 结束（空内容，只发结束标签）
						} else {
							// 已经开始了，发送剩余内容和结束
							sendChunk(content, 3) // 结束
						}
					}
					textBuffer = textBuffer[thinkingEnd+11:] // 移除 </thinking>
					inThinkingBlock = false
					dropTagThinking = false
					thinkingStarted = false
				} else if forceFlush {
					// 强制刷新：输出剩余内容
					if textBuffer != "" {
						if !dropTagThinking {
							if !thinkingStarted {
								sendChunk(textBuffer, 1) // 开始
								sendChunk("", 3)         // 结束
							} else {
								sendChunk(textBuffer, 3) // 结束
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
						safeLen := len(runes) - 15 // 保留可能的 </thinking> 部分
						if safeLen > 0 {
							if !dropTagThinking {
								if !thinkingStarted {
									sendChunk(string(runes[:safeLen]), 1) // 开始
									thinkingStarted = true
								} else {
									sendChunk(string(runes[:safeLen]), 2) // 中间
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

	callback := &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			if text == "" {
				return
			}
			if isThinking {
				rawReasoningBuilder.WriteString(text)
			} else {
				rawContentBuilder.WriteString(text)
			}
			processText(text, isThinking, false)
		},
		OnToolUse: func(tu KiroToolUse) {
			tu.Name = RestoreToolName(tu.Name)
			// 先刷新缓冲区
			processText("", false, true)

			args, _ := json.Marshal(tu.Input)
			rawContentBuilder.WriteString(tu.Name)
			rawContentBuilder.Write(args)
			tc := ToolCall{ID: tu.ToolUseID, Type: "function"}
			tc.Function.Name = tu.Name
			tc.Function.Arguments = string(args)
			toolCalls = append(toolCalls, tc)

			chunk := map[string]interface{}{
				"id":      chatID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   originalModel,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{{
							"index": toolCallIndex,
							"id":    tu.ToolUseID,
							"type":  "function",
							"function": map[string]string{
								"name":      tu.Name,
								"arguments": string(args),
							},
						}},
					},
					"finish_reason": nil,
				}},
			}
			toolCallIndex++
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
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
		isQuotaErr := strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota")
		h.pool.RecordError(account.ID, isQuotaErr)
		if isQuotaErr && !headersSent {
			fmt.Printf("[429-Retry] OpenAI Stream | %s → %s | account: %s | payload: %dKB | will retry\n",
				originalModel, model, account.Email, payloadKB)
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		// INVALID_MODEL_ID + 还未发流：token 提前失效，强制刷新后让外层重试
		if upstreamErr != nil && upstreamErr.StatusCode == 400 && !headersSent && strings.Contains(upstreamErr.Body, "INVALID_MODEL_ID") {
			fmt.Printf("[InvalidModel-Refresh] OpenAI Stream account %s got INVALID_MODEL_ID, force refreshing token\n", account.Email)
			if refreshErr := h.forceRefreshToken(account); refreshErr != nil {
				fmt.Printf("[InvalidModel-Refresh] Force refresh failed: %v\n", refreshErr)
			}
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		if payloadKB > 0 {
			fmt.Printf("[ERROR] OpenAI Stream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] OpenAI Stream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		var appErr *AppError
		if upstreamErr != nil {
			appErr = upstreamErr.ToAppError(requestID)
			if headersSent {
				WriteOpenAIStreamError(w, appErr)
			} else {
				WriteOpenAIError(w, appErr, upstreamErr.StatusCode)
			}
		} else if headersSent {
			// mid-stream 无结构化错误 → 仍发 SSE error chunk，保证客户端流不被截断成不完整状态
			WriteOpenAIStreamError(w, NewAppError(ErrorTypeUpstreamError, err.Error(), false, requestID))
		} else {
			h.sendOpenAIError(w, 500, "server_error", err.Error())
		}
		var partial *ChannelResult
		if headersSent {
			partial = &ChannelResult{
				ActualModel:     model,
				Account:         account.Email,
				Subscription:    account.SubscriptionType,
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				UpstreamCredits: credits,
				BillingModel:    billingModel,
				RequestID:       requestID,
				DurationMs:      time.Since(requestStart).Milliseconds(),
				PayloadKB:       payloadKB,
			}
		}
		return partial, &KiroExecError{Err: err, Retryable: false, ResponseStarted: headersSent, PayloadKB: payloadKB, UpstreamAppError: appErr}
	}

	// 刷新剩余缓冲区
	processText("", false, true)
	if eventThinkingOpen {
		sendChunk("", 3)
		eventThinkingOpen = false
	}

	inputTokens = estimatedInputTokens
	outputContent, extractedReasoning := extractThinkingFromContent(rawContentBuilder.String())
	reasoningOutput := rawReasoningBuilder.String()
	if thinking && reasoningOutput == "" && extractedReasoning != "" {
		reasoningOutput = extractedReasoning
	}
	if !thinking {
		reasoningOutput = ""
	}
	outputTokens = estimateApproxTokens(outputContent) + estimateApproxTokens(reasoningOutput)
	for _, tc := range toolCalls {
		outputTokens += estimateApproxTokens(tc.Function.Name)
		outputTokens += estimateApproxTokens(tc.Function.Arguments)
	}

	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	// 发送结束
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	durationMs := time.Since(requestStart).Milliseconds()
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, finishReason, credits, durationMs)

	chunk := map[string]interface{}{
		"id":      chatID,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   originalModel,
		"choices": []map[string]interface{}{{
			"index":         0,
			"delta":         map[string]interface{}{},
			"finish_reason": finishReason,
		}},
		"usage": map[string]int{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		},
	}
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	return &ChannelResult{
		ActualModel:     model,
		Account:         account.Email,
		Subscription:    account.SubscriptionType,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		UpstreamCredits: credits,
		BillingModel:    billingModel,
		StopReason:      finishReason,
		RequestID:       requestID,
		DurationMs:      durationMs,
		PayloadKB:       payloadKB,
	}, nil
}
