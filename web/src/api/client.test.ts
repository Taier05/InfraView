import { afterEach, describe, expect, it, vi } from 'vitest'

import { APIError, apiRequest, onUnauthorized } from './client'

afterEach(() => vi.unstubAllGlobals())

describe.each([
  ['空响应体', ''],
  ['损坏的 JSON', '{"data":'],
])('成功响应包含%s时', (_caseName, body) => {
  it('转换为稳定的无效响应错误', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(body, {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    )

    const request = apiRequest('/api/v1/example')
    await expect(request).rejects.toBeInstanceOf(APIError)
    await expect(request).rejects.toMatchObject({
      name: 'APIError',
      code: 'invalid_response',
      message: '服务器响应格式无效',
      retryable: false,
    })
  })
})

it('受保护的会话删除请求返回 401 时通知认证失效', async () => {
  const unauthorized = vi.fn()
  const unsubscribe = onUnauthorized(unauthorized)
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      new Response('', {
        status: 401,
        headers: { 'Content-Type': 'text/plain' },
      }),
    ),
  )

  await expect(
    apiRequest('/api/v1/session', { method: 'DELETE' }),
  ).rejects.toBeInstanceOf(APIError)
  expect(unauthorized).toHaveBeenCalledTimes(1)
  unsubscribe()
})

it('登录请求自身返回 401 时不通知认证失效', async () => {
  const unauthorized = vi.fn()
  const unsubscribe = onUnauthorized(unauthorized)
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      new Response('', {
        status: 401,
        headers: { 'Content-Type': 'text/plain' },
      }),
    ),
  )

  await expect(
    apiRequest('/api/v1/session', { method: 'POST' }),
  ).rejects.toBeInstanceOf(APIError)
  expect(unauthorized).not.toHaveBeenCalled()
  unsubscribe()
})
