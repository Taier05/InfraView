package service

import "time"

type RabbitMQOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type RabbitMQQuery struct {
	Search   string
	Cluster  string
	Status   Level
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type RabbitMQNodeStatusSource string

const (
	RabbitMQStatusAlarm          RabbitMQNodeStatusSource = "alarm"
	RabbitMQStatusCollection     RabbitMQNodeStatusSource = "collection"
	RabbitMQStatusMemory         RabbitMQNodeStatusSource = "memory"
	RabbitMQStatusDisk           RabbitMQNodeStatusSource = "disk"
	RabbitMQStatusFileDescriptor RabbitMQNodeStatusSource = "file_descriptor"
	RabbitMQStatusErlangProcess  RabbitMQNodeStatusSource = "erlang_process"
	RabbitMQStatusNormal         RabbitMQNodeStatusSource = "normal"
	RabbitMQStatusUnknown        RabbitMQNodeStatusSource = "unknown"
)

type RabbitMQLevelCounts struct {
	Total    int
	Normal   int
	Warning  int
	Critical int
	Unknown  int
}

type RabbitMQAlertCount struct {
	Warning  int
	Critical int
	Unknown  int
}

type RabbitMQOverviewAlerts struct {
	ClusterConnectivity RabbitMQAlertCount
	ResourceAlarms      RabbitMQAlertCount
	ResourcePressure    RabbitMQAlertCount
	Collection          RabbitMQAlertCount
}

type RabbitMQOverview struct {
	Status   Level
	Clusters RabbitMQLevelCounts
	Nodes    RabbitMQLevelCounts
	Alerts   RabbitMQOverviewAlerts
}

type RabbitMQNodeSummary struct {
	ID                         string
	Name                       string
	Cluster                    string
	Address                    string
	Version                    string
	MemoryUsagePercent         *float64
	DiskAvailableBytes         *int64
	FileDescriptorUsagePercent *float64
	ErlangProcessUsagePercent  *float64
	Connections                *int64
	Queues                     *int64
	Messages                   *int64
	PublishRate                *float64
	DeliverRate                *float64
	UptimeSeconds              *int64
	Status                     Level
	StatusSource               RabbitMQNodeStatusSource
	CollectionLevel            Level
}

type RabbitMQPage struct {
	Nodes             []RabbitMQNodeSummary
	AvailableClusters []string
	Total             int
	Page              int
	PageSize          int
}
