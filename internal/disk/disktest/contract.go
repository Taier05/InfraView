package disktest

import (
	"context"
	"testing"

	"github.com/Taier05/InfraView/internal/disk"
)

func RunContract(t testing.TB, provider disk.Provider) {
	t.Helper()
	first, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatalf("SMARTSnapshot() error = %v", err)
	}
	second, err := provider.SMARTSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second SMARTSnapshot() error = %v", err)
	}
	if len(first.Devices) == 0 {
		t.Fatal("SMARTSnapshot() returned no devices")
	}
	if len(first.Devices) != len(second.Devices) {
		t.Fatal("SMARTSnapshot() device count is not stable")
	}

	seen := map[string]struct{}{}
	for index, device := range first.Devices {
		if device.ID == "" || device.HostID == "" || device.Device == "" {
			t.Fatalf("incomplete fixture device at index %d", index)
		}
		if _, exists := seen[device.ID]; exists {
			t.Fatalf("duplicate stable ID %q", device.ID)
		}
		seen[device.ID] = struct{}{}
		if device.ID != second.Devices[index].ID {
			t.Fatalf("device ID at index %d is not stable", index)
		}
	}
}
