package nightingale

var fixedDiskPromQL = []string{
	`smart_device_health_ok`,
	`smart_device_temp_c`,
	`smart_attribute_temperature_celsius`,
	`smart_attribute_percentage_used`,
	`smart_attribute_power_on_hours`,
	`smart_attribute_critical_warning`,
	`smart_attribute_available_spare`,
	`smart_attribute_available_spare_threshold`,
	`smart_attribute_value{fail=~"FAILING_NOW|In_the_past"}`,
	`smart_device_pending_sector_count`,
	`smart_device_reallocated_sectors_count`,
	`smart_device_uncorrectable_sector_count`,
	`smart_device_udma_crc_errors`,
	`smart_attribute_media_and_data_integrity_errors`,
	`smart_attribute_error_information_log_entries`,
	`smart_attribute_unsafe_shutdowns`,
	`smart_disk_capacity_bytes`,
	`tlast_over_time(smart_device_health_ok[24h])`,
}

func diskPromQL() []string {
	return append([]string(nil), fixedDiskPromQL...)
}
