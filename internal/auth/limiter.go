package auth

import (
	"sync"
	"time"
)

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
	failures, ok := l.current(ip, now)
	return !ok || failures.count < l.limit
}

func (l *Limiter) RecordFailure(ip string) {
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	failures, ok := l.current(ip, now)
	if !ok {
		failures.startedAt = now
	}
	failures.count++
	l.failures[ip] = failures
}

func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	delete(l.failures, ip)
	l.mu.Unlock()
}

func (l *Limiter) current(ip string, now time.Time) (failureWindow, bool) {
	failures, ok := l.failures[ip]
	if !ok {
		return failureWindow{}, false
	}
	if now.Sub(failures.startedAt) >= l.window {
		delete(l.failures, ip)
		return failureWindow{}, false
	}
	return failures, true
}
