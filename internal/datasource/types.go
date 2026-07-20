package datasource

import (
	"errors"
	"time"
)

type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusUnknown Status = "unknown"
)

type MetricKey string

const (
	MetricCPUUsage                      MetricKey = "cpu_usage"
	MetricMemoryUsage                   MetricKey = "memory_usage"
	MetricLoad1                         MetricKey = "load_1"
	MetricDiskUsage                     MetricKey = "disk_usage"
	MetricDiskReadBytesPerSecond        MetricKey = "disk_read_bytes_per_second"
	MetricDiskWriteBytesPerSecond       MetricKey = "disk_write_bytes_per_second"
	MetricNetworkReceiveBytesPerSecond  MetricKey = "network_receive_bytes_per_second"
	MetricNetworkTransmitBytesPerSecond MetricKey = "network_transmit_bytes_per_second"
)

type Health struct {
	Healthy   bool
	CheckedAt time.Time
}

type Host struct {
	ID         string
	Name       string
	IP         string
	OS         string
	Status     Status
	StatusTime time.Time
	Uptime     time.Duration
}

type FilesystemMetrics struct {
	Mountpoint string
	Usage      *float64
}

type CurrentMetrics struct {
	Timestamp                     time.Time
	CPUUsage                      *float64
	MemoryUsage                   *float64
	Load1                         *float64
	DiskReadBytesPerSecond        *float64
	DiskWriteBytesPerSecond       *float64
	NetworkReceiveBytesPerSecond  *float64
	NetworkTransmitBytesPerSecond *float64
	Filesystems                   []FilesystemMetrics
}

type Point struct {
	Timestamp time.Time
	Value     *float64
}

type Series struct {
	HostID string
	Metric MetricKey
	Points []Point
}

type RangeRequest struct {
	HostIDs []string
	Metric  MetricKey
	From    time.Time
	To      time.Time
	Step    time.Duration
}

var (
	ErrNotFound      = errors.New("data source: not found")
	ErrUnavailable   = errors.New("data source: unavailable")
	ErrNotConfigured = errors.New("data source: not configured")
)
