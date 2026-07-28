package config

import (
	"net/netip"
	"strings"
	"testing"
	"time"
)

var _ Config

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_USERNAME": "admin",
		"INFRAVIEW_PASSWORD": "secret-value",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("listen = %q", cfg.ListenAddr)
	}
	if cfg.DataSource != "mock" {
		t.Fatalf("source = %q", cfg.DataSource)
	}
	if cfg.NightingaleBaseURL != "" || cfg.NightingaleToken != "" {
		t.Fatalf("Nightingale defaults must be empty")
	}
	if cfg.NightingaleInterfaceExcludeRegex != `lo|docker.*|veth.*|cali.*|br-.*|tunl.*` {
		t.Fatalf("Nightingale interface exclusion = %q", cfg.NightingaleInterfaceExcludeRegex)
	}
	if cfg.NightingaleAllowInsecureHTTP {
		t.Fatal("Nightingale insecure HTTP must be disabled by default")
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("session = %s", cfg.SessionTTL)
	}
	if cfg.CookieSecure {
		t.Fatal("cookie secure = true")
	}
	if len(cfg.TrustedProxyCIDRs) != 0 {
		t.Fatalf("trusted proxy CIDRs = %v, want none", cfg.TrustedProxyCIDRs)
	}
	if cfg.MockHostCount != 32 {
		t.Fatalf("mock hosts = %d", cfg.MockHostCount)
	}
	if cfg.RefreshInterval != 15*time.Second {
		t.Fatalf("refresh = %s", cfg.RefreshInterval)
	}
	if cfg.InventoryTTL != 60*time.Second || cfg.CurrentMetricsTTL != 15*time.Second {
		t.Fatalf("inventory/current TTL = %s/%s", cfg.InventoryTTL, cfg.CurrentMetricsTTL)
	}
	if cfg.RangeTTL != 60*time.Second || cfg.HealthTTL != 15*time.Second {
		t.Fatalf("range/health TTL = %s/%s", cfg.RangeTTL, cfg.HealthTTL)
	}
	if cfg.MaxStale != 5*time.Minute || cfg.UpstreamTimeout != 10*time.Second {
		t.Fatalf("max stale/upstream timeout = %s/%s", cfg.MaxStale, cfg.UpstreamTimeout)
	}
	if cfg.WarningPercent != 80 || cfg.CriticalPercent != 90 {
		t.Fatalf("thresholds = %v/%v", cfg.WarningPercent, cfg.CriticalPercent)
	}
	if cfg.NetworkWarningBPS != 80*1024*1024 || cfg.NetworkCriticalBPS != 100*1024*1024 {
		t.Fatalf("network thresholds = %v/%v", cfg.NetworkWarningBPS, cfg.NetworkCriticalBPS)
	}
}

func TestMockModeIgnoresNightingaleSettings(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_USERNAME":                            "admin",
		"INFRAVIEW_PASSWORD":                            "secret-value",
		"INFRAVIEW_DATA_SOURCE":                         "mock",
		"INFRAVIEW_NIGHTINGALE_BASE_URL":                "ftp://n9e.example.test",
		"INFRAVIEW_NIGHTINGALE_TOKEN":                   "unused-mock-token",
		"INFRAVIEW_NIGHTINGALE_INTERFACE_EXCLUDE_REGEX": "[",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DataSource != "mock" {
		t.Fatalf("data source = %q, want mock", cfg.DataSource)
	}
}

func TestLoadAllValues(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_LISTEN_ADDR":                         "127.0.0.1:9090",
		"INFRAVIEW_USERNAME":                            "operator",
		"INFRAVIEW_PASSWORD":                            "long-password",
		"INFRAVIEW_SESSION_TTL":                         "24h",
		"INFRAVIEW_COOKIE_SECURE":                       "true",
		"INFRAVIEW_TRUSTED_PROXY_CIDRS":                 "127.0.0.1/32, 10.23.4.5/8",
		"INFRAVIEW_DATA_SOURCE":                         "nightingale",
		"INFRAVIEW_NIGHTINGALE_BASE_URL":                "https://n9e.example.test/base",
		"INFRAVIEW_NIGHTINGALE_TOKEN":                   "fixture-config-token",
		"INFRAVIEW_NIGHTINGALE_INTERFACE_EXCLUDE_REGEX": `lo|veth.*`,
		"INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP":     "true",
		"INFRAVIEW_MOCK_HOST_COUNT":                     "100",
		"INFRAVIEW_REFRESH_INTERVAL":                    "45s",
		"INFRAVIEW_INVENTORY_TTL":                       "2m",
		"INFRAVIEW_CURRENT_METRICS_TTL":                 "25s",
		"INFRAVIEW_RANGE_TTL":                           "90s",
		"INFRAVIEW_HEALTH_TTL":                          "20s",
		"INFRAVIEW_MAX_STALE":                           "4m",
		"INFRAVIEW_UPSTREAM_TIMEOUT":                    "15s",
		"INFRAVIEW_WARNING_PERCENT":                     "75.5",
		"INFRAVIEW_CRITICAL_PERCENT":                    "95.5",
		"INFRAVIEW_NETWORK_WARNING_BPS":                 "1048576",
		"INFRAVIEW_NETWORK_CRITICAL_BPS":                "2097152",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:9090" || cfg.Username != "operator" || cfg.Password != "long-password" {
		t.Fatalf("identity values = %#v", cfg)
	}
	if cfg.SessionTTL != 24*time.Hour || !cfg.CookieSecure || cfg.DataSource != "nightingale" || cfg.MockHostCount != 100 {
		t.Fatalf("service values = %#v", cfg)
	}
	if cfg.NightingaleBaseURL != "https://n9e.example.test/base" || cfg.NightingaleToken != "fixture-config-token" || cfg.NightingaleInterfaceExcludeRegex != `lo|veth.*` {
		t.Fatalf("Nightingale values were not loaded")
	}
	if !cfg.NightingaleAllowInsecureHTTP {
		t.Fatal("Nightingale insecure HTTP opt-in was not loaded")
	}
	wantTrustedProxies := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("10.0.0.0/8")}
	if len(cfg.TrustedProxyCIDRs) != len(wantTrustedProxies) {
		t.Fatalf("trusted proxy CIDRs = %v, want %v", cfg.TrustedProxyCIDRs, wantTrustedProxies)
	}
	for index := range wantTrustedProxies {
		if cfg.TrustedProxyCIDRs[index] != wantTrustedProxies[index] {
			t.Fatalf("trusted proxy CIDR %d = %v, want %v", index, cfg.TrustedProxyCIDRs[index], wantTrustedProxies[index])
		}
	}
	if cfg.RefreshInterval != 45*time.Second || cfg.InventoryTTL != 2*time.Minute || cfg.CurrentMetricsTTL != 25*time.Second {
		t.Fatalf("short durations = %#v", cfg)
	}
	if cfg.RangeTTL != 90*time.Second || cfg.HealthTTL != 20*time.Second || cfg.MaxStale != 4*time.Minute || cfg.UpstreamTimeout != 15*time.Second {
		t.Fatalf("cache durations = %#v", cfg)
	}
	if cfg.WarningPercent != 75.5 || cfg.CriticalPercent != 95.5 {
		t.Fatalf("thresholds = %v/%v", cfg.WarningPercent, cfg.CriticalPercent)
	}
	if cfg.NetworkWarningBPS != 1048576 || cfg.NetworkCriticalBPS != 2097152 {
		t.Fatalf("network thresholds = %v/%v", cfg.NetworkWarningBPS, cfg.NetworkCriticalBPS)
	}
}

func TestLoadAllowsExplicitInsecureNightingaleHTTP(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_USERNAME":                        "admin",
		"INFRAVIEW_PASSWORD":                        "secret-value",
		"INFRAVIEW_DATA_SOURCE":                     "nightingale",
		"INFRAVIEW_NIGHTINGALE_BASE_URL":            "http://n9e.example.test",
		"INFRAVIEW_NIGHTINGALE_TOKEN":               "fixture-token",
		"INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.NightingaleAllowInsecureHTTP {
		t.Fatal("Nightingale insecure HTTP opt-in = false")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantError string
	}{
		{name: "missing username", overrides: map[string]string{"INFRAVIEW_USERNAME": ""}, wantError: "INFRAVIEW_USERNAME 必填"},
		{name: "missing password", overrides: map[string]string{"INFRAVIEW_PASSWORD": ""}, wantError: "INFRAVIEW_PASSWORD 必填"},
		{name: "short password", overrides: map[string]string{"INFRAVIEW_PASSWORD": "too-short"}, wantError: "INFRAVIEW_PASSWORD 长度必须至少为 12 个字符"},
		{name: "invalid duration", overrides: map[string]string{"INFRAVIEW_SESSION_TTL": "later"}, wantError: `INFRAVIEW_SESSION_TTL 必须是有效时长，当前值为 "later"`},
		{name: "non-positive duration", overrides: map[string]string{"INFRAVIEW_REFRESH_INTERVAL": "0s"}, wantError: `INFRAVIEW_REFRESH_INTERVAL 必须大于 0，当前值为 "0s"`},
		{name: "refresh interval below one second", overrides: map[string]string{"INFRAVIEW_REFRESH_INTERVAL": "500ms"}, wantError: `INFRAVIEW_REFRESH_INTERVAL 必须是不小于 1s 的整秒时长，当前值为 "500ms"`},
		{name: "refresh interval with fractional second", overrides: map[string]string{"INFRAVIEW_REFRESH_INTERVAL": "1500ms"}, wantError: `INFRAVIEW_REFRESH_INTERVAL 必须是不小于 1s 的整秒时长，当前值为 "1500ms"`},
		{name: "invalid boolean", overrides: map[string]string{"INFRAVIEW_COOKIE_SECURE": "sometimes"}, wantError: `INFRAVIEW_COOKIE_SECURE 必须是布尔值（true 或 false），当前值为 "sometimes"`},
		{name: "invalid trusted proxy CIDR", overrides: map[string]string{"INFRAVIEW_TRUSTED_PROXY_CIDRS": "10.0.0.1"}, wantError: `INFRAVIEW_TRUSTED_PROXY_CIDRS 必须是逗号分隔的有效 CIDR，非法值为 "10.0.0.1"`},
		{name: "empty trusted proxy CIDR", overrides: map[string]string{"INFRAVIEW_TRUSTED_PROXY_CIDRS": "10.0.0.0/8,,127.0.0.1/32"}, wantError: `INFRAVIEW_TRUSTED_PROXY_CIDRS 不得包含空的 CIDR`},
		{name: "unsupported source", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "other"}, wantError: `INFRAVIEW_DATA_SOURCE 仅支持 "mock" 或 "nightingale"，当前值为 "other"`},
		{name: "missing Nightingale base URL", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 在 Nightingale 模式下必填"},
		{name: "invalid Nightingale base URL", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "not-a-url", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 必须是有效的 HTTP(S) 绝对 URL"},
		{name: "unsupported Nightingale URL scheme", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "ftp://n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 必须是有效的 HTTP(S) 绝对 URL"},
		{name: "Nightingale HTTP without explicit opt-in", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "http://n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 使用 HTTP 时必须显式启用 INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP"},
		{name: "invalid Nightingale insecure HTTP boolean", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token", "INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP": "sometimes"}, wantError: `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP 必须是布尔值（true 或 false），当前值为 "sometimes"`},
		{name: "Nightingale URL user info", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://user:pass@n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 不得包含用户信息、查询参数或片段"},
		{name: "Nightingale URL query", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://n9e.example.test?secret=value", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token"}, wantError: "INFRAVIEW_NIGHTINGALE_BASE_URL 不得包含用户信息、查询参数或片段"},
		{name: "missing Nightingale token", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://n9e.example.test"}, wantError: "INFRAVIEW_NIGHTINGALE_TOKEN 在 Nightingale 模式下必填"},
		{name: "blank Nightingale token", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "   "}, wantError: "INFRAVIEW_NIGHTINGALE_TOKEN 在 Nightingale 模式下必填"},
		{name: "invalid Nightingale interface regex", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale", "INFRAVIEW_NIGHTINGALE_BASE_URL": "https://n9e.example.test", "INFRAVIEW_NIGHTINGALE_TOKEN": "fixture-token", "INFRAVIEW_NIGHTINGALE_INTERFACE_EXCLUDE_REGEX": "["}, wantError: "INFRAVIEW_NIGHTINGALE_INTERFACE_EXCLUDE_REGEX 必须是有效的 RE2 正则表达式"},
		{name: "invalid mock host count", overrides: map[string]string{"INFRAVIEW_MOCK_HOST_COUNT": "many"}, wantError: `INFRAVIEW_MOCK_HOST_COUNT 必须是整数，当前值为 "many"`},
		{name: "zero mock hosts", overrides: map[string]string{"INFRAVIEW_MOCK_HOST_COUNT": "0"}, wantError: "INFRAVIEW_MOCK_HOST_COUNT 必须在 1 到 100 之间"},
		{name: "too many mock hosts", overrides: map[string]string{"INFRAVIEW_MOCK_HOST_COUNT": "101"}, wantError: "INFRAVIEW_MOCK_HOST_COUNT 必须在 1 到 100 之间"},
		{name: "max stale over limit", overrides: map[string]string{"INFRAVIEW_MAX_STALE": "5m1s"}, wantError: `INFRAVIEW_MAX_STALE 不得超过 5m，当前值为 "5m1s"`},
		{name: "invalid warning percentage", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "101"}, wantError: `INFRAVIEW_WARNING_PERCENT 必须是 0 到 100 之间的有限数值，当前值为 "101"`},
		{name: "invalid critical percentage", overrides: map[string]string{"INFRAVIEW_CRITICAL_PERCENT": "invalid"}, wantError: `INFRAVIEW_CRITICAL_PERCENT 必须是 0 到 100 之间的有限数值，当前值为 "invalid"`},
		{name: "warning is NaN", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "NaN"}, wantError: `INFRAVIEW_WARNING_PERCENT 必须是 0 到 100 之间的有限数值，当前值为 "NaN"`},
		{name: "warning is positive infinity", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "+Inf"}, wantError: `INFRAVIEW_WARNING_PERCENT 必须是 0 到 100 之间的有限数值，当前值为 "+Inf"`},
		{name: "critical is negative infinity", overrides: map[string]string{"INFRAVIEW_CRITICAL_PERCENT": "-Inf"}, wantError: `INFRAVIEW_CRITICAL_PERCENT 必须是 0 到 100 之间的有限数值，当前值为 "-Inf"`},
		{name: "warning equals critical", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "90"}, wantError: "INFRAVIEW_WARNING_PERCENT 必须低于 INFRAVIEW_CRITICAL_PERCENT"},
		{name: "invalid network warning", overrides: map[string]string{"INFRAVIEW_NETWORK_WARNING_BPS": "0"}, wantError: `INFRAVIEW_NETWORK_WARNING_BPS 必须是大于 0 的有限数值，当前值为 "0"`},
		{name: "invalid network critical", overrides: map[string]string{"INFRAVIEW_NETWORK_CRITICAL_BPS": "NaN"}, wantError: `INFRAVIEW_NETWORK_CRITICAL_BPS 必须是大于 0 的有限数值，当前值为 "NaN"`},
		{name: "network warning equals critical", overrides: map[string]string{"INFRAVIEW_NETWORK_WARNING_BPS": "104857600"}, wantError: "INFRAVIEW_NETWORK_WARNING_BPS 必须低于 INFRAVIEW_NETWORK_CRITICAL_BPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{
				"INFRAVIEW_USERNAME": "admin",
				"INFRAVIEW_PASSWORD": "secret-value",
			}
			for key, value := range tt.overrides {
				env[key] = value
			}

			_, err := Load(mapEnv(env))
			if err == nil {
				t.Fatal("expected an error")
			}
			if err.Error() != tt.wantError {
				t.Fatalf("error = %q, want %q", err, tt.wantError)
			}
			if password := env["INFRAVIEW_PASSWORD"]; password != "" && strings.Contains(err.Error(), password) {
				t.Fatalf("error leaks password: %q", err)
			}
			if token := env["INFRAVIEW_NIGHTINGALE_TOKEN"]; token != "" && strings.Contains(err.Error(), token) {
				t.Fatalf("error leaks Nightingale token")
			}
		})
	}
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
