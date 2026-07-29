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
  mysqlOverviewFixture,
  overviewFixture,
  type ErrorFixture,
  type MySQLOverviewFixture,
  type OverviewFixture,
} from '../../test/fixtures'
import { OverviewPage } from './OverviewPage'

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
  mysql = mysqlOverviewFixture(),
  hostError,
  mysqlError,
}: {
  host?: OverviewFixture
  mysql?: MySQLOverviewFixture
  hostError?: ErrorFixture
  mysqlError?: ErrorFixture
} = {}) {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (requestedPath(input) === '/api/v1/mysql/overview') {
      return Promise.resolve(
        jsonResponse(mysqlError ?? mysql, mysqlError === undefined ? 200 : 503),
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

it('总览展示可进入 Linux 主机和 MySQL 板块的告警摘要卡', async () => {
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  const mysqlCard = screen.getByRole('link', {
    name: '查看 MySQL 板块',
  })
  const moduleGrid = screen.getByRole('group', { name: '基础设施模块' })
  expect(moduleGrid).toHaveClass(
    'overview-status-grid',
    'overview-compact-grid',
  )
  expect(
    within(moduleGrid).getByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
  expect(
    within(moduleGrid).getByRole('link', { name: '查看 MySQL 板块' }),
  ).toBeVisible()
  expect(screen.queryAllByRole('article')).toHaveLength(0)
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
  expect(within(hostCard).getByText('在线 9')).toBeInTheDocument()
  expect(within(hostCard).getByText('离线 2')).toBeInTheDocument()
  expect(within(hostCard).getByText('未知 1')).toBeInTheDocument()
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
  const controls = screen.getByRole('group', { name: '总览控制' })
  expect(
    within(controls).getByText(/上次刷新 \d{2}:\d{2}:\d{2}/),
  ).toBeInTheDocument()
  expect(within(controls).getByText(/每 15 秒自动刷新/)).toBeInTheDocument()
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
  })
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  expect(hostCard).toHaveAttribute('data-level', 'normal')
  for (const label of ['无严重', '无警告', '无离线', '无未知']) {
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
  expect(within(mysqlCard).getByText('无严重')).toBeVisible()
  expect(within(mysqlCard).getByText('无警告风险')).toBeVisible()
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
  expect(within(card).queryByText('无警告')).not.toBeInTheDocument()
  expect(within(card).getByText('未知 1')).toBeVisible()
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
    if (requestedPath(input) === '/api/v1/mysql/overview') {
      return new Promise<Response>(() => undefined)
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
    if (requestedPath(input) !== '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(overviewFixture()))
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
  const user = userEvent.setup()
  renderOverview()

  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  await user.click(screen.getByRole('button', { name: '刷新' }))

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
    if (requestedPath(input) === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
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

it('手动刷新会同时重新请求 Linux 和 MySQL 总览', async () => {
  const user = userEvent.setup()
  renderOverview()

  await screen.findByRole('link', { name: '查看 Linux 主机板块' })
  expect(globalThis.fetch).toHaveBeenCalledTimes(2)
  await user.click(screen.getByRole('button', { name: '刷新' }))

  await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(4))
  const calls = vi.mocked(globalThis.fetch).mock.calls
  expect(calls.map(([input]) => requestedPath(input))).toEqual(
    expect.arrayContaining([
      '/api/v1/overview',
      '/api/v1/mysql/overview',
    ]),
  )
  const hostCalls = calls.filter(
    ([input]) => requestedPath(input) === '/api/v1/overview',
  )
  expect(hostCalls).toHaveLength(2)
  expect(requestedRange(hostCalls[1][0])).toBe('24h')
})

it('每 15 秒刷新且前一请求未完成时不发起重叠请求', async () => {
  vi.useFakeTimers()
  let hostRequestCount = 0
  let resolveRefresh!: (response: Response) => void
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (requestedPath(input) === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
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
  expect(banner).toHaveTextContent('2026-07-21T00:30:00.000Z')
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-07-21T00:30:00.000Z',
  )
})

it('可重试错误显示中文信息并可重试成功', async () => {
  let attempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (requestedPath(input) === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
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
