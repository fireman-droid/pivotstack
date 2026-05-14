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
)

type loginAttempt struct {
	Failures    int
	WindowStart time.Time
	LockedUntil time.Time
	LastSeen    time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginAttempt
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: make(map[string]*loginAttempt)}
}

// IsLocked reports whether an IP is currently locked out.
// When the lockout has just expired, the entry is dropped so the IP starts fresh.
func (l *loginLimiter) IsLocked(ip string) (bool, time.Duration) {
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
func (l *loginLimiter) RecordFailure(ip string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok || now.Sub(entry.WindowStart) > adminLoginFailureWindow {
		entry = &loginAttempt{WindowStart: now}
		l.entries[ip] = entry
	}

	entry.LastSeen = now
	entry.Failures++
	if entry.Failures >= adminLoginMaxFailures {
		entry.LockedUntil = now.Add(adminLoginLockout)
		return true, adminLoginLockout
	}
	return false, 0
}

// RemainingAttempts returns how many failures the IP can still incur before lockout.
func (l *loginLimiter) RemainingAttempts(ip string) int {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok || now.Sub(entry.WindowStart) > adminLoginFailureWindow {
		return adminLoginMaxFailures
	}
	remaining := adminLoginMaxFailures - entry.Failures
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (l *loginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *loginLimiter) StartCleanup(ctx context.Context) {
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

func (l *loginLimiter) cleanupExpired() {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for ip, entry := range l.entries {
		if now.Sub(entry.LastSeen) > 30*time.Minute && !now.Before(entry.LockedUntil) {
			delete(l.entries, ip)
		}
	}
}
