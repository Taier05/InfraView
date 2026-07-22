import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
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
}: {
  healthy?: boolean
  stale?: boolean
} = {}) {
  return {
    data: {
      healthy,
      checked_at: '2026-07-22T02:03:04.000Z',
    },
    meta: {
      request_id: 'req-datasource-001',
      stale,
      collected_at: '2026-07-22T02:03:05.000Z',
    },
  }
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
            <Route index element={<p>页面内容</p>} />
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

it('展示健康数据源与最近检查时间', async () => {
  renderShell()

  const status = screen.getByLabelText('数据源状态')
  expect(await within(status).findByText('健康')).toBeInTheDocument()
  expect(within(status).getByText('Mock')).toBeInTheDocument()
  expect(within(status).getByText('最近检查')).toBeInTheDocument()
  expect(within(status).getByRole('time')).toHaveAttribute(
    'datetime',
    '2026-07-22T02:03:04.000Z',
  )
})

it('区分数据源异常和过期缓存状态', async () => {
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    jsonResponse(datasourceFixture({ healthy: false, stale: true })),
  )
  renderShell()

  const status = screen.getByLabelText('数据源状态')
  expect(await within(status).findByText('状态过期')).toBeInTheDocument()
  expect(within(status).getByText('上次检查异常')).toBeInTheDocument()
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

  const status = screen.getByLabelText('数据源状态')
  expect(await within(status).findByText('请求失败')).toBeInTheDocument()
  expect(status).toHaveAttribute('data-state', 'error')
})

it('每 30 秒定时重新检查数据源状态', async () => {
  vi.useFakeTimers()
  renderShell()

  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)

  await act(async () => vi.advanceTimersByTimeAsync(29_999))
  expect(globalThis.fetch).toHaveBeenCalledTimes(1)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(globalThis.fetch).toHaveBeenCalledTimes(2)
})
