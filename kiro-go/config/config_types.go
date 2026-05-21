package config

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
	Provider     string `json:"provider,omitempty"`     // Identity provider name (e.g. "BuilderId", "GitHub")
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
	// Deprecated (v8+): only for orphan keys and reseller child keys.
	// Bound user keys use users.User wallet — access via users.GetWalletTotals / OverlayWalletOnKey.
	Balance        float64          `json:"balance,omitempty"`
	GiftBalance    float64          `json:"giftBalance,omitempty"`
	TotalRecharged float64          `json:"totalRecharged,omitempty"`
	TotalGifted    float64          `json:"totalGifted,omitempty"`
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

	// === v5 new-api 聚合网关 ===
	// SeriesPreferences 是用户面板里的 per-series 渠道偏好；DebtUSD 记录异步对账时补扣失败的欠款。
	// v6 deprecated: 新逻辑用 ChannelPreferences (groupID → runtimeChannelID)，仍保留兼容旧 ApiKey 数据。
	SeriesPreferences  map[string]string `json:"seriesPreferences,omitempty"`  // seriesID → channelID (deprecated)
	ChannelPreferences map[string]string `json:"channelPreferences,omitempty"` // v6: groupID → runtime channel id
	DebtUSD            float64           `json:"debtUsd,omitempty"`
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

// DefaultPivotStackDollarsPerYuan 是 v5 全局单位默认值；业务代码必须通过 GetPivotStackDollarsPerYuan 读取。
const DefaultPivotStackDollarsPerYuan = 20.0

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

// Series 是 v4 系列抽象 — 按 model 名归类成 series（claude/gpt/gemini），admin 在 series 级别指定默认渠道。
// Series=[] 时整个 channel 路由走 v3 legacy flat 模式（零破坏向后兼容）。
type Series struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	DefaultChannelID string   `json:"defaultChannelId,omitempty"` // 必须指向已存在且 enabled 的 channel
	ModelPatterns    []string `json:"modelPatterns,omitempty"`    // 前缀 "claude-" 或正则 "re:^claude-.+$"
}

// ChannelGroup 是 v6 真正的"上层分组"：admin 手动建命名分组（如"claude 分组"/"codex 分组"），
// 手动挂载多个 NewAPI 物化渠道 / 自营直连渠道；user 按分组看渠道清单并精确选某条。
// 与 Series 的区别：Series 是按 ModelPatterns 自动匹配（model 名 → series → channel），
// ChannelGroup 主要靠 admin 手动挂渠道；ModelPatterns 仅作可选 matcher 兼容旧路径。
type ChannelGroup struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	Description   string                    `json:"description,omitempty"`
	Enabled       bool                      `json:"enabled"`
	ModelPatterns []string                  `json:"modelPatterns,omitempty"`  // 可选自动 matcher
	Channels      []ChannelGroupChannelRef  `json:"channels,omitempty"`       // 手动挂的成员
	DefaultRuntimeChannelID string          `json:"defaultRuntimeChannelId,omitempty"` // runtime channel id（newapi: 原 id；direct: "direct:<id>"）
	SortOrder     int                       `json:"sortOrder,omitempty"`      // 同 model 多 group 命中时的优先级（小者优先）
	CreatedAt     int64                     `json:"createdAt,omitempty"`
	UpdatedAt     int64                     `json:"updatedAt,omitempty"`
	DeletedAt     int64                     `json:"deletedAt,omitempty"`      // 软删
}

// ChannelGroupChannelRef 引用一条物化渠道（NewAPI 或自营直连）。
type ChannelGroupChannelRef struct {
	SourceType string `json:"sourceType"` // "newapi" | "direct"
	ChannelID  string `json:"channelId"`  // 配置层 ID（NewAPIChannel.ID 或 DirectChannel.ID，不带 runtime "direct:" 前缀）
}

// ChannelConfig 是上游 API 渠道配置。BillingMode != "" 时由 ChannelRouter 路由请求。
// channels 为空时回退到 legacy Kiro 路径（零破坏）。
//
// ModelPrices 是渠道独占售价 — 同一个 model 名从两个不同渠道接入时分别定价。
// v4 series 模式下渠道路由请求严格用 channel.ModelPrices（缺则失败关闭，不 fallback 全局）。
// v3 legacy flat 模式（无 Series）保留全局 SellPrices fallback。
//
// SeriesID: v4 关联到 Series.ID；空字符串 = legacy flat 模式渠道。
// ModelAliases: client 看到的 model 名 → 上游真实 model 名（用于中转商命名不一致场景）。
// ExtraHeaders: 注入额外 HTTP 头（denylist 防覆盖 Authorization/Content-Length/Host 等关键头）。
type ChannelConfig struct {
	ID           string                    `json:"id"`
	Type         string                    `json:"type"`               // "kiro" | "openai"
	SeriesID     string                    `json:"seriesId,omitempty"` // v4: 关联 Series.ID
	BaseURL      string                    `json:"baseUrl,omitempty"`
	APIKey       string                    `json:"apiKey,omitempty"` // 外部渠道 API key，admin 接口返回时必须 mask
	Models       []string                  `json:"models"`
	ModelPrices  map[string]ModelSellPrice `json:"modelPrices,omitempty"`  // v3：渠道内部售价（同 model 不同渠道可独立定价）
	ModelAliases map[string]string         `json:"modelAliases,omitempty"` // v4: public model → upstream model
	ExtraHeaders map[string]string         `json:"extraHeaders,omitempty"` // v4: 注入上游 header（denylist 保护）
	Enabled      bool                      `json:"enabled"`
}

// DirectSellPrice 是直连渠道的默认售价和 per-model 覆盖。
type DirectSellPrice struct {
	Default DirectSellPriceRow            `json:"default,omitempty"`
	Models  map[string]DirectSellPriceRow `json:"models,omitempty"`
}

// DirectSellPriceRow 是直连渠道按 token 计费的售价/成本行。
type DirectSellPriceRow struct {
	InputPerM      float64 `json:"inputPerM,omitempty"`
	OutputPerM     float64 `json:"outputPerM,omitempty"`
	CostInputPerM  float64 `json:"costInputPerM,omitempty"`
	CostOutputPerM float64 `json:"costOutputPerM,omitempty"`
}

// DirectChannel 是 v6 直连渠道配置。
//
// Type: "openai" | "kiro"
// Alias: admin/user 可见的渠道组名，必须在 DirectChannel 与 NewAPIChannel 中全局唯一。
// APIKeyEnc: 上游 key 的 AES-GCM 密文，明文只在运行期解密。
// ModelMapping: client model → upstream model。
type DirectChannel struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Alias        string            `json:"alias"`
	BaseURL      string            `json:"baseUrl,omitempty"`
	APIKeyEnc    string            `json:"apiKeyEnc,omitempty"`
	Models       []string          `json:"models,omitempty"`
	SellPrice    DirectSellPrice   `json:"sellPrice,omitempty"`
	ModelMapping map[string]string `json:"modelMapping,omitempty"`
	ExtraHeaders map[string]string `json:"extraHeaders,omitempty"`
	Enabled      bool              `json:"enabled"`
	Status       string            `json:"status,omitempty"` // "active" | "error" | "degraded"; 空 = 未初始化
	CreatedAt    int64             `json:"createdAt,omitempty"`
	UpdatedAt    int64             `json:"updatedAt,omitempty"`
	DeletedAt    int64             `json:"deletedAt,omitempty"`
}

// NewAPIProvider 是 v5 new-api 上游站点配置。
// 上游密码、access_token 都以 AES-256-GCM 密文持久化，运行期按需解密。
type NewAPIProvider struct {
	ID                    string  `json:"id"` // "apijing"
	Name                  string  `json:"name"`
	BaseURL               string  `json:"baseUrl"`
	Username              string  `json:"username"`
	PasswordEnc           string  `json:"passwordEnc,omitempty"`
	AccessTokenEnc        string  `json:"accessTokenEnc,omitempty"`
	AccessTokenExpiresAt  int64   `json:"accessTokenExpiresAt,omitempty"`
	UserID                int     `json:"userId,omitempty"`
	QuotaPerUnitDollar    float64 `json:"quotaPerUnitDollar"`
	YuanPerUpstreamDollar float64 `json:"yuanPerUpstreamDollar"`
	LastSyncAt            int64   `json:"lastSyncAt,omitempty"`
	LastSyncError         string  `json:"lastSyncError,omitempty"`
	SyncIntervalSec       int     `json:"syncIntervalSec,omitempty"`
	Enabled               bool    `json:"enabled"`
}

// NewAPIChannel 是由上游 sk-* token 物化出来的 PivotStack 渠道。
// admin 只维护 Alias / Markup / SeriesID / Enabled；其余字段由同步流程覆盖。
type NewAPIChannel struct {
	ID                string   `json:"id"` // stable: "apijing:tok-908"
	ProviderID        string   `json:"providerId"`
	Alias             string   `json:"alias"`
	UpstreamTokenID   int      `json:"upstreamTokenId"`
	UpstreamKeyEnc    string   `json:"upstreamKeyEnc,omitempty"`
	UpstreamTokenName string   `json:"upstreamTokenName,omitempty"`
	GroupName         string   `json:"groupName"`
	Models            []string `json:"models"`
	Markup            float64  `json:"markup"`
	SeriesID          string   `json:"seriesId,omitempty"`
	CreateMode        string   `json:"createMode,omitempty"` // "pivotstack" | "legacy_import"
	Enabled           bool     `json:"enabled"`
	RemainQuota       int64    `json:"remainQuota"`
	UnlimitedQuota    bool     `json:"unlimitedQuota"`
	Status            int      `json:"status"`
	CreatedAt         int64    `json:"createdAt,omitempty"`
	UpdatedAt         int64    `json:"updatedAt,omitempty"`
	LastSeenAt        int64    `json:"lastSeenAt,omitempty"`
	DeletedAt         int64    `json:"deletedAt,omitempty"`
}

// NewAPIGroup mirror upstream /api/user/groups 的最小字段。
// Phase 1 只定义缓存 DTO，后续同步 worker 再填充完整解析逻辑。
type NewAPIGroup struct {
	Name  string  `json:"name"`
	Desc  string  `json:"desc,omitempty"`
	Ratio float64 `json:"ratio"`
}

// NewAPIModel mirror upstream /api/pricing 的计费相关字段。
type NewAPIModel struct {
	ModelName       string   `json:"model_name"`
	ModelRatio      float64  `json:"model_ratio"`
	CompletionRatio float64  `json:"completion_ratio"`
	CacheRatio      float64  `json:"cache_ratio,omitempty"`
	EnableGroups    []string `json:"enable_groups,omitempty"`
}

// NewAPIToken mirror upstream /api/token/ 的 token 元数据。
type NewAPIToken struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key,omitempty"`
	Group          string `json:"group"`
	RemainQuota    int64  `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	Status         int    `json:"status"`
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

// Config represents the global application configuration.
type Config struct {
	// Server settings
	Password        string           `json:"password"`         // Admin panel password
	Port            int              `json:"port"`             // HTTP server port (default: 8080)
	SchemaVersion   int              `json:"schemaVersion,omitempty"`
	LastV6MigrationAt int64          `json:"lastV6MigrationAt,omitempty"`
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
	Series      []Series        `json:"series,omitempty"` // v4 系列抽象，空 = legacy flat 模式（零破坏向后兼容）
	Channels    []ChannelConfig `json:"channels,omitempty"`
	BillingMode string          `json:"billingMode,omitempty"`

	// === v5 new-api 聚合网关（空值零破坏）===
	NewAPIProviders          []NewAPIProvider `json:"newapiProviders,omitempty"`
	NewAPIChannels           []NewAPIChannel  `json:"newapiChannels,omitempty"`
	DirectChannels           []DirectChannel  `json:"directChannels,omitempty"`
	SecretKeySalt            string           `json:"secretKeySalt,omitempty"`
	PivotStackDollarsPerYuan float64          `json:"pivotStackDollarsPerYuan,omitempty"`

	// === v6 ChannelGroup：admin 手动建分组 + 手动挂渠道；user 按分组选具体渠道。空 = 走 legacy series 路径。
	ChannelGroups []ChannelGroup `json:"channelGroups,omitempty"`
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

// ThinkingConfig holds settings for AI thinking/reasoning mode.
// When enabled, models output their reasoning process alongside the response.
type ThinkingConfig struct {
	Suffix       string `json:"suffix"`       // Model name suffix that triggers thinking mode
	OpenAIFormat string `json:"openaiFormat"` // Output format for OpenAI-compatible responses
	ClaudeFormat string `json:"claudeFormat"` // Output format for Claude-compatible responses
}

// PivotStackUnitChangeStats 是 admin 改全局单位后的 diff 摘要（供 UI 显示影响范围）。
type PivotStackUnitChangeStats struct {
	OldValue        float64 `json:"oldValue"`
	NewValue        float64 `json:"newValue"`
	Rebalanced      bool    `json:"rebalanced"`
	UsersAffected   int     `json:"usersAffected"`
	PaidBalanceDiff float64 `json:"paidBalanceDiff"` // sum(after) - sum(before) — 虚拟$
	GiftBalanceDiff float64 `json:"giftBalanceDiff"`
}
