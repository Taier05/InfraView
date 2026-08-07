package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/elasticsearch"
	"github.com/Taier05/InfraView/internal/service"
)

func TestElasticsearchRoutesRequireAuthentication(t *testing.T) {
	handler, _ := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
	for _, path := range []string{
		"/api/v1/elasticsearch/overview",
		"/api/v1/elasticsearch/nodes",
	} {
		response := request(t, handler, http.MethodGet, path, "", nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestElasticsearchOverviewReturnsExplicitView(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
	response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/overview", "", sessionCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "status", "clusters", "nodes", "alerts")
	for _, path := range []string{"data.clusters", "data.nodes"} {
		assertJSONObjectKeys(t, body, path, "total", "normal", "warning", "critical", "unknown")
	}
	assertJSONObjectKeys(t, body, "data.alerts", "cluster_health", "node_resource", "unassigned_shards", "request_rejections")
	for _, category := range []string{"cluster_health", "node_resource", "unassigned_shards", "request_rejections"} {
		assertJSONObjectKeys(t, body, "data.alerts."+category, "warning", "critical")
	}
	assertElasticsearchResponseHasNoSensitiveKeys(t, body)
}

func TestElasticsearchNodesReturnsExplicitView(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
	response := request(t, handler, http.MethodGet,
		"/api/v1/elasticsearch/nodes?search=fixture&cluster=fixture-cluster&role=data&cluster_health=green&status=normal&sort=node&order=asc&page=1&page_size=20",
		"", sessionCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "nodes", "available_clusters", "available_roles", "total", "page", "page_size", "total_pages")
	assertJSONObjectKeys(t, body, "data.nodes.0",
		"id", "name", "cluster", "address", "roles", "cluster_health",
		"heap_usage_percent", "disk_usage_percent", "cpu_usage_percent", "index_rate", "search_rate",
		"documents", "store_size_bytes", "thread_pool_queue", "rejected_rate", "uptime_seconds",
		"status", "status_source", "collection_level",
	)
	if !jsonPathIsArray(t, body, "data.nodes.0.roles") || !jsonPathIsArray(t, body, "data.available_clusters") || !jsonPathIsArray(t, body, "data.available_roles") {
		t.Fatal("Elasticsearch list fields must be arrays")
	}
	assertElasticsearchResponseHasNoSensitiveKeys(t, body)
}

func TestElasticsearchNodesEncodesEmptyCollectionsAsArrays(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearch.Snapshot{})
	response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes", "", sessionCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"data.nodes", "data.available_clusters", "data.available_roles"} {
		items, ok := jsonPathValue(t, response.Body.Bytes(), path).([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("JSON path %q = %#v, want []", path, jsonPathValue(t, response.Body.Bytes(), path))
		}
	}
	if got := jsonPathValue(t, response.Body.Bytes(), "data.total_pages"); got != float64(0) {
		t.Fatalf("total_pages = %#v, want 0", got)
	}
}

func TestElasticsearchNodesPageSize500(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchPageSize500Fixture())

	response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes?page=1&page_size=500", "", sessionCookie)
	assertListPageSize500(t, response, "nodes", "nodes", "available_clusters", "available_roles", "total", "page", "page_size", "total_pages")
}

func TestElasticsearchNodesRejectsInvalidPageSize(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())

	assertRejectsInvalidListPageSizes(t, handler, sessionCookie, "/api/v1/elasticsearch/nodes")
}

func TestElasticsearchMissingServiceReturnsSafeUnavailable(t *testing.T) {
	handler := newElasticsearchAPIHandler(t, nil, testNow)
	sessionCookie := loginCookie(t, handler)
	for _, path := range []string{"/api/v1/elasticsearch/overview", "/api/v1/elasticsearch/nodes"} {
		response := request(t, handler, http.MethodGet, path, "", sessionCookie)
		assertError(t, response, http.StatusServiceUnavailable, "elasticsearch_unavailable", "数据源暂时不可用，请稍后重试")
	}
}

func TestElasticsearchInitialProviderFailureReturnsSafeUnavailable(t *testing.T) {
	provider := &elasticsearchHTTPTestProvider{err: errors.Join(elasticsearch.ErrUnavailable, errors.New("fixture upstream body"))}
	handler, sessionCookie := newElasticsearchAPIProviderTestHandler(t, provider, testNow, time.Second)
	response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/overview", "", sessionCookie)
	assertError(t, response, http.StatusServiceUnavailable, "elasticsearch_unavailable", "数据源暂时不可用，请稍后重试")
	if strings.Contains(response.Body.String(), "fixture upstream body") {
		t.Fatal("response leaks upstream error")
	}
}

func TestElasticsearchReturnsStaleSnapshotAfterProviderFailure(t *testing.T) {
	clock := &elasticsearchHTTPClock{now: time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)}
	provider := &elasticsearchHTTPTestProvider{snapshot: elasticsearchHTTPFixture()}
	handler, sessionCookie := newElasticsearchAPIProviderTestHandler(t, provider, clock.Now, time.Second)
	first := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes", "", sessionCookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	clock.Advance(2 * time.Second)
	provider.err = elasticsearch.ErrUnavailable
	stale := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes", "", sessionCookie)
	if stale.Code != http.StatusOK {
		t.Fatalf("stale status = %d, body = %s", stale.Code, stale.Body.String())
	}
	if got := jsonPathValue(t, stale.Body.Bytes(), "meta.stale"); got != true {
		t.Fatalf("meta.stale = %#v, want true", got)
	}
}

func TestElasticsearchReturnsFreshSnapshotAfterSuccessfulReload(t *testing.T) {
	clock := &elasticsearchHTTPClock{now: time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)}
	provider := &elasticsearchHTTPTestProvider{snapshot: elasticsearchHTTPFixture()}
	handler, sessionCookie := newElasticsearchAPIProviderTestHandler(t, provider, clock.Now, time.Second)
	first := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes", "", sessionCookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	if got := jsonPathValue(t, first.Body.Bytes(), "meta.stale"); got != false {
		t.Fatalf("first meta.stale = %#v, want false", got)
	}
	firstCollectedAt, ok := jsonPathValue(t, first.Body.Bytes(), "meta.collected_at").(string)
	if !ok {
		t.Fatalf("first meta.collected_at = %#v, want string", jsonPathValue(t, first.Body.Bytes(), "meta.collected_at"))
	}
	clock.Advance(2 * time.Second)
	provider.snapshot.Clusters[0].ReportedAt = provider.snapshot.Clusters[0].ReportedAt.Add(time.Second)
	provider.snapshot.Nodes[0].ReportedAt = provider.snapshot.Nodes[0].ReportedAt.Add(time.Second)
	wantCollectedAt := provider.snapshot.Nodes[0].ReportedAt.UTC().Format(time.RFC3339)
	second := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes", "", sessionCookie)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	if got := jsonPathValue(t, second.Body.Bytes(), "meta.stale"); got != false {
		t.Fatalf("second meta.stale = %#v, want false", got)
	}
	if got := jsonPathValue(t, second.Body.Bytes(), "meta.collected_at"); got != wantCollectedAt || got == firstCollectedAt {
		t.Fatalf("second meta.collected_at = %#v, want updated %q instead of %q", got, wantCollectedAt, firstCollectedAt)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

func TestElasticsearchOverviewRejectsQueryParameters(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
	response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/overview?search=fixture", "", sessionCookie)
	assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
}

func TestElasticsearchNodesRejectsInvalidQueryParameters(t *testing.T) {
	tests := map[string]string{
		"unknown":              "?unknown=value",
		"duplicate":            "?search=a&search=b",
		"empty search":         "?search=",
		"empty cluster":        "?cluster=",
		"empty role":           "?role=",
		"empty cluster health": "?cluster_health=",
		"empty status":         "?status=",
		"empty sort":           "?sort=",
		"empty order":          "?order=",
		"empty page":           "?page=",
		"empty page size":      "?page_size=",
		"invalid role":         "?role=invalid",
		"invalid health":       "?cluster_health=invalid",
		"invalid status":       "?status=invalid",
		"invalid sort":         "?sort=invalid",
		"invalid order":        "?order=invalid",
		"invalid page":         "?page=invalid",
		"zero page":            "?page=0",
		"zero page size":       "?page_size=0",
		"invalid page size":    "?page_size=10",
	}
	for name, query := range tests {
		t.Run(name, func(t *testing.T) {
			handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
			response := request(t, handler, http.MethodGet, "/api/v1/elasticsearch/nodes"+query, "", sessionCookie)
			assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
		})
	}
	t.Run("malformed encoding", func(t *testing.T) {
		handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
		response := requestWithRawQuery(t, handler, sessionCookie, "/api/v1/elasticsearch/nodes", "search=%ZZ")
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	})
}

func TestElasticsearchRoutesRejectOtherMethodsWithAllowGet(t *testing.T) {
	handler, sessionCookie := newElasticsearchAPITestHandler(t, elasticsearchHTTPFixture())
	for _, path := range []string{"/api/v1/elasticsearch/overview", "/api/v1/elasticsearch/nodes"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+path, func(t *testing.T) {
				response := request(t, handler, method, path, "", sessionCookie)
				assertError(t, response, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法")
				if got := response.Header().Get("Allow"); got != http.MethodGet {
					t.Fatalf("Allow = %q, want GET", got)
				}
			})
		}
	}
}

type elasticsearchHTTPTestProvider struct {
	snapshot elasticsearch.Snapshot
	err      error
	calls    int
}

func (provider *elasticsearchHTTPTestProvider) ElasticsearchSnapshot(context.Context) (elasticsearch.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return elasticsearch.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type elasticsearchHTTPClock struct{ now time.Time }

func (clock *elasticsearchHTTPClock) Now() time.Time              { return clock.now }
func (clock *elasticsearchHTTPClock) Advance(value time.Duration) { clock.now = clock.now.Add(value) }

func newElasticsearchAPITestHandler(t *testing.T, snapshot elasticsearch.Snapshot) (http.Handler, *http.Cookie) {
	t.Helper()
	return newElasticsearchAPIProviderTestHandler(t, &elasticsearchHTTPTestProvider{snapshot: snapshot}, testNow, time.Second)
}

func newElasticsearchAPIProviderTestHandler(t *testing.T, provider elasticsearch.Provider, clock func() time.Time, snapshotTTL time.Duration) (http.Handler, *http.Cookie) {
	t.Helper()
	store := cache.New(clock)
	elasticsearchService := service.NewElasticsearch(provider, store, service.ElasticsearchOptions{
		SnapshotTTL:        snapshotTTL,
		CollectionInterval: 15 * time.Second,
		MaxStale:           time.Minute,
		Clock:              clock,
	})
	handler := newElasticsearchAPIHandler(t, elasticsearchService, clock)
	return handler, loginCookie(t, handler)
}

func newElasticsearchAPIHandler(t *testing.T, elasticsearchService *service.ElasticsearchService, clock func() time.Time) http.Handler {
	t.Helper()
	cfg := config.Config{
		Username:   "admin",
		Password:   "correct-password",
		SessionTTL: 12 * time.Hour,
		DataSource: "mock",
	}
	return New(Dependencies{
		Config:               cfg,
		Auth:                 auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter:              auth.NewLimiter(5, time.Minute, clock),
		ElasticsearchService: elasticsearchService,
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func elasticsearchHTTPFixture() elasticsearch.Snapshot {
	reportedAt := time.Date(2026, 8, 1, 7, 59, 0, 0, time.UTC)
	zero := int64(0)
	heapUsed, heapMax := int64(40), int64(100)
	documents, storeSize, queue, uptime := int64(100), int64(200), int64(0), int64(3600)
	disk, cpu, indexRate, searchRate, rejectedRate := float64(40), float64(30), float64(10), float64(20), float64(0)
	return elasticsearch.Snapshot{
		Clusters: []elasticsearch.Cluster{{
			ID:                    "fixture-cluster-id",
			Name:                  "fixture-cluster",
			Availability:          elasticsearch.AvailabilityUp,
			NodeStatsAvailability: elasticsearch.AvailabilityUp,
			Health:                elasticsearch.HealthGreen,
			UnassignedShards:      &zero,
			ReportedAt:            reportedAt,
		}},
		Nodes: []elasticsearch.Node{{
			ID:               "fixture-node-id",
			Name:             "fixture-node",
			Cluster:          "fixture-cluster",
			Address:          "192.0.2.10",
			Roles:            []elasticsearch.Role{elasticsearch.RoleData},
			HeapUsedBytes:    &heapUsed,
			HeapMaxBytes:     &heapMax,
			DiskUsagePercent: &disk,
			CPUUsagePercent:  &cpu,
			IndexRate:        &indexRate,
			SearchRate:       &searchRate,
			Documents:        &documents,
			StoreSizeBytes:   &storeSize,
			ThreadPoolQueue:  &queue,
			RejectedRate:     &rejectedRate,
			UptimeSeconds:    &uptime,
			DataNode:         true,
			ReportedAt:       reportedAt,
		}},
	}
}

func elasticsearchPageSize500Fixture() elasticsearch.Snapshot {
	snapshot := elasticsearchHTTPFixture()
	template := snapshot.Nodes[0]
	snapshot.Nodes = make([]elasticsearch.Node, 501)
	for index := range snapshot.Nodes {
		item := template
		suffix := strconv.Itoa(index)
		item.ID = "page-size-500-elasticsearch-" + suffix
		item.Name = "page-size-500-elasticsearch-node-" + suffix
		snapshot.Nodes[index] = item
	}
	return snapshot
}

func assertElasticsearchResponseHasNoSensitiveKeys(t *testing.T, body []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	forbidden := map[string]struct{}{
		"ident": {}, "instance": {}, "url": {}, "cluster_uuid": {}, "labels": {}, "promql": {},
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, blocked := forbidden[strings.ToLower(key)]; blocked {
					t.Fatalf("response contains forbidden key %q", key)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	decoded, err := url.QueryUnescape(strings.ToLower(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(decoded, "promql") {
		t.Fatal("response contains PromQL")
	}
}
