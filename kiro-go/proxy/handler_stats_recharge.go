package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kiro-api-proxy/config"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
