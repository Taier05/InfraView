package elasticsearch

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
)

type Health string

const (
	HealthGreen   Health = "green"
	HealthYellow  Health = "yellow"
	HealthRed     Health = "red"
	HealthUnknown Health = "unknown"
)

type Availability string

const (
	AvailabilityUp      Availability = "up"
	AvailabilityDown    Availability = "down"
	AvailabilityUnknown Availability = "unknown"
)

type Role string

const (
	RoleMaster              Role = "master"
	RoleData                Role = "data"
	RoleDataContent         Role = "data_content"
	RoleDataHot             Role = "data_hot"
	RoleDataWarm            Role = "data_warm"
	RoleDataCold            Role = "data_cold"
	RoleDataFrozen          Role = "data_frozen"
	RoleIngest              Role = "ingest"
	RoleML                  Role = "ml"
	RoleTransform           Role = "transform"
	RoleRemoteClusterClient Role = "remote_cluster_client"
	RoleClient              Role = "client"
)

type Cluster struct {
	ID                    string
	Name                  string
	Availability          Availability
	NodeStatsAvailability Availability
	Health                Health
	NumberOfNodes         *int64
	NumberOfDataNodes     *int64
	ActivePrimaryShards   *int64
	ActiveShards          *int64
	RelocatingShards      *int64
	InitializingShards    *int64
	UnassignedShards      *int64
	PendingTasks          *int64
	TaskMaxWaitingMillis  *int64
	CollectionTracked     bool
	ReportedAt            time.Time
}

type Node struct {
	ID                string
	Name              string
	Cluster           string
	Address           string
	Roles             []Role
	HeapUsedBytes     *int64
	HeapMaxBytes      *int64
	DiskUsagePercent  *float64
	CPUUsagePercent   *float64
	IndexRate         *float64
	SearchRate        *float64
	Documents         *int64
	StoreSizeBytes    *int64
	ThreadPoolQueue   *int64
	RejectedRate      *float64
	UptimeSeconds     *int64
	DataNode          bool
	CollectionTracked bool
	ReportedAt        time.Time
}

type Snapshot struct {
	Clusters []Cluster
	Nodes    []Node
}

func StableClusterID(cluster string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return ""
	}
	return stableID("cluster\x00" + cluster)
}

func StableNodeID(cluster, name string) string {
	cluster = strings.TrimSpace(cluster)
	name = strings.TrimSpace(name)
	if cluster == "" || name == "" {
		return ""
	}
	return stableID("node\x00" + cluster + "\x00" + name)
}

func (snapshot Snapshot) Clone() Snapshot {
	clone := Snapshot{
		Clusters: make([]Cluster, len(snapshot.Clusters)),
		Nodes:    make([]Node, len(snapshot.Nodes)),
	}
	for index, cluster := range snapshot.Clusters {
		clone.Clusters[index] = cluster
		clone.Clusters[index].NumberOfNodes = cloneInt64(cluster.NumberOfNodes)
		clone.Clusters[index].NumberOfDataNodes = cloneInt64(cluster.NumberOfDataNodes)
		clone.Clusters[index].ActivePrimaryShards = cloneInt64(cluster.ActivePrimaryShards)
		clone.Clusters[index].ActiveShards = cloneInt64(cluster.ActiveShards)
		clone.Clusters[index].RelocatingShards = cloneInt64(cluster.RelocatingShards)
		clone.Clusters[index].InitializingShards = cloneInt64(cluster.InitializingShards)
		clone.Clusters[index].UnassignedShards = cloneInt64(cluster.UnassignedShards)
		clone.Clusters[index].PendingTasks = cloneInt64(cluster.PendingTasks)
		clone.Clusters[index].TaskMaxWaitingMillis = cloneInt64(cluster.TaskMaxWaitingMillis)
	}
	for index, node := range snapshot.Nodes {
		clone.Nodes[index] = node
		clone.Nodes[index].Roles = append([]Role(nil), node.Roles...)
		clone.Nodes[index].HeapUsedBytes = cloneInt64(node.HeapUsedBytes)
		clone.Nodes[index].HeapMaxBytes = cloneInt64(node.HeapMaxBytes)
		clone.Nodes[index].DiskUsagePercent = cloneFloat64(node.DiskUsagePercent)
		clone.Nodes[index].CPUUsagePercent = cloneFloat64(node.CPUUsagePercent)
		clone.Nodes[index].IndexRate = cloneFloat64(node.IndexRate)
		clone.Nodes[index].SearchRate = cloneFloat64(node.SearchRate)
		clone.Nodes[index].Documents = cloneInt64(node.Documents)
		clone.Nodes[index].StoreSizeBytes = cloneInt64(node.StoreSizeBytes)
		clone.Nodes[index].ThreadPoolQueue = cloneInt64(node.ThreadPoolQueue)
		clone.Nodes[index].RejectedRate = cloneFloat64(node.RejectedRate)
		clone.Nodes[index].UptimeSeconds = cloneInt64(node.UptimeSeconds)
	}
	return clone
}

func stableID(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
