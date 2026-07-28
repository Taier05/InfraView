package service

import (
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/mysql"
)

const mysqlSnapshotCacheKey = "service:mysql:snapshot"

type MySQLService struct {
	provider mysql.Provider
	store    *cache.Store
	options  MySQLOptions
}

func NewMySQL(provider mysql.Provider, store *cache.Store, options MySQLOptions) *MySQLService {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.CurrentMetricsTTL <= 0 {
		options.CurrentMetricsTTL = 15 * time.Second
	}
	if options.MaxStale <= 0 {
		options.MaxStale = 5 * time.Minute
	}
	if store == nil {
		store = cache.New(options.Clock)
	}
	return &MySQLService{provider: provider, store: store, options: options}
}

func summarizeMySQLInstance(source mysql.Instance) MySQLInstanceSummary {
	replication := replicationSummary(source.Role, source.ReplicationChannels)
	var connectionUsagePercent *float64
	if source.Connections != nil && source.MaxConnections != nil && *source.MaxConnections > 0 {
		usage := *source.Connections / *source.MaxConnections * 100
		connectionUsagePercent = &usage
	}
	return MySQLInstanceSummary{
		ID:                     source.ID,
		Name:                   source.Name,
		Address:                source.Address,
		Host:                   source.Host,
		Version:                source.Version,
		Role:                   source.Role,
		Connections:            cloneFloat(source.Connections),
		MaxConnections:         cloneFloat(source.MaxConnections),
		ConnectionUsagePercent: connectionUsagePercent,
		ThreadsRunning:         cloneFloat(source.ThreadsRunning),
		QPS:                    cloneFloat(source.QPS),
		SlowQueriesPerSecond:   cloneFloat(source.SlowQueriesPerSecond),
		BufferPoolUsagePercent: cloneFloat(source.BufferPoolUsagePercent),
		UptimeSeconds:          cloneFloat(source.UptimeSeconds),
		Replication:            replication,
		Status:                 mysqlHigherLevel(mysqlAvailabilityLevel(source.Availability), replication.Level),
	}
}

func replicationSummary(role mysql.Role, channels []mysql.ReplicationChannel) MySQLReplicationSummary {
	if len(channels) == 0 {
		if role == mysql.RoleWritable {
			return MySQLReplicationSummary{State: ReplicationNotConfigured, Level: LevelNormal}
		}
		return MySQLReplicationSummary{State: ReplicationUnknown, Level: LevelUnknown}
	}

	var maximumLag *float64
	incomplete := false
	for _, channel := range channels {
		if channel.IORunning != nil && !*channel.IORunning ||
			channel.SQLRunning != nil && !*channel.SQLRunning {
			return MySQLReplicationSummary{State: ReplicationThreadsStopped, Level: LevelCritical}
		}
		if channel.IORunning == nil || channel.SQLRunning == nil || channel.LagSeconds == nil {
			incomplete = true
		}
		if channel.LagSeconds != nil && (maximumLag == nil || *channel.LagSeconds > *maximumLag) {
			maximumLag = cloneFloat(channel.LagSeconds)
		}
	}
	if maximumLag == nil {
		return MySQLReplicationSummary{State: ReplicationUnknown, Level: LevelUnknown}
	}

	level := LevelNormal
	if *maximumLag >= 30 {
		level = LevelCritical
	} else if *maximumLag >= 5 {
		level = LevelWarning
	}
	if incomplete {
		return MySQLReplicationSummary{
			State:      ReplicationUnknown,
			LagSeconds: maximumLag,
			Level:      mysqlHigherLevel(LevelUnknown, level),
		}
	}
	return MySQLReplicationSummary{
		State:      ReplicationNormal,
		LagSeconds: maximumLag,
		Level:      level,
	}
}

func mysqlHigherLevel(left, right Level) Level {
	ranks := map[Level]int{
		LevelNormal:   0,
		LevelUnknown:  1,
		LevelWarning:  2,
		LevelCritical: 3,
	}
	if ranks[right] > ranks[left] {
		return right
	}
	return left
}

func mysqlAvailabilityLevel(availability mysql.Availability) Level {
	switch availability {
	case mysql.AvailabilityUp:
		return LevelNormal
	case mysql.AvailabilityDown:
		return LevelCritical
	default:
		return LevelUnknown
	}
}
