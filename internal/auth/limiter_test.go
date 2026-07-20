package auth

import (
	"fmt"
	"testing"
	"time"
)

const limiterCapacityForTest = 4096

func TestLimiterBlocksSixthFailurePerIP(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(5, time.Minute, func() time.Time { return now })

	for attempt := 1; attempt <= 5; attempt++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatalf("attempt %d was blocked", attempt)
		}
		limiter.RecordFailure("192.0.2.1")
	}
	if limiter.Allow("192.0.2.1") {
		t.Fatal("sixth attempt was allowed")
	}
	if !limiter.Allow("192.0.2.2") {
		t.Fatal("one IP blocked another IP")
	}
}

func TestLimiterResetAndWindowRecovery(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(1, time.Minute, func() time.Time { return now })

	limiter.RecordFailure("192.0.2.1")
	if limiter.Allow("192.0.2.1") {
		t.Fatal("failed IP was not blocked")
	}
	limiter.Reset("192.0.2.1")
	if !limiter.Allow("192.0.2.1") {
		t.Fatal("successful login did not reset failures")
	}

	limiter.RecordFailure("192.0.2.1")
	now = now.Add(time.Minute)
	if !limiter.Allow("192.0.2.1") {
		t.Fatal("IP did not recover in the next window")
	}
}

func TestLimiterGloballyPrunesExpiredEntriesOnEveryOperation(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(5, time.Minute, func() time.Time { return now })
	limiter.RecordFailure("192.0.2.1")
	limiter.RecordFailure("192.0.2.2")

	now = now.Add(time.Minute)
	if !limiter.Allow("192.0.2.3") {
		t.Fatal("unrelated IP was blocked")
	}
	if len(limiter.failures) != 0 {
		t.Fatalf("expired entries after Allow = %d, want 0", len(limiter.failures))
	}

	limiter.RecordFailure("192.0.2.4")
	now = now.Add(time.Minute)
	limiter.Reset("192.0.2.5")
	if len(limiter.failures) != 0 {
		t.Fatalf("expired entries after Reset = %d, want 0", len(limiter.failures))
	}
}

func TestLimiterBoundsIPEntriesAndEvictsOldestWindow(t *testing.T) {
	if maxLimiterEntries != limiterCapacityForTest {
		t.Fatalf("maxLimiterEntries = %d, want %d", maxLimiterEntries, limiterCapacityForTest)
	}
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(5, 24*time.Hour, func() time.Time { return now })
	for index := 0; index <= limiterCapacityForTest; index++ {
		ip := fmt.Sprintf("198.51.100.%d", index)
		if !limiter.RecordFailure(ip) {
			t.Fatalf("new IP %q was unexpectedly blocked", ip)
		}
		now = now.Add(time.Millisecond)
	}

	if got := len(limiter.failures); got > limiterCapacityForTest {
		t.Fatalf("limiter entries = %d, want at most %d", got, limiterCapacityForTest)
	}
	if _, exists := limiter.failures["198.51.100.0"]; exists {
		t.Fatal("oldest failure window was not evicted")
	}
	if _, exists := limiter.failures[fmt.Sprintf("198.51.100.%d", limiterCapacityForTest)]; !exists {
		t.Fatal("newest failure window was evicted")
	}
}
