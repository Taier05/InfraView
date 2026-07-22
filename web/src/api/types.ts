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

export interface OverviewData {
  total: number
  online: number
  offline: number
  unknown: number
  cpu_average: MetricValue
  memory_average: MetricValue
  trends: OverviewTrend[]
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

export type HostStatus = 'online' | 'offline' | 'unknown'

export interface FilesystemMetric {
  mountpoint: string
  usage: MetricValue
}

export interface CurrentMetrics {
  timestamp: string
  cpu_usage: MetricValue
  memory_usage: MetricValue
  load_1: MetricValue
  disk_read_bytes_per_second: MetricValue
  disk_write_bytes_per_second: MetricValue
  network_receive_bytes_per_second: MetricValue
  network_transmit_bytes_per_second: MetricValue
  filesystems: FilesystemMetric[]
}

export interface HostSummary {
  id: string
  name: string
  ip: string
  os: string
  status: HostStatus
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

export type HostDetailResponse = ApiResponse<HostSummary>

export type HostMetricKey =
  | 'cpu_usage'
  | 'memory_usage'
  | 'load_1'
  | 'disk_usage'
  | 'disk_read_bytes_per_second'
  | 'disk_write_bytes_per_second'
  | 'network_receive_bytes_per_second'
  | 'network_transmit_bytes_per_second'

export interface HostMetricSeries {
  metric: HostMetricKey
  points: TrendPoint[]
}

export interface HostMetricsData {
  host_id: string
  range: '1h' | '6h' | '24h' | '7d'
  from: string
  to: string
  step_seconds: number
  series: HostMetricSeries[]
}

export type HostMetricsResponse = ApiResponse<HostMetricsData>
