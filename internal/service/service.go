package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/datasource"
)

const (
	inventoryCacheKey = "service:inventory"
	healthCacheKey    = "service:health"
)

type Service struct {
	provider datasource.Provider
	store    *cache.Store
	options  Options
}

func New(provider datasource.Provider, store *cache.Store, options Options) *Service {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.InventoryTTL <= 0 {
		options.InventoryTTL = time.Minute
	}
	if options.CurrentMetricsTTL <= 0 {
		options.CurrentMetricsTTL = 20 * time.Second
	}
	if options.RangeTTL <= 0 {
		options.RangeTTL = time.Minute
	}
	if options.HealthTTL <= 0 {
		options.HealthTTL = 15 * time.Second
	}
	if options.MaxStale <= 0 {
		options.MaxStale = 5 * time.Minute
	}
	if options.WarningPercent == 0 {
		options.WarningPercent = 80
	}
	if options.CriticalPercent == 0 {
		options.CriticalPercent = 90
	}
	if options.NetworkWarningBPS == 0 {
		options.NetworkWarningBPS = 80 * 1024 * 1024
	}
	if options.NetworkCriticalBPS == 0 {
		options.NetworkCriticalBPS = 100 * 1024 * 1024
	}
	if store == nil {
		store = cache.New(options.Clock)
	}
	return &Service{provider: provider, store: store, options: options}
}

func (s *Service) inventory(ctx context.Context) ([]datasource.Host, Meta, error) {
	result, err := s.store.GetOrLoad(ctx, inventoryCacheKey, s.options.InventoryTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.provider.ListHosts(loadCtx)
	})
	if err != nil {
		return nil, Meta{}, mapProviderError(err)
	}
	hosts, ok := result.Value.([]datasource.Host)
	if !ok {
		return nil, Meta{}, fmt.Errorf("service: inventory cache contained %T", result.Value)
	}
	return hosts, resultMeta(result), nil
}

func (s *Service) host(ctx context.Context, id string) (datasource.Host, Meta, error) {
	key := "service:host:" + id
	result, err := s.store.GetOrLoad(ctx, key, s.options.InventoryTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.provider.GetHost(loadCtx, id)
	})
	if err != nil {
		return datasource.Host{}, Meta{}, mapProviderError(err)
	}
	host, ok := result.Value.(datasource.Host)
	if !ok {
		return datasource.Host{}, Meta{}, fmt.Errorf("service: host cache contained %T", result.Value)
	}
	return host, resultMeta(result), nil
}

func (s *Service) currentMetrics(ctx context.Context, ids []string) (map[string]datasource.CurrentMetrics, Meta, error) {
	if len(ids) == 0 {
		return map[string]datasource.CurrentMetrics{}, Meta{}, nil
	}
	stableIDs := append([]string(nil), ids...)
	sort.Strings(stableIDs)
	key := "service:current:" + strings.Join(stableIDs, ",")
	result, err := s.store.GetOrLoad(ctx, key, s.options.CurrentMetricsTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.provider.GetCurrentMetrics(loadCtx, stableIDs)
	})
	if err != nil {
		return nil, Meta{}, mapProviderError(err)
	}
	metrics, ok := result.Value.(map[string]datasource.CurrentMetrics)
	if !ok {
		return nil, Meta{}, fmt.Errorf("service: current metrics cache contained %T", result.Value)
	}
	return metrics, resultMeta(result), nil
}

func (s *Service) currentView(metrics datasource.CurrentMetrics) CurrentMetrics {
	filesystems := make([]Filesystem, len(metrics.Filesystems))
	for i, filesystem := range metrics.Filesystems {
		filesystems[i] = Filesystem{
			Mountpoint: filesystem.Mountpoint,
			Usage:      s.percentage(filesystem.Usage),
		}
	}
	return CurrentMetrics{
		Timestamp:                     metrics.Timestamp,
		CPUUsage:                      s.percentage(metrics.CPUUsage),
		MemoryUsage:                   s.percentage(metrics.MemoryUsage),
		Load1:                         scalar(metrics.Load1),
		IOBusyPercent:                 s.percentage(metrics.IOBusyPercent),
		DiskReadBytesPerSecond:        scalar(metrics.DiskReadBytesPerSecond),
		DiskWriteBytesPerSecond:       scalar(metrics.DiskWriteBytesPerSecond),
		NetworkReceiveBytesPerSecond:  s.networkThroughput(metrics.NetworkReceiveBytesPerSecond),
		NetworkTransmitBytesPerSecond: s.networkThroughput(metrics.NetworkTransmitBytesPerSecond),
		Filesystems:                   filesystems,
	}
}

func (s *Service) networkThroughput(value *float64) MetricValue {
	if value == nil {
		return MetricValue{Level: LevelUnknown}
	}
	level := LevelNormal
	if *value >= s.options.NetworkCriticalBPS {
		level = LevelCritical
	} else if *value >= s.options.NetworkWarningBPS {
		level = LevelWarning
	}
	return MetricValue{Value: cloneFloat(value), Level: level}
}

func (s *Service) percentage(value *float64) MetricValue {
	if value == nil {
		return MetricValue{Level: LevelUnknown}
	}
	level := LevelNormal
	if *value >= s.options.CriticalPercent {
		level = LevelCritical
	} else if *value >= s.options.WarningPercent {
		level = LevelWarning
	}
	return MetricValue{Value: cloneFloat(value), Level: level}
}

func scalar(value *float64) MetricValue {
	if value == nil {
		return MetricValue{Level: LevelUnknown}
	}
	return MetricValue{Value: cloneFloat(value), Level: LevelNormal}
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func resultMeta(result cache.Result) Meta {
	return Meta{Stale: result.State == cache.Stale, CollectedAt: result.StoredAt}
}

func mergeMeta(metas ...Meta) Meta {
	var merged Meta
	for _, meta := range metas {
		merged.Stale = merged.Stale || meta.Stale
		if meta.CollectedAt.IsZero() {
			continue
		}
		if merged.CollectedAt.IsZero() || meta.CollectedAt.Before(merged.CollectedAt) {
			merged.CollectedAt = meta.CollectedAt
		}
	}
	return merged
}

func mapProviderError(err error) error {
	if errors.Is(err, datasource.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return err
}
