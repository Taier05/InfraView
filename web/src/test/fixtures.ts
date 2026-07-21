import { HttpResponse, http } from 'msw'

export const SESSION_PATH = '/api/v1/session'
export const OVERVIEW_PATH = '*/api/v1/overview'

export interface SessionFixture {
  data: {
    authenticated: true
    username: string
  }
  meta: {
    request_id: string
    stale: false
  }
}

export interface ErrorFixture {
  code: string
  message: string
  request_id: string
  retryable: boolean
}

export type MetricLevelFixture =
  | 'normal'
  | 'warning'
  | 'critical'
  | 'unknown'

export interface OverviewFixture {
  data: {
    total: number
    online: number
    offline: number
    unknown: number
    cpu_average: {
      value: number | null
      level: MetricLevelFixture
    }
    memory_average: {
      value: number | null
      level: MetricLevelFixture
    }
  }
  meta: {
    request_id: string
    stale: boolean
    collected_at: string
  }
}

let authenticatedUsername: string | null = null

export function sessionFixture(username = 'admin'): SessionFixture {
  return {
    data: { authenticated: true, username },
    meta: { request_id: 'req-session-001', stale: false },
  }
}

export const unauthenticatedFixture: ErrorFixture = {
  code: 'unauthorized',
  message: '请先登录',
  request_id: 'req-session-unauthorized-001',
  retryable: false,
}

export const invalidCredentialsFixture: ErrorFixture = {
  code: 'invalid_credentials',
  message: '用户名或密码错误',
  request_id: 'req-login-invalid-001',
  retryable: false,
}

export function overviewFixture(
  overrides: {
    data?: Partial<OverviewFixture['data']>
    meta?: Partial<OverviewFixture['meta']>
  } = {},
): OverviewFixture {
  return {
    data: {
      total: 12,
      online: 9,
      offline: 2,
      unknown: 1,
      cpu_average: { value: 73.5, level: 'critical' },
      memory_average: { value: 42, level: 'warning' },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-overview-001',
      stale: false,
      collected_at: '2026-07-21T00:30:00.000Z',
      ...overrides.meta,
    },
  }
}

export function resetFixtureState() {
  authenticatedUsername = null
}

export const handlers = [
  http.get(SESSION_PATH, () => {
    if (authenticatedUsername === null) {
      return HttpResponse.json(unauthenticatedFixture, { status: 401 })
    }

    return HttpResponse.json(sessionFixture(authenticatedUsername))
  }),
  http.post(SESSION_PATH, async ({ request }) => {
    const credentials = (await request.json()) as {
      username?: string
      password?: string
    }

    if (
      credentials.username !== 'admin' ||
      credentials.password !== 'secret-value'
    ) {
      return HttpResponse.json(invalidCredentialsFixture, { status: 401 })
    }

    authenticatedUsername = credentials.username
    return new HttpResponse(null, { status: 204 })
  }),
  http.delete(SESSION_PATH, () => {
    authenticatedUsername = null
    return new HttpResponse(null, { status: 204 })
  }),
  http.get(OVERVIEW_PATH, () => HttpResponse.json(overviewFixture())),
]
