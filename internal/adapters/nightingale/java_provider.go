package nightingale

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/javaapp"
)

type javaLatest[T comparable] struct {
	value       *T
	timestamp   time.Time
	conflict    bool
	initialized bool
}

type javaServiceState struct {
	service javaapp.Service

	healthLatency       javaLatest[float64]
	healthUp            javaLatest[bool]
	portUp              javaLatest[bool]
	processCount        javaLatest[int64]
	processCPU          javaLatest[float64]
	processMemoryBytes  javaLatest[int64]
	processMemory       javaLatest[float64]
	portConsistent      javaLatest[bool]
	processStartSeconds javaLatest[int64]
	processUp           javaLatest[bool]
	inventorySampleAt   time.Time
}

var _ javaapp.Provider = (*Provider)(nil)

func (provider *Provider) JavaSnapshot(ctx context.Context) (javaapp.Snapshot, error) {
	if err := provider.ready(); err != nil {
		return javaapp.Snapshot{}, javaUnavailableError()
	}
	results, err := provider.queryInstant(ctx, javaPromQL())
	if err != nil || len(results) != javaQueryCount {
		return javaapp.Snapshot{}, javaUnavailableError()
	}
	return buildJavaSnapshot(results)
}

func buildJavaSnapshot(results [][]instantSeries) (javaapp.Snapshot, error) {
	if len(results) != javaQueryCount || results[javaInventoryQuery] == nil {
		return javaapp.Snapshot{}, javaUnavailableError()
	}
	states, err := buildJavaInventory(results[javaInventoryQuery])
	if err != nil {
		return javaapp.Snapshot{}, err
	}
	for queryIndex := javaHealthLatencyQuery; queryIndex < javaInventoryQuery; queryIndex++ {
		mergeJavaQuery(states, queryIndex, results[queryIndex])
	}

	snapshot := javaapp.Snapshot{Services: make([]javaapp.Service, 0, len(states))}
	for _, state := range states {
		finalizeJavaService(state)
		snapshot.Services = append(snapshot.Services, state.service)
	}
	sort.Slice(snapshot.Services, func(i, j int) bool {
		if snapshot.Services[i].Name != snapshot.Services[j].Name {
			return snapshot.Services[i].Name < snapshot.Services[j].Name
		}
		if snapshot.Services[i].Address != snapshot.Services[j].Address {
			return snapshot.Services[i].Address < snapshot.Services[j].Address
		}
		return snapshot.Services[i].ID < snapshot.Services[j].ID
	})
	return snapshot, nil
}

func buildJavaInventory(series []instantSeries) (map[string]*javaServiceState, error) {
	states := make(map[string]*javaServiceState, len(series))
	for _, candidate := range series {
		name, address, id, ok := javaIdentity(candidate.Metric)
		if !ok {
			continue
		}
		sampleAt, reportedAt, ok := javaInventoryTimes(candidate.Value)
		if !ok {
			return nil, javaUnavailableError()
		}
		state := states[id]
		if state == nil {
			states[id] = &javaServiceState{
				service: javaapp.Service{
					ID: id, Name: name, Address: address,
					CollectionTracked: true, ReportedAt: reportedAt,
				},
				inventorySampleAt: sampleAt,
			}
			continue
		}
		if sampleAt.Before(state.inventorySampleAt) {
			continue
		}
		if sampleAt.Equal(state.inventorySampleAt) {
			if state.service.Name != name || state.service.Address != address || !state.service.ReportedAt.Equal(reportedAt) {
				return nil, javaUnavailableError()
			}
			continue
		}
		state.service.Name = name
		state.service.Address = address
		state.service.ReportedAt = reportedAt
		state.inventorySampleAt = sampleAt
	}
	return states, nil
}

func javaIdentity(labels map[string]string) (name, address, id string, ok bool) {
	name = labels["name"]
	address = labels["server_ip"]
	if strings.TrimSpace(name) == "" || strings.TrimSpace(address) == "" {
		return "", "", "", false
	}
	id = javaapp.StableServiceID(name, address)
	return name, address, id, id != ""
}

func javaInventoryTimes(raw []json.RawMessage) (sampleAt, reportedAt time.Time, ok bool) {
	if len(raw) != 2 {
		return time.Time{}, time.Time{}, false
	}
	sampleAt, ok = parseUnixTime(raw[0])
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	reportedAt, ok = parseUnixTime(raw[1])
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return sampleAt, reportedAt, true
}

func mergeJavaQuery(states map[string]*javaServiceState, queryIndex int, series []instantSeries) {
	for _, candidate := range series {
		_, _, id, ok := javaIdentity(candidate.Metric)
		if !ok {
			continue
		}
		state := states[id]
		if state == nil {
			continue
		}
		switch queryIndex {
		case javaHealthLatencyQuery:
			mergeJavaFloat(&state.healthLatency, candidate, javaNonNegative)
		case javaHealthUpQuery:
			mergeJavaBool(&state.healthUp, candidate)
		case javaPortUpQuery:
			mergeJavaBool(&state.portUp, candidate)
		case javaProcessCountQuery:
			mergeJavaInt64(&state.processCount, candidate)
		case javaProcessCPUQuery:
			mergeJavaFloat(&state.processCPU, candidate, javaPercent)
		case javaProcessMemoryBytesQuery:
			mergeJavaInt64(&state.processMemoryBytes, candidate)
		case javaProcessMemoryPercentQuery:
			mergeJavaFloat(&state.processMemory, candidate, javaPercent)
		case javaPortConsistentQuery:
			mergeJavaBool(&state.portConsistent, candidate)
		case javaProcessStartTimeQuery:
			mergeJavaInt64(&state.processStartSeconds, candidate)
		case javaProcessUpQuery:
			mergeJavaBool(&state.processUp, candidate)
		}
	}
}

func mergeJavaFloat(target *javaLatest[float64], candidate instantSeries, valid func(float64) bool) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if ok && valid(value) {
		mergeJavaLatest(target, value, timestamp)
	}
}

func mergeJavaBool(target *javaLatest[bool], candidate instantSeries) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok || value != 0 && value != 1 {
		return
	}
	mergeJavaLatest(target, value == 1, timestamp)
}

func mergeJavaInt64(target *javaLatest[int64], candidate instantSeries) {
	value, timestamp, ok := parseJavaInt64(candidate.Value)
	if ok {
		mergeJavaLatest(target, value, timestamp)
	}
}

func parseJavaInt64(raw []json.RawMessage) (int64, time.Time, bool) {
	if len(raw) != 2 {
		return 0, time.Time{}, false
	}
	timestamp, ok := parseUnixTime(raw[0])
	if !ok {
		return 0, time.Time{}, false
	}
	var text string
	if err := json.Unmarshal(raw[1], &text); err != nil {
		text = string(raw[1])
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return 0, time.Time{}, false
	}
	return value, timestamp, true
}

func mergeJavaLatest[T comparable](target *javaLatest[T], value T, timestamp time.Time) {
	if !target.initialized || timestamp.After(target.timestamp) {
		copyValue := value
		target.value = &copyValue
		target.timestamp = timestamp
		target.conflict = false
		target.initialized = true
		return
	}
	if timestamp.Equal(target.timestamp) && target.value != nil && *target.value != value {
		target.value = nil
		target.conflict = true
	}
}

func finalizeJavaService(state *javaServiceState) {
	state.service.HealthLatencyMilliseconds = javaLatestPointer(state.healthLatency)
	state.service.HealthUp = javaLatestPointer(state.healthUp)
	state.service.PortUp = javaLatestPointer(state.portUp)
	state.service.ProcessCount = javaLatestPointer(state.processCount)
	state.service.ProcessCPUPercent = javaLatestPointer(state.processCPU)
	state.service.ProcessMemoryBytes = javaLatestPointer(state.processMemoryBytes)
	state.service.ProcessMemoryPercent = javaLatestPointer(state.processMemory)
	state.service.PortConsistent = javaLatestPointer(state.portConsistent)
	state.service.ProcessStartTimeSeconds = javaLatestPointer(state.processStartSeconds)
	state.service.ProcessUp = javaLatestPointer(state.processUp)
}

func javaLatestPointer[T comparable](latest javaLatest[T]) *T {
	if latest.value == nil || latest.conflict {
		return nil
	}
	value := *latest.value
	return &value
}

func javaNonNegative(value float64) bool { return value >= 0 }

func javaPercent(value float64) bool { return value >= 0 && value <= 100 }

func javaUnavailableError() error {
	return fmt.Errorf("%w: Nightingale Java 当前指标不可用", javaapp.ErrUnavailable)
}
