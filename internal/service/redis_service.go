package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/redis"
)

const redisSnapshotCacheKey = "service:redis:snapshot"

type RedisService struct {
	provider  redis.Provider
	store     *cache.Store
	options   RedisOptions
	freshness *freshnessTracker
}

func NewRedis(provider redis.Provider, store *cache.Store, options RedisOptions) *RedisService {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.SnapshotTTL <= 0 {
		options.SnapshotTTL = 15 * time.Second
	}
	if options.CollectionInterval <= 0 {
		options.CollectionInterval = 15 * time.Second
	}
	if options.MaxStale <= 0 {
		options.MaxStale = 5 * time.Minute
	}
	if store == nil {
		store = cache.New(options.Clock)
	}
	return &RedisService{
		provider:  provider,
		store:     store,
		options:   options,
		freshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (service *RedisService) snapshot(ctx context.Context) (redis.Snapshot, Meta, error) {
	result, err := service.store.GetOrLoad(
		ctx,
		redisSnapshotCacheKey,
		service.options.SnapshotTTL,
		service.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, loadErr := service.provider.RedisSnapshot(loadCtx)
			if loadErr != nil {
				return redis.Snapshot{}, loadErr
			}
			samples := make(map[string]time.Time, len(snapshot.Instances))
			for _, instance := range snapshot.Instances {
				if instance.CollectionTracked {
					samples[instance.ID] = instance.ReportedAt
				}
			}
			service.freshness.Observe(samples)
			return snapshot.Clone(), nil
		},
	)
	if err != nil {
		return redis.Snapshot{}, Meta{}, err
	}
	snapshot, ok := result.Value.(redis.Snapshot)
	if !ok {
		return redis.Snapshot{}, Meta{}, fmt.Errorf("service: redis cache contained %T", result.Value)
	}
	var collectedAt time.Time
	for _, instance := range snapshot.Instances {
		collectedAt = latestTime(collectedAt, instance.ReportedAt)
	}
	return snapshot.Clone(), resultMetaAt(result, collectedAt), nil
}

func (service *RedisService) Overview(ctx context.Context) (RedisOverview, Meta, error) {
	snapshot, meta, err := service.snapshot(ctx)
	if err != nil {
		return RedisOverview{}, Meta{}, err
	}
	overview := RedisOverview{Total: len(snapshot.Instances)}
	for _, instance := range snapshot.Instances {
		collection := LevelNormal
		if instance.CollectionTracked {
			collection = service.freshness.Level(instance.ID, instance.ReportedAt)
		}
		summary := summarizeRedisInstance(instance, collection)
		switch summary.Status {
		case LevelNormal:
			overview.Normal++
		case LevelWarning:
			overview.Warning++
			overview.WarningInstances++
			overview.AffectedInstances++
		case LevelCritical:
			overview.Critical++
			overview.CriticalInstances++
			overview.AffectedInstances++
		case LevelUnknown:
			overview.Unknown++
			overview.WarningInstances++
			overview.AffectedInstances++
		}
		switch instance.Role {
		case redis.RoleMaster:
			overview.Roles.Master++
		case redis.RoleSlave:
			overview.Roles.Slave++
		default:
			overview.Roles.Unknown++
		}
		availability := chooseRedisAssessment(
			redisAvailabilityAssessment(instance.Availability),
			redisAssessment{level: collection, source: RedisStatusCollection},
		)
		addRedisAlert(&overview.Alerts.Availability, availability.level)
		addRedisAlert(&overview.Alerts.Memory, redisMemoryAssessment(instance).level)
		addRedisAlert(&overview.Alerts.Connection, redisConnectionAssessment(instance).level)
		addRedisAlert(&overview.Alerts.Replication, redisReplicationAssessment(instance).level)
	}
	return overview, meta, nil
}

func (service *RedisService) Instances(ctx context.Context, query RedisQuery) (RedisPage, Meta, error) {
	query, err := normalizeRedisQuery(query)
	if err != nil {
		return RedisPage{}, Meta{}, err
	}
	snapshot, meta, err := service.snapshot(ctx)
	if err != nil {
		return RedisPage{}, Meta{}, err
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	items := make([]RedisInstanceSummary, 0, len(snapshot.Instances))
	for _, instance := range snapshot.Instances {
		collection := LevelNormal
		if instance.CollectionTracked {
			collection = service.freshness.Level(instance.ID, instance.ReportedAt)
		}
		summary := summarizeRedisInstance(instance, collection)
		roleMismatch := query.Role != "" && summary.Role != query.Role
		statusMismatch := query.Status != "" && summary.Status != query.Status
		searchMismatch := search != "" &&
			!strings.Contains(strings.ToLower(summary.Address), search)
		if roleMismatch || statusMismatch || searchMismatch {
			continue
		}
		items = append(items, summary)
	}
	sortRedisInstances(items, query.Sort, query.Order)
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	return RedisPage{
		Instances: append([]RedisInstanceSummary(nil), items[start:end]...),
		Total:     total,
		Page:      query.Page,
		PageSize:  query.PageSize,
	}, meta, nil
}

func normalizeRedisQuery(query RedisQuery) (RedisQuery, error) {
	if query.Page < 1 {
		return RedisQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return RedisQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.PageSize {
		return RedisQuery{}, fmt.Errorf("%w: page offset overflows int", ErrInvalidQuery)
	}
	switch query.Role {
	case "", redis.RoleMaster, redis.RoleSlave, redis.RoleUnknown:
	default:
		return RedisQuery{}, fmt.Errorf("%w: unsupported role %q", ErrInvalidQuery, query.Role)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return RedisQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = "instance"
	}
	switch query.Sort {
	case "instance", "role", "memory_limit", "memory", "connections", "blocked_connections", "qps", "hit_rate", "keys", "evicted", "replication_link", "replication_lag", "uptime", "status":
	default:
		return RedisQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return RedisQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

type redisAssessment struct {
	level  Level
	source RedisStatusSource
}

func summarizeRedisInstance(source redis.Instance, collection Level) RedisInstanceSummary {
	memoryUsage := redisUsagePercent(source.UsedMemoryBytes, source.MaxMemoryBytes)
	connectionUsage := redisUsagePercent(source.ConnectedClients, source.MaxClients)
	assessment := redisAssessment{level: LevelNormal, source: RedisStatusNormal}
	for _, candidate := range []redisAssessment{
		redisAvailabilityAssessment(source.Availability),
		redisReplicationAssessment(source),
		redisMemoryAssessment(source),
		redisConnectionAssessment(source),
		{level: collection, source: RedisStatusCollection},
	} {
		assessment = chooseRedisAssessment(assessment, candidate)
	}
	return RedisInstanceSummary{
		ID:                     source.ID,
		Address:                source.Address,
		Availability:           source.Availability,
		Role:                   source.Role,
		ClusterEnabled:         cloneBool(source.ClusterEnabled),
		UsedMemoryBytes:        cloneInt64(source.UsedMemoryBytes),
		MaxMemoryBytes:         cloneInt64(source.MaxMemoryBytes),
		MemoryUsagePercent:     memoryUsage,
		ConnectedClients:       cloneInt64(source.ConnectedClients),
		MaxClients:             cloneInt64(source.MaxClients),
		ConnectionUsagePercent: connectionUsage,
		BlockedClients:         cloneInt64(source.BlockedClients),
		QPS:                    cloneFloat(source.QPS),
		HitRate:                cloneFloat(source.HitRate),
		Keys:                   cloneInt64(source.Keys),
		ExpiredKeysPerSecond:   cloneFloat(source.ExpiredKeysPerSecond),
		EvictedKeysPerSecond:   cloneFloat(source.EvictedKeysPerSecond),
		RejectedConnectionsRate: cloneFloat(
			source.RejectedConnectionsRate,
		),
		Replication: RedisReplicationSummary{
			ConnectedReplicas:      cloneInt64(source.Replication.ConnectedReplicas),
			MasterLinkUp:           cloneBool(source.Replication.MasterLinkUp),
			MasterLastIOSecondsAgo: cloneFloat(source.Replication.MasterLastIOSecondsAgo),
			MasterSyncInProgress:   cloneBool(source.Replication.MasterSyncInProgress),
			WorstReplicaLagSeconds: cloneFloat(source.Replication.WorstReplicaLagSeconds),
		},
		UptimeSeconds:   cloneInt64(source.UptimeSeconds),
		Status:          assessment.level,
		StatusSource:    assessment.source,
		CollectionLevel: collection,
	}
}

func redisAvailabilityAssessment(value redis.Availability) redisAssessment {
	switch value {
	case redis.AvailabilityUp:
		return redisAssessment{level: LevelNormal, source: RedisStatusNormal}
	case redis.AvailabilityDown:
		return redisAssessment{level: LevelCritical, source: RedisStatusAvailability}
	default:
		return redisAssessment{level: LevelUnknown, source: RedisStatusAvailability}
	}
}

func redisReplicationAssessment(instance redis.Instance) redisAssessment {
	assessment := redisAssessment{level: LevelNormal, source: RedisStatusNormal}
	switch instance.Role {
	case redis.RoleMaster:
		if instance.ClusterEnabled != nil &&
			*instance.ClusterEnabled &&
			instance.Replication.ConnectedReplicas != nil &&
			*instance.Replication.ConnectedReplicas == 0 {
			assessment = redisAssessment{level: LevelCritical, source: RedisStatusReplication}
		}
		if lag := instance.Replication.WorstReplicaLagSeconds; lag != nil &&
			*lag >= 0 &&
			!math.IsNaN(*lag) &&
			!math.IsInf(*lag, 0) {
			level := LevelNormal
			if *lag >= 30 {
				level = LevelCritical
			} else if *lag >= 5 {
				level = LevelWarning
			}
			assessment = chooseRedisAssessment(assessment, redisAssessment{level: level, source: RedisStatusReplication})
		}
	case redis.RoleSlave:
		if instance.Replication.MasterLinkUp == nil {
			assessment = redisAssessment{level: LevelUnknown, source: RedisStatusReplication}
		} else if !*instance.Replication.MasterLinkUp {
			assessment = redisAssessment{level: LevelCritical, source: RedisStatusReplication}
		}
	default:
		assessment = redisAssessment{level: LevelUnknown, source: RedisStatusUnknown}
	}
	return assessment
}

func redisMemoryAssessment(instance redis.Instance) redisAssessment {
	if instance.MaxMemoryBytes == nil || *instance.MaxMemoryBytes <= 0 {
		return redisAssessment{level: LevelNormal, source: RedisStatusNormal}
	}
	usage := redisUsagePercent(instance.UsedMemoryBytes, instance.MaxMemoryBytes)
	if usage == nil {
		return redisAssessment{level: LevelUnknown, source: RedisStatusMemory}
	}
	if *usage >= 95 {
		return redisAssessment{level: LevelCritical, source: RedisStatusMemory}
	}
	if *usage >= 85 {
		return redisAssessment{level: LevelWarning, source: RedisStatusMemory}
	}
	return redisAssessment{level: LevelNormal, source: RedisStatusNormal}
}

func redisConnectionAssessment(instance redis.Instance) redisAssessment {
	if value := instance.RejectedConnectionsRate; value != nil &&
		*value > 0 &&
		!math.IsNaN(*value) &&
		!math.IsInf(*value, 0) {
		return redisAssessment{level: LevelWarning, source: RedisStatusConnection}
	}
	return redisAssessment{level: LevelNormal, source: RedisStatusNormal}
}

func redisUsagePercent(value, maximum *int64) *float64 {
	if value == nil || maximum == nil || *value < 0 || *maximum <= 0 {
		return nil
	}
	percent := float64(*value) / float64(*maximum) * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return nil
	}
	return &percent
}

func chooseRedisAssessment(left, right redisAssessment) redisAssessment {
	if right.level == LevelNormal {
		return left
	}
	rightLevel := redisLevelRank(right.level)
	leftLevel := redisLevelRank(left.level)
	if rightLevel > leftLevel ||
		rightLevel == leftLevel && redisSourceRank(right.source) > redisSourceRank(left.source) {
		return right
	}
	return left
}

func redisLevelRank(level Level) int {
	switch level {
	case LevelCritical:
		return 3
	case LevelWarning:
		return 2
	case LevelUnknown:
		return 1
	default:
		return 0
	}
}

func redisSourceRank(source RedisStatusSource) int {
	switch source {
	case RedisStatusAvailability:
		return 5
	case RedisStatusReplication:
		return 4
	case RedisStatusMemory:
		return 3
	case RedisStatusConnection:
		return 2
	case RedisStatusCollection:
		return 1
	default:
		return 0
	}
}

func addRedisAlert(target *RedisAlertCount, level Level) {
	if level == LevelCritical {
		target.Critical++
	} else if level == LevelWarning || level == LevelUnknown {
		target.Warning++
	}
}

func sortRedisInstances(items []RedisInstanceSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftInteger, leftOK, integer := redisIntegerSortValue(items[i], field)
		rightInteger, rightOK, _ := redisIntegerSortValue(items[j], field)
		if integer && leftOK != rightOK {
			return leftOK
		}
		comparison := 0
		if integer {
			if leftOK {
				comparison = compareInt64(leftInteger, rightInteger)
			}
		} else {
			left, leftOK, numeric := redisSortValue(items[i], field)
			right, rightOK, _ := redisSortValue(items[j], field)
			if numeric && leftOK != rightOK {
				return leftOK
			}
			if numeric && leftOK {
				comparison = compareFloat64(left, right)
			} else if field == "instance" {
				comparison = compareAddresses(items[i].Address, items[j].Address)
			} else if field == "status" {
				comparison = listLevelSortRank(items[i].Status) - listLevelSortRank(items[j].Status)
			}
		}
		if comparison == 0 {
			return items[i].ID < items[j].ID
		}
		if order == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func redisSortValue(item RedisInstanceSummary, field string) (float64, bool, bool) {
	switch field {
	case "role":
		return float64(redisRoleSortRank(item.Role)), true, true
	case "memory":
		return metricSortValue(item.MemoryUsagePercent)
	case "connections":
		return metricSortValue(item.ConnectionUsagePercent)
	case "blocked_connections":
		return redisIntSortValue(item.BlockedClients)
	case "qps":
		return metricSortValue(item.QPS)
	case "hit_rate":
		return metricSortValue(item.HitRate)
	case "keys":
		return redisIntSortValue(item.Keys)
	case "evicted":
		return metricSortValue(item.EvictedKeysPerSecond)
	case "replication_link":
		return redisReplicationLinkSortValue(item)
	case "replication_lag":
		return metricSortValue(item.Replication.WorstReplicaLagSeconds)
	case "uptime":
		return redisIntSortValue(item.UptimeSeconds)
	default:
		return 0, false, false
	}
}

func redisIntegerSortValue(item RedisInstanceSummary, field string) (int64, bool, bool) {
	var value *int64
	switch field {
	case "memory_limit":
		value = item.MaxMemoryBytes
	case "blocked_connections":
		value = item.BlockedClients
	case "keys":
		value = item.Keys
	case "uptime":
		value = item.UptimeSeconds
	default:
		return 0, false, false
	}
	if value == nil {
		return 0, false, true
	}
	return *value, true, true
}

func redisRoleSortRank(role redis.Role) int {
	switch role {
	case redis.RoleMaster:
		return 0
	case redis.RoleSlave:
		return 1
	default:
		return 2
	}
}

func redisReplicationLinkSortValue(item RedisInstanceSummary) (float64, bool, bool) {
	if item.Role != redis.RoleSlave {
		return 0, false, true
	}
	if item.Replication.MasterLinkUp == nil {
		return 2, true, true
	}
	if *item.Replication.MasterLinkUp {
		return 0, true, true
	}
	return 1, true, true
}

func redisIntSortValue(value *int64) (float64, bool, bool) {
	if value == nil {
		return 0, false, true
	}
	return float64(*value), true, true
}
