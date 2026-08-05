package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/javaapp"
)

func TestJavaPromQLIsFixedAndReturnsACopy(t *testing.T) {
	want := []string{
		"service_health_latency_ms",
		"service_health_up",
		"service_port_up",
		"service_process_count",
		"service_process_cpu_percent",
		"service_process_memory_bytes",
		"service_process_memory_percent",
		"service_process_port_consistent",
		"service_process_start_time_seconds",
		"service_process_up",
		"tlast_over_time(service_process_up[24h])",
	}
	got := javaPromQL()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queries = %#v", got)
	}
	got[0] = "changed"
	if javaPromQL()[0] != want[0] {
		t.Fatal("fixed Java queries were mutated")
	}
}

func TestJavaSnapshotUsesOneFixedBatchAndMapsFixture(t *testing.T) {
	batchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			batchCalls++
			if batchCalls != 1 {
				t.Fatal("JavaSnapshot sent a second batch")
			}
			assertJavaBatchRequest(t, request)
			writeFixture(t, w, "java-instant-batch.json")
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock,
	})
	snapshot, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatalf("JavaSnapshot() error = %v", err)
	}
	if batchCalls != 1 || len(snapshot.Services) != 1 {
		t.Fatalf("batchCalls=%d services=%d", batchCalls, len(snapshot.Services))
	}
	service := snapshot.Services[0]
	if service.ID != javaapp.StableServiceID("fixture-java-a", "fixture-address-a") ||
		service.Name != "fixture-java-a" || service.Address != "fixture-address-a" ||
		!service.CollectionTracked || !service.ReportedAt.Equal(time.Unix(1785200000, 0).UTC()) {
		t.Fatalf("service identity = %#v", service)
	}
	assertJavaFloat(t, service.HealthLatencyMilliseconds, 12.5)
	assertJavaBool(t, service.HealthUp, true)
	assertJavaBool(t, service.PortUp, true)
	assertJavaBool(t, service.ProcessUp, true)
	assertJavaBool(t, service.PortConsistent, true)
	assertJavaInt(t, service.ProcessCount, 2)
	assertJavaInt(t, service.ProcessMemoryBytes, 9007199254740993)
	assertJavaFloat(t, service.ProcessCPUPercent, 7.5)
	assertJavaFloat(t, service.ProcessMemoryPercent, 42.5)
	assertJavaInt(t, service.ProcessStartTimeSeconds, 1785100000)

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-ident-private", "service_process_up", "tlast_over_time"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("snapshot exposed %q", forbidden)
		}
	}
}

func TestBuildJavaSnapshotUsesInventoryOnlyAndDeduplicatesCollectors(t *testing.T) {
	groups := javaGroups()
	first := javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-a")
	second := javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-b")
	groups[javaInventoryQuery] = []instantSeries{
		javaSeries(first, 1785200200, "1785200000"),
		javaSeries(second, 1785200200, "1785200000"),
		javaSeries(javaLabels(" ", "fixture-address-missing-name", "fixture-ident-c"), 1785200200, "1785200000"),
		javaSeries(javaLabels("fixture-java-missing-address", " ", "fixture-ident-d"), 1785200200, "1785200000"),
	}
	groups[javaProcessUpQuery] = []instantSeries{
		javaSeries(first, 1785200300, "1"),
		javaSeries(javaLabels("fixture-java-ghost", "fixture-address-ghost", "fixture-ident-ghost"), 1785200300, "1"),
	}

	snapshot, err := buildJavaSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Services) != 1 {
		t.Fatalf("services = %#v", snapshot.Services)
	}
	service := snapshot.Services[0]
	if service.ID != javaapp.StableServiceID("fixture-java-a", "fixture-address-a") || service.ProcessUp == nil || !*service.ProcessUp {
		t.Fatalf("inventory service = %#v", service)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "fixture-ident") {
		t.Fatalf("snapshot exposed collector identity: %s", encoded)
	}
}

func TestBuildJavaSnapshotSelectsLatestValuesAndClearsLatestConflicts(t *testing.T) {
	groups := javaGroupsWithInventory()
	labelsA := javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-a")
	labelsB := javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-b")
	groups[javaHealthLatencyQuery] = []instantSeries{
		javaSeries(labelsA, 1785200100, "5"),
		javaSeries(labelsB, 1785200300, "7"),
		javaSeries(labelsA, 1785200200, "6"),
	}
	groups[javaHealthUpQuery] = []instantSeries{
		javaSeries(labelsA, 1785200300, "0"),
		javaSeries(labelsB, 1785200300, "1"),
	}
	groups[javaProcessCountQuery] = []instantSeries{javaSeries(labelsA, 1785200300, "9007199254740993")}
	groups[javaProcessMemoryBytesQuery] = []instantSeries{javaSeries(labelsA, 1785200300, "9223372036854775807")}
	groups[javaProcessStartTimeQuery] = []instantSeries{javaSeries(labelsA, 1785200300, "0")}

	snapshot, err := buildJavaSnapshot(groups)
	if err != nil {
		t.Fatal(err)
	}
	service := snapshot.Services[0]
	assertJavaFloat(t, service.HealthLatencyMilliseconds, 7)
	if service.HealthUp != nil {
		t.Fatalf("latest conflict survived = %v", *service.HealthUp)
	}
	assertJavaInt(t, service.ProcessCount, 9007199254740993)
	assertJavaInt(t, service.ProcessMemoryBytes, 9223372036854775807)
	assertJavaInt(t, service.ProcessStartTimeSeconds, 0)
	if !service.ReportedAt.Equal(time.Unix(1785200000, 0).UTC()) {
		t.Fatalf("ReportedAt = %s, want inventory sample value", service.ReportedAt)
	}
}

func TestBuildJavaSnapshotRejectsInvalidOptionalValues(t *testing.T) {
	tests := []struct {
		name       string
		queryIndex int
		values     []string
		missing    func(javaapp.Service) bool
	}{
		{name: "health binary", queryIndex: javaHealthUpQuery, values: []string{"2"}, missing: func(service javaapp.Service) bool { return service.HealthUp == nil }},
		{name: "port binary", queryIndex: javaPortUpQuery, values: []string{"-1"}, missing: func(service javaapp.Service) bool { return service.PortUp == nil }},
		{name: "consistent binary", queryIndex: javaPortConsistentQuery, values: []string{"0.5"}, missing: func(service javaapp.Service) bool { return service.PortConsistent == nil }},
		{name: "process binary", queryIndex: javaProcessUpQuery, values: []string{"NaN"}, missing: func(service javaapp.Service) bool { return service.ProcessUp == nil }},
		{name: "negative latency", queryIndex: javaHealthLatencyQuery, values: []string{"-1"}, missing: func(service javaapp.Service) bool { return service.HealthLatencyMilliseconds == nil }},
		{name: "infinite latency", queryIndex: javaHealthLatencyQuery, values: []string{"+Inf"}, missing: func(service javaapp.Service) bool { return service.HealthLatencyMilliseconds == nil }},
		{name: "cpu above percent", queryIndex: javaProcessCPUQuery, values: []string{"101"}, missing: func(service javaapp.Service) bool { return service.ProcessCPUPercent == nil }},
		{name: "memory below percent", queryIndex: javaProcessMemoryPercentQuery, values: []string{"-0.1"}, missing: func(service javaapp.Service) bool { return service.ProcessMemoryPercent == nil }},
		{name: "count decimal", queryIndex: javaProcessCountQuery, values: []string{"1.5"}, missing: func(service javaapp.Service) bool { return service.ProcessCount == nil }},
		{name: "memory exponent", queryIndex: javaProcessMemoryBytesQuery, values: []string{"9e3"}, missing: func(service javaapp.Service) bool { return service.ProcessMemoryBytes == nil }},
		{name: "start negative", queryIndex: javaProcessStartTimeQuery, values: []string{"-1"}, missing: func(service javaapp.Service) bool { return service.ProcessStartTimeSeconds == nil }},
		{name: "integer overflow", queryIndex: javaProcessMemoryBytesQuery, values: []string{"9223372036854775808"}, missing: func(service javaapp.Service) bool { return service.ProcessMemoryBytes == nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := javaGroupsWithInventory()
			for _, value := range test.values {
				groups[test.queryIndex] = append(groups[test.queryIndex], javaSeries(javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-a"), 1785200300, value))
			}
			snapshot, err := buildJavaSnapshot(groups)
			if err != nil {
				t.Fatal(err)
			}
			if !test.missing(snapshot.Services[0]) {
				t.Fatalf("invalid value survived in service = %#v", snapshot.Services[0])
			}
		})
	}
}

func TestBuildJavaSnapshotRejectsUnsafeBatchShapes(t *testing.T) {
	tests := []struct {
		name   string
		groups [][]instantSeries
	}{
		{name: "wrong group count", groups: javaGroups()[:javaQueryCount-1]},
		{name: "nil inventory", groups: func() [][]instantSeries {
			groups := javaGroups()
			groups[javaInventoryQuery] = nil
			return groups
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildJavaSnapshot(test.groups)
			assertJavaSafeError(t, err, "fixture-java-a", "fixture-ident-a", "service_health_latency_ms")
		})
	}
}

func TestJavaSnapshotMapsProtocolFailuresToSafeError(t *testing.T) {
	const upstreamSecret = "fixture-java-upstream-secret"
	const upstreamLabel = "fixture-java-private-label"
	tests := []struct {
		name     string
		status   int
		ctype    string
		body     string
		redirect bool
		maxBytes int64
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, ctype: "application/json", body: `{"dat":null,"err":"` + upstreamSecret + ` ` + upstreamLabel + `"}`},
		{name: "forbidden", status: http.StatusForbidden, ctype: "application/json", body: `{"dat":null,"err":"` + upstreamSecret + ` ` + upstreamLabel + `"}`},
		{name: "redirect", status: http.StatusFound, ctype: "text/html", body: upstreamSecret, redirect: true},
		{name: "non json", status: http.StatusOK, ctype: "text/html", body: `<html>` + upstreamSecret + ` ` + upstreamLabel + `</html>`},
		{name: "null data", status: http.StatusOK, ctype: "application/json", body: `{"dat":null,"err":""}`},
		{name: "error envelope", status: http.StatusOK, ctype: "application/json", body: `{"dat":[],"err":"` + upstreamSecret + ` ` + upstreamLabel + `"}`},
		{name: "oversize", status: http.StatusOK, ctype: "application/json", body: `{"dat":"` + strings.Repeat("x", 256) + `","err":""}`, maxBytes: 64},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/n9e/datasource/brief" {
					writeFixture(t, w, "datasource-brief.json")
					return
				}
				if test.redirect {
					w.Header().Set("Location", "/fixture-private-location")
				}
				w.Header().Set("Content-Type", test.ctype)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			provider := New(Options{
				BaseURL: server.URL, AllowInsecureHTTP: true, Token: upstreamSecret,
				HTTPClient: server.Client(), Clock: fixedClock, MaxResponseBytes: test.maxBytes,
			})
			_, err := provider.JavaSnapshot(context.Background())
			assertJavaSafeError(t, err, upstreamSecret, upstreamLabel, server.URL, "fixture-private-location", "service_health_latency_ms", test.body)
		})
	}
}

func TestJavaSnapshotMapsTimeoutToSafeError(t *testing.T) {
	const upstreamSecret = "fixture-java-timeout-secret"
	const baseURL = "https://fixture-java-private.invalid"
	client := &http.Client{Transport: javaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, errors.New(upstreamSecret)
	})}
	provider := New(Options{
		BaseURL: baseURL, Token: upstreamSecret,
		HTTPClient: client, Clock: fixedClock,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := provider.JavaSnapshot(ctx)
	assertJavaSafeError(t, err, upstreamSecret, baseURL, "service_process_up")
}

type javaRoundTripFunc func(*http.Request) (*http.Response, error)

func (function javaRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func assertJavaBatchRequest(t *testing.T, request *http.Request) {
	t.Helper()
	var body batchRequest
	decodeRequest(t, request, &body)
	want := javaPromQL()
	if request.Method != http.MethodPost || body.DatasourceID != 7 || len(body.Queries) != len(want) {
		t.Fatalf("Java batch shape = %#v", body)
	}
	for index, query := range body.Queries {
		if query.Query != want[index] || query.Time != fixedClock().Unix() {
			t.Fatalf("Java query %d = %#v", index, query)
		}
	}
}

func javaGroups() [][]instantSeries {
	groups := make([][]instantSeries, javaQueryCount)
	for index := range groups {
		groups[index] = []instantSeries{}
	}
	return groups
}

func javaGroupsWithInventory() [][]instantSeries {
	groups := javaGroups()
	groups[javaInventoryQuery] = []instantSeries{
		javaSeries(javaLabels("fixture-java-a", "fixture-address-a", "fixture-ident-a"), 1785200200, "1785200000"),
	}
	return groups
}

func javaLabels(name, address, ident string) map[string]string {
	return map[string]string{"name": name, "server_ip": address, "ident": ident}
}

func javaSeries(labels map[string]string, timestamp int64, value string) instantSeries {
	return instantSeries{Metric: cloneLabels(labels), Value: rawInstantValue(timestamp, value)}
}

func assertJavaFloat(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("float = %#v, want %v", got, want)
	}
}

func assertJavaBool(t *testing.T, got *bool, want bool) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("bool = %#v, want %v", got, want)
	}
}

func assertJavaInt(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("int64 = %#v, want %d", got, want)
	}
}

func assertJavaSafeError(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	if !errors.Is(err, javaapp.ErrUnavailable) {
		t.Fatalf("error = %v, want Java ErrUnavailable", err)
	}
	for _, value := range forbidden {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatalf("Java error exposed upstream data %q: %v", value, err)
		}
	}
}
