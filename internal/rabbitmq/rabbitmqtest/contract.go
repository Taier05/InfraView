package rabbitmqtest

import (
	"context"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/rabbitmq"
)

func RunContract(t testing.TB, provider rabbitmq.Provider) {
	t.Helper()
	first, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatalf("RabbitMQSnapshot() error = %v", err)
	}
	second, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second RabbitMQSnapshot() error = %v", err)
	}
	if len(first.Clusters) < 2 || len(first.Nodes) < 4 {
		t.Fatal("contract requires multiple clusters and four nodes")
	}
	if first.Nodes[0].Connections == nil || second.Nodes[0].Connections == nil {
		t.Fatal("contract requires node connection data")
	}
	original := *second.Nodes[0].Connections
	*first.Nodes[0].Connections = original + 1
	if *second.Nodes[0].Connections != original {
		t.Fatal("provider returned shared mutable state")
	}
}

func Snapshot() rabbitmq.Snapshot {
	reportedAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	cluster := "fixture-cluster-a"
	return rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{{
			ID:               rabbitmq.StableClusterID(cluster),
			Name:             cluster,
			UnreachablePeers: int64Pointer(0),
		}},
		Nodes: []rabbitmq.Node{{
			ID:                   rabbitmq.StableNodeID(cluster, "fixture-node-a"),
			Name:                 "fixture-node-a",
			Cluster:              cluster,
			Address:              "fixture-address-a",
			Version:              "fixture-version-a",
			MemoryUsedBytes:      int64Pointer(40),
			MemoryLimitBytes:     int64Pointer(100),
			DiskAvailableBytes:   int64Pointer(20),
			DiskLimitBytes:       int64Pointer(10),
			OpenFileDescriptors:  int64Pointer(40),
			MaxFileDescriptors:   int64Pointer(100),
			ErlangProcessesUsed:  int64Pointer(40),
			ErlangProcessesLimit: int64Pointer(100),
			Connections:          int64Pointer(10),
			Queues:               int64Pointer(2),
			Messages:             int64Pointer(3),
			PublishRate:          float64Pointer(1),
			DeliverRate:          float64Pointer(1),
			MemoryAlarm:          boolPointer(false),
			DiskAlarm:            boolPointer(false),
			FileDescriptorAlarm:  boolPointer(false),
			UptimeSeconds:        int64Pointer(60),
			CollectionTracked:    true,
			ReportedAt:           reportedAt,
		}},
	}
}

func int64Pointer(value int64) *int64 { return &value }

func float64Pointer(value float64) *float64 { return &value }

func boolPointer(value bool) *bool { return &value }
