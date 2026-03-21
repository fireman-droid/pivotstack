// Package pool provides account pool management for load-balanced API proxying.
// This file implements the EcomAgent account pool — a separate, isolated pool
// for load-balancing requests to api.ecomagent.in.
package pool

import (
	"fmt"
	"kiro-api-proxy/config"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// EcomPool manages EcomAgent accounts with weighted round-robin,
// cooldown, and concurrency control — same design as AccountPool.
type EcomPool struct {
	mu           sync.RWMutex
	accounts     map[string]*config.EcomAccount
	idList       []string // ordered account IDs for round-robin
	currentIndex uint64
	cooldowns    map[string]time.Time
	errorCounts  map[string]int
	inFlight     map[string]int32
}

const ecomMaxInFlight int32 = 50

var (
	ecomPool     *EcomPool
	ecomPoolOnce sync.Once
)

// GetEcomPool returns the global EcomAgent pool singleton.
func GetEcomPool() *EcomPool {
	ecomPoolOnce.Do(func() {
		ecomPool = &EcomPool{
			accounts:    make(map[string]*config.EcomAccount),
			cooldowns:   make(map[string]time.Time),
			errorCounts: make(map[string]int),
			inFlight:    make(map[string]int32),
		}
		ecomPool.Reload()
	})
	return ecomPool
}

// Reload reloads enabled EcomAgent accounts from config.
func (p *EcomPool) Reload() {
	p.mu.Lock()
	defer p.mu.Unlock()
	enabled := config.GetEnabledEcomAccounts()

	newAccounts := make(map[string]*config.EcomAccount, len(enabled))
	var newIDList []string

	for i := range enabled {
		acc := enabled[i]
		newAccounts[acc.ID] = &acc
		newIDList = append(newIDList, acc.ID)
	}

	p.accounts = newAccounts
	p.idList = newIDList

	// Clean up stale entries
	for id := range p.inFlight {
		if _, ok := newAccounts[id]; !ok {
			delete(p.inFlight, id)
			delete(p.cooldowns, id)
			delete(p.errorCounts, id)
		}
	}
}

// parseLimit parses a limit string like "100", "10M", "5K" to an integer.
func parseLimit(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	upper := strings.ToUpper(s)
	if strings.HasSuffix(upper, "M") {
		if v, err := strconv.ParseFloat(upper[:len(upper)-1], 64); err == nil {
			return int(v * 1_000_000)
		}
	}
	if strings.HasSuffix(upper, "K") {
		if v, err := strconv.ParseFloat(upper[:len(upper)-1], 64); err == nil {
			return int(v * 1_000)
		}
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return 0
}

// quotaStatus checks an account's quota status.
// Returns: 1 = has remaining quota, 0 = unknown (no data), -1 = exhausted.
func quotaStatus(acc *config.EcomAccount) int {
	limit := parseLimit(acc.RequestLimit)
	if limit <= 0 {
		return 0 // unknown — no limit data
	}
	if acc.UpstreamRequests >= limit {
		return -1 // exhausted
	}
	return 1 // has remaining quota
}

// GetNext returns the next available EcomAgent account via round-robin.
// Selection priority:
//  1. Accounts with known remaining quota (e.g. 2/100)
//  2. Accounts with unknown quota (no upstream data)
//  3. Skip accounts at limit (e.g. 100/100)
func (p *EcomPool) GetNext() *config.EcomAccount {
	p.mu.RLock()
	defer p.mu.RUnlock()

	n := len(p.idList)
	if n == 0 {
		return nil
	}

	now := time.Now()
	startIdx := atomic.AddUint64(&p.currentIndex, 1)

	// Pass 1: prefer accounts with KNOWN remaining quota
	for i := 0; i < n; i++ {
		idx := (startIdx + uint64(i)) % uint64(n)
		accID := p.idList[idx]
		acc, ok := p.accounts[accID]
		if !ok {
			continue
		}
		if cooldown, ok := p.cooldowns[accID]; ok && now.Before(cooldown) {
			continue
		}
		if p.inFlight[accID] >= ecomMaxInFlight {
			continue
		}
		qs := quotaStatus(acc)
		if qs != 1 {
			continue // skip exhausted (-1) and unknown (0) in first pass
		}
		p.inFlight[accID]++
		fmt.Printf("[EcomPool] Selected %s (quota: %d/%s, pass=1-known)\n",
			acc.Email, acc.UpstreamRequests, acc.RequestLimit)
		return acc
	}

	// Pass 2: fallback to accounts with UNKNOWN quota (no upstream data)
	for i := 0; i < n; i++ {
		idx := (startIdx + uint64(i)) % uint64(n)
		accID := p.idList[idx]
		acc, ok := p.accounts[accID]
		if !ok {
			continue
		}
		if cooldown, ok := p.cooldowns[accID]; ok && now.Before(cooldown) {
			continue
		}
		if p.inFlight[accID] >= ecomMaxInFlight {
			continue
		}
		qs := quotaStatus(acc)
		if qs != 0 {
			continue // skip known-quota (already tried) and exhausted
		}
		p.inFlight[accID]++
		fmt.Printf("[EcomPool] Selected %s (quota: unknown, pass=2-fallback)\n", acc.Email)
		return acc
	}

	// Pass 3: last resort — find account with shortest cooldown (all quota-OK are busy)
	var best *config.EcomAccount
	var earliest time.Time
	for id, acc := range p.accounts {
		if quotaStatus(acc) == -1 {
			continue // never pick exhausted accounts
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
		fmt.Printf("[EcomPool] Selected %s (pass=3-cooldown-wait)\n", best.Email)
	}
	return best
}

// ReleaseAccount decrements the in-flight count after a request completes.
func (p *EcomPool) ReleaseAccount(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight[id] > 0 {
		p.inFlight[id]--
	}
}

// RecordSuccess clears cooldown and error count for an account.
func (p *EcomPool) RecordSuccess(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cooldowns, id)
	p.errorCounts[id] = 0
}

// RecordError records an error and applies cooldown if needed.
func (p *EcomPool) RecordError(id string, isQuotaError bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorCounts[id]++

	if isQuotaError {
		p.cooldowns[id] = time.Now().Add(30 * time.Second)
	} else if p.errorCounts[id] >= 3 {
		p.cooldowns[id] = time.Now().Add(time.Minute)
	}
}

// UpdateStats updates in-memory stats for an EcomAgent account.
// Also increments UpstreamRequests so quota tracking stays accurate between refreshes.
func (p *EcomPool) UpdateStats(id string, tokens int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if acc, ok := p.accounts[id]; ok {
		acc.RequestCount++
		acc.TotalTokens += tokens
		acc.LastUsed = time.Now().Unix()
		// Increment upstream request count locally so quota check stays accurate
		acc.UpstreamRequests++
		acc.UpstreamTokens += tokens
	}
}

// Count returns the total number of accounts in the pool.
func (p *EcomPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.accounts)
}

// AvailableCount returns the number of accounts not in cooldown.
func (p *EcomPool) AvailableCount() int {
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

// GetAllAccounts returns copies of all accounts in the pool.
func (p *EcomPool) GetAllAccounts() []config.EcomAccount {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]config.EcomAccount, 0, len(p.accounts))
	for _, acc := range p.accounts {
		result = append(result, *acc)
	}
	return result
}

// FlushStatsToConfig writes in-memory stats back to config (no disk write).
func (p *EcomPool) FlushStatsToConfig() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for id, acc := range p.accounts {
		config.UpdateEcomAccountStatsNoSave(id, acc.RequestCount, acc.ErrorCount, acc.TotalTokens, acc.LastUsed)
	}
}

// GetByID returns an account by ID, or nil if not found.
func (p *EcomPool) GetByID(id string) *config.EcomAccount {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.accounts[id]
}
