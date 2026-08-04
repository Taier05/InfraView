package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/elasticsearch"
	"github.com/Taier05/InfraView/internal/elasticsearch/elasticsearchtest"
)

func TestElasticsearchPromQLIsFixedAndReturnsACopy(t *testing.T) {
	want := []string{
		`elasticsearch_clusterinfo_up`,
		`elasticsearch_node_stats_up`,
		`elasticsearch_cluster_health_status`,
		`elasticsearch_cluster_health_number_of_nodes`,
		`elasticsearch_cluster_health_number_of_data_nodes`,
		`elasticsearch_cluster_health_active_primary_shards`,
		`elasticsearch_cluster_health_active_shards`,
		`elasticsearch_cluster_health_relocating_shards`,
		`elasticsearch_cluster_health_initializing_shards`,
		`elasticsearch_cluster_health_unassigned_shards`,
		`elasticsearch_cluster_health_number_of_pending_tasks`,
		`elasticsearch_cluster_health_task_max_waiting_in_queue_millis`,
		`elasticsearch_nodes_roles`,
		`elasticsearch_jvm_memory_used_bytes{area="heap"}`,
		`elasticsearch_jvm_memory_max_bytes{area="heap"}`,
		`max by (cluster, name, host, ident, instance, es_client_node, es_data_node, es_ingest_node, es_master_node) (100 * (1 - elasticsearch_filesystem_data_available_bytes / elasticsearch_filesystem_data_size_bytes))`,
		`elasticsearch_process_cpu_percent`,
		`rate(elasticsearch_indices_indexing_index_total[5m])`,
		`rate(elasticsearch_indices_search_query_total[5m])`,
		`elasticsearch_indices_docs`,
		`elasticsearch_indices_store_size_bytes`,
		`elasticsearch_jvm_uptime_seconds`,
		`max by (cluster, name, host, ident, instance) (elasticsearch_thread_pool_queue_count)`,
		`sum by (cluster, name, host, ident, instance) (rate(elasticsearch_thread_pool_rejected_count[5m]))`,
		`tlast_over_time(elasticsearch_clusterinfo_up[24h])`,
		`tlast_over_time(elasticsearch_jvm_uptime_seconds[24h])`,
	}
	got := elasticsearchPromQL()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("elasticsearchPromQL() = %#v", got)
	}
	got[0] = "changed"
	if elasticsearchPromQL()[0] != want[0] {
		t.Fatal("fixed Elasticsearch queries were mutated")
	}
}

func TestElasticsearchSnapshotUsesOneFixedBatchAndMapsSafeFields(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		paths = append(paths, request.URL.Path)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			assertElasticsearchBatchRequest(t, request)
			writeFixture(t, w, "elasticsearch-instant-batch.json")
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock,
	})
	snapshot, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ElasticsearchSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(paths, []string{"/api/n9e/datasource/brief", "/api/n9e/query-instant-batch"}) {
		t.Fatalf("request paths = %#v", paths)
	}
	if len(snapshot.Clusters) != 2 || len(snapshot.Nodes) != 3 {
		t.Fatalf("snapshot cardinality = %#v", snapshot)
	}
	cluster := snapshot.Clusters[0]
	if cluster.ID != elasticsearch.StableClusterID("fixture-cluster-a") || cluster.Name != "fixture-cluster-a" ||
		cluster.Availability != elasticsearch.AvailabilityUp || cluster.NodeStatsAvailability != elasticsearch.AvailabilityUp ||
		cluster.Health != elasticsearch.HealthGreen || !cluster.CollectionTracked ||
		!cluster.ReportedAt.Equal(time.Unix(1785200000, 0).UTC()) {
		t.Fatalf("first cluster = %#v", cluster)
	}
	if cluster.NumberOfNodes == nil || *cluster.NumberOfNodes != 2 ||
		cluster.NumberOfDataNodes == nil || *cluster.NumberOfDataNodes != 1 ||
		cluster.ActiveShards == nil || *cluster.ActiveShards != 8 {
		t.Fatalf("first cluster counters = %#v", cluster)
	}
	node := snapshot.Nodes[0]
	if node.ID != elasticsearch.StableNodeID("fixture-cluster-a", "fixture-node-a") ||
		node.Cluster != "fixture-cluster-a" || node.Name != "fixture-node-a" || node.Address != "192.0.2.10" ||
		!reflect.DeepEqual(node.Roles, []elasticsearch.Role{elasticsearch.RoleMaster, elasticsearch.RoleDataHot, elasticsearch.RoleIngest}) ||
		!node.DataNode || !node.CollectionTracked || !node.ReportedAt.Equal(time.Unix(1785200001, 0).UTC()) {
		t.Fatalf("first node = %#v", node)
	}
	if node.HeapUsedBytes == nil || *node.HeapUsedBytes != 40 || node.HeapMaxBytes == nil || *node.HeapMaxBytes != 100 ||
		node.Documents == nil || *node.Documents != 12 || node.StoreSizeBytes == nil || *node.StoreSizeBytes != 80 {
		t.Fatalf("first node metrics = %#v", node)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-ident", "instance", "cluster_uuid", "elasticsearch_", "url"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("snapshot exposes %q", forbidden)
		}
	}
}

func TestElasticsearchProviderContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			writeFixture(t, w, "elasticsearch-instant-batch.json")
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()
	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	elasticsearchtest.RunContract(t, provider)
}

func TestBuildElasticsearchSnapshotRejectsUnsafeInventoryShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([][]instantSeries) [][]instantSeries
	}{
		{name: "wrong group count", mutate: func(groups [][]instantSeries) [][]instantSeries { return groups[:len(groups)-1] }},
		{name: "nil cluster inventory", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[elasticsearchClusterInventoryQuery] = nil
			return groups
		}},
		{name: "nil node inventory", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[elasticsearchNodeInventoryQuery] = nil
			return groups
		}},
		{name: "invalid inventory sample timestamp", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[elasticsearchClusterInventoryQuery][0].Value[0] = json.RawMessage(`"invalid"`)
			return groups
		}},
		{name: "invalid inventory reported value", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[elasticsearchNodeInventoryQuery][0].Value[1] = json.RawMessage(`"invalid"`)
			return groups
		}},
		{name: "node reported time conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := groups[elasticsearchNodeInventoryQuery][0]
			conflict.Metric = cloneLabels(conflict.Metric)
			conflict.Metric["ident"] = "fixture-ident-b"
			conflict.Value = rawInstantValue(1785200200, "1785200002")
			groups[elasticsearchNodeInventoryQuery] = append(groups[elasticsearchNodeInventoryQuery], conflict)
			return groups
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildElasticsearchSnapshot(test.mutate(elasticsearchGroups()))
			if !errors.Is(err, elasticsearch.ErrUnavailable) {
				t.Fatalf("error = %v, want Elasticsearch ErrUnavailable", err)
			}
		})
	}
}

func TestBuildElasticsearchSnapshotIgnoresInventoryWithoutDomainIdentity(t *testing.T) {
	groups := elasticsearchGroups()
	clusterWithoutIdentity := elasticsearchSeries(
		map[string]string{"ident": "fixture-ident-missing-cluster"},
		1785200200,
		"1785200002",
	)
	nodeWithoutIdentityLabels := elasticsearchNodeLabels("192.0.2.20", "")
	delete(nodeWithoutIdentityLabels, "name")
	nodeWithoutIdentity := elasticsearchSeries(nodeWithoutIdentityLabels, 1785200200, "1785200003")
	groups[elasticsearchClusterInventoryQuery] = append(groups[elasticsearchClusterInventoryQuery], clusterWithoutIdentity)
	groups[elasticsearchNodeInventoryQuery] = append(groups[elasticsearchNodeInventoryQuery], nodeWithoutIdentity)

	snapshot, err := buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 1 {
		t.Fatalf("inventory cardinality = %d/%d, want 1/1", len(snapshot.Clusters), len(snapshot.Nodes))
	}
}

func TestBuildElasticsearchSnapshotMergesNodeAddressByInventoryTime(t *testing.T) {
	withoutHost := elasticsearchNodeLabels("", "")
	delete(withoutHost, "host")
	older := elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200100, "1785200000")
	newer := elasticsearchSeries(elasticsearchNodeLabels("192.0.2.20", ""), 1785200200, "1785200001")
	conflict := elasticsearchSeries(elasticsearchNodeLabels("198.51.100.20", ""), 1785200200, "1785200001")
	tests := []struct {
		name        string
		inventory   []instantSeries
		wantAddress string
		wantReport  int64
	}{
		{
			name: "missing host remains unknown",
			inventory: []instantSeries{
				elasticsearchSeries(withoutHost, 1785200200, "1785200001"),
			},
			wantAddress: "",
			wantReport:  1785200001,
		},
		{
			name:        "newer address wins old then new",
			inventory:   []instantSeries{older, newer},
			wantAddress: "192.0.2.20",
			wantReport:  1785200001,
		},
		{
			name:        "newer address wins new then old",
			inventory:   []instantSeries{newer, older},
			wantAddress: "192.0.2.20",
			wantReport:  1785200001,
		},
		{
			name:        "same latest time different addresses stay unknown",
			inventory:   []instantSeries{newer, conflict},
			wantAddress: "",
			wantReport:  1785200001,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := elasticsearchGroups()
			groups[elasticsearchNodeInventoryQuery] = test.inventory
			snapshot, err := buildElasticsearchSnapshot(groups)
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Nodes) != 1 {
				t.Fatalf("node count = %d, want 1", len(snapshot.Nodes))
			}
			node := snapshot.Nodes[0]
			if node.Address != test.wantAddress || node.ReportedAt.Unix() != test.wantReport {
				t.Fatalf("address/reported = %q/%d, want %q/%d", node.Address, node.ReportedAt.Unix(), test.wantAddress, test.wantReport)
			}
		})
	}
}

func TestBuildElasticsearchSnapshotNormalizesInvalidOptionalValuesAndConflicts(t *testing.T) {
	groups := elasticsearchGroups()
	groups[elasticsearchClusterInfoUpQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchClusterLabels(), 1785200100, "2"),
	}
	groups[elasticsearchClusterHealthQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchClusterLabels(), 1785200100, "4"),
	}
	groups[elasticsearchNodeRolesQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", "unsupported"), 1785200100, "1"),
	}
	groups[elasticsearchHeapUsedQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200100, "invalid"),
	}
	groups[elasticsearchCPUUsageQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200100, "20"),
		elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200100, "21"),
	}

	snapshot, err := buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Clusters[0].Availability != elasticsearch.AvailabilityUnknown || snapshot.Clusters[0].Health != elasticsearch.HealthUnknown {
		t.Fatalf("invalid cluster states = %#v", snapshot.Clusters[0])
	}
	node := snapshot.Nodes[0]
	if len(node.Roles) != 0 || node.DataNode || node.HeapUsedBytes != nil || node.CPUUsagePercent != nil {
		t.Fatalf("invalid/conflicting node values survived = %#v", node)
	}
}

func TestBuildElasticsearchSnapshotParsesPrometheusUptimeSeconds(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  *int64
	}{
		{name: "scientific notation", value: "9e4", want: int64Pointer(90000)},
		{name: "fractional seconds floor", value: "90000.75", want: int64Pointer(90000)},
		{name: "subsecond floors to zero", value: "0.75", want: int64Pointer(0)},
		{name: "negative", value: "-1"},
		{name: "nan", value: "NaN"},
		{name: "positive infinity", value: "+Inf"},
		{name: "int64 overflow", value: "9.223372036854776e+18"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := elasticsearchGroups()
			groups[elasticsearchUptimeQuery] = []instantSeries{
				elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200100, test.value),
			}
			snapshot, err := buildElasticsearchSnapshot(groups)
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Nodes[0].UptimeSeconds
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("uptime = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestBuildElasticsearchSnapshotUsesValidHealthColorAndRejectsConflicts(t *testing.T) {
	tests := []struct {
		name   string
		series []instantSeries
		want   elasticsearch.Health
	}{
		{
			name: "yellow color overrides legacy value",
			series: []instantSeries{
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "yellow"}, 1785200100, "1"),
			},
			want: elasticsearch.HealthYellow,
		},
		{
			name: "red color overrides legacy value",
			series: []instantSeries{
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "red"}, 1785200100, "1"),
			},
			want: elasticsearch.HealthRed,
		},
		{
			name: "invalid color stays unknown",
			series: []instantSeries{
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "invalid"}, 1785200100, "1"),
			},
			want: elasticsearch.HealthUnknown,
		},
		{
			name: "inactive valid color stays unknown",
			series: []instantSeries{
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "red"}, 1785200100, "0"),
			},
			want: elasticsearch.HealthUnknown,
		},
		{
			name: "same latest time different colors conflict",
			series: []instantSeries{
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "green"}, 1785200100, "1"),
				elasticsearchSeries(map[string]string{"cluster": "fixture-cluster-a", "color": "yellow"}, 1785200100, "1"),
			},
			want: elasticsearch.HealthUnknown,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := elasticsearchGroups()
			groups[elasticsearchClusterHealthQuery] = test.series
			snapshot, err := buildElasticsearchSnapshot(groups)
			if err != nil {
				t.Fatal(err)
			}
			if got := snapshot.Clusters[0].Health; got != test.want {
				t.Fatalf("health = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildElasticsearchSnapshotDeduplicatesCollectorsAndIgnoresAuxiliaryIdentities(t *testing.T) {
	groups := elasticsearchGroups()
	duplicateCluster := groups[elasticsearchClusterInventoryQuery][0]
	duplicateCluster.Metric = cloneLabels(duplicateCluster.Metric)
	duplicateCluster.Metric["ident"] = "fixture-ident-b"
	groups[elasticsearchClusterInventoryQuery] = append(groups[elasticsearchClusterInventoryQuery], duplicateCluster)
	duplicateNode := groups[elasticsearchNodeInventoryQuery][0]
	duplicateNode.Metric = cloneLabels(duplicateNode.Metric)
	duplicateNode.Metric["ident"] = "fixture-ident-b"
	groups[elasticsearchNodeInventoryQuery] = append(groups[elasticsearchNodeInventoryQuery], duplicateNode)
	ghostLabels := map[string]string{
		"cluster": "fixture-cluster-ghost", "name": "fixture-node-ghost", "host": "203.0.113.10", "ident": "fixture-ident-ghost",
	}
	groups[elasticsearchCPUUsageQuery] = append(groups[elasticsearchCPUUsageQuery], elasticsearchSeries(ghostLabels, 1785200100, "50"))

	snapshot, err := buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Clusters) != 1 || len(snapshot.Nodes) != 1 {
		t.Fatalf("duplicate collectors or auxiliary identity changed cardinality: %#v", snapshot)
	}
}

func TestBuildElasticsearchSnapshotSumsSemanticSeriesWithoutOverflowOrCollectorDoubleCount(t *testing.T) {
	groups := elasticsearchGroups()
	firstIndex := elasticsearchNodeLabels("192.0.2.10", "")
	firstIndex["index"] = "fixture-node-index-a"
	firstCollectorDuplicate := cloneLabels(firstIndex)
	firstCollectorDuplicate["ident"] = "fixture-ident-b"
	secondIndex := elasticsearchNodeLabels("192.0.2.10", "")
	secondIndex["index"] = "fixture-node-index-b"
	groups[elasticsearchDocumentsQuery] = []instantSeries{
		elasticsearchSeries(firstIndex, 1785200100, "5"),
		elasticsearchSeries(firstCollectorDuplicate, 1785200100, "5"),
		elasticsearchSeries(secondIndex, 1785200100, "7"),
	}
	groups[elasticsearchIndexRateQuery] = []instantSeries{
		elasticsearchSeries(firstIndex, 1785200100, "2.5"),
		elasticsearchSeries(firstCollectorDuplicate, 1785200100, "2.5"),
		elasticsearchSeries(secondIndex, 1785200100, "1.5"),
	}

	snapshot, err := buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Nodes[0].Documents == nil || *snapshot.Nodes[0].Documents != 12 ||
		snapshot.Nodes[0].IndexRate == nil || *snapshot.Nodes[0].IndexRate != 4 {
		t.Fatalf("semantic sums = %#v", snapshot.Nodes[0])
	}

	groups[elasticsearchDocumentsQuery] = []instantSeries{
		elasticsearchSeries(firstIndex, 1785200100, "9223372036854775807"),
		elasticsearchSeries(secondIndex, 1785200100, "1"),
	}
	groups[elasticsearchIndexRateQuery] = []instantSeries{
		elasticsearchSeries(firstIndex, 1785200100, "1.7976931348623157e+308"),
		elasticsearchSeries(secondIndex, 1785200100, "1.7976931348623157e+308"),
	}
	snapshot, err = buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Nodes[0].Documents != nil || snapshot.Nodes[0].IndexRate != nil {
		t.Fatalf("overflowing sums survived = %#v", snapshot.Nodes[0])
	}
}

func TestSumElasticsearchFloatsUsesSortedSemanticKeys(t *testing.T) {
	states := map[string]*elasticsearchLatest[float64]{
		"a": {value: 1e16, present: true},
		"b": {value: 1, present: true},
		"c": {value: 1, present: true},
	}
	const want = float64(1e16)
	for attempt := 0; attempt < 256; attempt++ {
		got := sumElasticsearchFloats(states)
		if got == nil {
			t.Fatalf("attempt %d sum = nil, want stable %v", attempt, want)
		}
		if *got != want {
			t.Fatalf("attempt %d sum = %v, want stable %v", attempt, *got, want)
		}
	}
}

func TestBuildElasticsearchSnapshotIgnoresCollectorAddressWhenDeduplicatingSemanticSeries(t *testing.T) {
	groups := elasticsearchGroups()
	firstCollector := elasticsearchNodeLabels("192.0.2.10", "")
	firstCollector["index"] = "fixture-node-index-a"
	firstCollector["address"] = "192.0.2.20"
	secondCollector := cloneLabels(firstCollector)
	secondCollector["ident"] = "fixture-ident-b"
	secondCollector["address"] = "198.51.100.20"
	groups[elasticsearchDocumentsQuery] = []instantSeries{
		elasticsearchSeries(firstCollector, 1785200100, "5"),
		elasticsearchSeries(secondCollector, 1785200100, "5"),
	}

	snapshot, err := buildElasticsearchSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Nodes[0].Documents == nil {
		t.Fatal("collector address removed semantic documents")
	}
	if got := *snapshot.Nodes[0].Documents; got != 5 {
		t.Fatalf("collector address caused semantic documents = %d, want 5", got)
	}
}

func TestElasticsearchSnapshotWrapsUpstreamFailuresWithoutSensitiveData(t *testing.T) {
	const secret = "fixture-elasticsearch-secret"
	tests := []struct {
		name   string
		status int
		ctype  string
		body   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, ctype: "application/json", body: `{"dat":null,"err":"` + secret + `"}`},
		{name: "non json", status: http.StatusBadGateway, ctype: "text/html", body: `<html>` + secret + `</html>`},
		{name: "null data", status: http.StatusOK, ctype: "application/json", body: `{"dat":null,"err":""}`},
		{name: "envelope error", status: http.StatusOK, ctype: "application/json", body: `{"dat":[],"err":"` + secret + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/n9e/datasource/brief" {
					writeFixture(t, w, "datasource-brief.json")
					return
				}
				w.Header().Set("Content-Type", test.ctype)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: secret, HTTPClient: server.Client(), Clock: fixedClock})
			_, err := provider.ElasticsearchSnapshot(context.Background())
			if !errors.Is(err, elasticsearch.ErrUnavailable) {
				t.Fatalf("error = %v", err)
			}
			for _, forbidden := range []string{secret, server.URL, "elasticsearch_clusterinfo_up", "<html>"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("error exposes %q: %v", forbidden, err)
				}
			}
		})
	}
}

func assertElasticsearchBatchRequest(t *testing.T, request *http.Request) {
	t.Helper()
	var body batchRequest
	decodeRequest(t, request, &body)
	want := elasticsearchPromQL()
	if request.Method != http.MethodPost || body.DatasourceID != 7 || len(body.Queries) != len(want) {
		t.Fatalf("Elasticsearch batch shape = %#v", body)
	}
	for index, query := range body.Queries {
		if query.Query != want[index] || query.Time != fixedClock().Unix() {
			t.Fatalf("Elasticsearch query %d = %#v", index, query)
		}
	}
}

func elasticsearchSeries(labels map[string]string, timestamp int64, value string) instantSeries {
	return instantSeries{Metric: labels, Value: rawInstantValue(timestamp, value)}
}

func elasticsearchClusterLabels() map[string]string {
	return map[string]string{"cluster": "fixture-cluster-a", "ident": "fixture-ident-a"}
}

func elasticsearchNodeLabels(address, role string) map[string]string {
	labels := map[string]string{
		"cluster": "fixture-cluster-a", "name": "fixture-node-a", "host": address, "ident": "fixture-ident-a",
	}
	if role != "" {
		labels["role"] = role
	}
	return labels
}

func elasticsearchGroups() [][]instantSeries {
	groups := make([][]instantSeries, elasticsearchQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}
	groups[elasticsearchClusterInventoryQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchClusterLabels(), 1785200200, "1785200000"),
	}
	groups[elasticsearchNodeInventoryQuery] = []instantSeries{
		elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200200, "1785200001"),
	}
	return groups
}
