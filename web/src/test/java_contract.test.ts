import { HttpResponse, http } from 'msw'
import { expect, it } from 'vitest'

import { apiRequest } from '../api/client'
import type {
  JavaOverviewResponse,
  JavaServicePageResponse,
} from '../api/types'
import {
  JAVA_OVERVIEW_PATH,
  JAVA_SERVICES_PATH,
  javaErrorFixture,
  javaOverviewEmptyFixture,
  javaOverviewFixture,
  javaOverviewMalformedFixture,
  javaServicePageEmptyFixture,
  javaServicePageFixture,
  javaServicePageMalformedFixture,
} from './fixtures'
import { server } from './server'

it('两个固定 GET 返回 Java envelope，并保留全部服务状态与未知空值', async () => {
  const [overview, services] = await Promise.all([
    apiRequest<JavaOverviewResponse>('/api/v1/java/overview'),
    apiRequest<JavaServicePageResponse>('/api/v1/java/services'),
  ])

  expect(overview.data).toMatchObject({
    status: 'critical',
    services: { total: 4, normal: 1, warning: 1, critical: 1, unknown: 1 },
    alerts: {
      health: { warning: 0, critical: 1, unknown: 1 },
      port: { warning: 0, critical: 1, unknown: 1 },
      process: { warning: 0, critical: 1, unknown: 1 },
      collection: { warning: 1, critical: 0, unknown: 1 },
    },
  })
  expect(services.data.services.map((service) => service.status)).toEqual([
    'normal',
    'warning',
    'critical',
    'unknown',
  ])
  expect(services.data.services.at(-1)).toMatchObject({
    health_up: null,
    health_latency_ms: null,
    port_up: null,
    process_up: null,
    process_count: null,
    port_consistent: null,
    cpu_usage_percent: null,
    memory_bytes: null,
    memory_usage_percent: null,
    uptime_seconds: null,
    status_source: 'unknown',
    collection_level: 'unknown',
  })
})

it('handler 覆盖可返回 stale Java envelope', async () => {
  server.use(
    http.get(JAVA_OVERVIEW_PATH, () =>
      HttpResponse.json(javaOverviewFixture({ meta: { stale: true } })),
    ),
  )

  const overview = await apiRequest<JavaOverviewResponse>(
    '/api/v1/java/overview',
  )

  expect(overview.meta).toMatchObject({
    stale: true,
    collected_at: '2026-08-05T08:00:00.000Z',
  })
})

it('handler 覆盖可返回空 Java 总览与服务页', async () => {
  server.use(
    http.get(JAVA_OVERVIEW_PATH, () =>
      HttpResponse.json(javaOverviewEmptyFixture()),
    ),
    http.get(JAVA_SERVICES_PATH, () =>
      HttpResponse.json(javaServicePageEmptyFixture()),
    ),
  )

  const [overview, services] = await Promise.all([
    apiRequest<JavaOverviewResponse>('/api/v1/java/overview'),
    apiRequest<JavaServicePageResponse>('/api/v1/java/services'),
  ])

  expect(overview.data).toEqual({
    status: 'normal',
    services: { total: 0, normal: 0, warning: 0, critical: 0, unknown: 0 },
    alerts: {
      health: { warning: 0, critical: 0, unknown: 0 },
      port: { warning: 0, critical: 0, unknown: 0 },
      process: { warning: 0, critical: 0, unknown: 0 },
      collection: { warning: 0, critical: 0, unknown: 0 },
    },
  })
  expect(services.data).toEqual({
    services: [],
    available_names: [],
    total: 0,
    page: 1,
    page_size: 20,
    total_pages: 0,
  })
})

it('handler 覆盖的 Java error envelope 由 transport client 公开为安全错误', async () => {
  server.use(
    http.get(JAVA_OVERVIEW_PATH, () =>
      HttpResponse.json(javaErrorFixture(), { status: 503 }),
    ),
  )

  await expect(
    apiRequest<JavaOverviewResponse>('/api/v1/java/overview'),
  ).rejects.toMatchObject({
    name: 'APIError',
    status: 503,
    code: 'java_unavailable',
    retryable: true,
  })
})

it('合法 JSON 的结构错误 Java envelope 由 transport client 原样返回', async () => {
  server.use(
    http.get(JAVA_OVERVIEW_PATH, () =>
      HttpResponse.json(
        javaOverviewMalformedFixture() as { data: { status: string } },
      ),
    ),
    http.get(JAVA_SERVICES_PATH, () =>
      HttpResponse.json(
        javaServicePageMalformedFixture() as {
          data: { services: Array<{ id: number }> }
        },
      ),
    ),
  )

  const [overview, services] = await Promise.all([
    apiRequest<unknown>('/api/v1/java/overview'),
    apiRequest<unknown>('/api/v1/java/services'),
  ])

  expect(overview).toMatchObject({ data: { status: 'invalid' } })
  expect(overview).not.toMatchObject({ data: { services: expect.any(Object) } })
  expect(services).toMatchObject({ data: { services: [{ id: 42 }] } })
  expect(services).not.toMatchObject({
    data: { services: [{ id: expect.any(String) }] },
  })
})

it('Java fixtures 仅含脱敏文档值，不含标识、凭据、地址或查询正文', () => {
  const fixtureText = JSON.stringify({
    overview: javaOverviewFixture(),
    services: javaServicePageFixture(),
    emptyOverview: javaOverviewEmptyFixture(),
    emptyServices: javaServicePageEmptyFixture(),
    error: javaErrorFixture(),
    malformedOverview: javaOverviewMalformedFixture(),
    malformedServices: javaServicePageMalformedFixture(),
  }).toLowerCase()

  expect(fixtureText).not.toMatch(/ident|password|token|authorization|promql/)
  expect(fixtureText).not.toMatch(/\b(?:\d{1,3}\.){3}\d{1,3}\b/)
  expect(fixtureText).toContain('fixture-address-a')
})

it('Java POST 没有注册写 handler，并由全局严格 MSW 策略拒绝', async () => {
  await expect(
    apiRequest('/api/v1/java/overview', { method: 'POST' }),
  ).rejects.toMatchObject({
    name: 'APIError',
    code: 'network_error',
    retryable: true,
  })
})
