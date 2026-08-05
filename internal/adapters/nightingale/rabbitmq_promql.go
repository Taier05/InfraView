package nightingale

const (
	rabbitMQIdentityQuery = iota
	rabbitMQBuildInfoQuery
	rabbitMQUptimeQuery
	rabbitMQMemoryAlarmQuery
	rabbitMQDiskAlarmQuery
	rabbitMQFileDescriptorAlarmQuery
	rabbitMQUnreachablePeersQuery
	rabbitMQMemoryUsedQuery
	rabbitMQMemoryLimitQuery
	rabbitMQDiskAvailableQuery
	rabbitMQDiskLimitQuery
	rabbitMQOpenFDsQuery
	rabbitMQMaxFDsQuery
	rabbitMQErlangProcessesUsedQuery
	rabbitMQErlangProcessesLimitQuery
	rabbitMQConnectionsQuery
	rabbitMQQueuesQuery
	rabbitMQMessagesQuery
	rabbitMQPublishRateQuery
	rabbitMQDeliverRateQuery
	rabbitMQInventoryQuery
	rabbitMQCollectionQuery
	rabbitMQQueryCount
)

var fixedRabbitMQPromQL = [...]string{
	"rabbitmq_identity_info",
	"rabbitmq_build_info",
	"rabbitmq_erlang_uptime_seconds",
	"rabbitmq_alarms_memory_used_watermark",
	"rabbitmq_alarms_free_disk_space_watermark",
	"rabbitmq_alarms_file_descriptor_limit",
	"rabbitmq_unreachable_cluster_peers_count",
	"rabbitmq_process_resident_memory_bytes",
	"rabbitmq_resident_memory_limit_bytes",
	"rabbitmq_disk_space_available_bytes",
	"rabbitmq_disk_space_available_limit_bytes",
	"rabbitmq_process_open_fds",
	"rabbitmq_process_max_fds",
	"rabbitmq_erlang_processes_used",
	"rabbitmq_erlang_processes_limit",
	"rabbitmq_connections",
	"rabbitmq_queues",
	"rabbitmq_queue_messages",
	"sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_received_total[5m]))",
	"sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_delivered_total[5m]))",
	"tlast_over_time(rabbitmq_identity_info[24h])",
	"tlast_over_time(rabbitmq_erlang_uptime_seconds[24h])",
}

func rabbitMQPromQL() []string {
	queries := make([]string, len(fixedRabbitMQPromQL))
	copy(queries, fixedRabbitMQPromQL[:])
	return queries
}
