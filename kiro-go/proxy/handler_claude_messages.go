package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"kiro-api-proxy/config"
	"kiro-api-proxy/users"
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
	debugMode := os.Getenv("DEBUG_REQUESTS") == "true"
	if debugMode {
		debugFile, _ := os.OpenFile("data/debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if debugFile != nil {
			ts := time.Now().Format("2006-01-02 15:04:05")
			fmt.Fprintf(debugFile, "\n========== [%s] 新请求 ==========\n", ts)
			fmt.Fprintf(debugFile, "[请求体] %s\n", string(body))
			debugFile.Close()
		}
	}

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Invalid JSON: "+err.Error())
		return
	}

	// 用户层 tier 决策（必须在 GetNextByTier 之前）
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
			h.sendClaudeError(w, 429, "rate_limit_error", msg)
			return
		}
		defer OnRequestEnd(keyID)
	}

	// Model pool routing: determined by model, not by user tier
	// Credit users can use any model; day card restrictions handled by ValidateKeyAccessForModel
	pool := ResolveModelPool(req.Model)
	if keyID != "" {
		info := users.OverlayWalletOnKey(config.FindApiKeyByID(keyID))
		if info != nil {
			_, err := config.ValidateKeyAccessForModel(info, pool)
			if err != nil {
				h.sendClaudeError(w, 403, "forbidden", err.Error())
				return
			}
		}
	}

	// v3 渠道分发：channels 非空时所有渠道（含 Kiro）统一走 token 计费。
	if router := h.currentChannelRouter(); router != nil && router.HasChannels() {
		hint := &ResolveHint{
			Protocol:  ProtocolClaude,
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
				h.sendClaudeError(w, 400, "invalid_request_error",
					fmt.Sprintf("channel %q is not available for model %q", hint.ChannelID, req.Model))
				return
			}
			h.sendClaudeError(w, 404, "model_not_found",
				fmt.Sprintf("model %q is not available in any configured channel", req.Model))
			return
		}
		ch := rr.Channel
		maxOutput := 4096
		if req.MaxTokens > 0 {
			maxOutput = req.MaxTokens
		}
		h.handleChannelRequest(w, r, ch, &channelDispatch{
			Protocol:       ProtocolClaude,
			OriginalModel:  req.Model,
			Stream:         req.Stream,
			EstimatedInput: estimateClaudeRequestInputTokens(&req),
			MaxOutput:      maxOutput,
			RawBody:        body,
		}, uc)
		return
	}

	// === Legacy 路径（channels=[] 兜底，按 credit 计费）===
	originalModel := req.Model

	var preChargedPaid, preChargedGift float64
	if keyID != "" {
		estimatedInput := estimateClaudeRequestInputTokens(&req)
		maxTokens := 4096
		if req.MaxTokens > 0 {
			maxTokens = req.MaxTokens
		}
		var preErr error
		preChargedPaid, preChargedGift, preErr = PreAuthorize(keyID, originalModel, maxTokens, estimatedInput)
		if preErr != nil {
			h.sendClaudeError(w, 402, "insufficient_balance", preErr.Error())
			return
		}
	}

	legacyStart := time.Now()
	requestID := genRequestID()
	result, execErr := h.executeKiroClaude(r.Context(), w, &req, body, uc, requestID)
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
		h.addCallLogErrorWithKey("Claude", originalModel, "", "", req.Stream, execErr.Error(), payloadKB, uc)
		return
	}
	if result == nil {
		RefundPreAuth(keyID, preChargedPaid, preChargedGift)
		h.recordFailure()
		h.addCallLogErrorWithKey("Claude", originalModel, "", "", req.Stream, "executeKiroClaude returned nil result", 0, uc)
		return
	}

	billingCredits := result.UpstreamCredits
	if billingCredits <= 0 {
		billingCredits = EstimateCredits(result.OutputTokens, result.InputTokens)
	}
	// Claude 专属 15 秒低输出保护
	if time.Since(legacyStart).Milliseconds() < 15000 {
		billingCredits = ApplyLowOutputProtection(result.OutputTokens, billingCredits, result.InputTokens)
	}
	if result.BillingModel != "" && result.ActualModel != "" && result.BillingModel != result.ActualModel {
		billingCredits *= StealthCreditMultiplier(result.BillingModel, result.ActualModel)
	}
	paid, gift := ReconcileWithBillingModel(keyID, originalModel, result.BillingModel, billingCredits, preChargedPaid, preChargedGift)
	if uc != nil {
		uc.ActualPaidUSD = paid
		uc.ActualGiftUSD = gift
	}
	h.recordSuccess(result.InputTokens, result.OutputTokens, billingCredits)
	h.addCallLogWithKey("Claude", originalModel, result.ActualModel, result.Account, result.Subscription, result.InputTokens, result.OutputTokens, req.Stream, billingCredits, result.UpstreamCredits, "", "", result.StopReason, result.RequestID, result.DurationMs, uc)
}
