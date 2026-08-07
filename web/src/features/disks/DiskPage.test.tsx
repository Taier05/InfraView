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
    return Promise.resolve(
      jsonResponse(
        (() => {
          const fixture = diskPageFixtureForDisplayTests()
          fixture.data.page = page
          fixture.data.page_size = pageSize
          fixture.data.total_pages = Math.ceil(45 / pageSize)
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

it('严格渲染十列并格式化容量、温度、寿命、通电时间和 SMART 健康', async () => {
  renderDiskPage()

  expect(await screen.findByRole('heading', { name: '硬盘设备' })).toBeVisible()
  await screen.findByText('node-alpha')
  expect(
    screen.getAllByRole('columnheader').map((header) => header.ariaLabel),
  ).toEqual([
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
  ])

  const healthyRow = screen.getByText('node-alpha').closest('tr')
  expect(healthyRow).not.toBeNull()
  const healthyCells = within(healthyRow!).getAllByRole('cell')
  expect(healthyCells).toHaveLength(10)
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
  expect(screen.getByRole('button', { name: '温度' })).toHaveAttribute(
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
    target: { value: '100' },
  })
  await act(async () => vi.advanceTimersByTimeAsync(0))
  expect(lastRequest().searchParams.get('page_size')).toBe('100')
  expect(lastRequest().searchParams.get('page')).toBe('1')
  expect(window.location.search).toContain('search=atlas')
})

it('七种服务端排序均支持升序和降序并在变化时回到第一页', async () => {
  const user = userEvent.setup()
  renderDiskPage('/disks?page=4')
  await screen.findByText('node-alpha')

  const expected = [
	    ['主机', 'host'],
	    ['设备', 'device'],
	    ['容量', 'capacity'],
    ['温度', 'temperature'],
    ['寿命', 'lifetime'],
    ['通电时间', 'power_on_hours'],
    ['状态', 'status'],
  ] as const
  for (const [label, field] of expected) {
    await user.click(screen.getByRole('button', { name: label }))
    await waitFor(() => expect(lastRequest().searchParams.get('sort')).toBe(field))
    const firstOrder = field === 'host' ? 'desc' : 'asc'
    expect(lastRequest().searchParams.get('order')).toBe(firstOrder)
    expect(lastRequest().searchParams.get('page')).toBe('1')

    await user.click(screen.getByRole('button', { name: label }))
    await waitFor(() =>
      expect(lastRequest().searchParams.get('order')).toBe(
        firstOrder === 'asc' ? 'desc' : 'asc',
      ),
    )
  }
})

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
