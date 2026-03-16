package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleOpenAIChat OpenAI API 处理
func (h *Handler) handleOpenAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// 限制请求体大小为 100MB
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendOpenAIError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	var req OpenAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendOpenAIError(w, 400, "invalid_request_error", "Invalid JSON")
		return
	}

	// 请求内故障转移：最多尝试 3 个账号
	maxRetries := 3
	var lastErr error

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
			h.sendOpenAIError(w, 429, "rate_limit_error", "Request blocked: "+reason)
			return
		}
		defer OnRequestEnd(keyID)
	}

	// Model pool routing: determined by model, not by user tier
	pool := ResolveModelPool(req.Model)
	if keyID != "" {
		info := config.FindApiKeyByID(keyID)
		if info != nil {
			_, err := config.ValidateKeyAccessForModel(info, pool)
			if err != nil {
				h.sendOpenAIError(w, 403, "forbidden", err.Error())
				return
			}
		}
	}

	// Billing: PreAuthorize – lock estimated cost before calling upstream
	var preChargedUSD float64
	if keyID != "" {
		estimatedInput := estimateOpenAIRequestInputTokens(&req)
		maxTokens := 4096
		if req.MaxTokens > 0 {
			maxTokens = req.MaxTokens
		}
		var preErr error
		preChargedUSD, preErr = PreAuthorize(keyID, req.Model, maxTokens, estimatedInput)
		if preErr != nil {
			h.sendOpenAIError(w, 402, "insufficient_balance", preErr.Error())
			return
		}
	}

	tier := DeterminePoolTier(req.Model)

	for attempt := 0; attempt < maxRetries; attempt++ {
		account := h.pool.GetNextByTier(tier)
		if account == nil {
			RefundPreAuth(keyID, preChargedUSD)
			h.sendOpenAIError(w, 503, "server_error", fmt.Sprintf("No available accounts in %s pool", tier))
			return
		}

		if err := h.ensureValidToken(account); err != nil {
			h.pool.RecordError(account.ID, false)
			h.pool.ReleaseAccount(account.ID)
			lastErr = fmt.Errorf("token refresh failed: %v", err)
			continue
		}

		// 模型验证：在选择账号后根据订阅类型验证
		thinkingCfg := config.GetThinkingConfig()
		originalModel := req.Model
		mappedModel, validateErr := ValidateAndMapModel(req.Model, account.SubscriptionType)
		if validateErr != nil {
			h.pool.ReleaseAccount(account.ID)
			RefundPreAuth(keyID, preChargedUSD)
			h.sendOpenAIError(w, 400, "invalid_request_error", validateErr.Error())
			return
		}
		req.Model = mappedModel
		actualModel, thinking := ParseModelAndThinking(req.Model, thinkingCfg.Suffix)
		req.Model = actualModel
		estimatedInputTokens := estimateOpenAIRequestInputTokens(&req)

		if originalModel != actualModel {
			fmt.Printf("[Request] OpenAI API | %s → %s | account: %s | stream: %v | attempt: %d\n", originalModel, actualModel, account.Email, req.Stream, attempt+1)
		} else {
			fmt.Printf("[Request] OpenAI API | model: %s | account: %s | stream: %v | attempt: %d\n", actualModel, account.Email, req.Stream, attempt+1)
		}

		kiroPayload := OpenAIToKiro(&req, thinking)

		if req.Stream {
			h.handleOpenAIStream(w, account, kiroPayload, req.Model, originalModel, thinking, estimatedInputTokens, uc, preChargedUSD)
		} else {
			h.handleOpenAINonStream(w, account, kiroPayload, req.Model, originalModel, thinking, estimatedInputTokens, uc, preChargedUSD)
		}
		return
	}

	// 所有重试都失败 – refund pre-auth
	RefundPreAuth(keyID, preChargedUSD)
	h.recordFailure()
	errMsg := "All accounts failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	h.sendOpenAIError(w, 503, "server_error", errMsg)
}

// handleOpenAIStream OpenAI 流式响应
func (h *Handler) handleOpenAIStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, model, originalModel string, thinking bool, estimatedInputTokens int, uc *UserContext, preChargedUSD float64) {
	requestStart := time.Now()
	requestID := genRequestID()
	fmt.Printf("[req-%s] → OpenAI Stream | %s → %s | account: %s | input≈%dK | thinking=%v\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000, thinking)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.sendOpenAIError(w, 500, "server_error", "Streaming not supported")
		return
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
					"model":   model,
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
					"model":   model,
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
					"model":   model,
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
				"model":   model,
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
				"model":   model,
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
		h.recordFailure()
		h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429"))
		payloadKB := 0
		if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
			payloadKB = len(payloadBytes) / 1024
			fmt.Printf("[ERROR] OpenAI Stream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] OpenAI Stream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		h.addCallLogErrorWithKey("OpenAI", originalModel, model, account.Email, true, err.Error(), payloadKB, uc)
		if upstreamErr != nil {
			WriteOpenAIStreamError(w, upstreamErr.ToAppError(""))
		}
		return
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

	h.recordSuccess(inputTokens, outputTokens, credits)
	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	// Billing: Reconcile pre-authorized amount with actual credits
	if uc != nil && uc.KeyID != "" && preChargedUSD > 0 {
		actualCredits := credits
		if actualCredits <= 0 {
			actualCredits = EstimateCredits(outputTokens, inputTokens)
		}
		Reconcile(uc.KeyID, model, actualCredits, preChargedUSD)
	}

	// 发送结束
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	durationMs := time.Since(requestStart).Milliseconds()
	h.addCallLogWithKey("OpenAI", originalModel, model, account.Email, account.SubscriptionType, inputTokens, outputTokens, true, credits, "", "", finishReason, requestID, durationMs, uc)
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, finishReason, credits, durationMs)

	chunk := map[string]interface{}{
		"id":      chatID,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
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
}

// handleOpenAINonStream OpenAI 非流式响应
func (h *Handler) handleOpenAINonStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, model, originalModel string, thinking bool, estimatedInputTokens int, uc *UserContext, preChargedUSD float64) {
	requestStart := time.Now()
	requestID := genRequestID()
	fmt.Printf("[req-%s] → OpenAI NonStream | %s → %s | account: %s | input≈%dK\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000)

	var content string
	var reasoningContent string
	var toolUses []KiroToolUse
	var inputTokens, outputTokens int
	var credits float64

	callback := &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			if isThinking {
				reasoningContent += text
			} else {
				content += text
			}
		},
		OnToolUse:  func(tu KiroToolUse) { tu.Name = RestoreToolName(tu.Name); toolUses = append(toolUses, tu) },
		OnComplete: func(inTok, outTok int) { inputTokens = inTok; outputTokens = outTok },
		OnError:    func(err error) { h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429")) },
		OnCredits:  func(c float64) { credits = c },
	}

	upstreamErr, err := CallKiroAPI(account, payload, callback)
	if err != nil {
		h.recordFailure()
		h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429"))
		// Billing: refund pre-auth on API failure
		if uc != nil {
			RefundPreAuth(uc.KeyID, preChargedUSD)
		}
		payloadKB := 0
		if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
			payloadKB = len(payloadBytes) / 1024
			fmt.Printf("[ERROR] OpenAI NonStream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] OpenAI NonStream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		h.addCallLogErrorWithKey("OpenAI", originalModel, model, account.Email, false, err.Error(), payloadKB, uc)
		if upstreamErr != nil {
			appErr := upstreamErr.ToAppError("")
			WriteOpenAIError(w, appErr, upstreamErr.StatusCode)
		} else {
			h.sendOpenAIError(w, 500, "server_error", err.Error())
		}
		return
	}

	// 解析 content 中的 <thinking> 标签
	finalContent, extractedReasoning := extractThinkingFromContent(content)
	if thinking && reasoningContent == "" && extractedReasoning != "" {
		reasoningContent = extractedReasoning
	} else if !thinking {
		reasoningContent = ""
	}

	inputTokens = estimatedInputTokens
	outputTokens = estimateOpenAIOutputTokens(finalContent, reasoningContent, toolUses)

	h.recordSuccess(inputTokens, outputTokens, credits)
	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	// Billing: Reconcile pre-authorized amount with actual credits
	if uc != nil && uc.KeyID != "" && preChargedUSD > 0 {
		actualCredits := credits
		if actualCredits <= 0 {
			actualCredits = EstimateCredits(outputTokens, inputTokens)
		}
		Reconcile(uc.KeyID, model, actualCredits, preChargedUSD)
	}

	stopReason := "stop"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}
	durationMs := time.Since(requestStart).Milliseconds()
	h.addCallLogWithKey("OpenAI", originalModel, model, account.Email, account.SubscriptionType, inputTokens, outputTokens, false, credits, "", "", stopReason, requestID, durationMs, uc)
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, stopReason, credits, durationMs)

	thinkingFormat := config.GetThinkingConfig().OpenAIFormat
	resp := KiroToOpenAIResponseWithReasoning(finalContent, reasoningContent, toolUses, inputTokens, outputTokens, model, thinkingFormat)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) sendOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	})
}
