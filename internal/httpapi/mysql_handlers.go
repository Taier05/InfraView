package httpapi

import (
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/mysql"
	"github.com/Taier05/InfraView/internal/service"
)

type mysqlOverviewView struct {
	Total             int                     `json:"total"`
	Normal            int                     `json:"normal"`
	Warning           int                     `json:"warning"`
	Critical          int                     `json:"critical"`
	Unknown           int                     `json:"unknown"`
	AffectedInstances int                     `json:"affected_instances"`
	WarningInstances  int                     `json:"warning_instances"`
	CriticalInstances int                     `json:"critical_instances"`
	Alerts            mysqlOverviewAlertsView `json:"alerts"`
}

type mysqlOverviewAlertsView struct {
	Availability       alertCountView `json:"availability"`
	ReplicationThreads alertCountView `json:"replication_threads"`
	ReplicationLag     alertCountView `json:"replication_lag"`
	ReplicationData    alertCountView `json:"replication_data"`
}

type mysqlReplicationView struct {
	State      service.MySQLReplicationState `json:"state"`
	LagSeconds *float64                      `json:"lag_seconds"`
	Level      service.Level                 `json:"level"`
}

type mysqlInstanceView struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	Address                string               `json:"address"`
	Host                   string               `json:"host"`
	Version                string               `json:"version"`
	Role                   mysql.Role           `json:"role"`
	Connections            *float64             `json:"connections"`
	MaxConnections         *float64             `json:"max_connections"`
	ConnectionUsagePercent *float64             `json:"connection_usage_percent"`
	ThreadsRunning         *float64             `json:"threads_running"`
	QPS                    *float64             `json:"qps"`
	TPS                    *float64             `json:"tps"`
	SlowQueriesPerSecond   *float64             `json:"slow_queries_per_second"`
	BufferPoolUsagePercent *float64             `json:"buffer_pool_usage_percent"`
	BufferPoolSizeBytes    *float64             `json:"buffer_pool_size_bytes"`
	UptimeSeconds          *float64             `json:"uptime_seconds"`
	Replication            mysqlReplicationView `json:"replication"`
	Status                 service.Level        `json:"status"`
	CollectionLevel        service.Level        `json:"collection_level"`
}

type mysqlInstancePageView struct {
	Instances       []mysqlInstanceView `json:"instances"`
	AvailableLabels []string            `json:"available_labels"`
	Total           int                 `json:"total"`
	Page            int                 `json:"page"`
	PageSize        int                 `json:"page_size"`
	TotalPages      int                 `json:"total_pages"`
}

func (a *api) mysqlOverview(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.mysqlServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.mysqlService.Overview(r.Context())
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, mysqlOverviewView{
		Total:             value.Total,
		Normal:            value.Normal,
		Warning:           value.Warning,
		Critical:          value.Critical,
		Unknown:           value.Unknown,
		AffectedInstances: value.AffectedInstances,
		WarningInstances:  value.WarningInstances,
		CriticalInstances: value.CriticalInstances,
		Alerts: mysqlOverviewAlertsView{
			Availability:       mysqlAlertCountView(value.Alerts.Availability),
			ReplicationThreads: mysqlAlertCountView(value.Alerts.ReplicationThreads),
			ReplicationLag:     mysqlAlertCountView(value.Alerts.ReplicationLag),
			ReplicationData:    mysqlAlertCountView(value.Alerts.ReplicationData),
		},
	}, meta)
}

func (a *api) mysqlInstances(w http.ResponseWriter, r *http.Request) {
	query, ok := queryParameters(r, "search", "label", "status", "role", "sort", "order", "page", "page_size")
	if !ok || hasEmptyMySQLQueryParameter(query) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	page, err := intQueryParameter(query, "page", 1)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	pageSize, err := intQueryParameter(query, "page_size", 20)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.mysqlServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.mysqlService.Instances(r.Context(), service.MySQLQuery{
		Search:   query.Get("search"),
		Label:    query.Get("label"),
		Status:   service.Level(query.Get("status")),
		Role:     mysql.Role(query.Get("role")),
		Sort:     query.Get("sort"),
		Order:    query.Get("order"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	instances := make([]mysqlInstanceView, len(value.Instances))
	for i, instance := range value.Instances {
		instances[i] = mysqlInstanceViewFrom(instance)
	}
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	availableLabels := make([]string, len(value.AvailableLabels))
	copy(availableLabels, value.AvailableLabels)
	writeSuccess(w, r, mysqlInstancePageView{
		Instances:       instances,
		AvailableLabels: availableLabels,
		Total:           value.Total,
		Page:            value.Page,
		PageSize:        value.PageSize,
		TotalPages:      totalPages,
	}, meta)
}

func hasEmptyMySQLQueryParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func (a *api) mysqlServiceAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.mysqlService != nil {
		return true
	}
	writeError(w, r, http.StatusServiceUnavailable, "mysql_unavailable", "数据源暂时不可用，请稍后重试", true)
	return false
}

func mysqlAlertCountView(value service.MySQLAlertCount) alertCountView {
	return alertCountView{Warning: value.Warning, Critical: value.Critical}
}

func mysqlInstanceViewFrom(value service.MySQLInstanceSummary) mysqlInstanceView {
	return mysqlInstanceView{
		ID:                     value.ID,
		Name:                   value.Name,
		Address:                value.Address,
		Host:                   value.Host,
		Version:                value.Version,
		Role:                   value.Role,
		Connections:            value.Connections,
		MaxConnections:         value.MaxConnections,
		ConnectionUsagePercent: value.ConnectionUsagePercent,
		ThreadsRunning:         value.ThreadsRunning,
		QPS:                    value.QPS,
		TPS:                    value.TPS,
		SlowQueriesPerSecond:   value.SlowQueriesPerSecond,
		BufferPoolUsagePercent: value.BufferPoolUsagePercent,
		BufferPoolSizeBytes:    value.BufferPoolSizeBytes,
		UptimeSeconds:          value.UptimeSeconds,
		Replication: mysqlReplicationView{
			State:      value.Replication.State,
			LagSeconds: value.Replication.LagSeconds,
			Level:      value.Replication.Level,
		},
		Status:          value.Status,
		CollectionLevel: value.CollectionLevel,
	}
}
