package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
	"github.com/Taier05/InfraView/internal/service"
)

type metricValueView struct {
	Value *float64      `json:"value"`
	Level service.Level `json:"level"`
}

type overviewView struct {
	Total         int                `json:"total"`
	Online        int                `json:"online"`
	Offline       int                `json:"offline"`
	Unknown       int                `json:"unknown"`
	CPUAverage    metricValueView    `json:"cpu_average"`
	MemoryAverage metricValueView    `json:"memory_average"`
	Trends        []trendSeriesView  `json:"trends"`
	Alerts        overviewAlertsView `json:"alerts"`
}

type alertCountView struct {
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
}

type overviewAlertsView struct {
	AffectedHosts int            `json:"affected_hosts"`
	WarningHosts  int            `json:"warning_hosts"`
	CriticalHosts int            `json:"critical_hosts"`
	CPU           alertCountView `json:"cpu"`
	Memory        alertCountView `json:"memory"`
	IO            alertCountView `json:"io"`
	Network       alertCountView `json:"network"`
}

type trendSeriesView struct {
	Key    datasource.MetricKey `json:"key"`
	Unit   string               `json:"unit"`
	Points []metricPointView    `json:"points"`
}

type filesystemView struct {
	Mountpoint string          `json:"mountpoint"`
	Usage      metricValueView `json:"usage"`
}

type currentMetricsView struct {
	Timestamp                     time.Time        `json:"timestamp"`
	CPUUsage                      metricValueView  `json:"cpu_usage"`
	MemoryUsage                   metricValueView  `json:"memory_usage"`
	Load1                         metricValueView  `json:"load_1"`
	IOBusyPercent                 metricValueView  `json:"io_busy_percent"`
	DiskReadBytesPerSecond        metricValueView  `json:"disk_read_bytes_per_second"`
	DiskWriteBytesPerSecond       metricValueView  `json:"disk_write_bytes_per_second"`
	NetworkReceiveBytesPerSecond  metricValueView  `json:"network_receive_bytes_per_second"`
	NetworkTransmitBytesPerSecond metricValueView  `json:"network_transmit_bytes_per_second"`
	Filesystems                   []filesystemView `json:"filesystems"`
}

type hostView struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	IP               string                `json:"ip"`
	OS               string                `json:"os"`
	CPUCores         *int                  `json:"cpu_cores"`
	MemoryTotalBytes *int64                `json:"memory_total_bytes"`
	Status           datasource.HostStatus `json:"status"`
	StatusTime       time.Time             `json:"status_time"`
	UptimeSeconds    int64                 `json:"uptime_seconds"`
	Metrics          currentMetricsView    `json:"metrics"`
}

type hostPageView struct {
	Hosts      []hostView `json:"hosts"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

type metricPointView struct {
	Timestamp time.Time `json:"timestamp"`
	Value     *float64  `json:"value"`
}

type metricSeriesView struct {
	Metric datasource.MetricKey `json:"metric"`
	Points []metricPointView    `json:"points"`
}

type metricRangeView struct {
	HostID      string             `json:"host_id"`
	Range       string             `json:"range"`
	From        time.Time          `json:"from"`
	To          time.Time          `json:"to"`
	StepSeconds int64              `json:"step_seconds"`
	Series      []metricSeriesView `json:"series"`
}

type datasourceStatusView struct {
	Healthy   bool      `json:"healthy"`
	CheckedAt time.Time `json:"checked_at"`
}

func (a *api) overview(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r, "range") {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	rangeName := valueOrDefault(r, "range", "24h")
	value, meta, err := a.service.Overview(r.Context(), rangeName)
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, overviewView{
		Total:         value.Total,
		Online:        value.Online,
		Offline:       value.Offline,
		Unknown:       value.Unknown,
		CPUAverage:    metricView(value.CPUAverage),
		MemoryAverage: metricView(value.MemoryAverage),
		Trends:        trendViews(value.Trends),
		Alerts:        overviewAlertsViewFrom(value.Alerts),
	}, meta)
}

func overviewAlertsViewFrom(value service.OverviewAlerts) overviewAlertsView {
	return overviewAlertsView{
		AffectedHosts: value.AffectedHosts,
		WarningHosts:  value.WarningHosts,
		CriticalHosts: value.CriticalHosts,
		CPU:           alertCountView{Warning: value.CPU.Warning, Critical: value.CPU.Critical},
		Memory:        alertCountView{Warning: value.Memory.Warning, Critical: value.Memory.Critical},
		IO:            alertCountView{Warning: value.IO.Warning, Critical: value.IO.Critical},
		Network:       alertCountView{Warning: value.Network.Warning, Critical: value.Network.Critical},
	}
}

func trendViews(source []service.TrendSeries) []trendSeriesView {
	trends := make([]trendSeriesView, len(source))
	for i, series := range source {
		points := make([]metricPointView, len(series.Points))
		for j, point := range series.Points {
			points[j] = metricPointView{Timestamp: point.Timestamp, Value: point.Value}
		}
		trends[i] = trendSeriesView{Key: series.Key, Unit: series.Unit, Points: points}
	}
	return trends
}

func (a *api) hosts(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r, "q", "status", "sort", "order", "page", "page_size") {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	page, err := intParameter(r, "page", 1)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	pageSize, err := intParameter(r, "page_size", 20)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	value, meta, err := a.service.Hosts(r.Context(), service.HostQuery{
		Search:   r.URL.Query().Get("q"),
		Status:   datasource.HostStatus(r.URL.Query().Get("status")),
		Sort:     normalizedHostSort(r.URL.Query().Get("sort")),
		Order:    r.URL.Query().Get("order"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	hosts := make([]hostView, len(value.Hosts))
	for i, host := range value.Hosts {
		hosts[i] = summaryView(host)
	}
	totalPages := 0
	if value.Total > 0 {
		totalPages = (value.Total + value.PageSize - 1) / value.PageSize
	}
	writeSuccess(w, r, hostPageView{
		Hosts:      hosts,
		Total:      value.Total,
		Page:       value.Page,
		PageSize:   value.PageSize,
		TotalPages: totalPages,
	}, meta)
}

func normalizedHostSort(sortField string) string {
	if sortField == "load_1" {
		return "load"
	}
	return sortField
}

func (a *api) host(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	value, meta, err := a.service.Host(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, detailView(value), meta)
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r, "range") {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	rangeName := valueOrDefault(r, "range", "24h")
	value, meta, err := a.service.Metrics(r.Context(), r.PathValue("id"), rangeName)
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	series := make([]metricSeriesView, len(value.Series))
	for i, sourceSeries := range value.Series {
		points := make([]metricPointView, len(sourceSeries.Points))
		for j, point := range sourceSeries.Points {
			points[j] = metricPointView{Timestamp: point.Timestamp, Value: point.Value}
		}
		series[i] = metricSeriesView{Metric: sourceSeries.Metric, Points: points}
	}
	writeSuccess(w, r, metricRangeView{
		HostID:      value.HostID,
		Range:       value.Range,
		From:        value.From,
		To:          value.To,
		StepSeconds: int64(value.Step / time.Second),
		Series:      series,
	}, meta)
}

func (a *api) datasourceStatus(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	value, meta, err := a.service.DataSourceStatus(r.Context())
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, datasourceStatusView{Healthy: value.Healthy, CheckedAt: value.CheckedAt}, meta)
}

func (a *api) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidRange):
		writeError(w, r, http.StatusBadRequest, "invalid_range", "时间范围无效", false)
	case errors.Is(err, service.ErrInvalidQuery):
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
	case errors.Is(err, service.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "host_not_found", "该主机当前不在数据源中", false)
	case errors.Is(err, datasource.ErrUnavailable), errors.Is(err, datasource.ErrNotConfigured), errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusServiceUnavailable, "datasource_unavailable", "数据源暂时不可用，请稍后重试", true)
	default:
		a.logger.Error("只读查询失败", "request_id", requestIDFrom(r.Context()), "path", r.URL.Path, "error_type", "internal")
		writeError(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", true)
	}
}

func onlyQueryParameters(r *http.Request, allowed ...string) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}
	for name, values := range r.URL.Query() {
		if _, ok := allowedSet[name]; !ok || len(values) != 1 {
			return false
		}
	}
	return true
}

func valueOrDefault(r *http.Request, name, fallback string) string {
	if value := r.URL.Query().Get(name); value != "" {
		return value
	}
	return fallback
}

func intParameter(r *http.Request, name string, fallback int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func metricView(value service.MetricValue) metricValueView {
	return metricValueView{Value: value.Value, Level: value.Level}
}

func currentView(value service.CurrentMetrics) currentMetricsView {
	filesystems := make([]filesystemView, len(value.Filesystems))
	for i, filesystem := range value.Filesystems {
		filesystems[i] = filesystemView{Mountpoint: filesystem.Mountpoint, Usage: metricView(filesystem.Usage)}
	}
	return currentMetricsView{
		Timestamp:                     value.Timestamp,
		CPUUsage:                      metricView(value.CPUUsage),
		MemoryUsage:                   metricView(value.MemoryUsage),
		Load1:                         metricView(value.Load1),
		IOBusyPercent:                 metricView(value.IOBusyPercent),
		DiskReadBytesPerSecond:        metricView(value.DiskReadBytesPerSecond),
		DiskWriteBytesPerSecond:       metricView(value.DiskWriteBytesPerSecond),
		NetworkReceiveBytesPerSecond:  metricView(value.NetworkReceiveBytesPerSecond),
		NetworkTransmitBytesPerSecond: metricView(value.NetworkTransmitBytesPerSecond),
		Filesystems:                   filesystems,
	}
}

func summaryView(value service.HostSummary) hostView {
	return hostView{
		ID:               value.ID,
		Name:             value.Name,
		IP:               value.IP,
		OS:               value.OS,
		CPUCores:         value.CPUCores,
		MemoryTotalBytes: value.MemoryTotalBytes,
		Status:           value.Status,
		StatusTime:       value.StatusTime,
		UptimeSeconds:    int64(value.Uptime / time.Second),
		Metrics:          currentView(value.Metrics),
	}
}

func detailView(value service.HostDetail) hostView {
	return hostView{
		ID:               value.ID,
		Name:             value.Name,
		IP:               value.IP,
		OS:               value.OS,
		CPUCores:         value.CPUCores,
		MemoryTotalBytes: value.MemoryTotalBytes,
		Status:           value.Status,
		StatusTime:       value.StatusTime,
		UptimeSeconds:    int64(value.Uptime / time.Second),
		Metrics:          currentView(value.Metrics),
	}
}
