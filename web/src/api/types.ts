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

export interface DataSourceStatusData {
  healthy: boolean
  checked_at: string
}

export type DataSourceStatusResponse = ApiResponse<DataSourceStatusData>
