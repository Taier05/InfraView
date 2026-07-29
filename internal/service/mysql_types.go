package service

import (
	"time"

	"github.com/Taier05/InfraView/internal/mysql"
)

type MySQLOptions struct {
	CurrentMetricsTTL time.Duration
	MaxStale          time.Duration
	Clock             func() time.Time
}

type MySQLQuery struct {
	Search   string
	Label    string
	Status   Level
	Role     mysql.Role
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type MySQLAlertCount struct {
	Warning  int
	Critical int
}

type MySQLOverviewAlerts struct {
	Availability       MySQLAlertCount
	ReplicationThreads MySQLAlertCount
	ReplicationLag     MySQLAlertCount
	ReplicationData    MySQLAlertCount
}

type MySQLOverview struct {
	Total             int
	Normal            int
	Warning           int
	Critical          int
	Unknown           int
	AffectedInstances int
	WarningInstances  int
	CriticalInstances int
	Alerts            MySQLOverviewAlerts
}

type MySQLPage struct {
	Instances       []MySQLInstanceSummary
	AvailableLabels []string
	Total           int
	Page            int
	PageSize        int
}

type MySQLReplicationState string

const (
	ReplicationNormal         MySQLReplicationState = "normal"
	ReplicationThreadsStopped MySQLReplicationState = "threads_stopped"
	ReplicationNotConfigured  MySQLReplicationState = "not_configured"
	ReplicationUnknown        MySQLReplicationState = "unknown"
)

type MySQLReplicationSummary struct {
	State      MySQLReplicationState
	LagSeconds *float64
	Level      Level
}

type MySQLInstanceSummary struct {
	ID                     string
	Name                   string
	Address                string
	Host                   string
	Version                string
	Role                   mysql.Role
	Connections            *float64
	MaxConnections         *float64
	ConnectionUsagePercent *float64
	ThreadsRunning         *float64
	QPS                    *float64
	SlowQueriesPerSecond   *float64
	BufferPoolUsagePercent *float64
	BufferPoolSizeBytes    *float64
	UptimeSeconds          *float64
	Replication            MySQLReplicationSummary
	Status                 Level
}
