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
  RABBITMQ_NODES_PATH,
  SESSION_PATH,
  rabbitMQNodePageFixture,
  type RabbitMQNodePageFixture,
} from '../../test/fixtures'
import { server } from '../../test/server'
import { RabbitMQPage } from './RabbitMQPage'

const expectedHeaders = [
  '节点名称',
  '所属集群',
  '实例地址',
  '版本',
  '内存使用率',
  '磁盘余量',
  '文件描述符使用率',
  'Erlang进程使用率',
  '连接',
  '队列',
  '消息积压',
  '发布速率',
  '投递速率',
  '运行时间',
  '状态',
] as const

const sortFields = [
  'node',
  'cluster',
  'address',
  'version',
  'memory',
  'disk',
  'file_descriptors',
  'erlang_processes',
  'connections',
  'queues',
  'messages',
  'publish_rate',
  'deliver_rate',
  'uptime',
  'status',
] as const

let responseBody: unknown
let responseStatus: number
let responseDelay: number
const requests: URL[] = []

function renderPage(entry = '/rabbitmq') {
  window.history.replaceState({}, '', entry)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  const result = render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <RabbitMQPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
  return { ...result, queryClient }
}

function cloneFixture() {
  return structuredClone(rabbitMQNodePageFixture())
}

function errorFixture() {
  return {
    code: 'rabbitmq_unavailable',
    message: '数据源暂时不可用，请稍后重试',
    request_id: 'req-fixture-rabbitmq-error-001',
    retryable: true,
  }
}

function respondWithRequestedPage() {
  server.use(
    http.get(RABBITMQ_NODES_PATH, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      const pageSize = Number(url.searchParams.get('page_size'))
      const total = pageSize === 500 ? 1001 : 60
      return HttpResponse.json(
        rabbitMQNodePageFixture({
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
  responseBody = rabbitMQNodePageFixture()
  responseStatus = 200
  responseDelay = 0
  requests.length = 0
  server.use(
    http.get(RABBITMQ_NODES_PATH, async ({ request }) => {
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

it('严格渲染固定顺序的十五个单值单行列', async () => {
  const { container } = renderPage()

  await screen.findByText('fixture-rabbit-node-normal')
  expect(
    screen.getAllByRole('columnheader').map((cell) => cell.textContent),
  ).toEqual(expectedHeaders)
  const rows = screen.getAllByRole('row').slice(1)
  expect(rows).toHaveLength(4)
  for (const row of rows) {
    const cells = within(row).getAllByRole('cell')
    expect(cells).toHaveLength(15)
    for (const [index, cell] of cells.entries()) {
      expect(cell.querySelector('br')).toBeNull()
      expect(cell.querySelector('small, .secondary-value')).toBeNull()
      expect(getComputedStyle(cell).whiteSpace).toBe('nowrap')
      const titledValue = cell.querySelector('[title]')
      expect(titledValue).not.toBeNull()
      if (index === 14) {
        expect(titledValue?.getAttribute('title')).toMatch(/^状态来源：/)
      } else {
        expect(titledValue).toHaveAttribute('title', cell.textContent ?? '')
      }
    }
  }
  expect(container.querySelectorAll('tbody td[title]')).toHaveLength(0)
})

it('身份字段单行省略并通过原生 title 保留完整值', async () => {
  renderPage()

  await screen.findByText('fixture-rabbit-node-normal')
  const firstRow = screen.getAllByRole('row')[1]
  const cells = within(firstRow).getAllByRole('cell')
  const identities = cells.slice(0, 3).map((cell) =>
    cell.querySelector('.rabbitmq-identity'),
  )
  expect(identities[0]).toHaveAttribute('title', 'fixture-rabbit-node-normal')
  expect(identities[1]).toHaveAttribute('title', 'fixture-rabbit-cluster-a')
  expect(identities[2]).toHaveAttribute('title', '192.0.2.41:15692')
  for (const identity of identities) {
    expect(getComputedStyle(identity as Element).textOverflow).toBe('ellipsis')
    expect(getComputedStyle(identity as Element).whiteSpace).toBe('nowrap')
  }
})

it('节点名称缺失时显示暂无数据且不复用实例地址', async () => {
  const fixture = cloneFixture()
  fixture.data.nodes = [
    {
      ...fixture.data.nodes[0],
      name: '',
    },
  ]
  fixture.data.total = 1
  fixture.data.total_pages = 1
  responseBody = fixture
  renderPage()

  const table = await screen.findByRole('table', { name: 'RabbitMQ 节点列表' })
  const cells = within(within(table).getAllByRole('row')[1]).getAllByRole('cell')
  expect(cells[0]).toHaveTextContent('暂无数据')
  expect(cells[2]).toHaveTextContent(fixture.data.nodes[0].address)
  expect(cells[0].textContent).not.toBe(cells[2].textContent)
})

it('只用共享列表模板在同一控制行渲染搜索筛选页数与最新数据时间', async () => {
  renderPage()

  const search = await screen.findByRole('searchbox', {
    name: '搜索节点名称或地址',
  })
  const controls = search.closest('.host-list-controls')
  expect(controls).not.toBeNull()
  if (!(controls instanceof HTMLElement)) {
    throw new Error('RabbitMQ 控制区未渲染为 HTML 元素')
  }
  expect(controls).toHaveClass('host-list-controls')
  expect(controls).not.toHaveClass('rabbitmq-list-controls')
  expect(getComputedStyle(controls).gridAutoFlow).toBe('column')
  expect(within(controls).getAllByRole('searchbox')).toHaveLength(1)
  expect(within(controls).getAllByRole('combobox')).toHaveLength(3)
  for (const label of ['所属集群', '节点状态', '每页数量']) {
    expect(within(controls).getByRole('combobox', { name: label })).toBeVisible()
  }
  const dataTime = await within(controls).findByText('2026/08/04 08:00:00')
  expect(dataTime.closest('.data-time')).toHaveTextContent('最新数据时间：2026/08/04 08:00:00')
  expect(within(controls).queryByRole('button', { name: /刷新/ })).not.toBeInTheDocument()
  expect(within(controls).queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument()
  expect(controls.querySelector('.rabbitmq-search, .rabbitmq-select')).toBeNull()
  const table = await screen.findByRole('table', { name: 'RabbitMQ 节点列表' })
  expect(table).toHaveClass('host-table', 'rabbitmq-table', 'observability-table')
  expect(table.closest('.host-table-scroll')).toHaveClass(
    'rabbitmq-table-scroll',
  )
  expect(table.closest('.host-table-panel')).not.toBeNull()
})

it('服务端选项暂缺当前集群时仍保留 URL 与下拉框选择', async () => {
  responseBody = rabbitMQNodePageFixture({
    data: { available_clusters: ['fixture-rabbit-cluster-a'] },
  })
  renderPage('/rabbitmq?cluster=fixture-rabbit-cluster-removed')
  await screen.findByText('fixture-rabbit-node-normal')

  const clusterSelect = screen.getByRole('combobox', { name: '所属集群' })
  expect(clusterSelect).toHaveValue('fixture-rabbit-cluster-removed')
  expect(
    within(clusterSelect).getByRole('option', {
      name: 'fixture-rabbit-cluster-removed',
    }),
  ).toBeInTheDocument()
  expect(new URLSearchParams(window.location.search).get('cluster')).toBe(
    'fixture-rabbit-cluster-removed',
  )
  expect(requests.at(-1)?.searchParams.get('cluster')).toBe(
    'fixture-rabbit-cluster-removed',
  )
})

it('从白名单 URL 恢复筛选排序分页', async () => {
  renderPage(
    '/rabbitmq?search=fixture&cluster=fixture-rabbit-cluster-b&status=warning&sort=messages&direction=desc&page=2&page_size=50&unknown=value',
  )
  await screen.findByText('fixture-rabbit-node-normal')

  expect(screen.getByRole('searchbox')).toHaveValue('fixture')
  expect(screen.getByRole('combobox', { name: '所属集群' })).toHaveValue(
    'fixture-rabbit-cluster-b',
  )
  expect(screen.getByRole('combobox', { name: '节点状态' })).toHaveValue(
    'warning',
  )
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')
  await waitFor(() => {
    const parameters = requests.at(-1)?.searchParams
    expect(parameters?.get('search')).toBe('fixture')
    expect(parameters?.get('cluster')).toBe('fixture-rabbit-cluster-b')
    expect(parameters?.get('status')).toBe('warning')
    expect(parameters?.get('sort')).toBe('messages')
    expect(parameters?.get('direction')).toBe('desc')
    expect(parameters?.get('page')).toBe('2')
    expect(parameters?.get('page_size')).toBe('50')
    expect(parameters?.has('unknown')).toBe(false)
  })
  expect(screen.queryByRole('button', { name: /刷新/ })).not.toBeInTheDocument()
})

it('搜索精确等待 300ms 后写入 URL 并重置页码', async () => {
  respondWithRequestedPage()
  renderPage('/rabbitmq?page=3')
  await screen.findByText('fixture-rabbit-node-normal')
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

it('筛选与每页数量都重置页码且请求只含白名单参数', async () => {
  respondWithRequestedPage()
  const user = userEvent.setup()
  renderPage('/rabbitmq?page=3&page_size=500')
  expect(await screen.findByText('第 3 / 3 页，共 1001 个节点')).toBeVisible()
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('500')
  expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
    sort: 'node',
    direction: 'asc',
    page: '3',
    page_size: '500',
  })

  await user.selectOptions(
    screen.getByRole('combobox', { name: '所属集群' }),
    'fixture-rabbit-cluster-a',
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: '节点状态' }),
    'critical',
  )
  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search)
    expect(parameters.get('cluster')).toBe('fixture-rabbit-cluster-a')
    expect(parameters.get('status')).toBe('critical')
    expect(parameters.get('page_size')).toBe('500')
    expect(parameters.get('page')).toBe('1')
  })
  await waitFor(() => {
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      cluster: 'fixture-rabbit-cluster-a',
      status: 'critical',
      sort: 'node',
      direction: 'asc',
      page: '1',
      page_size: '500',
    })
  })
})

it('十五个表头使用精确排序白名单并切换 direction', async () => {
  const user = userEvent.setup()
  renderPage()
  await screen.findByText('fixture-rabbit-node-normal')

  for (const [index, header] of expectedHeaders.entries()) {
    await user.click(
      screen.getByRole('button', { name: new RegExp(`^${header}排序`) }),
    )
    await waitFor(() =>
      expect(requests.at(-1)?.searchParams.get('sort')).toBe(sortFields[index]),
    )
    expect(new URLSearchParams(window.location.search).get('page')).toBe('1')
  }
  expect(requests.at(-1)?.searchParams.get('direction')).toBe('asc')
  await user.click(screen.getByRole('button', { name: /^状态排序/ }))
  await waitFor(() =>
    expect(requests.at(-1)?.searchParams.get('direction')).toBe('desc'),
  )
})

it('规范非法 URL 参数并按服务端结果修正越界页码', async () => {
  responseBody = rabbitMQNodePageFixture({
    data: { page: 2, total: 40, total_pages: 2 },
  })
  renderPage(
    '/rabbitmq?status=bad&sort=bad&direction=sideways&page=-4&page_size=25&unknown=value',
  )
  await screen.findByText('fixture-rabbit-node-normal')

  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search)
    expect(parameters.get('sort')).toBe('node')
    expect(parameters.get('direction')).toBe('asc')
    expect(parameters.get('page')).toBe('2')
    expect(parameters.get('page_size')).toBe('20')
    expect(parameters.has('status')).toBe(false)
    expect(parameters.has('unknown')).toBe(false)
  })
})

it.each([499, 501])('将非法 page_size=%i 规范为 20 且回到第一页', async (pageSize) => {
  respondWithRequestedPage()
  renderPage(`/rabbitmq?page=3&page_size=${pageSize}`)

  await screen.findByText('fixture-rabbit-node-normal')
  await waitFor(() => {
    expect(window.location.search).toContain('page=1&page_size=20')
    expect(requests.at(-1)?.searchParams.get('page_size')).toBe('20')
  })
})

it('按约定格式化百分比 IEC 计数速率空值与运行时间', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = rabbitMQNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          memory_usage_percent: 72.55,
          disk_available_bytes: 2 * 1024 ** 3,
          file_descriptor_usage_percent: 81,
          erlang_process_usage_percent: 36.54,
          connections: 12_345,
          queues: 8,
          messages: 1_200,
          publish_rate: 14.25,
          deliver_rate: 28,
          uptime_seconds: 90_000,
        },
        {
          ...base,
          id: 'rabbitmq-fixture-node-empty',
          name: 'fixture-rabbit-node-empty',
          memory_usage_percent: null,
          disk_available_bytes: null,
          file_descriptor_usage_percent: null,
          erlang_process_usage_percent: null,
          connections: null,
          queues: null,
          messages: null,
          publish_rate: null,
          deliver_rate: null,
          uptime_seconds: null,
        },
        {
          ...base,
          id: 'rabbitmq-fixture-node-days',
          name: 'fixture-rabbit-node-days',
          uptime_seconds: 172_800,
        },
        {
          ...base,
          id: 'rabbitmq-fixture-node-hours',
          name: 'fixture-rabbit-node-hours',
          uptime_seconds: 7_200,
        },
      ],
      total: 4,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  expect(within(rows[0]).getAllByRole('cell').map((cell) => cell.textContent)).toEqual([
    'fixture-rabbit-node-normal',
    'fixture-rabbit-cluster-a',
    '192.0.2.41:15692',
    'fixture-rabbit-4.0',
    '72.5%',
    '2 GiB',
    '81.0%',
    '36.5%',
    '12,345',
    '8',
    '1,200',
    '14.25/s',
    '28/s',
    '1天 1小时',
    '正常',
  ])
  expect(
    within(rows[1]).getAllByRole('cell').slice(4, 14).map((cell) => cell.textContent),
  ).toEqual(Array.from({ length: 10 }, () => '暂无数据'))
  expect(within(rows[2]).getAllByRole('cell')[13]).toHaveTextContent('2天')
  expect(within(rows[3]).getAllByRole('cell')[13]).toHaveTextContent('2小时')
})

it('状态使用四色共享徽标并在 title 解释胜出来源', async () => {
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  const expected = [
    ['normal', '正常', '状态来源：正常'],
    ['warning', '内存', '状态来源：内存'],
    ['critical', '资源告警', '状态来源：资源告警'],
    ['unknown', '未知', '状态来源：未知'],
  ] as const
  for (const [index, [level, text, title]] of expected.entries()) {
    const cell = within(rows[index]).getAllByRole('cell')[14]
    expect(cell).toHaveTextContent(text)
    expect(cell.querySelector('.status-badge')).toHaveAttribute(
      'data-level',
      level,
    )
    expect(cell.querySelector('.rabbitmq-status')).toHaveAttribute('title', title)
  }
})

it('展示初始加载、过期数据和空结果状态', async () => {
  responseDelay = 20
  responseBody = rabbitMQNodePageFixture({
    data: { nodes: [], total: 0, total_pages: 0 },
    meta: { stale: true },
  })
  renderPage()

  expect(screen.getByRole('status')).toHaveTextContent('正在加载 RabbitMQ 节点')
  expect(await screen.findByText('没有符合条件的 RabbitMQ 节点')).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent('数据已过期')
  expect(screen.getByLabelText('RabbitMQ 节点列表分页')).toHaveTextContent(
    '暂无节点',
  )
})

it('初次 503 可重试且刷新错误保留旧数据', async () => {
  responseBody = errorFixture()
  responseStatus = 503
  const user = userEvent.setup()
  const { queryClient } = renderPage()

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'RabbitMQ 节点列表加载失败',
  )
  responseBody = rabbitMQNodePageFixture()
  responseStatus = 200
  await user.click(screen.getByRole('button', { name: '重试 RabbitMQ 节点列表' }))
  await waitFor(() =>
    expect(screen.getByText('fixture-rabbit-node-normal')).toBeVisible(),
  )

  responseBody = errorFixture()
  responseStatus = 503
  await queryClient.invalidateQueries({ queryKey: ['rabbitmq-nodes'] })
  expect(await screen.findByText('RabbitMQ 节点列表刷新失败')).toBeVisible()
  expect(screen.getByText('fixture-rabbit-node-normal')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
  expect(screen.getByText(/当前展示最近一次可用数据/)).toBeVisible()
})

describe('拒绝不完整或类型不安全的节点响应', () => {
  const malformedResponses: Array<
    [string, (fixture: RabbitMQNodePageFixture) => unknown]
  > = [
    ['null nodes', (fixture) => ({ ...fixture, data: { ...fixture.data, nodes: null } })],
    ['invalid status', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], status: 'offline' }],
      },
    })],
    ['invalid source', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], status_source: 'network' }],
      },
    })],
    ['negative metric', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], messages: -1 }],
      },
    })],
    ['missing pagination', (fixture) => {
      const { total_pages: _omitted, ...data } = fixture.data
      return { ...fixture, data }
    }],
    ['more nodes than page size', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: Array.from({ length: 21 }, (_, index) => ({
          ...fixture.data.nodes[0],
          id: `rabbitmq-fixture-page-node-${index}`,
        })),
        total: 21,
        page_size: 20,
        total_pages: 2,
      },
    })],
    ['more nodes than total', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, total: 3, total_pages: 1 },
    })],
    ['inconsistent total pages', (fixture) => ({
      ...fixture,
      data: { ...fixture.data, total_pages: 2 },
    })],
    ['normal status with resource source', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{ ...fixture.data.nodes[0], status_source: 'memory' }],
        total: 1,
      },
    })],
    ['collection source disagrees with collection level', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{
          ...fixture.data.nodes[0],
          status: 'critical',
          status_source: 'collection',
          collection_level: 'warning',
        }],
        total: 1,
      },
    })],
    ['resource source cannot tie critical collection', (fixture) => ({
      ...fixture,
      data: {
        ...fixture.data,
        nodes: [{
          ...fixture.data.nodes[0],
          status: 'critical',
          status_source: 'disk',
          collection_level: 'critical',
        }],
        total: 1,
      },
    })],
  ]

  it.each(malformedResponses)('%s', async (_name, mutate) => {
    responseBody = mutate(cloneFixture())
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '服务器响应格式无效',
    )
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })
})

it('接受 Service 可能输出的高优先级状态组合', async () => {
  const base = cloneFixture().data.nodes[0]
  responseBody = rabbitMQNodePageFixture({
    data: {
      nodes: [
        {
          ...base,
          id: 'rabbitmq-legal-alarm',
          status: 'critical',
          status_source: 'alarm',
          collection_level: 'critical',
        },
        {
          ...base,
          id: 'rabbitmq-legal-resource',
          status: 'critical',
          status_source: 'memory',
          collection_level: 'warning',
        },
        {
          ...base,
          id: 'rabbitmq-legal-collection',
          status: 'warning',
          status_source: 'collection',
          collection_level: 'warning',
        },
      ],
      total: 3,
      total_pages: 1,
    },
  })
  renderPage()

  expect(await screen.findByRole('table', { name: 'RabbitMQ 节点列表' })).toBeVisible()
  expect(screen.queryByText('服务器响应格式无效')).not.toBeInTheDocument()
})

it('真实 App 路由在 /rabbitmq 渲染节点页而非临时壳', async () => {
  server.use(
    http.get(SESSION_PATH, () =>
      HttpResponse.json({
        data: { authenticated: true, username: 'fixture-user' },
        meta: { request_id: 'req-fixture-session-rabbitmq-001', stale: false },
      }),
    ),
  )
  window.history.replaceState({}, '', '/rabbitmq')
  render(<App />)

  expect(
    await screen.findByRole('heading', { name: 'RabbitMQ 节点' }),
  ).toBeVisible()
  await waitFor(() =>
    expect(screen.getByText('fixture-rabbit-node-normal')).toBeVisible(),
  )
  expect(screen.queryByText(/即将上线|建设中|临时/)).not.toBeInTheDocument()
})
