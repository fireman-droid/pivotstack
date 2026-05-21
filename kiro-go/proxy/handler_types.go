package proxy

import (
	"kiro-api-proxy/pool"
	"sync"
	"sync/atomic"
)

// Handler HTTP 处理器
type Handler struct {
	pool *pool.AccountPool
	// 运行时统计 (使用原子操作)
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalTokens     int64
	totalCredits    float64 // float64 需要用锁保护
	creditsMu       sync.RWMutex
	startTime       int64
	stopRefresh     chan struct{}
	stopStatsSaver  chan struct{}
	// 模型缓存
	cachedModels    []ModelInfo
	modelsCacheMu   sync.RWMutex
	modelsCacheTime int64
	// 调用日志
	callLogs   []CallLog
	callLogsMu sync.RWMutex
	// API Key 统计 (内存缓存)
	apiKeyStats   map[string]*ApiKeyStats
	apiKeyStatsMu sync.RWMutex
	// SSE 实时日志订阅
	logSubscribers   map[chan CallLog]bool
	logSubscribersMu sync.RWMutex
	// call log 持久化队列：请求路径只入队，worker 顺序写盘 + fsync。
	// 队列满时 fallback 同步写，保证不丢日志。
	logPersistCh       chan CallLog
	logPersistStop     chan struct{}
	logPersistDone     chan struct{}
	logPersistStopOnce sync.Once
	// Credit 消耗预测
	creditPredictor     *CreditPredictor
	proCreditPredictor  *CreditPredictor
	freeCreditPredictor *CreditPredictor
	// v3 渠道路由（空 = legacy Kiro 路径，零破坏）
	// atomic.Pointer 保证 admin 改 channels 时并发安全
	channelRouter atomic.Pointer[ChannelRouter]

	// Admin auth 运行时状态：session/csrf/sse-token/login-limiter 集中管理
	adminSessions *adminSessionStore

	// v5 NewAPI 聚合管理器（provider/sync/scheduler）
	newapiManager *NewAPIManager
	// v5 Phase 4b：异步对账 worker（per-provider goroutine 轮询 /api/log/self）
	newapiReconciler *NewAPIReconciler
}

type contextKeyType string

const userCtxKey contextKeyType = "userCtx"

// UserContext holds per-request API key context.
type UserContext struct {
	KeyID         string
	KeyTier       string // "free" | "pro"
	ActualPaidUSD float64
	ActualGiftUSD float64
}

// CallLog 单次调用记录（结构化日志）
type CallLog struct {
	Time            string  `json:"time"`
	Timestamp       int64   `json:"timestamp"`
	RequestID       string  `json:"request_id,omitempty"`
	APIType         string  `json:"api_type"`
	OriginalModel   string  `json:"original_model"`
	ActualModel     string  `json:"actual_model"`
	Account         string  `json:"account"`
	ApiKeyID        string  `json:"api_key_id,omitempty"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Credits         float64 `json:"credits,omitempty"`          // 计费 credits（掺水后；若未掺水 = 上游原值）
	UpstreamCredits float64 `json:"upstream_credits,omitempty"` // 上游原始 credits（掺水前；admin 审计用，用户端清零）
	// PaidCredits/GiftedCredits/CostUSD 故意不带 omitempty：
	// 即使 0 也要显式落盘，否则纯 gift 余额扣费的请求（paid=0）会让对账误判为"未扣费"。
	// 注意 CostUSD 在 handler_stats 中被覆盖为 paidCostUSD（仅营收，不含 gift 部分）。
	PaidCredits   float64 `json:"paid_credits"`
	GiftedCredits float64 `json:"gifted_credits"`
	CostUSD       float64 `json:"cost_usd"`
	// ChargedUSD：token/newapi 路径实际扣费总额（paid + gift，虚拟 $）。
	// 与 CostUSD（仅 paid 营收）的区别：profit 看 CostUSD，用户/key 总扣费看 ChargedUSD。
	ChargedUSD    float64 `json:"charged_usd,omitempty"`
	// CostUSDLegacy 是按 v1 旧公式（PoolPriceUSD × ModelMultiplier）算的 shadow 值。
	// 部署 v2 后 24 小时观察期内 grep 看 cost_usd 跟 cost_usd_legacy 是否始终相等，
	// 不等说明迁移有偏差，立即回滚。观察期后下个迭代清理这个字段。
	CostUSDLegacy   float64 `json:"cost_usd_legacy,omitempty"`
	PriceModel      string  `json:"price_model,omitempty"` // 计费用 model（含 stealth originalModel 信息）
	Stream          bool    `json:"stream"`
	Error           string  `json:"error,omitempty"`
	PayloadKB       int     `json:"payload_kb,omitempty"`
	Status          string  `json:"status"`
	StopReason      string  `json:"stop_reason,omitempty"`
	DurationMs      int64   `json:"duration_ms,omitempty"`
	Attempt         int     `json:"attempt,omitempty"`
	Subscription    string  `json:"subscription,omitempty"`
	RequestSummary  string  `json:"request_summary,omitempty"`
	ResponseSummary string  `json:"response_summary,omitempty"`
	// v3 渠道与计费模式（legacy 路径下 ChannelID/ChannelType 留空，BillingMode="legacy_credits"）
	ChannelID      string `json:"channel_id,omitempty"`
	ChannelType    string `json:"channel_type,omitempty"`
	BillingMode    string `json:"billing_mode,omitempty"`
	BillingStatus  string `json:"billing_status,omitempty"` // "free"=天卡覆盖 | "paid"=扣款 | "estimated"=NewAPI 同步估算（Phase 4b 异步对账后会被覆盖）| ""(空=不适用)
	UsageEstimated bool   `json:"usage_estimated,omitempty"`
}

const maxCallLogs = 5000

// RechargeRecord 充值/赠送/调整流水（金额关键，每条立即落盘）
// 写入路径：data/recharge_records.jsonl
type RechargeRecord struct {
	Time          string  `json:"time"`      // "MM-DD HH:MM:SS" CST
	Timestamp     int64   `json:"timestamp"` // unix seconds
	KeyID         string  `json:"key_id"`    // ApiKey UUID
	KeyNote       string  `json:"key_note,omitempty"`
	Type          string  `json:"type"`           // "code_redeem" | "code_redeem_days" | "admin_balance" | "admin_gift" | "admin_adjust"
	Code          string  `json:"code,omitempty"` // 仅 code_redeem 类型有
	AmountUSD     float64 `json:"amount_usd"`     // USD face value 增量（带正负号）
	AmountCNY     float64 `json:"amount_cny"`     // ¥ 等价（cny = usd × 0.05）
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	GiftBefore    float64 `json:"gift_before"`
	GiftAfter     float64 `json:"gift_after"`
	Operator      string  `json:"operator"` // "user"（自助兑换）| "admin"
	Note          string  `json:"note,omitempty"`
	IP            string  `json:"ip,omitempty"`
}

type thinkingStreamSource int

const (
	thinkingSourceUnknown thinkingStreamSource = iota
	thinkingSourceReasoningEvent
	thinkingSourceTagBlock
)

// tokenRefreshLeadSec 是 token 过期前提前刷新的窗口（秒）。
//
// 实测 AWS 偶尔在 expiresAt 之前 ~10 分钟就让 token 失效（上游返回 400 INVALID_MODEL_ID
// 而非 401），所以必须提前刷新。30 分钟窗口给足缓冲，单 token 寿命通常 1 小时，仍可正常轮换。
const tokenRefreshLeadSec int64 = 1800
