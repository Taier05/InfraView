package httpapi

import (
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/redis"
	"github.com/Taier05/InfraView/internal/service"
)

type redisOverviewView struct {
	Total             int                     `json:"total"`
	Normal            int                     `json:"normal"`
	Warning           int                     `json:"warning"`
	Critical          int                     `json:"critical"`
	Unknown           int                     `json:"unknown"`
	AffectedInstances int                     `json:"affected_instances"`
	WarningInstances  int                     `json:"warning_instances"`
	CriticalInstances int                     `json:"critical_instances"`
	Roles             redisRoleCountsView     `json:"roles"`
	Alerts            redisOverviewAlertsView `json:"alerts"`
}

type redisRoleCountsView struct {
	Master  int `json:"master"`
	Slave   int `json:"slave"`
	Unknown int `json:"unknown"`
}

type redisOverviewAlertsView struct {
	Availability alertCountView `json:"availability"`
	Memory       alertCountView `json:"memory"`
	Connection   alertCountView `json:"connection"`
	Replication  alertCountView `json:"replication"`
}

type redisReplicationView struct {
	ConnectedReplicas      *int64   `json:"connected_replicas"`
	MasterLinkUp           *bool    `json:"master_link_up"`
	MasterLastIOSecondsAgo *float64 `json:"master_last_io_seconds_ago"`
	MasterSyncInProgress   *bool    `json:"master_sync_in_progress"`
	WorstReplicaLagSeconds *float64 `json:"worst_replica_lag_seconds"`
}

type redisInstanceView struct {
	ID                      string                    `json:"id"`
	Address                 string                    `json:"address"`
	Availability            redis.Availability        `json:"availability"`
	Role                    redis.Role                `json:"role"`
	ClusterEnabled          *bool                     `json:"cluster_enabled"`
	UsedMemoryBytes         *int64                    `json:"used_memory_bytes"`
	MaxMemoryBytes          *int64                    `json:"max_memory_bytes"`
	MemoryUsagePercent      *float64                  `json:"memory_usage_percent"`
	ConnectedClients        *int64                    `json:"connected_clients"`
	MaxClients              *int64                    `json:"max_clients"`
	ConnectionUsagePercent  *float64                  `json:"connection_usage_percent"`
	BlockedClients          *int64                    `json:"blocked_clients"`
	QPS                     *float64                  `json:"qps"`
	HitRate                 *float64                  `json:"hit_rate"`
	Keys                    *int64                    `json:"keys"`
	ExpiredKeysPerSecond    *float64                  `json:"expired_keys_per_second"`
	EvictedKeysPerSecond    *float64                  `json:"evicted_keys_per_second"`
	RejectedConnectionsRate *float64                  `json:"rejected_connections_rate"`
	Replication             redisReplicationView      `json:"replication"`
	UptimeSeconds           *int64                    `json:"uptime_seconds"`
	Status                  service.Level             `json:"status"`
	StatusSource            service.RedisStatusSource `json:"status_source"`
	CollectionLevel         service.Level             `json:"collection_level"`
}

type redisPageView struct {
	Instances  []redisInstanceView `json:"instances"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

func (a *api) redisOverview(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, 400, "invalid_query", "查询参数无效", false)
		return
	}
	if a.redisService == nil {
		writeError(w, r, 503, "redis_unavailable", "数据源暂时不可用，请稍后重试", true)
		return
	}
	value, meta, err := a.redisService.Overview(r.Context())
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, redisOverviewView{
		Total:             value.Total,
		Normal:            value.Normal,
		Warning:           value.Warning,
		Critical:          value.Critical,
		Unknown:           value.Unknown,
		AffectedInstances: value.AffectedInstances,
		WarningInstances:  value.WarningInstances,
		CriticalInstances: value.CriticalInstances,
		Roles: redisRoleCountsView{
			Master:  value.Roles.Master,
			Slave:   value.Roles.Slave,
			Unknown: value.Roles.Unknown,
		},
		Alerts: redisOverviewAlertsView{
			Availability: redisAlertView(value.Alerts.Availability),
			Memory:       redisAlertView(value.Alerts.Memory),
			Connection:   redisAlertView(value.Alerts.Connection),
			Replication:  redisAlertView(value.Alerts.Replication),
		},
	}, meta)
}

func (a *api) redisInstances(w http.ResponseWriter, r *http.Request) {
	query, ok := queryParameters(r, "search", "role", "status", "sort", "order", "page", "page_size")
	if !ok || hasEmptyRedisParameter(query) {
		writeError(w, r, 400, "invalid_query", "查询参数无效", false)
		return
	}
	page, err := intQueryParameter(query, "page", 1)
	if err != nil {
		writeError(w, r, 400, "invalid_query", "查询参数无效", false)
		return
	}
	size, err := intQueryParameter(query, "page_size", 20)
	if err != nil {
		writeError(w, r, 400, "invalid_query", "查询参数无效", false)
		return
	}
	if a.redisService == nil {
		writeError(w, r, 503, "redis_unavailable", "数据源暂时不可用，请稍后重试", true)
		return
	}
	value, meta, err := a.redisService.Instances(r.Context(), service.RedisQuery{
		Search:   query.Get("search"),
		Role:     redis.Role(query.Get("role")),
		Status:   service.Level(query.Get("status")),
		Sort:     query.Get("sort"),
		Order:    query.Get("order"),
		Page:     page,
		PageSize: size,
	})
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	items := make([]redisInstanceView, len(value.Instances))
	for index, item := range value.Instances {
		items[index] = redisInstanceViewFrom(item)
	}
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	writeSuccess(w, r, redisPageView{
		Instances:  items,
		Total:      value.Total,
		Page:       value.Page,
		PageSize:   value.PageSize,
		TotalPages: totalPages,
	}, meta)
}

func hasEmptyRedisParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func redisAlertView(value service.RedisAlertCount) alertCountView {
	return alertCountView{Warning: value.Warning, Critical: value.Critical}
}

func redisInstanceViewFrom(value service.RedisInstanceSummary) redisInstanceView {
	return redisInstanceView{
		ID:                      value.ID,
		Address:                 value.Address,
		Availability:            value.Availability,
		Role:                    value.Role,
		ClusterEnabled:          value.ClusterEnabled,
		UsedMemoryBytes:         value.UsedMemoryBytes,
		MaxMemoryBytes:          value.MaxMemoryBytes,
		MemoryUsagePercent:      value.MemoryUsagePercent,
		ConnectedClients:        value.ConnectedClients,
		MaxClients:              value.MaxClients,
		ConnectionUsagePercent:  value.ConnectionUsagePercent,
		BlockedClients:          value.BlockedClients,
		QPS:                     value.QPS,
		HitRate:                 value.HitRate,
		Keys:                    value.Keys,
		ExpiredKeysPerSecond:    value.ExpiredKeysPerSecond,
		EvictedKeysPerSecond:    value.EvictedKeysPerSecond,
		RejectedConnectionsRate: value.RejectedConnectionsRate,
		Replication: redisReplicationView{
			ConnectedReplicas:      value.Replication.ConnectedReplicas,
			MasterLinkUp:           value.Replication.MasterLinkUp,
			MasterLastIOSecondsAgo: value.Replication.MasterLastIOSecondsAgo,
			MasterSyncInProgress:   value.Replication.MasterSyncInProgress,
			WorstReplicaLagSeconds: value.Replication.WorstReplicaLagSeconds,
		},
		UptimeSeconds:   value.UptimeSeconds,
		Status:          value.Status,
		StatusSource:    value.StatusSource,
		CollectionLevel: value.CollectionLevel,
	}
}
