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

import { overviewFixture } from '../../test/fixtures'
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

beforeEach(() => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    jsonResponse(overviewFixture()),
  )
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('总览只展示可进入主机板块的告警摘要卡', async () => {
  renderOverview()

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
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
  const controls = screen.getByRole('group', { name: '总览控制' })
  expect(
    within(controls).getByText(/上次刷新 \d{2}:\d{2}:\d{2}/),
  ).toBeInTheDocument()
  expect(within(controls).getByText(/每 30 秒自动刷新/)).toBeInTheDocument()
})

it('使用固定查询范围且不显示总览时间范围控件', async () => {
  const requestedRanges: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
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

it('手动刷新会重新请求当前范围', async () => {
  const user = userEvent.setup()
  renderOverview()

  await screen.findByRole('link', { name: '查看 Linux 主机板块' })
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: '刷新' }))

  await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(2))
  expect(requestedRange(vi.mocked(globalThis.fetch).mock.calls[1][0])).toBe(
    '24h',
  )
})

it('每 30 秒刷新且前一请求未完成时不发起重叠请求', async () => {
  vi.useFakeTimers()
  let requestCount = 0
  let resolveRefresh!: (response: Response) => void
  vi.mocked(globalThis.fetch).mockImplementation(() => {
    requestCount += 1
    if (requestCount === 1) {
      return Promise.resolve(jsonResponse(overviewFixture()))
    }
    return new Promise<Response>((resolve) => {
      resolveRefresh = resolve
    })
  })

  renderOverview()
  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(requestCount).toBe(1)

  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(requestCount).toBe(2)

  await act(async () => vi.advanceTimersByTimeAsync(60_000))
  expect(requestCount).toBe(2)

  await act(async () => {
    resolveRefresh(jsonResponse(overviewFixture()))
    await vi.advanceTimersByTimeAsync(0)
  })
  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(requestCount).toBe(3)
})

it('过期数据提示显示服务端给出的精确采集时间', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValue(
    jsonResponse(
      overviewFixture({
        meta: {
          stale: true,
          collected_at: '2026-07-21T00:30:00.000Z',
        },
      }),
    ),
  )
  renderOverview()

  const banner = await screen.findByRole('alert')
  expect(banner).toHaveTextContent('数据已过期')
  expect(banner).toHaveTextContent('2026-07-21T00:30:00.000Z')
  expect(within(banner).getByRole('time')).toHaveAttribute(
    'dateTime',
    '2026-07-21T00:30:00.000Z',
  )
})

it('可重试错误显示中文信息并可重试成功', async () => {
  let attempts = 0
  vi.mocked(globalThis.fetch).mockImplementation(() => {
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
  vi.mocked(globalThis.fetch).mockResolvedValue(
    jsonResponse(
      {
        code: 'invalid_range',
        message: '时间范围无效',
        request_id: 'req-overview-error-002',
        retryable: false,
      },
      400,
    ),
  )
  renderOverview()

  expect(await screen.findByRole('alert')).toHaveTextContent('时间范围无效')
  expect(screen.queryByRole('button', { name: '重试' })).not.toBeInTheDocument()
})
