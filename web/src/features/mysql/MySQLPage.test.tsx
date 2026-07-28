import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { mysqlInstancePageFixture } from '../../test/fixtures'
import { MySQLPage } from './MySQLPage'

const requestedURLs: URL[] = []

function requestURL(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost')
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderMySQLPage(initialEntry = '/mysql') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })
  window.history.replaceState({}, '', initialEntry)

  render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/mysql" element={<MySQLPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

function lastRequest() {
  const url = requestedURLs.at(-1)
  if (url === undefined) throw new Error('尚未发送 MySQL 实例列表请求')
  return url
}

beforeEach(() => {
  requestedURLs.length = 0
  window.history.replaceState({}, '', '/')
  vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const page = Number(url.searchParams.get('page') ?? '1')
    const pageSize = Number(url.searchParams.get('page_size') ?? '20')
    return Promise.resolve(
      jsonResponse(
        mysqlInstancePageFixture({
          data: {
            page,
            page_size: pageSize,
            total_pages: Math.ceil(64 / pageSize),
          },
        }),
      ),
    )
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('renders the eleven compact MySQL columns', async () => {
  renderMySQLPage('/mysql')
  expect(
    await screen.findByRole('heading', { name: 'MySQL 实例' }),
  ).toBeVisible()
  for (const heading of [
    '实例',
    '所属主机',
    '版本 / 角色',
    '连接使用',
    '活跃线程',
    'QPS',
    '慢查询速率',
    'Buffer Pool 使用率',
    '复制状态 / 延迟',
    '运行时间',
    '状态',
  ]) {
    expect(
      screen.getByRole('columnheader', {
        name: new RegExp(`^${heading}(排序|$)`),
      }),
    ).toBeVisible()
  }

  const instance = (await screen.findByText('fixture-mysql-a')).closest('tr')
  expect(instance).not.toBeNull()
  const cells = within(instance!).getAllByRole('cell')
  expect(cells).toHaveLength(11)
  expect(cells[0]).toHaveTextContent('fixture-mysql-a · 192.0.2.101:3306')
  expect(cells[0].querySelector('.host-name-text')).toHaveAttribute(
    'title',
    'fixture-mysql-a · 192.0.2.101:3306',
  )
  expect(cells[1]).toHaveTextContent('fixture-db-host-a')
  expect(cells[2]).toHaveTextContent('8.4.1 · 可写')
  expect(cells[3]).toHaveTextContent('32 / 200 (16.0%)')
  expect(cells[4]).toHaveTextContent('5')
  expect(cells[5]).toHaveTextContent('123.46')
  expect(cells[6]).toHaveTextContent('0.13')
  expect(cells[7]).toHaveTextContent('82.3%')
  expect(cells[8]).toHaveTextContent('正常 · 2s')
  expect(cells[9]).toHaveTextContent('2天 3小时')
  expect(within(cells[10]).getByText('正常')).toHaveAttribute(
    'data-level',
    'normal',
  )
  expect(
    screen.queryByRole('button', { name: /重启|删除|执行|修改|详情|历史/ }),
  ).not.toBeInTheDocument()
})

it('writes filters sort and pagination to the URL', async () => {
  const user = userEvent.setup()
  renderMySQLPage('/mysql?page=3')
  await screen.findByText('fixture-mysql-a')
  await user.selectOptions(
    screen.getByRole('combobox', { name: '实例状态' }),
    'warning',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '读写属性' }),
    'read_only',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '每页数量' }),
    '50',
  )
  await user.click(screen.getByRole('button', { name: /^QPS排序/ }))
  expect(window.location.search).toContain('status=warning')
  expect(window.location.search).toContain('role=read_only')
  expect(window.location.search).toContain('page_size=50')
  expect(window.location.search).toContain('sort=qps')
  expect(window.location.search).toContain('page=1')

  await waitFor(() => {
    expect(lastRequest().pathname).toBe('/api/v1/mysql/instances')
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      status: 'warning',
      role: 'read_only',
      sort: 'qps',
      order: 'asc',
      page: '1',
      page_size: '50',
    })
  })
})

it('debounces search and sends only fixed GET parameters', async () => {
  vi.useFakeTimers()
  renderMySQLPage('/mysql')
  await act(async () => vi.advanceTimersByTimeAsync(0))
  fireEvent.change(
    screen.getByRole('searchbox', {
      name: '搜索实例名称、地址或所属主机',
    }),
    { target: { value: 'fixture' } },
  )
  await act(async () => vi.advanceTimersByTimeAsync(299))
  expect(requestedURLs).toHaveLength(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(requestedURLs).toHaveLength(2)
  expect(lastRequest().searchParams.get('search')).toBe('fixture')
  expect(lastRequest().searchParams.get('page')).toBe('1')
  expect([...lastRequest().searchParams.keys()].sort()).toEqual(
    ['order', 'page', 'page_size', 'search', 'sort'].sort(),
  )
})

it('renders missing metrics as 暂无数据', async () => {
  const fixture = mysqlInstancePageFixture()
  fixture.data.instances[0] = {
    ...fixture.data.instances[0],
    connections: null,
    max_connections: null,
    connection_usage_percent: null,
    threads_running: null,
    qps: null,
    slow_queries_per_second: null,
    buffer_pool_usage_percent: null,
    uptime_seconds: null,
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()
  const row = (await screen.findByText('fixture-mysql-a')).closest('tr')
  expect(row).not.toBeNull()
  expect(within(row!).getAllByText('暂无数据')).toHaveLength(6)
})

it('preserves available connection values without inventing zeroes', async () => {
  const fixture = mysqlInstancePageFixture()
  fixture.data.instances[0] = {
    ...fixture.data.instances[0],
    connections: 7,
    max_connections: null,
    connection_usage_percent: null,
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()
  const row = (await screen.findByText('fixture-mysql-a')).closest('tr')
  expect(row).not.toBeNull()
  const connectionCell = within(row!).getAllByRole('cell')[3]
  expect(connectionCell).toHaveTextContent('7')
  expect(connectionCell).not.toHaveTextContent('0')
})

it('renders every replication and instance state with text and level', async () => {
  renderMySQLPage()
  await screen.findByText('fixture-mysql-a')
  for (const label of ['正常', '线程异常', '未配置复制', '状态未知']) {
    expect(screen.getAllByText(label).length).toBeGreaterThan(0)
  }
  for (const [label, level] of [
    ['正常', 'normal'],
    ['警告', 'warning'],
    ['严重', 'critical'],
    ['未知', 'unknown'],
  ] as const) {
    expect(screen.getByText(label, { selector: '.mysql-status' })).toHaveAttribute(
      'data-level',
      level,
    )
  }
})

it('renders loading empty and first-load error states', async () => {
  let resolveInitial!: (response: Response) => void
  vi.mocked(globalThis.fetch).mockImplementationOnce(
    () =>
      new Promise<Response>((resolve) => {
        resolveInitial = resolve
      }),
  )
  renderMySQLPage()
  expect(screen.getByRole('status')).toHaveTextContent(
    '正在加载 MySQL 实例列表…',
  )
  await act(async () =>
    resolveInitial(
      jsonResponse(
        mysqlInstancePageFixture({
          data: { instances: [], total: 0, total_pages: 0 },
        }),
      ),
    ),
  )
  expect(await screen.findByText('没有符合条件的 MySQL 实例')).toBeVisible()

  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(
      {
        code: 'datasource_unavailable',
        message: '数据源暂时不可用，请稍后重试',
        request_id: 'req-fixture-mysql-load-error',
        retryable: true,
      },
      503,
    ),
  )
  renderMySQLPage('/mysql?status=critical')
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '无法加载 MySQL 实例列表',
  )
})

it('normalizes invalid URL state and out-of-range response pages', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const requestedPage = Number(url.searchParams.get('page') ?? '1')
    return Promise.resolve(
      jsonResponse(
        mysqlInstancePageFixture({
          data: {
            page: requestedPage > 4 ? 4 : requestedPage,
            total_pages: 4,
          },
        }),
      ),
    )
  })
  const historyLength = window.history.length
  renderMySQLPage(
    '/mysql?status=broken&role=admin&sort=sql&order=sideways&page=999&page_size=9',
  )
  expect(await screen.findByText('第 4 / 4 页，共 64 个实例')).toBeVisible()
  expect(requestedURLs.map((url) => url.searchParams.get('page'))).toEqual([
    '999',
    '4',
  ])
  expect(window.location.search).toBe(
    '?status=&role=&sort=instance&order=asc&page=4&page_size=20',
  )
  expect(window.history.length).toBe(historyLength)
})

it('keeps stale data visible and reports background errors', async () => {
  const stale = mysqlInstancePageFixture({ meta: { stale: true } })
  vi.mocked(globalThis.fetch)
    .mockResolvedValueOnce(jsonResponse(stale))
    .mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'datasource_unavailable',
          message: '数据源暂时不可用，请稍后重试',
          request_id: 'req-fixture-mysql-refresh-error',
          retryable: true,
        },
        503,
      ),
    )
  renderMySQLPage()
  expect(await screen.findByText('fixture-mysql-a')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: '刷新 MySQL 实例列表' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('刷新失败')
  expect(screen.getByText('fixture-mysql-a')).toBeVisible()
})
