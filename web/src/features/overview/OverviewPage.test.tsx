import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import {
  afterEach,
  beforeEach,
  expect,
  it,
  vi,
} from 'vitest'

import {
  diskOverviewFixture,
  elasticsearchOverviewFixture,
  javaOverviewEmptyFixture,
  javaOverviewFixture,
  mysqlOverviewFixture,
  overviewFixture,
  rabbitMQOverviewFixture,
  redisOverviewFixture,
  type DiskOverviewFixture,
  type ElasticsearchOverviewFixture,
  type ErrorFixture,
  type MySQLOverviewFixture,
  type OverviewFixture,
  type RabbitMQOverviewFixture,
  type RedisOverviewFixture,
} from '../../test/fixtures'
import '../../app/theme.css'
// @ts-expect-error Vitest 通过 Vite raw loader 提供 CSS 源文本。
import themeSource from '../../app/theme.css?raw'
import { OverviewPage } from './OverviewPage'

function cssRule(selector: string) {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return themeSource.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`))?.[1]
}

function renderOverview() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })

  render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <OverviewPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
  return queryClient
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestedRange(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost').searchParams.get('range') ?? ''
}

function requestedPath(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost').pathname
}

function mockOverviewRequests({
  host = overviewFixture(),
  disk = diskOverviewFixture(),
  mysql = mysqlOverviewFixture(),
  redis = redisOverviewFixture(),
  elasticsearch = elasticsearchOverviewFixture(),
  rabbitmq = rabbitMQOverviewFixture(),
  java = javaOverviewFixture(),
  hostError,
  diskError,
  mysqlError,
  redisError,
  elasticsearchError,
  rabbitmqError,
  javaError,
}: {
  host?: OverviewFixture
  disk?: DiskOverviewFixture
  mysql?: MySQLOverviewFixture
  redis?: RedisOverviewFixture
  elasticsearch?: unknown
  rabbitmq?: unknown
  java?: unknown
  hostError?: ErrorFixture
  diskError?: ErrorFixture
  mysqlError?: ErrorFixture
  redisError?: ErrorFixture
  elasticsearchError?: ErrorFixture
  rabbitmqError?: ErrorFixture
  javaError?: ErrorFixture
} = {}) {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(
        jsonResponse(mysqlError ?? mysql, mysqlError === undefined ? 200 : 503),
      )
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(
        jsonResponse(diskError ?? disk, diskError === undefined ? 200 : 503),
      )
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(
        jsonResponse(
          redisError ?? redis,
          redisError === undefined ? 200 : 503,
        ),
      )
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(
        jsonResponse(
          elasticsearchError ?? elasticsearch,
          elasticsearchError === undefined ? 200 : 503,
        ),
      )
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(
        jsonResponse(
          rabbitmqError ?? rabbitmq,
          rabbitmqError === undefined ? 200 : 503,
        ),
      )
    }
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(
        jsonResponse(javaError ?? java, javaError === undefined ? 200 : 503),
      )
    }
    return Promise.resolve(
      jsonResponse(hostError ?? host, hostError === undefined ? 200 : 503),
    )
  })
}

beforeEach(() => {
  vi.spyOn(globalThis, 'fetch')
  mockOverviewRequests()
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('共享总览样式为 unknown 卡片、等级和指标使用中性灰蓝色', () => {
  expect(cssRule('.module-status-card[data-level="unknown"]')).toContain(
    'border-color: #485966',
  )
  expect(cssRule('.module-status-card[data-level="unknown"]')).toContain(
    'rgb(36 45 53 / 88%)',
  )
  expect(cssRule('.module-status-level[data-level="unknown"]')).toContain(
    'color: #a7b0b6',
  )
  expect(cssRule('.module-status-level[data-level="unknown"]')).toContain(
    'background: #252c31',
  )
  expect(cssRule('.module-metric-alert[data-level="unknown"]')).toContain(
    'border-color: #46535c',
  )
  expect(cssRule('.module-metric-alert[data-level="unknown"]')).toContain(
    'rgb(37 45 51 / 76%)',
  )
  expect(
    cssRule('.module-metric-alert[data-level="unknown"] strong'),
  ).toContain('color: #a7b0b6')
})

it('总览按顺序展示七张模块卡并保留四列桌面网格', async () => {
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  const mysqlCard = screen.getByRole('link', {
    name: '查看 MySQL 板块',
  })
  const diskCard = screen.getByRole('link', {
    name: '查看主机硬盘板块',
  })
  const redisCard = screen.getByRole('link', { name: '查看 Redis 板块' })
  const elasticsearchCard = screen.getByRole('link', {
    name: '查看 Elasticsearch 板块',
  })
  const rabbitMQCard = screen.getByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  const javaCard = screen.getByRole('link', { name: '查看 Java 服务板块' })
  const moduleGrid = screen.getByRole('group', { name: '基础设施模块' })
  expect(moduleGrid).toHaveClass(
    'overview-status-grid',
    'overview-compact-grid',
  )
  expect(
    within(moduleGrid).getByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    within(moduleGrid).getByRole('link', { name: '查看主机硬盘板块' }),
  ).toBeVisible()
  expect(
    within(moduleGrid).getByRole('link', { name: '查看 MySQL 板块' }),
  ).toBeVisible()
  expect(redisCard).toHaveAttribute('href', '/redis')
  expect(within(redisCard).getByText('Redis')).toBeVisible()
  for (const label of ['可用性', '内存', '连接', '复制']) {
    expect(within(redisCard).getByText(label)).toBeVisible()
  }
  const moduleCards = within(moduleGrid).getAllByRole('link')
  expect(moduleCards).toHaveLength(7)
  expect(moduleCards).toEqual([
    hostCard,
    diskCard,
    mysqlCard,
    redisCard,
    elasticsearchCard,
    rabbitMQCard,
    javaCard,
  ])
  expect(getComputedStyle(moduleGrid).gridTemplateColumns).toBe(
    'repeat(4, minmax(0, 1fr))',
  )
  expect(elasticsearchCard).toHaveAttribute('href', '/elasticsearch')
  expect(elasticsearchCard).toHaveClass(
    'module-status-card',
    'elasticsearch-overview-card',
  )
  expect(elasticsearchCard).toHaveAttribute('data-level', 'critical')
  expect(within(elasticsearchCard).getByText('Elasticsearch')).toBeVisible()
  for (const label of [
    '集群健康',
    '节点资源',
    '未分配分片',
    '请求拒绝',
  ]) {
    expect(within(elasticsearchCard).getByText(label)).toBeVisible()
  }
  const elasticsearchSummary = within(elasticsearchCard)
    .getByText('异常节点')
    .closest('.module-alert-summary')
  expect(elasticsearchSummary).not.toBeNull()
  expect(within(elasticsearchSummary as HTMLElement).getByText('4')).toBeVisible()
  expect(within(elasticsearchSummary as HTMLElement).getByText('/ 9')).toBeVisible()
  expect(
    within(elasticsearchSummary as HTMLElement).getByText('严重 1'),
  ).toBeVisible()
  expect(
    within(elasticsearchSummary as HTMLElement).getByText('警告/未知 3'),
  ).toBeVisible()
  expect(rabbitMQCard).toHaveAttribute('href', '/rabbitmq')
  expect(rabbitMQCard).toHaveClass(
    'module-status-card',
    'rabbitmq-overview-card',
  )
  expect(rabbitMQCard).toHaveAttribute('data-level', 'critical')
  expect(within(rabbitMQCard).getByText('RabbitMQ')).toBeVisible()
  for (const label of ['集群通信', '资源告警', '资源压力', '采集状态']) {
    expect(within(rabbitMQCard).getByText(label)).toBeVisible()
  }
  const rabbitMQSummary = within(rabbitMQCard)
    .getByText('异常节点')
    .closest('.module-alert-summary')
  expect(rabbitMQSummary).not.toBeNull()
  expect(within(rabbitMQSummary as HTMLElement).getByText('3')).toBeVisible()
  expect(within(rabbitMQSummary as HTMLElement).getByText('/ 4')).toBeVisible()
  expect(
    within(rabbitMQSummary as HTMLElement).getByText('严重 1'),
  ).toBeVisible()
  expect(
    within(rabbitMQSummary as HTMLElement).getByText('警告/未知 2'),
  ).toBeVisible()
  const clusterConnectivity = within(rabbitMQCard)
    .getByText('集群通信')
    .closest('div')?.parentElement
  expect(clusterConnectivity).not.toBeNull()
  expect(
    within(clusterConnectivity as HTMLElement).getByText(
      '严重 0 · 警告 1 · 未知 1',
    ),
  ).toBeVisible()
  expect(screen.queryAllByRole('article')).toHaveLength(0)
  expect(diskCard).toHaveAttribute('href', '/disks')
  expect(diskCard).toHaveAttribute('data-level', 'critical')
  expect(within(diskCard).getByText('主机硬盘')).toBeVisible()
  expect(within(diskCard).getByText('异常设备')).toBeVisible()
  expect(within(diskCard).getByText('5')).toBeVisible()
  expect(within(diskCard).getByText('/ 6')).toBeVisible()
  expect(within(diskCard).getByText('严重 2')).toBeVisible()
  expect(within(diskCard).getByText('警告风险 3')).toBeVisible()
  for (const [label, total, details] of [
    ['SMART 自检', '1', '严重 1 · 警告 0'],
    ['设备警告', '1', '严重 1 · 警告 0'],
    ['属性失败', '1', '严重 0 · 警告 1'],
    ['采集状态', '1', '严重 0 · 警告 1'],
  ]) {
    const metric = within(diskCard).getByText(label).closest('div')?.parentElement
    expect(metric).not.toBeNull()
    expect(within(metric as HTMLElement).getByText(total)).toBeInTheDocument()
    expect(within(metric as HTMLElement).getByText(details)).toBeInTheDocument()
  }
  const diskFooter = diskCard.querySelector('.module-status-footer')
  expect(diskFooter).not.toBeNull()
  expect(within(diskFooter as HTMLElement).getByText(/查看硬盘/)).toBeVisible()
  expect(diskFooter?.querySelector('.module-status-level')).toBeNull()
  expect(within(diskFooter as HTMLElement).queryByText(/全部正常|存在/)).not.toBeInTheDocument()
  expect(hostCard).toHaveAttribute('href', '/hosts')
  expect(hostCard).toHaveAttribute('data-level', 'critical')
  expect(within(hostCard).getByText('Linux 主机')).toBeInTheDocument()
  expect(within(hostCard).getByText('存在严重异常')).toBeInTheDocument()
  expect(within(hostCard).getByText('异常主机')).toBeInTheDocument()
  expect(within(hostCard).getByText('7')).toBeInTheDocument()
  expect(within(hostCard).getByText('/ 12')).toBeInTheDocument()
  expect(within(hostCard).getByText('严重 4')).toBeInTheDocument()
  expect(within(hostCard).getByText('警告 3')).toBeInTheDocument()
  for (const [label, total, details] of [
    ['CPU', '2', '严重 1 · 警告 1'],
    ['内存', '1', '严重 1 · 警告 0'],
    ['IO', '2', '严重 0 · 警告 2'],
    ['网络', '3', '严重 2 · 警告 1'],
  ]) {
    const metric = within(hostCard).getByText(label).closest('div')?.parentElement
    expect(metric).not.toBeNull()
    expect(within(metric as HTMLElement).getByText(total)).toBeInTheDocument()
    expect(within(metric as HTMLElement).getByText(details)).toBeInTheDocument()
  }
  expect(hostCard.querySelector('.module-status-breakdown')).toBeNull()
  expect(screen.queryByText('CPU 平均使用率')).not.toBeInTheDocument()
  expect(screen.queryByText('内存平均使用率')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: '资源使用趋势' }),
  ).not.toBeInTheDocument()
  expect(mysqlCard).toHaveAttribute('href', '/mysql')
  expect(mysqlCard).toHaveAttribute('data-level', 'critical')
  expect(within(mysqlCard).getByText('MySQL')).toBeVisible()
  expect(within(mysqlCard).getByText('可用性')).toBeVisible()
  expect(within(mysqlCard).getByText('复制线程')).toBeVisible()
  expect(within(mysqlCard).getByText('复制延迟')).toBeVisible()
  expect(within(mysqlCard).getByText('复制数据缺失')).toBeVisible()
  expect(within(mysqlCard).queryByText(/连接|QPS/)).not.toBeInTheDocument()
  for (const [card, time] of [
    [hostCard, '2026/07/21 00:30:00'],
    [diskCard, '2026/07/30 08:30:00'],
    [mysqlCard, '2026/07/28 08:00:00'],
    [redisCard, '2026/08/01 08:00:00'],
    [elasticsearchCard, '2026/08/01 08:00:00'],
    [rabbitMQCard, '2026/08/04 08:00:00'],
    [javaCard, '2026/08/05 08:00:00'],
  ] as const) {
    expect(within(card).getByText(time)).toBeVisible()
  }
  expect(screen.queryByRole('group', { name: '总览控制' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '刷新' })).not.toBeInTheDocument()
  expect(screen.queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument()
})

it('RabbitMQ 集群和节点都为空时显示中性空状态且不渲染告警网格', async () => {
  const fixture = rabbitMQOverviewFixture()
  const emptyCounts = {
    total: 0,
    normal: 0,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  fixture.data.status = 'normal'
  fixture.data.clusters = { ...emptyCounts }
  fixture.data.nodes = { ...emptyCounts }
  fixture.data.alerts = {
    cluster_connectivity: { warning: 0, critical: 0, unknown: 0 },
    resource_alarms: { warning: 0, critical: 0, unknown: 0 },
    resource_pressure: { warning: 0, critical: 0, unknown: 0 },
    collection: { warning: 0, critical: 0, unknown: 0 },
  }
  mockOverviewRequests({ rabbitmq: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  expect(card).toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('暂无 RabbitMQ 节点')).toBeVisible()
  expect(card.querySelector('.module-metric-alert-grid')).toBeNull()
  expect(within(card).queryByText('集群通信')).not.toBeInTheDocument()
})

it.each([
  {
    name: '仅集群为零',
    mutate: (fixture: RabbitMQOverviewFixture) => {
      fixture.data.clusters = {
        total: 0,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 0,
      }
      fixture.data.alerts.cluster_connectivity = {
        warning: 0,
        critical: 0,
        unknown: 0,
      }
    },
  },
  {
    name: '仅节点为零',
    mutate: (fixture: RabbitMQOverviewFixture) => {
      fixture.data.status = 'warning'
      fixture.data.nodes = {
        total: 0,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 0,
      }
      fixture.data.alerts.resource_alarms = {
        warning: 0,
        critical: 0,
        unknown: 0,
      }
      fixture.data.alerts.resource_pressure = {
        warning: 0,
        critical: 0,
        unknown: 0,
      }
      fixture.data.alerts.collection = {
        warning: 0,
        critical: 0,
        unknown: 0,
      }
    },
  },
])('RabbitMQ $name时仍展示另一侧汇总', async ({ mutate }) => {
  const fixture = rabbitMQOverviewFixture()
  mutate(fixture)
  mockOverviewRequests({ rabbitmq: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  expect(card).not.toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('异常节点')).toBeVisible()
  expect(within(card).getByText('集群通信')).toBeVisible()
})

it('RabbitMQ 集群通信严重只提升卡片颜色而不混入异常节点数', async () => {
  const fixture = rabbitMQOverviewFixture()
  fixture.data.status = 'critical'
  fixture.data.clusters = {
    total: 1,
    normal: 0,
    warning: 0,
    critical: 1,
    unknown: 0,
  }
  fixture.data.nodes = {
    total: 2,
    normal: 2,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  fixture.data.alerts = {
    cluster_connectivity: { warning: 0, critical: 1, unknown: 0 },
    resource_alarms: { warning: 0, critical: 0, unknown: 0 },
    resource_pressure: { warning: 0, critical: 0, unknown: 0 },
    collection: { warning: 0, critical: 0, unknown: 0 },
  }
  mockOverviewRequests({ rabbitmq: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  expect(card).toHaveAttribute('data-level', 'critical')
  const summary = within(card)
    .getByText('异常节点')
    .closest('.module-alert-summary')
  expect(summary).not.toBeNull()
  expect(within(summary as HTMLElement).getByText('0')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('/ 2')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('无严重')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('无警告/未知')).toBeVisible()
})

it('RabbitMQ 非空集群和节点全 normal 时接受响应', async () => {
  const fixture = rabbitMQOverviewFixture()
  fixture.data.status = 'normal'
  fixture.data.clusters = {
    total: 1,
    normal: 1,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  fixture.data.nodes = {
    total: 2,
    normal: 2,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  fixture.data.alerts = {
    cluster_connectivity: { warning: 0, critical: 0, unknown: 0 },
    resource_alarms: { warning: 0, critical: 0, unknown: 0 },
    resource_pressure: { warning: 0, critical: 0, unknown: 0 },
    collection: { warning: 0, critical: 0, unknown: 0 },
  }
  mockOverviewRequests({ rabbitmq: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  expect(card).toHaveAttribute('data-level', 'normal')
  expect(within(card).getByText('全部正常')).toBeVisible()
  expect(
    screen.queryByRole('alert', { name: 'RabbitMQ 板块加载失败' }),
  ).not.toBeInTheDocument()
})

const invalidRabbitMQOverviewCases = [
  {
    name: 'status 枚举无效',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.status =
        'degraded' as RabbitMQOverviewFixture['data']['status']
      return fixture
    },
  },
  {
    name: 'status normal 但桶最高等级为 critical',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.status = 'normal'
      return fixture
    },
  },
  {
    name: 'status warning 但桶最高等级仅为 unknown',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.status = 'warning'
      fixture.data.clusters = {
        total: 1,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 1,
      }
      fixture.data.nodes = {
        total: 1,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 1,
      }
      fixture.data.alerts = {
        cluster_connectivity: { warning: 0, critical: 0, unknown: 1 },
        resource_alarms: { warning: 0, critical: 0, unknown: 0 },
        resource_pressure: { warning: 0, critical: 0, unknown: 0 },
        collection: { warning: 0, critical: 0, unknown: 1 },
      }
      return fixture
    },
  },
  {
    name: 'clusters 是数组',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.clusters =
        [] as unknown as RabbitMQOverviewFixture['data']['clusters']
      return fixture
    },
  },
  {
    name: 'alerts 是 null',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.alerts =
        null as unknown as RabbitMQOverviewFixture['data']['alerts']
      return fixture
    },
  },
  {
    name: '告警缺少 unknown',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      delete (
        fixture.data.alerts.collection as Partial<
          RabbitMQOverviewFixture['data']['alerts']['collection']
        >
      ).unknown
      return fixture
    },
  },
  {
    name: 'nodes total 不等于状态桶之和',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.data.nodes.total += 1
      return fixture
    },
  },
  {
    name: 'meta stale 不是 boolean',
    body: () => {
      const fixture = rabbitMQOverviewFixture()
      fixture.meta.stale = null as unknown as boolean
      return fixture
    },
  },
] satisfies Array<{ name: string; body: () => unknown }>

it.each(invalidRabbitMQOverviewCases)(
  'RabbitMQ 成功响应拒绝$name并保持旧模块可用',
  async ({ body }) => {
    mockOverviewRequests({ rabbitmq: body() })
    renderOverview()

    expect(
      await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
    ).toBeVisible()
    expect(
      screen.getByRole('alert', { name: 'RabbitMQ 板块加载失败' }),
    ).toHaveTextContent('服务器响应格式无效')
    expect(
      screen.queryByRole('link', { name: '查看 RabbitMQ 板块' }),
    ).not.toBeInTheDocument()
  },
)

it('其他模块已加载时独立显示 RabbitMQ 加载状态', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return new Promise<Response>(() => undefined)
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('status', { name: 'RabbitMQ 板块加载中' }),
  ).toBeVisible()
})

it('RabbitMQ 首次失败不阻塞旧模块且重试只刷新独立查询', async () => {
  const pathCounts = new Map<string, number>()
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    pathCounts.set(path, (pathCounts.get(path) ?? 0) + 1)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(
        pathCounts.get(path) === 1
          ? jsonResponse(
              {
                code: 'rabbitmq_unavailable',
                message: 'RabbitMQ 数据源暂时不可用',
                request_id: 'req-fixture-rabbitmq-overview-error',
                retryable: true,
              },
              503,
            )
          : jsonResponse(rabbitMQOverviewFixture()),
      )
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toBeVisible()
  const error = screen.getByRole('alert', {
    name: 'RabbitMQ 板块加载失败',
  })
  expect(error).toHaveTextContent('RabbitMQ 数据源暂时不可用')
  await user.click(within(error).getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('link', { name: '查看 RabbitMQ 板块' }),
  ).toBeVisible()
  expect(pathCounts.get('/api/v1/rabbitmq/overview')).toBe(2)
  for (const path of [
    '/api/v1/overview',
    '/api/v1/disks/overview',
    '/api/v1/mysql/overview',
    '/api/v1/redis/overview',
    '/api/v1/elasticsearch/overview',
  ]) {
    expect(pathCounts.get(path)).toBe(1)
  }
})

it('RabbitMQ 过期数据保持可见并显示精确采集时间', async () => {
  mockOverviewRequests({
    rabbitmq: rabbitMQOverviewFixture({
      meta: {
        stale: true,
        collected_at: '2026-08-04T07:30:00.000Z',
      },
    }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 RabbitMQ 板块' }),
  ).toBeVisible()
  const banner = screen.getByRole('alert', {
    name: 'RabbitMQ 数据已过期',
  })
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-08-04T07:30:00.000Z',
  )
})

it('总览第七张卡展示 Java 服务异常摘要和四个固定摘要', async () => {
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 Java 服务板块' })
  const moduleGrid = screen.getByRole('group', { name: '基础设施模块' })
  expect(within(moduleGrid).getAllByRole('link')).toHaveLength(7)
  expect(card).toHaveAttribute('href', '/java')
  expect(card).toHaveClass('module-status-card', 'java-overview-card')
  expect(card).toHaveAttribute('data-level', 'critical')
  expect(within(card).getByText('Java 服务')).toBeVisible()
  const summary = within(card)
    .getByText('异常服务')
    .closest('.module-alert-summary')
  expect(summary).not.toBeNull()
  expect(within(summary as HTMLElement).getByText('3')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('/ 4')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('严重 1')).toBeVisible()
  expect(within(summary as HTMLElement).getByText('警告/未知 2')).toBeVisible()
  for (const label of ['健康检查', '端口状态', '进程状态', '采集状态']) {
    expect(within(card).getByText(label)).toBeVisible()
  }
})

it('Java 服务全正常时四个摘要框显示无异常', async () => {
  mockOverviewRequests({
    java: javaOverviewFixture({
      data: {
        status: 'normal',
        services: { total: 2, normal: 2, warning: 0, critical: 0, unknown: 0 },
        alerts: {
          health: { warning: 0, critical: 0, unknown: 0 },
          port: { warning: 0, critical: 0, unknown: 0 },
          process: { warning: 0, critical: 0, unknown: 0 },
          collection: { warning: 0, critical: 0, unknown: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 Java 服务板块' })
  expect(card).toHaveAttribute('data-level', 'normal')
  expect(within(card).getAllByText('无异常')).toHaveLength(4)
  expect(within(card).getByText('严重 0')).toHaveAttribute(
    'data-level',
    'normal',
  )
  expect(within(card).getByText('警告/未知 0')).toHaveAttribute(
    'data-level',
    'normal',
  )
  expect(within(card).queryByText('无严重')).not.toBeInTheDocument()
  expect(within(card).queryByText('无警告/未知')).not.toBeInTheDocument()
})

it('Java 服务为空时显示中性空状态', async () => {
  mockOverviewRequests({ java: javaOverviewEmptyFixture() })
  renderOverview()

  const emptyCard = await screen.findByRole('link', {
    name: '查看 Java 服务板块',
  })
  expect(emptyCard).toHaveAttribute('data-level', 'empty')
  expect(within(emptyCard).getByText('暂无 Java 服务')).toBeVisible()
})

it('Java 初始加载只显示局部状态且不出现全局刷新控件', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return new Promise<Response>(() => undefined)
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(jsonResponse(rabbitMQOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 RabbitMQ 板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('status', { name: 'Java 服务板块加载中' }),
  ).toBeVisible()
  expect(screen.queryByRole('group', { name: '总览控制' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '刷新' })).not.toBeInTheDocument()
})

it('Java 首次 503 不影响其他卡且重试只刷新独立查询', async () => {
  const pathCounts = new Map<string, number>()
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    pathCounts.set(path, (pathCounts.get(path) ?? 0) + 1)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(
        pathCounts.get(path) === 1
          ? jsonResponse(
              {
                code: 'java_unavailable',
                message: 'Java 数据源暂时不可用',
                request_id: 'req-fixture-java-overview-error',
                retryable: true,
              },
              503,
            )
          : jsonResponse(javaOverviewFixture()),
      )
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(jsonResponse(rabbitMQOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 RabbitMQ 板块' }),
  ).toBeVisible()
  const error = screen.getByRole('alert', { name: 'Java 服务板块加载失败' })
  expect(error).toHaveTextContent('Java 数据源暂时不可用')
  expect(screen.queryByRole('group', { name: '总览控制' })).not.toBeInTheDocument()
  await user.click(within(error).getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('link', { name: '查看 Java 服务板块' }),
  ).toBeVisible()
  expect(pathCounts.get('/api/v1/java/overview')).toBe(2)
  for (const path of [
    '/api/v1/overview',
    '/api/v1/disks/overview',
    '/api/v1/mysql/overview',
    '/api/v1/redis/overview',
    '/api/v1/elasticsearch/overview',
    '/api/v1/rabbitmq/overview',
  ]) {
    expect(pathCounts.get(path)).toBe(1)
  }
})

it('Java 服务过期数据保持可见并显示精确采集时间', async () => {
  mockOverviewRequests({
    java: javaOverviewFixture({
      meta: { stale: true, collected_at: '2026-08-04T07:30:00.000Z' },
    }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Java 服务板块' }),
  ).toBeVisible()
  const banner = screen.getByRole('alert', { name: 'Java 服务数据已过期' })
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-08-04T07:30:00.000Z',
  )
})

it('Java 服务状态与服务计数不一致时独立报错且不影响已有六卡', async () => {
  mockOverviewRequests({
    java: javaOverviewFixture({ data: { status: 'normal' } }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('alert', { name: 'Java 服务板块加载失败' }),
  ).toHaveTextContent('服务器响应格式无效')
  expect(
    screen.queryByRole('link', { name: '查看 Java 服务板块' }),
  ).not.toBeInTheDocument()
})

it('Elasticsearch 集群和节点都为空时显示中性空状态', async () => {
  const fixture = elasticsearchOverviewFixture()
  const emptyCounts = {
    total: 0,
    normal: 0,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  fixture.data.status = 'normal'
  fixture.data.clusters = { ...emptyCounts }
  fixture.data.nodes = { ...emptyCounts }
  fixture.data.alerts = {
    cluster_health: { warning: 0, critical: 0 },
    node_resource: { warning: 0, critical: 0 },
    unassigned_shards: { warning: 0, critical: 0 },
    request_rejections: { warning: 0, critical: 0 },
  }
  mockOverviewRequests({ elasticsearch: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 Elasticsearch 板块',
  })
  expect(card).toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('暂无 Elasticsearch 节点')).toBeVisible()
  expect(within(card).queryByText('集群健康')).not.toBeInTheDocument()
})

it('Elasticsearch 仅节点为空但集群存在时仍展示汇总', async () => {
  const fixture = elasticsearchOverviewFixture()
  fixture.data.nodes = {
    total: 0,
    normal: 0,
    warning: 0,
    critical: 0,
    unknown: 0,
  }
  mockOverviewRequests({ elasticsearch: fixture })

  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 Elasticsearch 板块',
  })
  expect(card).not.toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('异常节点')).toBeVisible()
  expect(within(card).getByText('集群健康')).toBeVisible()
})

const invalidElasticsearchOverviewCases = [
  {
    name: 'data 不是 object',
    body: () => ({
      data: null,
      meta: {
        request_id: 'req-fixture-elasticsearch-invalid-data',
        stale: false,
        collected_at: '2026-08-01T08:00:00.000Z',
      },
    }),
  },
  {
    name: '缺少 request_rejections 告警组',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      delete (
        fixture.data.alerts as Partial<
          ElasticsearchOverviewFixture['data']['alerts']
        >
      ).request_rejections
      return fixture
    },
  },
  {
    name: 'status 枚举无效',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.status = 'degraded' as ElasticsearchOverviewFixture['data']['status']
      return fixture
    },
  },
  {
    name: 'meta collected_at 不是 string',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.meta.collected_at = 42 as unknown as string
      return fixture
    },
  },
  {
    name: '节点计数为负数',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.nodes.warning = -1
      return fixture
    },
  },
  {
    name: '集群计数不是安全整数',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.clusters.total = Number.MAX_SAFE_INTEGER + 1
      return fixture
    },
  },
  {
    name: 'AlertCount 不是整数',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.alerts.cluster_health.warning = 0.5
      return fixture
    },
  },
  {
    name: 'clusters total 不等于状态桶之和',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.clusters.total += 1
      return fixture
    },
  },
  {
    name: 'nodes total 不等于状态桶之和',
    body: () => {
      const fixture = elasticsearchOverviewFixture()
      fixture.data.nodes.total += 1
      return fixture
    },
  },
] satisfies Array<{ name: string; body: () => unknown }>

it('Elasticsearch overview 缺少 collected_at 时仍渲染有效卡片', async () => {
  const fixture = elasticsearchOverviewFixture()
  delete (
    fixture.meta as Partial<ElasticsearchOverviewFixture['meta']>
  ).collected_at
  mockOverviewRequests({ elasticsearch: fixture })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toBeVisible()
  expect(
    screen.queryByRole('alert', { name: 'Elasticsearch 板块加载失败' }),
  ).not.toBeInTheDocument()
})

it.each(invalidElasticsearchOverviewCases)(
  'Elasticsearch 成功响应拒绝$name并保持其他模块可用',
  async ({ body }) => {
    mockOverviewRequests({ elasticsearch: body() })
    renderOverview()

    expect(
      await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
    ).toBeVisible()
    expect(
      screen.getByRole('alert', { name: 'Elasticsearch 板块加载失败' }),
    ).toHaveTextContent('服务器响应格式无效')
    expect(
      screen.queryByRole('link', { name: '查看 Elasticsearch 板块' }),
    ).not.toBeInTheDocument()
  },
)

it('Elasticsearch 使用集群与节点中最坏级别而不直接信任 status', async () => {
  mockOverviewRequests({
    elasticsearch: elasticsearchOverviewFixture({
      data: {
        status: 'normal',
        clusters: {
          total: 1,
          normal: 0,
          warning: 1,
          critical: 0,
          unknown: 0,
        },
        nodes: {
          total: 1,
          normal: 0,
          warning: 0,
          critical: 1,
          unknown: 0,
        },
      },
    }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toHaveAttribute('data-level', 'critical')
})

it('其他模块已加载时独立显示 Elasticsearch 加载状态', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return new Promise<Response>(() => undefined)
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('status', { name: 'Elasticsearch 板块加载中' }),
  ).toBeVisible()
})

it('Elasticsearch 首次请求失败不阻塞其他模块且可独立重试', async () => {
  let attempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      attempts += 1
      return Promise.resolve(
        attempts === 1
          ? jsonResponse(
              {
                code: 'elasticsearch_unavailable',
                message: 'Elasticsearch 数据源暂时不可用',
                request_id: 'req-fixture-elasticsearch-overview-error',
                retryable: true,
              },
              503,
            )
          : jsonResponse(elasticsearchOverviewFixture()),
      )
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  const error = screen.getByRole('alert', {
    name: 'Elasticsearch 板块加载失败',
  })
  expect(error).toHaveTextContent('Elasticsearch 数据源暂时不可用')
  await user.click(within(error).getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toBeVisible()
  expect(attempts).toBe(2)
})

it('Elasticsearch 过期数据保持可见并显示精确采集时间', async () => {
  mockOverviewRequests({
    elasticsearch: elasticsearchOverviewFixture({
      meta: {
        stale: true,
        collected_at: '2026-08-01T07:30:00.000Z',
      },
    }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Elasticsearch 板块' }),
  ).toBeVisible()
  const banner = screen.getByRole('alert', {
    name: 'Elasticsearch 数据已过期',
  })
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-08-01T07:30:00.000Z',
  )
})

it('Elasticsearch 后台刷新失败时保留旧卡并显示独立重试状态', async () => {
  let attempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      attempts += 1
      return Promise.resolve(
        attempts === 1
          ? jsonResponse(elasticsearchOverviewFixture())
          : jsonResponse(
              {
                code: 'elasticsearch_unavailable',
                message: 'Elasticsearch 数据刷新失败',
                request_id:
                  'req-fixture-elasticsearch-overview-refresh-error',
                retryable: true,
              },
              503,
            ),
      )
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const queryClient = renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 Elasticsearch 板块',
  })
  await queryClient.invalidateQueries({ queryKey: ['elasticsearch-overview'] })

  expect(card).toBeVisible()
  const error = await screen.findByRole('alert', {
    name: 'Elasticsearch 板块刷新失败',
  })
  expect(error).toHaveTextContent('Elasticsearch 数据刷新失败')
  expect(
    within(error).getByRole('button', {
      name: '重试 Elasticsearch 板块',
    }),
  ).toBeVisible()
})

it('全正常时用绿色无异常文案展示所有零值状态', async () => {
  mockOverviewRequests({
    host: overviewFixture({
        data: {
          total: 3,
          online: 3,
          offline: 0,
          unknown: 0,
          alerts: {
            affected_hosts: 0,
            warning_hosts: 0,
            critical_hosts: 0,
            cpu: { warning: 0, critical: 0 },
            memory: { warning: 0, critical: 0 },
            io: { warning: 0, critical: 0 },
            network: { warning: 0, critical: 0 },
          },
        },
      }),
    mysql: mysqlOverviewFixture({
      data: {
        total: 2,
        normal: 2,
        warning: 0,
        critical: 0,
        unknown: 0,
        affected_instances: 0,
        warning_instances: 0,
        critical_instances: 0,
        alerts: {
          availability: { warning: 0, critical: 0 },
          replication_threads: { warning: 0, critical: 0 },
          replication_lag: { warning: 0, critical: 0 },
          replication_data: { warning: 0, critical: 0 },
        },
      },
    }),
    disk: diskOverviewFixture({
      data: {
        total: 4,
        normal: 4,
        warning: 0,
        critical: 0,
        unknown: 0,
        affected_devices: 0,
        warning_devices: 0,
        critical_devices: 0,
        alerts: {
          smart_health: { warning: 0, critical: 0 },
          device_warning: { warning: 0, critical: 0 },
          attribute_failure: { warning: 0, critical: 0 },
          collection: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  expect(hostCard).toHaveAttribute('data-level', 'normal')
  for (const label of ['无严重', '无警告']) {
    expect(within(hostCard).getByText(label)).toHaveAttribute(
      'data-level',
      'normal',
    )
  }
  const metricAlerts = hostCard.querySelectorAll('.module-metric-alert')
  expect(metricAlerts).toHaveLength(4)
  for (const metricAlert of metricAlerts) {
    expect(metricAlert).toHaveAttribute('data-level', 'normal')
    expect(within(metricAlert as HTMLElement).getByText('无异常')).toBeInTheDocument()
  }
  expect(within(hostCard).queryByText(/严重 0/)).not.toBeInTheDocument()
  expect(within(hostCard).queryByText(/警告 0/)).not.toBeInTheDocument()
  expect(within(hostCard).queryByText(/离线 0/)).not.toBeInTheDocument()

  const mysqlCard = screen.getByRole('link', { name: '查看 MySQL 板块' })
  expect(mysqlCard).toHaveAttribute('data-level', 'normal')
  expect(within(mysqlCard).getAllByText('无异常')).toHaveLength(4)
  expect(within(mysqlCard).getByText('无警告风险')).toBeVisible()
  expect(mysqlCard.querySelector('.module-status-breakdown')).toBeNull()

  const diskCard = screen.getByRole('link', { name: '查看主机硬盘板块' })
  expect(diskCard).toHaveAttribute('data-level', 'normal')
  expect(within(diskCard).getByText('全部正常')).toBeVisible()
  expect(within(diskCard).getAllByText('无异常')).toHaveLength(4)
})

it('RabbitMQ 全正常时四个摘要框统一显示无异常', async () => {
  mockOverviewRequests({
    rabbitmq: rabbitMQOverviewFixture({
      data: {
        status: 'normal',
        clusters: {
          total: 1,
          normal: 1,
          warning: 0,
          critical: 0,
          unknown: 0,
        },
        nodes: {
          total: 1,
          normal: 1,
          warning: 0,
          critical: 0,
          unknown: 0,
        },
        alerts: {
          cluster_connectivity: { warning: 0, critical: 0, unknown: 0 },
          resource_alarms: { warning: 0, critical: 0, unknown: 0 },
          resource_pressure: { warning: 0, critical: 0, unknown: 0 },
          collection: { warning: 0, critical: 0, unknown: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看 RabbitMQ 板块',
  })
  expect(card).toHaveAttribute('data-level', 'normal')
  expect(within(card).getAllByText('无异常')).toHaveLength(4)
  for (const metricAlert of card.querySelectorAll('.module-metric-alert')) {
    expect(metricAlert).toHaveAttribute('data-level', 'normal')
  }
  expect(within(card).queryByText(/严重 0/)).not.toBeInTheDocument()
  expect(within(card).queryByText(/警告 0/)).not.toBeInTheDocument()
  expect(within(card).queryByText(/未知 0/)).not.toBeInTheDocument()
})

it('硬盘无设备时显示中性空状态', async () => {
  mockOverviewRequests({
    disk: diskOverviewFixture({
      data: {
        total: 0,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 0,
        affected_devices: 0,
        warning_devices: 0,
        critical_devices: 0,
        alerts: {
          smart_health: { warning: 0, critical: 0 },
          device_warning: { warning: 0, critical: 0 },
          attribute_failure: { warning: 0, critical: 0 },
          collection: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看主机硬盘板块' })
  expect(card).toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('暂无硬盘设备')).toBeVisible()
  expect(within(card).queryByText('全部正常')).not.toBeInTheDocument()
  expect(within(card).queryByText('无异常')).not.toBeInTheDocument()
})

it('硬盘 unknown 直接使用后端 warning_devices 归入警告风险', async () => {
  mockOverviewRequests({
    disk: diskOverviewFixture({
      data: {
        total: 8,
        normal: 5,
        warning: 1,
        critical: 0,
        unknown: 2,
        affected_devices: 3,
        warning_devices: 3,
        critical_devices: 0,
        alerts: {
          smart_health: { warning: 1, critical: 0 },
          device_warning: { warning: 0, critical: 0 },
          attribute_failure: { warning: 0, critical: 0 },
          collection: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看主机硬盘板块' })
  expect(card).toHaveAttribute('data-level', 'warning')
  expect(within(card).getByText('存在警告或未知')).toBeVisible()
  expect(within(card).getByText('异常设备')).toBeVisible()
  expect(within(card).getByText('3')).toBeVisible()
  expect(within(card).getByText('警告风险 3')).toBeVisible()
  expect(within(card).queryByText('警告风险 1')).not.toBeInTheDocument()
})

it('Linux 和 MySQL 已加载时独立显示硬盘加载状态', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return new Promise<Response>(() => undefined)
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(screen.getByRole('link', { name: '查看 MySQL 板块' })).toBeVisible()
  expect(
    screen.getByRole('status', { name: '主机硬盘板块加载中' }),
  ).toBeVisible()
})

it('硬盘首次请求失败不阻塞 Linux 和 MySQL 且可独立重试', async () => {
  let diskAttempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      diskAttempts += 1
      return Promise.resolve(
        diskAttempts === 1
          ? jsonResponse(
              {
                code: 'datasource_unavailable',
                message: '硬盘数据源暂时不可用',
                request_id: 'req-fixture-disk-overview-error',
                retryable: true,
              },
              503,
            )
          : jsonResponse(diskOverviewFixture()),
      )
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(screen.getByRole('link', { name: '查看 MySQL 板块' })).toBeVisible()
  const error = screen.getByRole('alert', { name: '主机硬盘板块加载失败' })
  expect(error).toHaveTextContent('硬盘数据源暂时不可用')
  await user.click(within(error).getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('link', { name: '查看主机硬盘板块' }),
  ).toBeVisible()
  expect(diskAttempts).toBe(2)
})

it('硬盘成功响应结构无效时独立报错而不使总览崩溃', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(
        jsonResponse({
          data: { authenticated: true, username: 'admin' },
          meta: { request_id: 'req-malformed-disk-overview', stale: false },
        }),
      )
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(screen.getByRole('link', { name: '查看 MySQL 板块' })).toBeVisible()
  expect(
    screen.getByRole('alert', { name: '主机硬盘板块加载失败' }),
  ).toHaveTextContent('服务器响应格式无效')
})

const inconsistentDiskOverviewCases = [
  {
    name: '负数计数',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.normal = -1
    },
  },
  {
    name: '小数计数',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.alerts.smart_health.warning = 0.5
    },
  },
  {
    name: '状态桶总和不等于 total',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.total = 7
    },
  },
  {
    name: 'affected_devices 不等于风险状态去重数',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.affected_devices = 4
    },
  },
  {
    name: 'warning_devices 不等于 warning 加 unknown',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.warning_devices = 2
    },
  },
  {
    name: 'critical_devices 不等于 critical',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.critical_devices = 1
    },
  },
  {
    name: '单项 alert 计数大于 total',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.alerts.collection.critical = 7
    },
  },
  {
    name: '同一 alert 分类风险数之和大于 total',
    mutate: (fixture: DiskOverviewFixture) => {
      fixture.data.alerts.smart_health.warning = 4
      fixture.data.alerts.smart_health.critical = 3
    },
  },
] satisfies Array<{
  name: string
  mutate: (fixture: DiskOverviewFixture) => void
}>

it.each(inconsistentDiskOverviewCases)(
  '硬盘成功响应拒绝$name并保持其他模块可用',
  async ({ mutate }) => {
    const disk = diskOverviewFixture()
    mutate(disk)
    mockOverviewRequests({ disk })
    renderOverview()

    expect(
      await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
    ).toBeVisible()
    expect(screen.getByRole('link', { name: '查看 MySQL 板块' })).toBeVisible()
    expect(
      screen.getByRole('alert', { name: '主机硬盘板块加载失败' }),
    ).toHaveTextContent('服务器响应格式无效')
    expect(
      screen.queryByRole('link', { name: '查看主机硬盘板块' }),
    ).not.toBeInTheDocument()
  },
)

it('硬盘过期数据保持可见并显示精确采集时间', async () => {
  mockOverviewRequests({
    disk: diskOverviewFixture({
      meta: {
        stale: true,
        collected_at: '2026-07-30T09:45:00.000Z',
      },
    }),
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看主机硬盘板块' }),
  ).toBeVisible()
  const banner = screen.getByRole('alert', { name: '主机硬盘数据已过期' })
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-07-30T09:45:00.000Z',
  )
})

it('硬盘刷新失败时保留旧卡并显示独立重试状态', async () => {
  let diskAttempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      diskAttempts += 1
      return Promise.resolve(
        diskAttempts === 1
          ? jsonResponse(diskOverviewFixture())
          : jsonResponse(
              {
                code: 'datasource_unavailable',
                message: '硬盘数据刷新失败',
                request_id: 'req-fixture-disk-overview-refresh-error',
                retryable: true,
              },
              503,
            ),
      )
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const queryClient = renderOverview()

  const card = await screen.findByRole('link', {
    name: '查看主机硬盘板块',
  })
  await queryClient.invalidateQueries({ queryKey: ['disk-overview'] })

  expect(card).toBeVisible()
  const error = await screen.findByRole('alert', {
    name: '主机硬盘板块刷新失败',
  })
  expect(error).toHaveTextContent('硬盘数据刷新失败')
  expect(
    within(error).getByRole('button', { name: '重试 主机硬盘板块' }),
  ).toBeVisible()
})

it('把未知实例作为 warning 风险但保留未知文案', async () => {
  mockOverviewRequests({
    mysql: mysqlOverviewFixture({
      data: {
        total: 1,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 1,
        affected_instances: 1,
        warning_instances: 1,
        critical_instances: 0,
        alerts: {
          availability: { warning: 0, critical: 0 },
          replication_threads: { warning: 0, critical: 0 },
          replication_lag: { warning: 0, critical: 0 },
          replication_data: { warning: 1, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(card).toHaveAttribute('data-level', 'warning')
  expect(within(card).getByText('存在警告或未知')).toBeVisible()
  expect(within(card).getByText('警告风险 1')).toBeVisible()
  expect(card.querySelector('.module-status-breakdown')).toBeNull()
})

it('MySQL 卡片只保留上方异常严重度摘要', async () => {
  mockOverviewRequests({
    mysql: mysqlOverviewFixture({
      data: {
        total: 10,
        normal: 4,
        warning: 2,
        critical: 1,
        unknown: 3,
        affected_instances: 6,
        warning_instances: 5,
        critical_instances: 1,
        alerts: {
          availability: { warning: 2, critical: 1 },
          replication_threads: { warning: 0, critical: 0 },
          replication_lag: { warning: 0, critical: 0 },
          replication_data: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(within(card).getByText('警告风险 5')).toBeVisible()
  expect(card.querySelector('.module-status-breakdown')).toBeNull()
})

it('Linux 无主机时显示中性空状态且不影响 MySQL 正常卡', async () => {
  mockOverviewRequests({
    host: overviewFixture({
      data: {
        total: 0,
        online: 0,
        offline: 0,
        unknown: 0,
        alerts: {
          affected_hosts: 0,
          warning_hosts: 0,
          critical_hosts: 0,
          cpu: { warning: 0, critical: 0 },
          memory: { warning: 0, critical: 0 },
          io: { warning: 0, critical: 0 },
          network: { warning: 0, critical: 0 },
        },
      },
    }),
    mysql: mysqlOverviewFixture({
      data: {
        total: 2,
        normal: 2,
        warning: 0,
        critical: 0,
        unknown: 0,
        affected_instances: 0,
        warning_instances: 0,
        critical_instances: 0,
        alerts: {
          availability: { warning: 0, critical: 0 },
          replication_threads: { warning: 0, critical: 0 },
          replication_lag: { warning: 0, critical: 0 },
          replication_data: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  expect(hostCard).toHaveAttribute('data-level', 'empty')
  expect(within(hostCard).getByText('暂无 Linux 主机')).toBeVisible()
  expect(within(hostCard).queryByText('全部正常')).not.toBeInTheDocument()
  expect(within(hostCard).queryByText('/ 0')).not.toBeInTheDocument()
  expect(within(hostCard).queryByText('无严重')).not.toBeInTheDocument()
  expect(within(hostCard).queryByText('无警告')).not.toBeInTheDocument()
  expect(within(hostCard).queryByText('无异常')).not.toBeInTheDocument()
  expect(within(hostCard).queryByText('在线 0')).not.toBeInTheDocument()

  const mysqlCard = screen.getByRole('link', { name: '查看 MySQL 板块' })
  expect(mysqlCard).toHaveAttribute('data-level', 'normal')
  expect(within(mysqlCard).getByText('全部正常')).toBeVisible()
})

it('MySQL 无实例时显示空状态而不是异常', async () => {
  mockOverviewRequests({
    mysql: mysqlOverviewFixture({
      data: {
        total: 0,
        normal: 0,
        warning: 0,
        critical: 0,
        unknown: 0,
        affected_instances: 0,
        warning_instances: 0,
        critical_instances: 0,
        alerts: {
          availability: { warning: 0, critical: 0 },
          replication_threads: { warning: 0, critical: 0 },
          replication_lag: { warning: 0, critical: 0 },
          replication_data: { warning: 0, critical: 0 },
        },
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(card).toHaveAttribute('data-level', 'empty')
  expect(within(card).getByText('暂无 MySQL 实例')).toBeVisible()
  expect(within(card).queryByText('全部正常')).not.toBeInTheDocument()
  expect(within(card).queryByText('无严重')).not.toBeInTheDocument()
  expect(within(card).queryByText('无警告')).not.toBeInTheDocument()
  expect(within(card).queryByText('无异常')).not.toBeInTheDocument()
  expect(within(card).queryByText('正常 0')).not.toBeInTheDocument()
})

it('Linux 已加载时独立显示 MySQL 加载状态', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return new Promise<Response>(() => undefined)
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('status', { name: 'MySQL 板块加载中' }),
  ).toBeVisible()
})

it('Linux 和 MySQL 卡片失败互不影响', async () => {
  mockOverviewRequests({
    host: overviewFixture(),
    mysqlError: {
      code: 'datasource_unavailable',
      message: '数据源暂时不可用，请稍后重试',
      request_id: 'req-fixture-mysql-overview-error',
      retryable: true,
    },
  })
  renderOverview()

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    screen.getByRole('alert', { name: 'MySQL 板块加载失败' }),
  ).toBeVisible()
})

it('MySQL 刷新失败时保留旧卡并显示独立重试状态', async () => {
  let mysqlAttempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/overview') {
      return Promise.resolve(jsonResponse(overviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    mysqlAttempts += 1
    return Promise.resolve(
      mysqlAttempts === 1
        ? jsonResponse(mysqlOverviewFixture())
        : jsonResponse(
            {
              code: 'datasource_unavailable',
              message: 'MySQL 数据源暂时不可用',
              request_id: 'req-fixture-mysql-overview-refresh-error',
              retryable: true,
            },
            503,
          ),
    )
  })
  const queryClient = renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  await queryClient.invalidateQueries({ queryKey: ['mysql-overview'] })

  expect(card).toBeVisible()
  const error = await screen.findByRole('alert', {
    name: 'MySQL 板块刷新失败',
  })
  expect(error).toHaveTextContent('MySQL 数据源暂时不可用')
  expect(
    within(error).getByRole('button', { name: '重试 MySQL 板块' }),
  ).toBeVisible()
})

it('MySQL 过期数据保持可见并显示精确采集时间', async () => {
  mockOverviewRequests({
    mysql: mysqlOverviewFixture({
      meta: {
        stale: true,
        collected_at: '2026-07-28T07:30:00.000Z',
      },
    }),
  })
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(card).toBeVisible()
  const banner = screen.getByRole('alert', { name: 'MySQL 数据已过期' })
  expect(banner).toHaveTextContent('MySQL 数据已过期')
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-07-28T07:30:00.000Z',
  )
})

it('使用固定查询范围且不显示总览时间范围控件', async () => {
  const requestedRanges: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(jsonResponse(rabbitMQOverviewFixture()))
    }
    requestedRanges.push(requestedRange(input))
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  renderOverview()

  await screen.findByRole('link', { name: '查看 Linux 主机板块' })
  await waitFor(() => expect(requestedRanges).toEqual(['24h']))
  expect(screen.queryByRole('button', { name: '1小时' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '6小时' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '24小时' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '7天' })).not.toBeInTheDocument()
})

it('不提供全局手动刷新控件', async () => {
  renderOverview()

  await screen.findByRole('link', { name: '查看 Linux 主机板块' })
  await screen.findByRole('link', { name: '查看主机硬盘板块' })
  expect(globalThis.fetch).toHaveBeenCalledTimes(7)
  expect(screen.queryByRole('button', { name: '刷新' })).not.toBeInTheDocument()
  expect(screen.queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument()
})

it('每 15 秒刷新且前一请求未完成时不发起重叠请求', async () => {
  vi.useFakeTimers()
  let hostRequestCount = 0
  let resolveRefresh!: (response: Response) => void
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(jsonResponse(rabbitMQOverviewFixture()))
    }
    hostRequestCount += 1
    if (hostRequestCount === 1) {
      return Promise.resolve(jsonResponse(overviewFixture()))
    }
    return new Promise<Response>((resolve) => {
      resolveRefresh = resolve
    })
  })

  renderOverview()
  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(hostRequestCount).toBe(1)

  await act(async () => vi.advanceTimersByTimeAsync(15_000))
  expect(hostRequestCount).toBe(2)

  await act(async () => vi.advanceTimersByTimeAsync(60_000))
  expect(hostRequestCount).toBe(2)

  await act(async () => {
    resolveRefresh(jsonResponse(overviewFixture()))
    await vi.advanceTimersByTimeAsync(0)
  })
  await act(async () => vi.advanceTimersByTimeAsync(15_000))
  expect(hostRequestCount).toBe(3)
})

it('过期数据提示显示服务端给出的精确采集时间', async () => {
  mockOverviewRequests({
    host: overviewFixture({
        meta: {
          stale: true,
          collected_at: '2026-07-21T00:30:00.000Z',
        },
      }),
  })
  renderOverview()

  const banner = await screen.findByRole('alert', {
    name: 'Linux 主机数据已过期',
  })
  expect(banner).toHaveTextContent('Linux 主机数据已过期')
  expect(banner).toHaveTextContent('最新数据时间：2026/07/21 00:30:00')
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-07-21T00:30:00.000Z',
  )
})

it('可重试错误显示中文信息并可重试成功', async () => {
  let attempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const path = requestedPath(input)
    if (path === '/api/v1/java/overview') {
      return Promise.resolve(jsonResponse(javaOverviewFixture()))
    }
    if (path === '/api/v1/elasticsearch/overview') {
      return Promise.resolve(jsonResponse(elasticsearchOverviewFixture()))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/disks/overview') {
      return Promise.resolve(jsonResponse(diskOverviewFixture()))
    }
    if (path === '/api/v1/redis/overview') {
      return Promise.resolve(jsonResponse(redisOverviewFixture()))
    }
    if (path === '/api/v1/rabbitmq/overview') {
      return Promise.resolve(jsonResponse(rabbitMQOverviewFixture()))
    }
    attempts += 1
    if (attempts === 1) {
      return Promise.resolve(
        jsonResponse(
          {
            code: 'datasource_unavailable',
            message: '数据源暂时不可用，请稍后重试',
            request_id: 'req-overview-error-001',
            retryable: true,
          },
          503,
        ),
      )
    }
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  expect(await screen.findByRole('alert')).toHaveTextContent(
    '数据源暂时不可用，请稍后重试',
  )
  expect(screen.getByRole('alert')).toHaveTextContent('无法加载总览数据')
  await user.click(screen.getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeInTheDocument()
  expect(attempts).toBe(2)
})

it('不可重试错误不显示重试入口', async () => {
  mockOverviewRequests({
    hostError: {
        code: 'invalid_range',
        message: '时间范围无效',
        request_id: 'req-overview-error-002',
        retryable: false,
      },
  })
  renderOverview()

  expect(
    await screen.findByRole('alert', { name: 'Linux 主机板块加载失败' }),
  ).toHaveTextContent('时间范围无效')
  expect(screen.queryByRole('button', { name: '重试' })).not.toBeInTheDocument()
})
