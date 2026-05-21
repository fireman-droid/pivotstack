package proxy

import (
	"context"
	"sync"
	"time"
)

const (
	adminLoginMaxFailures   = 5
	adminLoginLockout       = 10 * time.Minute
	adminLoginFailureWindow = 10 * time.Minute

	userLoginMaxFailures   = 5
	userLoginLockout       = 5 * time.Minute
	userLoginFailureWindow = 60 * time.Second
)

type authAttempt struct {
	Failures    int
	WindowStart time.Time
	LockedUntil time.Time
	LastSeen    time.Time
}

type loginAttempt = authAttempt

// authLimiter 通用 IP 失败限流。admin login / user login / 兑换码限流统一走这个类型，
// 配置通过 newAuthLimiter 构造参数注入（maxFailures / failureWindow / lockout）。
type authLimiter struct {
	mu            sync.Mutex
	entries       map[string]*authAttempt
	maxFailures   int
	failureWindow time.Duration
	lockout       time.Duration
	cleanupAfter  time.Duration
}

type loginLimiter = authLimiter

func newAuthLimiter(maxFailures int, failureWindow, lockout, cleanupAfter time.Duration) *authLimiter {
	return &authLimiter{
		entries:       make(map[string]*authAttempt),
		maxFailures:   maxFailures,
		failureWindow: failureWindow,
		lockout:       lockout,
		cleanupAfter:  cleanupAfter,
	}
}

func newLoginLimiter() *loginLimiter {
	return newAuthLimiter(adminLoginMaxFailures, adminLoginFailureWindow, adminLoginLockout, 30*time.Minute)
}

// userLoginLimiter 用户登录 + 注册路径共享（5 次 / 60s / 锁 5min）。
var (
	userLoginLimiter            = newAuthLimiter(userLoginMaxFailures, userLoginFailureWindow, userLoginLockout, 30*time.Minute)
	userLoginLimiterCleanupOnce sync.Once
)

// startUserLoginLimiterCleanup 后台周期清理过期 entry，避免凭据填充攻击长期占内存。
func startUserLoginLimiterCleanup(ctx context.Context) {
	userLoginLimiterCleanupOnce.Do(func() {
		go userLoginLimiter.StartCleanup(ctx)
	})
}

// IsLocked reports whether an IP is currently locked out.
// When the lockout has just expired, the entry is dropped so the IP starts fresh.
func (l *authLimiter) IsLocked(ip string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok {
		return false, 0
	}
	entry.LastSeen = now

	if !entry.LockedUntil.IsZero() {
		if now.Before(entry.LockedUntil) {
			return true, entry.LockedUntil.Sub(now)
		}
		delete(l.entries, ip)
	}
	return false, 0
}

// RecordFailure increments the IP's failure count. Returns whether this failure
// triggered a lockout, along with the lockout duration.
func (l *authLimiter) RecordFailure(ip string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok || now.Sub(entry.WindowStart) > l.failureWindow {
		entry = &authAttempt{WindowStart: now}
		l.entries[ip] = entry
	}

	entry.LastSeen = now
	entry.Failures++
	if entry.Failures >= l.maxFailures {
		entry.LockedUntil = now.Add(l.lockout)
		return true, l.lockout
	}
	return false, 0
}

// RemainingAttempts returns how many failures the IP can still incur before lockout.
func (l *authLimiter) RemainingAttempts(ip string) int {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok || now.Sub(entry.WindowStart) > l.failureWindow {
		return l.maxFailures
	}
	remaining := l.maxFailures - entry.Failures
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (l *authLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *authLimiter) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.cleanupExpired()
		}
	}
}

func (l *authLimiter) cleanupExpired() {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for ip, entry := range l.entries {
		if now.Sub(entry.LastSeen) > l.cleanupAfter && !now.Before(entry.LockedUntil) {
			delete(l.entries, ip)
		}
	}
}
