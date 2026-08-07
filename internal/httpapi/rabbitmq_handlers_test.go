package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/rabbitmq"
	"github.com/Taier05/InfraView/internal/service"
)

func TestRabbitMQRoutesRequireAuthentication(t *testing.T) {
	handler, _ := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	for _, path := range []string{"/api/v1/rabbitmq/overview", "/api/v1/rabbitmq/nodes"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			response := request(t, handler, method, path, "", nil)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want %d", method, path, response.Code, http.StatusUnauthorized)
			}
		}
	}
}

func TestRabbitMQOverviewReturnsExplicitSafeView(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/overview", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "status", "clusters", "nodes", "alerts")
	for _, path := range []string{"data.clusters", "data.nodes"} {
		assertJSONObjectKeys(t, body, path, "total", "normal", "warning", "critical", "unknown")
	}
	assertJSONObjectKeys(t, body, "data.alerts", "cluster_connectivity", "resource_alarms", "resource_pressure", "collection")
	for _, category := range []string{"cluster_connectivity", "resource_alarms", "resource_pressure", "collection"} {
		assertJSONObjectKeys(t, body, "data.alerts."+category, "warning", "critical", "unknown")
	}
	assertRabbitMQResponseHasNoSensitiveKeys(t, body)
}

func TestRabbitMQNodesDoesNotExposePermanentClusterIdentity(t *testing.T) {
	const permanentClusterIdentity = "fixture-permanent-cluster-identity"
	provider := rabbitMQHTTPProvider{snapshot: rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{{ID: permanentClusterIdentity, Name: "fixture-cluster"}},
		Nodes: []rabbitmq.Node{{
			ID: "public-node-id", Name: "fixture-node", Cluster: "fixture-cluster", Address: "fixture-address",
		}},
	}}
	handler, cookie := newRabbitMQAPITestHandler(t, provider)
	response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/nodes", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), permanentClusterIdentity) {
		t.Fatal("response leaks permanent cluster identity")
	}
}

func TestRabbitMQNodesReturnsExplicitSafeView(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	response := request(t, handler, http.MethodGet,
		"/api/v1/rabbitmq/nodes?search=fixture&cluster=fixture-rabbitmq-cluster-a&status=normal&sort=file_descriptors&direction=desc&page=1&page_size=20",
		"", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "nodes", "available_clusters", "total", "page", "page_size", "total_pages")
	assertJSONObjectKeys(t, body, "data.nodes.0",
		"id", "name", "cluster", "address", "version", "memory_usage_percent", "disk_available_bytes",
		"file_descriptor_usage_percent", "erlang_process_usage_percent", "connections", "queues", "messages",
		"publish_rate", "deliver_rate", "uptime_seconds", "status", "status_source", "collection_level",
	)
	if !jsonPathIsArray(t, body, "data.nodes") || !jsonPathIsArray(t, body, "data.available_clusters") {
		t.Fatal("RabbitMQ list fields must be arrays")
	}
	assertRabbitMQResponseHasNoSensitiveKeys(t, body)
}

func TestRabbitMQNodesEncodesEmptyCollectionsAsArrays(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, rabbitMQHTTPProvider{})
	response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/nodes", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"data.nodes", "data.available_clusters"} {
		items, ok := jsonPathValue(t, response.Body.Bytes(), path).([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("JSON path %q = %#v, want []", path, jsonPathValue(t, response.Body.Bytes(), path))
		}
	}
	if got := jsonPathValue(t, response.Body.Bytes(), "data.total_pages"); got != float64(0) {
		t.Fatalf("total_pages = %#v, want 0", got)
	}
}

func TestRabbitMQNodesPageSize500(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, rabbitMQPageSize500Provider(t))

	response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/nodes?page=1&page_size=500", "", cookie)
	assertListPageSize500(t, response, "nodes", "nodes", "available_clusters", "total", "page", "page_size", "total_pages")
}

func TestRabbitMQNodesRejectsInvalidPageSize(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())

	assertRejectsInvalidListPageSizes(t, handler, cookie, "/api/v1/rabbitmq/nodes")
}

func TestRabbitMQOverviewRejectsEveryQueryParameter(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	for _, rawQuery := range []string{"search=fixture", "search=", "search=a&search=b", "search=%ZZ"} {
		response := requestWithRawQuery(t, handler, cookie, "/api/v1/rabbitmq/overview", rawQuery)
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	}
}

func TestRabbitMQNodesRejectsUnknownDuplicateEmptyAndInvalidParameters(t *testing.T) {
	tests := map[string]string{
		"unknown":              "?unknown=value",
		"duplicate":            "?search=a&search=b",
		"empty search":         "?search=",
		"empty cluster":        "?cluster=",
		"empty status":         "?status=",
		"empty sort":           "?sort=",
		"empty direction":      "?direction=",
		"empty page":           "?page=",
		"empty page size":      "?page_size=",
		"invalid status":       "?status=invalid",
		"invalid sort":         "?sort=invalid",
		"invalid direction":    "?direction=sideways",
		"invalid page":         "?page=invalid",
		"zero page":            "?page=0",
		"negative page":        "?page=-1",
		"invalid page size":    "?page_size=10",
		"zero page size":       "?page_size=0",
		"legacy order":         "?order=asc",
		"singular file sort":   "?sort=file_descriptor",
		"singular erlang sort": "?sort=erlang_process",
	}
	for name, query := range tests {
		t.Run(name, func(t *testing.T) {
			handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
			response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/nodes"+query, "", cookie)
			assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
		})
	}
	t.Run("malformed encoding", func(t *testing.T) {
		handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
		response := requestWithRawQuery(t, handler, cookie, "/api/v1/rabbitmq/nodes", "search=%ZZ")
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	})
}

func TestRabbitMQNodesAcceptsEverySortField(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	fields := []string{
		"node", "cluster", "address", "version", "memory", "disk", "file_descriptors", "erlang_processes",
		"connections", "queues", "messages", "publish_rate", "deliver_rate", "uptime", "status",
	}
	for _, field := range fields {
		response := request(t, handler, http.MethodGet, "/api/v1/rabbitmq/nodes?sort="+field+"&direction=asc&page=1&page_size=20", "", cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("sort %q status = %d, body = %s", field, response.Code, response.Body.String())
		}
	}
}

func TestRabbitMQRoutesRejectWriteMethodsWithAllowGet(t *testing.T) {
	handler, cookie := newRabbitMQAPITestHandler(t, mock.NewRabbitMQ())
	for _, path := range []string{"/api/v1/rabbitmq/overview", "/api/v1/rabbitmq/nodes"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			response := request(t, handler, method, path, `{}`, cookie)
			assertError(t, response, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法")
			if got := response.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("%s %s Allow = %q, want GET", method, path, got)
			}
		}
	}
}

func TestRabbitMQRoutesRejectAuthenticatedHEADWithoutLoadingSnapshot(t *testing.T) {
	for _, path := range []string{"/api/v1/rabbitmq/overview", "/api/v1/rabbitmq/nodes"} {
		t.Run(path, func(t *testing.T) {
			calls := 0
			provider := rabbitMQHTTPProvider{calls: &calls}
			handler, cookie := newRabbitMQAPITestHandler(t, provider)
			response := request(t, handler, http.MethodHead, path, "", cookie)
			assertError(t, response, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法")
			if got := response.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("HEAD %s Allow = %q, want GET", path, got)
			}
			if calls != 0 {
				t.Fatalf("HEAD %s loaded RabbitMQ snapshot %d times", path, calls)
			}
		})
	}
}

func TestRabbitMQUnavailableReturnsSafe503(t *testing.T) {
	provider := rabbitMQHTTPProvider{err: errors.Join(rabbitmq.ErrUnavailable, errors.New("fixture upstream private detail"))}
	handler, cookie := newRabbitMQAPITestHandler(t, provider)
	for _, path := range []string{"/api/v1/rabbitmq/overview", "/api/v1/rabbitmq/nodes"} {
		response := request(t, handler, http.MethodGet, path, "", cookie)
		assertError(t, response, http.StatusServiceUnavailable, "rabbitmq_unavailable", "数据源暂时不可用，请稍后重试")
		if strings.Contains(response.Body.String(), "fixture upstream private detail") {
			t.Fatal("response leaks upstream error")
		}
	}
}

type rabbitMQHTTPProvider struct {
	snapshot rabbitmq.Snapshot
	err      error
	calls    *int
}

func rabbitMQPageSize500Provider(t *testing.T) rabbitmq.Provider {
	t.Helper()
	snapshot, err := mock.NewRabbitMQ().RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatalf("load RabbitMQ test fixture: %v", err)
	}
	template := snapshot.Nodes[0]
	snapshot.Nodes = make([]rabbitmq.Node, 501)
	for index := range snapshot.Nodes {
		item := template
		suffix := strconv.Itoa(index)
		item.Name = "page-size-500-rabbitmq-node-" + suffix
		item.ID = rabbitmq.StableNodeID(item.Cluster, item.Name)
		snapshot.Nodes[index] = item
	}
	return rabbitMQHTTPProvider{snapshot: snapshot}
}

func (provider rabbitMQHTTPProvider) RabbitMQSnapshot(context.Context) (rabbitmq.Snapshot, error) {
	if provider.calls != nil {
		*provider.calls++
	}
	if provider.err != nil {
		return rabbitmq.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

func newRabbitMQAPITestHandler(t *testing.T, provider rabbitmq.Provider) (http.Handler, *http.Cookie) {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC) }
	cfg := config.Config{Username: "admin", Password: "correct-password", SessionTTL: 12 * time.Hour, DataSource: "mock"}
	handler := New(Dependencies{
		Config:  cfg,
		Auth:    auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter: auth.NewLimiter(5, time.Minute, clock),
		RabbitMQService: service.NewRabbitMQ(provider, cache.New(clock), service.RabbitMQOptions{
			SnapshotTTL: time.Second, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock,
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return handler, loginCookie(t, handler)
}

var rabbitMQForbiddenKey = regexp.MustCompile(`(?i)token|cookie|authorization|base.?url|promql|query|ident|permanent|raw|label`)

func assertRabbitMQResponseHasNoSensitiveKeys(t *testing.T, body []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if rabbitMQForbiddenKey.MatchString(key) {
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
}
