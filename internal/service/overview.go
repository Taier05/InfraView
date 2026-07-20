package service

import (
	"context"

	"github.com/Taier05/InfraView/internal/datasource"
)

func (s *Service) Overview(ctx context.Context, rangeName string) (Overview, Meta, error) {
	if _, ok := namedRange(rangeName); !ok {
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

	overview := Overview{Total: len(hosts)}
	var cpuTotal, memoryTotal float64
	var cpuCount, memoryCount int
	for _, host := range hosts {
		switch host.Status {
		case datasource.StatusOnline:
			overview.Online++
		case datasource.StatusOffline:
			overview.Offline++
		default:
			overview.Unknown++
		}
		current := metrics[host.ID]
		if current.CPUUsage != nil {
			cpuTotal += *current.CPUUsage
			cpuCount++
		}
		if current.MemoryUsage != nil {
			memoryTotal += *current.MemoryUsage
			memoryCount++
		}
	}
	overview.CPUAverage = s.average(cpuTotal, cpuCount)
	overview.MemoryAverage = s.average(memoryTotal, memoryCount)
	return overview, mergeMeta(inventoryMeta, metricsMeta), nil
}

func (s *Service) average(total float64, count int) MetricValue {
	if count == 0 {
		return MetricValue{Level: LevelUnknown}
	}
	average := total / float64(count)
	return s.percentage(&average)
}
