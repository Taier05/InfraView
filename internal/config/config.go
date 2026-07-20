package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr        string
	Username          string
	Password          string
	SessionTTL        time.Duration
	CookieSecure      bool
	DataSource        string
	MockHostCount     int
	RefreshInterval   time.Duration
	InventoryTTL      time.Duration
	CurrentMetricsTTL time.Duration
	RangeTTL          time.Duration
	HealthTTL         time.Duration
	MaxStale          time.Duration
	UpstreamTimeout   time.Duration
	WarningPercent    float64
	CriticalPercent   float64
}

func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	var err error

	cfg.ListenAddr = valueOrDefault(getenv, "INFRAVIEW_LISTEN_ADDR", ":8080")
	cfg.Username = getenv("INFRAVIEW_USERNAME")
	if strings.TrimSpace(cfg.Username) == "" {
		return Config{}, fmt.Errorf("INFRAVIEW_USERNAME is required")
	}
	cfg.Password = getenv("INFRAVIEW_PASSWORD")
	if cfg.Password == "" {
		return Config{}, fmt.Errorf("INFRAVIEW_PASSWORD is required")
	}
	if len(cfg.Password) < 12 {
		return Config{}, fmt.Errorf("INFRAVIEW_PASSWORD must be at least 12 characters")
	}

	if cfg.SessionTTL, err = durationValue(getenv, "INFRAVIEW_SESSION_TTL", "12h"); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = boolValue(getenv, "INFRAVIEW_COOKIE_SECURE", "false"); err != nil {
		return Config{}, err
	}
	cfg.DataSource = valueOrDefault(getenv, "INFRAVIEW_DATA_SOURCE", "mock")
	if cfg.DataSource != "mock" {
		return Config{}, fmt.Errorf("INFRAVIEW_DATA_SOURCE %q is unsupported; only mock is supported", cfg.DataSource)
	}
	if cfg.MockHostCount, err = intValue(getenv, "INFRAVIEW_MOCK_HOST_COUNT", "32"); err != nil {
		return Config{}, err
	}
	if cfg.MockHostCount < 1 || cfg.MockHostCount > 100 {
		return Config{}, fmt.Errorf("INFRAVIEW_MOCK_HOST_COUNT must be between 1 and 100")
	}

	if cfg.RefreshInterval, err = durationValue(getenv, "INFRAVIEW_REFRESH_INTERVAL", "30s"); err != nil {
		return Config{}, err
	}
	if cfg.InventoryTTL, err = durationValue(getenv, "INFRAVIEW_INVENTORY_TTL", "60s"); err != nil {
		return Config{}, err
	}
	if cfg.CurrentMetricsTTL, err = durationValue(getenv, "INFRAVIEW_CURRENT_METRICS_TTL", "20s"); err != nil {
		return Config{}, err
	}
	if cfg.RangeTTL, err = durationValue(getenv, "INFRAVIEW_RANGE_TTL", "60s"); err != nil {
		return Config{}, err
	}
	if cfg.HealthTTL, err = durationValue(getenv, "INFRAVIEW_HEALTH_TTL", "15s"); err != nil {
		return Config{}, err
	}
	if cfg.MaxStale, err = durationValue(getenv, "INFRAVIEW_MAX_STALE", "5m"); err != nil {
		return Config{}, err
	}
	if cfg.UpstreamTimeout, err = durationValue(getenv, "INFRAVIEW_UPSTREAM_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}
	if cfg.WarningPercent, err = percentageValue(getenv, "INFRAVIEW_WARNING_PERCENT", "80"); err != nil {
		return Config{}, err
	}
	if cfg.CriticalPercent, err = percentageValue(getenv, "INFRAVIEW_CRITICAL_PERCENT", "90"); err != nil {
		return Config{}, err
	}
	if cfg.WarningPercent >= cfg.CriticalPercent {
		return Config{}, fmt.Errorf("INFRAVIEW_WARNING_PERCENT must be lower than INFRAVIEW_CRITICAL_PERCENT")
	}

	return cfg, nil
}

func valueOrDefault(getenv func(string) string, key, fallback string) string {
	if value := getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationValue(getenv func(string) string, key, fallback string) (time.Duration, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}

func boolValue(getenv func(string) string, key, fallback string) (bool, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}
	return value, nil
}

func intValue(getenv func(string) string, key, fallback string) (int, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}
	return value, nil
}

func percentageValue(getenv func(string) string, key, fallback string) (float64, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid percentage: %w", key, err)
	}
	if value < 0 || value > 100 {
		return 0, fmt.Errorf("%s must be between 0 and 100", key)
	}
	return value, nil
}
