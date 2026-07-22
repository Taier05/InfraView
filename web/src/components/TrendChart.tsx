import { lazy, Suspense, useId } from 'react'

import {
  formatTrendValue,
  type TrendValueFormat,
} from './trendFormat'

const EChartsCanvas = lazy(() => import('./EChartsCanvas'))

export interface TrendPoint {
  timestamp: string
  value: number | null
}

export interface TrendSeries {
  name: string
  points: TrendPoint[]
}

export type { TrendValueFormat } from './trendFormat'
export { formatTrendValue } from './trendFormat'

export interface TrendChartProps {
  title: string
  series: TrendSeries[]
  valueFormat: TrendValueFormat
}

function seriesSummary(
  series: TrendSeries[],
  valueFormat: TrendValueFormat,
) {
  return series
    .map((item) => {
      const values = item.points.flatMap((point) =>
        point.value === null ? [] : [point.value],
      )
      if (values.length === 0) return `${item.name}趋势：暂无数据。`
      const recent = values.at(-1) as number
      return `${item.name}趋势：最低 ${formatTrendValue(Math.min(...values), valueFormat)}，最高 ${formatTrendValue(Math.max(...values), valueFormat)}，最近值 ${formatTrendValue(recent, valueFormat)}。`
    })
    .join('')
}

export function TrendChart({
  title,
  series,
  valueFormat,
}: TrendChartProps) {
  const titleID = useId()
  const hasData = series.some((item) =>
    item.points.some((point) => point.value !== null),
  )

  return (
    <section className="trend-panel" aria-labelledby={titleID}>
      <h2 id={titleID}>{title}</h2>
      <p className="sr-only">{seriesSummary(series, valueFormat)}</p>
      {hasData ? (
        <Suspense
          fallback={
            <div className="trend-chart" role="status">
              正在加载趋势图…
            </div>
          }
        >
          <EChartsCanvas series={series} valueFormat={valueFormat} />
        </Suspense>
      ) : (
        <p className="trend-chart-empty">暂无数据</p>
      )}
    </section>
  )
}
