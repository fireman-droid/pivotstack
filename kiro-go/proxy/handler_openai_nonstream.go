package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"strings"
	"time"
)

// handleOpenAINonStream OpenAI 非流式响应。返回 ChannelResult（成功）或 KiroExecError（错误）。
// 不做计费、不写日志、不更新 success/failure stats — 由 caller 负责。
func (h *Handler) handleOpenAINonStream(w http.ResponseWriter, account *config.Account, payload *KiroPayload, upstreamModel, originalModel, billingModel string, stealthSwapped bool, thinking bool, estimatedInputTokens int, requestID string) (*ChannelResult, *KiroExecError) {
	requestStart := time.Now()
	if requestID == "" {
		requestID = genRequestID()
	}
	model := upstreamModel
	payloadKB := 0
	if payloadBytes, jsonErr := json.Marshal(payload); jsonErr == nil {
		payloadKB = len(payloadBytes) / 1024
	}
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
		isQuotaErr := strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota")
		h.pool.RecordError(account.ID, isQuotaErr)
		if isQuotaErr {
			fmt.Printf("[429-Retry] OpenAI NonStream | %s → %s | account: %s | payload: %dKB | will retry\n",
				originalModel, model, account.Email, payloadKB)
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		if upstreamErr != nil && upstreamErr.StatusCode == 400 && strings.Contains(upstreamErr.Body, "INVALID_MODEL_ID") {
			fmt.Printf("[InvalidModel-Refresh] OpenAI NonStream account %s got INVALID_MODEL_ID, force refreshing token\n", account.Email)
			if refreshErr := h.forceRefreshToken(account); refreshErr != nil {
				fmt.Printf("[InvalidModel-Refresh] Force refresh failed: %v\n", refreshErr)
			}
			return nil, &KiroExecError{Err: err, Retryable: true, ResponseStarted: false, PayloadKB: payloadKB}
		}
		if payloadKB > 0 {
			fmt.Printf("[ERROR] OpenAI NonStream | %s → %s | account: %s | payload: %dKB | error: %s\n",
				originalModel, model, account.Email, payloadKB, err.Error())
		} else {
			fmt.Printf("[ERROR] OpenAI NonStream | %s → %s | account: %s | error: %s\n",
				originalModel, model, account.Email, err.Error())
		}
		var appErr *AppError
		if upstreamErr != nil {
			appErr = upstreamErr.ToAppError(requestID)
			WriteOpenAIError(w, appErr, upstreamErr.StatusCode)
		} else {
			h.sendOpenAIError(w, 500, "server_error", err.Error())
		}
		return nil, &KiroExecError{Err: err, Retryable: false, ResponseStarted: false, PayloadKB: payloadKB, UpstreamAppError: appErr}
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

	h.pool.RecordSuccess(account.ID)
	h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)

	stopReason := "stop"
	if len(toolUses) > 0 {
		stopReason = "tool_use"
	}
	durationMs := time.Since(requestStart).Milliseconds()
	fmt.Printf("[req-%s] ← Complete | out=%d | stop=%s | credits=%.2f | %dms\n",
		requestID, outputTokens, stopReason, credits, durationMs)

	thinkingFormat := config.GetThinkingConfig().OpenAIFormat
	resp := KiroToOpenAIResponseWithReasoning(finalContent, reasoningContent, toolUses, inputTokens, outputTokens, originalModel, thinkingFormat)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
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
