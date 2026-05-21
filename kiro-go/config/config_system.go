package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// ErrInvalidPivotStackDollarsPerYuan 表示 v5 虚拟单位换算配置非法。
// 调用方可用 errors.Is 判断是不是单位配置错误。
var ErrInvalidPivotStackDollarsPerYuan = fmt.Errorf("invalid pivotstack dollars per yuan")

// Runtime HTTP 监听地址：由 main.go 启动 listener 后回写，
// 用于让 admin settings 页区分"配置值"vs"实际生效值"（避免 8080 vs 8990 误导）。
// 只存内存，不写盘。
var (
	runtimeHTTPMu   sync.RWMutex
	runtimeHTTPHost string
	runtimeHTTPPort string
)

func SetRuntimeHTTPAddress(host, port string) {
	runtimeHTTPMu.Lock()
	defer runtimeHTTPMu.Unlock()
	runtimeHTTPHost = host
	runtimeHTTPPort = port
}

func GetRuntimeHTTPAddress() (host string, port string) {
	runtimeHTTPMu.RLock()
	host, port = runtimeHTTPHost, runtimeHTTPPort
	runtimeHTTPMu.RUnlock()
	if host == "" {
		host = GetHost()
	}
	if port == "" {
		port = strconv.Itoa(GetPort())
	}
	return host, port
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

// GetPivotStackDollarsPerYuan 返回 PivotStack 全局虚拟单位换算；旧配置为 0 时回退默认 20。
func GetPivotStackDollarsPerYuan() float64 {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return pivotStackDollarsPerYuanLocked()
}

func pivotStackDollarsPerYuanLocked() float64 {
	if cfg == nil || cfg.PivotStackDollarsPerYuan <= 0 {
		return DefaultPivotStackDollarsPerYuan
	}
	return cfg.PivotStackDollarsPerYuan
}

// VirtualUSDFromCNY 把真实 ¥ 金额换算成 virtual$ 余额；按当前 PivotStackDollarsPerYuan 动态读。
// 例：rate=20 时，¥1 → 20 virtual$。
func VirtualUSDFromCNY(cny float64) float64 {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cny * pivotStackDollarsPerYuanLocked()
}

// CNYFromVirtualUSD 把 virtual$ 余额换算成真实 ¥ 金额；按当前 PivotStackDollarsPerYuan 动态读。
// 例：rate=20 时，20 virtual$ → ¥1。
func CNYFromVirtualUSD(usd float64) float64 {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	rate := pivotStackDollarsPerYuanLocked()
	if rate <= 0 {
		return 0
	}
	return usd / rate
}

// UpdatePivotStackDollarsPerYuan 修改全局虚拟单位换算。
// rebalance=true 时保持用户真实¥购买力不变：Balance / PSDPY 恒定，所以余额乘以 newVal/oldVal。
func UpdatePivotStackDollarsPerYuan(newVal float64, rebalanceUserBalances bool) error {
	_, err := UpdatePivotStackDollarsPerYuanWithStats(newVal, rebalanceUserBalances)
	return err
}

// UpdatePivotStackDollarsPerYuanWithStats 同 UpdatePivotStackDollarsPerYuan 但返回 diff 摘要。
// admin 二次确认后调这个，把 stats 写进响应。
func UpdatePivotStackDollarsPerYuanWithStats(newVal float64, rebalanceUserBalances bool) (PivotStackUnitChangeStats, error) {
	stats := PivotStackUnitChangeStats{NewValue: newVal, Rebalanced: rebalanceUserBalances}
	if newVal <= 0 {
		return stats, fmt.Errorf("%w: %.6f", ErrInvalidPivotStackDollarsPerYuan, newVal)
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	oldVal := pivotStackDollarsPerYuanLocked()
	stats.OldValue = oldVal

	if rebalanceUserBalances {
		factor := newVal / oldVal
		for i := range cfg.ApiKeys {
			oldPaid := cfg.ApiKeys[i].Balance
			oldGift := cfg.ApiKeys[i].GiftBalance
			cfg.ApiKeys[i].Balance = oldPaid * factor
			cfg.ApiKeys[i].GiftBalance = oldGift * factor
			stats.PaidBalanceDiff += cfg.ApiKeys[i].Balance - oldPaid
			stats.GiftBalanceDiff += cfg.ApiKeys[i].GiftBalance - oldGift
			stats.UsersAffected++
		}
	}
	cfg.PivotStackDollarsPerYuan = newVal
	appendConfigAuditLog("pivotstack_unit_change",
		fmt.Sprintf("old=%.6f new=%.6f rebalance=%v users=%d paidDiff=%.4f giftDiff=%.4f",
			oldVal, newVal, rebalanceUserBalances, stats.UsersAffected, stats.PaidBalanceDiff, stats.GiftBalanceDiff))
	if err := Save(); err != nil {
		return stats, err
	}
	return stats, nil
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

// appendConfigAuditLog 在 config 包内记录无请求上下文的系统级配置变更。
// 这里不引用 proxy.AuditLog，避免 config → proxy 循环依赖。
func appendConfigAuditLog(action, detail string) {
	dir := GetDataDir()
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("[config-audit] mkdir failed: %v\n", err)
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[config-audit] open failed: %v\n", err)
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	_, _ = fmt.Fprintf(f, "[%s] action=%s operator=config %s\n", ts, action, detail)
}
