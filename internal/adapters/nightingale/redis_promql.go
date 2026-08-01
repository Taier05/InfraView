package nightingale

const (
	redisUpQuery = iota
	redisUptimeQuery
	redisClusterEnabledQuery
	redisUsedMemoryQuery
	redisMaxMemoryQuery
	redisConnectedClientsQuery
	redisMaxClientsQuery
	redisBlockedClientsQuery
	redisQPSQuery
	redisHitRateQuery
	redisKeysQuery
	redisExpiredKeysQuery
	redisEvictedKeysQuery
	redisRejectedConnectionsQuery
	redisConnectedSlavesQuery
	redisMasterLinkStatusQuery
	redisMasterLastIOQuery
	redisMasterSyncQuery
	redisReplicationLagQuery
	redisInventoryQuery
	redisHistoricalRoleQuery
	redisQueryCount
)

var fixedRedisPromQL = [...]string{
	"redis_up",
	"redis_uptime_in_seconds",
	"redis_cluster_enabled",
	"redis_used_memory",
	"redis_maxmemory",
	"redis_connected_clients",
	"redis_maxclients",
	"redis_blocked_clients",
	"redis_instantaneous_ops_per_sec",
	"redis_keyspace_hitrate",
	"sum by (ident, instance, address, replica_role) (redis_keyspace_keys)",
	"rate(redis_expired_keys[5m])",
	"rate(redis_evicted_keys[5m])",
	"rate(redis_rejected_connections[5m])",
	"redis_connected_slaves",
	"redis_master_link_status",
	"redis_master_last_io_seconds_ago",
	"redis_master_sync_in_progress",
	"max by (ident, instance, address, replica_role) (redis_replication_lag)",
	"tlast_over_time(redis_up[24h])",
	"tlast_over_time(redis_uptime_in_seconds[24h])",
}

func redisPromQL() []string {
	queries := make([]string, len(fixedRedisPromQL))
	copy(queries, fixedRedisPromQL[:])
	return queries
}
