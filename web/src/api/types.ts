export interface ResponseMeta {
  request_id: string
  stale: boolean
  collected_at?: string
}

export interface ApiResponse<T> {
  data: T
  meta: ResponseMeta
}

export interface ApiErrorBody {
  code: string
  message: string
  request_id: string
  retryable: boolean
}

export interface SessionData {
  authenticated: true
  username: string
}

export interface LoginCredentials {
  username: string
  password: string
}

export type MetricLevel = 'normal' | 'warning' | 'critical' | 'unknown'

export interface MetricValue {
  value: number | null
  level: MetricLevel
}

export interface AlertCount {
  warning: number
  critical: number
}

export interface OverviewAlerts {
  affected_hosts: number
  warning_hosts: number
  critical_hosts: number
  cpu: AlertCount
  memory: AlertCount
  io: AlertCount
  network: AlertCount
}

export interface OverviewData {
  total: number
  online: number
  offline: number
  unknown: number
  cpu_average: MetricValue
  memory_average: MetricValue
  trends: OverviewTrend[]
  alerts: OverviewAlerts
}

export interface TrendPoint {
  timestamp: string
  value: number | null
}

export interface OverviewTrend {
  key: 'cpu_usage' | 'memory_usage'
  unit: string
  points: TrendPoint[]
}

export type OverviewResponse = ApiResponse<OverviewData>

export interface MySQLOverviewData {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
  affected_instances: number
  warning_instances: number
  critical_instances: number
  alerts: {
    availability: AlertCount
    replication_threads: AlertCount
    replication_lag: AlertCount
    replication_data: AlertCount
  }
}

export type MySQLOverviewResponse = ApiResponse<MySQLOverviewData>

export type HostStatus = 'online' | 'offline' | 'unknown'

export interface CurrentMetrics {
  timestamp: string
  cpu_usage: MetricValue
  memory_usage: MetricValue
  load_1: MetricValue
  io_busy_percent: MetricValue
  network_receive_bytes_per_second: MetricValue
  network_transmit_bytes_per_second: MetricValue
}

export interface HostSummary {
  id: string
  name: string
  ip: string
  os: string
  cpu_cores: number | null
  memory_total_bytes: number | null
  status: HostStatus
  collection_level: MetricLevel
  status_time: string
  uptime_seconds: number
  metrics: CurrentMetrics
}

export interface HostPageData {
  hosts: HostSummary[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type HostPageResponse = ApiResponse<HostPageData>

export type MySQLRole = 'writable' | 'read_only' | 'unknown'

export type MySQLReplicationState =
  | 'normal'
  | 'threads_stopped'
  | 'not_configured'
  | 'unknown'

export interface MySQLInstance {
  id: string
  name: string
  address: string
  host: string
  version: string
  role: MySQLRole
  connections: number | null
  max_connections: number | null
  connection_usage_percent: number | null
  threads_running: number | null
  qps: number | null
  tps: number | null
  slow_queries_per_second: number | null
  buffer_pool_size_bytes: number | null
  buffer_pool_usage_percent: number | null
  uptime_seconds: number | null
  replication: {
    state: MySQLReplicationState
    lag_seconds: number | null
    level: MetricLevel
  }
  status: MetricLevel
  collection_level: MetricLevel
}

export interface MySQLInstancePageData {
  instances: MySQLInstance[]
  available_labels: string[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type MySQLInstancePageResponse = ApiResponse<MySQLInstancePageData>

export type RedisRole = 'master' | 'slave' | 'unknown'
export type RedisStatusSource =
  | 'availability'
  | 'replication'
  | 'memory'
  | 'connection'
  | 'collection'
  | 'normal'
  | 'unknown'

export interface RedisReplication {
  connected_replicas: number | null
  master_link_up: boolean | null
  master_last_io_seconds_ago: number | null
  master_sync_in_progress: boolean | null
  worst_replica_lag_seconds: number | null
}

export interface RedisInstance {
  id: string
  address: string
  availability: 'up' | 'down' | 'unknown'
  role: RedisRole
  cluster_enabled: boolean | null
  used_memory_bytes: number | null
  max_memory_bytes: number | null
  memory_usage_percent: number | null
  connected_clients: number | null
  max_clients: number | null
  connection_usage_percent: number | null
  blocked_clients: number | null
  qps: number | null
  hit_rate: number | null
  keys: number | null
  expired_keys_per_second: number | null
  evicted_keys_per_second: number | null
  rejected_connections_rate: number | null
  replication: RedisReplication
  uptime_seconds: number | null
  status: MetricLevel
  status_source: RedisStatusSource
  collection_level: MetricLevel
}

export interface RedisInstancePageData {
  instances: RedisInstance[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
export type RedisInstancePageResponse = ApiResponse<RedisInstancePageData>

export interface RedisOverviewData {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
  affected_instances: number
  warning_instances: number
  critical_instances: number
  roles: {
    master: number
    slave: number
    unknown: number
  }
  alerts: {
    availability: AlertCount
    memory: AlertCount
    connection: AlertCount
    replication: AlertCount
  }
}
export type RedisOverviewResponse = ApiResponse<RedisOverviewData>

export type ElasticsearchHealth = 'green' | 'yellow' | 'red' | 'unknown'

export type ElasticsearchRole =
  | 'master'
  | 'data'
  | 'data_content'
  | 'data_hot'
  | 'data_warm'
  | 'data_cold'
  | 'data_frozen'
  | 'ingest'
  | 'ml'
  | 'transform'
  | 'remote_cluster_client'
  | 'client'

export type ElasticsearchNodeStatusSource =
  | 'collection'
  | 'disk'
  | 'jvm'
  | 'thread_pool'
  | 'normal'
  | 'unknown'

export interface ElasticsearchNode {
  id: string
  name: string
  cluster: string
  address: string
  roles: ElasticsearchRole[]
  cluster_health: ElasticsearchHealth
  heap_usage_percent: number | null
  disk_usage_percent: number | null
  cpu_usage_percent: number | null
  index_rate: number | null
  search_rate: number | null
  documents: number | null
  store_size_bytes: number | null
  thread_pool_queue: number | null
  rejected_rate: number | null
  uptime_seconds: number | null
  status: MetricLevel
  status_source: ElasticsearchNodeStatusSource
  collection_level: MetricLevel
}

export interface ElasticsearchLevelCounts {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
}

export interface ElasticsearchOverviewData {
  status: MetricLevel
  clusters: ElasticsearchLevelCounts
  nodes: ElasticsearchLevelCounts
  alerts: {
    cluster_health: AlertCount
    node_resource: AlertCount
    unassigned_shards: AlertCount
    request_rejections: AlertCount
  }
}

export interface ElasticsearchNodePageData {
  nodes: ElasticsearchNode[]
  available_clusters: string[]
  available_roles: ElasticsearchRole[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type ElasticsearchOverviewResponse =
  ApiResponse<ElasticsearchOverviewData>
export type ElasticsearchNodePageResponse =
  ApiResponse<ElasticsearchNodePageData>

export type RabbitMQNodeStatusSource =
  | 'alarm'
  | 'collection'
  | 'memory'
  | 'disk'
  | 'file_descriptor'
  | 'erlang_process'
  | 'normal'
  | 'unknown'

export interface RabbitMQNode {
  id: string
  name: string
  cluster: string
  address: string
  version: string
  memory_usage_percent: number | null
  disk_available_bytes: number | null
  file_descriptor_usage_percent: number | null
  erlang_process_usage_percent: number | null
  connections: number | null
  queues: number | null
  messages: number | null
  publish_rate: number | null
  deliver_rate: number | null
  uptime_seconds: number | null
  status: MetricLevel
  status_source: RabbitMQNodeStatusSource
  collection_level: MetricLevel
}

export interface RabbitMQLevelCounts {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
}

export interface RabbitMQAlertCount {
  warning: number
  critical: number
  unknown: number
}

export interface RabbitMQOverviewData {
  status: MetricLevel
  clusters: RabbitMQLevelCounts
  nodes: RabbitMQLevelCounts
  alerts: {
    cluster_connectivity: RabbitMQAlertCount
    resource_alarms: RabbitMQAlertCount
    resource_pressure: RabbitMQAlertCount
    collection: RabbitMQAlertCount
  }
}

export interface RabbitMQNodePageData {
  nodes: RabbitMQNode[]
  available_clusters: string[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type RabbitMQOverviewResponse = ApiResponse<RabbitMQOverviewData>
export type RabbitMQNodePageResponse = ApiResponse<RabbitMQNodePageData>

export type JavaStatusSource =
  | 'health'
  | 'port'
  | 'process'
  | 'consistency'
  | 'collection'
  | 'normal'
  | 'unknown'

export interface JavaService {
  id: string
  name: string
  business: string
  address: string
  health_up: boolean | null
  health_latency_ms: number | null
  port_up: boolean | null
  process_up: boolean | null
  process_count: string | null
  port_consistent: boolean | null
  cpu_usage_percent: number | null
  memory_bytes: string | null
  memory_usage_percent: number | null
  uptime_seconds: string | null
  status: MetricLevel
  status_source: JavaStatusSource
  collection_level: MetricLevel
}

export interface JavaLevelCounts {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
}

export interface JavaAlertCount {
  warning: number
  critical: number
  unknown: number
}

export interface JavaOverviewData {
  status: MetricLevel
  services: JavaLevelCounts
  alerts: {
    health: JavaAlertCount
    port: JavaAlertCount
    process: JavaAlertCount
    collection: JavaAlertCount
  }
}

export interface JavaServicePageData {
  services: JavaService[]
  available_names: string[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type JavaOverviewResponse = ApiResponse<JavaOverviewData>
export type JavaServicePageResponse = ApiResponse<JavaServicePageData>

export type DiskSMARTHealth = 'healthy' | 'failed' | 'unknown'

export type DiskStatusSource =
  | 'smart_health'
  | 'device_warning'
  | 'attribute_failure'
  | 'collection'
  | 'unknown'
  | 'normal'

export interface DiskErrorCounters {
  pending_sectors: number | null
  reallocated_sectors: number | null
  uncorrectable_sectors: number | null
  udma_crc_errors: number | null
  media_integrity_errors: number | null
  error_log_entries: number | null
  unsafe_shutdowns: number | null
}

export interface DiskDevice {
  id: string
  host: string
  device: string
  model: string
  capacity_bytes: number | null
  smart_health: DiskSMARTHealth
  temperature_celsius: number | null
  lifetime_used_percent: number | null
  power_on_hours: number | null
  errors: DiskErrorCounters
  status: MetricLevel
  status_source: DiskStatusSource
  collection_level: MetricLevel
}

export interface DiskDevicePageData {
  devices: DiskDevice[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type DiskDevicePageResponse = ApiResponse<DiskDevicePageData>

export interface DiskOverviewData {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
  affected_devices: number
  warning_devices: number
  critical_devices: number
  alerts: {
    smart_health: AlertCount
    device_warning: AlertCount
    attribute_failure: AlertCount
    collection: AlertCount
  }
}

export type DiskOverviewResponse = ApiResponse<DiskOverviewData>

export interface DataSourceStatusData {
  type: 'mock' | 'nightingale'
  healthy: boolean
  checked_at: string
  refresh_interval_seconds: number
}

export type DataSourceStatusResponse = ApiResponse<DataSourceStatusData>
