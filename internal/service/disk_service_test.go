package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/disk"
)

func TestDiskServiceStatusMatrix(t *testing.T) {
	float := func(value float64) *float64 { return &value }
	integer := func(value int64) *int64 { return &value }
	tests := []struct {
		name   string
		device disk.Device
		want   Level
	}{
		{name: "healthy", device: disk.Device{SMARTHealth: disk.HealthHealthy}, want: LevelNormal},
		{name: "failed health", device: disk.Device{SMARTHealth: disk.HealthFailed}, want: LevelCritical},
		{name: "missing health", device: disk.Device{SMARTHealth: disk.HealthUnknown}, want: LevelUnknown},
		{name: "critical warning", device: disk.Device{SMARTHealth: disk.HealthHealthy, CriticalWarning: integer(1)}, want: LevelCritical},
		{name: "spare equals threshold", device: disk.Device{SMARTHealth: disk.HealthHealthy, AvailableSparePercent: float(10), AvailableSpareThresholdPercent: float(10)}, want: LevelCritical},
		{name: "spare below threshold", device: disk.Device{SMARTHealth: disk.HealthHealthy, AvailableSparePercent: float(9), AvailableSpareThresholdPercent: float(10)}, want: LevelCritical},
		{name: "missing spare does not infer", device: disk.Device{SMARTHealth: disk.HealthHealthy, AvailableSpareThresholdPercent: float(10)}, want: LevelNormal},
		{name: "missing threshold does not infer", device: disk.Device{SMARTHealth: disk.HealthHealthy, AvailableSparePercent: float(9)}, want: LevelNormal},
		{name: "attribute failing now", device: disk.Device{SMARTHealth: disk.HealthHealthy, AttributeFailure: disk.AttributeFailureNow}, want: LevelCritical},
		{name: "attribute failed in past", device: disk.Device{SMARTHealth: disk.HealthHealthy, AttributeFailure: disk.AttributeFailurePast}, want: LevelWarning},
		{
			name: "telemetry does not change status",
			device: disk.Device{
				SMARTHealth:         disk.HealthHealthy,
				TemperatureCelsius:  float(999),
				LifetimeUsedPercent: float(999),
				PowerOnHours:        float(999999),
				Errors: disk.ErrorCounters{
					PendingSectors:       float(999),
					ReallocatedSectors:   float(999),
					UncorrectableSectors: float(999),
					UDMACRCErrors:        float(999),
					MediaIntegrityErrors: float(999),
					ErrorLogEntries:      float(999),
					UnsafeShutdowns:      float(999),
				},
			},
			want: LevelNormal,
		},
		{
			name: "critical outranks warning and unknown",
			device: disk.Device{
				SMARTHealth:      disk.HealthUnknown,
				AttributeFailure: disk.AttributeFailurePast,
				CriticalWarning:  integer(1),
			},
			want: LevelCritical,
		},
		{
			name: "warning outranks unknown",
			device: disk.Device{
				SMARTHealth:      disk.HealthUnknown,
				AttributeFailure: disk.AttributeFailurePast,
			},
			want: LevelWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.device.ID = "fixture-device"
			test.device.HostID = "fixture-host"
			test.device.Device = "/dev/fixture"
			provider := &diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{test.device}}}
			service := NewDisk(provider, nil, DiskOptions{})
			page, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
			if err != nil {
				t.Fatalf("Devices() error = %v", err)
			}
			if len(page.Devices) != 1 || page.Devices[0].Status != test.want {
				t.Fatalf("devices = %#v, want status %q", page.Devices, test.want)
			}
		})
	}
}

func TestDiskServiceMetaUsesLatestSampleTime(t *testing.T) {
	now := time.Date(2026, time.August, 7, 8, 0, 0, 0, time.UTC)
	older := now.Add(-2 * time.Minute)
	latest := now.Add(-time.Minute)
	first := oneHealthyDisk(older).Devices[0]
	second := oneHealthyDisk(latest).Devices[0]
	second.ID = "fixture-device-latest"
	provider := &diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{first, second}}}
	service := NewDisk(provider, cache.New(func() time.Time { return now }), DiskOptions{Clock: func() time.Time { return now }})

	_, meta, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if !meta.CollectedAt.Equal(latest) {
		t.Fatalf("CollectedAt = %s, want latest sample %s", meta.CollectedAt, latest)
	}
}

func TestMergeMetaUsesLatestSampleTimeRegardlessOfStaleState(t *testing.T) {
	older := time.Date(2026, time.August, 7, 7, 58, 0, 0, time.UTC)
	latest := older.Add(time.Minute)

	fresh := mergeMeta(Meta{CollectedAt: older}, Meta{CollectedAt: latest})
	if fresh.Stale || !fresh.CollectedAt.Equal(latest) {
		t.Fatalf("fresh meta = %#v, want latest time", fresh)
	}
	stale := mergeMeta(Meta{Stale: true, CollectedAt: older}, Meta{CollectedAt: latest})
	if !stale.Stale || !stale.CollectedAt.Equal(latest) {
		t.Fatalf("stale meta = %#v, want latest sample", stale)
	}
}

func TestDiskServiceCollectionLevelOnlyChangesWithObservedSampleProgress(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := &diskTestClock{now: now}
	sampleAt := now.Add(-24 * time.Hour)
	provider := &diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{{
		ID:                "fixture-device",
		HostID:            "fixture-host",
		Device:            "/dev/fixture",
		SMARTHealth:       disk.HealthHealthy,
		CollectionTracked: true,
		ReportedAt:        sampleAt,
	}}}}
	service := NewDisk(provider, cache.New(clock.Now), DiskOptions{
		SnapshotTTL:        time.Nanosecond,
		CollectionInterval: time.Minute,
		MaxStale:           10 * time.Minute,
		Clock:              clock.Now,
	})

	assertLevels := func(wantCollection, wantStatus Level) {
		t.Helper()
		page, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("Devices() error = %v", err)
		}
		if got := page.Devices[0]; got.CollectionLevel != wantCollection || got.Status != wantStatus {
			t.Fatalf("device = %#v, want collection/status %q/%q", got, wantCollection, wantStatus)
		}
	}

	assertLevels(LevelNormal, LevelNormal)
	clock.Advance(119*time.Second + 999*time.Millisecond)
	assertLevels(LevelNormal, LevelNormal)
	clock.Advance(time.Millisecond)
	assertLevels(LevelWarning, LevelWarning)
	clock.Advance(179*time.Second + 999*time.Millisecond)
	assertLevels(LevelWarning, LevelWarning)
	clock.Advance(time.Millisecond)
	assertLevels(LevelCritical, LevelCritical)

	provider.SetSnapshot(disk.Snapshot{Devices: []disk.Device{{
		ID:                "fixture-device",
		HostID:            "fixture-host",
		Device:            "/dev/fixture",
		SMARTHealth:       disk.HealthHealthy,
		CollectionTracked: true,
		ReportedAt:        sampleAt.Add(time.Minute),
	}}})
	clock.Advance(time.Nanosecond)
	assertLevels(LevelNormal, LevelNormal)

	provider.SetSnapshot(disk.Snapshot{Devices: []disk.Device{{
		ID:                "fixture-device",
		HostID:            "fixture-host",
		Device:            "/dev/fixture",
		SMARTHealth:       disk.HealthHealthy,
		CollectionTracked: true,
		ReportedAt:        sampleAt.Add(-time.Minute),
	}}})
	clock.Advance(time.Nanosecond)
	assertLevels(LevelNormal, LevelNormal)
}

func TestDiskServiceStatusSourceUsesStableDevicePriorityAndStrictCollectionDominance(t *testing.T) {
	integer := func(value int64) *int64 { return &value }
	tests := []struct {
		name       string
		device     disk.Device
		elapsed    time.Duration
		wantStatus Level
		wantSource string
	}{
		{
			name:       "pure collection warning",
			device:     disk.Device{SMARTHealth: disk.HealthHealthy},
			elapsed:    2 * time.Minute,
			wantStatus: LevelWarning,
			wantSource: "collection",
		},
		{
			name: "device warning wins critical collection tie",
			device: disk.Device{
				SMARTHealth:     disk.HealthHealthy,
				CriticalWarning: integer(1),
			},
			elapsed:    5 * time.Minute,
			wantStatus: LevelCritical,
			wantSource: "device_warning",
		},
		{
			name: "attribute failure wins warning collection tie",
			device: disk.Device{
				SMARTHealth:      disk.HealthHealthy,
				AttributeFailure: disk.AttributeFailurePast,
			},
			elapsed:    2 * time.Minute,
			wantStatus: LevelWarning,
			wantSource: "attribute_failure",
		},
		{
			name: "smart health has first device priority",
			device: disk.Device{
				SMARTHealth:      disk.HealthFailed,
				CriticalWarning:  integer(1),
				AttributeFailure: disk.AttributeFailureNow,
			},
			elapsed:    5 * time.Minute,
			wantStatus: LevelCritical,
			wantSource: "smart_health",
		},
		{
			name: "device warning precedes attribute failure",
			device: disk.Device{
				SMARTHealth:      disk.HealthHealthy,
				CriticalWarning:  integer(1),
				AttributeFailure: disk.AttributeFailureNow,
			},
			elapsed:    5 * time.Minute,
			wantStatus: LevelCritical,
			wantSource: "device_warning",
		},
		{
			name:       "unknown smart health remains attributable",
			device:     disk.Device{SMARTHealth: disk.HealthUnknown},
			wantStatus: LevelUnknown,
			wantSource: "smart_health",
		},
		{
			name:       "all normal",
			device:     disk.Device{SMARTHealth: disk.HealthHealthy},
			wantStatus: LevelNormal,
			wantSource: "normal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
			clock := &diskTestClock{now: now}
			test.device.ID = "fixture-device"
			test.device.HostID = "fixture-host"
			test.device.Device = "/dev/fixture"
			test.device.CollectionTracked = true
			test.device.ReportedAt = now.Add(-time.Hour)
			provider := &diskTestProvider{
				snapshot: disk.Snapshot{Devices: []disk.Device{test.device}},
			}
			service := NewDisk(provider, cache.New(clock.Now), DiskOptions{
				SnapshotTTL:        time.Nanosecond,
				CollectionInterval: time.Minute,
				Clock:              clock.Now,
			})

			if _, _, err := service.Devices(
				context.Background(),
				DiskQuery{Page: 1, PageSize: 20},
			); err != nil {
				t.Fatalf("initial Devices() error = %v", err)
			}
			clock.Advance(test.elapsed)
			page, _, err := service.Devices(
				context.Background(),
				DiskQuery{Page: 1, PageSize: 20},
			)
			if err != nil {
				t.Fatalf("Devices() error = %v", err)
			}
			got := page.Devices[0]
			if got.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, test.wantStatus)
			}
			if source := string(got.StatusSource); source != test.wantSource {
				t.Fatalf("status source = %q, want %q", source, test.wantSource)
			}
		})
	}
}

func TestDiskServiceCacheHitSharesSnapshotWithoutReloading(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := &diskTestClock{now: now}
	provider := &diskTestProvider{snapshot: oneHealthyDisk(now)}
	service := NewDisk(provider, cache.New(clock.Now), DiskOptions{
		SnapshotTTL:        time.Minute,
		CollectionInterval: time.Minute,
		Clock:              clock.Now,
	})

	if _, _, err := service.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if calls := provider.Calls(); calls != 1 {
		t.Fatalf("SMARTSnapshot calls = %d, want 1", calls)
	}
}

func TestDiskServiceFailedRefreshReturnsAgingStaleSnapshot(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := &diskTestClock{now: now}
	provider := &diskTestProvider{snapshot: oneHealthyDisk(now)}
	service := NewDisk(provider, cache.New(clock.Now), DiskOptions{
		SnapshotTTL:        time.Nanosecond,
		CollectionInterval: time.Minute,
		MaxStale:           10 * time.Minute,
		Clock:              clock.Now,
	})
	if _, _, err := service.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}

	provider.SetError(disk.ErrUnavailable)
	clock.Advance(2 * time.Minute)
	page, meta, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
	if err != nil || !meta.Stale || page.Devices[0].CollectionLevel != LevelWarning {
		t.Fatalf("page/meta/error = %#v/%#v/%v", page, meta, err)
	}

	clock.Advance(3 * time.Minute)
	page, meta, err = service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
	if err != nil || !meta.Stale || page.Devices[0].CollectionLevel != LevelCritical {
		t.Fatalf("page/meta/error = %#v/%#v/%v", page, meta, err)
	}
}

func TestDiskServiceReturnsProviderErrorWithoutCachedSnapshot(t *testing.T) {
	provider := &diskTestProvider{err: disk.ErrUnavailable}
	service := NewDisk(provider, nil, DiskOptions{})
	_, _, err := service.Overview(context.Background())
	if !errors.Is(err, disk.ErrUnavailable) {
		t.Fatalf("Overview() error = %v, want %v", err, disk.ErrUnavailable)
	}
}

func TestDiskServiceOverviewCountsStatusesAndDeduplicatedAlerts(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := &diskTestClock{now: now}
	one := int64(1)
	spare := float64(10)
	threshold := float64(20)
	provider := &diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{
		{ID: "normal", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthHealthy},
		{ID: "unknown", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthUnknown},
		{ID: "health-critical", HostID: "host", Device: "disk3", SMARTHealth: disk.HealthFailed},
		{
			ID: "warning-critical", HostID: "host", Device: "disk4", SMARTHealth: disk.HealthHealthy,
			CriticalWarning: &one, AvailableSparePercent: &spare, AvailableSpareThresholdPercent: &threshold,
		},
		{
			ID: "attribute-warning", HostID: "host", Device: "disk5", SMARTHealth: disk.HealthHealthy,
			AttributeFailure: disk.AttributeFailurePast,
		},
		{
			ID: "collection-warning", HostID: "host", Device: "disk6", SMARTHealth: disk.HealthHealthy,
			CollectionTracked: true, ReportedAt: now.Add(-24 * time.Hour),
		},
	}}}
	service := NewDisk(provider, cache.New(clock.Now), DiskOptions{
		SnapshotTTL:        time.Nanosecond,
		CollectionInterval: time.Minute,
		MaxStale:           10 * time.Minute,
		Clock:              clock.Now,
	})
	if _, _, err := service.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Minute)
	overview, _, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := DiskOverview{
		Total: 6, Normal: 1, Warning: 2, Critical: 2, Unknown: 1,
		AffectedDevices: 5, WarningDevices: 3, CriticalDevices: 2,
		Alerts: DiskOverviewAlerts{
			SMARTHealth:      AlertCount{Critical: 1},
			DeviceWarning:    AlertCount{Critical: 1},
			AttributeFailure: AlertCount{Warning: 1},
			Collection:       AlertCount{Warning: 1},
		},
	}
	if overview != want {
		t.Fatalf("Overview() = %#v, want %#v", overview, want)
	}
}

func TestDiskServiceQuerySearchNaturalSortAndPagination(t *testing.T) {
	devices := make([]disk.Device, 0, 23)
	for index := 1; index <= 21; index++ {
		devices = append(devices, disk.Device{
			ID:          fmt.Sprintf("id-%02d", index),
			HostID:      "host2",
			Device:      fmt.Sprintf("disk%d", index),
			Model:       "fixture-model",
			SMARTHealth: disk.HealthHealthy,
		})
	}
	devices = append(devices,
		disk.Device{ID: "id-host10", HostID: "host10", Device: "disk1", Model: "other-model", SMARTHealth: disk.HealthHealthy},
		disk.Device{ID: "secret-only-id", HostID: "host3", Device: "disk1", Model: "other-model", SMARTHealth: disk.HealthHealthy},
	)
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	first, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 23 || first.Page != 1 || first.PageSize != 20 || first.TotalPages != 2 || len(first.Devices) != 20 {
		t.Fatalf("page = %#v", first)
	}
	if first.Devices[0].Device != "disk1" || first.Devices[1].Device != "disk2" ||
		first.Devices[9].Device != "disk10" || first.Devices[19].Device != "disk20" {
		t.Fatalf("natural order = %#v", diskDeviceNames(first.Devices))
	}
	second, _, err := service.Devices(context.Background(), DiskQuery{Page: 2, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Devices) != 3 || second.Devices[0].Device != "disk21" ||
		second.Devices[1].Host != "host3" || second.Devices[2].Host != "host10" {
		t.Fatalf("second page = %#v", second.Devices)
	}

	for _, search := range []string{"HOST10", "DISK21", "FIXTURE-MODEL"} {
		page, _, queryErr := service.Devices(context.Background(), DiskQuery{Search: search, Page: 1, PageSize: 100})
		if queryErr != nil || page.Total == 0 {
			t.Fatalf("search %q page/error = %#v/%v", search, page, queryErr)
		}
	}
	page, _, err := service.Devices(context.Background(), DiskQuery{Search: "secret-only-id", Page: 1, PageSize: 20})
	if err != nil || page.Total != 0 {
		t.Fatalf("ID-only search page/error = %#v/%v", page, err)
	}
}

func TestDiskServiceQueryValidationAndPageSizes(t *testing.T) {
	service := NewDisk(&diskTestProvider{}, nil, DiskOptions{})
	for _, pageSize := range []int{20, 50, 100, 500} {
		if _, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: pageSize}); err != nil {
			t.Fatalf("page size %d error = %v", pageSize, err)
		}
	}
	for _, pageSize := range []int{0, 1, 19, 21, 499, 501, -1} {
		if _, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: pageSize}); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("page size %d error = %v, want ErrInvalidQuery", pageSize, err)
		}
	}
	tests := []DiskQuery{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 10},
		{Status: Level("other"), Page: 1, PageSize: 20},
		{Sort: "host_device", Page: 1, PageSize: 20},
		{Order: "sideways", Page: 1, PageSize: 20},
	}
	for _, query := range tests {
		if _, _, err := service.Devices(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("Devices(%#v) error = %v, want ErrInvalidQuery", query, err)
		}
	}
	for _, status := range []Level{"", LevelNormal, LevelWarning, LevelCritical, LevelUnknown} {
		for _, sortField := range []string{"", "host", "device", "model", "capacity", "smart", "temperature", "lifetime", "power_on_hours", "errors", "status"} {
			for _, order := range []string{"", "asc", "desc"} {
				_, _, err := service.Devices(context.Background(), DiskQuery{
					Status: status, Sort: sortField, Order: order, Page: 1, PageSize: 20,
				})
				if err != nil {
					t.Fatalf("valid query status/sort/order %q/%q/%q error = %v", status, sortField, order, err)
				}
			}
		}
	}
}

func TestDiskServicePageSize500PaginatesAfterDescendingSort(t *testing.T) {
	devices := make([]disk.Device, 0, 501)
	for index := 1; index <= 501; index++ {
		value := fmt.Sprintf("disk-%03d", index)
		devices = append(devices, disk.Device{ID: value, HostID: "fixture-host", Device: value, SMARTHealth: disk.HealthHealthy})
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})
	first, _, err := service.Devices(context.Background(), DiskQuery{Sort: "device", Order: "desc", Page: 1, PageSize: 500})
	if err != nil || first.Total != 501 || len(first.Devices) != 500 || first.Devices[499].Device != "disk-002" {
		t.Fatalf("first page = %#v, err = %v", first, err)
	}
	second, _, err := service.Devices(context.Background(), DiskQuery{Sort: "device", Order: "desc", Page: 2, PageSize: 500})
	if err != nil || len(second.Devices) != 1 || second.Devices[0].Device != "disk-001" {
		t.Fatalf("second page = %#v, err = %v", second, err)
	}
}

func TestDiskServiceRejectsPageOffsetOverflow(t *testing.T) {
	service := NewDisk(&diskTestProvider{}, nil, DiskOptions{})
	maxInt := int(^uint(0) >> 1)

	_, _, err := service.Devices(context.Background(), DiskQuery{
		Page: maxInt, PageSize: 20,
	})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Devices() error = %v, want ErrInvalidQuery", err)
	}
}

func TestDiskServiceFiltersBeforeSortingAndPaging(t *testing.T) {
	devices := make([]disk.Device, 0, 22)
	for index := 0; index < 21; index++ {
		devices = append(devices, disk.Device{
			ID: fmt.Sprintf("normal-%02d", index), HostID: "host", Device: fmt.Sprintf("disk%d", index),
			SMARTHealth: disk.HealthHealthy,
		})
	}
	devices = append(devices, disk.Device{
		ID: "critical", HostID: "host", Device: "disk99", SMARTHealth: disk.HealthFailed,
	})
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})
	page, _, err := service.Devices(context.Background(), DiskQuery{
		Status: LevelCritical, Sort: "device", Order: "desc", Page: 1, PageSize: 20,
	})
	if err != nil || page.Total != 1 || len(page.Devices) != 1 || page.Devices[0].ID != "critical" {
		t.Fatalf("page/error = %#v/%v", page, err)
	}
}

func TestDiskServiceNumericSortKeepsMissingValuesLast(t *testing.T) {
	value1 := float64(1)
	value2 := float64(2)
	tests := []struct {
		field string
		set   func(*disk.Device, *float64)
	}{
		{field: "temperature", set: func(device *disk.Device, value *float64) { device.TemperatureCelsius = value }},
		{field: "lifetime", set: func(device *disk.Device, value *float64) { device.LifetimeUsedPercent = value }},
		{field: "power_on_hours", set: func(device *disk.Device, value *float64) { device.PowerOnHours = value }},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			devices := []disk.Device{
				{ID: "missing", HostID: "host", Device: "disk0", SMARTHealth: disk.HealthHealthy},
				{ID: "one", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthHealthy},
				{ID: "two", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy},
			}
			test.set(&devices[1], &value1)
			test.set(&devices[2], &value2)
			service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})
			for _, order := range []string{"asc", "desc"} {
				page, _, err := service.Devices(context.Background(), DiskQuery{
					Sort: test.field, Order: order, Page: 1, PageSize: 20,
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := page.Devices[len(page.Devices)-1].ID; got != "missing" {
					t.Fatalf("%s order IDs = %#v, missing item is not last", order, diskDeviceIDs(page.Devices))
				}
				if order == "asc" && page.Devices[0].ID != "one" {
					t.Fatalf("ascending IDs = %#v", diskDeviceIDs(page.Devices))
				}
				if order == "desc" && page.Devices[0].ID != "two" {
					t.Fatalf("descending IDs = %#v", diskDeviceIDs(page.Devices))
				}
			}
		})
	}
}

func TestDiskServiceCapacitySortKeepsMissingLastAndUsesStableTieBreaker(t *testing.T) {
	one := int64(1)
	two := int64(2)
	devices := []disk.Device{
		{ID: "missing", HostID: "host", Device: "disk0", SMARTHealth: disk.HealthHealthy},
		{ID: "two-b", HostID: "host", Device: "disk2", CapacityBytes: &two, SMARTHealth: disk.HealthHealthy},
		{ID: "one", HostID: "host", Device: "disk1", CapacityBytes: &one, SMARTHealth: disk.HealthHealthy},
		{ID: "two-a", HostID: "host", Device: "disk3", CapacityBytes: &two, SMARTHealth: disk.HealthHealthy},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})
	for _, test := range []struct {
		order string
		want  string
	}{
		{order: "asc", want: "[one two-a two-b missing]"},
		{order: "desc", want: "[two-a two-b one missing]"},
	} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: "capacity", Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != test.want {
			t.Fatalf("%s IDs = %s, want %s", test.order, got, test.want)
		}
	}
}

func TestDiskServiceModelSortUsesNaturalOrderKeepsBlankLastAndUsesIDTieBreaker(t *testing.T) {
	devices := []disk.Device{
		{ID: "id-model-10", HostID: "host", Device: "disk10", Model: "Disk 10", SMARTHealth: disk.HealthHealthy},
		{ID: "id-missing", HostID: "host", Device: "disk0", Model: " \t", SMARTHealth: disk.HealthHealthy},
		{ID: "id-model-2b", HostID: "host", Device: "disk2b", Model: "disk 2", SMARTHealth: disk.HealthHealthy},
		{ID: "id-model-2a", HostID: "host", Device: "disk2a", Model: "Disk 2", SMARTHealth: disk.HealthHealthy},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	for _, test := range []struct {
		order string
		want  string
	}{
		{order: "asc", want: "[id-model-2a id-model-2b id-model-10 id-missing]"},
		{order: "desc", want: "[id-model-10 id-model-2a id-model-2b id-missing]"},
	} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: "model", Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("%s: %v", test.order, err)
		}
		if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != test.want {
			t.Fatalf("%s IDs = %s, want %s", test.order, got, test.want)
		}
	}
}

func TestDiskServiceSMARTSortUsesBusinessRankAndIDTieBreaker(t *testing.T) {
	devices := []disk.Device{
		{ID: "id-unknown", HostID: "host", Device: "disk0", SMARTHealth: disk.HealthUnknown},
		{ID: "id-failed-b", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthFailed},
		{ID: "id-healthy-b", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy},
		{ID: "id-failed-a", HostID: "host", Device: "disk3", SMARTHealth: disk.HealthFailed},
		{ID: "id-healthy-a", HostID: "host", Device: "disk4", SMARTHealth: disk.HealthHealthy},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	for _, test := range []struct {
		order string
		want  string
	}{
		{order: "asc", want: "[id-healthy-a id-healthy-b id-failed-a id-failed-b id-unknown]"},
		{order: "desc", want: "[id-unknown id-failed-a id-failed-b id-healthy-a id-healthy-b]"},
	} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: "smart", Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("%s: %v", test.order, err)
		}
		if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != test.want {
			t.Fatalf("%s IDs = %s, want %s", test.order, got, test.want)
		}
	}
}

func TestDiskServiceErrorSortUsesDisplayedCountersKeepsMissingLastAndUsesIDTieBreaker(t *testing.T) {
	zero := float64(0)
	one := float64(1)
	two := float64(2)
	unsafe := float64(999)
	devices := []disk.Device{
		{ID: "id-missing", HostID: "host", Device: "disk0", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{UnsafeShutdowns: &unsafe}},
		{ID: "id-two-b", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{PendingSectors: &two}},
		{ID: "id-one", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{PendingSectors: &one, UnsafeShutdowns: &unsafe}},
		{ID: "id-zero", HostID: "host", Device: "disk3", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{PendingSectors: &zero}},
		{ID: "id-two-a", HostID: "host", Device: "disk4", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{ReallocatedSectors: &one, ErrorLogEntries: &one}},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	for _, test := range []struct {
		order string
		want  string
	}{
		{order: "asc", want: "[id-zero id-one id-two-a id-two-b id-missing]"},
		{order: "desc", want: "[id-two-a id-two-b id-one id-zero id-missing]"},
	} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: "errors", Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("%s: %v", test.order, err)
		}
		if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != test.want {
			t.Fatalf("%s IDs = %s, want %s", test.order, got, test.want)
		}
	}
}

func TestDiskServiceErrorSortIncludesEveryDisplayedCounter(t *testing.T) {
	one := float64(1)
	two := float64(2)
	tests := []struct {
		name string
		set  func(*disk.ErrorCounters, *float64)
	}{
		{name: "pending sectors", set: func(errors *disk.ErrorCounters, value *float64) { errors.PendingSectors = value }},
		{name: "reallocated sectors", set: func(errors *disk.ErrorCounters, value *float64) { errors.ReallocatedSectors = value }},
		{name: "uncorrectable sectors", set: func(errors *disk.ErrorCounters, value *float64) { errors.UncorrectableSectors = value }},
		{name: "udma crc errors", set: func(errors *disk.ErrorCounters, value *float64) { errors.UDMACRCErrors = value }},
		{name: "media integrity errors", set: func(errors *disk.ErrorCounters, value *float64) { errors.MediaIntegrityErrors = value }},
		{name: "error log entries", set: func(errors *disk.ErrorCounters, value *float64) { errors.ErrorLogEntries = value }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lowerErrors := disk.ErrorCounters{}
			test.set(&lowerErrors, &one)
			higherErrors := disk.ErrorCounters{}
			test.set(&higherErrors, &two)
			service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{
				{ID: "id-a-higher", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthHealthy, Errors: higherErrors},
				{ID: "id-z-lower", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy, Errors: lowerErrors},
			}}}, nil, DiskOptions{})

			page, _, err := service.Devices(context.Background(), DiskQuery{
				Sort: "errors", Order: "asc", Page: 1, PageSize: 20,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != "[id-z-lower id-a-higher]" {
				t.Fatalf("IDs = %s, want displayed counter to set ascending order", got)
			}
		})
	}
}

func TestDiskServiceErrorSortExcludesUnsafeShutdowns(t *testing.T) {
	zero := float64(0)
	one := float64(1)
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: []disk.Device{
		{ID: "id-a-unsafe-only", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{UnsafeShutdowns: &zero}},
		{ID: "id-z-displayed", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy, Errors: disk.ErrorCounters{PendingSectors: &one}},
	}}}, nil, DiskOptions{})

	page, _, err := service.Devices(context.Background(), DiskQuery{
		Sort: "errors", Order: "asc", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != "[id-z-displayed id-a-unsafe-only]" {
		t.Fatalf("IDs = %s, want unsafe shutdowns excluded and unavailable values last", got)
	}
}

func TestDiskServiceExplicitTextAndStatusSorts(t *testing.T) {
	devices := []disk.Device{
		{ID: "critical", HostID: "host10", Device: "disk10", SMARTHealth: disk.HealthFailed},
		{ID: "normal", HostID: "host2", Device: "disk2", SMARTHealth: disk.HealthHealthy},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})
	tests := []struct {
		field string
		order string
		want  string
	}{
		{field: "host", order: "asc", want: "normal"},
		{field: "device", order: "asc", want: "normal"},
		{field: "status", order: "asc", want: "normal"},
		{field: "status", order: "desc", want: "critical"},
	}
	for _, test := range tests {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: test.field, Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil || page.Devices[0].ID != test.want {
			t.Errorf("%s/%s page/error = %#v/%v, want first %q", test.field, test.order, page, err, test.want)
		}
	}
}

func TestDiskServiceStatusSortUsesListRankAndIDTieBreaker(t *testing.T) {
	devices := []disk.Device{
		{ID: "id-critical", HostID: "host", Device: "disk1", SMARTHealth: disk.HealthFailed},
		{ID: "id-warning-b", HostID: "host", Device: "disk2", SMARTHealth: disk.HealthHealthy, AttributeFailure: disk.AttributeFailurePast},
		{ID: "id-normal", HostID: "host", Device: "disk3", SMARTHealth: disk.HealthHealthy},
		{ID: "id-unknown", HostID: "host", Device: "disk4", SMARTHealth: disk.HealthUnknown},
		{ID: "id-warning-a", HostID: "host", Device: "disk5", SMARTHealth: disk.HealthHealthy, AttributeFailure: disk.AttributeFailurePast},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	for _, test := range []struct {
		order string
		want  string
	}{
		{order: "asc", want: "[id-normal id-warning-a id-warning-b id-critical id-unknown]"},
		{order: "desc", want: "[id-unknown id-critical id-warning-a id-warning-b id-normal]"},
	} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: "status", Order: test.order, Page: 1, PageSize: 20,
		})
		if err != nil {
			t.Fatalf("%s: %v", test.order, err)
		}
		if got := fmt.Sprint(diskDeviceIDs(page.Devices)); got != test.want {
			t.Fatalf("%s IDs = %s, want %s", test.order, got, test.want)
		}
	}
}

func TestDiskServiceHostSortUsesNaturalDeviceTieBreaker(t *testing.T) {
	devices := []disk.Device{
		{ID: "a-stable-hash", HostID: "host2", Device: "disk10", SMARTHealth: disk.HealthHealthy},
		{ID: "z-stable-hash", HostID: "host2", Device: "disk2", SMARTHealth: disk.HealthHealthy},
	}
	service := NewDisk(&diskTestProvider{snapshot: disk.Snapshot{Devices: devices}}, nil, DiskOptions{})

	page, _, err := service.Devices(context.Background(), DiskQuery{
		Sort: "host", Order: "asc", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := diskDeviceIDs(page.Devices); fmt.Sprint(got) != "[z-stable-hash a-stable-hash]" {
		t.Fatalf("host ascending IDs = %v, want device natural order", got)
	}
}

func TestDiskServiceReturnsDeepCopiesOfSnapshotAndSummaries(t *testing.T) {
	capacity := int64(100)
	criticalWarning := int64(1)
	temperature := float64(20)
	lifetime := float64(30)
	powerOnHours := float64(40)
	spare := float64(50)
	threshold := float64(10)
	errorValue := float64(60)
	source := disk.Snapshot{Devices: []disk.Device{{
		ID: "fixture", HostID: "host", Device: "disk", Model: "model",
		CapacityBytes: &capacity, SMARTHealth: disk.HealthHealthy,
		TemperatureCelsius: &temperature, LifetimeUsedPercent: &lifetime, PowerOnHours: &powerOnHours,
		CriticalWarning: &criticalWarning, AvailableSparePercent: &spare, AvailableSpareThresholdPercent: &threshold,
		Errors: disk.ErrorCounters{
			PendingSectors: &errorValue, ReallocatedSectors: &errorValue, UncorrectableSectors: &errorValue,
			UDMACRCErrors: &errorValue, MediaIntegrityErrors: &errorValue, ErrorLogEntries: &errorValue,
			UnsafeShutdowns: &errorValue,
		},
	}}}
	service := NewDisk(&diskTestProvider{snapshot: source}, nil, DiskOptions{})

	firstSnapshot, _, err := service.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mutateDiskPointers(&firstSnapshot.Devices[0])
	secondSnapshot, _, err := service.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDiskPointersNotMutated(t, secondSnapshot.Devices[0])

	firstPage, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	mutateDiskSummaryPointers(&firstPage.Devices[0])
	secondPage, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	assertDiskSummaryPointersNotMutated(t, secondPage.Devices[0])
}

func TestDiskServiceSupportsConcurrentOverviewAndDevices(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	clock := &diskTestClock{now: now}
	service := NewDisk(&diskTestProvider{snapshot: oneHealthyDisk(now)}, cache.New(clock.Now), DiskOptions{
		SnapshotTTL:        time.Nanosecond,
		CollectionInterval: time.Minute,
		Clock:              clock.Now,
	})
	var wait sync.WaitGroup
	for index := 0; index < 32; index++ {
		wait.Add(2)
		go func() {
			defer wait.Done()
			if _, _, err := service.Overview(context.Background()); err != nil {
				t.Errorf("Overview() error = %v", err)
			}
		}()
		go func() {
			defer wait.Done()
			if _, _, err := service.Devices(context.Background(), DiskQuery{Page: 1, PageSize: 20}); err != nil {
				t.Errorf("Devices() error = %v", err)
			}
		}()
	}
	wait.Wait()
}

type diskTestProvider struct {
	mu       sync.Mutex
	snapshot disk.Snapshot
	err      error
	calls    int
}

func (p *diskTestProvider) SMARTSnapshot(context.Context) (disk.Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.snapshot, p.err
}

func (p *diskTestProvider) SetSnapshot(snapshot disk.Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot = snapshot
	p.err = nil
}

func (p *diskTestProvider) SetError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *diskTestProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type diskTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *diskTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *diskTestClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func oneHealthyDisk(reportedAt time.Time) disk.Snapshot {
	return disk.Snapshot{Devices: []disk.Device{{
		ID:                "fixture-device",
		HostID:            "fixture-host",
		Device:            "disk1",
		SMARTHealth:       disk.HealthHealthy,
		CollectionTracked: true,
		ReportedAt:        reportedAt,
	}}}
}

func diskDeviceNames(devices []DiskDeviceSummary) []string {
	names := make([]string, len(devices))
	for index := range devices {
		names[index] = devices[index].Device
	}
	return names
}

func diskDeviceIDs(devices []DiskDeviceSummary) []string {
	ids := make([]string, len(devices))
	for index := range devices {
		ids[index] = devices[index].ID
	}
	return ids
}

func mutateDiskPointers(device *disk.Device) {
	*device.CapacityBytes = 999
	*device.TemperatureCelsius = 999
	*device.LifetimeUsedPercent = 999
	*device.PowerOnHours = 999
	*device.CriticalWarning = 999
	*device.AvailableSparePercent = 999
	*device.AvailableSpareThresholdPercent = 999
	*device.Errors.PendingSectors = 999
	*device.Errors.ReallocatedSectors = 999
	*device.Errors.UncorrectableSectors = 999
	*device.Errors.UDMACRCErrors = 999
	*device.Errors.MediaIntegrityErrors = 999
	*device.Errors.ErrorLogEntries = 999
	*device.Errors.UnsafeShutdowns = 999
}

func assertDiskPointersNotMutated(t *testing.T, device disk.Device) {
	t.Helper()
	if *device.CapacityBytes == 999 || *device.TemperatureCelsius == 999 ||
		*device.LifetimeUsedPercent == 999 || *device.PowerOnHours == 999 ||
		*device.CriticalWarning == 999 || *device.AvailableSparePercent == 999 ||
		*device.AvailableSpareThresholdPercent == 999 ||
		*device.Errors.PendingSectors == 999 || *device.Errors.ReallocatedSectors == 999 ||
		*device.Errors.UncorrectableSectors == 999 || *device.Errors.UDMACRCErrors == 999 ||
		*device.Errors.MediaIntegrityErrors == 999 || *device.Errors.ErrorLogEntries == 999 ||
		*device.Errors.UnsafeShutdowns == 999 {
		t.Fatalf("cached snapshot was mutated: %#v", device)
	}
}

func mutateDiskSummaryPointers(device *DiskDeviceSummary) {
	*device.CapacityBytes = 999
	*device.TemperatureCelsius = 999
	*device.LifetimeUsedPercent = 999
	*device.PowerOnHours = 999
	*device.Errors.PendingSectors = 999
	*device.Errors.ReallocatedSectors = 999
	*device.Errors.UncorrectableSectors = 999
	*device.Errors.UDMACRCErrors = 999
	*device.Errors.MediaIntegrityErrors = 999
	*device.Errors.ErrorLogEntries = 999
	*device.Errors.UnsafeShutdowns = 999
}

func assertDiskSummaryPointersNotMutated(t *testing.T, device DiskDeviceSummary) {
	t.Helper()
	if *device.CapacityBytes == 999 || *device.TemperatureCelsius == 999 ||
		*device.LifetimeUsedPercent == 999 || *device.PowerOnHours == 999 ||
		*device.Errors.PendingSectors == 999 || *device.Errors.ReallocatedSectors == 999 ||
		*device.Errors.UncorrectableSectors == 999 || *device.Errors.UDMACRCErrors == 999 ||
		*device.Errors.MediaIntegrityErrors == 999 || *device.Errors.ErrorLogEntries == 999 ||
		*device.Errors.UnsafeShutdowns == 999 {
		t.Fatalf("returned summary was mutated: %#v", device)
	}
}
