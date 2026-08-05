import { HttpResponse, http } from 'msw'
import { expect, it } from 'vitest'

import { apiRequest } from '../api/client'
import type {
  RabbitMQNodePageResponse,
  RabbitMQOverviewResponse,
} from '../api/types'
import {
  RABBITMQ_NODES_PATH,
  RABBITMQ_OVERVIEW_PATH,
  rabbitMQNodePageEmptyFixture,
  rabbitMQNodePageMalformedFixture,
  rabbitMQOverviewEmptyFixture,
  rabbitMQOverviewMalformedFixture,
  rabbitMQOverviewFixture,
} from './fixtures'
import { server } from './server'

it('两个固定 GET 返回完整 RabbitMQ envelope，并保留四种节点状态与未知空值', async () => {
  const [overview, nodes] = await Promise.all([
    apiRequest<RabbitMQOverviewResponse>('/api/v1/rabbitmq/overview'),
    apiRequest<RabbitMQNodePageResponse>('/api/v1/rabbitmq/nodes'),
  ])

  expect(overview.data.alerts).toEqual({
    cluster_connectivity: { warning: 1, critical: 0, unknown: 1 },
    resource_alarms: { warning: 0, critical: 1, unknown: 0 },
    resource_pressure: { warning: 1, critical: 1, unknown: 0 },
    collection: { warning: 1, critical: 0, unknown: 1 },
  })
  expect(nodes.data.nodes.map((node) => node.status)).toEqual([
    'normal',
    'warning',
    'critical',
    'unknown',
  ])
  expect(nodes.data.nodes.at(-1)).toMatchObject({
    memory_usage_percent: null,
    disk_available_bytes: null,
    connections: null,
    status_source: 'unknown',
    collection_level: 'unknown',
  })
})

it('handler 覆盖可返回 stale RabbitMQ envelope', async () => {
  server.use(
    http.get(RABBITMQ_OVERVIEW_PATH, () =>
      HttpResponse.json(rabbitMQOverviewFixture({ meta: { stale: true } })),
    ),
  )

  const overview = await apiRequest<RabbitMQOverviewResponse>(
    '/api/v1/rabbitmq/overview',
  )

  expect(overview.meta.stale).toBe(true)
  expect(overview.meta.collected_at).toBe('2026-08-04T08:00:00.000Z')
})

it('handler 覆盖可返回空 RabbitMQ 节点页', async () => {
  server.use(
    http.get(RABBITMQ_NODES_PATH, () =>
      HttpResponse.json(rabbitMQNodePageEmptyFixture()),
    ),
  )

  const nodes = await apiRequest<RabbitMQNodePageResponse>(
    '/api/v1/rabbitmq/nodes',
  )

  expect(nodes.data).toEqual({
    nodes: [],
    available_clusters: [],
    total: 0,
    page: 1,
    page_size: 20,
    total_pages: 0,
  })
})

it('handler 覆盖可返回空 RabbitMQ 总览，并保持零节点和零集群语义', async () => {
  server.use(
    http.get(RABBITMQ_OVERVIEW_PATH, () =>
      HttpResponse.json(rabbitMQOverviewEmptyFixture()),
    ),
  )

  const overview = await apiRequest<RabbitMQOverviewResponse>(
    '/api/v1/rabbitmq/overview',
  )

  expect(overview.data.status).toBe('normal')
  expect(overview.data.clusters).toEqual({
    total: 0,
    normal: 0,
    warning: 0,
    critical: 0,
    unknown: 0,
  })
  expect(overview.data.nodes).toEqual({
    total: 0,
    normal: 0,
    warning: 0,
    critical: 0,
    unknown: 0,
  })
  expect(overview.data.alerts).toEqual({
    cluster_connectivity: { warning: 0, critical: 0, unknown: 0 },
    resource_alarms: { warning: 0, critical: 0, unknown: 0 },
    resource_pressure: { warning: 0, critical: 0, unknown: 0 },
    collection: { warning: 0, critical: 0, unknown: 0 },
  })
})

it('合法 JSON 的结构错误 RabbitMQ fixtures 由 transport client 原样返回', async () => {
  server.use(
    http.get(RABBITMQ_OVERVIEW_PATH, () =>
      HttpResponse.json(
        rabbitMQOverviewMalformedFixture() as { data: { status: string } },
      ),
    ),
    http.get(RABBITMQ_NODES_PATH, () =>
      HttpResponse.json(
        rabbitMQNodePageMalformedFixture() as {
          data: { nodes: Array<{ id: number }> }
        },
      ),
    ),
  )

  const [overview, nodes] = await Promise.all([
    apiRequest<unknown>('/api/v1/rabbitmq/overview'),
    apiRequest<unknown>('/api/v1/rabbitmq/nodes'),
  ])

  expect(overview).toMatchObject({ data: { status: 'invalid' } })
  expect(overview).not.toMatchObject({ data: { clusters: expect.any(Object) } })
  expect(nodes).toMatchObject({ data: { nodes: [{ id: 42 }] } })
  expect(nodes).not.toMatchObject({ data: { nodes: [{ id: expect.any(String) }] } })
})

it('handler 覆盖的 malformed JSON 由 apiRequest 拒绝为 invalid_response', async () => {
  server.use(
    http.get(
      RABBITMQ_OVERVIEW_PATH,
      () =>
        new HttpResponse('{"data":', {
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )

  await expect(
    apiRequest<RabbitMQOverviewResponse>('/api/v1/rabbitmq/overview'),
  ).rejects.toMatchObject({
    name: 'APIError',
    code: 'invalid_response',
    retryable: false,
  })
})

it('RabbitMQ POST 没有注册写 handler，并由全局严格 MSW 策略拒绝', async () => {
  await expect(
    apiRequest('/api/v1/rabbitmq/overview', { method: 'POST' }),
  ).rejects.toMatchObject({
    name: 'APIError',
    code: 'network_error',
    retryable: true,
  })
})
