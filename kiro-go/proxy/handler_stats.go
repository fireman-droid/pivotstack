package proxy

import (
	"bufio"
	crand "crypto/rand"
	"encoding/hex"
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

// genRequestID 生成短请求 ID (8 字符 hex)
func genRequestID() string {
	b := make([]byte, 4)
	crand.Read(b)
	return hex.EncodeToString(b)
}

// ==================== API Key 统计 ====================

// ApiKeyStats 内存中的 API Key 使用统计
type ApiKeyStats struct {
	LastUsed int64
	Requests int64
	Errors   int64
	Tokens   int64
	Credits  float64
	Models   map[string]int64
}

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

// logRetentionDays 日志保留天数
const logRetentionDays = 7

func appendLogToFile(entry CallLog) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")

	// 文件超过 2MB 时清理过期条目（避免频繁扫描）
	if info, err := os.Stat(logPath); err == nil && info.Size() > 2*1024*1024 {
		cleanupLogFile(logPath)
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
		}
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Timestamp >= cutoff {
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

// ==================== 充值流水持久化 ====================

var rechargeFileMu sync.Mutex

// appendRechargeRecord 写一条充值流水到 data/recharge_records.jsonl（立即落盘）。
// 与 callLogs 异步写入不同：充值记录金额关键，必须同步落盘并刷盘以防丢失。
func appendRechargeRecord(rec RechargeRecord) {
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[Recharge] Failed to open recharge log file: %v\n", err)
		return
	}
	defer f.Close()

	data, _ := json.Marshal(rec)
	if _, err := f.Write(data); err != nil {
		fmt.Printf("[Recharge] Failed to write: %v\n", err)
		return
	}
	f.WriteString("\n")
	_ = f.Sync() // 强制刷盘，钱关键
}

// readRechargeRecords 读取充值流水。filterKeyID 为空时返回全部，否则只过滤该 key。
// 返回最新在前的列表 + 总数（满足过滤条件的总数，用于分页）。
func readRechargeRecords(filterKeyID string, page, limit int) ([]RechargeRecord, int) {
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	var all []RechargeRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var rec RechargeRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if filterKeyID != "" && rec.KeyID != filterKeyID {
			continue
		}
		all = append(all, rec)
	}

	// 倒序：最新在前
	total := len(all)
	out := make([]RechargeRecord, total)
	for i := 0; i < total; i++ {
		out[i] = all[total-1-i]
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	start := (page - 1) * limit
	if start >= total {
		return []RechargeRecord{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return out[start:end], total
}

// recentCallCount 计算某 key 过去 N 天的调用次数（含 success+error，从 call_logs.jsonl 文件扫描）。
// 用于活动门槛"活跃度"判断。
//
// 性能：当前 callLogs 上限 5000 条，扫一次 < 50ms，不需要索引。
func recentCallCount(keyID string, days int) int {
	if keyID == "" || days <= 0 {
		return 0
	}
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	cutoff := time.Now().Unix() - int64(days*86400)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		var entry struct {
			Timestamp int64  `json:"timestamp"`
			ApiKeyID  string `json:"api_key_id"`
			Status    string `json:"status"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.ApiKeyID != keyID {
			continue
		}
		// 只算成功调用——避免恶意发废请求绕过活跃度门槛（Bug #4）
		if entry.Status != "success" {
			continue
		}
		if entry.Timestamp >= cutoff {
			count++
		}
	}
	return count
}

// DailyStat 单日调用统计 bucket（用户活跃度图表）
type DailyStat struct {
	Date    string  `json:"date"`    // "YYYY-MM-DD" CST
	Calls   int     `json:"calls"`   // 成功调用数
	Errors  int     `json:"errors"`  // 失败数
	Tokens  int64   `json:"tokens"`  // input + output tokens 合计
	CostUSD float64 `json:"costUSD"` // 实际消耗（不区分余额来源）
}

// recentDailyStats 计算某 key 过去 N 天每日统计（按 CST 自然日 bucket，含当天）。
// 即使某天没有调用也会返回 0 值 bucket，方便前端画 7 天连续柱状图。
func recentDailyStats(keyID string, days int) []DailyStat {
	if keyID == "" || days <= 0 {
		return nil
	}
	if days > 60 {
		days = 60
	}

	loc := time.Now().Location()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	buckets := make(map[string]*DailyStat, days)
	order := make([]string, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		buckets[key] = &DailyStat{Date: key}
		order = append(order, key)
	}
	cutoff := today.AddDate(0, 0, -(days - 1)).Unix()

	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "call_logs.jsonl")
	if f, err := os.Open(logPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var entry struct {
				Timestamp    int64   `json:"timestamp"`
				ApiKeyID     string  `json:"api_key_id"`
				Status       string  `json:"status"`
				InputTokens  int     `json:"input_tokens"`
				OutputTokens int     `json:"output_tokens"`
				CostUSD      float64 `json:"cost_usd"`
			}
			if json.Unmarshal(scanner.Bytes(), &entry) != nil {
				continue
			}
			if entry.ApiKeyID != keyID {
				continue
			}
			if entry.Timestamp < cutoff {
				continue
			}
			ts := time.Unix(entry.Timestamp, 0).In(loc)
			dateKey := ts.Format("2006-01-02")
			b, ok := buckets[dateKey]
			if !ok {
				continue
			}
			if entry.Status == "error" {
				b.Errors++
			} else {
				b.Calls++
			}
			b.Tokens += int64(entry.InputTokens) + int64(entry.OutputTokens)
			b.CostUSD += entry.CostUSD
		}
	}

	out := make([]DailyStat, len(order))
	for i, k := range order {
		out[i] = *buckets[k]
	}
	return out
}

// monthlyRechargeSumCNY 计算一个 key 当前自然月的充值总额（CNY）。
// 用于活动门槛判断；与 ApiKeyInfo.MonthlyRechargedCNY 字段互为冗余但更可信。
func monthlyRechargeSumCNY(keyID string) float64 {
	if keyID == "" {
		return 0
	}
	rechargeFileMu.Lock()
	defer rechargeFileMu.Unlock()

	logPath := filepath.Join(config.GetDataDir(), "recharge_records.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	now := time.Now()
	year, month, _ := now.Date()
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, now.Location()).Unix()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var sum float64
	for scanner.Scan() {
		var rec RechargeRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.KeyID != keyID {
			continue
		}
		if rec.Timestamp < monthStart {
			continue
		}
		// 只统计正向"充值"类（不含 admin_adjust 减少之类）
		if rec.AmountCNY > 0 {
			sum += rec.AmountCNY
		}
	}
	return sum
}

// handleHealth 健康检查（不暴露统计数据）
func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": config.Version,
		"uptime":  time.Now().Unix() - h.startTime,
	})
}

// handleStats 统计数据（需要 API Key 鉴权）
func (h *Handler) handleStats(w http.ResponseWriter, _ *http.Request) {
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
	// 3. 刷新 API Key 统计到配置（内存操作）
	h.flushApiKeyStats()
	// 4. 一次性写盘
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
		CostUSDLegacy:   costUSDLegacy,
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

	// 持久化日志到 JSONL 文件
	go appendLogToFile(entry)
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

	// 持久化错误日志
	go appendLogToFile(entry)
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
