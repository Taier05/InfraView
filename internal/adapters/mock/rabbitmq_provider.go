package mock

import (
	"context"
	"time"

	"github.com/Taier05/InfraView/internal/rabbitmq"
)

type rabbitMQProvider struct{}

func NewRabbitMQ() rabbitmq.Provider {
	return &rabbitMQProvider{}
}

func (provider *rabbitMQProvider) RabbitMQSnapshot(context.Context) (rabbitmq.Snapshot, error) {
	return rabbitMQSnapshotFixture().Clone(), nil
}

func rabbitMQSnapshotFixture() rabbitmq.Snapshot {
	reportedAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	primaryCluster := "fixture-rabbitmq-cluster-a"
	secondaryCluster := "fixture-rabbitmq-cluster-b"
	return rabbitmq.Snapshot{
		Clusters: []rabbitmq.Cluster{
			{
				ID:               rabbitmq.StableClusterID(primaryCluster),
				Name:             primaryCluster,
				UnreachablePeers: pointer(int64(0)),
			},
			{
				ID:               rabbitmq.StableClusterID(secondaryCluster),
				Name:             secondaryCluster,
				UnreachablePeers: pointer(int64(0)),
			},
		},
		Nodes: []rabbitmq.Node{
			rabbitMQFixtureNode(primaryCluster, "fixture-rabbitmq-node-normal", reportedAt),
			rabbitMQFixtureNode(primaryCluster, "fixture-rabbitmq-node-warning", reportedAt),
			rabbitMQFixtureNode(secondaryCluster, "fixture-rabbitmq-node-critical", reportedAt),
			rabbitMQFixtureNode(secondaryCluster, "fixture-rabbitmq-node-unknown", reportedAt),
		},
	}
}

func rabbitMQFixtureNode(cluster, name string, reportedAt time.Time) rabbitmq.Node {
	node := rabbitmq.Node{
		ID:                   rabbitmq.StableNodeID(cluster, name),
		Name:                 name,
		Cluster:              cluster,
		Address:              "fixture-address-" + name,
		Version:              "fixture-version",
		MemoryUsedBytes:      pointer(int64(40)),
		MemoryLimitBytes:     pointer(int64(100)),
		DiskAvailableBytes:   pointer(int64(20)),
		DiskLimitBytes:       pointer(int64(10)),
		OpenFileDescriptors:  pointer(int64(40)),
		MaxFileDescriptors:   pointer(int64(100)),
		ErlangProcessesUsed:  pointer(int64(40)),
		ErlangProcessesLimit: pointer(int64(100)),
		Connections:          pointer(int64(10)),
		Queues:               pointer(int64(2)),
		Messages:             pointer(int64(3)),
		PublishRate:          pointer(1.0),
		DeliverRate:          pointer(1.0),
		MemoryAlarm:          pointer(false),
		DiskAlarm:            pointer(false),
		FileDescriptorAlarm:  pointer(false),
		UptimeSeconds:        pointer(int64(60)),
		CollectionTracked:    true,
		ReportedAt:           reportedAt,
	}
	switch name {
	case "fixture-rabbitmq-node-warning":
		node.MemoryUsedBytes = pointer(int64(80))
	case "fixture-rabbitmq-node-critical":
		node.MemoryAlarm = pointer(true)
	case "fixture-rabbitmq-node-unknown":
		node.MemoryUsedBytes = nil
		node.MemoryLimitBytes = nil
		node.CollectionTracked = false
	}
	return node
}
