package nightingale

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/rabbitmq"
)

type rabbitMQLatest[T comparable] struct {
	value     *T
	timestamp time.Time
	conflict  bool
}

type rabbitMQClusterState struct {
	cluster     rabbitmq.Cluster
	identity    string
	collections map[string]struct{}
}

type rabbitMQInventoryCandidate struct {
	joinKey, nodeID, clusterID, clusterIdentity string
	nodeName, clusterName                       string
	collection                                  string
	address                                     string
	sampleAt                                    time.Time
}

type rabbitMQNodeState struct {
	node      rabbitmq.Node
	clusterID string

	version              rabbitMQLatest[string]
	uptime               rabbitMQLatest[int64]
	memoryAlarm          rabbitMQLatest[bool]
	diskAlarm            rabbitMQLatest[bool]
	fileDescriptorAlarm  rabbitMQLatest[bool]
	memoryUsed           rabbitMQLatest[int64]
	memoryLimit          rabbitMQLatest[int64]
	diskAvailable        rabbitMQLatest[int64]
	diskLimit            rabbitMQLatest[int64]
	openFDs              rabbitMQLatest[int64]
	maxFDs               rabbitMQLatest[int64]
	erlangProcessesUsed  rabbitMQLatest[int64]
	erlangProcessesLimit rabbitMQLatest[int64]
	connections          rabbitMQLatest[int64]
	queues               rabbitMQLatest[int64]
	messages             rabbitMQLatest[int64]
	publishRate          rabbitMQLatest[float64]
	deliverRate          rabbitMQLatest[float64]
	unreachablePeers     rabbitMQLatest[int64]
}

type rabbitMQNodeIndex struct {
	byID      map[string]*rabbitMQNodeState
	byJoinKey map[string][]*rabbitMQNodeState
}

var _ rabbitmq.Provider = (*Provider)(nil)

func (p *Provider) RabbitMQSnapshot(ctx context.Context) (rabbitmq.Snapshot, error) {
	if err := p.ready(); err != nil {
		return rabbitmq.Snapshot{}, rabbitMQUnavailableError()
	}
	results, err := p.queryInstant(ctx, rabbitMQPromQL())
	if err != nil || len(results) != rabbitMQQueryCount {
		return rabbitmq.Snapshot{}, rabbitMQUnavailableError()
	}
	snapshot, err := buildRabbitMQSnapshot(results)
	if err != nil {
		return rabbitmq.Snapshot{}, rabbitMQUnavailableError()
	}
	return snapshot, nil
}

func buildRabbitMQSnapshot(results [][]instantSeries) (rabbitmq.Snapshot, error) {
	if len(results) != rabbitMQQueryCount || results[rabbitMQInventoryQuery] == nil {
		return rabbitmq.Snapshot{}, rabbitMQUnavailableError()
	}
	nodes, clusters, err := buildRabbitMQInventory(
		results[rabbitMQIdentityQuery],
		results[rabbitMQInventoryQuery],
	)
	if err != nil {
		return rabbitmq.Snapshot{}, err
	}
	discoverRabbitMQNodesFromConnections(
		nodes,
		clusters,
		results[rabbitMQConnectionsQuery],
		rabbitMQNodeNameHints(results),
	)
	for index := 0; index < rabbitMQInventoryQuery; index++ {
		mergeRabbitMQQuery(nodes, index, results[index])
	}
	mergeRabbitMQCollection(nodes, results[rabbitMQCollectionQuery])
	aggregateRabbitMQUnreachablePeers(nodes, clusters)

	snapshot := rabbitmq.Snapshot{
		Clusters: make([]rabbitmq.Cluster, 0, len(clusters)),
		Nodes:    make([]rabbitmq.Node, 0, len(nodes.byID)),
	}
	clusterKeys := sortedRabbitMQKeys(clusters)
	for _, key := range clusterKeys {
		state := clusters[key]
		snapshot.Clusters = append(snapshot.Clusters, state.cluster)
	}
	nodeKeys := sortedRabbitMQKeys(nodes.byID)
	for _, key := range nodeKeys {
		state := nodes.byID[key]
		finalizeRabbitMQNode(state)
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

func buildRabbitMQInventory(current, historical []instantSeries) (*rabbitMQNodeIndex, map[string]*rabbitMQClusterState, error) {
	byNodeID := make(map[string]rabbitMQInventoryCandidate, len(current)+len(historical))
	nodeConflicts := make(map[string]bool)
	for _, group := range []struct {
		series              []instantSeries
		useReportedSampleAt bool
	}{
		{series: historical, useReportedSampleAt: true},
		{series: current},
	} {
		for _, candidate := range group.series {
			parsed, ok := parseRabbitMQInventoryCandidate(candidate, group.useReportedSampleAt)
			if !ok {
				continue
			}
			existing, exists := byNodeID[parsed.nodeID]
			switch {
			case !exists, parsed.sampleAt.After(existing.sampleAt):
				byNodeID[parsed.nodeID] = parsed
				nodeConflicts[parsed.nodeID] = false
			case parsed.sampleAt.Equal(existing.sampleAt) &&
				(parsed.joinKey != existing.joinKey || !sameRabbitMQInventoryIdentity(existing, parsed)):
				nodeConflicts[parsed.nodeID] = true
			}
		}
	}

	nodes := &rabbitMQNodeIndex{
		byID:      make(map[string]*rabbitMQNodeState, len(byNodeID)),
		byJoinKey: make(map[string][]*rabbitMQNodeState),
	}
	clusters := make(map[string]*rabbitMQClusterState)
	for _, nodeID := range sortedRabbitMQKeys(byNodeID) {
		if nodeConflicts[nodeID] {
			continue
		}
		candidate := byNodeID[nodeID]
		if existing, exists := clusters[candidate.clusterID]; exists && existing.cluster.Name != candidate.clusterName {
			continue
		}
		if _, exists := clusters[candidate.clusterID]; !exists {
			clusters[candidate.clusterID] = &rabbitMQClusterState{
				cluster:     rabbitmq.Cluster{ID: candidate.clusterID, Name: candidate.clusterName},
				identity:    candidate.clusterIdentity,
				collections: make(map[string]struct{}),
			}
		}
		clusters[candidate.clusterID].collections[candidate.collection] = struct{}{}
		state := &rabbitMQNodeState{
			node: rabbitmq.Node{
				ID:      candidate.nodeID,
				Name:    candidate.nodeName,
				Cluster: candidate.clusterName,
				Address: candidate.address,
			},
			clusterID: candidate.clusterID,
		}
		nodes.byID[nodeID] = state
		nodes.byJoinKey[candidate.joinKey] = append(nodes.byJoinKey[candidate.joinKey], state)
	}
	return nodes, clusters, nil
}

func discoverRabbitMQNodesFromConnections(
	nodes *rabbitMQNodeIndex,
	clusters map[string]*rabbitMQClusterState,
	series []instantSeries,
	nameHints map[string]string,
) {
	for _, candidate := range series {
		joinKey, ok := rabbitMQSampleKey(candidate.Metric)
		if !ok || len(nodes.byJoinKey[joinKey]) > 0 {
			continue
		}
		instance := strings.TrimSpace(candidate.Metric["instance"])
		collection := strings.TrimSpace(candidate.Metric["cluster"])
		cluster := rabbitMQClusterForCollection(clusters, collection)
		if cluster == nil {
			clusterID := rabbitmq.StableClusterID(collection)
			if clusterID == "" {
				continue
			}
			cluster = &rabbitMQClusterState{
				cluster:     rabbitmq.Cluster{ID: clusterID, Name: collection},
				identity:    collection,
				collections: map[string]struct{}{collection: {}},
			}
			clusters[clusterID] = cluster
		}
		nodeName := strings.TrimSpace(candidate.Metric["rabbitmq_node"])
		if nodeName == "" {
			nodeName = nameHints[joinKey]
		}
		if nodeName != "" && rabbitMQNodeNameExists(nodes, cluster.cluster.ID, nodeName) {
			continue
		}
		nodeID := rabbitMQDiscoveredNodeID(cluster.identity, nodeName, instance)
		if nodeID == "" {
			continue
		}
		state := &rabbitMQNodeState{
			node: rabbitmq.Node{
				ID:      nodeID,
				Name:    nodeName,
				Cluster: cluster.cluster.Name,
				Address: instance,
			},
			clusterID: cluster.cluster.ID,
		}
		nodes.byID[nodeID] = state
		nodes.byJoinKey[joinKey] = append(nodes.byJoinKey[joinKey], state)
	}
}

func rabbitMQNodeNameHints(results [][]instantSeries) map[string]string {
	hints := make(map[string]string)
	conflicts := make(map[string]bool)
	for _, group := range results {
		for _, series := range group {
			joinKey, ok := rabbitMQSampleKey(series.Metric)
			name := strings.TrimSpace(series.Metric["rabbitmq_node"])
			if !ok || name == "" || conflicts[joinKey] {
				continue
			}
			if existing, exists := hints[joinKey]; exists && existing != name {
				delete(hints, joinKey)
				conflicts[joinKey] = true
				continue
			}
			hints[joinKey] = name
		}
	}
	return hints
}

func rabbitMQDiscoveredNodeID(clusterIdentity, nodeName, instance string) string {
	stableIdentity := strings.TrimSpace(nodeName)
	if stableIdentity == "" {
		instance = strings.TrimSpace(instance)
		if instance == "" {
			return ""
		}
		stableIdentity = "observed-instance\x00" + instance
	}
	return rabbitmq.StableNodeID(clusterIdentity, stableIdentity)
}

func rabbitMQNodeNameExists(nodes *rabbitMQNodeIndex, clusterID, nodeName string) bool {
	for _, nodeID := range sortedRabbitMQKeys(nodes.byID) {
		state := nodes.byID[nodeID]
		if state.clusterID == clusterID && state.node.Name == nodeName {
			return true
		}
	}
	return false
}

func rabbitMQClusterForCollection(clusters map[string]*rabbitMQClusterState, collection string) *rabbitMQClusterState {
	var matched *rabbitMQClusterState
	for _, clusterID := range sortedRabbitMQKeys(clusters) {
		cluster := clusters[clusterID]
		if _, exists := cluster.collections[collection]; !exists {
			continue
		}
		if matched != nil {
			return nil
		}
		matched = cluster
	}
	if matched == nil && len(clusters) == 1 {
		for _, cluster := range clusters {
			return cluster
		}
	}
	return matched
}

func parseRabbitMQInventoryCandidate(series instantSeries, useReportedSampleAt bool) (rabbitMQInventoryCandidate, bool) {
	nodeName := strings.TrimSpace(series.Metric["rabbitmq_node"])
	joinKey, ok := rabbitMQSampleKey(series.Metric)
	if nodeName == "" || !ok || len(series.Value) != 2 {
		return rabbitMQInventoryCandidate{}, false
	}
	sampleAtValue := series.Value[0]
	if useReportedSampleAt {
		sampleAtValue = series.Value[1]
	}
	sampleAt, ok := parseUnixTime(sampleAtValue)
	if !ok {
		return rabbitMQInventoryCandidate{}, false
	}
	permanent := strings.TrimSpace(series.Metric["rabbitmq_cluster_permanent_id"])
	logical := strings.TrimSpace(series.Metric["rabbitmq_cluster"])
	collection := strings.TrimSpace(series.Metric["cluster"])
	clusterIdentity := permanent
	if clusterIdentity == "" {
		clusterIdentity = logical
	}
	if clusterIdentity == "" {
		clusterIdentity = collection
	}
	clusterName := logical
	if clusterName == "" {
		clusterName = collection
	}
	clusterID := rabbitmq.StableClusterID(clusterIdentity)
	nodeID := rabbitmq.StableNodeID(clusterIdentity, nodeName)
	if clusterID == "" || nodeID == "" {
		return rabbitMQInventoryCandidate{}, false
	}
	return rabbitMQInventoryCandidate{
		joinKey: joinKey, nodeID: nodeID, clusterID: clusterID, clusterIdentity: clusterIdentity,
		nodeName: nodeName, clusterName: clusterName,
		collection: collection,
		address:    strings.TrimSpace(series.Metric["instance"]), sampleAt: sampleAt,
	}, true
}

func sameRabbitMQInventoryIdentity(first, second rabbitMQInventoryCandidate) bool {
	return first.nodeID == second.nodeID && first.clusterID == second.clusterID &&
		first.nodeName == second.nodeName && first.clusterName == second.clusterName &&
		first.address == second.address
}

func mergeRabbitMQQuery(nodes *rabbitMQNodeIndex, queryIndex int, series []instantSeries) {
	for _, candidate := range series {
		state, ok := nodes.match(candidate.Metric)
		if !ok {
			continue
		}
		switch queryIndex {
		case rabbitMQBuildInfoQuery:
			mergeRabbitMQText(&state.version, candidate.Metric["rabbitmq_version"], candidate.Value)
		case rabbitMQUptimeQuery:
			mergeRabbitMQParsed(&state.uptime, candidate.Value, rabbitMQUptimeSeconds)
		case rabbitMQMemoryAlarmQuery:
			mergeRabbitMQParsed(&state.memoryAlarm, candidate.Value, rabbitMQAlarm)
		case rabbitMQDiskAlarmQuery:
			mergeRabbitMQParsed(&state.diskAlarm, candidate.Value, rabbitMQAlarm)
		case rabbitMQFileDescriptorAlarmQuery:
			mergeRabbitMQParsed(&state.fileDescriptorAlarm, candidate.Value, rabbitMQAlarm)
		case rabbitMQUnreachablePeersQuery:
			mergeRabbitMQParsed(&state.unreachablePeers, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQMemoryUsedQuery:
			mergeRabbitMQParsed(&state.memoryUsed, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQMemoryLimitQuery:
			mergeRabbitMQParsed(&state.memoryLimit, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQDiskAvailableQuery:
			mergeRabbitMQParsed(&state.diskAvailable, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQDiskLimitQuery:
			mergeRabbitMQParsed(&state.diskLimit, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQOpenFDsQuery:
			mergeRabbitMQParsed(&state.openFDs, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQMaxFDsQuery:
			mergeRabbitMQParsed(&state.maxFDs, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQErlangProcessesUsedQuery:
			mergeRabbitMQParsed(&state.erlangProcessesUsed, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQErlangProcessesLimitQuery:
			mergeRabbitMQParsed(&state.erlangProcessesLimit, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQConnectionsQuery:
			mergeRabbitMQParsed(&state.connections, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQQueuesQuery:
			mergeRabbitMQParsed(&state.queues, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQMessagesQuery:
			mergeRabbitMQParsed(&state.messages, candidate.Value, rabbitMQNonNegativeInt)
		case rabbitMQPublishRateQuery:
			mergeRabbitMQParsed(&state.publishRate, candidate.Value, rabbitMQFiniteNonNegative)
		case rabbitMQDeliverRateQuery:
			mergeRabbitMQParsed(&state.deliverRate, candidate.Value, rabbitMQFiniteNonNegative)
		}
	}
}

func aggregateRabbitMQUnreachablePeers(nodes *rabbitMQNodeIndex, clusters map[string]*rabbitMQClusterState) {
	total := make(map[string]int, len(clusters))
	explicitZero := make(map[string]int, len(clusters))
	positiveMax := make(map[string]int64, len(clusters))
	for _, key := range sortedRabbitMQKeys(nodes.byID) {
		state := nodes.byID[key]
		total[state.clusterID]++
		if state.unreachablePeers.value == nil {
			continue
		}
		value := *state.unreachablePeers.value
		if value == 0 {
			explicitZero[state.clusterID]++
			continue
		}
		if current, exists := positiveMax[state.clusterID]; !exists || value > current {
			positiveMax[state.clusterID] = value
		}
	}
	for _, clusterID := range sortedRabbitMQKeys(clusters) {
		cluster := clusters[clusterID]
		if value, exists := positiveMax[clusterID]; exists {
			cluster.cluster.UnreachablePeers = &value
			continue
		}
		if total[clusterID] > 0 && explicitZero[clusterID] == total[clusterID] {
			value := int64(0)
			cluster.cluster.UnreachablePeers = &value
		}
	}
}

func mergeRabbitMQCollection(nodes *rabbitMQNodeIndex, series []instantSeries) {
	for _, candidate := range series {
		state, ok := nodes.match(candidate.Metric)
		if !ok || len(candidate.Value) != 2 {
			continue
		}
		reportedAt, ok := parseUnixTime(candidate.Value[1])
		if !ok || !reportedAt.After(state.node.ReportedAt) {
			continue
		}
		state.node.ReportedAt = reportedAt
		state.node.CollectionTracked = true
	}
}

func (nodes *rabbitMQNodeIndex) match(metric map[string]string) (*rabbitMQNodeState, bool) {
	joinKey, ok := rabbitMQSampleKey(metric)
	if !ok {
		return nil, false
	}
	candidates := nodes.byJoinKey[joinKey]
	nodeName := strings.TrimSpace(metric["rabbitmq_node"])
	if nodeName == "" {
		if len(candidates) != 1 {
			return nil, false
		}
		return candidates[0], true
	}
	var matched *rabbitMQNodeState
	for _, candidate := range candidates {
		if candidate.node.Name != nodeName {
			continue
		}
		if matched != nil {
			return nil, false
		}
		matched = candidate
	}
	return matched, matched != nil
}

func mergeRabbitMQText(target *rabbitMQLatest[string], raw string, sample []json.RawMessage) {
	value := strings.TrimSpace(raw)
	if value == "" || len(sample) != 2 {
		return
	}
	timestamp, ok := parseUnixTime(sample[0])
	if !ok {
		return
	}
	mergeRabbitMQLatest(target, value, timestamp)
}

func mergeRabbitMQParsed[T comparable](target *rabbitMQLatest[T], raw []json.RawMessage, parser func(json.RawMessage) (*T, bool)) {
	if len(raw) != 2 {
		return
	}
	timestamp, ok := parseUnixTime(raw[0])
	if !ok {
		return
	}
	value, ok := parser(raw[1])
	if !ok || value == nil {
		return
	}
	mergeRabbitMQLatest(target, *value, timestamp)
}

func mergeRabbitMQLatest[T comparable](target *rabbitMQLatest[T], value T, timestamp time.Time) {
	if timestamp.Before(target.timestamp) {
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

func finalizeRabbitMQNode(state *rabbitMQNodeState) {
	if state.version.value != nil {
		state.node.Version = *state.version.value
	}
	state.node.UptimeSeconds = state.uptime.value
	state.node.MemoryAlarm = state.memoryAlarm.value
	state.node.DiskAlarm = state.diskAlarm.value
	state.node.FileDescriptorAlarm = state.fileDescriptorAlarm.value
	state.node.MemoryUsedBytes = state.memoryUsed.value
	state.node.MemoryLimitBytes = state.memoryLimit.value
	state.node.DiskAvailableBytes = state.diskAvailable.value
	state.node.DiskLimitBytes = state.diskLimit.value
	state.node.OpenFileDescriptors = state.openFDs.value
	state.node.MaxFileDescriptors = state.maxFDs.value
	state.node.ErlangProcessesUsed = state.erlangProcessesUsed.value
	state.node.ErlangProcessesLimit = state.erlangProcessesLimit.value
	state.node.Connections = state.connections.value
	state.node.Queues = state.queues.value
	state.node.Messages = state.messages.value
	state.node.PublishRate = state.publishRate.value
	state.node.DeliverRate = state.deliverRate.value
}

func rabbitMQNonNegativeInt(raw json.RawMessage) (*int64, bool) {
	text := rabbitMQNumberText(raw)
	if text == "" || len(text) > 20 {
		return nil, false
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return nil, false
	}
	return &value, true
}

func rabbitMQFiniteNonNegative(raw json.RawMessage) (*float64, bool) {
	value, err := strconv.ParseFloat(rabbitMQNumberText(raw), 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, false
	}
	return &value, true
}

func rabbitMQUptimeSeconds(raw json.RawMessage) (*int64, bool) {
	text := rabbitMQNumberText(raw)
	if text == "" || strings.HasPrefix(text, "-") {
		return nil, false
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok || value.Sign() < 0 {
		return nil, false
	}
	integer := new(big.Int).Quo(value.Num(), value.Denom())
	if !integer.IsInt64() {
		return nil, false
	}
	seconds := integer.Int64()
	return &seconds, true
}

func rabbitMQAlarm(raw json.RawMessage) (*bool, bool) {
	value, ok := rabbitMQNonNegativeInt(raw)
	if !ok || value == nil || *value > 1 {
		return nil, false
	}
	alarm := *value == 1
	return &alarm, true
}

func rabbitMQSampleKey(metric map[string]string) (string, bool) {
	cluster := strings.TrimSpace(metric["cluster"])
	instance := strings.TrimSpace(metric["instance"])
	if cluster == "" || instance == "" {
		return "", false
	}
	return cluster + "\x00" + instance, true
}

func rabbitMQNumberText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func sortedRabbitMQKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func rabbitMQUnavailableError() error {
	return fmt.Errorf("%w: Nightingale RabbitMQ 当前指标不可用", rabbitmq.ErrUnavailable)
}
