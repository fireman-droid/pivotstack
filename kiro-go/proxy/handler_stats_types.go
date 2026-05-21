package proxy

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
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
