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

export interface TrendChartProps {
  title: string
  summary: string
  series: TrendSeries[]
  unit?: string
}

export function TrendChart({
  title,
  summary,
  series,
  unit = '%',
}: TrendChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const titleID = useId()

  useEffect(() => {
    const container = containerRef.current
    if (container === null) return

    let chart: EChartsType | null = null
    const option: TrendChartOption = {
      animation: false,
      color: ['#9bc5be', '#d9b77b', '#d39a96'],
      grid: { top: 20, right: 20, bottom: 42, left: 52 },
      tooltip: {
        trigger: 'axis',
        valueFormatter: (value) =>
          value === null || value === undefined ? '暂无数据' : `${value}${unit}`,
      },
      xAxis: {
        type: 'time',
        axisLabel: { color: '#91a0aa' },
        axisLine: { lineStyle: { color: '#3b4d59' } },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#91a0aa', formatter: `{value}${unit}` },
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
  }, [series, unit])

  const hasData = series.some((item) =>
    item.points.some((point) => point.value !== null),
  )

  return (
    <section className="trend-panel" aria-labelledby={titleID}>
      <h2 id={titleID}>{title}</h2>
      <p className="sr-only">{summary}</p>
      {hasData ? (
        <div className="trend-chart" ref={containerRef} aria-hidden="true" />
      ) : (
        <p className="trend-chart-empty">暂无数据</p>
      )}
    </section>
  )
}
