package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/javaapp"
)

func TestJavaBusinessNameUsesExactMapping(t *testing.T) {
	cases := map[string]string{
		"tikbee":       "用户端",
		"rider":        "骑手端",
		"mch":          "商家端",
		"saas":         "管理后台端",
		"mch_saas":     "商家 PC 端",
		"tikbee-extra": "tikbee-extra",
	}
	for input, want := range cases {
		if got := javaBusinessName(input); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
}

func TestJavaStatusUsesCriticalBusinessSignalsBeforeCollection(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*javaapp.Service)
		source JavaStatusSource
	}{
		{"health", func(item *javaapp.Service) { item.HealthUp = javaBool(false) }, JavaStatusHealth},
		{"port", func(item *javaapp.Service) { item.PortUp = javaBool(false) }, JavaStatusPort},
		{"process", func(item *javaapp.Service) { item.ProcessUp = javaBool(false) }, JavaStatusProcess},
		{"consistency", func(item *javaapp.Service) { item.PortConsistent = javaBool(false) }, JavaStatusConsistency},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
			test.mutate(&item)
			summary := summarizeJavaService(item, LevelWarning, now)
			if summary.Status != LevelCritical || summary.StatusSource != test.source {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}
}

func TestJavaStatusMissingRequiredFieldsIsUnknownOnlyWithoutHigherIssue(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*javaapp.Service)
	}{
		{"health", func(item *javaapp.Service) { item.HealthUp = nil }},
		{"port", func(item *javaapp.Service) { item.PortUp = nil }},
		{"process", func(item *javaapp.Service) { item.ProcessUp = nil }},
		{"consistency", func(item *javaapp.Service) { item.PortConsistent = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
			test.mutate(&item)
			summary := summarizeJavaService(item, LevelNormal, now)
			if summary.Status != LevelUnknown || summary.StatusSource != JavaStatusUnknown {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}

	item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.HealthUp = nil
	item.PortUp = javaBool(false)
	summary := summarizeJavaService(item, LevelNormal, now)
	if summary.Status != LevelCritical || summary.StatusSource != JavaStatusPort {
		t.Fatalf("critical business signal did not beat missing field: %#v", summary)
	}
}

func TestJavaStatusChoosesLevelThenExactSourcePriority(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.HealthUp = javaBool(false)
	item.PortUp = javaBool(false)
	item.ProcessUp = javaBool(false)
	item.PortConsistent = javaBool(false)
	got := summarizeJavaService(item, LevelCritical, now)
	if got.Status != LevelCritical || got.StatusSource != JavaStatusHealth {
		t.Fatalf("equal critical priority = %#v", got)
	}

	item = healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.PortUp = javaBool(false)
	item.ProcessUp = javaBool(false)
	item.PortConsistent = javaBool(false)
	got = summarizeJavaService(item, LevelCritical, now)
	if got.StatusSource != JavaStatusPort {
		t.Fatalf("port priority = %#v", got)
	}

	item = healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.ProcessUp = javaBool(false)
	item.PortConsistent = javaBool(false)
	got = summarizeJavaService(item, LevelCritical, now)
	if got.StatusSource != JavaStatusProcess {
		t.Fatalf("process priority = %#v", got)
	}

	item = healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.HealthUp = nil
	got = summarizeJavaService(item, LevelWarning, now)
	if got.Status != LevelWarning || got.StatusSource != JavaStatusCollection {
		t.Fatalf("warning collection did not beat unknown: %#v", got)
	}
}

func TestJavaDisplayMetricsNeverChangeNormalStatus(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
	item.HealthLatencyMilliseconds = javaFloat(math.MaxFloat64)
	item.ProcessCount = javaInt64(math.MaxInt64)
	item.ProcessMemoryBytes = javaInt64(math.MaxInt64)
	item.ProcessCPUPercent = javaFloat(math.MaxFloat64)
	item.ProcessMemoryPercent = javaFloat(math.MaxFloat64)
	item.ProcessStartTimeSeconds = javaInt64(0)
	got := summarizeJavaService(item, LevelNormal, now)
	if got.Status != LevelNormal || got.StatusSource != JavaStatusNormal {
		t.Fatalf("display metrics changed status: %#v", got)
	}
	if got.HealthLatencyMilliseconds == nil || got.ProcessCount == nil || got.MemoryBytes == nil || got.CPUUsagePercent == nil || got.MemoryUsagePercent == nil {
		t.Fatalf("display metrics were not preserved: %#v", got)
	}
}

func TestJavaUptimeRequiresValidStartTime(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 10, 0, time.UTC)
	tests := []struct {
		name  string
		start *int64
		want  *int64
	}{
		{"missing", nil, nil},
		{"negative", javaInt64(-1), nil},
		{"future", javaInt64(now.Unix() + 1), nil},
		{"equal", javaInt64(now.Unix()), javaInt64(0)},
		{"past", javaInt64(now.Unix() - 10), javaInt64(10)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
			item.ProcessStartTimeSeconds = test.start
			got := summarizeJavaService(item, LevelNormal, now).UptimeSeconds
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("uptime = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestJavaServiceFreshnessBoundariesCacheHitsAndProgress(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	clock := &javaTestClock{now: now}
	provider := &recordingJavaProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{
		healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now.Add(-24*time.Hour)),
	}}}
	service := NewJava(provider, cache.New(clock.Now), JavaOptions{
		SnapshotTTL: time.Minute, CollectionInterval: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: clock.Now,
	})

	page := mustJavaPage(t, service, JavaQuery{})
	if page.Services[0].CollectionLevel != LevelNormal {
		t.Fatalf("first load must only establish baseline: %#v", page.Services[0])
	}
	clock.Advance(30 * time.Second)
	page = mustJavaPage(t, service, JavaQuery{})
	if page.Services[0].CollectionLevel != LevelWarning || provider.calls != 1 {
		t.Fatalf("cache hit at two cycles = %#v, calls = %d", page.Services[0], provider.calls)
	}
	clock.Advance(45 * time.Second)
	page = mustJavaPage(t, service, JavaQuery{})
	if page.Services[0].CollectionLevel != LevelCritical || provider.calls != 2 {
		t.Fatalf("five cycles = %#v, calls = %d", page.Services[0], provider.calls)
	}

	provider.snapshot.Services[0].ReportedAt = now
	clock.Advance(time.Minute)
	page = mustJavaPage(t, service, JavaQuery{})
	if page.Services[0].CollectionLevel != LevelNormal {
		t.Fatalf("advance did not reset freshness: %#v", page.Services[0])
	}
	provider.snapshot.Services[0].ReportedAt = now.Add(-time.Second)
	clock.Advance(time.Minute)
	page = mustJavaPage(t, service, JavaQuery{})
	if page.Services[0].CollectionLevel != LevelNormal {
		t.Fatalf("rollback did not reset freshness: %#v", page.Services[0])
	}
}

func TestJavaCollectionLevelUsesExactCycleBoundaries(t *testing.T) {
	baseline := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	interval := 15 * time.Second
	tests := []struct {
		name string
		age  time.Duration
		want Level
	}{
		{"just before warning", 2*interval - time.Nanosecond, LevelNormal},
		{"warning boundary", 2 * interval, LevelWarning},
		{"just before critical", 5*interval - time.Nanosecond, LevelWarning},
		{"critical boundary", 5 * interval, LevelCritical},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := collectionLevelAt(baseline.Add(test.age), baseline, interval); got != test.want {
				t.Fatalf("collection level at %s = %q, want %q", test.age, got, test.want)
			}
		})
	}
}

func TestJavaResponsesReadUTCClockOnceAndReuseIt(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 30, 0, time.FixedZone("fixture-zone", 8*60*60))
	for _, test := range []struct {
		name string
		call func(*JavaService) error
	}{
		{"overview", func(service *JavaService) error { _, _, err := service.Overview(context.Background()); return err }},
		{"services", func(service *JavaService) error {
			_, _, err := service.Services(context.Background(), JavaQuery{})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &recordingJavaProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{
				healthyJavaService("id-a", "tikbee", "fixture-address-a", now),
				healthyJavaService("id-b", "rider", "fixture-address-b", now),
			}}}
			clockCalls := 0
			clock := func() time.Time {
				clockCalls++
				return now.Add(time.Duration(clockCalls-1) * time.Second)
			}
			service := NewJava(provider, cache.New(func() time.Time { return now }), JavaOptions{
				SnapshotTTL: time.Minute, CollectionInterval: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: clock,
			})
			if err := test.call(service); err != nil {
				t.Fatal(err)
			}
			if clockCalls != 1 {
				t.Fatalf("Clock calls = %d, want exactly 1 UTC response timestamp", clockCalls)
			}
		})
	}
}

func TestJavaOverviewDeduplicatesProcessConsistencyAndCombinesCollection(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		processUp      *bool
		consistent     *bool
		collection     Level
		wantStatus     Level
		wantSource     JavaStatusSource
		wantProcess    JavaAlertCount
		wantCollection JavaAlertCount
	}{
		{"process and consistency critical count once", javaBool(false), javaBool(false), LevelNormal, LevelCritical, JavaStatusProcess, JavaAlertCount{Critical: 1}, JavaAlertCount{}},
		{"process critical with collection warning", javaBool(false), javaBool(true), LevelWarning, LevelCritical, JavaStatusProcess, JavaAlertCount{Critical: 1}, JavaAlertCount{Warning: 1}},
		{"consistency critical with collection critical", javaBool(true), javaBool(false), LevelCritical, LevelCritical, JavaStatusConsistency, JavaAlertCount{Critical: 1}, JavaAlertCount{Critical: 1}},
		{"process unknown with collection warning", nil, javaBool(true), LevelWarning, LevelWarning, JavaStatusCollection, JavaAlertCount{Unknown: 1}, JavaAlertCount{Warning: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now)
			item.ProcessUp = test.processUp
			item.PortConsistent = test.consistent
			service := newJavaServiceWithSnapshot(now, javaapp.Snapshot{Services: []javaapp.Service{item}})
			state, _, err := service.snapshotState(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			advanced := state.serviceAdvancedAt[item.ID]
			service.options.Clock = func() time.Time {
				switch test.collection {
				case LevelWarning:
					return advanced.Add(2 * service.options.CollectionInterval)
				case LevelCritical:
					return advanced.Add(5 * service.options.CollectionInterval)
				default:
					return advanced
				}
			}
			overview, _, err := service.Overview(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if overview.Status != test.wantStatus || overview.Alerts.Process != test.wantProcess || overview.Alerts.Collection != test.wantCollection {
				t.Fatalf("overview = %#v, want status=%q process=%#v collection=%#v", overview, test.wantStatus, test.wantProcess, test.wantCollection)
			}
			page := mustJavaPage(t, service, JavaQuery{})
			if page.Services[0].StatusSource != test.wantSource {
				t.Fatalf("status source = %q, want %q", page.Services[0].StatusSource, test.wantSource)
			}
		})
	}
}

func TestJavaServiceStaleFallbackKeepsSnapshotAndContinuesAging(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	clock := &javaTestClock{now: now}
	provider := &recordingJavaProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{
		healthyJavaService("fixture-id", "tikbee", "fixture-address-a", now),
	}}}
	service := NewJava(provider, cache.New(clock.Now), JavaOptions{
		SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: clock.Now,
	})
	mustJavaPage(t, service, JavaQuery{})
	provider.snapshot.Services[0].Name = "rider"
	provider.err = errors.New("fixture unavailable")
	clock.Advance(30 * time.Second)
	page, meta, err := service.Services(context.Background(), JavaQuery{})
	if err != nil || !meta.Stale || page.Services[0].Name != "tikbee" || page.Services[0].CollectionLevel != LevelWarning {
		t.Fatalf("warning stale fallback page/meta/err = %#v/%#v/%v", page, meta, err)
	}
	clock.Advance(45 * time.Second)
	page, meta, err = service.Services(context.Background(), JavaQuery{})
	if err != nil || !meta.Stale || page.Services[0].CollectionLevel != LevelCritical {
		t.Fatalf("critical stale fallback page/meta/err = %#v/%#v/%v", page, meta, err)
	}
}

func TestJavaOverviewCountsStatusAndIndependentAlerts(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	clock := &javaTestClock{now: now}
	normal := healthyJavaService("id-normal", "tikbee", "address-normal", now)
	critical := healthyJavaService("id-critical", "rider", "address-critical", now)
	critical.HealthUp = javaBool(false)
	warning := healthyJavaService("id-warning", "mch", "address-warning", now)
	unknown := healthyJavaService("id-unknown", "saas", "address-unknown", now)
	unknown.HealthUp = nil
	unknown.PortUp = nil
	unknown.ProcessUp = nil
	unknown.PortConsistent = nil
	provider := &recordingJavaProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{normal, critical, warning, unknown}}}
	service := NewJava(provider, cache.New(clock.Now), JavaOptions{
		SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: clock.Now,
	})
	mustJavaPage(t, service, JavaQuery{})
	clock.Advance(30 * time.Second)
	provider.snapshot.Services[0].ReportedAt = now.Add(time.Second)
	provider.snapshot.Services[1].ReportedAt = now.Add(time.Second)
	provider.snapshot.Services[3].ReportedAt = now.Add(time.Second)
	overview, _, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := JavaLevelCounts{Total: 4, Normal: 1, Warning: 1, Critical: 1, Unknown: 1}
	if overview.Status != LevelCritical || overview.Services != wantCounts {
		t.Fatalf("overview status/counts = %q/%#v, want %q/%#v", overview.Status, overview.Services, LevelCritical, wantCounts)
	}
	wantAlerts := JavaOverviewAlerts{
		Health:     JavaAlertCount{Critical: 1, Unknown: 1},
		Port:       JavaAlertCount{Unknown: 1},
		Process:    JavaAlertCount{Unknown: 1},
		Collection: JavaAlertCount{Warning: 1},
	}
	if overview.Alerts != wantAlerts {
		t.Fatalf("alerts = %#v, want %#v", overview.Alerts, wantAlerts)
	}
}

func TestJavaServicesSearchFiltersAndCompleteAvailableNames(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	first := healthyJavaService("private-id-only", "tikbee", "fixture-address-a", now)
	second := healthyJavaService("id-b", "rider", "fixture-address-b", now)
	third := healthyJavaService("id-c", "mch", "fixture-address-c", now)
	third.HealthUp = javaBool(false)
	service := newJavaServiceWithSnapshot(now, javaapp.Snapshot{Services: []javaapp.Service{first, second, third, first}})

	tests := []struct {
		name  string
		query JavaQuery
		want  []string
	}{
		{"raw code", JavaQuery{Search: "TIKBEE"}, []string{"private-id-only", "private-id-only"}},
		{"business", JavaQuery{Search: "骑手端"}, []string{"id-b"}},
		{"address", JavaQuery{Search: "ADDRESS-C"}, []string{"id-c"}},
		{"id excluded", JavaQuery{Search: "private-id-only"}, []string{}},
		{"exact name", JavaQuery{Name: "rider"}, []string{"id-b"}},
		{"name not substring", JavaQuery{Name: "rid"}, []string{}},
		{"exact status", JavaQuery{Status: LevelCritical}, []string{"id-c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := mustJavaPage(t, service, test.query)
			if got := javaIDs(page.Services); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("IDs = %#v, want %#v", got, test.want)
			}
			if want := []string{"mch", "rider", "tikbee"}; !reflect.DeepEqual(page.AvailableNames, want) {
				t.Fatalf("AvailableNames = %#v, want %#v", page.AvailableNames, want)
			}
		})
	}
}

func TestJavaServicesSupportsAllSortKeysBothDirectionsAndNilLast(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 10, 0, time.UTC)
	tests := []struct {
		field  string
		mutate func(low, high, missing *javaapp.Service)
		asc    []string
		desc   []string
	}{
		{"business", func(low, high, missing *javaapp.Service) { low.Name, high.Name, missing.Name = "aaa", "zzz", "" }, []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
		{"address", func(low, high, missing *javaapp.Service) {
			low.Address, high.Address, missing.Address = "address-a", "address-z", ""
		}, []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
		{"health", func(low, high, missing *javaapp.Service) {
			low.HealthUp, high.HealthUp, missing.HealthUp = javaBool(false), javaBool(true), nil
		}, nil, nil},
		{"health_latency", func(low, high, missing *javaapp.Service) {
			low.HealthLatencyMilliseconds, high.HealthLatencyMilliseconds, missing.HealthLatencyMilliseconds = javaFloat(1), javaFloat(2), nil
		}, nil, nil},
		{"port", func(low, high, missing *javaapp.Service) {
			low.PortUp, high.PortUp, missing.PortUp = javaBool(false), javaBool(true), nil
		}, nil, nil},
		{"process", func(low, high, missing *javaapp.Service) {
			low.ProcessUp, high.ProcessUp, missing.ProcessUp = javaBool(false), javaBool(true), nil
		}, nil, nil},
		{"process_count", func(low, high, missing *javaapp.Service) {
			low.ProcessCount, high.ProcessCount, missing.ProcessCount = javaInt64(1), javaInt64(2), nil
		}, nil, nil},
		{"consistency", func(low, high, missing *javaapp.Service) {
			low.PortConsistent, high.PortConsistent, missing.PortConsistent = javaBool(false), javaBool(true), nil
		}, nil, nil},
		{"cpu", func(low, high, missing *javaapp.Service) {
			low.ProcessCPUPercent, high.ProcessCPUPercent, missing.ProcessCPUPercent = javaFloat(1), javaFloat(2), nil
		}, nil, nil},
		{"memory", func(low, high, missing *javaapp.Service) {
			low.ProcessMemoryBytes, high.ProcessMemoryBytes, missing.ProcessMemoryBytes = javaInt64(1), javaInt64(2), nil
		}, nil, nil},
		{"memory_percent", func(low, high, missing *javaapp.Service) {
			low.ProcessMemoryPercent, high.ProcessMemoryPercent, missing.ProcessMemoryPercent = javaFloat(1), javaFloat(2), nil
		}, nil, nil},
		{"uptime", func(low, high, missing *javaapp.Service) {
			low.ProcessStartTimeSeconds, high.ProcessStartTimeSeconds, missing.ProcessStartTimeSeconds = javaInt64(now.Unix()-1), javaInt64(now.Unix()-2), nil
		}, nil, nil},
		{"status", func(low, high, missing *javaapp.Service) {
			low.HealthUp, high.HealthUp, missing.HealthUp = javaBool(true), javaBool(false), nil
		}, []string{"id-low", "id-missing", "id-high"}, []string{"id-high", "id-missing", "id-low"}},
	}
	for _, test := range tests {
		for _, order := range []string{"asc", "desc"} {
			t.Run(test.field+"/"+order, func(t *testing.T) {
				low := healthyJavaService("id-low", "saas", "address-low", now)
				high := healthyJavaService("id-high", "saas", "address-high", now)
				missing := healthyJavaService("id-missing", "saas", "address-missing", now)
				test.mutate(&low, &high, &missing)
				page := mustJavaPage(t, newJavaServiceWithSnapshot(now, javaapp.Snapshot{Services: []javaapp.Service{missing, high, low}}), JavaQuery{
					Sort: test.field, Order: order, Page: 1, PageSize: 20,
				})
				want := test.asc
				if want == nil {
					want = []string{"id-low", "id-high", "id-missing"}
				}
				if order == "desc" {
					want = test.desc
					if want == nil {
						want = []string{"id-high", "id-low", "id-missing"}
					}
				}
				if got := javaIDs(page.Services); !reflect.DeepEqual(got, want) {
					t.Fatalf("IDs = %#v, want %#v", got, want)
				}
			})
		}
	}
}

func TestJavaServicesSortsMultipleMissingTextValuesLastByID(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 10, 0, time.UTC)
	tests := []struct {
		field  string
		mutate func(low, high, missingA, missingB *javaapp.Service)
	}{
		{"business", func(low, high, missingA, missingB *javaapp.Service) {
			low.Name, high.Name, missingA.Name, missingB.Name = "aaa", "zzz", " ", ""
		}},
		{"address", func(low, high, missingA, missingB *javaapp.Service) {
			low.Address, high.Address, missingA.Address, missingB.Address = "address-a", "address-z", " ", ""
		}},
	}
	for _, test := range tests {
		for _, order := range []string{"asc", "desc"} {
			t.Run(test.field+"/"+order, func(t *testing.T) {
				low := healthyJavaService("id-low", "saas", "address-low", now)
				high := healthyJavaService("id-high", "saas", "address-high", now)
				missingA := healthyJavaService("id-missing-a", "saas", "address-missing-a", now)
				missingB := healthyJavaService("id-missing-b", "saas", "address-missing-b", now)
				test.mutate(&low, &high, &missingA, &missingB)
				page := mustJavaPage(t, newJavaServiceWithSnapshot(now, javaapp.Snapshot{Services: []javaapp.Service{missingB, high, missingA, low}}), JavaQuery{
					Sort: test.field, Order: order, Page: 1, PageSize: 20,
				})
				want := []string{"id-low", "id-high", "id-missing-a", "id-missing-b"}
				if order == "desc" {
					want = []string{"id-high", "id-low", "id-missing-a", "id-missing-b"}
				}
				if got := javaIDs(page.Services); !reflect.DeepEqual(got, want) {
					t.Fatalf("IDs = %#v, want %#v", got, want)
				}
			})
		}
	}
}

func TestJavaServicesAlwaysUsesIDAscendingToCloseSortTies(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	services := []javaapp.Service{
		healthyJavaService("id-c", "tikbee", "same", now),
		healthyJavaService("id-a", "tikbee", "same", now),
		healthyJavaService("id-b", "tikbee", "same", now),
	}
	service := newJavaServiceWithSnapshot(now, javaapp.Snapshot{Services: services})
	for _, order := range []string{"asc", "desc"} {
		page := mustJavaPage(t, service, JavaQuery{Sort: "business", Order: order})
		if got, want := javaIDs(page.Services), []string{"id-a", "id-b", "id-c"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s tie IDs = %#v, want %#v", order, got, want)
		}
	}
}

func TestJavaServicesQueryDefaultsValidationPaginationAndOverflow(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	snapshot := javaapp.Snapshot{}
	for index := 0; index < 21; index++ {
		id := "id-" + string(rune('a'+index))
		snapshot.Services = append(snapshot.Services, healthyJavaService(id, "tikbee", id, now))
	}
	service := newJavaServiceWithSnapshot(now, snapshot)
	page := mustJavaPage(t, service, JavaQuery{})
	if page.Page != 1 || page.PageSize != 20 || page.Total != 21 || len(page.Services) != 20 {
		t.Fatalf("default page = %#v", page)
	}
	page = mustJavaPage(t, service, JavaQuery{Page: 2, PageSize: 20})
	if len(page.Services) != 1 || page.Services[0].ID != "id-u" {
		t.Fatalf("second page = %#v", page)
	}
	for _, pageSize := range []int{20, 50, 100} {
		if got := mustJavaPage(t, service, JavaQuery{Page: 1, PageSize: pageSize}); got.PageSize != pageSize {
			t.Fatalf("page size %d => %#v", pageSize, got)
		}
	}
	invalid := []JavaQuery{
		{Page: -1, PageSize: 20},
		{Page: 1, PageSize: 10},
		{Page: 1, PageSize: 20, Status: Level("bad")},
		{Page: 1, PageSize: 20, Sort: "bad"},
		{Page: 1, PageSize: 20, Order: "bad"},
		{Page: math.MaxInt, PageSize: 20},
	}
	for _, query := range invalid {
		if _, _, err := service.Services(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("query %#v error = %v, want ErrInvalidQuery", query, err)
		}
	}
}

func TestJavaServiceReturnsDetachedSnapshotValues(t *testing.T) {
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	provider := &recordingJavaProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{
		healthyJavaService("fixture-id", "tikbee", "fixture-address", now),
	}}}
	service := NewJava(provider, nil, JavaOptions{Clock: func() time.Time { return now }})
	first := mustJavaPage(t, service, JavaQuery{})
	*first.Services[0].HealthUp = false
	*first.Services[0].CPUUsagePercent = 999
	second := mustJavaPage(t, service, JavaQuery{})
	if !*second.Services[0].HealthUp || *second.Services[0].CPUUsagePercent == 999 {
		t.Fatalf("cached snapshot was mutated by caller: %#v", second.Services[0])
	}
}

type recordingJavaProvider struct {
	snapshot javaapp.Snapshot
	err      error
	calls    int
}

func (provider *recordingJavaProvider) JavaSnapshot(context.Context) (javaapp.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return javaapp.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type javaTestClock struct{ now time.Time }

func (clock *javaTestClock) Now() time.Time              { return clock.now }
func (clock *javaTestClock) Advance(value time.Duration) { clock.now = clock.now.Add(value) }

func healthyJavaService(id, name, address string, reportedAt time.Time) javaapp.Service {
	return javaapp.Service{
		ID:                        id,
		Name:                      name,
		Address:                   address,
		HealthLatencyMilliseconds: javaFloat(10),
		HealthUp:                  javaBool(true),
		PortUp:                    javaBool(true),
		ProcessUp:                 javaBool(true),
		PortConsistent:            javaBool(true),
		ProcessCount:              javaInt64(1),
		ProcessMemoryBytes:        javaInt64(1024),
		ProcessCPUPercent:         javaFloat(10),
		ProcessMemoryPercent:      javaFloat(20),
		ProcessStartTimeSeconds:   javaInt64(reportedAt.Unix() - 60),
		CollectionTracked:         true,
		ReportedAt:                reportedAt,
	}
}

func newJavaServiceWithSnapshot(now time.Time, snapshot javaapp.Snapshot) *JavaService {
	return NewJava(
		&recordingJavaProvider{snapshot: snapshot},
		cache.New(func() time.Time { return now }),
		JavaOptions{SnapshotTTL: time.Minute, CollectionInterval: 15 * time.Second, MaxStale: 5 * time.Minute, Clock: func() time.Time { return now }},
	)
}

func mustJavaPage(t *testing.T, service *JavaService, query JavaQuery) JavaPage {
	t.Helper()
	page, _, err := service.Services(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func javaIDs(services []JavaServiceSummary) []string {
	ids := make([]string, len(services))
	for index := range services {
		ids[index] = services[index].ID
	}
	return ids
}

func javaBool(value bool) *bool        { return &value }
func javaFloat(value float64) *float64 { return &value }
func javaInt64(value int64) *int64     { return &value }
