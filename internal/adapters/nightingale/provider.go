package nightingale

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/disk"
	"github.com/Taier05/InfraView/internal/mysql"
)

const (
	defaultMaxResponseBytes     int64 = 8 << 20
	defaultInterfaceExcludeExpr       = `lo|docker.*|veth.*|cali.*|br-.*|tunl.*`
	targetPageSize                    = 100
)

type Options struct {
	BaseURL              string
	Token                string
	InterfaceExcludeExpr string
	HTTPClient           *http.Client
	Clock                func() time.Time
	MaxResponseBytes     int64
	AllowInsecureHTTP    bool
}

type Provider struct {
	baseURL              *url.URL
	token                string
	interfaceExcludeExpr string
	httpClient           *http.Client
	clock                func() time.Time
	maxResponseBytes     int64
	configErr            error

	datasourceMu  sync.Mutex
	datasourceID  int64
	datasourceFly *datasourceDiscovery
}

func New(option Options) *Provider {
	provider := &Provider{configErr: datasource.ErrNotConfigured}
	baseURL, err := url.Parse(strings.TrimSpace(option.BaseURL))
	if err != nil || !baseURL.IsAbs() || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" || strings.TrimSpace(option.Token) == "" {
		return provider
	}
	if baseURL.Scheme == "http" && !option.AllowInsecureHTTP {
		return provider
	}

	excludeExpr := strings.TrimSpace(option.InterfaceExcludeExpr)
	if excludeExpr == "" {
		excludeExpr = defaultInterfaceExcludeExpr
	}
	if _, err := regexp.Compile(excludeExpr); err != nil {
		return provider
	}

	client := option.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	clock := option.Clock
	if clock == nil {
		clock = time.Now
	}
	maxResponseBytes := option.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}

	provider.baseURL = baseURL
	provider.token = strings.TrimSpace(option.Token)
	provider.interfaceExcludeExpr = excludeExpr
	provider.httpClient = &clientCopy
	provider.clock = clock
	provider.maxResponseBytes = maxResponseBytes
	provider.configErr = nil
	return provider
}

func (p *Provider) Health(ctx context.Context) (datasource.Health, error) {
	if err := p.ready(); err != nil {
		return datasource.Health{}, err
	}
	var profile map[string]any
	if err := p.get(ctx, "/api/n9e/self/profile", nil, &profile); err != nil {
		return datasource.Health{}, err
	}
	return datasource.Health{Healthy: true, CheckedAt: p.now()}, nil
}

func (p *Provider) ListHosts(ctx context.Context) ([]datasource.Host, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	records, err := p.listTargetRecords(ctx)
	if err != nil {
		return nil, err
	}
	hosts, ids, err := mapTargetRecords(records)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return hosts, nil
	}

	results, err := p.queryInstant(ctx, []string{
		inventoryPromQL("mem_total", ids),
		inventoryPromQL("system_uptime", ids),
	})
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(hosts))
	for i := range hosts {
		index[hosts[i].ID] = i
	}
	if len(results) > 0 {
		for _, sample := range results[0] {
			hostIndex, ok := index[sample.Metric["ident"]]
			if !ok {
				continue
			}
			value, _, ok := parseInstantValue(sample.Value)
			memory, ok := roundedFiniteInt64(value)
			if !ok || memory <= 0 {
				continue
			}
			hosts[hostIndex].MemoryTotalBytes = &memory
		}
	}
	if len(results) > 1 {
		for _, sample := range results[1] {
			hostIndex, ok := index[sample.Metric["ident"]]
			if !ok {
				continue
			}
			value, _, ok := parseInstantValue(sample.Value)
			uptime, ok := durationFromSeconds(value)
			if !ok {
				continue
			}
			hosts[hostIndex].Uptime = uptime
		}
	}
	return hosts, nil
}

func (p *Provider) GetHost(ctx context.Context, hostID string) (datasource.Host, error) {
	hosts, err := p.ListHosts(ctx)
	if err != nil {
		return datasource.Host{}, err
	}
	for _, host := range hosts {
		if host.ID == hostID {
			return host, nil
		}
	}
	return datasource.Host{}, datasource.ErrNotFound
}

func (p *Provider) GetCurrentMetrics(ctx context.Context, hostIDs []string) (map[string]datasource.CurrentMetrics, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	metrics := make(map[string]datasource.CurrentMetrics, len(hostIDs))
	for _, hostID := range hostIDs {
		metrics[hostID] = datasource.CurrentMetrics{}
	}
	if len(hostIDs) == 0 {
		return metrics, nil
	}

	queries := currentPromQL(hostIDs, p.interfaceExcludeExpr)
	results, err := p.queryInstant(ctx, queries)
	if err != nil {
		return nil, err
	}
	for queryIndex, series := range results {
		if queryIndex >= len(queries) {
			break
		}
		for _, sample := range series {
			hostID := sample.Metric["ident"]
			current, ok := metrics[hostID]
			if !ok {
				continue
			}
			value, _, ok := parseInstantValue(sample.Value)
			if !ok {
				continue
			}
			if queryIndex == currentCollectionTimestampQueryIndex {
				timestamp, ok := parseUnixTime(sample.Value[1])
				if ok && timestamp.After(current.Timestamp) {
					current.Timestamp = timestamp
				}
				metrics[hostID] = current
				continue
			}
			setCurrentMetric(&current, queryIndex, value)
			metrics[hostID] = current
		}
	}
	return metrics, nil
}

func (p *Provider) QueryRange(ctx context.Context, request datasource.RangeRequest) ([]datasource.Series, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	if request.Step <= 0 || request.To.Before(request.From) {
		return []datasource.Series{}, nil
	}
	series := make([]datasource.Series, len(request.HostIDs))
	seriesIndex := make(map[string]int, len(request.HostIDs))
	for i, hostID := range request.HostIDs {
		series[i] = datasource.Series{HostID: hostID, Metric: request.Metric, Points: []datasource.Point{}}
		seriesIndex[hostID] = i
	}
	if len(request.HostIDs) == 0 {
		return series, nil
	}

	query, ok := rangePromQL(request.Metric, request.HostIDs, p.interfaceExcludeExpr)
	if !ok {
		return series, nil
	}
	results, err := p.queryRange(ctx, []rangeQuery{{
		Start: request.From.Unix(),
		End:   request.To.Unix(),
		Step:  durationSeconds(request.Step),
		Query: query,
	}})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return series, nil
	}
	for _, candidate := range results[0] {
		index, ok := seriesIndex[candidate.Metric["ident"]]
		if !ok {
			continue
		}
		series[index].Points = parseRangePoints(candidate.Values)
	}
	return series, nil
}

func (p *Provider) QueryAggregateRange(ctx context.Context, request datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	if err := p.ready(); err != nil {
		return nil, err
	}
	if request.Step <= 0 || request.End.Before(request.Start) {
		return []datasource.Series{}, nil
	}
	series := make([]datasource.Series, len(request.Keys))
	queries := make([]rangeQuery, 0, len(request.Keys))
	queryIndexes := make([]int, 0, len(request.Keys))
	for i, key := range request.Keys {
		series[i] = datasource.Series{Metric: key, Points: []datasource.Point{}}
		query, ok := aggregatePromQL(key, p.interfaceExcludeExpr)
		if !ok {
			continue
		}
		queries = append(queries, rangeQuery{
			Start: request.Start.Unix(),
			End:   request.End.Unix(),
			Step:  durationSeconds(request.Step),
			Query: query,
		})
		queryIndexes = append(queryIndexes, i)
	}
	if len(queries) == 0 {
		return series, nil
	}

	results, err := p.queryRange(ctx, queries)
	if err != nil {
		return nil, err
	}
	for resultIndex, candidates := range results {
		if resultIndex >= len(queryIndexes) || len(candidates) == 0 {
			continue
		}
		series[queryIndexes[resultIndex]].Points = parseRangePoints(candidates[0].Values)
	}
	return series, nil
}

func (p *Provider) listTargetRecords(ctx context.Context) ([]targetRecord, error) {
	records := make([]targetRecord, 0, targetPageSize)
	wantTotal := -1
	for page := 1; ; page++ {
		var result targetPage
		query := url.Values{
			"limit": {fmt.Sprint(targetPageSize)},
			"p":     {fmt.Sprint(page)},
		}
		if err := p.get(ctx, "/api/n9e/targets", query, &result); err != nil {
			return nil, err
		}
		if result.List == nil || result.Total == nil {
			return nil, unavailableError()
		}
		list := *result.List
		total := *result.Total
		if total < 0 || (wantTotal >= 0 && total != wantTotal) {
			return nil, unavailableError()
		}
		wantTotal = total
		if len(list) == 0 && len(records) < wantTotal {
			return nil, unavailableError()
		}
		records = append(records, list...)
		if len(records) >= wantTotal {
			if len(records) != wantTotal {
				return nil, unavailableError()
			}
			return records, nil
		}
		if page >= 10000 {
			return nil, unavailableError()
		}
	}
}

func mapTargetRecords(records []targetRecord) ([]datasource.Host, []string, error) {
	hosts := make([]datasource.Host, 0, len(records))
	ids := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Ident) == "" {
			return nil, nil, unavailableError()
		}
		if _, exists := seen[record.Ident]; exists {
			return nil, nil, unavailableError()
		}
		seen[record.Ident] = struct{}{}
		host := datasource.Host{
			ID:     record.Ident,
			Name:   record.Ident,
			IP:     record.HostIP,
			OS:     record.OS,
			Status: targetStatus(record.TargetUp),
		}
		host.StatusTime = targetStatusTime(record)
		if record.CPUNum > 0 {
			cpuCores := record.CPUNum
			host.CPUCores = &cpuCores
		}
		hosts = append(hosts, host)
		ids = append(ids, record.Ident)
	}
	return hosts, ids, nil
}

func targetStatusTime(record targetRecord) time.Time {
	for _, candidate := range []int64{record.BeatTime, record.UpdateAt} {
		if candidate <= 0 {
			continue
		}
		if statusTime, ok := jsonUnixTime(candidate, 0); ok {
			return statusTime
		}
	}
	return time.Time{}
}

func targetStatus(value int) datasource.HostStatus {
	switch value {
	case 2:
		return datasource.StatusOnline
	case 0:
		return datasource.StatusOffline
	default:
		return datasource.StatusUnknown
	}
}

func setCurrentMetric(current *datasource.CurrentMetrics, queryIndex int, value float64) {
	copy := value
	switch queryIndex {
	case 0:
		current.CPUUsage = &copy
	case 1:
		current.MemoryUsage = &copy
	case 2:
		current.Load1 = &copy
	case 3:
		current.IOBusyPercent = &copy
	case 4:
		current.NetworkTransmitBytesPerSecond = &copy
	case 5:
		current.NetworkReceiveBytesPerSecond = &copy
	}
}

func durationSeconds(value time.Duration) int64 {
	seconds := int64(math.Ceil(value.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

const int64ExclusiveUpperBound = 1 << 63

func finiteInt64(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < -int64ExclusiveUpperBound || value >= int64ExclusiveUpperBound {
		return 0, false
	}
	return int64(value), true
}

func roundedFiniteInt64(value float64) (int64, bool) {
	return finiteInt64(math.Round(value))
}

func durationFromSeconds(value float64) (time.Duration, bool) {
	if value < 0 {
		return 0, false
	}
	nanoseconds, ok := finiteInt64(value * float64(time.Second))
	if !ok || nanoseconds < 0 {
		return 0, false
	}
	return time.Duration(nanoseconds), true
}

func (p *Provider) ready() error {
	if p == nil || p.configErr != nil {
		return datasource.ErrNotConfigured
	}
	return nil
}

func (p *Provider) now() time.Time {
	return p.clock().UTC()
}

func unavailableError() error {
	return fmt.Errorf("%w: Nightingale 上游请求失败", datasource.ErrUnavailable)
}

var _ datasource.Provider = (*Provider)(nil)
var _ disk.Provider = (*Provider)(nil)
var _ mysql.Provider = (*Provider)(nil)

func sortDatasourceRecords(records []datasourceRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].IsDefault != records[j].IsDefault {
			return records[i].IsDefault
		}
		return records[i].ID < records[j].ID
	})
}
