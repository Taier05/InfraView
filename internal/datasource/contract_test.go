package datasource_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/adapters/nightingale"
	"github.com/Taier05/InfraView/internal/datasource"
)

func TestHostStatusValues(t *testing.T) {
	statuses := map[datasource.HostStatus]string{
		datasource.StatusOnline:  "online",
		datasource.StatusOffline: "offline",
		datasource.StatusUnknown: "unknown",
	}
	for status, want := range statuses {
		if string(status) != want {
			t.Fatalf("status = %q, want %q", status, want)
		}
	}
}

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

	firstIDs := collectHostIDs(t, hosts)

	again, err := provider.ListHosts(ctx)
	if err != nil {
		t.Fatalf("second ListHosts() error = %v", err)
	}
	againIDs := collectHostIDs(t, again)
	if len(firstIDs) != len(againIDs) {
		t.Fatalf("ListHosts() returned %d stable IDs, then %d", len(firstIDs), len(againIDs))
	}
	for hostID := range firstIDs {
		if _, ok := againIDs[hostID]; !ok {
			t.Fatalf("ListHosts() did not preserve host ID %q", hostID)
		}
	}

	gotHost, err := provider.GetHost(ctx, hosts[0].ID)
	if err != nil {
		t.Fatalf("GetHost(%q) error = %v", hosts[0].ID, err)
	}
	if gotHost.ID != hosts[0].ID {
		t.Fatalf("GetHost(%q) returned ID %q", hosts[0].ID, gotHost.ID)
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

	aggregate, err := provider.QueryAggregateRange(ctx, datasource.AggregateRangeRequest{
		Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
		Start: start,
		End:   end,
		Step:  time.Minute,
	})
	if err != nil {
		t.Fatalf("QueryAggregateRange() error = %v", err)
	}
	if len(aggregate) != 2 {
		t.Fatalf("QueryAggregateRange() returned %d series, want 2", len(aggregate))
	}
	for i, metric := range []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage} {
		if aggregate[i].HostID != "" || aggregate[i].Metric != metric {
			t.Fatalf("aggregate series %d = %#v", i, aggregate[i])
		}
		if len(aggregate[i].Points) != 61 {
			t.Fatalf("aggregate %s points = %d, want 61", metric, len(aggregate[i].Points))
		}
		for pointIndex, point := range aggregate[i].Points {
			wantTimestamp := start.Add(time.Duration(pointIndex) * time.Minute)
			if !point.Timestamp.Equal(wantTimestamp) || point.Value == nil {
				t.Fatalf("aggregate %s point %d = %#v, want timestamp %s and value", metric, pointIndex, point, wantTimestamp)
			}
		}
	}

	if _, err := provider.GetHost(ctx, "unknown-host"); !errors.Is(err, datasource.ErrNotFound) {
		t.Fatalf("GetHost(unknown) error = %v, want ErrNotFound", err)
	}
}

func collectHostIDs(t *testing.T, hosts []datasource.Host) map[string]struct{} {
	t.Helper()
	ids := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host.ID == "" {
			t.Fatal("ListHosts() returned a host with an empty ID")
		}
		if _, exists := ids[host.ID]; exists {
			t.Fatalf("ListHosts() returned duplicate ID %q", host.ID)
		}
		ids[host.ID] = struct{}{}
	}
	return ids
}

func TestMockProviderContract(t *testing.T) {
	fixed := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	RunContract(t, mock.New(20, func() time.Time { return fixed }))
}

func TestProviderContractAllowsDynamicHostFields(t *testing.T) {
	fixed := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	RunContract(t, &dynamicHostProvider{
		Provider: mock.New(3, func() time.Time { return fixed }),
	})
}

type dynamicHostProvider struct {
	datasource.Provider
	listCalls int
	getCalls  int
}

func (p *dynamicHostProvider) ListHosts(ctx context.Context) ([]datasource.Host, error) {
	hosts, err := p.Provider.ListHosts(ctx)
	if err != nil {
		return nil, err
	}
	p.listCalls++
	for i := range hosts {
		hosts[i].StatusTime = hosts[i].StatusTime.Add(time.Duration(p.listCalls) * time.Minute)
		hosts[i].Uptime += time.Duration(p.listCalls) * time.Minute
	}
	if p.listCalls%2 == 0 {
		for left, right := 0, len(hosts)-1; left < right; left, right = left+1, right-1 {
			hosts[left], hosts[right] = hosts[right], hosts[left]
		}
	}
	return hosts, nil
}

func (p *dynamicHostProvider) GetHost(ctx context.Context, hostID string) (datasource.Host, error) {
	host, err := p.Provider.GetHost(ctx, hostID)
	if err != nil {
		return datasource.Host{}, err
	}
	p.getCalls++
	dynamicOffset := time.Hour + time.Duration(p.getCalls)*time.Minute
	host.StatusTime = host.StatusTime.Add(dynamicOffset)
	host.Uptime += dynamicOffset
	return host, nil
}

func TestNightingaleProviderIsNotConfigured(t *testing.T) {
	provider := nightingale.New(nightingale.Options{})
	ctx := context.Background()
	request := datasource.RangeRequest{}
	aggregateRequest := datasource.AggregateRangeRequest{}

	checks := []struct {
		name string
		call func() error
	}{
		{name: "Health", call: func() error { _, err := provider.Health(ctx); return err }},
		{name: "ListHosts", call: func() error { _, err := provider.ListHosts(ctx); return err }},
		{name: "GetHost", call: func() error { _, err := provider.GetHost(ctx, "host"); return err }},
		{name: "GetCurrentMetrics", call: func() error { _, err := provider.GetCurrentMetrics(ctx, []string{"host"}); return err }},
		{name: "QueryRange", call: func() error { _, err := provider.QueryRange(ctx, request); return err }},
		{name: "QueryAggregateRange", call: func() error { _, err := provider.QueryAggregateRange(ctx, aggregateRequest); return err }},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); !errors.Is(err, datasource.ErrNotConfigured) {
				t.Fatalf("error = %v, want ErrNotConfigured", err)
			}
		})
	}
}
