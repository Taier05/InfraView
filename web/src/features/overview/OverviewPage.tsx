import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  AlertCount,
  MetricLevel,
  MySQLOverviewData,
  MySQLOverviewResponse,
  OverviewData,
  OverviewResponse,
} from '../../api/types'
import { RefreshControl } from '../../components/RefreshControl'
import { StatusBadge } from '../../components/StatusBadge'
import { useRefreshIntervalMs } from '../../app/runtime'

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

type ModuleLabel = 'Linux 主机' | 'MySQL'

function moduleName(label: ModuleLabel) {
  return label === 'MySQL' ? 'MySQL 板块' : 'Linux 主机板块'
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
            : '无法加载 MySQL 板块'}
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
    label === 'MySQL' ? 'MySQL 数据已过期' : 'Linux 主机数据已过期'

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
        <div className="module-status-breakdown">
          <StatusBadge level="normal" label={`在线 ${data.online}`} />
          <StatusBadge
            level={data.offline > 0 ? 'critical' : 'normal'}
            label={data.offline > 0 ? `离线 ${data.offline}` : '无离线'}
          />
          <StatusBadge
            level={data.unknown > 0 ? 'unknown' : 'normal'}
            label={data.unknown > 0 ? `未知 ${data.unknown}` : '无未知'}
          />
        </div>
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
        <div className="module-status-breakdown">
          <StatusBadge level="normal" label={`正常 ${data.normal}`} />
          <StatusBadge
            level={data.warning > 0 ? 'warning' : 'normal'}
            label={data.warning > 0 ? `警告 ${data.warning}` : '无警告'}
          />
          <StatusBadge
            level={data.critical > 0 ? 'critical' : 'normal'}
            label={data.critical > 0 ? `严重 ${data.critical}` : '无严重'}
          />
          <StatusBadge
            level={data.unknown > 0 ? 'unknown' : 'normal'}
            label={data.unknown > 0 ? `未知 ${data.unknown}` : '无未知'}
          />
        </div>
        <span className="module-status-action">
          查看 MySQL <span aria-hidden="true">→</span>
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
  const allDataUpdatedAt =
    hostOverview.data !== undefined && mysqlOverview.data !== undefined
      ? Math.min(hostOverview.dataUpdatedAt, mysqlOverview.dataUpdatedAt)
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
            isFetching={hostOverview.isFetching || mysqlOverview.isFetching}
            dataUpdatedAt={allDataUpdatedAt}
            onRefresh={() => {
              void Promise.all([
                hostOverview.refetch(),
                mysqlOverview.refetch(),
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
      </div>
    </section>
  )
}
