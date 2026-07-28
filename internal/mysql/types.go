package mysql

import (
	"crypto/sha256"
	"encoding/base64"
)

type Availability string
type Role string

const (
	AvailabilityUp      Availability = "up"
	AvailabilityDown    Availability = "down"
	AvailabilityUnknown Availability = "unknown"
	RoleWritable        Role         = "writable"
	RoleReadOnly        Role         = "read_only"
	RoleUnknown         Role         = "unknown"
)

type ReplicationChannel struct {
	IORunning  *bool
	SQLRunning *bool
	LagSeconds *float64
}

type Instance struct {
	ID                     string
	Name                   string
	Address                string
	Host                   string
	Version                string
	Availability           Availability
	Role                   Role
	UptimeSeconds          *float64
	Connections            *float64
	MaxConnections         *float64
	ThreadsRunning         *float64
	QPS                    *float64
	SlowQueriesPerSecond   *float64
	BufferPoolUsagePercent *float64
	ReplicationChannels    []ReplicationChannel
}

type Snapshot struct {
	Instances []Instance
}

func StableInstanceID(host, name, address string) string {
	sum := sha256.Sum256([]byte(host + "\x00" + name + "\x00" + address))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
