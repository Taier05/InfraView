package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

const maxRangePoints = 600

var rangeMetrics = []datasource.MetricKey{
	datasource.MetricCPUUsage,
	datasource.MetricMemoryUsage,
	datasource.MetricLoad1,
	datasource.MetricDiskUsage,
	datasource.MetricDiskReadBytesPerSecond,
	datasource.MetricDiskWriteBytesPerSecond,
	datasource.MetricNetworkReceiveBytesPerSecond,
	datasource.MetricNetworkTransmitBytesPerSecond,
}

func (s *Service) Metrics(ctx context.Context, id, rangeName string) (MetricRange, Meta, error) {
	duration, ok := namedRange(rangeName)
	if !ok {
		return MetricRange{}, Meta{}, ErrInvalidRange
	}
	key := "service:range:" + id + ":" + rangeName
	result, err := s.store.GetOrLoad(ctx, key, s.options.RangeTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.loadMetricRange(loadCtx, id, rangeName, duration)
	})
	if err != nil {
		return MetricRange{}, Meta{}, mapProviderError(err)
	}
	metricRange, ok := result.Value.(MetricRange)
	if !ok {
		return MetricRange{}, Meta{}, fmt.Errorf("service: range cache contained %T", result.Value)
	}
	return cloneMetricRange(metricRange), resultMeta(result), nil
}

func (s *Service) loadMetricRange(ctx context.Context, id, rangeName string, duration time.Duration) (MetricRange, error) {
	to := s.options.Clock()
	from := to.Add(-duration)
	step := rangeStep(duration)
	metricRange := MetricRange{
		HostID: id,
		Range:  rangeName,
		From:   from,
		To:     to,
		Step:   step,
		Series: make([]MetricSeries, 0, len(rangeMetrics)),
	}
	for _, metric := range rangeMetrics {
		series, err := s.provider.QueryRange(ctx, datasource.RangeRequest{
			HostIDs: []string{id},
			Metric:  metric,
			From:    from,
			To:      to,
			Step:    step,
		})
		if err != nil {
			return MetricRange{}, err
		}
		metricSeries := MetricSeries{Metric: metric, Points: []MetricPoint{}}
		for _, candidate := range series {
			if candidate.HostID != id || candidate.Metric != metric {
				continue
			}
			metricSeries.Points = normalizePoints(candidate.Points)
			break
		}
		metricRange.Series = append(metricRange.Series, metricSeries)
	}
	return metricRange, nil
}

func namedRange(name string) (time.Duration, bool) {
	switch name {
	case "1h":
		return time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func rangeStep(duration time.Duration) time.Duration {
	intervals := int64(maxRangePoints - 1)
	step := duration / time.Duration(intervals)
	if duration%time.Duration(intervals) != 0 {
		step++
	}
	return step
}

func normalizePoints(points []datasource.Point) []MetricPoint {
	if len(points) <= maxRangePoints {
		result := make([]MetricPoint, len(points))
		for i, point := range points {
			result[i] = MetricPoint{Timestamp: point.Timestamp, Value: cloneFloat(point.Value)}
		}
		return result
	}
	result := make([]MetricPoint, maxRangePoints)
	last := len(points) - 1
	for i := range result {
		index := i * last / (maxRangePoints - 1)
		point := points[index]
		result[i] = MetricPoint{Timestamp: point.Timestamp, Value: cloneFloat(point.Value)}
	}
	return result
}

func cloneMetricRange(metricRange MetricRange) MetricRange {
	clone := metricRange
	clone.Series = make([]MetricSeries, len(metricRange.Series))
	for i, series := range metricRange.Series {
		clone.Series[i] = MetricSeries{
			Metric: series.Metric,
			Points: make([]MetricPoint, len(series.Points)),
		}
		for j, point := range series.Points {
			clone.Series[i].Points[j] = MetricPoint{
				Timestamp: point.Timestamp,
				Value:     cloneFloat(point.Value),
			}
		}
	}
	return clone
}
