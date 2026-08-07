import { QueryClient } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse, delay, http } from 'msw'
import { afterEach, vi } from 'vitest'

import { App } from '../app/App'
import {
  mysqlOverviewFixture,
  overviewFixture,
  SESSION_PATH,
  sessionFixture,
  unauthenticatedFixture,
} from '../test/fixtures'
import { server } from '../test/server'

beforeEach(() => {
  window.history.pushState({}, '', '/login')
})

afterEach(() => vi.restoreAllMocks())

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestPath(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost').pathname
}

function mockAuthenticatedRequests(
  logoutResponse: () => Response = () => new Response(null, { status: 204 }),
) {
  vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
    if (requestPath(input) === '/api/v1/overview') {
      return Promise.resolve(jsonResponse(overviewFixture()))
    }
    if (requestPath(input) === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (requestPath(input) === '/api/v1/datasource/status') {
      return Promise.resolve(
        jsonResponse({
          data: {
            healthy: true,
            checked_at: '2026-07-22T02:03:04.000Z',
          },
          meta: {
            request_id: 'req-datasource-001',
            stale: false,
            collected_at: '2026-07-22T02:03:05.000Z',
          },
        }),
      )
    }
    if (init?.method === 'DELETE') {
      return Promise.resolve(logoutResponse())
    }
    return Promise.resolve(jsonResponse(sessionFixture('admin')))
  })
}

it('会话 bootstrap 完成前不显示登录表单', async () => {
  let resolveSession!: (response: Response) => void
  vi.spyOn(globalThis, 'fetch').mockImplementationOnce(
    () =>
      new Promise<Response>((resolve) => {
        resolveSession = resolve
      }),
  )
  render(<App />)

  expect(screen.getByRole('status')).toHaveTextContent('正在验证登录状态')
  expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '登录' })).not.toBeInTheDocument()

  await act(async () => {
    resolveSession(jsonResponse(unauthenticatedFixture, 401))
  })

  expect(screen.getByLabelText('用户名')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()
})

it('已认证会话 bootstrap 后直接进入基础设施总览', async () => {
  mockAuthenticatedRequests()
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
  expect(
    await screen.findByRole('link', { name: '查看 MySQL 板块' }),
  ).toHaveAttribute('href', '/mysql')
  expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument()
})

it('已认证总览展示模块数据时间且不显示刷新或时间范围控件', async () => {
  mockAuthenticatedRequests()
  render(<App />)

  const hostCard = await screen.findByRole('link', {
    name: '查看 Linux 主机板块',
  })
  expect(within(hostCard).getByText('2026/07/21 00:30:00')).toBeVisible()
  expect(screen.queryByRole('button', { name: '刷新' })).not.toBeInTheDocument()
  expect(screen.queryByRole('group', { name: '总览控制' })).not.toBeInTheDocument()
  expect(
    screen.queryByRole('combobox', { name: '时间范围' }),
  ).not.toBeInTheDocument()
})

it('登录后进入基础设施总览', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(await screen.findByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'secret-value')
  await user.click(screen.getByRole('button', { name: '登录' }))

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
})

it('已认证用户可通过导航进入 MySQL 实例页', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(await screen.findByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'secret-value')
  await user.click(screen.getByRole('button', { name: '登录' }))
  await screen.findByRole('heading', { name: '基础设施总览' })

  await user.click(screen.getByRole('link', { name: 'MySQL' }))

  expect(
    await screen.findByRole('heading', { name: 'MySQL 实例' }),
  ).toBeInTheDocument()
  expect(window.location.pathname).toBe('/mysql')
})

it('凭据无效时显示后端返回的中文错误', async () => {
  const clear = vi.spyOn(QueryClient.prototype, 'clear')
  const user = userEvent.setup()
  render(<App />)

  await user.type(await screen.findByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'wrong-password')
  await user.click(screen.getByRole('button', { name: '登录' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('用户名或密码错误')
  expect(clear).not.toHaveBeenCalled()
  expect(window.location.pathname).toBe('/login')
})

it('登录失败后清空密码', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(await screen.findByLabelText('用户名'), 'admin')
  const password = screen.getByLabelText('密码')
  await user.type(password, 'wrong-password')
  await user.click(screen.getByRole('button', { name: '登录' }))

  await screen.findByRole('alert')
  expect(password).toHaveValue('')
})

it('登录请求进行时禁用提交按钮', async () => {
  server.use(
    http.post(SESSION_PATH, async () => {
      await delay(100)
      return new HttpResponse(null, { status: 204 })
    }),
  )
  const user = userEvent.setup()
  render(<App />)

  await user.type(await screen.findByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'secret-value')
  const submit = screen.getByRole('button', { name: '登录' })
  await user.click(submit)

  expect(submit).toBeDisabled()
  await waitFor(() => expect(submit).toBeEnabled())
})

it('未认证访问受保护页面时重定向到登录页', async () => {
  window.history.pushState({}, '', '/hosts')
  render(<App />)

  await waitFor(() => expect(window.location.pathname).toBe('/login'))
  expect(screen.getByRole('heading', { name: '登录 InfraView' })).toBeInTheDocument()
})

it('注销失败时保留应用壳和查询缓存并显示可重试错误', async () => {
  const clear = vi.spyOn(QueryClient.prototype, 'clear')
  mockAuthenticatedRequests(() =>
    jsonResponse(
      {
        code: 'logout_failed',
        message: '退出登录失败，请稍后重试',
        request_id: 'req-logout-failed-001',
        retryable: true,
      },
      503,
    ),
  )
  window.history.pushState({}, '', '/')
  const user = userEvent.setup()
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '退出登录' }))

  expect(
    await screen.findByText('退出登录失败，请稍后重试', { exact: true }),
  ).toHaveAttribute('role', 'alert')
  expect(
    screen.getByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '退出登录' })).toBeInTheDocument()
  expect(clear).not.toHaveBeenCalled()
  expect(window.location.pathname).toBe('/')
})

it('注销成功后清空查询缓存并回到登录页', async () => {
  const clear = vi.spyOn(QueryClient.prototype, 'clear')
  mockAuthenticatedRequests()
  window.history.pushState({}, '', '/')
  const user = userEvent.setup()
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '退出登录' }))

  expect(
    await screen.findByRole('heading', { name: '登录 InfraView' }),
  ).toBeInTheDocument()
  expect(clear).toHaveBeenCalledTimes(1)
  expect(window.location.pathname).toBe('/login')
})

it('容器重启导致受保护 API 会话失效时清空缓存并回到登录页', async () => {
  const clear = vi.spyOn(QueryClient.prototype, 'clear')
  let overviewRequests = 0
  vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
    const path = requestPath(input)
    if (path === '/api/v1/overview') {
      overviewRequests += 1
      if (overviewRequests === 1) {
        return Promise.resolve(jsonResponse(overviewFixture()))
      }
      return Promise.resolve(jsonResponse(unauthenticatedFixture, 401))
    }
    if (path === '/api/v1/mysql/overview') {
      return Promise.resolve(jsonResponse(mysqlOverviewFixture()))
    }
    if (path === '/api/v1/datasource/status') {
      return Promise.resolve(
        jsonResponse({
          data: {
            healthy: true,
            checked_at: '2026-07-22T02:03:04.000Z',
          },
          meta: { request_id: 'req-datasource-001', stale: false },
        }),
      )
    }
    return Promise.resolve(jsonResponse(sessionFixture('admin')))
  })
  window.history.pushState({}, '', '/')
  const user = userEvent.setup()
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
  await user.click(screen.getByRole('link', { name: '主机' }))
  await waitFor(() => expect(window.location.pathname).toBe('/hosts'))
  await user.click(screen.getByRole('link', { name: '总览' }))

  expect(
    await screen.findByRole('heading', { name: '登录 InfraView' }),
  ).toBeInTheDocument()
  expect(clear).toHaveBeenCalledTimes(1)
  expect(window.location.pathname).toBe('/login')
})
