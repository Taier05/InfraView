package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/rabbitmq"
)

func TestRabbitMQClusterConnectivityDoesNotContaminateNodes(t *testing.T) {
	snapshot := rabbitMQHealthySnapshot()
	snapshot.Clusters[0].UnreachablePeers = rabbitMQInt64(1)
	service := newRabbitMQTestService(snapshot)

	overview, _, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := service.Nodes(context.Background(), RabbitMQQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Clusters.Critical != 1 || overview.Nodes.Normal != 1 {
		t.Fatalf("overview counts = %#v/%#v", overview.Clusters, overview.Nodes)
	}
	if page.Nodes[0].Status != LevelNormal {
		t.Fatalf("cluster issue contaminated node: %#v", page.Nodes[0])
	}
}

func TestRabbitMQMessagesNeverAffectStatus(t *testing.T) {
	snapshot := rabbitMQHealthySnapshot()
	snapshot.Nodes[0].Messages = rabbitMQInt64(math.MaxInt64)
	page := mustRabbitMQPage(t, newRabbitMQTestService(snapshot), RabbitMQQuery{})
	if page.Nodes[0].Messages == nil || *page.Nodes[0].Messages != math.MaxInt64 || page.Nodes[0].Status != LevelNormal {
		t.Fatalf("messages changed status or were hidden: %#v", page.Nodes[0])
	}
}

func TestRabbitMQResourceThresholdsUseExactBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*rabbitmq.Node)
		level  Level
		source RabbitMQNodeStatusSource
	}{
		{name: "healthy", level: LevelNormal, source: RabbitMQStatusNormal},
		{name: "memory below warning", mutate: func(node *rabbitmq.Node) { node.MemoryUsedBytes = rabbitMQInt64(79) }, level: LevelNormal, source: RabbitMQStatusNormal},
		{name: "memory warning", mutate: func(node *rabbitmq.Node) { node.MemoryUsedBytes = rabbitMQInt64(80) }, level: LevelWarning, source: RabbitMQStatusMemory},
		{name: "memory critical", mutate: func(node *rabbitmq.Node) { node.MemoryUsedBytes = rabbitMQInt64(90) }, level: LevelCritical, source: RabbitMQStatusMemory},
		{name: "file descriptor warning", mutate: func(node *rabbitmq.Node) { node.OpenFileDescriptors = rabbitMQInt64(80) }, level: LevelWarning, source: RabbitMQStatusFileDescriptor},
		{name: "file descriptor critical", mutate: func(node *rabbitmq.Node) { node.OpenFileDescriptors = rabbitMQInt64(90) }, level: LevelCritical, source: RabbitMQStatusFileDescriptor},
		{name: "erlang process warning", mutate: func(node *rabbitmq.Node) { node.ErlangProcessesUsed = rabbitMQInt64(80) }, level: LevelWarning, source: RabbitMQStatusErlangProcess},
		{name: "erlang process critical", mutate: func(node *rabbitmq.Node) { node.ErlangProcessesUsed = rabbitMQInt64(90) }, level: LevelCritical, source: RabbitMQStatusErlangProcess},
		{name: "disk critical equal limit", mutate: func(node *rabbitmq.Node) { node.DiskAvailableBytes = rabbitMQInt64(100) }, level: LevelCritical, source: RabbitMQStatusDisk},
		{name: "disk warning above limit", mutate: func(node *rabbitmq.Node) { node.DiskAvailableBytes = rabbitMQInt64(119) }, level: LevelWarning, source: RabbitMQStatusDisk},
		{name: "disk normal at one point two", mutate: func(node *rabbitmq.Node) { node.DiskAvailableBytes = rabbitMQInt64(120) }, level: LevelNormal, source: RabbitMQStatusNormal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")
			if test.mutate != nil {
				test.mutate(&node)
			}
			got := summarizeRabbitMQNode(node, LevelNormal)
			if got.Status != test.level || got.StatusSource != test.source {
				t.Fatalf("status/source = %q/%q, want %q/%q", got.Status, got.StatusSource, test.level, test.source)
			}
		})
	}
}

func TestRabbitMQExplicitAlarmsAreCriticalWithStablePriority(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*rabbitmq.Node)
	}{
		{name: "memory alarm", mutate: func(node *rabbitmq.Node) { node.MemoryAlarm = rabbitMQBool(true) }},
		{name: "disk alarm", mutate: func(node *rabbitmq.Node) { node.DiskAlarm = rabbitMQBool(true) }},
		{name: "file descriptor alarm", mutate: func(node *rabbitmq.Node) { node.FileDescriptorAlarm = rabbitMQBool(true) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")
			test.mutate(&node)
			node.DiskAvailableBytes = rabbitMQInt64(100)
			got := summarizeRabbitMQNode(node, LevelCritical)
			if got.Status != LevelCritical || got.StatusSource != RabbitMQStatusAlarm {
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}

func TestRabbitMQMissingCoreStateIsUnknown(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*rabbitmq.Node)
		source RabbitMQNodeStatusSource
	}{
		{name: "alarm", mutate: func(node *rabbitmq.Node) { node.MemoryAlarm = nil }, source: RabbitMQStatusAlarm},
		{name: "memory numerator", mutate: func(node *rabbitmq.Node) { node.MemoryUsedBytes = nil }, source: RabbitMQStatusMemory},
		{name: "memory denominator", mutate: func(node *rabbitmq.Node) { node.MemoryLimitBytes = nil }, source: RabbitMQStatusMemory},
		{name: "disk", mutate: func(node *rabbitmq.Node) { node.DiskAvailableBytes = nil }, source: RabbitMQStatusDisk},
		{name: "file descriptor", mutate: func(node *rabbitmq.Node) { node.MaxFileDescriptors = nil }, source: RabbitMQStatusFileDescriptor},
		{name: "erlang process", mutate: func(node *rabbitmq.Node) { node.ErlangProcessesUsed = nil }, source: RabbitMQStatusErlangProcess},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")
			test.mutate(&node)
			got := summarizeRabbitMQNode(node, LevelNormal)
			if got.Status != LevelUnknown || got.StatusSource != test.source {
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}

func TestRabbitMQStatusUsesRequiredSeverityAndSourcePriority(t *testing.T) {
	node := rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")
	node.MemoryUsedBytes = rabbitMQInt64(80)
	node.DiskAvailableBytes = rabbitMQInt64(119)
	node.OpenFileDescriptors = rabbitMQInt64(80)
	node.ErlangProcessesUsed = rabbitMQInt64(80)
	got := summarizeRabbitMQNode(node, LevelWarning)
	if got.Status != LevelWarning || got.StatusSource != RabbitMQStatusCollection {
		t.Fatalf("equal warning priority = %#v", got)
	}
	node.DiskAvailableBytes = rabbitMQInt64(100)
	got = summarizeRabbitMQNode(node, LevelWarning)
	if got.Status != LevelCritical || got.StatusSource != RabbitMQStatusDisk {
		t.Fatalf("critical must dominate warning = %#v", got)
	}
}

func TestRabbitMQServiceSharesSnapshotCacheAndTracksObservedProgress(t *testing.T) {
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	clock := &rabbitMQTestClock{now: now}
	snapshot := rabbitMQHealthySnapshot()
	snapshot.Nodes[0].ReportedAt = now.Add(-24 * time.Hour)
	provider := &recordingRabbitMQProvider{snapshot: snapshot}
	service := NewRabbitMQ(provider, cache.New(clock.Now), RabbitMQOptions{
		SnapshotTTL: 15 * time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock.Now,
	})

	assertLevel := func(want Level) {
		t.Helper()
		page := mustRabbitMQPage(t, service, RabbitMQQuery{})
		if got := page.Nodes[0].CollectionLevel; got != want {
			t.Fatalf("collection = %q, want %q", got, want)
		}
	}
	assertLevel(LevelNormal)
	if _, _, err := service.Overview(context.Background()); err != nil || provider.calls != 1 {
		t.Fatalf("provider calls = %d, err = %v", provider.calls, err)
	}
	clock.Advance(30 * time.Second)
	assertLevel(LevelWarning)
	if provider.calls != 2 {
		t.Fatalf("provider calls after warning refresh = %d", provider.calls)
	}
	clock.Advance(45 * time.Second)
	assertLevel(LevelCritical)
	provider.snapshot.Nodes[0].ReportedAt = snapshot.Nodes[0].ReportedAt.Add(15 * time.Second)
	clock.Advance(16 * time.Second)
	assertLevel(LevelNormal)
	if provider.calls != 4 {
		t.Fatalf("provider calls after refresh = %d", provider.calls)
	}
}

func TestRabbitMQServiceReturnsAgingStaleDeepCopies(t *testing.T) {
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	clock := &rabbitMQTestClock{now: now}
	provider := &recordingRabbitMQProvider{snapshot: rabbitMQHealthySnapshot()}
	service := NewRabbitMQ(provider, cache.New(clock.Now), RabbitMQOptions{
		SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: 2 * time.Minute, Clock: clock.Now,
	})

	first := mustRabbitMQPage(t, service, RabbitMQQuery{})
	*first.Nodes[0].Connections = 999
	clock.Advance(30 * time.Second)
	provider.err = rabbitmq.ErrUnavailable
	page, meta, err := service.Nodes(context.Background(), RabbitMQQuery{})
	if err != nil || !meta.Stale || page.Nodes[0].CollectionLevel != LevelWarning || *page.Nodes[0].Connections == 999 {
		t.Fatalf("warning stale page = %#v, meta = %#v, err = %v", page, meta, err)
	}
	clock.Advance(45 * time.Second)
	page, meta, err = service.Nodes(context.Background(), RabbitMQQuery{})
	if err != nil || !meta.Stale || page.Nodes[0].CollectionLevel != LevelCritical {
		t.Fatalf("critical stale page = %#v, meta = %#v, err = %v", page, meta, err)
	}
}

func TestRabbitMQOverviewKeepsSeparateCountsAndAlertSemantics(t *testing.T) {
	snapshot := rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{
			{ID: "cluster-a", Name: "cluster-a", UnreachablePeers: rabbitMQInt64(0)},
			{ID: "cluster-b", Name: "cluster-b", UnreachablePeers: rabbitMQInt64(2)},
			{ID: "cluster-c", Name: "cluster-c"},
		},
		Nodes: []rabbitmq.Node{
			rabbitMQHealthyNode("node-normal", "node-normal", "cluster-a", "fixture-address-a"),
			rabbitMQHealthyNode("node-pressure", "node-pressure", "cluster-a", "fixture-address-b"),
			rabbitMQHealthyNode("node-alarm", "node-alarm", "cluster-b", "fixture-address-c"),
			rabbitMQHealthyNode("node-unknown", "node-unknown", "cluster-c", "fixture-address-d"),
		},
	}
	snapshot.Nodes[1].MemoryUsedBytes = rabbitMQInt64(80)
	snapshot.Nodes[2].DiskAlarm = rabbitMQBool(true)
	snapshot.Nodes[3].MemoryLimitBytes = nil
	overview, _, err := newRabbitMQTestService(snapshot).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantClusters := RabbitMQLevelCounts{Total: 3, Normal: 1, Critical: 1, Unknown: 1}
	wantNodes := RabbitMQLevelCounts{Total: 4, Normal: 1, Warning: 1, Critical: 1, Unknown: 1}
	if overview.Status != LevelCritical || overview.Clusters != wantClusters || overview.Nodes != wantNodes {
		t.Fatalf("overview = %#v", overview)
	}
	if overview.Alerts.ClusterConnectivity != (RabbitMQAlertCount{Critical: 1, Unknown: 1}) ||
		overview.Alerts.ResourceAlarms != (RabbitMQAlertCount{Critical: 1}) ||
		overview.Alerts.ResourcePressure != (RabbitMQAlertCount{Warning: 1, Unknown: 1}) ||
		overview.Alerts.Collection != (RabbitMQAlertCount{}) {
		t.Fatalf("alerts = %#v", overview.Alerts)
	}
}

func TestRabbitMQNodesSearchesNameAndAddressOnlyAndFiltersExactly(t *testing.T) {
	snapshot := rabbitMQQuerySnapshot()
	service := newRabbitMQTestService(snapshot)

	page := mustRabbitMQPage(t, service, RabbitMQQuery{Search: "node-b", Cluster: "cluster-b", Status: LevelWarning})
	if page.Total != 1 || page.Nodes[0].ID != "id-b" {
		t.Fatalf("filtered page = %#v", page)
	}
	page = mustRabbitMQPage(t, service, RabbitMQQuery{Search: "fixture-address-c"})
	if page.Total != 1 || page.Nodes[0].ID != "id-c" {
		t.Fatalf("address search page = %#v", page)
	}
	page = mustRabbitMQPage(t, service, RabbitMQQuery{Search: "cluster-a"})
	if page.Total != 0 {
		t.Fatalf("cluster leaked into search = %#v", page)
	}
	page = mustRabbitMQPage(t, service, RabbitMQQuery{Search: "version-a"})
	if page.Total != 0 {
		t.Fatalf("version leaked into search = %#v", page)
	}
}

func TestRabbitMQNodesReturnsSortedClustersFromCompleteSnapshot(t *testing.T) {
	page := mustRabbitMQPage(t, newRabbitMQTestService(rabbitMQQuerySnapshot()), RabbitMQQuery{Cluster: "cluster-b"})
	if want := []string{"cluster-a", "cluster-b"}; !reflect.DeepEqual(page.AvailableClusters, want) {
		t.Fatalf("available clusters = %#v, want %#v", page.AvailableClusters, want)
	}
}

func TestRabbitMQNodesSupportsExactFifteenSortFields(t *testing.T) {
	service := newRabbitMQTestService(rabbitMQQuerySnapshot())
	fields := []string{
		"node", "cluster", "address", "version", "memory", "disk", "file_descriptor", "erlang_process",
		"connections", "queues", "messages", "publish_rate", "deliver_rate", "uptime", "status",
	}
	for _, field := range fields {
		for _, order := range []string{"asc", "desc"} {
			t.Run(field+"/"+order, func(t *testing.T) {
				page, _, err := service.Nodes(context.Background(), RabbitMQQuery{Sort: field, Order: order, Page: 1, PageSize: 20})
				if err != nil || len(page.Nodes) != 3 {
					t.Fatalf("page = %#v, err = %v", page, err)
				}
			})
		}
	}
	for _, rejected := range []string{"name", "memory_usage_percent", "disk_available_bytes", "direction"} {
		if _, _, err := service.Nodes(context.Background(), RabbitMQQuery{Sort: rejected, Page: 1, PageSize: 20}); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("unsupported sort %q error = %v", rejected, err)
		}
	}
}

func TestRabbitMQNodesKeepMissingNumericValuesLastAndSortInt64Exactly(t *testing.T) {
	snapshot := rabbitMQQuerySnapshot()
	snapshot.Nodes[0].Connections = rabbitMQInt64(9007199254740993)
	snapshot.Nodes[1].Connections = rabbitMQInt64(9007199254740992)
	snapshot.Nodes[2].Connections = nil
	service := newRabbitMQTestService(snapshot)
	for _, test := range []struct {
		order string
		want  []string
	}{
		{order: "asc", want: []string{"id-a", "id-c", "id-b"}},
		{order: "desc", want: []string{"id-c", "id-a", "id-b"}},
	} {
		page := mustRabbitMQPage(t, service, RabbitMQQuery{Sort: "connections", Order: test.order, Page: 1, PageSize: 20})
		if got := rabbitMQNodeIDs(page.Nodes); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s IDs = %#v, want %#v", test.order, got, test.want)
		}
	}
}

func TestRabbitMQNodesEndEverySortWithAscendingStableIDTieBreak(t *testing.T) {
	first := rabbitMQHealthyNode("id-b", "node-same", "cluster-a", "fixture-address-a")
	second := first
	second.ID = "id-a"
	service := newRabbitMQTestService(rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{{ID: "cluster-a", Name: "cluster-a", UnreachablePeers: rabbitMQInt64(0)}},
		Nodes:    []rabbitmq.Node{first, second},
	})
	fields := []string{"node", "cluster", "address", "version", "memory", "disk", "file_descriptor", "erlang_process", "connections", "queues", "messages", "publish_rate", "deliver_rate", "uptime", "status"}
	for _, field := range fields {
		for _, order := range []string{"asc", "desc"} {
			page := mustRabbitMQPage(t, service, RabbitMQQuery{Sort: field, Order: order, Page: 1, PageSize: 20})
			if got := rabbitMQNodeIDs(page.Nodes); !reflect.DeepEqual(got, []string{"id-a", "id-b"}) {
				t.Fatalf("%s/%s IDs = %#v", field, order, got)
			}
		}
	}
}

func TestRabbitMQNodesSortTextFieldsNaturally(t *testing.T) {
	second := rabbitMQHealthyNode("id-second", "node-2", "cluster-2", "fixture-address-2")
	tenth := rabbitMQHealthyNode("id-tenth", "node-10", "cluster-10", "fixture-address-10")
	service := newRabbitMQTestService(rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{
			{ID: "cluster-10", Name: "cluster-10", UnreachablePeers: rabbitMQInt64(0)},
			{ID: "cluster-2", Name: "cluster-2", UnreachablePeers: rabbitMQInt64(0)},
		},
		Nodes: []rabbitmq.Node{tenth, second},
	})

	for _, field := range []string{"node", "cluster", "address"} {
		page := mustRabbitMQPage(t, service, RabbitMQQuery{Sort: field, Order: "asc"})
		if got := rabbitMQNodeIDs(page.Nodes); !reflect.DeepEqual(got, []string{"id-second", "id-tenth"}) {
			t.Fatalf("%s natural IDs = %#v", field, got)
		}
	}
}

func TestRabbitMQNodesSortNodeByNameAcrossClusters(t *testing.T) {
	second := rabbitMQHealthyNode("id-second", "node-2", "cluster-10", "fixture-address-second")
	tenth := rabbitMQHealthyNode("id-tenth", "node-10", "cluster-2", "fixture-address-tenth")
	service := newRabbitMQTestService(rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{
			{ID: "cluster-10", Name: "cluster-10", UnreachablePeers: rabbitMQInt64(0)},
			{ID: "cluster-2", Name: "cluster-2", UnreachablePeers: rabbitMQInt64(0)},
		},
		Nodes: []rabbitmq.Node{tenth, second},
	})

	page := mustRabbitMQPage(t, service, RabbitMQQuery{Sort: "node", Order: "asc"})
	if got := rabbitMQNodeIDs(page.Nodes); !reflect.DeepEqual(got, []string{"id-second", "id-tenth"}) {
		t.Fatalf("node-name natural IDs = %#v", got)
	}
}

func TestRabbitMQNodesNormalizeDefaultsValidateBeforeLoadAndRejectOverflow(t *testing.T) {
	provider := &recordingRabbitMQProvider{snapshot: rabbitMQQuerySnapshot()}
	service := NewRabbitMQ(provider, nil, RabbitMQOptions{})
	page, _, err := service.Nodes(context.Background(), RabbitMQQuery{})
	if err != nil || page.Page != 1 || page.PageSize != 20 || page.Nodes == nil || page.AvailableClusters == nil {
		t.Fatalf("default page = %#v, err = %v", page, err)
	}
	for _, pageSize := range []int{20, 50, 100} {
		if page, _, err = service.Nodes(context.Background(), RabbitMQQuery{Page: 1, PageSize: pageSize}); err != nil || page.PageSize != pageSize {
			t.Fatalf("page size %d page = %#v, err = %v", pageSize, page, err)
		}
	}
	invalidProvider := &recordingRabbitMQProvider{snapshot: rabbitMQQuerySnapshot()}
	invalidService := NewRabbitMQ(invalidProvider, nil, RabbitMQOptions{})
	invalid := []RabbitMQQuery{
		{Page: -1, PageSize: 20},
		{Page: 1, PageSize: 10},
		{Page: 1, PageSize: 20, Status: "bad"},
		{Page: 1, PageSize: 20, Sort: "bad"},
		{Page: 1, PageSize: 20, Order: "bad"},
		{Page: math.MaxInt/20 + 1, PageSize: 20},
		{Page: math.MaxInt, PageSize: 20},
	}
	for _, query := range invalid {
		if _, _, queryErr := invalidService.Nodes(context.Background(), query); !errors.Is(queryErr, ErrInvalidQuery) {
			t.Fatalf("query %#v error = %v", query, queryErr)
		}
	}
	if invalidProvider.calls != 0 {
		t.Fatalf("invalid query loaded provider %d times", invalidProvider.calls)
	}
}

func TestRabbitMQNodesPaginatesAfterFilteringAndStableSorting(t *testing.T) {
	snapshot := rabbitmq.Snapshot{Clusters: []rabbitmq.Cluster{{ID: "cluster-a", Name: "cluster-a", UnreachablePeers: rabbitMQInt64(0)}}}
	for index := 20; index >= 0; index-- {
		id := string(rune('a' + index))
		node := rabbitMQHealthyNode(id, "node-"+id, "cluster-a", "fixture-address-"+id)
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	page := mustRabbitMQPage(t, newRabbitMQTestService(snapshot), RabbitMQQuery{Page: 2, PageSize: 20})
	if page.Total != 21 || len(page.Nodes) != 1 || page.Nodes[0].ID != "u" {
		t.Fatalf("page = %#v", page)
	}
}

type recordingRabbitMQProvider struct {
	snapshot rabbitmq.Snapshot
	err      error
	calls    int
}

func (provider *recordingRabbitMQProvider) RabbitMQSnapshot(context.Context) (rabbitmq.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return rabbitmq.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type rabbitMQTestClock struct{ now time.Time }

func (clock *rabbitMQTestClock) Now() time.Time              { return clock.now }
func (clock *rabbitMQTestClock) Advance(value time.Duration) { clock.now = clock.now.Add(value) }

func newRabbitMQTestService(snapshot rabbitmq.Snapshot) *RabbitMQService {
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	return NewRabbitMQ(
		&recordingRabbitMQProvider{snapshot: snapshot},
		cache.New(func() time.Time { return now }),
		RabbitMQOptions{SnapshotTTL: 15 * time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: func() time.Time { return now }},
	)
}

func mustRabbitMQPage(t *testing.T, service *RabbitMQService, query RabbitMQQuery) RabbitMQPage {
	t.Helper()
	page, _, err := service.Nodes(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func rabbitMQHealthySnapshot() rabbitmq.Snapshot {
	return rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{{ID: "cluster-a", Name: "cluster-a", UnreachablePeers: rabbitMQInt64(0)}},
		Nodes:    []rabbitmq.Node{rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")},
	}
}

func rabbitMQQuerySnapshot() rabbitmq.Snapshot {
	first := rabbitMQHealthyNode("id-a", "node-a", "cluster-a", "fixture-address-a")
	first.Version = "version-c"
	first.Connections = rabbitMQInt64(30)
	first.PublishRate = rabbitMQFloat64(3)
	second := rabbitMQHealthyNode("id-b", "node-b", "cluster-b", "fixture-address-b")
	second.Version = "version-a"
	second.MemoryUsedBytes = rabbitMQInt64(80)
	second.Connections = rabbitMQInt64(10)
	second.PublishRate = rabbitMQFloat64(1)
	third := rabbitMQHealthyNode("id-c", "node-c", "cluster-b", "fixture-address-c")
	third.Version = "version-b"
	third.MemoryAlarm = rabbitMQBool(true)
	third.Connections = rabbitMQInt64(20)
	third.PublishRate = rabbitMQFloat64(2)
	return rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{
			{ID: "cluster-b", Name: "cluster-b", UnreachablePeers: rabbitMQInt64(0)},
			{ID: "cluster-a", Name: "cluster-a", UnreachablePeers: rabbitMQInt64(0)},
		},
		Nodes: []rabbitmq.Node{third, first, second},
	}
}

func rabbitMQHealthyNode(id, name, cluster, address string) rabbitmq.Node {
	return rabbitmq.Node{
		ID: id, Name: name, Cluster: cluster, Address: address, Version: "version-a",
		MemoryUsedBytes: rabbitMQInt64(40), MemoryLimitBytes: rabbitMQInt64(100),
		DiskAvailableBytes: rabbitMQInt64(200), DiskLimitBytes: rabbitMQInt64(100),
		OpenFileDescriptors: rabbitMQInt64(40), MaxFileDescriptors: rabbitMQInt64(100),
		ErlangProcessesUsed: rabbitMQInt64(40), ErlangProcessesLimit: rabbitMQInt64(100),
		Connections: rabbitMQInt64(10), Queues: rabbitMQInt64(2), Messages: rabbitMQInt64(3),
		PublishRate: rabbitMQFloat64(1), DeliverRate: rabbitMQFloat64(1),
		MemoryAlarm: rabbitMQBool(false), DiskAlarm: rabbitMQBool(false), FileDescriptorAlarm: rabbitMQBool(false),
		UptimeSeconds: rabbitMQInt64(60), CollectionTracked: true,
		ReportedAt: time.Date(2026, 8, 4, 7, 59, 0, 0, time.UTC),
	}
}

func rabbitMQNodeIDs(nodes []RabbitMQNodeSummary) []string {
	ids := make([]string, len(nodes))
	for index := range nodes {
		ids[index] = nodes[index].ID
	}
	return ids
}

func rabbitMQInt64(value int64) *int64       { return &value }
func rabbitMQFloat64(value float64) *float64 { return &value }
func rabbitMQBool(value bool) *bool          { return &value }
