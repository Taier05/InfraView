package service

import (
	"sync"
	"testing"
	"time"
)

func TestFreshnessTrackerUsesObservedProgressInsteadOfAbsoluteSampleAge(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	tracker := newFreshnessTracker(clock, 15*time.Second)
	oldSample := now.Add(-time.Hour)

	tracker.Observe(map[string]time.Time{"fixture": oldSample})
	if got := tracker.Level("fixture", oldSample); got != LevelNormal {
		t.Fatalf("first observation level = %q, want %q", got, LevelNormal)
	}

	now = now.Add(30 * time.Second)
	tracker.Observe(map[string]time.Time{"fixture": oldSample})
	if got := tracker.Level("fixture", oldSample); got != LevelWarning {
		t.Fatalf("frozen at two cycles level = %q, want %q", got, LevelWarning)
	}

	now = now.Add(45 * time.Second)
	tracker.Observe(map[string]time.Time{"fixture": oldSample})
	if got := tracker.Level("fixture", oldSample); got != LevelCritical {
		t.Fatalf("frozen at five cycles level = %q, want %q", got, LevelCritical)
	}

	advancedSample := oldSample.Add(15 * time.Second)
	tracker.Observe(map[string]time.Time{"fixture": advancedSample})
	if got := tracker.Level("fixture", advancedSample); got != LevelNormal {
		t.Fatalf("advanced sample level = %q, want %q", got, LevelNormal)
	}

	now = now.Add(29 * time.Second)
	tracker.Observe(map[string]time.Time{"fixture": advancedSample})
	if got := tracker.Level("fixture", advancedSample); got != LevelNormal {
		t.Fatalf("before two cycles level = %q, want %q", got, LevelNormal)
	}
}

func TestFreshnessTrackerRebasesWhenSampleTimeMovesBackward(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	tracker := newFreshnessTracker(clock, 15*time.Second)
	sample := now.Add(-time.Minute)
	tracker.Observe(map[string]time.Time{"fixture": sample})

	now = now.Add(75 * time.Second)
	if got := tracker.Level("fixture", sample); got != LevelCritical {
		t.Fatalf("frozen level = %q, want %q", got, LevelCritical)
	}

	rebased := sample.Add(-time.Hour)
	tracker.Observe(map[string]time.Time{"fixture": rebased})
	if got := tracker.Level("fixture", rebased); got != LevelNormal {
		t.Fatalf("rebased level = %q, want %q", got, LevelNormal)
	}
}

func TestFreshnessTrackerReadDoesNotTreatOlderCachedSampleAsProgress(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	tracker := newFreshnessTracker(clock, 15*time.Second)
	currentSample := now.Add(-time.Minute)
	tracker.Observe(map[string]time.Time{"fixture": currentSample})

	now = now.Add(30 * time.Second)
	olderCachedSample := currentSample.Add(-time.Minute)
	if got := tracker.Level("fixture", olderCachedSample); got != LevelWarning {
		t.Fatalf("older cached sample level = %q, want %q", got, LevelWarning)
	}

	tracker.Observe(map[string]time.Time{"fixture": olderCachedSample})
	if got := tracker.Level("fixture", olderCachedSample); got != LevelNormal {
		t.Fatalf("observed clock rebase level = %q, want %q", got, LevelNormal)
	}
}

func TestFreshnessTrackerTreatsMissingSampleAsCritical(t *testing.T) {
	tracker := newFreshnessTracker(time.Now, 15*time.Second)
	if got := tracker.Level("missing", time.Time{}); got != LevelCritical {
		t.Fatalf("missing sample level = %q, want %q", got, LevelCritical)
	}
}

func TestFreshnessTrackerSupportsConcurrentObservationAndReads(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	tracker := newFreshnessTracker(func() time.Time { return now }, 15*time.Second)
	sample := now.Add(-time.Minute)
	tracker.Observe(map[string]time.Time{"fixture": sample})

	var wait sync.WaitGroup
	for index := 0; index < 32; index++ {
		wait.Add(2)
		go func() {
			defer wait.Done()
			tracker.Observe(map[string]time.Time{"fixture": sample})
		}()
		go func() {
			defer wait.Done()
			if got := tracker.Level("fixture", sample); got != LevelNormal {
				t.Errorf("concurrent level = %q, want %q", got, LevelNormal)
			}
		}()
	}
	wait.Wait()
}
