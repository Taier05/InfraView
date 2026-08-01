package mock

import (
	"context"
	"time"

	"github.com/Taier05/InfraView/internal/redis"
)

type redisProvider struct {
	clock func() time.Time
}

func NewRedis(clock func() time.Time) redis.Provider {
	return &redisProvider{clock: clock}
}

func (provider *redisProvider) RedisSnapshot(context.Context) (redis.Snapshot, error) {
	now := time.Time{}
	if provider.clock != nil {
		now = provider.clock()
	}
	instances := []redis.Instance{
		redisFixture("fixture-ident-a", "fixture-redis-a", "192.0.2.20:6379", now, redis.AvailabilityUp, redis.RoleMaster, true),
		redisFixture("fixture-ident-b", "fixture-redis-b", "192.0.2.21:6379", now, redis.AvailabilityUp, redis.RoleSlave, true),
		redisFixture("fixture-ident-c", "fixture-redis-c", "192.0.2.22:6379", now, redis.AvailabilityUp, redis.RoleMaster, true),
		redisFixture("fixture-ident-d", "fixture-redis-d", "192.0.2.23:6379", now, redis.AvailabilityUp, redis.RoleSlave, true),
		redisFixture("fixture-ident-e", "fixture-redis-e", "198.51.100.20:6379", now, redis.AvailabilityUp, redis.RoleMaster, true),
		redisFixture("fixture-ident-f", "fixture-redis-f", "198.51.100.21:6379", now, redis.AvailabilityUnknown, redis.RoleUnknown, true),
		redisFixture("fixture-ident-g", "fixture-redis-g", "198.51.100.22:6379", now, redis.AvailabilityUp, redis.RoleMaster, true),
		redisFixture("fixture-ident-h", "fixture-redis-h", "198.51.100.23:6379", now, redis.AvailabilityDown, redis.RoleSlave, true),
	}

	instances[1].Replication.MasterLinkUp = pointer(true)
	instances[1].Replication.MasterLastIOSecondsAgo = pointer(1.0)
	instances[2].UsedMemoryBytes = pointer(int64(90))
	instances[2].MaxMemoryBytes = pointer(int64(100))
	instances[2].Replication.WorstReplicaLagSeconds = pointer(8.0)
	instances[3].Replication.MasterLinkUp = pointer(false)
	instances[3].Replication.MasterLastIOSecondsAgo = pointer(35.0)
	instances[3].Replication.MasterSyncInProgress = pointer(true)
	instances[4].MaxMemoryBytes = pointer(int64(0))
	instances[4].RejectedConnectionsRate = pointer(1.0)
	instances[5].ClusterEnabled = nil
	instances[5].UptimeSeconds = nil
	instances[5].UsedMemoryBytes = nil
	instances[5].MaxMemoryBytes = nil
	instances[5].ConnectedClients = nil
	instances[5].MaxClients = nil
	instances[5].BlockedClients = nil
	instances[5].QPS = nil
	instances[5].HitRate = nil
	instances[5].Keys = nil
	instances[5].ExpiredKeysPerSecond = nil
	instances[5].EvictedKeysPerSecond = nil
	instances[5].RejectedConnectionsRate = nil
	instances[5].Replication = redis.Replication{}
	instances[6].Replication.ConnectedReplicas = pointer(int64(0))
	instances[7].Replication.MasterLinkUp = pointer(false)

	return redis.Snapshot{Instances: instances}.Clone(), nil
}

func redisFixture(ident, instance, address string, reportedAt time.Time, availability redis.Availability, role redis.Role, cluster bool) redis.Instance {
	fixture := redis.Instance{
		ID:                      redis.StableInstanceID(ident, instance, address),
		Address:                 address,
		Availability:            availability,
		Role:                    role,
		ClusterEnabled:          pointer(cluster),
		UptimeSeconds:           pointer(int64(172800)),
		UsedMemoryBytes:         pointer(int64(40)),
		MaxMemoryBytes:          pointer(int64(100)),
		ConnectedClients:        pointer(int64(24)),
		MaxClients:              pointer(int64(160)),
		BlockedClients:          pointer(int64(0)),
		QPS:                     pointer(32.0),
		HitRate:                 pointer(0.98),
		Keys:                    pointer(int64(4096)),
		ExpiredKeysPerSecond:    pointer(0.5),
		EvictedKeysPerSecond:    pointer(0.0),
		RejectedConnectionsRate: pointer(0.0),
		CollectionTracked:       true,
		ReportedAt:              reportedAt,
	}
	if role == redis.RoleMaster {
		fixture.Replication.ConnectedReplicas = pointer(int64(1))
		fixture.Replication.WorstReplicaLagSeconds = pointer(1.0)
	}
	if role == redis.RoleSlave {
		fixture.Replication.MasterLinkUp = pointer(true)
		fixture.Replication.MasterLastIOSecondsAgo = pointer(1.0)
		fixture.Replication.MasterSyncInProgress = pointer(false)
	}
	return fixture
}

func pointer[T any](value T) *T {
	return &value
}
