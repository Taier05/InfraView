package service

import (
	"time"

	"github.com/Taier05/InfraView/internal/redis"
)

type RedisOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type RedisQuery struct {
	Search   string
	Role     redis.Role
	Status   Level
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type RedisStatusSource string

const (
	RedisStatusAvailability RedisStatusSource = "availability"
	RedisStatusReplication  RedisStatusSource = "replication"
	RedisStatusMemory       RedisStatusSource = "memory"
	RedisStatusConnection   RedisStatusSource = "connection"
	RedisStatusCollection   RedisStatusSource = "collection"
	RedisStatusNormal       RedisStatusSource = "normal"
	RedisStatusUnknown      RedisStatusSource = "unknown"
)

type RedisAlertCount struct {
	Warning  int
	Critical int
}

type RedisOverviewAlerts struct {
	Availability RedisAlertCount
	Memory       RedisAlertCount
	Connection   RedisAlertCount
	Replication  RedisAlertCount
}

type RedisRoleCounts struct {
	Master  int
	Slave   int
	Unknown int
}

type RedisOverview struct {
	Total             int
	Normal            int
	Warning           int
	Critical          int
	Unknown           int
	AffectedInstances int
	WarningInstances  int
	CriticalInstances int
	Roles             RedisRoleCounts
	Alerts            RedisOverviewAlerts
}

type RedisReplicationSummary struct {
	ConnectedReplicas      *int64
	MasterLinkUp           *bool
	MasterLastIOSecondsAgo *float64
	MasterSyncInProgress   *bool
	WorstReplicaLagSeconds *float64
}

type RedisInstanceSummary struct {
	ID                      string
	Address                 string
	Availability            redis.Availability
	Role                    redis.Role
	ClusterEnabled          *bool
	UsedMemoryBytes         *int64
	MaxMemoryBytes          *int64
	MemoryUsagePercent      *float64
	ConnectedClients        *int64
	MaxClients              *int64
	ConnectionUsagePercent  *float64
	BlockedClients          *int64
	QPS                     *float64
	HitRate                 *float64
	Keys                    *int64
	ExpiredKeysPerSecond    *float64
	EvictedKeysPerSecond    *float64
	RejectedConnectionsRate *float64
	Replication             RedisReplicationSummary
	UptimeSeconds           *int64
	Status                  Level
	StatusSource            RedisStatusSource
	CollectionLevel         Level
}

type RedisPage struct {
	Instances []RedisInstanceSummary
	Total     int
	Page      int
	PageSize  int
}
