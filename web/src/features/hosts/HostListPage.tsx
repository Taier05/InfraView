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
  HostPageResponse,
  HostStatus,
  HostSummary,
  MetricValue,
} from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import {
  ListPageControls,
  ListPageHeader,
  ListPageSizeField,
  ListSearchField,
  ListSelectField,
  ListTablePanel,
} from '../../components/ListPage'
import { StaleBanner } from '../../components/StaleBanner'
import { formatDurationDisplay } from '../../formatters/duration'
import { useRefreshIntervalMs } from '../../app/runtime'

const pageSizes = [20, 50, 100, 500] as const
type PageSize = (typeof pageSizes)[number]
const sortFields = [
  'name',
  'ip',
  'cpu_cores',
  'memory_total',
  'cpu',
  'memory',
  'load',
  'io',
  'network_transmit',
  'network_receive',
  'uptime',
  'status',
] as const
type HostSort = (typeof sortFields)[number]
type SortOrder = 'asc' | 'desc'

const statusLabels: Record<HostStatus, string> = {
  online: '在线',
  offline: '离线',
  unknown: '未知',
}

function isHostSort(value: string | null): value is HostSort {
  return sortFields.some((field) => field === value)
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : 1
}

function hostPageSize(value: string | null): PageSize {
  const parsed = Number(value)
  return pageSizes.some((pageSize) => pageSize === parsed)
    ? (parsed as PageSize)
    : 20
}

function percentage(metric: MetricValue) {
  if (metric.value === null) return '暂无数据'
  return `${metric.value.toFixed(1)}%`
}

function loadValue(metric: MetricValue) {
  if (metric.value === null) return '暂无数据'
  return metric.value.toFixed(1)
}

function bytesPerSecond(metric: MetricValue) {
  if (metric.value === null) return '暂无数据'
  const units = ['B/s', 'KiB/s', 'MiB/s', 'GiB/s']
  let value = metric.value
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(1)}${units[unitIndex]}`
}

function cpuCoreCount(value: number | null) {
  return value === null ? '暂无数据' : `${value} 核`
}

function memoryCapacity(bytes: number | null) {
  if (bytes === null) return '暂无数据'
  const gibibytes = bytes / (1024 * 1024 * 1024)
  return `${Number.isInteger(gibibytes) ? gibibytes.toFixed(0) : gibibytes.toFixed(1)} GiB`
}

function MetricText({ metric, text }: { metric: MetricValue; text: string }) {
  return (
    <span className="host-metric" data-level={metric.level}>
      {text}
    </span>
  )
}

function StatusText({
  status,
  collectionLevel,
}: {
  status: HostStatus
  collectionLevel: MetricValue['level']
}) {
  const text =
    collectionLevel === 'critical'
      ? '采集失联'
      : collectionLevel === 'warning'
        ? '采集延迟'
        : statusLabels[status]
  const effectiveLevel =
    collectionLevel === 'warning' || collectionLevel === 'critical'
      ? collectionLevel
      : status === 'online'
        ? 'normal'
        : status === 'offline'
          ? 'critical'
          : 'unknown'
  return (
    <span className="host-status" data-level={effectiveLevel}>
      <span className="host-status-dot" aria-hidden="true" />
      {text}
    </span>
  )
}

export function HostListPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = searchParams.get('q') ?? ''
  const queryStatus = searchParams.get('status')
  const status =
    queryStatus === 'online' || queryStatus === 'offline' ? queryStatus : ''
  const requestedSort = searchParams.get('sort')
  const sort: HostSort = isHostSort(requestedSort) ? requestedSort : 'name'
  const order: SortOrder = searchParams.get('order') === 'desc' ? 'desc' : 'asc'
  const requestedPageSize = searchParams.get('page_size')
  const pageSize = hostPageSize(requestedPageSize)
  const page =
    requestedPageSize !== null && !pageSizes.includes(Number(requestedPageSize) as PageSize)
      ? 1
      : positivePage(searchParams.get('page'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams(searchParams)
    canonical.set('q', querySearch)
    canonical.set('status', status)
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
    if (searchText === querySearch) return
    const timeout = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      next.set('q', searchText)
      next.set('page', '1')
      setSearchParams(next)
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [querySearch, searchParams, searchText, setSearchParams])

  const requestParameters = new URLSearchParams({
    q: querySearch,
    status,
    sort,
    order,
    page: String(page),
    page_size: String(pageSize),
  })
  const hosts = useQuery({
    queryKey: [
      'hosts',
      querySearch,
      status,
      sort,
      order,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) =>
      apiRequest<HostPageResponse>(
        `/api/v1/hosts?${requestParameters.toString()}`,
        { signal },
      ),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const responsePage = hosts.data?.data.page
  const responseTotalPages = hosts.data?.data.total_pages
  const canonicalResponsePage =
    responsePage === undefined || responseTotalPages === undefined
      ? page
      : responseTotalPages === 0
        ? 1
        : Math.min(Math.max(responsePage, 1), responseTotalPages)
  const responseNeedsPageNormalization =
    hosts.data !== undefined && canonicalResponsePage !== page

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
      next.set(key, value)
    }
    if (resetPage) next.set('page', '1')
    setSearchParams(next)
  }

  function changeSort(field: HostSort) {
    updateParameters({
      sort: field,
      order: sort === field && order === 'asc' ? 'desc' : 'asc',
    })
  }

  function sortButton(field: HostSort, label: string) {
    const current =
      sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
    const description = `${label}排序，当前${current}`
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={description}
        title={description}
        onClick={() => changeSort(field)}
      >
        {label}
      </button>
    )
  }

  const columns: ColumnDef<HostSummary>[] = [
    {
      id: 'name',
      header: () => sortButton('name', '主机名'),
      cell: ({ row }) => (
        <span className="host-name-text" title={row.original.name}>
          {row.original.name}
        </span>
      ),
    },
    {
      id: 'ip',
      header: () => sortButton('ip', 'IP 地址'),
      cell: ({ row }) => (
        <span className="host-cell-text host-ip" title={row.original.ip}>
          {row.original.ip}
        </span>
      ),
    },
    {
      id: 'cpu-cores',
      header: () => sortButton('cpu_cores', 'CPU 核数'),
      cell: ({ row }) => (
        <span className="host-config-value">
          {cpuCoreCount(row.original.cpu_cores)}
        </span>
      ),
    },
    {
      id: 'memory-total',
      header: () => sortButton('memory_total', '内存容量'),
      cell: ({ row }) => (
        <span className="host-config-value">
          {memoryCapacity(row.original.memory_total_bytes)}
        </span>
      ),
    },
    {
      id: 'cpu',
      header: () => sortButton('cpu', 'CPU 使用率'),
      cell: ({ row }) => (
        <MetricText
          metric={row.original.metrics.cpu_usage}
          text={percentage(row.original.metrics.cpu_usage)}
        />
      ),
    },
    {
      id: 'memory',
      header: () => sortButton('memory', '内存使用率'),
      cell: ({ row }) => (
        <MetricText
          metric={row.original.metrics.memory_usage}
          text={percentage(row.original.metrics.memory_usage)}
        />
      ),
    },
    {
      id: 'load',
      header: () => sortButton('load', '负载'),
      cell: ({ row }) => (
        <MetricText
          metric={row.original.metrics.load_1}
          text={loadValue(row.original.metrics.load_1)}
        />
      ),
    },
    {
      id: 'io',
      header: () => sortButton('io', 'IO 忙碌度'),
      cell: ({ row }) => (
        <MetricText
          metric={row.original.metrics.io_busy_percent}
          text={percentage(row.original.metrics.io_busy_percent)}
        />
      ),
    },
    {
      id: 'network-transmit',
      header: () => sortButton('network_transmit', '网络发送'),
      cell: ({ row }) => {
        const transmit = row.original.metrics.network_transmit_bytes_per_second
        return (
          <MetricText metric={transmit} text={bytesPerSecond(transmit)} />
        )
      },
    },
    {
      id: 'network-receive',
      header: () => sortButton('network_receive', '网络接收'),
      cell: ({ row }) => {
        const receive = row.original.metrics.network_receive_bytes_per_second
        return (
          <MetricText metric={receive} text={bytesPerSecond(receive)} />
        )
      },
    },
    {
      id: 'uptime',
      header: () => sortButton('uptime', '运行时间'),
      cell: ({ row }) => {
        const value = formatDurationDisplay(row.original.uptime_seconds)
        return <span title={value.title}>{value.text}</span>
      },
    },
    {
      id: 'status',
      header: () => sortButton('status', '状态'),
      cell: ({ row }) => (
        <StatusText
          status={row.original.status}
          collectionLevel={row.original.collection_level}
        />
      ),
    },
  ]

  const table = useReactTable({
    data: hosts.data?.data.hosts ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    rowCount: hosts.data?.data.total ?? 0,
  })
  const apiError = hosts.error instanceof APIError ? hosts.error : null

  return (
    <section aria-labelledby="hosts-title">
      <ListPageHeader
        eyebrow="资产清单"
        title="主机"
        description="查看纳入观测范围的 Linux 主机。"
        titleId="hosts-title"
      />

      <ListPageControls collectedAt={hosts.data?.meta.collected_at}>
        <ListSearchField
          label="搜索主机名或 IP"
          value={searchText}
          onChange={(event) => setSearchText(event.target.value)}
        />
        <ListSelectField
          label="主机状态"
          value={status}
          onChange={(event) => updateParameters({ status: event.target.value })}
          options={[
            { value: '', label: '全部状态' },
            { value: 'online', label: '在线' },
            { value: 'offline', label: '离线' },
          ]}
        />
        <ListPageSizeField
          value={pageSize}
          onChange={(event) => updateParameters({ page_size: event.target.value })}
          pageSizes={pageSizes}
        />
      </ListPageControls>

      {hosts.data?.meta.stale === true &&
        hosts.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={hosts.data.meta.collected_at} />
        )}

      {hosts.data !== undefined && apiError !== null && (
        <div className="host-refresh-error">
          <ErrorPanel
            title="主机列表刷新失败"
            message={apiError.message}
            retryable={apiError.retryable}
            retryLabel="重试主机列表"
            onRetry={() => void hosts.refetch()}
          />
        </div>
      )}

      {hosts.data === undefined && hosts.isPending ? (
        <div className="host-list-loading" role="status">
          正在加载主机列表…
        </div>
      ) : hosts.data === undefined ? (
        <ErrorPanel
          title="无法加载主机列表"
          message={apiError?.message ?? '服务暂时无法处理请求'}
          retryable={apiError?.retryable ?? false}
          retryLabel="重试主机列表"
          onRetry={() => void hosts.refetch()}
        />
      ) : responseNeedsPageNormalization ? (
        <div className="host-list-loading" role="status">
          正在调整主机列表页码…
        </div>
      ) : (
        <ListTablePanel
          emptyState={
            hosts.data.data.total === 0 ? (
              <div className="host-empty">没有符合条件的主机</div>
            ) : undefined
          }
          paginationLabel="主机列表分页"
          pagination={
            <>
              {hosts.data.data.total_pages === 0 ? (
                <span>共 0 台</span>
              ) : (
                <span>
                  第 {hosts.data.data.page} / {hosts.data.data.total_pages}{' '}
                  页，共 {hosts.data.data.total} 台
                </span>
              )}
              {hosts.data.data.total_pages > 1 && <div>
                <button
                  className="secondary-button"
                  type="button"
                  disabled={
                    hosts.data.data.total_pages === 0 ||
                    hosts.data.data.page <= 1
                  }
                  onClick={() =>
                    updateParameters(
                      { page: String(hosts.data.data.page - 1) },
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
                    hosts.data.data.total_pages === 0 ||
                    hosts.data.data.page >= hosts.data.data.total_pages
                  }
                  onClick={() =>
                    updateParameters(
                      { page: String(hosts.data.data.page + 1) },
                      false,
                    )
                  }
                >
                  下一页
                </button>
              </div>}
            </>
          }
        >
          {hosts.data.data.total > 0 && (
            <table className="host-table host-list-table observability-table">
                <thead>
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <th key={header.id} scope="col">
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
          )}
        </ListTablePanel>
      )}
    </section>
  )
}
