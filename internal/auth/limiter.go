package auth

import (
	"sync"
	"time"
)

const maxLimiterEntries = 4096

type Limiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	clock    func() time.Time
	failures map[string]failureWindow
}

type failureWindow struct {
	startedAt time.Time
	count     int
}

func NewLimiter(limit int, window time.Duration, clock func() time.Time) *Limiter {
	if clock == nil {
		clock = time.Now
	}
	return &Limiter{
		limit:    limit,
		window:   window,
		clock:    clock,
		failures: make(map[string]failureWindow),
	}
}

func (l *Limiter) Allow(ip string) bool {
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneExpired(now)
	failures, ok := l.current(ip)
	return !ok || failures.count < l.limit
}

func (l *Limiter) RecordFailure(ip string) {
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneExpired(now)
	l.recordFailure(ip, now)
}

// Attempt serializes the limit check and credential verification. The verify
// callback must not call methods on this Limiter.
func (l *Limiter) Attempt(ip string, verify func() bool) (allowed, success bool) {
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneExpired(now)
	if failures, ok := l.current(ip); ok && failures.count >= l.limit {
		return false, false
	}
	if verify() {
		delete(l.failures, ip)
		return true, true
	}
	l.recordFailure(ip, now)
	return true, false
}

func (l *Limiter) recordFailure(ip string, now time.Time) bool {
	failures, ok := l.current(ip)
	if !ok {
		failures.startedAt = now
		l.ensureCapacity()
	}
	if failures.count >= l.limit {
		return false
	}
	failures.count++
	l.failures[ip] = failures
	return true
}

func (l *Limiter) Reset(ip string) {
	now := l.clock()
	l.mu.Lock()
	l.pruneExpired(now)
	delete(l.failures, ip)
	l.mu.Unlock()
}

func (l *Limiter) current(ip string) (failureWindow, bool) {
	failures, ok := l.failures[ip]
	return failures, ok
}

func (l *Limiter) pruneExpired(now time.Time) {
	for ip, failures := range l.failures {
		if now.Sub(failures.startedAt) >= l.window {
			delete(l.failures, ip)
		}
	}
}

func (l *Limiter) ensureCapacity() {
	if len(l.failures) < maxLimiterEntries {
		return
	}
	var oldestIP string
	var oldestStartedAt time.Time
	for ip, failures := range l.failures {
		if oldestIP == "" || failures.startedAt.Before(oldestStartedAt) {
			oldestIP = ip
			oldestStartedAt = failures.startedAt
		}
	}
	delete(l.failures, oldestIP)
}
