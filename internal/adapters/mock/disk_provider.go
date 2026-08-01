package mock

import (
	"context"
	"time"

	"github.com/Taier05/InfraView/internal/disk"
)

type diskProvider struct {
	clock func() time.Time
}

func NewDisk(clock func() time.Time) disk.Provider {
	if clock == nil {
		clock = func() time.Time {
			return time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
		}
	}
	return &diskProvider{clock: clock}
}

func (p *diskProvider) SMARTSnapshot(context.Context) (disk.Snapshot, error) {
	now := p.clock().UTC()
	return disk.Snapshot{Devices: []disk.Device{
		fixtureDisk("fixture-disk-host-a", "fixture-wwn-ata-healthy", "/dev/fixture-ata-healthy", "fixture ATA healthy", now, disk.HealthHealthy, disk.AttributeFailureNone, allZeroDiskCounters(), diskValues{capacityBytes: int64Value(512 * 1024 * 1024 * 1024), temperature: float64Value(31), lifetime: float64Value(12), powerOnHours: float64Value(2400)}),
		fixtureDisk("fixture-disk-host-a", "fixture-wwn-ata-past", "/dev/fixture-ata-past", "fixture ATA past failure", now, disk.HealthHealthy, disk.AttributeFailurePast, disk.ErrorCounters{ReallocatedSectors: float64Value(0), UDMACRCErrors: float64Value(0)}, diskValues{capacityBytes: int64Value(1024 * 1024 * 1024 * 1024), temperature: float64Value(36), lifetime: float64Value(61), powerOnHours: float64Value(12000)}),
		fixtureDisk("fixture-disk-host-b", "fixture-wwn-ata-now", "/dev/fixture-ata-now", "fixture ATA failing now", now, disk.HealthFailed, disk.AttributeFailureNow, disk.ErrorCounters{PendingSectors: float64Value(4), ReallocatedSectors: float64Value(9), UncorrectableSectors: float64Value(2), UDMACRCErrors: float64Value(0)}, diskValues{capacityBytes: int64Value(256 * 1024 * 1024 * 1024), temperature: float64Value(49), lifetime: float64Value(98), powerOnHours: float64Value(36000)}),
		fixtureDisk("fixture-disk-host-b", "fixture-wwn-nvme-healthy", "/dev/fixture-nvme-healthy", "fixture NVMe healthy", now, disk.HealthHealthy, disk.AttributeFailureNone, allZeroDiskCounters(), diskValues{capacityBytes: int64Value(2 * 1024 * 1024 * 1024 * 1024), temperature: float64Value(34), lifetime: float64Value(8), powerOnHours: float64Value(1800), criticalWarning: int64Value(0), availableSpare: float64Value(100), availableSpareThreshold: float64Value(10)}),
		fixtureDisk("fixture-disk-host-c", "fixture-wwn-nvme-warning", "/dev/fixture-nvme-warning", "fixture NVMe critical warning", now, disk.HealthFailed, disk.AttributeFailureNone, disk.ErrorCounters{MediaIntegrityErrors: float64Value(1), ErrorLogEntries: float64Value(3), UnsafeShutdowns: float64Value(2)}, diskValues{capacityBytes: int64Value(1024 * 1024 * 1024 * 1024), temperature: float64Value(51), lifetime: float64Value(87), powerOnHours: float64Value(28000), criticalWarning: int64Value(1), availableSpare: float64Value(6), availableSpareThreshold: float64Value(10)}),
		fixtureDisk("fixture-disk-host-c", "fixture-wwn-unknown", "/dev/fixture-unknown", "fixture unknown SMART", now, disk.HealthUnknown, disk.AttributeFailureNone, disk.ErrorCounters{}, diskValues{}),
	}}, nil
}

type diskValues struct {
	capacityBytes           *int64
	temperature             *float64
	lifetime                *float64
	powerOnHours            *float64
	criticalWarning         *int64
	availableSpare          *float64
	availableSpareThreshold *float64
}

func fixtureDisk(hostID, wwn, device, model string, reportedAt time.Time, health disk.Health, attributeFailure disk.AttributeFailure, counters disk.ErrorCounters, values diskValues) disk.Device {
	return disk.Device{
		ID:                             disk.StableDeviceID(hostID, wwn, "", device),
		HostID:                         hostID,
		Device:                         device,
		Model:                          model,
		CapacityBytes:                  values.capacityBytes,
		SMARTHealth:                    health,
		TemperatureCelsius:             values.temperature,
		LifetimeUsedPercent:            values.lifetime,
		PowerOnHours:                   values.powerOnHours,
		CriticalWarning:                values.criticalWarning,
		AvailableSparePercent:          values.availableSpare,
		AvailableSpareThresholdPercent: values.availableSpareThreshold,
		AttributeFailure:               attributeFailure,
		Errors:                         counters,
		CollectionTracked:              true,
		ReportedAt:                     reportedAt,
	}
}

func allZeroDiskCounters() disk.ErrorCounters {
	return disk.ErrorCounters{
		PendingSectors:       float64Value(0),
		ReallocatedSectors:   float64Value(0),
		UncorrectableSectors: float64Value(0),
		UDMACRCErrors:        float64Value(0),
		MediaIntegrityErrors: float64Value(0),
		ErrorLogEntries:      float64Value(0),
		UnsafeShutdowns:      float64Value(0),
	}
}

func int64Value(value int64) *int64 {
	return &value
}

func float64Value(value float64) *float64 {
	return &value
}
