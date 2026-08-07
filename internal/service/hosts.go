package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Taier05/InfraView/internal/datasource"
)

func (s *Service) Hosts(ctx context.Context, query HostQuery) (HostPage, Meta, error) {
	query, err := normalizeHostQuery(query)
	if err != nil {
		return HostPage{}, Meta{}, err
	}
	hosts, inventoryMeta, err := s.inventory(ctx)
	if err != nil {
		return HostPage{}, Meta{}, err
	}
	ids := make([]string, len(hosts))
	for i, host := range hosts {
		ids[i] = host.ID
	}
	metrics, metricsMeta, err := s.currentMetrics(ctx, ids)
	if err != nil {
		return HostPage{}, Meta{}, err
	}

	search := strings.ToLower(strings.TrimSpace(query.Search))
	items := make([]HostSummary, 0, len(hosts))
	for _, host := range hosts {
		if search != "" && !strings.Contains(strings.ToLower(host.Name), search) && !strings.Contains(strings.ToLower(host.IP), search) {
			continue
		}
		summary := s.hostSummary(host, metrics[host.ID])
		if query.Status != "" && summary.Status != query.Status {
			continue
		}
		items = append(items, summary)
	}
	sortHosts(items, query.Sort, query.Order)

	total := len(items)
	start := total
	if query.Page-1 <= total/query.PageSize {
		start = (query.Page - 1) * query.PageSize
		if start > total {
			start = total
		}
	}
	end := min(start+query.PageSize, total)
	pageItems := append([]HostSummary(nil), items[start:end]...)
	return HostPage{
		Hosts:    pageItems,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, mergeMeta(inventoryMeta, metricsMeta), nil
}

func (s *Service) Host(ctx context.Context, id string) (HostDetail, Meta, error) {
	host, hostMeta, err := s.host(ctx, id)
	if err != nil {
		return HostDetail{}, Meta{}, err
	}
	metrics, metricsMeta, err := s.currentMetrics(ctx, []string{id})
	if err != nil {
		return HostDetail{}, Meta{}, err
	}
	collectionLevel := s.hostCollectionLevel(id, metrics[id])
	detail := HostDetail{
		ID:               host.ID,
		Name:             host.Name,
		IP:               host.IP,
		OS:               host.OS,
		CPUCores:         host.CPUCores,
		MemoryTotalBytes: host.MemoryTotalBytes,
		Status:           effectiveHostStatus(host.Status, collectionLevel),
		StatusTime:       host.StatusTime,
		Uptime:           host.Uptime,
		Metrics:          s.currentView(metrics[id]),
		CollectionLevel:  collectionLevel,
	}
	return detail, mergeMeta(hostMeta, metricsMeta), nil
}

func (s *Service) hostSummary(host datasource.Host, metrics datasource.CurrentMetrics) HostSummary {
	collectionLevel := s.hostCollectionLevel(host.ID, metrics)
	return HostSummary{
		ID:               host.ID,
		Name:             host.Name,
		IP:               host.IP,
		OS:               host.OS,
		CPUCores:         host.CPUCores,
		MemoryTotalBytes: host.MemoryTotalBytes,
		Status:           effectiveHostStatus(host.Status, collectionLevel),
		StatusTime:       host.StatusTime,
		Uptime:           host.Uptime,
		Metrics:          s.currentView(metrics),
		CollectionLevel:  collectionLevel,
	}
}

func normalizeHostQuery(query HostQuery) (HostQuery, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return HostQuery{}, fmt.Errorf("%w: page must be positive and page size must be between 1 and 100", ErrInvalidQuery)
	}
	if query.Status != "" && query.Status != datasource.StatusOnline && query.Status != datasource.StatusOffline && query.Status != datasource.StatusUnknown {
		return HostQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = "name"
	}
	switch query.Sort {
	case "id", "name", "ip", "cpu_cores", "memory_total", "cpu", "cpu_usage", "memory", "memory_usage", "load", "io", "io_busy_percent", "network", "network_transmit", "network_receive", "uptime", "status":
	default:
		return HostQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return HostQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

func sortHosts(hosts []HostSummary, field, order string) {
	sort.SliceStable(hosts, func(i, j int) bool {
		leftInteger, leftAvailable, integerSort := hostIntegerSortValue(hosts[i], field)
		rightInteger, rightAvailable, _ := hostIntegerSortValue(hosts[j], field)
		if integerSort && leftAvailable != rightAvailable {
			return leftAvailable
		}
		comparison := 0
		if integerSort && leftAvailable {
			comparison = compareInt64(leftInteger, rightInteger)
		} else {
			leftMetric, leftAvailable, metricSort := hostMetricSortValue(hosts[i], field)
			rightMetric, rightAvailable, _ := hostMetricSortValue(hosts[j], field)
			if metricSort && leftAvailable != rightAvailable {
				return leftAvailable
			}
			if metricSort && leftAvailable {
				comparison = compareFloat64(leftMetric, rightMetric)
			} else {
				comparison = compareHosts(hosts[i], hosts[j], field)
			}
		}
		if comparison == 0 {
			return hosts[i].ID < hosts[j].ID
		}
		if order == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func hostIntegerSortValue(host HostSummary, field string) (int64, bool, bool) {
	switch field {
	case "cpu_cores":
		if host.CPUCores == nil {
			return 0, false, true
		}
		return int64(*host.CPUCores), true, true
	case "memory_total":
		if host.MemoryTotalBytes == nil {
			return 0, false, true
		}
		return *host.MemoryTotalBytes, true, true
	default:
		return 0, false, false
	}
}

func hostMetricSortValue(host HostSummary, field string) (float64, bool, bool) {
	switch field {
	case "cpu", "cpu_usage":
		return metricSortValue(host.Metrics.CPUUsage.Value)
	case "memory", "memory_usage":
		return metricSortValue(host.Metrics.MemoryUsage.Value)
	case "load":
		return metricSortValue(host.Metrics.Load1.Value)
	case "io", "io_busy_percent":
		return metricSortValue(host.Metrics.IOBusyPercent.Value)
	case "network":
		var total float64
		available := false
		if value := host.Metrics.NetworkTransmitBytesPerSecond.Value; value != nil {
			total += *value
			available = true
		}
		if value := host.Metrics.NetworkReceiveBytesPerSecond.Value; value != nil {
			total += *value
			available = true
		}
		return total, available, true
	case "network_transmit":
		return metricSortValue(host.Metrics.NetworkTransmitBytesPerSecond.Value)
	case "network_receive":
		return metricSortValue(host.Metrics.NetworkReceiveBytesPerSecond.Value)
	default:
		return 0, false, false
	}
}

func metricSortValue(value *float64) (float64, bool, bool) {
	if value == nil {
		return 0, false, true
	}
	return *value, true, true
}

func compareFloat64(left, right float64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareHosts(left, right HostSummary, field string) int {
	switch field {
	case "id":
		return strings.Compare(left.ID, right.ID)
	case "name":
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	case "ip":
		return strings.Compare(left.IP, right.IP)
	case "status":
		return compareInt64(int64(hostStatusRank(left)), int64(hostStatusRank(right)))
	case "cpu", "cpu_usage":
		return compareMetricValues(left.Metrics.CPUUsage.Value, right.Metrics.CPUUsage.Value)
	case "memory", "memory_usage":
		return compareMetricValues(left.Metrics.MemoryUsage.Value, right.Metrics.MemoryUsage.Value)
	case "load":
		return compareMetricValues(left.Metrics.Load1.Value, right.Metrics.Load1.Value)
	case "io", "io_busy_percent":
		return compareMetricValues(left.Metrics.IOBusyPercent.Value, right.Metrics.IOBusyPercent.Value)
	case "uptime":
		if left.Uptime < right.Uptime {
			return -1
		}
		if left.Uptime > right.Uptime {
			return 1
		}
	}
	return 0
}

func hostStatusRank(host HostSummary) int {
	if host.CollectionLevel == LevelWarning {
		return 1
	}
	if host.CollectionLevel == LevelCritical {
		return 2
	}
	switch host.Status {
	case datasource.StatusOnline:
		return listLevelSortRank(LevelNormal)
	case datasource.StatusOffline:
		return listLevelSortRank(LevelCritical)
	default:
		return listLevelSortRank(LevelUnknown)
	}
}

func compareMetricValues(left, right *float64) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	if *left < *right {
		return -1
	}
	if *left > *right {
		return 1
	}
	return 0
}
