package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/adapters/nightingale"
	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/httpapi"
	"github.com/Taier05/InfraView/internal/mysql"
	"github.com/Taier05/InfraView/internal/service"
)

type commandDependencies struct {
	getenv      func(string) string
	serve       func(config.Config) error
	healthcheck func(string, *http.Client) error
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func main() {
	dependencies := commandDependencies{
		getenv:      os.Getenv,
		serve:       serve,
		healthcheck: healthcheck,
	}
	if err := runCommand(os.Args[1:], dependencies); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, dependencies commandDependencies) error {
	if len(args) != 1 {
		return fmt.Errorf("用法：infraview <serve|healthcheck>")
	}

	switch args[0] {
	case "serve":
		cfg, err := config.Load(dependencies.getenv)
		if err != nil {
			return fmt.Errorf("加载配置失败：%v", err)
		}
		return dependencies.serve(cfg)
	case "healthcheck":
		listenAddr := dependencies.getenv("INFRAVIEW_LISTEN_ADDR")
		if listenAddr == "" {
			listenAddr = ":8080"
		}
		return dependencies.healthcheck(listenAddr, &http.Client{Timeout: 2 * time.Second})
	default:
		return fmt.Errorf("未知命令 %q；用法：infraview <serve|healthcheck>", args[0])
	}
}

func serve(cfg config.Config) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           buildHandler(cfg, time.Now, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)
	return serveUntilSignal(server, shutdownSignals)
}

func buildHandler(cfg config.Config, clock func() time.Time, logger *slog.Logger) http.Handler {
	providers := dataSourceProviders(cfg, clock)
	provider := withUpstreamTimeout(providers.Hosts, cfg.UpstreamTimeout)
	store := cache.New(clock)
	queryService := service.New(provider, store, service.Options{
		InventoryTTL:       cfg.InventoryTTL,
		CurrentMetricsTTL:  cfg.CurrentMetricsTTL,
		RangeTTL:           cfg.RangeTTL,
		HealthTTL:          cfg.HealthTTL,
		MaxStale:           cfg.MaxStale,
		WarningPercent:     cfg.WarningPercent,
		CriticalPercent:    cfg.CriticalPercent,
		NetworkWarningBPS:  cfg.NetworkWarningBPS,
		NetworkCriticalBPS: cfg.NetworkCriticalBPS,
		Clock:              clock,
	})
	return httpapi.New(httpapi.Dependencies{
		Config:  cfg,
		Auth:    auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, clock),
		Limiter: auth.NewLimiter(5, time.Minute, clock),
		Service: queryService,
		Logger:  logger,
	})
}

type providerSet struct {
	Hosts datasource.Provider
	MySQL mysql.Provider
}

func dataSourceProviders(cfg config.Config, clock func() time.Time) providerSet {
	switch cfg.DataSource {
	case "mock":
		return providerSet{
			Hosts: mock.New(cfg.MockHostCount, clock),
			MySQL: mock.NewMySQL(),
		}
	case "nightingale":
		provider := nightingale.New(nightingale.Options{
			BaseURL:              cfg.NightingaleBaseURL,
			Token:                cfg.NightingaleToken,
			InterfaceExcludeExpr: cfg.NightingaleInterfaceExcludeRegex,
			Clock:                clock,
			AllowInsecureHTTP:    cfg.NightingaleAllowInsecureHTTP,
		})
		return providerSet{Hosts: provider, MySQL: provider}
	default:
		provider := nightingale.New(nightingale.Options{})
		return providerSet{Hosts: provider, MySQL: provider}
	}
}

type mysqlTimeoutProvider struct {
	provider mysql.Provider
	timeout  time.Duration
}

func withMySQLUpstreamTimeout(provider mysql.Provider, timeout time.Duration) mysql.Provider {
	return &mysqlTimeoutProvider{provider: provider, timeout: timeout}
}

func (p *mysqlTimeoutProvider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.MySQLSnapshot(ctx)
}

var _ mysql.Provider = (*mysqlTimeoutProvider)(nil)

type timeoutProvider struct {
	provider datasource.Provider
	timeout  time.Duration
}

func withUpstreamTimeout(provider datasource.Provider, timeout time.Duration) datasource.Provider {
	return &timeoutProvider{provider: provider, timeout: timeout}
}

func (p *timeoutProvider) Health(ctx context.Context) (datasource.Health, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.Health(ctx)
}

func (p *timeoutProvider) ListHosts(ctx context.Context) ([]datasource.Host, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.ListHosts(ctx)
}

func (p *timeoutProvider) GetHost(ctx context.Context, id string) (datasource.Host, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.GetHost(ctx, id)
}

func (p *timeoutProvider) GetCurrentMetrics(ctx context.Context, ids []string) (map[string]datasource.CurrentMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.GetCurrentMetrics(ctx, ids)
}

func (p *timeoutProvider) QueryRange(ctx context.Context, request datasource.RangeRequest) ([]datasource.Series, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.QueryRange(ctx, request)
}

func (p *timeoutProvider) QueryAggregateRange(ctx context.Context, request datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.QueryAggregateRange(ctx, request)
}

var _ datasource.Provider = (*timeoutProvider)(nil)

func serveUntilSignal(server httpServer, shutdownSignals <-chan os.Signal) error {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP 服务启动失败：请检查监听地址与端口是否可用")
	case <-shutdownSignals:
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return errors.New("HTTP 服务关闭失败：未能在 10 秒内完成优雅关闭")
		}
		if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP 服务关闭后异常退出")
		}
		return nil
	}
}

func healthcheckURL(listenAddr string) (string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("解析 INFRAVIEW_LISTEN_ADDR 失败：当前值为 %q", listenAddr)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz", nil
}

func healthcheck(listenAddr string, client *http.Client) error {
	targetURL, err := healthcheckURL(listenAddr)
	if err != nil {
		return err
	}
	response, err := client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("健康检查失败：无法访问服务")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("健康检查失败：服务返回 HTTP %d", response.StatusCode)
	}
	return nil
}
