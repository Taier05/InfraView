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
		if query.Status != "" && host.Status != query.Status {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(host.Name), search) && !strings.Contains(strings.ToLower(host.IP), search) {
			continue
		}
		items = append(items, s.hostSummary(host, metrics[host.ID]))
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
	detail := HostDetail{
		ID:         host.ID,
		Name:       host.Name,
		IP:         host.IP,
		OS:         host.OS,
		Status:     host.Status,
		StatusTime: host.StatusTime,
		Uptime:     host.Uptime,
		Metrics:    s.currentView(metrics[id]),
	}
	return detail, mergeMeta(hostMeta, metricsMeta), nil
}

func (s *Service) hostSummary(host datasource.Host, metrics datasource.CurrentMetrics) HostSummary {
	return HostSummary{
		ID:         host.ID,
		Name:       host.Name,
		IP:         host.IP,
		OS:         host.OS,
		Status:     host.Status,
		StatusTime: host.StatusTime,
		Uptime:     host.Uptime,
		Metrics:    s.currentView(metrics),
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
	case "id", "name", "ip", "status", "cpu", "cpu_usage", "memory", "memory_usage", "load", "uptime":
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
		if field == "load" {
			leftMissing := hosts[i].Metrics.Load1.Value == nil
			rightMissing := hosts[j].Metrics.Load1.Value == nil
			if leftMissing != rightMissing {
				return !leftMissing
			}
		}
		comparison := compareHosts(hosts[i], hosts[j], field)
		if comparison == 0 {
			return hosts[i].ID < hosts[j].ID
		}
		if order == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
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
		return strings.Compare(string(left.Status), string(right.Status))
	case "cpu", "cpu_usage":
		return compareMetricValues(left.Metrics.CPUUsage.Value, right.Metrics.CPUUsage.Value)
	case "memory", "memory_usage":
		return compareMetricValues(left.Metrics.MemoryUsage.Value, right.Metrics.MemoryUsage.Value)
	case "load":
		return compareMetricValues(left.Metrics.Load1.Value, right.Metrics.Load1.Value)
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
