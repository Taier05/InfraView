import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  BrowserRouter,
  Route,
  Routes,
  useNavigate,
} from 'react-router-dom'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import {
  hostDetailFixture,
  hostMetricsFixture,
} from '../../test/fixtures'
import { HostDetailPage } from './HostDetailPage'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestURL(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost')
}

function isHistoryRequest(input: RequestInfo | URL) {
  return requestURL(input).pathname.endsWith('/metrics')
}

function NavigationHarness() {
  const navigate = useNavigate()
  return (
    <button type="button" onClick={() => navigate('/hosts/host-002')}>
      测试切换主机
    </button>
  )
}

function renderHostDetail(initialEntry = '/hosts/host-001') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })
  window.history.replaceState({}, '', initialEntry)

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route
            path="/hosts/:id"
            element={
              <>
                <HostDetailPage />
                <NavigationHarness />
              </>
            }
          />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  window.history.replaceState({}, '', '/')
  vi.spyOn(globalThis, 'fetch').mockImplementation((input) =>
    Promise.resolve(
      jsonResponse(
        isHistoryRequest(input)
          ? hostMetricsFixture()
          : hostDetailFixture(),
      ),
    ),
  )
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('按获批顺序展示身份、全部当前指标、文件系统和三组历史趋势', async () => {
  renderHostDetail()

  expect(
    await screen.findByRole('heading', { name: 'linux-app-01' }),
  ).toBeInTheDocument()
  expect(screen.getByText('192.0.2.11')).toBeInTheDocument()
  expect(screen.getByText('Ubuntu 24.04')).toBeInTheDocument()
  expect(screen.getByText('在线')).toBeInTheDocument()
  expect(screen.getByText('1天 2小时')).toBeInTheDocument()

  const expectedCards = [
    ['CPU 使用率', '23.5%'],
    ['内存使用率', '67%'],
    ['1 分钟负载', '1.3'],
    ['磁盘读取速率', '1.5 MiB/s'],
    ['磁盘写入速率', '768 KiB/s'],
    ['网络接收速率', '2 MiB/s'],
    ['网络发送速率', '512 KiB/s'],
  ] as const
  for (const [label, value] of expectedCards) {
    expect(
      within(screen.getByRole('article', { name: label })).getByText(value),
    ).toBeInTheDocument()
  }

  const filesystemTable = screen.getByRole('table', {
    name: '文件系统容量',
  })
  expect(within(filesystemTable).getByText('/')).toBeInTheDocument()
  expect(within(filesystemTable).getByText('/data')).toBeInTheDocument()
  expect(within(filesystemTable).getByText('51.2%')).toBeInTheDocument()
  expect(within(filesystemTable).getByText('88.4%')).toBeInTheDocument()
  expect(within(filesystemTable).getByText('危险')).toBeInTheDocument()

  expect(
    screen.getByText(
      'CPU 使用率趋势：最低 21%，最高 43%，最近值 43%。内存使用率趋势：最低 58%，最高 67%，最近值 67%。1 分钟负载趋势：最低 0.8，最高 1.3，最近值 1.3。',
    ),
  ).toHaveClass('sr-only')
  expect(
    screen.getByText(
      '磁盘读取速率趋势：最低 1 MiB/s，最高 2 MiB/s，最近值 2 MiB/s。磁盘写入速率趋势：最低 512 KiB/s，最高 1 MiB/s，最近值 1 MiB/s。',
    ),
  ).toHaveClass('sr-only')
  expect(
    screen.getByText(
      '网络接收速率趋势：最低 2 MiB/s，最高 4 MiB/s，最近值 4 MiB/s。网络发送速率趋势：最低 1 MiB/s，最高 3 MiB/s，最近值 3 MiB/s。',
    ),
  ).toHaveClass('sr-only')

  const orderedSections = [
    screen.getByRole('heading', { name: '当前指标' }),
    screen.getByRole('heading', { name: 'CPU、内存与负载趋势' }),
    screen.getByRole('heading', { name: '文件系统容量' }),
    screen.getByRole('heading', { name: '磁盘 I/O 趋势' }),
    screen.getByRole('heading', { name: '网络流量趋势' }),
  ]
  for (let index = 1; index < orderedSections.length; index += 1) {
    expect(
      orderedSections[index - 1].compareDocumentPosition(
        orderedSections[index],
      ) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  }
  expect(
    screen.queryByRole('heading', {
      name: /进程|服务|命令|配置|告警|操作/,
    }),
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /重启|删除|执行|修改/ }),
  ).not.toBeInTheDocument()
})

it('默认请求 24 小时并支持切换全部四个历史范围', async () => {
  const ranges: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (isHistoryRequest(input)) {
      const range = requestURL(input).searchParams.get('range') ?? ''
      ranges.push(range)
      return Promise.resolve(
        jsonResponse(
          hostMetricsFixture({
            data: {
              range: range as '1h' | '6h' | '24h' | '7d',
            },
          }),
        ),
      )
    }
    return Promise.resolve(jsonResponse(hostDetailFixture()))
  })
  const user = userEvent.setup()
  renderHostDetail()

  const oneHour = screen.getByRole('button', { name: '1小时' })
  const sixHours = screen.getByRole('button', { name: '6小时' })
  const day = screen.getByRole('button', { name: '24小时' })
  const week = screen.getByRole('button', { name: '7天' })
  expect(day).toHaveAttribute('aria-pressed', 'true')
  await waitFor(() => expect(ranges).toEqual(['24h']))

  for (const [button, range] of [
    [oneHour, '1h'],
    [sixHours, '6h'],
    [week, '7d'],
    [day, '24h'],
  ] as const) {
    await user.click(button)
    await waitFor(() => expect(ranges.at(-1)).toBe(range))
    expect(button).toHaveAttribute('aria-pressed', 'true')
  }
})

it('每张当前指标卡和文件系统分别把 null 显示为暂无数据', async () => {
  const missing = { value: null, level: 'unknown' as const }
  vi.mocked(globalThis.fetch).mockImplementation((input) =>
    Promise.resolve(
      jsonResponse(
        isHistoryRequest(input)
          ? hostMetricsFixture()
          : hostDetailFixture({
              data: {
                metrics: {
                  timestamp: '2026-07-21T00:30:00.000Z',
                  cpu_usage: missing,
                  memory_usage: missing,
                  load_1: missing,
                  disk_read_bytes_per_second: missing,
                  disk_write_bytes_per_second: missing,
                  network_receive_bytes_per_second: missing,
                  network_transmit_bytes_per_second: missing,
                  filesystems: [{ mountpoint: '/', usage: missing }],
                },
              },
            }),
      ),
    ),
  )
  renderHostDetail()

  for (const label of [
    'CPU 使用率',
    '内存使用率',
    '1 分钟负载',
    '磁盘读取速率',
    '磁盘写入速率',
    '网络接收速率',
    '网络发送速率',
  ]) {
    const card = await screen.findByRole('article', { name: label })
    expect(within(card).getByText('暂无数据')).toBeInTheDocument()
    expect(within(card).queryByText(/^0/)).not.toBeInTheDocument()
  }
  const filesystem = screen.getByRole('table', { name: '文件系统容量' })
  expect(within(filesystem).getByText('暂无数据')).toBeInTheDocument()
})

it('当前与历史过期数据分别显示服务端采集时间', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) =>
    Promise.resolve(
      jsonResponse(
        isHistoryRequest(input)
          ? hostMetricsFixture({
              meta: {
                stale: true,
                collected_at: '2026-07-21T00:20:00.000Z',
              },
            })
          : hostDetailFixture({
              meta: {
                stale: true,
                collected_at: '2026-07-21T00:25:00.000Z',
              },
            }),
      ),
    ),
  )
  renderHostDetail()

  expect(await screen.findAllByText('数据已过期')).toHaveLength(2)
  expect(screen.getByText('2026-07-21T00:25:00.000Z')).toBeInTheDocument()
  expect(screen.getByText('2026-07-21T00:20:00.000Z')).toBeInTheDocument()
})

it('历史查询失败不隐藏当前数据并可独立重试成功', async () => {
  let historyAttempts = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (!isHistoryRequest(input)) {
      return Promise.resolve(jsonResponse(hostDetailFixture()))
    }
    historyAttempts += 1
    if (historyAttempts === 1) {
      return Promise.resolve(
        jsonResponse(
          {
            code: 'datasource_unavailable',
            message: '历史指标暂时不可用',
            request_id: 'req-history-error-001',
            retryable: true,
          },
          503,
        ),
      )
    }
    return Promise.resolve(jsonResponse(hostMetricsFixture()))
  })
  const user = userEvent.setup()
  renderHostDetail()

  expect(
    await screen.findByRole('article', { name: 'CPU 使用率' }),
  ).toHaveTextContent('23.5%')
  const error = await screen.findByRole('alert')
  expect(error).toHaveTextContent('历史指标暂时不可用')
  await user.click(within(error).getByRole('button', { name: '重试' }))

  expect(
    await screen.findByRole('heading', { name: '磁盘 I/O 趋势' }),
  ).toBeInTheDocument()
  expect(historyAttempts).toBe(2)
})

it('当前查询失败不隐藏成功的历史趋势', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) =>
    Promise.resolve(
      isHistoryRequest(input)
        ? jsonResponse(hostMetricsFixture())
        : jsonResponse(
            {
              code: 'datasource_unavailable',
              message: '当前指标暂时不可用',
              request_id: 'req-current-error-001',
              retryable: true,
            },
            503,
          ),
    ),
  )
  renderHostDetail()

  expect(await screen.findByRole('alert')).toHaveTextContent(
    '当前指标暂时不可用',
  )
  expect(
    screen.getByRole('heading', { name: 'CPU、内存与负载趋势' }),
  ).toBeInTheDocument()
  expect(
    screen.getByText(/CPU 使用率趋势：最低 21%/),
  ).toBeInTheDocument()
})

it('主机不存在时显示后端提供的中文消息', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValue(
    jsonResponse(
      {
        code: 'host_not_found',
        message: '该主机当前不在数据源中',
        request_id: 'req-host-missing-001',
        retryable: false,
      },
      404,
    ),
  )
  renderHostDetail('/hosts/missing-host')

  expect(
    (await screen.findAllByText('该主机当前不在数据源中')).length,
  ).toBeGreaterThan(0)
  expect(screen.queryByRole('button', { name: '重试' })).not.toBeInTheDocument()
})

it('当前数据每 30 秒刷新且历史数据每 60 秒刷新', async () => {
  vi.useFakeTimers()
  let currentRequests = 0
  let historyRequests = 0
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    if (isHistoryRequest(input)) {
      historyRequests += 1
      return Promise.resolve(jsonResponse(hostMetricsFixture()))
    }
    currentRequests += 1
    return Promise.resolve(jsonResponse(hostDetailFixture()))
  })
  renderHostDetail()

  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(currentRequests).toBe(1)
  expect(historyRequests).toBe(1)
  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(currentRequests).toBe(2)
  expect(historyRequests).toBe(1)
  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(currentRequests).toBe(3)
  expect(historyRequests).toBe(2)
})

it('范围、主机 ID 变化和卸载都会取消失去观察者的请求', async () => {
  const aborted: string[] = []
  vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
    const url = requestURL(input)
    const label = `${url.pathname}?${url.searchParams.toString()}`
    return new Promise<Response>((resolve) => {
      init?.signal?.addEventListener(
        'abort',
        () => {
          aborted.push(label)
          resolve(
            jsonResponse(
              isHistoryRequest(input)
                ? hostMetricsFixture()
                : hostDetailFixture(),
            ),
          )
        },
        { once: true },
      )
    })
  })
  const user = userEvent.setup()
  const view = renderHostDetail()

  await user.click(screen.getByRole('button', { name: '1小时' }))
  await waitFor(() =>
    expect(aborted).toContain('/api/v1/hosts/host-001/metrics?range=24h'),
  )
  await user.click(screen.getByRole('button', { name: '测试切换主机' }))
  await waitFor(() => {
    expect(aborted).toContain('/api/v1/hosts/host-001?')
    expect(aborted).toContain('/api/v1/hosts/host-001/metrics?range=1h')
  })
  view.unmount()
  await waitFor(() => {
    expect(aborted).toContain('/api/v1/hosts/host-002?')
    expect(aborted).toContain('/api/v1/hosts/host-002/metrics?range=1h')
  })
})
