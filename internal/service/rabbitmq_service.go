package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/rabbitmq"
)

const rabbitMQSnapshotCacheKey = "service:rabbitmq:snapshot"

type RabbitMQService struct {
	provider  rabbitmq.Provider
	store     *cache.Store
	options   RabbitMQOptions
	freshness *freshnessTracker
}

type rabbitMQSnapshotState struct {
	snapshot       rabbitmq.Snapshot
	nodeAdvancedAt map[string]time.Time
}

func NewRabbitMQ(provider rabbitmq.Provider, store *cache.Store, options RabbitMQOptions) *RabbitMQService {
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
	return &RabbitMQService{
		provider:  provider,
		store:     store,
		options:   options,
		freshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (service *RabbitMQService) snapshotState(ctx context.Context) (rabbitMQSnapshotState, Meta, error) {
	result, err := service.store.GetOrLoad(
		ctx,
		rabbitMQSnapshotCacheKey,
		service.options.SnapshotTTL,
		service.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, loadErr := service.provider.RabbitMQSnapshot(loadCtx)
			if loadErr != nil {
				return rabbitMQSnapshotState{}, loadErr
			}
			samples := make(map[string]time.Time, len(snapshot.Nodes))
			for _, node := range snapshot.Nodes {
				if node.CollectionTracked {
					samples[node.ID] = node.ReportedAt
				}
			}
			service.freshness.Observe(samples)
			return rabbitMQSnapshotState{
				snapshot:       snapshot.Clone(),
				nodeAdvancedAt: captureRabbitMQProgress(service.freshness, samples),
			}, nil
		},
	)
	if err != nil {
		return rabbitMQSnapshotState{}, Meta{}, err
	}
	state, ok := result.Value.(rabbitMQSnapshotState)
	if !ok {
		return rabbitMQSnapshotState{}, Meta{}, fmt.Errorf("service: rabbitmq cache contained %T", result.Value)
	}
	var collectedAt time.Time
	for _, node := range state.snapshot.Nodes {
		collectedAt = latestTime(collectedAt, node.ReportedAt)
	}
	return cloneRabbitMQSnapshotState(state), resultMetaAt(result, collectedAt), nil
}

func captureRabbitMQProgress(tracker *freshnessTracker, samples map[string]time.Time) map[string]time.Time {
	progress := make(map[string]time.Time, len(samples))
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for key, sampleAt := range samples {
		entry, exists := tracker.entries[key]
		if exists && entry.sampleAt.Equal(sampleAt.UTC()) {
			progress[key] = entry.advancedAt
		}
	}
	return progress
}

func cloneRabbitMQSnapshotState(source rabbitMQSnapshotState) rabbitMQSnapshotState {
	progress := make(map[string]time.Time, len(source.nodeAdvancedAt))
	for key, value := range source.nodeAdvancedAt {
		progress[key] = value
	}
	return rabbitMQSnapshotState{snapshot: source.snapshot.Clone(), nodeAdvancedAt: progress}
}

func (service *RabbitMQService) nodeCollectionLevel(node rabbitmq.Node, advancedAt map[string]time.Time) Level {
	if !node.CollectionTracked {
		return LevelUnknown
	}
	advanced, exists := advancedAt[node.ID]
	if !exists || node.ReportedAt.IsZero() {
		return LevelUnknown
	}
	return collectionLevelAt(service.options.Clock().UTC(), advanced, service.options.CollectionInterval)
}

func (service *RabbitMQService) Overview(ctx context.Context) (RabbitMQOverview, Meta, error) {
	state, meta, err := service.snapshotState(ctx)
	if err != nil {
		return RabbitMQOverview{}, Meta{}, err
	}
	overview := RabbitMQOverview{
		Status:   LevelNormal,
		Clusters: RabbitMQLevelCounts{Total: len(state.snapshot.Clusters)},
		Nodes:    RabbitMQLevelCounts{Total: len(state.snapshot.Nodes)},
	}
	for _, cluster := range state.snapshot.Clusters {
		level := rabbitMQClusterConnectivityLevel(cluster)
		addRabbitMQLevel(&overview.Clusters, level)
		addRabbitMQAlert(&overview.Alerts.ClusterConnectivity, level)
		overview.Status = higherRabbitMQLevel(overview.Status, level)
	}
	for _, node := range state.snapshot.Nodes {
		collection := service.nodeCollectionLevel(node, state.nodeAdvancedAt)
		summary := summarizeRabbitMQNode(node, collection)
		addRabbitMQLevel(&overview.Nodes, summary.Status)
		addRabbitMQAlert(&overview.Alerts.ResourceAlarms, rabbitMQAlarmAssessment(node).level)
		addRabbitMQAlert(&overview.Alerts.ResourcePressure, rabbitMQResourceAssessment(node).level)
		addRabbitMQAlert(&overview.Alerts.Collection, collection)
		overview.Status = higherRabbitMQLevel(overview.Status, summary.Status)
	}
	return overview, meta, nil
}

func (service *RabbitMQService) Nodes(ctx context.Context, query RabbitMQQuery) (RabbitMQPage, Meta, error) {
	query, err := normalizeRabbitMQQuery(query)
	if err != nil {
		return RabbitMQPage{}, Meta{}, err
	}
	state, meta, err := service.snapshotState(ctx)
	if err != nil {
		return RabbitMQPage{}, Meta{}, err
	}
	clusterOptions := make(map[string]struct{}, len(state.snapshot.Clusters))
	for _, cluster := range state.snapshot.Clusters {
		if name := strings.TrimSpace(cluster.Name); name != "" {
			clusterOptions[name] = struct{}{}
		}
	}
	for _, node := range state.snapshot.Nodes {
		if name := strings.TrimSpace(node.Cluster); name != "" {
			clusterOptions[name] = struct{}{}
		}
	}
	availableClusters := make([]string, 0, len(clusterOptions))
	for cluster := range clusterOptions {
		availableClusters = append(availableClusters, cluster)
	}
	sort.Strings(availableClusters)

	search := strings.ToLower(query.Search)
	items := make([]RabbitMQNodeSummary, 0, len(state.snapshot.Nodes))
	for _, node := range state.snapshot.Nodes {
		summary := summarizeRabbitMQNode(node, service.nodeCollectionLevel(node, state.nodeAdvancedAt))
		if query.Cluster != "" && summary.Cluster != query.Cluster || query.Status != "" && summary.Status != query.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(summary.Name), search) && !strings.Contains(strings.ToLower(summary.Address), search) {
			continue
		}
		items = append(items, summary)
	}
	sortRabbitMQNodes(items, query.Sort, query.Order)
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	return RabbitMQPage{
		Nodes:             append([]RabbitMQNodeSummary(nil), items[start:end]...),
		AvailableClusters: availableClusters,
		Total:             total,
		Page:              query.Page,
		PageSize:          query.PageSize,
	}, meta, nil
}

func normalizeRabbitMQQuery(query RabbitMQQuery) (RabbitMQQuery, error) {
	query.Search = strings.TrimSpace(query.Search)
	query.Cluster = strings.TrimSpace(query.Cluster)
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 {
		return RabbitMQQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return RabbitMQQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page > maxInt/query.PageSize {
		return RabbitMQQuery{}, fmt.Errorf("%w: page end overflows int", ErrInvalidQuery)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return RabbitMQQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = "node"
	}
	switch query.Sort {
	case "node", "cluster", "address", "version", "memory", "disk", "file_descriptor", "erlang_process",
		"connections", "queues", "messages", "publish_rate", "deliver_rate", "uptime", "status":
	default:
		return RabbitMQQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return RabbitMQQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

type rabbitMQAssessment struct {
	level  Level
	source RabbitMQNodeStatusSource
}

func summarizeRabbitMQNode(source rabbitmq.Node, collection Level) RabbitMQNodeSummary {
	assessment := rabbitMQAssessment{level: LevelNormal, source: RabbitMQStatusNormal}
	for _, candidate := range []rabbitMQAssessment{
		rabbitMQAlarmAssessment(source),
		{level: collection, source: RabbitMQStatusCollection},
		rabbitMQDiskAssessment(source),
		rabbitMQMemoryAssessment(source),
		rabbitMQFileDescriptorAssessment(source),
		rabbitMQErlangProcessAssessment(source),
	} {
		assessment = chooseRabbitMQAssessment(assessment, candidate)
	}
	return RabbitMQNodeSummary{
		ID:                         source.ID,
		Name:                       source.Name,
		Cluster:                    source.Cluster,
		Address:                    source.Address,
		Version:                    source.Version,
		MemoryUsagePercent:         rabbitMQUsagePercent(source.MemoryUsedBytes, source.MemoryLimitBytes),
		DiskAvailableBytes:         cloneInt64(source.DiskAvailableBytes),
		FileDescriptorUsagePercent: rabbitMQUsagePercent(source.OpenFileDescriptors, source.MaxFileDescriptors),
		ErlangProcessUsagePercent:  rabbitMQUsagePercent(source.ErlangProcessesUsed, source.ErlangProcessesLimit),
		Connections:                cloneInt64(source.Connections),
		Queues:                     cloneInt64(source.Queues),
		Messages:                   cloneInt64(source.Messages),
		PublishRate:                cloneFloat(source.PublishRate),
		DeliverRate:                cloneFloat(source.DeliverRate),
		UptimeSeconds:              cloneInt64(source.UptimeSeconds),
		Status:                     assessment.level,
		StatusSource:               assessment.source,
		CollectionLevel:            collection,
	}
}

func rabbitMQAlarmAssessment(node rabbitmq.Node) rabbitMQAssessment {
	alarms := []*bool{node.MemoryAlarm, node.DiskAlarm, node.FileDescriptorAlarm}
	missing := false
	for _, alarm := range alarms {
		if alarm == nil {
			missing = true
		} else if *alarm {
			return rabbitMQAssessment{level: LevelCritical, source: RabbitMQStatusAlarm}
		}
	}
	if missing {
		return rabbitMQAssessment{level: LevelUnknown, source: RabbitMQStatusAlarm}
	}
	return rabbitMQAssessment{level: LevelNormal, source: RabbitMQStatusNormal}
}

func rabbitMQMemoryAssessment(node rabbitmq.Node) rabbitMQAssessment {
	return rabbitMQRatioAssessment(node.MemoryUsedBytes, node.MemoryLimitBytes, RabbitMQStatusMemory)
}

func rabbitMQFileDescriptorAssessment(node rabbitmq.Node) rabbitMQAssessment {
	return rabbitMQRatioAssessment(node.OpenFileDescriptors, node.MaxFileDescriptors, RabbitMQStatusFileDescriptor)
}

func rabbitMQErlangProcessAssessment(node rabbitmq.Node) rabbitMQAssessment {
	return rabbitMQRatioAssessment(node.ErlangProcessesUsed, node.ErlangProcessesLimit, RabbitMQStatusErlangProcess)
}

func rabbitMQRatioAssessment(used, limit *int64, source RabbitMQNodeStatusSource) rabbitMQAssessment {
	usage := rabbitMQUsagePercent(used, limit)
	if usage == nil {
		return rabbitMQAssessment{level: LevelUnknown, source: source}
	}
	if *usage >= 90 {
		return rabbitMQAssessment{level: LevelCritical, source: source}
	}
	if *usage >= 80 {
		return rabbitMQAssessment{level: LevelWarning, source: source}
	}
	return rabbitMQAssessment{level: LevelNormal, source: RabbitMQStatusNormal}
}

func rabbitMQDiskAssessment(node rabbitmq.Node) rabbitMQAssessment {
	if node.DiskAvailableBytes == nil || node.DiskLimitBytes == nil || *node.DiskAvailableBytes < 0 || *node.DiskLimitBytes <= 0 {
		return rabbitMQAssessment{level: LevelUnknown, source: RabbitMQStatusDisk}
	}
	ratio := float64(*node.DiskAvailableBytes) / float64(*node.DiskLimitBytes)
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return rabbitMQAssessment{level: LevelUnknown, source: RabbitMQStatusDisk}
	}
	if ratio <= 1 {
		return rabbitMQAssessment{level: LevelCritical, source: RabbitMQStatusDisk}
	}
	if ratio < 1.2 {
		return rabbitMQAssessment{level: LevelWarning, source: RabbitMQStatusDisk}
	}
	return rabbitMQAssessment{level: LevelNormal, source: RabbitMQStatusNormal}
}

func rabbitMQResourceAssessment(node rabbitmq.Node) rabbitMQAssessment {
	assessment := rabbitMQAssessment{level: LevelNormal, source: RabbitMQStatusNormal}
	for _, candidate := range []rabbitMQAssessment{
		rabbitMQDiskAssessment(node),
		rabbitMQMemoryAssessment(node),
		rabbitMQFileDescriptorAssessment(node),
		rabbitMQErlangProcessAssessment(node),
	} {
		assessment = chooseRabbitMQAssessment(assessment, candidate)
	}
	return assessment
}

func rabbitMQUsagePercent(used, limit *int64) *float64 {
	if used == nil || limit == nil || *used < 0 || *limit <= 0 {
		return nil
	}
	usage := float64(*used) / float64(*limit) * 100
	if math.IsNaN(usage) || math.IsInf(usage, 0) {
		return nil
	}
	return &usage
}

func chooseRabbitMQAssessment(left, right rabbitMQAssessment) rabbitMQAssessment {
	if right.level == LevelNormal {
		return left
	}
	leftLevel := rabbitMQLevelRank(left.level)
	rightLevel := rabbitMQLevelRank(right.level)
	if rightLevel > leftLevel || rightLevel == leftLevel && rabbitMQSourceRank(right.source) > rabbitMQSourceRank(left.source) {
		return right
	}
	return left
}

func rabbitMQLevelRank(level Level) int {
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

func rabbitMQSourceRank(source RabbitMQNodeStatusSource) int {
	switch source {
	case RabbitMQStatusAlarm:
		return 6
	case RabbitMQStatusCollection:
		return 5
	case RabbitMQStatusDisk:
		return 4
	case RabbitMQStatusMemory:
		return 3
	case RabbitMQStatusFileDescriptor:
		return 2
	case RabbitMQStatusErlangProcess:
		return 1
	default:
		return 0
	}
}

func rabbitMQClusterConnectivityLevel(cluster rabbitmq.Cluster) Level {
	if cluster.UnreachablePeers == nil || *cluster.UnreachablePeers < 0 {
		return LevelUnknown
	}
	if *cluster.UnreachablePeers > 0 {
		return LevelCritical
	}
	return LevelNormal
}

func higherRabbitMQLevel(left, right Level) Level {
	if rabbitMQLevelRank(right) > rabbitMQLevelRank(left) {
		return right
	}
	return left
}

func addRabbitMQLevel(counts *RabbitMQLevelCounts, level Level) {
	switch level {
	case LevelCritical:
		counts.Critical++
	case LevelWarning:
		counts.Warning++
	case LevelUnknown:
		counts.Unknown++
	default:
		counts.Normal++
	}
}

func addRabbitMQAlert(count *RabbitMQAlertCount, level Level) {
	switch level {
	case LevelCritical:
		count.Critical++
	case LevelWarning:
		count.Warning++
	case LevelUnknown:
		count.Unknown++
	}
}

func sortRabbitMQNodes(items []RabbitMQNodeSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftInteger, leftIntegerOK, integer := rabbitMQIntegerSortValue(items[i], field)
		rightInteger, rightIntegerOK, _ := rabbitMQIntegerSortValue(items[j], field)
		if integer && leftIntegerOK != rightIntegerOK {
			return leftIntegerOK
		}
		leftNumber, leftNumberOK, numeric := rabbitMQNumberSortValue(items[i], field)
		rightNumber, rightNumberOK, _ := rabbitMQNumberSortValue(items[j], field)
		if numeric && leftNumberOK != rightNumberOK {
			return leftNumberOK
		}
		comparison := 0
		if integer && leftIntegerOK {
			comparison = compareInt64(leftInteger, rightInteger)
		} else if numeric && leftNumberOK {
			comparison = compareFloat64(leftNumber, rightNumber)
		} else if !integer && !numeric {
			comparison = compareRabbitMQNodeField(items[i], items[j], field)
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

func rabbitMQIntegerSortValue(item RabbitMQNodeSummary, field string) (int64, bool, bool) {
	var value *int64
	switch field {
	case "disk":
		value = item.DiskAvailableBytes
	case "connections":
		value = item.Connections
	case "queues":
		value = item.Queues
	case "messages":
		value = item.Messages
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

func rabbitMQNumberSortValue(item RabbitMQNodeSummary, field string) (float64, bool, bool) {
	var value *float64
	switch field {
	case "memory":
		value = item.MemoryUsagePercent
	case "file_descriptor":
		value = item.FileDescriptorUsagePercent
	case "erlang_process":
		value = item.ErlangProcessUsagePercent
	case "publish_rate":
		value = item.PublishRate
	case "deliver_rate":
		value = item.DeliverRate
	default:
		return 0, false, false
	}
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return 0, false, true
	}
	return *value, true, true
}

func compareRabbitMQNodeField(left, right RabbitMQNodeSummary, field string) int {
	switch field {
	case "node":
		return compareRabbitMQText(left.Name, right.Name)
	case "cluster":
		if comparison := compareRabbitMQText(left.Cluster, right.Cluster); comparison != 0 {
			return comparison
		}
		return compareRabbitMQText(left.Name, right.Name)
	case "address":
		return compareRabbitMQText(left.Address, right.Address)
	case "version":
		return compareRabbitMQText(left.Version, right.Version)
	case "status":
		return rabbitMQLevelRank(left.Status) - rabbitMQLevelRank(right.Status)
	default:
		return 0
	}
}

func compareRabbitMQText(left, right string) int {
	return compareNatural(strings.ToLower(left), strings.ToLower(right))
}
