package service

import (
	"sync"
	"time"
)

type sampleProgress struct {
	sampleAt   time.Time
	advancedAt time.Time
}

type freshnessTracker struct {
	mu       sync.Mutex
	entries  map[string]sampleProgress
	clock    func() time.Time
	interval time.Duration
}

func newFreshnessTracker(clock func() time.Time, interval time.Duration) *freshnessTracker {
	if clock == nil {
		clock = time.Now
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &freshnessTracker{
		entries:  make(map[string]sampleProgress),
		clock:    clock,
		interval: interval,
	}
}

func (t *freshnessTracker) Observe(samples map[string]time.Time) {
	t.ObserveAt(samples, t.clock().UTC())
}

func (t *freshnessTracker) ObserveAt(samples map[string]time.Time, now time.Time) {
	now = now.UTC()
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, sampleAt := range samples {
		t.observeLocked(key, sampleAt, now)
	}
}

func (t *freshnessTracker) Level(key string, sampleAt time.Time) Level {
	if sampleAt.IsZero() {
		return LevelCritical
	}
	now := t.clock().UTC()
	t.mu.Lock()
	defer t.mu.Unlock()
	progress, exists := t.entries[key]
	if !exists {
		return LevelCritical
	}
	return collectionLevelAt(now, progress.advancedAt, t.interval)
}

func (t *freshnessTracker) observeLocked(key string, sampleAt, now time.Time) {
	if key == "" || sampleAt.IsZero() {
		return
	}
	sampleAt = sampleAt.UTC()
	progress, exists := t.entries[key]
	if !exists || !progress.sampleAt.Equal(sampleAt) {
		t.entries[key] = sampleProgress{sampleAt: sampleAt, advancedAt: now}
	}
}
