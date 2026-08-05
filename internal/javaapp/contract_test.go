package javaapp_test

import (
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/javaapp"
	"github.com/Taier05/InfraView/internal/javaapp/javaapptest"
)

func TestStableServiceIDUsesNameAndAddressWithoutExposingEither(t *testing.T) {
	first := javaapp.StableServiceID("tikbee", "fixture-address-a")
	if first == "" || first == javaapp.StableServiceID("tikbee", "fixture-address-b") {
		t.Fatal("service ID must be non-empty and address scoped")
	}
	if strings.Contains(first, "tikbee") || strings.Contains(first, "fixture-address-a") {
		t.Fatal("stable ID exposed raw identity")
	}
}

func TestStableServiceIDTrimsIdentity(t *testing.T) {
	if got, want := javaapp.StableServiceID(" tikbee ", " fixture-address-a "), javaapp.StableServiceID("tikbee", "fixture-address-a"); got != want {
		t.Fatalf("StableServiceID() = %q, want %q", got, want)
	}
}

func TestSnapshotCloneDoesNotAliasPointers(t *testing.T) {
	source := javaapptest.Snapshot()
	clone := source.Clone()
	*clone.Services[0].ProcessCount = 999
	if *source.Services[0].ProcessCount == 999 {
		t.Fatal("clone aliases source")
	}
}

func TestSnapshotClonePreservesNilAndEmptyServiceSlices(t *testing.T) {
	if got := (javaapp.Snapshot{}).Clone().Services; got != nil {
		t.Fatalf("nil services = %#v, want nil", got)
	}
	if got := (javaapp.Snapshot{Services: []javaapp.Service{}}).Clone().Services; got == nil || len(got) != 0 {
		t.Fatalf("empty services = %#v, want non-nil empty slice", got)
	}
}
