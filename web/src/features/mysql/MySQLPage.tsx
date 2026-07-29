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
import { RefreshControl } from '../../components/RefreshControl'
import { StaleBanner } from '../../components/StaleBanner'

const pageSizes = [20, 50, 100] as const
type PageSize = (typeof pageSizes)[number]
const sortFields = [
  'instance',
  'connections',
  'threads_running',
  'qps',
  'slow_queries',
  'buffer_pool',
  'replication_lag',
  'uptime',
  'status',
] as const
type MySQLSort = (typeof sortFields)[number]
type SortOrder = 'asc' | 'desc'

const roleLabels: Record<MySQLRole, string> = {
  writable: '可写',
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
  return value.toFixed(digits).replace(/\.?0+$/, '')
}

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${value.toFixed(1)}%`
}

function connectionUsage(instance: MySQLInstance) {
  const { connections, max_connections: maximum, connection_usage_percent } =
    instance
  if (
    connections === null &&
    maximum === null &&
    connection_usage_percent === null
  ) {
    return '暂无数据'
  }
  const values =
    connections !== null && maximum !== null
      ? `${decimal(connections)} / ${decimal(maximum)}`
      : connections !== null
        ? decimal(connections)
        : maximum !== null
          ? `最大 ${decimal(maximum)}`
          : '暂无数据'
  return connection_usage_percent === null
    ? values
    : `${values} (${connection_usage_percent.toFixed(1)}%)`
}

function versionRole(instance: MySQLInstance) {
  const version = instance.version.trim()
  const versionLabel =
    version === '' || version.toLowerCase() === 'unknown' ? '未知' : version
  return `${versionLabel} · ${roleLabels[instance.role]}`
}

function uptime(seconds: number | null) {
  if (seconds === null) return '暂无数据'
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3_600)
  if (days > 0 && hours > 0) return `${days}天 ${hours}小时`
  if (days > 0) return `${days}天`
  return `${hours}小时`
}

function ReplicationText({
  state,
  lagSeconds,
  level,
}: {
  state: MySQLReplicationState
  lagSeconds: number | null
  level: MetricLevel
}) {
  const text =
    lagSeconds !== null
      ? `${replicationLabels[state]} · ${decimal(lagSeconds)}s`
      : state === 'normal'
        ? `${replicationLabels[state]} · 暂无数据`
        : replicationLabels[state]
  return (
    <span className="host-metric" data-level={level}>
      {text}
    </span>
  )
}

function StatusText({ level }: { level: MetricLevel }) {
  return (
    <span className="status-badge mysql-status" data-level={level}>
      <span className="status-badge-dot" aria-hidden="true" />
      {statusLabels[level]}
    </span>
  )
}

export function MySQLPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = searchParams.get('search') ?? ''
  const status = mysqlStatus(searchParams.get('status'))
  const role = mysqlRole(searchParams.get('role'))
  const requestedSort = searchParams.get('sort')
  const sort: MySQLSort = isMySQLSort(requestedSort)
    ? requestedSort
    : 'instance'
  const order: SortOrder = searchParams.get('order') === 'desc' ? 'desc' : 'asc'
  const page = positivePage(searchParams.get('page'))
  const pageSize = mysqlPageSize(searchParams.get('page_size'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams(searchParams)
    if (querySearch === '') canonical.delete('search')
    else canonical.set('search', querySearch)
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
  if (status !== '') requestParameters.set('status', status)
  if (role !== '') requestParameters.set('role', role)

  const instances = useQuery({
    queryKey: [
      'mysql-instances',
      querySearch,
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

  function sortButton(field: MySQLSort, label: string) {
    const state = sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={`${label}排序，当前${state}`}
        title={`点击按${label}排序`}
        onClick={() => changeSort(field)}
      >
        <span>{label}</span>
        <span className="host-sort-indicator" aria-hidden="true">
          {sort === field ? (order === 'asc' ? '↑' : '↓') : '⇅'}
        </span>
      </button>
    )
  }

  const columns: ColumnDef<MySQLInstance>[] = [
    {
      id: 'instance',
      header: () => sortButton('instance', '实例'),
      cell: ({ row }) => {
        const value = `${row.original.name} · ${row.original.address}`
        return (
          <span className="host-name-text" title={value}>
            <span>{row.original.name}</span>
            <span>{` · ${row.original.address}`}</span>
          </span>
        )
      },
    },
    {
      id: 'host',
      header: '所属主机',
      cell: ({ row }) => (
        <span className="host-cell-text" title={row.original.host}>
          {row.original.host}
        </span>
      ),
    },
    {
      id: 'version-role',
      header: '版本 / 角色',
      cell: ({ row }) => {
        const value = versionRole(row.original)
        return (
          <span className="mysql-version-role" title={value}>
            {value}
          </span>
        )
      },
    },
    {
      id: 'connections',
      header: () => sortButton('connections', '连接使用'),
      cell: ({ row }) => {
        const value = connectionUsage(row.original)
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
      id: 'slow-queries',
      header: () => sortButton('slow_queries', '慢查询速率'),
      cell: ({ row }) => decimal(row.original.slow_queries_per_second),
    },
    {
      id: 'buffer-pool',
      header: () => sortButton('buffer_pool', 'Buffer Pool 使用率'),
      cell: ({ row }) => percentage(row.original.buffer_pool_usage_percent),
    },
    {
      id: 'replication',
      header: () => sortButton('replication_lag', '复制状态 / 延迟'),
      cell: ({ row }) => (
        <ReplicationText
          state={row.original.replication.state}
          lagSeconds={row.original.replication.lag_seconds}
          level={row.original.replication.level}
        />
      ),
    },
    {
      id: 'uptime',
      header: () => sortButton('uptime', '运行时间'),
      cell: ({ row }) => uptime(row.original.uptime_seconds),
    },
    {
      id: 'status',
      header: () => sortButton('status', '状态'),
      cell: ({ row }) => <StatusText level={row.original.status} />,
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
      <p className="eyebrow">数据库观测</p>
      <h1 id="mysql-title">MySQL 实例</h1>
      <p className="page-description">查看 MySQL 实例的只读运行状态与指标。</p>

      <div className="host-list-controls mysql-list-controls">
        <label className="host-search">
          <span>搜索实例名称、地址或所属主机</span>
          <input
            type="search"
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
          />
        </label>
        <label className="host-status-filter">
          <span>实例状态</span>
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
        <label className="host-status-filter">
          <span>读写属性</span>
          <select
            value={role}
            onChange={(event) =>
              updateParameters({ role: event.target.value })
            }
          >
            <option value="">全部属性</option>
            <option value="writable">可写</option>
            <option value="read_only">只读</option>
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
        <RefreshControl
          isFetching={instances.isFetching}
          dataUpdatedAt={instances.dataUpdatedAt}
          onRefresh={() => void instances.refetch()}
          refreshIntervalSeconds={refreshIntervalMs / 1_000}
          ariaLabel="刷新 MySQL 实例列表"
        />
      </div>

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

      <div className="host-table-panel">
        <div className="host-table-scroll">
          <table className="host-table mysql-table">
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
              {instances.data !== undefined &&
                !responseNeedsPageNormalization &&
                table.getRowModel().rows.map((row) => (
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
        ) : instances.data.data.total === 0 ? (
          <div className="host-empty">没有符合条件的 MySQL 实例</div>
        ) : (
          <div className="host-pagination" aria-label="MySQL 实例列表分页">
            {instances.data.data.total_pages === 0 ? (
              <span>共 0 个实例</span>
            ) : (
              <span>
                第 {instances.data.data.page} /{' '}
                {instances.data.data.total_pages} 页，共{' '}
                {instances.data.data.total} 个实例
              </span>
            )}
            <div>
              <button
                className="secondary-button"
                type="button"
                disabled={
                  instances.data.data.total_pages === 0 ||
                  instances.data.data.page <= 1
                }
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
                  instances.data.data.total_pages === 0 ||
                  instances.data.data.page >=
                    instances.data.data.total_pages
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
          </div>
        )}
      </div>
    </section>
  )
}
