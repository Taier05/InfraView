package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/rabbitmq"
)

func TestRabbitMQPromQLContract(t *testing.T) {
	want := []string{
		"rabbitmq_identity_info", "rabbitmq_build_info",
		"rabbitmq_erlang_uptime_seconds",
		"rabbitmq_alarms_memory_used_watermark",
		"rabbitmq_alarms_free_disk_space_watermark",
		"rabbitmq_alarms_file_descriptor_limit",
		"rabbitmq_unreachable_cluster_peers_count",
		"rabbitmq_process_resident_memory_bytes",
		"rabbitmq_resident_memory_limit_bytes",
		"rabbitmq_disk_space_available_bytes",
		"rabbitmq_disk_space_available_limit_bytes",
		"rabbitmq_process_open_fds", "rabbitmq_process_max_fds",
		"rabbitmq_erlang_processes_used", "rabbitmq_erlang_processes_limit",
		"rabbitmq_connections", "rabbitmq_queues", "rabbitmq_queue_messages",
		"sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_received_total[5m]))",
		"sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_delivered_total[5m]))",
		"tlast_over_time(rabbitmq_identity_info[24h])",
		"tlast_over_time(rabbitmq_erlang_uptime_seconds[24h])",
	}
	got := rabbitMQPromQL()
	if len(got) != len(want) {
		t.Fatalf("RabbitMQ query count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("RabbitMQ query contract mismatch at index %d", index)
		}
	}
	got[0] = "changed"
	if rabbitMQPromQL()[0] != want[0] {
		t.Fatal("rabbitMQPromQL did not return a defensive copy")
	}
}

func TestRabbitMQSnapshotUsesExactlyOneFixedBatch(t *testing.T) {
	batchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			batchCalls++
			if batchCalls != 1 {
				t.Fatal("RabbitMQSnapshot sent a second batch")
			}
			var body batchRequest
			decodeRequest(t, request, &body)
			if len(body.Queries) != 22 {
				t.Fatalf("query groups = %d", len(body.Queries))
			}
			for index, query := range body.Queries {
				if query.Query != rabbitMQPromQL()[index] || query.Time != fixedClock().Unix() {
					t.Fatalf("RabbitMQ batch contract mismatch at index %d", index)
				}
			}
			fixture, err := os.ReadFile("testdata/rabbitmq-instant-batch.json")
			if err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	snapshot, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatalf("RabbitMQSnapshot() error = %v", err)
	}
	if batchCalls != 1 || len(snapshot.Clusters) != 3 || len(snapshot.Nodes) != 3 {
		t.Fatalf("batchCalls=%d snapshot=%#v", batchCalls, snapshot)
	}
	first := snapshot.Nodes[0]
	if first.Name != "fixture-node-shared" || first.Cluster != "fixture-cluster-a" || first.Address != "fixture-endpoint-a" || first.Version != "fixture-version-a" {
		t.Fatalf("first identity = %#v", first)
	}
	if first.ID != rabbitmq.StableNodeID("fixture-permanent-a", first.Name) || snapshot.Clusters[0].ID != rabbitmq.StableClusterID("fixture-permanent-a") {
		t.Fatal("permanent cluster identity was not used only for stable hashes")
	}
	if snapshot.Nodes[1].ID != rabbitmq.StableNodeID("fixture-cluster-b", snapshot.Nodes[1].Name) || snapshot.Clusters[1].ID != rabbitmq.StableClusterID("fixture-cluster-b") {
		t.Fatal("logical cluster identity fallback was not used")
	}
	if snapshot.Nodes[2].ID != rabbitmq.StableNodeID("fixture-collection-c", snapshot.Nodes[2].Name) || snapshot.Clusters[2].ID != rabbitmq.StableClusterID("fixture-collection-c") {
		t.Fatal("collection cluster identity fallback was not used")
	}
	if first.UptimeSeconds == nil || *first.UptimeSeconds != 1234 || first.Connections == nil || *first.Connections != 7 || first.PublishRate == nil || *first.PublishRate != 1.5 {
		t.Fatalf("mapped values = %#v", first)
	}
	if !first.CollectionTracked || first.ReportedAt.Unix() != 1785200000 {
		t.Fatalf("collection state = %#v", first)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-permanent", "fixture-ident", "rabbitmq_identity_info", "rabbitmq_cluster_permanent_id"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("snapshot exposed %q", forbidden)
		}
	}
}

func TestRabbitMQInventoryUsesOriginalSampleTimeAndDropsOnlyConflictingNode(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}

	oldKey := rabbitMQLabels("a")
	newKey := cloneLabels(oldKey)
	newKey["cluster"] = "fixture-collection-new"
	newKey["ident"] = "fixture-ident-new"
	newKey["instance"] = "fixture-endpoint-new"
	groups[rabbitMQInventoryQuery] = []instantSeries{
		rabbitMQSeries(oldKey, 100, "80"),
		rabbitMQSeries(newKey, 100, "90"),
	}
	groups[rabbitMQConnectionsQuery] = []instantSeries{
		rabbitMQSeries(oldKey, 101, "11"),
		rabbitMQSeries(newKey, 101, "22"),
	}
	groups[rabbitMQCollectionQuery] = []instantSeries{rabbitMQSeries(newKey, 101, "95")}

	conflictA := rabbitMQLabels("c")
	conflictA["rabbitmq_cluster_permanent_id"] = "fixture-permanent-c"
	conflictB := cloneLabels(conflictA)
	conflictB["cluster"] = "fixture-collection-conflict"
	conflictB["ident"] = "fixture-ident-conflict"
	conflictB["instance"] = "fixture-endpoint-conflict"
	groups[rabbitMQInventoryQuery] = append(groups[rabbitMQInventoryQuery],
		rabbitMQSeries(conflictA, 100, "90"),
		rabbitMQSeries(conflictB, 100, "90"),
	)

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("node-local inventory conflict made snapshot unavailable: %v", err)
	}
	if len(snapshot.Nodes) != 1 || len(snapshot.Clusters) != 1 {
		t.Fatalf("inventory selection counts = nodes %d clusters %d", len(snapshot.Nodes), len(snapshot.Clusters))
	}
	node := snapshot.Nodes[0]
	if node.Address != "fixture-endpoint-new" || node.Connections == nil || *node.Connections != 22 {
		t.Fatalf("newest inventory key was not selected: address=%q connections=%v", node.Address, node.Connections)
	}
	if node.ID != rabbitmq.StableNodeID("fixture-permanent-a", "fixture-node-shared") {
		t.Fatal("latest inventory changed the stable node identity")
	}
}

func TestRabbitMQUnreachablePeersAggregatesPerInventoryNode(t *testing.T) {
	tests := []struct {
		name   string
		series func(map[string]string, map[string]string) []instantSeries
		want   *int64
	}{
		{
			name: "one positive among explicit zero",
			series: func(first, second map[string]string) []instantSeries {
				return []instantSeries{rabbitMQSeries(first, 101, "0"), rabbitMQSeries(second, 101, "1")}
			},
			want: rabbitMQInt64Pointer(1),
		},
		{
			name: "all inventory nodes explicit zero",
			series: func(first, second map[string]string) []instantSeries {
				return []instantSeries{rabbitMQSeries(first, 101, "0"), rabbitMQSeries(second, 101, "0")}
			},
			want: rabbitMQInt64Pointer(0),
		},
		{
			name: "multiple positive nodes use maximum",
			series: func(first, second map[string]string) []instantSeries {
				return []instantSeries{rabbitMQSeries(first, 101, "1"), rabbitMQSeries(second, 101, "3")}
			},
			want: rabbitMQInt64Pointer(3),
		},
		{
			name: "missing node keeps cluster unknown",
			series: func(first, _ map[string]string) []instantSeries {
				return []instantSeries{rabbitMQSeries(first, 101, "0")}
			},
			want: nil,
		},
		{
			name: "zero with another node conflict keeps cluster unknown",
			series: func(first, second map[string]string) []instantSeries {
				return []instantSeries{
					rabbitMQSeries(first, 101, "0"),
					rabbitMQSeries(second, 101, "0"),
					rabbitMQSeries(second, 101, "1"),
				}
			},
			want: nil,
		},
		{
			name: "positive wins over another node conflict",
			series: func(first, second map[string]string) []instantSeries {
				return []instantSeries{
					rabbitMQSeries(first, 101, "2"),
					rabbitMQSeries(second, 101, "0"),
					rabbitMQSeries(second, 101, "1"),
				}
			},
			want: rabbitMQInt64Pointer(2),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := make([][]instantSeries, rabbitMQQueryCount)
			for index := range groups {
				groups[index] = []instantSeries{}
			}
			first := rabbitMQLabels("a")
			second := cloneLabels(first)
			second["ident"] = "fixture-ident-peer"
			second["instance"] = "fixture-endpoint-peer"
			second["rabbitmq_node"] = "fixture-node-peer"
			groups[rabbitMQInventoryQuery] = []instantSeries{
				rabbitMQSeries(first, 100, "90"),
				rabbitMQSeries(second, 100, "90"),
			}
			groups[rabbitMQUnreachablePeersQuery] = test.series(first, second)

			snapshot, err := buildRabbitMQSnapshot(groups)
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 2 {
				t.Fatalf("snapshot counts = clusters %d nodes %d", len(snapshot.Clusters), len(snapshot.Nodes))
			}
			got := snapshot.Clusters[0].UnreachablePeers
			if test.want == nil {
				if got != nil {
					t.Fatalf("unreachable peers = %d, want unknown", *got)
				}
			} else if got == nil || *got != *test.want {
				t.Fatalf("unreachable peers = %v, want %d", got, *test.want)
			}
		})
	}
}

func TestRabbitMQInventorySameJoinKeyPreservesDistinctNodes(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}
	conflictA := rabbitMQLabels("a")
	conflictB := cloneLabels(conflictA)
	conflictB["rabbitmq_node"] = "fixture-node-conflicting-identity"
	unaffected := rabbitMQLabels("b")
	groups[rabbitMQInventoryQuery] = []instantSeries{
		rabbitMQSeries(conflictA, 100, "90"),
		rabbitMQSeries(conflictB, 100, "90"),
		rabbitMQSeries(unaffected, 100, "90"),
	}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("join-key identity conflict made snapshot unavailable: %v", err)
	}
	if len(snapshot.Nodes) != 3 || len(snapshot.Clusters) != 2 {
		t.Fatalf("distinct nodes sharing a collection key were not preserved: nodes=%d clusters=%d", len(snapshot.Nodes), len(snapshot.Clusters))
	}
}

func TestRabbitMQInventoryPreservesMultipleNodesBehindOneCollectionTarget(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}

	base := rabbitMQLabels("shared-target")
	base["rabbitmq_cluster_permanent_id"] = "fixture-permanent-shared"
	nodeA := cloneLabels(base)
	nodeA["rabbitmq_node"] = "fixture-node-a"
	nodeB := cloneLabels(base)
	nodeB["rabbitmq_node"] = "fixture-node-b"
	nodeC := cloneLabels(base)
	nodeC["rabbitmq_node"] = "fixture-node-c"

	groups[rabbitMQInventoryQuery] = []instantSeries{
		rabbitMQSeries(nodeA, 100, "90"),
		rabbitMQSeries(nodeB, 100, "91"),
		rabbitMQSeries(nodeC, 100, "92"),
	}
	groups[rabbitMQConnectionsQuery] = []instantSeries{
		rabbitMQSeries(nodeA, 101, "11"),
		rabbitMQSeries(nodeB, 101, "22"),
		rabbitMQSeries(nodeC, 101, "33"),
	}
	groups[rabbitMQCollectionQuery] = []instantSeries{
		rabbitMQSeries(nodeA, 101, "95"),
		rabbitMQSeries(nodeB, 101, "96"),
		rabbitMQSeries(nodeC, 101, "97"),
	}
	ambiguousMemory := cloneLabels(base)
	delete(ambiguousMemory, "rabbitmq_node")
	groups[rabbitMQMemoryUsedQuery] = []instantSeries{rabbitMQSeries(ambiguousMemory, 101, "999")}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("shared collection target snapshot error: %v", err)
	}
	if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 3 {
		t.Fatalf("shared collection target counts = clusters %d nodes %d", len(snapshot.Clusters), len(snapshot.Nodes))
	}
	wantConnections := map[string]int64{
		"fixture-node-a": 11,
		"fixture-node-b": 22,
		"fixture-node-c": 33,
	}
	for _, node := range snapshot.Nodes {
		want, exists := wantConnections[node.Name]
		if !exists || node.Connections == nil || *node.Connections != want {
			t.Fatalf("node %q connections = %v", node.Name, node.Connections)
		}
		if node.MemoryUsedBytes != nil {
			t.Fatalf("ambiguous label-less memory was assigned to node %q", node.Name)
		}
		if !node.CollectionTracked {
			t.Fatalf("node %q collection progress was not joined", node.Name)
		}
	}
}

func TestRabbitMQConnectionsDiscoverNodesMissingFromIdentityInventory(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}

	identity := rabbitMQLabels("a")
	groups[rabbitMQInventoryQuery] = []instantSeries{rabbitMQSeries(identity, 100, "90")}
	for index, suffix := range []string{"a", "b", "c"} {
		labels := rabbitMQLabels(suffix)
		labels["cluster"] = identity["cluster"]
		delete(labels, "ident")
		delete(labels, "rabbitmq_node")
		delete(labels, "rabbitmq_cluster")
		delete(labels, "rabbitmq_cluster_permanent_id")
		groups[rabbitMQConnectionsQuery] = append(
			groups[rabbitMQConnectionsQuery],
			rabbitMQSeries(labels, 101, strconv.Itoa(index+1)),
		)
	}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("connections discovery snapshot error: %v", err)
	}
	if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 3 {
		t.Fatalf("connections discovery counts = clusters %d nodes %d", len(snapshot.Clusters), len(snapshot.Nodes))
	}
	missingNames := 0
	for _, node := range snapshot.Nodes {
		if node.Address == "" || node.Connections == nil {
			t.Fatalf("discovered node is incomplete: %#v", node)
		}
		if node.Name == node.Address {
			t.Fatalf("discovered node used its address as its name: %#v", node)
		}
		if node.Name == "" {
			missingNames++
		}
	}
	if missingNames != 2 {
		t.Fatalf("missing discovered node names = %d", missingNames)
	}
}

func TestRabbitMQConnectionDiscoveryUsesUniqueNodeLabelHints(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}

	wantNames := map[string]bool{
		"rabbit@fixture-host-a": false,
		"rabbit@fixture-host-b": false,
		"rabbit@fixture-host-c": false,
	}
	for index, suffix := range []string{"a", "b", "c"} {
		labels := map[string]string{
			"cluster":  "fixture-collection-shared",
			"ident":    "fixture-collector-" + suffix,
			"instance": "fixture-endpoint-" + suffix,
		}
		groups[rabbitMQConnectionsQuery] = append(
			groups[rabbitMQConnectionsQuery],
			rabbitMQSeries(labels, 101, strconv.Itoa(index+1)),
		)
		hint := cloneLabels(labels)
		hint["rabbitmq_node"] = "rabbit@fixture-host-" + suffix
		hint["rabbitmq_version"] = "fixture-version"
		groups[rabbitMQBuildInfoQuery] = append(
			groups[rabbitMQBuildInfoQuery],
			rabbitMQSeries(hint, 101, "1"),
		)
	}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("connection name discovery snapshot error: %v", err)
	}
	if len(snapshot.Nodes) != len(wantNames) {
		t.Fatalf("connection name discovery nodes = %d", len(snapshot.Nodes))
	}
	for _, node := range snapshot.Nodes {
		if _, exists := wantNames[node.Name]; !exists {
			t.Fatalf("unexpected discovered node name %q", node.Name)
		}
		wantNames[node.Name] = true
	}
	for name, found := range wantNames {
		if !found {
			t.Fatalf("missing discovered node name %q", name)
		}
	}
}

func TestRabbitMQCurrentIdentityEnrichesHistoricalInventory(t *testing.T) {
	groups := make([][]instantSeries, rabbitMQQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}

	wantNames := map[string]bool{
		"rabbit@fixture-host-a": false,
		"rabbit@fixture-host-b": false,
		"rabbit@fixture-host-c": false,
	}
	for index, suffix := range []string{"a", "b", "c"} {
		identity := map[string]string{
			"cluster":                       "fixture-collection-shared",
			"ident":                         "fixture-collector-" + suffix,
			"instance":                      "fixture-endpoint-" + suffix,
			"rabbitmq_node":                 "rabbit@fixture-host-" + suffix,
			"rabbitmq_cluster":              "fixture-cluster-shared",
			"rabbitmq_cluster_permanent_id": "fixture-permanent-shared",
		}
		groups[rabbitMQIdentityQuery] = append(
			groups[rabbitMQIdentityQuery],
			rabbitMQSeries(identity, 101, "1"),
		)
		connection := cloneLabels(identity)
		delete(connection, "rabbitmq_node")
		delete(connection, "rabbitmq_cluster")
		delete(connection, "rabbitmq_cluster_permanent_id")
		groups[rabbitMQConnectionsQuery] = append(
			groups[rabbitMQConnectionsQuery],
			rabbitMQSeries(connection, 101, strconv.Itoa(index+1)),
		)
		if suffix == "b" {
			groups[rabbitMQInventoryQuery] = []instantSeries{rabbitMQSeries(identity, 100, "90")}
		}
	}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatalf("current identity enrichment snapshot error: %v", err)
	}
	if len(snapshot.Nodes) != len(wantNames) {
		t.Fatalf("current identity enrichment nodes = %d", len(snapshot.Nodes))
	}
	for _, node := range snapshot.Nodes {
		if _, exists := wantNames[node.Name]; !exists {
			t.Fatalf("unexpected enriched node name %q", node.Name)
		}
		wantNames[node.Name] = true
	}
}

func rabbitMQInt64Pointer(value int64) *int64 { return &value }

func TestBuildRabbitMQSnapshotFallbacksConflictsAndNumericValidation(t *testing.T) {
	groups := rabbitMQGroups()
	groups[rabbitMQUptimeQuery] = []instantSeries{
		rabbitMQSeries(rabbitMQLabels("a"), 100, "1.234e3"),
		rabbitMQSeries(rabbitMQLabels("a"), 101, "9.99e2"),
	}
	groups[rabbitMQMemoryUsedQuery] = []instantSeries{
		rabbitMQSeries(rabbitMQLabels("a"), 100, "10"),
		rabbitMQSeries(rabbitMQLabels("a"), 101, "20"),
		rabbitMQSeries(rabbitMQLabels("a"), 101, "21"),
	}
	groups[rabbitMQConnectionsQuery] = []instantSeries{
		rabbitMQSeries(rabbitMQLabels("a"), 100, "9223372036854775807"),
		rabbitMQSeries(rabbitMQLabels("b"), 100, "-1"),
		rabbitMQSeries(rabbitMQLabels("c"), 100, "9223372036854775808"),
	}
	groups[rabbitMQPublishRateQuery] = []instantSeries{
		rabbitMQSeries(rabbitMQLabels("a"), 100, "NaN"),
		rabbitMQSeries(rabbitMQLabels("b"), 100, "Inf"),
		rabbitMQSeries(rabbitMQLabels("c"), 100, "-1"),
	}
	groups[rabbitMQMemoryAlarmQuery] = []instantSeries{rabbitMQSeries(rabbitMQLabels("a"), 100, "2")}

	snapshot, err := buildRabbitMQSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 3 || snapshot.Nodes[0].Name != "fixture-node-shared" || snapshot.Nodes[1].Name != "fixture-node-shared" {
		t.Fatalf("cross-cluster nodes = %#v", snapshot.Nodes)
	}
	if snapshot.Nodes[0].MemoryUsedBytes != nil || snapshot.Nodes[0].MemoryAlarm != nil {
		t.Fatal("same-time conflict or invalid alarm did not remain unknown")
	}
	if snapshot.Nodes[0].Connections == nil || *snapshot.Nodes[0].Connections != int64(^uint64(0)>>1) {
		t.Fatalf("exact int64 was not retained: %#v", snapshot.Nodes[0].Connections)
	}
	if snapshot.Nodes[1].Connections != nil || snapshot.Nodes[2].Connections != nil || snapshot.Nodes[0].PublishRate != nil || snapshot.Nodes[1].PublishRate != nil || snapshot.Nodes[2].PublishRate != nil {
		t.Fatal("negative/non-finite/overflow values were retained")
	}
	if snapshot.Nodes[0].UptimeSeconds == nil || *snapshot.Nodes[0].UptimeSeconds != 999 {
		t.Fatalf("newest scientific uptime was not floored: %#v", snapshot.Nodes[0].UptimeSeconds)
	}
}

func TestRabbitMQParsersRejectUnsafeValues(t *testing.T) {
	for _, raw := range []string{"-1", "NaN", "+Inf", "9223372036854775808", "1.5"} {
		if value, ok := rabbitMQNonNegativeInt(json.RawMessage(raw)); ok || value != nil {
			t.Fatalf("integer accepted %q", raw)
		}
	}
	for _, raw := range []string{"-1", `"NaN"`, `"Inf"`, "1e309"} {
		if value, ok := rabbitMQFiniteNonNegative(json.RawMessage(raw)); ok || value != nil {
			t.Fatalf("float accepted %q", raw)
		}
	}
	for raw, want := range map[string]int64{"1.9": 1, "1.234e3": 1234} {
		value, ok := rabbitMQUptimeSeconds(json.RawMessage(raw))
		if !ok || value == nil || *value != want {
			t.Fatalf("uptime %q = %#v,%v", raw, value, ok)
		}
	}
}

func TestRabbitMQSnapshotMapsUnsafeFailuresToSafeError(t *testing.T) {
	const secret = "fixture-rabbitmq-secret"
	tests := []struct {
		name    string
		handler func(http.ResponseWriter)
	}{
		{name: "unauthorized", handler: func(w http.ResponseWriter) { w.WriteHeader(http.StatusUnauthorized); _, _ = io.WriteString(w, secret) }},
		{name: "forbidden", handler: func(w http.ResponseWriter) { w.WriteHeader(http.StatusForbidden); _, _ = io.WriteString(w, secret) }},
		{name: "redirect", handler: func(w http.ResponseWriter) { w.Header().Set("Location", "/"+secret); w.WriteHeader(http.StatusFound) }},
		{name: "non-json", handler: func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, secret)
		}},
		{name: "null data", handler: func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":null,"err":""}`)
		}},
		{name: "error envelope", handler: func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":[],"err":"`+secret+`"}`)
		}},
		{name: "oversized", handler: func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":"`+strings.Repeat("x", 257)+`","err":""}`)
		}},
		{name: "group mismatch", handler: func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":[],"err":""}`)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/n9e/datasource/brief" {
					writeFixture(t, w, "datasource-brief.json")
					return
				}
				test.handler(w)
			}))
			defer server.Close()
			maxBytes := int64(0)
			if test.name == "oversized" {
				maxBytes = 256
			}
			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: secret, HTTPClient: server.Client(), Clock: fixedClock, MaxResponseBytes: maxBytes})
			_, err := provider.RabbitMQSnapshot(context.Background())
			if !errors.Is(err, rabbitmq.ErrUnavailable) {
				t.Fatalf("error = %v", err)
			}
			for _, forbidden := range []string{secret, server.URL, "rabbitmq_identity_info", "fixture-ident", "fixture-label"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("error exposed %q: %v", forbidden, err)
				}
			}
		})
	}
}

func TestRabbitMQSnapshotTimeoutIsSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/n9e/datasource/brief" {
			t.Fatal("timeout test sent an unexpected request to the fixture server")
		}
		writeFixture(t, w, "datasource-brief.json")
	}))
	defer server.Close()
	baseTransport := server.Client().Transport
	client := &http.Client{Transport: diskRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/n9e/query-instant-batch" {
			return baseTransport.RoundTrip(request)
		}
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: client, Clock: fixedClock})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := provider.RabbitMQSnapshot(ctx)
	if !errors.Is(err, rabbitmq.ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func rabbitMQGroups() [][]instantSeries {
	groups := make([][]instantSeries, 22)
	for index := range groups {
		groups[index] = []instantSeries{}
	}
	for _, suffix := range []string{"a", "b", "c"} {
		groups[20] = append(groups[20], rabbitMQSeries(rabbitMQLabels(suffix), 100, "90"))
		groups[21] = append(groups[21], rabbitMQSeries(rabbitMQLabels(suffix), 100, "95"))
	}
	missing := rabbitMQLabels("ghost")
	delete(missing, "rabbitmq_node")
	groups[20] = append(groups[20], rabbitMQSeries(missing, 100, "90"))
	return groups
}

func rabbitMQLabels(suffix string) map[string]string {
	labels := map[string]string{
		"cluster": "fixture-collection-" + suffix, "ident": "fixture-ident-" + suffix,
		"instance": "fixture-endpoint-" + suffix, "rabbitmq_node": "fixture-node-" + suffix,
		"rabbitmq_cluster": "fixture-cluster-" + suffix,
	}
	switch suffix {
	case "a":
		labels["rabbitmq_node"] = "fixture-node-shared"
		labels["rabbitmq_cluster_permanent_id"] = "fixture-permanent-a"
	case "b":
		labels["rabbitmq_node"] = "fixture-node-shared"
	case "c":
		delete(labels, "rabbitmq_cluster")
	}
	return labels
}

func rabbitMQSeries(labels map[string]string, timestamp int64, value string) instantSeries {
	return instantSeries{Metric: labels, Value: rawInstantValue(timestamp, value)}
}
