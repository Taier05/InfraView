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
