package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/redis"
)

func TestRedisSummaryAppliesApprovedStatusBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*redis.Instance)
		level  Level
		source RedisStatusSource
	}{
		{name: "available", level: LevelNormal, source: RedisStatusNormal},
		{name: "down", mutate: func(value *redis.Instance) { value.Availability = redis.AvailabilityDown }, level: LevelCritical, source: RedisStatusAvailability},
		{name: "unknown availability", mutate: func(value *redis.Instance) { value.Availability = redis.AvailabilityUnknown }, level: LevelUnknown, source: RedisStatusAvailability},
		{name: "memory warning", mutate: func(value *redis.Instance) { value.UsedMemoryBytes = redisInt64Pointer(85) }, level: LevelWarning, source: RedisStatusMemory},
		{name: "memory critical", mutate: func(value *redis.Instance) { value.UsedMemoryBytes = redisInt64Pointer(95) }, level: LevelCritical, source: RedisStatusMemory},
		{name: "lag warning", mutate: func(value *redis.Instance) { value.Replication.WorstReplicaLagSeconds = floatPointer(5) }, level: LevelWarning, source: RedisStatusReplication},
		{name: "lag critical", mutate: func(value *redis.Instance) { value.Replication.WorstReplicaLagSeconds = floatPointer(30) }, level: LevelCritical, source: RedisStatusReplication},
		{name: "rejected connection", mutate: func(value *redis.Instance) { value.RejectedConnectionsRate = floatPointer(0.01) }, level: LevelWarning, source: RedisStatusConnection},
		{name: "collection warning", level: LevelWarning, source: RedisStatusCollection},
		{name: "collection critical", level: LevelCritical, source: RedisStatusCollection},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instance := healthyRedisMaster("fixture-a", "192.0.2.10:6379")
			if test.mutate != nil {
				test.mutate(&instance)
			}
			collection := LevelNormal
			if test.name == "collection warning" {
				collection = LevelWarning
			}
			if test.name == "collection critical" {
				collection = LevelCritical
			}
			summary := summarizeRedisInstance(instance, collection)
			if summary.Status != test.level || summary.StatusSource != test.source {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}
}

func TestRedisServiceMetaUsesLatestSampleTime(t *testing.T) {
	now := time.Date(2026, time.August, 7, 8, 0, 0, 0, time.UTC)
	older := healthyRedisMaster("fixture-older", "192.0.2.10:6379")
	older.CollectionTracked = true
	older.ReportedAt = now.Add(-2 * time.Minute)
	latest := healthyRedisMaster("fixture-latest", "192.0.2.11:6379")
	latest.CollectionTracked = true
	latest.ReportedAt = now.Add(-time.Minute)
	provider := &recordingRedisProvider{snapshot: redis.Snapshot{Instances: []redis.Instance{older, latest}}}
	service := NewRedis(provider, cache.New(func() time.Time { return now }), RedisOptions{Clock: func() time.Time { return now }})

	_, meta, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if !meta.CollectedAt.Equal(latest.ReportedAt) {
		t.Fatalf("CollectedAt = %s, want latest sample %s", meta.CollectedAt, latest.ReportedAt)
	}
}

func TestRedisSummaryAppliesClusterAndSlaveReplicationRules(t *testing.T) {
	master := healthyRedisMaster("fixture-master", "192.0.2.10:6379")
	master.Replication.ConnectedReplicas = redisInt64Pointer(0)
	if got := summarizeRedisInstance(master, LevelNormal); got.Status != LevelCritical || got.StatusSource != RedisStatusReplication {
		t.Fatalf("cluster master = %#v", got)
	}
	*master.ClusterEnabled = false
	if got := summarizeRedisInstance(master, LevelNormal); got.Status != LevelNormal {
		t.Fatalf("standalone master = %#v", got)
	}

	slave := healthyRedisSlave("fixture-slave", "192.0.2.11:6379")
	slave.Replication.MasterLinkUp = nil
	if got := summarizeRedisInstance(slave, LevelNormal); got.Status != LevelUnknown || got.StatusSource != RedisStatusReplication {
		t.Fatalf("missing slave link = %#v", got)
	}
	slave.Replication.MasterLinkUp = boolPointer(false)
	if got := summarizeRedisInstance(slave, LevelNormal); got.Status != LevelCritical {
		t.Fatalf("down slave link = %#v", got)
	}
}

func TestRedisSummaryHandlesMemoryWithoutInventingPercent(t *testing.T) {
	instance := healthyRedisMaster("fixture-a", "192.0.2.10:6379")
	instance.MaxMemoryBytes = redisInt64Pointer(0)
	summary := summarizeRedisInstance(instance, LevelNormal)
	if summary.MemoryUsagePercent != nil || summary.Status != LevelNormal {
		t.Fatalf("unlimited memory = %#v", summary)
	}
	instance.MaxMemoryBytes = redisInt64Pointer(100)
	instance.UsedMemoryBytes = nil
	summary = summarizeRedisInstance(instance, LevelNormal)
	if summary.MemoryUsagePercent != nil || summary.Status != LevelUnknown || summary.StatusSource != RedisStatusMemory {
		t.Fatalf("missing used memory = %#v", summary)
	}
}

func TestRedisSummaryUsesSourcePriorityForEqualLevels(t *testing.T) {
	instance := healthyRedisMaster("fixture-a", "192.0.2.10:6379")
	instance.Availability = redis.AvailabilityDown
	instance.Replication.ConnectedReplicas = redisInt64Pointer(0)
	instance.UsedMemoryBytes = redisInt64Pointer(100)
	summary := summarizeRedisInstance(instance, LevelCritical)
	if summary.Status != LevelCritical || summary.StatusSource != RedisStatusAvailability {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestRedisServiceSharesCacheAndTracksSampleProgress(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &redisTestClock{now: now}
	instance := healthyRedisMaster("fixture-a", "192.0.2.10:6379")
	instance.ReportedAt = now.Add(-time.Hour)
	provider := &recordingRedisProvider{snapshot: redis.Snapshot{Instances: []redis.Instance{instance}}}
	service := NewRedis(provider, cache.New(clock.Now), RedisOptions{SnapshotTTL: 15 * time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	assertLevel := func(want Level) {
		t.Helper()
		page, _, err := service.Instances(context.Background(), RedisQuery{Page: 1, PageSize: 20})
		if err != nil || len(page.Instances) != 1 || page.Instances[0].CollectionLevel != want {
			t.Fatalf("page = %#v, err = %v", page, err)
		}
	}
	assertLevel(LevelNormal)
	if _, _, err := service.Overview(context.Background()); err != nil || provider.calls != 1 {
		t.Fatalf("cache calls = %d, err = %v", provider.calls, err)
	}
	clock.Advance(30 * time.Second)
	assertLevel(LevelWarning)
	clock.Advance(45 * time.Second)
	assertLevel(LevelCritical)
	provider.snapshot.Instances[0].ReportedAt = instance.ReportedAt.Add(-time.Second)
	clock.Advance(16 * time.Second)
	assertLevel(LevelNormal)
}

func TestRedisServiceReturnsStaleDeepCopies(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &redisTestClock{now: now}
	provider := &recordingRedisProvider{snapshot: redis.Snapshot{Instances: []redis.Instance{healthyRedisMaster("fixture-a", "192.0.2.10:6379")}}}
	service := NewRedis(provider, cache.New(clock.Now), RedisOptions{SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})
	first, _, err := service.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	*first.Instances[0].UsedMemoryBytes = 999
	clock.Advance(2 * time.Second)
	provider.err = redis.ErrUnavailable
	second, meta, err := service.snapshot(context.Background())
	if err != nil || !meta.Stale || *second.Instances[0].UsedMemoryBytes == 999 {
		t.Fatalf("snapshot = %#v, meta = %#v, err = %v", second, meta, err)
	}
}

func TestRedisInstancesFiltersSortsAndPaginates(t *testing.T) {
	snapshot := redis.Snapshot{Instances: []redis.Instance{
		healthyRedisMaster("fixture-c", "192.0.2.10:6380"),
		healthyRedisMaster("fixture-a", "192.0.2.2:6379"),
		healthyRedisSlave("fixture-b", "192.0.2.10:6379"),
	}}
	snapshot.Instances[0].UsedMemoryBytes = nil
	service := newRedisServiceWithSnapshot(snapshot)
	page, _, err := service.Instances(context.Background(), RedisQuery{Search: "192.0.2", Sort: "instance", Order: "asc", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"192.0.2.2:6379", "192.0.2.10:6379", "192.0.2.10:6380"}
	got := []string{page.Instances[0].Address, page.Instances[1].Address, page.Instances[2].Address}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses = %#v", got)
	}
	page, _, err = service.Instances(context.Background(), RedisQuery{Role: redis.RoleMaster, Sort: "memory", Order: "desc", Page: 1, PageSize: 20})
	if err != nil || len(page.Instances) != 2 || page.Instances[1].UsedMemoryBytes != nil {
		t.Fatalf("missing-last page = %#v, err = %v", page, err)
	}
	if _, _, err := service.Instances(context.Background(), RedisQuery{Sort: "invalid", Page: 1, PageSize: 20}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid query error = %v", err)
	}
}

func TestRedisInstancesRejectsOverflowPageOffset(t *testing.T) {
	service := newRedisServiceWithSnapshot(redis.Snapshot{Instances: []redis.Instance{
		healthyRedisMaster("fixture-a", "192.0.2.10:6379"),
	}})
	_, _, err := service.Instances(context.Background(), RedisQuery{
		Page: math.MaxInt, PageSize: 20,
	})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("overflow page error = %v", err)
	}
}

type recordingRedisProvider struct {
	snapshot redis.Snapshot
	err      error
	calls    int
}

func (provider *recordingRedisProvider) RedisSnapshot(context.Context) (redis.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return redis.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type redisTestClock struct{ now time.Time }

func (clock *redisTestClock) Now() time.Time              { return clock.now }
func (clock *redisTestClock) Advance(value time.Duration) { clock.now = clock.now.Add(value) }

func healthyRedisMaster(id, address string) redis.Instance {
	return redis.Instance{
		ID:                   id,
		Address:              address,
		Availability:         redis.AvailabilityUp,
		Role:                 redis.RoleMaster,
		ClusterEnabled:       boolPointer(true),
		UptimeSeconds:        redisInt64Pointer(3600),
		UsedMemoryBytes:      redisInt64Pointer(40),
		MaxMemoryBytes:       redisInt64Pointer(100),
		ConnectedClients:     redisInt64Pointer(10),
		MaxClients:           redisInt64Pointer(100),
		QPS:                  floatPointer(20),
		Keys:                 redisInt64Pointer(100),
		EvictedKeysPerSecond: floatPointer(0),
		Replication: redis.Replication{
			ConnectedReplicas:      redisInt64Pointer(1),
			WorstReplicaLagSeconds: floatPointer(1),
		},
		CollectionTracked: true,
		ReportedAt:        time.Date(2026, 8, 1, 7, 59, 0, 0, time.UTC),
	}
}

func healthyRedisSlave(id, address string) redis.Instance {
	instance := healthyRedisMaster(id, address)
	instance.Role = redis.RoleSlave
	instance.Replication = redis.Replication{
		MasterLinkUp:           boolPointer(true),
		MasterLastIOSecondsAgo: floatPointer(1),
		MasterSyncInProgress:   boolPointer(false),
	}
	return instance
}

func newRedisServiceWithSnapshot(snapshot redis.Snapshot) *RedisService {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	return NewRedis(
		&recordingRedisProvider{snapshot: snapshot},
		cache.New(func() time.Time { return now }),
		RedisOptions{
			SnapshotTTL:        15 * time.Second,
			CollectionInterval: 15 * time.Second,
			MaxStale:           time.Minute,
			Clock:              func() time.Time { return now },
		},
	)
}

func redisInt64Pointer(value int64) *int64 { return &value }
