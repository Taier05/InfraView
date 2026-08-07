package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/mysql"
)

func TestMySQLSummaryAggregatesReplicationChannelsAndBoundaries(t *testing.T) {
	tests := []struct {
		name string
		lag  float64
		want Level
	}{
		{name: "below warning", lag: 4.999, want: LevelNormal},
		{name: "warning boundary", lag: 5, want: LevelWarning},
		{name: "below critical", lag: 29.999, want: LevelWarning},
		{name: "critical boundary", lag: 30, want: LevelCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMySQLInstance(readOnlyInstanceWithLag(tt.lag))
			if summary.Replication.Level != tt.want {
				t.Fatalf("level = %q, want %q", summary.Replication.Level, tt.want)
			}
		})
	}
}

func TestMySQLServiceMetaUsesLatestSampleTime(t *testing.T) {
	now := time.Date(2026, time.August, 7, 8, 0, 0, 0, time.UTC)
	older := instanceWithChannels()
	older.ID = "fixture-mysql-older"
	older.CollectionTracked = true
	older.ReportedAt = now.Add(-2 * time.Minute)
	latest := instanceWithChannels()
	latest.ID = "fixture-mysql-latest"
	latest.CollectionTracked = false
	latest.ReportedAt = now.Add(-time.Minute)
	provider := &recordingMySQLProvider{snapshot: mysql.Snapshot{Instances: []mysql.Instance{older, latest}}}
	service := NewMySQL(provider, cache.New(func() time.Time { return now }), MySQLOptions{Clock: func() time.Time { return now }})

	_, meta, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if !meta.CollectedAt.Equal(latest.ReportedAt) {
		t.Fatalf("CollectedAt = %s, want latest sample %s", meta.CollectedAt, latest.ReportedAt)
	}
}

func TestMySQLSummaryMakesStoppedThreadCritical(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2)},
	)
	summary := summarizeMySQLInstance(instance)
	if summary.Status != LevelCritical ||
		summary.Replication.State != ReplicationThreadsStopped {
		t.Fatalf("instance = %#v", summary)
	}
}

func TestMySQLSummaryPreservesMaximumLagWhenThreadStopped(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2)},
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(11)},
	)
	summary := summarizeMySQLInstance(instance)
	if summary.Replication.State != ReplicationThreadsStopped ||
		summary.Replication.Level != LevelCritical ||
		summary.Replication.LagSeconds == nil ||
		*summary.Replication.LagSeconds != 11 {
		t.Fatalf("replication = %#v", summary.Replication)
	}
}

func TestMySQLSummaryAppliesRoleAndMissingReplicationSemantics(t *testing.T) {
	writable := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleWritable, Availability: mysql.AvailabilityUp})
	if writable.Replication.State != ReplicationNotConfigured || writable.Status != LevelNormal {
		t.Fatalf("writable = %#v", writable)
	}
	readOnly := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleReadOnly, Availability: mysql.AvailabilityUp})
	if readOnly.Replication.State != ReplicationUnknown || readOnly.Status != LevelUnknown {
		t.Fatalf("readOnly = %#v", readOnly)
	}
	unknownRole := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleUnknown, Availability: mysql.AvailabilityUp})
	if unknownRole.Replication.State != ReplicationUnknown || unknownRole.Status != LevelUnknown {
		t.Fatalf("unknown role = %#v", unknownRole)
	}
	unknownAvailability := summarizeMySQLInstance(mysql.Instance{
		Role: mysql.RoleWritable, Availability: mysql.AvailabilityUnknown,
	})
	if unknownAvailability.Status != LevelUnknown {
		t.Fatalf("unknown availability = %#v", unknownAvailability)
	}
}

func TestMySQLSummaryCalculatesOnlyValidConnectionUsageAndMaximumLag(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(8)},
	)
	instance.Connections = floatPointer(25)
	instance.MaxConnections = floatPointer(100)
	summary := summarizeMySQLInstance(instance)
	if summary.ConnectionUsagePercent == nil || *summary.ConnectionUsagePercent != 25 {
		t.Fatalf("connection usage = %#v", summary.ConnectionUsagePercent)
	}
	if summary.Replication.LagSeconds == nil || *summary.Replication.LagSeconds != 8 {
		t.Fatalf("replication lag = %#v", summary.Replication.LagSeconds)
	}
	instance.MaxConnections = floatPointer(0)
	if got := summarizeMySQLInstance(instance).ConnectionUsagePercent; got != nil {
		t.Fatalf("zero maximum produced usage %#v", got)
	}
}

func TestMySQLSummaryRejectsNonFiniteDerivedConnectionUsage(t *testing.T) {
	instance := mysql.Instance{
		Availability:   mysql.AvailabilityUp,
		Role:           mysql.RoleWritable,
		Connections:    floatPointer(math.MaxFloat64),
		MaxConnections: floatPointer(1),
	}

	if got := summarizeMySQLInstance(instance).ConnectionUsagePercent; got != nil {
		t.Fatalf("non-finite derived connection usage = %#v", got)
	}
}

func TestMySQLSummaryMakesIncompleteCriticalReplicationDataUnknown(t *testing.T) {
	instance := instanceWithChannels(mysql.ReplicationChannel{
		SQLRunning: boolPointer(true),
		LagSeconds: floatPointer(1),
	})
	summary := summarizeMySQLInstance(instance)
	if summary.Replication.State != ReplicationUnknown ||
		summary.Replication.Level != LevelUnknown ||
		summary.Status != LevelUnknown {
		t.Fatalf("instance = %#v", summary)
	}
}

func TestMySQLSummaryTreatsInvalidReplicationLagAsMissing(t *testing.T) {
	tests := []struct {
		name string
		lag  float64
	}{
		{name: "negative", lag: -1},
		{name: "nan", lag: math.NaN()},
		{name: "positive infinity", lag: math.Inf(1)},
		{name: "negative infinity", lag: math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMySQLInstance(readOnlyInstanceWithLag(tt.lag))
			if summary.Replication.State != ReplicationUnknown ||
				summary.Replication.Level != LevelUnknown ||
				summary.Replication.LagSeconds != nil ||
				summary.Status != LevelUnknown {
				t.Fatalf("instance = %#v", summary)
			}
			if _, err := json.Marshal(summary); err != nil {
				t.Fatalf("marshal summary: %v", err)
			}
		})
	}
}

func TestMySQLSummaryDoesNotThresholdOtherMetrics(t *testing.T) {
	instance := mysql.Instance{
		Availability:           mysql.AvailabilityUp,
		Role:                   mysql.RoleWritable,
		ThreadsRunning:         floatPointer(1_000_000),
		QPS:                    floatPointer(1_000_000),
		SlowQueriesPerSecond:   floatPointer(1_000_000),
		BufferPoolUsagePercent: floatPointer(100),
		BufferPoolSizeBytes:    floatPointer(1_000_000),
	}
	summary := summarizeMySQLInstance(instance)
	if summary.Status != LevelNormal {
		t.Fatalf("status = %q, want %q", summary.Status, LevelNormal)
	}
	if summary.ThreadsRunning == nil || *summary.ThreadsRunning != 1_000_000 ||
		summary.QPS == nil || *summary.QPS != 1_000_000 ||
		summary.SlowQueriesPerSecond == nil || *summary.SlowQueriesPerSecond != 1_000_000 ||
		summary.BufferPoolUsagePercent == nil || *summary.BufferPoolUsagePercent != 100 ||
		mysqlSummaryBufferPoolSizeBytes(t, summary) == nil || *mysqlSummaryBufferPoolSizeBytes(t, summary) != 1_000_000 {
		t.Fatalf("metrics = %#v", summary)
	}
}

func TestMySQLSummaryMakesUnavailableInstanceCritical(t *testing.T) {
	summary := summarizeMySQLInstance(mysql.Instance{
		Availability: mysql.AvailabilityDown,
		Role:         mysql.RoleWritable,
	})
	if summary.Status != LevelCritical {
		t.Fatalf("status = %q, want %q", summary.Status, LevelCritical)
	}
}

func TestMySQLServiceUsesObservedSampleProgressForCollectionLevel(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	sampleAt := now.Add(-time.Hour)
	instance := mysql.Instance{
		ID:                "fixture-instance",
		Name:              "fixture-instance",
		Address:           "192.0.2.10:3306",
		Availability:      mysql.AvailabilityUp,
		Role:              mysql.RoleWritable,
		CollectionTracked: true,
		Reporting:         true,
		ReportedAt:        sampleAt,
	}
	provider := &recordingMySQLProvider{snapshot: mysql.Snapshot{Instances: []mysql.Instance{instance}}}
	clock := &mysqlTestClock{now: now}
	svc := NewMySQL(provider, cache.New(clock.Now), MySQLOptions{
		CurrentMetricsTTL:  time.Second,
		CollectionInterval: 15 * time.Second,
		MaxStale:           5 * time.Minute,
		Clock:              clock.Now,
	})

	assertState := func(wantCollection, wantStatus Level, wantWarning, wantCritical int) {
		t.Helper()
		page, _, err := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("Instances() error = %v", err)
		}
		if len(page.Instances) != 1 ||
			page.Instances[0].CollectionLevel != wantCollection ||
			page.Instances[0].Status != wantStatus {
			t.Fatalf("instances = %#v", page.Instances)
		}
		overview, _, err := svc.Overview(context.Background())
		if err != nil {
			t.Fatalf("Overview() error = %v", err)
		}
		if overview.WarningInstances != wantWarning ||
			overview.CriticalInstances != wantCritical {
			t.Fatalf("overview = %#v", overview)
		}
	}

	assertState(LevelNormal, LevelNormal, 0, 0)

	clock.Advance(30 * time.Second)
	assertState(LevelWarning, LevelWarning, 1, 0)

	clock.Advance(45 * time.Second)
	assertState(LevelCritical, LevelCritical, 0, 1)

	instance.ReportedAt = sampleAt.Add(15 * time.Second)
	provider.snapshot = mysql.Snapshot{Instances: []mysql.Instance{instance}}
	clock.Advance(2 * time.Second)
	assertState(LevelNormal, LevelNormal, 0, 0)
}

func TestMySQLServiceSharesOneSnapshotAcrossOverviewAndList(t *testing.T) {
	provider := &recordingMySQLProvider{snapshot: fixtureMySQLSnapshot()}
	clock := &mysqlTestClock{now: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)}
	store := cache.New(clock.Now)
	svc := NewMySQL(provider, store, MySQLOptions{
		CurrentMetricsTTL: 15 * time.Second,
		MaxStale:          5 * time.Minute,
		Clock:             clock.Now,
	})
	if _, _, err := svc.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("MySQLSnapshot calls = %d, want 1", provider.calls)
	}
}

func TestMySQLServiceReturnsStaleSnapshotAfterUpstreamFailure(t *testing.T) {
	provider, clock, svc := newCachingMySQLService(fixtureMySQLSnapshot())
	if _, _, err := svc.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(16 * time.Second)
	provider.err = mysql.ErrUnavailable
	_, meta, err := svc.Overview(context.Background())
	if err != nil || !meta.Stale {
		t.Fatalf("meta = %#v, err = %v", meta, err)
	}
}

func TestMySQLServiceCachesSuccessfulEmptySnapshot(t *testing.T) {
	provider, _, svc := newCachingMySQLService(mysql.Snapshot{Instances: []mysql.Instance{}})
	overview, _, err := svc.Overview(context.Background())
	if err != nil || overview.Total != 0 || provider.calls != 1 {
		t.Fatalf("overview = %#v, calls = %d, err = %v", overview, provider.calls, err)
	}
	if _, _, err := svc.Overview(context.Background()); err != nil || provider.calls != 1 {
		t.Fatalf("second overview calls = %d, err = %v", provider.calls, err)
	}
}

func TestMySQLServiceReturnsIndependentSnapshotCopies(t *testing.T) {
	_, _, svc := newCachingMySQLService(fixtureMySQLSnapshot())
	first, _, err := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	first.Instances[0].Name = "mutated"
	*first.Instances[0].QPS = 999
	second, _, err := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if second.Instances[0].Name == "mutated" || *second.Instances[0].QPS == 999 {
		t.Fatal("cached snapshot was mutated by a caller")
	}
}

func TestMySQLServiceSnapshotDeepCopiesMetricsAndReplicationChannels(t *testing.T) {
	source := fixtureMySQLSnapshot()
	_, _, svc := newCachingMySQLService(source)
	first, _, err := svc.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	instance := &first.Instances[0]
	*instance.UptimeSeconds = 999
	*instance.Connections = 999
	*instance.MaxConnections = 999
	*instance.ThreadsRunning = 999
	*instance.QPS = 999
	*instance.SlowQueriesPerSecond = 999
	*instance.BufferPoolUsagePercent = 999
	*instance.BufferPoolSizeBytes = 999
	*instance.ReplicationChannels[0].IORunning = false
	*instance.ReplicationChannels[0].SQLRunning = false
	*instance.ReplicationChannels[0].LagSeconds = 999
	instance.ReplicationChannels = append(instance.ReplicationChannels, mysql.ReplicationChannel{})

	second, _, err := svc.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := second.Instances[0]
	if *got.UptimeSeconds == 999 ||
		*got.Connections == 999 ||
		*got.MaxConnections == 999 ||
		*got.ThreadsRunning == 999 ||
		*got.QPS == 999 ||
		*got.SlowQueriesPerSecond == 999 ||
		*got.BufferPoolUsagePercent == 999 ||
		*got.BufferPoolSizeBytes == 999 ||
		!*got.ReplicationChannels[0].IORunning ||
		!*got.ReplicationChannels[0].SQLRunning ||
		*got.ReplicationChannels[0].LagSeconds == 999 ||
		len(got.ReplicationChannels) != 1 {
		t.Fatalf("second snapshot was mutated: %#v", got)
	}
}

func TestMySQLOverviewCountsUnknownAsWarningRisk(t *testing.T) {
	overview, _, err := newMySQLServiceWithSnapshot(alertCategoryFixtureSnapshot()).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Total != 9 ||
		overview.Normal != 1 ||
		overview.Warning != 1 ||
		overview.Critical != 3 ||
		overview.Unknown != 4 ||
		overview.WarningInstances != 5 ||
		overview.CriticalInstances != 3 ||
		overview.AffectedInstances != 8 {
		t.Fatalf("overview = %#v", overview)
	}
}

func TestMySQLOverviewCountsAlertCategoriesIndependently(t *testing.T) {
	svc := newMySQLServiceWithSnapshot(alertCategoryFixtureSnapshot())
	overview, _, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := MySQLOverviewAlerts{
		Availability:       MySQLAlertCount{Warning: 1, Critical: 1},
		ReplicationThreads: MySQLAlertCount{Warning: 2, Critical: 1},
		ReplicationLag:     MySQLAlertCount{Warning: 1, Critical: 1},
		ReplicationData:    MySQLAlertCount{Warning: 3},
	}
	if overview.Alerts != want {
		t.Fatalf("alerts = %#v, want %#v", overview.Alerts, want)
	}
}

func TestMySQLOverviewClassifiesZeroReplicationChannelsByRole(t *testing.T) {
	base := mysql.Instance{Availability: mysql.AvailabilityUp}
	writable := base
	writable.ID = "fixture-writable"
	writable.Role = mysql.RoleWritable
	readOnly := base
	readOnly.ID = "fixture-read-only"
	readOnly.Role = mysql.RoleReadOnly
	unknown := base
	unknown.ID = "fixture-unknown"
	unknown.Role = mysql.RoleUnknown

	overview, _, err := newMySQLServiceWithSnapshot(mysql.Snapshot{
		Instances: []mysql.Instance{writable, readOnly, unknown},
	}).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := MySQLOverview{
		Total:             3,
		Normal:            1,
		Unknown:           2,
		AffectedInstances: 2,
		WarningInstances:  2,
		Alerts: MySQLOverviewAlerts{
			ReplicationThreads: MySQLAlertCount{Warning: 2},
			ReplicationData:    MySQLAlertCount{Warning: 2},
		},
	}
	if overview != want {
		t.Fatalf("overview = %#v, want %#v", overview, want)
	}
}

func TestMySQLInstancesSearchesOnlyAddress(t *testing.T) {
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Search: "192.0.2.11", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Instances) != 1 || page.Instances[0].Name != "fixture-mysql-b" {
		t.Fatalf("address search returned %#v", page)
	}

	page, _, err = fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Search: "fixture-host-c", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Instances) != 0 {
		t.Fatalf("hidden host search returned %#v", page)
	}
}

func TestMySQLInstancesDoesNotSearchInstanceName(t *testing.T) {
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Search: "fixture-mysql-a", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Instances) != 0 {
		t.Fatalf("page = %#v, want no results", page)
	}
}

func TestMySQLInstancesReturnsStableAvailableLabelsFromCompleteSnapshot(t *testing.T) {
	snapshot := fixtureMySQLSnapshot()
	duplicate := snapshot.Instances[1]
	duplicate.ID = mysql.StableInstanceID("fixture-host-d", duplicate.Name, "192.0.2.13:3306")
	duplicate.Address = "192.0.2.13:3306"
	duplicate.Host = "fixture-host-d"
	blank := snapshot.Instances[2]
	blank.ID = mysql.StableInstanceID("fixture-host-e", " ", "192.0.2.14:3306")
	blank.Name = " "
	blank.Address = "192.0.2.14:3306"
	blank.Host = "fixture-host-e"
	snapshot.Instances = append(snapshot.Instances, duplicate, blank)

	page, _, err := newMySQLServiceWithSnapshot(snapshot).Instances(context.Background(), MySQLQuery{
		Status:   LevelWarning,
		Role:     mysql.RoleReadOnly,
		Search:   "192.0.2.11",
		Label:    "fixture-mysql-b",
		Sort:     "status",
		Order:    "desc",
		Page:     2,
		PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fixture-mysql-a", "fixture-mysql-b", "fixture-mysql-c"}
	if !reflect.DeepEqual(page.AvailableLabels, want) {
		t.Fatalf("available labels = %#v, want %#v", page.AvailableLabels, want)
	}
}

func TestMySQLInstancesFiltersExactLabelAndKeepsSameNamedAddresses(t *testing.T) {
	snapshot := fixtureMySQLSnapshot()
	duplicate := snapshot.Instances[1]
	duplicate.ID = mysql.StableInstanceID("fixture-host-d", duplicate.Name, "192.0.2.13:3306")
	duplicate.Address = "192.0.2.13:3306"
	duplicate.Host = "fixture-host-d"
	snapshot.Instances = append(snapshot.Instances, duplicate)

	page, _, err := newMySQLServiceWithSnapshot(snapshot).Instances(context.Background(), MySQLQuery{
		Label: " fixture-mysql-b ", Search: "192.0.2", Sort: "instance", Order: "asc", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Instances) != 2 {
		t.Fatalf("page = %#v", page)
	}
	for _, item := range page.Instances {
		if item.Name != "fixture-mysql-b" {
			t.Fatalf("instance = %#v, want exact label match", item)
		}
	}
}

func TestMySQLInstancesFiltersStatusAndRole(t *testing.T) {
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Status: LevelWarning, Role: mysql.RoleReadOnly, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Instances) != 1 {
		t.Fatalf("page = %#v", page)
	}
	for _, instance := range page.Instances {
		if instance.Status != LevelWarning || instance.Role != mysql.RoleReadOnly {
			t.Fatalf("unexpected instance %#v", instance)
		}
	}
}

func TestMySQLInstancesDefaultsToAddressAscendingSort(t *testing.T) {
	snapshot := fixtureMySQLSnapshot()
	snapshot.Instances[0].Name = "fixture-z"
	snapshot.Instances[1].Name = "fixture-a"
	snapshot.Instances[2].Name = "fixture-m"
	page, _, err := newMySQLServiceWithSnapshot(snapshot).Instances(context.Background(), MySQLQuery{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"192.0.2.10:3306", "192.0.2.11:3306", "192.0.2.12:3306"} {
		if page.Instances[i].Address != want {
			t.Fatalf("instances[%d].Address = %q, want %q", i, page.Instances[i].Address, want)
		}
	}
}

func TestMySQLInstancesSortsMissingMetricsLastAndUsesStableTieBreak(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
			Sort: "qps", Order: order, Page: 1, PageSize: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		assertAvailableValuesBeforeMissing(t, page.Instances, func(item MySQLInstanceSummary) *float64 { return item.QPS })
		assertStableIDTieBreak(t, page.Instances)
	}
}

func TestMySQLInstancesSupportsAllMetricSortFields(t *testing.T) {
	tests := []struct {
		field string
		value func(MySQLInstanceSummary) *float64
	}{
		{field: "connections", value: func(item MySQLInstanceSummary) *float64 { return item.ConnectionUsagePercent }},
		{field: "threads_running", value: func(item MySQLInstanceSummary) *float64 { return item.ThreadsRunning }},
		{field: "qps", value: func(item MySQLInstanceSummary) *float64 { return item.QPS }},
		{field: "slow_queries", value: func(item MySQLInstanceSummary) *float64 { return item.SlowQueriesPerSecond }},
		{field: "buffer_pool", value: func(item MySQLInstanceSummary) *float64 { return item.BufferPoolUsagePercent }},
		{field: "replication_lag", value: func(item MySQLInstanceSummary) *float64 { return item.Replication.LagSeconds }},
		{field: "uptime", value: func(item MySQLInstanceSummary) *float64 { return item.UptimeSeconds }},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			for _, order := range []string{"asc", "desc"} {
				page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
					Sort: tt.field, Order: order, Page: 1, PageSize: 100,
				})
				if err != nil {
					t.Fatal(err)
				}
				assertAvailableValuesBeforeMissing(t, page.Instances, tt.value)
			}
		})
	}
}

func TestMySQLInstancesSortsInstanceAndStatusWithStableIDTies(t *testing.T) {
	for _, field := range []string{"instance", "status"} {
		for _, order := range []string{"asc", "desc"} {
			page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
				Sort: field, Order: order, Page: 1, PageSize: 100,
			})
			if err != nil {
				t.Fatalf("sort %s %s: %v", field, order, err)
			}
			if len(page.Instances) != 3 {
				t.Fatalf("sort %s %s returned %#v", field, order, page)
			}
			if field == "status" {
				assertEqualStatusUsesStableIDTieBreak(t, page.Instances)
			}
		}
	}
}

func TestMySQLInstancesRejectsUnsupportedQueryBeforeLoadingSnapshot(t *testing.T) {
	tests := []struct {
		name  string
		query MySQLQuery
	}{
		{name: "page", query: MySQLQuery{Page: 0, PageSize: 20}},
		{name: "page size", query: MySQLQuery{Page: 1, PageSize: 1}},
		{name: "status", query: MySQLQuery{Status: "degraded", Page: 1, PageSize: 20}},
		{name: "role", query: MySQLQuery{Role: "replica", Page: 1, PageSize: 20}},
		{name: "sort", query: MySQLQuery{Sort: "address", Page: 1, PageSize: 20}},
		{name: "order", query: MySQLQuery{Order: "sideways", Page: 1, PageSize: 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _, svc := newCachingMySQLService(fixtureMySQLSnapshot())
			_, _, err := svc.Instances(context.Background(), tt.query)
			if !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("error = %v, want ErrInvalidQuery", err)
			}
			if provider.calls != 0 {
				t.Fatalf("provider calls = %d, want 0", provider.calls)
			}
		})
	}
}

func TestMySQLInstancesAcceptsOnlySupportedPageSizesAndHandlesOverflow(t *testing.T) {
	for _, pageSize := range []int{0, 1, 19, 21, 101} {
		_, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{Page: 1, PageSize: pageSize})
		if !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("page size %d error = %v", pageSize, err)
		}
	}
	for _, pageSize := range []int{20, 50, 100} {
		if _, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{Page: 1, PageSize: pageSize}); err != nil {
			t.Fatalf("page size %d error = %v", pageSize, err)
		}
	}
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Page: math.MaxInt, PageSize: 20,
	})
	if err != nil || len(page.Instances) != 0 || page.Total != 3 {
		t.Fatalf("overflow page = %#v, err = %v", page, err)
	}
}

func TestMySQLInstancesPaginatesAfterFilteringAndSorting(t *testing.T) {
	svc := newMySQLServiceWithSnapshot(pagedMySQLFixtureSnapshot())
	first, _, err := svc.Instances(context.Background(), MySQLQuery{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.Instances(context.Background(), MySQLQuery{
		Page: 2, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 21 || len(first.Instances) != 20 ||
		second.Total != 21 || len(second.Instances) != 1 ||
		second.Instances[0].Name != "fixture-mysql-21" {
		t.Fatalf("first = %#v, second = %#v", first, second)
	}
}

func boolPointer(value bool) *bool        { return &value }
func floatPointer(value float64) *float64 { return &value }

func mysqlSummaryBufferPoolSizeBytes(t *testing.T, summary MySQLInstanceSummary) *float64 {
	t.Helper()
	field := reflect.ValueOf(summary).FieldByName("BufferPoolSizeBytes")
	if !field.IsValid() {
		t.Fatal("MySQL summary does not expose BufferPoolSizeBytes")
	}
	value, ok := field.Interface().(*float64)
	if !ok {
		t.Fatalf("BufferPoolSizeBytes type = %s, want *float64", field.Type())
	}
	return value
}

type mysqlTestClock struct{ now time.Time }

func (c *mysqlTestClock) Now() time.Time              { return c.now }
func (c *mysqlTestClock) Advance(delta time.Duration) { c.now = c.now.Add(delta) }

func newCachingMySQLService(snapshot mysql.Snapshot) (*recordingMySQLProvider, *mysqlTestClock, *MySQLService) {
	provider := &recordingMySQLProvider{snapshot: snapshot}
	clock := &mysqlTestClock{now: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)}
	svc := NewMySQL(provider, cache.New(clock.Now), MySQLOptions{
		CurrentMetricsTTL: 15 * time.Second,
		MaxStale:          5 * time.Minute,
		Clock:             clock.Now,
	})
	return provider, clock, svc
}

func newMySQLServiceWithSnapshot(snapshot mysql.Snapshot) *MySQLService {
	_, _, svc := newCachingMySQLService(snapshot)
	return svc
}

func fixtureMySQLService() *MySQLService {
	return newMySQLServiceWithSnapshot(fixtureMySQLSnapshot())
}

func fixtureMySQLSnapshot() mysql.Snapshot {
	first := instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2),
	})
	first.UptimeSeconds = floatPointer(3600)
	first.Connections = floatPointer(20)
	first.MaxConnections = floatPointer(100)
	first.ThreadsRunning = floatPointer(3)
	first.QPS = floatPointer(10)
	first.SlowQueriesPerSecond = floatPointer(0.5)
	first.BufferPoolUsagePercent = floatPointer(70)
	first.BufferPoolSizeBytes = floatPointer(1_073_741_824)

	second := instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(8),
	})
	second.ID = mysql.StableInstanceID("fixture-host-b", "fixture-mysql-b", "192.0.2.11:3306")
	second.Name = "fixture-mysql-b"
	second.Address = "192.0.2.11:3306"
	second.Host = "fixture-host-b"
	second.UptimeSeconds = floatPointer(7200)
	second.Connections = floatPointer(50)
	second.MaxConnections = floatPointer(100)
	second.ThreadsRunning = floatPointer(8)
	second.QPS = floatPointer(10)
	second.SlowQueriesPerSecond = floatPointer(1)
	second.BufferPoolUsagePercent = floatPointer(80)

	third := mysql.Instance{
		ID:           mysql.StableInstanceID("fixture-host-c", "fixture-mysql-c", "192.0.2.12:3306"),
		Name:         "fixture-mysql-c",
		Address:      "192.0.2.12:3306",
		Host:         "fixture-host-c",
		Availability: mysql.AvailabilityUp,
		Role:         mysql.RoleWritable,
	}
	return mysql.Snapshot{Instances: []mysql.Instance{first, second, third}}
}

func pagedMySQLFixtureSnapshot() mysql.Snapshot {
	instances := make([]mysql.Instance, 21)
	for i := range instances {
		name := fmt.Sprintf("fixture-mysql-%02d", i+1)
		instances[i] = mysql.Instance{
			ID:           name,
			Name:         name,
			Address:      fmt.Sprintf("192.0.2.%d:3306", i+1),
			Host:         fmt.Sprintf("fixture-host-%02d", i+1),
			Availability: mysql.AvailabilityUp,
			Role:         mysql.RoleWritable,
		}
	}
	return mysql.Snapshot{Instances: instances}
}

func assertAvailableValuesBeforeMissing(
	t *testing.T,
	items []MySQLInstanceSummary,
	value func(MySQLInstanceSummary) *float64,
) {
	t.Helper()
	seenMissing := false
	for _, item := range items {
		if value(item) == nil {
			seenMissing = true
		} else if seenMissing {
			t.Fatal("available metric sorted after a missing metric")
		}
	}
}

func assertStableIDTieBreak(t *testing.T, items []MySQLInstanceSummary) {
	t.Helper()
	for i := 1; i < len(items); i++ {
		left := items[i-1]
		right := items[i]
		if left.QPS != nil && right.QPS != nil && *left.QPS == *right.QPS && left.ID > right.ID {
			t.Fatalf("equal QPS IDs not ascending: %q before %q", left.ID, right.ID)
		}
	}
}

func assertEqualStatusUsesStableIDTieBreak(t *testing.T, items []MySQLInstanceSummary) {
	t.Helper()
	for i := 1; i < len(items); i++ {
		left := items[i-1]
		right := items[i]
		if left.Status == right.Status && left.ID > right.ID {
			t.Fatalf("equal status IDs not ascending: %q before %q", left.ID, right.ID)
		}
	}
}

func alertCategoryFixtureSnapshot() mysql.Snapshot {
	normal := mysql.Instance{
		ID:           "fixture-normal",
		Name:         "fixture-normal",
		Availability: mysql.AvailabilityUp,
		Role:         mysql.RoleWritable,
	}
	unknownAvailability := normal
	unknownAvailability.ID = "fixture-availability-unknown"
	unknownAvailability.Name = "fixture-availability-unknown"
	unknownAvailability.Availability = mysql.AvailabilityUnknown
	down := normal
	down.ID = "fixture-down"
	down.Name = "fixture-down"
	down.Availability = mysql.AvailabilityDown

	stopped := instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2),
	})
	stopped.ID = "fixture-stopped"
	stopped.Name = "fixture-stopped"
	warningLag := readOnlyInstanceWithLag(8)
	warningLag.ID = "fixture-lag-warning"
	warningLag.Name = "fixture-lag-warning"
	criticalLag := readOnlyInstanceWithLag(35)
	criticalLag.ID = "fixture-lag-critical"
	criticalLag.Name = "fixture-lag-critical"
	incompleteThreads := instanceWithChannels(mysql.ReplicationChannel{
		SQLRunning: boolPointer(true), LagSeconds: floatPointer(1),
	})
	incompleteThreads.ID = "fixture-threads-incomplete"
	incompleteThreads.Name = "fixture-threads-incomplete"
	missingLag := instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true),
	})
	missingLag.ID = "fixture-lag-missing"
	missingLag.Name = "fixture-lag-missing"
	unknownRole := mysql.Instance{
		ID:           "fixture-role-unknown",
		Name:         "fixture-role-unknown",
		Availability: mysql.AvailabilityUp,
		Role:         mysql.RoleUnknown,
	}

	return mysql.Snapshot{Instances: []mysql.Instance{
		normal,
		unknownAvailability,
		down,
		stopped,
		warningLag,
		criticalLag,
		incompleteThreads,
		missingLag,
		unknownRole,
	}}
}

func readOnlyInstanceWithLag(lag float64) mysql.Instance {
	return instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(lag),
	})
}

func instanceWithChannels(channels ...mysql.ReplicationChannel) mysql.Instance {
	return mysql.Instance{
		ID:                  mysql.StableInstanceID("fixture-host-a", "fixture-mysql-a", "192.0.2.10:3306"),
		Name:                "fixture-mysql-a",
		Address:             "192.0.2.10:3306",
		Host:                "fixture-host-a",
		Availability:        mysql.AvailabilityUp,
		Role:                mysql.RoleReadOnly,
		ReplicationChannels: channels,
	}
}

type recordingMySQLProvider struct {
	snapshot mysql.Snapshot
	err      error
	calls    int
}

func (p *recordingMySQLProvider) MySQLSnapshot(context.Context) (mysql.Snapshot, error) {
	p.calls++
	return p.snapshot, p.err
}
