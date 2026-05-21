package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// handleSSELogs 实时日志 SSE 端点
func (h *Handler) handleSSELogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 注册订阅
	ch := make(chan CallLog, 100)
	h.logSubscribersMu.Lock()
	h.logSubscribers[ch] = true
	h.logSubscribersMu.Unlock()

	defer func() {
		h.logSubscribersMu.Lock()
		delete(h.logSubscribers, ch)
		h.logSubscribersMu.Unlock()
		close(ch)
	}()

	// 先发送历史日志（最近 50 条）
	h.callLogsMu.RLock()
	start := 0
	if len(h.callLogs) > 50 {
		start = len(h.callLogs) - 50
	}
	history := make([]CallLog, len(h.callLogs[start:]))
	copy(history, h.callLogs[start:])
	h.callLogsMu.RUnlock()

	for _, entry := range history {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
	}
	flusher.Flush()

	// 持续监听 + 定时校验 session 有效性（改密时 InvalidateAll 后这里要主动断开）
	ctx := r.Context()
	sessionHash := adminSessionHashFromCtx(ctx)
	sessionCheck := time.NewTicker(15 * time.Second)
	defer sessionCheck.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sessionCheck.C:
			if sessionHash != "" && !h.adminSessions.IsValid(sessionHash) {
				return // session 被踢出，主动关闭 SSE
			}
		case entry := <-ch:
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}

// handleSSEStats 实时统计 SSE 端点（每 2 秒推送一次）
func (h *Handler) handleSSEStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sendStats := func() {
		proPool := h.pool.TierStats("pro")
		freePool := h.pool.TierStats("free")
		proRemaining := (proPool.UsageLimit - proPool.UsageCurrent) + (proPool.TrialLimit - proPool.TrialCurrent)
		freeRemaining := (freePool.UsageLimit - freePool.UsageCurrent) + (freePool.TrialLimit - freePool.TrialCurrent)
		stats := map[string]interface{}{
			"accounts":        h.pool.Count(),
			"available":       h.pool.AvailableCount(),
			"totalRequests":   atomic.LoadInt64(&h.totalRequests),
			"successRequests": atomic.LoadInt64(&h.successRequests),
			"failedRequests":  atomic.LoadInt64(&h.failedRequests),
			"totalTokens":     atomic.LoadInt64(&h.totalTokens),
			"totalCredits":    h.getCredits(),
			"uptime":          time.Now().Unix() - h.startTime,
			"freePool":        freePool,
			"proPool":         proPool,
			"prediction":      h.creditPredictor.Predict(proRemaining + freeRemaining),
			"proPrediction":   h.proCreditPredictor.Predict(proRemaining),
			"freePrediction":  h.freeCreditPredictor.Predict(freeRemaining),
		}
		data, _ := json.Marshal(stats)
		fmt.Fprintf(w, "event: stats\ndata: %s\n\n", string(data))
		flusher.Flush()
	}

	// 立即发送一次
	sendStats()

	sessionHash := adminSessionHashFromCtx(ctx)
	sessionCheck := time.NewTicker(15 * time.Second)
	defer sessionCheck.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sessionCheck.C:
			if sessionHash != "" && !h.adminSessions.IsValid(sessionHash) {
				return // session 被踢出，主动关闭 SSE
			}
		case <-ticker.C:
			sendStats()
		}
	}
}

// sendSSE 发送 Server-Sent Event
func (h *Handler) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	flusher.Flush()
}
