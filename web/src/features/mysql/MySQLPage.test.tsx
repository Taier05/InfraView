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
  return queryClient
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

it('renders the ten compact MySQL columns with QPS and TPS combined', async () => {
  renderMySQLPage('/mysql')
  expect(
    await screen.findByRole('heading', { name: 'MySQL 实例' }),
  ).toBeVisible()
  const headers = [
    ['实例地址', 'MySQL 实例地址'],
    ['版本 / 角色', 'MySQL 版本 / 角色'],
    ['连接', '连接使用'],
    ['线程', '活跃线程'],
    ['QPS / TPS', '每秒查询数 / 显式事务数（按 QPS 排序）'],
    ['慢查询', '慢查询速率'],
    ['Buffer Pool', 'Buffer Pool 容量 / 使用率'],
    ['复制 / 延迟', '复制状态 / 延迟'],
    ['运行时间', '运行时间'],
    ['状态', '实例状态'],
  ]
  expect(screen.getAllByRole('columnheader')).toHaveLength(10)
  for (const [heading, title] of headers) {
    const header = screen.getByRole('columnheader', {
      name: new RegExp(`^${heading}(排序|$)`),
    })
    expect(header).toBeVisible()
    const titleElement = within(header).getByTitle(title)
    expect(titleElement).toBeVisible()
    expect(titleElement).toHaveAttribute('title', title)
  }

  for (const title of [
    'MySQL 实例地址',
    '连接使用',
    '活跃线程',
    '每秒查询数 / 显式事务数（按 QPS 排序）',
    '慢查询速率',
    'Buffer Pool 容量 / 使用率',
    '复制状态 / 延迟',
    '运行时间',
    '实例状态',
  ]) {
    expect(screen.getByTitle(title)).toHaveClass('host-sort-button')
  }
  expect(screen.getByTitle('实例状态')).toHaveClass(
    'status-align-header',
    'mysql-status-align-header',
  )

  const instance = (await screen.findByText('192.0.2.101:3306')).closest('tr')
  expect(instance).not.toBeNull()
  const cells = within(instance!).getAllByRole('cell')
  expect(cells).toHaveLength(10)
  expect(cells[0]).toHaveTextContent('192.0.2.101:3306')
  expect(cells[0]).not.toHaveTextContent('fixture-mysql-a')
  expect(cells[0].querySelector('.host-cell-text')).toHaveAttribute(
    'title',
    '192.0.2.101:3306',
  )
  expect(cells[1]).toHaveTextContent('8.4.1 · 读写')
  expect(cells[2]).toHaveTextContent('32/200 · 16.0%')
  expect(cells[3]).toHaveTextContent('5')
  expect(cells[4]).toHaveTextContent('123.46 / 45.25')
  expect(cells[5]).toHaveTextContent('0.13')
  expect(cells[6]).toHaveTextContent('8 GiB / 82.3%')
  expect(cells[7]).toHaveTextContent('正常 · 2s')
  expect(cells[7].querySelector('.host-metric')).toHaveAttribute(
    'title',
    '正常 · 2s',
  )
  expect(cells[8]).toHaveTextContent('2天 3小时')
  expect(cells[8].querySelector('.mysql-uptime')).toHaveAttribute(
    'title',
    '2天 3小时',
  )
  expect(within(cells[9]).getByText('正常')).toHaveAttribute(
    'data-level',
    'normal',
  )
  expect(
    screen.queryByRole('button', { name: /重启|删除|执行|修改|详情|历史/ }),
  ).not.toBeInTheDocument()
  expect(
    screen
      .getByRole('searchbox', { name: '搜索实例地址' })
      .closest('.mysql-list-controls'),
  ).not.toBeNull()
  const controls = screen
    .getByRole('searchbox', { name: '搜索实例地址' })
    .closest('.mysql-list-controls')
  expect(controls).not.toBeNull()
  if (!(controls instanceof HTMLElement)) {
    throw new Error('MySQL 控制区未渲染为 HTML 元素')
  }
  expect(within(controls).getAllByRole('combobox')).toHaveLength(4)
  const dataTime = await within(controls).findByText('2026/07/28 08:00:00')
  expect(dataTime.closest('.data-time')).toHaveTextContent('最新数据时间：2026/07/28 08:00:00')
  expect(within(controls).queryByRole('button', { name: /刷新/ })).not.toBeInTheDocument()
  expect(screen.getByRole('option', { name: '读写' })).toHaveValue('writable')
  expect(screen.queryByRole('option', { name: '可写' })).not.toBeInTheDocument()
  const table = screen.getByRole('table')
  expect(table).toHaveClass(
    'host-table',
    'mysql-table',
    'mysql-table-compact',
  )
  expect(table.closest('.mysql-table-scroll')).not.toBeNull()
})

it('renders missing and unknown versions as Chinese unknown with the role', async () => {
  renderMySQLPage()

  const missingVersionRow = (
    await screen.findByText('192.0.2.104:3306')
  ).closest('tr')
  expect(missingVersionRow).not.toBeNull()
  const missingVersionCell = within(missingVersionRow!).getAllByRole('cell')[1]
  expect(missingVersionCell).toHaveTextContent('未知 · 只读')
  expect(missingVersionCell).not.toHaveTextContent(/^\s*·/)

  const unknownVersionRow = screen.getByText('192.0.2.105:3306').closest('tr')
  expect(unknownVersionRow).not.toBeNull()
  expect(within(unknownVersionRow!).getAllByRole('cell')[1]).toHaveTextContent(
    '未知 · 只读',
  )
})

it('uses only service-valid fixture replication combinations', () => {
  const [normal, threadsStopped, notConfigured, missingReadOnly, warning] =
    mysqlInstancePageFixture().data.instances

  expect(normal).toMatchObject({
    role: 'writable',
    replication: { state: 'normal', level: 'normal' },
    status: 'normal',
    collection_level: 'normal',
  })
  expect(threadsStopped).toMatchObject({
    role: 'read_only',
    replication: { state: 'threads_stopped', level: 'critical' },
    status: 'critical',
    collection_level: 'normal',
  })
  expect(notConfigured).toMatchObject({
    role: 'writable',
    replication: { state: 'not_configured', level: 'normal' },
    status: 'normal',
    collection_level: 'normal',
  })
  expect(missingReadOnly).toMatchObject({
    version: '',
    role: 'read_only',
    replication: { state: 'unknown', level: 'unknown' },
    status: 'critical',
    collection_level: 'critical',
  })
  expect(warning).toMatchObject({
    version: 'unknown',
    role: 'read_only',
    replication: { state: 'normal', level: 'warning' },
    status: 'warning',
    collection_level: 'warning',
  })
})

it('writes filters sort and pagination to the URL', async () => {
  const user = userEvent.setup()
  renderMySQLPage('/mysql?page=3')
  await screen.findByText('192.0.2.101:3306')
  await user.selectOptions(
    screen.getByRole('combobox', { name: '实例状态' }),
    'warning',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '读写属性' }),
    'read_only',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '实例标签' }),
    'tier-fixture',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '每页数量' }),
    '50',
  )
  await user.click(
    screen.getByRole('button', {
      name: /^每秒查询数 \/ 显式事务数（按 QPS 排序）排序/,
    }),
  )
  expect(window.location.search).toContain('status=warning')
  expect(window.location.search).toContain('role=read_only')
  expect(window.location.search).toContain('label=tier-fixture')
  expect(window.location.search).toContain('page_size=50')
  expect(window.location.search).toContain('sort=qps')
  expect(window.location.search).toContain('page=1')

  await waitFor(() => {
    expect(lastRequest().pathname).toBe('/api/v1/mysql/instances')
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      status: 'warning',
      role: 'read_only',
      label: 'tier-fixture',
      sort: 'qps',
      order: 'asc',
      page: '1',
      page_size: '50',
    })
  })
})

it('places instance labels before status and restores labels from the URL', async () => {
  renderMySQLPage('/mysql?label=%20legacy-fixture%20')
  await screen.findByText('192.0.2.101:3306')

  const label = screen.getByRole('combobox', { name: '实例标签' })
  const status = screen.getByRole('combobox', { name: '实例状态' })
  expect(label.compareDocumentPosition(status)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING,
  )
  expect(label).toHaveValue('legacy-fixture')
  expect(within(label).getByRole('option', { name: '全部标签' })).toHaveValue('')
  expect(
    within(label).getByRole('option', { name: 'tier-fixture' }),
  ).toHaveValue('tier-fixture')
  expect(
    within(label).getByRole('option', { name: 'team-fixture' }),
  ).toHaveValue('team-fixture')
  expect(lastRequest().searchParams.get('label')).toBe('legacy-fixture')
  await waitFor(() =>
    expect(window.location.search).toContain('label=legacy-fixture'),
  )
  expect(requestedURLs).toHaveLength(1)
  expect(requestedURLs.every((url) => url.pathname === '/api/v1/mysql/instances')).toBe(
    true,
  )
})

it('selecting an instance label preserves filters and resets to page one', async () => {
  const user = userEvent.setup()
  renderMySQLPage(
    '/mysql?status=warning&role=read_only&sort=qps&order=desc&page=2&page_size=50',
  )
  await screen.findByText('192.0.2.101:3306')
  expect(requestedURLs).toHaveLength(1)
  await user.selectOptions(
    screen.getByRole('combobox', { name: '实例标签' }),
    'tier-fixture',
  )

  await waitFor(() => {
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      label: 'tier-fixture',
      status: 'warning',
      role: 'read_only',
      sort: 'qps',
      order: 'desc',
      page: '1',
      page_size: '50',
    })
  })
  expect(requestedURLs).toHaveLength(2)
  expect(requestedURLs.every((url) => url.pathname === '/api/v1/mysql/instances')).toBe(
    true,
  )
  expect(window.location.search).toContain('label=tier-fixture')
  expect(window.location.search).toContain('page=1')
})

it('requests the fixed endpoint with GET and an AbortSignal', async () => {
  const queryClient = renderMySQLPage()
  await screen.findByText('192.0.2.101:3306')

  const [input, init] = vi.mocked(globalThis.fetch).mock.calls[0]
  expect(requestURL(input).pathname).toBe('/api/v1/mysql/instances')
  expect(init?.method).toBe('GET')
  expect(init?.signal).toBeInstanceOf(AbortSignal)
})

it('uses server pagination for next and previous pages', async () => {
  const user = userEvent.setup()
  renderMySQLPage('/mysql?page=2')
  expect(await screen.findByText('第 2 / 4 页，共 64 个实例')).toBeVisible()

  await user.click(screen.getByRole('button', { name: '下一页' }))
  await waitFor(() => expect(lastRequest().searchParams.get('page')).toBe('3'))
  expect(window.location.search).toContain('page=3')

  await user.click(screen.getByRole('button', { name: '上一页' }))
  await waitFor(() => expect(lastRequest().searchParams.get('page')).toBe('2'))
  expect(window.location.search).toContain('page=2')
})

it('debounces search and sends only fixed GET parameters', async () => {
  vi.useFakeTimers()
  renderMySQLPage('/mysql')
  await act(async () => vi.advanceTimersByTimeAsync(0))
  fireEvent.change(
    screen.getByRole('searchbox', {
      name: '搜索实例地址',
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
    tps: null,
    slow_queries_per_second: null,
    buffer_pool_size_bytes: null,
    buffer_pool_usage_percent: null,
    uptime_seconds: null,
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()
  const row = (await screen.findByText('192.0.2.101:3306')).closest('tr')
  expect(row).not.toBeNull()
  expect(within(row!).getAllByText('暂无数据')).toHaveLength(5)
  expect(within(row!).getAllByRole('cell')[6]).toHaveTextContent('—')
})

it('renders all Buffer Pool capacity and usage availability combinations', async () => {
  const fixture = mysqlInstancePageFixture()
  fixture.data.instances[0] = {
    ...fixture.data.instances[0],
    buffer_pool_size_bytes: 1024 ** 3,
    buffer_pool_usage_percent: 50,
  }
  fixture.data.instances[1] = {
    ...fixture.data.instances[1],
    buffer_pool_size_bytes: 1,
    buffer_pool_usage_percent: null,
  }
  fixture.data.instances[2] = {
    ...fixture.data.instances[2],
    buffer_pool_size_bytes: null,
    buffer_pool_usage_percent: 25,
  }
  fixture.data.instances[3] = {
    ...fixture.data.instances[3],
    buffer_pool_size_bytes: null,
    buffer_pool_usage_percent: null,
  }
  fixture.data.instances[4] = {
    ...fixture.data.instances[4],
    buffer_pool_size_bytes: 1024,
    buffer_pool_usage_percent: 75,
  }
  fixture.data.instances.push(
    {
      ...fixture.data.instances[4],
      id: 'mysql-fixture-006',
      name: 'fixture-mysql-f',
      address: '192.0.2.106:3306',
      buffer_pool_size_bytes: 1024 ** 2,
      buffer_pool_usage_percent: 80,
    },
    {
      ...fixture.data.instances[4],
      id: 'mysql-fixture-007',
      name: 'fixture-mysql-g',
      address: '192.0.2.107:3306',
      buffer_pool_size_bytes: 1024 ** 4,
      buffer_pool_usage_percent: 90,
    },
  )
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()

  for (const [address, value] of [
    ['192.0.2.101:3306', '1 GiB / 50.0%'],
    ['192.0.2.102:3306', '1 B / —'],
    ['192.0.2.103:3306', '— / 25.0%'],
    ['192.0.2.104:3306', '—'],
    ['192.0.2.105:3306', '1 KiB / 75.0%'],
    ['192.0.2.106:3306', '1 MiB / 80.0%'],
    ['192.0.2.107:3306', '1 TiB / 90.0%'],
  ]) {
    const row = (await screen.findByText(address)).closest('tr')
    expect(row).not.toBeNull()
    expect(within(row!).getAllByRole('cell')[6]).toHaveTextContent(value)
  }
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
  const row = (await screen.findByText('192.0.2.101:3306')).closest('tr')
  expect(row).not.toBeNull()
  const connectionCell = within(row!).getAllByRole('cell')[2]
  expect(connectionCell).toHaveTextContent('7')
  expect(connectionCell).not.toHaveTextContent('0')
})

it('renders replication states and explicit collection delay states', async () => {
  const fixture = mysqlInstancePageFixture()
  fixture.data.instances[1] = {
    ...fixture.data.instances[1],
    replication: {
      ...fixture.data.instances[1].replication,
      lag_seconds: 35,
    },
  }
  fixture.data.instances[3] = {
    ...fixture.data.instances[3],
    replication: {
      ...fixture.data.instances[3].replication,
      lag_seconds: 7,
    },
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()
  await screen.findByText('192.0.2.101:3306')
  for (const label of ['正常', '线程异常', '未配置复制', '状态未知']) {
    expect(screen.getAllByText(new RegExp(`^${label}`)).length).toBeGreaterThan(0)
  }
  const stoppedRow = screen.getByText('192.0.2.102:3306').closest('tr')
  const unknownRow = screen.getByText('192.0.2.104:3306').closest('tr')
  expect(stoppedRow).not.toBeNull()
  expect(unknownRow).not.toBeNull()
  expect(within(stoppedRow!).getAllByRole('cell')[7]).toHaveTextContent(
    '线程异常 · 35s',
  )
  expect(within(unknownRow!).getAllByRole('cell')[7]).toHaveTextContent(
    '状态未知 · 7s',
  )
  for (const [label, level] of [
    ['正常', 'normal'],
    ['严重', 'critical'],
    ['采集延迟', 'warning'],
    ['采集失联', 'critical'],
  ] as const) {
    expect(
      screen
        .getAllByText(label, { selector: '.mysql-status' })
        .some((element) => element.dataset.level === level),
    ).toBe(true)
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
    '/mysql?label=%20%20&status=broken&role=admin&sort=sql&order=sideways&page=999&page_size=9',
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
  expect(requestedURLs.every((url) => url.searchParams.has('label'))).toBe(false)
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
  const queryClient = renderMySQLPage()
  expect(await screen.findByText('192.0.2.101:3306')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
  await act(async () => {
    await queryClient.invalidateQueries({ queryKey: ['mysql-instances'] })
  })
  expect(await screen.findByText('MySQL 实例列表刷新失败')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
  expect(screen.getAllByRole('alert')).toHaveLength(2)
  expect(screen.getByText('192.0.2.101:3306')).toBeVisible()
})
