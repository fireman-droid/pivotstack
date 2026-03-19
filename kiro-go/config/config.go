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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
	Enabled   bool   `json:"enabled"`             // Whether account is active in the pool
	BanStatus string `json:"banStatus,omitempty"` // Ban status: "ACTIVE", "BANNED", "SUSPENDED"
	BanReason string `json:"banReason,omitempty"` // Reason for ban/suspension
	BanTime   int64  `json:"banTime,omitempty"`   // Timestamp when ban was detected

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
}

// ActivationCode represents a redeemable code for balance or time extension.
type ActivationCode struct {
	Code          string  `json:"code"`                    // e.g. KIRO-XXXX-XXXX-XXXX
	Type          string  `json:"type"`                    // "balance" | "days"
	Amount        float64 `json:"amount"`                  // balance: USD face value; days: number of days
	Tier          string  `json:"tier,omitempty"`          // "free" | "pro" (only for type=days)
	CodeExpiresAt int64   `json:"codeExpiresAt,omitempty"` // code itself expires (0=never)
	Used          bool    `json:"used"`
	UsedBy        string  `json:"usedBy,omitempty"` // ApiKey ID
	UsedAt        int64   `json:"usedAt,omitempty"`
	CreatedAt     int64   `json:"createdAt"`
	Note          string  `json:"note,omitempty"`
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

// PricingConfig holds credit-based pricing.
type PricingConfig struct {
	FreePoolPriceUSD float64 `json:"freePoolPriceUSD"` // face-value USD per credit for FREE pool (default: 0.40)
	ProPoolPriceUSD  float64 `json:"proPoolPriceUSD"`  // face-value USD per credit for PRO pool (default: 2.00)

	ProCostEntries  []CostEntry `json:"proCostEntries,omitempty"`
	FreeCostEntries []CostEntry `json:"freeCostEntries,omitempty"`

	// Deprecated fields kept for backward compat
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
func GetPricing() PricingConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	p := cfg.Pricing
	if p.FreePoolPriceUSD == 0 {
		p.FreePoolPriceUSD = 0.40
	}
	if p.ProPoolPriceUSD == 0 {
		p.ProPoolPriceUSD = 2.00
	}
	return p
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

	// Global statistics (persisted across restarts)
	TotalRequests   int     `json:"totalRequests,omitempty"`   // Total API requests received
	SuccessRequests int     `json:"successRequests,omitempty"` // Successful requests count
	FailedRequests  int     `json:"failedRequests,omitempty"`  // Failed requests count
	TotalTokens     int     `json:"totalTokens,omitempty"`     // Total tokens processed
	TotalCredits    float64 `json:"totalCredits,omitempty"`    // Total credits consumed
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
			// Create default configuration.
			// Binds to 0.0.0.0 by default for Docker/container compatibility.
			cfg = &Config{
				Password:      "changeme",
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

// SetPassword updates the admin password.
// Primarily used for environment variable override in containerized deployments.
func SetPassword(password string) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.Password = password
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
	if idx := findAccountByEmailLocked(account.Email); idx >= 0 {
		return fmt.Errorf("duplicate: account with email %s already exists (id: %s)", account.Email, cfg.Accounts[idx].ID)
	}
	cfg.Accounts = append(cfg.Accounts, account)
	return Save()
}

// AddOrUpdateAccount adds a new account, or updates credentials if one with the same email exists.
// Returns (accountID, isNew, error).
func AddOrUpdateAccount(account Account) (string, bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	if idx := findAccountByEmailLocked(account.Email); idx >= 0 {
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
	existingEmails := make(map[string]bool)

	// Build map of existing account IDs and emails
	for _, a := range cfg.Accounts {
		existingIDs[a.ID] = true
		if a.Email != "" {
			existingEmails[strings.ToLower(a.Email)] = true
		}
	}

	// Import new accounts (skip duplicates by ID or email)
	for _, account := range accounts {
		if existingIDs[account.ID] {
			skipped++
			continue
		}
		if account.Email != "" && existingEmails[strings.ToLower(account.Email)] {
			skipped++
			continue
		}

		// Generate machine ID if not present
		if account.MachineId == "" {
			account.MachineId = GenerateMachineId()
		}

		cfg.Accounts = append(cfg.Accounts, account)
		existingIDs[account.ID] = true
		if account.Email != "" {
			existingEmails[strings.ToLower(account.Email)] = true
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

func UpdateSettings(apiKey string, requireApiKey bool, password string) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.ApiKey = apiKey
	cfg.RequireApiKey = requireApiKey
	if password != "" {
		cfg.Password = password
	}
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
func ValidateKeyAccessForModel(info *ApiKeyInfo, modelPool string) (string, error) {
	if info == nil || !info.Enabled {
		return "", fmt.Errorf("api key disabled")
	}
	now := time.Now().Unix()
	hasDayCard := (info.Plan == "timed" || info.Plan == "hybrid") && (info.ExpiresAt == 0 || now <= info.ExpiresAt)
	hasBalance := info.Balance > 0 || info.GiftBalance > 0

	switch modelPool {
	case "free":
		if hasDayCard {
			return "free", nil // day card covers free pool models
		}
		if hasBalance {
			return "deduct", nil
		}
		return "", fmt.Errorf("no active plan or balance")
	case "pro":
		if hasDayCard && info.Tier == "pro" {
			return "free", nil // pro day card covers all models
		}
		// free day card + balance OR pure balance → deduct
		if hasBalance {
			return "deduct", nil
		}
		return "", fmt.Errorf("pro models require pro day-card or balance")
	}
	return "", fmt.Errorf("unknown pool: %s", modelPool)
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
						cfg.ApiKeys[j].ExpiresAt = base + int64(ac.Amount)
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
