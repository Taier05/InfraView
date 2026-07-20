package auth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const limiterCapacityForTest = 4096

type limiterContract interface {
	Allow(ip string) bool
	RecordFailure(ip string)
	Reset(ip string)
}

var _ limiterContract = (*Limiter)(nil)

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
		limiter.RecordFailure(ip)
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

func TestLimiterConcurrentAttemptRunsOnlyOneVerificationAtRemainingSlot(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(5, time.Minute, func() time.Time { return now })
	for range 4 {
		limiter.RecordFailure("192.0.2.1")
	}

	const attempts = 64
	start := make(chan struct{})
	var verifications atomic.Int64
	var allowed atomic.Int64
	var wait sync.WaitGroup
	wait.Add(attempts)
	for range attempts {
		go func() {
			defer wait.Done()
			<-start
			wasAllowed, _ := limiter.Attempt("192.0.2.1", func() bool {
				verifications.Add(1)
				return false
			})
			if wasAllowed {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wait.Wait()

	if got := verifications.Load(); got != 1 {
		t.Fatalf("credential verifications = %d, want 1", got)
	}
	if got := allowed.Load(); got != 1 {
		t.Fatalf("allowed attempts = %d, want 1", got)
	}
}

func TestLimiterSuccessfulAttemptResetsFailures(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	limiter := NewLimiter(5, time.Minute, func() time.Time { return now })
	for range 4 {
		limiter.RecordFailure("192.0.2.1")
	}

	allowed, success := limiter.Attempt("192.0.2.1", func() bool { return true })
	if !allowed || !success {
		t.Fatalf("successful attempt = allowed %v, success %v", allowed, success)
	}
	if len(limiter.failures) != 0 {
		t.Fatalf("failure windows after success = %d, want 0", len(limiter.failures))
	}
}
