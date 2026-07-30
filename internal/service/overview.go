package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

func (s *Service) Overview(ctx context.Context, rangeName string) (Overview, Meta, error) {
	duration, ok := namedRange(rangeName)
	if !ok {
		return Overview{}, Meta{}, ErrInvalidRange
	}
	hosts, inventoryMeta, err := s.inventory(ctx)
	if err != nil {
		return Overview{}, Meta{}, err
	}
	ids := make([]string, len(hosts))
	for i, host := range hosts {
		ids[i] = host.ID
	}
	metrics, metricsMeta, err := s.currentMetrics(ctx, ids)
	if err != nil {
		return Overview{}, Meta{}, err
	}
	trends, trendsMeta, err := s.overviewTrends(ctx, rangeName, duration)
	if err != nil {
		return Overview{}, Meta{}, err
	}

	overview := Overview{Total: len(hosts), Trends: trends}
	var cpuTotal, memoryTotal float64
	var cpuCount, memoryCount int
	for _, host := range hosts {
		current := metrics[host.ID]
		collectionLevel := s.hostCollectionLevel(host.ID, current)
		status := effectiveHostStatus(host.Status, collectionLevel)
		switch status {
		case datasource.StatusOnline:
			overview.Online++
		case datasource.StatusOffline:
			overview.Offline++
		default:
			overview.Unknown++
		}
		if current.CPUUsage != nil {
			cpuTotal += *current.CPUUsage
			cpuCount++
		}
		if current.MemoryUsage != nil {
			memoryTotal += *current.MemoryUsage
			memoryCount++
		}
		view := s.currentView(current)
		updateAlertCount(&overview.Alerts.CPU, view.CPUUsage.Level)
		updateAlertCount(&overview.Alerts.Memory, view.MemoryUsage.Level)
		updateAlertCount(&overview.Alerts.IO, view.IOBusyPercent.Level)
		updateAlertCount(
			&overview.Alerts.Network,
			higherLevel(
				view.NetworkTransmitBytesPerSecond.Level,
				view.NetworkReceiveBytesPerSecond.Level,
			),
		)

		hostLevel := LevelNormal
		switch status {
		case datasource.StatusOffline:
			hostLevel = LevelCritical
		case datasource.StatusUnknown:
			hostLevel = LevelWarning
		}
		hostLevel = higherLevel(hostLevel, collectionLevel)
		for _, level := range []Level{
			view.CPUUsage.Level,
			view.MemoryUsage.Level,
			view.IOBusyPercent.Level,
			view.NetworkTransmitBytesPerSecond.Level,
			view.NetworkReceiveBytesPerSecond.Level,
		} {
			hostLevel = higherLevel(hostLevel, level)
		}
		switch hostLevel {
		case LevelCritical:
			overview.Alerts.CriticalHosts++
		case LevelWarning:
			overview.Alerts.WarningHosts++
		}
	}
	overview.Alerts.AffectedHosts =
		overview.Alerts.WarningHosts + overview.Alerts.CriticalHosts
	overview.CPUAverage = s.average(cpuTotal, cpuCount)
	overview.MemoryAverage = s.average(memoryTotal, memoryCount)
	return overview, mergeMeta(inventoryMeta, metricsMeta, trendsMeta), nil
}

func updateAlertCount(count *AlertCount, level Level) {
	switch level {
	case LevelWarning:
		count.Warning++
	case LevelCritical:
		count.Critical++
	}
}

func higherLevel(left, right Level) Level {
	if levelRank(right) > levelRank(left) {
		return right
	}
	return left
}

func levelRank(level Level) int {
	switch level {
	case LevelCritical:
		return 3
	case LevelWarning:
		return 2
	case LevelNormal:
		return 1
	default:
		return 0
	}
}

func (s *Service) overviewTrends(ctx context.Context, rangeName string, duration time.Duration) ([]TrendSeries, Meta, error) {
	key := "service:overview:trends:" + rangeName
	result, err := s.store.GetOrLoad(ctx, key, s.options.RangeTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.loadOverviewTrends(loadCtx, duration)
	})
	if err != nil {
		return nil, Meta{}, mapProviderError(err)
	}
	trends, ok := result.Value.([]TrendSeries)
	if !ok {
		return nil, Meta{}, fmt.Errorf("service: overview trends cache contained %T", result.Value)
	}
	return cloneTrendSeries(trends), resultMeta(result), nil
}

func (s *Service) loadOverviewTrends(ctx context.Context, duration time.Duration) ([]TrendSeries, error) {
	end := s.options.Clock()
	start := end.Add(-duration)
	keys := []datasource.MetricKey{datasource.MetricCPUUsage, datasource.MetricMemoryUsage}
	series, err := s.provider.QueryAggregateRange(ctx, datasource.AggregateRangeRequest{
		Keys:  keys,
		Start: start,
		End:   end,
		Step:  rangeStep(duration),
	})
	if err != nil {
		return nil, err
	}

	trends := make([]TrendSeries, 0, len(keys))
	for _, key := range keys {
		trend := TrendSeries{Key: key, Unit: "%", Points: []MetricPoint{}}
		for _, candidate := range series {
			if candidate.HostID != "" || candidate.Metric != key {
				continue
			}
			trend.Points = normalizePoints(candidate.Points)
			break
		}
		trends = append(trends, trend)
	}
	return trends, nil
}

func cloneTrendSeries(series []TrendSeries) []TrendSeries {
	clone := make([]TrendSeries, len(series))
	for i, trend := range series {
		clone[i] = TrendSeries{
			Key:    trend.Key,
			Unit:   trend.Unit,
			Points: make([]MetricPoint, len(trend.Points)),
		}
		for j, point := range trend.Points {
			clone[i].Points[j] = MetricPoint{Timestamp: point.Timestamp, Value: cloneFloat(point.Value)}
		}
	}
	return clone
}

func (s *Service) average(total float64, count int) MetricValue {
	if count == 0 {
		return MetricValue{Level: LevelUnknown}
	}
	average := total / float64(count)
	return s.percentage(&average)
}
