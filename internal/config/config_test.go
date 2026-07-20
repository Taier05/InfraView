package config

import (
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
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("session = %s", cfg.SessionTTL)
	}
	if cfg.WarningPercent != 80 || cfg.CriticalPercent != 90 {
		t.Fatalf("thresholds = %v/%v", cfg.WarningPercent, cfg.CriticalPercent)
	}
}

func TestLoadAllValues(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_LISTEN_ADDR":         "127.0.0.1:9090",
		"INFRAVIEW_USERNAME":            "operator",
		"INFRAVIEW_PASSWORD":            "long-password",
		"INFRAVIEW_SESSION_TTL":         "24h",
		"INFRAVIEW_COOKIE_SECURE":       "true",
		"INFRAVIEW_DATA_SOURCE":         "mock",
		"INFRAVIEW_MOCK_HOST_COUNT":     "100",
		"INFRAVIEW_REFRESH_INTERVAL":    "45s",
		"INFRAVIEW_INVENTORY_TTL":       "2m",
		"INFRAVIEW_CURRENT_METRICS_TTL": "25s",
		"INFRAVIEW_RANGE_TTL":           "90s",
		"INFRAVIEW_HEALTH_TTL":          "20s",
		"INFRAVIEW_MAX_STALE":           "10m",
		"INFRAVIEW_UPSTREAM_TIMEOUT":    "15s",
		"INFRAVIEW_WARNING_PERCENT":     "75.5",
		"INFRAVIEW_CRITICAL_PERCENT":    "95.5",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:9090" || cfg.Username != "operator" || cfg.Password != "long-password" {
		t.Fatalf("identity values = %#v", cfg)
	}
	if cfg.SessionTTL != 24*time.Hour || !cfg.CookieSecure || cfg.DataSource != "mock" || cfg.MockHostCount != 100 {
		t.Fatalf("service values = %#v", cfg)
	}
	if cfg.RefreshInterval != 45*time.Second || cfg.InventoryTTL != 2*time.Minute || cfg.CurrentMetricsTTL != 25*time.Second {
		t.Fatalf("short durations = %#v", cfg)
	}
	if cfg.RangeTTL != 90*time.Second || cfg.HealthTTL != 20*time.Second || cfg.MaxStale != 10*time.Minute || cfg.UpstreamTimeout != 15*time.Second {
		t.Fatalf("cache durations = %#v", cfg)
	}
	if cfg.WarningPercent != 75.5 || cfg.CriticalPercent != 95.5 {
		t.Fatalf("thresholds = %v/%v", cfg.WarningPercent, cfg.CriticalPercent)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantError string
	}{
		{name: "missing username", overrides: map[string]string{"INFRAVIEW_USERNAME": ""}, wantError: "INFRAVIEW_USERNAME"},
		{name: "missing password", overrides: map[string]string{"INFRAVIEW_PASSWORD": ""}, wantError: "INFRAVIEW_PASSWORD"},
		{name: "short password", overrides: map[string]string{"INFRAVIEW_PASSWORD": "too-short"}, wantError: "INFRAVIEW_PASSWORD"},
		{name: "invalid duration", overrides: map[string]string{"INFRAVIEW_SESSION_TTL": "later"}, wantError: "INFRAVIEW_SESSION_TTL"},
		{name: "unsupported source", overrides: map[string]string{"INFRAVIEW_DATA_SOURCE": "nightingale"}, wantError: "INFRAVIEW_DATA_SOURCE"},
		{name: "zero mock hosts", overrides: map[string]string{"INFRAVIEW_MOCK_HOST_COUNT": "0"}, wantError: "INFRAVIEW_MOCK_HOST_COUNT"},
		{name: "too many mock hosts", overrides: map[string]string{"INFRAVIEW_MOCK_HOST_COUNT": "101"}, wantError: "INFRAVIEW_MOCK_HOST_COUNT"},
		{name: "invalid warning percentage", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "101"}, wantError: "INFRAVIEW_WARNING_PERCENT"},
		{name: "invalid critical percentage", overrides: map[string]string{"INFRAVIEW_CRITICAL_PERCENT": "invalid"}, wantError: "INFRAVIEW_CRITICAL_PERCENT"},
		{name: "warning equals critical", overrides: map[string]string{"INFRAVIEW_WARNING_PERCENT": "90"}, wantError: "lower"},
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
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantError)
			}
		})
	}
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
