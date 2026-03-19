// Package pool 账号池管理
// 实现轮询负载均衡、错误冷却、Token 刷新、并发控制
package pool

import (
	"kiro-api-proxy/config"
	"sync"
	"sync/atomic"
	"time"
)

// weightedEntry 权重条目（引用唯一账号ID，不复制账号）
type weightedEntry struct {
	accountID string
}

// AccountPool 账号池
type AccountPool struct {
	mu           sync.RWMutex
	accounts     map[string]*config.Account // 唯一账号引用（ID -> Account）
	weightTable  []weightedEntry            // 加权轮询表
	currentIndex uint64
	cooldowns    map[string]time.Time // 账号冷却时间
	errorCounts  map[string]int       // 连续错误计数
	inFlight     map[string]int32     // 每账号当前并发请求数
}

// getMaxInFlightPerAccount returns the per-account concurrency limit from config.
func getMaxInFlightPerAccount(tier string) int32 {
	_, perFree, perPro := config.GetConcurrencyConfig()
	if tier == "pro" {
		return int32(perPro)
	}
	return int32(perFree)
}

var (
	pool     *AccountPool
	poolOnce sync.Once
)

// GetPool 获取全局账号池单例
func GetPool() *AccountPool {
	poolOnce.Do(func() {
		pool = &AccountPool{
			accounts:    make(map[string]*config.Account),
			cooldowns:   make(map[string]time.Time),
			errorCounts: make(map[string]int),
			inFlight:    make(map[string]int32),
		}
		pool.Reload()
	})
	return pool
}

// Reload 从配置重新加载账号
// 重构为唯一账号 + 权重结构，杜绝复制副本造成的统计偏差
func (p *AccountPool) Reload() {
	p.mu.Lock()
	defer p.mu.Unlock()
	enabled := config.GetEnabledAccounts()

	// 构建唯一账号 map
	newAccounts := make(map[string]*config.Account, len(enabled))
	var newWeightTable []weightedEntry

	for i := range enabled {
		acc := enabled[i] // 复制一份
		newAccounts[acc.ID] = &acc

		// 构建权重表：weight<1 出现 1 次，weight>=2 出现 weight 次
		w := acc.Weight
		if w < 1 {
			w = 1
		}
		for j := 0; j < w; j++ {
			newWeightTable = append(newWeightTable, weightedEntry{accountID: acc.ID})
		}
	}

	p.accounts = newAccounts
	p.weightTable = newWeightTable

	// 清理不存在的账号的 inFlight/cooldown/errorCounts
	for id := range p.inFlight {
		if _, ok := newAccounts[id]; !ok {
			delete(p.inFlight, id)
			delete(p.cooldowns, id)
			delete(p.errorCounts, id)
		}
	}
}

// GetNext 获取下一个可用账号（加权轮询 + InFlight 并发控制）
func (p *AccountPool) GetNext() *config.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.weightTable) == 0 {
		return nil
	}

	now := time.Now()
	n := len(p.weightTable)
	seen := make(map[string]bool)

	// 加权轮询查找可用账号
	for i := 0; i < n; i++ {
		idx := atomic.AddUint64(&p.currentIndex, 1) % uint64(n)
		entry := p.weightTable[idx]
		accID := entry.accountID

		if seen[accID] {
			continue
		}

		acc, ok := p.accounts[accID]
		if !ok {
			continue
		}

		// 跳过冷却中的账号
		if cooldown, ok := p.cooldowns[accID]; ok && now.Before(cooldown) {
			seen[accID] = true
			continue
		}

		// 跳过 InFlight 已满的账号
		if p.inFlight[accID] >= getMaxInFlightPerAccount("free") {
			seen[accID] = true
			continue
		}

		// 跳过额度已用尽的账号（检查主配额和试用配额）
		mainQuotaExhausted := acc.UsageLimit > 0 && acc.UsageCurrent >= acc.UsageLimit
		trialQuotaExhausted := acc.TrialUsageLimit > 0 && acc.TrialUsageCurrent >= acc.TrialUsageLimit
		if mainQuotaExhausted && trialQuotaExhausted {
			seen[accID] = true
			continue
		}

		// 选中账号，递增 InFlight
		p.inFlight[accID]++
		return acc
	}

	// 无可用账号，返回冷却时间最短的（排除额度用尽的）
	var best *config.Account
	var earliest time.Time
	for id, acc := range p.accounts {
		// 额度用尽的账号不作为 fallback
		if acc.UsageLimit > 0 && acc.UsageCurrent >= acc.UsageLimit {
			continue
		}
		if cooldown, ok := p.cooldowns[id]; ok {
			if best == nil || cooldown.Before(earliest) {
				best = acc
				earliest = cooldown
			}
		} else {
			p.inFlight[id]++
			return acc
		}
	}
	if best != nil {
		p.inFlight[best.ID]++
	}
	return best
}

// ReleaseAccount 释放账号的 InFlight 计数（请求完成后调用）
func (p *AccountPool) ReleaseAccount(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight[id] > 0 {
		p.inFlight[id]--
	}
}

// GetByID 根据 ID 获取账号
func (p *AccountPool) GetByID(id string) *config.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.accounts[id]
}

// RecordSuccess 记录请求成功，清除冷却
func (p *AccountPool) RecordSuccess(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cooldowns, id)
	p.errorCounts[id] = 0
}

// RecordError 记录请求错误，设置冷却
func (p *AccountPool) RecordError(id string, isQuotaError bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorCounts[id]++

	if isQuotaError {
		// 配额错误，冷却 30 秒（号多时短冷却即可）
		p.cooldowns[id] = time.Now().Add(30 * time.Second)
	} else if p.errorCounts[id] >= 3 {
		// 连续 3 次错误，冷却 1 分钟
		p.cooldowns[id] = time.Now().Add(time.Minute)
	}
}

// UpdateToken 更新账号 Token
func (p *AccountPool) UpdateToken(id, accessToken, refreshToken string, expiresAt int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if acc, ok := p.accounts[id]; ok {
		acc.AccessToken = accessToken
		if refreshToken != "" {
			acc.RefreshToken = refreshToken
		}
		acc.ExpiresAt = expiresAt
	}
}

// Count 返回账号总数（唯一账号数）
func (p *AccountPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.accounts)
}

// AvailableCount 返回可用账号数
func (p *AccountPool) AvailableCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	now := time.Now()
	count := 0
	for id := range p.accounts {
		if cooldown, ok := p.cooldowns[id]; ok && now.Before(cooldown) {
			continue
		}
		count++
	}
	return count
}

// UpdateStats 更新账号统计（内存中更新，不触发写盘）
func (p *AccountPool) UpdateStats(id string, tokens int, credits float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if acc, ok := p.accounts[id]; ok {
		acc.RequestCount++
		acc.TotalTokens += tokens
		acc.TotalCredits += credits
		acc.LastUsed = time.Now().Unix()
	}
}

// GetAllAccounts 获取所有唯一账号副本
func (p *AccountPool) GetAllAccounts() []config.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]config.Account, 0, len(p.accounts))
	for _, acc := range p.accounts {
		result = append(result, *acc)
	}
	return result
}

// GetInFlight 获取指定账号的当前并发数
func (p *AccountPool) GetInFlight(id string) int32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.inFlight[id]
}

// FlushStatsToConfig 将内存中的统计数据批量写入配置（定时调用）
func (p *AccountPool) FlushStatsToConfig() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for id, acc := range p.accounts {
		config.UpdateAccountStatsNoSave(id, acc.RequestCount, acc.ErrorCount, acc.TotalTokens, acc.TotalCredits, acc.LastUsed)
	}
}

// isAccountInTier 判断账号是否属于指定号池
// "free" 池：FREE 或空订阅类型
// "pro" 池：PRO, PRO_PLUS, POWER
func isAccountInTier(acc *config.Account, tier string) bool {
	if tier == "pro" {
		return acc.SubscriptionType == "PRO" || acc.SubscriptionType == "PRO_PLUS" || acc.SubscriptionType == "POWER"
	}
	// "free" 池
	return acc.SubscriptionType == "" || acc.SubscriptionType == "FREE"
}

// GetNextByTier 根据号池类型获取下一个可用账号
// tier: "free" 或 "pro"
func (p *AccountPool) GetNextByTier(tier string) *config.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.weightTable) == 0 {
		return nil
	}

	now := time.Now()
	n := len(p.weightTable)
	seen := make(map[string]bool)

	for i := 0; i < n; i++ {
		idx := atomic.AddUint64(&p.currentIndex, 1) % uint64(n)
		entry := p.weightTable[idx]
		accID := entry.accountID

		if seen[accID] {
			continue
		}

		acc, ok := p.accounts[accID]
		if !ok {
			continue
		}

		// 跳过不属于目标号池的账号
		if !isAccountInTier(acc, tier) {
			seen[accID] = true
			continue
		}

		// 跳过冷却中的账号
		if cooldown, ok := p.cooldowns[accID]; ok && now.Before(cooldown) {
			seen[accID] = true
			continue
		}

		// 跳过 InFlight 已满的账号
		if p.inFlight[accID] >= getMaxInFlightPerAccount(tier) {
			seen[accID] = true
			continue
		}

		// 跳过额度已用尽的账号
		mainQuotaExhausted := acc.UsageLimit > 0 && acc.UsageCurrent >= acc.UsageLimit
		trialQuotaExhausted := acc.TrialUsageLimit > 0 && acc.TrialUsageCurrent >= acc.TrialUsageLimit
		if mainQuotaExhausted && trialQuotaExhausted {
			seen[accID] = true
			continue
		}

		// 选中，递增 InFlight
		p.inFlight[accID]++
		return acc
	}

	// fallback：找冷却最短的同池账号
	var best *config.Account
	var earliest time.Time
	for id, acc := range p.accounts {
		if !isAccountInTier(acc, tier) {
			continue
		}
		if acc.UsageLimit > 0 && acc.UsageCurrent >= acc.UsageLimit {
			continue
		}
		if cooldown, ok := p.cooldowns[id]; ok {
			if best == nil || cooldown.Before(earliest) {
				best = acc
				earliest = cooldown
			}
		} else {
			p.inFlight[id]++
			return acc
		}
	}
	if best != nil {
		p.inFlight[best.ID]++
	}
	return best
}

// PoolTierStats 号池分层统计
type PoolTierStats struct {
	Total        int     `json:"total"`
	Available    int     `json:"available"`
	UsageLimit   float64 `json:"usageLimit"`
	UsageCurrent float64 `json:"usageCurrent"`
	TrialLimit   float64 `json:"trialLimit"`
	TrialCurrent float64 `json:"trialCurrent"`
}

// TierStats 获取指定号池的统计信息
func (p *AccountPool) TierStats(tier string) PoolTierStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	stats := PoolTierStats{}

	for id, acc := range p.accounts {
		if !isAccountInTier(acc, tier) {
			continue
		}
		stats.Total++
		stats.UsageLimit += acc.UsageLimit
		stats.UsageCurrent += acc.UsageCurrent
		stats.TrialLimit += acc.TrialUsageLimit
		stats.TrialCurrent += acc.TrialUsageCurrent

		// 判断是否可用
		if cooldown, ok := p.cooldowns[id]; ok && now.Before(cooldown) {
			continue
		}
		stats.Available++
	}
	return stats
}
