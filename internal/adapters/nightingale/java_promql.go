package nightingale

const (
	javaHealthLatencyQuery = iota
	javaHealthUpQuery
	javaPortUpQuery
	javaProcessCountQuery
	javaProcessCPUQuery
	javaProcessMemoryBytesQuery
	javaProcessMemoryPercentQuery
	javaPortConsistentQuery
	javaProcessStartTimeQuery
	javaProcessUpQuery
	javaInventoryQuery
	javaQueryCount
)

var fixedJavaPromQL = [...]string{
	"service_health_latency_ms",
	"service_health_up",
	"service_port_up",
	"service_process_count",
	"service_process_cpu_percent",
	"service_process_memory_bytes",
	"service_process_memory_percent",
	"service_process_port_consistent",
	"service_process_start_time_seconds",
	"service_process_up",
	"tlast_over_time(service_process_up[24h])",
}

func javaPromQL() []string {
	queries := make([]string, len(fixedJavaPromQL))
	copy(queries, fixedJavaPromQL[:])
	return queries
}
