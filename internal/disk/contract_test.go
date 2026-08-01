package disk_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/disk"
	"github.com/Taier05/InfraView/internal/disk/disktest"
)

func TestStableDeviceIDUsesPriorityAndHost(t *testing.T) {
	id := disk.StableDeviceID("fixture-host-a", "fixture-wwn", "fixture-serial", "/dev/sda")
	if id == "" || id != disk.StableDeviceID("fixture-host-a", "fixture-wwn", "changed", "/dev/sdb") {
		t.Fatal("WWN priority is not stable")
	}
	if id == disk.StableDeviceID("fixture-host-b", "fixture-wwn", "fixture-serial", "/dev/sda") {
		t.Fatal("host identity is missing")
	}
	for _, raw := range []string{"fixture-host", "fixture-wwn", "fixture-serial", "/dev/sda"} {
		if strings.Contains(id, raw) {
			t.Fatalf("stable ID exposes raw identity")
		}
	}
}

func TestStableDeviceIDUsesSerialThenDeviceAndSeparatesIdentityKinds(t *testing.T) {
	serialID := disk.StableDeviceID("fixture-host", "", "fixture-serial", "/dev/sda")
	if serialID == "" || serialID != disk.StableDeviceID("fixture-host", "", "fixture-serial", "/dev/sdb") {
		t.Fatal("serial_no priority is not stable")
	}
	deviceID := disk.StableDeviceID("fixture-host", "", "", "/dev/sda")
	if deviceID == "" || deviceID == serialID {
		t.Fatal("identity kind does not separate serial_no and device")
	}
	serialKindID := disk.StableDeviceID("fixture-host", "", "fixture-shared", "")
	deviceKindID := disk.StableDeviceID("fixture-host", "", "", "fixture-shared")
	if serialKindID == deviceKindID {
		t.Fatal("identity kind is absent from the stable ID")
	}
}

func TestStableDeviceIDRequiresHostAndIdentity(t *testing.T) {
	if got := disk.StableDeviceID("", "fixture-wwn", "fixture-serial", "/dev/sda"); got != "" {
		t.Fatalf("empty host ID = %q, want empty", got)
	}
	if got := disk.StableDeviceID("fixture-host", "", "", ""); got != "" {
		t.Fatalf("empty device identity = %q, want empty", got)
	}
}

func TestDeviceDoesNotExposeRawIdentityFields(t *testing.T) {
	deviceType := reflect.TypeFor[disk.Device]()
	for _, forbidden := range []string{"SerialNo", "WWN", "Labels"} {
		if _, exists := deviceType.FieldByName(forbidden); exists {
			t.Fatalf("Device must not expose %s", forbidden)
		}
	}
}

func TestRunContractValidatesSnapshotFixture(t *testing.T) {
	disktest.RunContract(t, contractFixtureProvider{})
}

type contractFixtureProvider struct{}

func (contractFixtureProvider) SMARTSnapshot(context.Context) (disk.Snapshot, error) {
	return disk.Snapshot{Devices: []disk.Device{{
		ID:     disk.StableDeviceID("contract-host", "contract-wwn", "", "/dev/sda"),
		HostID: "contract-host",
		Device: "/dev/sda",
	}}}, nil
}
