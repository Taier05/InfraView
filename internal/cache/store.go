package cache

import (
	"context"
	"sync"
	"time"
)

type State string

const (
	Fresh State = "fresh"
	Stale State = "stale"
)

type Result struct {
	Value    any
	State    State
	StoredAt time.Time
}

type Loader func(context.Context) (any, error)

type Store struct {
	mu       sync.Mutex
	entries  map[string]entry
	inFlight map[string]*call
	clock    func() time.Time
}

type entry struct {
	value    any
	storedAt time.Time
}

type call struct {
	done      chan struct{}
	cancel    context.CancelFunc
	waiters   int
	abandoned bool
	value     any
	err       error
	loadedAt  time.Time
	stale     *entry
}

func New(clock func() time.Time) *Store {
	return &Store{clock: clock}
}

func (s *Store) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	maxStale time.Duration,
	loader Loader,
) (Result, error) {
	now := s.now()

	s.mu.Lock()
	s.initialize()
	if cached, ok := s.entries[key]; ok && now.Sub(cached.storedAt) < ttl {
		s.mu.Unlock()
		return Result{Value: cached.value, State: Fresh, StoredAt: cached.storedAt}, nil
	}
	if existing, ok := s.inFlight[key]; ok {
		existing.waiters++
		s.mu.Unlock()
		return s.wait(ctx, key, maxStale, existing)
	}

	loadCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	pending := &call{done: make(chan struct{}), cancel: cancel, waiters: 1}
	s.inFlight[key] = pending
	s.mu.Unlock()

	go s.load(loadCtx, key, loader, pending)
	return s.wait(ctx, key, maxStale, pending)
}

func (s *Store) load(ctx context.Context, key string, loader Loader, pending *call) {
	defer pending.cancel()
	value, err := loader(ctx)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	pending.value = value
	pending.err = err
	pending.loadedAt = now
	if pending.abandoned || s.inFlight[key] != pending {
		close(pending.done)
		return
	}
	if err == nil {
		s.entries[key] = entry{value: value, storedAt: now}
	} else if cached, ok := s.entries[key]; ok {
		cachedCopy := cached
		pending.stale = &cachedCopy
	}
	delete(s.inFlight, key)
	close(pending.done)
}

func (s *Store) initialize() {
	if s.entries == nil {
		s.entries = make(map[string]entry)
	}
	if s.inFlight == nil {
		s.inFlight = make(map[string]*call)
	}
}

func (s *Store) now() time.Time {
	if s.clock == nil {
		return time.Now()
	}
	return s.clock()
}

func (s *Store) wait(ctx context.Context, key string, maxStale time.Duration, pending *call) (Result, error) {
	select {
	case <-pending.done:
		if pending.err == nil {
			return Result{Value: pending.value, State: Fresh, StoredAt: pending.loadedAt}, nil
		}
		if pending.stale != nil && pending.loadedAt.Sub(pending.stale.storedAt) <= maxStale {
			return Result{Value: pending.stale.value, State: Stale, StoredAt: pending.stale.storedAt}, nil
		}
		return Result{}, pending.err
	case <-ctx.Done():
		s.mu.Lock()
		if s.inFlight[key] == pending {
			pending.waiters--
			if pending.waiters == 0 {
				delete(s.inFlight, key)
				pending.abandoned = true
				pending.cancel()
			}
		}
		s.mu.Unlock()
		return Result{}, ctx.Err()
	}
}
