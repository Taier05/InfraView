import { useQuery } from '@tanstack/react-query'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from '@tanstack/react-table'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  DiskDevice,
  DiskDevicePageResponse,
  DiskErrorCounters,
  DiskSMARTHealth,
  MetricLevel,
} from '../../api/types'
import { useRefreshIntervalMs } from '../../app/runtime'
import { ErrorPanel } from '../../components/ErrorPanel'
import { DataTime } from '../../components/DataTime'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'

const pageSizes = [20, 50, 100] as const
type PageSize = (typeof pageSizes)[number]
const sortFields = [
  'host',
  'device',
  'capacity',
  'temperature',
  'lifetime',
  'power_on_hours',
  'status',
] as const
type DiskSort = (typeof sortFields)[number]
type SortOrder = 'asc' | 'desc'

const statusLabels: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '严重',
  unknown: '未知',
}

const smartLabels: Record<DiskSMARTHealth, string> = {
  healthy: 'SMART 正常',
  failed: 'SMART 失败',
  unknown: 'SMART 未知',
}

const smartLevels: Record<DiskSMARTHealth, MetricLevel> = {
  healthy: 'normal',
  failed: 'critical',
  unknown: 'unknown',
}

type ErrorItem = {
  label: string
  value: number | null
  suffix?: string
}

function isDiskSort(value: string | null): value is DiskSort {
  return sortFields.some((field) => field === value)
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : 1
}

function diskPageSize(value: string | null): PageSize {
  const parsed = Number(value)
  return pageSizes.some((pageSize) => pageSize === parsed)
    ? (parsed as PageSize)
    : 20
}

function diskStatus(value: string | null): MetricLevel | '' {
  return value === 'normal' ||
    value === 'warning' ||
    value === 'critical' ||
    value === 'unknown'
    ? value
    : ''
}

function compactNumber(value: number) {
  return value.toFixed(1).replace(/\.0$/, '')
}

function binaryCapacity(bytes: number | null) {
  if (bytes === null) return '暂无数据'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${compactNumber(value)} ${units[unitIndex]}`
}

function temperature(value: number | null) {
  return value === null ? '暂无数据' : `${compactNumber(value)}°C`
}

function lifetime(value: number | null) {
  return value === null ? '暂无数据' : `已用 ${compactNumber(value)}%`
}

function powerOnTime(value: number | null) {
  if (value === null) return '暂无数据'
  const wholeHours = Math.max(0, Math.floor(value))
  const days = Math.floor(wholeHours / 24)
  const hours = wholeHours % 24
  if (days > 0 && hours > 0) return `${days}天 ${hours}小时`
  if (days > 0) return `${days}天`
  return `${hours}小时`
}

function errorItems(errors: DiskErrorCounters): ErrorItem[] {
  return [
    { label: '待处理扇区', value: errors.pending_sectors },
    { label: '重映射扇区', value: errors.reallocated_sectors },
    { label: '不可校正扇区', value: errors.uncorrectable_sectors },
    { label: 'UDMA CRC 错误', value: errors.udma_crc_errors },
    { label: '介质完整性错误', value: errors.media_integrity_errors },
    { label: '错误日志', value: errors.error_log_entries },
    { label: '异常断电', value: errors.unsafe_shutdowns, suffix: ' 次' },
  ]
}

function errorItemText(item: ErrorItem) {
  return `${item.label} ${item.value}${item.suffix ?? ''}`
}

function errorSummary(errors: DiskErrorCounters) {
  const items = errorItems(errors)
  const known = items.filter((item) => item.value !== null)
  const nonZero = known.filter((item) => item.value !== 0)
  if (nonZero.length > 0) {
    const labels = nonZero.map(errorItemText)
    const hasUnsafeShutdowns = nonZero.some(
      (item) => item.label === '异常断电',
    )
    return {
      text: labels.slice(0, 2).join(' · '),
      title: `${labels.join(' · ')}${
        hasUnsafeShutdowns ? '（累计次数，仅展示，不参与状态判断）' : ''
      }`,
    }
  }
  if (known.length === 0) return { text: '暂无数据', title: '暂无数据' }
  if (known.length === items.length) {
    return { text: '无已报告错误', title: '无已报告错误' }
  }
  return {
    text: '未发现错误 · 部分暂无',
    title: '未发现错误 · 部分暂无',
  }
}

function statusPresentation(device: DiskDevice) {
  if (
    device.status_source === 'collection' &&
    (device.collection_level === 'warning' ||
      device.collection_level === 'critical')
  ) {
    return {
      level: device.collection_level,
      label:
        device.collection_level === 'critical' ? '采集失联' : '采集延迟',
    }
  }
  return { level: device.status, label: statusLabels[device.status] }
}

const columnLabels: Record<string, string> = {
  host: '主机',
  device: '设备',
  model: '型号',
  capacity: '容量',
  smart: 'SMART 健康',
  temperature: '温度',
  lifetime: '寿命',
  'power-on-hours': '通电时间',
  errors: '错误摘要',
  status: '状态',
}

export function DiskPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = (searchParams.get('search') ?? '').trim()
  const status = diskStatus(searchParams.get('status'))
  const requestedSort = searchParams.get('sort')
  const sort: DiskSort = isDiskSort(requestedSort) ? requestedSort : 'host'
  const order: SortOrder = searchParams.get('order') === 'desc' ? 'desc' : 'asc'
  const page = positivePage(searchParams.get('page'))
  const pageSize = diskPageSize(searchParams.get('page_size'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams(searchParams)
    if (querySearch === '') canonical.delete('search')
    else canonical.set('search', querySearch)
    if (status === '') canonical.delete('status')
    else canonical.set('status', status)
    canonical.set('sort', sort)
    canonical.set('order', order)
    canonical.set('page', String(page))
    canonical.set('page_size', String(pageSize))
    if (canonical.toString() !== searchParams.toString()) {
      setSearchParams(canonical, { replace: true })
    }
  }, [
    order,
    page,
    pageSize,
    querySearch,
    searchParams,
    setSearchParams,
    sort,
    status,
  ])

  useEffect(() => {
    setSearchText(querySearch)
  }, [querySearch])

  useEffect(() => {
    if (searchText.trim() === querySearch) return
    const timeout = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      const normalized = searchText.trim()
      if (normalized === '') next.delete('search')
      else next.set('search', normalized)
      next.set('page', '1')
      setSearchParams(next)
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [querySearch, searchParams, searchText, setSearchParams])

  const requestParameters = new URLSearchParams({
    sort,
    order,
    page: String(page),
    page_size: String(pageSize),
  })
  if (querySearch !== '') requestParameters.set('search', querySearch)
  if (status !== '') requestParameters.set('status', status)

  const devices = useQuery({
    queryKey: [
      'disk-devices',
      querySearch,
      status,
      sort,
      order,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) =>
      apiRequest<DiskDevicePageResponse>(
        `/api/v1/disks/devices?${requestParameters.toString()}`,
        { method: 'GET', signal },
      ),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })

  const responsePage = devices.data?.data.page
  const responseTotalPages = devices.data?.data.total_pages
  const canonicalResponsePage =
    responsePage === undefined || responseTotalPages === undefined
      ? page
      : responseTotalPages === 0
        ? 1
        : Math.min(Math.max(responsePage, 1), responseTotalPages)
  const responseNeedsPageNormalization =
    devices.data !== undefined && canonicalResponsePage !== page

  useEffect(() => {
    if (!responseNeedsPageNormalization) return
    const next = new URLSearchParams(searchParams)
    next.set('page', String(canonicalResponsePage))
    setSearchParams(next, { replace: true })
  }, [
    canonicalResponsePage,
    responseNeedsPageNormalization,
    searchParams,
    setSearchParams,
  ])

  function updateParameters(
    updates: Record<string, string>,
    resetPage = true,
  ) {
    const next = new URLSearchParams(searchParams)
    for (const [key, value] of Object.entries(updates)) {
      if (value === '') next.delete(key)
      else next.set(key, value)
    }
    if (resetPage) next.set('page', '1')
    setSearchParams(next)
  }

  function changeSort(field: DiskSort) {
    updateParameters({
      sort: field,
      order: sort === field && order === 'asc' ? 'desc' : 'asc',
    })
  }

  function sortButton(field: DiskSort, label: string) {
    const state = sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={label}
        title={`${label}排序，当前${state}`}
        onClick={() => changeSort(field)}
      >
        <span>{label}</span>
        <span className="host-sort-indicator" aria-hidden="true">
          {sort === field ? (order === 'asc' ? '↑' : '↓') : '⇅'}
        </span>
      </button>
    )
  }

  const columns: ColumnDef<DiskDevice>[] = [
    {
      id: 'host',
      header: () => sortButton('host', '主机'),
      cell: ({ row }) => (
        <span className="disk-cell-text" title={row.original.host}>
          {row.original.host}
        </span>
      ),
    },
    {
      id: 'device',
      header: () => sortButton('device', '设备'),
      cell: ({ row }) => (
        <span className="disk-cell-text" title={row.original.device}>
          {row.original.device}
        </span>
      ),
    },
    {
      id: 'model',
      header: () => <span title="型号">型号</span>,
      cell: ({ row }) => {
        const model = row.original.model.trim() || '暂无数据'
        return (
          <span className="disk-model" title={model}>
            {model}
          </span>
        )
      },
    },
    {
      id: 'capacity',
      header: () => sortButton('capacity', '容量'),
      cell: ({ row }) => {
        const capacity = binaryCapacity(row.original.capacity_bytes)
        return (
          <span className="disk-capacity" title={capacity}>
            {capacity}
          </span>
        )
      },
    },
    {
      id: 'smart',
      header: () => <span title="SMART 健康">SMART 健康</span>,
      cell: ({ row }) => {
        const label = smartLabels[row.original.smart_health]
        return (
          <span className="disk-badge-value" title={label}>
            <StatusBadge
              level={smartLevels[row.original.smart_health]}
              label={label}
            />
          </span>
        )
      },
    },
    {
      id: 'temperature',
      header: () => sortButton('temperature', '温度'),
      cell: ({ row }) => {
        const value = temperature(row.original.temperature_celsius)
        return (
          <span className="disk-cell-value" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'lifetime',
      header: () => sortButton('lifetime', '寿命'),
      cell: ({ row }) => {
        const value = lifetime(row.original.lifetime_used_percent)
        return (
          <span className="disk-cell-value" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'power-on-hours',
      header: () => sortButton('power_on_hours', '通电时间'),
      cell: ({ row }) => {
        const value = powerOnTime(row.original.power_on_hours)
        return (
          <span className="disk-cell-value" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'errors',
      header: () => <span title="错误摘要">错误摘要</span>,
      cell: ({ row }) => {
        const summary = errorSummary(row.original.errors)
        return (
          <span className="disk-error-summary" title={summary.title}>
            {summary.text}
          </span>
        )
      },
    },
    {
      id: 'status',
      header: () => sortButton('status', '状态'),
      cell: ({ row }) => {
        const presentation = statusPresentation(row.original)
        return (
          <span className="disk-badge-value" title={presentation.label}>
            <StatusBadge
              level={presentation.level}
              label={presentation.label}
            />
          </span>
        )
      },
    },
  ]

  const table = useReactTable({
    data: devices.data?.data.devices ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    rowCount: devices.data?.data.total ?? 0,
  })
  const apiError = devices.error instanceof APIError ? devices.error : null

  return (
    <section aria-labelledby="disks-title">
      <p className="eyebrow">硬盘观测</p>
      <h1 id="disks-title">硬盘设备</h1>
      <p className="page-description">查看主机硬盘的只读 SMART 健康与寿命数据。</p>

      <div className="host-list-controls disk-list-controls">
        <label className="host-search">
          <span>搜索主机、设备或型号</span>
          <input
            type="search"
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
          />
        </label>
        <label className="host-status-filter">
          <span>设备状态</span>
          <select
            value={status}
            onChange={(event) =>
              updateParameters({ status: event.target.value })
            }
          >
            <option value="">全部状态</option>
            <option value="normal">正常</option>
            <option value="warning">警告</option>
            <option value="critical">严重</option>
            <option value="unknown">未知</option>
          </select>
        </label>
        <label className="host-page-size">
          <span>每页数量</span>
          <select
            value={pageSize}
            onChange={(event) =>
              updateParameters({ page_size: event.target.value })
            }
          >
            {pageSizes.map((value) => (
              <option key={value} value={value}>
                {value} 条
              </option>
            ))}
          </select>
        </label>
        <DataTime
          collectedAt={devices.data?.meta.collected_at}
          className="data-time"
        />
      </div>

      {devices.data?.meta.stale === true &&
        devices.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={devices.data.meta.collected_at} />
        )}

      {devices.data !== undefined && apiError !== null && (
        <div className="host-refresh-error">
          <ErrorPanel
            title="硬盘设备列表刷新失败"
            message={apiError.message}
            retryable={apiError.retryable}
            retryLabel="重试硬盘设备列表"
            onRetry={() => void devices.refetch()}
          />
        </div>
      )}

      {devices.data === undefined && devices.isPending ? (
        <div className="host-list-loading" role="status">
          正在加载硬盘设备列表…
        </div>
      ) : devices.data === undefined ? (
        <ErrorPanel
          title="无法加载硬盘设备列表"
          message={apiError?.message ?? '服务暂时无法处理请求'}
          retryable={apiError?.retryable ?? false}
          retryLabel="重试硬盘设备列表"
          onRetry={() => void devices.refetch()}
        />
      ) : responseNeedsPageNormalization ? (
        <div className="host-list-loading" role="status">
          正在调整硬盘设备列表页码…
        </div>
      ) : (
        <div className="host-table-panel">
          {devices.data.data.total === 0 ? (
            <div className="host-empty">没有符合条件的硬盘设备</div>
          ) : (
            <>
              <div className="disk-table-scroll">
                <table className="host-table disk-table">
                  <thead>
                    {table.getHeaderGroups().map((headerGroup) => (
                      <tr key={headerGroup.id}>
                        {headerGroup.headers.map((header) => (
                          <th
                            key={header.id}
                            scope="col"
                            aria-label={columnLabels[header.column.id]}
                          >
                            {header.isPlaceholder
                              ? null
                              : flexRender(
                                  header.column.columnDef.header,
                                  header.getContext(),
                                )}
                          </th>
                        ))}
                      </tr>
                    ))}
                  </thead>
                  <tbody>
                    {table.getRowModel().rows.map((row) => (
                      <tr key={row.id}>
                        {row.getVisibleCells().map((cell) => (
                          <td key={cell.id}>
                            {flexRender(
                              cell.column.columnDef.cell,
                              cell.getContext(),
                            )}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="host-pagination" aria-label="硬盘设备列表分页">
                <span>
                  第 {devices.data.data.page} /{' '}
                  {devices.data.data.total_pages} 页，共{' '}
                  {devices.data.data.total} 块
                </span>
                <div>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={devices.data.data.page <= 1}
                    onClick={() =>
                      updateParameters(
                        { page: String(devices.data.data.page - 1) },
                        false,
                      )
                    }
                  >
                    上一页
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={
                      devices.data.data.page >=
                      devices.data.data.total_pages
                    }
                    onClick={() =>
                      updateParameters(
                        { page: String(devices.data.data.page + 1) },
                        false,
                      )
                    }
                  >
                    下一页
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  )
}
