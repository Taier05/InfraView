package auth

import (
	"testing"
	"time"
)

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
