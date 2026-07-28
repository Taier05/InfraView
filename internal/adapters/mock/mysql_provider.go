package mock

import (
	"context"

	"github.com/Taier05/InfraView/internal/mysql"
)

type mysqlProvider struct{}

func NewMySQL() mysql.Provider {
	return &mysqlProvider{}
}

func (p *mysqlProvider) MySQLSnapshot(context.Context) (mysql.Snapshot, error) {
	return mysql.Snapshot{Instances: []mysql.Instance{
		mysqlFixtureInstance("fixture-host-writable", "fixture-mysql-writable", "192.0.2.10:3306", mysql.AvailabilityUp, mysql.RoleWritable),
		mysqlFixtureInstance("fixture-host-low-lag", "fixture-mysql-low-lag", "192.0.2.11:3306", mysql.AvailabilityUp, mysql.RoleReadOnly, mysql.ReplicationChannel{IORunning: boolValue(true), SQLRunning: boolValue(true), LagSeconds: floatValue(3)}),
		mysqlFixtureInstance("fixture-host-warning-lag", "fixture-mysql-warning-lag", "192.0.2.12:3306", mysql.AvailabilityUp, mysql.RoleReadOnly, mysql.ReplicationChannel{IORunning: boolValue(true), SQLRunning: boolValue(true), LagSeconds: floatValue(12)}),
		mysqlFixtureInstance("fixture-host-critical-lag", "fixture-mysql-critical-lag", "192.0.2.13:3306", mysql.AvailabilityUp, mysql.RoleReadOnly, mysql.ReplicationChannel{IORunning: boolValue(true), SQLRunning: boolValue(true), LagSeconds: floatValue(60)}),
		mysqlFixtureInstance("fixture-host-stopped-replication", "fixture-mysql-stopped-replication", "192.0.2.14:3306", mysql.AvailabilityUp, mysql.RoleReadOnly, mysql.ReplicationChannel{IORunning: boolValue(false), SQLRunning: boolValue(false), LagSeconds: floatValue(0)}),
		mysqlFixtureInstance("fixture-host-missing-replication", "fixture-mysql-missing-replication", "192.0.2.15:3306", mysql.AvailabilityUp, mysql.RoleReadOnly, mysql.ReplicationChannel{}),
		mysqlFixtureInstance("fixture-host-unknown", "fixture-mysql-unknown", "192.0.2.16:3306", mysql.AvailabilityUnknown, mysql.RoleUnknown),
	}}, nil
}

func mysqlFixtureInstance(host, name, address string, availability mysql.Availability, role mysql.Role, channels ...mysql.ReplicationChannel) mysql.Instance {
	instance := mockMySQLInstance(host, name, address)
	instance.Availability = availability
	instance.Role = role
	instance.Version = "fixture-mysql-version"
	instance.UptimeSeconds = floatValue(172800)
	instance.Connections = floatValue(24)
	instance.MaxConnections = floatValue(160)
	instance.ThreadsRunning = floatValue(4)
	instance.QPS = floatValue(32)
	instance.SlowQueriesPerSecond = floatValue(0)
	instance.BufferPoolUsagePercent = floatValue(48)
	instance.ReplicationChannels = channels
	if availability == mysql.AvailabilityUnknown {
		instance.Version = ""
		instance.UptimeSeconds = nil
		instance.Connections = nil
		instance.MaxConnections = nil
		instance.ThreadsRunning = nil
		instance.QPS = nil
		instance.SlowQueriesPerSecond = nil
		instance.BufferPoolUsagePercent = nil
	}
	return instance
}

func mockMySQLInstance(host, name, address string) mysql.Instance {
	return mysql.Instance{
		ID:      mysql.StableInstanceID(host, name, address),
		Host:    host,
		Name:    name,
		Address: address,
	}
}

func boolValue(value bool) *bool {
	return &value
}

func floatValue(value float64) *float64 {
	return &value
}
