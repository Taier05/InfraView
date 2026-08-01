import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  AlertCount,
  DiskOverviewData,
  DiskOverviewResponse,
  MetricLevel,
  MySQLOverviewData,
  MySQLOverviewResponse,
  OverviewData,
  OverviewResponse,
  RedisOverviewData,
  RedisOverviewResponse,
} from '../../api/types'
import { ModuleStatusCardShell } from '../../components/ModuleStatusCardShell'
import { RefreshControl } from '../../components/RefreshControl'
import { StatusBadge } from '../../components/StatusBadge'
import { useRefreshIntervalMs } from '../../app/runtime'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
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

function alertLevel(alerts: AlertCount): MetricLevel {
  if (alerts.critical > 0) return 'critical'
  if (alerts.warning > 0) return 'warning'
  return 'normal'
}

function MetricAlert({ label, alerts }: { label: string; alerts: AlertCount }) {
  const total = alerts.warning + alerts.critical

  return (
    <div className="module-metric-alert" data-level={alertLevel(alerts)}>
      <div>
        <span>{label}</span>
        <strong>{total}</strong>
      </div>
      <span>
        {total === 0
          ? '无异常'
          : `严重 ${alerts.critical} · 警告 ${alerts.warning}`}
      </span>
    </div>
  )
}

type ModuleLabel = 'Linux 主机' | '主机硬盘' | 'MySQL' | 'Redis'

function moduleName(label: ModuleLabel) {
  if (label === 'MySQL') return 'MySQL 板块'
  if (label === 'Redis') return 'Redis 板块'
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
              : '无法加载 Redis 板块'}
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
        当前展示最近一次可用数据，采集时间：
        <time dateTime={collectedAt}>{collectedAt}</time>
      </span>
    </div>
  )
}

function HostStatusCard({ data }: { data: OverviewData }) {
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
        <span className="module-status-action">
          查看主机 <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  )
}

function MySQLStatusCard({ data }: { data: MySQLOverviewData }) {
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
        <span className="module-status-action">
          查看 MySQL <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  )
}

function RedisStatusCard({ data }: { data: RedisOverviewData }) {
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

function DiskStatusCard({ data }: { data: DiskOverviewData }) {
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
  const allDataUpdatedAt =
    hostOverview.data !== undefined &&
    diskOverview.data !== undefined &&
    mysqlOverview.data !== undefined &&
    redisOverview.data !== undefined
      ? Math.min(
          hostOverview.dataUpdatedAt,
          diskOverview.dataUpdatedAt,
          mysqlOverview.dataUpdatedAt,
          redisOverview.dataUpdatedAt,
        )
      : 0

  return (
    <section aria-labelledby="overview-title">
      <div className="overview-heading">
        <div>
          <p className="eyebrow">运行态势</p>
          <h1 id="overview-title">基础设施总览</h1>
          <p className="page-description">
            集中查看各基础设施板块状态，每{' '}
            {refreshIntervalMs / 1_000} 秒自动刷新。
          </p>
        </div>
        <div className="overview-controls" role="group" aria-label="总览控制">
          <RefreshControl
            isFetching={
              hostOverview.isFetching ||
              diskOverview.isFetching ||
              mysqlOverview.isFetching ||
              redisOverview.isFetching
            }
            dataUpdatedAt={allDataUpdatedAt}
            onRefresh={() => {
              void Promise.all([
                hostOverview.refetch(),
                diskOverview.refetch(),
                mysqlOverview.refetch(),
                redisOverview.refetch(),
              ])
            }}
            refreshIntervalSeconds={refreshIntervalMs / 1_000}
          />
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
          <HostStatusCard data={hostOverview.data.data} />
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
          <DiskStatusCard data={diskOverview.data.data} />
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
          <MySQLStatusCard data={mysqlOverview.data.data} />
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
          <RedisStatusCard data={redisOverview.data.data} />
        )}
      </div>
    </section>
  )
}
