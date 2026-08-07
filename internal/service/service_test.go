package service_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/service"
)

func TestOverviewCountsHostsAndAveragesAvailableMetrics(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	overview, meta, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.Total != 3 || overview.Online != 1 || overview.Offline != 1 || overview.Unknown != 1 {
		t.Fatalf("host counts = %#v", overview)
	}
	if overview.Alerts.AffectedHosts != 2 || overview.Alerts.WarningHosts != 1 || overview.Alerts.CriticalHosts != 1 {
		t.Fatalf("host alerts = %#v", overview.Alerts)
	}
	assertMetric(t, "CPU average", overview.CPUAverage, 20, service.LevelNormal)
	assertMetric(t, "memory average", overview.MemoryAverage, 50, service.LevelNormal)
	if meta.Stale || !meta.CollectedAt.Equal(clock.Now()) {
		t.Fatalf("meta = %#v", meta)
	}
	if provider.currentCalls != 1 {
		t.Fatalf("GetCurrentMetrics() calls = %d, want 1", provider.currentCalls)
	}
	if got, want := provider.currentRequests[0], []string{"h1", "h2", "h3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentMetrics() IDs = %v, want %v", got, want)
	}
	if len(provider.aggregateRequests) != 1 {
		t.Fatalf("QueryAggregateRange() calls = %d, want 1", len(provider.aggregateRequests))
	}
	if got, want := provider.aggregateRequests[0].Keys, []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage}; !reflect.DeepEqual(got, want) {
		t.Fatalf("aggregate keys = %v, want %v", got, want)
	}
	if len(overview.Trends) != 2 {
		t.Fatalf("overview trends = %#v", overview.Trends)
	}
	for i, key := range []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage} {
		if overview.Trends[i].Key != key || overview.Trends[i].Unit != "%" || len(overview.Trends[i].Points) == 0 || len(overview.Trends[i].Points) > 600 {
			t.Fatalf("trend %d = %#v", i, overview.Trends[i])
		}
	}
}

func TestHostPageMetaUsesLatestMetricSampleTime(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	older := clock.Now().Add(-2 * time.Minute)
	latest := clock.Now().Add(-time.Minute)
	provider.metrics["h1"] = datasource.CurrentMetrics{Timestamp: older}
	provider.metrics["h2"] = datasource.CurrentMetrics{Timestamp: latest}
	provider.metrics["h3"] = datasource.CurrentMetrics{}
	svc := newService(provider, clock)

	_, meta, err := svc.Hosts(context.Background(), service.HostQuery{Sort: "name", Order: "asc", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	if !meta.CollectedAt.Equal(latest) {
		t.Fatalf("CollectedAt = %s, want latest sample %s", meta.CollectedAt, latest)
	}
}

func TestOverviewSummarizesMetricAlertsAndUsesHighestHostLevel(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {
			Timestamp:                     clock.Now(),
			CPUUsage:                      float64Pointer(85),
			MemoryUsage:                   float64Pointer(95),
			NetworkReceiveBytesPerSecond:  float64Pointer(110 * 1024 * 1024),
			NetworkTransmitBytesPerSecond: float64Pointer(90 * 1024 * 1024),
		},
		"h2": {
			Timestamp:     clock.Now(),
			IOBusyPercent: float64Pointer(85),
		},
		"h3": {Timestamp: clock.Now()},
	}
	svc := newService(provider, clock)

	overview, _, err := svc.Overview(context.Background(), "24h")
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.Alerts.AffectedHosts != 3 || overview.Alerts.WarningHosts != 1 || overview.Alerts.CriticalHosts != 2 {
		t.Fatalf("host alerts = %#v", overview.Alerts)
	}
	if overview.Alerts.CPU != (service.AlertCount{Warning: 1}) {
		t.Fatalf("CPU alerts = %#v", overview.Alerts.CPU)
	}
	if overview.Alerts.Memory != (service.AlertCount{Critical: 1}) {
		t.Fatalf("memory alerts = %#v", overview.Alerts.Memory)
	}
	if overview.Alerts.IO != (service.AlertCount{Warning: 1}) {
		t.Fatalf("IO alerts = %#v", overview.Alerts.IO)
	}
	if overview.Alerts.Network != (service.AlertCount{Critical: 1}) {
		t.Fatalf("network alerts = %#v", overview.Alerts.Network)
	}
}

func TestServiceDefaultsCurrentMetricsTTLToFifteenSeconds(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := service.New(provider, cache.New(clock.Now), service.Options{
		InventoryTTL: time.Minute,
		RangeTTL:     time.Minute,
		HealthTTL:    15 * time.Second,
		MaxStale:     5 * time.Minute,
		Clock:        clock.Now,
	})
	query := service.HostQuery{Sort: "name", Order: "asc", Page: 1, PageSize: 20}

	if _, _, err := svc.Hosts(context.Background(), query); err != nil {
		t.Fatalf("first Hosts() error = %v", err)
	}
	clock.Advance(14 * time.Second)
	if _, _, err := svc.Hosts(context.Background(), query); err != nil {
		t.Fatalf("Hosts() before default TTL error = %v", err)
	}
	if provider.currentCalls != 1 {
		t.Fatalf("current metric calls before 15s = %d, want 1", provider.currentCalls)
	}
	clock.Advance(2 * time.Second)
	if _, _, err := svc.Hosts(context.Background(), query); err != nil {
		t.Fatalf("Hosts() after default TTL error = %v", err)
	}
	if provider.currentCalls != 2 {
		t.Fatalf("current metric calls after 15s = %d, want 2", provider.currentCalls)
	}
}

func TestOverviewTrendsSupportNamedRangesWithOneAggregateQuery(t *testing.T) {
	clock := newServiceClock()
	ranges := map[string]time.Duration{
		"1h":  time.Hour,
		"6h":  6 * time.Hour,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
	}
	for rangeName, duration := range ranges {
		t.Run(rangeName, func(t *testing.T) {
			provider := fixtureProvider(clock.Now())
			svc := newService(provider, clock)

			overview, _, err := svc.Overview(context.Background(), rangeName)
			if err != nil {
				t.Fatalf("Overview() error = %v", err)
			}
			if len(provider.aggregateRequests) != 1 {
				t.Fatalf("aggregate requests = %d, want 1", len(provider.aggregateRequests))
			}
			request := provider.aggregateRequests[0]
			if !request.Start.Equal(clock.Now().Add(-duration)) || !request.End.Equal(clock.Now()) || request.Step <= 0 {
				t.Fatalf("aggregate request = %#v", request)
			}
			if int(request.End.Sub(request.Start)/request.Step)+1 > 600 {
				t.Fatalf("aggregate request exceeds 600 points: %#v", request)
			}
			if len(overview.Trends) != 2 {
				t.Fatalf("trends = %#v", overview.Trends)
			}
		})
	}
}

func TestOverviewReturnsIndependentCopyOfCachedTrends(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	first, _, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("first Overview() error = %v", err)
	}
	first.Trends[0].Key = datasource.MetricLoad1
	first.Trends[0].Points[0].Timestamp = time.Time{}
	*first.Trends[0].Points[0].Value = -1

	second, _, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("second Overview() error = %v", err)
	}
	if second.Trends[0].Key != datasource.MetricCPUUsage || second.Trends[0].Points[0].Timestamp.IsZero() || second.Trends[0].Points[0].Value == nil || *second.Trends[0].Points[0].Value != 42 {
		t.Fatalf("cached trends were mutated: %#v", second.Trends)
	}
	if len(provider.aggregateRequests) != 1 {
		t.Fatalf("aggregate requests = %d, want one cached load", len(provider.aggregateRequests))
	}
}

func TestOverviewPropagatesStaleAggregateTrendMetadata(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	_, firstMeta, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("first Overview() error = %v", err)
	}
	clock.Advance(2 * time.Minute)
	provider.aggregateErr = datasource.ErrUnavailable

	_, staleMeta, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("stale Overview() error = %v", err)
	}
	if !staleMeta.Stale || !staleMeta.CollectedAt.Equal(firstMeta.CollectedAt) {
		t.Fatalf("stale meta = %#v, first = %#v", staleMeta, firstMeta)
	}
	if len(provider.aggregateRequests) != 2 {
		t.Fatalf("aggregate requests = %d, want failed refresh after cache expiry", len(provider.aggregateRequests))
	}
}

func TestHostsSearchFilterStableSortAndPaginate(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "h3", Name: "db", IP: "192.0.2.30", Status: datasource.StatusOffline},
		{ID: "h2", Name: "web", IP: "192.0.2.20", Status: datasource.StatusOnline},
		{ID: "h1", Name: "web", IP: "192.0.2.10", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {Timestamp: clock.Now(), CPUUsage: float64Pointer(10)},
		"h2": {Timestamp: clock.Now(), CPUUsage: nil},
		"h3": {Timestamp: clock.Now(), CPUUsage: float64Pointer(90)},
	}
	svc := newService(provider, clock)

	page, _, err := svc.Hosts(context.Background(), service.HostQuery{
		Search:   "WEB",
		Status:   datasource.StatusOnline,
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	if page.Total != 2 || page.Page != 2 || page.PageSize != 1 || len(page.Hosts) != 1 || page.Hosts[0].ID != "h2" {
		t.Fatalf("page = %#v", page)
	}
	if page.Hosts[0].Metrics.CPUUsage.Level != service.LevelUnknown {
		t.Fatalf("nil CPU level = %q, want unknown", page.Hosts[0].Metrics.CPUUsage.Level)
	}
	if provider.currentCalls != 1 {
		t.Fatalf("GetCurrentMetrics() calls = %d, want 1", provider.currentCalls)
	}

	provider2 := fixtureProvider(clock.Now())
	svc2 := newService(provider2, clock)
	ipPage, _, err := svc2.Hosts(context.Background(), service.HostQuery{
		Search:   "0.2.2",
		Sort:     "ip",
		Order:    "desc",
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("IP search Hosts() error = %v", err)
	}
	if ipPage.Total != 1 || ipPage.Hosts[0].ID != "h2" {
		t.Fatalf("IP search page = %#v", ipPage)
	}
}

func TestHostsSortsLoadWithMissingValuesLastAndStableTies(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "h4", Name: "missing", IP: "192.0.2.4", Status: datasource.StatusOnline},
		{ID: "h2", Name: "load-b", IP: "192.0.2.2", Status: datasource.StatusOnline},
		{ID: "h3", Name: "load-low", IP: "192.0.2.3", Status: datasource.StatusOnline},
		{ID: "h1", Name: "load-a", IP: "192.0.2.1", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {Timestamp: clock.Now(), Load1: float64Pointer(5)},
		"h2": {Timestamp: clock.Now(), Load1: float64Pointer(5)},
		"h3": {Timestamp: clock.Now(), Load1: float64Pointer(1)},
		"h4": {Timestamp: clock.Now(), Load1: nil},
	}
	svc := newService(provider, clock)

	for _, test := range []struct {
		name  string
		sort  string
		order string
		want  []string
	}{
		{name: "ascending", sort: "load", order: "asc", want: []string{"h3", "h1", "h2", "h4"}},
		{name: "descending", sort: "load", order: "desc", want: []string{"h1", "h2", "h3", "h4"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, _, err := svc.Hosts(context.Background(), service.HostQuery{
				Sort:     test.sort,
				Order:    test.order,
				Page:     1,
				PageSize: 20,
			})
			if err != nil {
				t.Fatalf("Hosts() error = %v", err)
			}
			got := make([]string, len(page.Hosts))
			for i, host := range page.Hosts {
				got[i] = host.ID
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("host IDs = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHostsAssignsPercentageAndNetworkThresholdLevels(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.metrics["h1"] = datasource.CurrentMetrics{
		Timestamp:                     clock.Now(),
		CPUUsage:                      float64Pointer(79),
		MemoryUsage:                   float64Pointer(80),
		IOBusyPercent:                 float64Pointer(90),
		NetworkTransmitBytesPerSecond: float64Pointer(80 * 1024 * 1024),
		NetworkReceiveBytesPerSecond:  float64Pointer(100 * 1024 * 1024),
	}
	svc := newService(provider, clock)

	page, _, err := svc.Hosts(context.Background(), service.HostQuery{
		Sort: "name", Order: "asc", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	var host service.HostSummary
	for _, candidate := range page.Hosts {
		if candidate.ID == "h1" {
			host = candidate
			break
		}
	}
	assertMetric(t, "CPU", host.Metrics.CPUUsage, 79, service.LevelNormal)
	assertMetric(t, "memory", host.Metrics.MemoryUsage, 80, service.LevelWarning)
	assertMetric(t, "IO busy", host.Metrics.IOBusyPercent, 90, service.LevelCritical)
	assertMetric(t, "network transmit", host.Metrics.NetworkTransmitBytesPerSecond, 80*1024*1024, service.LevelWarning)
	assertMetric(t, "network receive", host.Metrics.NetworkReceiveBytesPerSecond, 100*1024*1024, service.LevelCritical)
	if host.CPUCores == nil || *host.CPUCores != 4 || host.MemoryTotalBytes == nil || *host.MemoryTotalBytes != 8*1024*1024*1024 {
		t.Fatalf("host hardware = %#v", host)
	}
}

func TestHostsSortsIOAndCombinedNetworkWithMissingValuesLast(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "h4", Name: "four", Status: datasource.StatusOnline},
		{ID: "h2", Name: "two", Status: datasource.StatusOnline},
		{ID: "h3", Name: "missing", Status: datasource.StatusOnline},
		{ID: "h1", Name: "one", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {
			IOBusyPercent:                 float64Pointer(20),
			NetworkTransmitBytesPerSecond: float64Pointer(10),
			NetworkReceiveBytesPerSecond:  float64Pointer(20),
		},
		"h2": {
			IOBusyPercent:                 float64Pointer(90),
			NetworkTransmitBytesPerSecond: float64Pointer(50),
		},
		"h3": {},
		"h4": {
			IOBusyPercent:                 float64Pointer(50),
			NetworkTransmitBytesPerSecond: float64Pointer(15),
			NetworkReceiveBytesPerSecond:  float64Pointer(15),
		},
	}
	svc := newService(provider, clock)

	for _, test := range []struct {
		name  string
		sort  string
		order string
		want  []string
	}{
		{name: "IO ascending", sort: "io", order: "asc", want: []string{"h1", "h4", "h2", "h3"}},
		{name: "IO descending", sort: "io", order: "desc", want: []string{"h2", "h4", "h1", "h3"}},
		{name: "network ascending", sort: "network", order: "asc", want: []string{"h1", "h4", "h2", "h3"}},
		{name: "network descending", sort: "network", order: "desc", want: []string{"h2", "h1", "h4", "h3"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, _, err := svc.Hosts(context.Background(), service.HostQuery{
				Sort: test.sort, Order: test.order, Page: 1, PageSize: 20,
			})
			if err != nil {
				t.Fatalf("Hosts() error = %v", err)
			}
			got := make([]string, len(page.Hosts))
			for i, host := range page.Hosts {
				got[i] = host.ID
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("host IDs = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHostsSortsHardwareAndNetworkColumnsWithMissingValuesLastInBothDirections(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "id-missing", Name: "missing", Status: datasource.StatusOnline},
		{ID: "id-high", Name: "high", CPUCores: intPointer(8), MemoryTotalBytes: int64Pointer(9_007_199_254_740_993), Status: datasource.StatusOnline},
		{ID: "id-low", Name: "low", CPUCores: intPointer(4), MemoryTotalBytes: int64Pointer(9_007_199_254_740_992), Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"id-low": {
			Timestamp:                     clock.Now(),
			NetworkTransmitBytesPerSecond: float64Pointer(10),
			NetworkReceiveBytesPerSecond:  float64Pointer(20),
		},
		"id-high": {
			Timestamp:                     clock.Now(),
			NetworkTransmitBytesPerSecond: float64Pointer(20),
			NetworkReceiveBytesPerSecond:  float64Pointer(10),
		},
		"id-missing": {Timestamp: clock.Now()},
	}
	svc := newService(provider, clock)

	tests := []struct {
		field             string
		wantAsc, wantDesc []string
	}{
		{"cpu_cores", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
		{"memory_total", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
		{"network_transmit", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
		{"network_receive", []string{"id-high", "id-low", "id-missing"}, []string{"id-low", "id-high", "id-missing"}},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			for _, order := range []struct {
				name  string
				order string
				want  []string
			}{
				{name: "ascending", order: "asc", want: test.wantAsc},
				{name: "descending", order: "desc", want: test.wantDesc},
			} {
				t.Run(order.name, func(t *testing.T) {
					page, _, err := svc.Hosts(context.Background(), service.HostQuery{
						Sort: test.field, Order: order.order, Page: 1, PageSize: 20,
					})
					if err != nil {
						t.Fatalf("Hosts() error = %v", err)
					}
					got := make([]string, len(page.Hosts))
					for index, host := range page.Hosts {
						got[index] = host.ID
					}
					if !reflect.DeepEqual(got, order.want) {
						t.Fatalf("host IDs = %v, want %v", got, order.want)
					}
				})
			}
		})
	}
}

func TestHostsSortsMissingNameAndIPLastInBothDirectionsWithStableID(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "id-empty-z", Name: "  ", IP: "\t", Status: datasource.StatusOnline},
		{ID: "id-high", Name: "beta", IP: "192.0.2.20", Status: datasource.StatusOnline},
		{ID: "id-empty-a", Name: "", IP: "", Status: datasource.StatusOnline},
		{ID: "id-low", Name: "alpha", IP: "192.0.2.10", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"id-low":     {Timestamp: clock.Now()},
		"id-high":    {Timestamp: clock.Now()},
		"id-empty-a": {Timestamp: clock.Now()},
		"id-empty-z": {Timestamp: clock.Now()},
	}
	svc := newService(provider, clock)

	for _, test := range []struct {
		field             string
		wantAsc, wantDesc []string
	}{
		{field: "name", wantAsc: []string{"id-low", "id-high", "id-empty-a", "id-empty-z"}, wantDesc: []string{"id-high", "id-low", "id-empty-a", "id-empty-z"}},
		{field: "ip", wantAsc: []string{"id-low", "id-high", "id-empty-a", "id-empty-z"}, wantDesc: []string{"id-high", "id-low", "id-empty-a", "id-empty-z"}},
	} {
		t.Run(test.field, func(t *testing.T) {
			for _, order := range []struct {
				name  string
				order string
				want  []string
			}{
				{name: "ascending", order: "asc", want: test.wantAsc},
				{name: "descending", order: "desc", want: test.wantDesc},
			} {
				t.Run(order.name, func(t *testing.T) {
					page, _, err := svc.Hosts(context.Background(), service.HostQuery{
						Sort: test.field, Order: order.order, Page: 1, PageSize: 20,
					})
					if err != nil {
						t.Fatalf("Hosts() error = %v", err)
					}
					got := make([]string, len(page.Hosts))
					for index, host := range page.Hosts {
						got[index] = host.ID
					}
					if !reflect.DeepEqual(got, order.want) {
						t.Fatalf("host IDs = %v, want %v", got, order.want)
					}
				})
			}
		})
	}
}

func TestHostsSortsNameAndIPNaturallyIgnoringCaseAndWhitespace(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "id-node-10", Name: "node10", IP: "192.0.2.10", Status: datasource.StatusOnline},
		{ID: "id-node-2b", Name: " NODE2 ", IP: " 192.0.2.2 ", Status: datasource.StatusOnline},
		{ID: "id-missing-z", Name: " \t", IP: "\t", Status: datasource.StatusOnline},
		{ID: "id-node-2a", Name: "node2", IP: "192.0.2.2", Status: datasource.StatusOnline},
		{ID: "id-missing-a", Name: "", IP: "", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"id-node-10":   {Timestamp: clock.Now()},
		"id-node-2a":   {Timestamp: clock.Now()},
		"id-node-2b":   {Timestamp: clock.Now()},
		"id-missing-a": {Timestamp: clock.Now()},
		"id-missing-z": {Timestamp: clock.Now()},
	}
	svc := newService(provider, clock)

	for _, field := range []string{"name", "ip"} {
		t.Run(field, func(t *testing.T) {
			for _, test := range []struct {
				order string
				want  []string
			}{
				{order: "asc", want: []string{"id-node-2a", "id-node-2b", "id-node-10", "id-missing-a", "id-missing-z"}},
				{order: "desc", want: []string{"id-node-10", "id-node-2a", "id-node-2b", "id-missing-a", "id-missing-z"}},
			} {
				page, _, err := svc.Hosts(context.Background(), service.HostQuery{
					Sort: field, Order: test.order, Page: 1, PageSize: 20,
				})
				if err != nil {
					t.Fatalf("Hosts() error = %v", err)
				}
				got := make([]string, len(page.Hosts))
				for index, host := range page.Hosts {
					got[index] = host.ID
				}
				if !reflect.DeepEqual(got, test.want) {
					t.Fatalf("%s/%s IDs = %v, want %v", field, test.order, got, test.want)
				}
			}
		})
	}
}

func TestHostsSortsStatusByDisplayedCollectionLevelThenStableID(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.hosts = []datasource.Host{
		{ID: "id-unknown", Name: "unknown", Status: datasource.StatusUnknown},
		{ID: "id-offline", Name: "offline", Status: datasource.StatusOffline},
		{ID: "id-warning", Name: "warning", Status: datasource.StatusOnline},
		{ID: "id-normal-z", Name: "normal-z", Status: datasource.StatusOnline},
		{ID: "id-normal-a", Name: "normal-a", Status: datasource.StatusOnline},
	}
	provider.metrics = map[string]datasource.CurrentMetrics{
		"id-normal-a": {Timestamp: clock.Now()},
		"id-normal-z": {Timestamp: clock.Now()},
		"id-warning":  {Timestamp: clock.Now()},
		"id-offline":  {Timestamp: clock.Now()},
		"id-unknown":  {Timestamp: clock.Now()},
	}
	svc := service.New(provider, cache.New(clock.Now), service.Options{
		InventoryTTL: time.Minute, CurrentMetricsTTL: time.Nanosecond, CollectionInterval: 15 * time.Second,
		RangeTTL: time.Minute, HealthTTL: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: clock.Now,
	})
	if _, _, err := svc.Hosts(context.Background(), service.HostQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("initial Hosts() error = %v", err)
	}
	clock.Advance(30 * time.Second)
	provider.metrics["id-normal-a"] = datasource.CurrentMetrics{Timestamp: clock.Now()}
	provider.metrics["id-normal-z"] = datasource.CurrentMetrics{Timestamp: clock.Now()}
	provider.metrics["id-offline"] = datasource.CurrentMetrics{Timestamp: clock.Now()}
	provider.metrics["id-unknown"] = datasource.CurrentMetrics{Timestamp: clock.Now()}

	page, _, err := svc.Hosts(context.Background(), service.HostQuery{Sort: "status", Order: "asc", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	got := make([]string, len(page.Hosts))
	for index, host := range page.Hosts {
		got[index] = host.ID
	}
	want := []string{"id-normal-a", "id-normal-z", "id-warning", "id-offline", "id-unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host IDs = %v, want %v", got, want)
	}
}

func TestHostsRejectsPageSizesOutsideOneToOneHundred(t *testing.T) {
	clock := newServiceClock()
	for _, pageSize := range []int{0, 101} {
		t.Run(fmt.Sprintf("page_size_%d", pageSize), func(t *testing.T) {
			svc := newService(fixtureProvider(clock.Now()), clock)
			_, _, err := svc.Hosts(context.Background(), service.HostQuery{Page: 1, PageSize: pageSize})
			if !errors.Is(err, service.ErrInvalidQuery) {
				t.Fatalf("Hosts() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestHostsReturnsEmptyPageForPageNumberThatWouldOverflowOffset(t *testing.T) {
	clock := newServiceClock()
	svc := newService(fixtureProvider(clock.Now()), clock)

	page, _, err := svc.Hosts(context.Background(), service.HostQuery{
		Page:     math.MaxInt,
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	if page.Total != 3 || len(page.Hosts) != 0 {
		t.Fatalf("page = %#v, want total 3 with no hosts", page)
	}
}

func TestHostReturnsDetailFilesystemsAndThresholdLevels(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.metrics["h1"] = datasource.CurrentMetrics{
		Timestamp:   clock.Now(),
		CPUUsage:    float64Pointer(85),
		MemoryUsage: float64Pointer(95),
		Load1:       float64Pointer(0),
		Filesystems: []datasource.FilesystemMetrics{
			{Mountpoint: "/", Usage: float64Pointer(91)},
			{Mountpoint: "/data", Usage: nil},
		},
	}
	svc := newService(provider, clock)

	detail, _, err := svc.Host(context.Background(), "h1")
	if err != nil {
		t.Fatalf("Host() error = %v", err)
	}
	if detail.ID != "h1" || detail.Status != datasource.StatusOnline {
		t.Fatalf("detail host = %#v", detail)
	}
	assertMetric(t, "CPU", detail.Metrics.CPUUsage, 85, service.LevelWarning)
	assertMetric(t, "memory", detail.Metrics.MemoryUsage, 95, service.LevelCritical)
	assertMetric(t, "load", detail.Metrics.Load1, 0, service.LevelNormal)
	if len(detail.Metrics.Filesystems) != 2 {
		t.Fatalf("filesystems = %#v", detail.Metrics.Filesystems)
	}
	assertMetric(t, "root disk", detail.Metrics.Filesystems[0].Usage, 91, service.LevelCritical)
	if detail.Metrics.Filesystems[1].Usage.Value != nil || detail.Metrics.Filesystems[1].Usage.Level != service.LevelUnknown {
		t.Fatalf("unknown filesystem metric = %#v", detail.Metrics.Filesystems[1])
	}
	if provider.currentCalls != 1 || !reflect.DeepEqual(provider.currentRequests[0], []string{"h1"}) {
		t.Fatalf("current metric calls/IDs = %d/%v", provider.currentCalls, provider.currentRequests)
	}
}

func TestHostMapsDataSourceNotFound(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.getHostErr = datasource.ErrNotFound
	svc := newService(provider, clock)

	_, _, err := svc.Host(context.Background(), "missing")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("Host() error = %v, want ErrNotFound", err)
	}
}

func TestMetricsSupportsNamedRangesWithAtMostSixHundredPoints(t *testing.T) {
	clock := newServiceClock()
	ranges := map[string]time.Duration{
		"1h":  time.Hour,
		"6h":  6 * time.Hour,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
	}
	for rangeName, duration := range ranges {
		t.Run(rangeName, func(t *testing.T) {
			provider := fixtureProvider(clock.Now())
			svc := newService(provider, clock)

			metricRange, _, err := svc.Metrics(context.Background(), "h1", rangeName)
			if err != nil {
				t.Fatalf("Metrics() error = %v", err)
			}
			if metricRange.HostID != "h1" || metricRange.Range != rangeName || !metricRange.To.Equal(clock.Now()) || !metricRange.From.Equal(clock.Now().Add(-duration)) {
				t.Fatalf("range metadata = %#v", metricRange)
			}
			if len(metricRange.Series) != 8 || len(provider.rangeRequests) != 8 {
				t.Fatalf("series/requests = %d/%d, want 8/8", len(metricRange.Series), len(provider.rangeRequests))
			}
			for _, series := range metricRange.Series {
				if len(series.Points) > 600 {
					t.Fatalf("%s points = %d, want <= 600", series.Metric, len(series.Points))
				}
			}
			for _, request := range provider.rangeRequests {
				if !reflect.DeepEqual(request.HostIDs, []string{"h1"}) || !request.From.Equal(metricRange.From) || !request.To.Equal(metricRange.To) || request.Step <= 0 {
					t.Fatalf("range request = %#v", request)
				}
				if int(request.To.Sub(request.From)/request.Step)+1 > 600 {
					t.Fatalf("request would produce more than 600 points: %#v", request)
				}
			}
		})
	}
}

func TestMetricsRejectsUnknownHostBeforeRangeQueries(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	provider.getHostErr = datasource.ErrNotFound
	svc := newService(provider, clock)

	_, _, err := svc.Metrics(context.Background(), "unknown-host", "1h")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("Metrics() error = %v, want ErrNotFound", err)
	}
	if provider.getHostCalls != 1 {
		t.Fatalf("GetHost() calls = %d, want 1", provider.getHostCalls)
	}
	if len(provider.rangeRequests) != 0 {
		t.Fatalf("QueryRange() calls = %d, want 0", len(provider.rangeRequests))
	}
}

func TestMetricsReturnsAnIndependentCopyOfCachedRange(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	first, _, err := svc.Metrics(context.Background(), "h1", "1h")
	if err != nil {
		t.Fatalf("first Metrics() error = %v", err)
	}
	first.Series[0].Metric = datasource.MetricLoad1
	first.Series[0].Points[0].Timestamp = time.Time{}
	*first.Series[0].Points[0].Value = -1

	second, _, err := svc.Metrics(context.Background(), "h1", "1h")
	if err != nil {
		t.Fatalf("second Metrics() error = %v", err)
	}
	if second.Series[0].Metric != datasource.MetricCPUUsage {
		t.Fatalf("cached metric = %q, want %q", second.Series[0].Metric, datasource.MetricCPUUsage)
	}
	if second.Series[0].Points[0].Timestamp.IsZero() || second.Series[0].Points[0].Value == nil || *second.Series[0].Points[0].Value != 42 {
		t.Fatalf("cached point was mutated: %#v", second.Series[0].Points[0])
	}
	if provider.rangeRequests == nil || len(provider.rangeRequests) != 8 {
		t.Fatalf("range requests = %d, want one cached load of 8 metrics", len(provider.rangeRequests))
	}
	if provider.getHostCalls != 1 {
		t.Fatalf("GetHost() calls = %d, want one cached load validation", provider.getHostCalls)
	}
}

func TestOverviewPropagatesStaleCacheMetadata(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	_, firstMeta, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("first Overview() error = %v", err)
	}
	clock.Advance(2 * time.Minute)
	provider.listErr = datasource.ErrUnavailable
	provider.currentErr = datasource.ErrUnavailable

	_, staleMeta, err := svc.Overview(context.Background(), "1h")
	if err != nil {
		t.Fatalf("stale Overview() error = %v", err)
	}
	if !staleMeta.Stale || !staleMeta.CollectedAt.Equal(firstMeta.CollectedAt) {
		t.Fatalf("stale meta = %#v, first = %#v", staleMeta, firstMeta)
	}
}

func TestDataSourceStatusIsCachedForFifteenSeconds(t *testing.T) {
	clock := newServiceClock()
	provider := fixtureProvider(clock.Now())
	svc := newService(provider, clock)

	first, _, err := svc.DataSourceStatus(context.Background())
	if err != nil || !first.Healthy {
		t.Fatalf("first DataSourceStatus() = %#v, %v", first, err)
	}
	clock.Advance(14 * time.Second)
	if _, _, err := svc.DataSourceStatus(context.Background()); err != nil {
		t.Fatalf("cached DataSourceStatus() error = %v", err)
	}
	if provider.healthCalls != 1 {
		t.Fatalf("Health() calls before expiry = %d, want 1", provider.healthCalls)
	}
	clock.Advance(time.Second)
	if _, _, err := svc.DataSourceStatus(context.Background()); err != nil {
		t.Fatalf("expired DataSourceStatus() error = %v", err)
	}
	if provider.healthCalls != 2 {
		t.Fatalf("Health() calls after expiry = %d, want 2", provider.healthCalls)
	}
}

func newService(provider datasource.Provider, clock *serviceClock) *service.Service {
	return service.New(provider, cache.New(clock.Now), service.Options{
		InventoryTTL:       time.Minute,
		CurrentMetricsTTL:  time.Minute,
		CollectionInterval: 15 * time.Second,
		RangeTTL:           time.Minute,
		HealthTTL:          15 * time.Second,
		MaxStale:           5 * time.Minute,
		WarningPercent:     80,
		CriticalPercent:    90,
		NetworkWarningBPS:  80 * 1024 * 1024,
		NetworkCriticalBPS: 100 * 1024 * 1024,
		Clock:              clock.Now,
	})
}

func TestHostsAndOverviewUseObservedSampleProgressForCollectionLevel(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	clock := newServiceClock()
	clock.now = now
	provider := fixtureProvider(now)
	provider.hosts = []datasource.Host{{
		ID: "h1", Name: "fixture-host", Status: datasource.StatusOnline,
		StatusTime: now,
	}}
	sampleAt := now.Add(-time.Hour)
	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {Timestamp: sampleAt},
	}
	svc := service.New(provider, cache.New(clock.Now), service.Options{
		InventoryTTL:       time.Minute,
		CurrentMetricsTTL:  time.Second,
		CollectionInterval: 15 * time.Second,
		RangeTTL:           time.Minute,
		HealthTTL:          15 * time.Second,
		MaxStale:           5 * time.Minute,
		WarningPercent:     80,
		CriticalPercent:    90,
		NetworkWarningBPS:  80 * 1024 * 1024,
		NetworkCriticalBPS: 100 * 1024 * 1024,
		Clock:              clock.Now,
	})

	assertState := func(wantCollection service.Level, wantStatus datasource.HostStatus, wantWarning, wantCritical int) {
		t.Helper()
		page, _, err := svc.Hosts(context.Background(), service.HostQuery{
			Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("Hosts() error = %v", err)
		}
		if len(page.Hosts) != 1 ||
			page.Hosts[0].CollectionLevel != wantCollection ||
			page.Hosts[0].Status != wantStatus {
			t.Fatalf("host = %#v", page.Hosts)
		}

		overview, _, err := svc.Overview(context.Background(), "24h")
		if err != nil {
			t.Fatalf("Overview() error = %v", err)
		}
		if overview.Alerts.WarningHosts != wantWarning ||
			overview.Alerts.CriticalHosts != wantCritical {
			t.Fatalf("alerts = %#v", overview.Alerts)
		}
	}

	assertState(service.LevelNormal, datasource.StatusOnline, 0, 0)

	clock.Advance(30 * time.Second)
	assertState(service.LevelWarning, datasource.StatusUnknown, 1, 0)

	clock.Advance(45 * time.Second)
	assertState(service.LevelCritical, datasource.StatusOffline, 0, 1)

	provider.metrics = map[string]datasource.CurrentMetrics{
		"h1": {Timestamp: sampleAt.Add(15 * time.Second)},
	}
	clock.Advance(2 * time.Second)
	assertState(service.LevelNormal, datasource.StatusOnline, 0, 0)
}

func TestHostsCollectionRemainsNormalWhileOldSampleTimeKeepsAdvancing(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	clock := newServiceClock()
	clock.now = now
	provider := fixtureProvider(now)
	provider.hosts = []datasource.Host{{
		ID: "h1", Name: "fixture-host", Status: datasource.StatusOnline,
	}}
	sampleAt := now.Add(-time.Hour)
	provider.metrics = map[string]datasource.CurrentMetrics{"h1": {Timestamp: sampleAt}}
	svc := service.New(provider, cache.New(clock.Now), service.Options{
		InventoryTTL:       time.Minute,
		CurrentMetricsTTL:  time.Second,
		CollectionInterval: 15 * time.Second,
		RangeTTL:           time.Minute,
		HealthTTL:          15 * time.Second,
		MaxStale:           5 * time.Minute,
		Clock:              clock.Now,
	})

	for index := 0; index < 4; index++ {
		page, _, err := svc.Hosts(context.Background(), service.HostQuery{
			Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("Hosts() error = %v", err)
		}
		if len(page.Hosts) != 1 ||
			page.Hosts[0].CollectionLevel != service.LevelNormal ||
			page.Hosts[0].Status != datasource.StatusOnline {
			t.Fatalf("host = %#v", page.Hosts)
		}
		clock.Advance(15 * time.Second)
		sampleAt = sampleAt.Add(15 * time.Second)
		provider.metrics = map[string]datasource.CurrentMetrics{"h1": {Timestamp: sampleAt}}
	}
}

func TestHostsTreatMissingMetricHeartbeatAsCriticalEvenWhenTargetIsFresh(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := newServiceClock()
	clock.now = now
	provider := fixtureProvider(now)
	provider.hosts = []datasource.Host{{
		ID: "h1", Name: "fixture-host", Status: datasource.StatusOnline,
		StatusTime: now,
	}}
	provider.metrics = map[string]datasource.CurrentMetrics{"h1": {}}
	svc := newService(provider, clock)

	page, _, err := svc.Hosts(context.Background(), service.HostQuery{
		Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}
	if len(page.Hosts) != 1 ||
		page.Hosts[0].CollectionLevel != service.LevelCritical ||
		page.Hosts[0].Status != datasource.StatusOffline {
		t.Fatalf("host = %#v", page.Hosts)
	}
}

func fixtureProvider(now time.Time) *recordingProvider {
	hosts := []datasource.Host{
		{ID: "h1", Name: "web-one", IP: "192.0.2.1", OS: "linux", CPUCores: intPointer(4), MemoryTotalBytes: int64Pointer(8 * 1024 * 1024 * 1024), Status: datasource.StatusOnline, StatusTime: now, Uptime: time.Hour},
		{ID: "h2", Name: "web-two", IP: "192.0.2.2", OS: "linux", Status: datasource.StatusOffline, StatusTime: now, Uptime: 2 * time.Hour},
		{ID: "h3", Name: "db-one", IP: "192.0.2.3", OS: "linux", CPUCores: intPointer(16), MemoryTotalBytes: int64Pointer(64 * 1024 * 1024 * 1024), Status: datasource.StatusUnknown, StatusTime: now, Uptime: 3 * time.Hour},
	}
	return &recordingProvider{
		now:   now,
		hosts: hosts,
		metrics: map[string]datasource.CurrentMetrics{
			"h1": {Timestamp: now, CPUUsage: float64Pointer(10), MemoryUsage: nil},
			"h2": {Timestamp: now, CPUUsage: nil, MemoryUsage: float64Pointer(40)},
			"h3": {Timestamp: now, CPUUsage: float64Pointer(30), MemoryUsage: float64Pointer(60)},
		},
	}
}

type recordingProvider struct {
	now               time.Time
	hosts             []datasource.Host
	metrics           map[string]datasource.CurrentMetrics
	listErr           error
	getHostErr        error
	currentErr        error
	aggregateErr      error
	healthErr         error
	healthCalls       int
	listCalls         int
	getHostCalls      int
	currentCalls      int
	currentRequests   [][]string
	rangeRequests     []datasource.RangeRequest
	aggregateRequests []datasource.AggregateRangeRequest
}

func (p *recordingProvider) Health(context.Context) (datasource.Health, error) {
	p.healthCalls++
	if p.healthErr != nil {
		return datasource.Health{}, p.healthErr
	}
	return datasource.Health{Healthy: true, CheckedAt: p.now}, nil
}

func (p *recordingProvider) ListHosts(context.Context) ([]datasource.Host, error) {
	p.listCalls++
	if p.listErr != nil {
		return nil, p.listErr
	}
	return append([]datasource.Host(nil), p.hosts...), nil
}

func (p *recordingProvider) GetHost(_ context.Context, id string) (datasource.Host, error) {
	p.getHostCalls++
	if p.getHostErr != nil {
		return datasource.Host{}, p.getHostErr
	}
	for _, host := range p.hosts {
		if host.ID == id {
			return host, nil
		}
	}
	return datasource.Host{}, datasource.ErrNotFound
}

func (p *recordingProvider) GetCurrentMetrics(_ context.Context, ids []string) (map[string]datasource.CurrentMetrics, error) {
	p.currentCalls++
	p.currentRequests = append(p.currentRequests, append([]string(nil), ids...))
	if p.currentErr != nil {
		return nil, p.currentErr
	}
	result := make(map[string]datasource.CurrentMetrics, len(ids))
	for _, id := range ids {
		if metric, ok := p.metrics[id]; ok {
			result[id] = metric
		}
	}
	return result, nil
}

func (p *recordingProvider) QueryRange(_ context.Context, request datasource.RangeRequest) ([]datasource.Series, error) {
	p.rangeRequests = append(p.rangeRequests, request)
	points := make([]datasource.Point, 0, int(request.To.Sub(request.From)/request.Step)+1)
	for timestamp := request.From; !timestamp.After(request.To); timestamp = timestamp.Add(request.Step) {
		points = append(points, datasource.Point{Timestamp: timestamp, Value: float64Pointer(42)})
	}
	return []datasource.Series{{HostID: request.HostIDs[0], Metric: request.Metric, Points: points}}, nil
}

func (p *recordingProvider) QueryAggregateRange(_ context.Context, request datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	p.aggregateRequests = append(p.aggregateRequests, request)
	if p.aggregateErr != nil {
		return nil, p.aggregateErr
	}
	series := make([]datasource.Series, len(request.Keys))
	for i, key := range request.Keys {
		points := make([]datasource.Point, 0, int(request.End.Sub(request.Start)/request.Step)+1)
		for timestamp := request.Start; !timestamp.After(request.End); timestamp = timestamp.Add(request.Step) {
			points = append(points, datasource.Point{Timestamp: timestamp, Value: float64Pointer(42 + float64(i))})
		}
		series[i] = datasource.Series{Metric: key, Points: points}
	}
	return series, nil
}

func assertMetric(t *testing.T, name string, got service.MetricValue, wantValue float64, wantLevel service.Level) {
	t.Helper()
	if got.Value == nil || *got.Value != wantValue || got.Level != wantLevel {
		t.Fatalf("%s = %#v, want value %v and level %q", name, got, wantValue, wantLevel)
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}

func intPointer(value int) *int {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}

type serviceClock struct {
	mu  sync.Mutex
	now time.Time
}

func newServiceClock() *serviceClock {
	return &serviceClock{now: time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC)}
}

func (c *serviceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *serviceClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

var _ datasource.Provider = (*recordingProvider)(nil)
