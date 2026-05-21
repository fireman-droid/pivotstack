package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// channelDispatch 把协议无关的请求元信息打包传给 handleChannelRequest，
// 避免 handleChannelRequest 直接依赖 OpenAI/Claude 具体请求类型。
type channelDispatch struct {
	Protocol       Protocol
	OriginalModel  string
	Stream         bool
	EstimatedInput int
	MaxOutput      int
	RawBody        []byte
}

// handleChannelRequest 是渠道层的统一请求路径（Kiro 与外部渠道共用）。
// 流程：
//  1. 协议兼容性 + PreAuthorizeTokens（按 sellPrices 预扣 virtual$）
//  2. ch.Execute() → 渠道把响应写给客户端，返回 ChannelResult
//  3. 失败 → RefundTokenReservation；headers 没发出去时还能写错误响应
//  4. 成功 → ReconcileTokenUsage（按实际 token 数补/退）
//  5. addCallLog（带 channel_id + channel_type + billing_mode=token + billing_status）
func (h *Handler) handleChannelRequest(
	w http.ResponseWriter,
	r *http.Request,
	ch Channel,
	d *channelDispatch,
	uc *UserContext,
) {
	if nc, ok := ch.(*NewAPIRuntimeChannel); ok {
		// v5: newapi 渠道用独立单位链（上游 quota → ¥ → 虚拟$ × markup），
		// 跟 token 计费的 TokenReservation 单位语义不兼容，必须走自己的 reservation 路径。
		h.handleNewAPIChannelRequest(w, r, nc, d, uc)
		return
	}

	start := time.Now()
	requestID := genRequestID()
	billingMode := "token"

	if d == nil {
		h.sendProtocolError(w, ProtocolOpenAI, 400, "invalid_request_error", "missing channel dispatch")
		return
	}
	if !ch.SupportsProtocol(d.Protocol) {
		h.sendProtocolError(w, d.Protocol, 400, "unsupported_channel",
			fmt.Sprintf("channel %q does not support %s protocol", ch.ID(), d.Protocol))
		return
	}

	var keyID string
	if uc != nil {
		keyID = uc.KeyID
	}

	tokenRes, preErr := PreAuthorizeTokensForChannel(keyID, ch.ID(), d.OriginalModel, TokenUsage{
		InputTokens:  d.EstimatedInput,
		OutputTokens: d.MaxOutput,
	})
	// Panic 守门（billing_audit P0-4）：渠道适配器 panic 后，预扣余额必须退回，
	// 否则上游崩溃 = 用户被白扣。退完再 re-panic，让上层 panic recovery / log 链照常运作。
	defer func() {
		if rec := recover(); rec != nil {
			if tokenRes != nil {
				RefundTokenReservation(tokenRes)
			}
			panic(rec)
		}
	}()
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
		// 区分两个独立信号：
		//   responseWritten = 本地是否已写过响应给 client（true → 不能再写新错误）
		//   refundAllowed   = 是否可以退款（KiroExecError.ResponseStarted=true 表示上游成本已发生，不退）
		responseWritten := cw.WroteHeader()
		refundAllowed := !responseWritten
		var ke *KiroExecError
		if errors.As(execErr, &ke) {
			refundAllowed = !ke.ResponseStarted
		}
		// v4: 上游 HTTP 错误（4xx/5xx）— UpstreamHTTPError.Chargeable=false 默认不计费
		var up *UpstreamHTTPError
		if errors.As(execErr, &up) && !up.Chargeable {
			refundAllowed = true
		}
		if refundAllowed {
			RefundTokenReservation(tokenRes)
		}
		h.writeChannelErrorLog(d.Protocol, d.OriginalModel, ch, d.Stream, execErr.Error(), uc, billingMode, requestID, time.Since(start).Milliseconds())
		if !responseWritten {
			// v4: 上游 HTTP 错误 → 透传原状态码 + body（不再被 502 吞掉）
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
			RefundTokenReservation(tokenRes)
		}
		h.writeChannelErrorLog(d.Protocol, d.OriginalModel, ch, d.Stream, "channel returned nil result", uc, billingMode, requestID, time.Since(start).Milliseconds())
		if !cw.WroteHeader() {
			h.sendProtocolError(w, d.Protocol, 502, "upstream_error", "channel returned nil result")
		}
		return
	}

	// 防漏扣：上游返回 0 token + UsageEstimated 标记时，回退到预估值兜底计费
	// （未做这步的话 ReconcileTokenUsage(0,0) 会全额退款 = 客户白嫖）
	actualUsage := TokenUsage{
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
	}
	if actualUsage.InputTokens+actualUsage.OutputTokens == 0 && result.UsageEstimated {
		actualUsage = TokenUsage{InputTokens: d.EstimatedInput, OutputTokens: d.MaxOutput}
		result.InputTokens = actualUsage.InputTokens
		result.OutputTokens = actualUsage.OutputTokens
	}

	paidUSD, giftUSD, recErr := ReconcileTokenUsage(tokenRes, actualUsage)
	if recErr != nil {
		fmt.Printf("[Billing-Token] reconcile error: %v\n", recErr)
	}
	if uc != nil {
		uc.ActualPaidUSD = paidUSD
		uc.ActualGiftUSD = giftUSD
	}

	if result.InputTokens+result.OutputTokens > 0 {
		h.recordSuccess(result.InputTokens, result.OutputTokens, result.UpstreamCredits)
	}
	h.writeChannelSuccessLog(d.Protocol, ch, d.OriginalModel, result, d.Stream,
		time.Since(start).Milliseconds(), requestID, uc, billingMode, billingStatus(tokenRes), paidUSD, giftUSD)
}

func (h *Handler) writeChannelSuccessLog(
	protocol Protocol,
	ch Channel,
	originalModel string,
	result *ChannelResult,
	stream bool,
	durationMs int64,
	requestID string,
	uc *UserContext,
	billingMode string,
	billingStat string,
	paidUSD float64,
	giftUSD float64,
) {
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)
	apiType := "OpenAI"
	if protocol == ProtocolClaude {
		apiType = "Claude"
	}

	entry := CallLog{
		Time:            now.In(cst).Format("01-02 15:04:05"),
		Timestamp:       now.Unix(),
		RequestID:       requestID,
		APIType:         apiType,
		OriginalModel:   originalModel,
		ActualModel:     result.ActualModel,
		Account:         strings.TrimSpace(result.Account),
		InputTokens:     result.InputTokens,
		OutputTokens:    result.OutputTokens,
		TotalTokens:     result.InputTokens + result.OutputTokens,
		Credits:         0,
		UpstreamCredits: result.UpstreamCredits,
		PaidCredits:     0,
		GiftedCredits:   0,
		CostUSD:         paidUSD,
		ChargedUSD:      paidUSD + giftUSD,
		PriceModel:      originalModel,
		Stream:          stream,
		Status:          "success",
		StopReason:      result.StopReason,
		DurationMs:      durationMs,
		PayloadKB:       result.PayloadKB,
		ChannelID:       ch.ID(),
		ChannelType:     ch.Type(),
		BillingMode:     billingMode,
		BillingStatus:   billingStat,
		UsageEstimated:  result.UsageEstimated,
	}
	if uc != nil {
		entry.ApiKeyID = uc.KeyID
	}

	h.persistCallLog(entry, uc, result.InputTokens, result.OutputTokens)
}

func (h *Handler) writeChannelErrorLog(
	protocol Protocol,
	originalModel string,
	ch Channel,
	stream bool,
	errMsg string,
	uc *UserContext,
	billingMode string,
	requestID string,
	durationMs int64,
) {
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)
	apiType := "OpenAI"
	if protocol == ProtocolClaude {
		apiType = "Claude"
	}

	entry := CallLog{
		Time:          now.In(cst).Format("01-02 15:04:05"),
		Timestamp:     now.Unix(),
		RequestID:     requestID,
		APIType:       apiType,
		OriginalModel: originalModel,
		Account:       ch.ID(),
		Stream:        stream,
		Error:         errMsg,
		Status:        "error",
		DurationMs:    durationMs,
		ChannelID:     ch.ID(),
		ChannelType:   ch.Type(),
		BillingMode:   billingMode,
	}
	if uc != nil {
		entry.ApiKeyID = uc.KeyID
	}

	h.persistCallLog(entry, uc, 0, 0)
}

// persistCallLog 把构造好的 CallLog 写到所有持久化层：内存环、SSE、JSONL。
// 与 addCallLogWithKey 的写法对齐（避免 channel 路径走丢任何一个 sink）。
func (h *Handler) persistCallLog(entry CallLog, uc *UserContext, inputTokens, outputTokens int) {
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
	h.broadcastLog(entry)

	if entry.Status == "success" && uc != nil && uc.KeyID != "" && (inputTokens+outputTokens) > 0 {
		h.recordKeyUsage(uc.KeyID, entry.OriginalModel, int64(inputTokens+outputTokens), 0, false)
	}

	go appendLogToFile(entry)
}

// billingStatus 根据 TokenReservation.Action 映射 CallLog.BillingStatus。
//   - "" 表示无 keyID 或不适用
//   - "free" 表示天卡覆盖
//   - "paid" 表示从余额扣款
func billingStatus(res *TokenReservation) string {
	if res == nil || res.KeyID == "" || res.Action == "" {
		return ""
	}
	if res.Action == "free" {
		return "free"
	}
	return "paid"
}

// sendProtocolError 根据协议类型路由到对应的错误响应函数。
func (h *Handler) sendProtocolError(w http.ResponseWriter, p Protocol, code int, kind, msg string) {
	if p == ProtocolClaude {
		h.sendClaudeError(w, code, kind, msg)
		return
	}
	h.sendOpenAIError(w, code, kind, msg)
}

// channelResponseWriter 包装 http.ResponseWriter 以追踪 header 是否已写出。
// 渠道执行失败时，如果响应还没开始就可以补一个错误 JSON；如果已经流式输出过就只能日志记录。
type channelResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *channelResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return // 防止重复 WriteHeader 触发 panic
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *channelResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *channelResponseWriter) Flush() {
	// Flush 意味着上游已经开始往客户端推数据 — 标记 wroteHeader 以正确触发漏扣保护
	w.wroteHeader = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *channelResponseWriter) WroteHeader() bool {
	return w.wroteHeader
}
