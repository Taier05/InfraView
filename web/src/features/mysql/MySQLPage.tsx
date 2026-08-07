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
  MetricLevel,
  MySQLInstance,
  MySQLInstancePageResponse,
  MySQLReplicationState,
  MySQLRole,
} from '../../api/types'
import { useRefreshIntervalMs } from '../../app/runtime'
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
import { formatDurationSeconds } from '../../formatters/duration'

const pageSizes = [20, 50, 100, 500] as const
type PageSize = (typeof pageSizes)[number]
const sortFields = [
  'instance',
  'version',
  'role',
  'connections',
  'threads_running',
  'qps',
  'tps',
  'slow_queries',
  'buffer_pool_size',
  'buffer_pool_usage',
  'replication_state',
  'replication_lag',
  'uptime',
  'status',
] as const
type MySQLSort = (typeof sortFields)[number]
type SortOrder = 'asc' | 'desc'

const roleLabels: Record<MySQLRole, string> = {
  writable: '读写',
  read_only: '只读',
  unknown: '未知',
}

const replicationLabels: Record<MySQLReplicationState, string> = {
  normal: '正常',
  threads_stopped: '线程异常',
  not_configured: '未配置复制',
  unknown: '状态未知',
}

const statusLabels: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '严重',
  unknown: '未知',
}

function isMySQLSort(value: string | null): value is MySQLSort {
  return sortFields.some((field) => field === value)
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : 1
}

function mysqlPageSize(value: string | null): PageSize {
  const parsed = Number(value)
  return pageSizes.some((pageSize) => pageSize === parsed)
    ? (parsed as PageSize)
    : 20
}

function mysqlStatus(value: string | null): MetricLevel | '' {
  return value === 'normal' ||
    value === 'warning' ||
    value === 'critical' ||
    value === 'unknown'
    ? value
    : ''
}

function mysqlRole(value: string | null): MySQLRole | '' {
  return value === 'writable' ||
    value === 'read_only' ||
    value === 'unknown'
    ? value
    : ''
}

function decimal(value: number | null, digits = 2) {
  if (value === null) return '暂无数据'
  return value
    .toFixed(digits)
    .replace(/(\.\d*?)0+$/, '$1')
    .replace(/\.$/, '')
}

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${value.toFixed(1)}%`
}

function byteSize(value: number | null) {
  if (value === null) return null
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let unitIndex = 0
  let size = value
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(1).replace(/\.0$/, '')} ${units[unitIndex]}`
}

function connectionCount(instance: MySQLInstance) {
  if (instance.connections === null && instance.max_connections === null) {
    return '暂无数据'
  }
  return `${decimal(instance.connections, 0)}/${decimal(instance.max_connections, 0)}`
}

function version(instance: MySQLInstance) {
  const version = instance.version.trim()
  return version === '' || version.toLowerCase() === 'unknown' ? '未知' : version
}

function ReplicationStateText({
  state,
  level,
}: {
  state: MySQLReplicationState
  level: MetricLevel
}) {
  const text = replicationLabels[state]
  return (
    <span className="host-metric" data-level={level} title={text}>
      {text}
    </span>
  )
}

function StatusText({
  level,
  collectionLevel,
}: {
  level: MetricLevel
  collectionLevel: MetricLevel
}) {
  const effectiveLevel =
    collectionLevel === 'warning' || collectionLevel === 'critical'
      ? collectionLevel
      : level
  const text =
    collectionLevel === 'critical'
      ? '采集失联'
      : collectionLevel === 'warning'
        ? '采集延迟'
        : statusLabels[level]
  return (
    <span className="status-badge mysql-status" data-level={effectiveLevel}>
      <span className="status-badge-dot" aria-hidden="true" />
      {text}
    </span>
  )
}

export function MySQLPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = searchParams.get('search') ?? ''
  const label = (searchParams.get('label') ?? '').trim()
  const status = mysqlStatus(searchParams.get('status'))
  const role = mysqlRole(searchParams.get('role'))
  const requestedSort = searchParams.get('sort')
  const sort: MySQLSort = isMySQLSort(requestedSort)
    ? requestedSort
    : 'instance'
  const order: SortOrder = searchParams.get('order') === 'desc' ? 'desc' : 'asc'
  const requestedPageSize = searchParams.get('page_size')
  const pageSize = mysqlPageSize(requestedPageSize)
  const page =
    requestedPageSize !== null && !pageSizes.includes(Number(requestedPageSize) as PageSize)
      ? 1
      : positivePage(searchParams.get('page'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams(searchParams)
    if (querySearch === '') canonical.delete('search')
    else canonical.set('search', querySearch)
    if (label === '') canonical.delete('label')
    else canonical.set('label', label)
    canonical.set('status', status)
    canonical.set('role', role)
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
    label,
    role,
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
      if (searchText === '') next.delete('search')
      else next.set('search', searchText)
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
  if (label !== '') requestParameters.set('label', label)
  if (status !== '') requestParameters.set('status', status)
  if (role !== '') requestParameters.set('role', role)

  const instances = useQuery({
    queryKey: [
      'mysql-instances',
      querySearch,
      label,
      status,
      role,
      sort,
      order,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) =>
      apiRequest<MySQLInstancePageResponse>(
        `/api/v1/mysql/instances?${requestParameters.toString()}`,
        { method: 'GET', signal },
      ),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })

  const responsePage = instances.data?.data.page
  const responseTotalPages = instances.data?.data.total_pages
  const availableLabels = instances.data?.data.available_labels ?? []
  const labelOptions =
    label === '' || availableLabels.includes(label)
      ? availableLabels
      : [label, ...availableLabels]
  const canonicalResponsePage =
    responsePage === undefined || responseTotalPages === undefined
      ? page
      : responseTotalPages === 0
        ? 1
        : Math.min(Math.max(responsePage, 1), responseTotalPages)
  const responseNeedsPageNormalization =
    instances.data !== undefined && canonicalResponsePage !== page

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

  function changeSort(field: MySQLSort) {
    updateParameters({
      sort: field,
      order: sort === field && order === 'asc' ? 'desc' : 'asc',
    })
  }

  function sortButton(
    field: MySQLSort,
    label: string,
    className?: string,
  ) {
    const current =
      sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
    const description = `${label}排序，当前${current}`
    return (
      <button
        className={
          className === undefined
            ? 'host-sort-button'
            : `host-sort-button ${className}`
        }
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

  const columns: ColumnDef<MySQLInstance>[] = [
    {
      id: 'instance',
      header: () => sortButton('instance', '实例地址'),
      cell: ({ row }) => {
        const value = row.original.address
        return (
          <span className="host-cell-text" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'version',
      header: () => sortButton('version', '版本'),
      cell: ({ row }) => {
        const value = version(row.original)
        return (
          <span className="mysql-value" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'role',
      header: () => sortButton('role', '角色'),
      cell: ({ row }) => roleLabels[row.original.role],
    },
    {
      id: 'connections',
      header: () => sortButton('connections', '连接'),
      cell: ({ row }) => {
        const value = connectionCount(row.original)
        return (
          <span className="mysql-connection" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'threads-running',
      header: () => sortButton('threads_running', '活跃线程'),
      cell: ({ row }) => decimal(row.original.threads_running),
    },
    {
      id: 'qps',
      header: () => sortButton('qps', 'QPS'),
      cell: ({ row }) => decimal(row.original.qps),
    },
    {
      id: 'tps',
      header: () => sortButton('tps', 'TPS'),
      cell: ({ row }) => decimal(row.original.tps),
    },
    {
      id: 'slow-queries',
      header: () => sortButton('slow_queries', '慢查询'),
      cell: ({ row }) => decimal(row.original.slow_queries_per_second),
    },
    {
      id: 'buffer-pool-size',
      header: () => sortButton('buffer_pool_size', 'Buffer Pool 容量'),
      cell: ({ row }) => {
        const value = byteSize(row.original.buffer_pool_size_bytes) ?? '暂无数据'
        return (
          <span className="mysql-value" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'buffer-pool-usage',
      header: () => sortButton('buffer_pool_usage', 'Buffer Pool 使用率'),
      cell: ({ row }) => percentage(row.original.buffer_pool_usage_percent),
    },
    {
      id: 'replication-state',
      header: () => sortButton('replication_state', '复制状态'),
      cell: ({ row }) => (
        <ReplicationStateText
          state={row.original.replication.state}
          level={row.original.replication.level}
        />
      ),
    },
    {
      id: 'replication-lag',
      header: () => sortButton('replication_lag', '复制延迟'),
      cell: ({ row }) =>
        row.original.replication.lag_seconds === null
          ? '暂无数据'
          : `${decimal(row.original.replication.lag_seconds)}s`,
    },
    {
      id: 'uptime',
      header: () => sortButton('uptime', '运行时间'),
      cell: ({ row }) => {
        const value = formatDurationSeconds(row.original.uptime_seconds)
        return (
          <span className="mysql-uptime" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'status',
      header: () =>
        sortButton(
          'status', '状态', 'status-align-header mysql-status-align-header',
        ),
      cell: ({ row }) => (
        <StatusText
          level={row.original.status}
          collectionLevel={row.original.collection_level}
        />
      ),
    },
  ]

  const table = useReactTable({
    data: instances.data?.data.instances ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    rowCount: instances.data?.data.total ?? 0,
  })
  const apiError = instances.error instanceof APIError ? instances.error : null

  return (
    <section aria-labelledby="mysql-title">
      <ListPageHeader
        eyebrow="数据库观测"
        title="MySQL 实例"
        description="查看 MySQL 实例的只读运行状态与指标。"
        titleId="mysql-title"
      />

      <ListPageControls
        className="mysql-list-controls"
        collectedAt={instances.data?.meta.collected_at}
      >
        <ListSearchField
          label="搜索实例地址"
          value={searchText}
          onChange={(event) => setSearchText(event.target.value)}
        />
        <ListSelectField
          label="实例标签"
          value={label}
          onChange={(event) => updateParameters({ label: event.target.value })}
          options={[
            { value: '', label: '全部标签' },
            ...labelOptions.map((value) => ({ value, label: value })),
          ]}
        />
        <ListSelectField
          label="实例状态"
          value={status}
          onChange={(event) => updateParameters({ status: event.target.value })}
          options={[
            { value: '', label: '全部状态' },
            { value: 'normal', label: '正常' },
            { value: 'warning', label: '警告' },
            { value: 'critical', label: '严重' },
            { value: 'unknown', label: '未知' },
          ]}
        />
        <ListSelectField
          label="读写属性"
          value={role}
          onChange={(event) => updateParameters({ role: event.target.value })}
          options={[
            { value: '', label: '全部属性' },
            { value: 'writable', label: '读写' },
            { value: 'read_only', label: '只读' },
            { value: 'unknown', label: '未知' },
          ]}
        />
        <ListPageSizeField
          value={pageSize}
          onChange={(event) =>
            updateParameters({ page_size: event.target.value })
          }
          pageSizes={pageSizes}
        />
      </ListPageControls>

      {instances.data?.meta.stale === true &&
        instances.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={instances.data.meta.collected_at} />
        )}

      {instances.data !== undefined && apiError !== null && (
        <div className="host-refresh-error">
          <ErrorPanel
            title="MySQL 实例列表刷新失败"
            message={apiError.message}
            retryable={apiError.retryable}
            retryLabel="重试 MySQL 实例列表"
            onRetry={() => void instances.refetch()}
          />
        </div>
      )}

      {instances.data === undefined && instances.isPending ? (
        <div className="host-list-loading" role="status">
          正在加载 MySQL 实例列表…
        </div>
      ) : instances.data === undefined ? (
        <ErrorPanel
          title="无法加载 MySQL 实例列表"
          message={apiError?.message ?? '服务暂时无法处理请求'}
          retryable={apiError?.retryable ?? false}
          retryLabel="重试 MySQL 实例列表"
          onRetry={() => void instances.refetch()}
        />
      ) : responseNeedsPageNormalization ? (
        <div className="host-list-loading" role="status">
          正在调整 MySQL 实例列表页码…
        </div>
      ) : (
        <ListTablePanel
          scrollClassName="mysql-table-scroll"
          emptyState={
            instances.data.data.total === 0 ? (
              <div className="host-empty">没有符合条件的 MySQL 实例</div>
            ) : undefined
          }
          paginationLabel="MySQL 实例列表分页"
          pagination={
            instances.data.data.total === 0 ? undefined : (
              <>
                <span>
                  第 {instances.data.data.page} / {instances.data.data.total_pages}{' '}
                  页，共 {instances.data.data.total} 个实例
                </span>
                <div>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={instances.data.data.page <= 1}
                    onClick={() =>
                      updateParameters(
                        { page: String(instances.data.data.page - 1) },
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
                      instances.data.data.page >= instances.data.data.total_pages
                    }
                    onClick={() =>
                      updateParameters(
                        { page: String(instances.data.data.page + 1) },
                        false,
                      )
                    }
                  >
                    下一页
                  </button>
                </div>
              </>
            )
          }
        >
          <table className="host-table mysql-table mysql-table-compact observability-table">
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
        </ListTablePanel>
      )}
    </section>
  )
}
