package nightingale

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/redis"
)

type redisFloatState struct {
	value     *float64
	timestamp time.Time
	conflict  bool
}

type redisInt64State struct {
	value     *int64
	timestamp time.Time
	conflict  bool
}

type redisBoolState struct {
	value     *bool
	timestamp time.Time
	conflict  bool
}

type redisRoleState struct {
	role      redis.Role
	timestamp time.Time
	present   bool
	conflict  bool
}

type redisInstanceState struct {
	instance redis.Instance
	up       redisBoolState

	currentRole    redisRoleState
	historicalRole redisRoleState
	uptime         redisInt64State
	cluster        redisBoolState
	usedMemory     redisInt64State
	maxMemory      redisInt64State
	clients        redisInt64State
	maxClients     redisInt64State
	blockedClients redisInt64State
	qps            redisFloatState
	hitRate        redisFloatState
	keys           redisInt64State
	expired        redisFloatState
	evicted        redisFloatState
	rejected       redisFloatState
	connected      redisInt64State
	masterLink     redisBoolState
	masterLastIO   redisFloatState
	masterSync     redisBoolState
	replicationLag redisFloatState
}

var _ redis.Provider = (*Provider)(nil)

func (provider *Provider) RedisSnapshot(ctx context.Context) (redis.Snapshot, error) {
	if err := provider.ready(); err != nil {
		return redis.Snapshot{}, redisUnavailableError()
	}
	results, err := provider.queryInstant(ctx, redisPromQL())
	if err != nil || len(results) != redisQueryCount {
		return redis.Snapshot{}, redisUnavailableError()
	}
	return buildRedisSnapshot(results)
}

func buildRedisSnapshot(results [][]instantSeries) (redis.Snapshot, error) {
	if len(results) != redisQueryCount || results[redisInventoryQuery] == nil {
		return redis.Snapshot{}, redisUnavailableError()
	}
	states := make(map[string]*redisInstanceState, len(results[redisInventoryQuery]))
	for _, series := range results[redisInventoryQuery] {
		_, _, address, key, ok := redisIdentity(series.Metric)
		if !ok || len(series.Value) != 2 {
			return redis.Snapshot{}, redisUnavailableError()
		}
		reportedAt, ok := parseUnixTime(series.Value[1])
		if !ok {
			return redis.Snapshot{}, redisUnavailableError()
		}
		if _, exists := states[key]; exists {
			return redis.Snapshot{}, redisUnavailableError()
		}
		ident, instance, _, _, _ := redisIdentity(series.Metric)
		stableID := redis.StableInstanceID(ident, instance, address)
		if stableID == "" {
			return redis.Snapshot{}, redisUnavailableError()
		}
		states[key] = &redisInstanceState{instance: redis.Instance{
			ID:                stableID,
			Address:           address,
			Availability:      redis.AvailabilityUnknown,
			Role:              redis.RoleUnknown,
			CollectionTracked: true,
			ReportedAt:        reportedAt,
		}}
	}

	mergeRedisHistoricalRoles(states, results[redisHistoricalRoleQuery])
	for queryIndex := redisUpQuery; queryIndex < redisInventoryQuery; queryIndex++ {
		mergeRedisQuery(states, queryIndex, results[queryIndex])
	}

	instances := make([]redis.Instance, 0, len(states))
	for _, state := range states {
		finalizeRedisInstance(state)
		instances = append(instances, state.instance)
	}
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Address != instances[j].Address {
			return instances[i].Address < instances[j].Address
		}
		return instances[i].ID < instances[j].ID
	})
	return redis.Snapshot{Instances: instances}, nil
}

func redisIdentity(labels map[string]string) (ident, instance, address, key string, ok bool) {
	ident = strings.TrimSpace(labels["ident"])
	instance = strings.TrimSpace(labels["instance"])
	address = strings.TrimSpace(labels["address"])
	if ident == "" || instance == "" || address == "" {
		return "", "", "", "", false
	}
	return ident, instance, address, ident + "\x00" + instance + "\x00" + address, true
}

func mergeRedisHistoricalRoles(states map[string]*redisInstanceState, series []instantSeries) {
	for _, candidate := range series {
		state, ok := redisStateForSeries(states, candidate)
		if !ok {
			continue
		}
		_, timestamp, valid := parseInstantValue(candidate.Value)
		if !valid {
			continue
		}
		mergeRedisRole(&state.historicalRole, candidate.Metric["replica_role"], timestamp)
	}
}

func mergeRedisQuery(states map[string]*redisInstanceState, queryIndex int, series []instantSeries) {
	for _, candidate := range series {
		state, ok := redisStateForSeries(states, candidate)
		if !ok {
			continue
		}
		if queryIndex != redisUpQuery {
			if _, timestamp, valid := parseInstantValue(candidate.Value); valid {
				mergeRedisRole(&state.currentRole, candidate.Metric["replica_role"], timestamp)
			}
		}
		switch queryIndex {
		case redisUpQuery:
			mergeRedisBool(&state.up, candidate)
		case redisUptimeQuery:
			mergeRedisInt64(&state.uptime, candidate)
		case redisClusterEnabledQuery:
			mergeRedisBool(&state.cluster, candidate)
		case redisUsedMemoryQuery:
			mergeRedisInt64(&state.usedMemory, candidate)
		case redisMaxMemoryQuery:
			mergeRedisInt64(&state.maxMemory, candidate)
		case redisConnectedClientsQuery:
			mergeRedisInt64(&state.clients, candidate)
		case redisMaxClientsQuery:
			mergeRedisInt64(&state.maxClients, candidate)
		case redisBlockedClientsQuery:
			mergeRedisInt64(&state.blockedClients, candidate)
		case redisQPSQuery:
			mergeRedisFloat(&state.qps, candidate, redisNonNegative)
		case redisHitRateQuery:
			mergeRedisFloat(&state.hitRate, candidate, redisRatio)
		case redisKeysQuery:
			mergeRedisInt64(&state.keys, candidate)
		case redisExpiredKeysQuery:
			mergeRedisFloat(&state.expired, candidate, redisNonNegative)
		case redisEvictedKeysQuery:
			mergeRedisFloat(&state.evicted, candidate, redisNonNegative)
		case redisRejectedConnectionsQuery:
			mergeRedisFloat(&state.rejected, candidate, redisNonNegative)
		case redisConnectedSlavesQuery:
			mergeRedisInt64(&state.connected, candidate)
		case redisMasterLinkStatusQuery:
			mergeRedisBool(&state.masterLink, candidate)
		case redisMasterLastIOQuery:
			mergeRedisFloat(&state.masterLastIO, candidate, redisNonNegative)
		case redisMasterSyncQuery:
			mergeRedisBool(&state.masterSync, candidate)
		case redisReplicationLagQuery:
			mergeRedisFloat(&state.replicationLag, candidate, redisNonNegative)
		}
	}
}

func redisStateForSeries(states map[string]*redisInstanceState, series instantSeries) (*redisInstanceState, bool) {
	_, _, _, key, ok := redisIdentity(series.Metric)
	if !ok {
		return nil, false
	}
	state, exists := states[key]
	return state, exists
}

func mergeRedisRole(target *redisRoleState, raw string, timestamp time.Time) {
	role, valid := canonicalRedisRole(raw)
	if timestamp.Before(target.timestamp) {
		return
	}
	if timestamp.After(target.timestamp) {
		target.timestamp = timestamp
		target.present = true
		target.conflict = !valid
		target.role = role
		return
	}
	if !target.present {
		target.present = true
		target.conflict = !valid
		target.role = role
		return
	}
	if !valid || target.role != role {
		target.conflict = true
		target.role = redis.RoleUnknown
	}
}

func canonicalRedisRole(raw string) (redis.Role, bool) {
	switch strings.TrimSpace(raw) {
	case string(redis.RoleMaster):
		return redis.RoleMaster, true
	case string(redis.RoleSlave):
		return redis.RoleSlave, true
	default:
		return redis.RoleUnknown, false
	}
}

func mergeRedisFloat(target *redisFloatState, candidate instantSeries, validator func(float64) bool) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || !validator(value) || timestamp.Before(target.timestamp) {
		return
	}
	if timestamp.After(target.timestamp) || !target.present() {
		target.value = &value
		target.timestamp = timestamp
		target.conflict = false
		return
	}
	if !target.conflict && (target.value == nil || *target.value != value) {
		target.value = nil
		target.conflict = true
	}
}

func (state redisFloatState) present() bool {
	return state.value != nil || state.conflict || !state.timestamp.IsZero()
}

func mergeRedisInt64(target *redisInt64State, candidate instantSeries) {
	value, timestamp, ok := parseRedisInt64(candidate.Value)
	if !ok || timestamp.Before(target.timestamp) {
		return
	}
	if timestamp.After(target.timestamp) || target.value == nil && !target.conflict && target.timestamp.IsZero() {
		target.value = &value
		target.timestamp = timestamp
		target.conflict = false
		return
	}
	if !target.conflict && (target.value == nil || *target.value != value) {
		target.value = nil
		target.conflict = true
	}
}

func parseRedisInt64(raw []json.RawMessage) (int64, time.Time, bool) {
	if len(raw) != 2 {
		return 0, time.Time{}, false
	}
	timestamp, ok := parseUnixTime(raw[0])
	if !ok {
		return 0, time.Time{}, false
	}
	var text string
	if err := json.Unmarshal(raw[1], &text); err != nil {
		text = string(raw[1])
	}
	if text == "" || len(text) > 20 {
		return 0, time.Time{}, false
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return 0, time.Time{}, false
	}
	return value, timestamp, true
}

func mergeRedisBool(target *redisBoolState, candidate instantSeries) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || timestamp.Before(target.timestamp) || value != 0 && value != 1 {
		return
	}
	binary := value == 1
	if timestamp.After(target.timestamp) || target.value == nil && !target.conflict && target.timestamp.IsZero() {
		target.value = &binary
		target.timestamp = timestamp
		target.conflict = false
		return
	}
	if !target.conflict && (target.value == nil || *target.value != binary) {
		target.value = nil
		target.conflict = true
	}
}

func redisNonNegative(value float64) bool { return value >= 0 }
func redisRatio(value float64) bool       { return value >= 0 && value <= 1 }

func finalizeRedisInstance(state *redisInstanceState) {
	if state.up.value != nil {
		if *state.up.value {
			state.instance.Availability = redis.AvailabilityUp
		} else {
			state.instance.Availability = redis.AvailabilityDown
		}
	}
	if state.currentRole.present {
		if !state.currentRole.conflict {
			state.instance.Role = state.currentRole.role
		}
	} else if state.historicalRole.present && !state.historicalRole.conflict {
		state.instance.Role = state.historicalRole.role
	}
	state.instance.UptimeSeconds = state.uptime.value
	state.instance.ClusterEnabled = state.cluster.value
	state.instance.UsedMemoryBytes = state.usedMemory.value
	state.instance.MaxMemoryBytes = state.maxMemory.value
	state.instance.ConnectedClients = state.clients.value
	state.instance.MaxClients = state.maxClients.value
	state.instance.BlockedClients = state.blockedClients.value
	state.instance.QPS = state.qps.value
	state.instance.HitRate = state.hitRate.value
	state.instance.Keys = state.keys.value
	state.instance.ExpiredKeysPerSecond = state.expired.value
	state.instance.EvictedKeysPerSecond = state.evicted.value
	state.instance.RejectedConnectionsRate = state.rejected.value
	state.instance.Replication.ConnectedReplicas = state.connected.value
	state.instance.Replication.MasterLinkUp = state.masterLink.value
	state.instance.Replication.MasterLastIOSecondsAgo = state.masterLastIO.value
	state.instance.Replication.MasterSyncInProgress = state.masterSync.value
	state.instance.Replication.WorstReplicaLagSeconds = state.replicationLag.value
}

func redisUnavailableError() error {
	return fmt.Errorf("%w: Nightingale Redis 当前指标不可用", redis.ErrUnavailable)
}
