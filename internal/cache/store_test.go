package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
)

func TestStoreReturnsFreshEntryWithoutReloading(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	loads := 0
	loader := func(context.Context) (any, error) {
		loads++
		return "first", nil
	}

	first, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
	if err != nil {
		t.Fatalf("first GetOrLoad() error = %v", err)
	}
	clock.Advance(30 * time.Second)
	second, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
	if err != nil {
		t.Fatalf("second GetOrLoad() error = %v", err)
	}

	if loads != 1 {
		t.Fatalf("loader calls = %d, want 1", loads)
	}
	if first.Value != "first" || first.State != cache.Fresh || !first.StoredAt.Equal(clock.Start()) {
		t.Fatalf("first result = %#v", first)
	}
	if second != first {
		t.Fatalf("second result = %#v, want %#v", second, first)
	}
}

func TestStoreReloadsExpiredEntry(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	loads := 0
	loader := func(context.Context) (any, error) {
		loads++
		return loads, nil
	}

	if _, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader); err != nil {
		t.Fatalf("first GetOrLoad() error = %v", err)
	}
	clock.Advance(time.Minute)
	result, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
	if err != nil {
		t.Fatalf("expired GetOrLoad() error = %v", err)
	}

	if loads != 2 || result.Value != 2 || result.State != cache.Fresh {
		t.Fatalf("loader calls/result = %d/%#v", loads, result)
	}
	if !result.StoredAt.Equal(clock.Now()) {
		t.Fatalf("StoredAt = %s, want %s", result.StoredAt, clock.Now())
	}
}

func TestStoreReturnsStaleEntryWhenReloadFailsWithinMaxStale(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	if _, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return "cached", nil
	}); err != nil {
		t.Fatalf("prime GetOrLoad() error = %v", err)
	}

	clock.Advance(2 * time.Minute)
	result, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return nil, errors.New("provider unavailable")
	})
	if err != nil {
		t.Fatalf("stale GetOrLoad() error = %v", err)
	}
	if result.Value != "cached" || result.State != cache.Stale || !result.StoredAt.Equal(clock.Start()) {
		t.Fatalf("stale result = %#v", result)
	}
}

func TestStoreReturnsLoaderErrorWhenEntryExceedsMaxStale(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	if _, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return "cached", nil
	}); err != nil {
		t.Fatalf("prime GetOrLoad() error = %v", err)
	}

	wantErr := errors.New("provider unavailable")
	clock.Advance(5*time.Minute + time.Nanosecond)
	_, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetOrLoad() error = %v, want %v", err, wantErr)
	}
}

func TestStoreCoalescesConcurrentMissesPerKey(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	started := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32
	loader := func(context.Context) (any, error) {
		if loads.Add(1) == 1 {
			close(started)
		}
		<-release
		return "shared", nil
	}

	const callers = 20
	results := make(chan cache.Result, callers)
	errorsCh := make(chan error, callers)
	waiting := make(chan struct{}, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	begin := make(chan struct{})
	for range callers {
		go func() {
			ready.Done()
			<-begin
			ctx := &observedContext{Context: context.Background(), observed: waiting}
			result, err := store.GetOrLoad(ctx, "hosts", time.Minute, 5*time.Minute, loader)
			results <- result
			errorsCh <- err
		}()
	}
	ready.Wait()
	close(begin)
	<-started
	for range callers {
		<-waiting
	}
	close(release)

	for range callers {
		if err := <-errorsCh; err != nil {
			t.Fatalf("GetOrLoad() error = %v", err)
		}
		result := <-results
		if result.Value != "shared" || result.State != cache.Fresh {
			t.Fatalf("result = %#v", result)
		}
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

func TestStoreAppliesMaxStalePerWaitingCaller(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	if _, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return "cached", nil
	}); err != nil {
		t.Fatalf("prime GetOrLoad() error = %v", err)
	}
	clock.Advance(2 * time.Minute)

	started := make(chan struct{})
	release := make(chan struct{})
	wantErr := errors.New("provider unavailable")
	loader := func(context.Context) (any, error) {
		close(started)
		<-release
		return nil, wantErr
	}
	type outcome struct {
		result cache.Result
		err    error
	}
	leaderDone := make(chan outcome, 1)
	go func() {
		result, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
		leaderDone <- outcome{result: result, err: err}
	}()
	<-started

	waiting := make(chan struct{}, 1)
	waiterDone := make(chan outcome, 1)
	go func() {
		ctx := &observedContext{Context: context.Background(), observed: waiting}
		result, err := store.GetOrLoad(ctx, "hosts", time.Minute, time.Minute, loader)
		waiterDone <- outcome{result: result, err: err}
	}()
	<-waiting
	close(release)

	leader := <-leaderDone
	if leader.err != nil || leader.result.State != cache.Stale {
		t.Fatalf("leader result/error = %#v/%v, want stale result", leader.result, leader.err)
	}
	waiter := <-waiterDone
	if !errors.Is(waiter.err, wantErr) {
		t.Fatalf("waiter result/error = %#v/%v, want loader error", waiter.result, waiter.err)
	}
}

func TestStoreCanceledWaiterDoesNotCancelSharedLoad(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(ctx context.Context) (any, error) {
		close(started)
		<-release
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return "shared", nil
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
		firstDone <- err
	}()
	<-started

	waiterCtx, cancel := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	go func() {
		_, err := store.GetOrLoad(waiterCtx, "hosts", time.Minute, 5*time.Minute, loader)
		waiterDone <- err
	}()
	cancel()
	if err := <-waiterDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting GetOrLoad() error = %v, want context.Canceled", err)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("shared load error = %v", err)
	}
	result, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, loader)
	if err != nil || result.Value != "shared" {
		t.Fatalf("cached result/error = %#v/%v", result, err)
	}
}

func TestStoreCancelsLoadWhenLastWaiterLeaves(t *testing.T) {
	clock := newTestClock(time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC))
	store := cache.New(clock.Now)
	loaderStarted := make(chan struct{})
	loaderCanceled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	callerDone := make(chan error, 1)
	go func() {
		_, err := store.GetOrLoad(ctx, "hosts", time.Minute, 5*time.Minute, func(loadCtx context.Context) (any, error) {
			close(loaderStarted)
			<-loadCtx.Done()
			close(loaderCanceled)
			return nil, loadCtx.Err()
		})
		callerDone <- err
	}()
	<-loaderStarted
	cancel()
	if err := <-callerDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("GetOrLoad() error = %v, want context.Canceled", err)
	}
	select {
	case <-loaderCanceled:
	case <-time.After(time.Second):
		t.Fatal("loader was not canceled after its last waiter left")
	}

	result, err := store.GetOrLoad(context.Background(), "hosts", time.Minute, 5*time.Minute, func(context.Context) (any, error) {
		return "replacement", nil
	})
	if err != nil || result.Value != "replacement" {
		t.Fatalf("replacement result/error = %#v/%v", result, err)
	}
}

type testClock struct {
	mu    sync.Mutex
	start time.Time
	now   time.Time
}

type observedContext struct {
	context.Context
	once     sync.Once
	observed chan<- struct{}
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { c.observed <- struct{}{} })
	return c.Context.Done()
}

func newTestClock(start time.Time) *testClock {
	return &testClock{start: start, now: start}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Start() time.Time {
	return c.start
}

func (c *testClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}
