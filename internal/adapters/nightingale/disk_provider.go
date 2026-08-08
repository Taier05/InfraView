package nightingale

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/disk"
)

const (
	diskHealthQuery = iota
	diskDeviceTemperatureQuery
	diskAttributeTemperatureQuery
	diskLifetimeUsedQuery
	diskPowerOnHoursQuery
	diskCriticalWarningQuery
	diskAvailableSpareQuery
	diskAvailableSpareThresholdQuery
	diskAttributeFailureQuery
	diskPendingSectorsQuery
	diskReallocatedSectorsQuery
	diskUncorrectableSectorsQuery
	diskUDMACRCErrorsQuery
	diskMediaIntegrityErrorsQuery
	diskErrorLogEntriesQuery
	diskUnsafeShutdownsQuery
	diskCommandTimeoutsQuery
	diskCapacityQuery
	diskInventoryQuery
	diskQueryCount
)

type diskScalarState struct {
	value     *float64
	timestamp time.Time
	conflict  bool
}

type diskInt64State struct {
	value     *int64
	timestamp time.Time
	conflict  bool
}

type diskDeviceState struct {
	device    disk.Device
	serialNo  string
	wwn       string
	reporting bool

	health                  diskScalarState
	capacity                diskInt64State
	temperature             diskScalarState
	lifetimeUsed            diskScalarState
	powerOnHours            diskScalarState
	criticalWarning         diskScalarState
	availableSpare          diskScalarState
	availableSpareThreshold diskScalarState
	pendingSectors          diskScalarState
	reallocatedSectors      diskScalarState
	uncorrectableSectors    diskScalarState
	udmaCRCErrors           diskScalarState
	mediaIntegrityErrors    diskScalarState
	errorLogEntries         diskScalarState
	commandTimeouts         diskScalarState
	unsafeShutdowns         diskScalarState
}

type diskInventoryState struct {
	host       string
	deviceName string
	serialNo   string
	wwn        string
	model      string
	reportedAt time.Time
}

func (p *Provider) SMARTSnapshot(ctx context.Context) (disk.Snapshot, error) {
	if err := p.ready(); err != nil {
		return disk.Snapshot{}, diskUnavailableError()
	}
	results, err := p.queryInstant(ctx, diskPromQL())
	if err != nil {
		return disk.Snapshot{}, diskUnavailableError()
	}
	if len(results) != diskQueryCount || results[diskHealthQuery] == nil || results[diskInventoryQuery] == nil {
		return disk.Snapshot{}, diskUnavailableError()
	}

	states, err := buildDiskStates(results[diskInventoryQuery])
	if err != nil {
		return disk.Snapshot{}, diskUnavailableError()
	}
	mergeDiskHealth(states, results[diskHealthQuery])
	mergeDiskCapacity(states, results[diskCapacityQuery])
	mergeDiskAuxiliary(states, results)

	devices := make([]disk.Device, 0, len(states))
	for _, state := range states {
		finalizeDiskDevice(state)
		devices = append(devices, state.device)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].HostID != devices[j].HostID {
			return devices[i].HostID < devices[j].HostID
		}
		if devices[i].Device != devices[j].Device {
			return devices[i].Device < devices[j].Device
		}
		return devices[i].ID < devices[j].ID
	})
	return disk.Snapshot{Devices: devices}, nil
}

func buildDiskStates(inventory []instantSeries) (map[string]*diskDeviceState, error) {
	inventoryStates := make(map[string]diskInventoryState, len(inventory))
	for _, series := range inventory {
		host, deviceName, serialNo, wwn, key, ok := diskIdentity(series.Metric)
		if !ok || len(series.Value) != 2 {
			return nil, diskUnavailableError()
		}
		reportedAt, ok := parseUnixTime(series.Value[1])
		if !ok {
			return nil, diskUnavailableError()
		}
		candidate := diskInventoryState{
			host:       host,
			deviceName: deviceName,
			serialNo:   serialNo,
			wwn:        wwn,
			model:      strings.TrimSpace(series.Metric["model"]),
			reportedAt: reportedAt,
		}
		current, exists := inventoryStates[key]
		if !exists {
			inventoryStates[key] = candidate
			continue
		}
		current.serialNo, ok = mergeDiskInventoryIdentity(current.serialNo, candidate.serialNo)
		if !ok {
			return nil, diskUnavailableError()
		}
		current.wwn, ok = mergeDiskInventoryIdentity(current.wwn, candidate.wwn)
		if !ok {
			return nil, diskUnavailableError()
		}
		switch {
		case candidate.reportedAt.After(current.reportedAt):
			if candidate.model != "" {
				current.model = candidate.model
			}
			current.reportedAt = candidate.reportedAt
		case candidate.reportedAt.Equal(current.reportedAt):
			if diskInventoryMetadataConflict(current, candidate) {
				return nil, diskUnavailableError()
			}
		}
		if current.model == "" {
			current.model = candidate.model
		}
		inventoryStates[key] = current
	}

	states := make(map[string]*diskDeviceState, len(inventoryStates))
	stableIdentities := make(map[string]string, len(inventory))
	serialIdentities := make(map[string]string, len(inventory))
	wwnIdentities := make(map[string]string, len(inventory))

	for key, inventoryState := range inventoryStates {
		stableID := disk.StableDeviceID(inventoryState.host, inventoryState.wwn, inventoryState.serialNo, inventoryState.deviceName)
		if stableID == "" {
			return nil, diskUnavailableError()
		}
		if diskIdentityConflict(stableIdentities, stableID, key) ||
			inventoryState.serialNo != "" && diskIdentityConflict(serialIdentities, inventoryState.host+"\x00"+inventoryState.serialNo, stableID) ||
			inventoryState.wwn != "" && diskIdentityConflict(wwnIdentities, inventoryState.host+"\x00"+inventoryState.wwn, stableID) {
			return nil, diskUnavailableError()
		}
		stableIdentities[stableID] = key
		if inventoryState.serialNo != "" {
			serialIdentities[inventoryState.host+"\x00"+inventoryState.serialNo] = stableID
		}
		if inventoryState.wwn != "" {
			wwnIdentities[inventoryState.host+"\x00"+inventoryState.wwn] = stableID
		}
		states[key] = &diskDeviceState{
			device: disk.Device{
				ID:                stableID,
				HostID:            inventoryState.host,
				Device:            inventoryState.deviceName,
				Model:             inventoryState.model,
				SMARTHealth:       disk.HealthUnknown,
				AttributeFailure:  disk.AttributeFailureNone,
				CollectionTracked: true,
				ReportedAt:        inventoryState.reportedAt,
			},
			serialNo: inventoryState.serialNo,
			wwn:      inventoryState.wwn,
		}
	}
	return states, nil
}

func mergeDiskInventoryIdentity(current, candidate string) (string, bool) {
	switch {
	case current == "":
		return candidate, true
	case candidate == "", candidate == current:
		return current, true
	default:
		return "", false
	}
}

func diskInventoryMetadataConflict(current, candidate diskInventoryState) bool {
	return current.model != "" && candidate.model != "" && current.model != candidate.model
}

func diskIdentityConflict(index map[string]string, key, value string) bool {
	existing, exists := index[key]
	return exists && existing != value
}

func diskIdentity(labels map[string]string) (host, deviceName, serialNo, wwn, key string, ok bool) {
	host = strings.TrimSpace(labels["ident"])
	deviceName = strings.TrimSpace(labels["device"])
	serialNo = strings.TrimSpace(labels["serial_no"])
	wwn = strings.TrimSpace(labels["wwn"])
	if host == "" || deviceName == "" {
		return "", "", "", "", "", false
	}
	return host, deviceName, serialNo, wwn, host + "\x00" + deviceName, true
}

func mergeDiskHealth(states map[string]*diskDeviceState, series []instantSeries) {
	for _, candidate := range series {
		state, ok := diskStateForSeries(states, candidate)
		if !ok {
			continue
		}
		state.reporting = true
		mergeDiskScalar(&state.health, candidate, diskBinary)
	}
}

func mergeDiskCapacity(states map[string]*diskDeviceState, series []instantSeries) {
	for _, candidate := range series {
		state, ok := diskStateForSeries(states, candidate)
		if !ok {
			continue
		}
		value, timestamp, ok := parseDiskCapacity(candidate.Value)
		if !ok || timestamp.Before(state.capacity.timestamp) {
			continue
		}
		if timestamp.After(state.capacity.timestamp) || state.capacity.timestamp.IsZero() && state.capacity.value == nil && !state.capacity.conflict {
			state.capacity.value = &value
			state.capacity.timestamp = timestamp
			state.capacity.conflict = false
			continue
		}
		if state.capacity.conflict {
			continue
		}
		if state.capacity.value == nil || *state.capacity.value != value {
			state.capacity.value = nil
			state.capacity.conflict = true
		}
	}
}

func parseDiskCapacity(raw []json.RawMessage) (int64, time.Time, bool) {
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
	if len(text) == 0 || len(text) > 64 {
		return 0, time.Time{}, false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			switch character {
			case '+', '-', '.', 'e', 'E':
			default:
				return 0, time.Time{}, false
			}
		}
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok || value.Sign() < 0 || !value.IsInt() || !value.Num().IsInt64() {
		return 0, time.Time{}, false
	}
	return value.Num().Int64(), timestamp, true
}

func mergeDiskAuxiliary(states map[string]*diskDeviceState, results [][]instantSeries) {
	for queryIndex := diskDeviceTemperatureQuery; queryIndex < diskCapacityQuery; queryIndex++ {
		for _, series := range results[queryIndex] {
			state, ok := diskStateForSeries(states, series)
			if !ok || !state.reporting {
				continue
			}
			switch queryIndex {
			case diskDeviceTemperatureQuery, diskAttributeTemperatureQuery:
				mergeDiskScalar(&state.temperature, series, diskFinite)
			case diskLifetimeUsedQuery:
				mergeDiskScalar(&state.lifetimeUsed, series, diskNonNegative)
			case diskPowerOnHoursQuery:
				mergeDiskScalar(&state.powerOnHours, series, diskNonNegative)
			case diskCriticalWarningQuery:
				mergeDiskScalar(&state.criticalWarning, series, diskNonNegativeInteger)
			case diskAvailableSpareQuery:
				mergeDiskScalar(&state.availableSpare, series, diskPercentage)
			case diskAvailableSpareThresholdQuery:
				mergeDiskScalar(&state.availableSpareThreshold, series, diskPercentage)
			case diskAttributeFailureQuery:
				mergeDiskAttributeFailure(state, series)
			case diskPendingSectorsQuery:
				mergeDiskScalar(&state.pendingSectors, series, diskNonNegative)
			case diskReallocatedSectorsQuery:
				mergeDiskScalar(&state.reallocatedSectors, series, diskNonNegative)
			case diskUncorrectableSectorsQuery:
				mergeDiskScalar(&state.uncorrectableSectors, series, diskNonNegative)
			case diskUDMACRCErrorsQuery:
				mergeDiskScalar(&state.udmaCRCErrors, series, diskNonNegative)
			case diskMediaIntegrityErrorsQuery:
				mergeDiskScalar(&state.mediaIntegrityErrors, series, diskNonNegative)
			case diskErrorLogEntriesQuery:
				mergeDiskScalar(&state.errorLogEntries, series, diskNonNegative)
			case diskUnsafeShutdownsQuery:
				mergeDiskScalar(&state.unsafeShutdowns, series, diskNonNegative)
			case diskCommandTimeoutsQuery:
				mergeDiskScalar(&state.commandTimeouts, series, diskNonNegative)
			}
		}
	}
}

func diskStateForSeries(states map[string]*diskDeviceState, series instantSeries) (*diskDeviceState, bool) {
	_, _, candidateSerialNo, candidateWWN, key, ok := diskIdentity(series.Metric)
	if !ok {
		return nil, false
	}
	state, ok := states[key]
	if !ok ||
		state.serialNo != "" && candidateSerialNo != "" && candidateSerialNo != state.serialNo ||
		state.wwn != "" && candidateWWN != "" && candidateWWN != state.wwn {
		return nil, false
	}
	return state, true
}

func mergeDiskScalar(state *diskScalarState, series instantSeries, valid func(float64) bool) {
	value, timestamp, ok := parseInstantValue(series.Value)
	if !ok || !valid(value) || timestamp.Before(state.timestamp) {
		return
	}
	if timestamp.After(state.timestamp) || state.timestamp.IsZero() && state.value == nil && !state.conflict {
		state.value = &value
		state.timestamp = timestamp
		state.conflict = false
		return
	}
	if state.conflict {
		return
	}
	if state.value == nil || *state.value != value {
		state.value = nil
		state.conflict = true
	}
}

func diskFinite(float64) bool {
	return true
}

func diskNonNegative(value float64) bool {
	return value >= 0
}

func diskBinary(value float64) bool {
	return value == 0 || value == 1
}

func diskPercentage(value float64) bool {
	return value >= 0 && value <= 100
}

func diskNonNegativeInteger(value float64) bool {
	integer, ok := finiteInt64(value)
	return ok && integer >= 0 && value == math.Trunc(value)
}

func mergeDiskAttributeFailure(state *diskDeviceState, series instantSeries) {
	value, _, ok := parseInstantValue(series.Value)
	if !ok || value < 0 {
		return
	}
	switch series.Metric["fail"] {
	case "FAILING_NOW":
		state.device.AttributeFailure = disk.AttributeFailureNow
	case "In_the_past":
		if state.device.AttributeFailure != disk.AttributeFailureNow {
			state.device.AttributeFailure = disk.AttributeFailurePast
		}
	}
}

func finalizeDiskDevice(state *diskDeviceState) {
	if state.health.value != nil {
		if *state.health.value == 1 {
			state.device.SMARTHealth = disk.HealthHealthy
		} else {
			state.device.SMARTHealth = disk.HealthFailed
		}
	}
	if state.capacity.value != nil {
		value := *state.capacity.value
		state.device.CapacityBytes = &value
	}
	state.device.TemperatureCelsius = state.temperature.value
	state.device.LifetimeUsedPercent = state.lifetimeUsed.value
	state.device.PowerOnHours = state.powerOnHours.value
	if state.criticalWarning.value != nil {
		value := int64(*state.criticalWarning.value)
		state.device.CriticalWarning = &value
	}
	state.device.AvailableSparePercent = state.availableSpare.value
	state.device.AvailableSpareThresholdPercent = state.availableSpareThreshold.value
	state.device.Errors = disk.ErrorCounters{
		PendingSectors:       state.pendingSectors.value,
		ReallocatedSectors:   state.reallocatedSectors.value,
		UncorrectableSectors: state.uncorrectableSectors.value,
		UDMACRCErrors:        state.udmaCRCErrors.value,
		MediaIntegrityErrors: state.mediaIntegrityErrors.value,
		ErrorLogEntries:      state.errorLogEntries.value,
		CommandTimeouts:      state.commandTimeouts.value,
		UnsafeShutdowns:      state.unsafeShutdowns.value,
	}
}

func diskUnavailableError() error {
	return fmt.Errorf("%w: Nightingale SMART 当前指标不可用", disk.ErrUnavailable)
}
