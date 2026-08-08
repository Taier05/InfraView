package httpapi

import (
	"net/http"
	"net/url"

	"github.com/Taier05/InfraView/internal/disk"
	"github.com/Taier05/InfraView/internal/service"
)

type diskOverviewView struct {
	Total           int                    `json:"total"`
	Normal          int                    `json:"normal"`
	Warning         int                    `json:"warning"`
	Critical        int                    `json:"critical"`
	Unknown         int                    `json:"unknown"`
	AffectedDevices int                    `json:"affected_devices"`
	WarningDevices  int                    `json:"warning_devices"`
	CriticalDevices int                    `json:"critical_devices"`
	Alerts          diskOverviewAlertsView `json:"alerts"`
}

type diskOverviewAlertsView struct {
	SMARTHealth      alertCountView `json:"smart_health"`
	DeviceWarning    alertCountView `json:"device_warning"`
	AttributeFailure alertCountView `json:"attribute_failure"`
	Collection       alertCountView `json:"collection"`
}

type diskErrorsView struct {
	PendingSectors       *float64 `json:"pending_sectors"`
	ReallocatedSectors   *float64 `json:"reallocated_sectors"`
	UncorrectableSectors *float64 `json:"uncorrectable_sectors"`
	UDMACRCErrors        *float64 `json:"udma_crc_errors"`
	MediaIntegrityErrors *float64 `json:"media_integrity_errors"`
	ErrorLogEntries      *float64 `json:"error_log_entries"`
	CommandTimeouts      *float64 `json:"command_timeouts"`
	UnsafeShutdowns      *float64 `json:"unsafe_shutdowns"`
}

type diskDeviceView struct {
	ID                  string                   `json:"id"`
	Host                string                   `json:"host"`
	Device              string                   `json:"device"`
	Model               string                   `json:"model"`
	CapacityBytes       *int64                   `json:"capacity_bytes"`
	SMARTHealth         disk.Health              `json:"smart_health"`
	TemperatureCelsius  *float64                 `json:"temperature_celsius"`
	LifetimeUsedPercent *float64                 `json:"lifetime_used_percent"`
	PowerOnHours        *float64                 `json:"power_on_hours"`
	Errors              diskErrorsView           `json:"errors"`
	Status              service.Level            `json:"status"`
	StatusSource        service.DiskStatusSource `json:"status_source"`
	CollectionLevel     service.Level            `json:"collection_level"`
}

type diskDevicePageView struct {
	Devices    []diskDeviceView `json:"devices"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

func (a *api) diskOverview(w http.ResponseWriter, r *http.Request) {
	if !onlyQueryParameters(r) {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.diskServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.diskService.Overview(r.Context())
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	writeSuccess(w, r, diskOverviewView{
		Total:           value.Total,
		Normal:          value.Normal,
		Warning:         value.Warning,
		Critical:        value.Critical,
		Unknown:         value.Unknown,
		AffectedDevices: value.AffectedDevices,
		WarningDevices:  value.WarningDevices,
		CriticalDevices: value.CriticalDevices,
		Alerts: diskOverviewAlertsView{
			SMARTHealth:      diskAlertCountView(value.Alerts.SMARTHealth),
			DeviceWarning:    diskAlertCountView(value.Alerts.DeviceWarning),
			AttributeFailure: diskAlertCountView(value.Alerts.AttributeFailure),
			Collection:       diskAlertCountView(value.Alerts.Collection),
		},
	}, meta)
}

func (a *api) diskDevices(w http.ResponseWriter, r *http.Request) {
	query, ok := queryParameters(r, "search", "status", "sort", "order", "page", "page_size")
	if !ok || hasEmptyDiskQueryParameter(query) {
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
	if page > 1 && pageSize > 0 && page-1 > int(^uint(0)>>1)/pageSize {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "查询参数无效", false)
		return
	}
	if !a.diskServiceAvailable(w, r) {
		return
	}
	value, meta, err := a.diskService.Devices(r.Context(), service.DiskQuery{
		Search:   query.Get("search"),
		Status:   service.Level(query.Get("status")),
		Sort:     query.Get("sort"),
		Order:    query.Get("order"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeServiceError(w, r, err)
		return
	}
	devices := make([]diskDeviceView, len(value.Devices))
	for i, device := range value.Devices {
		devices[i] = diskDeviceViewFrom(device)
	}
	writeSuccess(w, r, diskDevicePageView{
		Devices:    devices,
		Total:      value.Total,
		Page:       value.Page,
		PageSize:   value.PageSize,
		TotalPages: value.TotalPages,
	}, meta)
}

func hasEmptyDiskQueryParameter(query url.Values) bool {
	for _, values := range query {
		if len(values) == 0 || values[0] == "" {
			return true
		}
	}
	return false
}

func (a *api) diskServiceAvailable(w http.ResponseWriter, r *http.Request) bool {
	if a.diskService != nil {
		return true
	}
	writeError(w, r, http.StatusServiceUnavailable, "disk_unavailable", "数据源暂时不可用，请稍后重试", true)
	return false
}

func diskAlertCountView(value service.AlertCount) alertCountView {
	return alertCountView{Warning: value.Warning, Critical: value.Critical}
}

func diskDeviceViewFrom(value service.DiskDeviceSummary) diskDeviceView {
	return diskDeviceView{
		ID:                  value.ID,
		Host:                value.Host,
		Device:              value.Device,
		Model:               value.Model,
		CapacityBytes:       value.CapacityBytes,
		SMARTHealth:         value.SMARTHealth,
		TemperatureCelsius:  value.TemperatureCelsius,
		LifetimeUsedPercent: value.LifetimeUsedPercent,
		PowerOnHours:        value.PowerOnHours,
		Errors: diskErrorsView{
			PendingSectors:       value.Errors.PendingSectors,
			ReallocatedSectors:   value.Errors.ReallocatedSectors,
			UncorrectableSectors: value.Errors.UncorrectableSectors,
			UDMACRCErrors:        value.Errors.UDMACRCErrors,
			MediaIntegrityErrors: value.Errors.MediaIntegrityErrors,
			ErrorLogEntries:      value.Errors.ErrorLogEntries,
			CommandTimeouts:      value.Errors.CommandTimeouts,
			UnsafeShutdowns:      value.Errors.UnsafeShutdowns,
		},
		Status:          value.Status,
		StatusSource:    value.StatusSource,
		CollectionLevel: value.CollectionLevel,
	}
}
