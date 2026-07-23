import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type { OverviewResponse } from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'

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
          <button
            className="secondary-button"
            type="button"
            disabled={overview.isFetching}
            onClick={() => void overview.refetch()}
          >
            刷新
          </button>
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
          <Link
            className="module-status-card"
            to="/hosts"
            aria-label="查看 Linux 主机板块"
          >
            <div className="module-status-heading">
              <div>
                <span>主机板块</span>
                <h2>Linux 主机</h2>
              </div>
              <span className="module-status-action">
                查看主机 <span aria-hidden="true">→</span>
              </span>
            </div>
            <div className="module-status-body">
              <div className="module-status-total">
                <span>主机总数</span>
                <strong>{overview.data.data.total}</strong>
              </div>
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
            </div>
          </Link>
        </div>
      )}
    </section>
  )
}
