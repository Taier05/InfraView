package config

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr         string
	Username           string
	Password           string
	SessionTTL         time.Duration
	CookieSecure       bool
	TrustedProxyCIDRs  []netip.Prefix
	DataSource         string
	MockHostCount      int
	RefreshInterval    time.Duration
	InventoryTTL       time.Duration
	CurrentMetricsTTL  time.Duration
	RangeTTL           time.Duration
	HealthTTL          time.Duration
	MaxStale           time.Duration
	UpstreamTimeout    time.Duration
	WarningPercent     float64
	CriticalPercent    float64
	NetworkWarningBPS  float64
	NetworkCriticalBPS float64
}

func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	var err error

	cfg.ListenAddr = valueOrDefault(getenv, "INFRAVIEW_LISTEN_ADDR", ":8080")
	cfg.Username = getenv("INFRAVIEW_USERNAME")
	if strings.TrimSpace(cfg.Username) == "" {
		return Config{}, fmt.Errorf("INFRAVIEW_USERNAME 必填")
	}
	cfg.Password = getenv("INFRAVIEW_PASSWORD")
	if cfg.Password == "" {
		return Config{}, fmt.Errorf("INFRAVIEW_PASSWORD 必填")
	}
	if len(cfg.Password) < 12 {
		return Config{}, fmt.Errorf("INFRAVIEW_PASSWORD 长度必须至少为 12 个字符")
	}

	if cfg.SessionTTL, err = durationValue(getenv, "INFRAVIEW_SESSION_TTL", "12h"); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = boolValue(getenv, "INFRAVIEW_COOKIE_SECURE", "false"); err != nil {
		return Config{}, err
	}
	if cfg.TrustedProxyCIDRs, err = trustedProxyCIDRsValue(getenv, "INFRAVIEW_TRUSTED_PROXY_CIDRS"); err != nil {
		return Config{}, err
	}
	cfg.DataSource = valueOrDefault(getenv, "INFRAVIEW_DATA_SOURCE", "mock")
	if cfg.DataSource != "mock" {
		return Config{}, fmt.Errorf("INFRAVIEW_DATA_SOURCE 仅支持 %q，当前值为 %q", "mock", cfg.DataSource)
	}
	if cfg.MockHostCount, err = intValue(getenv, "INFRAVIEW_MOCK_HOST_COUNT", "32"); err != nil {
		return Config{}, err
	}
	if cfg.MockHostCount < 1 || cfg.MockHostCount > 100 {
		return Config{}, fmt.Errorf("INFRAVIEW_MOCK_HOST_COUNT 必须在 1 到 100 之间")
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
	if cfg.MaxStale > 5*time.Minute {
		return Config{}, fmt.Errorf("INFRAVIEW_MAX_STALE 不得超过 5m，当前值为 %q", valueOrDefault(getenv, "INFRAVIEW_MAX_STALE", "5m"))
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
		return Config{}, fmt.Errorf("INFRAVIEW_WARNING_PERCENT 必须低于 INFRAVIEW_CRITICAL_PERCENT")
	}
	if cfg.NetworkWarningBPS, err = positiveFloatValue(getenv, "INFRAVIEW_NETWORK_WARNING_BPS", "83886080"); err != nil {
		return Config{}, err
	}
	if cfg.NetworkCriticalBPS, err = positiveFloatValue(getenv, "INFRAVIEW_NETWORK_CRITICAL_BPS", "104857600"); err != nil {
		return Config{}, err
	}
	if cfg.NetworkWarningBPS >= cfg.NetworkCriticalBPS {
		return Config{}, fmt.Errorf("INFRAVIEW_NETWORK_WARNING_BPS 必须低于 INFRAVIEW_NETWORK_CRITICAL_BPS")
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
		return 0, fmt.Errorf("%s 必须是有效时长，当前值为 %q", key, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s 必须大于 0，当前值为 %q", key, raw)
	}
	return value, nil
}

func boolValue(getenv func(string) string, key, fallback string) (bool, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s 必须是布尔值（true 或 false），当前值为 %q", key, raw)
	}
	return value, nil
}

func trustedProxyCIDRsValue(getenv func(string) string, key string) ([]netip.Prefix, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, fmt.Errorf("%s 不得包含空的 CIDR", key)
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s 必须是逗号分隔的有效 CIDR，非法值为 %q", key, value)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func intValue(getenv func(string) string, key, fallback string) (int, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，当前值为 %q", key, raw)
	}
	return value, nil
}

func percentageValue(getenv func(string) string, key, fallback string) (float64, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
		return 0, fmt.Errorf("%s 必须是 0 到 100 之间的有限数值，当前值为 %q", key, raw)
	}
	return value, nil
}

func positiveFloatValue(getenv func(string) string, key, fallback string) (float64, error) {
	raw := valueOrDefault(getenv, key, fallback)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, fmt.Errorf("%s 必须是大于 0 的有限数值，当前值为 %q", key, raw)
	}
	return value, nil
}
