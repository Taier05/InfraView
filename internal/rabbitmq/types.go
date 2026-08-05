package rabbitmq

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
)

type Cluster struct {
	ID               string
	Name             string
	UnreachablePeers *int64
}

type Node struct {
	ID, Name, Cluster, Address, Version         string
	MemoryUsedBytes, MemoryLimitBytes           *int64
	DiskAvailableBytes, DiskLimitBytes          *int64
	OpenFileDescriptors, MaxFileDescriptors     *int64
	ErlangProcessesUsed, ErlangProcessesLimit   *int64
	Connections, Queues, Messages               *int64
	PublishRate, DeliverRate                    *float64
	MemoryAlarm, DiskAlarm, FileDescriptorAlarm *bool
	UptimeSeconds                               *int64
	CollectionTracked                           bool
	ReportedAt                                  time.Time
}

type Snapshot struct {
	Clusters []Cluster
	Nodes    []Node
}

func StableClusterID(identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	return stableID("cluster\x00" + identity)
}

func StableNodeID(clusterIdentity, nodeName string) string {
	clusterIdentity = strings.TrimSpace(clusterIdentity)
	nodeName = strings.TrimSpace(nodeName)
	if clusterIdentity == "" || nodeName == "" {
		return ""
	}
	return stableID("node\x00" + clusterIdentity + "\x00" + nodeName)
}

func (snapshot Snapshot) Clone() Snapshot {
	var clone Snapshot
	if snapshot.Clusters != nil {
		clone.Clusters = make([]Cluster, len(snapshot.Clusters))
	}
	if snapshot.Nodes != nil {
		clone.Nodes = make([]Node, len(snapshot.Nodes))
	}
	for index, cluster := range snapshot.Clusters {
		clone.Clusters[index] = cluster
		clone.Clusters[index].UnreachablePeers = cloneInt64(cluster.UnreachablePeers)
	}
	for index, node := range snapshot.Nodes {
		clone.Nodes[index] = node
		clone.Nodes[index].MemoryUsedBytes = cloneInt64(node.MemoryUsedBytes)
		clone.Nodes[index].MemoryLimitBytes = cloneInt64(node.MemoryLimitBytes)
		clone.Nodes[index].DiskAvailableBytes = cloneInt64(node.DiskAvailableBytes)
		clone.Nodes[index].DiskLimitBytes = cloneInt64(node.DiskLimitBytes)
		clone.Nodes[index].OpenFileDescriptors = cloneInt64(node.OpenFileDescriptors)
		clone.Nodes[index].MaxFileDescriptors = cloneInt64(node.MaxFileDescriptors)
		clone.Nodes[index].ErlangProcessesUsed = cloneInt64(node.ErlangProcessesUsed)
		clone.Nodes[index].ErlangProcessesLimit = cloneInt64(node.ErlangProcessesLimit)
		clone.Nodes[index].Connections = cloneInt64(node.Connections)
		clone.Nodes[index].Queues = cloneInt64(node.Queues)
		clone.Nodes[index].Messages = cloneInt64(node.Messages)
		clone.Nodes[index].PublishRate = cloneFloat64(node.PublishRate)
		clone.Nodes[index].DeliverRate = cloneFloat64(node.DeliverRate)
		clone.Nodes[index].MemoryAlarm = cloneBool(node.MemoryAlarm)
		clone.Nodes[index].DiskAlarm = cloneBool(node.DiskAlarm)
		clone.Nodes[index].FileDescriptorAlarm = cloneBool(node.FileDescriptorAlarm)
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

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
