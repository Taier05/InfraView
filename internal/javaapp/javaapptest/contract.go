package javaapptest

import (
	"context"
	"testing"

	"github.com/Taier05/InfraView/internal/javaapp"
)

func RunContract(t testing.TB, provider javaapp.Provider) {
	t.Helper()
	first, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatalf("JavaSnapshot() error = %v", err)
	}
	second, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second JavaSnapshot() error = %v", err)
	}
	if len(first.Services) == 0 || len(first.Services) != len(second.Services) {
		t.Fatal("JavaSnapshot() service count is empty or unstable")
	}

	service := first.Services[0]
	if service.ID == "" || service.Name == "" || service.Address == "" {
		t.Fatal("contract requires display identity fields")
	}
	if service.HealthLatencyMilliseconds == nil || service.HealthUp == nil || service.PortUp == nil || service.ProcessUp == nil || service.PortConsistent == nil || service.ProcessCount == nil || service.ProcessMemoryBytes == nil || service.ProcessCPUPercent == nil || service.ProcessMemoryPercent == nil || service.ProcessStartTimeSeconds == nil {
		t.Fatal("contract requires display data in the primary fixture")
	}

	*first.Services[0].ProcessCount++
	if *first.Services[0].ProcessCount == *second.Services[0].ProcessCount {
		t.Fatal("provider returned shared mutable state")
	}
}

func Snapshot() javaapp.Snapshot {
	return javaapp.Snapshot{Services: []javaapp.Service{{
		ID:                        javaapp.StableServiceID("fixture-javaapp", "fixture-address-a"),
		Name:                      "fixture-javaapp",
		Address:                   "fixture-address-a",
		HealthLatencyMilliseconds: float64Value(1),
		HealthUp:                  boolValue(true),
		PortUp:                    boolValue(true),
		ProcessUp:                 boolValue(true),
		PortConsistent:            boolValue(true),
		ProcessCount:              int64Value(1),
		ProcessMemoryBytes:        int64Value(1),
		ProcessCPUPercent:         float64Value(1),
		ProcessMemoryPercent:      float64Value(1),
		ProcessStartTimeSeconds:   int64Value(1),
		CollectionTracked:         true,
	}}}
}

func boolValue(value bool) *bool { return &value }

func int64Value(value int64) *int64 { return &value }

func float64Value(value float64) *float64 { return &value }
