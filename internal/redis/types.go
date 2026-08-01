package redis

import (
	"crypto/sha256"
	"encoding/base64"
	"time"
)

type Availability string
type Role string

const (
	AvailabilityUp      Availability = "up"
	AvailabilityDown    Availability = "down"
	AvailabilityUnknown Availability = "unknown"

	RoleMaster  Role = "master"
	RoleSlave   Role = "slave"
	RoleUnknown Role = "unknown"
)

type Replication struct {
	ConnectedReplicas      *int64
	MasterLinkUp           *bool
	MasterLastIOSecondsAgo *float64
	MasterSyncInProgress   *bool
	WorstReplicaLagSeconds *float64
}

type Instance struct {
	ID                      string
	Address                 string
	Availability            Availability
	Role                    Role
	ClusterEnabled          *bool
	UptimeSeconds           *int64
	UsedMemoryBytes         *int64
	MaxMemoryBytes          *int64
	ConnectedClients        *int64
	MaxClients              *int64
	BlockedClients          *int64
	QPS                     *float64
	HitRate                 *float64
	Keys                    *int64
	ExpiredKeysPerSecond    *float64
	EvictedKeysPerSecond    *float64
	RejectedConnectionsRate *float64
	Replication             Replication
	CollectionTracked       bool
	ReportedAt              time.Time
}

type Snapshot struct {
	Instances []Instance
}

func StableInstanceID(ident, instance, address string) string {
	if ident == "" || instance == "" || address == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(ident + "\x00" + instance + "\x00" + address))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (snapshot Snapshot) Clone() Snapshot {
	cloned := Snapshot{Instances: make([]Instance, len(snapshot.Instances))}
	for index, instance := range snapshot.Instances {
		cloned.Instances[index] = cloneInstance(instance)
	}
	return cloned
}

func cloneInstance(instance Instance) Instance {
	cloned := instance
	cloned.ClusterEnabled = clonePointer(instance.ClusterEnabled)
	cloned.UptimeSeconds = clonePointer(instance.UptimeSeconds)
	cloned.UsedMemoryBytes = clonePointer(instance.UsedMemoryBytes)
	cloned.MaxMemoryBytes = clonePointer(instance.MaxMemoryBytes)
	cloned.ConnectedClients = clonePointer(instance.ConnectedClients)
	cloned.MaxClients = clonePointer(instance.MaxClients)
	cloned.BlockedClients = clonePointer(instance.BlockedClients)
	cloned.QPS = clonePointer(instance.QPS)
	cloned.HitRate = clonePointer(instance.HitRate)
	cloned.Keys = clonePointer(instance.Keys)
	cloned.ExpiredKeysPerSecond = clonePointer(instance.ExpiredKeysPerSecond)
	cloned.EvictedKeysPerSecond = clonePointer(instance.EvictedKeysPerSecond)
	cloned.RejectedConnectionsRate = clonePointer(instance.RejectedConnectionsRate)
	cloned.Replication.ConnectedReplicas = clonePointer(instance.Replication.ConnectedReplicas)
	cloned.Replication.MasterLinkUp = clonePointer(instance.Replication.MasterLinkUp)
	cloned.Replication.MasterLastIOSecondsAgo = clonePointer(instance.Replication.MasterLastIOSecondsAgo)
	cloned.Replication.MasterSyncInProgress = clonePointer(instance.Replication.MasterSyncInProgress)
	cloned.Replication.WorstReplicaLagSeconds = clonePointer(instance.Replication.WorstReplicaLagSeconds)
	return cloned
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
