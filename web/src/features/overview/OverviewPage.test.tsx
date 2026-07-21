import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest'

import { TrendChart } from '../../components/TrendChart'
import { overviewFixture } from '../../test/fixtures'
import { OverviewPage } from './OverviewPage'

function renderOverview() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <OverviewPage />
    </QueryClientProvider>,
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

it('展示四张主指标卡以及不依赖颜色的状态文字', async () => {
  renderOverview()

  const cards = await screen.findAllByRole('article')
  expect(cards).toHaveLength(4)

  expect(
    within(screen.getByRole('article', { name: '主机总数' })).getByText('12'),
  ).toBeInTheDocument()
  expect(
    within(screen.getByRole('article', { name: '在线主机' })).getByText('9'),
  ).toBeInTheDocument()

  const totalCard = screen.getByRole('article', { name: '主机总数' })
  expect(within(totalCard).getByText('在线 9')).toBeInTheDocument()
  expect(within(totalCard).getByText('离线 2')).toBeInTheDocument()
  expect(within(totalCard).getByText('未知 1')).toBeInTheDocument()

  const cpuCard = screen.getByRole('article', { name: 'CPU 平均使用率' })
  expect(cpuCard).toHaveTextContent('73.5%')
  expect(within(cpuCard).getByText('危险')).toBeInTheDocument()
  expect(cpuCard).toHaveAttribute('data-level', 'critical')

  const memoryCard = screen.getByRole('article', {
    name: '内存平均使用率',
  })
  expect(memoryCard).toHaveTextContent('42%')
  expect(within(memoryCard).getByText('警告')).toBeInTheDocument()
  expect(memoryCard).toHaveAttribute('data-level', 'warning')
})

it('默认请求 24 小时并提供全部四个范围', async () => {
  const requestedRanges: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    requestedRanges.push(requestedRange(input))
    return Promise.resolve(jsonResponse(overviewFixture()))
  })
  const user = userEvent.setup()
  renderOverview()

  const oneHour = screen.getByRole('button', { name: '1小时' })
  const sixHours = screen.getByRole('button', { name: '6小时' })
  const day = screen.getByRole('button', { name: '24小时' })
  const week = screen.getByRole('button', { name: '7天' })
  expect(day).toHaveAttribute('aria-pressed', 'true')
  expect(oneHour).toHaveAttribute('aria-pressed', 'false')
  await waitFor(() => expect(requestedRanges).toEqual(['24h']))

  for (const [button, range] of [
    [oneHour, '1h'],
    [sixHours, '6h'],
    [week, '7d'],
    [day, '24h'],
  ] as const) {
    await user.click(button)
    await waitFor(() => expect(requestedRanges.at(-1)).toBe(range))
    expect(button).toHaveAttribute('aria-pressed', 'true')
  }
})

it('渲染服务端聚合趋势并在切换范围后展示对应序列摘要', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const range = requestedRange(input)
    const recentCPU = range === '7d' ? 77 : 52
    const recentMemory = range === '7d' ? 68 : 61
    return Promise.resolve(
      jsonResponse(
        overviewFixture({
          data: {
            trends: [
              {
                key: 'cpu_usage',
                unit: '%',
                points: [
                  { timestamp: '2026-07-14T00:30:00.000Z', value: 31 },
                  {
                    timestamp: '2026-07-21T00:30:00.000Z',
                    value: recentCPU,
                  },
                ],
              },
              {
                key: 'memory_usage',
                unit: '%',
                points: [
                  { timestamp: '2026-07-14T00:30:00.000Z', value: 45 },
                  {
                    timestamp: '2026-07-21T00:30:00.000Z',
                    value: recentMemory,
                  },
                ],
              },
            ],
          },
        }),
      ),
    )
  })
  const user = userEvent.setup()
  renderOverview()

  expect(
    await screen.findByRole('heading', { name: '资源使用趋势' }),
  ).toBeInTheDocument()
  expect(
    screen.getByText(
      'CPU 使用率趋势：最低 31%，最高 52%，最近值 52%。内存使用率趋势：最低 45%，最高 61%，最近值 61%。',
    ),
  ).toHaveClass('sr-only')

  await user.click(screen.getByRole('button', { name: '7天' }))
  expect(
    await screen.findByText(
      'CPU 使用率趋势：最低 31%，最高 77%，最近值 77%。内存使用率趋势：最低 45%，最高 68%，最近值 68%。',
    ),
  ).toHaveClass('sr-only')
})

it('手动刷新会重新请求当前范围', async () => {
  const user = userEvent.setup()
  renderOverview()

  await screen.findByRole('article', { name: '主机总数' })
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: '刷新' }))

  await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(2))
  expect(requestedRange(vi.mocked(globalThis.fetch).mock.calls[1][0])).toBe(
    '24h',
  )
})

it('切换范围时取消已经失去观察者的旧请求', async () => {
  const requestedRanges: string[] = []
  const abortedRanges: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
    const range = requestedRange(input)
    requestedRanges.push(range)
    if (range === '24h' || range === '6h') {
      return Promise.resolve(jsonResponse(overviewFixture()))
    }

    return new Promise<Response>((resolve) => {
      init?.signal?.addEventListener(
        'abort',
        () => {
          abortedRanges.push(range)
          resolve(jsonResponse(overviewFixture()))
        },
        { once: true },
      )
    })
  })
  const user = userEvent.setup()
  renderOverview()

  await screen.findByRole('article', { name: 'CPU 平均使用率' })
  await user.click(screen.getByRole('button', { name: '1小时' }))
  await waitFor(() => expect(requestedRanges).toContain('1h'))
  await user.click(screen.getByRole('button', { name: '6小时' }))

  await waitFor(() => expect(abortedRanges).toContain('1h'))
  await waitFor(() => expect(requestedRanges).toContain('6h'))
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

it('指标值为 null 时显示暂无数据而不是零', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValue(
    jsonResponse(
      overviewFixture({
        data: {
          cpu_average: { value: null, level: 'unknown' },
        },
      }),
    ),
  )
  renderOverview()

  const cpuCard = await screen.findByRole('article', {
    name: 'CPU 平均使用率',
  })
  expect(within(cpuCard).getByText('暂无数据')).toBeInTheDocument()
  expect(within(cpuCard).getByText('未知')).toBeInTheDocument()
  expect(within(cpuCard).queryByText('0%')).not.toBeInTheDocument()
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
  await user.click(screen.getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('article', { name: 'CPU 平均使用率' }),
  ).toHaveTextContent('73.5%')
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

describe('TrendChart', () => {
  it('为图表提供无需观察画布即可理解的读屏摘要', () => {
    render(
      <TrendChart
        title="CPU 使用率趋势"
        summary="CPU 使用率趋势：最低 32%，最高 61%，最近值 48%。"
        series={[
          {
            name: 'CPU 使用率',
            points: [
              { timestamp: '2026-07-21T00:00:00.000Z', value: 32 },
              { timestamp: '2026-07-21T00:30:00.000Z', value: 48 },
            ],
          },
        ]}
      />,
    )

    expect(
      screen.getByRole('heading', { name: 'CPU 使用率趋势' }),
    ).toBeInTheDocument()
    expect(
      screen.getByText('CPU 使用率趋势：最低 32%，最高 61%，最近值 48%。'),
    ).toHaveClass('sr-only')
  })
})
