package service

import (
	"errors"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

var (
	ErrNotFound     = errors.New("service: not found")
	ErrInvalidQuery = errors.New("service: invalid query")
	ErrInvalidRange = errors.New("service: invalid range")
)

type Level string

const (
	LevelNormal   Level = "normal"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
	LevelUnknown  Level = "unknown"
)

type Options struct {
	InventoryTTL      time.Duration
	CurrentMetricsTTL time.Duration
	RangeTTL          time.Duration
	HealthTTL         time.Duration
	MaxStale          time.Duration
	WarningPercent    float64
	CriticalPercent   float64
	Clock             func() time.Time
}

type Meta struct {
	Stale       bool
	CollectedAt time.Time
}

type HostQuery struct {
	Search   string
	Status   datasource.HostStatus
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type MetricValue struct {
	Value *float64
	Level Level
}

type Filesystem struct {
	Mountpoint string
	Usage      MetricValue
}

type CurrentMetrics struct {
	Timestamp                     time.Time
	CPUUsage                      MetricValue
	MemoryUsage                   MetricValue
	Load1                         MetricValue
	DiskReadBytesPerSecond        MetricValue
	DiskWriteBytesPerSecond       MetricValue
	NetworkReceiveBytesPerSecond  MetricValue
	NetworkTransmitBytesPerSecond MetricValue
	Filesystems                   []Filesystem
}

type Overview struct {
	Total         int
	Online        int
	Offline       int
	Unknown       int
	CPUAverage    MetricValue
	MemoryAverage MetricValue
}

type HostSummary struct {
	ID         string
	Name       string
	IP         string
	OS         string
	Status     datasource.HostStatus
	StatusTime time.Time
	Uptime     time.Duration
	Metrics    CurrentMetrics
}

type HostPage struct {
	Hosts    []HostSummary
	Total    int
	Page     int
	PageSize int
}

type HostDetail struct {
	ID         string
	Name       string
	IP         string
	OS         string
	Status     datasource.HostStatus
	StatusTime time.Time
	Uptime     time.Duration
	Metrics    CurrentMetrics
}

type MetricPoint struct {
	Timestamp time.Time
	Value     *float64
}

type MetricSeries struct {
	Metric datasource.MetricKey
	Points []MetricPoint
}

type MetricRange struct {
	HostID string
	Range  string
	From   time.Time
	To     time.Time
	Step   time.Duration
	Series []MetricSeries
}

type DataSourceStatus struct {
	Healthy   bool
	CheckedAt time.Time
}
