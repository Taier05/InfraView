package nightingale

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Taier05/InfraView/internal/datasource"
)

const currentCollectionTimestampQueryIndex = 6

func inventoryPromQL(metric string, hostIDs []string) string {
	return metric + "{" + identMatcher(hostIDs) + "}"
}

func currentPromQL(hostIDs []string, excludeExpr string) []string {
	ident := identMatcher(hostIDs)
	exclude := `interface!~` + strconv.Quote(excludeExpr)
	return []string{
		`cpu_usage_active{cpu="cpu-total",` + ident + `}`,
		`mem_used_percent{` + ident + `}`,
		`system_load1{` + ident + `}`,
		`max by (ident) (diskio_io_util{` + ident + `})`,
		`sum by (ident) (rate(net_bytes_sent{` + ident + `,` + exclude + `}[2m]))`,
		`sum by (ident) (rate(net_bytes_recv{` + ident + `,` + exclude + `}[2m]))`,
		`tlast_over_time(system_uptime{` + ident + `}[24h])`,
	}
}

func rangePromQL(metric datasource.MetricKey, hostIDs []string, excludeExpr string) (string, bool) {
	queries := currentPromQL(hostIDs, excludeExpr)
	switch metric {
	case datasource.MetricCPUUsage:
		return queries[0], true
	case datasource.MetricMemoryUsage:
		return queries[1], true
	case datasource.MetricLoad1:
		return queries[2], true
	case datasource.MetricIOBusyPercent:
		return queries[3], true
	case datasource.MetricNetworkTransmitBytesPerSecond:
		return queries[4], true
	case datasource.MetricNetworkReceiveBytesPerSecond:
		return queries[5], true
	default:
		return "", false
	}
}

func aggregatePromQL(metric datasource.MetricKey, excludeExpr string) (string, bool) {
	exclude := `interface!~` + strconv.Quote(excludeExpr)
	switch metric {
	case datasource.MetricCPUUsage:
		return `avg(cpu_usage_active{cpu="cpu-total",ident!=""})`, true
	case datasource.MetricMemoryUsage:
		return `avg(mem_used_percent{ident!=""})`, true
	case datasource.MetricLoad1:
		return `avg(system_load1{ident!=""})`, true
	case datasource.MetricIOBusyPercent:
		return `avg(max by (ident) (diskio_io_util{ident!=""}))`, true
	case datasource.MetricNetworkTransmitBytesPerSecond:
		return `avg(sum by (ident) (rate(net_bytes_sent{ident!="",` + exclude + `}[2m])))`, true
	case datasource.MetricNetworkReceiveBytesPerSecond:
		return `avg(sum by (ident) (rate(net_bytes_recv{ident!="",` + exclude + `}[2m])))`, true
	default:
		return "", false
	}
}

func identMatcher(hostIDs []string) string {
	escaped := make([]string, len(hostIDs))
	for i, hostID := range hostIDs {
		escaped[i] = regexp.QuoteMeta(hostID)
	}
	pattern := `^(?:` + strings.Join(escaped, `|`) + `)$`
	return `ident=~` + strconv.Quote(pattern)
}
