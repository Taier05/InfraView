package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/javaapp"
)

const javaSnapshotCacheKey = "service:java:snapshot"

type JavaService struct {
	provider  javaapp.Provider
	store     *cache.Store
	options   JavaOptions
	freshness *freshnessTracker
}

type javaSnapshotState struct {
	snapshot          javaapp.Snapshot
	serviceAdvancedAt map[string]time.Time
}

func NewJava(provider javaapp.Provider, store *cache.Store, options JavaOptions) *JavaService {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.SnapshotTTL <= 0 {
		options.SnapshotTTL = 15 * time.Second
	}
	if options.CollectionInterval <= 0 {
		options.CollectionInterval = 15 * time.Second
	}
	if options.MaxStale <= 0 {
		options.MaxStale = 5 * time.Minute
	}
	if store == nil {
		store = cache.New(options.Clock)
	}
	return &JavaService{
		provider:  provider,
		store:     store,
		options:   options,
		freshness: newFreshnessTracker(options.Clock, options.CollectionInterval),
	}
}

func (service *JavaService) snapshotState(ctx context.Context) (javaSnapshotState, Meta, error) {
	return service.snapshotStateAt(ctx, service.options.Clock().UTC())
}

func (service *JavaService) snapshotStateAt(ctx context.Context, now time.Time) (javaSnapshotState, Meta, error) {
	result, err := service.store.GetOrLoad(
		ctx,
		javaSnapshotCacheKey,
		service.options.SnapshotTTL,
		service.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			snapshot, loadErr := service.provider.JavaSnapshot(loadCtx)
			if loadErr != nil {
				return javaSnapshotState{}, loadErr
			}
			samples := make(map[string]time.Time, len(snapshot.Services))
			for _, item := range snapshot.Services {
				if item.CollectionTracked {
					samples[item.ID] = item.ReportedAt
				}
			}
			service.freshness.ObserveAt(samples, now)
			return javaSnapshotState{
				snapshot:          snapshot.Clone(),
				serviceAdvancedAt: captureJavaProgress(service.freshness, samples),
			}, nil
		},
	)
	if err != nil {
		return javaSnapshotState{}, Meta{}, err
	}
	state, ok := result.Value.(javaSnapshotState)
	if !ok {
		return javaSnapshotState{}, Meta{}, fmt.Errorf("service: java cache contained %T", result.Value)
	}
	return cloneJavaSnapshotState(state), resultMeta(result), nil
}

func captureJavaProgress(tracker *freshnessTracker, samples map[string]time.Time) map[string]time.Time {
	progress := make(map[string]time.Time, len(samples))
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for key, sampleAt := range samples {
		entry, exists := tracker.entries[key]
		if exists && entry.sampleAt.Equal(sampleAt.UTC()) {
			progress[key] = entry.advancedAt
		}
	}
	return progress
}

func cloneJavaSnapshotState(source javaSnapshotState) javaSnapshotState {
	progress := make(map[string]time.Time, len(source.serviceAdvancedAt))
	for key, value := range source.serviceAdvancedAt {
		progress[key] = value
	}
	return javaSnapshotState{snapshot: source.snapshot.Clone(), serviceAdvancedAt: progress}
}

func (service *JavaService) collectionLevel(item javaapp.Service, advancedAt map[string]time.Time, now time.Time) Level {
	if !item.CollectionTracked {
		return LevelUnknown
	}
	advanced, exists := advancedAt[item.ID]
	if !exists || item.ReportedAt.IsZero() {
		return LevelUnknown
	}
	return collectionLevelAt(now, advanced, service.options.CollectionInterval)
}

func (service *JavaService) Overview(ctx context.Context) (JavaOverview, Meta, error) {
	now := service.options.Clock().UTC()
	state, meta, err := service.snapshotStateAt(ctx, now)
	if err != nil {
		return JavaOverview{}, Meta{}, err
	}
	overview := JavaOverview{
		Status:   LevelNormal,
		Services: JavaLevelCounts{Total: len(state.snapshot.Services)},
	}
	for _, item := range state.snapshot.Services {
		collection := service.collectionLevel(item, state.serviceAdvancedAt, now)
		summary := summarizeJavaService(item, collection, now)
		addJavaLevel(&overview.Services, summary.Status)
		overview.Status = higherJavaLevel(overview.Status, summary.Status)
		addJavaAlert(&overview.Alerts.Health, javaBinaryLevel(item.HealthUp))
		addJavaAlert(&overview.Alerts.Port, javaBinaryLevel(item.PortUp))
		addJavaAlert(&overview.Alerts.Process, higherJavaLevel(javaBinaryLevel(item.ProcessUp), javaBinaryLevel(item.PortConsistent)))
		addJavaAlert(&overview.Alerts.Collection, collection)
	}
	return overview, meta, nil
}

func (service *JavaService) Services(ctx context.Context, query JavaQuery) (JavaPage, Meta, error) {
	query, err := normalizeJavaQuery(query)
	if err != nil {
		return JavaPage{}, Meta{}, err
	}
	now := service.options.Clock().UTC()
	state, meta, err := service.snapshotStateAt(ctx, now)
	if err != nil {
		return JavaPage{}, Meta{}, err
	}
	names := make(map[string]struct{}, len(state.snapshot.Services))
	for _, item := range state.snapshot.Services {
		if strings.TrimSpace(item.Name) != "" {
			names[item.Name] = struct{}{}
		}
	}
	availableNames := make([]string, 0, len(names))
	for name := range names {
		availableNames = append(availableNames, name)
	}
	sort.Strings(availableNames)

	search := strings.ToLower(query.Search)
	items := make([]JavaServiceSummary, 0, len(state.snapshot.Services))
	for _, item := range state.snapshot.Services {
		summary := summarizeJavaService(item, service.collectionLevel(item, state.serviceAdvancedAt, now), now)
		if query.Name != "" && summary.Name != query.Name || query.Status != "" && summary.Status != query.Status {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(summary.Name), search) &&
			!strings.Contains(strings.ToLower(summary.Business), search) &&
			!strings.Contains(strings.ToLower(summary.Address), search) {
			continue
		}
		items = append(items, summary)
	}
	sortJavaServices(items, query.Sort, query.Order)
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	return JavaPage{
		Services:       append([]JavaServiceSummary(nil), items[start:end]...),
		AvailableNames: availableNames,
		Total:          total,
		Page:           query.Page,
		PageSize:       query.PageSize,
	}, meta, nil
}

func normalizeJavaQuery(query JavaQuery) (JavaQuery, error) {
	query.Search = strings.TrimSpace(query.Search)
	query.Name = strings.TrimSpace(query.Name)
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 {
		return JavaQuery{}, fmt.Errorf("%w: page must be positive", ErrInvalidQuery)
	}
	switch query.PageSize {
	case 20, 50, 100:
	default:
		return JavaQuery{}, fmt.Errorf("%w: unsupported page size %d", ErrInvalidQuery, query.PageSize)
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.PageSize {
		return JavaQuery{}, fmt.Errorf("%w: page offset overflows int", ErrInvalidQuery)
	}
	switch query.Status {
	case "", LevelNormal, LevelWarning, LevelCritical, LevelUnknown:
	default:
		return JavaQuery{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidQuery, query.Status)
	}
	if query.Sort == "" {
		query.Sort = "business"
	}
	switch query.Sort {
	case "business", "address", "health", "health_latency", "port", "process", "process_count",
		"consistency", "cpu", "memory", "memory_percent", "uptime", "status":
	default:
		return JavaQuery{}, fmt.Errorf("%w: unsupported sort %q", ErrInvalidQuery, query.Sort)
	}
	if query.Order == "" {
		query.Order = "asc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return JavaQuery{}, fmt.Errorf("%w: unsupported order %q", ErrInvalidQuery, query.Order)
	}
	return query, nil
}

type javaAssessment struct {
	level  Level
	source JavaStatusSource
}

func summarizeJavaService(source javaapp.Service, collection Level, now time.Time) JavaServiceSummary {
	assessment := javaAssessment{level: LevelNormal, source: JavaStatusNormal}
	for _, candidate := range []javaAssessment{
		javaBinaryAssessment(source.HealthUp, JavaStatusHealth),
		javaBinaryAssessment(source.PortUp, JavaStatusPort),
		javaBinaryAssessment(source.ProcessUp, JavaStatusProcess),
		javaBinaryAssessment(source.PortConsistent, JavaStatusConsistency),
		{level: collection, source: JavaStatusCollection},
	} {
		assessment = chooseJavaAssessment(assessment, candidate)
	}
	return JavaServiceSummary{
		ID:                        source.ID,
		Name:                      source.Name,
		Business:                  javaBusinessName(source.Name),
		Address:                   source.Address,
		HealthUp:                  cloneBool(source.HealthUp),
		HealthLatencyMilliseconds: cloneFloat(source.HealthLatencyMilliseconds),
		PortUp:                    cloneBool(source.PortUp),
		ProcessUp:                 cloneBool(source.ProcessUp),
		ProcessCount:              cloneInt64(source.ProcessCount),
		PortConsistent:            cloneBool(source.PortConsistent),
		CPUUsagePercent:           cloneFloat(source.ProcessCPUPercent),
		MemoryBytes:               cloneInt64(source.ProcessMemoryBytes),
		MemoryUsagePercent:        cloneFloat(source.ProcessMemoryPercent),
		UptimeSeconds:             javaUptime(source.ProcessStartTimeSeconds, now),
		Status:                    assessment.level,
		StatusSource:              assessment.source,
		CollectionLevel:           collection,
	}
}

func javaBusinessName(name string) string {
	switch name {
	case "tikbee":
		return "用户端"
	case "rider":
		return "骑手端"
	case "mch":
		return "商家端"
	case "saas":
		return "管理后台端"
	case "mch_saas":
		return "商家 PC 端"
	default:
		return name
	}
}

func javaBinaryAssessment(value *bool, source JavaStatusSource) javaAssessment {
	if value == nil {
		return javaAssessment{level: LevelUnknown, source: JavaStatusUnknown}
	}
	if !*value {
		return javaAssessment{level: LevelCritical, source: source}
	}
	return javaAssessment{level: LevelNormal, source: JavaStatusNormal}
}

func javaBinaryLevel(value *bool) Level {
	if value == nil {
		return LevelUnknown
	}
	if !*value {
		return LevelCritical
	}
	return LevelNormal
}

func chooseJavaAssessment(left, right javaAssessment) javaAssessment {
	if right.level == LevelNormal {
		return left
	}
	leftRank := javaLevelRank(left.level)
	rightRank := javaLevelRank(right.level)
	if rightRank > leftRank || rightRank == leftRank && javaSourceRank(right.source) > javaSourceRank(left.source) {
		return right
	}
	return left
}

func javaLevelRank(level Level) int {
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

func javaSourceRank(source JavaStatusSource) int {
	switch source {
	case JavaStatusUnknown:
		return 6
	case JavaStatusHealth:
		return 5
	case JavaStatusPort:
		return 4
	case JavaStatusProcess:
		return 3
	case JavaStatusConsistency:
		return 2
	case JavaStatusCollection:
		return 1
	default:
		return 0
	}
}

func javaUptime(start *int64, now time.Time) *int64 {
	if start == nil || *start < 0 || *start > now.Unix() {
		return nil
	}
	uptime := now.Unix() - *start
	return &uptime
}

func higherJavaLevel(left, right Level) Level {
	if javaLevelRank(right) > javaLevelRank(left) {
		return right
	}
	return left
}

func addJavaLevel(counts *JavaLevelCounts, level Level) {
	switch level {
	case LevelCritical:
		counts.Critical++
	case LevelWarning:
		counts.Warning++
	case LevelUnknown:
		counts.Unknown++
	default:
		counts.Normal++
	}
}

func addJavaAlert(count *JavaAlertCount, level Level) {
	switch level {
	case LevelCritical:
		count.Critical++
	case LevelWarning:
		count.Warning++
	case LevelUnknown:
		count.Unknown++
	}
}

func sortJavaServices(items []JavaServiceSummary, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftTextOK, text := javaTextSortAvailable(items[i], field)
		rightTextOK, _ := javaTextSortAvailable(items[j], field)
		if text && leftTextOK != rightTextOK {
			return leftTextOK
		}
		if text && !leftTextOK {
			return items[i].ID < items[j].ID
		}
		leftBool, leftBoolOK, boolean := javaBoolSortValue(items[i], field)
		rightBool, rightBoolOK, _ := javaBoolSortValue(items[j], field)
		if boolean && leftBoolOK != rightBoolOK {
			return leftBoolOK
		}
		leftInteger, leftIntegerOK, integer := javaIntegerSortValue(items[i], field)
		rightInteger, rightIntegerOK, _ := javaIntegerSortValue(items[j], field)
		if integer && leftIntegerOK != rightIntegerOK {
			return leftIntegerOK
		}
		leftNumber, leftNumberOK, numeric := javaNumberSortValue(items[i], field)
		rightNumber, rightNumberOK, _ := javaNumberSortValue(items[j], field)
		if numeric && leftNumberOK != rightNumberOK {
			return leftNumberOK
		}

		comparison := 0
		switch {
		case boolean && leftBoolOK:
			comparison = int(leftBool) - int(rightBool)
		case integer && leftIntegerOK:
			comparison = compareInt64(leftInteger, rightInteger)
		case numeric && leftNumberOK:
			comparison = compareFloat64(leftNumber, rightNumber)
		default:
			comparison = compareJavaField(items[i], items[j], field)
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

func javaTextSortAvailable(item JavaServiceSummary, field string) (bool, bool) {
	switch field {
	case "business":
		return strings.TrimSpace(item.Business) != "", true
	case "address":
		return strings.TrimSpace(item.Address) != "", true
	default:
		return false, false
	}
}

func javaBoolSortValue(item JavaServiceSummary, field string) (int, bool, bool) {
	var value *bool
	switch field {
	case "health":
		value = item.HealthUp
	case "port":
		value = item.PortUp
	case "process":
		value = item.ProcessUp
	case "consistency":
		value = item.PortConsistent
	default:
		return 0, false, false
	}
	if value == nil {
		return 0, false, true
	}
	if *value {
		return 1, true, true
	}
	return 0, true, true
}

func javaIntegerSortValue(item JavaServiceSummary, field string) (int64, bool, bool) {
	var value *int64
	switch field {
	case "process_count":
		value = item.ProcessCount
	case "memory":
		value = item.MemoryBytes
	case "uptime":
		value = item.UptimeSeconds
	default:
		return 0, false, false
	}
	if value == nil {
		return 0, false, true
	}
	return *value, true, true
}

func javaNumberSortValue(item JavaServiceSummary, field string) (float64, bool, bool) {
	var value *float64
	switch field {
	case "health_latency":
		value = item.HealthLatencyMilliseconds
	case "cpu":
		value = item.CPUUsagePercent
	case "memory_percent":
		value = item.MemoryUsagePercent
	default:
		return 0, false, false
	}
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return 0, false, true
	}
	return *value, true, true
}

func compareJavaField(left, right JavaServiceSummary, field string) int {
	switch field {
	case "business":
		return compareNatural(strings.ToLower(left.Business), strings.ToLower(right.Business))
	case "address":
		return compareNatural(strings.ToLower(left.Address), strings.ToLower(right.Address))
	case "status":
		return javaLevelRank(left.Status) - javaLevelRank(right.Status)
	default:
		return 0
	}
}
