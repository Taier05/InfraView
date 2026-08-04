package service

import (
	"time"

	"github.com/Taier05/InfraView/internal/elasticsearch"
)

type ElasticsearchOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type ElasticsearchQuery struct {
	Search        string
	Cluster       string
	Role          elasticsearch.Role
	ClusterHealth elasticsearch.Health
	Status        Level
	Sort          string
	Order         string
	Page          int
	PageSize      int
}

type ElasticsearchNodeStatusSource string

const (
	ElasticsearchNodeStatusCollection ElasticsearchNodeStatusSource = "collection"
	ElasticsearchNodeStatusDisk       ElasticsearchNodeStatusSource = "disk"
	ElasticsearchNodeStatusJVM        ElasticsearchNodeStatusSource = "jvm"
	ElasticsearchNodeStatusThreadPool ElasticsearchNodeStatusSource = "thread_pool"
	ElasticsearchNodeStatusNormal     ElasticsearchNodeStatusSource = "normal"
	ElasticsearchNodeStatusUnknown    ElasticsearchNodeStatusSource = "unknown"
)

type ElasticsearchClusterStatusSource string

const (
	ElasticsearchClusterStatusAvailability ElasticsearchClusterStatusSource = "availability"
	ElasticsearchClusterStatusHealth       ElasticsearchClusterStatusSource = "health"
	ElasticsearchClusterStatusCollection   ElasticsearchClusterStatusSource = "collection"
	ElasticsearchClusterStatusNormal       ElasticsearchClusterStatusSource = "normal"
	ElasticsearchClusterStatusUnknown      ElasticsearchClusterStatusSource = "unknown"
)

type ElasticsearchLevelCounts struct {
	Total    int
	Normal   int
	Warning  int
	Critical int
	Unknown  int
}

type ElasticsearchAlertCount struct {
	Warning  int
	Critical int
}

type ElasticsearchOverviewAlerts struct {
	ClusterHealth     ElasticsearchAlertCount
	NodeResource      ElasticsearchAlertCount
	UnassignedShards  ElasticsearchAlertCount
	RequestRejections ElasticsearchAlertCount
}

type ElasticsearchOverview struct {
	Status   Level
	Clusters ElasticsearchLevelCounts
	Nodes    ElasticsearchLevelCounts
	Alerts   ElasticsearchOverviewAlerts
}

type ElasticsearchNodeSummary struct {
	ID               string
	Name             string
	Cluster          string
	Address          string
	Roles            []elasticsearch.Role
	ClusterHealth    elasticsearch.Health
	HeapUsagePercent *float64
	DiskUsagePercent *float64
	CPUUsagePercent  *float64
	IndexRate        *float64
	SearchRate       *float64
	Documents        *int64
	StoreSizeBytes   *int64
	ThreadPoolQueue  *int64
	RejectedRate     *float64
	UptimeSeconds    *int64
	Status           Level
	StatusSource     ElasticsearchNodeStatusSource
	CollectionLevel  Level
}

type ElasticsearchPage struct {
	Nodes             []ElasticsearchNodeSummary
	AvailableClusters []string
	AvailableRoles    []elasticsearch.Role
	Total             int
	Page              int
	PageSize          int
}
