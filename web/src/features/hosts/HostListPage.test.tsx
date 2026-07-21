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
import {
  BrowserRouter,
  Route,
  Routes,
  useNavigate,
  useParams,
} from 'react-router-dom'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { hostPageFixture } from '../../test/fixtures'
import { HostListPage } from './HostListPage'

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

function DetailStub() {
  const navigate = useNavigate()
  const { id } = useParams()
  return (
    <section>
      <h1>主机详情 {id}</h1>
      <button type="button" onClick={() => navigate(-1)}>
        返回主机列表
      </button>
    </section>
  )
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
          <Route path="/hosts/:id" element={<DetailStub />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  )
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
    return Promise.resolve(
      jsonResponse(hostPageFixture({ data: { page } })),
    )
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('按真实 hosts schema 渲染筛选器、可排序列、状态和空指标', async () => {
  renderHostList()

  expect(
    screen.getByRole('searchbox', { name: '搜索主机名或 IP' }),
  ).toBeInTheDocument()
  expect(
    screen.getAllByRole('option').map((option) => option.textContent),
  ).toEqual(['全部状态', '在线', '离线'])

  const appLink = await screen.findByRole('link', { name: 'linux-app-01' })

  for (const label of ['主机', 'CPU', '内存', '负载', '运行时间']) {
    expect(
      screen.getByRole('button', { name: new RegExp(`^${label}`) }),
    ).toBeInTheDocument()
  }

  expect(appLink).toHaveAttribute('href', '/hosts/host-001')
  const appRow = appLink.closest('tr')
  expect(appRow).not.toBeNull()
  expect(within(appRow!).getByText('在线')).toHaveAttribute(
    'data-status',
    'online',
  )
  expect(appRow).toHaveTextContent('23.5%')
  expect(appRow).toHaveTextContent('67.0%')
  expect(appRow).toHaveTextContent('1.3')
  expect(appRow).toHaveTextContent('1天 2小时')

  const dbRow = screen.getByRole('link', { name: 'linux-db-02' }).closest('tr')
  expect(dbRow).not.toBeNull()
  expect(within(dbRow!).getByText('离线')).toHaveAttribute(
    'data-status',
    'offline',
  )
  expect(within(dbRow!).getAllByText('暂无数据')).toHaveLength(3)

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

it('从 URL 恢复列表状态并在详情返回后完整保留', async () => {
  const user = userEvent.setup()
  const initialEntry =
    '/hosts?q=db&status=offline&sort=memory&order=desc&page=2'
  renderHostList(initialEntry)

  expect(
    await screen.findByRole('link', { name: 'linux-app-01' }),
  ).toBeInTheDocument()
  expect(screen.getByRole('searchbox', { name: '搜索主机名或 IP' })).toHaveValue(
    'db',
  )
  expect(screen.getByRole('combobox', { name: '主机状态' })).toHaveValue(
    'offline',
  )
  expectRequestParameters(lastRequest(), {
    q: 'db',
    status: 'offline',
    sort: 'memory',
    order: 'desc',
    page: '2',
    page_size: '20',
  })

  await user.click(screen.getByRole('link', { name: 'linux-app-01' }))
  expect(window.location.pathname).toBe('/hosts/host-001')
  await user.click(screen.getByRole('button', { name: '返回主机列表' }))

  expect(window.location.pathname + window.location.search).toBe(initialEntry)
  expect(
    await screen.findByRole('searchbox', { name: '搜索主机名或 IP' }),
  ).toHaveValue('db')
  expect(screen.getByRole('combobox', { name: '主机状态' })).toHaveValue(
    'offline',
  )
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

it('筛选和服务端排序变化都重置页码并使用规范 load 字段', async () => {
  const user = userEvent.setup()
  renderHostList('/hosts?page=3')
  await screen.findByRole('link', { name: 'linux-app-01' })

  await user.selectOptions(
    screen.getByRole('combobox', { name: '主机状态' }),
    'offline',
  )
  await waitFor(() =>
    expect(lastRequest().searchParams.get('status')).toBe('offline'),
  )
  expect(lastRequest().searchParams.get('page')).toBe('1')

  await user.click(screen.getByRole('button', { name: /^CPU/ }))
  await waitFor(() =>
    expect(lastRequest().searchParams.get('sort')).toBe('cpu'),
  )
  expect(lastRequest().searchParams.get('order')).toBe('asc')
  expect(lastRequest().searchParams.get('page')).toBe('1')
  expect(lastRequest().searchParams.get('status')).toBe('offline')
  expect(window.location.search).toContain('status=offline')

  await user.click(screen.getByRole('button', { name: /^CPU/ }))
  await waitFor(() =>
    expect(lastRequest().searchParams.get('order')).toBe('desc'),
  )

  await user.click(screen.getByRole('button', { name: /^负载/ }))
  await waitFor(() =>
    expect(lastRequest().searchParams.get('sort')).toBe('load'),
  )
  expect(lastRequest().searchParams.get('order')).toBe('asc')
  expect(window.location.search).not.toContain('load_1')
})

it('使用服务端分页且每页固定请求 20 条', async () => {
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
    '/hosts?q=keep&status=broken&sort=password&order=sideways&page=nope',
  )

  await screen.findByRole('link', { name: 'linux-app-01' })
  await waitFor(() =>
    expect(window.location.search).toBe(
      '?q=keep&status=&sort=name&order=asc&page=1',
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
