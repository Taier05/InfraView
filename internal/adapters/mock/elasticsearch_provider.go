package mock

import (
	"context"
	"time"

	"github.com/Taier05/InfraView/internal/elasticsearch"
)

type elasticsearchProvider struct{}

func NewElasticsearch() elasticsearch.Provider {
	return &elasticsearchProvider{}
}

func (provider *elasticsearchProvider) ElasticsearchSnapshot(context.Context) (elasticsearch.Snapshot, error) {
	return elasticsearchSnapshotFixture().Clone(), nil
}

func elasticsearchSnapshotFixture() elasticsearch.Snapshot {
	reportedAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	greenCluster := "fixture-cluster-green"
	yellowCluster := "fixture-cluster-yellow"
	redCluster := "fixture-cluster-red"
	return elasticsearch.Snapshot{
		Clusters: []elasticsearch.Cluster{
			fixtureCluster(greenCluster, elasticsearch.HealthGreen, elasticsearch.AvailabilityUp, reportedAt),
			fixtureCluster(yellowCluster, elasticsearch.HealthYellow, elasticsearch.AvailabilityUp, reportedAt),
			fixtureCluster(redCluster, elasticsearch.HealthRed, elasticsearch.AvailabilityDown, reportedAt),
		},
		Nodes: []elasticsearch.Node{
			{
				ID:                elasticsearch.StableNodeID(greenCluster, "fixture-node-data-hot"),
				Name:              "fixture-node-data-hot",
				Cluster:           greenCluster,
				Address:           "192.0.2.10",
				Roles:             []elasticsearch.Role{elasticsearch.RoleData, elasticsearch.RoleDataHot, elasticsearch.RoleIngest},
				HeapUsedBytes:     int64Value(40),
				HeapMaxBytes:      int64Value(100),
				DiskUsagePercent:  float64Value(75),
				CPUUsagePercent:   float64Value(48),
				IndexRate:         float64Value(8),
				SearchRate:        float64Value(12),
				Documents:         int64Value(120),
				StoreSizeBytes:    int64Value(80),
				ThreadPoolQueue:   int64Value(3),
				RejectedRate:      float64Value(1),
				UptimeSeconds:     int64Value(3600),
				DataNode:          true,
				CollectionTracked: true,
				ReportedAt:        reportedAt,
			},
			{
				ID:                elasticsearch.StableNodeID(greenCluster, "fixture-node-master"),
				Name:              "fixture-node-master",
				Cluster:           greenCluster,
				Address:           "192.0.2.11",
				Roles:             []elasticsearch.Role{elasticsearch.RoleMaster},
				HeapUsedBytes:     int64Value(30),
				HeapMaxBytes:      int64Value(100),
				DiskUsagePercent:  float64Value(40),
				CPUUsagePercent:   float64Value(18),
				ThreadPoolQueue:   int64Value(0),
				UptimeSeconds:     int64Value(3600),
				CollectionTracked: true,
				ReportedAt:        reportedAt,
			},
			{
				ID:                elasticsearch.StableNodeID(yellowCluster, "fixture-node-data-warm"),
				Name:              "fixture-node-data-warm",
				Cluster:           yellowCluster,
				Address:           "198.51.100.10",
				Roles:             []elasticsearch.Role{elasticsearch.RoleData, elasticsearch.RoleDataWarm},
				HeapUsedBytes:     int64Value(80),
				HeapMaxBytes:      int64Value(100),
				DiskUsagePercent:  float64Value(92),
				CPUUsagePercent:   float64Value(72),
				IndexRate:         float64Value(4),
				SearchRate:        float64Value(6),
				Documents:         int64Value(80),
				StoreSizeBytes:    int64Value(70),
				ThreadPoolQueue:   int64Value(7),
				RejectedRate:      float64Value(0),
				UptimeSeconds:     int64Value(1800),
				DataNode:          true,
				CollectionTracked: true,
				ReportedAt:        reportedAt,
			},
			{
				ID:         elasticsearch.StableNodeID(redCluster, "fixture-node-unavailable"),
				Name:       "fixture-node-unavailable",
				Cluster:    redCluster,
				Address:    "198.51.100.11",
				Roles:      []elasticsearch.Role{elasticsearch.RoleData, elasticsearch.RoleDataCold},
				DataNode:   true,
				ReportedAt: reportedAt,
			},
		},
	}
}

func fixtureCluster(name string, health elasticsearch.Health, availability elasticsearch.Availability, reportedAt time.Time) elasticsearch.Cluster {
	return elasticsearch.Cluster{
		ID:                    elasticsearch.StableClusterID(name),
		Name:                  name,
		Availability:          availability,
		NodeStatsAvailability: availability,
		Health:                health,
		NumberOfNodes:         int64Value(2),
		NumberOfDataNodes:     int64Value(1),
		ActivePrimaryShards:   int64Value(5),
		ActiveShards:          int64Value(10),
		RelocatingShards:      int64Value(0),
		InitializingShards:    int64Value(0),
		UnassignedShards:      int64Value(0),
		PendingTasks:          int64Value(0),
		TaskMaxWaitingMillis:  int64Value(0),
		CollectionTracked:     true,
		ReportedAt:            reportedAt,
	}
}
