package mock

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

type provider struct {
	hostCount int
	clock     func() time.Time
}

func New(hostCount int, clock func() time.Time) datasource.Provider {
	if hostCount < 0 {
		hostCount = 0
	}
	if clock == nil {
		clock = time.Now
	}
	return &provider{hostCount: hostCount, clock: clock}
}

func (p *provider) Health(context.Context) (datasource.Health, error) {
	return datasource.Health{Healthy: true, CheckedAt: p.now()}, nil
}

func (p *provider) ListHosts(context.Context) ([]datasource.Host, error) {
	hosts := make([]datasource.Host, p.hostCount)
	now := p.now()
	for i := range hosts {
		hosts[i] = buildHost(i+1, now)
	}
	return hosts, nil
}

func (p *provider) GetHost(_ context.Context, hostID string) (datasource.Host, error) {
	index, ok := p.hostIndex(hostID)
	if !ok {
		return datasource.Host{}, datasource.ErrNotFound
	}
	return buildHost(index, p.now()), nil
}

func (p *provider) GetCurrentMetrics(_ context.Context, hostIDs []string) (map[string]datasource.CurrentMetrics, error) {
	indices := make([]int, len(hostIDs))
	for i, hostID := range hostIDs {
		index, ok := p.hostIndex(hostID)
		if !ok {
			return nil, datasource.ErrNotFound
		}
		indices[i] = index
	}

	now := p.now()
	metrics := make(map[string]datasource.CurrentMetrics, len(hostIDs))
	for i, hostID := range hostIDs {
		metrics[hostID] = currentMetrics(indices[i], now)
	}
	return metrics, nil
}

func (p *provider) QueryRange(_ context.Context, request datasource.RangeRequest) ([]datasource.Series, error) {
	indices := make([]int, len(request.HostIDs))
	for i, hostID := range request.HostIDs {
		index, ok := p.hostIndex(hostID)
		if !ok {
			return nil, datasource.ErrNotFound
		}
		indices[i] = index
	}
	if request.Step <= 0 || request.To.Before(request.From) {
		return []datasource.Series{}, nil
	}

	pointCount := int(request.To.Sub(request.From)/request.Step) + 1
	series := make([]datasource.Series, len(request.HostIDs))
	for i, hostID := range request.HostIDs {
		points := make([]datasource.Point, 0, pointCount)
		for timestamp := request.From; !timestamp.After(request.To); timestamp = timestamp.Add(request.Step) {
			value := metricValue(request.Metric, indices[i], timestamp)
			points = append(points, datasource.Point{Timestamp: timestamp, Value: value})
		}
		series[i] = datasource.Series{HostID: hostID, Metric: request.Metric, Points: points}
	}
	return series, nil
}

func (p *provider) QueryAggregateRange(_ context.Context, request datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	if request.Step <= 0 || request.End.Before(request.Start) {
		return []datasource.Series{}, nil
	}

	hostCount := min(p.hostCount, 100)
	pointCount := int(request.End.Sub(request.Start)/request.Step) + 1
	series := make([]datasource.Series, len(request.Keys))
	for keyIndex, key := range request.Keys {
		points := make([]datasource.Point, 0, pointCount)
		for timestamp := request.Start; !timestamp.After(request.End); timestamp = timestamp.Add(request.Step) {
			var total float64
			count := 0
			for hostIndex := 1; hostIndex <= hostCount; hostIndex++ {
				value := metricValue(key, hostIndex, timestamp)
				if value == nil {
					continue
				}
				total += *value
				count++
			}
			var average *float64
			if count > 0 {
				value := total / float64(count)
				average = &value
			}
			points = append(points, datasource.Point{Timestamp: timestamp, Value: average})
		}
		series[keyIndex] = datasource.Series{Metric: key, Points: points}
	}
	return series, nil
}

func (p *provider) hostIndex(hostID string) (int, bool) {
	var index int
	if _, err := fmt.Sscanf(hostID, "mock-host-%03d", &index); err != nil {
		return 0, false
	}
	if index < 1 || index > p.hostCount || hostID != fmt.Sprintf("mock-host-%03d", index) {
		return 0, false
	}
	return index, true
}

func (p *provider) now() time.Time {
	return p.clock().UTC()
}

func buildHost(index int, now time.Time) datasource.Host {
	status := datasource.StatusOnline
	if index%17 == 0 {
		status = datasource.StatusOffline
	}
	return datasource.Host{
		ID:         fmt.Sprintf("mock-host-%03d", index),
		Name:       fmt.Sprintf("linux-%03d", index),
		IP:         fmt.Sprintf("192.0.2.%d", ((index-1)%254)+1),
		OS:         "linux",
		Status:     status,
		StatusTime: now,
		Uptime:     time.Duration(24+index) * time.Hour,
	}
}

func currentMetrics(index int, timestamp time.Time) datasource.CurrentMetrics {
	cpu := percentageValue(datasource.MetricCPUUsage, index, timestamp)
	memory := percentageValue(datasource.MetricMemoryUsage, index, timestamp)
	load := scalarValue(datasource.MetricLoad1, index, timestamp)
	diskUsage := percentageValue(datasource.MetricDiskUsage, index, timestamp)
	diskRead := scalarValue(datasource.MetricDiskReadBytesPerSecond, index, timestamp)
	diskWrite := scalarValue(datasource.MetricDiskWriteBytesPerSecond, index, timestamp)
	networkReceive := scalarValue(datasource.MetricNetworkReceiveBytesPerSecond, index, timestamp)
	networkTransmit := scalarValue(datasource.MetricNetworkTransmitBytesPerSecond, index, timestamp)

	return datasource.CurrentMetrics{
		Timestamp:                     timestamp,
		CPUUsage:                      &cpu,
		MemoryUsage:                   &memory,
		Load1:                         &load,
		DiskReadBytesPerSecond:        &diskRead,
		DiskWriteBytesPerSecond:       &diskWrite,
		NetworkReceiveBytesPerSecond:  &networkReceive,
		NetworkTransmitBytesPerSecond: &networkTransmit,
		Filesystems: []datasource.FilesystemMetrics{
			{Mountpoint: "/", Usage: &diskUsage},
		},
	}
}

func metricValue(metric datasource.MetricKey, index int, timestamp time.Time) *float64 {
	var value float64
	switch metric {
	case datasource.MetricCPUUsage, datasource.MetricMemoryUsage, datasource.MetricDiskUsage:
		value = percentageValue(metric, index, timestamp)
	case datasource.MetricLoad1,
		datasource.MetricDiskReadBytesPerSecond,
		datasource.MetricDiskWriteBytesPerSecond,
		datasource.MetricNetworkReceiveBytesPerSecond,
		datasource.MetricNetworkTransmitBytesPerSecond:
		value = scalarValue(metric, index, timestamp)
	default:
		return nil
	}
	return &value
}

func percentageValue(metric datasource.MetricKey, index int, timestamp time.Time) float64 {
	wave := wave(index, timestamp)
	var value float64
	switch metric {
	case datasource.MetricCPUUsage:
		value = 15 + float64(index%10) + 55*wave
	case datasource.MetricMemoryUsage:
		value = 30 + float64(index%15) + 35*wave
	case datasource.MetricDiskUsage:
		value = 40 + float64(index%20) + 20*wave
	}
	return math.Max(0, math.Min(100, value))
}

func scalarValue(metric datasource.MetricKey, index int, timestamp time.Time) float64 {
	wave := wave(index, timestamp)
	switch metric {
	case datasource.MetricLoad1:
		return 0.1 + float64(index%8)*0.15 + 2.5*wave
	case datasource.MetricDiskReadBytesPerSecond:
		return 1024 * (20 + float64(index%50) + 200*wave)
	case datasource.MetricDiskWriteBytesPerSecond:
		return 1024 * (10 + float64(index%30) + 120*wave)
	case datasource.MetricNetworkReceiveBytesPerSecond:
		return 1024 * (50 + float64(index%70) + 500*wave)
	case datasource.MetricNetworkTransmitBytesPerSecond:
		return 1024 * (30 + float64(index%60) + 350*wave)
	default:
		return 0
	}
}

func wave(index int, timestamp time.Time) float64 {
	const periodSeconds = int64(6 * time.Hour / time.Second)
	position := (timestamp.Unix() + int64(index*137)) % periodSeconds
	angle := 2 * math.Pi * float64(position) / float64(periodSeconds)
	return (math.Sin(angle) + 1) / 2
}

var _ datasource.Provider = (*provider)(nil)
