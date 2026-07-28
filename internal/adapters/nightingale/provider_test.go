package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

const fixtureToken = "fixture-token-must-never-appear"

func TestListHostsPaginatesMapsAssetsAndAvoidsNPlusOne(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		mu.Lock()
		calls[request.URL.Path]++
		mu.Unlock()

		switch request.URL.Path {
		case "/api/n9e/targets":
			if request.Method != http.MethodGet || request.URL.Query().Get("limit") != "100" {
				t.Fatalf("targets request = %s %s", request.Method, request.URL.String())
			}
			page := request.URL.Query().Get("p")
			if page == "1" {
				writeFixture(t, w, "targets-page-1.json")
				return
			}
			if page == "2" {
				writeFixture(t, w, "targets-page-2.json")
				return
			}
			t.Fatalf("unexpected targets page %q", page)
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			if body.DatasourceID != 7 || len(body.Queries) != 2 {
				t.Fatalf("inventory batch = %#v", body)
			}
			writeFixture(t, w, "inventory-instant-batch.json")
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token:                fixtureToken,
		InterfaceExcludeExpr: defaultInterfaceExcludeExpr,
		HTTPClient:           server.Client(),
		Clock:                fixedClock,
	})

	hosts, err := provider.ListHosts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("hosts = %d, want 3", len(hosts))
	}
	if hosts[0].ID != "host-alpha" || hosts[0].Name != "host-alpha" || hosts[0].IP != "192.0.2.10" {
		t.Fatalf("first host identity = %#v", hosts[0])
	}
	if hosts[0].Status != datasource.StatusOnline || hosts[1].Status != datasource.StatusUnknown || hosts[2].Status != datasource.StatusOffline {
		t.Fatalf("statuses = %q, %q, %q", hosts[0].Status, hosts[1].Status, hosts[2].Status)
	}
	if hosts[0].CPUCores == nil || *hosts[0].CPUCores != 8 || hosts[1].CPUCores != nil {
		t.Fatalf("CPU cores = %#v, %#v", hosts[0].CPUCores, hosts[1].CPUCores)
	}
	if hosts[0].MemoryTotalBytes == nil || *hosts[0].MemoryTotalBytes != 17179869184 || hosts[1].MemoryTotalBytes != nil {
		t.Fatalf("memory totals = %#v, %#v", hosts[0].MemoryTotalBytes, hosts[1].MemoryTotalBytes)
	}
	if hosts[0].Uptime != time.Hour || hosts[1].Uptime != 90*time.Second+500*time.Millisecond || hosts[2].Uptime != 0 {
		t.Fatalf("uptimes = %s, %s, %s", hosts[0].Uptime, hosts[1].Uptime, hosts[2].Uptime)
	}
	if !hosts[0].StatusTime.Equal(time.Unix(1785123000, 0).UTC()) {
		t.Fatalf("status time = %s", hosts[0].StatusTime)
	}

	if _, err := provider.ListHosts(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls["/api/n9e/targets"] != 4 || calls["/api/n9e/query-instant-batch"] != 2 || calls["/api/n9e/datasource/brief"] != 1 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestGetCurrentMetricsUsesOneNestedBatchAndMapsMissingValues(t *testing.T) {
	instantCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			instantCalls++
			var body batchRequest
			decodeRequest(t, request, &body)
			if body.DatasourceID != 7 || len(body.Queries) != 6 {
				t.Fatalf("current batch = %#v", body)
			}
			for _, query := range body.Queries {
				if !strings.Contains(query.Query, "host-alpha") || !strings.Contains(query.Query, "host-beta") {
					t.Fatalf("query does not batch all hosts: %q", query.Query)
				}
			}
			if !strings.Contains(body.Queries[4].Query, `interface!~"lo|docker.*|veth.*|cali.*|br-.*|tunl.*"`) {
				t.Fatalf("network query = %q", body.Queries[4].Query)
			}
			writeFixture(t, w, "current-instant-batch.json")
		default:
			t.Fatalf("unexpected request %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	metrics, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha", "host-beta"})
	if err != nil {
		t.Fatal(err)
	}
	if instantCalls != 1 {
		t.Fatalf("instant calls = %d, want 1", instantCalls)
	}
	alpha := metrics["host-alpha"]
	assertFloat(t, alpha.CPUUsage, 12.5)
	assertFloat(t, alpha.MemoryUsage, 63.25)
	assertFloat(t, alpha.Load1, 1.75)
	assertFloat(t, alpha.IOBusyPercent, 8.5)
	assertFloat(t, alpha.NetworkTransmitBytesPerSecond, 2048)
	assertFloat(t, alpha.NetworkReceiveBytesPerSecond, 4096)
	if !alpha.Timestamp.Equal(time.Unix(1785123006, 0).UTC()) {
		t.Fatalf("timestamp = %s", alpha.Timestamp)
	}
	if metrics["host-beta"].CPUUsage != nil {
		t.Fatalf("NaN CPU must be missing: %#v", metrics["host-beta"].CPUUsage)
	}
}

func TestRangeAndAggregateQueriesMapNestedResponses(t *testing.T) {
	rangeCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-range-batch":
			rangeCalls++
			var body batchRequest
			decodeRequest(t, request, &body)
			if body.DatasourceID != 7 {
				t.Fatalf("datasource = %d", body.DatasourceID)
			}
			if rangeCalls == 1 {
				if len(body.Queries) != 1 || body.Queries[0].Start != 1785119400 || body.Queries[0].End != 1785119460 || body.Queries[0].Step != 60 {
					t.Fatalf("range body = %#v", body)
				}
				writeFixture(t, w, "range-batch.json")
				return
			}
			if len(body.Queries) != 2 {
				t.Fatalf("aggregate body = %#v", body)
			}
			writeFixture(t, w, "aggregate-range-batch.json")
		default:
			t.Fatalf("unexpected request %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	from := time.Unix(1785119400, 0).UTC()
	to := time.Unix(1785119460, 0).UTC()
	series, err := provider.QueryRange(context.Background(), datasource.RangeRequest{
		HostIDs: []string{"host-alpha", "host-beta"},
		Metric:  datasource.MetricCPUUsage,
		From:    from,
		To:      to,
		Step:    time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 || len(series[0].Points) != 2 || len(series[1].Points) != 2 {
		t.Fatalf("series = %#v", series)
	}
	assertFloat(t, series[0].Points[0].Value, 10.5)
	if series[1].Points[0].Value != nil {
		t.Fatalf("NaN range value must be missing: %#v", series[1].Points[0])
	}

	aggregate, err := provider.QueryAggregateRange(context.Background(), datasource.AggregateRangeRequest{
		Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
		Start: from,
		End:   to,
		Step:  time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(aggregate) != 2 || aggregate[0].HostID != "" || aggregate[0].Metric != datasource.MetricCPUUsage || aggregate[1].Metric != datasource.MetricMemoryUsage {
		t.Fatalf("aggregate = %#v", aggregate)
	}
	assertFloat(t, aggregate[0].Points[1].Value, 21)
	assertFloat(t, aggregate[1].Points[1].Value, 61)
}

func TestEmptyBatchReturnsMissingDataWithoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		if request.URL.Path == "/api/n9e/datasource/brief" {
			writeFixture(t, w, "datasource-brief.json")
			return
		}
		writeEnvelope(t, w, make([][]any, 6))
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	metrics, err := provider.GetCurrentMetrics(context.Background(), []string{"missing-host"})
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 || metrics["missing-host"].CPUUsage != nil {
		t.Fatalf("metrics = %#v", metrics)
	}
}

func TestListHostsHandlesOneHundredTargetsWithOneAssetBatch(t *testing.T) {
	instantCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/targets":
			list := make([]map[string]any, 100)
			for i := range list {
				list[i] = map[string]any{
					"ident":     "scale-host-" + strconv.Itoa(i+1),
					"host_ip":   "192.0.2.1",
					"os":        "linux",
					"beat_time": int64(1785123000),
					"target_up": 2,
					"cpu_num":   4,
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"dat": map[string]any{"list": list, "total": 100}, "err": "", "request_id": "scale"})
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			instantCalls++
			var body batchRequest
			decodeRequest(t, request, &body)
			if len(body.Queries) != 2 || !strings.Contains(body.Queries[0].Query, "scale-host-100") {
				t.Fatalf("scale batch = %#v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dat":[[],[]],"err":"","request_id":"scale-metrics"}`)
		default:
			t.Fatalf("unexpected request %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	hosts, err := provider.ListHosts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 100 || instantCalls != 1 {
		t.Fatalf("hosts = %d, instant calls = %d", len(hosts), instantCalls)
	}
}

func TestPromQLEscapesHostLiteralsAndConfiguredRegexString(t *testing.T) {
	queries := currentPromQL([]string{`node".*\`}, `lo|bad".*\`)
	if got, want := queries[0], `cpu_usage_active{cpu="cpu-total",ident=~"^(?:node\"\\.\\*\\\\)$"}`; got != want {
		t.Fatalf("CPU query = %q, want %q", got, want)
	}
	if got, want := queries[4], `sum by (ident) (rate(net_bytes_sent{ident=~"^(?:node\"\\.\\*\\\\)$",interface!~"lo|bad\".*\\"}[2m]))`; got != want {
		t.Fatalf("network query = %q, want %q", got, want)
	}
}

func TestHTTPFailuresAreSafeUnavailableErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		fixture     string
		body        string
		maxBytes    int64
	}{
		{name: "unauthorized text", status: http.StatusUnauthorized, contentType: "text/plain", body: "unauthorized"},
		{name: "non JSON", status: http.StatusOK, contentType: "text/html", fixture: "non-json.html"},
		{name: "problem JSON media type", status: http.StatusOK, contentType: "application/problem+json", body: `{"dat":{},"err":""}`},
		{name: "text plus JSON media type", status: http.StatusOK, contentType: "text/plain+json", body: `{"dat":{},"err":""}`},
		{name: "envelope error", status: http.StatusOK, contentType: "application/json", fixture: "envelope-error.json"},
		{name: "blank envelope error", status: http.StatusOK, contentType: "application/json", body: `{"dat":{},"err":"   "}`},
		{name: "malformed JSON", status: http.StatusOK, contentType: "application/json", body: `{"dat":`},
		{name: "missing envelope error field", status: http.StatusOK, contentType: "application/json", body: `{"dat":{}}`},
		{name: "response too large", status: http.StatusOK, contentType: "application/json", body: `{"dat":{},"err":""}`, maxBytes: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Header.Get("X-User-Token") != fixtureToken {
					t.Fatalf("token header = %q", request.Header.Get("X-User-Token"))
				}
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				if tt.fixture != "" {
					fixture, err := os.ReadFile("testdata/" + tt.fixture)
					if err != nil {
						t.Fatal(err)
					}
					_, _ = w.Write(fixture)
					return
				}
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock, MaxResponseBytes: tt.maxBytes})
			_, err := provider.Health(context.Background())
			if !errors.Is(err, datasource.ErrUnavailable) {
				t.Fatalf("error = %v, want ErrUnavailable", err)
			}
			if strings.Contains(err.Error(), fixtureToken) || strings.Contains(err.Error(), "fixture upstream failure") || strings.Contains(err.Error(), "unauthorized") {
				t.Fatalf("unsafe error = %q", err)
			}
		})
	}
}

func TestProviderAcceptsJSONContentTypeWithParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"dat":{},"err":""}`)
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	if _, err := provider.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}

func TestProviderRejectsRedirectWithoutForwardingToken(t *testing.T) {
	var destinationMu sync.Mutex
	destinationRequests := 0
	destinationToken := ""
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		destinationMu.Lock()
		destinationRequests++
		destinationToken = request.Header.Get("X-User-Token")
		destinationMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"dat":{},"err":""}`)
	}))
	defer destination.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/n9e/self/profile" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		http.Redirect(w, request, destination.URL, http.StatusFound)
	}))
	defer upstream.Close()

	callerRedirectError := errors.New("caller redirect policy")
	callerRedirect := func(*http.Request, []*http.Request) error {
		return callerRedirectError
	}
	callerClient := upstream.Client()
	callerClient.CheckRedirect = callerRedirect
	callerClient.Timeout = 2 * time.Second
	provider := New(Options{BaseURL: upstream.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: callerClient, Clock: fixedClock})
	if callerClient.Timeout != 2*time.Second {
		t.Fatalf("caller Timeout = %s, want 2s", callerClient.Timeout)
	}
	if callerClient.CheckRedirect == nil || !errors.Is(callerClient.CheckRedirect(nil, nil), callerRedirectError) {
		t.Fatal("caller CheckRedirect was modified")
	}
	_, err := provider.Health(context.Background())
	if !errors.Is(err, datasource.ErrUnavailable) {
		t.Fatalf("Health() error = %v, want ErrUnavailable", err)
	}
	destinationMu.Lock()
	defer destinationMu.Unlock()
	if destinationRequests != 0 || destinationToken != "" {
		t.Fatalf("redirect destination requests/token = %d/%t, want 0/false", destinationRequests, destinationToken != "")
	}
}

func TestProviderRejectsNullData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"dat":null,"err":""}`)
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	_, err := provider.Health(context.Background())
	if !errors.Is(err, datasource.ErrUnavailable) {
		t.Fatalf("Health() error = %v, want ErrUnavailable", err)
	}
}

func TestProviderRejectsInvalidTargetPage(t *testing.T) {
	tests := []struct {
		name  string
		pages []any
	}{
		{name: "missing list", pages: []any{map[string]any{"total": 0}}},
		{name: "missing total", pages: []any{map[string]any{"list": []any{}}}},
		{name: "negative total", pages: []any{map[string]any{"list": []any{}, "total": -1}}},
		{name: "total changes between pages", pages: []any{
			map[string]any{"list": []any{map[string]any{"ident": "page-one"}}, "total": 2},
			map[string]any{"list": []any{map[string]any{"ident": "page-two"}}, "total": 3},
		}},
		{name: "empty page before total", pages: []any{map[string]any{"list": []any{}, "total": 1}}},
		{name: "records exceed total", pages: []any{map[string]any{"list": []any{
			map[string]any{"ident": "one"}, map[string]any{"ident": "two"},
		}, "total": 1}}},
		{name: "empty ident", pages: []any{map[string]any{"list": []any{map[string]any{"ident": ""}}, "total": 1}}},
		{name: "duplicate ident", pages: []any{map[string]any{"list": []any{
			map[string]any{"ident": "same"}, map[string]any{"ident": "same"},
		}, "total": 2}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assertAuthenticatedJSONRequest(t, request)
				if request.URL.Path != "/api/n9e/targets" {
					t.Fatalf("path = %q", request.URL.Path)
				}
				page, err := strconv.Atoi(request.URL.Query().Get("p"))
				if err != nil || page < 1 || page > len(tt.pages) {
					t.Fatalf("page = %q", request.URL.Query().Get("p"))
				}
				writeEnvelope(t, w, tt.pages[page-1])
			}))
			defer server.Close()

			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
			_, err := provider.ListHosts(context.Background())
			if !errors.Is(err, datasource.ErrUnavailable) {
				t.Fatalf("ListHosts() error = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestProviderRejectsMismatchedBatchResultCount(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		groups int
		run    func(datasource.Provider) error
	}{
		{
			name: "instant fewer result groups", path: "/api/n9e/query-instant-batch", groups: 5,
			run: func(provider datasource.Provider) error {
				_, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
				return err
			},
		},
		{
			name: "instant more result groups", path: "/api/n9e/query-instant-batch", groups: 7,
			run: func(provider datasource.Provider) error {
				_, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
				return err
			},
		},
		{
			name: "range fewer result groups", path: "/api/n9e/query-range-batch", groups: 1,
			run: func(provider datasource.Provider) error {
				_, err := provider.QueryAggregateRange(context.Background(), datasource.AggregateRangeRequest{
					Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
					Start: time.Unix(1785119400, 0).UTC(),
					End:   time.Unix(1785119460, 0).UTC(),
					Step:  time.Minute,
				})
				return err
			},
		},
		{
			name: "range more result groups", path: "/api/n9e/query-range-batch", groups: 3,
			run: func(provider datasource.Provider) error {
				_, err := provider.QueryAggregateRange(context.Background(), datasource.AggregateRangeRequest{
					Keys:  []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage},
					Start: time.Unix(1785119400, 0).UTC(),
					End:   time.Unix(1785119460, 0).UTC(),
					Step:  time.Minute,
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assertAuthenticatedJSONRequest(t, request)
				switch request.URL.Path {
				case "/api/n9e/datasource/brief":
					writeFixture(t, w, "datasource-brief.json")
				case tt.path:
					groups := make([][]any, tt.groups)
					writeEnvelope(t, w, groups)
				default:
					t.Fatalf("path = %q", request.URL.Path)
				}
			}))
			defer server.Close()

			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
			if err := tt.run(provider); !errors.Is(err, datasource.ErrUnavailable) {
				t.Fatalf("error = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestDatasourceDiscoveryCoalescesConcurrentRequests(t *testing.T) {
	const workers = 8
	var mu sync.Mutex
	datasourceCalls := 0
	discoveryStarted := make(chan struct{})
	releaseDiscovery := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			mu.Lock()
			datasourceCalls++
			call := datasourceCalls
			mu.Unlock()
			if call == 1 {
				close(discoveryStarted)
				<-releaseDiscovery
			}
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			writeEnvelope(t, w, make([][]any, len(body.Queries)))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	start := make(chan struct{})
	errorsByWorker := make(chan error, workers)
	var workersReady sync.WaitGroup
	workersReady.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			workersReady.Done()
			<-start
			_, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
			errorsByWorker <- err
		}()
	}
	workersReady.Wait()
	close(start)
	<-discoveryStarted
	close(releaseDiscovery)
	for index := 0; index < workers; index++ {
		if err := <-errorsByWorker; err != nil {
			t.Fatalf("GetCurrentMetrics() error = %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if datasourceCalls != 1 {
		t.Fatalf("datasource discovery calls = %d, want 1", datasourceCalls)
	}
}

func TestDatasourceDiscoveryRetriesAfterFailure(t *testing.T) {
	datasourceCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			datasourceCalls++
			if datasourceCalls == 1 {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"dat":[],"err":"failed"}`)
				return
			}
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			writeEnvelope(t, w, make([][]any, len(body.Queries)))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	if _, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"}); !errors.Is(err, datasource.ErrUnavailable) {
		t.Fatalf("first GetCurrentMetrics() error = %v, want ErrUnavailable", err)
	}
	if _, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"}); err != nil {
		t.Fatalf("second GetCurrentMetrics() error = %v", err)
	}
	if datasourceCalls != 2 {
		t.Fatalf("datasource discovery calls = %d, want 2", datasourceCalls)
	}
}

func TestDatasourceDiscoveryCoalescesConcurrentFailureAndRetries(t *testing.T) {
	const workers = 8
	var mu sync.Mutex
	datasourceCalls := 0
	discoveryStarted := make(chan struct{})
	releaseDiscovery := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			mu.Lock()
			datasourceCalls++
			call := datasourceCalls
			mu.Unlock()
			if call == 1 {
				close(discoveryStarted)
				<-releaseDiscovery
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"dat":[],"err":"failed"}`)
				return
			}
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			writeEnvelope(t, w, make([][]any, len(body.Queries)))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	start := make(chan struct{})
	errorsByWorker := make(chan error, workers)
	var workersReady sync.WaitGroup
	workersReady.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			workersReady.Done()
			<-start
			_, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
			errorsByWorker <- err
		}()
	}
	workersReady.Wait()
	close(start)
	<-discoveryStarted
	close(releaseDiscovery)
	for index := 0; index < workers; index++ {
		if err := <-errorsByWorker; !errors.Is(err, datasource.ErrUnavailable) {
			t.Fatalf("concurrent GetCurrentMetrics() error = %v, want ErrUnavailable", err)
		}
	}
	mu.Lock()
	if datasourceCalls != 1 {
		mu.Unlock()
		t.Fatalf("failed discovery calls = %d, want 1", datasourceCalls)
	}
	mu.Unlock()

	if _, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"}); err != nil {
		t.Fatalf("retry GetCurrentMetrics() error = %v", err)
	}
	if _, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"}); err != nil {
		t.Fatalf("cached GetCurrentMetrics() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if datasourceCalls != 2 {
		t.Fatalf("discovery calls after retry/cache = %d, want 2", datasourceCalls)
	}
}

func TestDatasourceDiscoveryWaiterRespectsContextCancellation(t *testing.T) {
	var mu sync.Mutex
	datasourceCalls := 0
	discoveryStarted := make(chan struct{})
	releaseDiscovery := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseDiscovery) })
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			mu.Lock()
			datasourceCalls++
			mu.Unlock()
			close(discoveryStarted)
			<-releaseDiscovery
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			writeEnvelope(t, w, make([][]any, len(body.Queries)))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	defer release()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	leaderResult := make(chan error, 1)
	go func() {
		_, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
		leaderResult <- err
	}()
	<-discoveryStarted

	ctx, cancel := context.WithCancel(context.Background())
	waiterResult := make(chan error, 1)
	go func() {
		_, err := provider.GetCurrentMetrics(ctx, []string{"host-alpha"})
		waiterResult <- err
	}()
	cancel()
	select {
	case err := <-waiterResult:
		if !errors.Is(err, datasource.ErrUnavailable) {
			t.Fatalf("cancelled waiter error = %v, want ErrUnavailable", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter remained blocked on discovery")
	}
	release()
	if err := <-leaderResult; err != nil {
		t.Fatalf("leader GetCurrentMetrics() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if datasourceCalls != 1 {
		t.Fatalf("datasource discovery calls = %d, want 1", datasourceCalls)
	}
}

func TestPromQLWhitelistUsesExactQueriesAndSkipsUnsupportedMetrics(t *testing.T) {
	var rangeQueries []string
	datasourceCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			datasourceCalls++
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-range-batch":
			var body batchRequest
			decodeRequest(t, request, &body)
			for _, query := range body.Queries {
				rangeQueries = append(rangeQueries, query.Query)
			}
			writeEnvelope(t, w, make([][]any, len(body.Queries)))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	from := time.Unix(1785119400, 0).UTC()
	to := time.Unix(1785119460, 0).UTC()
	for _, metric := range []datasource.MetricKey{
		datasource.MetricDiskUsage,
		datasource.MetricDiskReadBytesPerSecond,
		datasource.MetricDiskWriteBytesPerSecond,
	} {
		if _, err := provider.QueryRange(context.Background(), datasource.RangeRequest{HostIDs: []string{"host-alpha", "host-beta"}, Metric: metric, From: from, To: to, Step: time.Minute}); err != nil {
			t.Fatalf("unsupported %q error = %v", metric, err)
		}
	}
	if len(rangeQueries) != 0 || datasourceCalls != 0 {
		t.Fatalf("unsupported metric requests/discoveries = %d/%d, want 0/0", len(rangeQueries), datasourceCalls)
	}

	supported := []struct {
		metric datasource.MetricKey
		query  string
	}{
		{datasource.MetricCPUUsage, `cpu_usage_active{cpu="cpu-total",ident=~"^(?:host-alpha|host-beta)$"}`},
		{datasource.MetricMemoryUsage, `mem_used_percent{ident=~"^(?:host-alpha|host-beta)$"}`},
		{datasource.MetricLoad1, `system_load1{ident=~"^(?:host-alpha|host-beta)$"}`},
		{datasource.MetricIOBusyPercent, `max by (ident) (diskio_io_util{ident=~"^(?:host-alpha|host-beta)$"})`},
		{datasource.MetricNetworkTransmitBytesPerSecond, `sum by (ident) (rate(net_bytes_sent{ident=~"^(?:host-alpha|host-beta)$",interface!~"lo|docker.*|veth.*|cali.*|br-.*|tunl.*"}[2m]))`},
		{datasource.MetricNetworkReceiveBytesPerSecond, `sum by (ident) (rate(net_bytes_recv{ident=~"^(?:host-alpha|host-beta)$",interface!~"lo|docker.*|veth.*|cali.*|br-.*|tunl.*"}[2m]))`},
	}
	for _, tt := range supported {
		if _, err := provider.QueryRange(context.Background(), datasource.RangeRequest{HostIDs: []string{"host-alpha", "host-beta"}, Metric: tt.metric, From: from, To: to, Step: time.Minute}); err != nil {
			t.Fatalf("QueryRange(%q) error = %v", tt.metric, err)
		}
	}
	if len(rangeQueries) != len(supported) {
		t.Fatalf("range queries = %d, want %d", len(rangeQueries), len(supported))
	}
	for index, tt := range supported {
		if rangeQueries[index] != tt.query {
			t.Fatalf("range query %d = %q, want %q", index, rangeQueries[index], tt.query)
		}
	}

	if _, err := provider.QueryAggregateRange(context.Background(), datasource.AggregateRangeRequest{
		Keys: []datasource.MetricKey{
			datasource.MetricCPUUsage,
			datasource.MetricMemoryUsage,
			datasource.MetricLoad1,
			datasource.MetricIOBusyPercent,
			datasource.MetricNetworkTransmitBytesPerSecond,
			datasource.MetricNetworkReceiveBytesPerSecond,
		},
		Start: from,
		End:   to,
		Step:  time.Minute,
	}); err != nil {
		t.Fatalf("QueryAggregateRange() error = %v", err)
	}
	aggregateQueries := rangeQueries[len(supported):]
	wantAggregateQueries := []string{
		`avg(cpu_usage_active{cpu="cpu-total",ident!=""})`,
		`avg(mem_used_percent{ident!=""})`,
		`avg(system_load1{ident!=""})`,
		`avg(max by (ident) (diskio_io_util{ident!=""}))`,
		`avg(sum by (ident) (rate(net_bytes_sent{ident!="",interface!~"lo|docker.*|veth.*|cali.*|br-.*|tunl.*"}[2m])))`,
		`avg(sum by (ident) (rate(net_bytes_recv{ident!="",interface!~"lo|docker.*|veth.*|cali.*|br-.*|tunl.*"}[2m])))`,
	}
	if len(aggregateQueries) != len(wantAggregateQueries) {
		t.Fatalf("aggregate queries = %d, want %d", len(aggregateQueries), len(wantAggregateQueries))
	}
	for index, want := range wantAggregateQueries {
		if aggregateQueries[index] != want {
			t.Fatalf("aggregate query %d = %q, want %q", index, aggregateQueries[index], want)
		}
	}
}

func TestProviderRejectsUnsafeBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"ftp://n9e.example.test",
		"http://n9e.example.test",
		"https://user:pass@n9e.example.test",
		"https://n9e.example.test?query=value",
		"https://n9e.example.test#fragment",
	} {
		t.Run(baseURL, func(t *testing.T) {
			provider := New(Options{BaseURL: baseURL, Token: fixtureToken, Clock: fixedClock})
			_, err := provider.Health(context.Background())
			if !errors.Is(err, datasource.ErrNotConfigured) {
				t.Fatalf("Health() error = %v, want ErrNotConfigured", err)
			}
			if strings.Contains(err.Error(), baseURL) || strings.Contains(err.Error(), fixtureToken) {
				t.Fatalf("unsafe constructor error = %q", err)
			}
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/base/api/n9e/self/profile" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		assertAuthenticatedJSONRequest(t, request)
		writeEnvelope(t, w, map[string]any{})
	})
	plainServer := httptest.NewServer(handler)
	defer plainServer.Close()
	secureServer := httptest.NewTLSServer(handler)
	defer secureServer.Close()
	for _, tt := range []struct {
		name              string
		baseURL           string
		client            *http.Client
		allowInsecureHTTP bool
	}{
		{name: "explicit insecure http", baseURL: plainServer.URL + "/base", client: plainServer.Client(), allowInsecureHTTP: true},
		{name: "https", baseURL: secureServer.URL + "/base", client: secureServer.Client()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := New(Options{BaseURL: tt.baseURL, Token: fixtureToken, HTTPClient: tt.client, Clock: fixedClock, AllowInsecureHTTP: tt.allowInsecureHTTP})
			if _, err := provider.Health(context.Background()); err != nil {
				t.Fatalf("Health() error = %v", err)
			}
		})
	}
}

func TestProviderNumericBoundaries(t *testing.T) {
	assetCases := []struct {
		name       string
		memory     string
		uptime     string
		wantMemory *int64
		wantUptime time.Duration
	}{
		{name: "memory max int64 loses integer precision", memory: "9223372036854775807", uptime: "3600", wantUptime: time.Hour},
		{name: "memory NaN", memory: "NaN", uptime: "3600", wantUptime: time.Hour},
		{name: "memory positive infinity", memory: "+Inf", uptime: "3600", wantUptime: time.Hour},
		{name: "memory negative infinity", memory: "-Inf", uptime: "3600", wantUptime: time.Hour},
		{name: "uptime duration boundary", memory: "1024", uptime: "9223372036.854776", wantMemory: int64Pointer(1024)},
		{name: "uptime NaN", memory: "1024", uptime: "NaN", wantMemory: int64Pointer(1024)},
		{name: "uptime positive infinity", memory: "1024", uptime: "+Inf", wantMemory: int64Pointer(1024)},
		{name: "uptime negative infinity", memory: "1024", uptime: "-Inf", wantMemory: int64Pointer(1024)},
		{name: "valid values", memory: "1024", uptime: "90.5", wantMemory: int64Pointer(1024), wantUptime: 90*time.Second + 500*time.Millisecond},
	}
	for _, tt := range assetCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assertAuthenticatedJSONRequest(t, request)
				switch request.URL.Path {
				case "/api/n9e/targets":
					writeEnvelope(t, w, map[string]any{
						"list":  []any{map[string]any{"ident": "host-alpha"}},
						"total": 1,
					})
				case "/api/n9e/datasource/brief":
					writeFixture(t, w, "datasource-brief.json")
				case "/api/n9e/query-instant-batch":
					writeEnvelope(t, w, [][]any{
						{map[string]any{"metric": map[string]string{"ident": "host-alpha"}, "value": []any{"1785123000", tt.memory}}},
						{map[string]any{"metric": map[string]string{"ident": "host-alpha"}, "value": []any{"1785123000", tt.uptime}}},
					})
				default:
					t.Fatalf("path = %q", request.URL.Path)
				}
			}))
			defer server.Close()

			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
			hosts, err := provider.ListHosts(context.Background())
			if err != nil {
				t.Fatalf("ListHosts() error = %v", err)
			}
			if len(hosts) != 1 {
				t.Fatalf("hosts = %#v", hosts)
			}
			if tt.wantMemory == nil {
				if hosts[0].MemoryTotalBytes != nil {
					t.Fatalf("memory = %d, want missing", *hosts[0].MemoryTotalBytes)
				}
			} else if hosts[0].MemoryTotalBytes == nil || *hosts[0].MemoryTotalBytes != *tt.wantMemory {
				t.Fatalf("memory = %#v, want %d", hosts[0].MemoryTotalBytes, *tt.wantMemory)
			}
			if hosts[0].Uptime != tt.wantUptime || hosts[0].Uptime < 0 {
				t.Fatalf("uptime = %s, want %s", hosts[0].Uptime, tt.wantUptime)
			}
		})
	}

	timestampCases := []struct {
		name          string
		timestamp     string
		wantTimestamp time.Time
		wantValue     *float64
	}{
		{name: "timestamp beyond safe Unix conversion", timestamp: "1e300", wantTimestamp: fixedClock()},
		{name: "timestamp NaN", timestamp: "NaN", wantTimestamp: fixedClock()},
		{name: "timestamp positive infinity", timestamp: "+Inf", wantTimestamp: fixedClock()},
		{name: "timestamp negative infinity", timestamp: "-Inf", wantTimestamp: fixedClock()},
		{name: "timestamp preserves fractional seconds", timestamp: "1785123000.5", wantTimestamp: time.Unix(1785123000, 500000000).UTC(), wantValue: float64Pointer(12.5)},
	}
	for _, tt := range timestampCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				assertAuthenticatedJSONRequest(t, request)
				switch request.URL.Path {
				case "/api/n9e/datasource/brief":
					writeFixture(t, w, "datasource-brief.json")
				case "/api/n9e/query-instant-batch":
					groups := make([][]any, 6)
					groups[0] = []any{map[string]any{"metric": map[string]string{"ident": "host-alpha"}, "value": []any{tt.timestamp, "12.5"}}}
					writeEnvelope(t, w, groups)
				default:
					t.Fatalf("path = %q", request.URL.Path)
				}
			}))
			defer server.Close()

			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
			metrics, err := provider.GetCurrentMetrics(context.Background(), []string{"host-alpha"})
			if err != nil {
				t.Fatalf("GetCurrentMetrics() error = %v", err)
			}
			got := metrics["host-alpha"]
			if !got.Timestamp.Equal(tt.wantTimestamp) {
				t.Fatalf("timestamp = %s, want %s", got.Timestamp, tt.wantTimestamp)
			}
			if tt.wantValue == nil {
				if got.CPUUsage != nil {
					t.Fatalf("CPU value = %v, want missing", *got.CPUUsage)
				}
			} else {
				assertFloat(t, got.CPUUsage, *tt.wantValue)
			}
		})
	}
}

func TestParseUnixTimeEnforcesJSONYearBounds(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want time.Time
		ok   bool
	}{
		{name: "year zero lower boundary", raw: json.RawMessage(`-62167219200`), want: time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC), ok: true},
		{name: "year zero fractional second", raw: json.RawMessage(`-62167219199.5`), want: time.Date(0, time.January, 1, 0, 0, 0, 500000000, time.UTC), ok: true},
		{name: "before year zero", raw: json.RawMessage(`-62167219201`), ok: false},
		{name: "fraction before year zero", raw: json.RawMessage(`-62167219200.5`), ok: false},
		{name: "year 9999 fractional second", raw: json.RawMessage(`253402300799.5`), want: time.Date(9999, time.December, 31, 23, 59, 59, 500000000, time.UTC), ok: true},
		{name: "year 10000 boundary", raw: json.RawMessage(`253402300800`), ok: false},
		{name: "after year 10000", raw: json.RawMessage(`253402300800.5`), ok: false},
		{name: "ordinary fractional second", raw: json.RawMessage(`1785123000.5`), want: time.Unix(1785123000, 500000000).UTC(), ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseUnixTime(tt.raw)
			if ok != tt.ok {
				t.Fatalf("parseUnixTime(%s) ok = %t, want %t", tt.raw, ok, tt.ok)
			}
			if ok && !got.Equal(tt.want) {
				t.Fatalf("parseUnixTime(%s) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMapTargetRecordsRejectsUnrepresentableBeatTime(t *testing.T) {
	tests := []struct {
		name           string
		beatTime       int64
		wantStatusTime time.Time
		wantMissing    bool
	}{
		{name: "year 9999 upper boundary", beatTime: jsonUnixMaxSecondsExclusive - 1, wantStatusTime: time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)},
		{name: "year 10000 boundary", beatTime: jsonUnixMaxSecondsExclusive, wantMissing: true},
		{name: "extreme int64", beatTime: int64ExclusiveUpperBound - 1, wantMissing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts, _, err := mapTargetRecords([]targetRecord{{Ident: "host-alpha", BeatTime: tt.beatTime}})
			if err != nil {
				t.Fatalf("mapTargetRecords() error = %v", err)
			}
			if len(hosts) != 1 {
				t.Fatalf("hosts = %#v", hosts)
			}
			if tt.wantMissing {
				if !hosts[0].StatusTime.IsZero() {
					t.Fatalf("StatusTime = %s, want zero", hosts[0].StatusTime)
				}
			} else if !hosts[0].StatusTime.Equal(tt.wantStatusTime) {
				t.Fatalf("StatusTime = %s, want %s", hosts[0].StatusTime, tt.wantStatusTime)
			}
			if _, err := json.Marshal(hosts); err != nil {
				t.Fatalf("json.Marshal(hosts) error = %v", err)
			}
		})
	}
}

func TestUnconfiguredProviderReturnsNotConfigured(t *testing.T) {
	provider := New(Options{})
	_, err := provider.Health(context.Background())
	if !errors.Is(err, datasource.ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

type batchRequest struct {
	DatasourceID int64        `json:"datasource_id"`
	Queries      []batchQuery `json:"queries"`
}

type batchQuery struct {
	Time  int64  `json:"time"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Step  int64  `json:"step"`
	Query string `json:"query"`
}

func assertAuthenticatedJSONRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("X-User-Token") != fixtureToken {
		t.Fatalf("X-User-Token = %q", request.Header.Get("X-User-Token"))
	}
	if request.Header.Get("Accept") != "application/json" {
		t.Fatalf("Accept = %q", request.Header.Get("Accept"))
	}
	if request.Method == http.MethodPost && request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", request.Header.Get("Content-Type"))
	}
}

func decodeRequest(t *testing.T, request *http.Request, target any) {
	t.Helper()
	defer request.Body.Close()
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func writeFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	fixture, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(fixture)
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"dat": data, "err": ""}); err != nil {
		t.Fatal(err)
	}
}

func fixedClock() time.Time {
	return time.Unix(1785123060, 0).UTC()
}

func assertFloat(t *testing.T, value *float64, want float64) {
	t.Helper()
	if value == nil || *value != want {
		t.Fatalf("value = %#v, want %v", value, want)
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func float64Pointer(value float64) *float64 {
	return &value
}
