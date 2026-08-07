import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse, delay, http } from 'msw'
import { BrowserRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { App } from '../../app/App'
import {
  ELASTICSEARCH_NODES_PATH,
  SESSION_PATH,
  elasticsearchNodePageFixture,
  type ElasticsearchNodePageFixture,
} from '../../test/fixtures'
import { server } from '../../test/server'
import { ElasticsearchPage } from './ElasticsearchPage'

const expectedHeaders = [
  '节点名称',
  '所属集群',
  '节点地址',
  '节点角色',
  '集群健康',
  'JVM堆使用率',
  '磁盘使用率',
  'CPU使用率',
  '索引速率',
  '搜索速率',
  '文档数',
  '存储大小',
  '线程池队列',
  '拒绝速率',
  '运行时间',
  '状态',
] as const

const sortFields = [
  'node',
  'cluster',
  'address',
  'role',
  'cluster_health',
  'heap',
  'disk',
  'cpu',
  'index_rate',
  'search_rate',
  'documents',
  'store',
  'thread_queue',
  'rejected_rate',
  'uptime',
  'status',
] as const

let responseBody: unknown
let responseStatus: number
let responseDelay: number
const requests: URL[] = []

function renderPage(entry = '/elasticsearch') {
  window.history.replaceState({}, '', entry)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  const result = render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ElasticsearchPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
  return { ...result, queryClient }
}

function errorFixture() {
  return {
    code: 'elasticsearch_unavailable',
    message: '数据源暂时不可用，请稍后重试',
    request_id: 'req-fixture-elasticsearch-error-001',
    retryable: true,
  }
}

function cloneFixture() {
  return structuredClone(elasticsearchNodePageFixture())
}

function respondWithRequestedPage() {
  server.use(
    http.get(ELASTICSEARCH_NODES_PATH, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      const pageSize = Number(url.searchParams.get('page_size'))
      const total = pageSize === 500 ? 1001 : 60
      return HttpResponse.json(
        elasticsearchNodePageFixture({
          data: {
            page: Number(url.searchParams.get('page')),
            page_size: pageSize,
            total,
            total_pages: Math.ceil(total / pageSize),
          },
        }),
      )
    }),
  )
}

beforeEach(() => {
  responseBody = elasticsearchNodePageFixture()
  responseStatus = 200
  responseDelay = 0
  requests.length = 0
  server.use(
    http.get(ELASTICSEARCH_NODES_PATH, async ({ request }) => {
      requests.push(new URL(request.url))
      if (responseDelay > 0) await delay(responseDelay)
      return HttpResponse.json(responseBody as Record<string, unknown>, {
        status: responseStatus,
      })
    }),
  )
})

afterEach(() => {
  vi.useRealTimers()
})

it('严格渲染固定顺序的十六个单值列', async () => {
  const { container } = renderPage()

  expect(
    await screen.findByRole('heading', { name: 'Elasticsearch 节点' }),
  ).toBeVisible()
  await screen.findByText('fixture-es-node-a')
  expect(
    screen.getAllByRole('columnheader').map((cell) => cell.textContent),
  ).toEqual(expectedHeaders)
  expect(container.querySelectorAll('tbody td')).toHaveLength(
    expectedHeaders.length,
  )
  expect(container.querySelector('tbody br')).toBeNull()
  expect(container.querySelectorAll('tbody td[title]')).toHaveLength(0)
  for (const cell of container.querySelectorAll('tbody td')) {
    expect(getComputedStyle(cell).textOverflow).not.toBe('ellipsis')
  }
})

it('桌面表格使用固定紧凑布局并为可裁剪身份保留完整提示', async () => {
  renderPage()

  const table = await screen.findByRole('table', {
    name: 'Elasticsearch 节点列表',
  })
  expect(table).toHaveClass('host-table', 'elasticsearch-table', 'observability-table')
  await screen.findByText('fixture-es-node-a')
  expect(getComputedStyle(table).tableLayout).toBe('fixed')
  expect(getComputedStyle(table).width).toBe('100%')

  const cells = within(screen.getAllByRole('row')[1]).getAllByRole('cell')
  const identityValues = cells.slice(0, 3).map((cell) =>
    cell.querySelector('.elasticsearch-identity'),
  )
  expect(identityValues[0]).toHaveAttribute('title', 'fixture-es-node-a')
  expect(identityValues[1]).toHaveAttribute('title', 'fixture-es-cluster-a')
  expect(identityValues[2]).toHaveAttribute('title', '192.0.2.31:9200')
})

it('只用共享列表模板在同一控制行渲染搜索、四个筛选、页数与最新数据时间', async () => {
  renderPage()

  const search = await screen.findByRole('searchbox', {
    name: '搜索节点名称或地址',
  })
  const controls = search.closest('.elasticsearch-list-controls')
  expect(controls).not.toBeNull()
  if (!(controls instanceof HTMLElement)) {
    throw new Error('Elasticsearch 控制区未渲染为 HTML 元素')
  }
  expect(controls).toHaveClass('host-list-controls')
  expect(within(controls).getAllByRole('searchbox')).toHaveLength(1)
  expect(within(controls).getAllByRole('combobox')).toHaveLength(5)
  for (const label of [
    '所属集群',
    '节点角色',
    '集群健康',
    '节点状态',
    '每页数量',
  ]) {
    expect(within(controls).getByRole('combobox', { name: label })).toBeVisible()
  }
  const dataTime = await within(controls).findByText('2026/08/01 08:00:00')
  expect(dataTime.closest('.data-time')).toHaveTextContent('最新数据时间：2026/08/01 08:00:00')
  expect(within(controls).queryByRole('button', { name: /刷新/ })).not.toBeInTheDocument()
  expect(within(controls).queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument()
  expect(controls.querySelector('.elasticsearch-search')).toBeNull()
  expect(controls.querySelector('.elasticsearch-select')).toBeNull()
  const table = await screen.findByRole('table', {
    name: 'Elasticsearch 节点列表',
  })
  const scrollOwner = table.closest('.host-table-scroll')
  expect(scrollOwner).toHaveClass('elasticsearch-table-scroll')
  expect(scrollOwner?.parentElement).toHaveClass('host-table-panel')
})

it('把四种筛选、分页大小与固定白名单写入 URL 和请求', async () => {
  respondWithRequestedPage()
  const user = userEvent.setup()
  renderPage('/elasticsearch?unknown=value&page=3&page_size=500')
  expect(await screen.findByText('第 3 / 3 页，共 1001 个节点')).toBeVisible()
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('500')
  expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
    sort: 'node',
    order: 'asc',
    page: '3',
    page_size: '500',
  })

  await user.selectOptions(
    screen.getByRole('combobox', { name: '所属集群' }),
    'fixture-es-cluster-a',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '节点角色' }),
    'data_hot',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '集群健康' }),
    'yellow',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '节点状态' }),
    'warning',
  )
  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search)
    expect(parameters.get('cluster')).toBe('fixture-es-cluster-a')
    expect(parameters.get('role')).toBe('data_hot')
    expect(parameters.get('cluster_health')).toBe('yellow')
    expect(parameters.get('status')).toBe('warning')
    expect(parameters.get('page_size')).toBe('500')
    expect(parameters.get('page')).toBe('1')
  })
  expect(await screen.findByText('第 1 / 3 页，共 1001 个节点')).toBeVisible()
  expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
    cluster: 'fixture-es-cluster-a',
    role: 'data_hot',
    cluster_health: 'yellow',
    status: 'warning',
    sort: 'node',
    order: 'asc',
    page: '1',
    page_size: '500',
  })
  expect(requests.at(-1)?.searchParams.has('unknown')).toBe(false)
})

it('通过每页数量下拉切换到 500 并发送最后 GET', async () => {
  respondWithRequestedPage()
  const user = userEvent.setup()
  renderPage('/elasticsearch?page=3&page_size=20')

  await screen.findByText('第 3 / 3 页，共 60 个节点')
  await user.selectOptions(
    screen.getByRole('combobox', { name: '每页数量' }),
    '500',
  )
  await waitFor(() => {
    expect(window.location.search).toContain('page=1&page_size=500')
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      sort: 'node',
      order: 'asc',
      page: '1',
      page_size: '500',
    })
  })
})

it('搜索等待精确 300ms 后写入 URL 并重置页码', async () => {
  respondWithRequestedPage()
  renderPage('/elasticsearch?page=3')
  await screen.findByText('fixture-es-node-a')
  vi.useFakeTimers()
  fireEvent.change(
    screen.getByRole('searchbox', { name: '搜索节点名称或地址' }),
    { target: { value: 'fixture-node' } },
  )
  act(() => vi.advanceTimersByTime(299))
  expect(new URLSearchParams(window.location.search).has('search')).toBe(false)
  act(() => vi.advanceTimersByTime(1))
  expect(new URLSearchParams(window.location.search).get('search')).toBe(
    'fixture-node',
  )
  expect(new URLSearchParams(window.location.search).get('page')).toBe('1')
})

it('十六个表头都使用精确排序字段并可切换升降序', async () => {
  const user = userEvent.setup()
  renderPage()
  await screen.findByText('fixture-es-node-a')

  for (const [index, header] of expectedHeaders.entries()) {
    await user.click(
      screen.getByRole('button', { name: new RegExp(`^${header}排序`) }),
    )
    await waitFor(() =>
      expect(requests.at(-1)?.searchParams.get('sort')).toBe(sortFields[index]),
    )
  }
  expect(requests.at(-1)?.searchParams.get('order')).toBe('asc')
  await user.click(screen.getByRole('button', { name: /^状态排序/ }))
  await waitFor(() =>
    expect(requests.at(-1)?.searchParams.get('order')).toBe('desc'),
  )
})

it('规范非法 URL 参数并按服务端结果修正越界页码', async () => {
  responseBody = elasticsearchNodePageFixture({
    data: { page: 2, total: 40, total_pages: 2 },
  })
  renderPage(
    '/elasticsearch?role=invalid&cluster_health=blue&status=bad&sort=bad&order=bad&page=-4&page_size=25',
  )
  await screen.findByText('fixture-es-node-a')

  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search)
    expect(parameters.get('sort')).toBe('node')
    expect(parameters.get('order')).toBe('asc')
    expect(parameters.get('page')).toBe('2')
    expect(parameters.get('page_size')).toBe('20')
    expect(parameters.has('role')).toBe(false)
    expect(parameters.has('cluster_health')).toBe(false)
    expect(parameters.has('status')).toBe(false)
  })
})

it.each([499, 501])('将非法 page_size=%i 规范为 20 且回到第一页', async (pageSize) => {
  respondWithRequestedPage()
  renderPage(`/elasticsearch?page=3&page_size=${pageSize}`)

  await screen.findByText('fixture-es-node-a')
  await waitFor(() => {
    expect(window.location.search).toContain('page=1&page_size=20')
    expect(requests.at(-1)?.searchParams.get('page_size')).toBe('20')
  })
})

it('按既定规则格式化角色、空值、比例、速率、整数、IEC 与运行时间', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = elasticsearchNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          roles: ['master', 'data_hot'],
          heap_usage_percent: 72.55,
          disk_usage_percent: 81,
          cpu_usage_percent: 36.54,
          index_rate: 14.25,
          search_rate: 28,
          documents: 1200,
          store_size_bytes: 2 * 1024 ** 3,
          thread_pool_queue: 3,
          rejected_rate: 0.02,
          uptime_seconds: 90_000,
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-002',
          name: 'fixture-es-node-empty',
          address: '',
          roles: [],
          heap_usage_percent: null,
          disk_usage_percent: null,
          cpu_usage_percent: null,
          index_rate: null,
          search_rate: null,
          documents: null,
          store_size_bytes: null,
          thread_pool_queue: null,
          rejected_rate: null,
          uptime_seconds: null,
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-003',
          name: 'fixture-es-node-days',
          uptime_seconds: 172_800,
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-004',
          name: 'fixture-es-node-hours',
          uptime_seconds: 7_200,
        },
      ],
      total: 4,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  const values = within(rows[0]).getAllByRole('cell')
  expect(values.map((cell) => cell.textContent)).toEqual([
    'fixture-es-node-a',
    'fixture-es-cluster-a',
    '192.0.2.31:9200',
    'master / data_hot',
    '黄色',
    '72.5%',
    '81.0%',
    '36.5%',
    '14.25/s',
    '28/s',
    '1200',
    '2 GiB',
    '3',
    '0.02/s',
    '1天 1小时',
    '磁盘',
  ])
  expect(within(rows[1]).getAllByRole('cell').slice(2, 15).map((cell) => cell.textContent))
    .toEqual([
      '暂无数据',
      '未知',
      '黄色',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
      '暂无数据',
    ])
  expect(within(rows[2]).getAllByRole('cell')[14]).toHaveTextContent('2天')
  expect(within(rows[3]).getAllByRole('cell')[14]).toHaveTextContent('2小时')
})

it('节点角色只展示前两个并保留完整提示，集群健康使用四色徽标', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = elasticsearchNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          roles: ['master', 'data_hot', 'ingest'],
          cluster_health: 'green',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-002',
          name: 'fixture-es-node-yellow',
          roles: ['data_hot'],
          cluster_health: 'yellow',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-003',
          name: 'fixture-es-node-red',
          roles: [],
          cluster_health: 'red',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-004',
          name: 'fixture-es-node-unknown',
          roles: ['master', 'ingest'],
          cluster_health: 'unknown',
        },
      ],
      total: 4,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  const roleCells = rows.map((row) => within(row).getAllByRole('cell')[3])
  const roleValues = roleCells.map((cell) =>
    cell.querySelector('.elasticsearch-role'),
  )
  expect(roleCells[0]).toHaveTextContent('master / data_hot / …')
  expect(roleValues[0]).toHaveAttribute(
    'title',
    'master / data_hot / ingest',
  )
  expect(roleCells[1]).toHaveTextContent('data_hot')
  expect(roleCells[1]).not.toHaveTextContent('…')
  expect(roleValues[1]).toHaveAttribute('title', 'data_hot')
  expect(roleCells[2]).toHaveTextContent('未知')
  expect(roleCells[2]).not.toHaveTextContent('…')
  expect(roleValues[2]).toHaveAttribute('title', '未知')
  expect(roleCells[3]).toHaveTextContent('master / ingest')
  expect(roleCells[3]).not.toHaveTextContent('…')

  const healthCells = rows.map((row) => within(row).getAllByRole('cell')[4])
  for (const [index, level] of [
    'normal',
    'warning',
    'critical',
    'unknown',
  ].entries()) {
    expect(healthCells[index].querySelector('.status-badge')).toHaveAttribute(
      'data-level',
      level,
    )
  }
})

it('集群健康独立展示且不污染节点状态，采集胜出来源展示采集文案', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = elasticsearchNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          cluster_health: 'red',
          status: 'normal',
          status_source: 'normal',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-002',
          name: 'fixture-es-node-delay',
          cluster_health: 'green',
          status: 'warning',
          status_source: 'collection',
          collection_level: 'warning',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-003',
          name: 'fixture-es-node-lost',
          cluster_health: 'unknown',
          status: 'critical',
          status_source: 'collection',
          collection_level: 'critical',
        },
      ],
      total: 3,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  expect(within(rows[0]).getAllByRole('cell')[4]).toHaveTextContent('红色')
  expect(within(rows[0]).getAllByRole('cell')[15]).toHaveTextContent('正常')
  expect(within(rows[1]).getAllByRole('cell')[4]).toHaveTextContent('绿色')
  expect(within(rows[1]).getAllByRole('cell')[15]).toHaveTextContent('采集延迟')
  expect(within(rows[2]).getAllByRole('cell')[4]).toHaveTextContent('未知')
  expect(within(rows[2]).getAllByRole('cell')[15]).toHaveTextContent('采集失联')
})

it('以后端合成状态及胜出来源展示混合资源与采集级别', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = elasticsearchNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          status: 'critical',
          status_source: 'disk',
          collection_level: 'warning',
        },
        {
          ...base,
          id: 'elasticsearch-fixture-node-002',
          name: 'fixture-es-node-collection-critical',
          status: 'critical',
          status_source: 'collection',
          collection_level: 'critical',
        },
      ],
      total: 2,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  const resourceStatus = within(rows[0]).getAllByRole('cell')[15]
  const collectionStatus = within(rows[1]).getAllByRole('cell')[15]
  expect(resourceStatus).toHaveTextContent('磁盘')
  expect(resourceStatus.querySelector('.status-badge')).toHaveAttribute(
    'data-level',
    'critical',
  )
  expect(collectionStatus).toHaveTextContent('采集失联')
  expect(collectionStatus.querySelector('.status-badge')).toHaveAttribute(
    'data-level',
    'critical',
  )
})

it('展示初始加载、过期数据和空结果状态', async () => {
  responseDelay = 20
  responseBody = elasticsearchNodePageFixture({
    data: { nodes: [], total: 0, total_pages: 0 },
    meta: { stale: true },
  })
  renderPage()
  expect(screen.getByRole('status')).toHaveTextContent('正在加载 Elasticsearch 节点')
  expect(await screen.findByText('没有符合条件的 Elasticsearch 节点')).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent('数据已过期')
  expect(screen.getByLabelText('Elasticsearch 节点列表分页')).toHaveTextContent('暂无节点')
})

it('过期响应无采集时间时显示缓存提示且不伪造时间', async () => {
  responseBody = elasticsearchNodePageFixture({
    meta: { stale: true, collected_at: undefined },
  })
  renderPage()

  await screen.findByText('fixture-es-node-a')
  const banner = screen.getByRole('alert')
  expect(banner).toHaveTextContent('数据已过期')
  expect(banner).toHaveTextContent('正在展示缓存数据')
  expect(within(banner).queryByRole('time')).not.toBeInTheDocument()
})

it('初次 503 可重试，成功后刷新错误仍保留旧数据', async () => {
  responseBody = errorFixture()
  responseStatus = 503
  const user = userEvent.setup()
  const { queryClient } = renderPage()

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Elasticsearch 节点列表加载失败',
  )
  responseBody = elasticsearchNodePageFixture()
  responseStatus = 200
  await user.click(screen.getByRole('button', { name: '重试 Elasticsearch 节点列表' }))
  expect(await screen.findByText('fixture-es-node-a')).toBeVisible()

  responseBody = errorFixture()
  responseStatus = 503
  await queryClient.invalidateQueries({ queryKey: ['elasticsearch-nodes'] })
  expect(await screen.findByText('Elasticsearch 节点列表刷新失败')).toBeVisible()
  expect(screen.getByText('fixture-es-node-a')).toBeVisible()
})

describe('拒绝不完整或类型不安全的节点响应', () => {
  const malformedResponses: Array<[string, (fixture: ElasticsearchNodePageFixture) => unknown]> = [
    ['null nodes', (fixture) => ({ ...fixture, data: { ...fixture.data, nodes: null } })],
    ['missing pagination', (fixture) => {
      const { total_pages: _omitted, ...data } = fixture.data
      return { ...fixture, data }
    }],
    ['invalid role', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, nodes: [{ ...fixture.data.nodes[0], roles: ['invalid'] }] },
    })],
    ['invalid health', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, nodes: [{ ...fixture.data.nodes[0], cluster_health: 'blue' }] },
    })],
    ['invalid status', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, nodes: [{ ...fixture.data.nodes[0], status: 'offline' }] },
    })],
    ['invalid source', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, nodes: [{ ...fixture.data.nodes[0], status_source: 'cluster' }] },
    })],
    ['wrong primitive', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, nodes: [{ ...fixture.data.nodes[0], name: 42 }] },
    })],
    ['invalid options', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, available_roles: ['invalid'] },
    })],
    ['negative heap percentage', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], heap_usage_percent: -0.1 }],
      },
    })],
    ['negative disk percentage', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], disk_usage_percent: -0.1 }],
      },
    })],
    ['negative cpu percentage', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], cpu_usage_percent: -0.1 }],
      },
    })],
    ['negative index rate', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], index_rate: -0.1 }],
      },
    })],
    ['negative search rate', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], search_rate: -0.1 }],
      },
    })],
    ['negative rejected rate', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], rejected_rate: -0.1 }],
      },
    })],
  ]

  it.each(malformedResponses)('%s', async (_name, mutate) => {
    responseBody = mutate(cloneFixture())
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('服务器响应格式无效')
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('拒绝 JSON 解析后产生的非有限数字', async () => {
    server.use(
      http.get(ELASTICSEARCH_NODES_PATH, () => {
        const raw = JSON.stringify(elasticsearchNodePageFixture()).replace(
          '72.5',
          '1e400',
        )
        return new HttpResponse(raw, {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }),
    )
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('服务器响应格式无效')
  })
})

it('真实 App 路由在 /elasticsearch 渲染节点页而非临时壳', async () => {
  server.use(
    http.get(SESSION_PATH, () =>
      HttpResponse.json({
        data: { authenticated: true, username: 'fixture-user' },
        meta: { request_id: 'req-fixture-session-001', stale: false },
      }),
    ),
  )
  window.history.replaceState({}, '', '/elasticsearch')
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: 'Elasticsearch 节点' }),
  ).toBeVisible()
  expect(screen.queryByText(/即将上线|建设中|临时/)).not.toBeInTheDocument()
})
