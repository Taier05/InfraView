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
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { diskDevicePageFixture } from '../../test/fixtures'
import { DiskPage } from './DiskPage'

const requestedURLs: URL[] = []

const expectedDiskHeaders = [
  '主机',
  '设备',
  '型号',
  '容量',
  'SMART 健康',
  '温度',
  '寿命',
  '通电时间',
  '错误摘要',
  '状态',
] as const

const expectedDiskSorts = [
  ['主机', 'host'],
  ['设备', 'device'],
  ['型号', 'model'],
  ['容量', 'capacity'],
  ['SMART 健康', 'smart'],
  ['温度', 'temperature'],
  ['寿命', 'lifetime'],
  ['通电时间', 'power_on_hours'],
  ['错误摘要', 'errors'],
  ['状态', 'status'],
] as const

function requestURL(input: RequestInfo | URL) {
  const rawURL =
    typeof input === 'string'
      ? input
      : input instanceof URL
        ? input.href
        : input.url
  return new URL(rawURL, 'http://localhost')
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderDiskPage(initialEntry = '/disks') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  })
  window.history.replaceState({}, '', initialEntry)

  render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/disks" element={<DiskPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  )
  return queryClient
}

function lastRequest() {
  const url = requestedURLs.at(-1)
  if (url === undefined) throw new Error('尚未发送硬盘设备列表请求')
  return url
}

function expectRequestParameters(
  url: URL,
  expected: Record<string, string>,
) {
  expect(url.pathname).toBe('/api/v1/disks/devices')
  expect(Object.fromEntries(url.searchParams)).toEqual(expected)
}

function diskPageFixtureForDisplayTests() {
  const fixture = diskDevicePageFixture()
  const alpha = fixture.data.devices[0]
  alpha.model = 'Atlas Enterprise NVMe Fixture Model 2TB'
  alpha.errors = {
    ...alpha.errors,
    uncorrectable_sectors: 3,
    udma_crc_errors: 0,
    unsafe_shutdowns: 7,
  }
  return fixture
}

beforeEach(() => {
  requestedURLs.length = 0
  window.history.replaceState({}, '', '/')
  vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const page = Number(url.searchParams.get('page') ?? '1')
    const pageSize = Number(url.searchParams.get('page_size') ?? '20')
    const total = pageSize === 500 ? 1001 : 45
    return Promise.resolve(
      jsonResponse(
        (() => {
          const fixture = diskPageFixtureForDisplayTests()
          fixture.data.page = page
          fixture.data.page_size = pageSize
          fixture.data.total = total
          fixture.data.total_pages = Math.ceil(total / pageSize)
          return fixture
        })(),
      ),
    )
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('默认请求按主机升序并发送完整分页参数', async () => {
  renderDiskPage()

  await screen.findByText('node-alpha')
  expectRequestParameters(lastRequest(), {
    sort: 'host',
    order: 'asc',
    page: '1',
    page_size: '20',
  })
})

it('严格渲染十个可排序单值单行列，并使用共享列表壳', async () => {
  renderDiskPage()

  expect(await screen.findByRole('heading', { name: '硬盘设备' })).toBeVisible()
  await screen.findByText('node-alpha')
  const headers = screen.getAllByRole('columnheader')
  expect(headers.map((header) => header.textContent)).toEqual(
    expectedDiskHeaders,
  )
  for (const header of headers) {
    expect(within(header).getByRole('button')).toHaveClass('host-sort-button')
    expect(header.textContent).not.toMatch(/[⇅↑↓]/)
  }
  const table = screen.getByRole('table')
  expect(table).toHaveClass('host-table', 'disk-table', 'observability-table')
  const scrollOwner = table.closest('.host-table-scroll')
  expect(scrollOwner).not.toBeNull()
  expect(scrollOwner?.parentElement).toHaveClass('host-table-panel')
  expect(scrollOwner?.parentElement?.querySelectorAll('.host-table-scroll')).toHaveLength(1)

  const healthyRow = screen.getByText('node-alpha').closest('tr')
  expect(healthyRow).not.toBeNull()
  const healthyCells = within(healthyRow!).getAllByRole('cell')
  expect(healthyCells).toHaveLength(10)

  for (const cell of healthyCells) {
    expect(cell.querySelector('br')).toBeNull()
  }
	  expect(healthyCells[1]).toHaveTextContent('/dev/nvme0n1')
	  const modelCell = healthyCells[2]
	  const capacityCell = healthyCells[3]
	  expect(
	    within(modelCell).getByText('Atlas Enterprise NVMe Fixture Model 2TB'),
	  ).toHaveClass('disk-model')
	  expect(
	    within(modelCell).getByText('Atlas Enterprise NVMe Fixture Model 2TB'),
	  ).toHaveAttribute('title', 'Atlas Enterprise NVMe Fixture Model 2TB')
	  expect(within(capacityCell).getByText('2 TiB')).toHaveClass(
	    'disk-capacity',
	  )
	  expect(within(capacityCell).getByText('2 TiB')).toHaveAttribute(
	    'title',
	    '2 TiB',
	  )
	  expect(healthyCells[4]).toHaveTextContent('SMART 正常')
	  expect(healthyCells[5]).toHaveTextContent('42.5°C')
	  expect(healthyCells[6]).toHaveTextContent('已用 17.4%')
	  expect(healthyCells[7]).toHaveTextContent('2天 2小时')
	  expect(within(healthyCells[4]).getByTitle('SMART 正常')).toBeVisible()
	  expect(within(healthyCells[5]).getByTitle('42.5°C')).toBeVisible()
	  expect(within(healthyCells[6]).getByTitle('已用 17.4%')).toBeVisible()
	  expect(within(healthyCells[7]).getByTitle('2天 2小时')).toBeVisible()

  const failedRow = screen.getByText('node-beta').closest('tr')
  expect(failedRow).not.toBeNull()
	  expect(within(failedRow!).getAllByRole('cell')[4]).toHaveTextContent(
    'SMART 失败',
  )

  const unknownRow = screen.getByText('node-gamma').closest('tr')
  expect(unknownRow).not.toBeNull()
	  expect(within(unknownRow!).getAllByRole('cell')[4]).toHaveTextContent(
    'SMART 未知',
  )
  const unknownCells = within(unknownRow!).getAllByRole('cell')
  expect(unknownCells[2]).toHaveTextContent('Cirrus Virtual Disk')
	  expect(within(unknownCells[3]).getByText('暂无数据')).toHaveClass(
	    'disk-capacity',
	  )
	  expect(unknownCells[6]).toHaveTextContent('暂无数据')
  expect(within(unknownRow!).getAllByText('暂无数据').length).toBeGreaterThan(0)
})

it('以共享时长格式显示通电小时、保留完整 title，并保持原始排序参数', async () => {
  const fixture = diskPageFixtureForDisplayTests()
  const powerOnHours = [50, 1.5, 8_766, 0, null] as const
  fixture.data.devices = fixture.data.devices.slice(0, powerOnHours.length).map(
    (device, index) => ({
      ...device,
      host: `node-duration-${index + 1}`,
      power_on_hours: powerOnHours[index],
    }),
  )
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    requestedURLs.push(requestURL(input))
    return Promise.resolve(jsonResponse(fixture))
  })

  const user = userEvent.setup()
  renderDiskPage('/disks?sort=host&order=asc&page=3&page_size=20')

  const expected = [
    ['node-duration-1', '2天 2小时'],
    ['node-duration-2', '1小时 30分钟'],
    ['node-duration-3', '1年 6小时'],
    ['node-duration-4', '不足1分钟'],
    ['node-duration-5', '暂无数据'],
  ] as const
  for (const [host, duration] of expected) {
    const row = (await screen.findByText(host)).closest('tr')
    expect(row).not.toBeNull()
    const cell = within(row!).getAllByRole('cell')[7]
    expect(cell).toHaveTextContent(duration)
    expect(cell.firstElementChild).toHaveAttribute('title', duration)
  }

  await user.click(screen.getByRole('button', { name: '通电时间排序，当前未排序' }))
  await waitFor(() => {
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      sort: 'power_on_hours',
      order: 'asc',
      page: '1',
      page_size: '20',
    })
  })
})

it('型号缺失时独立显示暂无数据且不影响容量', async () => {
  const fixture = diskPageFixtureForDisplayTests()
  fixture.data.devices = [
    {
      ...fixture.data.devices[0],
      host: 'node-model-missing',
      model: '',
      errors: {
        pending_sectors: 0,
        reallocated_sectors: 0,
        uncorrectable_sectors: 0,
        udma_crc_errors: 0,
        media_integrity_errors: 0,
        error_log_entries: 0,
        unsafe_shutdowns: 0,
      },
    },
  ]
  vi.mocked(globalThis.fetch).mockImplementation(() =>
    Promise.resolve(jsonResponse(fixture)),
  )

  renderDiskPage()

  const row = (await screen.findByText('node-model-missing')).closest('tr')
  expect(row).not.toBeNull()
	  const cells = within(row!).getAllByRole('cell')
	  expect(within(cells[2]).getByText('暂无数据')).toHaveClass(
	    'disk-model',
	  )
	  expect(within(cells[2]).getByText('暂无数据')).toHaveAttribute(
	    'title',
	    '暂无数据',
	  )
	  expect(within(cells[3]).getByText('2 TiB')).toHaveClass(
	    'disk-capacity',
	  )
})

it('错误摘要最多展示两个非零项并在 title 保留全部已知非零项且不求和', async () => {
  renderDiskPage()

  const row = (await screen.findByText('node-alpha')).closest('tr')
  expect(row).not.toBeNull()
	  const errorSummary = within(row!).getAllByRole('cell')[8]
  expect(errorSummary).toHaveTextContent('待处理扇区 2 · 重映射扇区 1')
  expect(errorSummary).not.toHaveTextContent('不可校正扇区 3')
  expect(errorSummary).not.toHaveTextContent('异常断电 7 次')
  expect(errorSummary.firstElementChild).toHaveAttribute(
    'title',
    '待处理扇区 2 · 重映射扇区 1 · 不可校正扇区 3 · 异常断电 7 次（累计次数，仅展示，不参与状态判断）',
  )
  expect(errorSummary).not.toHaveTextContent('总计')
})

it('仅异常断电时以次数展示且不再使用非安全关机文案', async () => {
  const fixture = diskPageFixtureForDisplayTests()
  fixture.data.devices = [
    {
      ...fixture.data.devices[0],
      host: 'node-unsafe-only',
      errors: {
        pending_sectors: 0,
        reallocated_sectors: 0,
        uncorrectable_sectors: 0,
        udma_crc_errors: 0,
        media_integrity_errors: 0,
        error_log_entries: 0,
        unsafe_shutdowns: 12,
      },
    },
  ]
  vi.mocked(globalThis.fetch).mockImplementation(() =>
    Promise.resolve(jsonResponse(fixture)),
  )

  renderDiskPage()

  const row = (await screen.findByText('node-unsafe-only')).closest('tr')
  expect(row).not.toBeNull()
	  const errorSummary = within(row!).getAllByRole('cell')[8]
  expect(errorSummary).toHaveTextContent('异常断电 12 次')
  expect(errorSummary.firstElementChild).toHaveAttribute(
    'title',
    '异常断电 12 次（累计次数，仅展示，不参与状态判断）',
  )
  expect(screen.queryByText(/非安全关机/)).not.toBeInTheDocument()
})

it('错误摘要精确区分全零、全缺失和部分缺失', async () => {
  renderDiskPage()

  const allZero = (await screen.findByText('node-beta')).closest('tr')
  const allMissing = screen.getByText('node-gamma').closest('tr')
  const partial = screen.getByText('node-delta').closest('tr')
  expect(allZero).not.toBeNull()
  expect(allMissing).not.toBeNull()
  expect(partial).not.toBeNull()
	  expect(within(allZero!).getAllByRole('cell')[8]).toHaveTextContent(
    '无已报告错误',
  )
	  expect(within(allMissing!).getAllByRole('cell')[8]).toHaveTextContent(
    '暂无数据',
  )
	  expect(within(partial!).getAllByRole('cell')[8]).toHaveTextContent(
    '未发现错误 · 部分暂无',
  )
})

it('仅按 status_source 展示采集文案且设备来源赢得同级竞争', async () => {
  renderDiskPage()

  const deviceCritical = (await screen.findByText('node-alpha')).closest('tr')
  const attributeWarning = screen.getByText('node-zeta').closest('tr')
  const delayed = screen.getByText('node-delta').closest('tr')
  const disconnected = screen.getByText('node-epsilon').closest('tr')
  expect(deviceCritical).not.toBeNull()
  expect(attributeWarning).not.toBeNull()
  expect(delayed).not.toBeNull()
  expect(disconnected).not.toBeNull()

  expect(within(deviceCritical!).getByText('严重')).toHaveAttribute(
    'data-level',
    'critical',
  )
  expect(within(deviceCritical!).getByTitle('严重')).toBeVisible()
  expect(within(deviceCritical!).queryByText('采集失联')).not.toBeInTheDocument()
  expect(within(attributeWarning!).getByText('警告')).toHaveAttribute(
    'data-level',
    'warning',
  )
  expect(
    within(attributeWarning!).queryByText('采集延迟'),
  ).not.toBeInTheDocument()
  expect(within(delayed!).getByText('采集延迟')).toHaveAttribute(
    'data-level',
    'warning',
  )
  expect(within(disconnected!).getByText('采集失联')).toHaveAttribute(
    'data-level',
    'critical',
  )
})

it('从 URL 恢复搜索、状态、排序和分页并发送全部规范参数', async () => {
  const initialEntry =
    '/disks?search=nvme&status=warning&sort=temperature&order=desc&page=2&page_size=20'
  renderDiskPage(initialEntry)

  expect(await screen.findByText('node-alpha')).toBeInTheDocument()
  expect(
    screen.getByRole('searchbox', { name: '搜索主机、设备或型号' }),
  ).toHaveValue('nvme')
  expect(screen.getByRole('combobox', { name: '设备状态' })).toHaveValue(
    'warning',
  )
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('20')
  expect(screen.getByRole('button', { name: '温度排序，当前降序' })).toHaveAttribute(
    'data-active',
    'true',
  )
  expectRequestParameters(lastRequest(), {
    search: 'nvme',
    status: 'warning',
    sort: 'temperature',
    order: 'desc',
    page: '2',
    page_size: '20',
  })
  expect(window.location.pathname + window.location.search).toBe(initialEntry)
})

it('搜索、状态和每页数量变化写入 URL 并回到第一页', async () => {
  vi.useFakeTimers()
  renderDiskPage('/disks?page=3')
  await act(async () => vi.advanceTimersByTimeAsync(0))

  fireEvent.change(
    screen.getByRole('searchbox', { name: '搜索主机、设备或型号' }),
    { target: { value: 'atlas' } },
  )
  await act(async () => vi.advanceTimersByTimeAsync(299))
  expect(lastRequest().searchParams.get('search')).toBeNull()
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(lastRequest().searchParams.get('search')).toBe('atlas')
  expect(lastRequest().searchParams.get('page')).toBe('1')

  fireEvent.change(screen.getByRole('combobox', { name: '设备状态' }), {
    target: { value: 'critical' },
  })
  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(lastRequest().searchParams.get('status')).toBe('critical')
  expect(lastRequest().searchParams.get('page')).toBe('1')

  fireEvent.change(screen.getByRole('combobox', { name: '每页数量' }), {
    target: { value: '500' },
  })
  await act(async () => vi.advanceTimersByTimeAsync(0))
  vi.useRealTimers()
  await waitFor(() => {
    const searchParams = new URLSearchParams(window.location.search)
    expect(searchParams.get('page')).toBe('1')
    expect(searchParams.get('page_size')).toBe('500')
    expect(searchParams.get('search')).toBe('atlas')
    expectRequestParameters(lastRequest(), {
      search: 'atlas',
      status: 'critical',
      sort: 'host',
      order: 'asc',
      page: '1',
      page_size: '500',
    })
  })
})

it('从第 3 页恢复 500 条多页契约并在排序后保留页大小', async () => {
  const user = userEvent.setup()
  renderDiskPage('/disks?page=3&page_size=500')

  expect(await screen.findByText('第 3 / 3 页，共 1001 块')).toBeVisible()
  expect(screen.getByRole('combobox', { name: '每页数量' })).toHaveValue('500')
  expectRequestParameters(lastRequest(), {
    sort: 'host',
    order: 'asc',
    page: '3',
    page_size: '500',
  })

  await user.click(screen.getByRole('button', { name: '温度排序，当前未排序' }))
  await waitFor(() => {
    expect(window.location.search).toContain('page=1&page_size=500')
    expectRequestParameters(lastRequest(), {
      sort: 'temperature',
      order: 'asc',
      page: '1',
      page_size: '500',
    })
  })
})

it.each([499, 501])('将非法 page_size=%i 规范为 20 且回到第一页', async (pageSize) => {
  renderDiskPage(`/disks?page=3&page_size=${pageSize}`)

  await screen.findByText('node-alpha')
  await waitFor(() => {
    expect(window.location.search).toContain('page=1&page_size=20')
    expect(lastRequest().searchParams.get('page_size')).toBe('20')
  })
})

it.each(expectedDiskSorts)(
  '从 fresh page 3 排序硬盘 %s 时首击升序、再击降序，并发送精确参数',
  async (label, field) => {
  const user = userEvent.setup()
  const initialSort = field === 'status' ? 'host' : 'status'
  renderDiskPage(
    `/disks?status=warning&sort=${initialSort}&order=desc&page=3&page_size=20`,
  )
  await screen.findByText('node-alpha')

  const button = screen.getByRole('button', {
    name: `${label}排序，当前未排序`,
  })
  expect(button).toHaveAttribute('data-active', 'false')
  expect(button).toHaveAttribute('title', `${label}排序，当前未排序`)
  expect(button).not.toHaveTextContent(/[⇅↑↓]/)

  await user.click(button)
  await waitFor(() => {
    const ascending = screen.getByRole('button', {
      name: `${label}排序，当前升序`,
    })
    expect(ascending).toHaveAttribute('data-active', 'true')
    expect(ascending).toHaveAttribute('title', `${label}排序，当前升序`)
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      sort: field,
      order: 'asc',
      page: '1',
      page_size: '20',
      status: 'warning',
    })
  })

  await user.click(
    screen.getByRole('button', { name: `${label}排序，当前升序` }),
  )
  await waitFor(() => {
    const descending = screen.getByRole('button', {
      name: `${label}排序，当前降序`,
    })
    expect(descending).toHaveAttribute('data-active', 'true')
    expect(descending).toHaveAttribute('title', `${label}排序，当前降序`)
    expect(Object.fromEntries(lastRequest().searchParams)).toEqual({
      sort: field,
      order: 'desc',
      page: '1',
      page_size: '20',
      status: 'warning',
    })
  })
  },
)

it('使用 URL 分页并在后端总页数缩小时替换为末页重新请求', async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const url = requestURL(input)
    requestedURLs.push(url)
    const page = Number(url.searchParams.get('page') ?? '1')
    if (page === 999) {
      return Promise.resolve(
        jsonResponse(
          diskDevicePageFixture({
            data: { devices: [], page: 999, total_pages: 3 },
          }),
        ),
      )
    }
    return Promise.resolve(
      jsonResponse(diskDevicePageFixture({ data: { page, total_pages: 3 } })),
    )
  })

  const historyLength = window.history.length
  renderDiskPage('/disks?page=999&page_size=20')

  expect(await screen.findByText('第 3 / 3 页，共 45 块')).toBeInTheDocument()
  expect(requestedURLs.map((url) => url.searchParams.get('page'))).toEqual([
    '999',
    '3',
  ])
  expect(window.location.search).toContain('page=3')
  expect(window.history.length).toBe(historyLength)
  expect(screen.queryByText(/第 999 \/ 3 页/)).not.toBeInTheDocument()
})

it('首次加载、后台刷新失败时保留已有表格并提供重试', async () => {
  let resolveInitial!: (response: Response) => void
  let resolveRefresh!: (response: Response) => void
  vi.mocked(globalThis.fetch)
    .mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          resolveInitial = resolve
        }),
    )
    .mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          resolveRefresh = resolve
        }),
    )
    .mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'datasource_unavailable',
          message: '数据源暂时不可用，请稍后重试',
          request_id: 'req-disk-refresh-failed-001',
          retryable: true,
        },
        503,
      ),
    )
    .mockResolvedValueOnce(jsonResponse(diskDevicePageFixture()))

  const user = userEvent.setup()
  const queryClient = renderDiskPage()
  expect(screen.getByRole('status')).toHaveTextContent('正在加载硬盘设备列表…')

  await act(async () => resolveInitial(jsonResponse(diskDevicePageFixture())))
  await screen.findByText('node-alpha')
  await act(async () => {
    void queryClient.invalidateQueries({ queryKey: ['disk-devices'] })
  })
  await act(async () => resolveRefresh(jsonResponse(diskDevicePageFixture())))

  await act(async () => {
    await queryClient.invalidateQueries({ queryKey: ['disk-devices'] })
  })
  const error = await screen.findByRole('alert')
  expect(error).toHaveTextContent('硬盘设备列表刷新失败')
  expect(error).toHaveTextContent('数据源暂时不可用，请稍后重试')
  expect(screen.getByText('node-alpha')).toBeInTheDocument()
  await user.click(
    within(error).getByRole('button', { name: '重试硬盘设备列表' }),
  )
  await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
})

it('首次失败按 retryable 决定是否显示重试并可恢复', async () => {
  vi.mocked(globalThis.fetch)
    .mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'datasource_unavailable',
          message: '硬盘数据暂不可用',
          request_id: 'req-disk-load-failed-001',
          retryable: true,
        },
        503,
      ),
    )
    .mockResolvedValueOnce(jsonResponse(diskDevicePageFixture()))
  const user = userEvent.setup()
  renderDiskPage()

  const error = await screen.findByRole('alert')
  expect(error).toHaveTextContent('无法加载硬盘设备列表')
  expect(error).toHaveTextContent('硬盘数据暂不可用')
  await user.click(
    within(error).getByRole('button', { name: '重试硬盘设备列表' }),
  )
  expect(await screen.findByText('node-alpha')).toBeInTheDocument()
})

it('显示 stale 与空结果且页面严格只读、不暴露硬盘身份信息', async () => {
  vi.mocked(globalThis.fetch)
    .mockResolvedValueOnce(
      jsonResponse(
        diskDevicePageFixture({
          meta: {
            stale: true,
            collected_at: '2026-07-30T08:30:00.000Z',
          },
        }),
      ),
    )
    .mockResolvedValueOnce(
      jsonResponse(
        diskDevicePageFixture({
          data: { devices: [], total: 0, page: 1, total_pages: 0 },
        }),
      ),
    )
  const user = userEvent.setup()
  renderDiskPage()

  expect(await screen.findByText('数据已过期')).toBeInTheDocument()
  expect(screen.getByText('node-alpha')).toBeInTheDocument()
  expect(
    screen.queryByRole('button', {
      name: /扫描|自检|修复|启停|擦除|删除|重启|执行/,
    }),
  ).not.toBeInTheDocument()
  expect(document.body).not.toHaveTextContent(/serial|WWN/i)

  await user.selectOptions(
    screen.getByRole('combobox', { name: '设备状态' }),
    'normal',
  )
  expect(await screen.findByText('没有符合条件的硬盘设备')).toBeInTheDocument()
  expect(screen.queryByText(/第 1 \/ 0 页/)).not.toBeInTheDocument()
})
