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

import { hostPageFixture } from '../../test/fixtures'
import { HostListPage } from './HostListPage'

const requestedURLs: URL[] = []

const expectedHostHeaders = [
  '主机名',
  'IP 地址',
  'CPU 核数',
  '内存容量',
  'CPU 使用率',
  '内存使用率',
  '负载',
  'IO 忙碌度',
  '网络发送',
  '网络接收',
  '运行时间',
  '状态',
] as const

const expectedHostSorts = [
  ['主机名', 'name'],
  ['IP 地址', 'ip'],
  ['CPU 核数', 'cpu_cores'],
  ['内存容量', 'memory_total'],
  ['CPU 使用率', 'cpu'],
  ['内存使用率', 'memory'],
  ['负载', 'load'],
  ['IO 忙碌度', 'io'],
  ['网络发送', 'network_transmit'],
  ['网络接收', 'network_receive'],
  ['运行时间', 'uptime'],
  ['状态', 'status'],
] as const

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

function renderHostList(initialEntry = '/hosts') {
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
          <Route path="/hosts" element={<HostListPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  )
  return queryClient
}

function lastRequest() {
  const url = requestedURLs.at(-1)
  if (url === undefined) throw new Error('尚未发送主机列表请求')
  return url
}

function expectRequestParameters(
  url: URL,
  expected: Record<string, string>,
) {
  expect(url.pathname).toBe('/api/v1/hosts')
  expect(Object.fromEntries(url.searchParams)).toEqual(expected)
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
        hostPageFixture({
          data: {
            page,
            page_size: pageSize,
            total_pages: Math.ceil(41 / pageSize),
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

it('按固定顺序渲染十二个单值单行列，并使用共享列表壳', async () => {
  renderHostList()

  expect(
    screen.getByRole('searchbox', { name: '搜索主机名或 IP' }),
  ).toBeInTheDocument()
  expect(
    within(screen.getByRole('combobox', { name: '主机状态' }))
      .getAllByRole('option')
      .map((option) => option.textContent),
  ).toEqual(['全部状态', '在线', '离线'])
  expect(
    within(screen.getByRole('combobox', { name: '每页数量' }))
      .getAllByRole('option')
      .map((option) => option.textContent),
  ).toEqual(['20 条', '50 条', '100 条'])

  const appName = await screen.findByText('linux-app-01')

  const dataTime = await screen.findByText('2026/07/21 00:30:00')
  expect(dataTime.closest('.data-time')).toHaveTextContent(
    '最新数据时间：2026/07/21 00:30:00',
  )
  expect(screen.queryByRole('button', { name: /刷新/ })).not.toBeInTheDocument()
  expect(screen.queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument()

  const headers = screen.getAllByRole('columnheader')
  expect(headers.map((header) => header.textContent)).toEqual(
    expectedHostHeaders,
  )
  for (const header of headers) {
    expect(within(header).getByRole('button')).toHaveClass('host-sort-button')
    expect(header.textContent).not.toMatch(/[⇅↑↓]/)
  }

  const table = screen.getByRole('table')
  const scrollOwner = table.closest('.host-table-scroll')
  expect(scrollOwner).not.toBeNull()
  expect(scrollOwner?.parentElement).toHaveClass('host-table-panel')
  expect(scrollOwner?.parentElement?.querySelectorAll('.host-table-scroll')).toHaveLength(1)

  expect(
    screen.queryByRole('link', { name: 'linux-app-01' }),
  ).not.toBeInTheDocument()
  const appRow = appName.closest('tr')
  expect(appRow).not.toBeNull()
  const appCells = within(appRow!).getAllByRole('cell')
  expect(appCells).toHaveLength(12)
  expect(appCells[0]).toHaveTextContent('linux-app-01')
  expect(appCells[1]).toHaveTextContent('192.0.2.11')
  expect(appCells[2]).toHaveTextContent('8 核')
  expect(appCells[3]).toHaveTextContent('32 GiB')
  expect(appCells[7]).toHaveTextContent('91.2%')
  expect(appCells[8]).toHaveTextContent('8.0KiB/s')
  expect(appCells[9]).toHaveTextContent('4.0KiB/s')
  expect(appCells[4].querySelector('[data-level="normal"]')).not.toBeNull()
  expect(appCells[5].querySelector('[data-level="warning"]')).not.toBeNull()
  expect(appCells[7].querySelector('[data-level="critical"]')).not.toBeNull()
  expect(appCells[8].querySelector('[data-level="warning"]')).not.toBeNull()
  expect(appCells[9].querySelector('[data-level="critical"]')).not.toBeNull()
  for (const cell of appCells) {
    expect(cell.querySelector('br')).toBeNull()
  }
  expect(within(appRow!).getByText('在线')).toHaveAttribute(
    'data-level',
    'normal',
  )
  expect(appRow).toHaveTextContent('23.5%')
  expect(appRow).toHaveTextContent('67.0%')
  expect(appRow).toHaveTextContent('1.3')
  expect(appRow).toHaveTextContent('1天 2小时')

  const dbRow = screen.getByText('linux-db-02').closest('tr')
  expect(dbRow).not.toBeNull()
  expect(within(dbRow!).getByText('采集失联')).toHaveAttribute(
    'data-level',
    'critical',
  )
  expect(within(dbRow!).getAllByText('暂无数据')).toHaveLength(8)

  expect(
    screen.queryByRole('button', { name: /重启|删除|执行|修改/ }),
  ).not.toBeInTheDocument()
  expectRequestParameters(lastRequest(), {
    q: '',
    status: '',
    sort: 'name',
    order: 'asc',
    page: '1',
    page_size: '20',
  })
})

it('主机采集延迟使用 warning 等级而不是灰色 unknown', async () => {
  const fixture = hostPageFixture()
  const delayed = {
    ...fixture.data.hosts[0],
    id: 'host-delayed',
    name: 'linux-delayed',
    status: 'unknown' as const,
    collection_level: 'warning' as const,
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(
    jsonResponse(
      hostPageFixture({
        data: {
          hosts: [delayed],
          total: 1,
          page: 1,
          page_size: 20,
          total_pages: 1,
        },
      }),
    ),
  )

  renderHostList()

  expect(await screen.findByText('采集延迟')).toHaveAttribute(
    'data-level',
    'warning',
  )
})

it('长主机名保留完整提示并使用列内截断容器', async () => {
  const longName =
    'production-payment-service-linux-node-with-an-extremely-long-hostname-001'
  const fixture = hostPageFixture()
  fixture.data.hosts[0].name = longName
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))

  renderHostList()

  const hostName = await screen.findByText(longName)
  expect(hostName).toHaveClass('host-name-text')
  expect(hostName).toHaveAttribute('title', longName)
  const row = hostName.closest('tr')
  expect(row).not.toBeNull()
  expect(within(row!).getAllByRole('cell')[1]).toHaveTextContent('192.0.2.11')
})

it('从 URL 恢复列表筛选和排序状态', async () => {
  const initialEntry =
    '/hosts?q=db&status=offline&sort=memory&order=desc&page=1&page_size=50'
  renderHostList(initialEntry)

  expect(
    await screen.findByText('linux-app-01'),
  ).toBeInTheDocument()
  expect(screen.getByRole('searchbox', { name: '搜索主机名或 IP' })).toHaveValue(
    'db',
  )
  expect(screen.getByRole('combobox', { name: '主机状态' })).toHaveValue(
    'offline',
  )
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')
  expectRequestParameters(lastRequest(), {
    q: 'db',
    status: 'offline',
    sort: 'memory',
    order: 'desc',
    page: '1',
    page_size: '50',
  })

  expect(window.location.pathname + window.location.search).toBe(initialEntry)
})

it('搜索等待 300ms 后请求并把页码重置为 1', async () => {
  vi.useFakeTimers()
  renderHostList('/hosts?page=3')

  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(requestedURLs).toHaveLength(1)
  fireEvent.change(
    screen.getByRole('searchbox', { name: '搜索主机名或 IP' }),
    { target: { value: 'linux' } },
  )

  await act(async () => vi.advanceTimersByTimeAsync(299))
  expect(requestedURLs).toHaveLength(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(requestedURLs).toHaveLength(2)
  expectRequestParameters(lastRequest(), {
    q: 'linux',
    status: '',
    sort: 'name',
    order: 'asc',
    page: '1',
    page_size: '20',
  })
  expect(window.location.search).toContain('q=linux')
  expect(window.location.search).toContain('page=1')
})

it('所有主机表头排序写入精确 URL 键、重置页码并可切换降序', async () => {
  const user = userEvent.setup()
  renderHostList('/hosts?sort=status&order=asc&page=3')
  await screen.findByText('linux-app-01')

  for (const [label, field] of expectedHostSorts) {
    const button = screen.getByRole('button', {
      name: new RegExp(`^${label}排序，当前`),
    })
    await user.click(button)
    await waitFor(() =>
      expect(lastRequest().searchParams.get('sort')).toBe(field),
    )
    expect(lastRequest().searchParams.get('order')).toBe('asc')
    expect(lastRequest().searchParams.get('page')).toBe('1')
    expect(window.location.search).toContain(`sort=${field}`)
    expect(window.location.search).toContain('page=1')

    await user.click(
      screen.getByRole('button', {
        name: new RegExp(`^${label}排序，当前升序$`),
      }),
    )
    await waitFor(() =>
      expect(lastRequest().searchParams.get('order')).toBe('desc'),
    )
  }
})

it('使用服务端分页并保持当前每页数量', async () => {
  const user = userEvent.setup()
  renderHostList('/hosts?page=2')

  expect(await screen.findByText('第 2 / 3 页，共 41 台')).toBeInTheDocument()
  expect(lastRequest().searchParams.get('page_size')).toBe('20')

  await user.click(screen.getByRole('button', { name: '下一页' }))
  await waitFor(() => expect(lastRequest().searchParams.get('page')).toBe('3'))
  expect(lastRequest().searchParams.get('page_size')).toBe('20')
  expect(window.location.search).toContain('page=3')

  await user.click(screen.getByRole('button', { name: '上一页' }))
  await waitFor(() => expect(lastRequest().searchParams.get('page')).toBe('2'))
  expect(lastRequest().searchParams.get('page_size')).toBe('20')
})

it('切换每页数量会回到第一页并写入 URL', async () => {
  const user = userEvent.setup()
  renderHostList('/hosts?page=2&page_size=20')

  expect(await screen.findByText('第 2 / 3 页，共 41 台')).toBeInTheDocument()
  await user.selectOptions(
    screen.getByRole('combobox', { name: '每页数量' }),
    '50',
  )

  await waitFor(() => {
    expect(lastRequest().searchParams.get('page')).toBe('1')
    expect(lastRequest().searchParams.get('page_size')).toBe('50')
  })
  expect(window.location.search).toContain('page=1')
  expect(window.location.search).toContain('page_size=50')
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')
  expect(await screen.findByText('第 1 / 1 页，共 41 台')).toBeInTheDocument()
})

it('把超出服务端总页数的 URL 替换为末页并直接重新请求', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const page = Number(url.searchParams.get('page') ?? '1')
    if (page === 999) {
      return Promise.resolve(
        jsonResponse(
          hostPageFixture({
            data: { hosts: [], page: 999, total_pages: 3 },
          }),
        ),
      )
    }
    return Promise.resolve(jsonResponse(hostPageFixture({ data: { page } })))
  })

  const historyLength = window.history.length
  renderHostList('/hosts?page=999')

  expect(await screen.findByText('第 3 / 3 页，共 41 台')).toBeInTheDocument()
  expect(requestedURLs.map((url) => url.searchParams.get('page'))).toEqual([
    '999',
    '3',
  ])
  expect(window.location.search).toContain('page=3')
  expect(window.history.length).toBe(historyLength)
  expect(screen.queryByText(/第 999 \/ 3 页/)).not.toBeInTheDocument()
})

it('零结果时规范为第一页并显示空状态而不是 1/0 页', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const page = Number(url.searchParams.get('page') ?? '1')
    return Promise.resolve(
      jsonResponse(
        hostPageFixture({
          data: { hosts: [], total: 0, page, total_pages: 0 },
        }),
      ),
    )
  })

  const historyLength = window.history.length
  renderHostList('/hosts?page=4')

  expect(await screen.findByText('没有符合条件的主机')).toBeInTheDocument()
  expect(requestedURLs.map((url) => url.searchParams.get('page'))).toEqual([
    '4',
    '1',
  ])
  expect(window.location.search).toContain('page=1')
  expect(window.history.length).toBe(historyLength)
  expect(screen.queryByText(/第 1 \/ 0 页/)).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
})

it('用 replace 规范非法 URL 参数并保留搜索词', async () => {
  const historyLength = window.history.length
  renderHostList(
    '/hosts?q=keep&status=broken&sort=password&order=sideways&page=nope&page_size=999',
  )

  await screen.findByText('linux-app-01')
  await waitFor(() =>
    expect(window.location.search).toBe(
      '?q=keep&status=&sort=name&order=asc&page=1&page_size=20',
    ),
  )
  expect(window.history.length).toBe(historyLength)
  expectRequestParameters(lastRequest(), {
    q: 'keep',
    status: '',
    sort: 'name',
    order: 'asc',
    page: '1',
    page_size: '20',
  })
})

it('连续快速输入只在最后一次变更 300ms 后请求最终搜索词', async () => {
  vi.useFakeTimers()
  renderHostList()
  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(requestedURLs).toHaveLength(1)

  const search = screen.getByRole('searchbox', { name: '搜索主机名或 IP' })
  fireEvent.change(search, { target: { value: 'a' } })
  await act(async () => vi.advanceTimersByTimeAsync(100))
  fireEvent.change(search, { target: { value: 'ab' } })
  await act(async () => vi.advanceTimersByTimeAsync(100))
  fireEvent.change(search, { target: { value: 'abc' } })

  await act(async () => vi.advanceTimersByTimeAsync(299))
  expect(requestedURLs).toHaveLength(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))

  expect(requestedURLs).toHaveLength(2)
  expect(lastRequest().searchParams.get('q')).toBe('abc')
  expect(window.location.search).toContain('q=abc')
})

it('每 15 秒非重叠自动刷新主机列表', async () => {
  vi.useFakeTimers()
  let resolveInitial!: (response: Response) => void
  vi.mocked(globalThis.fetch).mockImplementationOnce(
    (input) => {
      requestedURLs.push(requestURL(input))
      return new Promise<Response>((resolve) => {
        resolveInitial = resolve
      })
    },
  )
  renderHostList()

  await act(async () => vi.advanceTimersByTimeAsync(60_000))
  expect(requestedURLs).toHaveLength(1)

  await act(async () => {
    resolveInitial(jsonResponse(hostPageFixture()))
    await vi.advanceTimersByTimeAsync(0)
  })
  expect(screen.getByText('linux-app-01')).toBeInTheDocument()

  await act(async () => vi.advanceTimersByTimeAsync(14_999))
  expect(requestedURLs).toHaveLength(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(requestedURLs).toHaveLength(2)
})

it('后台刷新失败时保留已有表格并显示可重试提示', async () => {
  vi.mocked(globalThis.fetch)
    .mockResolvedValueOnce(jsonResponse(hostPageFixture()))
    .mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'datasource_unavailable',
          message: '数据源暂时不可用，请稍后重试',
          request_id: 'req-host-list-refresh-failed-001',
          retryable: true,
        },
        503,
      ),
    )
    .mockResolvedValueOnce(jsonResponse(hostPageFixture()))
  const user = userEvent.setup()
  const queryClient = renderHostList()

  await screen.findByText('linux-app-01')
  await act(async () => {
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
  })

  const error = await screen.findByRole('alert')
  expect(error).toHaveTextContent('主机列表刷新失败')
  expect(error).toHaveTextContent('数据源暂时不可用，请稍后重试')
  expect(screen.getByText('linux-app-01')).toBeInTheDocument()
  expect(screen.getByText('第 1 / 3 页，共 41 台')).toBeInTheDocument()

  await user.click(
    within(error).getByRole('button', { name: '重试主机列表' }),
  )
  await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
})
