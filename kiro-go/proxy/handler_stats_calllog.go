package proxy

import (
	"kiro-api-proxy/config"
	"strings"
	"time"
)

func (h *Handler) addCallLog(apiType, originalModel, actualModel, account, tier string, inputTokens, outputTokens int, stream bool, credits float64, reqSummary, respSummary, stopReason, requestID string, durationMs int64) {
	h.addCallLogWithKey(apiType, originalModel, actualModel, account, tier, inputTokens, outputTokens, stream, credits, credits, reqSummary, respSummary, stopReason, requestID, durationMs, nil)
}

// addCallLogWithKey 写入一条调用日志。
// credits         = 计费 credits（掺水后；若未掺水 = 上游原值）
// upstreamCredits = 上游原始 credits（掺水前的真实上游消耗，用于 admin 审计）
func (h *Handler) addCallLogWithKey(apiType, originalModel, actualModel, account, tier string, inputTokens, outputTokens int, stream bool, credits, upstreamCredits float64, reqSummary, respSummary, stopReason, requestID string, durationMs int64, uc *UserContext) {
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)
	keyID := ""
	if uc != nil {
		keyID = uc.KeyID
	}
	costUSD := CreditsToCostUSDForKey(credits, ResolveModelPool(originalModel), keyID, originalModel)
	// Shadow 校验：同时算 v1 旧公式（PoolPriceUSD × ModelMultiplier）的 cost，存到 CostUSDLegacy。
	// 部署 v2 后 24 小时观察期内 grep 看 cost_usd 跟 cost_usd_legacy 是否始终相等，
	// 不等说明迁移有偏差，立即回滚。
	costUSDLegacy := credits * LegacyModelPriceUSD(originalModel)
	var paidCostUSD, giftCostUSD float64
	var paidCredits, giftedCredits float64

	if uc != nil && uc.KeyID != "" {
		paidCostUSD = uc.ActualPaidUSD
		giftCostUSD = uc.ActualGiftUSD
		costUSD = paidCostUSD // Only report actual paid Revenue in metrics!

		// Derive credits back from proportion of cost, or if cost is 0 and credits exist, this might just be 0
		totalCost := paidCostUSD + giftCostUSD
		if totalCost > 0 {
			paidRatio := paidCostUSD / totalCost
			paidCredits = credits * paidRatio
			giftedCredits = credits - paidCredits
		} else if credits > 0 {
			// If action="free" and no USD charged
			paidCredits = 0
			giftedCredits = 0
		}
	} else {
		paidCredits = credits
	}

	entry := CallLog{
		Time:            now.In(cst).Format("01-02 15:04:05"),
		Timestamp:       now.Unix(),
		APIType:         apiType,
		OriginalModel:   originalModel,
		ActualModel:     actualModel,
		Account:         account,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		TotalTokens:     inputTokens + outputTokens,
		Credits:         credits,
		UpstreamCredits: upstreamCredits,
		PaidCredits:     paidCredits,
		GiftedCredits:   giftedCredits,
		CostUSD:         costUSD,
		// ChargedUSD 必须是虚拟 $（与 token/newapi 一致），不是 credits。
		// 用 paidCostUSD + giftCostUSD 而非 paidCredits + giftedCredits 避免 billingAmount() 把
		// credits 当 $ 累计导致 legacy 模式量级飘走。
		ChargedUSD:    paidCostUSD + giftCostUSD,
		CostUSDLegacy: costUSDLegacy,
		PriceModel:      originalModel,
		Stream:          stream,
		Status:          "success",
		Subscription:    tier,
		StopReason:      stopReason,
		DurationMs:      durationMs,
		RequestID:       requestID,
		RequestSummary:  reqSummary,
		ResponseSummary: respSummary,
	}
	if uc != nil {
		entry.ApiKeyID = uc.KeyID
	}
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
	h.broadcastLog(entry)

	// 记录 API Key 使用统计
	if uc != nil && uc.KeyID != "" {
		h.recordKeyUsage(uc.KeyID, originalModel, int64(inputTokens+outputTokens), credits, false)
	}

	// 记录 credit 历史用于预测
	if credits > 0 && h.creditPredictor != nil {
		rec := CreditRecord{
			Timestamp: now.Unix(),
			Credits:   credits,
			Model:     actualModel,
			Tokens:    inputTokens + outputTokens,
		}
		h.creditPredictor.Add(rec)
		if strings.EqualFold(tier, "PRO") {
			h.proCreditPredictor.Add(rec)
		} else {
			h.freeCreditPredictor.Add(rec)
		}
	}

	// 持久化日志：入队让 worker 顺序消费 + fsync，队列满 fallback 同步写
	h.enqueueCallLog(entry)
}

func (h *Handler) addCallLogError(apiType, originalModel, actualModel, account string, stream bool, errMsg string, payloadKB int) {
	h.addCallLogErrorWithKey(apiType, originalModel, actualModel, account, stream, errMsg, payloadKB, nil)
}

func (h *Handler) addCallLogErrorWithKey(apiType, originalModel, actualModel, account string, stream bool, errMsg string, payloadKB int, uc *UserContext) {
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)
	entry := CallLog{
		Time:          now.In(cst).Format("01-02 15:04:05"),
		Timestamp:     now.Unix(),
		APIType:       apiType,
		OriginalModel: originalModel,
		ActualModel:   actualModel,
		Account:       account,
		Stream:        stream,
		Error:         errMsg,
		PayloadKB:     payloadKB,
		Status:        "error",
	}
	if uc != nil {
		entry.ApiKeyID = uc.KeyID
	}
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
	h.broadcastLog(entry)

	// 记录 API Key 错误统计
	if uc != nil && uc.KeyID != "" {
		h.recordKeyUsage(uc.KeyID, originalModel, 0, 0, true)
	}

	// 持久化错误日志：同 success 路径，走队列 + fsync
	h.enqueueCallLog(entry)
}

// recordKeyUsage 记录 API Key 使用统计到内存缓存
func (h *Handler) recordKeyUsage(keyID, model string, tokens int64, credits float64, isError bool) {
	h.apiKeyStatsMu.Lock()
	defer h.apiKeyStatsMu.Unlock()

	stats, ok := h.apiKeyStats[keyID]
	if !ok {
		stats = &ApiKeyStats{Models: make(map[string]int64)}
		h.apiKeyStats[keyID] = stats
	}
	stats.LastUsed = time.Now().Unix()
	stats.Requests++
	if isError {
		stats.Errors++
	}
	stats.Tokens += tokens
	stats.Credits += credits
	if model != "" {
		stats.Models[model]++
	}
}

// flushApiKeyStats 将内存中的 API Key 统计刷新到配置
func (h *Handler) flushApiKeyStats() {
	h.apiKeyStatsMu.RLock()
	snapshot := make(map[string]*ApiKeyStats, len(h.apiKeyStats))
	for k, v := range h.apiKeyStats {
		cp := *v
		cp.Models = make(map[string]int64, len(v.Models))
		for m, c := range v.Models {
			cp.Models[m] = c
		}
		snapshot[k] = &cp
	}
	h.apiKeyStatsMu.RUnlock()

	for id, stats := range snapshot {
		config.UpdateApiKeyStatsNoSave(id, stats.LastUsed, stats.Requests, stats.Errors, stats.Tokens, stats.Credits, stats.Models)
	}
}

// broadcastLog 向所有 SSE 订阅者广播日志
func (h *Handler) broadcastLog(entry CallLog) {
	h.logSubscribersMu.RLock()
	defer h.logSubscribersMu.RUnlock()
	for ch := range h.logSubscribers {
		select {
		case ch <- entry:
		default:
			// 订阅者接收慢，跳过（不阻塞主流程）
		}
	}
}
