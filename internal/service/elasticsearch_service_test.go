package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/elasticsearch"
)

func TestElasticsearchNodeStatusThresholds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*elasticsearch.Node)
		level  Level
		source ElasticsearchNodeStatusSource
	}{
		{"heap below warning", func(n *elasticsearch.Node) {
			n.HeapUsedBytes = elasticsearchInt64Ptr(74)
			n.HeapMaxBytes = elasticsearchInt64Ptr(100)
		}, LevelNormal, ElasticsearchNodeStatusNormal},
		{"heap warning", func(n *elasticsearch.Node) {
			n.HeapUsedBytes = elasticsearchInt64Ptr(75)
			n.HeapMaxBytes = elasticsearchInt64Ptr(100)
		}, LevelWarning, ElasticsearchNodeStatusJVM},
		{"heap critical", func(n *elasticsearch.Node) {
			n.HeapUsedBytes = elasticsearchInt64Ptr(85)
			n.HeapMaxBytes = elasticsearchInt64Ptr(100)
		}, LevelCritical, ElasticsearchNodeStatusJVM},
		{"disk below warning", func(n *elasticsearch.Node) { n.DiskUsagePercent = elasticsearchFloatPtr(84.99) }, LevelNormal, ElasticsearchNodeStatusNormal},
		{"disk warning", func(n *elasticsearch.Node) { n.DiskUsagePercent = elasticsearchFloatPtr(85) }, LevelWarning, ElasticsearchNodeStatusDisk},
		{"disk critical", func(n *elasticsearch.Node) { n.DiskUsagePercent = elasticsearchFloatPtr(90) }, LevelCritical, ElasticsearchNodeStatusDisk},
		{"rejection warning", func(n *elasticsearch.Node) { n.RejectedRate = elasticsearchFloatPtr(0.01) }, LevelWarning, ElasticsearchNodeStatusThreadPool},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := healthyElasticsearchNode("fixture-cluster-a", "fixture-node-a")
			test.mutate(&node)
			got := summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelNormal)
			if got.Status != test.level || got.StatusSource != test.source {
				t.Fatalf("status/source = %q/%q, want %q/%q", got.Status, got.StatusSource, test.level, test.source)
			}
		})
	}
}

func TestElasticsearchServiceMetaUsesLatestClusterOrNodeSampleTime(t *testing.T) {
	now := time.Date(2026, time.August, 7, 8, 0, 0, 0, time.UTC)
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	cluster.ReportedAt = now.Add(-2 * time.Minute)
	node := healthyElasticsearchNode(cluster.Name, "fixture-node-a")
	node.ReportedAt = now.Add(-time.Minute)
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{node}}}
	service := NewElasticsearch(provider, cache.New(func() time.Time { return now }), ElasticsearchOptions{Clock: func() time.Time { return now }})

	_, meta, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if !meta.CollectedAt.Equal(node.ReportedAt) {
		t.Fatalf("CollectedAt = %s, want latest sample %s", meta.CollectedAt, node.ReportedAt)
	}
}

func TestElasticsearchNodeStatusMissingResourcesAndPriority(t *testing.T) {
	node := healthyElasticsearchNode("fixture-cluster-a", "fixture-node-a")
	node.HeapUsedBytes = nil
	got := summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelNormal)
	if got.Status != LevelUnknown || got.StatusSource != ElasticsearchNodeStatusJVM {
		t.Fatalf("missing heap status/source = %q/%q", got.Status, got.StatusSource)
	}

	node = healthyElasticsearchNode("fixture-cluster-a", "fixture-node-a")
	node.DiskUsagePercent = nil
	got = summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelNormal)
	if got.Status != LevelUnknown || got.StatusSource != ElasticsearchNodeStatusDisk {
		t.Fatalf("missing data disk status/source = %q/%q", got.Status, got.StatusSource)
	}

	node.DataNode = false
	got = summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelNormal)
	if got.Status != LevelNormal || got.StatusSource != ElasticsearchNodeStatusNormal {
		t.Fatalf("missing non-data disk status/source = %q/%q", got.Status, got.StatusSource)
	}

	node = healthyElasticsearchNode("fixture-cluster-a", "fixture-node-a")
	node.DiskUsagePercent = elasticsearchFloatPtr(85)
	node.HeapUsedBytes = elasticsearchInt64Ptr(75)
	node.RejectedRate = elasticsearchFloatPtr(1)
	got = summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelWarning)
	if got.Status != LevelWarning || got.StatusSource != ElasticsearchNodeStatusCollection {
		t.Fatalf("equal-level priority status/source = %q/%q", got.Status, got.StatusSource)
	}
	got = summarizeElasticsearchNode(node, elasticsearch.HealthGreen, LevelNormal)
	if got.Status != LevelWarning || got.StatusSource != ElasticsearchNodeStatusDisk {
		t.Fatalf("resource priority status/source = %q/%q", got.Status, got.StatusSource)
	}
}

func TestElasticsearchNodeDisplayMetricsAndClusterHealthNeverChangeNodeStatus(t *testing.T) {
	node := healthyElasticsearchNode("fixture-cluster-a", "fixture-node-a")
	node.CPUUsagePercent = elasticsearchFloatPtr(100)
	node.IndexRate = elasticsearchFloatPtr(math.MaxFloat64)
	node.SearchRate = elasticsearchFloatPtr(math.MaxFloat64)
	node.Documents = elasticsearchInt64Ptr(math.MaxInt64)
	node.StoreSizeBytes = elasticsearchInt64Ptr(math.MaxInt64)
	node.ThreadPoolQueue = elasticsearchInt64Ptr(math.MaxInt64)
	node.UptimeSeconds = elasticsearchInt64Ptr(0)
	for _, health := range []elasticsearch.Health{elasticsearch.HealthGreen, elasticsearch.HealthYellow, elasticsearch.HealthRed, elasticsearch.HealthUnknown} {
		got := summarizeElasticsearchNode(node, health, LevelNormal)
		if got.Status != LevelNormal || got.StatusSource != ElasticsearchNodeStatusNormal || got.ClusterHealth != health {
			t.Fatalf("health %q changed node status: %#v", health, got)
		}
	}
}

func TestElasticsearchClusterStatusBoundariesAndPriority(t *testing.T) {
	tests := []struct {
		name         string
		availability elasticsearch.Availability
		nodeStats    elasticsearch.Availability
		health       elasticsearch.Health
		collection   Level
		level        Level
		source       ElasticsearchClusterStatusSource
	}{
		{"green and up", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUp, elasticsearch.HealthGreen, LevelNormal, LevelNormal, ElasticsearchClusterStatusNormal},
		{"down", elasticsearch.AvailabilityDown, elasticsearch.AvailabilityUp, elasticsearch.HealthGreen, LevelNormal, LevelCritical, ElasticsearchClusterStatusAvailability},
		{"unknown", elasticsearch.AvailabilityUnknown, elasticsearch.AvailabilityUp, elasticsearch.HealthGreen, LevelNormal, LevelUnknown, ElasticsearchClusterStatusAvailability},
		{"yellow", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUp, elasticsearch.HealthYellow, LevelNormal, LevelWarning, ElasticsearchClusterStatusHealth},
		{"red", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUp, elasticsearch.HealthRed, LevelNormal, LevelCritical, ElasticsearchClusterStatusHealth},
		{"unknown health", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUp, elasticsearch.HealthUnknown, LevelNormal, LevelUnknown, ElasticsearchClusterStatusHealth},
		{"node collector down", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityDown, elasticsearch.HealthGreen, LevelNormal, LevelCritical, ElasticsearchClusterStatusCollection},
		{"node collector unknown", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUnknown, elasticsearch.HealthGreen, LevelNormal, LevelUnknown, ElasticsearchClusterStatusCollection},
		{"freshness wins node collector unknown", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUnknown, elasticsearch.HealthGreen, LevelCritical, LevelCritical, ElasticsearchClusterStatusCollection},
		{"availability wins equal critical", elasticsearch.AvailabilityDown, elasticsearch.AvailabilityUp, elasticsearch.HealthRed, LevelCritical, LevelCritical, ElasticsearchClusterStatusAvailability},
		{"health wins equal warning", elasticsearch.AvailabilityUp, elasticsearch.AvailabilityUp, elasticsearch.HealthYellow, LevelWarning, LevelWarning, ElasticsearchClusterStatusHealth},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := healthyElasticsearchCluster("fixture-cluster-a")
			cluster.Availability = test.availability
			cluster.NodeStatsAvailability = test.nodeStats
			cluster.Health = test.health
			got := assessElasticsearchCluster(cluster, test.collection)
			if got.level != test.level || got.source != test.source {
				t.Fatalf("status/source = %q/%q, want %q/%q", got.level, got.source, test.level, test.source)
			}
		})
	}
}

func TestElasticsearchServiceTracksClusterAndNodeFreshnessIndependently(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &elasticsearchTestClock{now: now}
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	node := healthyElasticsearchNode(cluster.Name, "fixture-node-a")
	cluster.ReportedAt = now.Add(-24 * time.Hour)
	node.ReportedAt = now.Add(-24 * time.Hour)
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{node}}}
	service := NewElasticsearch(provider, cache.New(clock.Now), ElasticsearchOptions{SnapshotTTL: time.Minute, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	assertLevels := func(wantCluster, wantNode Level) {
		t.Helper()
		overview, _, err := service.Overview(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if levelFromCounts(overview.Clusters) != wantCluster || levelFromCounts(overview.Nodes) != wantNode {
			t.Fatalf("counts = %#v/%#v, want levels %q/%q", overview.Clusters, overview.Nodes, wantCluster, wantNode)
		}
	}

	assertLevels(LevelNormal, LevelNormal) // Process baseline ignores absolute sample age.
	provider.snapshot.Clusters[0].ReportedAt = now
	provider.snapshot.Nodes[0].ReportedAt = now
	clock.Advance(30 * time.Second)
	assertLevels(LevelWarning, LevelWarning) // Cache hit must not observe provider mutations.
	if provider.calls != 1 {
		t.Fatalf("provider calls on cache hit = %d, want 1", provider.calls)
	}

	provider.snapshot.Nodes[0].ReportedAt = node.ReportedAt
	clock.Advance(46 * time.Second)
	assertLevels(LevelNormal, LevelCritical) // Successful reload advances only cluster progress.
	if provider.calls != 2 {
		t.Fatalf("provider calls after expiry = %d, want 2", provider.calls)
	}
}

func TestElasticsearchServiceKeepsFreshnessBoundToTheReturnedSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &elasticsearchTestClock{now: now}
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	node := healthyElasticsearchNode(cluster.Name, "fixture-node-a")
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{node}}}
	service := NewElasticsearch(provider, cache.New(clock.Now), ElasticsearchOptions{SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	oldState, _, err := service.snapshotState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(30 * time.Second)
	provider.snapshot.Clusters[0].ReportedAt = now
	provider.snapshot.Nodes[0].ReportedAt = now
	if _, _, err = service.snapshotState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := service.clusterCollectionLevel(oldState.snapshot.Clusters[0], oldState.clusterAdvancedAt); got != LevelWarning {
		t.Fatalf("old cluster level = %q, want %q", got, LevelWarning)
	}
	if got := service.nodeCollectionLevel(oldState.snapshot.Nodes[0], oldState.nodeAdvancedAt); got != LevelWarning {
		t.Fatalf("old node level = %q, want %q", got, LevelWarning)
	}
}

func TestElasticsearchServiceFreshnessBoundariesAndTimestampRollback(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &elasticsearchTestClock{now: now}
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	node := healthyElasticsearchNode(cluster.Name, "fixture-node-a")
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{node}}}
	service := NewElasticsearch(provider, cache.New(clock.Now), ElasticsearchOptions{SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Nodes[0].CollectionLevel != LevelNormal {
		t.Fatalf("initial page = %#v, err = %v", page, err)
	}
	clock.Advance(30 * time.Second)
	page, _, err = service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Nodes[0].CollectionLevel != LevelWarning {
		t.Fatalf("2-cycle page = %#v, err = %v", page, err)
	}
	clock.Advance(45 * time.Second)
	page, _, err = service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Nodes[0].CollectionLevel != LevelCritical {
		t.Fatalf("5-cycle page = %#v, err = %v", page, err)
	}
	provider.snapshot.Nodes[0].ReportedAt = node.ReportedAt.Add(-time.Second)
	clock.Advance(2 * time.Second)
	page, _, err = service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Nodes[0].CollectionLevel != LevelNormal {
		t.Fatalf("rollback page = %#v, err = %v", page, err)
	}
}

func TestElasticsearchServiceClusterFreshnessReachesFiveCyclesAndResetsOnRollback(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &elasticsearchTestClock{now: now}
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}}}
	service := NewElasticsearch(provider, cache.New(clock.Now), ElasticsearchOptions{SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	overview, _, err := service.Overview(context.Background())
	if err != nil || levelFromCounts(overview.Clusters) != LevelNormal {
		t.Fatalf("baseline overview = %#v, err = %v", overview, err)
	}
	clock.Advance(75 * time.Second)
	overview, _, err = service.Overview(context.Background())
	if err != nil || levelFromCounts(overview.Clusters) != LevelCritical {
		t.Fatalf("5-cycle overview = %#v, err = %v", overview, err)
	}
	provider.snapshot.Clusters[0].ReportedAt = cluster.ReportedAt.Add(-time.Second)
	clock.Advance(2 * time.Second)
	overview, _, err = service.Overview(context.Background())
	if err != nil || levelFromCounts(overview.Clusters) != LevelNormal {
		t.Fatalf("rollback overview = %#v, err = %v", overview, err)
	}
}

func TestElasticsearchServiceReturnsStaleDeepCopies(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	clock := &elasticsearchTestClock{now: now}
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	node := healthyElasticsearchNode(cluster.Name, "fixture-node-a")
	provider := &recordingElasticsearchProvider{snapshot: elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{node}}}
	service := NewElasticsearch(provider, cache.New(clock.Now), ElasticsearchOptions{SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now})

	first, _, err := service.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first.Clusters[0].Name = "mutated-cluster"
	first.Nodes[0].Roles[0] = elasticsearch.RoleML
	*first.Nodes[0].HeapUsedBytes = 999
	clock.Advance(30 * time.Second)
	provider.err = elasticsearch.ErrUnavailable
	second, meta, err := service.snapshot(context.Background())
	if err != nil || !meta.Stale {
		t.Fatalf("meta = %#v, err = %v", meta, err)
	}
	if second.Clusters[0].Name == "mutated-cluster" || second.Nodes[0].Roles[0] == elasticsearch.RoleML || *second.Nodes[0].HeapUsedBytes == 999 {
		t.Fatalf("stale snapshot shares mutable state: %#v", second)
	}
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Nodes[0].CollectionLevel != LevelWarning {
		t.Fatalf("failed load advanced freshness: page = %#v, err = %v", page, err)
	}
	second.Nodes[0].Roles[0] = elasticsearch.RoleML
	third, _, err := service.snapshot(context.Background())
	if err != nil || third.Nodes[0].Roles[0] == elasticsearch.RoleML {
		t.Fatalf("stale returns share mutable state: %#v, err = %v", third, err)
	}
}

func TestElasticsearchOverviewSaturatesUnassignedShardSums(t *testing.T) {
	first := healthyElasticsearchCluster("fixture-cluster-a")
	first.Health = elasticsearch.HealthYellow
	first.UnassignedShards = elasticsearchInt64Ptr(math.MaxInt64)
	second := healthyElasticsearchCluster("fixture-cluster-b")
	second.Health = elasticsearch.HealthYellow
	second.UnassignedShards = elasticsearchInt64Ptr(math.MaxInt64)
	service := newElasticsearchServiceWithSnapshot(elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{first, second}})
	overview, _, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := int(^uint(0) >> 1)
	if overview.Alerts.UnassignedShards.Warning != want {
		t.Fatalf("unassigned warning = %d, want saturation %d", overview.Alerts.UnassignedShards.Warning, want)
	}
}

func TestElasticsearchOverviewUsesSeparateCountsAndExactAlertSemantics(t *testing.T) {
	snapshot := elasticsearchOverviewFixture()
	service := newElasticsearchServiceWithSnapshot(snapshot)
	overview, _, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Status != LevelCritical {
		t.Fatalf("overview status = %q", overview.Status)
	}
	wantClusters := ElasticsearchLevelCounts{Total: 4, Normal: 1, Warning: 1, Critical: 1, Unknown: 1}
	wantNodes := ElasticsearchLevelCounts{Total: 4, Warning: 2, Critical: 1, Unknown: 1}
	wantAlerts := ElasticsearchOverviewAlerts{
		ClusterHealth:     ElasticsearchAlertCount{Warning: 1, Critical: 1},
		NodeResource:      ElasticsearchAlertCount{Warning: 2, Critical: 1},
		UnassignedShards:  ElasticsearchAlertCount{Warning: 3, Critical: 2},
		RequestRejections: ElasticsearchAlertCount{Warning: 1},
	}
	if !reflect.DeepEqual(overview.Clusters, wantClusters) || !reflect.DeepEqual(overview.Nodes, wantNodes) || !reflect.DeepEqual(overview.Alerts, wantAlerts) {
		t.Fatalf("overview = %#v, want counts %#v/%#v and alerts %#v", overview, wantClusters, wantNodes, wantAlerts)
	}
}

func TestElasticsearchNodesFiltersAndBuildsOptionsFromCompleteSnapshot(t *testing.T) {
	snapshot := elasticsearchQueryFixture()
	service := newElasticsearchServiceWithSnapshot(snapshot)
	tests := []struct {
		name  string
		query ElasticsearchQuery
		want  string
	}{
		{"search node", ElasticsearchQuery{Search: "NODE-B"}, "fixture-node-b"},
		{"search address", ElasticsearchQuery{Search: "198.51.100.11"}, "fixture-node-c"},
		{"exact cluster", ElasticsearchQuery{Cluster: "fixture-cluster-a", Search: "192.0.2.11"}, "fixture-node-b"},
		{"role membership", ElasticsearchQuery{Role: elasticsearch.RoleIngest}, "fixture-node-a"},
		{"cluster health", ElasticsearchQuery{ClusterHealth: elasticsearch.HealthYellow}, "fixture-node-c"},
		{"node status", ElasticsearchQuery{Status: LevelCritical}, "fixture-node-c"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, _, err := service.Nodes(context.Background(), test.query)
			if err != nil || len(page.Nodes) != 1 || page.Nodes[0].Name != test.want {
				t.Fatalf("page = %#v, err = %v", page, err)
			}
			if !reflect.DeepEqual(page.AvailableClusters, []string{"fixture-cluster-a", "fixture-cluster-b"}) {
				t.Fatalf("clusters = %#v", page.AvailableClusters)
			}
			wantRoles := []elasticsearch.Role{elasticsearch.RoleData, elasticsearch.RoleDataHot, elasticsearch.RoleIngest, elasticsearch.RoleMaster}
			if !reflect.DeepEqual(page.AvailableRoles, wantRoles) {
				t.Fatalf("roles = %#v", page.AvailableRoles)
			}
		})
	}
}

func TestElasticsearchNodesSearchDoesNotMatchCluster(t *testing.T) {
	service := newElasticsearchServiceWithSnapshot(elasticsearchQueryFixture())
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Search: "CLUSTER-B"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Nodes) != 0 {
		t.Fatalf("cluster-only search page = %#v", page)
	}
	if !reflect.DeepEqual(page.AvailableClusters, []string{"fixture-cluster-a", "fixture-cluster-b"}) {
		t.Fatalf("clusters = %#v", page.AvailableClusters)
	}
}

func TestElasticsearchNodesSupportsExactSixteenSortFields(t *testing.T) {
	service := newElasticsearchServiceWithSnapshot(elasticsearchQueryFixture())
	tests := []struct {
		field string
		want  []string
	}{
		{"node", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"cluster", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"address", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"role", []string{"fixture-node-c", "fixture-node-a", "fixture-node-b"}},
		{"cluster_health", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"heap", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"disk", []string{"fixture-node-a", "fixture-node-c", "fixture-node-b"}},
		{"cpu", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"index_rate", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"search_rate", []string{"fixture-node-c", "fixture-node-b", "fixture-node-a"}},
		{"documents", []string{"fixture-node-c", "fixture-node-b", "fixture-node-a"}},
		{"store", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"thread_queue", []string{"fixture-node-c", "fixture-node-b", "fixture-node-a"}},
		{"rejected_rate", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
		{"uptime", []string{"fixture-node-c", "fixture-node-a", "fixture-node-b"}},
		{"status", []string{"fixture-node-a", "fixture-node-b", "fixture-node-c"}},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: test.field, Order: "asc", Page: 1, PageSize: 20})
			if err != nil || len(page.Nodes) != 3 {
				t.Fatalf("sort %q page = %#v, err = %v", test.field, page, err)
			}
			got := []string{page.Nodes[0].Name, page.Nodes[1].Name, page.Nodes[2].Name}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("sort %q names = %#v, want %#v", test.field, got, test.want)
			}
		})
	}
	for _, rejected := range []string{"roles", "store_size", "thread_pool_queue"} {
		if _, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: rejected, Page: 1, PageSize: 20}); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("legacy sort %q err = %v", rejected, err)
		}
	}
}

func TestElasticsearchNodesSortsLargeIntegerMetricsWithoutPrecisionLoss(t *testing.T) {
	snapshot := elasticsearchQueryFixture()
	snapshot.Nodes[0].Documents = elasticsearchInt64Ptr(9007199254740992)
	snapshot.Nodes[1].Documents = elasticsearchInt64Ptr(9007199254740993)
	snapshot.Nodes[2].Documents = elasticsearchInt64Ptr(9007199254740994)
	service := newElasticsearchServiceWithSnapshot(snapshot)
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: "documents", Order: "asc", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{*page.Nodes[0].Documents, *page.Nodes[1].Documents, *page.Nodes[2].Documents}
	want := []int64{9007199254740992, 9007199254740993, 9007199254740994}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documents = %#v, want %#v", got, want)
	}
}

func TestElasticsearchNodesKeepsMissingMetricsLastInBothOrders(t *testing.T) {
	tests := []struct {
		field  string
		mutate func(*elasticsearch.Node)
	}{
		{"cpu", func(node *elasticsearch.Node) { node.CPUUsagePercent = nil }},
		{"documents", func(node *elasticsearch.Node) { node.Documents = nil }},
	}
	for _, test := range tests {
		for _, order := range []string{"asc", "desc"} {
			t.Run(test.field+"/"+order, func(t *testing.T) {
				snapshot := elasticsearchQueryFixture()
				test.mutate(&snapshot.Nodes[1])
				service := newElasticsearchServiceWithSnapshot(snapshot)
				page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: test.field, Order: order, Page: 1, PageSize: 20})
				if err != nil || len(page.Nodes) != 3 || page.Nodes[2].Name != "fixture-node-a" {
					t.Fatalf("missing-last page = %#v, err = %v", page, err)
				}
			})
		}
	}
}

func TestElasticsearchNodesEndsEverySortWithStableIDTieBreak(t *testing.T) {
	cluster := healthyElasticsearchCluster("fixture-cluster-a")
	first := healthyElasticsearchNode(cluster.Name, "fixture-node-same")
	first.ID = "id-b"
	second := first
	second.ID = "id-a"
	service := newElasticsearchServiceWithSnapshot(elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{cluster}, Nodes: []elasticsearch.Node{first, second}})
	fields := []string{"node", "cluster", "address", "role", "cluster_health", "heap", "disk", "cpu", "index_rate", "search_rate", "documents", "store", "thread_queue", "rejected_rate", "uptime", "status"}
	for _, field := range fields {
		for _, order := range []string{"asc", "desc"} {
			t.Run(field+"/"+order, func(t *testing.T) {
				page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: field, Order: order, Page: 1, PageSize: 20})
				ids := make([]string, len(page.Nodes))
				for index := range page.Nodes {
					ids[index] = page.Nodes[index].ID
				}
				if err != nil || !reflect.DeepEqual(ids, []string{"id-a", "id-b"}) {
					t.Fatalf("IDs = %#v, err = %v", ids, err)
				}
			})
		}
	}
}

func TestElasticsearchNodesUsesNaturalDefaultOrderAndStableIDTieBreak(t *testing.T) {
	snapshot := elasticsearchQueryFixture()
	snapshot.Nodes[0].Name = "fixture-node-z"
	snapshot.Nodes[0].ID = "id-z"
	snapshot.Nodes[1].Name = "fixture-node-a"
	snapshot.Nodes[1].ID = "id-a"
	snapshot.Nodes[2].Cluster = "fixture-cluster-a"
	snapshot.Nodes[2].Name = "fixture-node-a"
	snapshot.Nodes[2].ID = "id-b"
	service := newElasticsearchServiceWithSnapshot(snapshot)
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{page.Nodes[0].ID, page.Nodes[1].ID, page.Nodes[2].ID}
	if want := []string{"id-a", "id-b", "id-z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default IDs = %#v, want %#v", got, want)
	}
	for index := range snapshot.Nodes {
		snapshot.Nodes[index].CPUUsagePercent = elasticsearchFloatPtr(50)
	}
	service = newElasticsearchServiceWithSnapshot(snapshot)
	page, _, err = service.Nodes(context.Background(), ElasticsearchQuery{Sort: "cpu", Order: "desc", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	got = []string{page.Nodes[0].ID, page.Nodes[1].ID, page.Nodes[2].ID}
	if want := []string{"id-a", "id-b", "id-z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tie-break IDs = %#v, want %#v", got, want)
	}
}

func TestElasticsearchNodesNormalizesDefaultsAndValidatesQuery(t *testing.T) {
	service := newElasticsearchServiceWithSnapshot(elasticsearchQueryFixture())
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{})
	if err != nil || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("default page = %#v, err = %v", page, err)
	}
	for _, pageSize := range []int{20, 50, 100} {
		page, _, err = service.Nodes(context.Background(), ElasticsearchQuery{Page: 1, PageSize: pageSize})
		if err != nil || page.PageSize != pageSize {
			t.Fatalf("page size %d page = %#v, err = %v", pageSize, page, err)
		}
	}
	invalid := []ElasticsearchQuery{
		{Page: -1, PageSize: 20},
		{Page: 1, PageSize: 10},
		{Page: 1, PageSize: 20, Status: Level("bad")},
		{Page: 1, PageSize: 20, Role: elasticsearch.Role("bad")},
		{Page: 1, PageSize: 20, ClusterHealth: elasticsearch.Health("bad")},
		{Page: 1, PageSize: 20, Sort: "bad"},
		{Page: 1, PageSize: 20, Order: "bad"},
	}
	for _, query := range invalid {
		if _, _, queryErr := service.Nodes(context.Background(), query); !errors.Is(queryErr, ErrInvalidQuery) {
			t.Fatalf("query %#v err = %v", query, queryErr)
		}
	}
}

func TestElasticsearchNodesPaginatesAfterStableSorting(t *testing.T) {
	snapshot := elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{healthyElasticsearchCluster("fixture-cluster-a")}}
	for index := 0; index < 21; index++ {
		name := fmt.Sprintf("fixture-node-%02d", index)
		node := healthyElasticsearchNode("fixture-cluster-a", name)
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	service := newElasticsearchServiceWithSnapshot(snapshot)
	page, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Page: 2, PageSize: 20})
	if err != nil || page.Total != 21 || len(page.Nodes) != 1 || page.Nodes[0].Name != "fixture-node-20" {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
}

func TestElasticsearchNodesRejectsOverflowPageOffset(t *testing.T) {
	service := newElasticsearchServiceWithSnapshot(elasticsearchQueryFixture())
	_, _, err := service.Nodes(context.Background(), ElasticsearchQuery{Sort: "node", Order: "asc", Page: math.MaxInt, PageSize: 20})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("err = %v", err)
	}
}

type recordingElasticsearchProvider struct {
	snapshot elasticsearch.Snapshot
	err      error
	calls    int
}

func (provider *recordingElasticsearchProvider) ElasticsearchSnapshot(context.Context) (elasticsearch.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return elasticsearch.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type elasticsearchTestClock struct{ now time.Time }

func (clock *elasticsearchTestClock) Now() time.Time              { return clock.now }
func (clock *elasticsearchTestClock) Advance(value time.Duration) { clock.now = clock.now.Add(value) }

func healthyElasticsearchCluster(name string) elasticsearch.Cluster {
	return elasticsearch.Cluster{
		ID:                    elasticsearch.StableClusterID(name),
		Name:                  name,
		Availability:          elasticsearch.AvailabilityUp,
		NodeStatsAvailability: elasticsearch.AvailabilityUp,
		Health:                elasticsearch.HealthGreen,
		UnassignedShards:      elasticsearchInt64Ptr(0),
		CollectionTracked:     true,
		ReportedAt:            time.Date(2026, 8, 1, 7, 59, 0, 0, time.UTC),
	}
}

func healthyElasticsearchNode(cluster, name string) elasticsearch.Node {
	return elasticsearch.Node{
		ID:                elasticsearch.StableNodeID(cluster, name),
		Name:              name,
		Cluster:           cluster,
		Address:           "192.0.2.10",
		Roles:             []elasticsearch.Role{elasticsearch.RoleData},
		HeapUsedBytes:     elasticsearchInt64Ptr(40),
		HeapMaxBytes:      elasticsearchInt64Ptr(100),
		DiskUsagePercent:  elasticsearchFloatPtr(40),
		CPUUsagePercent:   elasticsearchFloatPtr(30),
		IndexRate:         elasticsearchFloatPtr(10),
		SearchRate:        elasticsearchFloatPtr(20),
		Documents:         elasticsearchInt64Ptr(100),
		StoreSizeBytes:    elasticsearchInt64Ptr(200),
		ThreadPoolQueue:   elasticsearchInt64Ptr(0),
		RejectedRate:      elasticsearchFloatPtr(0),
		UptimeSeconds:     elasticsearchInt64Ptr(3600),
		DataNode:          true,
		CollectionTracked: true,
		ReportedAt:        time.Date(2026, 8, 1, 7, 59, 0, 0, time.UTC),
	}
}

func elasticsearchOverviewFixture() elasticsearch.Snapshot {
	green := healthyElasticsearchCluster("fixture-cluster-green")
	yellow := healthyElasticsearchCluster("fixture-cluster-yellow")
	yellow.Health = elasticsearch.HealthYellow
	yellow.UnassignedShards = elasticsearchInt64Ptr(3)
	red := healthyElasticsearchCluster("fixture-cluster-red")
	red.Health = elasticsearch.HealthRed
	red.UnassignedShards = elasticsearchInt64Ptr(2)
	unknown := healthyElasticsearchCluster("fixture-cluster-unknown")
	unknown.Health = elasticsearch.HealthUnknown

	diskCritical := healthyElasticsearchNode(green.Name, "fixture-node-disk")
	diskCritical.DiskUsagePercent = elasticsearchFloatPtr(90)
	heapWarning := healthyElasticsearchNode(yellow.Name, "fixture-node-heap")
	heapWarning.HeapUsedBytes = elasticsearchInt64Ptr(75)
	rejected := healthyElasticsearchNode(red.Name, "fixture-node-rejected")
	rejected.RejectedRate = elasticsearchFloatPtr(0.01)
	missingDisk := healthyElasticsearchNode(unknown.Name, "fixture-node-missing")
	missingDisk.DiskUsagePercent = nil
	return elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{green, yellow, red, unknown}, Nodes: []elasticsearch.Node{diskCritical, heapWarning, rejected, missingDisk}}
}

func elasticsearchQueryFixture() elasticsearch.Snapshot {
	clusterA := healthyElasticsearchCluster("fixture-cluster-a")
	clusterB := healthyElasticsearchCluster("fixture-cluster-b")
	clusterB.Health = elasticsearch.HealthYellow
	nodeA := healthyElasticsearchNode(clusterA.Name, "fixture-node-a")
	nodeA.ID = "id-a"
	nodeA.Address = "192.0.2.10"
	nodeA.Roles = []elasticsearch.Role{elasticsearch.RoleData, elasticsearch.RoleDataHot, elasticsearch.RoleIngest}
	nodeA.HeapUsedBytes = elasticsearchInt64Ptr(10)
	nodeA.DiskUsagePercent = elasticsearchFloatPtr(10)
	nodeA.CPUUsagePercent = elasticsearchFloatPtr(10)
	nodeA.IndexRate = elasticsearchFloatPtr(10)
	nodeA.SearchRate = elasticsearchFloatPtr(30)
	nodeA.Documents = elasticsearchInt64Ptr(300)
	nodeA.StoreSizeBytes = elasticsearchInt64Ptr(10)
	nodeA.ThreadPoolQueue = elasticsearchInt64Ptr(30)
	nodeA.UptimeSeconds = elasticsearchInt64Ptr(20)
	nodeB := healthyElasticsearchNode(clusterA.Name, "fixture-node-b")
	nodeB.ID = "id-b"
	nodeB.Address = "192.0.2.11"
	nodeB.Roles = []elasticsearch.Role{elasticsearch.RoleMaster}
	nodeB.DataNode = false
	nodeB.DiskUsagePercent = nil
	nodeB.HeapUsedBytes = elasticsearchInt64Ptr(20)
	nodeB.CPUUsagePercent = elasticsearchFloatPtr(40)
	nodeB.IndexRate = elasticsearchFloatPtr(20)
	nodeB.SearchRate = elasticsearchFloatPtr(20)
	nodeB.Documents = elasticsearchInt64Ptr(200)
	nodeB.StoreSizeBytes = elasticsearchInt64Ptr(20)
	nodeB.ThreadPoolQueue = elasticsearchInt64Ptr(20)
	nodeB.UptimeSeconds = elasticsearchInt64Ptr(30)
	nodeC := healthyElasticsearchNode(clusterB.Name, "fixture-node-c")
	nodeC.ID = "id-c"
	nodeC.Address = "198.51.100.11"
	nodeC.HeapUsedBytes = elasticsearchInt64Ptr(30)
	nodeC.DiskUsagePercent = elasticsearchFloatPtr(90)
	nodeC.CPUUsagePercent = elasticsearchFloatPtr(50)
	nodeC.IndexRate = elasticsearchFloatPtr(30)
	nodeC.SearchRate = elasticsearchFloatPtr(10)
	nodeC.Documents = elasticsearchInt64Ptr(100)
	nodeC.StoreSizeBytes = elasticsearchInt64Ptr(30)
	nodeC.ThreadPoolQueue = elasticsearchInt64Ptr(10)
	nodeC.RejectedRate = elasticsearchFloatPtr(1)
	nodeC.UptimeSeconds = elasticsearchInt64Ptr(10)
	return elasticsearch.Snapshot{Clusters: []elasticsearch.Cluster{clusterA, clusterB}, Nodes: []elasticsearch.Node{nodeC, nodeA, nodeB}}
}

func newElasticsearchServiceWithSnapshot(snapshot elasticsearch.Snapshot) *ElasticsearchService {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	return NewElasticsearch(
		&recordingElasticsearchProvider{snapshot: snapshot},
		cache.New(func() time.Time { return now }),
		ElasticsearchOptions{SnapshotTTL: 15 * time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: func() time.Time { return now }},
	)
}

func elasticsearchInt64Ptr(value int64) *int64     { return &value }
func elasticsearchFloatPtr(value float64) *float64 { return &value }

// levelFromCounts extracts the only represented level in one-item freshness tests.
func levelFromCounts(counts ElasticsearchLevelCounts) Level {
	switch {
	case counts.Critical == 1:
		return LevelCritical
	case counts.Warning == 1:
		return LevelWarning
	case counts.Unknown == 1:
		return LevelUnknown
	default:
		return LevelNormal
	}
}
