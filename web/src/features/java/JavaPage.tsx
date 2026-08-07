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
  JavaService,
  JavaServicePageResponse,
  JavaStatusSource,
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
import { StatusBadge } from '../../components/StatusBadge'

const pageSizes = [20, 50, 100] as const
const sortFields = [
  'business', 'address', 'health', 'health_latency', 'port', 'process',
  'process_count', 'consistency', 'cpu', 'memory', 'memory_percent',
  'uptime', 'status',
] as const
const metricLevels = ['normal', 'warning', 'critical', 'unknown'] as const
const javaStatusSources = [
  'health',
  'port',
  'process',
  'consistency',
  'collection',
  'normal',
  'unknown',
] as const
const maxInt64 = 9_223_372_036_854_775_807n
const canonicalNonNegativeInteger = /^(?:0|[1-9][0-9]*)$/

type PageSize = (typeof pageSizes)[number]
type JavaSort = (typeof sortFields)[number]
type SortDirection = 'asc' | 'desc'

const levelText: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '严重',
  unknown: '未知',
}

const sourceText: Record<string, string> = {
  health: '健康检查',
  port: '端口状态',
  process: '进程状态',
  consistency: '端口进程一致性',
  collection: '采集状态',
}

const businessLabels: Readonly<Record<string, string>> = {
  tikbee: '用户端',
  rider: '骑手端',
  mch: '商家端',
  saas: '管理后台端',
  mch_saas: '商家 PC 端',
}

const numberFormatter = new Intl.NumberFormat('zh-CN', {
  maximumFractionDigits: 2,
})

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isOneOf<T extends string>(value: unknown, options: readonly T[]): value is T {
  return typeof value === 'string' && options.includes(value as T)
}

function isNonNegativeFiniteNumberOrNull(value: unknown) {
  return value === null || (typeof value === 'number' && Number.isFinite(value) && value >= 0)
}

function isPercentageOrNull(value: unknown) {
  return value === null || (typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 100)
}

function isCanonicalInt64OrNull(value: unknown) {
  return value === null || (
    typeof value === 'string' &&
    canonicalNonNegativeInteger.test(value) &&
    BigInt(value) <= maxInt64
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

function sourceRank(value: JavaStatusSource) {
  switch (value) {
    case 'unknown':
      return 6
    case 'health':
      return 5
    case 'port':
      return 4
    case 'process':
      return 3
    case 'consistency':
      return 2
    case 'collection':
      return 1
    default:
      return 0
  }
}

function binaryAssessment(
  value: boolean | null,
  source: JavaStatusSource,
) {
  if (value === null) return { level: 'unknown' as const, source: 'unknown' as const }
  if (!value) return { level: 'critical' as const, source }
  return { level: 'normal' as const, source: 'normal' as const }
}

function hasValidJavaStatusCombination(service: JavaService) {
  let assessment: { level: MetricLevel; source: JavaStatusSource } = {
    level: 'normal',
    source: 'normal',
  }
  const candidates = [
    binaryAssessment(service.health_up, 'health'),
    binaryAssessment(service.port_up, 'port'),
    binaryAssessment(service.process_up, 'process'),
    binaryAssessment(service.port_consistent, 'consistency'),
    { level: service.collection_level, source: 'collection' as const },
  ]
  for (const candidate of candidates) {
    if (candidate.level === 'normal') continue
    if (
      levelRank(candidate.level) > levelRank(assessment.level) ||
      (levelRank(candidate.level) === levelRank(assessment.level) &&
        sourceRank(candidate.source) > sourceRank(assessment.source))
    ) {
      assessment = candidate
    }
  }
  return (
    service.status === assessment.level &&
    service.status_source === assessment.source
  )
}

function isJavaService(value: unknown): value is JavaService {
  if (!isRecord(value)) return false
  return (
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    typeof value.business === 'string' &&
    typeof value.address === 'string' &&
    (typeof value.health_up === 'boolean' || value.health_up === null) &&
    isNonNegativeFiniteNumberOrNull(value.health_latency_ms) &&
    (typeof value.port_up === 'boolean' || value.port_up === null) &&
    (typeof value.process_up === 'boolean' || value.process_up === null) &&
    isCanonicalInt64OrNull(value.process_count) &&
    (typeof value.port_consistent === 'boolean' || value.port_consistent === null) &&
    isPercentageOrNull(value.cpu_usage_percent) &&
    isCanonicalInt64OrNull(value.memory_bytes) &&
    isPercentageOrNull(value.memory_usage_percent) &&
    isCanonicalInt64OrNull(value.uptime_seconds) &&
    isOneOf(value.status, metricLevels) &&
    isOneOf(value.status_source, javaStatusSources) &&
    isOneOf(value.collection_level, metricLevels) &&
    hasValidJavaStatusCombination(value as unknown as JavaService)
  )
}

export function isJavaServicePageResponse(value: unknown): value is JavaServicePageResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) return false
  const { data, meta } = value
  return (
    Array.isArray(data.services) && data.services.every(isJavaService) &&
    isStringArray(data.available_names) &&
    typeof data.total === 'number' && Number.isSafeInteger(data.total) && data.total >= 0 &&
    typeof data.page === 'number' && Number.isSafeInteger(data.page) && data.page >= 1 &&
    typeof data.page_size === 'number' && pageSizes.includes(data.page_size as PageSize) &&
    data.services.length <= data.page_size && data.services.length <= data.total &&
    typeof data.total_pages === 'number' && Number.isSafeInteger(data.total_pages) && data.total_pages >= 0 &&
    data.total_pages === (data.total === 0 ? 0 : Math.ceil(data.total / data.page_size)) &&
    typeof meta.request_id === 'string' && typeof meta.stale === 'boolean' &&
    (meta.collected_at === undefined || typeof meta.collected_at === 'string')
  )
}

function invalidResponse(): never {
  throw new APIError(200, 'invalid_response', '服务器响应格式无效', '', false)
}

async function requestJavaServices(signal: AbortSignal, parameters: URLSearchParams) {
  const value = await apiRequest<unknown>(`/api/v1/java/services?${parameters.toString()}`, {
    method: 'GET', signal,
  })
  if (!isJavaServicePageResponse(value)) invalidResponse()
  return value
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : 1
}

function pageSize(value: string | null): PageSize {
  const parsed = Number(value)
  return pageSizes.includes(parsed as PageSize) ? parsed as PageSize : 20
}

function sortField(value: string | null): JavaSort {
  return sortFields.includes(value as JavaSort) ? value as JavaSort : 'business'
}

function statusFilter(value: string | null): MetricLevel | '' {
  return isOneOf(value, metricLevels) ? value : ''
}

function binary(value: boolean | null) {
  if (value === null) return '暂无数据'
  return value ? '正常' : '异常'
}

function javaBusinessLabel(code: string) {
  return businessLabels[code] ?? code
}

function BinaryStatus({ value }: { value: boolean | null }) {
  const level: MetricLevel = value === null ? 'unknown' : value ? 'normal' : 'critical'
  const label = value === null ? '暂无数据' : value ? '正常' : '异常'
  return <StatusBadge level={level} label={label} />
}

function decimal(value: number) {
  return numberFormatter.format(value)
}

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${value.toFixed(1)}%`
}

function integer(value: string | null) {
  return value === null ? '暂无数据' : numberFormatter.format(BigInt(value))
}

function latency(value: number | null) {
  return value === null ? '暂无数据' : `${decimal(value)} ms`
}

function byteSize(value: string | null) {
  if (value === null) return '暂无数据'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB']
  const bytes = BigInt(value)
  let divisor = 1n
  let unitIndex = 0
  while (bytes >= divisor * 1024n && unitIndex < units.length - 1) {
    divisor *= 1024n
    unitIndex += 1
  }
  const scaledHundredths = (bytes * 100n + divisor / 2n) / divisor
  const whole = scaledHundredths / 100n
  const fraction = String(scaledHundredths % 100n).padStart(2, '0').replace(/0+$/, '')
  return `${numberFormatter.format(whole)}${fraction === '' ? '' : `.${fraction}`} ${units[unitIndex]}`
}

function uptime(value: string | null) {
  if (value === null) return '暂无数据'
  const seconds = BigInt(value)
  const days = seconds / 86_400n
  const hours = (seconds % 86_400n) / 3_600n
  if (days > 0n && hours > 0n) return `${days}天 ${hours}小时`
  if (days > 0n) return `${days}天`
  return `${hours}小时`
}

function serviceStatus(service: JavaService) {
  if (service.status_source === 'normal') return levelText[service.status]
  if (service.status_source === 'unknown') return '未知'
  return sourceText[service.status_source] ?? service.status_source
}

function TitledValue({ value, className }: { value: string; className?: string }) {
  return <span className={className ?? 'java-value'} title={value}>{value}</span>
}

export function JavaPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const [searchParams, setSearchParams] = useSearchParams()
  const querySearch = (searchParams.get('search') ?? '').trim()
  const name = (searchParams.get('name') ?? '').trim()
  const status = statusFilter(searchParams.get('status'))
  const sort = sortField(searchParams.get('sort'))
  const direction: SortDirection = searchParams.get('direction') === 'desc' ? 'desc' : 'asc'
  const page = positivePage(searchParams.get('page'))
  const size = pageSize(searchParams.get('page_size'))
  const [searchText, setSearchText] = useState(querySearch)

  useEffect(() => {
    const canonical = new URLSearchParams()
    if (querySearch !== '') canonical.set('search', querySearch)
    if (name !== '') canonical.set('name', name)
    if (status !== '') canonical.set('status', status)
    canonical.set('sort', sort)
    canonical.set('direction', direction)
    canonical.set('page', String(page))
    canonical.set('page_size', String(size))
    if (canonical.toString() !== searchParams.toString()) setSearchParams(canonical, { replace: true })
  }, [direction, name, page, querySearch, searchParams, setSearchParams, size, sort, status])

  useEffect(() => setSearchText(querySearch), [querySearch])

  useEffect(() => {
    if (searchText === querySearch) return
    const timeout = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      const normalizedSearch = searchText.trim()
      if (normalizedSearch === '') next.delete('search')
      else next.set('search', normalizedSearch)
      next.set('page', '1')
      setSearchParams(next)
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [querySearch, searchParams, searchText, setSearchParams])

  const requestParameters = new URLSearchParams()
  if (querySearch !== '') requestParameters.set('search', querySearch)
  if (name !== '') requestParameters.set('name', name)
  if (status !== '') requestParameters.set('status', status)
  requestParameters.set('sort', sort)
  requestParameters.set('direction', direction)
  requestParameters.set('page', String(page))
  requestParameters.set('page_size', String(size))

  const services = useQuery({
    queryKey: ['java-services', querySearch, name, status, sort, direction, page, size],
    queryFn: ({ signal }) => requestJavaServices(signal, requestParameters),
    placeholderData: (previous) => previous,
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })

  const responsePage = services.data?.data.page
  const responseTotalPages = services.data?.data.total_pages
  const availableNames = services.data?.data.available_names ?? []
  const nameOptions = name === '' || availableNames.includes(name) ? availableNames : [name, ...availableNames]
  const canonicalResponsePage = responsePage === undefined || responseTotalPages === undefined
    ? page : responseTotalPages === 0 ? 1 : Math.min(Math.max(responsePage, 1), responseTotalPages)

  useEffect(() => {
    if (services.data === undefined || services.isPlaceholderData || canonicalResponsePage === page) return
    const next = new URLSearchParams(searchParams)
    next.set('page', String(canonicalResponsePage))
    setSearchParams(next, { replace: true })
  }, [canonicalResponsePage, page, searchParams, services.data, services.isPlaceholderData, setSearchParams])

  function updateParameter(key: string, value: string, resetPage = true) {
    const next = new URLSearchParams(searchParams)
    if (value === '') next.delete(key)
    else next.set(key, value)
    if (resetPage) next.set('page', '1')
    setSearchParams(next)
  }

  function sortButton(field: JavaSort, label: string) {
    const current = sort === field ? (direction === 'asc' ? '升序' : '降序') : '未排序'
    return <button className="host-sort-button" type="button" data-active={sort === field}
      aria-label={`${label}排序，当前${current}`}
      onClick={() => {
        const next = new URLSearchParams(searchParams)
        next.set('sort', field)
        next.set('direction', sort === field && direction === 'asc' ? 'desc' : 'asc')
        next.set('page', '1')
        setSearchParams(next)
      }}>{label}</button>
  }

  const columns: ColumnDef<JavaService>[] = [
    { id: 'business', header: () => sortButton('business', '业务端'), cell: ({ row }) => <TitledValue value={row.original.business || '暂无数据'} className="java-identity" /> },
    { id: 'address', header: () => sortButton('address', '服务地址'), cell: ({ row }) => <TitledValue value={row.original.address || '暂无数据'} className="java-identity" /> },
    { id: 'health', header: () => sortButton('health', '健康检查'), cell: ({ row }) => <BinaryStatus value={row.original.health_up} /> },
    { id: 'health-latency', header: () => sortButton('health_latency', '健康延迟'), cell: ({ row }) => <TitledValue value={latency(row.original.health_latency_ms)} /> },
    { id: 'port', header: () => sortButton('port', '端口状态'), cell: ({ row }) => <BinaryStatus value={row.original.port_up} /> },
    { id: 'process', header: () => sortButton('process', '进程状态'), cell: ({ row }) => <BinaryStatus value={row.original.process_up} /> },
    { id: 'process-count', header: () => sortButton('process_count', '进程数'), cell: ({ row }) => <TitledValue value={integer(row.original.process_count)} /> },
    { id: 'consistency', header: () => sortButton('consistency', '端口进程一致性'), cell: ({ row }) => <TitledValue value={binary(row.original.port_consistent)} /> },
    { id: 'cpu', header: () => sortButton('cpu', 'CPU 使用率'), cell: ({ row }) => <TitledValue value={percentage(row.original.cpu_usage_percent)} /> },
    { id: 'memory', header: () => sortButton('memory', '内存占用'), cell: ({ row }) => <TitledValue value={byteSize(row.original.memory_bytes)} /> },
    { id: 'memory-percent', header: () => sortButton('memory_percent', '内存使用率'), cell: ({ row }) => <TitledValue value={percentage(row.original.memory_usage_percent)} /> },
    { id: 'uptime', header: () => sortButton('uptime', '运行时间'), cell: ({ row }) => <TitledValue value={uptime(row.original.uptime_seconds)} /> },
    { id: 'status', header: () => sortButton('status', '状态'), cell: ({ row }) => {
      const label = serviceStatus(row.original)
      return <span className="java-status" title={`状态来源：${label}`}><StatusBadge level={row.original.status} label={label} /></span>
    } },
  ]

  const table = useReactTable({
    data: services.data?.data.services ?? [], columns, getCoreRowModel: getCoreRowModel(),
    manualPagination: true, manualSorting: true, rowCount: services.data?.data.total ?? 0,
  })
  const apiError = services.error instanceof APIError ? services.error : null
  const hasData = services.data !== undefined
  const isStale = services.data?.meta.stale === true || (hasData && services.isError)

  return <section aria-labelledby="java-title">
    <ListPageHeader eyebrow="Java 业务观测" title="Java 业务服务" description="只读展示业务服务健康、进程资源与采集状态。" titleId="java-title" />
    <ListPageControls collectedAt={services.data?.meta.collected_at}>
      <ListSearchField label="搜索业务端、服务名称或地址" value={searchText} onChange={(event) => setSearchText(event.target.value)} />
      <ListSelectField label="业务端" value={name} onChange={(event) => updateParameter('name', event.target.value)} options={[{ value: '', label: '全部业务端' }, ...nameOptions.map((value) => ({ value, label: javaBusinessLabel(value) }))]} />
      <ListSelectField label="服务状态" value={status} onChange={(event) => updateParameter('status', event.target.value)} options={[{ value: '', label: '全部服务状态' }, ...metricLevels.map((value) => ({ value, label: levelText[value] }))]} />
      <ListPageSizeField value={size} onChange={(event) => updateParameter('page_size', event.target.value)} pageSizes={pageSizes} />
    </ListPageControls>
    {isStale && (services.data?.meta.collected_at !== undefined
      ? <StaleBanner collectedAt={services.data.meta.collected_at} />
      : <div className="stale-banner" role="alert"><strong>数据已过期</strong><span>正在展示缓存数据</span></div>)}
    {services.isError && <ErrorPanel title={hasData ? 'Java 业务服务列表刷新失败' : 'Java 业务服务列表加载失败'} message={apiError?.message ?? '暂时无法加载 Java 业务服务'} retryable={apiError?.retryable ?? true} retryLabel="重试 Java 业务服务列表" onRetry={() => void services.refetch()} />}
    {!hasData && services.isPending ? <div className="host-list-loading" role="status">正在加载 Java 业务服务…</div> : hasData ?
      <ListTablePanel scrollClassName="java-table-scroll" emptyState={services.data.data.services.length === 0 ? <div className="host-empty">没有符合条件的 Java 业务服务</div> : undefined} paginationLabel="Java 业务服务列表分页" pagination={<>
        <span>{services.data.data.total_pages === 0 ? '暂无服务' : `第 ${services.data.data.page} / ${services.data.data.total_pages} 页，共 ${services.data.data.total} 个服务`}</span>
        <div><button className="secondary-button" type="button" disabled={services.data.data.total_pages === 0 || services.data.data.page <= 1} onClick={() => updateParameter('page', String(Math.max(services.data!.data.page - 1, 1)), false)}>上一页</button>
        <button className="secondary-button" type="button" disabled={services.data.data.total_pages === 0 || services.data.data.page >= services.data.data.total_pages} onClick={() => updateParameter('page', String(Math.min(services.data!.data.page + 1, services.data!.data.total_pages)), false)}>下一页</button></div>
      </>}>
        <table className="host-table java-table" aria-label="Java 业务服务列表"><thead>{table.getHeaderGroups().map((group) => <tr key={group.id}>{group.headers.map((header) => <th key={header.id} scope="col">{header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}</th>)}</tr>)}</thead><tbody>{table.getRowModel().rows.map((row) => <tr key={row.id}>{row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}</tr>)}</tbody></table>
      </ListTablePanel> : null}
  </section>
}
