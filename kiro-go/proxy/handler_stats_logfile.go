package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ==================== JSONL 日志持久化 ====================

var logFileMu sync.Mutex

// logRetentionDays 日志保留天数
const logRetentionDays = 7

// callLogPersistQueueSize 内存队列容量。worker 顺序消费 + fsync；满了 fallback 同步写。
const callLogPersistQueueSize = 1024

func appendLogToFile(entry CallLog) {
	if err := appendLogToFileChecked(entry); err != nil {
		fmt.Printf("[LogPersist] Failed to append log: %v\n", err)
	}
}

// appendLogToFileChecked 真实落盘 + fsync，返回错误以便上层观测。
func appendLogToFileChecked(entry CallLog) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")

	// 文件超过 2MB 时清理过期条目（避免频繁扫描）
	if info, err := os.Stat(logPath); err == nil && info.Size() > 2*1024*1024 {
		cleanupLogFile(logPath)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// enqueueCallLog 请求路径快速入队，worker 异步消费。
// 队列满时退化为同步落盘，保证不丢日志。
func (h *Handler) enqueueCallLog(entry CallLog) {
	if h == nil || h.logPersistCh == nil {
		go appendLogToFile(entry)
		return
	}
	select {
	case h.logPersistCh <- entry:
	default:
		appendLogToFile(entry)
	}
}

func (h *Handler) startLogPersistWorker() {
	if h == nil || h.logPersistCh == nil {
		return
	}
	go func() {
		defer close(h.logPersistDone)
		for {
			select {
			case entry := <-h.logPersistCh:
				appendLogToFile(entry)
			case <-h.logPersistStop:
				h.drainCallLogQueue()
				return
			}
		}
	}()
}

func (h *Handler) stopLogPersistWorker() {
	if h == nil || h.logPersistStop == nil || h.logPersistDone == nil {
		return
	}
	h.logPersistStopOnce.Do(func() { close(h.logPersistStop) })
	<-h.logPersistDone
}

func (h *Handler) drainCallLogQueue() {
	for {
		select {
		case entry := <-h.logPersistCh:
			appendLogToFile(entry)
		default:
			return
		}
	}
}

// appendCallLogReconcileEvent 把 Phase 4b 异步对账事件追加到 call_logs.jsonl，
// 用 type=reconcile + request_id 让 UI 查询时能 LEFT-JOIN 原 CallLog 行。
// 不修改原 CallLog（JSONL append-only），admin UI 自行 merge by request_id。
func appendCallLogReconcileEvent(requestID, status string, upstreamQuota int64, paidDelta, giftDelta, debtDelta float64) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	if info, err := os.Stat(logPath); err == nil && info.Size() > 2*1024*1024 {
		cleanupLogFile(logPath)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[LogPersist] Failed to open log file: %v\n", err)
		return
	}
	defer f.Close()

	entry := struct {
		Type          string  `json:"type"`
		RequestID     string  `json:"request_id"`
		BillingStatus string  `json:"billing_status"`
		UpstreamQuota int64   `json:"upstream_quota"`
		PaidDelta     float64 `json:"paid_delta"`
		GiftDelta     float64 `json:"gift_delta"`
		DebtDelta     float64 `json:"debt_delta"`
		At            int64   `json:"at"`
	}{
		Type:          "reconcile",
		RequestID:     requestID,
		BillingStatus: status,
		UpstreamQuota: upstreamQuota,
		PaidDelta:     paidDelta,
		GiftDelta:     giftDelta,
		DebtDelta:     debtDelta,
		At:            time.Now().Unix(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("[LogPersist] reconcile event marshal failed: %v\n", err)
		return
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		fmt.Printf("[LogPersist] reconcile event write failed: %v\n", err)
		return
	}
	if err := f.Sync(); err != nil {
		fmt.Printf("[LogPersist] reconcile event fsync failed: %v\n", err)
	}
}

// cleanupLogFile 清理超过 logRetentionDays 天的日志条目
func cleanupLogFile(path string) {
	cutoff := time.Now().Unix() - int64(logRetentionDays*86400)

	f, err := os.Open(path)
	if err != nil {
		return
	}
	var kept []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	total := 0
	for scanner.Scan() {
		total++
		line := scanner.Text()
		var entry struct {
			Timestamp int64 `json:"timestamp"`
			At        int64 `json:"at"` // Phase 4b reconcile event 用 "at" 而非 "timestamp"
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		ts := entry.Timestamp
		if ts == 0 {
			ts = entry.At
		}
		if ts >= cutoff {
			kept = append(kept, line)
		}
	}
	f.Close()

	removed := total - len(kept)
	if removed == 0 {
		return
	}

	f2, err := os.Create(path)
	if err != nil {
		return
	}
	defer f2.Close()
	for _, line := range kept {
		f2.WriteString(line + "\n")
	}
	fmt.Printf("[LogCleanup] Removed %d expired entries (>%d days), kept %d\n", removed, logRetentionDays, len(kept))
}

// startLogCleanupTicker 每 6 小时自动清理过期日志（磁盘 + 内存）
func (h *Handler) startLogCleanupTicker() {
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			// 清理磁盘
			logFileMu.Lock()
			cleanupLogFile(filepath.Join(config.GetDataDir(), "call_logs.jsonl"))
			logFileMu.Unlock()

			// 清理内存
			cutoff := time.Now().Unix() - int64(logRetentionDays*86400)
			h.callLogsMu.Lock()
			newLogs := make([]CallLog, 0, len(h.callLogs))
			for _, l := range h.callLogs {
				if l.Timestamp >= cutoff {
					newLogs = append(newLogs, l)
				}
			}
			removed := len(h.callLogs) - len(newLogs)
			h.callLogs = newLogs
			h.callLogsMu.Unlock()
			if removed > 0 {
				fmt.Printf("[LogCleanup] Cleaned %d expired entries from memory\n", removed)
			}
		}
	}()
}

// loadLogsFromDisk 启动时从 JSONL 恢复历史日志和 CreditPredictor
func (h *Handler) loadLogsFromDisk() {
	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("[LogRestore] No log file found at %s, starting fresh\n", logPath)
		return
	}
	defer f.Close()

	var allLogs []CallLog
	cutoff := time.Now().Unix() - int64(logRetentionDays*86400)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineCount := 0
	skipped := 0
	for scanner.Scan() {
		lineCount++
		var entry CallLog
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		// 只加载 7 天内的日志
		if entry.Timestamp > 0 && entry.Timestamp < cutoff {
			skipped++
			continue
		}
		allLogs = append(allLogs, entry)
	}
	if skipped > 0 {
		fmt.Printf("[LogRestore] Skipped %d expired entries (>%d days)\n", skipped, logRetentionDays)
	}

	if len(allLogs) == 0 {
		fmt.Printf("[LogRestore] Log file empty\n")
		return
	}

	// 恢复内存 callLogs（最近 maxCallLogs 条）
	h.callLogsMu.Lock()
	if len(allLogs) > maxCallLogs {
		h.callLogs = allLogs[len(allLogs)-maxCallLogs:]
	} else {
		h.callLogs = allLogs
	}
	h.callLogsMu.Unlock()

	// 恢复 CreditPredictor 历史 + API Key 统计
	// v3 token 模式下 Credits=0 但 UpstreamCredits>0 — 用 UpstreamCredits 兜底，
	// 否则 token 调用不会进预测器，账号配额预测会被错估
	creditRestored := 0
	for _, entry := range allLogs {
		predictorCredits := entry.Credits
		if predictorCredits == 0 && entry.UpstreamCredits > 0 {
			predictorCredits = entry.UpstreamCredits
		}
		if predictorCredits > 0 && entry.Timestamp > 0 {
			rec := CreditRecord{
				Timestamp: entry.Timestamp,
				Credits:   predictorCredits,
				Model:     entry.ActualModel,
				Tokens:    entry.TotalTokens,
			}
			h.creditPredictor.Add(rec)
			// 按 tier 分流
			if strings.EqualFold(entry.Subscription, "PRO") {
				h.proCreditPredictor.Add(rec)
			} else {
				h.freeCreditPredictor.Add(rec)
			}
			creditRestored++
		}
		// 恢复 API Key 统计
		if entry.ApiKeyID != "" {
			stats, ok := h.apiKeyStats[entry.ApiKeyID]
			if !ok {
				stats = &ApiKeyStats{Models: make(map[string]int64)}
				h.apiKeyStats[entry.ApiKeyID] = stats
			}
			stats.Requests++
			if entry.Status == "error" {
				stats.Errors++
			}
			stats.Tokens += int64(entry.TotalTokens)
			stats.Credits += entry.Credits
			if entry.Timestamp > stats.LastUsed {
				stats.LastUsed = entry.Timestamp
			}
			if entry.ActualModel != "" {
				stats.Models[entry.ActualModel]++
			}
		}
	}

	fmt.Printf("[LogRestore] Restored %d logs from disk, %d credit records for prediction\n",
		len(h.callLogs), creditRestored)
}
