package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/elasticsearch"
)

const elasticsearchSnapshotCacheKey = "service:elasticsearch:snapshot"

type ElasticsearchService struct {
	provider         elasticsearch.Provider
	store            *cache.Store
	options          ElasticsearchOptions
	clusterFreshness *freshnessTracker
	nodeFreshness    *freshnessTracker
}

type elasticsearchSnapshotState struct {
	snapshot          elasticsearch.Snapshot
	clusterAdvancedAt map[string]time.Time
	nodeAdvancedAt    map[string]time.Time
}

func NewElasticsearch(provider elasticsearch.Provider, store *cache.Store, options ElasticsearchOptions) *ElasticsearchService {
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
	return &ElasticsearchService{
		provider:         provider,
		store:            store,
		options:          options,
		clusterFreshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
		nodeFreshness:    newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (service *ElasticsearchService) snapshot(ctx context.Context) (elasticsearch.Snapshot, Meta, error) {
	state, meta, err := service.snapshotState(ctx)
	if err != nil {
		return elasticsearch.Snapshot{}, Meta{}, err
	}
	return state.snapshot.Clone(), meta, nil
}

func (service *ElasticsearchService) snapshotState(ctx context.Context) (elasticsearchSnapshotState, Meta, error) {
	result, err := service.store.GetOrLoad(
		ctx,
		elasticsearchSnapshotCacheKey,
		service.options.SnapshotTTL,
		service.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, loadErr := service.provider.ElasticsearchSnapshot(loadCtx)
			if loadErr != nil {
				return elasticsearch.Snapshot{}, loadErr
			}
			clusterSamples := make(map[string]time.Time, len(snapshot.Clusters))
			for _, cluster := range snapshot.Clusters {
				if cluster.CollectionTracked {
					clusterSamples[cluster.ID] = cluster.ReportedAt
				}
			}
			nodeSamples := make(map[string]time.Time, len(snapshot.Nodes))
			for _, node := range snapshot.Nodes {
				if node.CollectionTracked {
					nodeSamples[node.ID] = node.ReportedAt
				}
			}
			service.clusterFreshness.Observe(clusterSamples)
			service.nodeFreshness.Observe(nodeSamples)
			return elasticsearchSnapshotState{
				snapshot:          snapshot.Clone(),
				clusterAdvancedAt: captureElasticsearchProgress(service.clusterFreshness, clusterSamples),
				nodeAdvancedAt:    captureElasticsearchProgress(service.nodeFreshness, nodeSamples),
			}, nil
		},
	)
	if err != nil {
		return elasticsearchSnapshotState{}, Meta{}, err
	}
	state, ok := result.Value.(elasticsearchSnapshotState)
	if !ok {
		return elasticsearchSnapshotState{}, Meta{}, fmt.Errorf("service: elasticsearch cache contained %T", result.Value)
	}
	return cloneElasticsearchSnapshotState(state), resultMeta(result), nil
}

func (service *ElasticsearchService) Overview(ctx context.Context) (ElasticsearchOverview, Meta, error) {
	state, meta, err := service.snapshotState(ctx)
	if err != nil {
		return ElasticsearchOverview{}, Meta{}, err
	}
	snapshot := state.snapshot
	overview := ElasticsearchOverview{
		Status:   LevelNormal,
		Clusters: ElasticsearchLevelCounts{Total: len(snapshot.Clusters)},
		Nodes:    ElasticsearchLevelCounts{Total: len(snapshot.Nodes)},
	}
	healthByCluster := make(map[string]elasticsearch.Health, len(snapshot.Clusters))
	for _, cluster := range snapshot.Clusters {
		healthByCluster[cluster.Name] = cluster.Health
		collection := service.clusterCollectionLevel(cluster, state.clusterAdvancedAt)
		assessment := assessElasticsearchCluster(cluster, collection)
		addElasticsearchLevel(&overview.Clusters, assessment.level)
		overview.Status = higherElasticsearchLevel(overview.Status, assessment.level)
		switch cluster.Health {
		case elasticsearch.HealthYellow:
			overview.Alerts.ClusterHealth.Warning++
			if validElasticsearchCount(cluster.UnassignedShards) {
				overview.Alerts.UnassignedShards.Warning = saturatingAddInt64(overview.Alerts.UnassignedShards.Warning, *cluster.UnassignedShards)
			}
		case elasticsearch.HealthRed:
			overview.Alerts.ClusterHealth.Critical++
			if validElasticsearchCount(cluster.UnassignedShards) {
				overview.Alerts.UnassignedShards.Critical = saturatingAddInt64(overview.Alerts.UnassignedShards.Critical, *cluster.UnassignedShards)
			}
		}
	}
	for _, node := range snapshot.Nodes {
		health, exists := healthByCluster[node.Cluster]
		if !exists {
			health = elasticsearch.HealthUnknown
		}
		summary := summarizeElasticsearchNode(node, health, service.nodeCollectionLevel(node, state.nodeAdvancedAt))
		addElasticsearchLevel(&overview.Nodes, summary.Status)
		overview.Status = higherElasticsearchLevel(overview.Status, summary.Status)
		if summary.StatusSource == ElasticsearchNodeStatusDisk || summary.StatusSource == ElasticsearchNodeStatusJVM {
			addElasticsearchAlert(&overview.Alerts.NodeResource, summary.Status)
		}
		if validPositiveElasticsearchFloat(node.RejectedRate) {
			overview.Alerts.RequestRejections.Warning++
		}
	}
	return overview, meta, nil
}

func (service *ElasticsearchService) Nodes(ctx context.Context, query ElasticsearchQuery) (ElasticsearchPage, Meta, error) {
	query, err := normalizeElasticsearchQuery(query)
	if err != nil {
		return ElasticsearchPage{}, Meta{}, err
	}
	state, meta, err := service.snapshotState(ctx)
	if err != nil {
		return ElasticsearchPage{}, Meta{}, err
	}
	snapshot := state.snapshot
	healthByCluster := make(map[string]elasticsearch.Health, len(snapshot.Clusters))
	clusterOptions := make(map[string]struct{}, len(snapshot.Clusters))
	roleOptions := make(map[elasticsearch.Role]struct{})
	for _, cluster := range snapshot.Clusters {
		healthByCluster[cluster.Name] = cluster.Health
		if name := strings.TrimSpace(cluster.Name); name != "" {
			clusterOptions[name] = struct{}{}
		}
	}
	for _, node := range snapshot.Nodes {
		if name := strings.TrimSpace(node.Cluster); name != "" {
			clusterOptions[name] = struct{}{}
		}
		for _, role := range node.Roles {
			if validElasticsearchRole(role) {
				roleOptions[role] = struct{}{}
			}
		}
	}
	availableClusters := make([]string, 0, len(clusterOptions))
	for cluster := range clusterOptions {
		availableClusters = append(availableClusters, cluster)
	}
	sort.Strings(availableClusters)
	availableRoles := make([]elasticsearch.Role, 0, len(roleOptions))
	for role := range roleOptions {
		availableRoles = append(availableRoles, role)
	}
	sort.Slice(availableRoles, func(i, j int) bool { return availableRoles[i] < availableRoles[j] })

	search := strings.ToLower(strings.TrimSpace(query.Search))
	items := make([]ElasticsearchNodeSummary, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		health, exists := healthByCluster[node.Cluster]
		if !exists {
			health = elasticsearch.HealthUnknown
		}
		summary := summarizeElasticsearchNode(node, health, service.nodeCollectionLevel(node, state.nodeAdvancedAt))
		if query.Cluster != "" && summary.Cluster != query.Cluster ||
			query.Role != "" && !elasticsearchHasRole(summary.Roles, query.Role) ||
			query.ClusterHealth != "" && summary.ClusterHealth != query.ClusterHealth ||
			query.Status != "" && summary.Status != query.Status {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(summary.Name), search) &&
			!strings.Contains(strings.ToLower(summary.Address), search) {
			continue
		}
		items = append(items, summary)
	}
	sortElasticsearchNodes(items, query.Sort, query.Order)
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	return ElasticsearchPage{
		Nodes:             append([]ElasticsearchNodeSummary(nil), items[start:end]...),
		AvailableClusters: availableClusters,
		AvailableRoles:    availableRoles,
		Total:             total,
		Page:              query.Page,
		PageSize:          query.PageSize,
	}, meta, nil
}

func (service *ElasticsearchService) clusterCollectionLevel(cluster elasticsearch.Cluster, advancedAt map[string]time.Time) Level {
	if !cluster.CollectionTracked {
		return LevelNormal
	}
	advanced, exists := advancedAt[cluster.ID]
	if !exists || cluster.ReportedAt.IsZero() {
		return LevelCritical
	}
	return collectionLevelAt(service.options.Clock().UTC(), advanced, service.options.CollectionInterval)
}

func (service *ElasticsearchService) nodeCollectionLevel(node elasticsearch.Node, advancedAt map[string]time.Time) Level {
	if !node.CollectionTracked {
		return LevelNormal
	}
	advanced, exists := advancedAt[node.ID]
	if !exists || node.ReportedAt.IsZero() {
		return LevelCritical
	}
	return collectionLevelAt(service.options.Clock().UTC(), advanced, service.options.CollectionInterval)
}

func captureElasticsearchProgress(tracker *freshnessTracker, samples map[string]time.Time) map[string]time.Time {
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

func cloneElasticsearchSnapshotState(source elasticsearchSnapshotState) elasticsearchSnapshotState {
	return elasticsearchSnapshotState{
		snapshot:          source.snapshot.Clone(),
		clusterAdvancedAt: cloneElasticsearchProgress(source.clusterAdvancedAt),
		nodeAdvancedAt:    cloneElasticsearchProgress(source.nodeAdvancedAt),
	}
}

func cloneElasticsearchProgress(source map[string]time.Time) map[string]time.Time {
	clone := make(map[string]time.Time, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

type elasticsearchNodeAssessment struct {
	level  Level
	source ElasticsearchNodeStatusSource
}

func summarizeElasticsearchNode(source elasticsearch.Node, clusterHealth elasticsearch.Health, collection Level) ElasticsearchNodeSummary {
	assessment := elasticsearchNodeAssessment{level: LevelNormal, source: ElasticsearchNodeStatusNormal}
	for _, candidate := range []elasticsearchNodeAssessment{
		{level: collection, source: ElasticsearchNodeStatusCollection},
		elasticsearchDiskAssessment(source),
		elasticsearchJVMAssessment(source),
		elasticsearchThreadPoolAssessment(source),
	} {
		assessment = chooseElasticsearchNodeAssessment(assessment, candidate)
	}
	return ElasticsearchNodeSummary{
		ID:               source.ID,
		Name:             source.Name,
		Cluster:          source.Cluster,
		Address:          source.Address,
		Roles:            append([]elasticsearch.Role(nil), source.Roles...),
		ClusterHealth:    clusterHealth,
		HeapUsagePercent: elasticsearchHeapUsage(source.HeapUsedBytes, source.HeapMaxBytes),
		DiskUsagePercent: cloneFloat(source.DiskUsagePercent),
		CPUUsagePercent:  cloneFloat(source.CPUUsagePercent),
		IndexRate:        cloneFloat(source.IndexRate),
		SearchRate:       cloneFloat(source.SearchRate),
		Documents:        cloneInt64(source.Documents),
		StoreSizeBytes:   cloneInt64(source.StoreSizeBytes),
		ThreadPoolQueue:  cloneInt64(source.ThreadPoolQueue),
		RejectedRate:     cloneFloat(source.RejectedRate),
		UptimeSeconds:    cloneInt64(source.UptimeSeconds),
		Status:           assessment.level,
		StatusSource:     assessment.source,
		CollectionLevel:  collection,
	}
}

func elasticsearchDiskAssessment(node elasticsearch.Node) elasticsearchNodeAssessment {
	if !node.DataNode {
		return elasticsearchNodeAssessment{level: LevelNormal, source: ElasticsearchNodeStatusNormal}
	}
	if !validElasticsearchPercent(node.DiskUsagePercent) {
		return elasticsearchNodeAssessment{level: LevelUnknown, source: ElasticsearchNodeStatusDisk}
	}
	if *node.DiskUsagePercent >= 90 {
		return elasticsearchNodeAssessment{level: LevelCritical, source: ElasticsearchNodeStatusDisk}
	}
	if *node.DiskUsagePercent >= 85 {
		return elasticsearchNodeAssessment{level: LevelWarning, source: ElasticsearchNodeStatusDisk}
	}
	return elasticsearchNodeAssessment{level: LevelNormal, source: ElasticsearchNodeStatusNormal}
}

func elasticsearchJVMAssessment(node elasticsearch.Node) elasticsearchNodeAssessment {
	usage := elasticsearchHeapUsage(node.HeapUsedBytes, node.HeapMaxBytes)
	if usage == nil {
		return elasticsearchNodeAssessment{level: LevelUnknown, source: ElasticsearchNodeStatusJVM}
	}
	if *usage >= 85 {
		return elasticsearchNodeAssessment{level: LevelCritical, source: ElasticsearchNodeStatusJVM}
	}
	if *usage >= 75 {
		return elasticsearchNodeAssessment{level: LevelWarning, source: ElasticsearchNodeStatusJVM}
	}
	return elasticsearchNodeAssessment{level: LevelNormal, source: ElasticsearchNodeStatusNormal}
}

func elasticsearchThreadPoolAssessment(node elasticsearch.Node) elasticsearchNodeAssessment {
	if validPositiveElasticsearchFloat(node.RejectedRate) {
		return elasticsearchNodeAssessment{level: LevelWarning, source: ElasticsearchNodeStatusThreadPool}
	}
	return elasticsearchNodeAssessment{level: LevelNormal, source: ElasticsearchNodeStatusNormal}
}

func chooseElasticsearchNodeAssessment(left, right elasticsearchNodeAssessment) elasticsearchNodeAssessment {
	if right.level == LevelNormal {
		return left
	}
	leftLevel := elasticsearchLevelRank(left.level)
	rightLevel := elasticsearchLevelRank(right.level)
	if rightLevel > leftLevel || rightLevel == leftLevel && elasticsearchNodeSourceRank(right.source) > elasticsearchNodeSourceRank(left.source) {
		return right
	}
	return left
}

type elasticsearchClusterAssessment struct {
	level  Level
	source ElasticsearchClusterStatusSource
}

func assessElasticsearchCluster(cluster elasticsearch.Cluster, collection Level) elasticsearchClusterAssessment {
	assessment := elasticsearchClusterAssessment{level: LevelNormal, source: ElasticsearchClusterStatusNormal}
	for _, candidate := range []elasticsearchClusterAssessment{
		elasticsearchClusterAvailabilityAssessment(cluster.Availability),
		elasticsearchClusterNodeStatsAssessment(cluster.NodeStatsAvailability),
		elasticsearchClusterHealthAssessment(cluster.Health),
		{level: collection, source: ElasticsearchClusterStatusCollection},
	} {
		assessment = chooseElasticsearchClusterAssessment(assessment, candidate)
	}
	return assessment
}

func elasticsearchClusterAvailabilityAssessment(availability elasticsearch.Availability) elasticsearchClusterAssessment {
	if availability == elasticsearch.AvailabilityDown {
		return elasticsearchClusterAssessment{level: LevelCritical, source: ElasticsearchClusterStatusAvailability}
	}
	if availability != elasticsearch.AvailabilityUp {
		return elasticsearchClusterAssessment{level: LevelUnknown, source: ElasticsearchClusterStatusAvailability}
	}
	return elasticsearchClusterAssessment{level: LevelNormal, source: ElasticsearchClusterStatusNormal}
}

func elasticsearchClusterNodeStatsAssessment(availability elasticsearch.Availability) elasticsearchClusterAssessment {
	if availability == elasticsearch.AvailabilityDown {
		return elasticsearchClusterAssessment{level: LevelCritical, source: ElasticsearchClusterStatusCollection}
	}
	if availability != elasticsearch.AvailabilityUp {
		return elasticsearchClusterAssessment{level: LevelUnknown, source: ElasticsearchClusterStatusCollection}
	}
	return elasticsearchClusterAssessment{level: LevelNormal, source: ElasticsearchClusterStatusNormal}
}

func elasticsearchClusterHealthAssessment(health elasticsearch.Health) elasticsearchClusterAssessment {
	switch health {
	case elasticsearch.HealthGreen:
		return elasticsearchClusterAssessment{level: LevelNormal, source: ElasticsearchClusterStatusNormal}
	case elasticsearch.HealthYellow:
		return elasticsearchClusterAssessment{level: LevelWarning, source: ElasticsearchClusterStatusHealth}
	case elasticsearch.HealthRed:
		return elasticsearchClusterAssessment{level: LevelCritical, source: ElasticsearchClusterStatusHealth}
	default:
		return elasticsearchClusterAssessment{level: LevelUnknown, source: ElasticsearchClusterStatusHealth}
	}
}

func chooseElasticsearchClusterAssessment(left, right elasticsearchClusterAssessment) elasticsearchClusterAssessment {
	if right.level == LevelNormal {
		return left
	}
	leftLevel := elasticsearchLevelRank(left.level)
	rightLevel := elasticsearchLevelRank(right.level)
	if rightLevel > leftLevel || rightLevel == leftLevel && elasticsearchClusterSourceRank(right.source) > elasticsearchClusterSourceRank(left.source) {
		return right
	}
	return left
}

func elasticsearchLevelRank(level Level) int {
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

func elasticsearchNodeSourceRank(source ElasticsearchNodeStatusSource) int {
	switch source {
	case ElasticsearchNodeStatusCollection:
		return 4
	case ElasticsearchNodeStatusDisk:
		return 3
	case ElasticsearchNodeStatusJVM:
		return 2
	case ElasticsearchNodeStatusThreadPool:
		return 1
	default:
		return 0
	}
}

func elasticsearchClusterSourceRank(source ElasticsearchClusterStatusSource) int {
	switch source {
	case ElasticsearchClusterStatusAvailability:
		return 3
	case ElasticsearchClusterStatusHealth:
		return 2
	case ElasticsearchClusterStatusCollection:
		return 1
	default:
		return 0
	}
}

func higherElasticsearchLevel(left, right Level) Level {
	if elasticsearchLevelRank(right) > elasticsearchLevelRank(left) {
		return right
	}
	return left
}

func addElasticsearchLevel(counts *ElasticsearchLevelCounts, level Level) {
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

func addElasticsearchAlert(count *ElasticsearchAlertCount, level Level) {
	if level == LevelCritical {
		count.Critical++
	} else if level == LevelWarning || level == LevelUnknown {
		count.Warning++
	}
}

func normalizeElasticsearchQuery(query ElasticsearchQuery) (ElasticsearchQuery, error) {
	query.Search = strings.TrimSpace(query.Search)
	query.Cluster = strings.TrimSpace(query.Cluster)
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 {
		return ElasticsearchQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.PageSize {
		return ElasticsearchQuery{}, fmt.Errorf("%w: page offset overflows int", ErrInvalidQuery)
	}
	if query.Role != "" && !validElasticsearchRole(query.Role) {
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported role %q", ErrInvalidQuery, query.Role)
	}
	switch query.ClusterHealth {
	case "", elasticsearch.HealthGreen, elasticsearch.HealthYellow, elasticsearch.HealthRed, elasticsearch.HealthUnknown:
	default:
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported cluster health %q", ErrInvalidQuery, query.ClusterHealth)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = "node"
	}
	switch query.Sort {
	case "node", "cluster", "address", "role", "cluster_health", "heap", "disk", "cpu", "index_rate", "search_rate", "documents", "store", "thread_queue", "rejected_rate", "uptime", "status":
	default:
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return ElasticsearchQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

func validElasticsearchRole(role elasticsearch.Role) bool {
	switch role {
	case elasticsearch.RoleMaster, elasticsearch.RoleData, elasticsearch.RoleDataContent,
		elasticsearch.RoleDataHot, elasticsearch.RoleDataWarm, elasticsearch.RoleDataCold,
		elasticsearch.RoleDataFrozen, elasticsearch.RoleIngest, elasticsearch.RoleML,
		elasticsearch.RoleTransform, elasticsearch.RoleRemoteClusterClient, elasticsearch.RoleClient:
		return true
	default:
		return false
	}
}

func elasticsearchHasRole(roles []elasticsearch.Role, target elasticsearch.Role) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func sortElasticsearchNodes(items []ElasticsearchNodeSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftInteger, leftIntegerOK, integer := elasticsearchNodeSortInteger(items[i], field)
		rightInteger, rightIntegerOK, _ := elasticsearchNodeSortInteger(items[j], field)
		if integer && leftIntegerOK != rightIntegerOK {
			return leftIntegerOK
		}
		leftNumber, leftOK, numeric := elasticsearchNodeSortNumber(items[i], field)
		rightNumber, rightOK, _ := elasticsearchNodeSortNumber(items[j], field)
		if numeric && leftOK != rightOK {
			return leftOK
		}
		comparison := 0
		if integer && leftIntegerOK {
			comparison = compareInt64(leftInteger, rightInteger)
		} else if numeric && leftOK {
			comparison = compareFloat64(leftNumber, rightNumber)
		} else if !numeric && !integer {
			comparison = compareElasticsearchNodeField(items[i], items[j], field)
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

func elasticsearchNodeSortNumber(item ElasticsearchNodeSummary, field string) (float64, bool, bool) {
	switch field {
	case "heap":
		return elasticsearchFloatSortValue(item.HeapUsagePercent)
	case "disk":
		return elasticsearchFloatSortValue(item.DiskUsagePercent)
	case "cpu":
		return elasticsearchFloatSortValue(item.CPUUsagePercent)
	case "index_rate":
		return elasticsearchFloatSortValue(item.IndexRate)
	case "search_rate":
		return elasticsearchFloatSortValue(item.SearchRate)
	case "rejected_rate":
		return elasticsearchFloatSortValue(item.RejectedRate)
	default:
		return 0, false, false
	}
}

func elasticsearchNodeSortInteger(item ElasticsearchNodeSummary, field string) (int64, bool, bool) {
	var value *int64
	switch field {
	case "documents":
		value = item.Documents
	case "store":
		value = item.StoreSizeBytes
	case "thread_queue":
		value = item.ThreadPoolQueue
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

func compareElasticsearchNodeField(left, right ElasticsearchNodeSummary, field string) int {
	switch field {
	case "node":
		if comparison := compareElasticsearchText(left.Cluster, right.Cluster); comparison != 0 {
			return comparison
		}
		return compareElasticsearchText(left.Name, right.Name)
	case "cluster":
		if comparison := compareElasticsearchText(left.Cluster, right.Cluster); comparison != 0 {
			return comparison
		}
		return compareElasticsearchText(left.Name, right.Name)
	case "address":
		return compareElasticsearchText(left.Address, right.Address)
	case "role":
		return compareElasticsearchText(elasticsearchRoleSortKey(left.Roles), elasticsearchRoleSortKey(right.Roles))
	case "cluster_health":
		return elasticsearchHealthRank(left.ClusterHealth) - elasticsearchHealthRank(right.ClusterHealth)
	case "status":
		return elasticsearchLevelRank(left.Status) - elasticsearchLevelRank(right.Status)
	default:
		return 0
	}
}

func compareElasticsearchText(left, right string) int {
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func elasticsearchRoleSortKey(roles []elasticsearch.Role) string {
	values := append([]elasticsearch.Role(nil), roles...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	parts := make([]string, len(values))
	for index, role := range values {
		parts[index] = string(role)
	}
	return strings.Join(parts, ",")
}

func elasticsearchHealthRank(health elasticsearch.Health) int {
	switch health {
	case elasticsearch.HealthGreen:
		return 0
	case elasticsearch.HealthYellow:
		return 1
	case elasticsearch.HealthRed:
		return 2
	default:
		return 3
	}
}

func elasticsearchFloatSortValue(value *float64) (float64, bool, bool) {
	if !validElasticsearchFloat(value) {
		return 0, false, true
	}
	return *value, true, true
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func elasticsearchHeapUsage(used, maximum *int64) *float64 {
	if used == nil || maximum == nil || *used < 0 || *maximum <= 0 {
		return nil
	}
	usage := float64(*used) / float64(*maximum) * 100
	if math.IsNaN(usage) || math.IsInf(usage, 0) {
		return nil
	}
	return &usage
}

func validElasticsearchPercent(value *float64) bool {
	return validElasticsearchFloat(value) && *value >= 0
}

func validPositiveElasticsearchFloat(value *float64) bool {
	return validElasticsearchFloat(value) && *value > 0
}

func validElasticsearchFloat(value *float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func validElasticsearchCount(value *int64) bool {
	return value != nil && *value >= 0
}

func saturatingAddInt64(current int, value int64) int {
	maxInt := int(^uint(0) >> 1)
	if value > int64(maxInt-current) {
		return maxInt
	}
	return current + int(value)
}
