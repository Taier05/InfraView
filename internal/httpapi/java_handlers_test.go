package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
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
	"github.com/Taier05/InfraView/internal/javaapp"
	"github.com/Taier05/InfraView/internal/service"
)

func TestJavaRoutesRequireAuthentication(t *testing.T) {
	handler, _ := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	for _, path := range []string{"/api/v1/java/overview", "/api/v1/java/services"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			response := request(t, handler, method, path, "", nil)
			assertError(t, response, http.StatusUnauthorized, "unauthorized", "请先登录")
		}
	}
}

func TestJavaOverviewReturnsExplicitSafeView(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	response := request(t, handler, http.MethodGet, "/api/v1/java/overview", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "status", "services", "alerts")
	assertJSONObjectKeys(t, body, "data.services", "total", "normal", "warning", "critical", "unknown")
	assertJSONObjectKeys(t, body, "data.alerts", "health", "port", "process", "collection")
	for _, category := range []string{"health", "port", "process", "collection"} {
		assertJSONObjectKeys(t, body, "data.alerts."+category, "warning", "critical", "unknown")
	}
	assertJavaResponseHasNoSensitiveKeys(t, body)
}

func TestJavaServicesReturnsExplicitSafeViewAndArrays(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	response := request(t, handler, http.MethodGet,
		"/api/v1/java/services?search=fixture&name=fixture-java-normal&status=normal&sort=business&direction=asc&page=1&page_size=20",
		"", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	assertJSONObjectKeys(t, body, "", "data", "meta")
	assertJSONObjectKeys(t, body, "data", "services", "available_names", "total", "page", "page_size", "total_pages")
	assertJSONObjectKeys(t, body, "data.services.0",
		"id", "name", "business", "address", "health_up", "health_latency_ms", "port_up", "process_up",
		"process_count", "port_consistent", "cpu_usage_percent", "memory_bytes", "memory_usage_percent",
		"uptime_seconds", "status", "status_source", "collection_level",
	)
	if !jsonPathIsArray(t, body, "data.services") || !jsonPathIsArray(t, body, "data.available_names") {
		t.Fatal("Java list fields must be arrays")
	}
	assertJavaResponseHasNoSensitiveKeys(t, body)
}

func TestJavaServiceViewEncodesExactInt64FieldsAsCanonicalDecimalStrings(t *testing.T) {
	beyondSafeInteger := int64(9_007_199_254_740_993)
	maximum := int64(math.MaxInt64)
	body, err := json.Marshal(javaServiceViewFrom(service.JavaServiceSummary{
		ProcessCount:  &beyondSafeInteger,
		MemoryBytes:   &maximum,
		UptimeSeconds: &maximum,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"process_count":  "9007199254740993",
		"memory_bytes":   "9223372036854775807",
		"uptime_seconds": "9223372036854775807",
	} {
		if got := jsonPathValue(t, body, path); got != want {
			t.Fatalf("%s = %#v, want canonical decimal string %q", path, got, want)
		}
	}

	nullBody, err := json.Marshal(javaServiceViewFrom(service.JavaServiceSummary{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"process_count", "memory_bytes", "uptime_seconds"} {
		if got := jsonPathValue(t, nullBody, path); got != nil {
			t.Fatalf("%s = %#v, want null", path, got)
		}
	}

	negative := int64(-1)
	negativeBody, err := json.Marshal(javaServiceViewFrom(service.JavaServiceSummary{
		ProcessCount:  &negative,
		MemoryBytes:   &negative,
		UptimeSeconds: &negative,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"process_count", "memory_bytes", "uptime_seconds"} {
		if got := jsonPathValue(t, negativeBody, path); got != nil {
			t.Fatalf("%s = %#v, want null for negative domain value", path, got)
		}
	}
}

func TestJavaServicesEncodesEmptyCollectionsAsArrays(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, &javaHTTPProvider{})
	response := request(t, handler, http.MethodGet, "/api/v1/java/services", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"data.services", "data.available_names"} {
		items, ok := jsonPathValue(t, response.Body.Bytes(), path).([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("JSON path %q = %#v, want []", path, jsonPathValue(t, response.Body.Bytes(), path))
		}
	}
	if got := jsonPathValue(t, response.Body.Bytes(), "data.total_pages"); got != float64(0) {
		t.Fatalf("total_pages = %#v, want 0", got)
	}
}

func TestJavaOverviewRejectsEveryQueryParameter(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	for _, rawQuery := range []string{"search=fixture", "search=", "search=a&search=b", "search=%ZZ"} {
		response := requestWithRawQuery(t, handler, cookie, "/api/v1/java/overview", rawQuery)
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	}
}

func TestJavaServicesRejectsUnknownDuplicateEmptyAndInvalidParameters(t *testing.T) {
	tests := map[string]string{
		"unknown":           "?unknown=value",
		"duplicate":         "?search=a&search=b",
		"empty search":      "?search=",
		"empty name":        "?name=",
		"empty status":      "?status=",
		"empty sort":        "?sort=",
		"empty direction":   "?direction=",
		"empty page":        "?page=",
		"empty page size":   "?page_size=",
		"invalid status":    "?status=invalid",
		"invalid sort":      "?sort=invalid",
		"invalid direction": "?direction=sideways",
		"invalid page":      "?page=invalid",
		"zero page":         "?page=0",
		"negative page":     "?page=-1",
		"invalid page size": "?page_size=10",
		"zero page size":    "?page_size=0",
		"legacy order":      "?order=asc",
	}
	for name, query := range tests {
		t.Run(name, func(t *testing.T) {
			provider := &javaHTTPProvider{}
			handler, cookie := newJavaAPITestHandler(t, provider)
			response := request(t, handler, http.MethodGet, "/api/v1/java/services"+query, "", cookie)
			assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
			if provider.calls != 0 {
				t.Fatalf("invalid query loaded Java snapshot %d times", provider.calls)
			}
		})
	}
	t.Run("malformed encoding", func(t *testing.T) {
		handler, cookie := newJavaAPITestHandler(t, &javaHTTPProvider{})
		response := requestWithRawQuery(t, handler, cookie, "/api/v1/java/services", "search=%ZZ")
		assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	})
}

func TestJavaServicesRejectsWhitespaceSearchAndName(t *testing.T) {
	for name, query := range map[string]string{
		"search": "?search=%20",
		"name":   "?name=%20",
	} {
		t.Run(name, func(t *testing.T) {
			provider := &javaHTTPProvider{}
			handler, cookie := newJavaAPITestHandler(t, provider)
			response := request(t, handler, http.MethodGet, "/api/v1/java/services"+query, "", cookie)
			assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
			if provider.calls != 0 {
				t.Fatalf("whitespace query loaded Java snapshot %d times", provider.calls)
			}
		})
	}
}

func TestJavaServicesAcceptsEverySortField(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	fields := []string{
		"business", "address", "health", "health_latency", "port", "process", "process_count",
		"consistency", "cpu", "memory", "memory_percent", "uptime", "status",
	}
	for _, field := range fields {
		response := request(t, handler, http.MethodGet, "/api/v1/java/services?sort="+field+"&direction=desc&page=1&page_size=20", "", cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("sort %q status = %d, body = %s", field, response.Code, response.Body.String())
		}
	}
}

func TestJavaRoutesRejectWriteMethodsWithAllowGet(t *testing.T) {
	handler, cookie := newJavaAPITestHandler(t, mock.NewJava(time.Now))
	for _, path := range []string{"/api/v1/java/overview", "/api/v1/java/services"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			response := request(t, handler, method, path, `{}`, cookie)
			assertError(t, response, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法")
			if got := response.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("%s %s Allow = %q, want GET", method, path, got)
			}
		}
	}
}

func TestJavaRoutesRejectAuthenticatedHEADWithoutLoadingSnapshot(t *testing.T) {
	for _, path := range []string{"/api/v1/java/overview", "/api/v1/java/services"} {
		t.Run(path, func(t *testing.T) {
			provider := &javaHTTPProvider{}
			handler, cookie := newJavaAPITestHandler(t, provider)
			response := request(t, handler, http.MethodHead, path, "", cookie)
			assertError(t, response, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法")
			if got := response.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("HEAD %s Allow = %q, want GET", path, got)
			}
			if provider.calls != 0 {
				t.Fatalf("HEAD %s loaded Java snapshot %d times", path, provider.calls)
			}
		})
	}
}

func TestJavaServiceInvalidQueryMapsToBadRequest(t *testing.T) {
	provider := &javaHTTPProvider{}
	handler, cookie := newJavaAPITestHandler(t, provider)
	maxInt := strconv.Itoa(int(^uint(0) >> 1))
	response := request(t, handler, http.MethodGet, "/api/v1/java/services?page="+maxInt+"&page_size=100", "", cookie)
	assertError(t, response, http.StatusBadRequest, "invalid_query", "查询参数无效")
	if provider.calls != 0 {
		t.Fatalf("service-invalid query loaded Java snapshot %d times", provider.calls)
	}
}

func TestJavaUnavailableReturnsSafeRetryable503(t *testing.T) {
	provider := &javaHTTPProvider{err: errors.Join(javaapp.ErrUnavailable, errors.New("fixture upstream private body"))}
	handler, cookie := newJavaAPITestHandler(t, provider)
	for _, path := range []string{"/api/v1/java/overview", "/api/v1/java/services"} {
		response := request(t, handler, http.MethodGet, path, "", cookie)
		assertError(t, response, http.StatusServiceUnavailable, "java_unavailable", "数据源暂时不可用，请稍后重试")
		var body ErrorBody
		decodeJSON(t, response, &body)
		if !body.Retryable {
			t.Fatal("Java unavailable response must be retryable")
		}
		if strings.Contains(response.Body.String(), "fixture upstream private body") {
			t.Fatal("response leaks upstream error")
		}
	}
}

func TestJavaMissingServiceReturnsSafeRetryable503(t *testing.T) {
	handler := newJavaAPIHandler(t, nil, time.Now)
	cookie := loginCookie(t, handler)
	for _, path := range []string{"/api/v1/java/overview", "/api/v1/java/services"} {
		response := request(t, handler, http.MethodGet, path, "", cookie)
		assertError(t, response, http.StatusServiceUnavailable, "java_unavailable", "数据源暂时不可用，请稍后重试")
		var body ErrorBody
		decodeJSON(t, response, &body)
		if !body.Retryable {
			t.Fatal("missing Java service response must be retryable")
		}
	}
}

func TestJavaServicesPreservesStaleMetadata(t *testing.T) {
	clock := &javaHTTPClock{now: time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)}
	provider := &javaHTTPProvider{snapshot: javaapp.Snapshot{Services: []javaapp.Service{{
		ID: javaapp.StableServiceID("fixture-java", "fixture-address"), Name: "fixture-java", Address: "fixture-address",
		CollectionTracked: true, ReportedAt: clock.Now(),
	}}}}
	handler, cookie := newJavaAPIProviderTestHandler(t, provider, clock.Now, time.Second)
	first := request(t, handler, http.MethodGet, "/api/v1/java/services", "", cookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	firstCollectedAt := jsonPathValue(t, first.Body.Bytes(), "meta.collected_at")
	clock.Advance(2 * time.Second)
	provider.err = javaapp.ErrUnavailable
	stale := request(t, handler, http.MethodGet, "/api/v1/java/services", "", cookie)
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

type javaHTTPProvider struct {
	snapshot javaapp.Snapshot
	err      error
	calls    int
}

func (provider *javaHTTPProvider) JavaSnapshot(context.Context) (javaapp.Snapshot, error) {
	provider.calls++
	if provider.err != nil {
		return javaapp.Snapshot{}, provider.err
	}
	return provider.snapshot.Clone(), nil
}

type javaHTTPClock struct{ now time.Time }

func (clock *javaHTTPClock) Now() time.Time { return clock.now }

func (clock *javaHTTPClock) Advance(delta time.Duration) { clock.now = clock.now.Add(delta) }

func newJavaAPITestHandler(t *testing.T, provider javaapp.Provider) (http.Handler, *http.Cookie) {
	t.Helper()
	return newJavaAPIProviderTestHandler(t, provider, time.Now, time.Second)
}

func newJavaAPIProviderTestHandler(t *testing.T, provider javaapp.Provider, clock func() time.Time, ttl time.Duration) (http.Handler, *http.Cookie) {
	t.Helper()
	javaService := service.NewJava(provider, cache.New(clock), service.JavaOptions{
		SnapshotTTL: ttl, CollectionInterval: 15 * time.Second, MaxStale: time.Minute, Clock: clock,
	})
	handler := newJavaAPIHandler(t, javaService, clock)
	return handler, loginCookie(t, handler)
}

func newJavaAPIHandler(t *testing.T, javaService *service.JavaService, clock func() time.Time) http.Handler {
	t.Helper()
	cfg := config.Config{Username: "admin", Password: "correct-password", SessionTTL: 12 * time.Hour, DataSource: "mock"}
	return New(Dependencies{
		Config: cfg, Auth: auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter: auth.NewLimiter(5, time.Minute, clock), JavaService: javaService,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

var javaForbiddenKey = regexp.MustCompile(`(?i)ident|labels?|promql|datasource|token|cookie|authorization|auth|base.?url|upstream|raw`)

func assertJavaResponseHasNoSensitiveKeys(t *testing.T, body []byte) {
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
				if javaForbiddenKey.MatchString(key) {
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
