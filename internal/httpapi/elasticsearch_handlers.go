package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/elasticsearch"
	"github.com/Taier05/InfraView/internal/service"
)

type elasticsearchLevelCountsView struct {
	Total    int `json:"total"`
	Normal   int `json:"normal"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type elasticsearchOverviewAlertsView struct {
	ClusterHealth     alertCountView `json:"cluster_health"`
	NodeResource      alertCountView `json:"node_resource"`
	UnassignedShards  alertCountView `json:"unassigned_shards"`
	RequestRejections alertCountView `json:"request_rejections"`
}

type elasticsearchOverviewView struct {
	Status   service.Level                   `json:"status"`
	Clusters elasticsearchLevelCountsView    `json:"clusters"`
	Nodes    elasticsearchLevelCountsView    `json:"nodes"`
	Alerts   elasticsearchOverviewAlertsView `json:"alerts"`
}

type elasticsearchNodeView struct {
	ID               string                                `json:"id"`
	Name             string                                `json:"name"`
	Cluster          string                                `json:"cluster"`
	Address          string                                `json:"address"`
	Roles            []elasticsearch.Role                  `json:"roles"`
	ClusterHealth    elasticsearch.Health                  `json:"cluster_health"`
	HeapUsagePercent *float64                              `json:"heap_usage_percent"`
	DiskUsagePercent *float64                              `json:"disk_usage_percent"`
	CPUUsagePercent  *float64                              `json:"cpu_usage_percent"`
	IndexRate        *float64                              `json:"index_rate"`
	SearchRate       *float64                              `json:"search_rate"`
	Documents        *int64                                `json:"documents"`
	StoreSizeBytes   *int64                                `json:"store_size_bytes"`
	ThreadPoolQueue  *int64                                `json:"thread_pool_queue"`
	RejectedRate     *float64                              `json:"rejected_rate"`
	UptimeSeconds    *int64                                `json:"uptime_seconds"`
	Status           service.Level                         `json:"status"`
	StatusSource     service.ElasticsearchNodeStatusSource `json:"status_source"`
	CollectionLevel  service.Level                         `json:"collection_level"`
}

type elasticsearchPageView struct {
	Nodes             []elasticsearchNodeView `json:"nodes"`
	AvailableClusters []string                `json:"available_clusters"`
	AvailableRoles    []elasticsearch.Role    `json:"available_roles"`
	Total             int                     `json:"total"`
	Page              int                     `json:"page"`
	PageSize          int                     `json:"page_size"`
	TotalPages        int                     `json:"total_pages"`
}

func (a *api) elasticsearchOverview(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.elasticsearchServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.elasticsearchService.Overview(r.Context())
	if err != nil {
		a.writeElasticsearchServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, elasticsearchOverviewView{
		Status:   value.Status,
		Clusters: elasticsearchLevelCountsViewFrom(value.Clusters),
		Nodes:    elasticsearchLevelCountsViewFrom(value.Nodes),
		Alerts: elasticsearchOverviewAlertsView{
			ClusterHealth:     elasticsearchAlertCountView(value.Alerts.ClusterHealth),
			NodeResource:      elasticsearchAlertCountView(value.Alerts.NodeResource),
			UnassignedShards:  elasticsearchAlertCountView(value.Alerts.UnassignedShards),
			RequestRejections: elasticsearchAlertCountView(value.Alerts.RequestRejections),
		},
	}, meta)
}

func (a *api) elasticsearchNodes(w http.ResponseWriter, r *http.Request) {
	query, ok := queryParameters(r, "search", "cluster", "role", "cluster_health", "status", "sort", "order", "page", "page_size")
	if !ok || hasEmptyElasticsearchQueryParameter(query) {
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
	if page < 1 || pageSize < 1 {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.elasticsearchServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.elasticsearchService.Nodes(r.Context(), service.ElasticsearchQuery{
		Search:        query.Get("search"),
		Cluster:       query.Get("cluster"),
		Role:          elasticsearch.Role(query.Get("role")),
		ClusterHealth: elasticsearch.Health(query.Get("cluster_health")),
		Status:        service.Level(query.Get("status")),
		Sort:          query.Get("sort"),
		Order:         query.Get("order"),
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		a.writeElasticsearchServiceError(w, r, err)
		return
	}
	nodes := make([]elasticsearchNodeView, len(value.Nodes))
	for index, node := range value.Nodes {
		nodes[index] = elasticsearchNodeViewFrom(node)
	}
	availableClusters := make([]string, len(value.AvailableClusters))
	copy(availableClusters, value.AvailableClusters)
	availableRoles := make([]elasticsearch.Role, len(value.AvailableRoles))
	copy(availableRoles, value.AvailableRoles)
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	writeSuccess(w, r, elasticsearchPageView{
		Nodes:             nodes,
		AvailableClusters: availableClusters,
		AvailableRoles:    availableRoles,
		Total:             value.Total,
		Page:              value.Page,
		PageSize:          value.PageSize,
		TotalPages:        totalPages,
	}, meta)
}

func hasEmptyElasticsearchQueryParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func (a *api) elasticsearchServiceAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.elasticsearchService != nil {
		return true
	}
	writeError(w, r, http.StatusServiceUnavailable, "elasticsearch_unavailable", "数据源暂时不可用，请稍后重试", true)
	return false
}

func (a *api) writeElasticsearchServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidQuery):
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
	case errors.Is(err, elasticsearch.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusServiceUnavailable, "elasticsearch_unavailable", "数据源暂时不可用，请稍后重试", true)
	default:
		a.logger.Error("只读查询失败", "request_id", requestIDFrom(r.Context()), "path", r.URL.Path, "error_type", "internal")
		writeError(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", true)
	}
}

func elasticsearchLevelCountsViewFrom(value service.ElasticsearchLevelCounts) elasticsearchLevelCountsView {
	return elasticsearchLevelCountsView{
		Total:    value.Total,
		Normal:   value.Normal,
		Warning:  value.Warning,
		Critical: value.Critical,
		Unknown:  value.Unknown,
	}
}

func elasticsearchAlertCountView(value service.ElasticsearchAlertCount) alertCountView {
	return alertCountView{Warning: value.Warning, Critical: value.Critical}
}

func elasticsearchNodeViewFrom(value service.ElasticsearchNodeSummary) elasticsearchNodeView {
	roles := make([]elasticsearch.Role, len(value.Roles))
	copy(roles, value.Roles)
	return elasticsearchNodeView{
		ID:               value.ID,
		Name:             value.Name,
		Cluster:          value.Cluster,
		Address:          value.Address,
		Roles:            roles,
		ClusterHealth:    value.ClusterHealth,
		HeapUsagePercent: value.HeapUsagePercent,
		DiskUsagePercent: value.DiskUsagePercent,
		CPUUsagePercent:  value.CPUUsagePercent,
		IndexRate:        value.IndexRate,
		SearchRate:       value.SearchRate,
		Documents:        value.Documents,
		StoreSizeBytes:   value.StoreSizeBytes,
		ThreadPoolQueue:  value.ThreadPoolQueue,
		RejectedRate:     value.RejectedRate,
		UptimeSeconds:    value.UptimeSeconds,
		Status:           value.Status,
		StatusSource:     value.StatusSource,
		CollectionLevel:  value.CollectionLevel,
	}
}
