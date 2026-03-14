package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ==================== Credit 消耗记录 & EMA 预测 ====================

// CreditRecord 单次 credit 消耗记录
type CreditRecord struct {
	Timestamp int64   `json:"ts"`
	Credits   float64 `json:"credits"`
	Model     string  `json:"model"`
	Tokens    int     `json:"tokens"`
}

// CreditPredictor EMA 加权移动平均预测器
type CreditPredictor struct {
	mu      sync.RWMutex
	records []CreditRecord
	maxSize int
	alpha   float64 // EMA 平滑系数
}

// CreditPrediction 预测结果
type CreditPrediction struct {
	RatePerHour      float64 `json:"ratePerHour"`      // 活跃时段 Credits/小时
	DailyRate        float64 `json:"dailyRate"`        // 日均 Credit 消耗
	RemainingHours   float64 `json:"remainingHours"`   // 预计剩余小时（活跃时段）
	RemainingDays    float64 `json:"remainingDays"`    // 预计剩余天（按日均）
	AvgPerRequest    float64 `json:"avgPerRequest"`    // 平均每次请求 credit
	AvgTokens        int     `json:"avgTokens"`        // 平均 token/请求
	TotalRecords     int     `json:"totalRecords"`     // 历史记录数
	Sufficient       bool    `json:"sufficient"`       // 数据是否足够预测
	ActiveSessions   int     `json:"activeSessions"`   // 检测到的会话数
	AvgSessionLength float64 `json:"avgSessionLength"` // 平均会话时长(分钟)
	Confidence       string  `json:"confidence"`       // 预测置信度: low/medium/high
}

const sessionGapSeconds = 1800 // 30 分钟无请求 = 新会话

func newCreditPredictor(maxSize int, alpha float64) *CreditPredictor {
	return &CreditPredictor{maxSize: maxSize, alpha: alpha}
}

func (cp *CreditPredictor) Add(rec CreditRecord) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.records = append(cp.records, rec)
	if len(cp.records) > cp.maxSize {
		cp.records = cp.records[len(cp.records)-cp.maxSize:]
	}
}

// Predict 使用会话感知 + 日级消耗 + EMA 预测 credit 耗尽时间
func (cp *CreditPredictor) Predict(remainingCredits float64) CreditPrediction {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := CreditPrediction{TotalRecords: len(cp.records)}

	if len(cp.records) < 3 {
		result.Confidence = "low"
		return result // 数据不足
	}
	result.Sufficient = true

	// ========== 1. 基础统计 ==========
	var totalCredits float64
	var totalTokens int
	for _, r := range cp.records {
		totalCredits += r.Credits
		totalTokens += r.Tokens
	}
	result.AvgPerRequest = totalCredits / float64(len(cp.records))
	result.AvgTokens = totalTokens / len(cp.records)

	// ========== 2. 会话检测 ==========
	// 会话 = 间隔 < 30 分钟的连续请求序列
	type session struct {
		startTs  int64
		endTs    int64
		credits  float64
		requests int
	}
	var sessions []session
	cur := session{
		startTs:  cp.records[0].Timestamp,
		endTs:    cp.records[0].Timestamp,
		credits:  cp.records[0].Credits,
		requests: 1,
	}
	for i := 1; i < len(cp.records); i++ {
		gap := cp.records[i].Timestamp - cp.records[i-1].Timestamp
		if gap > sessionGapSeconds {
			// 新会话
			sessions = append(sessions, cur)
			cur = session{
				startTs:  cp.records[i].Timestamp,
				endTs:    cp.records[i].Timestamp,
				credits:  cp.records[i].Credits,
				requests: 1,
			}
		} else {
			cur.endTs = cp.records[i].Timestamp
			cur.credits += cp.records[i].Credits
			cur.requests++
		}
	}
	sessions = append(sessions, cur) // 最后一个会话
	result.ActiveSessions = len(sessions)

	// ========== 3. 会话感知活跃速率（EMA） ==========
	// 对每个会话算出 credits/hour，然后 EMA 加权
	var emaActiveRate float64
	var emaInitialized bool
	var totalActiveSecs float64

	for _, s := range sessions {
		durSec := float64(s.endTs - s.startTs)
		if durSec < 60 {
			durSec = 60 // 单次请求会话至少算 1 分钟
		}
		totalActiveSecs += durSec
		ratePerHour := s.credits / (durSec / 3600.0)

		if !emaInitialized {
			emaActiveRate = ratePerHour
			emaInitialized = true
		} else {
			emaActiveRate = cp.alpha*ratePerHour + (1-cp.alpha)*emaActiveRate
		}
	}

	result.RatePerHour = emaActiveRate
	if len(sessions) > 0 {
		result.AvgSessionLength = (totalActiveSecs / float64(len(sessions))) / 60.0 // 分钟
	}

	// ========== 4. 日级消耗（为了长期预测） ==========
	// 按天分桶，算每天消耗了多少 credit
	dayBuckets := make(map[string]float64)
	for _, r := range cp.records {
		day := fmt.Sprintf("%d", r.Timestamp/(86400)) // Unix 天号
		dayBuckets[day] += r.Credits
	}

	var emaDailyRate float64
	var dailyInitialized bool
	// 排序天号
	days := make([]string, 0, len(dayBuckets))
	for d := range dayBuckets {
		days = append(days, d)
	}
	// 简单排序
	for i := range days {
		for j := i + 1; j < len(days); j++ {
			if days[i] > days[j] {
				days[i], days[j] = days[j], days[i]
			}
		}
	}
	for _, d := range days {
		daily := dayBuckets[d]
		if !dailyInitialized {
			emaDailyRate = daily
			dailyInitialized = true
		} else {
			emaDailyRate = cp.alpha*daily + (1-cp.alpha)*emaDailyRate
		}
	}
	result.DailyRate = emaDailyRate

	// ========== 5. 综合预测 ==========
	// 活跃速率 → 剩余活跃小时数
	if emaActiveRate > 0 && remainingCredits > 0 {
		result.RemainingHours = remainingCredits / emaActiveRate
	}

	// 日均消耗 → 剩余天数（更适合长期预测）
	if emaDailyRate > 0 && remainingCredits > 0 {
		result.RemainingDays = remainingCredits / emaDailyRate
	}

	// 置信度
	if len(cp.records) >= 50 && len(sessions) >= 5 {
		result.Confidence = "high"
	} else if len(cp.records) >= 10 {
		result.Confidence = "medium"
	} else {
		result.Confidence = "low"
	}

	return result
}

// ==================== JSONL 日志持久化 ====================

var logFileMu sync.Mutex

func appendLogToFile(entry CallLog) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")

	// 检查文件大小，超过 10MB 轮转
	if info, err := os.Stat(logPath); err == nil && info.Size() > 10*1024*1024 {
		rotateLogFile(logPath)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[LogPersist] Failed to open log file: %v\n", err)
		return
	}
	defer f.Close()

	data, _ := json.Marshal(entry)
	f.Write(data)
	f.WriteString("\n")
}

func rotateLogFile(path string) {
	// 读取文件，保留后半部分
	f, err := os.Open(path)
	if err != nil {
		return
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	f.Close()

	// 保留后半部分
	half := len(lines) / 2
	if half < 1 {
		return
	}

	f, err = os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	for _, line := range lines[half:] {
		f.WriteString(line + "\n")
	}
	fmt.Printf("[LogPersist] Rotated log file, kept %d/%d entries\n", len(lines)-half, len(lines))
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
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var entry CallLog
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		allLogs = append(allLogs, entry)
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

	// 恢复 CreditPredictor 历史
	creditRestored := 0
	for _, entry := range allLogs {
		if entry.Credits > 0 && entry.Timestamp > 0 {
			rec := CreditRecord{
				Timestamp: entry.Timestamp,
				Credits:   entry.Credits,
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
	}

	fmt.Printf("[LogRestore] Restored %d logs from disk, %d credit records for prediction\n",
		len(h.callLogs), creditRestored)
}

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
		"freePool":        h.pool.TierStats("free"),
		"proPool":         h.pool.TierStats("pro"),
	})
}

// backgroundStatsSaver 后台定时保存统计数据（每 5 分钟批量写入）
func (h *Handler) backgroundStatsSaver() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.flushAllStats()
		case <-h.stopStatsSaver:
			h.flushAllStats() // 退出前保存一次
			return
		}
	}
}

// flushAllStats 批量刷新所有统计到磁盘
func (h *Handler) flushAllStats() {
	// 1. 保存全局统计
	h.saveStats()
	// 2. 从账号池批量更新到配置（内存操作）
	h.pool.FlushStatsToConfig()
	// 3. 一次性写盘
	config.SaveConfig()
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

func (h *Handler) addCallLog(apiType, originalModel, actualModel, account, tier string, inputTokens, outputTokens int, stream bool, credits float64, reqSummary, respSummary string) {
	now := time.Now()
	cst := time.FixedZone("CST", 8*3600)
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
		Stream:          stream,
		Status:          "success",
		Subscription:    tier,
		RequestSummary:  reqSummary,
		ResponseSummary: respSummary,
	}
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
	h.broadcastLog(entry)

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

	// 持久化日志到 JSONL 文件
	go appendLogToFile(entry)
}

func (h *Handler) addCallLogError(apiType, originalModel, actualModel, account string, stream bool, errMsg string, payloadKB int) {
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
	h.callLogsMu.Lock()
	h.callLogs = append(h.callLogs, entry)
	if len(h.callLogs) > maxCallLogs {
		h.callLogs = h.callLogs[len(h.callLogs)-maxCallLogs:]
	}
	h.callLogsMu.Unlock()
	h.broadcastLog(entry)

	// 持久化错误日志
	go appendLogToFile(entry)
}

func (h *Handler) recordFailure() {
	atomic.AddInt64(&h.totalRequests, 1)
	atomic.AddInt64(&h.failedRequests, 1)
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

	// 持续监听
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
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

	for {
		select {
		case <-ctx.Done():
			return
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
