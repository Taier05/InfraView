package service

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/mysql"
)

const mysqlSnapshotCacheKey = "service:mysql:snapshot"

type MySQLService struct {
	provider  mysql.Provider
	store     *cache.Store
	options   MySQLOptions
	freshness *freshnessTracker
}

func NewMySQL(provider mysql.Provider, store *cache.Store, options MySQLOptions) *MySQLService {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.CurrentMetricsTTL <= 0 {
		options.CurrentMetricsTTL = 15 * time.Second
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
	return &MySQLService{
		provider:  provider,
		store:     store,
		options:   options,
		freshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (s *MySQLService) snapshot(ctx context.Context) (mysql.Snapshot, Meta, error) {
	result, err := s.store.GetOrLoad(
		ctx,
		mysqlSnapshotCacheKey,
		s.options.CurrentMetricsTTL,
		s.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, err := s.provider.MySQLSnapshot(loadCtx)
			if err != nil {
				return mysql.Snapshot{}, err
			}
			samples := make(map[string]time.Time, len(snapshot.Instances))
			for _, instance := range snapshot.Instances {
				if instance.CollectionTracked {
					samples[instance.ID] = instance.ReportedAt
				}
			}
			s.freshness.Observe(samples)
			return snapshot, nil
		},
	)
	if err != nil {
		return mysql.Snapshot{}, Meta{}, err
	}
	snapshot, ok := result.Value.(mysql.Snapshot)
	if !ok {
		return mysql.Snapshot{}, Meta{}, fmt.Errorf("service: mysql cache contained %T", result.Value)
	}
	return cloneMySQLSnapshot(snapshot), resultMeta(result), nil
}

func cloneMySQLSnapshot(source mysql.Snapshot) mysql.Snapshot {
	instances := make([]mysql.Instance, len(source.Instances))
	for i, instance := range source.Instances {
		instances[i] = instance
		instances[i].UptimeSeconds = cloneFloat(instance.UptimeSeconds)
		instances[i].Connections = cloneFloat(instance.Connections)
		instances[i].MaxConnections = cloneFloat(instance.MaxConnections)
		instances[i].ThreadsRunning = cloneFloat(instance.ThreadsRunning)
		instances[i].QPS = cloneFloat(instance.QPS)
		instances[i].TPS = cloneFloat(instance.TPS)
		instances[i].SlowQueriesPerSecond = cloneFloat(instance.SlowQueriesPerSecond)
		instances[i].BufferPoolUsagePercent = cloneFloat(instance.BufferPoolUsagePercent)
		instances[i].BufferPoolSizeBytes = cloneFloat(instance.BufferPoolSizeBytes)
		instances[i].ReplicationChannels = make([]mysql.ReplicationChannel, len(instance.ReplicationChannels))
		for j, channel := range instance.ReplicationChannels {
			instances[i].ReplicationChannels[j] = mysql.ReplicationChannel{
				IORunning:  cloneBool(channel.IORunning),
				SQLRunning: cloneBool(channel.SQLRunning),
				LagSeconds: cloneFloat(channel.LagSeconds),
			}
		}
	}
	return mysql.Snapshot{Instances: instances}
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (s *MySQLService) Overview(ctx context.Context) (MySQLOverview, Meta, error) {
	snapshot, meta, err := s.snapshot(ctx)
	if err != nil {
		return MySQLOverview{}, Meta{}, err
	}
	overview := MySQLOverview{Total: len(snapshot.Instances)}
	for _, instance := range snapshot.Instances {
		summary := s.summarizeMySQLInstance(instance)
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

		addMySQLAlert(
			&overview.Alerts.Availability,
			mysqlHigherLevel(mysqlAvailabilityLevel(instance.Availability), summary.CollectionLevel),
		)
		addMySQLAlert(&overview.Alerts.ReplicationThreads, mysqlReplicationThreadsLevel(instance.Role, instance.ReplicationChannels))
		if level, available := mysqlReplicationLagLevel(instance.ReplicationChannels); available {
			addMySQLAlert(&overview.Alerts.ReplicationLag, level)
		}
		addMySQLAlert(&overview.Alerts.ReplicationData, mysqlReplicationDataLevel(instance.Role, instance.ReplicationChannels))
	}
	return overview, meta, nil
}

func (s *MySQLService) Instances(ctx context.Context, query MySQLQuery) (MySQLPage, Meta, error) {
	query, err := normalizeMySQLQuery(query)
	if err != nil {
		return MySQLPage{}, Meta{}, err
	}
	snapshot, meta, err := s.snapshot(ctx)
	if err != nil {
		return MySQLPage{}, Meta{}, err
	}
	labels := make(map[string]struct{}, len(snapshot.Instances))
	for _, instance := range snapshot.Instances {
		if strings.TrimSpace(instance.Name) != "" {
			labels[instance.Name] = struct{}{}
		}
	}
	availableLabels := make([]string, 0, len(labels))
	for label := range labels {
		availableLabels = append(availableLabels, label)
	}
	sort.Strings(availableLabels)

	search := strings.ToLower(strings.TrimSpace(query.Search))
	items := make([]MySQLInstanceSummary, 0, len(snapshot.Instances))
	for _, instance := range snapshot.Instances {
		summary := s.summarizeMySQLInstance(instance)
		if query.Label != "" && summary.Name != query.Label {
			continue
		}
		if query.Status != "" && summary.Status != query.Status {
			continue
		}
		if query.Role != "" && summary.Role != query.Role {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(summary.Address), search) {
			continue
		}
		items = append(items, summary)
	}
	sortMySQLInstances(items, query.Sort, query.Order)

	total := len(items)
	start := total
	if query.Page-1 <= total/query.PageSize {
		start = (query.Page - 1) * query.PageSize
		if start > total {
			start = total
		}
	}
	end := min(start+query.PageSize, total)
	return MySQLPage{
		Instances:       append([]MySQLInstanceSummary(nil), items[start:end]...),
		AvailableLabels: availableLabels,
		Total:           total,
		Page:            query.Page,
		PageSize:        query.PageSize,
	}, meta, nil
}

func normalizeMySQLQuery(query MySQLQuery) (MySQLQuery, error) {
	query.Label = strings.TrimSpace(query.Label)
	if query.Page < 1 {
		return MySQLQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return MySQLQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return MySQLQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	switch query.Role {
	case "", mysql.RoleWritable, mysql.RoleReadOnly, mysql.RoleUnknown:
	default:
		return MySQLQuery{}, fmt.Errorf("%w: unsupported role %q", ErrInvalidQuery, query.Role)
	}
	if query.Sort == "" {
		query.Sort = "instance"
	}
	switch query.Sort {
	case "instance", "connections", "threads_running", "qps", "slow_queries", "buffer_pool", "replication_lag", "uptime", "status":
	default:
		return MySQLQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return MySQLQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

func sortMySQLInstances(items []MySQLInstanceSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftValue, leftAvailable, metricSort := mysqlMetricSortValue(items[i], field)
		rightValue, rightAvailable, _ := mysqlMetricSortValue(items[j], field)
		if metricSort && leftAvailable != rightAvailable {
			return leftAvailable
		}
		comparison := 0
		if metricSort && leftAvailable {
			comparison = compareFloat64(leftValue, rightValue)
		} else {
			comparison = compareMySQLInstances(items[i], items[j], field)
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

func mysqlMetricSortValue(item MySQLInstanceSummary, field string) (float64, bool, bool) {
	switch field {
	case "connections":
		return metricSortValue(item.ConnectionUsagePercent)
	case "threads_running":
		return metricSortValue(item.ThreadsRunning)
	case "qps":
		return metricSortValue(item.QPS)
	case "slow_queries":
		return metricSortValue(item.SlowQueriesPerSecond)
	case "buffer_pool":
		return metricSortValue(item.BufferPoolUsagePercent)
	case "replication_lag":
		return metricSortValue(item.Replication.LagSeconds)
	case "uptime":
		return metricSortValue(item.UptimeSeconds)
	default:
		return 0, false, false
	}
}

func compareMySQLInstances(left, right MySQLInstanceSummary, field string) int {
	switch field {
	case "instance":
		return compareMySQLAddresses(left.Address, right.Address)
	case "status":
		return strings.Compare(string(left.Status), string(right.Status))
	default:
		return 0
	}
}

func compareMySQLAddresses(left, right string) int {
	leftHost, leftPort, leftOK := parseMySQLAddress(left)
	rightHost, rightPort, rightOK := parseMySQLAddress(right)
	if leftOK && rightOK {
		if comparison := leftHost.Compare(rightHost); comparison != 0 {
			return comparison
		}
		return leftPort - rightPort
	}
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func parseMySQLAddress(value string) (netip.Addr, int, bool) {
	host, portValue, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	return address, port, true
}

func addMySQLAlert(count *MySQLAlertCount, level Level) {
	switch level {
	case LevelWarning, LevelUnknown:
		count.Warning++
	case LevelCritical:
		count.Critical++
	}
}

func mysqlReplicationThreadsLevel(role mysql.Role, channels []mysql.ReplicationChannel) Level {
	if len(channels) == 0 {
		if role == mysql.RoleWritable {
			return LevelNormal
		}
		return LevelUnknown
	}
	level := LevelNormal
	for _, channel := range channels {
		if channel.IORunning != nil && !*channel.IORunning ||
			channel.SQLRunning != nil && !*channel.SQLRunning {
			return LevelCritical
		}
		if channel.IORunning == nil || channel.SQLRunning == nil {
			level = LevelUnknown
		}
	}
	return level
}

func mysqlReplicationLagLevel(channels []mysql.ReplicationChannel) (Level, bool) {
	var maximum float64
	available := false
	for _, channel := range channels {
		if validMySQLLag(channel.LagSeconds) && (!available || *channel.LagSeconds > maximum) {
			maximum = *channel.LagSeconds
			available = true
		}
	}
	if !available {
		return LevelNormal, false
	}
	if maximum >= 30 {
		return LevelCritical, true
	}
	if maximum >= 5 {
		return LevelWarning, true
	}
	return LevelNormal, true
}

func mysqlReplicationDataLevel(role mysql.Role, channels []mysql.ReplicationChannel) Level {
	if len(channels) == 0 {
		if role == mysql.RoleWritable {
			return LevelNormal
		}
		return LevelUnknown
	}
	for _, channel := range channels {
		if channel.IORunning == nil || channel.SQLRunning == nil || !validMySQLLag(channel.LagSeconds) {
			return LevelUnknown
		}
	}
	return LevelNormal
}

func summarizeMySQLInstance(source mysql.Instance) MySQLInstanceSummary {
	replication := replicationSummary(source.Role, source.ReplicationChannels)
	var connectionUsagePercent *float64
	if source.Connections != nil && source.MaxConnections != nil && *source.MaxConnections > 0 {
		usage := *source.Connections / *source.MaxConnections * 100
		if !math.IsNaN(usage) && !math.IsInf(usage, 0) {
			connectionUsagePercent = &usage
		}
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
		TPS:                    cloneFloat(source.TPS),
		SlowQueriesPerSecond:   cloneFloat(source.SlowQueriesPerSecond),
		BufferPoolUsagePercent: cloneFloat(source.BufferPoolUsagePercent),
		BufferPoolSizeBytes:    cloneFloat(source.BufferPoolSizeBytes),
		UptimeSeconds:          cloneFloat(source.UptimeSeconds),
		Replication:            replication,
		Status:                 mysqlHigherLevel(mysqlAvailabilityLevel(source.Availability), replication.Level),
		CollectionLevel:        LevelNormal,
	}
}

func (s *MySQLService) summarizeMySQLInstance(source mysql.Instance) MySQLInstanceSummary {
	summary := summarizeMySQLInstance(source)
	if !source.CollectionTracked {
		return summary
	}
	summary.CollectionLevel = s.freshness.Level(source.ID, source.ReportedAt)
	summary.Status = mysqlHigherLevel(summary.Status, summary.CollectionLevel)
	return summary
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
	threadStopped := false
	for _, channel := range channels {
		if channel.IORunning != nil && !*channel.IORunning ||
			channel.SQLRunning != nil && !*channel.SQLRunning {
			threadStopped = true
		}
		validLag := validMySQLLag(channel.LagSeconds)
		if channel.IORunning == nil || channel.SQLRunning == nil || !validLag {
			incomplete = true
		}
		if validLag && (maximumLag == nil || *channel.LagSeconds > *maximumLag) {
			maximumLag = cloneFloat(channel.LagSeconds)
		}
	}
	if threadStopped {
		return MySQLReplicationSummary{
			State:      ReplicationThreadsStopped,
			LagSeconds: maximumLag,
			Level:      LevelCritical,
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

func validMySQLLag(value *float64) bool {
	return value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
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
