package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/disk"
)

var wantDiskPromQL = []string{
	`smart_device_health_ok`,
	`smart_device_temp_c`,
	`smart_attribute_temperature_celsius`,
	`smart_attribute_percentage_used`,
	`smart_attribute_power_on_hours`,
	`smart_attribute_critical_warning`,
	`smart_attribute_available_spare`,
	`smart_attribute_available_spare_threshold`,
	`smart_attribute_value{fail=~"FAILING_NOW|In_the_past"}`,
	`smart_device_pending_sector_count`,
	`smart_device_reallocated_sectors_count`,
	`smart_device_uncorrectable_sector_count`,
	`smart_device_udma_crc_errors`,
	`smart_attribute_media_and_data_integrity_errors`,
	`smart_attribute_error_information_log_entries`,
	`smart_attribute_unsafe_shutdowns`,
	`smart_disk_capacity_bytes`,
	`tlast_over_time(smart_device_health_ok[24h])`,
}

func TestDiskPromQLIsFixedAndReturnsACopy(t *testing.T) {
	got := diskPromQL()
	if !reflect.DeepEqual(got, wantDiskPromQL) {
		t.Fatal("disk PromQL contract changed")
	}
	got[0] = "mutated"
	if !reflect.DeepEqual(diskPromQL(), wantDiskPromQL) {
		t.Fatal("caller mutation changed the fixed disk PromQL")
	}
}

func TestSMARTSnapshotUsesOneFixedBatch(t *testing.T) {
	groups := validDiskBatch()
	discoveryCalls := 0
	batchCalls := 0
	provider, closeServer := newDiskProvider(t, func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			discoveryCalls++
			if discoveryCalls != 1 {
				t.Fatal("disk snapshot repeated datasource discovery")
			}
			writeEnvelope(t, w, []datasourceRecord{{ID: 7, PluginType: "prometheus", IsDefault: true}})
		case "/api/n9e/query-instant-batch":
			batchCalls++
			if batchCalls != 1 {
				t.Fatal("disk snapshot sent more than one instant batch")
			}
			assertDiskBatchRequest(t, request)
			writeEnvelope(t, w, groups)
		default:
			t.Fatal("disk snapshot sent an unexpected or per-device request")
		}
	})
	defer closeServer()

	if _, err := provider.SMARTSnapshot(context.Background()); err != nil {
		t.Fatal("SMARTSnapshot() returned an unexpected error")
	}
	if discoveryCalls != 1 || batchCalls != 1 {
		t.Fatal("disk snapshot request counts do not match the fixed contract")
	}
}

func TestSMARTSnapshotMapsAnonymizedFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/disk-instant-batch.json")
	if err != nil {
		t.Fatal("cannot read anonymized disk fixture")
	}
	provider, closeServer := newDiskProvider(t, func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeEnvelope(t, w, []datasourceRecord{{ID: 7, PluginType: "prometheus", IsDefault: true}})
		case "/api/n9e/query-instant-batch":
			assertDiskBatchRequest(t, request)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		default:
			t.Fatal("disk fixture server received an unexpected path")
		}
	})
	defer closeServer()

	snapshot, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatal("SMARTSnapshot() rejected the anonymized fixture")
	}
	if len(snapshot.Devices) != 4 {
		t.Fatal("SMARTSnapshot() did not preserve the complete recent device set")
	}
	ata := diskDeviceByName(t, snapshot, "/dev/fixture-a")
	nvme := diskDeviceByName(t, snapshot, "/dev/fixture-b")
	historyOnly := diskDeviceByName(t, snapshot, "/dev/fixture-c")
	deviceOnly := diskDeviceByName(t, snapshot, "/dev/fixture-d")

	if ata.ID != disk.StableDeviceID("fixture-host-a", "fixture-wwn-a", "fixture-serial-a", "/dev/fixture-a") {
		t.Fatal("ATA device did not use WWN-priority stable identity")
	}
	if nvme.ID != disk.StableDeviceID("fixture-host-a", "", "fixture-serial-b", "/dev/fixture-b") {
		t.Fatal("NVMe device did not use serial-priority stable identity")
	}
	if historyOnly.ID != disk.StableDeviceID("fixture-host-b", "", "fixture-serial-c", "/dev/fixture-c") {
		t.Fatal("recent device did not retain its stable identity")
	}
	if deviceOnly.ID != disk.StableDeviceID("fixture-host-c", "", "", "/dev/fixture-d") || deviceOnly.ID == "" {
		t.Fatal("device-only primary identity did not use the device fallback")
	}
	for _, raw := range []string{"fixture-host-c", "/dev/fixture-d"} {
		if strings.Contains(deviceOnly.ID, raw) {
			t.Fatal("device-only stable ID exposes raw identity")
		}
	}
	if ata.SMARTHealth != disk.HealthHealthy || nvme.SMARTHealth != disk.HealthUnknown || historyOnly.SMARTHealth != disk.HealthUnknown {
		t.Fatal("current SMART health mapping is incorrect")
	}
	if !ata.CollectionTracked || !nvme.CollectionTracked || !historyOnly.CollectionTracked {
		t.Fatal("recent device inventory is not collection-tracked")
	}
	if !ata.ReportedAt.Equal(time.Unix(1785123000, 250000000).UTC()) ||
		!nvme.ReportedAt.Equal(time.Unix(1785123010, 500000000).UTC()) ||
		!historyOnly.ReportedAt.Equal(time.Unix(1785123020, 0).UTC()) {
		t.Fatal("ReportedAt did not come from the raw tlast sample value")
	}
	if ata.CapacityBytes == nil || *ata.CapacityBytes != 1000000000 || nvme.CapacityBytes != nil || historyOnly.CapacityBytes != nil {
		t.Fatal("capacity metric validation is incorrect")
	}
	if ata.TemperatureCelsius != nil {
		t.Fatal("same-time temperature conflict must remain missing")
	}
	assertDiskFloat(t, nvme.TemperatureCelsius, 30)
	if ata.LifetimeUsedPercent != nil || nvme.LifetimeUsedPercent != nil {
		t.Fatal("invalid or conflicting percentage_used must remain missing")
	}
	assertDiskFloat(t, ata.PowerOnHours, 1200)
	if ata.CriticalWarning != nil || nvme.CriticalWarning == nil || *nvme.CriticalWarning != 1 {
		t.Fatal("critical_warning validation is incorrect")
	}
	assertDiskFloat(t, nvme.AvailableSparePercent, 90)
	assertDiskFloat(t, nvme.AvailableSpareThresholdPercent, 10)
	if ata.AttributeFailure != disk.AttributeFailureNow || nvme.AttributeFailure != disk.AttributeFailurePast {
		t.Fatal("attribute failure precedence is incorrect")
	}
	assertDiskFloat(t, ata.Errors.PendingSectors, 3)
	if ata.Errors.ReallocatedSectors != nil {
		t.Fatal("same-time error counter conflict must remain missing")
	}
	assertDiskFloat(t, ata.Errors.UncorrectableSectors, 0)
	if ata.Errors.UDMACRCErrors != nil {
		t.Fatal("invalid error counter must remain missing")
	}
	assertDiskFloat(t, nvme.Errors.MediaIntegrityErrors, 2)
	assertDiskFloat(t, nvme.Errors.ErrorLogEntries, 4)
	assertDiskFloat(t, nvme.Errors.UnsafeShutdowns, 6)

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal("cannot marshal disk snapshot")
	}
	for _, forbidden := range []string{"fixture-wwn", "fixture-serial", `"Metric"`, `"metric"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatal("disk snapshot exposes internal identity or raw labels")
		}
	}
}

func TestSMARTSnapshotUsesCapacityMetricAsOnlySource(t *testing.T) {
	groups := validDiskBatch()
	groups[17][0].Metric["capacity"] = "999999999"

	snapshot, err := runDiskBatch(t, groups)
	if err != nil {
		t.Fatal(err)
	}
	device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
	if device.CapacityBytes == nil || *device.CapacityBytes != 1000000000 {
		t.Fatalf("CapacityBytes = %#v, want capacity metric value", device.CapacityBytes)
	}
}

func TestSMARTSnapshotKeepsInvalidOrConflictingCapacityMissing(t *testing.T) {
	tests := []struct {
		name   string
		series []instantSeries
	}{
		{name: "missing", series: nil},
		{name: "negative", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "-1")}},
		{name: "fractional", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "1.5")}},
		{name: "fractional above exact float range", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "9007199254740992.5")}},
		{name: "nan", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "NaN")}},
		{name: "infinity", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "Inf")}},
		{name: "overflow", series: []instantSeries{diskSeries(diskLabels(), 1785123900, "9223372036854775808")}},
		{name: "same-time conflict", series: []instantSeries{
			diskSeries(diskLabels(), 1785123900, "1000000000"),
			diskSeries(diskLabels(), 1785123900, "2000000000"),
		}},
		{name: "same-time conflict above exact float range", series: []instantSeries{
			diskSeries(diskLabels(), 1785123900, "9007199254740992"),
			diskSeries(diskLabels(), 1785123900, "9007199254740993"),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := validDiskBatch()
			groups[16] = tt.series

			snapshot, err := runDiskBatch(t, groups)
			if err != nil {
				t.Fatal(err)
			}
			if got := diskDeviceByName(t, snapshot, "/dev/fixture-a").CapacityBytes; got != nil {
				t.Fatalf("CapacityBytes = %d, want nil", *got)
			}
		})
	}
}

func TestSMARTSnapshotPreservesExactLargeCapacity(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "above exact float range", value: "9007199254740993", want: 9007199254740993},
		{name: "maximum int64", value: "9223372036854775807", want: 9223372036854775807},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := validDiskBatch()
			groups[16] = []instantSeries{diskSeries(diskLabels(), 1785123900, tt.value)}

			snapshot, err := runDiskBatch(t, groups)
			if err != nil {
				t.Fatal(err)
			}
			got := diskDeviceByName(t, snapshot, "/dev/fixture-a").CapacityBytes
			if got == nil || *got != tt.want {
				t.Fatalf("CapacityBytes = %#v, want %d", got, tt.want)
			}
		})
	}
}

func TestSMARTSnapshotSelectsLatestCapacityWithoutChangingReportedAt(t *testing.T) {
	groups := validDiskBatch()
	groups[16] = []instantSeries{
		diskSeries(diskLabels(), 1785123900, "1000000000"),
		diskSeries(diskLabels(), 1785123800, "500000000"),
		diskSeries(diskLabels(), 1785123950, "2000000000"),
	}

	snapshot, err := runDiskBatch(t, groups)
	if err != nil {
		t.Fatal(err)
	}
	device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
	if device.CapacityBytes == nil || *device.CapacityBytes != 2000000000 {
		t.Fatalf("CapacityBytes = %#v, want latest value", device.CapacityBytes)
	}
	if !device.ReportedAt.Equal(time.Unix(1785123000, 250000000).UTC()) {
		t.Fatal("capacity timestamp changed ReportedAt")
	}
}

func TestSMARTSnapshotCapacityHonorsIdentityAndKnownInventory(t *testing.T) {
	groups := validDiskBatch()
	conflict := diskLabels()
	conflict["serial_no"] = "fixture-serial-conflict"
	unknown := diskLabels()
	unknown["device"] = "/dev/fixture-unknown"
	groups[16] = []instantSeries{
		diskSeries(conflict, 1785123900, "2000000000"),
		diskSeries(unknown, 1785123900, "3000000000"),
	}

	snapshot, err := runDiskBatch(t, groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Devices) != 1 {
		t.Fatalf("device count = %d, want inventory-only device set", len(snapshot.Devices))
	}
	if got := diskDeviceByName(t, snapshot, "/dev/fixture-a").CapacityBytes; got != nil {
		t.Fatalf("conflicting capacity was merged: %d", *got)
	}
}

func TestSMARTSnapshotCoalescesCompatibleDuplicateInventoryAtLatestReportedTime(t *testing.T) {
	groups := validDiskBatch()
	duplicate := cloneDiskSeries(groups[17][0])
	duplicate.Value[1] = json.RawMessage(`"1785123055.5"`)
	groups[17] = append(groups[17], duplicate)

	snapshot, err := runDiskBatch(t, groups)
	if err != nil {
		t.Fatal("compatible duplicate inventory series rejected")
	}
	if len(snapshot.Devices) != 1 {
		t.Fatal("compatible duplicate inventory series created duplicate devices")
	}
	device := snapshot.Devices[0]
	if !device.ReportedAt.Equal(time.Unix(1785123055, 500000000).UTC()) {
		t.Fatal("duplicate inventory did not retain the latest raw sample time")
	}
	if device.ID != disk.StableDeviceID("fixture-host-a", "fixture-wwn-a", "fixture-serial-a", "/dev/fixture-a") {
		t.Fatal("duplicate inventory changed the stable identity")
	}
}

func TestSMARTSnapshotCoalescesComplementaryDuplicateInventoryIdentity(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "older first", true: "newer first"}[reverse], func(t *testing.T) {
			groups := validDiskBatch()
			older := cloneDiskSeries(groups[17][0])
			older.Metric["serial_no"] = ""
			older.Metric["wwn"] = ""
			newer := cloneDiskSeries(older)
			newer.Metric["serial_no"] = "fixture-serial-a"
			newer.Metric["wwn"] = "fixture-wwn-a"
			newer.Metric["model"] = ""
			newer.Metric["capacity"] = ""
			newer.Value[1] = json.RawMessage(`"1785123055.5"`)
			groups[17] = []instantSeries{older, newer}
			if reverse {
				groups[17] = []instantSeries{newer, older}
			}

			snapshot, err := runDiskBatch(t, groups)
			if err != nil {
				t.Fatal("complementary duplicate inventory series rejected")
			}
			device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
			if device.ID != disk.StableDeviceID("fixture-host-a", "fixture-wwn-a", "fixture-serial-a", "/dev/fixture-a") {
				t.Fatal("complementary duplicate inventory did not use the strongest stable identity")
			}
			if device.Model != "Fixture Disk A" || device.CapacityBytes == nil || *device.CapacityBytes != 1000000000 {
				t.Fatal("inventory order changed the retained non-empty metadata")
			}
		})
	}
}

func TestSMARTSnapshotRejectsMalformedPrimaryStructure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([][]instantSeries) [][]instantSeries
	}{
		{name: "wrong outer count", mutate: func(groups [][]instantSeries) [][]instantSeries { return groups[:17] }},
		{name: "null current group", mutate: func(groups [][]instantSeries) [][]instantSeries { groups[0] = nil; return groups }},
		{name: "null inventory group", mutate: func(groups [][]instantSeries) [][]instantSeries { groups[17] = nil; return groups }},
		{name: "missing required inventory identity", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[17][0].Metric["device"] = ""
			return groups
		}},
		{name: "duplicate inventory serial conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := cloneDiskSeries(groups[17][0])
			conflict.Metric["serial_no"] = "fixture-serial-conflict"
			groups[17] = append(groups[17], conflict)
			return groups
		}},
		{name: "stable identity conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := cloneDiskSeries(groups[17][0])
			conflict.Metric["device"] = "/dev/fixture-conflict"
			conflict.Metric["serial_no"] = "fixture-serial-conflict"
			groups[17] = append(groups[17], conflict)
			return groups
		}},
		{name: "duplicate inventory WWN conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := cloneDiskSeries(groups[17][0])
			conflict.Metric["wwn"] = "fixture-wwn-conflict"
			groups[17] = append(groups[17], conflict)
			return groups
		}},
		{name: "cross-device serial conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := cloneDiskSeries(groups[17][0])
			conflict.Metric["device"] = "/dev/fixture-conflict"
			conflict.Metric["wwn"] = "fixture-wwn-conflict"
			groups[17] = append(groups[17], conflict)
			return groups
		}},
		{name: "same-time model conflict", mutate: func(groups [][]instantSeries) [][]instantSeries {
			conflict := cloneDiskSeries(groups[17][0])
			conflict.Metric["model"] = "Conflicting Fixture Disk"
			groups[17] = append(groups[17], conflict)
			return groups
		}},
		{name: "invalid raw tlast", mutate: func(groups [][]instantSeries) [][]instantSeries {
			groups[17][0].Value[1] = json.RawMessage(`"not-a-time"`)
			return groups
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runDiskBatch(t, tt.mutate(validDiskBatch()))
			assertDiskUnavailable(t, err)
		})
	}
}

func TestSMARTSnapshotMatchesOptionalSerialAndWWNSymmetrically(t *testing.T) {
	tests := []struct {
		name            string
		primarySerial   string
		primaryWWN      string
		candidateSerial string
		candidateWWN    string
		wantMatched     bool
	}{
		{name: "primary missing serial", primaryWWN: "fixture-wwn-a", candidateSerial: "fixture-serial-a", candidateWWN: "fixture-wwn-a", wantMatched: true},
		{name: "candidate missing serial", primarySerial: "fixture-serial-a", primaryWWN: "fixture-wwn-a", candidateWWN: "fixture-wwn-a", wantMatched: true},
		{name: "primary missing WWN", primarySerial: "fixture-serial-a", candidateSerial: "fixture-serial-a", candidateWWN: "fixture-wwn-a", wantMatched: true},
		{name: "candidate missing WWN", primarySerial: "fixture-serial-a", primaryWWN: "fixture-wwn-a", candidateSerial: "fixture-serial-a", wantMatched: true},
		{name: "serial conflict", primarySerial: "fixture-serial-a", candidateSerial: "fixture-serial-conflict", wantMatched: false},
		{name: "WWN conflict", primarySerial: "fixture-serial-a", primaryWWN: "fixture-wwn-a", candidateSerial: "fixture-serial-a", candidateWWN: "fixture-wwn-conflict", wantMatched: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := validDiskBatch()
			groups[17][0].Metric["serial_no"] = tt.primarySerial
			groups[17][0].Metric["wwn"] = tt.primaryWWN
			candidateLabels := diskLabels()
			candidateLabels["serial_no"] = tt.candidateSerial
			candidateLabels["wwn"] = tt.candidateWWN
			groups[0] = []instantSeries{diskSeries(candidateLabels, 1785123100, "1")}
			groups[1] = []instantSeries{diskSeries(candidateLabels, 1785123200, "27")}

			snapshot, err := runDiskBatch(t, groups)
			if err != nil {
				t.Fatal("optional identity matching rejected a valid primary snapshot")
			}
			device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
			if tt.wantMatched {
				if device.SMARTHealth != disk.HealthHealthy {
					t.Fatal("compatible optional identity did not merge current health")
				}
				assertDiskFloat(t, device.TemperatureCelsius, 27)
				return
			}
			if device.SMARTHealth != disk.HealthUnknown || device.TemperatureCelsius != nil {
				t.Fatal("conflicting non-empty optional identity was merged")
			}
		})
	}
}

func TestSMARTSnapshotAllowsNullAuxiliaryGroupsAndDropsInvalidFields(t *testing.T) {
	groups := validDiskBatch()
	groups[1] = nil
	groups[3] = []instantSeries{diskSeries(diskLabels(), 1785123200, "-1")}
	groups[5] = []instantSeries{diskSeries(diskLabels(), 1785123200, "1.5")}
	groups[6] = []instantSeries{diskSeries(diskLabels(), 1785123200, "Inf")}
	groups[9] = []instantSeries{diskSeries(diskLabels(), 1785123200, "-1")}

	snapshot, err := runDiskBatch(t, groups)
	if err != nil {
		t.Fatal("auxiliary null or invalid field rejected the complete snapshot")
	}
	device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
	if device.TemperatureCelsius != nil || device.LifetimeUsedPercent != nil ||
		device.CriticalWarning != nil || device.AvailableSparePercent != nil ||
		device.Errors.PendingSectors != nil {
		t.Fatal("invalid auxiliary value did not remain field-local")
	}
}

func TestSMARTSnapshotMapsUnsafeUpstreamFailuresToSafeDiskError(t *testing.T) {
	const upstreamSecret = "fixture-upstream-secret"
	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "unauthorized", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, upstreamSecret)
		}},
		{name: "forbidden", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"secret":"`+upstreamSecret+`"}`)
		}},
		{name: "redirect", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "/"+upstreamSecret)
			w.WriteHeader(http.StatusFound)
		}},
		{name: "non json", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>"+upstreamSecret+"</html>")
		}},
		{name: "wrong content type", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<secret>`+upstreamSecret+`</secret>`)
		}},
		{name: "malformed json", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":"`+upstreamSecret)
		}},
		{name: "envelope error", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":[],"err":"`+upstreamSecret+`"}`)
		}},
		{name: "oversize", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":"`+strings.Repeat("x", int(defaultMaxResponseBytes)+1)+`","err":""}`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, closeServer := newDiskProvider(t, func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/n9e/datasource/brief" {
					writeEnvelope(t, w, []datasourceRecord{{ID: 7, PluginType: "prometheus", IsDefault: true}})
					return
				}
				tt.handler(w, request)
			})
			defer closeServer()
			_, err := provider.SMARTSnapshot(context.Background())
			assertDiskSafeError(t, err, upstreamSecret, provider.baseURL.String(), fixtureToken)
		})
	}
}

func TestSMARTSnapshotMapsTimeoutToSafeDiskError(t *testing.T) {
	const upstreamSecret = "fixture-timeout-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/n9e/datasource/brief" {
			t.Fatal("timeout test sent an unexpected request to the fixture server")
		}
		writeEnvelope(t, w, []datasourceRecord{{ID: 7, PluginType: "prometheus", IsDefault: true}})
	}))
	defer server.Close()
	baseTransport := server.Client().Transport
	client := &http.Client{Transport: diskRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/n9e/query-instant-batch" {
			return baseTransport.RoundTrip(request)
		}
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token: fixtureToken, HTTPClient: client, Clock: fixedClock,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := provider.SMARTSnapshot(ctx)
	assertDiskSafeError(t, err, upstreamSecret, provider.baseURL.String(), fixtureToken)
}

type diskRoundTripFunc func(*http.Request) (*http.Response, error)

func (function diskRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func newDiskProvider(t *testing.T, handler http.HandlerFunc) (*Provider, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-User-Token") != fixtureToken ||
			request.Header.Get("Accept") != "application/json" ||
			request.Method == http.MethodPost && request.Header.Get("Content-Type") != "application/json" {
			t.Fatal("disk request authentication or JSON headers are incorrect")
		}
		handler(w, request)
	}))
	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock,
	})
	return provider, server.Close
}

func assertDiskBatchRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Method != http.MethodPost {
		t.Fatal("disk instant batch is not POST")
	}
	var body batchRequest
	decodeRequest(t, request, &body)
	if body.DatasourceID != 7 || len(body.Queries) != len(wantDiskPromQL) {
		t.Fatal("disk instant batch shape is incorrect")
	}
	for index, query := range body.Queries {
		if query.Query != wantDiskPromQL[index] || query.Time != fixedClock().Unix() {
			t.Fatal("disk instant batch query order, text, or evaluation time is incorrect")
		}
	}
}

func runDiskBatch(t *testing.T, groups [][]instantSeries) (disk.Snapshot, error) {
	t.Helper()
	provider, closeServer := newDiskProvider(t, func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeEnvelope(t, w, []datasourceRecord{{ID: 7, PluginType: "prometheus", IsDefault: true}})
		case "/api/n9e/query-instant-batch":
			assertDiskBatchRequest(t, request)
			writeEnvelope(t, w, groups)
		default:
			t.Fatal("disk batch server received an unexpected path")
		}
	})
	defer closeServer()
	return provider.SMARTSnapshot(context.Background())
}

func validDiskBatch() [][]instantSeries {
	groups := make([][]instantSeries, 18)
	labels := diskLabels()
	groups[0] = []instantSeries{diskSeries(labels, 1785123100, "1")}
	groups[16] = []instantSeries{diskSeries(labels, 1785123900, "1000000000")}
	groups[17] = []instantSeries{diskSeries(labels, 1785124000, "1785123000.25")}
	return groups
}

func diskLabels() map[string]string {
	return map[string]string{
		"__name__":  "smart_device_health_ok",
		"ident":     "fixture-host-a",
		"device":    "/dev/fixture-a",
		"model":     "Fixture Disk A",
		"serial_no": "fixture-serial-a",
		"wwn":       "fixture-wwn-a",
		"capacity":  "999999999",
	}
}

func diskSeries(labels map[string]string, timestamp int64, value string) instantSeries {
	return instantSeries{
		Metric: cloneLabels(labels),
		Value: []json.RawMessage{
			json.RawMessage(strconv.FormatInt(timestamp, 10)),
			json.RawMessage(strconv.Quote(value)),
		},
	}
}

func cloneDiskSeries(series instantSeries) instantSeries {
	return instantSeries{Metric: cloneLabels(series.Metric), Value: append([]json.RawMessage(nil), series.Value...)}
}

func diskDeviceByName(t *testing.T, snapshot disk.Snapshot, deviceName string) disk.Device {
	t.Helper()
	for _, device := range snapshot.Devices {
		if device.Device == deviceName {
			return device
		}
	}
	t.Fatal("disk snapshot is missing an expected fixture device")
	return disk.Device{}
}

func assertDiskFloat(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatal("disk scalar value is incorrect")
	}
}

func assertDiskUnavailable(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, disk.ErrUnavailable) {
		t.Fatal("disk error does not satisfy disk.ErrUnavailable")
	}
}

func assertDiskSafeError(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	assertDiskUnavailable(t, err)
	for _, value := range forbidden {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatal("disk error exposes upstream request or response data")
		}
	}
}
