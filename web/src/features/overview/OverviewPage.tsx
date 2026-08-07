import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  AlertCount,
  DiskOverviewData,
  DiskOverviewResponse,
  ElasticsearchLevelCounts,
  ElasticsearchOverviewData,
  ElasticsearchOverviewResponse,
  JavaAlertCount,
  JavaLevelCounts,
  JavaOverviewData,
  JavaOverviewResponse,
  MetricLevel,
  MySQLOverviewData,
  MySQLOverviewResponse,
  OverviewData,
  OverviewResponse,
  RedisOverviewData,
  RedisOverviewResponse,
  RabbitMQAlertCount,
  RabbitMQLevelCounts,
  RabbitMQOverviewData,
  RabbitMQOverviewResponse,
} from '../../api/types'
import { DataTime } from '../../components/DataTime'
import { ModuleStatusCardShell } from '../../components/ModuleStatusCardShell'
import { StatusBadge } from '../../components/StatusBadge'
import { useRefreshIntervalMs } from '../../app/runtime'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isStrictRecord(value: unknown): value is Record<string, unknown> {
  return isRecord(value) && !Array.isArray(value)
}

function isNaturalCount(value: unknown): value is number {
  return (
    typeof value === 'number' &&
    Number.isSafeInteger(value) &&
    value >= 0
  )
}

function isAlertCount(value: unknown): value is AlertCount {
  return (
    isRecord(value) &&
    isNaturalCount(value.warning) &&
    isNaturalCount(value.critical)
  )
}

function isMetricLevel(value: unknown): value is MetricLevel {
  return (
    value === 'normal' ||
    value === 'warning' ||
    value === 'critical' ||
    value === 'unknown'
  )
}

function isElasticsearchLevelCounts(
  value: unknown,
): value is ElasticsearchLevelCounts {
  if (!isRecord(value)) return false
  const { total, normal, warning, critical, unknown } = value
  return (
    isNaturalCount(total) &&
    isNaturalCount(normal) &&
    isNaturalCount(warning) &&
    isNaturalCount(critical) &&
    isNaturalCount(unknown) &&
    total === normal + warning + critical + unknown
  )
}

function isElasticsearchOverviewResponse(
  value: unknown,
): value is ElasticsearchOverviewResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) {
    return false
  }
  const { data, meta } = value
  return (
    isMetricLevel(data.status) &&
    isElasticsearchLevelCounts(data.clusters) &&
    isElasticsearchLevelCounts(data.nodes) &&
    isRecord(data.alerts) &&
    isAlertCount(data.alerts.cluster_health) &&
    isAlertCount(data.alerts.node_resource) &&
    isAlertCount(data.alerts.unassigned_shards) &&
    isAlertCount(data.alerts.request_rejections) &&
    typeof meta.request_id === 'string' &&
    typeof meta.stale === 'boolean' &&
    (meta.collected_at === undefined ||
      typeof meta.collected_at === 'string')
  )
}

async function requestElasticsearchOverview(signal: AbortSignal) {
  const response = await apiRequest<unknown>(
    '/api/v1/elasticsearch/overview',
    { signal },
  )
  if (!isElasticsearchOverviewResponse(response)) {
    throw new APIError(
      200,
      'invalid_response',
      '服务器响应格式无效',
      '',
      false,
    )
  }
  return response
}

function isRabbitMQLevelCounts(value: unknown): value is RabbitMQLevelCounts {
  if (!isStrictRecord(value)) return false
  const { total, normal, warning, critical, unknown } = value
  return (
    isNaturalCount(total) &&
    isNaturalCount(normal) &&
    isNaturalCount(warning) &&
    isNaturalCount(critical) &&
    isNaturalCount(unknown) &&
    total === normal + warning + critical + unknown
  )
}

function isRabbitMQAlertCount(value: unknown): value is RabbitMQAlertCount {
  return (
    isStrictRecord(value) &&
    isNaturalCount(value.warning) &&
    isNaturalCount(value.critical) &&
    isNaturalCount(value.unknown)
  )
}

function isRabbitMQOverviewResponse(
  value: unknown,
): value is RabbitMQOverviewResponse {
  if (
    !isStrictRecord(value) ||
    !isStrictRecord(value.data) ||
    !isStrictRecord(value.meta)
  ) {
    return false
  }
  const { data, meta } = value
  if (
    !isMetricLevel(data.status) ||
    !isRabbitMQLevelCounts(data.clusters) ||
    !isRabbitMQLevelCounts(data.nodes) ||
    !isStrictRecord(data.alerts) ||
    !isRabbitMQAlertCount(data.alerts.cluster_connectivity) ||
    !isRabbitMQAlertCount(data.alerts.resource_alarms) ||
    !isRabbitMQAlertCount(data.alerts.resource_pressure) ||
    !isRabbitMQAlertCount(data.alerts.collection) ||
    typeof meta.request_id !== 'string' ||
    typeof meta.stale !== 'boolean' ||
    (meta.collected_at !== undefined && typeof meta.collected_at !== 'string')
  ) {
    return false
  }

  const alertCountFits = (alerts: RabbitMQAlertCount, total: number) =>
    alerts.warning + alerts.critical + alerts.unknown <= total
  const derivedStatus: MetricLevel =
    data.clusters.critical > 0 || data.nodes.critical > 0
      ? 'critical'
      : data.clusters.warning > 0 || data.nodes.warning > 0
        ? 'warning'
        : data.clusters.unknown > 0 || data.nodes.unknown > 0
          ? 'unknown'
          : 'normal'

  return (
    data.status === derivedStatus &&
    alertCountFits(data.alerts.cluster_connectivity, data.clusters.total) &&
    alertCountFits(data.alerts.resource_alarms, data.nodes.total) &&
    alertCountFits(data.alerts.resource_pressure, data.nodes.total) &&
    alertCountFits(data.alerts.collection, data.nodes.total)
  )
}

async function requestRabbitMQOverview(signal: AbortSignal) {
  const response = await apiRequest<unknown>('/api/v1/rabbitmq/overview', {
    signal,
  })
  if (!isRabbitMQOverviewResponse(response)) {
    throw new APIError(200, 'invalid_response', '服务器响应格式无效', '', false)
  }
  return response
}

function isJavaLevelCounts(value: unknown): value is JavaLevelCounts {
  if (!isStrictRecord(value)) return false
  const { total, normal, warning, critical, unknown } = value
  return (
    isNaturalCount(total) &&
    isNaturalCount(normal) &&
    isNaturalCount(warning) &&
    isNaturalCount(critical) &&
    isNaturalCount(unknown) &&
    total === normal + warning + critical + unknown
  )
}

function isJavaAlertCount(value: unknown): value is JavaAlertCount {
  return (
    isStrictRecord(value) &&
    isNaturalCount(value.warning) &&
    isNaturalCount(value.critical) &&
    isNaturalCount(value.unknown)
  )
}

function javaOverviewLevel(data: JavaOverviewData): MetricLevel {
  if (data.services.critical > 0) return 'critical'
  if (data.services.warning > 0) return 'warning'
  if (data.services.unknown > 0) return 'unknown'
  return 'normal'
}

function isJavaOverviewResponse(value: unknown): value is JavaOverviewResponse {
  if (
    !isStrictRecord(value) ||
    !isStrictRecord(value.data) ||
    !isStrictRecord(value.meta)
  ) {
    return false
  }
  const { data, meta } = value
  if (
    !isMetricLevel(data.status) ||
    !isJavaLevelCounts(data.services) ||
    !isStrictRecord(data.alerts) ||
    !isJavaAlertCount(data.alerts.health) ||
    !isJavaAlertCount(data.alerts.port) ||
    !isJavaAlertCount(data.alerts.process) ||
    !isJavaAlertCount(data.alerts.collection) ||
    typeof meta.request_id !== 'string' ||
    typeof meta.stale !== 'boolean' ||
    (meta.collected_at !== undefined && typeof meta.collected_at !== 'string')
  ) {
    return false
  }

  const javaData = data as unknown as JavaOverviewData
  const alertCountFits = (alerts: JavaAlertCount) =>
    alerts.warning + alerts.critical + alerts.unknown <= javaData.services.total

  return (
    javaData.status === javaOverviewLevel(javaData) &&
    alertCountFits(data.alerts.health) &&
    alertCountFits(data.alerts.port) &&
    alertCountFits(data.alerts.process) &&
    alertCountFits(data.alerts.collection)
  )
}

async function requestJavaOverview(signal: AbortSignal) {
  const response = await apiRequest<unknown>('/api/v1/java/overview', {
    signal,
  })
  if (!isJavaOverviewResponse(response)) {
    throw new APIError(200, 'invalid_response', '服务器响应格式无效', '', false)
  }
  return response
}

function isDiskOverviewResponse(
  value: unknown,
): value is DiskOverviewResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) {
    return false
  }
  const { data, meta } = value
  const {
    total,
    normal,
    warning,
    critical,
    unknown,
    affected_devices: affectedDevices,
    warning_devices: warningDevices,
    critical_devices: criticalDevices,
    alerts,
  } = data

  if (
    !isNaturalCount(total) ||
    !isNaturalCount(normal) ||
    !isNaturalCount(warning) ||
    !isNaturalCount(critical) ||
    !isNaturalCount(unknown) ||
    !isNaturalCount(affectedDevices) ||
    !isNaturalCount(warningDevices) ||
    !isNaturalCount(criticalDevices) ||
    !isRecord(alerts) ||
    !isAlertCount(alerts.smart_health) ||
    !isAlertCount(alerts.device_warning) ||
    !isAlertCount(alerts.attribute_failure) ||
    !isAlertCount(alerts.collection)
  ) {
    return false
  }

  const alertCounts = [
    alerts.smart_health,
    alerts.device_warning,
    alerts.attribute_failure,
    alerts.collection,
  ]

  return (
    total === normal + warning + critical + unknown &&
    affectedDevices === warning + critical + unknown &&
    warningDevices === warning + unknown &&
    criticalDevices === critical &&
    alertCounts.every(
      (count) =>
        count.warning <= total &&
        count.critical <= total &&
        count.warning + count.critical <= total,
    ) &&
    typeof meta.request_id === 'string' &&
    typeof meta.stale === 'boolean' &&
    (meta.collected_at === undefined || typeof meta.collected_at === 'string')
  )
}

async function requestDiskOverview(signal: AbortSignal) {
  const response = await apiRequest<unknown>('/api/v1/disks/overview', {
    signal,
  })
  if (!isDiskOverviewResponse(response)) {
    throw new APIError(
      200,
      'invalid_response',
      '服务器响应格式无效',
      '',
      false,
    )
  }
  return response
}

function isRedisOverviewResponse(
  value: unknown,
): value is RedisOverviewResponse {
  if (!isRecord(value) || !isRecord(value.data) || !isRecord(value.meta)) {
    return false
  }
  const data = value.data
  if (
    !isNaturalCount(data.total) ||
    !isNaturalCount(data.normal) ||
    !isNaturalCount(data.warning) ||
    !isNaturalCount(data.critical) ||
    !isNaturalCount(data.unknown) ||
    !isNaturalCount(data.affected_instances) ||
    !isNaturalCount(data.warning_instances) ||
    !isNaturalCount(data.critical_instances) ||
    !isRecord(data.roles) ||
    !isNaturalCount(data.roles.master) ||
    !isNaturalCount(data.roles.slave) ||
    !isNaturalCount(data.roles.unknown) ||
    !isRecord(data.alerts) ||
    !isAlertCount(data.alerts.availability) ||
    !isAlertCount(data.alerts.memory) ||
    !isAlertCount(data.alerts.connection) ||
    !isAlertCount(data.alerts.replication)
  ) {
    return false
  }
  return (
    data.total === data.normal + data.warning + data.critical + data.unknown &&
    data.total === data.roles.master + data.roles.slave + data.roles.unknown &&
    data.affected_instances ===
      data.warning + data.critical + data.unknown &&
    data.warning_instances === data.warning + data.unknown &&
    data.critical_instances === data.critical &&
    typeof value.meta.request_id === 'string' &&
    typeof value.meta.stale === 'boolean' &&
    (value.meta.collected_at === undefined ||
      typeof value.meta.collected_at === 'string')
  )
}

async function requestRedisOverview(signal: AbortSignal) {
  const response = await apiRequest<unknown>('/api/v1/redis/overview', {
    signal,
  })
  if (!isRedisOverviewResponse(response)) {
    throw new APIError(
      200,
      'invalid_response',
      '服务器响应格式无效',
      '',
      false,
    )
  }
  return response
}

type MetricAlertCount = AlertCount & { unknown?: number }

function alertLevel(alerts: MetricAlertCount): MetricLevel {
  if (alerts.critical > 0) return 'critical'
  if (alerts.warning > 0) return 'warning'
  if ((alerts.unknown ?? 0) > 0) return 'unknown'
  return 'normal'
}

function MetricAlert({
  label,
  alerts,
}: {
  label: string
  alerts: MetricAlertCount
}) {
  const total = alerts.warning + alerts.critical + (alerts.unknown ?? 0)
  const details =
    total === 0
      ? '无异常'
      : alerts.unknown === undefined
        ? `严重 ${alerts.critical} · 警告 ${alerts.warning}`
      : `严重 ${alerts.critical} · 警告 ${alerts.warning} · 未知 ${alerts.unknown}`

  return (
    <div className="module-metric-alert" data-level={alertLevel(alerts)}>
      <div>
        <span>{label}</span>
        <strong>{total}</strong>
      </div>
      <span>{details}</span>
    </div>
  )
}

type ModuleLabel =
  | 'Linux 主机'
  | '主机硬盘'
  | 'MySQL'
  | 'Redis'
  | 'Elasticsearch'
  | 'RabbitMQ'
  | 'Java 服务'

function moduleName(label: ModuleLabel) {
  if (label === 'MySQL') return 'MySQL 板块'
  if (label === 'Redis') return 'Redis 板块'
  if (label === 'Elasticsearch') return 'Elasticsearch 板块'
  if (label === 'RabbitMQ') return 'RabbitMQ 板块'
  if (label === 'Java 服务') return 'Java 服务板块'
  if (label === '主机硬盘') return '主机硬盘板块'
  return 'Linux 主机板块'
}

function ModuleLoading({ label }: { label: ModuleLabel }) {
  return (
    <div
      className="overview-module-state"
      role="status"
      aria-label={`${moduleName(label)}加载中`}
    >
      正在加载{moduleName(label)}…
    </div>
  )
}

function ModuleError({
  label,
  error,
  onRetry,
}: {
  label: ModuleLabel
  error: unknown
  onRetry: () => void
}) {
  const apiError = error instanceof APIError ? error : null

  return (
    <div
      className="error-panel overview-module-state"
      role="alert"
      aria-label={`${moduleName(label)}加载失败`}
    >
      <div>
        <strong>
          {label === 'Linux 主机'
            ? '无法加载总览数据'
            : label === '主机硬盘'
              ? '无法加载主机硬盘板块'
              : label === 'MySQL'
                ? '无法加载 MySQL 板块'
                : label === 'Redis'
                  ? '无法加载 Redis 板块'
                  : label === 'Elasticsearch'
                    ? '无法加载 Elasticsearch 板块'
                    : label === 'RabbitMQ'
                      ? '无法加载 RabbitMQ 板块'
                      : '无法加载 Java 服务板块'}
        </strong>
        <p>{apiError?.message ?? '服务暂时无法处理请求'}</p>
      </div>
      {apiError?.retryable === true && (
        <button className="secondary-button" type="button" onClick={onRetry}>
          重试
        </button>
      )}
    </div>
  )
}

function ModuleRefreshError({
  label,
  error,
  onRetry,
}: {
  label: ModuleLabel
  error: unknown
  onRetry: () => void
}) {
  const apiError = error instanceof APIError ? error : null

  return (
    <div
      className="error-panel overview-refresh-error"
      role="alert"
      aria-label={`${moduleName(label)}刷新失败`}
    >
      <div>
        <strong>{moduleName(label)}刷新失败</strong>
        <p>{apiError?.message ?? '服务暂时无法处理请求'}</p>
      </div>
      {apiError?.retryable === true && (
        <button className="secondary-button" type="button" onClick={onRetry}>
          重试 {moduleName(label)}
        </button>
      )}
    </div>
  )
}

function ModuleStaleBanner({
  label,
  collectedAt,
}: {
  label: ModuleLabel
  collectedAt: string
}) {
  const staleLabel =
    label === 'MySQL'
      ? 'MySQL 数据已过期'
      : label === 'Redis'
        ? 'Redis 数据已过期'
        : label === 'Elasticsearch'
          ? 'Elasticsearch 数据已过期'
          : label === 'RabbitMQ'
            ? 'RabbitMQ 数据已过期'
            : label === 'Java 服务'
              ? 'Java 服务数据已过期'
            : label === '主机硬盘'
              ? '主机硬盘数据已过期'
              : 'Linux 主机数据已过期'

  return (
    <div
      className="stale-banner"
      role="alert"
      aria-label={staleLabel}
    >
      <strong>{staleLabel}</strong>
      <span>
        当前展示最近一次可用数据，
        <DataTime collectedAt={collectedAt} label="最新数据时间：" />
      </span>
    </div>
  )
}

function HostStatusCard({
  data,
  collectedAt,
}: {
  data: OverviewData
  collectedAt?: string
}) {
  if (data.total === 0) {
    return (
      <Link
        className="module-status-card"
        data-level="empty"
        to="/hosts"
        aria-label="查看 Linux 主机板块"
      >
        <div className="module-status-heading">
          <div>
            <span>主机板块</span>
            <h2>Linux 主机</h2>
          </div>
          <span className="module-status-level" data-level="empty">
            暂无主机
          </span>
        </div>
        <div className="module-overview-empty-state">
          <strong>暂无 Linux 主机</strong>
          <span>尚无可展示的主机健康数据</span>
        </div>
        <div className="module-status-footer">
          <DataTime collectedAt={collectedAt} className="data-time" />
          <span className="module-status-action">
            查看主机 <span aria-hidden="true">→</span>
          </span>
        </div>
      </Link>
    )
  }

  const { alerts } = data
  const level: MetricLevel =
    alerts.critical_hosts > 0
      ? 'critical'
      : alerts.warning_hosts > 0
        ? 'warning'
        : 'normal'
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? '存在警告'
        : '全部正常'

  return (
    <Link
      className="module-status-card"
      data-level={level}
      to="/hosts"
      aria-label="查看 Linux 主机板块"
    >
      <div className="module-status-heading">
        <div>
          <span>主机板块</span>
          <h2>Linux 主机</h2>
        </div>
        <span className="module-status-level" data-level={level}>
          {levelLabel}
        </span>
      </div>

      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常主机</span>
          <strong>
            {alerts.affected_hosts}
            <small> / {data.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={alerts.critical_hosts > 0 ? 'critical' : 'normal'}
            label={
              alerts.critical_hosts > 0
                ? `严重 ${alerts.critical_hosts}`
                : '无严重'
            }
          />
          <StatusBadge
            level={alerts.warning_hosts > 0 ? 'warning' : 'normal'}
            label={
              alerts.warning_hosts > 0
                ? `警告 ${alerts.warning_hosts}`
                : '无警告'
            }
          />
        </div>
      </div>

      <div className="module-metric-alert-grid">
        <MetricAlert label="CPU" alerts={alerts.cpu} />
        <MetricAlert label="内存" alerts={alerts.memory} />
        <MetricAlert label="IO" alerts={alerts.io} />
        <MetricAlert label="网络" alerts={alerts.network} />
      </div>

      <div className="module-status-footer">
        <DataTime collectedAt={collectedAt} className="data-time" />
        <span className="module-status-action">
          查看主机 <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  )
}

function MySQLStatusCard({
  data,
  collectedAt,
}: {
  data: MySQLOverviewData
  collectedAt?: string
}) {
  if (data.total === 0) {
    return (
      <Link
        className="module-status-card mysql-overview-card"
        data-level="empty"
        to="/mysql"
        aria-label="查看 MySQL 板块"
      >
        <div className="module-status-heading">
          <div>
            <span>数据库板块</span>
            <h2>MySQL</h2>
          </div>
          <span className="module-status-level" data-level="empty">
            暂无实例
          </span>
        </div>
        <div className="module-overview-empty-state">
          <strong>暂无 MySQL 实例</strong>
          <span>尚无可展示的实例健康数据</span>
        </div>
        <div className="module-status-footer">
          <DataTime collectedAt={collectedAt} className="data-time" />
          <span className="module-status-action">
            查看 MySQL <span aria-hidden="true">→</span>
          </span>
        </div>
      </Link>
    )
  }

  const level: MetricLevel =
    data.critical_instances > 0
      ? 'critical'
      : data.warning_instances > 0 || data.unknown > 0
        ? 'warning'
        : 'normal'
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? data.unknown > 0
          ? '存在警告或未知'
          : '存在警告'
        : '全部正常'

  return (
    <Link
      className="module-status-card mysql-overview-card"
      data-level={level}
      to="/mysql"
      aria-label="查看 MySQL 板块"
    >
      <div className="module-status-heading">
        <div>
          <span>数据库板块</span>
          <h2>MySQL</h2>
        </div>
        <span className="module-status-level" data-level={level}>
          {levelLabel}
        </span>
      </div>

      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常实例</span>
          <strong>
            {data.affected_instances}
            <small> / {data.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.critical_instances > 0 ? 'critical' : 'normal'}
            label={
              data.critical_instances > 0
                ? `严重 ${data.critical_instances}`
                : '无严重'
            }
          />
          <StatusBadge
            level={data.warning_instances > 0 ? 'warning' : 'normal'}
            label={
              data.warning_instances > 0
                ? `警告风险 ${data.warning_instances}`
                : '无警告风险'
            }
          />
        </div>
      </div>

      <div className="module-metric-alert-grid">
        <MetricAlert label="可用性" alerts={data.alerts.availability} />
        <MetricAlert
          label="复制线程"
          alerts={data.alerts.replication_threads}
        />
        <MetricAlert label="复制延迟" alerts={data.alerts.replication_lag} />
        <MetricAlert
          label="复制数据缺失"
          alerts={data.alerts.replication_data}
        />
      </div>

      <div className="module-status-footer">
        <DataTime collectedAt={collectedAt} className="data-time" />
        <span className="module-status-action">
          查看 MySQL <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  )
}

function RedisStatusCard({
  data,
  collectedAt,
}: {
  data: RedisOverviewData
  collectedAt?: string
}) {
  if (data.total === 0) {
    return (
      <ModuleStatusCardShell
        to="/redis"
        ariaLabel="查看 Redis 板块"
        category="缓存板块"
        title="Redis"
        level="empty"
        levelLabel="暂无实例"
        actionLabel="查看 Redis"
        className="redis-overview-card"
        collectedAt={collectedAt}
        emptyState={{
          title: '暂无 Redis 实例',
          description: '尚无可展示的实例健康数据',
        }}
      />
    )
  }
  const level: MetricLevel =
    data.critical_instances > 0
      ? 'critical'
      : data.warning_instances > 0 || data.unknown > 0
        ? 'warning'
        : 'normal'
  const label =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? data.unknown > 0
          ? '存在警告或未知'
          : '存在警告'
        : '全部正常'
  return (
    <ModuleStatusCardShell
      to="/redis"
      ariaLabel="查看 Redis 板块"
      category="缓存板块"
      title="Redis"
      level={level}
      levelLabel={label}
      actionLabel="查看 Redis"
      className="redis-overview-card"
      collectedAt={collectedAt}
    >
      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常实例</span>
          <strong>
            {data.affected_instances}
            <small> / {data.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.critical_instances > 0 ? 'critical' : 'normal'}
            label={
              data.critical_instances > 0
                ? `严重 ${data.critical_instances}`
                : '无严重'
            }
          />
          <StatusBadge
            level={data.warning_instances > 0 ? 'warning' : 'normal'}
            label={
              data.warning_instances > 0
                ? `警告风险 ${data.warning_instances}`
                : '无警告风险'
            }
          />
        </div>
      </div>
      <div className="module-metric-alert-grid">
        <MetricAlert label="可用性" alerts={data.alerts.availability} />
        <MetricAlert label="内存" alerts={data.alerts.memory} />
        <MetricAlert label="连接" alerts={data.alerts.connection} />
        <MetricAlert label="复制" alerts={data.alerts.replication} />
      </div>
    </ModuleStatusCardShell>
  )
}

function elasticsearchOverviewLevel(
  data: ElasticsearchOverviewData,
): MetricLevel {
  if (data.clusters.critical > 0 || data.nodes.critical > 0) {
    return 'critical'
  }
  if (data.clusters.warning > 0 || data.nodes.warning > 0) {
    return 'warning'
  }
  if (data.clusters.unknown > 0 || data.nodes.unknown > 0) {
    return 'unknown'
  }
  return 'normal'
}

function ElasticsearchStatusCard({
  data,
  collectedAt,
}: {
  data: ElasticsearchOverviewData
  collectedAt?: string
}) {
  if (data.clusters.total === 0 && data.nodes.total === 0) {
    return (
      <ModuleStatusCardShell
        to="/elasticsearch"
        ariaLabel="查看 Elasticsearch 板块"
        category="搜索板块"
        title="Elasticsearch"
        level="empty"
        levelLabel="暂无节点"
        actionLabel="查看 Elasticsearch"
        className="elasticsearch-overview-card"
        collectedAt={collectedAt}
        emptyState={{
          title: '暂无 Elasticsearch 节点',
          description: '尚无可展示的集群与节点健康数据',
        }}
      />
    )
  }

  const level = elasticsearchOverviewLevel(data)
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? '存在警告'
        : level === 'unknown'
          ? '存在未知'
          : '全部正常'
  const affectedNodes =
    data.nodes.critical + data.nodes.warning + data.nodes.unknown
  const warningOrUnknown = data.nodes.warning + data.nodes.unknown
  const warningOrUnknownLevel: MetricLevel =
    data.nodes.warning > 0
      ? 'warning'
      : data.nodes.unknown > 0
        ? 'unknown'
        : 'normal'

  return (
    <ModuleStatusCardShell
      to="/elasticsearch"
      ariaLabel="查看 Elasticsearch 板块"
      category="搜索板块"
      title="Elasticsearch"
      level={level}
      levelLabel={levelLabel}
      actionLabel="查看 Elasticsearch"
      className="elasticsearch-overview-card"
      collectedAt={collectedAt}
    >
      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常节点</span>
          <strong>
            {affectedNodes}
            <small> / {data.nodes.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.nodes.critical > 0 ? 'critical' : 'normal'}
            label={
              data.nodes.critical > 0
                ? `严重 ${data.nodes.critical}`
                : '无严重'
            }
          />
          <StatusBadge
            level={warningOrUnknownLevel}
            label={
              warningOrUnknown > 0
                ? `警告/未知 ${warningOrUnknown}`
                : '无警告/未知'
            }
          />
        </div>
      </div>
      <div className="module-metric-alert-grid">
        <MetricAlert label="集群健康" alerts={data.alerts.cluster_health} />
        <MetricAlert label="节点资源" alerts={data.alerts.node_resource} />
        <MetricAlert
          label="未分配分片"
          alerts={data.alerts.unassigned_shards}
        />
        <MetricAlert
          label="请求拒绝"
          alerts={data.alerts.request_rejections}
        />
      </div>
    </ModuleStatusCardShell>
  )
}

function rabbitMQOverviewLevel(data: RabbitMQOverviewData): MetricLevel {
  if (data.clusters.critical > 0 || data.nodes.critical > 0) {
    return 'critical'
  }
  if (data.clusters.warning > 0 || data.nodes.warning > 0) {
    return 'warning'
  }
  if (data.clusters.unknown > 0 || data.nodes.unknown > 0) {
    return 'unknown'
  }
  return 'normal'
}

function RabbitMQStatusCard({
  data,
  collectedAt,
}: {
  data: RabbitMQOverviewData
  collectedAt?: string
}) {
  if (data.clusters.total === 0 && data.nodes.total === 0) {
    return (
      <ModuleStatusCardShell
        to="/rabbitmq"
        ariaLabel="查看 RabbitMQ 板块"
        category="消息队列板块"
        title="RabbitMQ"
        level="empty"
        levelLabel="暂无节点"
        actionLabel="查看 RabbitMQ"
        className="rabbitmq-overview-card"
        collectedAt={collectedAt}
        emptyState={{
          title: '暂无 RabbitMQ 节点',
          description: '尚无可展示的集群与节点健康数据',
        }}
      />
    )
  }

  const level = rabbitMQOverviewLevel(data)
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? '存在警告'
        : level === 'unknown'
          ? '存在未知'
          : '全部正常'
  const affectedNodes =
    data.nodes.warning + data.nodes.critical + data.nodes.unknown
  const warningOrUnknown = data.nodes.warning + data.nodes.unknown
  const warningOrUnknownLevel: MetricLevel =
    data.nodes.warning > 0
      ? 'warning'
      : data.nodes.unknown > 0
        ? 'unknown'
        : 'normal'

  return (
    <ModuleStatusCardShell
      to="/rabbitmq"
      ariaLabel="查看 RabbitMQ 板块"
      category="消息队列板块"
      title="RabbitMQ"
      level={level}
      levelLabel={levelLabel}
      actionLabel="查看 RabbitMQ"
      className="rabbitmq-overview-card"
      collectedAt={collectedAt}
    >
      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常节点</span>
          <strong>
            {affectedNodes}
            <small> / {data.nodes.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.nodes.critical > 0 ? 'critical' : 'normal'}
            label={
              data.nodes.critical > 0 ? `严重 ${data.nodes.critical}` : '无严重'
            }
          />
          <StatusBadge
            level={warningOrUnknownLevel}
            label={
              warningOrUnknown > 0
                ? `警告/未知 ${warningOrUnknown}`
                : '无警告/未知'
            }
          />
        </div>
      </div>
      <div className="module-metric-alert-grid">
        <MetricAlert
          label="集群通信"
          alerts={data.alerts.cluster_connectivity}
        />
        <MetricAlert label="资源告警" alerts={data.alerts.resource_alarms} />
        <MetricAlert label="资源压力" alerts={data.alerts.resource_pressure} />
        <MetricAlert label="采集状态" alerts={data.alerts.collection} />
      </div>
    </ModuleStatusCardShell>
  )
}

function JavaStatusCard({
  data,
  collectedAt,
}: {
  data: JavaOverviewData
  collectedAt?: string
}) {
  if (data.services.total === 0) {
    return (
      <ModuleStatusCardShell
        to="/java"
        ariaLabel="查看 Java 服务板块"
        category="应用板块"
        title="Java 服务"
        level="empty"
        levelLabel="暂无服务"
        actionLabel="查看 Java 服务"
        className="java-overview-card"
        collectedAt={collectedAt}
        emptyState={{
          title: '暂无 Java 服务',
          description: '尚无可展示的 Java 服务健康数据',
        }}
      />
    )
  }

  const level = javaOverviewLevel(data)
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? '存在警告'
        : level === 'unknown'
          ? '存在未知'
          : '全部正常'
  const affectedServices =
    data.services.warning + data.services.critical + data.services.unknown
  const warningOrUnknown = data.services.warning + data.services.unknown
  const warningOrUnknownLevel: MetricLevel =
    data.services.warning > 0
      ? 'warning'
      : data.services.unknown > 0
        ? 'unknown'
        : 'normal'

  return (
    <ModuleStatusCardShell
      to="/java"
      ariaLabel="查看 Java 服务板块"
      category="应用板块"
      title="Java 服务"
      level={level}
      levelLabel={levelLabel}
      actionLabel="查看 Java 服务"
      className="java-overview-card"
      collectedAt={collectedAt}
    >
      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常服务</span>
          <strong>
            {affectedServices}
            <small> / {data.services.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.services.critical > 0 ? 'critical' : 'normal'}
            label={`严重 ${data.services.critical}`}
          />
          <StatusBadge
            level={warningOrUnknownLevel}
            label={`警告/未知 ${warningOrUnknown}`}
          />
        </div>
      </div>
      <div className="module-metric-alert-grid">
        <MetricAlert label="健康检查" alerts={data.alerts.health} />
        <MetricAlert label="端口状态" alerts={data.alerts.port} />
        <MetricAlert label="进程状态" alerts={data.alerts.process} />
        <MetricAlert label="采集状态" alerts={data.alerts.collection} />
      </div>
    </ModuleStatusCardShell>
  )
}

function DiskStatusCard({
  data,
  collectedAt,
}: {
  data: DiskOverviewData
  collectedAt?: string
}) {
  if (data.total === 0) {
    return (
      <Link
        className="module-status-card disk-overview-card"
        data-level="empty"
        to="/disks"
        aria-label="查看主机硬盘板块"
      >
        <div className="module-status-heading">
          <div>
            <span>存储板块</span>
            <h2>主机硬盘</h2>
          </div>
          <span className="module-status-level" data-level="empty">
            暂无设备
          </span>
        </div>
        <div className="module-overview-empty-state">
          <strong>暂无硬盘设备</strong>
          <span>尚无可展示的硬盘健康数据</span>
        </div>
        <div className="module-status-footer">
          <DataTime collectedAt={collectedAt} className="data-time" />
          <span className="module-status-action">
            查看硬盘 <span aria-hidden="true">→</span>
          </span>
        </div>
      </Link>
    )
  }

  const level: MetricLevel =
    data.critical_devices > 0
      ? 'critical'
      : data.warning_devices > 0
        ? 'warning'
        : 'normal'
  const levelLabel =
    level === 'critical'
      ? '存在严重异常'
      : level === 'warning'
        ? data.unknown > 0
          ? '存在警告或未知'
          : '存在警告'
        : '全部正常'

  return (
    <Link
      className="module-status-card disk-overview-card"
      data-level={level}
      to="/disks"
      aria-label="查看主机硬盘板块"
    >
      <div className="module-status-heading">
        <div>
          <span>存储板块</span>
          <h2>主机硬盘</h2>
        </div>
        <span className="module-status-level" data-level={level}>
          {levelLabel}
        </span>
      </div>

      <div className="module-alert-summary">
        <div className="module-alert-total">
          <span>异常设备</span>
          <strong>
            {data.affected_devices}
            <small> / {data.total}</small>
          </strong>
        </div>
        <div className="module-alert-levels">
          <StatusBadge
            level={data.critical_devices > 0 ? 'critical' : 'normal'}
            label={
              data.critical_devices > 0
                ? `严重 ${data.critical_devices}`
                : '无严重'
            }
          />
          <StatusBadge
            level={data.warning_devices > 0 ? 'warning' : 'normal'}
            label={
              data.warning_devices > 0
                ? `警告风险 ${data.warning_devices}`
                : '无警告风险'
            }
          />
        </div>
      </div>

      <div className="module-metric-alert-grid">
        <MetricAlert label="SMART 自检" alerts={data.alerts.smart_health} />
        <MetricAlert label="设备警告" alerts={data.alerts.device_warning} />
        <MetricAlert label="属性失败" alerts={data.alerts.attribute_failure} />
        <MetricAlert label="采集状态" alerts={data.alerts.collection} />
      </div>

      <div className="module-status-footer">
        <DataTime collectedAt={collectedAt} className="data-time" />
        <span className="module-status-action">
          查看硬盘 <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  )
}

export function OverviewPage() {
  const refreshIntervalMs = useRefreshIntervalMs()
  const hostOverview = useQuery({
    queryKey: ['overview'],
    queryFn: ({ signal }) =>
      apiRequest<OverviewResponse>('/api/v1/overview?range=24h', {
        signal,
      }),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const mysqlOverview = useQuery({
    queryKey: ['mysql-overview'],
    queryFn: ({ signal }) =>
      apiRequest<MySQLOverviewResponse>('/api/v1/mysql/overview', { signal }),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const diskOverview = useQuery({
    queryKey: ['disk-overview'],
    queryFn: ({ signal }) => requestDiskOverview(signal),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const redisOverview = useQuery({
    queryKey: ['redis-overview'],
    queryFn: ({ signal }) => requestRedisOverview(signal),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const elasticsearchOverview = useQuery({
    queryKey: ['elasticsearch-overview'],
    queryFn: ({ signal }) => requestElasticsearchOverview(signal),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const rabbitMQOverview = useQuery({
    queryKey: ['rabbitmq-overview'],
    queryFn: ({ signal }) => requestRabbitMQOverview(signal),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  const javaOverview = useQuery({
    queryKey: ['java-overview'],
    queryFn: ({ signal }) => requestJavaOverview(signal),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  })
  return (
    <section aria-labelledby="overview-title">
      <div className="overview-heading">
        <div>
          <p className="eyebrow">运行态势</p>
          <h1 id="overview-title">基础设施总览</h1>
          <p className="page-description">
            集中查看各基础设施板块状态与最新数据时间。
          </p>
        </div>
      </div>

      {hostOverview.data?.meta.stale === true &&
        hostOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="Linux 主机"
            collectedAt={hostOverview.data.meta.collected_at}
          />
        )}
      {mysqlOverview.data?.meta.stale === true &&
        mysqlOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="MySQL"
            collectedAt={mysqlOverview.data.meta.collected_at}
          />
        )}
      {diskOverview.data?.meta.stale === true &&
        diskOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="主机硬盘"
            collectedAt={diskOverview.data.meta.collected_at}
          />
        )}
      {redisOverview.data?.meta.stale === true &&
        redisOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="Redis"
            collectedAt={redisOverview.data.meta.collected_at}
          />
        )}
      {elasticsearchOverview.data?.meta.stale === true &&
        elasticsearchOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="Elasticsearch"
            collectedAt={elasticsearchOverview.data.meta.collected_at}
          />
        )}
      {rabbitMQOverview.data?.meta.stale === true &&
        rabbitMQOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="RabbitMQ"
            collectedAt={rabbitMQOverview.data.meta.collected_at}
          />
        )}
      {javaOverview.data?.meta.stale === true &&
        javaOverview.data.meta.collected_at !== undefined && (
          <ModuleStaleBanner
            label="Java 服务"
            collectedAt={javaOverview.data.meta.collected_at}
          />
        )}
      {hostOverview.data !== undefined && hostOverview.isError && (
        <ModuleRefreshError
          label="Linux 主机"
          error={hostOverview.error}
          onRetry={() => void hostOverview.refetch()}
        />
      )}
      {mysqlOverview.data !== undefined && mysqlOverview.isError && (
        <ModuleRefreshError
          label="MySQL"
          error={mysqlOverview.error}
          onRetry={() => void mysqlOverview.refetch()}
        />
      )}
      {diskOverview.data !== undefined && diskOverview.isError && (
        <ModuleRefreshError
          label="主机硬盘"
          error={diskOverview.error}
          onRetry={() => void diskOverview.refetch()}
        />
      )}
      {redisOverview.data !== undefined && redisOverview.isError && (
        <ModuleRefreshError
          label="Redis"
          error={redisOverview.error}
          onRetry={() => void redisOverview.refetch()}
        />
      )}
      {elasticsearchOverview.data !== undefined &&
        elasticsearchOverview.isError && (
          <ModuleRefreshError
            label="Elasticsearch"
            error={elasticsearchOverview.error}
            onRetry={() => void elasticsearchOverview.refetch()}
          />
        )}
      {rabbitMQOverview.data !== undefined && rabbitMQOverview.isError && (
        <ModuleRefreshError
          label="RabbitMQ"
          error={rabbitMQOverview.error}
          onRetry={() => void rabbitMQOverview.refetch()}
        />
      )}
      {javaOverview.data !== undefined && javaOverview.isError && (
        <ModuleRefreshError
          label="Java 服务"
          error={javaOverview.error}
          onRetry={() => void javaOverview.refetch()}
        />
      )}

      <div
        className="overview-status-grid overview-compact-grid"
        role="group"
        aria-label="基础设施模块"
      >
        {hostOverview.data === undefined ? (
          hostOverview.isPending ? (
            <ModuleLoading label="Linux 主机" />
          ) : (
            <ModuleError
              label="Linux 主机"
              error={hostOverview.error}
              onRetry={() => void hostOverview.refetch()}
            />
          )
        ) : (
          <HostStatusCard
            data={hostOverview.data.data}
            collectedAt={hostOverview.data.meta.collected_at}
          />
        )}

        {diskOverview.data === undefined ? (
          diskOverview.isPending ? (
            <ModuleLoading label="主机硬盘" />
          ) : (
            <ModuleError
              label="主机硬盘"
              error={diskOverview.error}
              onRetry={() => void diskOverview.refetch()}
            />
          )
        ) : (
          <DiskStatusCard
            data={diskOverview.data.data}
            collectedAt={diskOverview.data.meta.collected_at}
          />
        )}

        {mysqlOverview.data === undefined ? (
          mysqlOverview.isPending ? (
            <ModuleLoading label="MySQL" />
          ) : (
            <ModuleError
              label="MySQL"
              error={mysqlOverview.error}
              onRetry={() => void mysqlOverview.refetch()}
            />
          )
        ) : (
          <MySQLStatusCard
            data={mysqlOverview.data.data}
            collectedAt={mysqlOverview.data.meta.collected_at}
          />
        )}

        {redisOverview.data === undefined ? (
          redisOverview.isPending ? (
            <ModuleLoading label="Redis" />
          ) : (
            <ModuleError
              label="Redis"
              error={redisOverview.error}
              onRetry={() => void redisOverview.refetch()}
            />
          )
        ) : (
          <RedisStatusCard
            data={redisOverview.data.data}
            collectedAt={redisOverview.data.meta.collected_at}
          />
        )}

        {elasticsearchOverview.data === undefined ? (
          elasticsearchOverview.isPending ? (
            <ModuleLoading label="Elasticsearch" />
          ) : (
            <ModuleError
              label="Elasticsearch"
              error={elasticsearchOverview.error}
              onRetry={() => void elasticsearchOverview.refetch()}
            />
          )
        ) : (
          <ElasticsearchStatusCard
            data={elasticsearchOverview.data.data}
            collectedAt={elasticsearchOverview.data.meta.collected_at}
          />
        )}

        {rabbitMQOverview.data === undefined ? (
          rabbitMQOverview.isPending ? (
            <ModuleLoading label="RabbitMQ" />
          ) : (
            <ModuleError
              label="RabbitMQ"
              error={rabbitMQOverview.error}
              onRetry={() => void rabbitMQOverview.refetch()}
            />
          )
        ) : (
          <RabbitMQStatusCard
            data={rabbitMQOverview.data.data}
            collectedAt={rabbitMQOverview.data.meta.collected_at}
          />
        )}

        {javaOverview.data === undefined ? (
          javaOverview.isPending ? (
            <ModuleLoading label="Java 服务" />
          ) : (
            <ModuleError
              label="Java 服务"
              error={javaOverview.error}
              onRetry={() => void javaOverview.refetch()}
            />
          )
        ) : (
          <JavaStatusCard
            data={javaOverview.data.data}
            collectedAt={javaOverview.data.meta.collected_at}
          />
        )}
      </div>
    </section>
  )
}
