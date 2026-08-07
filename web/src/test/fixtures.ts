import { HttpResponse, http } from 'msw'

import type {
  JavaOverviewResponse,
  JavaServicePageResponse,
  RabbitMQNodePageResponse,
  RabbitMQNodeStatusSource,
  RabbitMQOverviewResponse,
} from '../api/types'

export const SESSION_PATH = '/api/v1/session'
export const OVERVIEW_PATH = '*/api/v1/overview'
export const MYSQL_OVERVIEW_PATH = '*/api/v1/mysql/overview'
export const MYSQL_INSTANCES_PATH = '*/api/v1/mysql/instances'
export const REDIS_OVERVIEW_PATH = '*/api/v1/redis/overview'
export const REDIS_INSTANCES_PATH = '*/api/v1/redis/instances'
export const ELASTICSEARCH_OVERVIEW_PATH =
  '*/api/v1/elasticsearch/overview'
export const ELASTICSEARCH_NODES_PATH = '*/api/v1/elasticsearch/nodes'
export const RABBITMQ_OVERVIEW_PATH = '*/api/v1/rabbitmq/overview'
export const RABBITMQ_NODES_PATH = '*/api/v1/rabbitmq/nodes'
export const JAVA_OVERVIEW_PATH = '*/api/v1/java/overview'
export const JAVA_SERVICES_PATH = '*/api/v1/java/services'
export const DISK_OVERVIEW_PATH = '*/api/v1/disks/overview'
export const DISK_DEVICES_PATH = '*/api/v1/disks/devices'

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
      collection_level: MetricLevelFixture
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
      tps: number | null
      slow_queries_per_second: number | null
      buffer_pool_size_bytes: number | null
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
      collection_level: MetricLevelFixture
    }>
    available_labels: string[]
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

export interface MySQLOverviewFixture {
  data: {
    total: number
    normal: number
    warning: number
    critical: number
    unknown: number
    affected_instances: number
    warning_instances: number
    critical_instances: number
    alerts: {
      availability: { warning: number; critical: number }
      replication_threads: { warning: number; critical: number }
      replication_lag: { warning: number; critical: number }
      replication_data: { warning: number; critical: number }
    }
  }
  meta: {
    request_id: string
    stale: boolean
    collected_at: string
  }
}

export interface RedisOverviewFixture {
  data: {
    total: number
    normal: number
    warning: number
    critical: number
    unknown: number
    affected_instances: number
    warning_instances: number
    critical_instances: number
    roles: { master: number; slave: number; unknown: number }
    alerts: {
      availability: { warning: number; critical: number }
      memory: { warning: number; critical: number }
      connection: { warning: number; critical: number }
      replication: { warning: number; critical: number }
    }
  }
  meta: { request_id: string; stale: boolean; collected_at: string }
}

export interface RedisInstancePageFixture {
  data: {
    instances: Array<{
      id: string
      address: string
      availability: 'up' | 'down' | 'unknown'
      role: 'master' | 'slave' | 'unknown'
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
      replication: {
        connected_replicas: number | null
        master_link_up: boolean | null
        master_last_io_seconds_ago: number | null
        master_sync_in_progress: boolean | null
        worst_replica_lag_seconds: number | null
      }
      uptime_seconds: number | null
      status: MetricLevelFixture
      status_source:
        | 'availability'
        | 'replication'
        | 'memory'
        | 'connection'
        | 'collection'
        | 'normal'
        | 'unknown'
      collection_level: MetricLevelFixture
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

export type ElasticsearchRoleFixture =
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

export interface ElasticsearchOverviewFixture {
  data: {
    status: MetricLevelFixture
    clusters: {
      total: number
      normal: number
      warning: number
      critical: number
      unknown: number
    }
    nodes: {
      total: number
      normal: number
      warning: number
      critical: number
      unknown: number
    }
    alerts: {
      cluster_health: { warning: number; critical: number }
      node_resource: { warning: number; critical: number }
      unassigned_shards: { warning: number; critical: number }
      request_rejections: { warning: number; critical: number }
    }
  }
  meta: {
    request_id: string
    stale: boolean
    collected_at: string
  }
}

export interface ElasticsearchNodePageFixture {
  data: {
    nodes: Array<{
      id: string
      name: string
      cluster: string
      address: string
      roles: ElasticsearchRoleFixture[]
      cluster_health: 'green' | 'yellow' | 'red' | 'unknown'
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
      status: MetricLevelFixture
      status_source:
        | 'collection'
        | 'disk'
        | 'jvm'
        | 'thread_pool'
        | 'normal'
        | 'unknown'
      collection_level: MetricLevelFixture
    }>
    available_clusters: string[]
    available_roles: ElasticsearchRoleFixture[]
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

export type RabbitMQNodeStatusSourceFixture = RabbitMQNodeStatusSource
export type RabbitMQOverviewFixture = RabbitMQOverviewResponse
export type RabbitMQNodePageFixture = RabbitMQNodePageResponse

export interface DiskDevicePageFixture {
  data: {
    devices: Array<{
      id: string
      host: string
      device: string
      model: string
      capacity_bytes: number | null
      smart_health: 'healthy' | 'failed' | 'unknown'
      temperature_celsius: number | null
      lifetime_used_percent: number | null
      power_on_hours: number | null
      errors: {
        pending_sectors: number | null
        reallocated_sectors: number | null
        uncorrectable_sectors: number | null
        udma_crc_errors: number | null
        media_integrity_errors: number | null
        error_log_entries: number | null
        unsafe_shutdowns: number | null
      }
      status: MetricLevelFixture
      status_source:
        | 'smart_health'
        | 'device_warning'
        | 'attribute_failure'
        | 'collection'
        | 'unknown'
        | 'normal'
      collection_level: MetricLevelFixture
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

export interface DiskOverviewFixture {
  data: {
    total: number
    normal: number
    warning: number
    critical: number
    unknown: number
    affected_devices: number
    warning_devices: number
    critical_devices: number
    alerts: {
      smart_health: { warning: number; critical: number }
      device_warning: { warning: number; critical: number }
      attribute_failure: { warning: number; critical: number }
      collection: { warning: number; critical: number }
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
          collection_level: 'normal',
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
          collection_level: 'critical',
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
          tps: 45.25,
          slow_queries_per_second: 0.125,
          buffer_pool_size_bytes: 8 * 1024 ** 3,
          buffer_pool_usage_percent: 82.34,
          uptime_seconds: 183_600,
          replication: {
            state: 'normal',
            lag_seconds: 2,
            level: 'normal',
          },
          status: 'normal',
          collection_level: 'normal',
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
          tps: 20,
          slow_queries_per_second: 1.2,
          buffer_pool_size_bytes: 16 * 1024 ** 3,
          buffer_pool_usage_percent: 91.25,
          uptime_seconds: 43_200,
          replication: {
            state: 'threads_stopped',
            lag_seconds: null,
            level: 'critical',
          },
          status: 'critical',
          collection_level: 'normal',
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
          tps: 80.5,
          slow_queries_per_second: 4.75,
          buffer_pool_size_bytes: 4 * 1024 ** 3,
          buffer_pool_usage_percent: 98.6,
          uptime_seconds: 900,
          replication: {
            state: 'not_configured',
            lag_seconds: null,
            level: 'normal',
          },
          status: 'normal',
          collection_level: 'normal',
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
          tps: null,
          slow_queries_per_second: null,
          buffer_pool_size_bytes: null,
          buffer_pool_usage_percent: null,
          uptime_seconds: null,
          replication: {
            state: 'unknown',
            lag_seconds: null,
            level: 'unknown',
          },
          status: 'critical',
          collection_level: 'critical',
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
          tps: 12,
          slow_queries_per_second: 0.25,
          buffer_pool_size_bytes: 2 * 1024 ** 3,
          buffer_pool_usage_percent: 73.5,
          uptime_seconds: 266_400,
          replication: {
            state: 'normal',
            lag_seconds: 12,
            level: 'warning',
          },
          status: 'warning',
          collection_level: 'warning',
        },
      ],
      available_labels: ['tier-fixture', 'team-fixture'],
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

export function mysqlOverviewFixture(
  overrides: {
    data?: Partial<MySQLOverviewFixture['data']>
    meta?: Partial<MySQLOverviewFixture['meta']>
  } = {},
): MySQLOverviewFixture {
  return {
    data: {
      total: 5,
      normal: 2,
      warning: 1,
      critical: 1,
      unknown: 1,
      affected_instances: 3,
      warning_instances: 2,
      critical_instances: 1,
      alerts: {
        availability: { warning: 1, critical: 0 },
        replication_threads: { warning: 0, critical: 1 },
        replication_lag: { warning: 1, critical: 0 },
        replication_data: { warning: 1, critical: 0 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-mysql-overview-001',
      stale: false,
      collected_at: '2026-07-28T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function redisOverviewFixture(
  overrides: {
    data?: Partial<RedisOverviewFixture['data']>
    meta?: Partial<RedisOverviewFixture['meta']>
  } = {},
): RedisOverviewFixture {
  return {
    data: {
      total: 8,
      normal: 3,
      warning: 2,
      critical: 2,
      unknown: 1,
      affected_instances: 5,
      warning_instances: 3,
      critical_instances: 2,
      roles: { master: 4, slave: 3, unknown: 1 },
      alerts: {
        availability: { warning: 1, critical: 1 },
        memory: { warning: 1, critical: 0 },
        connection: { warning: 1, critical: 0 },
        replication: { warning: 1, critical: 1 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-redis-overview-001',
      stale: false,
      collected_at: '2026-08-01T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function elasticsearchOverviewFixture(
  overrides: {
    data?: Partial<ElasticsearchOverviewFixture['data']>
    meta?: Partial<ElasticsearchOverviewFixture['meta']>
  } = {},
): ElasticsearchOverviewFixture {
  return {
    data: {
      status: 'critical',
      clusters: {
        total: 4,
        normal: 2,
        warning: 1,
        critical: 0,
        unknown: 1,
      },
      nodes: {
        total: 9,
        normal: 5,
        warning: 2,
        critical: 1,
        unknown: 1,
      },
      alerts: {
        cluster_health: { warning: 1, critical: 0 },
        node_resource: { warning: 2, critical: 1 },
        unassigned_shards: { warning: 3, critical: 0 },
        request_rejections: { warning: 1, critical: 1 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-elasticsearch-overview-001',
      stale: false,
      collected_at: '2026-08-01T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function elasticsearchNodePageFixture(
  overrides: {
    data?: Partial<ElasticsearchNodePageFixture['data']>
    meta?: Partial<ElasticsearchNodePageFixture['meta']>
  } = {},
): ElasticsearchNodePageFixture {
  return {
    data: {
      nodes: [
        {
          id: 'elasticsearch-fixture-node-001',
          name: 'fixture-es-node-a',
          cluster: 'fixture-es-cluster-a',
          address: '192.0.2.31:9200',
          roles: ['master', 'data_hot', 'ingest'],
          cluster_health: 'yellow',
          heap_usage_percent: 72.5,
          disk_usage_percent: 81,
          cpu_usage_percent: 36.5,
          index_rate: 14.25,
          search_rate: 28.5,
          documents: 1200,
          store_size_bytes: 2 * 1024 ** 3,
          thread_pool_queue: 3,
          rejected_rate: 0.02,
          uptime_seconds: 172_800,
          status: 'warning',
          status_source: 'disk',
          collection_level: 'normal',
        },
      ],
      available_clusters: ['fixture-es-cluster-a'],
      available_roles: ['master', 'data_hot', 'ingest'],
      total: 1,
      page: 1,
      page_size: 20,
      total_pages: 1,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-elasticsearch-nodes-001',
      stale: false,
      collected_at: '2026-08-01T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function rabbitMQOverviewFixture(
  overrides: {
    data?: Partial<RabbitMQOverviewFixture['data']>
    meta?: Partial<RabbitMQOverviewFixture['meta']>
  } = {},
): RabbitMQOverviewFixture {
  return {
    data: {
      status: 'critical',
      clusters: {
        total: 3,
        normal: 1,
        warning: 1,
        critical: 0,
        unknown: 1,
      },
      nodes: {
        total: 4,
        normal: 1,
        warning: 1,
        critical: 1,
        unknown: 1,
      },
      alerts: {
        cluster_connectivity: { warning: 1, critical: 0, unknown: 1 },
        resource_alarms: { warning: 0, critical: 1, unknown: 0 },
        resource_pressure: { warning: 1, critical: 1, unknown: 0 },
        collection: { warning: 1, critical: 0, unknown: 1 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-rabbitmq-overview-001',
      stale: false,
      collected_at: '2026-08-04T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function rabbitMQNodePageFixture(
  overrides: {
    data?: Partial<RabbitMQNodePageFixture['data']>
    meta?: Partial<RabbitMQNodePageFixture['meta']>
  } = {},
): RabbitMQNodePageFixture {
  return {
    data: {
      nodes: [
        {
          id: 'rabbitmq-fixture-node-001',
          name: 'fixture-rabbit-node-normal',
          cluster: 'fixture-rabbit-cluster-a',
          address: '192.0.2.41:15692',
          version: 'fixture-rabbit-4.0',
          memory_usage_percent: 48.5,
          disk_available_bytes: 12 * 1024 ** 3,
          file_descriptor_usage_percent: 24,
          erlang_process_usage_percent: 31.5,
          connections: 16,
          queues: 8,
          messages: 42,
          publish_rate: 12.5,
          deliver_rate: 11.75,
          uptime_seconds: 86_400,
          status: 'normal',
          status_source: 'normal',
          collection_level: 'normal',
        },
        {
          id: 'rabbitmq-fixture-node-002',
          name: 'fixture-rabbit-node-warning',
          cluster: 'fixture-rabbit-cluster-a',
          address: '192.0.2.42:15692',
          version: 'fixture-rabbit-4.0',
          memory_usage_percent: 84,
          disk_available_bytes: 9 * 1024 ** 3,
          file_descriptor_usage_percent: 44,
          erlang_process_usage_percent: 39,
          connections: 23,
          queues: 11,
          messages: 64,
          publish_rate: 9.25,
          deliver_rate: 9,
          uptime_seconds: 172_800,
          status: 'warning',
          status_source: 'memory',
          collection_level: 'normal',
        },
        {
          id: 'rabbitmq-fixture-node-003',
          name: 'fixture-rabbit-node-critical',
          cluster: 'fixture-rabbit-cluster-b',
          address: '192.0.2.43:15692',
          version: 'fixture-rabbit-4.0',
          memory_usage_percent: 93,
          disk_available_bytes: 2 * 1024 ** 3,
          file_descriptor_usage_percent: 91,
          erlang_process_usage_percent: 88,
          connections: 31,
          queues: 17,
          messages: 128,
          publish_rate: 7.5,
          deliver_rate: 6.75,
          uptime_seconds: 259_200,
          status: 'critical',
          status_source: 'alarm',
          collection_level: 'normal',
        },
        {
          id: 'rabbitmq-fixture-node-004',
          name: 'fixture-rabbit-node-unknown',
          cluster: 'fixture-rabbit-cluster-c',
          address: '192.0.2.44:15692',
          version: 'fixture-rabbit-4.0',
          memory_usage_percent: null,
          disk_available_bytes: null,
          file_descriptor_usage_percent: null,
          erlang_process_usage_percent: null,
          connections: null,
          queues: null,
          messages: null,
          publish_rate: null,
          deliver_rate: null,
          uptime_seconds: null,
          status: 'unknown',
          status_source: 'unknown',
          collection_level: 'unknown',
        },
      ],
      available_clusters: [
        'fixture-rabbit-cluster-a',
        'fixture-rabbit-cluster-b',
        'fixture-rabbit-cluster-c',
      ],
      total: 4,
      page: 1,
      page_size: 20,
      total_pages: 1,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-rabbitmq-nodes-001',
      stale: false,
      collected_at: '2026-08-04T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function rabbitMQOverviewEmptyFixture(): RabbitMQOverviewFixture {
  return rabbitMQOverviewFixture({
    data: {
      status: 'normal',
      clusters: { total: 0, normal: 0, warning: 0, critical: 0, unknown: 0 },
      nodes: { total: 0, normal: 0, warning: 0, critical: 0, unknown: 0 },
      alerts: {
        cluster_connectivity: { warning: 0, critical: 0, unknown: 0 },
        resource_alarms: { warning: 0, critical: 0, unknown: 0 },
        resource_pressure: { warning: 0, critical: 0, unknown: 0 },
        collection: { warning: 0, critical: 0, unknown: 0 },
      },
    },
  })
}

export function rabbitMQNodePageEmptyFixture(): RabbitMQNodePageFixture {
  return rabbitMQNodePageFixture({
    data: {
      nodes: [],
      available_clusters: [],
      total: 0,
      page: 1,
      page_size: 20,
      total_pages: 0,
    },
  })
}

export function rabbitMQOverviewMalformedFixture(): unknown {
  return {
    ...rabbitMQOverviewFixture(),
    data: { status: 'invalid' },
  }
}

export function rabbitMQNodePageMalformedFixture(): unknown {
  return {
    ...rabbitMQNodePageFixture(),
    data: { nodes: [{ id: 42 }] },
  }
}

export type JavaOverviewFixture = JavaOverviewResponse
export type JavaServicePageFixture = JavaServicePageResponse

export function javaOverviewFixture(
  overrides: {
    data?: Partial<JavaOverviewFixture['data']>
    meta?: Partial<JavaOverviewFixture['meta']>
  } = {},
): JavaOverviewFixture {
  return {
    data: {
      status: 'critical',
      services: { total: 4, normal: 1, warning: 1, critical: 1, unknown: 1 },
      alerts: {
        health: { warning: 0, critical: 1, unknown: 1 },
        port: { warning: 0, critical: 1, unknown: 1 },
        process: { warning: 0, critical: 1, unknown: 1 },
        collection: { warning: 1, critical: 0, unknown: 1 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-java-overview-001',
      stale: false,
      collected_at: '2026-08-05T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function javaServicePageFixture(
  overrides: {
    data?: Partial<JavaServicePageFixture['data']>
    meta?: Partial<JavaServicePageFixture['meta']>
  } = {},
): JavaServicePageFixture {
  return {
    data: {
      services: [
        {
          id: 'java-fixture-service-001',
          name: 'fixture-service-a',
          business: 'fixture-business-a',
          address: 'fixture-address-a',
          health_up: true,
          health_latency_ms: 12.5,
          port_up: true,
          process_up: true,
          process_count: '1',
          port_consistent: true,
          cpu_usage_percent: 22.5,
          memory_bytes: '536870912',
          memory_usage_percent: 36,
          uptime_seconds: '86400',
          status: 'normal',
          status_source: 'normal',
          collection_level: 'normal',
        },
        {
          id: 'java-fixture-service-002',
          name: 'fixture-service-b',
          business: 'fixture-business-b',
          address: 'fixture-address-b',
          health_up: true,
          health_latency_ms: 24,
          port_up: true,
          process_up: true,
          process_count: '2',
          port_consistent: true,
          cpu_usage_percent: 48,
          memory_bytes: '1073741824',
          memory_usage_percent: 52.5,
          uptime_seconds: '172800',
          status: 'warning',
          status_source: 'collection',
          collection_level: 'warning',
        },
        {
          id: 'java-fixture-service-003',
          name: 'fixture-service-c',
          business: 'fixture-business-c',
          address: 'fixture-address-c',
          health_up: false,
          health_latency_ms: 0,
          port_up: false,
          process_up: false,
          process_count: '0',
          port_consistent: false,
          cpu_usage_percent: 91,
          memory_bytes: '2147483648',
          memory_usage_percent: 94,
          uptime_seconds: '259200',
          status: 'critical',
          status_source: 'health',
          collection_level: 'normal',
        },
        {
          id: 'java-fixture-service-004',
          name: 'fixture-service-d',
          business: 'fixture-business-d',
          address: 'fixture-address-d',
          health_up: null,
          health_latency_ms: null,
          port_up: null,
          process_up: null,
          process_count: null,
          port_consistent: null,
          cpu_usage_percent: null,
          memory_bytes: null,
          memory_usage_percent: null,
          uptime_seconds: null,
          status: 'unknown',
          status_source: 'unknown',
          collection_level: 'unknown',
        },
      ],
      available_names: [
        'fixture-service-a',
        'fixture-service-b',
        'fixture-service-c',
        'fixture-service-d',
      ],
      total: 4,
      page: 1,
      page_size: 20,
      total_pages: 1,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-java-services-001',
      stale: false,
      collected_at: '2026-08-05T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function javaOverviewEmptyFixture(): JavaOverviewFixture {
  return javaOverviewFixture({
    data: {
      status: 'normal',
      services: { total: 0, normal: 0, warning: 0, critical: 0, unknown: 0 },
      alerts: {
        health: { warning: 0, critical: 0, unknown: 0 },
        port: { warning: 0, critical: 0, unknown: 0 },
        process: { warning: 0, critical: 0, unknown: 0 },
        collection: { warning: 0, critical: 0, unknown: 0 },
      },
    },
  })
}

export function javaServicePageEmptyFixture(): JavaServicePageFixture {
  return javaServicePageFixture({
    data: {
      services: [],
      available_names: [],
      total: 0,
      page: 1,
      page_size: 20,
      total_pages: 0,
    },
  })
}

export function javaErrorFixture(): ErrorFixture {
  return {
    code: 'java_unavailable',
    message: '数据源暂时不可用，请稍后重试',
    request_id: 'req-fixture-java-error-001',
    retryable: true,
  }
}

export function javaOverviewMalformedFixture(): unknown {
  return {
    ...javaOverviewFixture(),
    data: { status: 'invalid' },
  }
}

export function javaServicePageMalformedFixture(): unknown {
  return {
    ...javaServicePageFixture(),
    data: { services: [{ id: 42 }] },
  }
}

export function redisInstancePageFixture(
  overrides: {
    data?: Partial<RedisInstancePageFixture['data']>
    meta?: Partial<RedisInstancePageFixture['meta']>
  } = {},
): RedisInstancePageFixture {
  return {
    data: {
      instances: [
        {
          id: 'redis-fixture-001',
          address: '192.0.2.201:6379',
          availability: 'up',
          role: 'master',
          cluster_enabled: true,
          used_memory_bytes: 1024 ** 3,
          max_memory_bytes: 2 * 1024 ** 3,
          memory_usage_percent: 50,
          connected_clients: 24,
          max_clients: 1000,
          connection_usage_percent: 2.4,
          blocked_clients: 0,
          qps: 80.5,
          hit_rate: 0.97,
          keys: 4096,
          expired_keys_per_second: 0.5,
          evicted_keys_per_second: 0,
          rejected_connections_rate: 0,
          replication: {
            connected_replicas: 2,
            master_link_up: null,
            master_last_io_seconds_ago: null,
            master_sync_in_progress: null,
            worst_replica_lag_seconds: 2,
          },
          uptime_seconds: 183_600,
          status: 'normal',
          status_source: 'normal',
          collection_level: 'normal',
        },
        {
          id: 'redis-fixture-002',
          address: '192.0.2.202:6379',
          availability: 'up',
          role: 'slave',
          cluster_enabled: true,
          used_memory_bytes: null,
          max_memory_bytes: null,
          memory_usage_percent: null,
          connected_clients: null,
          max_clients: null,
          connection_usage_percent: null,
          blocked_clients: null,
          qps: null,
          hit_rate: null,
          keys: null,
          expired_keys_per_second: null,
          evicted_keys_per_second: null,
          rejected_connections_rate: null,
          replication: {
            connected_replicas: null,
            master_link_up: false,
            master_last_io_seconds_ago: 35,
            master_sync_in_progress: false,
            worst_replica_lag_seconds: null,
          },
          uptime_seconds: null,
          status: 'critical',
          status_source: 'replication',
          collection_level: 'normal',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      total_pages: 1,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-redis-instances-001',
      stale: false,
      collected_at: '2026-08-01T08:00:00.000Z',
      ...overrides.meta,
    },
  }
}

export function diskDevicePageFixture(
  overrides: {
    data?: Partial<DiskDevicePageFixture['data']>
    meta?: Partial<DiskDevicePageFixture['meta']>
  } = {},
): DiskDevicePageFixture {
  return {
    data: {
      devices: [
        {
          id: 'disk-fixture-001',
          host: 'node-alpha',
          device: '/dev/nvme0n1',
          model: 'Atlas NVMe 2TB',
          capacity_bytes: 2 * 1024 ** 4,
          smart_health: 'healthy',
          temperature_celsius: 42.5,
          lifetime_used_percent: 17.4,
          power_on_hours: 50,
          errors: {
            pending_sectors: 2,
            reallocated_sectors: 1,
            uncorrectable_sectors: 0,
            udma_crc_errors: 7,
            media_integrity_errors: 0,
            error_log_entries: 0,
            unsafe_shutdowns: 0,
          },
          status: 'critical',
          status_source: 'device_warning',
          collection_level: 'critical',
        },
        {
          id: 'disk-fixture-002',
          host: 'node-beta',
          device: '/dev/sda',
          model: 'Boreal SATA 960GB',
          capacity_bytes: 960 * 1024 ** 3,
          smart_health: 'failed',
          temperature_celsius: 51,
          lifetime_used_percent: 86,
          power_on_hours: 24_600,
          errors: {
            pending_sectors: 0,
            reallocated_sectors: 0,
            uncorrectable_sectors: 0,
            udma_crc_errors: 0,
            media_integrity_errors: 0,
            error_log_entries: 0,
            unsafe_shutdowns: 0,
          },
          status: 'critical',
          status_source: 'smart_health',
          collection_level: 'critical',
        },
        {
          id: 'disk-fixture-003',
          host: 'node-gamma',
          device: '/dev/vda',
          model: 'Cirrus Virtual Disk',
          capacity_bytes: null,
          smart_health: 'unknown',
          temperature_celsius: null,
          lifetime_used_percent: null,
          power_on_hours: null,
          errors: {
            pending_sectors: null,
            reallocated_sectors: null,
            uncorrectable_sectors: null,
            udma_crc_errors: null,
            media_integrity_errors: null,
            error_log_entries: null,
            unsafe_shutdowns: null,
          },
          status: 'unknown',
          status_source: 'smart_health',
          collection_level: 'normal',
        },
        {
          id: 'disk-fixture-004',
          host: 'node-delta',
          device: '/dev/nvme1n1',
          model: 'Dawn NVMe 4TB',
          capacity_bytes: 4 * 1024 ** 4,
          smart_health: 'healthy',
          temperature_celsius: 39,
          lifetime_used_percent: 4,
          power_on_hours: 730,
          errors: {
            pending_sectors: 0,
            reallocated_sectors: null,
            uncorrectable_sectors: 0,
            udma_crc_errors: null,
            media_integrity_errors: 0,
            error_log_entries: null,
            unsafe_shutdowns: 0,
          },
          status: 'warning',
          status_source: 'collection',
          collection_level: 'warning',
        },
        {
          id: 'disk-fixture-005',
          host: 'node-epsilon',
          device: '/dev/sdb',
          model: 'Ember Archive 8TB',
          capacity_bytes: 8 * 1024 ** 4,
          smart_health: 'healthy',
          temperature_celsius: 36,
          lifetime_used_percent: 11,
          power_on_hours: 12_345,
          errors: {
            pending_sectors: 0,
            reallocated_sectors: 0,
            uncorrectable_sectors: 0,
            udma_crc_errors: 0,
            media_integrity_errors: 0,
            error_log_entries: 0,
            unsafe_shutdowns: 0,
          },
          status: 'critical',
          status_source: 'collection',
          collection_level: 'critical',
        },
        {
          id: 'disk-fixture-006',
          host: 'node-zeta',
          device: '/dev/sdc',
          model: 'Fable SATA 1TB',
          capacity_bytes: 1024 ** 4,
          smart_health: 'healthy',
          temperature_celsius: 34,
          lifetime_used_percent: 22,
          power_on_hours: 900,
          errors: {
            pending_sectors: 0,
            reallocated_sectors: 0,
            uncorrectable_sectors: 0,
            udma_crc_errors: 0,
            media_integrity_errors: 0,
            error_log_entries: 0,
            unsafe_shutdowns: 0,
          },
          status: 'warning',
          status_source: 'attribute_failure',
          collection_level: 'warning',
        },
      ],
      total: 45,
      page: 1,
      page_size: 20,
      total_pages: 3,
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-disk-devices-001',
      stale: false,
      collected_at: '2026-07-30T08:30:00.000Z',
      ...overrides.meta,
    },
  }
}

export function diskOverviewFixture(
  overrides: {
    data?: Partial<DiskOverviewFixture['data']>
    meta?: Partial<DiskOverviewFixture['meta']>
  } = {},
): DiskOverviewFixture {
  return {
    data: {
      total: 6,
      normal: 1,
      warning: 2,
      critical: 2,
      unknown: 1,
      affected_devices: 5,
      warning_devices: 3,
      critical_devices: 2,
      alerts: {
        smart_health: { warning: 0, critical: 1 },
        device_warning: { warning: 0, critical: 1 },
        attribute_failure: { warning: 1, critical: 0 },
        collection: { warning: 1, critical: 0 },
      },
      ...overrides.data,
    },
    meta: {
      request_id: 'req-fixture-disk-overview-001',
      stale: false,
      collected_at: '2026-07-30T08:30:00.000Z',
      ...overrides.meta,
    },
  }
}

export function resetFixtureState() {
  authenticatedUsername = null
}

export const mysqlHandlers = [
  http.get(MYSQL_OVERVIEW_PATH, () =>
    HttpResponse.json(mysqlOverviewFixture()),
  ),
  http.get(MYSQL_INSTANCES_PATH, () =>
    HttpResponse.json(mysqlInstancePageFixture()),
  ),
]

export const elasticsearchHandlers = [
  http.get(ELASTICSEARCH_OVERVIEW_PATH, () =>
    HttpResponse.json(elasticsearchOverviewFixture()),
  ),
  http.get(ELASTICSEARCH_NODES_PATH, () =>
    HttpResponse.json(elasticsearchNodePageFixture()),
  ),
]

export const rabbitMQHandlers = [
  http.get(RABBITMQ_OVERVIEW_PATH, () =>
    HttpResponse.json(rabbitMQOverviewFixture()),
  ),
  http.get(RABBITMQ_NODES_PATH, () =>
    HttpResponse.json(rabbitMQNodePageFixture()),
  ),
]

export const javaHandlers = [
  http.get(JAVA_OVERVIEW_PATH, () => HttpResponse.json(javaOverviewFixture())),
  http.get(JAVA_SERVICES_PATH, () =>
    HttpResponse.json(javaServicePageFixture()),
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
  http.get(DISK_OVERVIEW_PATH, () =>
    HttpResponse.json(diskOverviewFixture()),
  ),
  http.get(REDIS_OVERVIEW_PATH, () => HttpResponse.json(redisOverviewFixture())),
  http.get(REDIS_INSTANCES_PATH, () =>
    HttpResponse.json(redisInstancePageFixture()),
  ),
  http.get(OVERVIEW_PATH, () => HttpResponse.json(overviewFixture())),
  http.get(DISK_DEVICES_PATH, () =>
    HttpResponse.json(diskDevicePageFixture()),
  ),
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
