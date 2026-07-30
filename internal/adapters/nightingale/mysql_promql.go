package nightingale

func mysqlPromQL() []string {
	return []string{
		"mysql_up",
		"mysql_version_info",
		"mysql_global_status_uptime",
		"mysql_global_variables_read_only",
		"mysql_global_status_threads_connected",
		"mysql_global_variables_max_connections",
		"mysql_global_status_threads_running",
		"rate(mysql_global_status_questions[5m])",
		"rate(mysql_global_status_slow_queries[5m])",
		"mysql_global_status_buffer_pool_pages_utilization",
		"mysql_global_variables_innodb_buffer_pool_size",
		`sum by (ident, instance, address) (rate(mysql_global_status_commands_total{command=~"commit|rollback"}[5m]))`,
		"mysql_slave_status_seconds_behind_master",
		"mysql_slave_status_slave_io_running",
		"mysql_slave_status_slave_sql_running",
		"tlast_over_time(mysql_up[24h])",
	}
}
