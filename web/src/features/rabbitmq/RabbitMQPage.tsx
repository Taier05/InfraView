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
  RabbitMQNode,
  RabbitMQNodePageResponse,
  RabbitMQNodeStatusSource,
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
import { StatusBadge } from '../../components/StatusBadge'
import { formatDurationSeconds } from '../../formatters/duration'

const pageSizes = [20, 50, 100, 500] as const
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
const metricLevels = ['normal', 'warning', 'critical', 'unknown'] as const
const statusSources = [
  'alarm',
  'collection',
  'memory',
  'disk',
  'file_descriptor',
  'erlang_process',
  'normal',
  'unknown',
] as const

type PageSize = (typeof pageSizes)[number]
type RabbitMQSort = (typeof sortFields)[number]
type SortDirection = 'asc' | 'desc'

const levelText: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '严重',
  unknown: '未知',
}

const sourceText: Record<RabbitMQNodeStatusSource, string> = {
  alarm: '资源告警',
  collection: '采集',
  memory: '内存',
  disk: '磁盘',
  file_descriptor: '文件描述符',
  erlang_process: 'Erlang进程',
  normal: '正常',
  unknown: '未知',
}

const numberFormatter = new Intl.NumberFormat('zh-CN', {
  maximumFractionDigits: 2,
})

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
    (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0)
  )
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string')
}

function levelRank(value: MetricLevel) {
  switch (value) {
    case 'critical':
      return 3
    case 'warning':
      return 2
    case 'unknown':
      return 1
    default:
      return 0
  }
}

function hasValidRabbitMQStatusCombination(node: RabbitMQNode) {
  if (levelRank(node.status) < levelRank(node.collection_level)) return false

  switch (node.status_source) {
    case 'normal':
      return node.status === 'normal' && node.collection_level === 'normal'
    case 'unknown':
      return node.status === 'unknown'
    case 'alarm':
      if (node.status !== 'critical' && node.status !== 'unknown') return false
      break
    case 'collection':
      return node.status !== 'normal' && node.status === node.collection_level
    default:
      if (node.status === 'normal') return false
  }

  if (
    node.collection_level !== 'normal' &&
    node.status === node.collection_level
  ) {
    return node.status_source === 'alarm'
  }
  return true
}

function isRabbitMQNode(value: unknown): value is RabbitMQNode {
  if (!isRecord(value)) return false
  return (
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    typeof value.cluster === 'string' &&
    typeof value.address === 'string' &&
    typeof value.version === 'string' &&
    isNonNegativeFiniteNumberOrNull(value.memory_usage_percent) &&
    isNonNegativeIntegerOrNull(value.disk_available_bytes) &&
    isNonNegativeFiniteNumberOrNull(value.file_descriptor_usage_percent) &&
    isNonNegativeFiniteNumberOrNull(value.erlang_process_usage_percent) &&
    isNonNegativeIntegerOrNull(value.connections) &&
    isNonNegativeIntegerOrNull(value.queues) &&
    isNonNegativeIntegerOrNull(value.messages) &&
    isNonNegativeFiniteNumberOrNull(value.publish_rate) &&
    isNonNegativeFiniteNumberOrNull(value.deliver_rate) &&
    isNonNegativeIntegerOrNull(value.uptime_seconds) &&
    isOneOf(value.status, metricLevels) &&
    isOneOf(value.status_source, statusSources) &&
    isOneOf(value.collection_level, metricLevels) &&
    hasValidRabbitMQStatusCombination(value as unknown as RabbitMQNode)
  )
}

function isRabbitMQNodePageResponse(
  value: unknown,
): value is RabbitMQNodePageResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) {
    return false
  }
  const { data, meta } = value
  return (
    Array.isArray(data.nodes) &&
    data.nodes.every(isRabbitMQNode) &&
    isStringArray(data.available_clusters) &&
    typeof data.total === 'number' &&
    Number.isSafeInteger(data.total) &&
    data.total >= 0 &&
    typeof data.page === 'number' &&
    Number.isSafeInteger(data.page) &&
    data.page >= 1 &&
    typeof data.page_size === 'number' &&
    pageSizes.includes(data.page_size as PageSize) &&
    data.nodes.length <= data.page_size &&
    data.nodes.length <= data.total &&
    typeof data.total_pages === 'number' &&
    Number.isSafeInteger(data.total_pages) &&
    data.total_pages >= 0 &&
    data.total_pages ===
      (data.total === 0 ? 0 : Math.ceil(data.total / data.page_size)) &&
    typeof meta.request_id === 'string' &&
    typeof meta.stale === 'boolean' &&
    (meta.collected_at === undefined || typeof meta.collected_at === 'string')
  )
}

function invalidResponse(): never {
  throw new APIError(200, 'invalid_response', '服务器响应格式无效', '', false)
}

async function requestRabbitMQNodes(
  signal: AbortSignal,
  parameters: URLSearchParams,
) {
  const value = await apiRequest<unknown>(
    `/api/v1/rabbitmq/nodes?${parameters.toString()}`,
    { method: 'GET', signal },
  )
  if (!isRabbitMQNodePageResponse(value)) invalidResponse()
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

function sortField(value: string | null): RabbitMQSort {
  return sortFields.includes(value as RabbitMQSort)
    ? (value as RabbitMQSort)
    : 'node'
}

function statusFilter(value: string | null): MetricLevel | '' {
  return isOneOf(value, metricLevels) ? value : ''
}

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${value.toFixed(1)}%`
}

function decimal(value: number) {
  return numberFormatter.format(value)
}

function integer(value: number | null) {
  return value === null ? '暂无数据' : numberFormatter.format(value)
}

function rate(value: number | null) {
  return value === null ? '暂无数据' : `${decimal(value)}/s`
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

function nodeStatus(node: RabbitMQNode) {
  if (node.status_source === 'collection') {
    if (node.collection_level === 'critical') return '采集失联'
    if (node.collection_level === 'warning') return '采集延迟'
    if (node.collection_level === 'unknown') return '采集未知'
  }
  if (node.status_source === 'normal') return levelText[node.status]
  return sourceText[node.status_source]
}

function TitledValue({
  value,
  className,
}: {
  value: string
  className?: string
}) {
  return (
    <span className={className ?? 'rabbitmq-value'} title={value}>
      {value}
    </span>
  )
}

export function RabbitMQPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = searchParams.get('search') ?? ''
  const cluster = (searchParams.get('cluster') ?? '').trim()
  const status = statusFilter(searchParams.get('status'))
  const sort = sortField(searchParams.get('sort'))
  const direction: SortDirection =
    searchParams.get('direction') === 'desc' ? 'desc' : 'asc'
  const requestedPageSize = searchParams.get('page_size')
  const size = pageSize(requestedPageSize)
  const page =
    requestedPageSize !== null && !pageSizes.includes(Number(requestedPageSize) as PageSize)
      ? 1
      : positivePage(searchParams.get('page'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams()
    if (querySearch !== '') canonical.set('search', querySearch)
    if (cluster !== '') canonical.set('cluster', cluster)
    if (status !== '') canonical.set('status', status)
    canonical.set('sort', sort)
    canonical.set('direction', direction)
    canonical.set('page', String(page))
    canonical.set('page_size', String(size))
    if (canonical.toString() !== searchParams.toString()) {
      setSearchParams(canonical, { replace: true })
    }
  }, [
    cluster,
    direction,
    page,
    querySearch,
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
  if (status !== '') requestParameters.set('status', status)
  requestParameters.set('sort', sort)
  requestParameters.set('direction', direction)
  requestParameters.set('page', String(page))
  requestParameters.set('page_size', String(size))

  const nodes = useQuery({
    queryKey: [
      'rabbitmq-nodes',
      querySearch,
      cluster,
      status,
      sort,
      direction,
      page,
      size,
    ],
    queryFn: ({ signal }) => requestRabbitMQNodes(signal, requestParameters),
    placeholderData: (previous) => previous,
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })

  const responsePage = nodes.data?.data.page
  const responseTotalPages = nodes.data?.data.total_pages
  const availableClusters = nodes.data?.data.available_clusters ?? []
  const clusterOptions =
    cluster === '' || availableClusters.includes(cluster)
      ? availableClusters
      : [cluster, ...availableClusters]
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

  function sortButton(field: RabbitMQSort, label: string) {
    const current =
      sort === field ? (direction === 'asc' ? '升序' : '降序') : '未排序'
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={`${label}排序，当前${current}`}
        onClick={() => {
          const next = new URLSearchParams(searchParams)
          next.set('sort', field)
          next.set(
            'direction',
            sort === field && direction === 'asc' ? 'desc' : 'asc',
          )
          next.set('page', '1')
          setSearchParams(next)
        }}
      >
        {label}
      </button>
    )
  }

  const columns: ColumnDef<RabbitMQNode>[] = [
    {
      id: 'node',
      header: () => sortButton('node', '节点名称'),
      cell: ({ row }) => (
        <TitledValue
          value={row.original.name || '暂无数据'}
          className="rabbitmq-identity"
        />
      ),
    },
    {
      id: 'cluster',
      header: () => sortButton('cluster', '所属集群'),
      cell: ({ row }) => (
        <TitledValue
          value={row.original.cluster}
          className="rabbitmq-identity"
        />
      ),
    },
    {
      id: 'address',
      header: () => sortButton('address', '实例地址'),
      cell: ({ row }) => {
        const address = row.original.address || '暂无数据'
        return (
          <TitledValue value={address} className="rabbitmq-identity" />
        )
      },
    },
    {
      id: 'version',
      header: () => sortButton('version', '版本'),
      cell: ({ row }) => (
        <TitledValue value={row.original.version || '暂无数据'} />
      ),
    },
    {
      id: 'memory',
      header: () => sortButton('memory', '内存使用率'),
      cell: ({ row }) => (
        <TitledValue value={percentage(row.original.memory_usage_percent)} />
      ),
    },
    {
      id: 'disk',
      header: () => sortButton('disk', '磁盘余量'),
      cell: ({ row }) => (
        <TitledValue value={byteSize(row.original.disk_available_bytes)} />
      ),
    },
    {
      id: 'file-descriptors',
      header: () => sortButton('file_descriptors', '文件描述符使用率'),
      cell: ({ row }) => (
        <TitledValue
          value={percentage(row.original.file_descriptor_usage_percent)}
        />
      ),
    },
    {
      id: 'erlang-processes',
      header: () => sortButton('erlang_processes', 'Erlang进程使用率'),
      cell: ({ row }) => (
        <TitledValue
          value={percentage(row.original.erlang_process_usage_percent)}
        />
      ),
    },
    {
      id: 'connections',
      header: () => sortButton('connections', '连接'),
      cell: ({ row }) => (
        <TitledValue value={integer(row.original.connections)} />
      ),
    },
    {
      id: 'queues',
      header: () => sortButton('queues', '队列'),
      cell: ({ row }) => <TitledValue value={integer(row.original.queues)} />,
    },
    {
      id: 'messages',
      header: () => sortButton('messages', '消息积压'),
      cell: ({ row }) => <TitledValue value={integer(row.original.messages)} />,
    },
    {
      id: 'publish-rate',
      header: () => sortButton('publish_rate', '发布速率'),
      cell: ({ row }) => <TitledValue value={rate(row.original.publish_rate)} />,
    },
    {
      id: 'deliver-rate',
      header: () => sortButton('deliver_rate', '投递速率'),
      cell: ({ row }) => <TitledValue value={rate(row.original.deliver_rate)} />,
    },
    {
      id: 'uptime',
      header: () => sortButton('uptime', '运行时间'),
      cell: ({ row }) => <TitledValue value={formatDurationSeconds(row.original.uptime_seconds)} />,
    },
    {
      id: 'status',
      header: () => sortButton('status', '状态'),
      cell: ({ row }) => {
        const label = nodeStatus(row.original)
        return (
          <span className="rabbitmq-status" title={`状态来源：${label}`}>
            <StatusBadge level={row.original.status} label={label} />
          </span>
        )
      },
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
  const isStale = nodes.data?.meta.stale === true || (hasData && nodes.isError)

  return (
    <section aria-labelledby="rabbitmq-title">
      <ListPageHeader
        eyebrow="消息队列观测"
        title="RabbitMQ 节点"
        description="只读展示节点资源、队列聚合、吞吐与采集状态。"
        titleId="rabbitmq-title"
      />

      <ListPageControls collectedAt={nodes.data?.meta.collected_at}>
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
            ...clusterOptions.map((value) => ({
              value,
              label: value,
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

      {isStale &&
        (nodes.data?.meta.collected_at !== undefined ? (
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
              ? 'RabbitMQ 节点列表刷新失败'
              : 'RabbitMQ 节点列表加载失败'
          }
          message={apiError?.message ?? '暂时无法加载 RabbitMQ 节点'}
          retryable={apiError?.retryable ?? true}
          retryLabel="重试 RabbitMQ 节点列表"
          onRetry={() => void nodes.refetch()}
        />
      )}

      {!hasData && nodes.isPending ? (
        <div className="host-list-loading" role="status">
          正在加载 RabbitMQ 节点…
        </div>
      ) : hasData ? (
        <ListTablePanel
          scrollClassName="rabbitmq-table-scroll"
          emptyState={
            nodes.data.data.nodes.length === 0 ? (
              <div className="host-empty">没有符合条件的 RabbitMQ 节点</div>
            ) : undefined
          }
          paginationLabel="RabbitMQ 节点列表分页"
          pagination={
            <>
              <span>
                {nodes.data.data.total_pages === 0
                  ? '暂无节点'
                  : `第 ${nodes.data.data.page} / ${nodes.data.data.total_pages} 页，共 ${nodes.data.data.total} 个节点`}
              </span>
              {nodes.data.data.total_pages > 1 && <div>
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
              </div>}
            </>
          }
        >
          <table
            className="host-table rabbitmq-table observability-table"
            aria-label="RabbitMQ 节点列表"
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
