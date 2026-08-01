package mock_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/disk"
	"github.com/Taier05/InfraView/internal/disk/disktest"
)

func TestDiskProviderContract(t *testing.T) {
	disktest.RunContract(t, mock.NewDisk(fixedDiskClock))
}

func TestDiskProviderContainsDeterministicSMARTScenarios(t *testing.T) {
	provider := mock.NewDisk(fixedDiskClock)
	first, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock SMART snapshot is not deterministic with a fixed clock")
	}
	if len(first.Devices) < 6 {
		t.Fatalf("mock SMART snapshot has %d devices, want at least 6", len(first.Devices))
	}

	var healthyATA, pastFailureATA, nowFailureATA, healthyNVMe, warningNVMe, unknown bool
	var allZero, partialMissingKnownZero, multipleNonZero, allMissing bool
	for _, device := range first.Devices {
		if device.ReportedAt.IsZero() || !device.ReportedAt.Equal(fixedDiskClock()) {
			t.Fatalf("device %q ReportedAt = %v, want fixed non-zero time", device.ID, device.ReportedAt)
		}
		switch device.Device {
		case "/dev/fixture-ata-healthy":
			healthyATA = device.SMARTHealth == disk.HealthHealthy && device.AttributeFailure == disk.AttributeFailureNone
			allZero = allErrorCountersEqual(device.Errors, 0)
		case "/dev/fixture-ata-past":
			pastFailureATA = device.SMARTHealth == disk.HealthHealthy && device.AttributeFailure == disk.AttributeFailurePast
			partialMissingKnownZero = device.Errors.PendingSectors == nil && device.Errors.ReallocatedSectors != nil && *device.Errors.ReallocatedSectors == 0
		case "/dev/fixture-ata-now":
			nowFailureATA = device.SMARTHealth == disk.HealthFailed && device.AttributeFailure == disk.AttributeFailureNow
			multipleNonZero = countNonZero(device.Errors) >= 2
		case "/dev/fixture-nvme-healthy":
			healthyNVMe = device.SMARTHealth == disk.HealthHealthy
		case "/dev/fixture-nvme-warning":
			warningNVMe = device.SMARTHealth == disk.HealthFailed && device.CriticalWarning != nil && *device.CriticalWarning != 0
		case "/dev/fixture-unknown":
			unknown = device.SMARTHealth == disk.HealthUnknown && device.AttributeFailure == disk.AttributeFailureNone
			allMissing = allErrorCountersNil(device.Errors)
			if device.CapacityBytes != nil || device.TemperatureCelsius != nil || device.LifetimeUsedPercent != nil {
				t.Fatalf("unknown fixture should keep selected optional fields absent: %#v", device)
			}
		}
	}
	if !healthyATA || !pastFailureATA || !nowFailureATA || !healthyNVMe || !warningNVMe || !unknown {
		t.Fatal("mock must cover ATA/NVMe healthy, warning, critical, and unknown SMART scenarios")
	}
	if !allZero || !partialMissingKnownZero || !multipleNonZero || !allMissing {
		t.Fatal("mock must cover all required error counter states")
	}
}

func TestDiskProviderUsesFixedClockWhenNil(t *testing.T) {
	provider := mock.NewDisk(nil)
	first, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock SMART snapshot is not deterministic when the clock is nil")
	}
}

func fixedDiskClock() time.Time {
	return time.Date(2026, time.July, 30, 12, 34, 56, 0, time.UTC)
}

func allErrorCountersEqual(counters disk.ErrorCounters, want float64) bool {
	values := []*float64{
		counters.PendingSectors,
		counters.ReallocatedSectors,
		counters.UncorrectableSectors,
		counters.UDMACRCErrors,
		counters.MediaIntegrityErrors,
		counters.ErrorLogEntries,
		counters.UnsafeShutdowns,
	}
	for _, value := range values {
		if value == nil || *value != want {
			return false
		}
	}
	return true
}

func allErrorCountersNil(counters disk.ErrorCounters) bool {
	return counters.PendingSectors == nil &&
		counters.ReallocatedSectors == nil &&
		counters.UncorrectableSectors == nil &&
		counters.UDMACRCErrors == nil &&
		counters.MediaIntegrityErrors == nil &&
		counters.ErrorLogEntries == nil &&
		counters.UnsafeShutdowns == nil
}

func countNonZero(counters disk.ErrorCounters) int {
	values := []*float64{
		counters.PendingSectors,
		counters.ReallocatedSectors,
		counters.UncorrectableSectors,
		counters.UDMACRCErrors,
		counters.MediaIntegrityErrors,
		counters.ErrorLogEntries,
		counters.UnsafeShutdowns,
	}
	count := 0
	for _, value := range values {
		if value != nil && *value != 0 {
			count++
		}
	}
	return count
}
