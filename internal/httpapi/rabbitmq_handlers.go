package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/rabbitmq"
	"github.com/Taier05/InfraView/internal/service"
)

type rabbitMQLevelCountsView struct {
	Total    int `json:"total"`
	Normal   int `json:"normal"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type rabbitMQAlertCountView struct {
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type rabbitMQOverviewAlertsView struct {
	ClusterConnectivity rabbitMQAlertCountView `json:"cluster_connectivity"`
	ResourceAlarms      rabbitMQAlertCountView `json:"resource_alarms"`
	ResourcePressure    rabbitMQAlertCountView `json:"resource_pressure"`
	Collection          rabbitMQAlertCountView `json:"collection"`
}

type rabbitMQOverviewView struct {
	Status   service.Level              `json:"status"`
	Clusters rabbitMQLevelCountsView    `json:"clusters"`
	Nodes    rabbitMQLevelCountsView    `json:"nodes"`
	Alerts   rabbitMQOverviewAlertsView `json:"alerts"`
}

type rabbitMQNodeView struct {
	ID                         string                           `json:"id"`
	Name                       string                           `json:"name"`
	Cluster                    string                           `json:"cluster"`
	Address                    string                           `json:"address"`
	Version                    string                           `json:"version"`
	MemoryUsagePercent         *float64                         `json:"memory_usage_percent"`
	DiskAvailableBytes         *int64                           `json:"disk_available_bytes"`
	FileDescriptorUsagePercent *float64                         `json:"file_descriptor_usage_percent"`
	ErlangProcessUsagePercent  *float64                         `json:"erlang_process_usage_percent"`
	Connections                *int64                           `json:"connections"`
	Queues                     *int64                           `json:"queues"`
	Messages                   *int64                           `json:"messages"`
	PublishRate                *float64                         `json:"publish_rate"`
	DeliverRate                *float64                         `json:"deliver_rate"`
	UptimeSeconds              *int64                           `json:"uptime_seconds"`
	Status                     service.Level                    `json:"status"`
	StatusSource               service.RabbitMQNodeStatusSource `json:"status_source"`
	CollectionLevel            service.Level                    `json:"collection_level"`
}

type rabbitMQPageView struct {
	Nodes             []rabbitMQNodeView `json:"nodes"`
	AvailableClusters []string           `json:"available_clusters"`
	Total             int                `json:"total"`
	Page              int                `json:"page"`
	PageSize          int                `json:"page_size"`
	TotalPages        int                `json:"total_pages"`
}

func (a *api) rabbitMQOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.rabbitMQServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.rabbitMQService.Overview(r.Context())
	if err != nil {
		a.writeRabbitMQServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, rabbitMQOverviewView{
		Status:   value.Status,
		Clusters: rabbitMQLevelCountsViewFrom(value.Clusters),
		Nodes:    rabbitMQLevelCountsViewFrom(value.Nodes),
		Alerts: rabbitMQOverviewAlertsView{
			ClusterConnectivity: rabbitMQAlertCountViewFrom(value.Alerts.ClusterConnectivity),
			ResourceAlarms:      rabbitMQAlertCountViewFrom(value.Alerts.ResourceAlarms),
			ResourcePressure:    rabbitMQAlertCountViewFrom(value.Alerts.ResourcePressure),
			Collection:          rabbitMQAlertCountViewFrom(value.Alerts.Collection),
		},
	}, meta)
}

func (a *api) rabbitMQNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	query, ok := queryParameters(r, "search", "cluster", "status", "sort", "direction", "page", "page_size")
	if !ok || hasEmptyRabbitMQQueryParameter(query) {
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
	sort, ok := rabbitMQServiceSort(query.Get("sort"))
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.rabbitMQServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.rabbitMQService.Nodes(r.Context(), service.RabbitMQQuery{
		Search:   query.Get("search"),
		Cluster:  query.Get("cluster"),
		Status:   service.Level(query.Get("status")),
		Sort:     sort,
		Order:    query.Get("direction"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeRabbitMQServiceError(w, r, err)
		return
	}
	nodes := make([]rabbitMQNodeView, len(value.Nodes))
	for index, node := range value.Nodes {
		nodes[index] = rabbitMQNodeViewFrom(node)
	}
	availableClusters := make([]string, len(value.AvailableClusters))
	copy(availableClusters, value.AvailableClusters)
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	writeSuccess(w, r, rabbitMQPageView{
		Nodes:             nodes,
		AvailableClusters: availableClusters,
		Total:             value.Total,
		Page:              value.Page,
		PageSize:          value.PageSize,
		TotalPages:        totalPages,
	}, meta)
}

func hasEmptyRabbitMQQueryParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func rabbitMQServiceSort(value string) (string, bool) {
	switch value {
	case "":
		return "", true
	case "file_descriptors":
		return "file_descriptor", true
	case "erlang_processes":
		return "erlang_process", true
	case "node", "cluster", "address", "version", "memory", "disk", "connections", "queues", "messages", "publish_rate", "deliver_rate", "uptime", "status":
		return value, true
	default:
		return "", false
	}
}

func (a *api) rabbitMQServiceAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.rabbitMQService != nil {
		return true
	}
	writeError(w, r, http.StatusServiceUnavailable, "rabbitmq_unavailable", "数据源暂时不可用，请稍后重试", true)
	return false
}

func (a *api) writeRabbitMQServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidQuery):
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
	case errors.Is(err, rabbitmq.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusServiceUnavailable, "rabbitmq_unavailable", "数据源暂时不可用，请稍后重试", true)
	default:
		a.logger.Error("只读查询失败", "request_id", requestIDFrom(r.Context()), "path", r.URL.Path, "error_type", "internal")
		writeError(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", true)
	}
}

func rabbitMQLevelCountsViewFrom(value service.RabbitMQLevelCounts) rabbitMQLevelCountsView {
	return rabbitMQLevelCountsView{
		Total: value.Total, Normal: value.Normal, Warning: value.Warning, Critical: value.Critical, Unknown: value.Unknown,
	}
}

func rabbitMQAlertCountViewFrom(value service.RabbitMQAlertCount) rabbitMQAlertCountView {
	return rabbitMQAlertCountView{Warning: value.Warning, Critical: value.Critical, Unknown: value.Unknown}
}

func rabbitMQNodeViewFrom(value service.RabbitMQNodeSummary) rabbitMQNodeView {
	return rabbitMQNodeView{
		ID: value.ID, Name: value.Name, Cluster: value.Cluster, Address: value.Address, Version: value.Version,
		MemoryUsagePercent: value.MemoryUsagePercent, DiskAvailableBytes: value.DiskAvailableBytes,
		FileDescriptorUsagePercent: value.FileDescriptorUsagePercent, ErlangProcessUsagePercent: value.ErlangProcessUsagePercent,
		Connections: value.Connections, Queues: value.Queues, Messages: value.Messages, PublishRate: value.PublishRate,
		DeliverRate: value.DeliverRate, UptimeSeconds: value.UptimeSeconds, Status: value.Status,
		StatusSource: value.StatusSource, CollectionLevel: value.CollectionLevel,
	}
}
