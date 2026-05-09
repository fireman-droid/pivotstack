package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ParseAbuseReason 把 OnRequestStart 返回的 reason 字符串解析为 (kind, retryAfterSec)。
//   - kind:           "rate_limit" | "concurrency_limit" | <原值>
//   - retryAfterSec:  仅 rate_limit 类型有意义，0 表示无建议
//
// 调用方据此设 Retry-After 头 + 决定具体错误文案。
func ParseAbuseReason(reason string) (kind string, retryAfterSec int) {
	if i := strings.Index(reason, ":"); i > 0 {
		if n, err := strconv.Atoi(reason[i+1:]); err == nil {
			return reason[:i], n
		}
		return reason[:i], 0
	}
	return reason, 0
}

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

// 老兜底速率（credit / 已过期天卡走这个）。保留写死，避免破坏现有行为。
const legacyDefaultRPM = 200

// isTimedActive 当前 key 是否处于"按时长收费"的活跃期。
// 只有这种 key 才需要走防共享速率限制（credit 计费自带反共享）。
func isTimedActive(info *config.ApiKeyInfo) bool {
	if info == nil {
		return false
	}
	if info.Plan != "timed" && info.Plan != "hybrid" {
		return false
	}
	if info.ExpiresAt <= 0 {
		return false
	}
	return time.Now().Unix() <= info.ExpiresAt
}

// getEffectiveRPM 返回当前 key 应该用的 60s 滑动窗口上限。
//
// 决策顺序：
//  1. 非天卡活跃期 → 老兜底 200/min（credit / 过期 hybrid）
//  2. key.RateLimitPerMin > 0 → 用 key 自己的（per-key override，本期 UI 不暴露）
//  3. 否则用 Settings.TimedKeyRPM
//  4. Settings 也没配 → 回退老兜底
func getEffectiveRPM(info *config.ApiKeyInfo) int {
	if !isTimedActive(info) {
		return legacyDefaultRPM
	}
	if info != nil && info.RateLimitPerMin > 0 {
		return info.RateLimitPerMin
	}
	rpm := config.GetTimedKeyRPM()
	if rpm <= 0 {
		return legacyDefaultRPM
	}
	return rpm
}

// computeRetryAfter 根据 60s 滑动窗口，算下一次请求最少要等多少秒（向上取整）。
// 用窗口里最早一条的"距离 60 秒过去"还差多久。
func computeRetryAfter(times []int64, now int64) int {
	if len(times) == 0 {
		return 1
	}
	earliest := times[0]
	wait := earliest + 60 - now
	if wait < 1 {
		wait = 1
	}
	return int(wait)
}

// OnRequestStart checks abuse limits before allowing a request.
// Returns (allowed, reason). reason 形如 "rate_limit:N"（N 秒后重试），调用方据此设 Retry-After。
func OnRequestStart(keyID, ip string) (bool, string) {
	if keyID == "" {
		return true, ""
	}

	rt := getOrCreateRuntime(keyID)
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now().Unix()

	// Concurrency limit: dynamic from config (default: 20)
	maxPerKey, _, _ := config.GetConcurrencyConfig()
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

	// Rate limit: 天卡走 per-key/global RPM；其他走老兜底 200/min
	info := config.FindApiKeyByID(keyID)
	limit := getEffectiveRPM(info)
	rt.RequestTimes = pruneOldTimestamps(rt.RequestTimes, now, 60)
	if len(rt.RequestTimes) >= limit {
		retryAfter := computeRetryAfter(rt.RequestTimes, now)
		fmt.Printf("[Abuse] key=%s blocked: rate_limit (%d req/60s, limit=%d, retry=%ds)\n",
			keyID[:8], len(rt.RequestTimes), limit, retryAfter)
		return false, fmt.Sprintf("rate_limit:%d", retryAfter)
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
