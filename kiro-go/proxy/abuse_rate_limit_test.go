package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"kiro-api-proxy/config"
	"strings"
	"testing"
	"time"
)

// ============== 工具函数 ==============

func mustRandomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

// makeKey 创建一个测试 key 写入 config，返回 ID 和 cleanup。
// plan: "credit" / "timed" / "hybrid"
// expiresAt: 0=不设, >0=绝对 unix 时间, <0=相对 now 偏移秒
func makeKey(t *testing.T, plan string, expiresAt int64, perKeyRPM int) (string, func()) {
	t.Helper()
	id := "test-rl-" + mustRandomHex(t, 4)
	exp := expiresAt
	if expiresAt < 0 {
		exp = time.Now().Unix() + expiresAt
	}
	k := config.ApiKeyInfo{
		ID:              id,
		Key:             "sk-test-" + mustRandomHex(t, 8),
		Plan:            plan,
		Enabled:         true,
		ExpiresAt:       exp,
		Balance:         100,
		RateLimitPerMin: perKeyRPM,
		CreatedAt:       time.Now().Unix(),
	}
	if err := config.AddApiKey(k); err != nil {
		t.Fatalf("AddApiKey: %v", err)
	}
	cleanup := func() {
		_ = config.DeleteApiKey(id)
		// 清掉 runtime
		keyRuntimes.Delete(id)
	}
	return id, cleanup
}

// burst 模拟连续 N 次调用 OnRequestStart，统计成功/失败。
func burst(t *testing.T, keyID string, n int) (allowed, blocked int, lastReason string) {
	t.Helper()
	for i := 0; i < n; i++ {
		ok, reason := OnRequestStart(keyID, fmt.Sprintf("1.2.3.%d", (i%200)+1))
		if ok {
			allowed++
			OnRequestEnd(keyID)
		} else {
			blocked++
			lastReason = reason
		}
	}
	return
}

// ============== 测试用例 ==============

// TestCreditPlan_BypassesNewRules: plan=credit + RateLimitPerMin=5 也不限速
func TestCreditPlan_BypassesNewRules(t *testing.T) {
	id, cleanup := makeKey(t, "credit", 0, 5)
	defer cleanup()

	// 即使 key.RateLimitPerMin=5 也不应该被限速，因为 plan=credit
	allowed, blocked, _ := burst(t, id, 50)
	if blocked > 0 {
		t.Errorf("credit plan should not be rate limited, but blocked %d/50", blocked)
	}
	if allowed != 50 {
		t.Errorf("expected 50 allowed, got %d", allowed)
	}
}

// TestExpiredTimed_BypassesNewRules: plan=timed + ExpiresAt 已过期 → 不限速
func TestExpiredTimed_BypassesNewRules(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()-3600, 5) // 1 小时前过期
	defer cleanup()

	allowed, blocked, _ := burst(t, id, 50)
	if blocked > 0 {
		t.Errorf("expired timed key should not be rate limited, but blocked %d/50", blocked)
	}
	if allowed != 50 {
		t.Errorf("expected 50 allowed, got %d", allowed)
	}
}

// TestActiveTimed_PerKeyRPMEnforced: 天卡有效期内 + RateLimitPerMin=5 → 第 6 次限流
func TestActiveTimed_PerKeyRPMEnforced(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()+3600, 5)
	defer cleanup()

	allowed, blocked, reason := burst(t, id, 10)
	if allowed != 5 {
		t.Errorf("expected 5 allowed (RPM=5), got %d", allowed)
	}
	if blocked != 5 {
		t.Errorf("expected 5 blocked, got %d", blocked)
	}
	if !strings.HasPrefix(reason, "rate_limit:") {
		t.Errorf("expected reason 'rate_limit:N', got %q", reason)
	}
}

// TestActiveHybrid_PerKeyRPMEnforced: hybrid 也走限速逻辑
func TestActiveHybrid_PerKeyRPMEnforced(t *testing.T) {
	id, cleanup := makeKey(t, "hybrid", time.Now().Unix()+3600, 3)
	defer cleanup()

	allowed, blocked, _ := burst(t, id, 10)
	if allowed != 3 {
		t.Errorf("hybrid: expected 3 allowed, got %d", allowed)
	}
	if blocked != 7 {
		t.Errorf("hybrid: expected 7 blocked, got %d", blocked)
	}
}

// TestActiveTimed_GlobalRPMFallback: key.RateLimitPerMin=0 但 Settings.TimedKeyRPM=4 → 用全局
func TestActiveTimed_GlobalRPMFallback(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()+3600, 0)
	defer cleanup()

	// 临时设全局 RPM
	prev := config.GetTimedKeyRPM()
	if err := config.UpdateTimedKeyRPM(4); err != nil {
		t.Fatalf("UpdateTimedKeyRPM: %v", err)
	}
	defer config.UpdateTimedKeyRPM(prev)

	allowed, blocked, _ := burst(t, id, 10)
	if allowed != 4 {
		t.Errorf("global RPM=4, expected 4 allowed, got %d", allowed)
	}
	if blocked != 6 {
		t.Errorf("expected 6 blocked, got %d", blocked)
	}
}

// TestActiveTimed_PerKeyOverridesGlobal: per-key=2 比 global=10 严，应取 per-key
func TestActiveTimed_PerKeyOverridesGlobal(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()+3600, 2)
	defer cleanup()

	prev := config.GetTimedKeyRPM()
	_ = config.UpdateTimedKeyRPM(10)
	defer config.UpdateTimedKeyRPM(prev)

	allowed, _, _ := burst(t, id, 10)
	if allowed != 2 {
		t.Errorf("per-key RPM=2 should override global=10, got allowed=%d", allowed)
	}
}

// TestActiveTimed_NoConfigFallsBackTo200: 天卡有效但 per-key=0 且 global=0 → 老兜底 200
func TestActiveTimed_NoConfigFallsBackTo200(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()+3600, 0)
	defer cleanup()

	prev := config.GetTimedKeyRPM()
	_ = config.UpdateTimedKeyRPM(0) // 显式禁用
	defer config.UpdateTimedKeyRPM(prev)

	// 200 个应当全过；第 201 个应被拦
	allowed, _, _ := burst(t, id, 199)
	if allowed != 199 {
		t.Errorf("expected 199 allowed under fallback 200, got %d", allowed)
	}
	// 已用 199 → 还能再 1 次（合计 200），第 2 次拦
	a, b, _ := burst(t, id, 2)
	if a != 1 || b != 1 {
		t.Errorf("at boundary expected 1/1, got %d/%d", a, b)
	}
}

// TestRetryAfterMatchesWindow: 触发限速时返回的 retry_after 应等于"最早请求 + 60 - now"
func TestRetryAfterMatchesWindow(t *testing.T) {
	id, cleanup := makeKey(t, "timed", time.Now().Unix()+3600, 3)
	defer cleanup()

	// 用满 3 个名额
	for i := 0; i < 3; i++ {
		ok, _ := OnRequestStart(id, "1.1.1.1")
		if !ok {
			t.Fatalf("first 3 should pass at i=%d", i)
		}
		OnRequestEnd(id)
	}
	// 第 4 个应被拦，且 reason 形如 "rate_limit:N"，N 应是 60 左右
	ok, reason := OnRequestStart(id, "1.1.1.1")
	if ok {
		t.Fatalf("4th request should be blocked")
	}
	kind, retry := ParseAbuseReason(reason)
	if kind != "rate_limit" {
		t.Errorf("expected kind=rate_limit, got %q", kind)
	}
	if retry < 1 || retry > 61 {
		t.Errorf("retry should be 1..60s, got %d", retry)
	}
}

// TestParseAbuseReason: 解析逻辑覆盖各种格式
func TestParseAbuseReason(t *testing.T) {
	cases := []struct {
		in     string
		kind   string
		retry  int
	}{
		{"rate_limit:30", "rate_limit", 30},
		{"rate_limit:0", "rate_limit", 0},
		{"concurrency_limit", "concurrency_limit", 0},
		{"", "", 0},
		{"rate_limit:abc", "rate_limit", 0}, // 非法数字
	}
	for _, c := range cases {
		k, r := ParseAbuseReason(c.in)
		if k != c.kind || r != c.retry {
			t.Errorf("ParseAbuseReason(%q): got (%q, %d), want (%q, %d)", c.in, k, r, c.kind, c.retry)
		}
	}
}

// TestIsTimedActive: 门控函数边界
func TestIsTimedActive(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		name string
		info *config.ApiKeyInfo
		want bool
	}{
		{"nil", nil, false},
		{"credit no expiry", &config.ApiKeyInfo{Plan: "credit"}, false},
		{"credit with expiry", &config.ApiKeyInfo{Plan: "credit", ExpiresAt: now + 100}, false},
		{"timed no expiry", &config.ApiKeyInfo{Plan: "timed"}, false},
		{"timed expired", &config.ApiKeyInfo{Plan: "timed", ExpiresAt: now - 1}, false},
		{"timed active", &config.ApiKeyInfo{Plan: "timed", ExpiresAt: now + 100}, true},
		{"hybrid expired", &config.ApiKeyInfo{Plan: "hybrid", ExpiresAt: now - 1}, false},
		{"hybrid active", &config.ApiKeyInfo{Plan: "hybrid", ExpiresAt: now + 100}, true},
	}
	for _, c := range cases {
		if got := isTimedActive(c.info); got != c.want {
			t.Errorf("%s: isTimedActive() = %v, want %v", c.name, got, c.want)
		}
	}
}
