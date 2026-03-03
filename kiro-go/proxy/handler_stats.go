package proxy

import (
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"sync/atomic"
	"time"
)

// handleHealth 健康检查（不暴露统计数据）
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": config.Version,
		"uptime":  time.Now().Unix() - h.startTime,
	})
}

// handleStats 统计数据（需要 API Key 鉴权）
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ok",
		"version":         config.Version,
		"accounts":        h.pool.Count(),
		"available":       h.pool.AvailableCount(),
		"totalRequests":   atomic.LoadInt64(&h.totalRequests),
		"successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests":  atomic.LoadInt64(&h.failedRequests),
		"totalTokens":     atomic.LoadInt64(&h.totalTokens),
		"totalCredits":    h.getCredits(),
		"uptime":          time.Now().Unix() - h.startTime,
	})
}

// backgroundStatsSaver 后台定时保存统计数据
func (h *Handler) backgroundStatsSaver() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.saveStats()
		case <-h.stopStatsSaver:
			h.saveStats() // 退出前保存一次
			return
		}
	}
}

// saveStats 保存统计到配置文件
func (h *Handler) saveStats() {
	config.UpdateStats(
		int(atomic.LoadInt64(&h.totalRequests)),
		int(atomic.LoadInt64(&h.successRequests)),
		int(atomic.LoadInt64(&h.failedRequests)),
		int(atomic.LoadInt64(&h.totalTokens)),
		h.getCredits(),
	)
}

// getCredits 线程安全获取 credits
func (h *Handler) getCredits() float64 {
	h.creditsMu.RLock()
	defer h.creditsMu.RUnlock()
	return h.totalCredits
}

// addCredits 线程安全增加 credits
func (h *Handler) addCredits(credits float64) {
	h.creditsMu.Lock()
	h.totalCredits += credits
	h.creditsMu.Unlock()
}

// 统计记录 (使用原子操作)
func (h *Handler) recordSuccess(inputTokens, outputTokens int, credits float64) {
	atomic.AddInt64(&h.totalRequests, 1)
	atomic.AddInt64(&h.successRequests, 1)
	atomic.AddInt64(&h.totalTokens, int64(inputTokens+outputTokens))
	h.addCredits(credits)
}

func (h *Handler) addCallLog(apiType, originalModel, actualModel, account string, inputTokens, outputTokens int, stream bool) {
	cst := time.FixedZone("CST", 8*3600)
	entry := CallLog{
		Time:          time.Now().In(cst).Format("01-02 15:04:05"),
		APIType:       apiType,
		OriginalModel: originalModel,
		ActualModel:   actualModel,
		Account:       account,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		TotalTokens:   inputTokens + outputTokens,
		Stream:        stream,
	}
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
}

func (h *Handler) addCallLogError(apiType, originalModel, actualModel, account string, stream bool, errMsg string, payloadKB int) {
	cst := time.FixedZone("CST", 8*3600)
	entry := CallLog{
		Time:          time.Now().In(cst).Format("01-02 15:04:05"),
		APIType:       apiType,
		OriginalModel: originalModel,
		ActualModel:   actualModel,
		Account:       account,
		Stream:        stream,
		Error:         errMsg,
		PayloadKB:     payloadKB,
	}
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
}

func (h *Handler) recordFailure() {
	atomic.AddInt64(&h.totalRequests, 1)
	atomic.AddInt64(&h.failedRequests, 1)
}

// sendSSE 发送 Server-Sent Event
func (h *Handler) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	flusher.Flush()
}
