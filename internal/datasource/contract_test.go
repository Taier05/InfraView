package datasource_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/adapters/nightingale"
	"github.com/Taier05/InfraView/internal/datasource"
)

func RunContract(t *testing.T, provider datasource.Provider) {
	t.Helper()

	ctx := context.Background()
	hosts, err := provider.ListHosts(ctx)
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	if len(hosts) == 0 {
		t.Fatal("ListHosts() returned an empty inventory")
	}

	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host.ID == "" {
			t.Fatal("ListHosts() returned a host with an empty ID")
		}
		if _, exists := seen[host.ID]; exists {
			t.Fatalf("ListHosts() returned duplicate ID %q", host.ID)
		}
		seen[host.ID] = struct{}{}
	}

	again, err := provider.ListHosts(ctx)
	if err != nil {
		t.Fatalf("second ListHosts() error = %v", err)
	}
	if !reflect.DeepEqual(hosts, again) {
		t.Fatal("ListHosts() returned unstable inventory")
	}

	gotHost, err := provider.GetHost(ctx, hosts[0].ID)
	if err != nil {
		t.Fatalf("GetHost(%q) error = %v", hosts[0].ID, err)
	}
	if !reflect.DeepEqual(gotHost, hosts[0]) {
		t.Fatalf("GetHost(%q) = %#v, want %#v", hosts[0].ID, gotHost, hosts[0])
	}

	batchSize := min(3, len(hosts))
	hostIDs := make([]string, batchSize)
	for i := range batchSize {
		hostIDs[i] = hosts[i].ID
	}
	metrics, err := provider.GetCurrentMetrics(ctx, hostIDs)
	if err != nil {
		t.Fatalf("GetCurrentMetrics() error = %v", err)
	}
	if len(metrics) != len(hostIDs) {
		t.Fatalf("GetCurrentMetrics() returned %d hosts, want %d", len(metrics), len(hostIDs))
	}
	for _, hostID := range hostIDs {
		current, ok := metrics[hostID]
		if !ok {
			t.Fatalf("GetCurrentMetrics() omitted host %q", hostID)
		}
		if current.Timestamp.IsZero() {
			t.Fatalf("GetCurrentMetrics()[%q] has a zero timestamp", hostID)
		}
	}

	start := time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	series, err := provider.QueryRange(ctx, datasource.RangeRequest{
		HostIDs: []string{hosts[0].ID},
		Metric:  datasource.MetricCPUUsage,
		From:    start,
		To:      end,
		Step:    time.Minute,
	})
	if err != nil {
		t.Fatalf("QueryRange() error = %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("QueryRange() returned %d series, want 1", len(series))
	}
	if len(series[0].Points) != 61 {
		t.Fatalf("QueryRange() returned %d points, want 61", len(series[0].Points))
	}
	for i, point := range series[0].Points {
		wantTimestamp := start.Add(time.Duration(i) * time.Minute)
		if !point.Timestamp.Equal(wantTimestamp) {
			t.Fatalf("point %d timestamp = %s, want %s", i, point.Timestamp, wantTimestamp)
		}
		if point.Value == nil {
			t.Fatalf("point %d has a nil value", i)
		}
	}

	if _, err := provider.GetHost(ctx, "unknown-host"); !errors.Is(err, datasource.ErrNotFound) {
		t.Fatalf("GetHost(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestMockProviderContract(t *testing.T) {
	fixed := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	RunContract(t, mock.New(20, func() time.Time { return fixed }))
}

func TestNightingaleProviderIsNotConfigured(t *testing.T) {
	provider := nightingale.New()
	ctx := context.Background()
	request := datasource.RangeRequest{}

	checks := []struct {
		name string
		call func() error
	}{
		{name: "Health", call: func() error { _, err := provider.Health(ctx); return err }},
		{name: "ListHosts", call: func() error { _, err := provider.ListHosts(ctx); return err }},
		{name: "GetHost", call: func() error { _, err := provider.GetHost(ctx, "host"); return err }},
		{name: "GetCurrentMetrics", call: func() error { _, err := provider.GetCurrentMetrics(ctx, []string{"host"}); return err }},
		{name: "QueryRange", call: func() error { _, err := provider.QueryRange(ctx, request); return err }},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); !errors.Is(err, datasource.ErrNotConfigured) {
				t.Fatalf("error = %v, want ErrNotConfigured", err)
			}
		})
	}
}
