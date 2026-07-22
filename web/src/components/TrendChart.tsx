import { useEffect, useId, useRef } from 'react'
import {
  init,
  use,
  type ComposeOption,
  type EChartsType,
} from 'echarts/core'
import { LineChart, type LineSeriesOption } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  type GridComponentOption,
  type TooltipComponentOption,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

type TrendChartOption = ComposeOption<
  LineSeriesOption | GridComponentOption | TooltipComponentOption
>

export interface TrendPoint {
  timestamp: string
  value: number | null
}

export interface TrendSeries {
  name: string
  points: TrendPoint[]
}

export type TrendValueFormat = 'percent' | 'bytesPerSecond' | 'number'

export interface TrendChartProps {
  title: string
  series: TrendSeries[]
  valueFormat: TrendValueFormat
}

function decimal(value: number) {
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 1,
  }).format(value)
}

export function formatTrendValue(
  value: number,
  valueFormat: TrendValueFormat,
) {
  if (valueFormat === 'percent') return `${decimal(value)}%`
  if (valueFormat === 'number') return decimal(value)

  const units = ['B/s', 'KiB/s', 'MiB/s', 'GiB/s'] as const
  let scaled = value
  let unitIndex = 0
  while (Math.abs(scaled) >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024
    unitIndex += 1
  }
  return `${decimal(scaled)} ${units[unitIndex]}`
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
  const containerRef = useRef<HTMLDivElement>(null)
  const titleID = useId()

  useEffect(() => {
    const container = containerRef.current
    if (container === null) return

    let chart: EChartsType | null = null
    const valueFormatter = (value: number) =>
      formatTrendValue(value, valueFormat)
    const option: TrendChartOption = {
      animation: false,
      color: ['#9bc5be', '#d9b77b', '#d39a96'],
      grid: { top: 20, right: 20, bottom: 42, left: 52 },
      tooltip: {
        trigger: 'axis',
        valueFormatter: (value) =>
          typeof value === 'number' ? valueFormatter(value) : '暂无数据',
      },
      xAxis: {
        type: 'time',
        axisLabel: { color: '#91a0aa' },
        axisLine: { lineStyle: { color: '#3b4d59' } },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#91a0aa', formatter: valueFormatter },
        splitLine: { lineStyle: { color: '#293640' } },
      },
      series: series.map((item) => ({
        type: 'line',
        name: item.name,
        showSymbol: false,
        connectNulls: false,
        data: item.points.map((point) => [point.timestamp, point.value]),
      })),
    }

    const renderChart = () => {
      if (container.clientWidth === 0 || container.clientHeight === 0) return
      if (chart === null) {
        chart = init(container, undefined, { renderer: 'canvas' })
        chart.setOption(option)
        return
      }
      chart.resize()
    }

    renderChart()
    const resizeObserver =
      typeof ResizeObserver === 'undefined'
        ? null
        : new ResizeObserver(renderChart)
    resizeObserver?.observe(container)
    window.addEventListener('resize', renderChart)

    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', renderChart)
      chart?.dispose()
    }
  }, [series, valueFormat])

  const hasData = series.some((item) =>
    item.points.some((point) => point.value !== null),
  )

  return (
    <section className="trend-panel" aria-labelledby={titleID}>
      <h2 id={titleID}>{title}</h2>
      <p className="sr-only">{seriesSummary(series, valueFormat)}</p>
      {hasData ? (
        <div className="trend-chart" ref={containerRef} aria-hidden="true" />
      ) : (
        <p className="trend-chart-empty">暂无数据</p>
      )}
    </section>
  )
}
