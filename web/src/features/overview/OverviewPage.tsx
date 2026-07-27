import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  AlertCount,
  MetricLevel,
  OverviewResponse,
} from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import { RefreshControl } from '../../components/RefreshControl'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'

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
        严重 {alerts.critical} · 警告 {alerts.warning}
      </span>
    </div>
  )
}

export function OverviewPage() {
  const overview = useQuery({
    queryKey: ['overview'],
    queryFn: ({ signal }) =>
      apiRequest<OverviewResponse>('/api/v1/overview?range=24h', {
        signal,
      }),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  })

  const error = overview.error
  const apiError = error instanceof APIError ? error : null

  return (
    <section aria-labelledby="overview-title">
      <div className="overview-heading">
        <div>
          <p className="eyebrow">运行态势</p>
          <h1 id="overview-title">基础设施总览</h1>
          <p className="page-description">
            集中查看各基础设施板块状态，每 30 秒自动刷新。
          </p>
        </div>
        <div className="overview-controls" role="group" aria-label="总览控制">
          <RefreshControl
            isFetching={overview.isFetching}
            dataUpdatedAt={overview.dataUpdatedAt}
            onRefresh={() => void overview.refetch()}
          />
        </div>
      </div>

      {overview.data?.meta.stale === true &&
        overview.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={overview.data.meta.collected_at} />
        )}

      {overview.isPending ? (
        <div className="overview-loading" role="status">
          正在加载总览数据…
        </div>
      ) : overview.isError ? (
        <ErrorPanel
          title="无法加载总览数据"
          message={apiError?.message ?? '服务暂时无法处理请求'}
          retryable={apiError?.retryable ?? false}
          retryLabel="重试"
          onRetry={() => void overview.refetch()}
        />
      ) : (
        <div className="overview-status-grid">
          {(() => {
            const { alerts } = overview.data.data
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
                      <small> / {overview.data.data.total}</small>
                    </strong>
                  </div>
                  <div className="module-alert-levels">
                    <StatusBadge
                      level="critical"
                      label={`严重 ${alerts.critical_hosts}`}
                    />
                    <StatusBadge
                      level="warning"
                      label={`警告 ${alerts.warning_hosts}`}
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
                    <StatusBadge
                      level="normal"
                      label={`在线 ${overview.data.data.online}`}
                    />
                    <StatusBadge
                      level="critical"
                      label={`离线 ${overview.data.data.offline}`}
                    />
                    <StatusBadge
                      level="unknown"
                      label={`未知 ${overview.data.data.unknown}`}
                    />
                  </div>
                  <span className="module-status-action">
                    查看主机 <span aria-hidden="true">→</span>
                  </span>
                </div>
              </Link>
            )
          })()}
        </div>
      )}
    </section>
  )
}
