package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"sync"
	"time"
)

// KeyRuntime tracks per-key runtime state for abuse prevention.
type KeyRuntime struct {
	mu            sync.Mutex
	ActiveStreams int
	IPLastSeen    map[string]int64 // ip -> last request timestamp
	RequestTimes  []int64          // sliding window of request timestamps
	Flagged       bool
	FlagReason    string
}

var keyRuntimes = sync.Map{} // keyID -> *KeyRuntime

func getOrCreateRuntime(keyID string) *KeyRuntime {
	if v, ok := keyRuntimes.Load(keyID); ok {
		return v.(*KeyRuntime)
	}
	rt := &KeyRuntime{
		IPLastSeen: make(map[string]int64),
	}
	actual, _ := keyRuntimes.LoadOrStore(keyID, rt)
	return actual.(*KeyRuntime)
}

// OnRequestStart checks abuse limits before allowing a request.
// Returns (allowed, reason).
func OnRequestStart(keyID, ip string) (bool, string) {
	if keyID == "" {
		return true, ""
	}

	rt := getOrCreateRuntime(keyID)
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now().Unix()

	// Concurrency limit: dynamic from config (default: 20)
	maxPerKey, _ := config.GetConcurrencyConfig()
	if rt.ActiveStreams >= maxPerKey {
		fmt.Printf("[Abuse] key=%s blocked: concurrency_limit (%d/%d active)\n", keyID[:8], rt.ActiveStreams, maxPerKey)
		return false, "concurrency_limit"
	}

	// IP diversity detection: >10 distinct IPs in 1 hour -> flag (don't block)
	distinctIPs := countDistinctIPsInWindow(rt.IPLastSeen, now, 3600)
	if distinctIPs > 10 && !rt.Flagged {
		rt.Flagged = true
		rt.FlagReason = fmt.Sprintf("ip_diversity: %d IPs in 1h", distinctIPs)
		fmt.Printf("[Abuse] key=%s flagged: %s\n", keyID[:8], rt.FlagReason)
	}

	// Rate limit: max 200 requests in 60 seconds
	rt.RequestTimes = pruneOldTimestamps(rt.RequestTimes, now, 60)
	if len(rt.RequestTimes) >= 200 {
		fmt.Printf("[Abuse] key=%s blocked: rate_limit (%d req/60s)\n", keyID[:8], len(rt.RequestTimes))
		return false, "rate_limit"
	}

	rt.ActiveStreams++
	rt.IPLastSeen[ip] = now
	rt.RequestTimes = append(rt.RequestTimes, now)
	return true, ""
}

// OnRequestEnd decrements active stream count.
func OnRequestEnd(keyID string) {
	if keyID == "" {
		return
	}
	rt := getOrCreateRuntime(keyID)
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.ActiveStreams > 0 {
		rt.ActiveStreams--
	}
}

// GetFlaggedKeys returns all flagged key IDs with their reasons.
func GetFlaggedKeys() []map[string]interface{} {
	var results []map[string]interface{}
	keyRuntimes.Range(func(key, value interface{}) bool {
		rt := value.(*KeyRuntime)
		rt.mu.Lock()
		defer rt.mu.Unlock()
		if rt.Flagged {
			results = append(results, map[string]interface{}{
				"keyId":         key.(string),
				"reason":        rt.FlagReason,
				"activeStreams": rt.ActiveStreams,
				"distinctIPs":   len(rt.IPLastSeen),
			})
		}
		return true
	})
	return results
}

// ClearFlag removes the flag for a given key.
func ClearFlag(keyID string) {
	if v, ok := keyRuntimes.Load(keyID); ok {
		rt := v.(*KeyRuntime)
		rt.mu.Lock()
		defer rt.mu.Unlock()
		rt.Flagged = false
		rt.FlagReason = ""
	}
}

// --- helpers ---

func countDistinctIPsInWindow(ips map[string]int64, now, windowSec int64) int {
	count := 0
	cutoff := now - windowSec
	for _, ts := range ips {
		if ts >= cutoff {
			count++
		}
	}
	return count
}

func pruneOldTimestamps(times []int64, now, windowSec int64) []int64 {
	cutoff := now - windowSec
	start := 0
	for start < len(times) && times[start] < cutoff {
		start++
	}
	if start > 0 {
		return times[start:]
	}
	return times
}

// --- Redeem rate limiting ---

var redeemAttempts = sync.Map{} // ip -> []int64 timestamps

// CheckRedeemRateLimit checks if an IP has exceeded the redeem attempt limit.
// Returns (allowed, reason). Max 5 failed attempts per IP per 60 seconds.
func CheckRedeemRateLimit(ip string) (bool, string) {
	now := time.Now().Unix()
	v, _ := redeemAttempts.LoadOrStore(ip, &struct {
		mu    sync.Mutex
		times []int64
	}{})
	rl := v.(*struct {
		mu    sync.Mutex
		times []int64
	})
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.times = pruneOldTimestamps(rl.times, now, 60)
	if len(rl.times) >= 5 {
		return false, "too many redeem attempts, please try again later"
	}
	rl.times = append(rl.times, now)
	return true, ""
}
