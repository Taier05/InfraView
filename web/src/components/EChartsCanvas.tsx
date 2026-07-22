import { useEffect, useRef } from 'react'
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

import type { TrendSeries } from './TrendChart'
import { formatTrendValue, type TrendValueFormat } from './trendFormat'

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

type TrendChartOption = ComposeOption<
  LineSeriesOption | GridComponentOption | TooltipComponentOption
>

export interface EChartsCanvasProps {
  series: TrendSeries[]
  valueFormat: TrendValueFormat
}

export default function EChartsCanvas({
  series,
  valueFormat,
}: EChartsCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null)

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

  return <div className="trend-chart" ref={containerRef} aria-hidden="true" />
}
