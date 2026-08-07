package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/javaapp"
	"github.com/Taier05/InfraView/internal/service"
)

type javaLevelCountsView struct {
	Total    int `json:"total"`
	Normal   int `json:"normal"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type javaAlertCountView struct {
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type javaOverviewAlertsView struct {
	Health     javaAlertCountView `json:"health"`
	Port       javaAlertCountView `json:"port"`
	Process    javaAlertCountView `json:"process"`
	Collection javaAlertCountView `json:"collection"`
}

type javaOverviewView struct {
	Status   service.Level          `json:"status"`
	Services javaLevelCountsView    `json:"services"`
	Alerts   javaOverviewAlertsView `json:"alerts"`
}

type javaServiceView struct {
	ID                        string                   `json:"id"`
	Name                      string                   `json:"name"`
	Business                  string                   `json:"business"`
	Address                   string                   `json:"address"`
	HealthUp                  *bool                    `json:"health_up"`
	HealthLatencyMilliseconds *float64                 `json:"health_latency_ms"`
	PortUp                    *bool                    `json:"port_up"`
	ProcessUp                 *bool                    `json:"process_up"`
	ProcessCount              *int64                   `json:"process_count"`
	PortConsistent            *bool                    `json:"port_consistent"`
	CPUUsagePercent           *float64                 `json:"cpu_usage_percent"`
	MemoryBytes               *int64                   `json:"memory_bytes"`
	MemoryUsagePercent        *float64                 `json:"memory_usage_percent"`
	UptimeSeconds             *int64                   `json:"uptime_seconds"`
	Status                    service.Level            `json:"status"`
	StatusSource              service.JavaStatusSource `json:"status_source"`
	CollectionLevel           service.Level            `json:"collection_level"`
}

type javaPageView struct {
	Services       []javaServiceView `json:"services"`
	AvailableNames []string          `json:"available_names"`
	Total          int               `json:"total"`
	Page           int               `json:"page"`
	PageSize       int               `json:"page_size"`
	TotalPages     int               `json:"total_pages"`
}

func (a *api) javaOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.javaServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.javaService.Overview(r.Context())
	if err != nil {
		a.writeJavaServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, javaOverviewView{
		Status:   value.Status,
		Services: javaLevelCountsViewFrom(value.Services),
		Alerts: javaOverviewAlertsView{
			Health:     javaAlertCountViewFrom(value.Alerts.Health),
			Port:       javaAlertCountViewFrom(value.Alerts.Port),
			Process:    javaAlertCountViewFrom(value.Alerts.Process),
			Collection: javaAlertCountViewFrom(value.Alerts.Collection),
		},
	}, meta)
}

func (a *api) javaServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	query, ok := queryParameters(r, "search", "name", "status", "sort", "direction", "page", "page_size")
	if !ok || hasEmptyJavaQueryParameter(query) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	page, err := intQueryParameter(query, "page", 1)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	pageSize, err := intQueryParameter(query, "page_size", 20)
	if err != nil || page < 1 || pageSize < 1 {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.javaServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.javaService.Services(r.Context(), service.JavaQuery{
		Search:   query.Get("search"),
		Name:     query.Get("name"),
		Status:   service.Level(query.Get("status")),
		Sort:     query.Get("sort"),
		Order:    query.Get("direction"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeJavaServiceError(w, r, err)
		return
	}
	services := make([]javaServiceView, len(value.Services))
	for index, item := range value.Services {
		services[index] = javaServiceViewFrom(item)
	}
	availableNames := make([]string, len(value.AvailableNames))
	copy(availableNames, value.AvailableNames)
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	writeSuccess(w, r, javaPageView{
		Services:       services,
		AvailableNames: availableNames,
		Total:          value.Total,
		Page:           value.Page,
		PageSize:       value.PageSize,
		TotalPages:     totalPages,
	}, meta)
}

func hasEmptyJavaQueryParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func (a *api) javaServiceAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.javaService != nil {
		return true
	}
	writeError(w, r, http.StatusServiceUnavailable, "java_unavailable", "数据源暂时不可用，请稍后重试", true)
	return false
}

func (a *api) writeJavaServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidQuery):
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
	case errors.Is(err, javaapp.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusServiceUnavailable, "java_unavailable", "数据源暂时不可用，请稍后重试", true)
	default:
		a.logger.Error("只读查询失败", "request_id", requestIDFrom(r.Context()), "path", r.URL.Path, "error_type", "internal")
		writeError(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", true)
	}
}

func javaLevelCountsViewFrom(value service.JavaLevelCounts) javaLevelCountsView {
	return javaLevelCountsView{Total: value.Total, Normal: value.Normal, Warning: value.Warning, Critical: value.Critical, Unknown: value.Unknown}
}

func javaAlertCountViewFrom(value service.JavaAlertCount) javaAlertCountView {
	return javaAlertCountView{Warning: value.Warning, Critical: value.Critical, Unknown: value.Unknown}
}

func javaServiceViewFrom(value service.JavaServiceSummary) javaServiceView {
	return javaServiceView{
		ID:                        value.ID,
		Name:                      value.Name,
		Business:                  value.Business,
		Address:                   value.Address,
		HealthUp:                  value.HealthUp,
		HealthLatencyMilliseconds: value.HealthLatencyMilliseconds,
		PortUp:                    value.PortUp,
		ProcessUp:                 value.ProcessUp,
		ProcessCount:              value.ProcessCount,
		PortConsistent:            value.PortConsistent,
		CPUUsagePercent:           value.CPUUsagePercent,
		MemoryBytes:               value.MemoryBytes,
		MemoryUsagePercent:        value.MemoryUsagePercent,
		UptimeSeconds:             value.UptimeSeconds,
		Status:                    value.Status,
		StatusSource:              value.StatusSource,
		CollectionLevel:           value.CollectionLevel,
	}
}
