package proxy

import (
	"context"
	"regexp"
	"sync"
	"time"
)

const (
	defaultNewAPIReconcilePollInterval = 5 * time.Second
	defaultNewAPIReconcilePollDelay    = 3 * time.Second
	defaultNewAPIReconcileRetryBudget  = 6
	newAPIReconcileFetchTimeout        = 10 * time.Second
	newAPIReconcileLogPageSize         = "50"
	newAPIReconcileEventLimit          = 100
	newAPIReconcileAdminEventLimit     = 20
	newAPIReconcileTimeWindowSec       = 30
	newAPIReconcileMoneyEpsilon        = 1e-9
)

type NewAPILog = NewAPILogEntry

type NewAPIReconciler struct {
	manager        *NewAPIManager
	mu             sync.Mutex
	queues         map[string]*providerReconcileQueue
	pollInterval   time.Duration
	pollDelayFirst time.Duration
	retryBudget    int
	stopCh         chan struct{}

	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	workers  map[string]struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

type pendingReservation struct {
	Reservation *NewAPIReservation
	RequestID   string
	KeyID       string
	EnqueuedAt  time.Time
	Attempt     int
	LastErr     string
}

type providerReconcileQueue struct {
	mu         sync.Mutex
	pending    map[string]*pendingReservation
	recent     []reconcileEvent
	debtCounts map[string]float64
	errorCount int // 累计 match_error / ambiguous / no_match 数；admin UI 用来红色高亮异常 provider
}

// reconcileEvent.Timestamp 用 unix seconds 与 call_logs.jsonl 的 "at"/"timestamp" 字段对齐，
// 前端直接 new Date(ts*1000)，不用解析 RFC3339。
type reconcileEvent struct {
	RequestID      string  `json:"requestId,omitempty"`
	KeyID          string  `json:"keyId,omitempty"`
	ChannelID      string  `json:"channelId,omitempty"`
	ProviderID     string  `json:"providerId,omitempty"`
	Status         string  `json:"status"`
	Attempt        int     `json:"attempt,omitempty"`
	UpstreamQuota  int64   `json:"upstreamQuota,omitempty"`
	EstimatedQuota int64   `json:"estimatedQuota,omitempty"`
	PaidUSDDelta   float64 `json:"paidUsdDelta,omitempty"`
	GiftUSDDelta   float64 `json:"giftUsdDelta,omitempty"`
	DebtUSDAdded   float64 `json:"debtUsdAdded,omitempty"`
	Error          string  `json:"error,omitempty"`
	Timestamp      int64   `json:"timestamp"`
}

type reconcileProviderStatus struct {
	ProviderID           string           `json:"providerId"`
	PendingCount         int              `json:"pendingCount"`
	RecentEvents         []reconcileEvent `json:"recentEvents"` // reverse-chronological：最新在前
	DebtAddedThisSession float64          `json:"debtAddedThisSession"`
	ErrorCount           int              `json:"errorCount"`
}

// bearerSecretRegex 防 upstream error 消息里 echo 的 sk-* 泄露给 admin UI / 审计日志。
// 跟 Phase 4a 运行时 redactUpstreamSecret 互补（运行时是 body redact，这里是 error string redact）。
var bearerSecretRegex = regexp.MustCompile(`(?i)Bearer\s+sk-[a-zA-Z0-9_\-]+`)
