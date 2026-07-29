import { HttpResponse, http } from 'msw'

export const SESSION_PATH = '/api/v1/session'
export const OVERVIEW_PATH = '*/api/v1/overview'
export const MYSQL_INSTANCES_PATH = '*/api/v1/mysql/instances'

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
    alerts: {
      affected_hosts: number
      warning_hosts: number
      critical_hosts: number
      cpu: { warning: number; critical: number }
      memory: { warning: number; critical: number }
      io: { warning: number; critical: number }
      network: { warning: number; critical: number }
    }
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
      cpu_cores: number | null
      memory_total_bytes: number | null
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
        io_busy_percent: {
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

export interface MySQLInstancePageFixture {
  data: {
    instances: Array<{
      id: string
      name: string
      address: string
      host: string
      version: string
      role: 'writable' | 'read_only' | 'unknown'
      connections: number | null
      max_connections: number | null
      connection_usage_percent: number | null
      threads_running: number | null
      qps: number | null
      slow_queries_per_second: number | null
      buffer_pool_usage_percent: number | null
      uptime_seconds: number | null
      replication: {
        state:
          | 'normal'
          | 'threads_stopped'
          | 'not_configured'
          | 'unknown'
        lag_seconds: number | null
        level: MetricLevelFixture
      }
      status: MetricLevelFixture
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
      alerts: {
        affected_hosts: 7,
        warning_hosts: 3,
        critical_hosts: 4,
        cpu: { warning: 1, critical: 1 },
        memory: { warning: 0, critical: 1 },
        io: { warning: 2, critical: 0 },
        network: { warning: 1, critical: 2 },
      },
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
          cpu_cores: 8,
          memory_total_bytes: 32 * 1024 * 1024 * 1024,
          status: 'online',
          status_time: '2026-07-21T00:30:00.000Z',
          uptime_seconds: 93_600,
          metrics: {
            timestamp: '2026-07-21T00:30:00.000Z',
            cpu_usage: availableMetric(23.46, 'normal'),
            memory_usage: availableMetric(67.04, 'warning'),
            load_1: availableMetric(1.25, 'normal'),
            io_busy_percent: availableMetric(91.2, 'critical'),
            network_receive_bytes_per_second: availableMetric(4096, 'critical'),
            network_transmit_bytes_per_second: availableMetric(8192, 'warning'),
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
          cpu_cores: null,
          memory_total_bytes: null,
          status: 'offline',
          status_time: '2026-07-20T22:30:00.000Z',
          uptime_seconds: 7_200,
          metrics: {
            timestamp: '2026-07-20T22:30:00.000Z',
            cpu_usage: missingMetric,
            memory_usage: missingMetric,
            load_1: missingMetric,
            io_busy_percent: missingMetric,
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

export function mysqlInstancePageFixture(
  overrides: {
    data?: Partial<MySQLInstancePageFixture['data']>
    meta?: Partial<MySQLInstancePageFixture['meta']>
  } = {},
): MySQLInstancePageFixture {
  return {
    data: {
      instances: [
        {
          id: 'mysql-fixture-001',
          name: 'fixture-mysql-a',
          address: '192.0.2.101:3306',
          host: 'fixture-db-host-a',
          version: '8.4.1',
          role: 'writable',
          connections: 32,
          max_connections: 200,
          connection_usage_percent: 16,
          threads_running: 5,
          qps: 123.456,
          slow_queries_per_second: 0.125,
          buffer_pool_usage_percent: 82.34,
          uptime_seconds: 183_600,
          replication: {
            state: 'normal',
            lag_seconds: 2,
            level: 'normal',
          },
          status: 'normal',
        },
        {
          id: 'mysql-fixture-002',
          name: 'fixture-mysql-b',
          address: '192.0.2.102:3306',
          host: 'fixture-db-host-b',
          version: '8.0.39',
          role: 'read_only',
          connections: 120,
          max_connections: 160,
          connection_usage_percent: 75,
          threads_running: 18,
          qps: 87,
          slow_queries_per_second: 1.2,
          buffer_pool_usage_percent: 91.25,
          uptime_seconds: 43_200,
          replication: {
            state: 'threads_stopped',
            lag_seconds: null,
            level: 'critical',
          },
          status: 'critical',
        },
        {
          id: 'mysql-fixture-003',
          name: 'fixture-mysql-c',
          address: '192.0.2.103:3306',
          host: 'fixture-db-host-c',
          version: '5.7.44',
          role: 'writable',
          connections: 198,
          max_connections: 200,
          connection_usage_percent: 99,
          threads_running: 43,
          qps: 212.5,
          slow_queries_per_second: 4.75,
          buffer_pool_usage_percent: 98.6,
          uptime_seconds: 900,
          replication: {
            state: 'not_configured',
            lag_seconds: null,
            level: 'normal',
          },
          status: 'normal',
        },
        {
          id: 'mysql-fixture-004',
          name: 'fixture-mysql-d',
          address: '192.0.2.104:3306',
          host: 'fixture-db-host-d',
          version: '',
          role: 'read_only',
          connections: null,
          max_connections: null,
          connection_usage_percent: null,
          threads_running: null,
          qps: null,
          slow_queries_per_second: null,
          buffer_pool_usage_percent: null,
          uptime_seconds: null,
          replication: {
            state: 'unknown',
            lag_seconds: null,
            level: 'unknown',
          },
          status: 'unknown',
        },
        {
          id: 'mysql-fixture-005',
          name: 'fixture-mysql-e',
          address: '192.0.2.105:3306',
          host: 'fixture-db-host-e',
          version: 'unknown',
          role: 'read_only',
          connections: 96,
          max_connections: 200,
          connection_usage_percent: 48,
          threads_running: 11,
          qps: 64.75,
          slow_queries_per_second: 0.25,
          buffer_pool_usage_percent: 73.5,
          uptime_seconds: 266_400,
          replication: {
            state: 'normal',
            lag_seconds: 12,
            level: 'warning',
          },
          status: 'warning',
        },
      ],
      total: 64,
      page: 1,
      page_size: 20,
      total_pages: 4,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-mysql-instances-001',
      stale: false,
      collected_at: '2026-07-28T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function resetFixtureState() {
  authenticatedUsername = null
}

export const mysqlHandlers = [
  http.get(MYSQL_INSTANCES_PATH, () =>
    HttpResponse.json(mysqlInstancePageFixture()),
  ),
]

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
  http.get('/api/v1/datasource/status', () =>
    HttpResponse.json({
      data: {
        type: 'mock',
        healthy: true,
        checked_at: '2026-07-22T02:03:04.000Z',
        refresh_interval_seconds: 15,
      },
      meta: {
        request_id: 'req-datasource-default-001',
        stale: false,
        collected_at: '2026-07-22T02:03:05.000Z',
      },
    }),
  ),
]
