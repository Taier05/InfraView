package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/Taier05/InfraView/internal/mysql"
)

func TestMySQLSummaryAggregatesReplicationChannelsAndBoundaries(t *testing.T) {
	tests := []struct {
		name string
		lag  float64
		want Level
	}{
		{name: "below warning", lag: 4.999, want: LevelNormal},
		{name: "warning boundary", lag: 5, want: LevelWarning},
		{name: "below critical", lag: 29.999, want: LevelWarning},
		{name: "critical boundary", lag: 30, want: LevelCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMySQLInstance(readOnlyInstanceWithLag(tt.lag))
			if summary.Replication.Level != tt.want {
				t.Fatalf("level = %q, want %q", summary.Replication.Level, tt.want)
			}
		})
	}
}

func TestMySQLSummaryMakesStoppedThreadCritical(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2)},
	)
	summary := summarizeMySQLInstance(instance)
	if summary.Status != LevelCritical ||
		summary.Replication.State != ReplicationThreadsStopped {
		t.Fatalf("instance = %#v", summary)
	}
}

func TestMySQLSummaryPreservesMaximumLagWhenThreadStopped(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2)},
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(11)},
	)
	summary := summarizeMySQLInstance(instance)
	if summary.Replication.State != ReplicationThreadsStopped ||
		summary.Replication.Level != LevelCritical ||
		summary.Replication.LagSeconds == nil ||
		*summary.Replication.LagSeconds != 11 {
		t.Fatalf("replication = %#v", summary.Replication)
	}
}

func TestMySQLSummaryAppliesRoleAndMissingReplicationSemantics(t *testing.T) {
	writable := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleWritable, Availability: mysql.AvailabilityUp})
	if writable.Replication.State != ReplicationNotConfigured || writable.Status != LevelNormal {
		t.Fatalf("writable = %#v", writable)
	}
	readOnly := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleReadOnly, Availability: mysql.AvailabilityUp})
	if readOnly.Replication.State != ReplicationUnknown || readOnly.Status != LevelUnknown {
		t.Fatalf("readOnly = %#v", readOnly)
	}
	unknownRole := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleUnknown, Availability: mysql.AvailabilityUp})
	if unknownRole.Replication.State != ReplicationUnknown || unknownRole.Status != LevelUnknown {
		t.Fatalf("unknown role = %#v", unknownRole)
	}
	unknownAvailability := summarizeMySQLInstance(mysql.Instance{
		Role: mysql.RoleWritable, Availability: mysql.AvailabilityUnknown,
	})
	if unknownAvailability.Status != LevelUnknown {
		t.Fatalf("unknown availability = %#v", unknownAvailability)
	}
}

func TestMySQLSummaryCalculatesOnlyValidConnectionUsageAndMaximumLag(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(8)},
	)
	instance.Connections = floatPointer(25)
	instance.MaxConnections = floatPointer(100)
	summary := summarizeMySQLInstance(instance)
	if summary.ConnectionUsagePercent == nil || *summary.ConnectionUsagePercent != 25 {
		t.Fatalf("connection usage = %#v", summary.ConnectionUsagePercent)
	}
	if summary.Replication.LagSeconds == nil || *summary.Replication.LagSeconds != 8 {
		t.Fatalf("replication lag = %#v", summary.Replication.LagSeconds)
	}
	instance.MaxConnections = floatPointer(0)
	if got := summarizeMySQLInstance(instance).ConnectionUsagePercent; got != nil {
		t.Fatalf("zero maximum produced usage %#v", got)
	}
}

func TestMySQLSummaryMakesIncompleteCriticalReplicationDataUnknown(t *testing.T) {
	instance := instanceWithChannels(mysql.ReplicationChannel{
		SQLRunning: boolPointer(true),
		LagSeconds: floatPointer(1),
	})
	summary := summarizeMySQLInstance(instance)
	if summary.Replication.State != ReplicationUnknown ||
		summary.Replication.Level != LevelUnknown ||
		summary.Status != LevelUnknown {
		t.Fatalf("instance = %#v", summary)
	}
}

func TestMySQLSummaryTreatsInvalidReplicationLagAsMissing(t *testing.T) {
	tests := []struct {
		name string
		lag  float64
	}{
		{name: "negative", lag: -1},
		{name: "nan", lag: math.NaN()},
		{name: "positive infinity", lag: math.Inf(1)},
		{name: "negative infinity", lag: math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMySQLInstance(readOnlyInstanceWithLag(tt.lag))
			if summary.Replication.State != ReplicationUnknown ||
				summary.Replication.Level != LevelUnknown ||
				summary.Replication.LagSeconds != nil ||
				summary.Status != LevelUnknown {
				t.Fatalf("instance = %#v", summary)
			}
			if _, err := json.Marshal(summary); err != nil {
				t.Fatalf("marshal summary: %v", err)
			}
		})
	}
}

func TestMySQLSummaryDoesNotThresholdOtherMetrics(t *testing.T) {
	instance := mysql.Instance{
		Availability:           mysql.AvailabilityUp,
		Role:                   mysql.RoleWritable,
		ThreadsRunning:         floatPointer(1_000_000),
		QPS:                    floatPointer(1_000_000),
		SlowQueriesPerSecond:   floatPointer(1_000_000),
		BufferPoolUsagePercent: floatPointer(100),
	}
	summary := summarizeMySQLInstance(instance)
	if summary.Status != LevelNormal {
		t.Fatalf("status = %q, want %q", summary.Status, LevelNormal)
	}
	if summary.ThreadsRunning == nil || *summary.ThreadsRunning != 1_000_000 ||
		summary.QPS == nil || *summary.QPS != 1_000_000 ||
		summary.SlowQueriesPerSecond == nil || *summary.SlowQueriesPerSecond != 1_000_000 ||
		summary.BufferPoolUsagePercent == nil || *summary.BufferPoolUsagePercent != 100 {
		t.Fatalf("metrics = %#v", summary)
	}
}

func TestMySQLSummaryMakesUnavailableInstanceCritical(t *testing.T) {
	summary := summarizeMySQLInstance(mysql.Instance{
		Availability: mysql.AvailabilityDown,
		Role:         mysql.RoleWritable,
	})
	if summary.Status != LevelCritical {
		t.Fatalf("status = %q, want %q", summary.Status, LevelCritical)
	}
}

func boolPointer(value bool) *bool        { return &value }
func floatPointer(value float64) *float64 { return &value }

func readOnlyInstanceWithLag(lag float64) mysql.Instance {
	return instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(lag),
	})
}

func instanceWithChannels(channels ...mysql.ReplicationChannel) mysql.Instance {
	return mysql.Instance{
		ID:                  mysql.StableInstanceID("fixture-host-a", "fixture-mysql-a", "192.0.2.10:3306"),
		Name:                "fixture-mysql-a",
		Address:             "192.0.2.10:3306",
		Host:                "fixture-host-a",
		Availability:        mysql.AvailabilityUp,
		Role:                mysql.RoleReadOnly,
		ReplicationChannels: channels,
	}
}

type recordingMySQLProvider struct {
	snapshot mysql.Snapshot
	err      error
	calls    int
}

func (p *recordingMySQLProvider) MySQLSnapshot(context.Context) (mysql.Snapshot, error) {
	p.calls++
	return p.snapshot, p.err
}
