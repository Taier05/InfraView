package mock_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/datasource"
)

func TestProviderBuildsStableInventory(t *testing.T) {
	fixed := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	provider := mock.New(34, func() time.Time { return fixed })

	hosts, err := provider.ListHosts(context.Background())
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	if len(hosts) != 34 {
		t.Fatalf("ListHosts() returned %d hosts, want 34", len(hosts))
	}

	first := hosts[0]
	if first.ID != "mock-host-001" || first.Name != "linux-001" || first.IP != "192.0.2.1" {
		t.Fatalf("first host = %#v", first)
	}
	if first.OS != "linux" || first.Status != datasource.StatusOnline {
		t.Fatalf("first host metadata = %#v", first)
	}
	if !first.StatusTime.Equal(fixed) || first.Uptime <= 0 {
		t.Fatalf("first host timing = %#v", first)
	}
	if hosts[16].Status != datasource.StatusOffline || hosts[33].Status != datasource.StatusOffline {
		t.Fatalf("hosts 17 and 34 must be offline: %q, %q", hosts[16].Status, hosts[33].Status)
	}
	if hosts[15].Status != datasource.StatusOnline {
		t.Fatalf("host 16 status = %q, want online", hosts[15].Status)
	}
}

func TestProviderIsDeterministicWithFixedClock(t *testing.T) {
	fixed := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	provider := mock.New(3, func() time.Time { return fixed })
	ctx := context.Background()
	hostIDs := []string{"mock-host-001", "mock-host-002"}
	request := datasource.RangeRequest{
		HostIDs: hostIDs,
		Metric:  datasource.MetricNetworkReceiveBytesPerSecond,
		From:    fixed.Add(-10 * time.Minute),
		To:      fixed,
		Step:    time.Minute,
	}
	aggregateRequest := datasource.AggregateRangeRequest{
		Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
		Start: fixed.Add(-10 * time.Minute),
		End:   fixed,
		Step:  time.Minute,
	}

	firstHealth, firstHealthErr := provider.Health(ctx)
	secondHealth, secondHealthErr := provider.Health(ctx)
	assertDeepEqual(t, "Health result", []any{firstHealth, firstHealthErr}, []any{secondHealth, secondHealthErr})

	firstHosts, firstHostsErr := provider.ListHosts(ctx)
	secondHosts, secondHostsErr := provider.ListHosts(ctx)
	assertDeepEqual(t, "ListHosts result", []any{firstHosts, firstHostsErr}, []any{secondHosts, secondHostsErr})

	firstHost, firstHostErr := provider.GetHost(ctx, hostIDs[0])
	secondHost, secondHostErr := provider.GetHost(ctx, hostIDs[0])
	assertDeepEqual(t, "GetHost result", []any{firstHost, firstHostErr}, []any{secondHost, secondHostErr})

	firstMetrics, firstMetricsErr := provider.GetCurrentMetrics(ctx, hostIDs)
	secondMetrics, secondMetricsErr := provider.GetCurrentMetrics(ctx, hostIDs)
	assertDeepEqual(t, "GetCurrentMetrics result", []any{firstMetrics, firstMetricsErr}, []any{secondMetrics, secondMetricsErr})

	firstSeries, firstSeriesErr := provider.QueryRange(ctx, request)
	secondSeries, secondSeriesErr := provider.QueryRange(ctx, request)
	assertDeepEqual(t, "QueryRange result", []any{firstSeries, firstSeriesErr}, []any{secondSeries, secondSeriesErr})

	firstAggregate, firstAggregateErr := provider.QueryAggregateRange(ctx, aggregateRequest)
	secondAggregate, secondAggregateErr := provider.QueryAggregateRange(ctx, aggregateRequest)
	assertDeepEqual(t, "QueryAggregateRange result", []any{firstAggregate, firstAggregateErr}, []any{secondAggregate, secondAggregateErr})
}

func TestProviderAggregatesAtMostOneHundredConfiguredHosts(t *testing.T) {
	fixed := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	provider := mock.New(150, func() time.Time { return fixed })
	boundedProvider := mock.New(100, func() time.Time { return fixed })
	request := datasource.AggregateRangeRequest{
		Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
		Start: fixed,
		End:   fixed,
		Step:  time.Minute,
	}

	series, err := provider.QueryAggregateRange(context.Background(), request)
	if err != nil {
		t.Fatalf("QueryAggregateRange() error = %v", err)
	}
	if len(series) != 2 || len(series[0].Points) != 1 || len(series[1].Points) != 1 {
		t.Fatalf("aggregate series = %#v", series)
	}
	boundedSeries, err := boundedProvider.QueryAggregateRange(context.Background(), request)
	if err != nil {
		t.Fatalf("bounded QueryAggregateRange() error = %v", err)
	}
	assertDeepEqual(t, "aggregate host cap", series, boundedSeries)
	for _, candidate := range series {
		if candidate.HostID != "" || candidate.Points[0].Value == nil || *candidate.Points[0].Value < 0 || *candidate.Points[0].Value > 100 {
			t.Fatalf("aggregate series = %#v", candidate)
		}
	}
}

func TestProviderReturnsBoundedCurrentPercentages(t *testing.T) {
	fixed := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	provider := mock.New(1, func() time.Time { return fixed })

	metrics, err := provider.GetCurrentMetrics(context.Background(), []string{"mock-host-001"})
	if err != nil {
		t.Fatalf("GetCurrentMetrics() error = %v", err)
	}
	current := metrics["mock-host-001"]
	for name, value := range map[string]*float64{
		"cpu":    current.CPUUsage,
		"memory": current.MemoryUsage,
	} {
		if value == nil || *value < 0 || *value > 100 {
			t.Fatalf("%s value = %v, want a non-nil value in [0, 100]", name, value)
		}
	}
	if len(current.Filesystems) == 0 || current.Filesystems[0].Usage == nil {
		t.Fatalf("filesystems = %#v, want usage data", current.Filesystems)
	}
	if usage := *current.Filesystems[0].Usage; usage < 0 || usage > 100 {
		t.Fatalf("filesystem usage = %v, want value in [0, 100]", usage)
	}
}

func assertDeepEqual(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs:\n got: %#v\nwant: %#v", name, got, want)
	}
}
