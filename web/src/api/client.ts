import type { ApiErrorBody } from './types'

const fallbackMessages: Record<number, string> = {
  400: '请求内容无效，请检查后重试',
  401: '请先登录',
  403: '当前账号无权访问此内容',
  404: '请求的内容不存在',
  429: '请求过于频繁，请稍后重试',
  500: '服务器发生错误，请稍后重试',
  503: '服务暂时不可用，请稍后重试',
}

export class APIError extends Error {
  readonly name = 'APIError'

  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly requestID: string,
    readonly retryable: boolean,
  ) {
    super(message)
  }
}

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<ApiErrorBody>
  return (
    typeof candidate.code === 'string' &&
    typeof candidate.message === 'string' &&
    typeof candidate.request_id === 'string' &&
    typeof candidate.retryable === 'boolean'
  )
}

function fallbackMessage(status: number) {
  return fallbackMessages[status] ?? '请求失败，请稍后重试'
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      credentials: 'same-origin',
      headers,
    })
  } catch {
    throw new APIError(
      0,
      'network_error',
      '无法连接服务器，请检查网络后重试',
      '',
      true,
    )
  }

  if (!response.ok) {
    let body: unknown
    if (response.headers.get('Content-Type')?.includes('application/json')) {
      try {
        body = await response.json()
      } catch {
        body = undefined
      }
    }

    if (isApiErrorBody(body)) {
      throw new APIError(
        response.status,
        body.code,
        body.message,
        body.request_id,
        body.retryable,
      )
    }

    throw new APIError(
      response.status,
      'request_failed',
      fallbackMessage(response.status),
      '',
      response.status >= 500,
    )
  }

  if (response.status === 204) return undefined as T

  if (!response.headers.get('Content-Type')?.includes('application/json')) {
    throw new APIError(
      response.status,
      'invalid_response',
      '服务器响应格式无效',
      '',
      false,
    )
  }

  return (await response.json()) as T
}
