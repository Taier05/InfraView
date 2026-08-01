package service

import (
	"time"

	"github.com/Taier05/InfraView/internal/disk"
)

type DiskOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type DiskQuery struct {
	Search   string
	Status   Level
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type DiskStatusSource string

const (
	DiskStatusSourceSMARTHealth      DiskStatusSource = "smart_health"
	DiskStatusSourceDeviceWarning    DiskStatusSource = "device_warning"
	DiskStatusSourceAttributeFailure DiskStatusSource = "attribute_failure"
	DiskStatusSourceCollection       DiskStatusSource = "collection"
	DiskStatusSourceUnknown          DiskStatusSource = "unknown"
	DiskStatusSourceNormal           DiskStatusSource = "normal"
)

type DiskOverviewAlerts struct {
	SMARTHealth      AlertCount
	DeviceWarning    AlertCount
	AttributeFailure AlertCount
	Collection       AlertCount
}

type DiskOverview struct {
	Total           int
	Normal          int
	Warning         int
	Critical        int
	Unknown         int
	AffectedDevices int
	WarningDevices  int
	CriticalDevices int
	Alerts          DiskOverviewAlerts
}

type DiskDeviceSummary struct {
	ID                  string
	Host                string
	Device              string
	Model               string
	CapacityBytes       *int64
	SMARTHealth         disk.Health
	TemperatureCelsius  *float64
	LifetimeUsedPercent *float64
	PowerOnHours        *float64
	Errors              disk.ErrorCounters
	Status              Level
	StatusSource        DiskStatusSource
	CollectionLevel     Level
}

type DiskPage struct {
	Devices    []DiskDeviceSummary
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}
