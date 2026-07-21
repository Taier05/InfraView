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
