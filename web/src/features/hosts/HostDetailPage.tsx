import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useParams } from 'react-router-dom'

import { APIError, apiRequest } from '../../api/client'
import type {
  HostDetailResponse,
  HostMetricKey,
  HostMetricSeries,
  HostMetricsResponse,
  HostStatus,
  MetricValue,
  TrendPoint,
} from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import { MetricCard } from '../../components/MetricCard'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'
import {
  TimeRangeSelector,
  type TimeRange,
} from '../../components/TimeRangeSelector'
import { TrendChart, type TrendSeries } from '../../components/TrendChart'

const statusLabels: Record<HostStatus, string> = {
  online: '在线',
  offline: '离线',
  unknown: '未知',
}

function decimal(value: number) {
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 1,
  }).format(value)
}

function percentage(value: number) {
  return `${decimal(value)}%`
}

function bytesPerSecond(value: number) {
  const units = ['B/s', 'KiB/s', 'MiB/s', 'GiB/s'] as const
  let scaled = value
  let unitIndex = 0
  while (Math.abs(scaled) >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024
    unitIndex += 1
  }
  return `${decimal(scaled)} ${units[unitIndex]}`
}

function metricText(
  metric: MetricValue,
  formatter: (value: number) => string,
) {
  return metric.value === null ? null : formatter(metric.value)
}

function uptime(seconds: number) {
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3_600)
  if (days > 0 && hours > 0) return `${days}天 ${hours}小时`
  if (days > 0) return `${days}天`
  return `${hours}小时`
}

function seriesByMetric(
  series: HostMetricSeries[],
  metric: HostMetricKey,
) {
  return series.find((item) => item.metric === metric)?.points ?? []
}

function summarySentence(
  label: string,
  points: TrendPoint[],
  formatter: (value: number) => string,
) {
  const values = points.flatMap((point) =>
    point.value === null ? [] : [point.value],
  )
  if (values.length === 0) return `${label}趋势：暂无数据。`
  const recent = values.at(-1) as number
  return `${label}趋势：最低 ${formatter(Math.min(...values))}，最高 ${formatter(Math.max(...values))}，最近值 ${formatter(recent)}。`
}

function trendSeries(
  series: HostMetricSeries[],
  definitions: ReadonlyArray<{
    metric: HostMetricKey
    label: string
  }>,
): TrendSeries[] {
  return definitions.map(({ metric, label }) => ({
    name: label,
    points: seriesByMetric(series, metric),
  }))
}

function HostStatusText({ status }: { status: HostStatus }) {
  return (
    <span className="host-status" data-status={status}>
      <span className="host-status-dot" aria-hidden="true" />
      {statusLabels[status]}
    </span>
  )
}

export function HostDetailPage() {
  const { id = '' } = useParams()
  const [range, setRange] = useState<TimeRange>('24h')
  const encodedID = encodeURIComponent(id)

  const current = useQuery({
    queryKey: ['host', id],
    queryFn: ({ signal }) =>
      apiRequest<HostDetailResponse>(`/api/v1/hosts/${encodedID}`, {
        signal,
      }),
    refetchInterval: 30_000,
  })
  const history = useQuery({
    queryKey: ['host-metrics', id, range],
    queryFn: ({ signal }) =>
      apiRequest<HostMetricsResponse>(
        `/api/v1/hosts/${encodedID}/metrics?range=${range}`,
        { signal },
      ),
    refetchInterval: 60_000,
  })

  const currentError = current.error instanceof APIError ? current.error : null
  const historyError = history.error instanceof APIError ? history.error : null
  const historySeries = history.data?.data.series ?? []
  const resourceDefinitions = [
    { metric: 'cpu_usage', label: 'CPU 使用率' },
    { metric: 'memory_usage', label: '内存使用率' },
    { metric: 'load_1', label: '1 分钟负载' },
  ] as const
  const diskDefinitions = [
    { metric: 'disk_read_bytes_per_second', label: '磁盘读取速率' },
    { metric: 'disk_write_bytes_per_second', label: '磁盘写入速率' },
  ] as const
  const networkDefinitions = [
    { metric: 'network_receive_bytes_per_second', label: '网络接收速率' },
    { metric: 'network_transmit_bytes_per_second', label: '网络发送速率' },
  ] as const

  return (
    <section aria-label="主机详情">
      {current.isPending ? (
        <div className="host-detail-loading" role="status">
          正在加载主机详情…
        </div>
      ) : current.isError ? (
        <ErrorPanel
          message={currentError?.message ?? '服务暂时无法处理请求'}
          retryable={currentError?.retryable ?? false}
          onRetry={() => void current.refetch()}
        />
      ) : (
        <>
          <p className="eyebrow">主机详情</p>
          <div className="host-detail-heading">
            <div>
              <h1>{current.data.data.name}</h1>
              <p className="page-description">
                当前状态与最近一次采集的资源指标。
              </p>
            </div>
            <HostStatusText status={current.data.data.status} />
          </div>

          {current.data.meta.stale &&
            current.data.meta.collected_at !== undefined && (
              <StaleBanner collectedAt={current.data.meta.collected_at} />
            )}

          <dl className="host-metadata">
            <div>
              <dt>IP 地址</dt>
              <dd>{current.data.data.ip}</dd>
            </div>
            <div>
              <dt>操作系统</dt>
              <dd>{current.data.data.os}</dd>
            </div>
            <div>
              <dt>运行时间</dt>
              <dd>{uptime(current.data.data.uptime_seconds)}</dd>
            </div>
          </dl>

          <section className="host-detail-section" aria-labelledby="current-metrics-title">
            <h2 id="current-metrics-title">当前指标</h2>
            <div className="metric-grid host-current-metrics">
              <MetricCard
                label="CPU 使用率"
                value={metricText(current.data.data.metrics.cpu_usage, percentage)}
                level={current.data.data.metrics.cpu_usage.level}
              />
              <MetricCard
                label="内存使用率"
                value={metricText(current.data.data.metrics.memory_usage, percentage)}
                level={current.data.data.metrics.memory_usage.level}
              />
              <MetricCard
                label="1 分钟负载"
                value={metricText(current.data.data.metrics.load_1, decimal)}
                level={current.data.data.metrics.load_1.level}
              />
              <MetricCard
                label="磁盘读取速率"
                value={metricText(current.data.data.metrics.disk_read_bytes_per_second, bytesPerSecond)}
                level={current.data.data.metrics.disk_read_bytes_per_second.level}
              />
              <MetricCard
                label="磁盘写入速率"
                value={metricText(current.data.data.metrics.disk_write_bytes_per_second, bytesPerSecond)}
                level={current.data.data.metrics.disk_write_bytes_per_second.level}
              />
              <MetricCard
                label="网络接收速率"
                value={metricText(current.data.data.metrics.network_receive_bytes_per_second, bytesPerSecond)}
                level={current.data.data.metrics.network_receive_bytes_per_second.level}
              />
              <MetricCard
                label="网络发送速率"
                value={metricText(current.data.data.metrics.network_transmit_bytes_per_second, bytesPerSecond)}
                level={current.data.data.metrics.network_transmit_bytes_per_second.level}
              />
            </div>
          </section>
        </>
      )}

      <div className="host-history-heading">
        <div>
          <p className="eyebrow">历史指标</p>
          <p className="page-description">按时间范围查看资源变化。</p>
        </div>
        <TimeRangeSelector value={range} onChange={setRange} />
      </div>

      {history.data?.meta.stale === true &&
        history.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={history.data.meta.collected_at} />
        )}

      {history.isPending ? (
        <div className="host-detail-loading" role="status">
          正在加载历史指标…
        </div>
      ) : history.isError ? (
        <ErrorPanel
          message={historyError?.message ?? '服务暂时无法处理请求'}
          retryable={historyError?.retryable ?? false}
          onRetry={() => void history.refetch()}
        />
      ) : (
        <TrendChart
          title="CPU、内存与负载趋势"
          summary={
            summarySentence(
              'CPU 使用率',
              seriesByMetric(historySeries, 'cpu_usage'),
              percentage,
            ) +
            summarySentence(
              '内存使用率',
              seriesByMetric(historySeries, 'memory_usage'),
              percentage,
            ) +
            summarySentence(
              '1 分钟负载',
              seriesByMetric(historySeries, 'load_1'),
              decimal,
            )
          }
          series={trendSeries(historySeries, resourceDefinitions)}
          unit=""
        />
      )}

      {current.data !== undefined && (
        <section className="host-detail-section" aria-labelledby="filesystem-title">
          <h2 id="filesystem-title">文件系统容量</h2>
          <div className="host-filesystem-panel">
            <table className="host-filesystem-table" aria-label="文件系统容量">
              <thead>
                <tr>
                  <th scope="col">挂载点</th>
                  <th scope="col">使用率</th>
                  <th scope="col">状态</th>
                </tr>
              </thead>
              <tbody>
                {current.data.data.metrics.filesystems.map((filesystem) => (
                  <tr key={filesystem.mountpoint}>
                    <th scope="row">{filesystem.mountpoint}</th>
                    <td>
                      {filesystem.usage.value === null
                        ? '暂无数据'
                        : percentage(filesystem.usage.value)}
                    </td>
                    <td>
                      <StatusBadge level={filesystem.usage.level} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {history.data !== undefined && (
        <>
          <TrendChart
            title="磁盘 I/O 趋势"
            summary={
              summarySentence(
                '磁盘读取速率',
                seriesByMetric(historySeries, 'disk_read_bytes_per_second'),
                bytesPerSecond,
              ) +
              summarySentence(
                '磁盘写入速率',
                seriesByMetric(historySeries, 'disk_write_bytes_per_second'),
                bytesPerSecond,
              )
            }
            series={trendSeries(historySeries, diskDefinitions)}
            unit=" B/s"
          />
          <TrendChart
            title="网络流量趋势"
            summary={
              summarySentence(
                '网络接收速率',
                seriesByMetric(historySeries, 'network_receive_bytes_per_second'),
                bytesPerSecond,
              ) +
              summarySentence(
                '网络发送速率',
                seriesByMetric(historySeries, 'network_transmit_bytes_per_second'),
                bytesPerSecond,
              )
            }
            series={trendSeries(historySeries, networkDefinitions)}
            unit=" B/s"
          />
        </>
      )}
    </section>
  )
}
