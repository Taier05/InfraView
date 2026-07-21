import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse, delay, http } from 'msw'

import { App } from '../app/App'
import { SESSION_PATH } from '../test/fixtures'
import { server } from '../test/server'

beforeEach(() => {
  window.history.pushState({}, '', '/login')
})

it('登录后进入基础设施总览', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(screen.getByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'secret-value')
  await user.click(screen.getByRole('button', { name: '登录' }))

  expect(
    await screen.findByRole('heading', { name: '基础设施总览' }),
  ).toBeInTheDocument()
})

it('凭据无效时显示后端返回的中文错误', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(screen.getByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'wrong-password')
  await user.click(screen.getByRole('button', { name: '登录' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('用户名或密码错误')
})

it('登录失败后清空密码', async () => {
  const user = userEvent.setup()
  render(<App />)

  await user.type(screen.getByLabelText('用户名'), 'admin')
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

  await user.type(screen.getByLabelText('用户名'), 'admin')
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
