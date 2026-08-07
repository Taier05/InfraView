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
  JAVA_SERVICES_PATH,
  SESSION_PATH,
  javaErrorFixture,
  javaServicePageFixture,
  type JavaServicePageFixture,
} from '../../test/fixtures'
import { server } from '../../test/server'
import { isJavaServicePageResponse, JavaPage } from './JavaPage'

const exactHeaders = [
  '业务端', '服务地址', '健康检查', '健康延迟', '端口状态',
  '进程状态', '进程数', '端口进程一致性', 'CPU 使用率',
  '内存占用', '内存使用率', '运行时间', '状态',
] as const

const sortFields = [
  'business', 'address', 'health', 'health_latency', 'port', 'process',
  'process_count', 'consistency', 'cpu', 'memory', 'memory_percent',
  'uptime', 'status',
] as const

let responseBody: unknown
let responseStatus: number
let responseDelay: number
const requests: URL[] = []

function renderPage(entry = '/java') {
  window.history.replaceState({}, '', entry)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <JavaPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
  return queryClient
}

function cloneFixture() {
  return structuredClone(javaServicePageFixture())
}

function respondWithRequestedPage() {
  server.use(
    http.get(JAVA_SERVICES_PATH, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      return HttpResponse.json(
        javaServicePageFixture({
          data: {
            page: Number(url.searchParams.get('page')),
            total: 60,
            total_pages: 3,
          },
        }),
      )
    }),
  )
}

beforeEach(() => {
  responseBody = javaServicePageFixture()
  responseStatus = 200
  responseDelay = 0
  requests.length = 0
  server.use(
    http.get(JAVA_SERVICES_PATH, async ({ request }) => {
      requests.push(new URL(request.url))
      if (responseDelay > 0) await delay(responseDelay)
      return HttpResponse.json(responseBody as Record<string, unknown>, {
        status: responseStatus,
      })
    }),
  )
})

afterEach(() => vi.useRealTimers())

it('严格渲染固定顺序的十三个单值单行列', async () => {
  renderPage()

  await screen.findByText('fixture-business-a')
  expect(
    screen.getAllByRole('columnheader').map((cell) => cell.textContent),
  ).toEqual(exactHeaders)
  for (const row of screen.getAllByRole('row').slice(1)) {
    const cells = within(row).getAllByRole('cell')
    expect(cells).toHaveLength(13)
    for (const cell of cells) {
      expect(cell.querySelector('br')).toBeNull()
      expect(getComputedStyle(cell).whiteSpace).toBe('nowrap')
    }
  }
})

it('身份字段单行省略并通过原生 title 保留完整值', async () => {
  const queryClient = renderPage()

  await screen.findByText('fixture-business-a')
  const cells = within(screen.getAllByRole('row')[1]).getAllByRole('cell')
  for (const [cell, value] of [
    [cells[0], 'fixture-business-a'],
    [cells[1], 'fixture-address-a'],
  ] as const) {
    const identity = cell.querySelector('.java-identity')
    expect(identity).toHaveAttribute('title', value)
    expect(getComputedStyle(identity as Element).textOverflow).toBe('ellipsis')
  }
})

it('映射五种已知状态来源并格式化空值', async () => {
  const base = cloneFixture().data.services[0]
  responseBody = javaServicePageFixture({
    data: {
      services: [
        { ...base, id: 'health', health_up: false, status: 'critical', status_source: 'health' },
        { ...base, id: 'port', port_up: false, status: 'critical', status_source: 'port' },
        { ...base, id: 'process', process_up: false, status: 'critical', status_source: 'process' },
        { ...base, id: 'consistency', port_consistent: false, status: 'critical', status_source: 'consistency' },
        { ...base, id: 'collection', status: 'warning', status_source: 'collection', collection_level: 'warning' },
        {
          ...base,
          id: 'empty',
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
          status: 'unknown',
          status_source: 'unknown',
          collection_level: 'unknown',
        },
      ],
      total: 6,
      total_pages: 1,
    },
  })
  renderPage()

  const rows = (await screen.findAllByRole('row')).slice(1)
  expect(rows.slice(0, 5).map((row) => within(row).getAllByRole('cell')[12].textContent)).toEqual([
    '健康检查', '端口状态', '进程状态', '端口进程一致性', '采集状态',
  ])
  const values = within(rows[5]).getAllByRole('cell').map((cell) => cell.textContent)
  expect(values.slice(2, 12)).toEqual(Array.from({ length: 10 }, () => '暂无数据'))
})

it('按约定格式化延迟百分比 IEC 字节进程数与运行时间', async () => {
  const base = cloneFixture().data.services[0]
  responseBody = javaServicePageFixture({
    data: {
      services: [{
        ...base,
        health_latency_ms: 12.5,
        process_count: '12345',
        cpu_usage_percent: 72.55,
        memory_bytes: '2147483648',
        memory_usage_percent: 36,
        uptime_seconds: '90000',
      }],
      total: 1,
      total_pages: 1,
    },
  })
  renderPage()

  const cells = within((await screen.findAllByRole('row'))[1]).getAllByRole('cell')
  expect(cells.map((cell) => cell.textContent)).toEqual([
    'fixture-business-a', 'fixture-address-a', '正常', '12.5 ms', '正常',
    '正常', '12,345', '正常', '72.5%', '2 GiB', '36.0%', '1天 1小时', '正常',
  ])
})

it('无损校验并格式化超过 JS 安全整数和 MaxInt64 边界', async () => {
  const base = cloneFixture().data.services[0]
  responseBody = javaServicePageFixture({
    data: {
      services: [{
        ...base,
        process_count: '9007199254740993',
        memory_bytes: '9223372036854775807',
        uptime_seconds: '9223372036854775807',
      }],
      total: 1,
      total_pages: 1,
    },
  })
  renderPage()

  const cells = within((await screen.findAllByRole('row'))[1]).getAllByRole('cell')
  expect(cells[6]).toHaveTextContent('9,007,199,254,740,993')
  expect(cells[9]).toHaveTextContent('8 EiB')
  expect(cells[11]).toHaveTextContent('106751991167300天 15小时')
})

it('从白名单 URL 恢复筛选排序分页并规范非法参数', async () => {
  responseBody = javaServicePageFixture({
    data: { page: 2, total: 40, total_pages: 2 },
  })
  renderPage('/java?search=fixture&name=fixture-service-b&status=warning&sort=memory&direction=desc&page=2&page_size=50&unknown=value')
  await screen.findByText('fixture-business-a')

  expect(screen.getByRole('searchbox', { name: '搜索业务端、服务名称或地址' })).toHaveValue('fixture')
  expect(screen.getByRole('combobox', { name: '业务端' })).toHaveValue('fixture-service-b')
  expect(screen.getByRole('combobox', { name: '服务状态' })).toHaveValue('warning')
  await waitFor(() => expect(requests.at(-1)?.search).toBe('?search=fixture&name=fixture-service-b&status=warning&sort=memory&direction=desc&page=2&page_size=50'))

  renderPage('/java?name=%20&status=bad&sort=bad&direction=sideways&page=-1&page_size=25&unknown=value')
  await screen.findByText('fixture-business-a')
  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search)
    expect(parameters.toString()).toBe('sort=business&direction=asc&page=2&page_size=20')
  })
})

it('服务端暂缺当前业务端时仍保留筛选选项', async () => {
  responseBody = javaServicePageFixture({
    data: { available_names: ['fixture-service-a'] },
  })
  renderPage('/java?name=fixture-service-removed')
  await screen.findByText('fixture-business-a')

  const select = screen.getByRole('combobox', { name: '业务端' })
  expect(select).toHaveValue('fixture-service-removed')
  expect(within(select).getByRole('option', { name: 'fixture-service-removed' })).toBeVisible()
})

it('以中文标签展示业务端但保留业务代码作为筛选值', async () => {
  responseBody = javaServicePageFixture({
    data: {
      available_names: [
        'tikbee', 'rider', 'mch', 'saas', 'mch_saas', 'future_business', 'constructor',
      ],
    },
  })
  const user = userEvent.setup()
  renderPage()

  await screen.findByText('fixture-business-a')
  const select = screen.getByRole('combobox', { name: '业务端' })
  const expectedOptions = [
    ['用户端', 'tikbee'], ['骑手端', 'rider'], ['商家端', 'mch'],
    ['管理后台端', 'saas'], ['商家 PC 端', 'mch_saas'],
    ['future_business', 'future_business'],
    ['constructor', 'constructor'],
  ]
  for (const [label, value] of expectedOptions) {
    expect(within(select).getByRole('option', { name: label })).toHaveValue(value)
  }

  await user.selectOptions(select, 'rider')
  await waitFor(() => expect(new URLSearchParams(window.location.search).get('name')).toBe('rider'))
  await waitFor(() => expect(requests.at(-1)?.searchParams.get('name')).toBe('rider'))

  await user.selectOptions(select, 'constructor')
  await waitFor(() => expect(new URLSearchParams(window.location.search).get('name')).toBe('constructor'))
  await waitFor(() => expect(requests.at(-1)?.searchParams.get('name')).toBe('constructor'))
})

it('以带等级的徽标展示健康端口和进程的可空二值状态', async () => {
  const base = cloneFixture().data.services[0]
  responseBody = javaServicePageFixture({
    data: {
      services: [{
        ...base,
        id: 'mixed-binary-status',
        health_up: true,
        port_up: false,
        process_up: null,
        status: 'critical',
        status_source: 'port',
      }],
      total: 1,
      total_pages: 1,
    },
  })
  renderPage()

  const cells = within((await screen.findAllByRole('row'))[1]).getAllByRole('cell')
  for (const [index, label, level] of [
    [2, '正常', 'normal'],
    [4, '异常', 'critical'],
    [5, '暂无数据', 'unknown'],
  ] as const) {
    const badge = cells[index].querySelector('.status-badge')
    expect(badge).toHaveTextContent(label)
    expect(badge).toHaveAttribute('data-level', level)
  }
})

it('搜索精确等待 300ms 后写入 URL 并重置页码', async () => {
  respondWithRequestedPage()
  renderPage('/java?page=3')
  await screen.findByText('fixture-business-a')
  vi.useFakeTimers()
  fireEvent.change(screen.getByRole('searchbox', { name: '搜索业务端、服务名称或地址' }), {
    target: { value: 'fixture' },
  })
  act(() => vi.advanceTimersByTime(299))
  expect(new URLSearchParams(window.location.search).has('search')).toBe(false)
  act(() => vi.advanceTimersByTime(1))
  expect(new URLSearchParams(window.location.search).get('search')).toBe('fixture')
  expect(new URLSearchParams(window.location.search).get('page')).toBe('1')
})

it('空白搜索在 URL 和 300ms 输入后都删除且不透传给后端', async () => {
  respondWithRequestedPage()
  renderPage('/java?search=%20&page=3')
  await screen.findByText('fixture-business-a')
  await waitFor(() => {
    expect(new URLSearchParams(window.location.search).has('search')).toBe(false)
    expect(requests.at(-1)?.searchParams.has('search')).toBe(false)
  })

  vi.useFakeTimers()
  fireEvent.change(screen.getByRole('searchbox', { name: '搜索业务端、服务名称或地址' }), {
    target: { value: '   ' },
  })
  act(() => vi.advanceTimersByTime(300))
  expect(new URLSearchParams(window.location.search).has('search')).toBe(false)
  expect(new URLSearchParams(window.location.search).get('page')).toBe('1')
})

it('筛选页数和全部十三个排序键都重置页码', async () => {
  respondWithRequestedPage()
  const user = userEvent.setup()
  renderPage('/java?page=3')
  await screen.findByText('fixture-business-a')

  await user.selectOptions(screen.getByRole('combobox', { name: '业务端' }), 'fixture-service-a')
  await user.selectOptions(screen.getByRole('combobox', { name: '服务状态' }), 'critical')
  await user.selectOptions(screen.getByRole('combobox', { name: '每页数量' }), '100')
  for (const [index, header] of exactHeaders.entries()) {
    await user.click(screen.getByRole('button', { name: new RegExp(`^${header}排序`) }))
    await waitFor(() => expect(requests.at(-1)?.searchParams.get('sort')).toBe(sortFields[index]))
  }
  expect(new URLSearchParams(window.location.search).get('page')).toBe('1')
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('100')
})

it('展示初始加载、空、过期和后台刷新错误状态', async () => {
  responseDelay = 20
  responseBody = javaServicePageFixture({
    data: { services: [], total: 0, total_pages: 0 },
    meta: { stale: true },
  })
  const queryClient = renderPage()
  expect(screen.getByRole('status')).toHaveTextContent('正在加载 Java 业务服务')
  expect(await screen.findByText('没有符合条件的 Java 业务服务')).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent('数据已过期')

  responseBody = javaErrorFixture()
  responseStatus = 503
  await queryClient.invalidateQueries({ queryKey: ['java-services'] })
  expect(await screen.findByText('Java 业务服务列表刷新失败')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
})

it('初次错误可重试，且不安全响应被拒绝', async () => {
  responseBody = javaErrorFixture()
  responseStatus = 503
  const user = userEvent.setup()
  const queryClient = renderPage()
  expect(await screen.findByRole('alert')).toHaveTextContent('Java 业务服务列表加载失败')

  responseBody = javaServicePageFixture()
  responseStatus = 200
  await user.click(screen.getByRole('button', { name: '重试 Java 业务服务列表' }))
  expect(await screen.findByText('fixture-business-a')).toBeVisible()

  responseBody = { ...cloneFixture(), data: { services: [{ id: 42 }] } }
  await queryClient.invalidateQueries({ queryKey: ['java-services'] })
  expect(await screen.findByText('服务器响应格式无效')).toBeVisible()
})

it('真实 App 路由在 /java 渲染业务服务页而非临时壳', async () => {
  server.use(
    http.get(SESSION_PATH, () => HttpResponse.json({
      data: { authenticated: true, username: 'fixture-user' },
      meta: { request_id: 'req-fixture-session-java-001', stale: false },
    })),
  )
  window.history.replaceState({}, '', '/java')
  render(<App />)

  expect(await screen.findByRole('heading', { name: 'Java 业务服务' })).toBeVisible()
  expect(screen.queryByText(/即将上线|建设中|临时/)).not.toBeInTheDocument()
})

describe('严格响应校验', () => {
  const malformedResponses: Array<[string, (fixture: JavaServicePageFixture) => unknown]> = [
    ['services 非数组', (fixture) => ({ ...fixture, data: { ...fixture.data, services: null } })],
    ['未知状态等级', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], status: 'offline' }] } })],
    ['整数仍使用 JSON number', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], process_count: 1 }] } })],
    ['负数字符串', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], process_count: '-1' }] } })],
    ['科学计数法字符串', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], memory_bytes: '1e3' }] } })],
    ['带符号字符串', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], uptime_seconds: '+1' }] } })],
    ['非规范前导零', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], process_count: '01' }] } })],
    ['超过 MaxInt64', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], process_count: '9223372036854775808' }] } })],
    ['分页缺失', (fixture) => { const { total_pages: _omitted, ...data } = fixture.data; return { ...fixture, data } }],
    ['页内服务超出页大小', (fixture) => ({ ...fixture, data: { ...fixture.data, services: Array.from({ length: 21 }, (_, index) => ({ ...fixture.data.services[0], id: `java-${index}` })), total: 21, page_size: 20, total_pages: 2 } })],
    ['未知状态来源', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], status_source: 'future_source' }] } })],
    ['critical 搭配 normal 来源', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], status: 'critical', status_source: 'normal' }] } })],
    ['normal 搭配 health 来源', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], status_source: 'health' }] } })],
    ['normal 搭配 critical 采集等级', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], collection_level: 'critical' }] } })],
    ['port 来源遗漏对应失败信号', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], status: 'critical', status_source: 'port' }] } })],
    ['port 来源被更高优先级 health 失败遮蔽', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], health_up: false, port_up: false, status: 'critical', status_source: 'port' }] } })],
    ['collection 来源被同级 health 失败遮蔽', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], health_up: false, status: 'critical', status_source: 'collection', collection_level: 'critical' }] } })],
    ['normal 来源未保持所有信号正常', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[0], process_up: null }] } })],
    ['unknown 来源没有缺失信号', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[3], health_up: true, port_up: true, process_up: true, port_consistent: true }] } })],
    ['unknown 来源含失败信号', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[3], health_up: false }] } })],
    ['unknown 来源采集等级高于 unknown', (fixture) => ({ ...fixture, data: { ...fixture.data, services: [{ ...fixture.data.services[3], collection_level: 'warning' }] } })],
  ]

  it.each(malformedResponses)('%s', async (_name, mutate) => {
    responseBody = mutate(cloneFixture())
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('服务器响应格式无效')
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it.each([
    ['CPU 负数', { cpu_usage_percent: -1 }],
    ['CPU 超过 100', { cpu_usage_percent: 101 }],
    ['内存负数', { memory_usage_percent: -1 }],
    ['内存超过 100', { memory_usage_percent: 101 }],
  ])('%s', (_name, override) => {
    const fixture = cloneFixture()
    fixture.data.services[0] = { ...fixture.data.services[0], ...override }
    expect(isJavaServicePageResponse(fixture)).toBe(false)
  })

  it.each([
    ['CPU NaN', { cpu_usage_percent: Number.NaN }],
    ['CPU Infinity', { cpu_usage_percent: Number.POSITIVE_INFINITY }],
    ['内存 NaN', { memory_usage_percent: Number.NaN }],
    ['内存 Infinity', { memory_usage_percent: Number.NEGATIVE_INFINITY }],
  ])('%s', (_name, override) => {
    const fixture = cloneFixture()
    fixture.data.services[0] = { ...fixture.data.services[0], ...override }
    expect(isJavaServicePageResponse(fixture)).toBe(false)
  })
})
