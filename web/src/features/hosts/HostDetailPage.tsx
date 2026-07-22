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
} from '../../api/types'
import { ErrorPanel } from '../../components/ErrorPanel'
import { MetricCard } from '../../components/MetricCard'
import { StaleBanner } from '../../components/StaleBanner'
import { StatusBadge } from '../../components/StatusBadge'
import {
  TimeRangeSelector,
  type TimeRange,
} from '../../components/TimeRangeSelector'
import {
  formatTrendValue,
  TrendChart,
  type TrendSeries,
} from '../../components/TrendChart'

const statusLabels: Record<HostStatus, string> = {
  online: '在线',
  offline: '离线',
  unknown: '未知',
}

function decimal(value: number) {
  return formatTrendValue(value, 'number')
}

function percentage(value: number) {
  return formatTrendValue(value, 'percent')
}

function bytesPerSecond(value: number) {
  return formatTrendValue(value, 'bytesPerSecond')
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
  const currentError = current.error instanceof APIError ? current.error : null
  const hostNotFound = currentError?.code === 'host_not_found'
  const history = useQuery({
    queryKey: ['host-metrics', id, range],
    queryFn: ({ signal }) =>
      apiRequest<HostMetricsResponse>(
        `/api/v1/hosts/${encodedID}/metrics?range=${range}`,
        { signal },
      ),
    refetchInterval: 60_000,
    enabled: !hostNotFound,
  })

  const historyError = history.error instanceof APIError ? history.error : null
  const currentData = current.data
  const historyData = history.data
  const historySeries = historyData?.data.series ?? []
  const resourceDefinitions = [
    { metric: 'cpu_usage', label: 'CPU 使用率' },
    { metric: 'memory_usage', label: '内存使用率' },
  ] as const
  const loadDefinitions = [
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
      {currentData === undefined && current.isPending ? (
        <div className="host-detail-loading" role="status">
          正在加载主机详情…
        </div>
      ) : currentData === undefined ? (
        <ErrorPanel
          title="无法加载当前主机指标"
          message={currentError?.message ?? '服务暂时无法处理请求'}
          retryable={currentError?.retryable ?? false}
          retryLabel="重试当前指标"
          onRetry={() => void current.refetch()}
        />
      ) : (
        <>
          <p className="eyebrow">主机详情</p>
          <div className="host-detail-heading">
            <div>
              <h1>{currentData.data.name}</h1>
              <p className="page-description">
                当前状态与最近一次采集的资源指标。
              </p>
            </div>
            <HostStatusText status={currentData.data.status} />
          </div>

          {currentData.meta.stale &&
            currentData.meta.collected_at !== undefined && (
              <StaleBanner collectedAt={currentData.meta.collected_at} />
            )}

          {currentError !== null && (
            <div className="host-refresh-error">
              <ErrorPanel
                title="当前指标刷新失败"
                message={currentError?.message ?? '服务暂时无法处理请求'}
                retryable={currentError?.retryable ?? false}
                retryLabel="重试当前指标"
                onRetry={() => void current.refetch()}
              />
            </div>
          )}

          <dl className="host-metadata">
            <div>
              <dt>IP 地址</dt>
              <dd>{currentData.data.ip}</dd>
            </div>
            <div>
              <dt>操作系统</dt>
              <dd>{currentData.data.os}</dd>
            </div>
            <div>
              <dt>运行时间</dt>
              <dd>{uptime(currentData.data.uptime_seconds)}</dd>
            </div>
          </dl>

          <section
            className="host-detail-section"
            aria-labelledby="current-metrics-title"
          >
            <h2 id="current-metrics-title">当前指标</h2>
            <div className="metric-grid host-current-metrics">
              <MetricCard
                label="CPU 使用率"
                value={metricText(currentData.data.metrics.cpu_usage, percentage)}
                level={currentData.data.metrics.cpu_usage.level}
              />
              <MetricCard
                label="内存使用率"
                value={metricText(
                  currentData.data.metrics.memory_usage,
                  percentage,
                )}
                level={currentData.data.metrics.memory_usage.level}
              />
              <MetricCard
                label="1 分钟负载"
                value={metricText(currentData.data.metrics.load_1, decimal)}
                level={currentData.data.metrics.load_1.level}
              />
              <MetricCard
                label="磁盘读取速率"
                value={metricText(
                  currentData.data.metrics.disk_read_bytes_per_second,
                  bytesPerSecond,
                )}
                level={
                  currentData.data.metrics.disk_read_bytes_per_second.level
                }
              />
              <MetricCard
                label="磁盘写入速率"
                value={metricText(
                  currentData.data.metrics.disk_write_bytes_per_second,
                  bytesPerSecond,
                )}
                level={
                  currentData.data.metrics.disk_write_bytes_per_second.level
                }
              />
              <MetricCard
                label="网络接收速率"
                value={metricText(
                  currentData.data.metrics.network_receive_bytes_per_second,
                  bytesPerSecond,
                )}
                level={
                  currentData.data.metrics.network_receive_bytes_per_second
                    .level
                }
              />
              <MetricCard
                label="网络发送速率"
                value={metricText(
                  currentData.data.metrics.network_transmit_bytes_per_second,
                  bytesPerSecond,
                )}
                level={
                  currentData.data.metrics.network_transmit_bytes_per_second
                    .level
                }
              />
            </div>
          </section>
        </>
      )}

      {!hostNotFound && (
        <>
          <div className="host-history-heading">
            <div>
              <p className="eyebrow">历史指标</p>
              <p className="page-description">按时间范围查看资源变化。</p>
            </div>
            <TimeRangeSelector value={range} onChange={setRange} />
          </div>

          {historyData?.meta.stale === true &&
            historyData.meta.collected_at !== undefined && (
              <StaleBanner collectedAt={historyData.meta.collected_at} />
            )}

          {historyData === undefined && history.isPending ? (
            <div className="host-detail-loading" role="status">
              正在加载历史指标…
            </div>
          ) : historyData === undefined ? (
            <ErrorPanel
              title="无法加载历史指标"
              message={historyError?.message ?? '服务暂时无法处理请求'}
              retryable={historyError?.retryable ?? false}
              retryLabel="重试历史指标"
              onRetry={() => void history.refetch()}
            />
          ) : (
            <>
              {historyError !== null && (
                <div className="host-refresh-error">
                  <ErrorPanel
                    title="历史指标刷新失败"
                    message={historyError?.message ?? '服务暂时无法处理请求'}
                    retryable={historyError?.retryable ?? false}
                    retryLabel="重试历史指标"
                    onRetry={() => void history.refetch()}
                  />
                </div>
              )}
              <TrendChart
                title="CPU 与内存使用率趋势"
                series={trendSeries(historySeries, resourceDefinitions)}
                valueFormat="percent"
              />
              <TrendChart
                title="1 分钟负载趋势"
                series={trendSeries(historySeries, loadDefinitions)}
                valueFormat="number"
              />
            </>
          )}

          {currentData !== undefined && (
            <section
              className="host-detail-section"
              aria-labelledby="filesystem-title"
            >
              <h2 id="filesystem-title">文件系统容量</h2>
              <div className="host-filesystem-panel">
                <table
                  className="host-filesystem-table"
                  aria-label="文件系统容量"
                >
                  <thead>
                    <tr>
                      <th scope="col">挂载点</th>
                      <th scope="col">使用率</th>
                      <th scope="col">状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    {currentData.data.metrics.filesystems.length === 0 ? (
                      <tr>
                        <td colSpan={3} className="host-filesystem-empty">
                          没有文件系统数据
                        </td>
                      </tr>
                    ) : (
                      currentData.data.metrics.filesystems.map((filesystem) => (
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
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </section>
          )}

          {historyData !== undefined && (
            <>
              <TrendChart
                title="磁盘 I/O 趋势"
                series={trendSeries(historySeries, diskDefinitions)}
                valueFormat="bytesPerSecond"
              />
              <TrendChart
                title="网络流量趋势"
                series={trendSeries(historySeries, networkDefinitions)}
                valueFormat="bytesPerSecond"
              />
            </>
          )}
        </>
      )}
    </section>
  )
}
