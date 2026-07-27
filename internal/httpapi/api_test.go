package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/service"
)

func TestHealthzRemainsPublic(t *testing.T) {
	response := request(t, newTestAPI(t, mock.New(3, testNow)), http.MethodGet, "/healthz", "", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("body = %q", response.Body.String())
	}
	assertSecurityHeaders(t, response)
}

func TestProtectedRoutesRequireAuthentication(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	paths := []string{
		"/api/v1/session",
		"/api/v1/overview?range=24h",
		"/api/v1/hosts?q=linux&status=online&sort=cpu&order=desc&page=1&page_size=20",
		"/api/v1/hosts/mock-host-001",
		"/api/v1/hosts/mock-host-001/metrics?range=6h",
		"/api/v1/datasource/status",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, path, "", nil)
			assertError(t, response, http.StatusUnauthorized, "unauthorized", "请先登录")
			assertSecurityHeaders(t, response)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestLoginSessionAndLogout(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	login := request(t, handler, http.MethodPost, "/api/v1/session", `{"username":"admin","password":"correct-password"}`, nil)
	if login.Code != http.StatusNoContent || login.Body.Len() != 0 {
		t.Fatalf("login response = %d %q", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Value == "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1" {
		t.Fatalf("session cookie = %#v", cookie)
	}

	session := request(t, handler, http.MethodGet, "/api/v1/session", "", cookie)
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", session.Code, session.Body.String())
	}
	var body struct {
		Data struct {
			Authenticated bool   `json:"authenticated"`
			Username      string `json:"username"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	decodeJSON(t, session, &body)
	if !body.Data.Authenticated || body.Data.Username != "admin" || body.Meta.RequestID == "" || body.Meta.Stale || !body.Meta.CollectedAt.IsZero() {
		t.Fatalf("session body = %#v", body)
	}

	logoutRequest := httptest.NewRequest(http.MethodDelete, "http://example.com/api/v1/session", nil)
	logoutRequest.AddCookie(cookie)
	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusNoContent || logout.Body.Len() != 0 {
		t.Fatalf("logout response = %d %q", logout.Code, logout.Body.String())
	}
	cleared := logout.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 || cleared[0].Value != "" {
		t.Fatalf("cleared cookie = %#v", cleared)
	}
	assertError(t, request(t, handler, http.MethodGet, "/api/v1/session", "", cookie), http.StatusUnauthorized, "unauthorized", "请先登录")
}

func TestLoginRejectsCrossOriginAndMalformedBodies(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))

	crossOriginRequest := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	crossOriginRequest.Header.Set("Origin", "https://evil.example")
	crossOrigin := httptest.NewRecorder()
	handler.ServeHTTP(crossOrigin, crossOriginRequest)
	assertError(t, crossOrigin, http.StatusForbidden, "cross_origin_forbidden", "不允许跨域请求")

	unknown := request(t, handler, http.MethodPost, "/api/v1/session", `{"username":"admin","password":"correct-password","token":"secret"}`, nil)
	assertError(t, unknown, http.StatusBadRequest, "invalid_request", "登录请求格式无效")
	if strings.Contains(unknown.Body.String(), "token") || strings.Contains(unknown.Body.String(), "secret") {
		t.Fatalf("response leaks submitted field: %s", unknown.Body.String())
	}

	oversized := request(t, handler, http.MethodPost, "/api/v1/session", strings.Repeat("x", 4097), nil)
	assertError(t, oversized, http.StatusBadRequest, "invalid_request", "登录请求格式无效")
}

func TestLoginRateLimitBlocksSixthFailure(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	for attempt := 1; attempt <= 5; attempt++ {
		response := request(t, handler, http.MethodPost, "/api/v1/session", `{"username":"admin","password":"wrong-password"}`, nil)
		assertError(t, response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
	}
	sixth := request(t, handler, http.MethodPost, "/api/v1/session", `{"username":"admin","password":"wrong-password"}`, nil)
	assertError(t, sixth, http.StatusTooManyRequests, "rate_limited", "登录尝试过于频繁，请稍后重试")
}

func TestLoginRateLimitIgnoresForgedRealIPFromDirectClients(t *testing.T) {
	handler := newTestAPIWithConfig(t, mock.New(3, testNow), config.Config{
		TrustedProxyCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	})
	for attempt := 1; attempt <= 5; attempt++ {
		response := loginRequestFrom(t, handler, "192.0.2.10:12345", "198.51.100."+string(rune('0'+attempt)))
		assertError(t, response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
	}
	sixth := loginRequestFrom(t, handler, "192.0.2.10:54321", "203.0.113.200")
	assertError(t, sixth, http.StatusTooManyRequests, "rate_limited", "登录尝试过于频繁，请稍后重试")
}

func TestLoginRateLimitSeparatesRealClientsBehindTrustedProxy(t *testing.T) {
	handler := newTestAPIWithConfig(t, mock.New(3, testNow), config.Config{
		TrustedProxyCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	})
	for range 5 {
		response := loginRequestFrom(t, handler, "10.0.0.5:12345", "198.51.100.10")
		assertError(t, response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
	}
	otherClient := loginRequestFrom(t, handler, "10.0.0.5:12345", "198.51.100.11")
	assertError(t, otherClient, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
	blockedClient := loginRequestFrom(t, handler, "10.0.0.5:12345", "198.51.100.10")
	assertError(t, blockedClient, http.StatusTooManyRequests, "rate_limited", "登录尝试过于频繁，请稍后重试")
}

func TestClientIPAcceptsOnlySingleValidRealIPFromTrustedProxy(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name       string
		remoteAddr string
		headers    []string
		want       string
	}{
		{name: "trusted IPv4", remoteAddr: "10.0.0.5:12345", headers: []string{"198.51.100.10"}, want: "198.51.100.10"},
		{name: "trusted mapped IPv4", remoteAddr: "10.0.0.5:12345", headers: []string{"::ffff:198.51.100.10"}, want: "198.51.100.10"},
		{name: "direct forged header", remoteAddr: "192.0.2.10:12345", headers: []string{"198.51.100.10"}, want: "192.0.2.10"},
		{name: "chained value", remoteAddr: "10.0.0.5:12345", headers: []string{"198.51.100.10, 10.0.0.4"}, want: "10.0.0.5"},
		{name: "multiple header fields", remoteAddr: "10.0.0.5:12345", headers: []string{"198.51.100.10", "198.51.100.11"}, want: "10.0.0.5"},
		{name: "invalid IP", remoteAddr: "10.0.0.5:12345", headers: []string{"not-an-ip"}, want: "10.0.0.5"},
		{name: "zoned IP", remoteAddr: "10.0.0.5:12345", headers: []string{"fe80::1%eth0"}, want: "10.0.0.5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", nil)
			request.RemoteAddr = test.remoteAddr
			for _, value := range test.headers {
				request.Header.Add("X-Real-IP", value)
			}
			if got := clientIP(request, trusted); got != test.want {
				t.Fatalf("client IP = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseRemoteIPRemovesIPv6ZoneForStableMatching(t *testing.T) {
	address, ok := parseRemoteIP("[fe80::1%eth0]:12345")
	if !ok || address.String() != "fe80::1" {
		t.Fatalf("parsed remote IP = %q, ok = %v; want unzoned fe80::1", address, ok)
	}
}

func TestSameOriginTrustsForwardedProtoOnlyFromConfiguredProxy(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name       string
		remoteAddr string
		tls        bool
		forwarded  []string
		want       bool
	}{
		{name: "direct client cannot forge https", remoteAddr: "192.0.2.10:12345", forwarded: []string{"https"}, want: false},
		{name: "trusted proxy supplies https", remoteAddr: "10.0.0.5:12345", forwarded: []string{"https"}, want: true},
		{name: "trusted proxy chained value rejected", remoteAddr: "10.0.0.5:12345", forwarded: []string{"https, http"}, want: false},
		{name: "trusted proxy multiple fields rejected", remoteAddr: "10.0.0.5:12345", forwarded: []string{"https", "http"}, want: false},
		{name: "direct tls remains valid", remoteAddr: "192.0.2.10:12345", tls: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", nil)
			request.RemoteAddr = test.remoteAddr
			request.Header.Set("Origin", "https://example.com")
			if test.tls {
				request.TLS = &tls.ConnectionState{}
			}
			for _, value := range test.forwarded {
				request.Header.Add("X-Forwarded-Proto", value)
			}
			if got := isSameOrigin(request, trusted); got != test.want {
				t.Fatalf("same origin = %v, want %v", got, test.want)
			}
		})
	}
}

func TestConcurrentInvalidLoginsCannotBypassFailureLimit(t *testing.T) {
	limiter := auth.NewLimiter(5, time.Minute, testNow)
	for range 4 {
		limiter.RecordFailure("192.0.2.10")
	}
	var verifications atomic.Int64
	server := &api{
		config:  config.Config{DataSource: "mock"},
		limiter: limiter,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		verifyLogin: func(string, string) (auth.Session, bool) {
			verifications.Add(1)
			return auth.Session{}, false
		},
	}
	handler := server.middleware(server.sameOrigin(http.HandlerFunc(server.login)))

	const concurrentAttempts = 64
	start := make(chan struct{})
	statuses := make(chan int, concurrentAttempts)
	var wait sync.WaitGroup
	wait.Add(concurrentAttempts)
	for range concurrentAttempts {
		go func() {
			defer wait.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", strings.NewReader(`{"username":"admin","password":"wrong-password"}`))
			req.RemoteAddr = "192.0.2.10:12345"
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			statuses <- recorder.Code
		}()
	}
	close(start)
	wait.Wait()
	close(statuses)

	unauthorized := 0
	rateLimited := 0
	for status := range statuses {
		switch status {
		case http.StatusUnauthorized:
			unauthorized++
		case http.StatusTooManyRequests:
			rateLimited++
		default:
			t.Fatalf("unexpected concurrent login status = %d", status)
		}
	}
	if unauthorized != 1 || rateLimited != concurrentAttempts-1 {
		t.Fatalf("concurrent statuses: 401=%d, 429=%d; want 1 and %d", unauthorized, rateLimited, concurrentAttempts-1)
	}
	if got := verifications.Load(); got != 1 {
		t.Fatalf("credential verifications = %d, want 1", got)
	}
}

func TestAuthenticatedReadOnlyQueriesUseApprovedSnakeCaseViews(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)

	tests := []struct {
		name string
		path string
	}{
		{name: "overview", path: "/api/v1/overview?range=24h"},
		{name: "hosts", path: "/api/v1/hosts?q=linux&status=online&sort=cpu&order=desc&page=1&page_size=20"},
		{name: "host", path: "/api/v1/hosts/mock-host-001"},
		{name: "metrics", path: "/api/v1/hosts/mock-host-001/metrics?range=6h"},
		{name: "datasource", path: "/api/v1/datasource/status"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, test.path, "", cookie)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var envelope map[string]json.RawMessage
			decodeJSON(t, response, &envelope)
			if len(envelope) != 2 || envelope["data"] == nil || envelope["meta"] == nil {
				t.Fatalf("envelope = %s", response.Body.String())
			}
			if strings.Contains(response.Body.String(), `"HostID"`) || strings.Contains(response.Body.String(), `"CPUUsage"`) || strings.Contains(response.Body.String(), `"Uptime"`) {
				t.Fatalf("response contains Go field names: %s", response.Body.String())
			}
			assertSecurityHeaders(t, response)
		})
	}

	hosts := request(t, handler, http.MethodGet, tests[1].path, "", cookie)
	var hostPage struct {
		Data struct {
			Hosts []struct {
				ID               string `json:"id"`
				CPUCores         *int   `json:"cpu_cores"`
				MemoryTotalBytes *int64 `json:"memory_total_bytes"`
				UptimeSeconds    int64  `json:"uptime_seconds"`
			} `json:"hosts"`
			Page       int `json:"page"`
			PageSize   int `json:"page_size"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"data"`
	}
	decodeJSON(t, hosts, &hostPage)
	if len(hostPage.Data.Hosts) != 3 || hostPage.Data.Page != 1 || hostPage.Data.PageSize != 20 || hostPage.Data.Total != 3 || hostPage.Data.TotalPages != 1 || hostPage.Data.Hosts[0].UptimeSeconds <= 0 {
		t.Fatalf("host page = %#v", hostPage.Data)
	}
	if hostPage.Data.Hosts[0].CPUCores == nil || *hostPage.Data.Hosts[0].CPUCores != 4 || hostPage.Data.Hosts[0].MemoryTotalBytes == nil || *hostPage.Data.Hosts[0].MemoryTotalBytes != 16*1024*1024*1024 {
		t.Fatalf("host hardware view = %#v", hostPage.Data.Hosts[0])
	}
}

func TestOverviewReturnsNormalizedAggregateTrends(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)
	response := request(t, handler, http.MethodGet, "/api/v1/overview?range=6h", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	var body struct {
		Data struct {
			Alerts struct {
				AffectedHosts int `json:"affected_hosts"`
				WarningHosts  int `json:"warning_hosts"`
				CriticalHosts int `json:"critical_hosts"`
				CPU           struct {
					Warning  int `json:"warning"`
					Critical int `json:"critical"`
				} `json:"cpu"`
			} `json:"alerts"`
			Trends []struct {
				Key    datasource.MetricKey `json:"key"`
				Unit   string               `json:"unit"`
				Points []struct {
					Timestamp string   `json:"timestamp"`
					Value     *float64 `json:"value"`
				} `json:"points"`
			} `json:"trends"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	decodeJSON(t, response, &body)
	if len(body.Data.Trends) != 2 || body.Meta.RequestID == "" || body.Meta.Stale || body.Meta.CollectedAt.IsZero() {
		t.Fatalf("overview body = %#v", body)
	}
	if body.Data.Alerts.AffectedHosts != body.Data.Alerts.WarningHosts+body.Data.Alerts.CriticalHosts {
		t.Fatalf("overview alerts = %#v", body.Data.Alerts)
	}
	for i, key := range []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage} {
		trend := body.Data.Trends[i]
		if trend.Key != key || trend.Unit != "%" || len(trend.Points) == 0 || len(trend.Points) > 600 {
			t.Fatalf("trend %d = %#v", i, trend)
		}
		for pointIndex, point := range trend.Points {
			if _, err := time.Parse(time.RFC3339Nano, point.Timestamp); err != nil || point.Value == nil {
				t.Fatalf("trend %s point %d = %#v, parse error = %v", key, pointIndex, point, err)
			}
		}
	}
	if strings.Contains(response.Body.String(), `"metric"`) {
		t.Fatalf("overview trend must use key, not metric: %s", response.Body.String())
	}
}

func TestOverviewReturnsStaleAggregateTrendsWithCollectionTime(t *testing.T) {
	now := testNow()
	provider := &aggregateFailureProvider{Provider: mock.New(3, func() time.Time { return now })}
	handler := newTestAPIAt(t, provider, func() time.Time { return now })
	cookie := loginCookie(t, handler)

	first := request(t, handler, http.MethodGet, "/api/v1/overview?range=24h", "", cookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	var firstBody struct {
		Meta ResponseMeta `json:"meta"`
	}
	decodeJSON(t, first, &firstBody)

	now = now.Add(2 * time.Minute)
	provider.fail = true
	stale := request(t, handler, http.MethodGet, "/api/v1/overview?range=24h", "", cookie)
	if stale.Code != http.StatusOK {
		t.Fatalf("stale status = %d, body = %s", stale.Code, stale.Body.String())
	}
	var staleBody struct {
		Data struct {
			Trends []json.RawMessage `json:"trends"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	decodeJSON(t, stale, &staleBody)
	if !staleBody.Meta.Stale || !staleBody.Meta.CollectedAt.Equal(firstBody.Meta.CollectedAt) || len(staleBody.Data.Trends) != 2 {
		t.Fatalf("stale body = %#v, first meta = %#v", staleBody, firstBody.Meta)
	}
}

func TestQueryValidationAndMissingHost(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)
	tests := []struct {
		path    string
		status  int
		code    string
		message string
	}{
		{path: "/api/v1/overview?range=2h", status: http.StatusBadRequest, code: "invalid_range", message: "时间范围无效"},
		{path: "/api/v1/hosts?page=0&page_size=20", status: http.StatusBadRequest, code: "invalid_query", message: "查询参数无效"},
		{path: "/api/v1/hosts?sort=password&page=1&page_size=20", status: http.StatusBadRequest, code: "invalid_query", message: "查询参数无效"},
		{path: "/api/v1/hosts/mock-host-001/metrics?range=2h", status: http.StatusBadRequest, code: "invalid_range", message: "时间范围无效"},
		{path: "/api/v1/hosts/unknown-host", status: http.StatusNotFound, code: "host_not_found", message: "该主机当前不在数据源中"},
	}
	for _, test := range tests {
		response := request(t, handler, http.MethodGet, test.path, "", cookie)
		assertError(t, response, test.status, test.code, test.message)
	}
}

func TestHostListAcceptsLoadSortAliases(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)
	for _, sortField := range []string{"load", "load_1"} {
		response := request(t, handler, http.MethodGet, "/api/v1/hosts?sort="+sortField+"&order=desc&page=1&page_size=20", "", cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("sort=%s status = %d, body = %s", sortField, response.Code, response.Body.String())
		}
	}
}

func TestUnavailableSourceReturnsSanitized503WithoutStaleData(t *testing.T) {
	handler := newTestAPI(t, unavailableProvider{})
	cookie := loginCookie(t, handler)
	response := request(t, handler, http.MethodGet, "/api/v1/overview?range=24h", "", cookie)
	assertError(t, response, http.StatusServiceUnavailable, "datasource_unavailable", "数据源暂时不可用，请稍后重试")
	if strings.Contains(response.Body.String(), "upstream-secret") {
		t.Fatalf("response leaks upstream error: %s", response.Body.String())
	}
}

func TestBusinessWriteRoutesAreNotRegistered(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)
	for _, path := range []string{"/api/v1/overview", "/api/v1/hosts", "/api/v1/hosts/mock-host-001", "/api/v1/proxy", "/api/v1/query"} {
		response := request(t, handler, http.MethodPost, path, `{}`, cookie)
		if response.Code != http.StatusMethodNotAllowed && response.Code != http.StatusNotFound {
			t.Fatalf("POST %s status = %d", path, response.Code)
		}
		if strings.Contains(response.Body.String(), "Method Not Allowed") || strings.Contains(response.Body.String(), "404 page not found") {
			t.Fatalf("POST %s has non-Chinese error: %q", path, response.Body.String())
		}
	}
}

func TestMiddlewareLogsRecoveredPanicAs500(t *testing.T) {
	var logs bytes.Buffer
	server := &api{
		config: config.Config{DataSource: "mock"},
		logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	}
	handler := server.middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive-panic-value")
	}))

	response := request(t, handler, http.MethodGet, "/api/v1/overview", "", nil)
	assertError(t, response, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求")
	if strings.Contains(response.Body.String(), "sensitive-panic-value") || strings.Contains(logs.String(), "sensitive-panic-value") {
		t.Fatalf("panic value leaked: response=%s logs=%s", response.Body.String(), logs.String())
	}

	requestLogFound := false
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log %q: %v", line, err)
		}
		if entry["msg"] == "HTTP 请求完成" {
			requestLogFound = true
			if entry["status"] != float64(http.StatusInternalServerError) {
				t.Fatalf("logged status = %#v, want 500; logs=%s", entry["status"], logs.String())
			}
		}
	}
	if !requestLogFound {
		t.Fatalf("request completion log not found: %s", logs.String())
	}
}

func newTestAPI(t *testing.T, provider datasource.Provider) http.Handler {
	return newTestAPIAt(t, provider, testNow)
}

func newTestAPIAt(t *testing.T, provider datasource.Provider, clock func() time.Time) http.Handler {
	t.Helper()
	cfg := config.Config{
		Username:          "admin",
		Password:          "correct-password",
		SessionTTL:        12 * time.Hour,
		DataSource:        "mock",
		InventoryTTL:      time.Minute,
		CurrentMetricsTTL: 20 * time.Second,
		RangeTTL:          time.Minute,
		HealthTTL:         15 * time.Second,
		MaxStale:          5 * time.Minute,
		WarningPercent:    80,
		CriticalPercent:   90,
	}
	return newTestAPIWithConfigAt(t, provider, clock, cfg)
}

func newTestAPIWithConfig(t *testing.T, provider datasource.Provider, overrides config.Config) http.Handler {
	return newTestAPIWithConfigAt(t, provider, testNow, overrides)
}

func newTestAPIWithConfigAt(t *testing.T, provider datasource.Provider, clock func() time.Time, overrides config.Config) http.Handler {
	t.Helper()
	cfg := config.Config{
		Username:          "admin",
		Password:          "correct-password",
		SessionTTL:        12 * time.Hour,
		DataSource:        "mock",
		InventoryTTL:      time.Minute,
		CurrentMetricsTTL: 20 * time.Second,
		RangeTTL:          time.Minute,
		HealthTTL:         15 * time.Second,
		MaxStale:          5 * time.Minute,
		WarningPercent:    80,
		CriticalPercent:   90,
		TrustedProxyCIDRs: overrides.TrustedProxyCIDRs,
	}
	return New(Dependencies{
		Config:  cfg,
		Auth:    auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter: auth.NewLimiter(5, time.Minute, clock),
		Service: service.New(provider, cache.New(clock), service.Options{
			InventoryTTL:      cfg.InventoryTTL,
			CurrentMetricsTTL: cfg.CurrentMetricsTTL,
			RangeTTL:          cfg.RangeTTL,
			HealthTTL:         cfg.HealthTTL,
			MaxStale:          cfg.MaxStale,
			WarningPercent:    cfg.WarningPercent,
			CriticalPercent:   cfg.CriticalPercent,
			Clock:             clock,
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	response := request(t, handler, http.MethodPost, "/api/v1/session", `{"username":"admin","password":"correct-password"}`, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	return response.Result().Cookies()[0]
}

func request(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:12345"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, status, response.Body.String())
	}
	var body ErrorBody
	decodeJSON(t, response, &body)
	if body.Code != code || body.Message != message || body.RequestID == "" {
		t.Fatalf("error body = %#v", body)
	}
	var fields map[string]json.RawMessage
	decodeJSON(t, response, &fields)
	if stale, ok := fields["stale"]; !ok || string(stale) != "false" {
		t.Fatalf("error stale field = %s, present = %v; body = %s", stale, ok, response.Body.String())
	}
}

func loginRequestFrom(t *testing.T, handler http.Handler, remoteAddr, realIP string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", strings.NewReader(`{"username":"admin","password":"wrong-password"}`))
	request.RemoteAddr = remoteAddr
	request.Header.Set("X-Real-IP", realIP)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertSecurityHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
	}
	for name, value := range want {
		if got := response.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
}

func testNow() time.Time {
	return time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
}

type unavailableProvider struct{}

type aggregateFailureProvider struct {
	datasource.Provider
	fail bool
}

func (p *aggregateFailureProvider) QueryAggregateRange(ctx context.Context, request datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	if p.fail {
		return nil, datasource.ErrUnavailable
	}
	return p.Provider.QueryAggregateRange(ctx, request)
}

func (unavailableProvider) Health(context.Context) (datasource.Health, error) {
	return datasource.Health{}, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

func (unavailableProvider) ListHosts(context.Context) ([]datasource.Host, error) {
	return nil, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

func (unavailableProvider) GetHost(context.Context, string) (datasource.Host, error) {
	return datasource.Host{}, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

func (unavailableProvider) GetCurrentMetrics(context.Context, []string) (map[string]datasource.CurrentMetrics, error) {
	return nil, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

func (unavailableProvider) QueryRange(context.Context, datasource.RangeRequest) ([]datasource.Series, error) {
	return nil, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

func (unavailableProvider) QueryAggregateRange(context.Context, datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	return nil, errors.Join(datasource.ErrUnavailable, errors.New("upstream-secret"))
}

var _ datasource.Provider = unavailableProvider{}
