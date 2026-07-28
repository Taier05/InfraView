package nightingale

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/mysql"
)

const (
	mysqlUptime = iota
	mysqlConnections
	mysqlMaxConnections
	mysqlThreadsRunning
	mysqlQPS
	mysqlSlowQueries
	mysqlBufferPoolUsage
	mysqlScalarCount
)

type mysqlScalarState struct {
	value     *float64
	timestamp time.Time
}

type mysqlBoolState struct {
	value     *bool
	timestamp time.Time
}

type mysqlReplicationState struct {
	channel mysql.ReplicationChannel

	lagTime time.Time
	io      mysqlBoolState
	sql     mysqlBoolState
}

type mysqlInstanceState struct {
	instance mysql.Instance

	versionTime time.Time
	readOnly    mysqlBoolState
	scalars     [mysqlScalarCount]mysqlScalarState
	channels    map[string]*mysqlReplicationState
}

func (p *Provider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error) {
	if err := p.ready(); err != nil {
		return mysql.Snapshot{}, err
	}
	results, err := p.queryInstant(ctx, mysqlPromQL())
	if err != nil {
		return mysql.Snapshot{}, mysqlUnavailableError()
	}

	states := make(map[string]*mysqlInstanceState, len(results[0]))
	for _, series := range results[0] {
		host, name, address, key, ok := mysqlIdentity(series.Metric)
		if !ok {
			return mysql.Snapshot{}, mysqlUnavailableError()
		}
		if _, exists := states[key]; exists {
			return mysql.Snapshot{}, mysqlUnavailableError()
		}
		availability := mysql.AvailabilityUnknown
		if up, _, ok := mysqlBinary(series); ok {
			if *up {
				availability = mysql.AvailabilityUp
			} else {
				availability = mysql.AvailabilityDown
			}
		}
		states[key] = &mysqlInstanceState{
			instance: mysql.Instance{
				ID:           mysql.StableInstanceID(host, name, address),
				Name:         name,
				Address:      address,
				Host:         host,
				Availability: availability,
				Role:         mysql.RoleUnknown,
			},
			channels: make(map[string]*mysqlReplicationState),
		}
	}

	for _, series := range results[1] {
		state, ok := mysqlStateForSeries(states, series)
		if !ok {
			continue
		}
		mergeMySQLVersion(&state.instance.Version, &state.versionTime, series)
	}
	mergeMySQLScalars(states, results)
	mergeMySQLReplication(states, results)

	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	instances := make([]mysql.Instance, 0, len(keys))
	for _, key := range keys {
		state := states[key]
		finalizeMySQLInstance(state)
		instances = append(instances, state.instance)
	}
	return mysql.Snapshot{Instances: instances}, nil
}

func mysqlIdentity(labels map[string]string) (host, name, address, key string, ok bool) {
	host = labels["ident"]
	name = labels["instance"]
	address = labels["address"]
	if strings.TrimSpace(host) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(address) == "" {
		return "", "", "", "", false
	}
	return host, name, address, host + "\x00" + name + "\x00" + address, true
}

func mysqlBinary(series instantSeries) (*bool, time.Time, bool) {
	value, timestamp, ok := parseInstantValue(series.Value)
	if !ok {
		return nil, time.Time{}, false
	}
	var binary bool
	switch value {
	case 0:
		binary = false
	case 1:
		binary = true
	default:
		return nil, time.Time{}, false
	}
	return &binary, timestamp, true
}

func mergeMySQLScalar(target **float64, targetTime *time.Time, candidate instantSeries) {
	value, timestamp, ok := parseInstantValue(candidate.Value)
	if !ok {
		return
	}
	if newestMySQLValue(target, targetTime, value, timestamp) && !nonNegative(value) {
		*target = nil
	}
}

func nonNegative(value float64) bool {
	return value >= 0
}

func percentage(value float64) bool {
	return nonNegative(value) && value <= 100
}

func newestMySQLValue(current **float64, currentAt *time.Time, candidate float64, candidateAt time.Time) bool {
	if candidateAt.Before(*currentAt) {
		return false
	}
	if candidateAt.After(*currentAt) || currentAt.IsZero() && *current == nil {
		*current = &candidate
		*currentAt = candidateAt
		return true
	}
	if *current == nil {
		return false
	}
	if **current != candidate {
		*current = nil
		return false
	}
	return true
}

func mergeMySQLScalars(states map[string]*mysqlInstanceState, results [][]instantSeries) {
	for queryIndex := 2; queryIndex <= 9; queryIndex++ {
		for _, series := range results[queryIndex] {
			state, ok := mysqlStateForSeries(states, series)
			if !ok {
				continue
			}
			if queryIndex == 3 {
				mergeMySQLBool(&state.readOnly, series)
				continue
			}
			scalarIndex, ok := mysqlScalarIndex(queryIndex)
			if !ok {
				continue
			}
			scalar := &state.scalars[scalarIndex]
			mergeMySQLScalar(&scalar.value, &scalar.timestamp, series)
			if scalarIndex == mysqlBufferPoolUsage && scalar.value != nil && !percentage(*scalar.value) {
				scalar.value = nil
			}
		}
	}
}

func mysqlScalarIndex(queryIndex int) (int, bool) {
	switch queryIndex {
	case 2:
		return mysqlUptime, true
	case 4:
		return mysqlConnections, true
	case 5:
		return mysqlMaxConnections, true
	case 6:
		return mysqlThreadsRunning, true
	case 7:
		return mysqlQPS, true
	case 8:
		return mysqlSlowQueries, true
	case 9:
		return mysqlBufferPoolUsage, true
	default:
		return 0, false
	}
}

func mergeMySQLReplication(states map[string]*mysqlInstanceState, results [][]instantSeries) {
	for queryIndex := 10; queryIndex <= 12; queryIndex++ {
		for _, series := range results[queryIndex] {
			state, ok := mysqlStateForSeries(states, series)
			if !ok {
				continue
			}
			channelKey, ok := mysqlReplicationIdentity(series.Metric)
			if !ok {
				continue
			}
			replication, exists := state.channels[channelKey]
			if !exists {
				replication = &mysqlReplicationState{}
				state.channels[channelKey] = replication
			}
			switch queryIndex {
			case 10:
				mergeMySQLScalar(&replication.channel.LagSeconds, &replication.lagTime, series)
			case 11:
				mergeMySQLBool(&replication.io, series)
			case 12:
				mergeMySQLBool(&replication.sql, series)
			}
		}
	}
}

func mysqlStateForSeries(states map[string]*mysqlInstanceState, series instantSeries) (*mysqlInstanceState, bool) {
	_, _, _, key, ok := mysqlIdentity(series.Metric)
	if !ok {
		return nil, false
	}
	state, ok := states[key]
	return state, ok
}

func mysqlReplicationIdentity(labels map[string]string) (string, bool) {
	channel := labels["channel_name"]
	source := labels["master_host"]
	uuid := labels["master_uuid"]
	if strings.TrimSpace(channel) == "" || strings.TrimSpace(source) == "" || strings.TrimSpace(uuid) == "" {
		return "", false
	}
	return channel + "\x00" + source + "\x00" + uuid, true
}

func mergeMySQLVersion(target *string, targetTime *time.Time, candidate instantSeries) {
	_, timestamp, ok := parseInstantValue(candidate.Value)
	version := candidate.Metric["version"]
	if !ok || strings.TrimSpace(version) == "" || timestamp.Before(*targetTime) {
		return
	}
	if timestamp.After(*targetTime) || targetTime.IsZero() && *target == "" {
		*target = version
		*targetTime = timestamp
		return
	}
	if *target != version {
		*target = ""
	}
}

func mergeMySQLBool(target *mysqlBoolState, candidate instantSeries) {
	value, timestamp, ok := mysqlBinary(candidate)
	if !ok || timestamp.Before(target.timestamp) {
		return
	}
	if timestamp.After(target.timestamp) || target.timestamp.IsZero() && target.value == nil {
		target.value = value
		target.timestamp = timestamp
		return
	}
	if target.value != nil && *target.value != *value {
		target.value = nil
	}
}

func finalizeMySQLInstance(state *mysqlInstanceState) {
	if state.readOnly.value != nil {
		if *state.readOnly.value {
			state.instance.Role = mysql.RoleReadOnly
		} else {
			state.instance.Role = mysql.RoleWritable
		}
	}
	state.instance.UptimeSeconds = state.scalars[mysqlUptime].value
	state.instance.Connections = state.scalars[mysqlConnections].value
	state.instance.MaxConnections = state.scalars[mysqlMaxConnections].value
	state.instance.ThreadsRunning = state.scalars[mysqlThreadsRunning].value
	state.instance.QPS = state.scalars[mysqlQPS].value
	state.instance.SlowQueriesPerSecond = state.scalars[mysqlSlowQueries].value
	state.instance.BufferPoolUsagePercent = state.scalars[mysqlBufferPoolUsage].value

	channelKeys := make([]string, 0, len(state.channels))
	for key := range state.channels {
		channelKeys = append(channelKeys, key)
	}
	sort.Strings(channelKeys)
	state.instance.ReplicationChannels = make([]mysql.ReplicationChannel, 0, len(channelKeys))
	for _, key := range channelKeys {
		replication := state.channels[key]
		replication.channel.IORunning = replication.io.value
		replication.channel.SQLRunning = replication.sql.value
		state.instance.ReplicationChannels = append(state.instance.ReplicationChannels, replication.channel)
	}
}

func mysqlUnavailableError() error {
	return fmt.Errorf("%w: Nightingale MySQL 当前指标不可用", mysql.ErrUnavailable)
}
