package service

import "time"

type JavaOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type JavaQuery struct {
	Search   string
	Name     string
	Sort     string
	Order    string
	Status   Level
	Page     int
	PageSize int
}

type JavaStatusSource string

const (
	JavaStatusHealth      JavaStatusSource = "health"
	JavaStatusPort        JavaStatusSource = "port"
	JavaStatusProcess     JavaStatusSource = "process"
	JavaStatusConsistency JavaStatusSource = "consistency"
	JavaStatusCollection  JavaStatusSource = "collection"
	JavaStatusNormal      JavaStatusSource = "normal"
	JavaStatusUnknown     JavaStatusSource = "unknown"
)

type JavaLevelCounts struct {
	Total    int
	Normal   int
	Warning  int
	Critical int
	Unknown  int
}

type JavaAlertCount struct {
	Warning  int
	Critical int
	Unknown  int
}

type JavaOverviewAlerts struct {
	Health     JavaAlertCount
	Port       JavaAlertCount
	Process    JavaAlertCount
	Collection JavaAlertCount
}

type JavaOverview struct {
	Status   Level
	Services JavaLevelCounts
	Alerts   JavaOverviewAlerts
}

type JavaServiceSummary struct {
	ID                        string
	Name                      string
	Business                  string
	Address                   string
	HealthUp                  *bool
	HealthLatencyMilliseconds *float64
	PortUp                    *bool
	ProcessUp                 *bool
	ProcessCount              *int64
	PortConsistent            *bool
	CPUUsagePercent           *float64
	MemoryBytes               *int64
	MemoryUsagePercent        *float64
	UptimeSeconds             *int64
	Status                    Level
	StatusSource              JavaStatusSource
	CollectionLevel           Level
}

type JavaPage struct {
	Services       []JavaServiceSummary
	AvailableNames []string
	Total          int
	Page           int
	PageSize       int
}
