import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within } from '@testing-library/react'
import {
  MemoryRouter,
  Route,
  Routes,
  useOutletContext,
} from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { AppShell } from './AppShell'

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({
    logout: vi.fn(),
    username: 'admin',
  }),
}))

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function datasourceFixture({
  healthy = true,
  stale = false,
  type = 'nightingale',
  refreshIntervalSeconds = 15,
}: {
  healthy?: boolean
  stale?: boolean
  type?: 'mock' | 'nightingale'
  refreshIntervalSeconds?: number
} = {}) {
  return {
    data: {
      type,
      healthy,
      checked_at: '2026-07-22T02:03:04.000Z',
      refresh_interval_seconds: refreshIntervalSeconds,
    },
    meta: {
      request_id: 'req-datasource-001',
      stale,
      collected_at: '2026-07-22T02:03:05.000Z',
    },
  }
}

function RuntimePage() {
  const context = useOutletContext<{ refreshIntervalMs: number } | null>()
  return <p>页面刷新周期 {context?.refreshIntervalMs ?? 0} 毫秒</p>
}

function renderShell() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<RuntimePage />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    jsonResponse(datasourceFixture()),
  )
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('默认使用紧凑布局密度', () => {
  renderShell()

  expect(document.querySelector('.app-shell')).toHaveAttribute(
    'data-density',
    'compact',
  )
  expect(document.querySelector('.workspace')).toHaveAttribute(
    'data-density',
    'dense',
  )
})

it('健康连接默认显示紧凑汇总并可展开 Nightingale 详情', async () => {
  const user = userEvent.setup()
  renderShell()

  const status = screen.getByLabelText('数据连接汇总')
  expect(await within(status).findByText('1/1 正常')).toBeInTheDocument()
  expect(within(status).getByText('数据连接')).toBeInTheDocument()
  expect(status).not.toHaveAttribute('open')

  await user.click(within(status).getByText('数据连接'))

  expect(status).toHaveAttribute('open')
  expect(within(status).getByText('指标')).toBeInTheDocument()
  expect(within(status).getByText('Nightingale')).toBeInTheDocument()
  expect(within(status).getByText('健康')).toBeInTheDocument()
  expect(within(status).getByText('最近检查')).toBeInTheDocument()
  expect(screen.getByText('页面刷新周期 15000 毫秒')).toBeInTheDocument()
  expect(within(status).getByRole('time')).toHaveAttribute(
    'datetime',
    '2026-07-22T02:03:04.000Z',
  )
})

it('按总览主机硬盘MySQLRedis顺序展示只读导航', async () => {
  renderShell()

  const navigation = screen.getByRole('navigation', { name: '主导航' })
  const links = within(navigation).getAllByRole('link')
  expect(links.map((link) => link.textContent)).toEqual([
    '总览',
    '主机',
    '硬盘',
    'MySQL',
    'Redis',
  ])
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/',
    '/hosts',
    '/disks',
    '/mysql',
    '/redis',
  ])
  expect(within(navigation).queryByRole('button')).not.toBeInTheDocument()
  expect(within(navigation).queryByText(/详情|操作/)).not.toBeInTheDocument()
  const connection = screen.getByLabelText('数据连接汇总')
  expect(await within(connection).findByText('1/1 正常')).toBeVisible()
  expect(within(connection).queryByText('MySQL')).not.toBeInTheDocument()
})

it('把后端返回的非默认刷新周期传给当前页面', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(datasourceFixture({ refreshIntervalSeconds: 45 })),
  )
  renderShell()

  expect(
    await screen.findByText('页面刷新周期 45000 毫秒'),
  ).toBeInTheDocument()
})

it('区分数据源异常和过期缓存状态', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(datasourceFixture({ healthy: false, stale: true })),
  )
  renderShell()

  const status = screen.getByLabelText('数据连接汇总')
  const summary = status.querySelector('summary')
  expect(summary).not.toBeNull()
  expect(
    await within(summary as HTMLElement).findByText('状态过期'),
  ).toBeInTheDocument()
  expect(within(status).getByText('上次检查异常')).toBeInTheDocument()
})

it('Mock 模式在汇总行明确提示非真实数据', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(datasourceFixture({ type: 'mock' })),
  )
  renderShell()

  const status = screen.getByLabelText('数据连接汇总')
  expect(
    await within(status).findByText('包含 Mock 数据'),
  ).toBeInTheDocument()
  expect(status).toHaveAttribute('data-mode', 'mock')
})

it('首次状态请求失败时显示请求失败而不伪装为健康', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(
      {
        code: 'datasource_unavailable',
        message: '数据源暂时不可用，请稍后重试',
        request_id: 'req-datasource-failed-001',
        retryable: true,
      },
      503,
    ),
  )
  renderShell()

  const status = screen.getByLabelText('数据连接汇总')
  expect(
    await within(status).findByText('状态获取失败'),
  ).toBeInTheDocument()
  expect(status).toHaveAttribute('data-state', 'error')
})

it('按后端返回的 15 秒周期重新检查数据源状态', async () => {
  vi.useFakeTimers()
  renderShell()

  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)

  await act(async () => vi.advanceTimersByTimeAsync(14_999))
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(globalThis.fetch).toHaveBeenCalledTimes(2)
})
