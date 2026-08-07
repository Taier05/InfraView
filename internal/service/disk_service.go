package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/disk"
)

const (
	diskSnapshotCacheKey = "service:disk:snapshot"
	diskDefaultSort      = "host_device"
)

type DiskService struct {
	provider  disk.Provider
	store     *cache.Store
	options   DiskOptions
	freshness *freshnessTracker
}

func NewDisk(provider disk.Provider, store *cache.Store, options DiskOptions) *DiskService {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.SnapshotTTL <= 0 {
		options.SnapshotTTL = 60 * time.Second
	}
	if options.CollectionInterval <= 0 {
		options.CollectionInterval = 60 * time.Second
	}
	if options.MaxStale <= 0 {
		options.MaxStale = 5 * time.Minute
	}
	if store == nil {
		store = cache.New(options.Clock)
	}
	return &DiskService{
		provider:  provider,
		store:     store,
		options:   options,
		freshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (s *DiskService) snapshot(ctx context.Context) (disk.Snapshot, Meta, error) {
	result, err := s.store.GetOrLoad(
		ctx,
		diskSnapshotCacheKey,
		s.options.SnapshotTTL,
		s.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, loadErr := s.provider.SMARTSnapshot(loadCtx)
			if loadErr != nil {
				return disk.Snapshot{}, loadErr
			}
			samples := make(map[string]time.Time, len(snapshot.Devices))
			for _, device := range snapshot.Devices {
				if device.CollectionTracked {
					samples[device.ID] = device.ReportedAt
				}
			}
			s.freshness.Observe(samples)
			return snapshot, nil
		},
	)
	if err != nil {
		return disk.Snapshot{}, Meta{}, err
	}
	snapshot, ok := result.Value.(disk.Snapshot)
	if !ok {
		return disk.Snapshot{}, Meta{}, fmt.Errorf("service: disk cache contained %T", result.Value)
	}
	var collectedAt time.Time
	for _, device := range snapshot.Devices {
		collectedAt = latestTime(collectedAt, device.ReportedAt)
	}
	return cloneDiskSnapshot(snapshot), resultMetaAt(result, collectedAt), nil
}

func (s *DiskService) Overview(ctx context.Context) (DiskOverview, Meta, error) {
	snapshot, meta, err := s.snapshot(ctx)
	if err != nil {
		return DiskOverview{}, Meta{}, err
	}
	overview := DiskOverview{Total: len(snapshot.Devices)}
	for _, device := range snapshot.Devices {
		summary := s.summarizeDiskDevice(device)
		switch summary.Status {
		case LevelNormal:
			overview.Normal++
		case LevelWarning:
			overview.Warning++
			overview.AffectedDevices++
			overview.WarningDevices++
		case LevelCritical:
			overview.Critical++
			overview.AffectedDevices++
			overview.CriticalDevices++
		case LevelUnknown:
			overview.Unknown++
			overview.AffectedDevices++
			overview.WarningDevices++
		}

		addDiskAlert(&overview.Alerts.SMARTHealth, diskSMARTHealthLevel(device.SMARTHealth))
		addDiskAlert(&overview.Alerts.DeviceWarning, diskDeviceWarningLevel(device))
		addDiskAlert(&overview.Alerts.AttributeFailure, diskAttributeFailureLevel(device.AttributeFailure))
		addDiskAlert(&overview.Alerts.Collection, summary.CollectionLevel)
	}
	return overview, meta, nil
}

func (s *DiskService) Devices(ctx context.Context, query DiskQuery) (DiskPage, Meta, error) {
	query, err := normalizeDiskQuery(query)
	if err != nil {
		return DiskPage{}, Meta{}, err
	}
	snapshot, meta, err := s.snapshot(ctx)
	if err != nil {
		return DiskPage{}, Meta{}, err
	}

	search := strings.ToLower(strings.TrimSpace(query.Search))
	items := make([]DiskDeviceSummary, 0, len(snapshot.Devices))
	for _, device := range snapshot.Devices {
		summary := s.summarizeDiskDevice(device)
		if query.Status != "" && summary.Status != query.Status {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(summary.Host), search) &&
			!strings.Contains(strings.ToLower(summary.Device), search) &&
			!strings.Contains(strings.ToLower(summary.Model), search) {
			continue
		}
		items = append(items, summary)
	}
	sortDiskDevices(items, query.Sort, query.Order)

	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return DiskPage{
		Devices:    append([]DiskDeviceSummary(nil), items[start:end]...),
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, meta, nil
}

func (s *DiskService) summarizeDiskDevice(device disk.Device) DiskDeviceSummary {
	collectionLevel := LevelNormal
	if device.CollectionTracked {
		collectionLevel = s.freshness.Level(device.ID, device.ReportedAt)
	}
	status, statusSource := diskStatusAndSource(
		diskSMARTHealthLevel(device.SMARTHealth),
		diskDeviceWarningLevel(device),
		diskAttributeFailureLevel(device.AttributeFailure),
		collectionLevel,
	)
	return DiskDeviceSummary{
		ID:                  device.ID,
		Host:                device.HostID,
		Device:              device.Device,
		Model:               device.Model,
		CapacityBytes:       cloneInt64(device.CapacityBytes),
		SMARTHealth:         device.SMARTHealth,
		TemperatureCelsius:  cloneFloat(device.TemperatureCelsius),
		LifetimeUsedPercent: cloneFloat(device.LifetimeUsedPercent),
		PowerOnHours:        cloneFloat(device.PowerOnHours),
		Errors:              cloneDiskErrors(device.Errors),
		Status:              status,
		StatusSource:        statusSource,
		CollectionLevel:     collectionLevel,
	}
}

func diskSMARTHealthLevel(health disk.Health) Level {
	switch health {
	case disk.HealthHealthy:
		return LevelNormal
	case disk.HealthFailed:
		return LevelCritical
	default:
		return LevelUnknown
	}
}

func diskDeviceWarningLevel(device disk.Device) Level {
	if device.CriticalWarning != nil && *device.CriticalWarning != 0 {
		return LevelCritical
	}
	if device.AvailableSparePercent != nil &&
		device.AvailableSpareThresholdPercent != nil &&
		*device.AvailableSparePercent <= *device.AvailableSpareThresholdPercent {
		return LevelCritical
	}
	return LevelNormal
}

func diskAttributeFailureLevel(failure disk.AttributeFailure) Level {
	switch failure {
	case disk.AttributeFailureNow:
		return LevelCritical
	case disk.AttributeFailurePast:
		return LevelWarning
	default:
		return LevelNormal
	}
}

func diskHigherLevel(levels ...Level) Level {
	highest := LevelNormal
	for _, level := range levels {
		if diskLevelRank(level) > diskLevelRank(highest) {
			highest = level
		}
	}
	return highest
}

func diskStatusAndSource(
	smartHealth Level,
	deviceWarning Level,
	attributeFailure Level,
	collection Level,
) (Level, DiskStatusSource) {
	status := diskHigherLevel(
		smartHealth,
		deviceWarning,
		attributeFailure,
		collection,
	)
	if status == LevelNormal {
		return status, DiskStatusSourceNormal
	}
	deviceSources := []struct {
		level  Level
		source DiskStatusSource
	}{
		{level: smartHealth, source: DiskStatusSourceSMARTHealth},
		{level: deviceWarning, source: DiskStatusSourceDeviceWarning},
		{level: attributeFailure, source: DiskStatusSourceAttributeFailure},
	}
	for _, candidate := range deviceSources {
		if candidate.level == status {
			return status, candidate.source
		}
	}
	deviceLevel := diskHigherLevel(smartHealth, deviceWarning, attributeFailure)
	if collection == status &&
		diskLevelRank(collection) > diskLevelRank(deviceLevel) {
		return status, DiskStatusSourceCollection
	}
	return status, DiskStatusSourceUnknown
}

func diskLevelRank(level Level) int {
	switch level {
	case LevelCritical:
		return 3
	case LevelWarning:
		return 2
	case LevelUnknown:
		return 1
	default:
		return 0
	}
}

func addDiskAlert(count *AlertCount, level Level) {
	switch level {
	case LevelWarning:
		count.Warning++
	case LevelCritical:
		count.Critical++
	}
}

func normalizeDiskQuery(query DiskQuery) (DiskQuery, error) {
	if query.Page < 1 {
		return DiskQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return DiskQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.PageSize {
		return DiskQuery{}, fmt.Errorf("%w: page offset overflows int", ErrInvalidQuery)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return DiskQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = diskDefaultSort
	} else {
		switch query.Sort {
		case "host", "device", "model", "capacity", "smart", "temperature", "lifetime", "power_on_hours", "errors", "status":
		default:
			return DiskQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
		}
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return DiskQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

func sortDiskDevices(items []DiskDeviceSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		if field == "capacity" {
			left := items[i].CapacityBytes
			right := items[j].CapacityBytes
			if (left == nil) != (right == nil) {
				return left != nil
			}
			comparison := 0
			if left != nil {
				switch {
				case *left < *right:
					comparison = -1
				case *left > *right:
					comparison = 1
				}
			}
			if comparison == 0 {
				return items[i].ID < items[j].ID
			}
			if order == "desc" {
				return comparison > 0
			}
			return comparison < 0
		}
		if field == "model" {
			leftAvailable := strings.TrimSpace(items[i].Model) != ""
			rightAvailable := strings.TrimSpace(items[j].Model) != ""
			if leftAvailable != rightAvailable {
				return leftAvailable
			}
			comparison := 0
			if leftAvailable {
				comparison = compareNatural(items[i].Model, items[j].Model)
			}
			if comparison == 0 {
				return items[i].ID < items[j].ID
			}
			if order == "desc" {
				return comparison > 0
			}
			return comparison < 0
		}
		leftValue, leftAvailable, numeric := diskNumericSortValue(items[i], field)
		rightValue, rightAvailable, _ := diskNumericSortValue(items[j], field)
		if numeric && leftAvailable != rightAvailable {
			return leftAvailable
		}
		comparison := 0
		if numeric && leftAvailable {
			comparison = compareFloat64(leftValue, rightValue)
		} else {
			comparison = compareDiskDevices(items[i], items[j], field)
		}
		if comparison == 0 {
			return items[i].ID < items[j].ID
		}
		if order == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func diskNumericSortValue(device DiskDeviceSummary, field string) (float64, bool, bool) {
	switch field {
	case "temperature":
		return metricSortValue(device.TemperatureCelsius)
	case "lifetime":
		return metricSortValue(device.LifetimeUsedPercent)
	case "power_on_hours":
		return metricSortValue(device.PowerOnHours)
	case "errors":
		value, available := diskErrorSortValue(device.Errors)
		return value, available, true
	default:
		return 0, false, false
	}
}

func diskErrorSortValue(errors disk.ErrorCounters) (float64, bool) {
	values := []*float64{
		errors.PendingSectors,
		errors.ReallocatedSectors,
		errors.UncorrectableSectors,
		errors.UDMACRCErrors,
		errors.MediaIntegrityErrors,
		errors.ErrorLogEntries,
	}
	var total float64
	available := false
	for _, value := range values {
		if value != nil {
			total += *value
			available = true
		}
	}
	return total, available
}

func diskHealthRank(value disk.Health) int {
	switch value {
	case disk.HealthHealthy:
		return 0
	case disk.HealthFailed:
		return 2
	default:
		return 3
	}
}

func compareDiskDevices(left, right DiskDeviceSummary, field string) int {
	switch field {
	case diskDefaultSort:
		if comparison := compareNatural(left.Host, right.Host); comparison != 0 {
			return comparison
		}
		return compareNatural(left.Device, right.Device)
	case "host":
		if comparison := compareNatural(left.Host, right.Host); comparison != 0 {
			return comparison
		}
		return compareNatural(left.Device, right.Device)
	case "device":
		return compareNatural(left.Device, right.Device)
	case "smart":
		return diskHealthRank(left.SMARTHealth) - diskHealthRank(right.SMARTHealth)
	case "status":
		return listLevelSortRank(left.Status) - listLevelSortRank(right.Status)
	default:
		return 0
	}
}

func compareNatural(left, right string) int {
	left = strings.ToLower(left)
	right = strings.ToLower(right)
	for leftIndex, rightIndex := 0, 0; ; {
		if leftIndex == len(left) || rightIndex == len(right) {
			switch {
			case leftIndex == len(left) && rightIndex == len(right):
				return 0
			case leftIndex == len(left):
				return -1
			default:
				return 1
			}
		}
		leftDigit := isASCIIDigit(left[leftIndex])
		rightDigit := isASCIIDigit(right[rightIndex])
		if leftDigit != rightDigit {
			if left[leftIndex] < right[rightIndex] {
				return -1
			}
			return 1
		}
		leftEnd := naturalSegmentEnd(left, leftIndex, leftDigit)
		rightEnd := naturalSegmentEnd(right, rightIndex, rightDigit)
		leftSegment := left[leftIndex:leftEnd]
		rightSegment := right[rightIndex:rightEnd]
		comparison := strings.Compare(leftSegment, rightSegment)
		if leftDigit {
			comparison = compareNaturalNumber(leftSegment, rightSegment)
		}
		if comparison != 0 {
			return comparison
		}
		leftIndex = leftEnd
		rightIndex = rightEnd
	}
}

func naturalSegmentEnd(value string, start int, digits bool) int {
	end := start
	for end < len(value) && isASCIIDigit(value[end]) == digits {
		end++
	}
	return end
}

func compareNaturalNumber(left, right string) int {
	leftSignificant := strings.TrimLeft(left, "0")
	rightSignificant := strings.TrimLeft(right, "0")
	if leftSignificant == "" {
		leftSignificant = "0"
	}
	if rightSignificant == "" {
		rightSignificant = "0"
	}
	if len(leftSignificant) < len(rightSignificant) {
		return -1
	}
	if len(leftSignificant) > len(rightSignificant) {
		return 1
	}
	if comparison := strings.Compare(leftSignificant, rightSignificant); comparison != 0 {
		return comparison
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func cloneDiskSnapshot(source disk.Snapshot) disk.Snapshot {
	devices := make([]disk.Device, len(source.Devices))
	for index, device := range source.Devices {
		devices[index] = device
		devices[index].CapacityBytes = cloneInt64(device.CapacityBytes)
		devices[index].TemperatureCelsius = cloneFloat(device.TemperatureCelsius)
		devices[index].LifetimeUsedPercent = cloneFloat(device.LifetimeUsedPercent)
		devices[index].PowerOnHours = cloneFloat(device.PowerOnHours)
		devices[index].CriticalWarning = cloneInt64(device.CriticalWarning)
		devices[index].AvailableSparePercent = cloneFloat(device.AvailableSparePercent)
		devices[index].AvailableSpareThresholdPercent = cloneFloat(device.AvailableSpareThresholdPercent)
		devices[index].Errors = cloneDiskErrors(device.Errors)
	}
	return disk.Snapshot{Devices: devices}
}

func cloneDiskErrors(source disk.ErrorCounters) disk.ErrorCounters {
	return disk.ErrorCounters{
		PendingSectors:       cloneFloat(source.PendingSectors),
		ReallocatedSectors:   cloneFloat(source.ReallocatedSectors),
		UncorrectableSectors: cloneFloat(source.UncorrectableSectors),
		UDMACRCErrors:        cloneFloat(source.UDMACRCErrors),
		MediaIntegrityErrors: cloneFloat(source.MediaIntegrityErrors),
		ErrorLogEntries:      cloneFloat(source.ErrorLogEntries),
		UnsafeShutdowns:      cloneFloat(source.UnsafeShutdowns),
	}
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
