package proxy

import (
	"context"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
)

// executeKiroChat 是 KiroExecutor.executeKiroChat 的真实实现。
// 包含 stealth + thinking 解析 + 3 次账号 retry。调用 handleOpenAIStream/NonStream
// 完成单次上游请求 + 响应翻译。**不**做计费 — 由 caller（handleChannelRequest 或 legacy handleOpenAIChat）负责。
func (h *Handler) executeKiroChat(ctx context.Context, w http.ResponseWriter,
	req *OpenAIRequest, body []byte, uc *UserContext, requestID string,
) (*ChannelResult, error) {
	if requestID == "" {
		requestID = genRequestID()
	}
	thinkingCfg := config.GetThinkingConfig()
	originalModel := req.Model
	billingModel, thinking := ParseModelAndThinking(originalModel, thinkingCfg.Suffix)
	upstreamModel, stealthSwapped := ApplyStealth(billingModel, originalModel)
	if stealthSwapped {
		upstreamModel, thinking = ParseModelAndThinking(upstreamModel, thinkingCfg.Suffix)
	}
	tier := ResolveModelPool(upstreamModel)
	req.Model = upstreamModel
	estimatedInputTokens := estimateOpenAIRequestInputTokens(req)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		account := h.pool.GetNextByTier(tier)
		if account == nil {
			return nil, fmt.Errorf("no available accounts in %s pool: %w", tier, lastErr)
		}
		if err := h.ensureValidToken(account); err != nil {
			h.pool.RecordError(account.ID, false)
			h.pool.ReleaseAccount(account.ID)
			lastErr = err
			continue
		}
		if _, err := ValidateAndMapModel(upstreamModel, account.SubscriptionType); err != nil {
			h.pool.ReleaseAccount(account.ID)
			lastErr = err
			continue
		}

		kiroPayload := OpenAIToKiro(req, thinking)
		var result *ChannelResult
		var execErr *KiroExecError
		if req.Stream {
			result, execErr = h.handleOpenAIStream(w, account, kiroPayload, upstreamModel, originalModel, billingModel, stealthSwapped, thinking, estimatedInputTokens, requestID)
		} else {
			result, execErr = h.handleOpenAINonStream(w, account, kiroPayload, upstreamModel, originalModel, billingModel, stealthSwapped, thinking, estimatedInputTokens, requestID)
		}

		if execErr != nil && execErr.Retryable && !execErr.ResponseStarted {
			h.pool.ReleaseAccount(account.ID)
			lastErr = execErr.Err
			continue
		}
		// 成功 / 终态错误 / mid-stream 错误 → 释放账号后返给 caller
		if execErr != nil {
			h.pool.ReleaseAccount(account.ID)
			return result, execErr
		}
		if result != nil {
			result.BillingModel = billingModel
		}
		h.pool.ReleaseAccount(account.ID)
		return result, nil
	}
	return nil, fmt.Errorf("retry exhausted: %w", lastErr)
}

// executeKiroClaude Claude 协议版本，结构同 executeKiroChat。
func (h *Handler) executeKiroClaude(ctx context.Context, w http.ResponseWriter,
	req *ClaudeRequest, body []byte, uc *UserContext, requestID string,
) (*ChannelResult, error) {
	if requestID == "" {
		requestID = genRequestID()
	}
	thinkingCfg := config.GetThinkingConfig()
	originalModel := req.Model
	billingModel, thinking := ParseModelAndThinking(originalModel, thinkingCfg.Suffix)
	upstreamModel, stealthSwapped := ApplyStealth(billingModel, originalModel)
	if stealthSwapped {
		upstreamModel, thinking = ParseModelAndThinking(upstreamModel, thinkingCfg.Suffix)
	}
	tier := ResolveModelPool(upstreamModel)
	req.Model = upstreamModel
	estimatedInputTokens := estimateClaudeRequestInputTokens(req)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		account := h.pool.GetNextByTier(tier)
		if account == nil {
			return nil, fmt.Errorf("no available accounts in %s pool: %w", tier, lastErr)
		}
		if err := h.ensureValidToken(account); err != nil {
			h.pool.RecordError(account.ID, false)
			h.pool.ReleaseAccount(account.ID)
			lastErr = err
			continue
		}
		if _, err := ValidateAndMapModel(upstreamModel, account.SubscriptionType); err != nil {
			h.pool.ReleaseAccount(account.ID)
			lastErr = err
			continue
		}

		kiroPayload := ClaudeToKiro(req, thinking)
		var result *ChannelResult
		var execErr *KiroExecError
		if req.Stream {
			result, execErr = h.handleClaudeStream(w, account, kiroPayload, upstreamModel, originalModel, billingModel, stealthSwapped, thinking, estimatedInputTokens, requestID)
		} else {
			result, execErr = h.handleClaudeNonStream(w, account, kiroPayload, upstreamModel, originalModel, billingModel, stealthSwapped, thinking, estimatedInputTokens, requestID)
		}

		if execErr != nil && execErr.Retryable && !execErr.ResponseStarted {
			h.pool.ReleaseAccount(account.ID)
			lastErr = execErr.Err
			continue
		}
		if execErr != nil {
			h.pool.ReleaseAccount(account.ID)
			return result, execErr
		}
		if result != nil {
			result.BillingModel = billingModel
		}
		h.pool.ReleaseAccount(account.ID)
		return result, nil
	}
	return nil, fmt.Errorf("retry exhausted: %w", lastErr)
}
