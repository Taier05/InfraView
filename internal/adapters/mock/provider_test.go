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

	firstHealth, firstHealthErr := provider.Health(ctx)
	secondHealth, secondHealthErr := provider.Health(ctx)
	assertDeepEqual(t, "Health result", []any{firstHealth, firstHealthErr}, []any{secondHealth, secondHealthErr})

	firstHosts, firstHostsErr := provider.ListHosts(ctx)
	secondHosts, secondHostsErr := provider.ListHosts(ctx)
	assertDeepEqual(t, "ListHosts result", []any{firstHosts, firstHostsErr}, []any{secondHosts, secondHostsErr})

	firstMetrics, firstMetricsErr := provider.GetCurrentMetrics(ctx, hostIDs)
	secondMetrics, secondMetricsErr := provider.GetCurrentMetrics(ctx, hostIDs)
	assertDeepEqual(t, "GetCurrentMetrics result", []any{firstMetrics, firstMetricsErr}, []any{secondMetrics, secondMetricsErr})

	firstSeries, firstSeriesErr := provider.QueryRange(ctx, request)
	secondSeries, secondSeriesErr := provider.QueryRange(ctx, request)
	assertDeepEqual(t, "QueryRange result", []any{firstSeries, firstSeriesErr}, []any{secondSeries, secondSeriesErr})
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
