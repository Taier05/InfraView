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
  ElasticsearchHealth,
  ElasticsearchNode,
  ElasticsearchNodePageResponse,
  ElasticsearchNodeStatusSource,
  ElasticsearchRole,
  MetricLevel,
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

const pageSizes = [20, 50, 100, 500] as const
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
const roles = [
  'master',
  'data',
  'data_content',
  'data_hot',
  'data_warm',
  'data_cold',
  'data_frozen',
  'ingest',
  'ml',
  'transform',
  'remote_cluster_client',
  'client',
] as const
const healthLevels = ['green', 'yellow', 'red', 'unknown'] as const
const metricLevels = ['normal', 'warning', 'critical', 'unknown'] as const
const statusSources = [
  'collection',
  'disk',
  'jvm',
  'thread_pool',
  'normal',
  'unknown',
] as const

type PageSize = (typeof pageSizes)[number]
type ElasticsearchSort = (typeof sortFields)[number]
type SortOrder = 'asc' | 'desc'

const healthText: Record<ElasticsearchHealth, string> = {
  green: '绿色',
  yellow: '黄色',
  red: '红色',
  unknown: '未知',
}
const healthLevel: Record<ElasticsearchHealth, MetricLevel> = {
  green: 'normal',
  yellow: 'warning',
  red: 'critical',
  unknown: 'unknown',
}
const levelText: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '严重',
  unknown: '未知',
}
const sourceText: Record<ElasticsearchNodeStatusSource, string> = {
  collection: '采集',
  disk: '磁盘',
  jvm: 'JVM',
  thread_pool: '线程池',
  normal: '正常',
  unknown: '未知',
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isOneOf<T extends string>(
  value: unknown,
  options: readonly T[],
): value is T {
  return typeof value === 'string' && options.includes(value as T)
}

function isNonNegativeFiniteNumberOrNull(value: unknown) {
  return (
    value === null ||
    (typeof value === 'number' && Number.isFinite(value) && value >= 0)
  )
}

function isNonNegativeIntegerOrNull(value: unknown) {
  return (
    value === null ||
    (typeof value === 'number' &&
      Number.isSafeInteger(value) &&
      value >= 0)
  )
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string')
}

function isRoleArray(value: unknown): value is ElasticsearchRole[] {
  return Array.isArray(value) && value.every((item) => isOneOf(item, roles))
}

function isElasticsearchNode(value: unknown): value is ElasticsearchNode {
  if (!isRecord(value)) return false
  return (
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    typeof value.cluster === 'string' &&
    typeof value.address === 'string' &&
    isRoleArray(value.roles) &&
    isOneOf(value.cluster_health, healthLevels) &&
    isNonNegativeFiniteNumberOrNull(value.heap_usage_percent) &&
    isNonNegativeFiniteNumberOrNull(value.disk_usage_percent) &&
    isNonNegativeFiniteNumberOrNull(value.cpu_usage_percent) &&
    isNonNegativeFiniteNumberOrNull(value.index_rate) &&
    isNonNegativeFiniteNumberOrNull(value.search_rate) &&
    isNonNegativeIntegerOrNull(value.documents) &&
    isNonNegativeIntegerOrNull(value.store_size_bytes) &&
    isNonNegativeIntegerOrNull(value.thread_pool_queue) &&
    isNonNegativeFiniteNumberOrNull(value.rejected_rate) &&
    isNonNegativeIntegerOrNull(value.uptime_seconds) &&
    isOneOf(value.status, metricLevels) &&
    isOneOf(value.status_source, statusSources) &&
    isOneOf(value.collection_level, metricLevels)
  )
}

function isElasticsearchNodePageResponse(
  value: unknown,
): value is ElasticsearchNodePageResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) {
    return false
  }
  const { data, meta } = value
  return (
    Array.isArray(data.nodes) &&
    data.nodes.every(isElasticsearchNode) &&
    isStringArray(data.available_clusters) &&
    isRoleArray(data.available_roles) &&
    typeof data.total === 'number' &&
    Number.isSafeInteger(data.total) &&
    data.total >= 0 &&
    typeof data.page === 'number' &&
    Number.isSafeInteger(data.page) &&
    data.page >= 1 &&
    typeof data.page_size === 'number' &&
    pageSizes.includes(data.page_size as PageSize) &&
    typeof data.total_pages === 'number' &&
    Number.isSafeInteger(data.total_pages) &&
    data.total_pages >= 0 &&
    typeof meta.request_id === 'string' &&
    typeof meta.stale === 'boolean' &&
    (meta.collected_at === undefined || typeof meta.collected_at === 'string')
  )
}

function invalidResponse(): never {
  throw new APIError(
    200,
    'invalid_response',
    '服务器响应格式无效',
    '',
    false,
  )
}

async function requestElasticsearchNodes(
  signal: AbortSignal,
  parameters: URLSearchParams,
) {
  const value = await apiRequest<unknown>(
    `/api/v1/elasticsearch/nodes?${parameters.toString()}`,
    { method: 'GET', signal },
  )
  if (!isElasticsearchNodePageResponse(value)) invalidResponse()
  return value
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : 1
}

function pageSize(value: string | null): PageSize {
  const parsed = Number(value)
  return pageSizes.includes(parsed as PageSize) ? (parsed as PageSize) : 20
}

function sortField(value: string | null): ElasticsearchSort {
  return sortFields.includes(value as ElasticsearchSort)
    ? (value as ElasticsearchSort)
    : 'node'
}

function roleFilter(value: string | null): ElasticsearchRole | '' {
  return isOneOf(value, roles) ? value : ''
}

function healthFilter(value: string | null): ElasticsearchHealth | '' {
  return isOneOf(value, healthLevels) ? value : ''
}

function statusFilter(value: string | null): MetricLevel | '' {
  return isOneOf(value, metricLevels) ? value : ''
}

function decimal(value: number | null) {
  if (value === null) return '暂无数据'
  return value.toFixed(2).replace(/(\.\d*?)0+$/, '$1').replace(/\.$/, '')
}

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${value.toFixed(1)}%`
}

function rate(value: number | null) {
  return value === null ? '暂无数据' : `${decimal(value)}/s`
}

function integer(value: number | null) {
  return value === null ? '暂无数据' : String(value)
}

function byteSize(value: number | null) {
  if (value === null) return '暂无数据'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${decimal(size)} ${units[unitIndex]}`
}

function uptime(value: number | null) {
  if (value === null) return '暂无数据'
  const days = Math.floor(value / 86_400)
  const hours = Math.floor((value % 86_400) / 3_600)
  if (days > 0 && hours > 0) return `${days}天 ${hours}小时`
  if (days > 0) return `${days}天`
  return `${hours}小时`
}

function roleDisplay(values: ElasticsearchRole[]) {
  const full = values.length === 0 ? '未知' : values.join(' / ')
  const summary =
    values.length > 2 ? `${values.slice(0, 2).join(' / ')} / …` : full
  return { full, summary }
}

function nodeStatus(node: ElasticsearchNode) {
  if (node.status_source === 'collection') {
    if (node.status === 'critical') return '采集失联'
    if (node.status === 'warning') return '采集延迟'
    if (node.status === 'unknown') return '采集未知'
  }
  if (node.status_source === 'normal') return levelText[node.status]
  return sourceText[node.status_source]
}

export function ElasticsearchPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = searchParams.get('search') ?? ''
  const cluster = (searchParams.get('cluster') ?? '').trim()
  const role = roleFilter(searchParams.get('role'))
  const clusterHealth = healthFilter(searchParams.get('cluster_health'))
  const status = statusFilter(searchParams.get('status'))
  const sort = sortField(searchParams.get('sort'))
  const order: SortOrder = searchParams.get('order') === 'desc' ? 'desc' : 'asc'
  const page = positivePage(searchParams.get('page'))
  const size = pageSize(searchParams.get('page_size'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams()
    if (querySearch !== '') canonical.set('search', querySearch)
    if (cluster !== '') canonical.set('cluster', cluster)
    if (role !== '') canonical.set('role', role)
    if (clusterHealth !== '') canonical.set('cluster_health', clusterHealth)
    if (status !== '') canonical.set('status', status)
    canonical.set('sort', sort)
    canonical.set('order', order)
    canonical.set('page', String(page))
    canonical.set('page_size', String(size))
    if (canonical.toString() !== searchParams.toString()) {
      setSearchParams(canonical, { replace: true })
    }
  }, [
    cluster,
    clusterHealth,
    order,
    page,
    querySearch,
    role,
    searchParams,
    setSearchParams,
    size,
    sort,
    status,
  ])

  useEffect(() => setSearchText(querySearch), [querySearch])

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

  const requestParameters = new URLSearchParams()
  if (querySearch !== '') requestParameters.set('search', querySearch)
  if (cluster !== '') requestParameters.set('cluster', cluster)
  if (role !== '') requestParameters.set('role', role)
  if (clusterHealth !== '') {
    requestParameters.set('cluster_health', clusterHealth)
  }
  if (status !== '') requestParameters.set('status', status)
  requestParameters.set('sort', sort)
  requestParameters.set('order', order)
  requestParameters.set('page', String(page))
  requestParameters.set('page_size', String(size))

  const nodes = useQuery({
    queryKey: [
      'elasticsearch-nodes',
      querySearch,
      cluster,
      role,
      clusterHealth,
      status,
      sort,
      order,
      page,
      size,
    ],
    queryFn: ({ signal }) =>
      requestElasticsearchNodes(signal, requestParameters),
    placeholderData: (previous) => previous,
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })

  const responsePage = nodes.data?.data.page
  const responseTotalPages = nodes.data?.data.total_pages
  const canonicalResponsePage =
    responsePage === undefined || responseTotalPages === undefined
      ? page
      : responseTotalPages === 0
        ? 1
        : Math.min(Math.max(responsePage, 1), responseTotalPages)

  useEffect(() => {
    if (
      nodes.data === undefined ||
      nodes.isPlaceholderData ||
      canonicalResponsePage === page
    ) {
      return
    }
    const next = new URLSearchParams(searchParams)
    next.set('page', String(canonicalResponsePage))
    setSearchParams(next, { replace: true })
  }, [
    canonicalResponsePage,
    nodes.data,
    nodes.isPlaceholderData,
    page,
    searchParams,
    setSearchParams,
  ])

  function updateParameter(key: string, value: string, resetPage = true) {
    const next = new URLSearchParams(searchParams)
    if (value === '') next.delete(key)
    else next.set(key, value)
    if (resetPage) next.set('page', '1')
    setSearchParams(next)
  }

  function sortButton(field: ElasticsearchSort, label: string) {
    const current =
      sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={`${label}排序，当前${current}`}
        onClick={() => {
          const next = new URLSearchParams(searchParams)
          next.set('sort', field)
          next.set('order', sort === field && order === 'asc' ? 'desc' : 'asc')
          next.set('page', '1')
          setSearchParams(next)
        }}
      >
        {label}
      </button>
    )
  }

  const columns: ColumnDef<ElasticsearchNode>[] = [
    {
      id: 'node',
      header: () => sortButton('node', '节点名称'),
      cell: ({ row }) => (
        <span className="elasticsearch-identity" title={row.original.name}>
          {row.original.name}
        </span>
      ),
    },
    {
      id: 'cluster',
      header: () => sortButton('cluster', '所属集群'),
      cell: ({ row }) => (
        <span className="elasticsearch-identity" title={row.original.cluster}>
          {row.original.cluster}
        </span>
      ),
    },
    {
      id: 'address',
      header: () => sortButton('address', '节点地址'),
      cell: ({ row }) => {
        const address = row.original.address || '暂无数据'
        return (
          <span className="elasticsearch-identity" title={address}>
            {address}
          </span>
        )
      },
    },
    {
      id: 'role',
      header: () => sortButton('role', '节点角色'),
      cell: ({ row }) => {
        const role = roleDisplay(row.original.roles)
        return (
          <span className="elasticsearch-role" title={role.full}>
            {role.summary}
          </span>
        )
      },
    },
    {
      id: 'cluster-health',
      header: () => sortButton('cluster_health', '集群健康'),
      cell: ({ row }) => (
        <span
          className="status-badge elasticsearch-health"
          data-level={healthLevel[row.original.cluster_health]}
        >
          <span className="status-badge-dot" aria-hidden="true" />
          {healthText[row.original.cluster_health]}
        </span>
      ),
    },
    {
      id: 'heap',
      header: () => sortButton('heap', 'JVM堆使用率'),
      cell: ({ row }) => percentage(row.original.heap_usage_percent),
    },
    {
      id: 'disk',
      header: () => sortButton('disk', '磁盘使用率'),
      cell: ({ row }) => percentage(row.original.disk_usage_percent),
    },
    {
      id: 'cpu',
      header: () => sortButton('cpu', 'CPU使用率'),
      cell: ({ row }) => percentage(row.original.cpu_usage_percent),
    },
    {
      id: 'index-rate',
      header: () => sortButton('index_rate', '索引速率'),
      cell: ({ row }) => rate(row.original.index_rate),
    },
    {
      id: 'search-rate',
      header: () => sortButton('search_rate', '搜索速率'),
      cell: ({ row }) => rate(row.original.search_rate),
    },
    {
      id: 'documents',
      header: () => sortButton('documents', '文档数'),
      cell: ({ row }) => integer(row.original.documents),
    },
    {
      id: 'store',
      header: () => sortButton('store', '存储大小'),
      cell: ({ row }) => byteSize(row.original.store_size_bytes),
    },
    {
      id: 'thread-queue',
      header: () => sortButton('thread_queue', '线程池队列'),
      cell: ({ row }) => integer(row.original.thread_pool_queue),
    },
    {
      id: 'rejected-rate',
      header: () => sortButton('rejected_rate', '拒绝速率'),
      cell: ({ row }) => rate(row.original.rejected_rate),
    },
    {
      id: 'uptime',
      header: () => sortButton('uptime', '运行时间'),
      cell: ({ row }) => uptime(row.original.uptime_seconds),
    },
    {
      id: 'status',
      header: () => sortButton('status', '状态'),
      cell: ({ row }) => (
        <span
          className="status-badge"
          data-level={row.original.status}
        >
          <span className="status-badge-dot" aria-hidden="true" />
          {nodeStatus(row.original)}
        </span>
      ),
    },
  ]

  const table = useReactTable({
    data: nodes.data?.data.nodes ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    rowCount: nodes.data?.data.total ?? 0,
  })
  const apiError = nodes.error instanceof APIError ? nodes.error : null
  const hasData = nodes.data !== undefined

  return (
    <section aria-labelledby="elasticsearch-title">
      <ListPageHeader
        eyebrow="搜索观测"
        title="Elasticsearch 节点"
        description="只读展示节点、资源、吞吐与采集状态。"
        titleId="elasticsearch-title"
      />

      <ListPageControls
        className="elasticsearch-list-controls"
        collectedAt={nodes.data?.meta.collected_at}
      >
        <ListSearchField
          label="搜索节点名称或地址"
          value={searchText}
          onChange={(event) => setSearchText(event.target.value)}
        />
        <ListSelectField
          label="所属集群"
          value={cluster}
          onChange={(event) => updateParameter('cluster', event.target.value)}
          options={[
            { value: '', label: '全部集群' },
            ...(nodes.data?.data.available_clusters ?? []).map((value) => ({
              value,
              label: value,
            })),
          ]}
        />
        <ListSelectField
          label="节点角色"
          value={role}
          onChange={(event) => updateParameter('role', event.target.value)}
          options={[
            { value: '', label: '全部角色' },
            ...(nodes.data?.data.available_roles ?? []).map((value) => ({
              value,
              label: value,
            })),
          ]}
        />
        <ListSelectField
          label="集群健康"
          value={clusterHealth}
          onChange={(event) =>
            updateParameter('cluster_health', event.target.value)
          }
          options={[
            { value: '', label: '全部健康状态' },
            ...healthLevels.map((value) => ({
              value,
              label: healthText[value],
            })),
          ]}
        />
        <ListSelectField
          label="节点状态"
          value={status}
          onChange={(event) => updateParameter('status', event.target.value)}
          options={[
            { value: '', label: '全部节点状态' },
            ...metricLevels.map((value) => ({
              value,
              label: levelText[value],
            })),
          ]}
        />
        <ListPageSizeField
          value={size}
          onChange={(event) =>
            updateParameter('page_size', event.target.value)
          }
          pageSizes={pageSizes}
        />
      </ListPageControls>

      {nodes.data?.meta.stale === true &&
        (nodes.data.meta.collected_at !== undefined ? (
          <StaleBanner collectedAt={nodes.data.meta.collected_at} />
        ) : (
          <div className="stale-banner" role="alert">
            <strong>数据已过期</strong>
            <span>正在展示缓存数据</span>
          </div>
        ))}

      {nodes.isError && (
        <ErrorPanel
          title={
            hasData
              ? 'Elasticsearch 节点列表刷新失败'
              : 'Elasticsearch 节点列表加载失败'
          }
          message={apiError?.message ?? '暂时无法加载 Elasticsearch 节点'}
          retryable={apiError?.retryable ?? true}
          retryLabel="重试 Elasticsearch 节点列表"
          onRetry={() => void nodes.refetch()}
        />
      )}

      {!hasData && nodes.isPending ? (
        <div className="host-list-loading" role="status">
          正在加载 Elasticsearch 节点…
        </div>
      ) : hasData ? (
        <ListTablePanel
          scrollClassName="elasticsearch-table-scroll"
          emptyState={
            nodes.data.data.nodes.length === 0 ? (
              <div className="host-empty">
                没有符合条件的 Elasticsearch 节点
              </div>
            ) : undefined
          }
          paginationLabel="Elasticsearch 节点列表分页"
          pagination={
            <>
              <span>
                {nodes.data.data.total_pages === 0
                  ? '暂无节点'
                  : `第 ${nodes.data.data.page} / ${nodes.data.data.total_pages} 页，共 ${nodes.data.data.total} 个节点`}
              </span>
              <div>
                <button
                  className="secondary-button"
                  type="button"
                  disabled={
                    nodes.data.data.total_pages === 0 ||
                    nodes.data.data.page <= 1
                  }
                  onClick={() =>
                    updateParameter(
                      'page',
                      String(Math.max(nodes.data!.data.page - 1, 1)),
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
                    nodes.data.data.total_pages === 0 ||
                    nodes.data.data.page >= nodes.data.data.total_pages
                  }
                  onClick={() =>
                    updateParameter(
                      'page',
                      String(
                        Math.min(
                          nodes.data!.data.page + 1,
                          nodes.data!.data.total_pages,
                        ),
                      ),
                      false,
                    )
                  }
                >
                  下一页
                </button>
              </div>
            </>
          }
        >
          <table
            className="host-table elasticsearch-table observability-table"
            aria-label="Elasticsearch 节点列表"
          >
            <thead>
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  {group.headers.map((header) => (
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
      ) : null}
    </section>
  )
}
