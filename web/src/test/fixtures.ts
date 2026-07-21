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
    trends: Array<{
      key: 'cpu_usage' | 'memory_usage'
      unit: '%'
      points: Array<{
        timestamp: string
        value: number | null
      }>
    }>
  }
  meta: {
    request_id: string
    stale: boolean
    collected_at: string
  }
}

export type HostStatusFixture = 'online' | 'offline' | 'unknown'

export interface HostPageFixture {
  data: {
    hosts: Array<{
      id: string
      name: string
      ip: string
      os: string
      status: HostStatusFixture
      status_time: string
      uptime_seconds: number
      metrics: {
        timestamp: string
        cpu_usage: {
          value: number | null
          level: MetricLevelFixture
        }
        memory_usage: {
          value: number | null
          level: MetricLevelFixture
        }
        load_1: {
          value: number | null
          level: MetricLevelFixture
        }
        disk_read_bytes_per_second: {
          value: number | null
          level: MetricLevelFixture
        }
        disk_write_bytes_per_second: {
          value: number | null
          level: MetricLevelFixture
        }
        network_receive_bytes_per_second: {
          value: number | null
          level: MetricLevelFixture
        }
        network_transmit_bytes_per_second: {
          value: number | null
          level: MetricLevelFixture
        }
        filesystems: Array<{
          mountpoint: string
          usage: {
            value: number | null
            level: MetricLevelFixture
          }
        }>
      }
    }>
    total: number
    page: number
    page_size: number
    total_pages: number
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
      trends: [
        {
          key: 'cpu_usage',
          unit: '%',
          points: [
            { timestamp: '2026-07-20T00:30:00.000Z', value: 31 },
            { timestamp: '2026-07-21T00:30:00.000Z', value: 52 },
          ],
        },
        {
          key: 'memory_usage',
          unit: '%',
          points: [
            { timestamp: '2026-07-20T00:30:00.000Z', value: 45 },
            { timestamp: '2026-07-21T00:30:00.000Z', value: 61 },
          ],
        },
      ],
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

export function hostPageFixture(
  overrides: {
    data?: Partial<HostPageFixture['data']>
    meta?: Partial<HostPageFixture['meta']>
  } = {},
): HostPageFixture {
  const availableMetric = (value: number, level: MetricLevelFixture) => ({
    value,
    level,
  })
  const missingMetric = { value: null, level: 'unknown' as const }

  return {
    data: {
      hosts: [
        {
          id: 'host-001',
          name: 'linux-app-01',
          ip: '192.0.2.11',
          os: 'Ubuntu 24.04',
          status: 'online',
          status_time: '2026-07-21T00:30:00.000Z',
          uptime_seconds: 93_600,
          metrics: {
            timestamp: '2026-07-21T00:30:00.000Z',
            cpu_usage: availableMetric(23.46, 'normal'),
            memory_usage: availableMetric(67.04, 'warning'),
            load_1: availableMetric(1.25, 'normal'),
            disk_read_bytes_per_second: availableMetric(1024, 'normal'),
            disk_write_bytes_per_second: availableMetric(2048, 'normal'),
            network_receive_bytes_per_second: availableMetric(4096, 'normal'),
            network_transmit_bytes_per_second: availableMetric(8192, 'normal'),
            filesystems: [
              {
                mountpoint: '/',
                usage: availableMetric(51.2, 'normal'),
              },
            ],
          },
        },
        {
          id: 'host-002',
          name: 'linux-db-02',
          ip: '192.0.2.22',
          os: 'Debian 13',
          status: 'offline',
          status_time: '2026-07-20T22:30:00.000Z',
          uptime_seconds: 7_200,
          metrics: {
            timestamp: '2026-07-20T22:30:00.000Z',
            cpu_usage: missingMetric,
            memory_usage: missingMetric,
            load_1: missingMetric,
            disk_read_bytes_per_second: missingMetric,
            disk_write_bytes_per_second: missingMetric,
            network_receive_bytes_per_second: missingMetric,
            network_transmit_bytes_per_second: missingMetric,
            filesystems: [],
          },
        },
      ],
      total: 41,
      page: 1,
      page_size: 20,
      total_pages: 3,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-hosts-001',
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
