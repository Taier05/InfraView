package nightingale

const (
	elasticsearchClusterInfoUpQuery = iota
	elasticsearchNodeStatsUpQuery
	elasticsearchClusterHealthQuery
	elasticsearchNumberOfNodesQuery
	elasticsearchNumberOfDataNodesQuery
	elasticsearchActivePrimaryShardsQuery
	elasticsearchActiveShardsQuery
	elasticsearchRelocatingShardsQuery
	elasticsearchInitializingShardsQuery
	elasticsearchUnassignedShardsQuery
	elasticsearchPendingTasksQuery
	elasticsearchTaskMaxWaitingMillisQuery
	elasticsearchNodeRolesQuery
	elasticsearchHeapUsedQuery
	elasticsearchHeapMaxQuery
	elasticsearchDiskUsageQuery
	elasticsearchCPUUsageQuery
	elasticsearchIndexRateQuery
	elasticsearchSearchRateQuery
	elasticsearchDocumentsQuery
	elasticsearchStoreSizeQuery
	elasticsearchUptimeQuery
	elasticsearchThreadPoolQueueQuery
	elasticsearchRejectedRateQuery
	elasticsearchClusterInventoryQuery
	elasticsearchNodeInventoryQuery
	elasticsearchQueryCount
)

var fixedElasticsearchPromQL = [...]string{
	`elasticsearch_clusterinfo_up`,
	`elasticsearch_node_stats_up`,
	`elasticsearch_cluster_health_status`,
	`elasticsearch_cluster_health_number_of_nodes`,
	`elasticsearch_cluster_health_number_of_data_nodes`,
	`elasticsearch_cluster_health_active_primary_shards`,
	`elasticsearch_cluster_health_active_shards`,
	`elasticsearch_cluster_health_relocating_shards`,
	`elasticsearch_cluster_health_initializing_shards`,
	`elasticsearch_cluster_health_unassigned_shards`,
	`elasticsearch_cluster_health_number_of_pending_tasks`,
	`elasticsearch_cluster_health_task_max_waiting_in_queue_millis`,
	`elasticsearch_nodes_roles`,
	`elasticsearch_jvm_memory_used_bytes{area="heap"}`,
	`elasticsearch_jvm_memory_max_bytes{area="heap"}`,
	`max by (cluster, name, host, ident, instance, es_client_node, es_data_node, es_ingest_node, es_master_node) (100 * (1 - elasticsearch_filesystem_data_available_bytes / elasticsearch_filesystem_data_size_bytes))`,
	`elasticsearch_process_cpu_percent`,
	`rate(elasticsearch_indices_indexing_index_total[5m])`,
	`rate(elasticsearch_indices_search_query_total[5m])`,
	`elasticsearch_indices_docs`,
	`elasticsearch_indices_store_size_bytes`,
	`elasticsearch_jvm_uptime_seconds`,
	`max by (cluster, name, host, ident, instance) (elasticsearch_thread_pool_queue_count)`,
	`sum by (cluster, name, host, ident, instance) (rate(elasticsearch_thread_pool_rejected_count[5m]))`,
	`tlast_over_time(elasticsearch_clusterinfo_up[24h])`,
	`tlast_over_time(elasticsearch_jvm_uptime_seconds[24h])`,
}

func elasticsearchPromQL() []string {
	queries := make([]string, len(fixedElasticsearchPromQL))
	copy(queries, fixedElasticsearchPromQL[:])
	return queries
}
