import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { APIError, apiRequest } from '../../api/client'
import type {
  MetricValue,
  OverviewResponse,
  OverviewTrend,
} from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import { MetricCard } from '../../components/MetricCard'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'
import { TrendChart } from '../../components/TrendChart'
import {
  TimeRangeSelector,
  type TimeRange,
} from '../../components/TimeRangeSelector'

function percentage(value: MetricValue) {
  if (value.value === null) return null
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 1,
  }).format(value.value)
}

const trendLabels: Record<OverviewTrend['key'], string> = {
  cpu_usage: 'CPU 使用率',
  memory_usage: '内存使用率',
}

export function OverviewPage() {
  const [range, setRange] = useState<TimeRange>('24h')
  const overview = useQuery({
    queryKey: ['overview', range],
    queryFn: ({ signal }) =>
      apiRequest<OverviewResponse>(`/api/v1/overview?range=${range}`, {
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
            集中查看主机健康与当前资源利用率，每 30 秒自动刷新。
          </p>
        </div>
        <div className="overview-controls" role="group" aria-label="总览控制">
          <TimeRangeSelector value={range} onChange={setRange} />
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
        <>
          <div className="metric-grid">
            <MetricCard label="主机总数" value={overview.data.data.total}>
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
            </MetricCard>
            <MetricCard
              label="在线主机"
              value={overview.data.data.online}
              level="normal"
              statusLabel="在线"
            />
            <MetricCard
              label="CPU 平均使用率"
              value={percentage(overview.data.data.cpu_average)}
              unit="%"
              level={overview.data.data.cpu_average.level}
            />
            <MetricCard
              label="内存平均使用率"
              value={percentage(overview.data.data.memory_average)}
              unit="%"
              level={overview.data.data.memory_average.level}
            />
          </div>
          <TrendChart
            title="资源使用趋势"
            valueFormat="percent"
            series={overview.data.data.trends.map((trend) => ({
              name: trendLabels[trend.key],
              points: trend.points,
            }))}
          />
        </>
      )}
    </section>
  )
}
