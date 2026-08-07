package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/service"
)

func TestRedisReadOnlyAPIsRequireAuthenticationAndReturnSafeViews(t *testing.T) {
	handler, cookie := newRedisAPIHandler(t)
	if response := request(t, handler, http.MethodGet, "/api/v1/redis/overview", "", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}
	for _, path := range []string{"/api/v1/redis/overview", "/api/v1/redis/instances?role=master&sort=memory&order=desc&page=1&page_size=20"} {
		response := request(t, handler, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d body = %s", path, response.Code, response.Body.String())
		}
		for _, forbidden := range []string{"replica_ip", "replica_port", "replica_id", "ident", "query-instant", "PromQL"} {
			if strings.Contains(response.Body.String(), forbidden) {
				t.Fatalf("GET %s exposes %q", path, forbidden)
			}
		}
	}
	instances := request(t, handler, http.MethodGet, "/api/v1/redis/instances", "", cookie)
	for _, required := range []string{`"address"`, `"memory_usage_percent"`, `"replication"`, `"status_source"`, `"collection_level"`, `"total_pages"`} {
		if !strings.Contains(instances.Body.String(), required) {
			t.Fatalf("instances missing %s: %s", required, instances.Body.String())
		}
	}
}

func TestRedisOverviewUsesLowercaseRoleFields(t *testing.T) {
	handler, cookie := newRedisAPIHandler(t)
	response := request(t, handler, http.MethodGet, "/api/v1/redis/overview", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}

	var payload struct {
		Data struct {
			Roles map[string]int `json:"roles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"master", "slave", "unknown"} {
		if _, ok := payload.Data.Roles[key]; !ok {
			t.Fatalf("roles missing lowercase field %q: %#v", key, payload.Data.Roles)
		}
	}
	for _, key := range []string{"Master", "Slave", "Unknown"} {
		if _, ok := payload.Data.Roles[key]; ok {
			t.Fatalf("roles exposes uppercase field %q: %#v", key, payload.Data.Roles)
		}
	}
}

func TestRedisInstancesAPIAcceptsVisibleSortColumns(t *testing.T) {
	handler, cookie := newRedisAPIHandler(t)
	for _, sortField := range []string{
		"instance", "role", "memory_limit", "memory", "connections",
		"blocked_connections", "qps", "hit_rate", "keys",
		"replication_link", "replication_lag", "uptime", "status",
	} {
		t.Run(sortField, func(t *testing.T) {
			response := request(t, handler, http.MethodGet,
				"/api/v1/redis/instances?sort="+sortField+"&order=asc&page=1&page_size=20", "", cookie)
			if response.Code != http.StatusOK {
				t.Fatalf("sort %q status = %d body = %s", sortField, response.Code, response.Body.String())
			}
		})
	}
}

func TestRedisInstancesAPIAcceptsEvictedCompatibilitySortForAuthenticatedGET(t *testing.T) {
	handler, cookie := newRedisAPIHandler(t)
	response := request(t, handler, http.MethodGet,
		"/api/v1/redis/instances?sort=evicted&order=asc&page=1&page_size=20", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated evicted sort status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestRedisAPIsRejectInvalidQueriesAndWriteMethods(t *testing.T) {
	handler, cookie := newRedisAPIHandler(t)
	for _, path := range []string{
		"/api/v1/redis/overview?unexpected=1",
		"/api/v1/redis/instances?role=",
		"/api/v1/redis/instances?sort=invalid",
	} {
		if response := request(t, handler, http.MethodGet, path, "", cookie); response.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d", path, response.Code)
		}
	}
	response := request(
		t,
		handler,
		http.MethodPost,
		"/api/v1/redis/instances",
		`{}`,
		cookie,
	)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET" {
		t.Fatalf("POST status = %d Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func newRedisAPIHandler(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	cfg := config.Config{
		Username:          "admin",
		Password:          "correct-password",
		SessionTTL:        12 * time.Hour,
		DataSource:        "mock",
		CurrentMetricsTTL: 15 * time.Second,
		MaxStale:          time.Minute,
	}
	store := cache.New(testNow)
	handler := New(Dependencies{
		Config:  cfg,
		Auth:    auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, testNow),
		Limiter: auth.NewLimiter(5, time.Minute, testNow),
		RedisService: service.NewRedis(
			mock.NewRedis(testNow),
			store,
			service.RedisOptions{
				SnapshotTTL:        15 * time.Second,
				CollectionInterval: 15 * time.Second,
				MaxStale:           time.Minute,
				Clock:              testNow,
			},
		),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return handler, loginCookie(t, handler)
}
