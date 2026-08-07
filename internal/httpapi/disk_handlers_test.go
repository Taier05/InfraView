package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/disk"
	"github.com/Taier05/InfraView/internal/service"
)

func TestDiskRoutesRequireAuthentication(t *testing.T) {
	handler, _ := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	for _, path := range []string{"/api/v1/disks/overview", "/api/v1/disks/devices"} {
		response := request(t, handler, http.MethodGet, path, "", nil)
		assertError(t, response, http.StatusUnauthorized, "unauthorized", "请先登录")
	}
}

func TestDiskOverviewReturnsExactReadOnlyViewAndMeta(t *testing.T) {
	handler, sessionCookie := newDiskAPITestHandler(t, disk.Snapshot{})
	response := request(t, handler, http.MethodGet, "/api/v1/disks/overview", "", sessionCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(
		t,
		body,
		"data",
		"total",
		"normal",
		"warning",
		"critical",
		"unknown",
		"affected_devices",
		"warning_devices",
		"critical_devices",
		"alerts",
	)
	assertJSONObjectKeys(t, body, "data.alerts", "smart_health", "device_warning", "attribute_failure", "collection")
	for _, category := range []string{"smart_health", "device_warning", "attribute_failure", "collection"} {
		assertJSONObjectKeys(t, body, "data.alerts."+category, "warning", "critical")
		for _, level := range []string{"warning", "critical"} {
			if got := jsonPathValue(t, body, "data.alerts."+category+"."+level); got != float64(0) {
				t.Fatalf("%s.%s = %#v, want 0", category, level, got)
			}
		}
	}
	for _, field := range []string{
		"total",
		"normal",
		"warning",
		"critical",
		"unknown",
		"affected_devices",
		"warning_devices",
		"critical_devices",
	} {
		if got := jsonPathValue(t, body, "data."+field); got != float64(0) {
			t.Fatalf("%s = %#v, want 0", field, got)
		}
	}
	assertJSONObjectKeys(t, body, "meta", "request_id", "stale")
	if !jsonPathIsString(t, body, "meta.request_id") {
		t.Fatal("empty disk response request_id is invalid")
	}
	if _, ok := jsonPathValue(t, body, "meta.stale").(bool); !ok {
		t.Fatal("empty disk response stale is not a boolean")
	}
}

func TestDiskOverviewReturnsStaleMetaAfterRefreshFailure(t *testing.T) {
	now := testNow()
	clock := func() time.Time { return now }
	snapshot := fixtureDiskSnapshot()
	for index := range snapshot.Devices {
		snapshot.Devices[index].ReportedAt = now.Add(-time.Minute)
	}
	provider := &diskSnapshotProvider{snapshot: snapshot}
	handler, sessionCookie := newDiskAPIProviderTestHandler(t, provider, clock)

	first := request(t, handler, http.MethodGet, "/api/v1/disks/overview", "", sessionCookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	firstCollectedAt := jsonPathValue(t, first.Body.Bytes(), "meta.collected_at")

	now = now.Add(2 * time.Minute)
	provider.err = errors.Join(disk.ErrUnavailable, errors.New("upstream-test-secret"))
	stale := request(t, handler, http.MethodGet, "/api/v1/disks/overview", "", sessionCookie)
	if stale.Code != http.StatusOK {
		t.Fatalf("stale status = %d, body = %s", stale.Code, stale.Body.String())
	}
	if got := jsonPathValue(t, stale.Body.Bytes(), "meta.stale"); got != true {
		t.Fatalf("meta.stale = %#v, want true", got)
	}
	if got := jsonPathValue(t, stale.Body.Bytes(), "meta.collected_at"); got != firstCollectedAt {
		t.Fatalf("collected_at = %#v, want %#v", got, firstCollectedAt)
	}
}

func TestDiskOverviewRejectsEveryQueryParameter(t *testing.T) {
	handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	for _, query := range []string{"?search=fixture", "?status=normal", "?unknown=value"} {
		response := request(t, handler, http.MethodGet, "/api/v1/disks/overview"+query, "", sessionCookie)
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	}
}

func TestDiskRoutesRejectMutationMethods(t *testing.T) {
	handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		for _, path := range []string{"/api/v1/disks/overview", "/api/v1/disks/devices"} {
			response := request(t, handler, method, path, "", sessionCookie)
			if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
				t.Fatalf("%s %s status=%d allow=%q", method, path, response.Code, response.Header().Get("Allow"))
			}
		}
	}
}

func TestDiskRoutesFailSafelyWithoutService(t *testing.T) {
	handler := newDiskAPIHandler(t, nil, testNow)
	sessionCookie := loginCookie(t, handler)
	for _, path := range []string{"/api/v1/disks/overview", "/api/v1/disks/devices"} {
		response := request(t, handler, http.MethodGet, path, "", sessionCookie)
		assertRetryableDiskUnavailable(t, response)
	}
}

func TestDiskUnavailableMapsToSafeRetryable503(t *testing.T) {
	provider := &diskSnapshotProvider{
		err: errors.Join(disk.ErrUnavailable, errors.New("upstream-test-secret serial_no wwn labels promql")),
	}
	handler, sessionCookie := newDiskAPIProviderTestHandler(t, provider, testNow)
	for _, path := range []string{"/api/v1/disks/overview", "/api/v1/disks/devices"} {
		response := request(t, handler, http.MethodGet, path, "", sessionCookie)
		assertRetryableDiskUnavailable(t, response)
		for _, forbidden := range []string{"upstream-test-secret", "serial_no", "wwn", "labels", "promql"} {
			if strings.Contains(strings.ToLower(response.Body.String()), forbidden) {
				t.Fatalf("%s leaked %q: %s", path, forbidden, response.Body.String())
			}
		}
	}
}

func TestDiskDevicesReturnsFixedViewDefaultsAndPreservesNulls(t *testing.T) {
	handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	response := request(t, handler, http.MethodGet, "/api/v1/disks/devices", "", sessionCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "devices", "total", "page", "page_size", "total_pages")
	for path, want := range map[string]float64{
		"data.total":       2,
		"data.page":        1,
		"data.page_size":   20,
		"data.total_pages": 1,
	} {
		if got := jsonPathValue(t, body, path); got != want {
			t.Fatalf("%s = %#v, want %v", path, got, want)
		}
	}
	assertJSONObjectKeys(
		t,
		body,
		"data.devices.0",
		"id",
		"host",
		"device",
		"model",
		"capacity_bytes",
		"smart_health",
		"temperature_celsius",
		"lifetime_used_percent",
		"power_on_hours",
		"errors",
		"status",
		"status_source",
		"collection_level",
	)
	assertJSONObjectKeys(
		t,
		body,
		"data.devices.0.errors",
		"pending_sectors",
		"reallocated_sectors",
		"uncorrectable_sectors",
		"udma_crc_errors",
		"media_integrity_errors",
		"error_log_entries",
		"unsafe_shutdowns",
	)
	if got := jsonPathValue(t, body, "data.devices.0.capacity_bytes"); got != float64(1_234_567_890) {
		t.Fatalf("capacity_bytes = %#v", got)
	}
	if got := jsonPathValue(t, body, "data.devices.0.temperature_celsius"); got != 31.5 {
		t.Fatalf("temperature_celsius = %#v", got)
	}
	if !jsonPathIsNull(t, body, "data.devices.0.lifetime_used_percent") {
		t.Fatal("lifetime_used_percent is not null")
	}
	if got := jsonPathValue(t, body, "data.devices.0.errors.reallocated_sectors"); got != float64(2) {
		t.Fatalf("reallocated_sectors = %#v", got)
	}
	if !jsonPathIsNull(t, body, "data.devices.0.errors.pending_sectors") {
		t.Fatal("pending_sectors is not null")
	}
	if got := jsonPathValue(t, body, "data.devices.0.status_source"); got != "normal" {
		t.Fatalf("status_source = %#v, want normal", got)
	}
	if got := jsonPathValue(t, body, "data.devices.1.status_source"); got != "smart_health" {
		t.Fatalf("unknown status_source = %#v, want smart_health", got)
	}
	for _, path := range []string{
		"data.devices.1.capacity_bytes",
		"data.devices.1.temperature_celsius",
		"data.devices.1.lifetime_used_percent",
		"data.devices.1.power_on_hours",
		"data.devices.1.errors.unsafe_shutdowns",
	} {
		if !jsonPathIsNull(t, body, path) {
			t.Fatalf("%s is not null", path)
		}
	}
	assertDiskResponseMetaSchema(t, body)

	lowerBody := strings.ToLower(response.Body.String())
	for _, forbidden := range []string{"serial_no", "wwn", "labels", "promql", "upstream-test-secret"} {
		if strings.Contains(lowerBody, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestDiskDevicesAcceptsEverySupportedSortAndOrder(t *testing.T) {
	for _, sortField := range []string{"host", "device", "capacity", "temperature", "lifetime", "power_on_hours", "status"} {
		for _, order := range []string{"asc", "desc"} {
			t.Run(sortField+"/"+order, func(t *testing.T) {
				handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
				path := "/api/v1/disks/devices?sort=" + sortField + "&order=" + order + "&page=1&page_size=20"
				response := request(t, handler, http.MethodGet, path, "", sessionCookie)
				if response.Code != http.StatusOK {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
				}
			})
		}
	}
}

func TestDiskDevicesAcceptsWhitelistedFilters(t *testing.T) {
	handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	response := request(
		t,
		handler,
		http.MethodGet,
		"/api/v1/disks/devices?search=fixture-host-a&status=normal&sort=host&order=asc&page=1&page_size=20",
		"",
		sessionCookie,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := jsonPathValue(t, response.Body.Bytes(), "data.total"); got != float64(1) {
		t.Fatalf("total = %#v, want 1", got)
	}
}

func TestDiskDevicesRejectsUnknownRepeatedAndInvalidQueryParameters(t *testing.T) {
	for _, query := range []string{
		"?unknown=value",
		"?status=normal&status=warning",
		"?status=offline",
		"?sort=serial_no",
		"?order=random",
		"?page=0",
		"?page=-1",
		"?page=not-a-number",
		"?page=" + strconv.Itoa(int(^uint(0)>>1)),
		"?page_size=10",
		"?page_size=not-a-number",
	} {
		t.Run(query, func(t *testing.T) {
			provider := &diskSnapshotProvider{snapshot: fixtureDiskSnapshot()}
			handler, sessionCookie := newDiskAPIProviderTestHandler(t, provider, testNow)
			response := request(t, handler, http.MethodGet, "/api/v1/disks/devices"+query, "", sessionCookie)
			assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
		})
	}

	handler, sessionCookie := newDiskAPITestHandler(t, fixtureDiskSnapshot())
	response := requestWithRawQuery(t, handler, sessionCookie, "/api/v1/disks/devices", "unknown=%ZZ")
	assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
}

func TestDiskDevicesRejectsEmptyAllowedParametersBeforeProvider(t *testing.T) {
	for _, parameter := range []string{"search", "status", "sort", "order", "page", "page_size"} {
		for _, suffix := range []string{"", "="} {
			t.Run(parameter+"/"+suffix, func(t *testing.T) {
				provider := &diskSnapshotProvider{snapshot: fixtureDiskSnapshot()}
				handler, sessionCookie := newDiskAPIProviderTestHandler(t, provider, testNow)
				response := request(
					t,
					handler,
					http.MethodGet,
					"/api/v1/disks/devices?"+parameter+suffix,
					"",
					sessionCookie,
				)
				assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
				if provider.calls != 0 {
					t.Fatalf("provider calls = %d, want 0", provider.calls)
				}
			})
		}
	}
}

func newDiskAPITestHandler(t *testing.T, snapshot disk.Snapshot) (http.Handler, *http.Cookie) {
	t.Helper()
	return newDiskAPIProviderTestHandler(t, &diskSnapshotProvider{snapshot: snapshot}, testNow)
}

func newDiskAPIProviderTestHandler(t *testing.T, provider disk.Provider, clock func() time.Time) (http.Handler, *http.Cookie) {
	t.Helper()
	diskService := service.NewDisk(provider, cache.New(clock), service.DiskOptions{
		SnapshotTTL:        time.Minute,
		CollectionInterval: time.Minute,
		MaxStale:           5 * time.Minute,
		Clock:              clock,
	})
	handler := newDiskAPIHandler(t, diskService, clock)
	return handler, loginCookie(t, handler)
}

func newDiskAPIHandler(t *testing.T, diskService *service.DiskService, clock func() time.Time) http.Handler {
	t.Helper()
	cfg := config.Config{
		Username:   "admin",
		Password:   "correct-password",
		SessionTTL: 12 * time.Hour,
	}
	return New(Dependencies{
		Config:      cfg,
		Auth:        auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter:     auth.NewLimiter(5, time.Minute, clock),
		DiskService: diskService,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func fixtureDiskSnapshot() disk.Snapshot {
	capacity := int64(1_234_567_890)
	temperature := 31.5
	powerOnHours := float64(2400)
	reallocatedSectors := float64(2)
	return disk.Snapshot{Devices: []disk.Device{
		{
			ID:                 "public-device-id-a",
			HostID:             "fixture-host-a",
			Device:             "/dev/sda",
			Model:              "fixture model a",
			CapacityBytes:      &capacity,
			SMARTHealth:        disk.HealthHealthy,
			TemperatureCelsius: &temperature,
			PowerOnHours:       &powerOnHours,
			Errors: disk.ErrorCounters{
				ReallocatedSectors: &reallocatedSectors,
			},
			ReportedAt: time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC),
		},
		{
			ID:          "public-device-id-b",
			HostID:      "fixture-host-b",
			Device:      "/dev/nvme0n1",
			Model:       "fixture model b",
			SMARTHealth: disk.HealthUnknown,
			ReportedAt:  time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC),
		},
	}}
}

func assertRetryableDiskUnavailable(t *testing.T, response interface {
	Result() *http.Response
}) {
	t.Helper()
	result := response.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", result.StatusCode, body)
	}
	var decoded ErrorBody
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if decoded.Code != "disk_unavailable" ||
		decoded.Message != "数据源暂时不可用，请稍后重试" ||
		!decoded.Retryable ||
		decoded.RequestID == "" {
		t.Fatalf("error body = %#v", decoded)
	}
}

func assertDiskResponseMetaSchema(t *testing.T, body []byte) {
	t.Helper()
	assertJSONObjectKeys(t, body, "meta", "request_id", "stale", "collected_at")
	if !jsonPathIsString(t, body, "meta.request_id") ||
		!jsonPathIsString(t, body, "meta.collected_at") {
		t.Fatal("disk response meta string fields are invalid")
	}
	if _, ok := jsonPathValue(t, body, "meta.stale").(bool); !ok {
		t.Fatal("disk response meta stale is not a boolean")
	}
}

type diskSnapshotProvider struct {
	snapshot disk.Snapshot
	err      error
	calls    int
}

func (p *diskSnapshotProvider) SMARTSnapshot(context.Context) (disk.Snapshot, error) {
	p.calls++
	return p.snapshot, p.err
}

var _ disk.Provider = (*diskSnapshotProvider)(nil)
