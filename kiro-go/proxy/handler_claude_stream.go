package proxy
import ("encoding/json"; "fmt"; "kiro-api-proxy/config"; "net/http"; "os"; "strings"; "time"; "github.com/google/uuid")
func (h *Handler) handleClaudeStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, upstreamModel, originalModel, billingModel string, stealthSwapped bool, thinking bool, estimatedInputTokens int, requestID string) (*ChannelResult, *KiroExecError) {
	requestStart := time.Now()
	if requestID == "" {
		requestID = genRequestID()
	}
	model := upstreamModel
	payloadKB := 0
	if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
		payloadKB = len(payloadBytes) / 1024
	}
	fmt.Printf("[req-%s] → Claude Stream | %s → %s | account: %s | input≈%dK | thinking=%v\n",
		requestID, originalModel, model, account.Email, estimatedInputTokens/1000, thinking)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("streaming not supported")
		h.sendClaudeError(w, 500, "api_error", "Streaming not supported")
		return nil, &KiroExecError{Err: err, Retryable: false, ResponseStarted: false, PayloadKB: payloadKB}
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
	headersSent := false // 追踪是否已发送SSE数据，用于判断429时能否换号重试

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

	// message_start 延迟发送，429时可以换号重试
	sendMessageStart := func() {
		if headersSent {
			return
		}
		h.sendSSE(w, flusher, "message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":            msgID,
				"type":          "message",
				"role":          "assistant",
				"content":       []interface{}{},
				"model":         originalModel,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]int{
					"input_tokens":  startInputTokens,
					"output_tokens": 0,
				},
			},
		})
		headersSent = true
	}

	debugLog := func(format string, args ...interface{}) {
		if os.Getenv("DEBUG_REQUESTS") != "true" {
			return
		}
		f, _ := os.OpenFile("data/debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if f != nil {
			fmt.Fprintf(f, format+"\n", args...)
			f.Close()
		}
	}

	callback := &KiroStreamCallback{
		OnText: func(text string, isThinking bool) {
			if text == "" {
				return
			}
			debugLog("[SSE OnText] thinking=%v len=%d text=%.200s", isThinking, len(text), text)
			sendMessageStart()
			if isThinking {
				rawThinkingBuilder.WriteString(text)
			} else {
				rawContentBuilder.WriteString(text)
			}
			processClaudeText(text, isThinking, false)
		},
		OnToolUse: func(tu KiroToolUse) {
			debugLog("[SSE OnToolUse] name=%s id=%s", tu.Name, tu.ToolUseID)
			sendMessageStart()
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
			debugLog("[SSE OnComplete] input=%d output=%d", inTok, outTok)
			inputTokens = inTok
			outputTokens = outTok
		},
		OnError: func(err error) {
			debugLog("[SSE OnError] %v", err)
			h.pool.RecordError(account.ID, strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota"))
		},
		OnCredits: func(c float64) {
			credits = c
		},
	}

	upstreamErr, err := CallKiroAPI(account, payload, callback)
	if err != nil {
		debugLog("[SSE Error] CallKiroAPI failed: %v", err)
		isQuotaErr := strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota")
		h.pool.RecordError(account.ID, isQuotaErr)
		if isQuotaErr && !headersSent {
			fmt.Printf("[429-Retry] Claude Stream | %s → %s | account: %s | payload: %dKB | will retry\n",
				originalModel, model, account.Email, payloadKB)
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		if upstreamErr != nil && upstreamErr.StatusCode == 400 && !headersSent && strings.Contains(upstreamErr.Body, "INVALID_MODEL_ID") {
			fmt.Printf("[InvalidModel-Refresh] Stream account %s got INVALID_MODEL_ID, force refreshing token\n", account.Email)
			if refreshErr := h.forceRefreshToken(account); refreshErr != nil {
				fmt.Printf("[InvalidModel-Refresh] Force refresh failed: %v\n", refreshErr)
			}
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		if payloadKB > 0 {
			fmt.Printf("[ERROR] Claude Stream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] Claude Stream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		var appErr *AppError
		if upstreamErr != nil {
			appErr = upstreamErr.ToAppError(requestID)
			if headersSent {
				WriteClaudeStreamError(w, appErr)
			} else {
				WriteClaudeError(w, appErr, upstreamErr.StatusCode)
			}
		} else if headersSent {
			h.sendSSE(w, flusher, "error", map[string]interface{}{
				"type":  "error",
				"error": map[string]string{"type": "api_error", "message": err.Error()},
			})
		} else {
			h.sendClaudeError(w, 500, "api_error", err.Error())
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

	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	stopReason := "end_turn"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}

	durationMs := time.Since(requestStart).Milliseconds()
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
	return &ChannelResult{
		ActualModel:     model,
		Account:         account.Email,
		Subscription:    account.SubscriptionType,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		UpstreamCredits: credits,
		BillingModel:    billingModel,
		StopReason:      stopReason,
		RequestID:       requestID,
		DurationMs:      durationMs,
		PayloadKB:       payloadKB,
	}, nil
}
