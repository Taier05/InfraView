package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/nightingale"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/mysql"
	"github.com/Taier05/InfraView/internal/mysql/mysqltest"
)

func TestRunCommandRequiresExactlyOneCommand(t *testing.T) {
	err := runCommand(nil, commandDependencies{})
	if got, want := err.Error(), "用法：infraview <serve|healthcheck>"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestRunCommandRejectsUnknownCommand(t *testing.T) {
	err := runCommand([]string{"deploy"}, commandDependencies{})
	if got, want := err.Error(), `未知命令 "deploy"；用法：infraview <serve|healthcheck>`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestRunCommandServeLoadsConfigurationAndDispatches(t *testing.T) {
	called := false
	deps := commandDependencies{
		getenv: validEnv,
		serve: func(cfg config.Config) error {
			called = true
			if cfg.ListenAddr != ":8080" || cfg.Username != "admin" {
				t.Fatalf("config = %#v", cfg)
			}
			return nil
		},
	}

	if err := runCommand([]string{"serve"}, deps); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("serve was not called")
	}
}

func TestRunCommandReportsConfigurationFailureWithoutLeakingPassword(t *testing.T) {
	const password = "secret"
	deps := commandDependencies{
		getenv: func(key string) string {
			switch key {
			case "INFRAVIEW_USERNAME":
				return "admin"
			case "INFRAVIEW_PASSWORD":
				return password
			default:
				return ""
			}
		},
		serve: func(config.Config) error {
			t.Fatal("serve must not be called")
			return nil
		},
	}

	err := runCommand([]string{"serve"}, deps)
	if got, want := err.Error(), "加载配置失败：INFRAVIEW_PASSWORD 长度必须至少为 12 个字符"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("error leaks password: %q", err)
	}
}

func TestRunCommandHealthcheckUsesDefaultAddressAndTimeout(t *testing.T) {
	called := false
	deps := commandDependencies{
		getenv: func(string) string { return "" },
		healthcheck: func(listenAddr string, client *http.Client) error {
			called = true
			if listenAddr != ":8080" {
				t.Fatalf("listen address = %q", listenAddr)
			}
			if client.Timeout != 2*time.Second {
				t.Fatalf("timeout = %s", client.Timeout)
			}
			return nil
		},
	}

	if err := runCommand([]string{"healthcheck"}, deps); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("healthcheck was not called")
	}
}

func TestHealthcheckURLReplacesWildcardHosts(t *testing.T) {
	tests := map[string]string{
		":8080":        "http://127.0.0.1:8080/healthz",
		"0.0.0.0:8080": "http://127.0.0.1:8080/healthz",
		"[::]:8080":    "http://127.0.0.1:8080/healthz",
		"*:8080":       "http://127.0.0.1:8080/healthz",
	}
	for listenAddr, want := range tests {
		t.Run(listenAddr, func(t *testing.T) {
			got, err := healthcheckURL(listenAddr)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("URL = %q, want %q", got, want)
			}
		})
	}
}

func TestHealthcheckURLReportsInvalidListenAddressInChinese(t *testing.T) {
	_, err := healthcheckURL("8080")
	if got, want := err.Error(), `解析 INFRAVIEW_LISTEN_ADDR 失败：当前值为 "8080"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestHealthcheckCallsHealthEndpoint(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		called = true
		if request.Method != http.MethodGet || request.URL.Path != "/healthz" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := healthcheck(strings.TrimPrefix(server.URL, "http://"), server.Client()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("health endpoint was not called")
	}
}

func TestHealthcheckReportsNonOKStatusInChinese(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	err := healthcheck(strings.TrimPrefix(server.URL, "http://"), server.Client())
	if got, want := err.Error(), "健康检查失败：服务返回 HTTP 503"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestBuildHandlerWiresAuthenticatedMockAPI(t *testing.T) {
	cfg := config.Config{
		Username:          "admin",
		Password:          "correct-password",
		SessionTTL:        12 * time.Hour,
		DataSource:        "mock",
		MockHostCount:     3,
		InventoryTTL:      time.Minute,
		CurrentMetricsTTL: 20 * time.Second,
		RangeTTL:          time.Minute,
		HealthTTL:         15 * time.Second,
		MaxStale:          5 * time.Minute,
		UpstreamTimeout:   10 * time.Second,
		WarningPercent:    80,
		CriticalPercent:   90,
	}
	handler := buildHandler(cfg, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))

	loginRequest := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusNoContent || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login response = %d, cookies = %#v, body = %s", login.Code, login.Result().Cookies(), login.Body.String())
	}

	overviewRequest := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/overview?range=24h", nil)
	overviewRequest.AddCookie(login.Result().Cookies()[0])
	overview := httptest.NewRecorder()
	handler.ServeHTTP(overview, overviewRequest)
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), `"total":3`) {
		t.Fatalf("overview response = %d %s", overview.Code, overview.Body.String())
	}
}

func TestDataSourceProvidersWiresNightingaleConfiguration(t *testing.T) {
	const token = "fixture-main-token"
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		called = true
		if request.Method != http.MethodGet || request.URL.Path != "/api/n9e/self/profile" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("X-User-Token") != token {
			t.Fatalf("Nightingale token header was not injected")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"dat":{},"err":"","request_id":"fixture"}`)
	}))
	defer server.Close()

	clock := func() time.Time { return time.Unix(1785123060, 0).UTC() }
	providers := dataSourceProviders(config.Config{
		DataSource:                       "nightingale",
		NightingaleBaseURL:               server.URL,
		NightingaleToken:                 token,
		NightingaleInterfaceExcludeRegex: `lo|veth.*`,
		NightingaleAllowInsecureHTTP:     true,
	}, clock)
	health, err := providers.Hosts.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !called || !health.Healthy || !health.CheckedAt.Equal(clock()) {
		t.Fatalf("health = %#v, called = %v", health, called)
	}
}

func TestProviderSetUsesMockForHostsAndMySQL(t *testing.T) {
	providers := dataSourceProviders(config.Config{
		DataSource: "mock", MockHostCount: 8,
	}, time.Now)
	if providers.Hosts == nil || providers.MySQL == nil {
		t.Fatalf("providers = %#v", providers)
	}
	mysqltest.RunContract(t, providers.MySQL)
}

func TestProviderSetSharesOneNightingaleProvider(t *testing.T) {
	providers := dataSourceProviders(config.Config{
		DataSource:         "nightingale",
		NightingaleBaseURL: "https://n9e.example.com",
		NightingaleToken:   "fixture-token",
	}, time.Now)
	hostProvider, hostOK := providers.Hosts.(*nightingale.Provider)
	mysqlProvider, mysqlOK := providers.MySQL.(*nightingale.Provider)
	if !hostOK || !mysqlOK || hostProvider != mysqlProvider {
		t.Fatalf("providers do not share one Nightingale client")
	}
}

func TestMySQLTimeoutProviderCancelsSlowSnapshot(t *testing.T) {
	provider := withMySQLUpstreamTimeout(blockingMySQLProvider{}, 10*time.Millisecond)
	start := time.Now()
	_, err := provider.MySQLSnapshot(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("error = %v, elapsed = %s", err, time.Since(start))
	}
}

func TestComposeUsesUnlessStoppedRestartPolicy(t *testing.T) {
	compose, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), "\n    restart: unless-stopped\n") {
		t.Fatalf("infraview service must use restart: unless-stopped:\n%s", compose)
	}
}

func TestUpstreamTimeoutProviderAddsDeadlineToEveryCall(t *testing.T) {
	provider := &deadlineCheckingProvider{t: t}
	timed := withUpstreamTimeout(provider, time.Hour)
	ctx := context.Background()
	_, _ = timed.Health(ctx)
	_, _ = timed.ListHosts(ctx)
	_, _ = timed.GetHost(ctx, "host-1")
	_, _ = timed.GetCurrentMetrics(ctx, []string{"host-1"})
	_, _ = timed.QueryRange(ctx, datasource.RangeRequest{HostIDs: []string{"host-1"}})
	_, _ = timed.QueryAggregateRange(ctx, datasource.AggregateRangeRequest{Keys: []datasource.MetricKey{datasource.MetricCPUUsage}})
	if provider.calls != 6 {
		t.Fatalf("provider calls = %d, want 6", provider.calls)
	}
}

func TestServeUntilSignalReportsStartupErrorInChinese(t *testing.T) {
	server := &controlledServer{listenErr: errors.New("bind: address already in use")}
	err := serveUntilSignal(server, make(chan os.Signal))
	if got, want := err.Error(), "HTTP 服务启动失败：请检查监听地址与端口是否可用"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestServeUntilSignalShutsDownWithinTenSeconds(t *testing.T) {
	server := newControlledServer()
	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveUntilSignal(server, signals)
	}()

	select {
	case <-server.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start")
	}
	signals <- syscall.SIGTERM

	select {
	case shutdownContext := <-server.shutdownContext:
		deadline, ok := shutdownContext.Deadline()
		if !ok {
			t.Fatal("shutdown context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining < 9*time.Second || remaining > 10*time.Second {
			t.Fatalf("shutdown deadline remaining = %s", remaining)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown was not called")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveUntilSignal did not return")
	}
}

func TestServeUntilSignalReportsShutdownErrorInChinese(t *testing.T) {
	server := newControlledServer()
	server.shutdownErr = errors.New("context deadline exceeded")
	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveUntilSignal(server, signals)
	}()
	<-server.started
	signals <- syscall.SIGTERM

	err := <-done
	if got, want := err.Error(), "HTTP 服务关闭失败：未能在 10 秒内完成优雅关闭"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

type controlledServer struct {
	started         chan struct{}
	stopped         chan struct{}
	shutdownContext chan context.Context
	listenErr       error
	shutdownErr     error
	stopOnce        sync.Once
}

type deadlineCheckingProvider struct {
	t     *testing.T
	calls int
}

type blockingMySQLProvider struct{}

func (blockingMySQLProvider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error) {
	<-ctx.Done()
	return mysql.Snapshot{}, ctx.Err()
}

func (p *deadlineCheckingProvider) check(ctx context.Context) {
	p.t.Helper()
	p.calls++
	deadline, ok := ctx.Deadline()
	if !ok {
		p.t.Fatal("provider context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 59*time.Minute || remaining > time.Hour {
		p.t.Fatalf("provider deadline remaining = %s", remaining)
	}
}

func (p *deadlineCheckingProvider) Health(ctx context.Context) (datasource.Health, error) {
	p.check(ctx)
	return datasource.Health{}, nil
}

func (p *deadlineCheckingProvider) ListHosts(ctx context.Context) ([]datasource.Host, error) {
	p.check(ctx)
	return nil, nil
}

func (p *deadlineCheckingProvider) GetHost(ctx context.Context, _ string) (datasource.Host, error) {
	p.check(ctx)
	return datasource.Host{}, nil
}

func (p *deadlineCheckingProvider) GetCurrentMetrics(ctx context.Context, _ []string) (map[string]datasource.CurrentMetrics, error) {
	p.check(ctx)
	return nil, nil
}

func (p *deadlineCheckingProvider) QueryRange(ctx context.Context, _ datasource.RangeRequest) ([]datasource.Series, error) {
	p.check(ctx)
	return nil, nil
}

func (p *deadlineCheckingProvider) QueryAggregateRange(ctx context.Context, _ datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	p.check(ctx)
	return nil, nil
}

var _ datasource.Provider = (*deadlineCheckingProvider)(nil)

func newControlledServer() *controlledServer {
	return &controlledServer{
		started:         make(chan struct{}),
		stopped:         make(chan struct{}),
		shutdownContext: make(chan context.Context, 1),
	}
}

func (server *controlledServer) ListenAndServe() error {
	if server.listenErr != nil {
		return server.listenErr
	}
	close(server.started)
	<-server.stopped
	return http.ErrServerClosed
}

func (server *controlledServer) Shutdown(ctx context.Context) error {
	server.shutdownContext <- ctx
	server.stopOnce.Do(func() { close(server.stopped) })
	return server.shutdownErr
}

func validEnv(key string) string {
	switch key {
	case "INFRAVIEW_USERNAME":
		return "admin"
	case "INFRAVIEW_PASSWORD":
		return "secret-value"
	default:
		return ""
	}
}
