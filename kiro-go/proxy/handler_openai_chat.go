package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
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

	uc := getUserContext(r.Context())
	var keyID string
	if uc != nil {
		keyID = uc.KeyID
	}

	// Abuse prevention: check rate/concurrency limits（只信 RemoteAddr，不读 XFF）
	if keyID != "" {
		ip := requestIP(r)
		allowed, reason := OnRequestStart(keyID, ip)
		if !allowed {
			kind, retryAfter := ParseAbuseReason(reason)
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			}
			msg := "Request blocked: " + kind
			if kind == "rate_limit" && retryAfter > 0 {
				msg = fmt.Sprintf("rate limited, retry after %ds", retryAfter)
			}
			h.sendOpenAIError(w, 429, "rate_limit_error", msg)
			return
		}
		defer OnRequestEnd(keyID)
	}

	// Model pool routing: determined by model, not by user tier
	pool := ResolveModelPool(req.Model)
	if keyID != "" {
		info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
		if info != nil {
			_, err := config.ValidateKeyAccessForModel(info, pool)
			if err != nil {
				h.sendOpenAIError(w, 403, "forbidden", err.Error())
				return
			}
		}
	}

	// v3 渠道分发：channels 非空时所有渠道（含 Kiro）统一走 token 计费。
	if router := h.currentChannelRouter(); router != nil && router.HasChannels() {
		hint := &ResolveHint{
			Protocol:  ProtocolOpenAI,
			ChannelID: strings.TrimSpace(r.Header.Get("X-Pivotstack-Channel")),
		}
		if keyID != "" {
			if info := config.FindApiKeyByID(keyID); info != nil {
				if len(info.SeriesPreferences) > 0 {
					hint.UserPreferences = make(map[string]string, len(info.SeriesPreferences))
					for k, v := range info.SeriesPreferences {
						hint.UserPreferences[k] = v
					}
				}
				if len(info.ChannelPreferences) > 0 {
					hint.ChannelPreferences = make(map[string]string, len(info.ChannelPreferences))
					for k, v := range info.ChannelPreferences {
						hint.ChannelPreferences[k] = v
					}
				}
			}
		}
		rr, found := router.ResolveDetailed(req.Model, hint)
		if !found {
			if hint.ChannelID != "" {
				h.sendOpenAIError(w, 400, "invalid_request_error",
					fmt.Sprintf("channel %q is not available for model %q", hint.ChannelID, req.Model))
				return
			}
			h.sendOpenAIError(w, 404, "model_not_found",
				fmt.Sprintf("model %q is not available in any configured channel", req.Model))
			return
		}
		ch := rr.Channel
		maxOutput := 4096
		if req.MaxTokens > 0 {
			maxOutput = req.MaxTokens
		}
		h.handleChannelRequest(w, r, ch, &channelDispatch{
			Protocol:       ProtocolOpenAI,
			OriginalModel:  req.Model,
			Stream:         req.Stream,
			EstimatedInput: estimateOpenAIRequestInputTokens(&req),
			MaxOutput:      maxOutput,
			RawBody:        body,
		}, uc)
		return
	}

	// === Legacy 路径（channels=[] 兜底，按 credit 计费）===
	originalModel := req.Model

	var preChargedPaid, preChargedGift float64
	if keyID != "" {
		estimatedInput := estimateOpenAIRequestInputTokens(&req)
		maxTokens := 4096
		if req.MaxTokens > 0 {
			maxTokens = req.MaxTokens
		}
		var preErr error
		preChargedPaid, preChargedGift, preErr = PreAuthorize(keyID, originalModel, maxTokens, estimatedInput)
		if preErr != nil {
			h.sendOpenAIError(w, 402, "insufficient_balance", preErr.Error())
			return
		}
	}

	requestID := genRequestID()
	result, execErr := h.executeKiroChat(r.Context(), w, &req, body, uc, requestID)
	if execErr != nil {
		responseStarted := false
		payloadKB := 0
		var ke *KiroExecError
		if errors.As(execErr, &ke) {
			responseStarted = ke.ResponseStarted
			payloadKB = ke.PayloadKB
		}
		if !responseStarted {
			RefundPreAuth(keyID, preChargedPaid, preChargedGift)
		}
		h.recordFailure()
		h.addCallLogErrorWithKey("OpenAI", originalModel, "", "", req.Stream, execErr.Error(), payloadKB, uc)
		return
	}
	if result == nil {
		RefundPreAuth(keyID, preChargedPaid, preChargedGift)
		h.recordFailure()
		h.addCallLogErrorWithKey("OpenAI", originalModel, "", "", req.Stream, "executeKiroChat returned nil result", 0, uc)
		return
	}

	billingCredits := result.UpstreamCredits
	if billingCredits <= 0 {
		billingCredits = EstimateCredits(result.OutputTokens, result.InputTokens)
	}
	billingCredits = ApplyLowOutputProtection(result.OutputTokens, billingCredits, result.InputTokens)
	if result.BillingModel != "" && result.ActualModel != "" && result.BillingModel != result.ActualModel {
		billingCredits *= StealthCreditMultiplier(result.BillingModel, result.ActualModel)
	}
	paid, gift := ReconcileWithBillingModel(keyID, originalModel, result.BillingModel, billingCredits, preChargedPaid, preChargedGift)
	if uc != nil {
		uc.ActualPaidUSD = paid
		uc.ActualGiftUSD = gift
	}
	h.recordSuccess(result.InputTokens, result.OutputTokens, billingCredits)
	h.addCallLogWithKey("OpenAI", originalModel, result.ActualModel, result.Account, result.Subscription, result.InputTokens, result.OutputTokens, req.Stream, billingCredits, result.UpstreamCredits, "", "", result.StopReason, result.RequestID, result.DurationMs, uc)
}
