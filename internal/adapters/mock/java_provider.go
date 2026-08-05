package mock

import (
	"context"
	"time"

	"github.com/Taier05/InfraView/internal/javaapp"
)

type javaProvider struct {
	clock func() time.Time
}

func NewJava(clock func() time.Time) javaapp.Provider {
	return &javaProvider{clock: clock}
}

func (provider *javaProvider) JavaSnapshot(context.Context) (javaapp.Snapshot, error) {
	now := time.Time{}
	if provider.clock != nil {
		now = provider.clock()
	}
	return javaSnapshotFixture(now).Clone(), nil
}

func javaSnapshotFixture(reportedAt time.Time) javaapp.Snapshot {
	return javaapp.Snapshot{Services: []javaapp.Service{
		javaFixture("fixture-java-normal", "fixture-address-normal", reportedAt),
		{
			ID:                        javaapp.StableServiceID("fixture-java-health-failed", "fixture-address-health-failed"),
			Name:                      "fixture-java-health-failed",
			Address:                   "fixture-address-health-failed",
			HealthLatencyMilliseconds: javaFloat64(1),
			HealthUp:                  javaBool(false),
			PortUp:                    javaBool(true),
			ProcessUp:                 javaBool(true),
			PortConsistent:            javaBool(true),
			ProcessCount:              javaInt64(1),
			ProcessMemoryBytes:        javaInt64(1),
			ProcessCPUPercent:         javaFloat64(1),
			ProcessMemoryPercent:      javaFloat64(1),
			ProcessStartTimeSeconds:   javaInt64(1),
			CollectionTracked:         true,
			ReportedAt:                reportedAt,
		},
		{
			ID:                javaapp.StableServiceID("fixture-java-missing-required", "fixture-address-missing-required"),
			Name:              "fixture-java-missing-required",
			Address:           "fixture-address-missing-required",
			CollectionTracked: true,
			ReportedAt:        reportedAt,
		},
		javaFixture("fixture-java-collection-warning", "fixture-address-collection-warning", reportedAt.Add(-2*time.Minute)),
	}}
}

func javaFixture(name, address string, reportedAt time.Time) javaapp.Service {
	return javaapp.Service{
		ID:                        javaapp.StableServiceID(name, address),
		Name:                      name,
		Address:                   address,
		HealthLatencyMilliseconds: javaFloat64(1),
		HealthUp:                  javaBool(true),
		PortUp:                    javaBool(true),
		ProcessUp:                 javaBool(true),
		PortConsistent:            javaBool(true),
		ProcessCount:              javaInt64(1),
		ProcessMemoryBytes:        javaInt64(1),
		ProcessCPUPercent:         javaFloat64(1),
		ProcessMemoryPercent:      javaFloat64(1),
		ProcessStartTimeSeconds:   javaInt64(1),
		CollectionTracked:         true,
		ReportedAt:                reportedAt,
	}
}

func javaBool(value bool) *bool { return &value }

func javaInt64(value int64) *int64 { return &value }

func javaFloat64(value float64) *float64 { return &value }
