package nightingale

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/elasticsearch"
)

type elasticsearchLatest[T comparable] struct {
	value     T
	timestamp time.Time
	present   bool
	conflict  bool
}

type elasticsearchClusterState struct {
	cluster elasticsearch.Cluster

	availability          elasticsearchLatest[elasticsearch.Availability]
	nodeStatsAvailability elasticsearchLatest[elasticsearch.Availability]
	health                elasticsearchLatest[elasticsearch.Health]
	numberOfNodes         elasticsearchLatest[int64]
	numberOfDataNodes     elasticsearchLatest[int64]
	activePrimaryShards   elasticsearchLatest[int64]
	activeShards          elasticsearchLatest[int64]
	relocatingShards      elasticsearchLatest[int64]
	initializingShards    elasticsearchLatest[int64]
	unassignedShards      elasticsearchLatest[int64]
	pendingTasks          elasticsearchLatest[int64]
	taskMaxWaitingMillis  elasticsearchLatest[int64]
	inventoryReportedAt   time.Time
}

type elasticsearchNodeState struct {
	node elasticsearch.Node

	roles                    map[elasticsearch.Role]*elasticsearchLatest[bool]
	heapUsed                 elasticsearchLatest[int64]
	heapMax                  elasticsearchLatest[int64]
	diskUsage                elasticsearchLatest[float64]
	cpuUsage                 elasticsearchLatest[float64]
	indexRates               map[string]*elasticsearchLatest[float64]
	searchRates              map[string]*elasticsearchLatest[float64]
	documents                map[string]*elasticsearchLatest[int64]
	storeSizes               map[string]*elasticsearchLatest[int64]
	uptime                   elasticsearchLatest[int64]
	threadPoolQueue          elasticsearchLatest[int64]
	rejectedRate             elasticsearchLatest[float64]
	inventoryReportedAt      time.Time
	inventoryAddressConflict bool
}

var _ elasticsearch.Provider = (*Provider)(nil)

func (provider *Provider) ElasticsearchSnapshot(ctx context.Context) (elasticsearch.Snapshot, error) {
	if err := provider.ready(); err != nil {
		return elasticsearch.Snapshot{}, elasticsearchUnavailableError()
	}
	results, err := provider.queryInstant(ctx, elasticsearchPromQL())
	if err != nil || len(results) != elasticsearchQueryCount {
		return elasticsearch.Snapshot{}, elasticsearchUnavailableError()
	}
	return buildElasticsearchSnapshot(results)
}

func buildElasticsearchSnapshot(results [][]instantSeries) (elasticsearch.Snapshot, error) {
	if len(results) != elasticsearchQueryCount ||
		results[elasticsearchClusterInventoryQuery] == nil ||
		results[elasticsearchNodeInventoryQuery] == nil {
		return elasticsearch.Snapshot{}, elasticsearchUnavailableError()
	}
	clusters, err := buildElasticsearchClusterInventory(results[elasticsearchClusterInventoryQuery])
	if err != nil {
		return elasticsearch.Snapshot{}, err
	}
	nodes, err := buildElasticsearchNodeInventory(results[elasticsearchNodeInventoryQuery])
	if err != nil {
		return elasticsearch.Snapshot{}, err
	}

	mergeElasticsearchClusterQuery(clusters, elasticsearchClusterInfoUpQuery, results[elasticsearchClusterInfoUpQuery])
	mergeElasticsearchClusterQuery(clusters, elasticsearchNodeStatsUpQuery, results[elasticsearchNodeStatsUpQuery])
	mergeElasticsearchClusterQuery(clusters, elasticsearchClusterHealthQuery, results[elasticsearchClusterHealthQuery])
	for queryIndex := elasticsearchNumberOfNodesQuery; queryIndex <= elasticsearchTaskMaxWaitingMillisQuery; queryIndex++ {
		mergeElasticsearchClusterQuery(clusters, queryIndex, results[queryIndex])
	}
	for queryIndex := elasticsearchNodeRolesQuery; queryIndex < elasticsearchClusterInventoryQuery; queryIndex++ {
		mergeElasticsearchNodeQuery(nodes, queryIndex, results[queryIndex])
	}

	snapshot := elasticsearch.Snapshot{
		Clusters: make([]elasticsearch.Cluster, 0, len(clusters)),
		Nodes:    make([]elasticsearch.Node, 0, len(nodes)),
	}
	for _, state := range clusters {
		finalizeElasticsearchCluster(state)
		snapshot.Clusters = append(snapshot.Clusters, state.cluster)
	}
	for _, state := range nodes {
		finalizeElasticsearchNode(state)
		snapshot.Nodes = append(snapshot.Nodes, state.node)
	}
	sort.Slice(snapshot.Clusters, func(i, j int) bool {
		if snapshot.Clusters[i].Name != snapshot.Clusters[j].Name {
			return snapshot.Clusters[i].Name < snapshot.Clusters[j].Name
		}
		return snapshot.Clusters[i].ID < snapshot.Clusters[j].ID
	})
	sort.Slice(snapshot.Nodes, func(i, j int) bool {
		if snapshot.Nodes[i].Cluster != snapshot.Nodes[j].Cluster {
			return snapshot.Nodes[i].Cluster < snapshot.Nodes[j].Cluster
		}
		if snapshot.Nodes[i].Name != snapshot.Nodes[j].Name {
			return snapshot.Nodes[i].Name < snapshot.Nodes[j].Name
		}
		return snapshot.Nodes[i].ID < snapshot.Nodes[j].ID
	})
	return snapshot, nil
}

func buildElasticsearchClusterInventory(series []instantSeries) (map[string]*elasticsearchClusterState, error) {
	states := make(map[string]*elasticsearchClusterState, len(series))
	for _, candidate := range series {
		cluster := strings.TrimSpace(candidate.Metric["cluster"])
		if cluster == "" {
			continue
		}
		_, reportedAt, ok := elasticsearchInventoryTimes(candidate.Value)
		if !ok {
			continue
		}
		state, exists := states[cluster]
		if !exists {
			id := elasticsearch.StableClusterID(cluster)
			if id == "" {
				return nil, elasticsearchUnavailableError()
			}
			states[cluster] = &elasticsearchClusterState{
				cluster: elasticsearch.Cluster{
					ID:                    id,
					Name:                  cluster,
					Availability:          elasticsearch.AvailabilityUnknown,
					NodeStatsAvailability: elasticsearch.AvailabilityUnknown,
					Health:                elasticsearch.HealthUnknown,
					CollectionTracked:     true,
					ReportedAt:            reportedAt,
				},
				inventoryReportedAt: reportedAt,
			}
			continue
		}
		if reportedAt.After(state.inventoryReportedAt) {
			state.inventoryReportedAt = reportedAt
			state.cluster.ReportedAt = reportedAt
		}
	}
	if len(states) == 0 {
		return nil, elasticsearchUnavailableError()
	}
	return states, nil
}

func buildElasticsearchNodeInventory(series []instantSeries) (map[string]*elasticsearchNodeState, error) {
	states := make(map[string]*elasticsearchNodeState, len(series))
	for _, candidate := range series {
		if strings.TrimSpace(candidate.Metric["cluster"]) == "" || strings.TrimSpace(candidate.Metric["name"]) == "" {
			continue
		}
		cluster, name, address, key, ok := elasticsearchNodeIdentity(candidate.Metric)
		_, reportedAt, validTimes := elasticsearchInventoryTimes(candidate.Value)
		if !ok || !validTimes {
			continue
		}
		state, exists := states[key]
		if !exists {
			id := elasticsearch.StableNodeID(cluster, name)
			if id == "" {
				continue
			}
			states[key] = &elasticsearchNodeState{
				node: elasticsearch.Node{
					ID:                id,
					Name:              name,
					Cluster:           cluster,
					Address:           address,
					Roles:             []elasticsearch.Role{},
					CollectionTracked: true,
					ReportedAt:        reportedAt,
				},
				roles:               make(map[elasticsearch.Role]*elasticsearchLatest[bool]),
				indexRates:          make(map[string]*elasticsearchLatest[float64]),
				searchRates:         make(map[string]*elasticsearchLatest[float64]),
				documents:           make(map[string]*elasticsearchLatest[int64]),
				storeSizes:          make(map[string]*elasticsearchLatest[int64]),
				inventoryReportedAt: reportedAt,
			}
			continue
		}
		switch {
		case reportedAt.After(state.inventoryReportedAt):
			state.inventoryReportedAt = reportedAt
			state.inventoryAddressConflict = false
			state.node.ReportedAt = reportedAt
			state.node.Address = address
		case reportedAt.Equal(state.inventoryReportedAt):
			if state.inventoryAddressConflict {
				continue
			}
			if address != state.node.Address {
				state.inventoryAddressConflict = true
				state.node.Address = ""
			}
		}
	}
	if len(states) == 0 {
		return nil, elasticsearchUnavailableError()
	}
	return states, nil
}

func elasticsearchInventoryTimes(raw []json.RawMessage) (time.Time, time.Time, bool) {
	if len(raw) != 2 {
		return time.Time{}, time.Time{}, false
	}
	sampleAt, ok := parseUnixTime(raw[0])
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	reportedAt, ok := parseUnixTime(raw[1])
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return sampleAt, reportedAt, true
}

func elasticsearchNodeIdentity(labels map[string]string) (cluster, name, address, key string, ok bool) {
	cluster = strings.TrimSpace(labels["cluster"])
	name = strings.TrimSpace(labels["name"])
	address = strings.TrimSpace(labels["host"])
	if cluster == "" || name == "" {
		return "", "", "", "", false
	}
	return cluster, name, address, cluster + "\x00" + name, true
}

func mergeElasticsearchClusterQuery(states map[string]*elasticsearchClusterState, queryIndex int, series []instantSeries) {
	for _, candidate := range series {
		state := states[strings.TrimSpace(candidate.Metric["cluster"])]
		if state == nil {
			continue
		}
		switch queryIndex {
		case elasticsearchClusterInfoUpQuery:
			if value, timestamp, ok := elasticsearchAvailabilityValue(candidate); ok {
				mergeElasticsearchLatest(&state.availability, value, timestamp)
			}
		case elasticsearchNodeStatsUpQuery:
			if value, timestamp, ok := elasticsearchAvailabilityValue(candidate); ok {
				mergeElasticsearchLatest(&state.nodeStatsAvailability, value, timestamp)
			}
		case elasticsearchClusterHealthQuery:
			if value, timestamp, ok := elasticsearchHealthValue(candidate); ok {
				mergeElasticsearchLatest(&state.health, value, timestamp)
			}
		default:
			value, timestamp, ok := elasticsearchIntValue(candidate)
			if !ok {
				continue
			}
			if target := elasticsearchClusterIntTarget(state, queryIndex); target != nil {
				mergeElasticsearchLatest(target, value, timestamp)
			}
		}
	}
}

func elasticsearchClusterIntTarget(state *elasticsearchClusterState, queryIndex int) *elasticsearchLatest[int64] {
	switch queryIndex {
	case elasticsearchNumberOfNodesQuery:
		return &state.numberOfNodes
	case elasticsearchNumberOfDataNodesQuery:
		return &state.numberOfDataNodes
	case elasticsearchActivePrimaryShardsQuery:
		return &state.activePrimaryShards
	case elasticsearchActiveShardsQuery:
		return &state.activeShards
	case elasticsearchRelocatingShardsQuery:
		return &state.relocatingShards
	case elasticsearchInitializingShardsQuery:
		return &state.initializingShards
	case elasticsearchUnassignedShardsQuery:
		return &state.unassignedShards
	case elasticsearchPendingTasksQuery:
		return &state.pendingTasks
	case elasticsearchTaskMaxWaitingMillisQuery:
		return &state.taskMaxWaitingMillis
	default:
		return nil
	}
}

func mergeElasticsearchNodeQuery(states map[string]*elasticsearchNodeState, queryIndex int, series []instantSeries) {
	for _, candidate := range series {
		cluster := strings.TrimSpace(candidate.Metric["cluster"])
		name := strings.TrimSpace(candidate.Metric["name"])
		state := states[cluster+"\x00"+name]
		if state == nil {
			continue
		}
		switch queryIndex {
		case elasticsearchNodeRolesQuery:
			mergeElasticsearchRole(state, candidate)
		case elasticsearchHeapUsedQuery:
			mergeElasticsearchNodeInt(&state.heapUsed, candidate)
		case elasticsearchHeapMaxQuery:
			mergeElasticsearchNodeInt(&state.heapMax, candidate)
		case elasticsearchDiskUsageQuery:
			mergeElasticsearchNodeFloat(&state.diskUsage, candidate, elasticsearchPercent)
		case elasticsearchCPUUsageQuery:
			mergeElasticsearchNodeFloat(&state.cpuUsage, candidate, elasticsearchPercent)
		case elasticsearchIndexRateQuery:
			mergeElasticsearchSemanticFloat(state.indexRates, candidate)
		case elasticsearchSearchRateQuery:
			mergeElasticsearchSemanticFloat(state.searchRates, candidate)
		case elasticsearchDocumentsQuery:
			mergeElasticsearchSemanticInt(state.documents, candidate)
		case elasticsearchStoreSizeQuery:
			mergeElasticsearchSemanticInt(state.storeSizes, candidate)
		case elasticsearchUptimeQuery:
			mergeElasticsearchNodeUptime(&state.uptime, candidate)
		case elasticsearchThreadPoolQueueQuery:
			mergeElasticsearchNodeInt(&state.threadPoolQueue, candidate)
		case elasticsearchRejectedRateQuery:
			mergeElasticsearchNodeFloat(&state.rejectedRate, candidate, elasticsearchNonNegative)
		}
	}
}

func mergeElasticsearchRole(state *elasticsearchNodeState, candidate instantSeries) {
	role, ok := canonicalElasticsearchRole(candidate.Metric["role"])
	if !ok {
		return
	}
	value, timestamp, ok := elasticsearchBinaryValue(candidate)
	if !ok {
		return
	}
	target := state.roles[role]
	if target == nil {
		target = &elasticsearchLatest[bool]{}
		state.roles[role] = target
	}
	mergeElasticsearchLatest(target, value, timestamp)
}

func mergeElasticsearchNodeInt(target *elasticsearchLatest[int64], candidate instantSeries) {
	if value, timestamp, ok := elasticsearchIntValue(candidate); ok {
		mergeElasticsearchLatest(target, value, timestamp)
	}
}

func mergeElasticsearchNodeUptime(target *elasticsearchLatest[int64], candidate instantSeries) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || !elasticsearchNonNegative(value) || value >= float64(math.MaxInt64) {
		return
	}
	mergeElasticsearchLatest(target, int64(math.Floor(value)), timestamp)
}

func mergeElasticsearchNodeFloat(target *elasticsearchLatest[float64], candidate instantSeries, validator func(float64) bool) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if ok && validator(value) {
		mergeElasticsearchLatest(target, value, timestamp)
	}
}

func mergeElasticsearchSemanticInt(targets map[string]*elasticsearchLatest[int64], candidate instantSeries) {
	value, timestamp, ok := elasticsearchIntValue(candidate)
	if !ok {
		return
	}
	key := elasticsearchSemanticKey(candidate.Metric)
	target := targets[key]
	if target == nil {
		target = &elasticsearchLatest[int64]{}
		targets[key] = target
	}
	mergeElasticsearchLatest(target, value, timestamp)
}

func mergeElasticsearchSemanticFloat(targets map[string]*elasticsearchLatest[float64], candidate instantSeries) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || !elasticsearchNonNegative(value) {
		return
	}
	key := elasticsearchSemanticKey(candidate.Metric)
	target := targets[key]
	if target == nil {
		target = &elasticsearchLatest[float64]{}
		targets[key] = target
	}
	mergeElasticsearchLatest(target, value, timestamp)
}

func elasticsearchSemanticKey(labels map[string]string) string {
	ignored := map[string]struct{}{
		"__name__": {}, "cluster": {}, "name": {}, "host": {}, "address": {}, "ident": {}, "instance": {},
		"job": {}, "url": {}, "cluster_uuid": {}, "es_client_node": {}, "es_data_node": {},
		"es_ingest_node": {}, "es_master_node": {},
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		if _, skip := ignored[key]; !skip {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(0)
		builder.WriteString(labels[key])
		builder.WriteByte(0)
	}
	return builder.String()
}

func elasticsearchAvailabilityValue(candidate instantSeries) (elasticsearch.Availability, time.Time, bool) {
	value, timestamp, ok := elasticsearchBinaryValue(candidate)
	if !ok {
		return elasticsearch.AvailabilityUnknown, time.Time{}, false
	}
	if value {
		return elasticsearch.AvailabilityUp, timestamp, true
	}
	return elasticsearch.AvailabilityDown, timestamp, true
}

func elasticsearchBinaryValue(candidate instantSeries) (bool, time.Time, bool) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || value != 0 && value != 1 {
		return false, time.Time{}, false
	}
	return value == 1, timestamp, true
}

func elasticsearchHealthValue(candidate instantSeries) (elasticsearch.Health, time.Time, bool) {
	if color, exists := candidate.Metric["color"]; exists {
		active, timestamp, ok := elasticsearchBinaryValue(candidate)
		if !ok || !active {
			return elasticsearch.HealthUnknown, time.Time{}, false
		}
		switch strings.TrimSpace(color) {
		case string(elasticsearch.HealthGreen):
			return elasticsearch.HealthGreen, timestamp, true
		case string(elasticsearch.HealthYellow):
			return elasticsearch.HealthYellow, timestamp, true
		case string(elasticsearch.HealthRed):
			return elasticsearch.HealthRed, timestamp, true
		default:
			return elasticsearch.HealthUnknown, time.Time{}, false
		}
	}
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok {
		return elasticsearch.HealthUnknown, time.Time{}, false
	}
	switch value {
	case 1:
		return elasticsearch.HealthGreen, timestamp, true
	case 2:
		return elasticsearch.HealthYellow, timestamp, true
	case 3:
		return elasticsearch.HealthRed, timestamp, true
	default:
		return elasticsearch.HealthUnknown, time.Time{}, false
	}
}

func elasticsearchIntValue(candidate instantSeries) (int64, time.Time, bool) {
	if len(candidate.Value) != 2 {
		return 0, time.Time{}, false
	}
	timestamp, ok := parseUnixTime(candidate.Value[0])
	if !ok {
		return 0, time.Time{}, false
	}
	var text string
	if err := json.Unmarshal(candidate.Value[1], &text); err != nil {
		text = string(candidate.Value[1])
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return 0, time.Time{}, false
	}
	return value, timestamp, true
}

func mergeElasticsearchLatest[T comparable](target *elasticsearchLatest[T], value T, timestamp time.Time) {
	if !target.present || timestamp.After(target.timestamp) {
		target.value = value
		target.timestamp = timestamp
		target.present = true
		target.conflict = false
		return
	}
	if timestamp.Equal(target.timestamp) && target.value != value {
		target.conflict = true
	}
}

func finalizeElasticsearchCluster(state *elasticsearchClusterState) {
	state.cluster.Availability = elasticsearchLatestValue(state.availability, elasticsearch.AvailabilityUnknown)
	state.cluster.NodeStatsAvailability = elasticsearchLatestValue(state.nodeStatsAvailability, elasticsearch.AvailabilityUnknown)
	state.cluster.Health = elasticsearchLatestValue(state.health, elasticsearch.HealthUnknown)
	state.cluster.NumberOfNodes = elasticsearchLatestPointer(state.numberOfNodes)
	state.cluster.NumberOfDataNodes = elasticsearchLatestPointer(state.numberOfDataNodes)
	state.cluster.ActivePrimaryShards = elasticsearchLatestPointer(state.activePrimaryShards)
	state.cluster.ActiveShards = elasticsearchLatestPointer(state.activeShards)
	state.cluster.RelocatingShards = elasticsearchLatestPointer(state.relocatingShards)
	state.cluster.InitializingShards = elasticsearchLatestPointer(state.initializingShards)
	state.cluster.UnassignedShards = elasticsearchLatestPointer(state.unassignedShards)
	state.cluster.PendingTasks = elasticsearchLatestPointer(state.pendingTasks)
	state.cluster.TaskMaxWaitingMillis = elasticsearchLatestPointer(state.taskMaxWaitingMillis)
}

func finalizeElasticsearchNode(state *elasticsearchNodeState) {
	state.node.Roles = state.node.Roles[:0]
	for _, role := range elasticsearchRoleOrder {
		if value := state.roles[role]; value != nil && value.present && !value.conflict && value.value {
			state.node.Roles = append(state.node.Roles, role)
			if role == elasticsearch.RoleData || strings.HasPrefix(string(role), "data_") {
				state.node.DataNode = true
			}
		}
	}
	state.node.HeapUsedBytes = elasticsearchLatestPointer(state.heapUsed)
	state.node.HeapMaxBytes = elasticsearchLatestPointer(state.heapMax)
	state.node.DiskUsagePercent = elasticsearchLatestPointer(state.diskUsage)
	state.node.CPUUsagePercent = elasticsearchLatestPointer(state.cpuUsage)
	state.node.IndexRate = sumElasticsearchFloats(state.indexRates)
	state.node.SearchRate = sumElasticsearchFloats(state.searchRates)
	state.node.Documents = sumElasticsearchInts(state.documents)
	state.node.StoreSizeBytes = sumElasticsearchInts(state.storeSizes)
	state.node.UptimeSeconds = elasticsearchLatestPointer(state.uptime)
	state.node.ThreadPoolQueue = elasticsearchLatestPointer(state.threadPoolQueue)
	state.node.RejectedRate = elasticsearchLatestPointer(state.rejectedRate)
}

func elasticsearchLatestValue[T comparable](state elasticsearchLatest[T], fallback T) T {
	if !state.present || state.conflict {
		return fallback
	}
	return state.value
}

func elasticsearchLatestPointer[T comparable](state elasticsearchLatest[T]) *T {
	if !state.present || state.conflict {
		return nil
	}
	value := state.value
	return &value
}

func sumElasticsearchInts(states map[string]*elasticsearchLatest[int64]) *int64 {
	if len(states) == 0 {
		return nil
	}
	var total int64
	for _, state := range states {
		if !state.present || state.conflict || state.value > math.MaxInt64-total {
			return nil
		}
		total += state.value
	}
	return &total
}

func sumElasticsearchFloats(states map[string]*elasticsearchLatest[float64]) *float64 {
	if len(states) == 0 {
		return nil
	}
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var total float64
	for _, key := range keys {
		state := states[key]
		if !state.present || state.conflict {
			return nil
		}
		total += state.value
		if math.IsNaN(total) || math.IsInf(total, 0) {
			return nil
		}
	}
	return &total
}

func canonicalElasticsearchRole(raw string) (elasticsearch.Role, bool) {
	switch strings.TrimSpace(raw) {
	case string(elasticsearch.RoleMaster):
		return elasticsearch.RoleMaster, true
	case string(elasticsearch.RoleData):
		return elasticsearch.RoleData, true
	case string(elasticsearch.RoleDataContent):
		return elasticsearch.RoleDataContent, true
	case string(elasticsearch.RoleDataHot):
		return elasticsearch.RoleDataHot, true
	case string(elasticsearch.RoleDataWarm):
		return elasticsearch.RoleDataWarm, true
	case string(elasticsearch.RoleDataCold):
		return elasticsearch.RoleDataCold, true
	case string(elasticsearch.RoleDataFrozen):
		return elasticsearch.RoleDataFrozen, true
	case string(elasticsearch.RoleIngest):
		return elasticsearch.RoleIngest, true
	case string(elasticsearch.RoleML):
		return elasticsearch.RoleML, true
	case string(elasticsearch.RoleTransform):
		return elasticsearch.RoleTransform, true
	case string(elasticsearch.RoleRemoteClusterClient):
		return elasticsearch.RoleRemoteClusterClient, true
	case string(elasticsearch.RoleClient):
		return elasticsearch.RoleClient, true
	default:
		return "", false
	}
}

var elasticsearchRoleOrder = [...]elasticsearch.Role{
	elasticsearch.RoleMaster,
	elasticsearch.RoleData,
	elasticsearch.RoleDataContent,
	elasticsearch.RoleDataHot,
	elasticsearch.RoleDataWarm,
	elasticsearch.RoleDataCold,
	elasticsearch.RoleDataFrozen,
	elasticsearch.RoleIngest,
	elasticsearch.RoleML,
	elasticsearch.RoleTransform,
	elasticsearch.RoleRemoteClusterClient,
	elasticsearch.RoleClient,
}

func elasticsearchNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func elasticsearchPercent(value float64) bool {
	return elasticsearchNonNegative(value) && value <= 100
}

func elasticsearchUnavailableError() error {
	return fmt.Errorf("%w: Nightingale Elasticsearch 当前指标不可用", elasticsearch.ErrUnavailable)
}
