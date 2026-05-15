// Package config provides configuration management for Kiro API Proxy.
//
// This package handles persistent storage and retrieval of:
//   - Account credentials and authentication tokens
//   - Server settings (port, host, API keys)
//   - Usage statistics and metrics
//   - Thinking mode configuration for AI responses
//
// All configuration is stored in a JSON file with thread-safe access
// via read-write mutex protection.
package config

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// argon2id parameters for admin password hashing.
// 64 MiB / 3 iterations / 2 lanes — OWASP 2026 recommended baseline.
const (
	adminPasswordArgonMemory      uint32 = 64 * 1024 // KiB → 64 MiB
	adminPasswordArgonTime        uint32 = 3
	adminPasswordArgonParallelism uint8  = 2
	adminPasswordSaltLen                 = 16
	adminPasswordKeyLen                  = 32
)

// passwordEnvOverride 标记当前内存中的 cfg.Password 是否来自 ADMIN_PASSWORD 环境变量。
// 为真时，UI 上的改密接口会直接拒绝（防止 admin 改完发现重启后又被 env 覆盖）。
var passwordEnvOverride bool

// ErrInvalidOldPassword 用于 ChangeAdminPassword：调用方据此区分
// "旧密码错"（401 给前端）vs hash/写盘等服务端错（500）。
var ErrInvalidOldPassword = fmt.Errorf("invalid old password")

// GenerateMachineId generates a UUID v4 format machine identifier.
// This ID is used to uniquely identify the proxy instance in Kiro API requests,
// helping with request tracking and rate limiting on the server side.
func GenerateMachineId() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // 版本 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // 变体
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// Account represents a Kiro API account with authentication credentials and usage statistics.
type Account struct {
	// Basic identification
	ID       string `json:"id"`                 // Unique account identifier (UUID)
	Email    string `json:"email,omitempty"`    // User email address
	UserId   string `json:"userId,omitempty"`   // Kiro user ID
	Nickname string `json:"nickname,omitempty"` // Display name for admin panel

	// Authentication credentials
	AccessToken  string `json:"accessToken"`            // OAuth access token for API calls
	RefreshToken string `json:"refreshToken"`           // OAuth refresh token for token renewal
	ClientID     string `json:"clientId,omitempty"`     // OIDC client ID (for IdC auth)
	ClientSecret string `json:"clientSecret,omitempty"` // OIDC client secret (for IdC auth)
	AuthMethod   string `json:"authMethod"`             // Authentication method: "idc" (AWS IdC) or "social" (GitHub/Google)
	Provider     string `json:"provider,omitempty"`     // Identity provider name (e.g., "BuilderId", "GitHub")
	Region       string `json:"region"`                 // AWS region for OIDC endpoints
	StartUrl     string `json:"startUrl,omitempty"`     // AWS SSO start URL
	ExpiresAt    int64  `json:"expiresAt,omitempty"`    // Token expiration timestamp (Unix seconds)
	MachineId    string `json:"machineId,omitempty"`    // UUID machine identifier for request tracking

	// Priority weight for load balancing (higher = more requests)
	Weight int `json:"weight,omitempty"` // 0 or 1 = normal, 2+ = higher priority

	// Account status
	Enabled        bool   `json:"enabled"`                  // Whether account is active in the pool
	AllowOverQuota bool   `json:"allowOverQuota,omitempty"` // Allow this account to be selected even when quota is exhausted
	BanStatus      string `json:"banStatus,omitempty"`      // Ban status: "ACTIVE", "BANNED", "SUSPENDED"
	BanReason      string `json:"banReason,omitempty"`      // Reason for ban/suspension
	BanTime        int64  `json:"banTime,omitempty"`        // Timestamp when ban was detected

	// Subscription information
	SubscriptionType  string `json:"subscriptionType,omitempty"`  // Tier: FREE, PRO, PRO_PLUS, or POWER
	SubscriptionTitle string `json:"subscriptionTitle,omitempty"` // Human-readable subscription name
	DaysRemaining     int    `json:"daysRemaining,omitempty"`     // Days until subscription expires

	// Usage tracking
	UsageCurrent  float64 `json:"usageCurrent,omitempty"`  // Current period usage (credits)
	UsageLimit    float64 `json:"usageLimit,omitempty"`    // Maximum allowed usage per period
	UsagePercent  float64 `json:"usagePercent,omitempty"`  // Usage percentage (0.0-1.0)
	NextResetDate string  `json:"nextResetDate,omitempty"` // Date when usage resets (YYYY-MM-DD)
	LastRefresh   int64   `json:"lastRefresh,omitempty"`   // Last info refresh timestamp

	// Trial usage tracking
	TrialUsageCurrent float64 `json:"trialUsageCurrent,omitempty"` // Trial quota current usage
	TrialUsageLimit   float64 `json:"trialUsageLimit,omitempty"`   // Trial quota total limit
	TrialUsagePercent float64 `json:"trialUsagePercent,omitempty"` // Trial quota usage percentage (0.0-1.0)
	TrialStatus       string  `json:"trialStatus,omitempty"`       // Trial status: ACTIVE, EXPIRED, NONE
	TrialExpiresAt    int64   `json:"trialExpiresAt,omitempty"`    // Trial expiration timestamp (Unix seconds)

	// Runtime statistics (updated during operation)
	RequestCount int     `json:"requestCount,omitempty"` // Total requests processed
	ErrorCount   int     `json:"errorCount,omitempty"`   // Total errors encountered
	LastUsed     int64   `json:"lastUsed,omitempty"`     // Last request timestamp
	TotalTokens  int     `json:"totalTokens,omitempty"`  // Cumulative tokens processed
	TotalCredits float64 `json:"totalCredits,omitempty"` // Cumulative credits consumed
}

// ApiKeyInfo represents a commercial API key with subscription and usage stats.
type ApiKeyInfo struct {
	ID             string           `json:"id"`
	Key            string           `json:"key"`
	Tier           string           `json:"tier,omitempty"` // "free" | "pro" (set via activation code)
	Plan           string           `json:"plan"`           // "timed" | "credit" | "hybrid"
	ExpiresAt      int64            `json:"expiresAt"`      // Unix seconds, 0 = never
	Enabled        bool             `json:"enabled"`
	Balance        float64          `json:"balance,omitempty"`        // USD balance (paid via activation codes)
	GiftBalance    float64          `json:"giftBalance,omitempty"`    // USD balance (gifted manually by admin)
	TotalRecharged float64          `json:"totalRecharged,omitempty"` // cumulative amount recharged via activation codes (USD)
	TotalGifted    float64          `json:"totalGifted,omitempty"`    // cumulative amount gifted by admin (USD)
	Note           string           `json:"note,omitempty"`
	CreatedAt      int64            `json:"createdAt"`
	LastUsed       int64            `json:"lastUsed,omitempty"`
	Requests       int64            `json:"requests"`
	Errors         int64            `json:"errors"`
	Tokens         int64            `json:"tokens"`
	Credits        float64          `json:"credits"` // cumulative credits consumed
	Models         map[string]int64 `json:"models,omitempty"`

	// === 代理体系（v1 新增；旧 key 默认零值，零破坏）===
	ParentKeyID      string  `json:"parentKeyId,omitempty"`      // 子 key 才有，指向 reseller key.ID
	IsReseller       bool    `json:"isReseller,omitempty"`       // admin 在面板上勾"开通代理"
	MaxChildKeys     int     `json:"maxChildKeys,omitempty"`     // 0 = 无限；admin 设的子 key 数量上限
	ResellerDiscount float64 `json:"resellerDiscount,omitempty"` // 0 / 1.0 = 无折扣；0.5 = 半价进货
	SoldToChildren   float64 `json:"soldToChildren,omitempty"`   // reseller 累计转给子 key 的总额（USD）

	// === 速率限制（防天卡共享）===
	// 0 = 走全局默认（200/min）；> 0 = 这张 key 单独的每分钟请求上限。
	// 天卡分级卖：便宜的卡 5/min（单人都嫌慢），贵的 60/min（小团队够用）。
	// 共享给 N 人 → N 人挤同一配额，自然劝退分发。兑换天卡时从 ActivationCode.RateLimitPerMin 拷贝过来。
	RateLimitPerMin int `json:"rateLimitPerMin,omitempty"`
}

// ActivationCode represents a redeemable code for balance or time extension.
type ActivationCode struct {
	Code          string  `json:"code"`                    // e.g. KIRO-XXXX-XXXX-XXXX
	Type          string  `json:"type"`                    // "balance" | "days" | "time"
	Amount        float64 `json:"amount"`                  // balance: CNY; days: number of days; time: seconds
	Tier          string  `json:"tier,omitempty"`          // "free" | "pro" (only for type=days/time)
	CodeExpiresAt int64   `json:"codeExpiresAt,omitempty"` // code itself expires (0=never)
	Used          bool    `json:"used"`
	UsedBy        string  `json:"usedBy,omitempty"` // ApiKey ID
	UsedAt        int64   `json:"usedAt,omitempty"`
	CreatedAt     int64   `json:"createdAt"`
	Note          string  `json:"note,omitempty"`

	// 仅 type=days/time 用：兑换后写入 ApiKeyInfo.RateLimitPerMin。
	// 0 = 兑换时不修改 key 的速率（保留 key 现有值）。
	RateLimitPerMin int `json:"rateLimitPerMin,omitempty"`

	// 仅 type=days/time 用：admin 卖给客户的实际价格（¥）。
	// 兑换时写入 RechargeRecord.AmountCNY 作为利润计算的"真实收入"来源。
	// balance 类型不需要这个字段（amount 本身就是 CNY 售价）。
	// 0 = 历史卡或白送的卡（不计入 revenue）。
	SalePriceCNY float64 `json:"salePriceCNY,omitempty"`
}

// PromotionConfig 活动门槛配置：admin 开启后，凡满足 OR 三个条件之一的 key 才享受活动价。
//
// 资格判定（OR 关系，任一满足即可）：
//  1. 在白名单（Whitelist）→ 直接通过
//  2. 本月累计充值 ≥ MinMonthlyRechargeCNY（且阈值 > 0）
//  3. 过去 RecentCallsDays 天调用次数 ≥ MinRecentCalls（且两者均 > 0）
//
// 都不满足时走原价（Pricing.ProPoolPriceUSD/FreePoolPriceUSD）。
// 阈值字段为 0 则视为该条件未启用。
type PromotionConfig struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"` // 活动名（如"五一骨折"）

	// === v2 主字段（per-model 活动价）===
	ModelPrices         map[string]float64 `json:"modelPrices,omitempty"`         // 活动期每个 model 的 USD/credit
	DefaultProPriceUSD  float64            `json:"defaultProPriceUSD,omitempty"`  // 活动期 PRO 池兜底
	DefaultFreePriceUSD float64            `json:"defaultFreePriceUSD,omitempty"` // 活动期 FREE 池兜底

	// === 资格判定（不变）===
	MinMonthlyRechargeCNY float64  `json:"minMonthlyRechargeCNY"` // 本月充值门槛（¥），0=不启用
	MinRecentCalls        int      `json:"minRecentCalls"`        // 活跃度门槛：调用次数，0=不启用
	RecentCallsDays       int      `json:"recentCallsDays"`       // 活跃度门槛：观察窗口（天），默认 7
	Whitelist             []string `json:"whitelist,omitempty"`   // 白名单：ApiKey UUID 数组
	StartTs               int64    `json:"startTs,omitempty"`     // 活动开始（unix sec），0=立即
	EndTs                 int64    `json:"endTs,omitempty"`       // 活动结束（unix sec），0=无限
	UpdatedAt             int64    `json:"updatedAt,omitempty"`
	UpdatedBy             string   `json:"updatedBy,omitempty"`

	// === Deprecated v1 字段（保留 JSON 兼容；启动时迁移）===
	ProPoolPriceUSD  float64 `json:"proPoolPriceUSD,omitempty"`
	FreePoolPriceUSD float64 `json:"freePoolPriceUSD,omitempty"`
}

// CostEntry represents a single account purchase record.
type CostEntry struct {
	ID        string  `json:"id"`                  // unique ID
	Count     int     `json:"count"`               // number of accounts
	CostCNY   float64 `json:"costCNY"`             // total cost in CNY
	Credits   float64 `json:"credits,omitempty"`   // credits per account (PRO only, FREE fixed 550)
	CreatedAt int64   `json:"createdAt,omitempty"` // unix timestamp
}

const FreeAccountCredits = 550.0 // fixed credits per FREE account
const CNYPerUSDFace = 0.05       // $1 face value = ¥0.05 real CNY

// ModelSellPrice 是按 token 计费的模型售价 + 成本（virtual$/1M token）。
//
// InputPerM / OutputPerM：售价 — 用于实际扣费（PreAuthorizeTokens / ReconcileTokenUsage）
// CostInputPerM / CostOutputPerM：成本 — admin 追踪进货成本，用于利润率显示与对账
//
// 成本字段可选；为 0 表示未追踪（旧配置兼容）。计费逻辑只看售价，不读成本。
type ModelSellPrice struct {
	InputPerM      float64 `json:"inputPerM"`
	OutputPerM     float64 `json:"outputPerM"`
	CostInputPerM  float64 `json:"costInputPerM,omitempty"`
	CostOutputPerM float64 `json:"costOutputPerM,omitempty"`
}

// ChannelConfig 是上游 API 渠道配置。BillingMode != "" 时由 ChannelRouter 路由请求。
// channels 为空时回退到 legacy Kiro 路径（零破坏）。
//
// ModelPrices 是渠道内部售价 — 同一个 model 名从两个不同渠道接入时可以分别定价。
// 查询优先级：channel.ModelPrices[model] → pricing.SellPrices[model]（全局兜底）→ ErrSellPriceMissing
type ChannelConfig struct {
	ID          string                    `json:"id"`
	Type        string                    `json:"type"` // "kiro" | "openai"
	BaseURL     string                    `json:"baseUrl,omitempty"`
	APIKey      string                    `json:"apiKey,omitempty"` // 外部渠道 API key，admin 接口返回时必须 mask
	Models      []string                  `json:"models"`
	ModelPrices map[string]ModelSellPrice `json:"modelPrices,omitempty"` // v3：渠道内部售价（同 model 不同渠道可独立定价）
	Enabled     bool                      `json:"enabled"`
}

// PricingConfig holds credit-based pricing.
//
// 模型级定价（v2，主路径）：
//   ModelPrices map[model] = USD/credit 售价，admin 在 UI 里逐 model 配
//   未配置的 model 按 ResolveModelPool 兜底到 DefaultProPriceUSD/DefaultFreePriceUSD
//
// Pool 仅用于路由判断（model→号池、能不能用、成本统计），不再做定价语义。
//
// 旧字段（ProPoolPriceUSD/FreePoolPriceUSD/ModelMultipliers）保留 JSON 兼容，
// 启动时 MigratePricingToModelLevel 会自动算 ModelPrices = pool_price × multiplier 注入。
type PricingConfig struct {
	// === v2 主字段 ===
	ModelPrices         map[string]float64 `json:"modelPrices,omitempty"`         // model 名小写 → USD/credit 售价
	DefaultProPriceUSD  float64            `json:"defaultProPriceUSD,omitempty"`  // PRO 池兜底（ModelPrices 未列出时），默认 0.20
	DefaultFreePriceUSD float64            `json:"defaultFreePriceUSD,omitempty"` // FREE 池兜底，默认 0.04

	// === 成本端（不变，定价端跟成本端独立）===
	ProCostEntries  []CostEntry `json:"proCostEntries,omitempty"`
	FreeCostEntries []CostEntry `json:"freeCostEntries,omitempty"`

	// === Token 计费路径（与 credit 路径并行；BillingMode="token" 才生效）===
	SellPrices map[string]ModelSellPrice `json:"sellPrices,omitempty"`

	// === Deprecated v1 字段（保留 JSON 兼容；启动时 MigratePricingToModelLevel 自动迁移）===
	FreePoolPriceUSD float64            `json:"freePoolPriceUSD,omitempty"` // 旧：FREE 池单价
	ProPoolPriceUSD  float64            `json:"proPoolPriceUSD,omitempty"`  // 旧：PRO 池单价
	ModelMultipliers map[string]float64 `json:"modelMultipliers,omitempty"` // 旧：模型乘数

	// 旧采购成本兜底字段（不动）
	PurchasePriceCNY      float64 `json:"purchasePriceCNY,omitempty"`
	ProAccountPriceCNY    float64 `json:"proAccountPriceCNY,omitempty"`
	ProAccountCredits     float64 `json:"proAccountCredits,omitempty"`
	FreeAccountBatchPrice float64 `json:"freeAccountBatchPrice,omitempty"`
	FreeAccountBatchCount int     `json:"freeAccountBatchCount,omitempty"`
	FreeAccountCredits    float64 `json:"freeAccountCredits,omitempty"`
}

// ProCostPerCredit returns the admin's cost per credit for PRO accounts (in CNY).
func (p PricingConfig) ProCostPerCredit() float64 {
	var totalCost float64
	var totalCredits float64
	for _, e := range p.ProCostEntries {
		totalCost += e.CostCNY
		totalCredits += float64(e.Count) * e.Credits
	}
	if totalCredits > 0 {
		return totalCost / totalCredits
	}
	// fallback to old fields
	if p.ProAccountCredits > 0 && p.ProAccountPriceCNY > 0 {
		return p.ProAccountPriceCNY / p.ProAccountCredits
	}
	if p.PurchasePriceCNY > 0 {
		return p.PurchasePriceCNY
	}
	return 0.04
}

// FreeCostPerCredit returns the admin's cost per credit for FREE accounts (in CNY).
func (p PricingConfig) FreeCostPerCredit() float64 {
	var totalCost float64
	var totalCredits float64
	for _, e := range p.FreeCostEntries {
		totalCost += e.CostCNY
		totalCredits += float64(e.Count) * FreeAccountCredits
	}
	if totalCredits > 0 {
		return totalCost / totalCredits
	}
	// fallback
	if p.FreeAccountBatchCount > 0 && p.FreeAccountCredits > 0 && p.FreeAccountBatchPrice > 0 {
		return p.FreeAccountBatchPrice / (float64(p.FreeAccountBatchCount) * p.FreeAccountCredits)
	}
	return 0.0002
}

// ProTotalCost returns total investment in PRO accounts (CNY).
func (p PricingConfig) ProTotalCost() float64 {
	var total float64
	for _, e := range p.ProCostEntries {
		total += e.CostCNY
	}
	return total
}

// FreeTotalCost returns total investment in FREE accounts (CNY).
func (p PricingConfig) FreeTotalCost() float64 {
	var total float64
	for _, e := range p.FreeCostEntries {
		total += e.CostCNY
	}
	return total
}

// GetPricing returns the pricing configuration with defaults.
//
// 迁移由 MaybeMigratePricing 在程序启动时显式触发（main.go），此处只做"读 + 兜底默认值"。
func GetPricing() PricingConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	p := cfg.Pricing
	// v2 默认值
	if p.DefaultProPriceUSD == 0 {
		p.DefaultProPriceUSD = 0.20
	}
	if p.DefaultFreePriceUSD == 0 {
		p.DefaultFreePriceUSD = 0.04
	}
	// v1 deprecated 默认值（仍然填，给报表/外部脚本兜底）
	if p.FreePoolPriceUSD == 0 {
		p.FreePoolPriceUSD = 0.40
	}
	if p.ProPoolPriceUSD == 0 {
		p.ProPoolPriceUSD = 2.00
	}
	return p
}

// MaybeMigratePricing 启动时显式调用一次（main.go 在 SetSupportedModels 之后）。
// 检测旧字段是否需要迁移到 v2 ModelPrices，迁移则持久化到磁盘。
//
// 返回 (migrated, err)：migrated=true 表示真的发生了迁移并写盘成功。
func MaybeMigratePricing() (bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	pricingMigrated := MigratePricingToModelLevel(&cfg.Pricing)
	promoMigrated := false
	if cfg.Promotion != nil {
		promoMigrated = MigratePromotionToModelLevel(cfg.Promotion)
	}
	if !pricingMigrated && !promoMigrated {
		return false, nil
	}
	if err := Save(); err != nil {
		return true, fmt.Errorf("save after migrate: %w", err)
	}
	return true, nil
}

// AddCostEntry adds a cost entry to PRO or FREE list.
func AddCostEntry(pool string, entry CostEntry) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if pool == "pro" {
		cfg.Pricing.ProCostEntries = append(cfg.Pricing.ProCostEntries, entry)
	} else {
		cfg.Pricing.FreeCostEntries = append(cfg.Pricing.FreeCostEntries, entry)
	}
	return Save()
}

// RemoveCostEntry removes a cost entry by ID.
func RemoveCostEntry(pool, id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if pool == "pro" {
		for i, e := range cfg.Pricing.ProCostEntries {
			if e.ID == id {
				cfg.Pricing.ProCostEntries = append(cfg.Pricing.ProCostEntries[:i], cfg.Pricing.ProCostEntries[i+1:]...)
				return Save()
			}
		}
	} else {
		for i, e := range cfg.Pricing.FreeCostEntries {
			if e.ID == id {
				cfg.Pricing.FreeCostEntries = append(cfg.Pricing.FreeCostEntries[:i], cfg.Pricing.FreeCostEntries[i+1:]...)
				return Save()
			}
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// Config represents the global application configuration.
type Config struct {
	// Server settings
	Password        string           `json:"password"`         // Admin panel password
	Port            int              `json:"port"`             // HTTP server port (default: 8080)
	Host            string           `json:"host"`             // HTTP server bind address (default: 0.0.0.0)
	ApiKey          string           `json:"apiKey,omitempty"` // Legacy single key (auto-migrated)
	RequireApiKey   bool             `json:"requireApiKey"`    // Whether to enforce API key validation
	ApiKeys         []ApiKeyInfo     `json:"apiKeys,omitempty"`
	Accounts        []Account        `json:"accounts"`
	ActivationCodes []ActivationCode `json:"activationCodes,omitempty"`
	Pricing         PricingConfig    `json:"pricing,omitempty"`
	Stealth         StealthConfig    `json:"stealth,omitempty"`
	Promotion       *PromotionConfig `json:"promotion,omitempty"`

	// Thinking mode configuration for extended reasoning output
	ThinkingSuffix       string `json:"thinkingSuffix,omitempty"`       // Model suffix to trigger thinking mode (default: "-thinking")
	OpenAIThinkingFormat string `json:"openaiThinkingFormat,omitempty"` // OpenAI output format: "reasoning_content", "thinking", or "think"
	ClaudeThinkingFormat string `json:"claudeThinkingFormat,omitempty"` // Claude output format: "reasoning_content", "thinking", or "think"

	// Endpoint configuration: "auto", "codewhisperer", or "amazonq"
	PreferredEndpoint string `json:"preferredEndpoint,omitempty"`

	// Concurrency limits (configurable from admin UI)
	MaxConcurrentPerKey       int `json:"maxConcurrentPerKey,omitempty"`       // Per API key max concurrent streams (default: 20)
	MaxInFlightPerAccount     int `json:"maxInFlightPerAccount,omitempty"`     // Legacy: unified per-account limit (kept for migration)
	MaxInFlightPerAccountFree int `json:"maxInFlightPerAccountFree,omitempty"` // Per FREE account max in-flight requests (default: 50)
	MaxInFlightPerAccountPro  int `json:"maxInFlightPerAccountPro,omitempty"`  // Per PRO account max in-flight requests (default: 50)

	// 天卡防共享速率限制：仅对 plan=timed/hybrid 且 ExpiresAt 未过期的 key 生效。
	// 0 = 走老兜底 200/min。默认 10。设值越低，N 人共享一张卡时人均配额越糟糕，自然劝退分发。
	TimedKeyRPM int `json:"timedKeyRPM,omitempty"`

	// 利润计算时是否把 admin 主动 gift 给 key 的总额计入 revenue。
	// false（默认）= 不计入；true = 计入"period 内 admin 触发的赠送总额"作为收入。
	ProfitIncludeGift bool `json:"profitIncludeGift,omitempty"`

	// Global statistics (persisted across restarts)
	TotalRequests   int     `json:"totalRequests,omitempty"`   // Total API requests received
	SuccessRequests int     `json:"successRequests,omitempty"` // Successful requests count
	FailedRequests  int     `json:"failedRequests,omitempty"`  // Failed requests count
	TotalTokens     int     `json:"totalTokens,omitempty"`     // Total tokens processed
	TotalCredits    float64 `json:"totalCredits,omitempty"`    // Total credits consumed

	// Leaderboard configuration
	LeaderboardEnabled   bool `json:"leaderboardEnabled,omitempty"`   // Whether to expose user-side leaderboard
	LeaderboardFakeUsers int  `json:"leaderboardFakeUsers,omitempty"` // Number of synthetic entries mixed into user-side top (0 = disabled, max 30)

	// === 渠道与计费模式（v3 新增；空值 = 走 legacy Kiro 路径，零破坏）===
	// Channels 为空时 ChannelRouter.HasChannels() = false，handler 走老路；
	// BillingMode="" 或 "legacy_credits" 走 credit 路径；"token" 走 SellPrices 路径。
	Channels    []ChannelConfig `json:"channels,omitempty"`
	BillingMode string          `json:"billingMode,omitempty"`
}

// GetLeaderboardConfig returns leaderboard settings (enabled, fakeUsers).
func GetLeaderboardConfig() (bool, int) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	n := cfg.LeaderboardFakeUsers
	if n < 0 {
		n = 0
	}
	if n > 30 {
		n = 30
	}
	return cfg.LeaderboardEnabled, n
}

// UpdateLeaderboardConfig updates leaderboard settings.
func UpdateLeaderboardConfig(enabled bool, fakeUsers int) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if fakeUsers < 0 {
		fakeUsers = 0
	}
	if fakeUsers > 30 {
		fakeUsers = 30
	}
	cfg.LeaderboardEnabled = enabled
	cfg.LeaderboardFakeUsers = fakeUsers
	return Save()
}

// ClearAllGiftBalances zeros GiftBalance on every key (does NOT touch Balance or TotalGifted).
// Returns (count, totalCleared).
func ClearAllGiftBalances() (int, float64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	count := 0
	var total float64
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].GiftBalance > 0 {
			total += cfg.ApiKeys[i].GiftBalance
			cfg.ApiKeys[i].GiftBalance = 0
			count++
		}
	}
	if count > 0 {
		_ = Save()
	}
	return count, total
}

// AccountInfo contains account metadata retrieved from Kiro API.
// Used for updating subscription and usage information.
type AccountInfo struct {
	Email             string
	UserId            string
	SubscriptionType  string
	SubscriptionTitle string
	DaysRemaining     int
	UsageCurrent      float64
	UsageLimit        float64
	UsagePercent      float64
	NextResetDate     string
	LastRefresh       int64
	TrialUsageCurrent float64
	TrialUsageLimit   float64
	TrialUsagePercent float64
	TrialStatus       string
	TrialExpiresAt    int64
}

// Version 当前版本号
const Version = "1.0.3"

var (
	cfg     *Config
	cfgLock sync.RWMutex
	cfgPath string
)

// Init initializes the configuration system with the specified file path.
// If the file doesn't exist, a default configuration is created.
func Init(path string) error {
	cfgPath = path
	return Load()
}

// GetDataDir returns the directory containing the config file (used for log persistence)
func GetDataDir() string {
	if cfgPath == "" {
		return "."
	}
	dir := cfgPath
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' || dir[i] == '\\' {
			return dir[:i]
		}
	}
	return "."
}

func Load() error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 默认密码也写 hash，不写明文 "changeme"
			defaultHash, hashErr := HashAdminPassword("changeme")
			if hashErr != nil {
				return hashErr
			}
			cfg = &Config{
				Password:      defaultHash,
				Port:          8080,
				Host:          "0.0.0.0",
				RequireApiKey: false,
				Accounts:      []Account{},
			}
			return Save()
		}
		return err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	// Backward-compatible migration: single ApiKey → ApiKeys[]
	migrated := false
	if len(c.ApiKeys) == 0 && c.ApiKey != "" {
		c.ApiKeys = []ApiKeyInfo{{
			ID: GenerateMachineId(), Key: c.ApiKey, Plan: "timed",
			ExpiresAt: 0, Enabled: true, Note: "migrated", CreatedAt: time.Now().Unix(),
		}}
		c.ApiKey = ""
		migrated = true
	}
	cfg = &c
	// 密码迁移失败不阻止启动 —— 否则备份/写盘失败会让管理员被锁在后台外面，
	// 反而比"暂时还是明文"更糟。verifyAdminPasswordHash 在迁移期支持明文兜底。
	if err := migrateAdminPasswordLocked(); err != nil {
		fmt.Printf("[config] WARN: admin password migration skipped: %v\n", err)
	}
	if migrated {
		return Save()
	}
	return nil
}

// Save persists the current configuration to the JSON file.
// Uses indented formatting for human readability.
func Save() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0600)
}

// SetPassword 接受明文密码（典型来自 ADMIN_PASSWORD 环境变量），hash 后写入内存。
// 不写盘（避免 env 覆盖回写到 config.json 造成混淆）。同时标记 envOverride=true，
// 之后 UI 的改密接口会返回 409 拒绝。
func SetPassword(password string) error {
	if password == "" {
		return fmt.Errorf("admin password cannot be empty")
	}
	hash, err := HashAdminPassword(password)
	if err != nil {
		return err
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Password = hash
	passwordEnvOverride = true
	return nil
}

// HashAdminPassword 用 argon2id 生成 PHC 格式的密码 hash。
func HashAdminPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	salt := make([]byte, adminPasswordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey(
		[]byte(password), salt,
		adminPasswordArgonTime, adminPasswordArgonMemory, adminPasswordArgonParallelism,
		adminPasswordKeyLen,
	)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		adminPasswordArgonMemory, adminPasswordArgonTime, adminPasswordArgonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyAdminPassword 验证明文密码与存储 hash 是否匹配。
// 支持 argon2id（推荐）/ bcrypt（兼容）/ 明文（迁移期最后兜底）。
func VerifyAdminPassword(password string) bool {
	cfgLock.RLock()
	stored := cfg.Password
	cfgLock.RUnlock()
	return verifyAdminPasswordHash(password, stored)
}

// ChangeAdminPassword 校验旧密码后写入新 hash。
// 调用前会检查 ADMIN_PASSWORD env override，若启用则直接拒绝。
func ChangeAdminPassword(oldPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if passwordEnvOverride {
		return fmt.Errorf("password managed by ADMIN_PASSWORD env")
	}
	if !verifyAdminPasswordHash(oldPassword, cfg.Password) {
		return ErrInvalidOldPassword
	}
	hash, err := HashAdminPassword(newPassword)
	if err != nil {
		return err
	}
	cfg.Password = hash
	return Save()
}

// IsSupportedPasswordHash 检测一个字符串是否是已知的 hash 格式。
func IsSupportedPasswordHash(s string) bool {
	return strings.HasPrefix(s, "$argon2id$") ||
		strings.HasPrefix(s, "$2a$") ||
		strings.HasPrefix(s, "$2b$") ||
		strings.HasPrefix(s, "$2y$")
}

// IsPasswordEnvOverride 报告当前密码是否被 ADMIN_PASSWORD env 覆盖。
func IsPasswordEnvOverride() bool {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return passwordEnvOverride
}

func verifyAdminPasswordHash(password, stored string) bool {
	if stored == "" {
		return false
	}
	switch {
	case strings.HasPrefix(stored, "$argon2id$"):
		return verifyArgon2IDPassword(password, stored)
	case strings.HasPrefix(stored, "$2a$"),
		strings.HasPrefix(stored, "$2b$"),
		strings.HasPrefix(stored, "$2y$"):
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	default:
		// 明文兜底（迁移期；migrateAdminPasswordLocked 失败时仍允许登录修复）
		return subtle.ConstantTimeCompare([]byte(password), []byte(stored)) == 1
	}
}

func verifyArgon2IDPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory == 0 || iterations == 0 || parallelism == 0 || parallelism > 255 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// migrateAdminPasswordLocked 把明文 cfg.Password 升级为 argon2id hash。
// 调用方必须已经持有 cfgLock.Lock()。
// 迁移前自动备份 config.json 到 config.json.bak_admin_password_<timestamp>。
// 迁移失败时回滚内存到原值（避免锁死自己）。
func migrateAdminPasswordLocked() error {
	if cfg == nil || cfg.Password == "" || IsSupportedPasswordHash(cfg.Password) {
		return nil
	}
	backupPath := fmt.Sprintf("%s.bak_admin_password_%s", cfgPath, time.Now().Format("20060102_150405"))
	if data, err := os.ReadFile(cfgPath); err == nil {
		if werr := os.WriteFile(backupPath, data, 0600); werr != nil {
			return fmt.Errorf("backup admin password config: %w", werr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config for admin password backup: %w", err)
	}
	original := cfg.Password
	hash, err := HashAdminPassword(original)
	if err != nil {
		return err
	}
	cfg.Password = hash
	if err := Save(); err != nil {
		cfg.Password = original
		return fmt.Errorf("save migrated admin password failed (backup=%s): %w", backupPath, err)
	}
	fmt.Printf("[config] admin password migrated to argon2id (backup=%s)\n", backupPath)
	return nil
}

func Get() *Config {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg
}

func GetPassword() string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.Password
}

func GetPort() int {
	// 优先使用环境变量
	if portStr := os.Getenv("PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			return port
		}
	}

	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg.Port == 0 {
		return 8080
	}
	return cfg.Port
}

func GetHost() string {
	// 优先使用环境变量
	if host := os.Getenv("HOST"); host != "" {
		return host
	}

	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg.Host == "" {
		return "127.0.0.1"
	}
	return cfg.Host
}

func GetAccounts() []Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	accounts := make([]Account, len(cfg.Accounts))
	copy(accounts, cfg.Accounts)
	return accounts
}

func GetEnabledAccounts() []Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	var accounts []Account
	for _, a := range cfg.Accounts {
		if a.Enabled {
			accounts = append(accounts, a)
		}
	}
	return accounts
}

// ==================== API Key CRUD ====================

func FindApiKey(key string) *ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for _, k := range cfg.ApiKeys {
		if k.Key == key {
			c := k
			if c.Models != nil {
				c.Models = copyModelCounts(c.Models)
			}
			return &c
		}
	}
	return nil
}

func GetAllApiKeys() []ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	keys := make([]ApiKeyInfo, len(cfg.ApiKeys))
	for i, k := range cfg.ApiKeys {
		keys[i] = k
		if k.Models != nil {
			keys[i].Models = copyModelCounts(k.Models)
		}
	}
	return keys
}

func AddApiKey(key ApiKeyInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ApiKeys = append(cfg.ApiKeys, key)
	return Save()
}

func DeleteApiKey(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys = append(cfg.ApiKeys[:i], cfg.ApiKeys[i+1:]...)
			return Save()
		}
	}
	return nil
}

func UpdateApiKey(id string, key ApiKeyInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys[i] = key
			return Save()
		}
	}
	return nil
}

func UpdateApiKeyStatsNoSave(id string, lastUsed, requests, errors, tokens int64, credits float64, models map[string]int64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == id {
			cfg.ApiKeys[i].LastUsed = lastUsed
			cfg.ApiKeys[i].Requests = requests
			cfg.ApiKeys[i].Errors = errors
			cfg.ApiKeys[i].Tokens = tokens
			cfg.ApiKeys[i].Credits = credits
			if models != nil {
				cfg.ApiKeys[i].Models = copyModelCounts(models)
			}
			return
		}
	}
}

func GenerateApiKeyString() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func copyModelCounts(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// FindAccountByEmail returns the index of an account with matching email, or -1.
// Must be called with cfgLock held.
func findAccountByEmailLocked(email string) int {
	if email == "" {
		return -1
	}
	emailLower := strings.ToLower(strings.TrimSpace(email))
	for i, a := range cfg.Accounts {
		if strings.ToLower(strings.TrimSpace(a.Email)) == emailLower {
			return i
		}
	}
	return -1
}

func normalizeIdentityKey(email, authMethod, provider string) string {
	emailLower := strings.ToLower(strings.TrimSpace(email))
	if emailLower == "" {
		return ""
	}
	providerLower := strings.ToLower(strings.TrimSpace(provider))
	authLower := strings.ToLower(strings.TrimSpace(authMethod))
	if authLower == "" && providerLower != "" {
		authLower = "social"
	}
	if authLower == "social" || authLower == "google" || authLower == "github" {
		if providerLower != "" {
			return "social|" + emailLower + "|" + providerLower
		}
		return "social|" + emailLower
	}
	return "idc|" + emailLower
}

func findAccountByIdentityLocked(email, authMethod, provider string) int {
	key := normalizeIdentityKey(email, authMethod, provider)
	if key == "" {
		return -1
	}
	for i, a := range cfg.Accounts {
		if normalizeIdentityKey(a.Email, a.AuthMethod, a.Provider) == key {
			return i
		}
	}
	return -1
}

func FindAccountByEmail(email string) *Account {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	idx := findAccountByEmailLocked(email)
	if idx < 0 {
		return nil
	}
	a := cfg.Accounts[idx]
	return &a
}

func AddAccount(account Account) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if idx := findAccountByIdentityLocked(account.Email, account.AuthMethod, account.Provider); idx >= 0 {
		return fmt.Errorf("duplicate: account with same identity already exists (id: %s)", cfg.Accounts[idx].ID)
	}
	cfg.Accounts = append(cfg.Accounts, account)
	return Save()
}

// AddOrUpdateAccount adds a new account, or updates credentials if one with the same identity exists.
// Identity rule:
//   - social accounts: email + provider
//   - non-social accounts: email
//
// Returns (accountID, isNew, error).
func AddOrUpdateAccount(account Account) (string, bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if idx := findAccountByIdentityLocked(account.Email, account.AuthMethod, account.Provider); idx >= 0 {
		existing := &cfg.Accounts[idx]
		if account.AccessToken != "" {
			existing.AccessToken = account.AccessToken
		}
		if account.RefreshToken != "" {
			existing.RefreshToken = account.RefreshToken
		}
		if account.ClientID != "" {
			existing.ClientID = account.ClientID
		}
		if account.ClientSecret != "" {
			existing.ClientSecret = account.ClientSecret
		}
		if account.ExpiresAt > 0 {
			existing.ExpiresAt = account.ExpiresAt
		}
		if account.AuthMethod != "" {
			existing.AuthMethod = account.AuthMethod
		}
		if account.Provider != "" {
			existing.Provider = account.Provider
		}
		existing.Enabled = true
		if existing.BanStatus != "" && existing.BanStatus != "ACTIVE" {
			existing.BanStatus = "ACTIVE"
			existing.BanReason = ""
			existing.BanTime = 0
		}
		return existing.ID, false, Save()
	}
	cfg.Accounts = append(cfg.Accounts, account)
	return account.ID, true, Save()
}

func UpdateAccount(id string, account Account) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i] = account
			return Save()
		}
	}
	return nil
}

// UpdateAccountBanStatus 只更新封禁相关字段，不覆盖 token
// 避免用旧副本覆盖刚刷新的 refreshToken
func UpdateAccountBanStatus(id string, enabled bool, banStatus, banReason string, banTime int64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].Enabled = enabled
			cfg.Accounts[i].BanStatus = banStatus
			cfg.Accounts[i].BanReason = banReason
			cfg.Accounts[i].BanTime = banTime
			return Save()
		}
	}
	return nil
}

func DeleteAccount(id string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts = append(cfg.Accounts[:i], cfg.Accounts[i+1:]...)
			return Save()
		}
	}
	return nil
}

// ImportAccounts imports multiple accounts from a JSON array.
// This function is useful for batch importing accounts from external tools like KAM.
// Duplicate accounts (same ID) are skipped with a warning.
func ImportAccounts(accounts []Account) (int, int, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	imported := 0
	skipped := 0
	existingIDs := make(map[string]bool)
	existingIdentities := make(map[string]bool)

	// Build map of existing account IDs and identities
	for _, a := range cfg.Accounts {
		existingIDs[a.ID] = true
		if key := normalizeIdentityKey(a.Email, a.AuthMethod, a.Provider); key != "" {
			existingIdentities[key] = true
		}
	}

	// Import new accounts (skip duplicates by ID or identity)
	for _, account := range accounts {
		if existingIDs[account.ID] {
			skipped++
			continue
		}
		if key := normalizeIdentityKey(account.Email, account.AuthMethod, account.Provider); key != "" && existingIdentities[key] {
			skipped++
			continue
		}

		// Generate machine ID if not present
		if account.MachineId == "" {
			account.MachineId = GenerateMachineId()
		}

		cfg.Accounts = append(cfg.Accounts, account)
		existingIDs[account.ID] = true
		if key := normalizeIdentityKey(account.Email, account.AuthMethod, account.Provider); key != "" {
			existingIdentities[key] = true
		}
		imported++
	}

	if imported > 0 {
		if err := Save(); err != nil {
			return imported, skipped, err
		}
	}

	return imported, skipped, nil
}

func UpdateAccountToken(id, accessToken, refreshToken string, expiresAt int64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].AccessToken = accessToken
			if refreshToken != "" {
				cfg.Accounts[i].RefreshToken = refreshToken
			}
			cfg.Accounts[i].ExpiresAt = expiresAt
			return Save()
		}
	}
	return nil
}

func GetApiKey() string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.ApiKey
}

func IsApiKeyRequired() bool {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.RequireApiKey
}

// UpdateSettings 更新非密码类设置。
// 改 admin 密码请走 ChangeAdminPassword（带旧密码校验 + argon2id hash）。
func UpdateSettings(apiKey string, requireApiKey bool) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ApiKey = apiKey
	cfg.RequireApiKey = requireApiKey
	return Save()
}

// GetConcurrencyConfig returns (maxPerKey, maxPerAccountFree, maxPerAccountPro) with safe defaults.
func GetConcurrencyConfig() (int, int, int) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	perKey := cfg.MaxConcurrentPerKey
	if perKey <= 0 {
		perKey = 20
	}
	// Migration: if new fields are 0 but old field has value, use old field
	fallback := cfg.MaxInFlightPerAccount
	if fallback <= 0 {
		fallback = 50
	}
	perFree := cfg.MaxInFlightPerAccountFree
	if perFree <= 0 {
		perFree = fallback
	}
	perPro := cfg.MaxInFlightPerAccountPro
	if perPro <= 0 {
		perPro = fallback
	}
	return perKey, perFree, perPro
}

// UpdateConcurrencyConfig updates concurrency limits and persists to disk.
func UpdateConcurrencyConfig(perKey, perAccountFree, perAccountPro int) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if perKey > 0 {
		cfg.MaxConcurrentPerKey = perKey
	}
	if perAccountFree > 0 {
		cfg.MaxInFlightPerAccountFree = perAccountFree
	}
	if perAccountPro > 0 {
		cfg.MaxInFlightPerAccountPro = perAccountPro
	}
	return Save()
}

// GetTimedKeyRPM 返回天卡 key 的全局每分钟请求上限。
// 0 = 走老兜底 200/min；admin 没设过返 0（不强加默认 10，让 abuse.go 决定 fallback）。
func GetTimedKeyRPM() int {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.TimedKeyRPM
}

// UpdateTimedKeyRPM 持久化天卡全局 RPM。<= 0 视为禁用（走老兜底）。
func UpdateTimedKeyRPM(rpm int) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if rpm < 0 {
		rpm = 0
	}
	cfg.TimedKeyRPM = rpm
	return Save()
}

// GetProfitIncludeGift 返回利润计算是否计入赠送余额。
func GetProfitIncludeGift() bool {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.ProfitIncludeGift
}

// UpdateProfitIncludeGift 持久化偏好。
func UpdateProfitIncludeGift(v bool) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ProfitIncludeGift = v
	return Save()
}

func UpdateStats(totalReq, successReq, failedReq, totalTokens int, totalCredits float64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.TotalRequests = totalReq
	cfg.SuccessRequests = successReq
	cfg.FailedRequests = failedReq
	cfg.TotalTokens = totalTokens
	cfg.TotalCredits = totalCredits
	return Save()
}

func GetStats() (int, int, int, int, float64) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.TotalRequests, cfg.SuccessRequests, cfg.FailedRequests, cfg.TotalTokens, cfg.TotalCredits
}

func UpdateAccountStats(id string, requestCount, errorCount, totalTokens int, totalCredits float64, lastUsed int64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].RequestCount = requestCount
			cfg.Accounts[i].ErrorCount = errorCount
			cfg.Accounts[i].TotalTokens = totalTokens
			cfg.Accounts[i].TotalCredits = totalCredits
			cfg.Accounts[i].LastUsed = lastUsed
			return Save()
		}
	}
	return nil
}

// UpdateAccountStatsNoSave 更新账号统计但不写盘（用于批量刷新）
func UpdateAccountStatsNoSave(id string, requestCount, errorCount, totalTokens int, totalCredits float64, lastUsed int64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			cfg.Accounts[i].RequestCount = requestCount
			cfg.Accounts[i].ErrorCount = errorCount
			cfg.Accounts[i].TotalTokens = totalTokens
			cfg.Accounts[i].TotalCredits = totalCredits
			cfg.Accounts[i].LastUsed = lastUsed
			return
		}
	}
}

// SaveConfig 显式保存配置到磁盘（用于批量写入后的统一保存）
func SaveConfig() error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return Save()
}

// UpdateAccountInfo updates an account's subscription and usage information.
// Called after refreshing account data from Kiro API.
func UpdateAccountInfo(id string, info AccountInfo) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, a := range cfg.Accounts {
		if a.ID == id {
			if info.Email != "" {
				cfg.Accounts[i].Email = info.Email
			}
			if info.UserId != "" {
				cfg.Accounts[i].UserId = info.UserId
			}
			cfg.Accounts[i].SubscriptionType = info.SubscriptionType
			cfg.Accounts[i].SubscriptionTitle = info.SubscriptionTitle
			cfg.Accounts[i].DaysRemaining = info.DaysRemaining
			cfg.Accounts[i].UsageCurrent = info.UsageCurrent
			cfg.Accounts[i].UsageLimit = info.UsageLimit
			cfg.Accounts[i].UsagePercent = info.UsagePercent
			cfg.Accounts[i].NextResetDate = info.NextResetDate
			cfg.Accounts[i].LastRefresh = info.LastRefresh
			cfg.Accounts[i].TrialUsageCurrent = info.TrialUsageCurrent
			cfg.Accounts[i].TrialUsageLimit = info.TrialUsageLimit
			cfg.Accounts[i].TrialUsagePercent = info.TrialUsagePercent
			cfg.Accounts[i].TrialStatus = info.TrialStatus
			cfg.Accounts[i].TrialExpiresAt = info.TrialExpiresAt
			return Save()
		}
	}
	return nil
}

// ThinkingConfig holds settings for AI thinking/reasoning mode.
// When enabled, models output their reasoning process alongside the response.
type ThinkingConfig struct {
	Suffix       string `json:"suffix"`       // Model name suffix that triggers thinking mode
	OpenAIFormat string `json:"openaiFormat"` // Output format for OpenAI-compatible responses
	ClaudeFormat string `json:"claudeFormat"` // Output format for Claude-compatible responses
}

// GetThinkingConfig 获取 thinking 配置
func GetThinkingConfig() ThinkingConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()

	suffix := cfg.ThinkingSuffix
	if suffix == "" {
		suffix = "-thinking"
	}
	openaiFormat := cfg.OpenAIThinkingFormat
	if openaiFormat == "" {
		openaiFormat = "reasoning_content"
	}
	claudeFormat := cfg.ClaudeThinkingFormat
	if claudeFormat == "" {
		claudeFormat = "thinking"
	}

	return ThinkingConfig{
		Suffix:       suffix,
		OpenAIFormat: openaiFormat,
		ClaudeFormat: claudeFormat,
	}
}

// UpdateThinkingConfig 更新 thinking 配置
func UpdateThinkingConfig(suffix, openaiFormat, claudeFormat string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ThinkingSuffix = suffix
	cfg.OpenAIThinkingFormat = openaiFormat
	cfg.ClaudeThinkingFormat = claudeFormat
	return Save()
}

// GetPreferredEndpoint 获取首选端点配置
func GetPreferredEndpoint() string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg.PreferredEndpoint == "" {
		return "auto"
	}
	return cfg.PreferredEndpoint
}

// UpdatePreferredEndpoint 更新首选端点配置
func UpdatePreferredEndpoint(endpoint string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.PreferredEndpoint = endpoint
	return Save()
}

// ==================== Promotion ====================

// GetPromotion 返回当前活动配置（线程安全副本）。未配置则返回 nil。
func GetPromotion() *PromotionConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg.Promotion == nil {
		return nil
	}
	// 返回副本（含白名单深拷贝）
	cp := *cfg.Promotion
	if len(cfg.Promotion.Whitelist) > 0 {
		cp.Whitelist = make([]string, len(cfg.Promotion.Whitelist))
		copy(cp.Whitelist, cfg.Promotion.Whitelist)
	}
	return &cp
}

// UpdatePromotion 更新活动配置（传 nil 视为关闭）。
func UpdatePromotion(p *PromotionConfig, operator string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if p != nil {
		p.UpdatedAt = time.Now().Unix()
		p.UpdatedBy = operator
		// 默认窗口
		if p.RecentCallsDays <= 0 {
			p.RecentCallsDays = 7
		}
	}
	cfg.Promotion = p
	return Save()
}

// AddPromotionWhitelist 把 keyID 加入白名单（去重）。
func AddPromotionWhitelist(keyID, operator string) error {
	if keyID == "" {
		return fmt.Errorf("keyID required")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if cfg.Promotion == nil {
		cfg.Promotion = &PromotionConfig{Enabled: false, RecentCallsDays: 7}
	}
	for _, k := range cfg.Promotion.Whitelist {
		if k == keyID {
			return nil // 已在
		}
	}
	cfg.Promotion.Whitelist = append(cfg.Promotion.Whitelist, keyID)
	cfg.Promotion.UpdatedAt = time.Now().Unix()
	cfg.Promotion.UpdatedBy = operator
	return Save()
}

// RemovePromotionWhitelist 把 keyID 从白名单移除。
func RemovePromotionWhitelist(keyID, operator string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if cfg.Promotion == nil {
		return nil
	}
	out := cfg.Promotion.Whitelist[:0]
	for _, k := range cfg.Promotion.Whitelist {
		if k != keyID {
			out = append(out, k)
		}
	}
	cfg.Promotion.Whitelist = out
	cfg.Promotion.UpdatedAt = time.Now().Unix()
	cfg.Promotion.UpdatedBy = operator
	return Save()
}

// ==================== Pricing ====================

// UpdatePricing updates the pricing configuration.
func UpdatePricing(p PricingConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Pricing = p
	return Save()
}

// ==================== ApiKey Billing ====================

// FindApiKeyByID returns a pointer to ApiKeyInfo by ID.
func FindApiKeyByID(id string) *ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for _, k := range cfg.ApiKeys {
		if k.ID == id {
			c := k
			if c.Models != nil {
				c.Models = copyModelCounts(c.Models)
			}
			return &c
		}
	}
	return nil
}

// DeductKeyBalance atomically deducts amount from an API key's balance.
// It prioritizes burning `Balance` (paid) first. If insufficient, it burns `GiftBalance`.
// Returns (success, remainingTotalBalance, paidAmountDeducted, giftedAmountDeducted).
func DeductKeyBalance(keyID string, amount float64) (bool, float64, float64, float64) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			totalBalance := cfg.ApiKeys[i].Balance + cfg.ApiKeys[i].GiftBalance
			if totalBalance < amount {
				return false, totalBalance, 0, 0
			}

			var paidDeducted, giftedDeducted float64

			// 1. Deduct from true Paid Balance first
			if cfg.ApiKeys[i].Balance >= amount {
				cfg.ApiKeys[i].Balance -= amount
				paidDeducted = amount
			} else {
				// Paid balance completely exhausted by this deduction
				paidDeducted = cfg.ApiKeys[i].Balance
				remainingAmount := amount - paidDeducted
				cfg.ApiKeys[i].Balance = 0

				// 2. Fallback to GiftBalance
				cfg.ApiKeys[i].GiftBalance -= remainingAmount
				giftedDeducted = remainingAmount
			}

			remainingTotal := cfg.ApiKeys[i].Balance + cfg.ApiKeys[i].GiftBalance
			Save()
			return true, remainingTotal, paidDeducted, giftedDeducted
		}
	}
	return false, 0, 0, 0
}

// AddKeyBalance adds paid balance to an API key. Reverses a deduction (used by RefundPreAuth).
func AddKeyBalance(keyID string, paidAmount, giftAmount float64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			if paidAmount != 0 {
				cfg.ApiKeys[i].Balance += paidAmount
			}
			if giftAmount != 0 {
				cfg.ApiKeys[i].GiftBalance += giftAmount
			}
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// SetKeyBalances specifically sets both balance fields (used by admin panel).
func SetKeyBalances(keyID string, paidBalance float64, giftBalance float64) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			// Track cumulative gifted amount (only increases)
			if giftBalance > cfg.ApiKeys[i].GiftBalance {
				cfg.ApiKeys[i].TotalGifted += giftBalance - cfg.ApiKeys[i].GiftBalance
			}
			cfg.ApiKeys[i].Balance = paidBalance
			cfg.ApiKeys[i].GiftBalance = giftBalance
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// ExtendKeyExpiry extends expiration by N days. If current expiry is past, extends from now.
func ExtendKeyExpiry(keyID string, days int) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, k := range cfg.ApiKeys {
		if k.ID == keyID {
			base := cfg.ApiKeys[i].ExpiresAt
			now := time.Now().Unix()
			if base < now {
				base = now
			}
			cfg.ApiKeys[i].ExpiresAt = base + int64(days)*86400
			return Save()
		}
	}
	return fmt.Errorf("api key not found: %s", keyID)
}

// ValidateKeyAccess checks if an API key has any active plan or balance.
// This is the initial gate check — model-level access is checked by ValidateKeyAccessForModel.
func ValidateKeyAccess(info *ApiKeyInfo) (string, error) {
	if !info.Enabled {
		return "key_disabled", fmt.Errorf("api key is disabled")
	}
	now := time.Now().Unix()
	hasDayCard := (info.Plan == "timed" || info.Plan == "hybrid") && (info.ExpiresAt == 0 || now <= info.ExpiresAt)
	hasBalance := info.Balance > 0 || info.GiftBalance > 0
	hasCreditPlan := info.Plan == "credit"

	// 如果没有 Plan 但有余额（赠送或付费），则视为隐式 credit 计划
	// 用户不需要激活码即可使用管理员赠送的余额
	if info.Plan == "" && hasBalance {
		hasCreditPlan = true
	}

	if !hasDayCard && !hasBalance && !hasCreditPlan {
		if info.Plan == "" {
			return "not_activated", fmt.Errorf("api key not activated, please redeem an activation code")
		}
		return "key_expired", fmt.Errorf("api key expired and insufficient balance")
	}
	return "", nil
}

// ValidateKeyAccessForModel checks if a key can access a model in the given pool.
// Returns action: "free" (no charge), "deduct" (charge balance), or error.
//
// v3.5 简化（2026-05-09）：取消"free 天卡 vs pro 天卡"区分。
//   - 任何天卡（plan=timed/hybrid 且未过期）覆盖**所有**模型（free + pro 池），不扣费
//   - 没天卡但有余额 → deduct
//   - 都没有 → 错误
//
// 历史背景：早期 ApiKeyInfo.Tier ("free"/"pro") 用于限制 free 天卡只能调 sonnet-4.5，
// 防止低价天卡用户调高成本 PRO 模型。但 admin UI 后来就不暴露 tier 选择了，
// 字段成了悬空逻辑（旧 key 残留 tier="free" 反而把用户卡住）。
// 想限制成本走 RateLimitPerMin（速率限制），不走 tier 区分。
// 字段保留兼容（不读不写）。
func ValidateKeyAccessForModel(info *ApiKeyInfo, modelPool string) (string, error) {
	if info == nil || !info.Enabled {
		return "", fmt.Errorf("api key disabled")
	}
	now := time.Now().Unix()
	hasDayCard := (info.Plan == "timed" || info.Plan == "hybrid") && (info.ExpiresAt == 0 || now <= info.ExpiresAt)
	hasBalance := info.Balance > 0 || info.GiftBalance > 0

	// 任何天卡覆盖所有模型，不扣费
	if hasDayCard {
		return "free", nil
	}
	// 余额按需扣（free / pro 池都从同一个 balance 扣）
	if hasBalance {
		return "deduct", nil
	}
	// modelPool 参数保留是为了不破坏调用方签名（未来如要区分计费规则可重新启用）
	_ = modelPool
	return "", fmt.Errorf("api key has no active day-card and no balance")
}

// ==================== Activation Codes ====================

// GetActivationCodes returns all activation codes.
func GetActivationCodes() []ActivationCode {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	codes := make([]ActivationCode, len(cfg.ActivationCodes))
	copy(codes, cfg.ActivationCodes)
	return codes
}

// AddActivationCode adds a new activation code.
func AddActivationCode(code ActivationCode) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ActivationCodes = append(cfg.ActivationCodes, code)
	return Save()
}

// RedeemActivationCode tries to redeem a code for the given ApiKey ID.
func RedeemActivationCode(codeStr, keyID string) (string, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, ac := range cfg.ActivationCodes {
		if ac.Code == codeStr {
			if ac.Used {
				return "", fmt.Errorf("activation code already used")
			}
			if ac.CodeExpiresAt > 0 && time.Now().Unix() > ac.CodeExpiresAt {
				return "", fmt.Errorf("activation code has expired")
			}

			// Process balance/time addition before deleting
			switch ac.Type {
			case "balance":
				for j, k := range cfg.ApiKeys {
					if k.ID == keyID {
						// 激活码面值即 balance，无任何系统层杠杆。
						// admin 想给 reseller 让利？出卡时手算面值（如客户付 ¥200，admin 给 ¥285 面值卡）。
						amountUSD := ac.Amount / CNYPerUSDFace
						cfg.ApiKeys[j].Balance += amountUSD
						cfg.ApiKeys[j].TotalRecharged += amountUSD
						// Set plan: if already timed → hybrid, otherwise credit
						if cfg.ApiKeys[j].Plan == "timed" || cfg.ApiKeys[j].Plan == "hybrid" {
							cfg.ApiKeys[j].Plan = "hybrid"
						} else {
							cfg.ApiKeys[j].Plan = "credit"
						}
						break
					}
				}
			case "days", "time":
				for j, k := range cfg.ApiKeys {
					if k.ID == keyID {
						base := cfg.ApiKeys[j].ExpiresAt
						now := time.Now().Unix()
						if base < now {
							base = now
						}
						// 单位区分（CodeManagement.vue 行为）：
						//   type=days → ac.Amount 是"天数"，需要 ×86400 转秒
						//   type=time → ac.Amount 已经是"秒"（前端把 天/时/分 折算后送过来），直接加
						// 历史 BUG：曾 days/time 共用 +amount → 30天卡只加30秒；
						// 后修成统一 ×86400 → 反过来 1天卡变 86400天。
						// 回归测试见 TestRedeemActivationCode_DaysAddsCorrectSeconds 与
						// TestRedeemActivationCode_TimeUsesSecondsDirectly。
						var deltaSec int64
						if ac.Type == "days" {
							deltaSec = int64(ac.Amount) * 86400
						} else { // "time"
							deltaSec = int64(ac.Amount)
						}
						cfg.ApiKeys[j].ExpiresAt = base + deltaSec
						// Set plan and tier
						if cfg.ApiKeys[j].Plan == "credit" || cfg.ApiKeys[j].Plan == "hybrid" {
							cfg.ApiKeys[j].Plan = "hybrid"
						} else {
							cfg.ApiKeys[j].Plan = "timed"
						}
						if ac.Tier != "" {
							cfg.ApiKeys[j].Tier = ac.Tier
						}
						break
					}
				}
			default:
				return "", fmt.Errorf("unknown activation code type: %s", ac.Type)
			}

			// Delete the code permanently instead of marking it used
			cfg.ActivationCodes = append(cfg.ActivationCodes[:i], cfg.ActivationCodes[i+1:]...)

			Save()
			return ac.Type, nil
		}
	}
	return "", fmt.Errorf("activation code not found")
}

// DeleteActivationCode deletes an activation code by its code string.
func DeleteActivationCode(codeStr string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	for i, ac := range cfg.ActivationCodes {
		if ac.Code == codeStr {
			cfg.ActivationCodes = append(cfg.ActivationCodes[:i], cfg.ActivationCodes[i+1:]...)
			return Save()
		}
	}
	return nil
}

// ==================== Reseller (代理体系) ====================

// IsResellerKey 判断是不是开通了代理的 key
func (i *ApiKeyInfo) IsResellerKey() bool {
	return i != nil && i.IsReseller
}

// IsChildKey 判断是不是某 reseller 的子 key
func (i *ApiKeyInfo) IsChildKey() bool {
	return i != nil && i.ParentKeyID != ""
}

// GetChildKeys 返回某 reseller 的所有子 key（深拷贝）
func GetChildKeys(parentKeyID string) []ApiKeyInfo {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	var children []ApiKeyInfo
	for _, k := range cfg.ApiKeys {
		if k.ParentKeyID == parentKeyID {
			c := k
			if c.Models != nil {
				c.Models = copyModelCounts(c.Models)
			}
			children = append(children, c)
		}
	}
	return children
}

// TransferBalance 原子操作：reseller→child 转账
//   - 校验 to.ParentKeyID == fromKeyID（防横向越权）
//   - 校验 from.Balance >= amountUSD
//   - 一次写盘
// TransferBalance 在 reseller(parent) 与 child key 之间转账。
//
// amountUSD 语义（双向）：
//   - amountUSD > 0: parent → child（充入；同步 parent.SoldToChildren += amount, child.TotalRecharged += amount）
//   - amountUSD < 0: child → parent（扣回；同步 parent.SoldToChildren -= |amount|, child.TotalRecharged -= |amount|，最少为 0 不到负）
//   - amountUSD == 0: 拒绝
//
// fromKeyID 始终是 reseller(parent) ID（不是资金来源方向）；toKeyID 始终是 child ID。
// 负数方向由后端语义决定，调用方不需要倒置参数。
func TransferBalance(fromKeyID, toKeyID string, amountUSD float64) error {
	if amountUSD == 0 {
		return fmt.Errorf("amount must be non-zero")
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	var fromIdx, toIdx int = -1, -1
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == fromKeyID {
			fromIdx = i
		}
		if cfg.ApiKeys[i].ID == toKeyID {
			toIdx = i
		}
	}
	if fromIdx < 0 {
		return fmt.Errorf("source key not found")
	}
	if toIdx < 0 {
		return fmt.Errorf("target key not found")
	}
	if cfg.ApiKeys[toIdx].ParentKeyID != fromKeyID {
		return fmt.Errorf("not your child key") // 横向越权拦截
	}

	if amountUSD > 0 {
		// 充入：parent → child
		if cfg.ApiKeys[fromIdx].Balance < amountUSD {
			return fmt.Errorf("insufficient balance")
		}
		cfg.ApiKeys[fromIdx].Balance -= amountUSD
		cfg.ApiKeys[fromIdx].SoldToChildren += amountUSD
		cfg.ApiKeys[toIdx].Balance += amountUSD
		cfg.ApiKeys[toIdx].TotalRecharged += amountUSD
	} else {
		// 扣回：child → parent；amount = -amountUSD（正数）
		recall := -amountUSD
		if cfg.ApiKeys[toIdx].Balance < recall {
			return fmt.Errorf("child balance insufficient for recall")
		}
		cfg.ApiKeys[toIdx].Balance -= recall
		cfg.ApiKeys[fromIdx].Balance += recall
		// 修正历史统计（不到负）
		if cfg.ApiKeys[fromIdx].SoldToChildren >= recall {
			cfg.ApiKeys[fromIdx].SoldToChildren -= recall
		} else {
			cfg.ApiKeys[fromIdx].SoldToChildren = 0
		}
		if cfg.ApiKeys[toIdx].TotalRecharged >= recall {
			cfg.ApiKeys[toIdx].TotalRecharged -= recall
		} else {
			cfg.ApiKeys[toIdx].TotalRecharged = 0
		}
	}
	return Save()
}

// RefundChildBalance 删除子 key 时把它剩余余额（Balance + GiftBalance）退回 reseller 的 Balance。
// 返回退还的总金额（USD）。
func RefundChildBalance(childKeyID string) (float64, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	var childIdx, parentIdx int = -1, -1
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == childKeyID {
			childIdx = i
		}
	}
	if childIdx < 0 {
		return 0, fmt.Errorf("child not found")
	}
	parentID := cfg.ApiKeys[childIdx].ParentKeyID
	for i := range cfg.ApiKeys {
		if cfg.ApiKeys[i].ID == parentID {
			parentIdx = i
		}
	}
	refund := cfg.ApiKeys[childIdx].Balance + cfg.ApiKeys[childIdx].GiftBalance
	if parentIdx >= 0 && refund > 0 {
		cfg.ApiKeys[parentIdx].Balance += refund
		// 修正"已销售"统计（避免负数）
		cfg.ApiKeys[parentIdx].SoldToChildren -= refund
		if cfg.ApiKeys[parentIdx].SoldToChildren < 0 {
			cfg.ApiKeys[parentIdx].SoldToChildren = 0
		}
	}
	cfg.ApiKeys[childIdx].Balance = 0
	cfg.ApiKeys[childIdx].GiftBalance = 0
	return refund, Save()
}

// ==================== Channels & BillingMode (v3) ====================

// GetChannels 返回已配置渠道的线程安全副本。
func GetChannels() []ChannelConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	out := make([]ChannelConfig, len(cfg.Channels))
	for i, c := range cfg.Channels {
		cp := c
		if len(c.Models) > 0 {
			cp.Models = append([]string{}, c.Models...)
		}
		out[i] = cp
	}
	return out
}

// UpdateChannels 替换渠道列表（admin 接口用）。
func UpdateChannels(channels []ChannelConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Channels = channels
	return Save()
}

// GetBillingMode 返回当前计费模式（""/"legacy_credits"=旧路径，"token"=新路径）。
func GetBillingMode() string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.BillingMode
}

// UpdateBillingMode 切换计费模式。
func UpdateBillingMode(mode string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.BillingMode = mode
	return Save()
}

// GetSellPrice 查全局售价（兜底用）。优先用 GetSellPriceForChannel。
func GetSellPrice(model string) (ModelSellPrice, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return lookupSellPriceLocked(cfg.Pricing.SellPrices, model)
}

// GetSellPriceForChannel 渠道感知的售价查找。
//
// 查询优先级：
//  1. channel.ModelPrices[model]  ← 渠道独立定价（同 model 不同渠道可不同价）
//  2. pricing.SellPrices[model]   ← 全局兜底
//  3. ok=false → ErrSellPriceMissing（fail closed，防漏扣）
//
// channelID 为空时只查全局；用于 legacy path 或孤儿调用。
func GetSellPriceForChannel(channelID, model string) (ModelSellPrice, bool) {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if channelID != "" {
		for _, ch := range cfg.Channels {
			if ch.ID != channelID {
				continue
			}
			if price, ok := lookupSellPriceLocked(ch.ModelPrices, model); ok {
				return price, true
			}
			break
		}
	}
	return lookupSellPriceLocked(cfg.Pricing.SellPrices, model)
}

// lookupSellPriceLocked 在给定 map 里查 model 单价（'-/.' 互换、大小写不敏感、剥离 thinking 后缀）。
// 调用方必须持有 cfgLock。
func lookupSellPriceLocked(m map[string]ModelSellPrice, model string) (ModelSellPrice, bool) {
	if len(m) == 0 || model == "" {
		return ModelSellPrice{}, false
	}
	low := strings.ToLower(strings.TrimSpace(model))
	if v, ok := m[low]; ok {
		return v, true
	}
	target := normalizeSellPriceKey(low)
	for k, v := range m {
		if normalizeSellPriceKey(strings.ToLower(k)) == target {
			return v, true
		}
	}
	stripped := strings.TrimSuffix(strings.TrimSuffix(low, "-thinking"), "-think")
	if stripped != low {
		if v, ok := m[stripped]; ok {
			return v, true
		}
		st := normalizeSellPriceKey(stripped)
		for k, v := range m {
			if normalizeSellPriceKey(strings.ToLower(k)) == st {
				return v, true
			}
		}
	}
	return ModelSellPrice{}, false
}

func normalizeSellPriceKey(s string) string {
	return strings.ReplaceAll(s, "-", ".")
}

// CleanupUsedCodes completely removes all voided/used activation codes from the storage.
func CleanupUsedCodes() int {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	var activeCodes []ActivationCode
	removedCount := 0

	for _, ac := range cfg.ActivationCodes {
		if ac.Used {
			removedCount++
		} else {
			activeCodes = append(activeCodes, ac)
		}
	}

	if removedCount > 0 {
		cfg.ActivationCodes = activeCodes
		_ = Save()
	}

	return removedCount
}
