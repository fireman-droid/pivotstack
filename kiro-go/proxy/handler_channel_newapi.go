package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleNewAPIChannelRequest v5 newapi 渠道的同步请求路径。
// 与 handleChannelRequest 的 token 模式镜像，但用 NewAPIReservation 做单位 snapshot。
// Phase 4a 同步扣费，Phase 4b 会异步对账并覆盖 call_log.billing_status。
func (h *Handler) handleNewAPIChannelRequest(
	w http.ResponseWriter,
	r *http.Request,
	ch *NewAPIRuntimeChannel,
	d *channelDispatch,
	uc *UserContext,
) {
	start := time.Now()
	requestID := genRequestID()
	billingMode := "newapi"

	if d == nil {
		h.sendProtocolError(w, ProtocolOpenAI, 400, "invalid_request_error", "missing channel dispatch")
		return
	}
	if ch == nil {
		h.sendProtocolError(w, d.Protocol, 400, "invalid_request_error", "missing newapi channel")
		return
	}
	if !ch.SupportsProtocol(d.Protocol) {
		h.sendProtocolError(w, d.Protocol, 400, "unsupported_channel",
			fmt.Sprintf("channel %q does not support %s protocol", ch.ID(), d.Protocol))
		return
	}

	manager := h.newapiManager
	if manager == nil {
		manager = h.ensureNewAPIManager()
	}
	cache, ok := manager.Cache(ch.ProviderID())
	if !ok {
		// 不要把 admin 侧的 "provider 未同步" 状态泄露给用户。用 503 + 中立文案，
		// SDK 会按标准 channel unavailable 处理；Phase 5 前端会把这种 503 显示为 "渠道暂不可用"。
		h.sendProtocolError(w, d.Protocol, 503, "channel_unavailable", "channel temporarily unavailable")
		return
	}

	var keyID string
	if uc != nil {
		keyID = uc.KeyID
	}

	res, preErr := PreAuthorizeNewAPIRequest(keyID, ch, cache, d.OriginalModel, d.EstimatedInput, d.MaxOutput)
	if preErr != nil {
		code := 402
		kind := "insufficient_balance"
		if errors.Is(preErr, ErrSellPriceMissing) {
			code = 400
			kind = "sell_price_missing"
		} else if strings.Contains(preErr.Error(), "disabled") || strings.Contains(preErr.Error(), "no active") {
			code = 403
			kind = "forbidden"
		}
		h.sendProtocolError(w, d.Protocol, code, kind, preErr.Error())
		return
	}

	chReq := ChannelRequest{
		Protocol:      d.Protocol,
		OriginalModel: d.OriginalModel,
		Stream:        d.Stream,
		RawBody:       d.RawBody,
		RequestID:     requestID,
		UserContext:   uc,
	}

	cw := &channelResponseWriter{ResponseWriter: w}
	result, execErr := ch.Execute(r.Context(), cw, chReq)
	if execErr != nil {
		h.recordFailure()
		responseWritten := cw.WroteHeader()
		refundAllowed := !responseWritten
		var ke *KiroExecError
		if errors.As(execErr, &ke) {
			refundAllowed = !ke.ResponseStarted
		}
		var up *UpstreamHTTPError
		if errors.As(execErr, &up) && !up.Chargeable {
			refundAllowed = true
		}
		if refundAllowed {
			RefundNewAPIReservation(res)
		}
		h.writeChannelErrorLog(d.Protocol, d.OriginalModel, ch, d.Stream, execErr.Error(), uc, billingMode, requestID, time.Since(start).Milliseconds())
		if !responseWritten {
			if up != nil {
				copySafeHeaders(w.Header(), up.Header)
				if w.Header().Get("Content-Type") == "" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(up.StatusCode)
				_, _ = w.Write(up.Body)
				return
			}
			h.sendProtocolError(w, d.Protocol, 502, "upstream_error", execErr.Error())
		}
		return
	}
	if result == nil {
		h.recordFailure()
		if !cw.WroteHeader() {
			RefundNewAPIReservation(res)
		}
		h.writeChannelErrorLog(d.Protocol, d.OriginalModel, ch, d.Stream, "channel returned nil result", uc, billingMode, requestID, time.Since(start).Milliseconds())
		if !cw.WroteHeader() {
			h.sendProtocolError(w, d.Protocol, 502, "upstream_error", "channel returned nil result")
		}
		return
	}

	actualUsage := TokenUsage{
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
	}
	if actualUsage.InputTokens+actualUsage.OutputTokens == 0 && result.UsageEstimated {
		actualUsage = TokenUsage{InputTokens: d.EstimatedInput, OutputTokens: d.MaxOutput}
		result.InputTokens = actualUsage.InputTokens
		result.OutputTokens = actualUsage.OutputTokens
	}

	// Phase 4a: 把 reservation 的 EstQuota 落到 call_log.UpstreamCredits 上，避免 UI 「上游消耗」列空白。
	// Phase 4b 异步对账时会用真实 /api/log/self 拉到的 quota 覆盖。
	if result.UpstreamCredits == 0 && res != nil && res.EstQuota > 0 {
		result.UpstreamCredits = float64(res.EstQuota)
	}

	paidUSD, giftUSD, recErr := ReconcileNewAPIRequest(res, actualUsage)
	if recErr != nil {
		fmt.Printf("[Billing-NewAPI] reconcile error: %v\n", recErr)
	}
	if uc != nil {
		uc.ActualPaidUSD = paidUSD
		uc.ActualGiftUSD = giftUSD
	}

	if result.InputTokens+result.OutputTokens > 0 {
		h.recordSuccess(result.InputTokens, result.OutputTokens, result.UpstreamCredits)
	}
	h.writeChannelSuccessLog(d.Protocol, ch, d.OriginalModel, result, d.Stream,
		time.Since(start).Milliseconds(), requestID, uc, billingMode, newAPIBillingStatus(res), paidUSD, giftUSD)

	// Phase 4b 异步对账钩子：把已经过同步 reconcile 的 reservation 推入 worker queue。
	// 注意覆盖 res.PrePaid/PreGift 为 Phase 4a 同步 reconcile 后的累计扣款，
	// 这样 Phase 4b 的 diff = upstream真实cost - bookedSoFar 才正确。
	if res != nil && res.Action == "estimated" && h.newapiReconciler != nil {
		res.PromptTokens = actualUsage.InputTokens
		res.MaxOutputTokens = actualUsage.OutputTokens
		res.PrePaidUSD = paidUSD
		res.PreGiftUSD = giftUSD
		h.newapiReconciler.Enqueue(res.ProviderID, &pendingReservation{
			Reservation: res, RequestID: requestID, KeyID: res.KeyID, EnqueuedAt: time.Now(),
		})
	}
}
