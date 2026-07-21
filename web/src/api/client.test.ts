import { afterEach, describe, expect, it, vi } from 'vitest'

import { APIError, apiRequest } from './client'

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
