import type { MetricLevel } from '../api/types'

const levelLabels: Record<MetricLevel, string> = {
  normal: '正常',
  warning: '警告',
  critical: '危险',
  unknown: '未知',
}

export interface StatusBadgeProps {
  level: MetricLevel
  label?: string
}

export function StatusBadge({ level, label }: StatusBadgeProps) {
  return (
    <span className="status-badge" data-level={level}>
      <span className="status-badge-dot" aria-hidden="true" />
      {label ?? levelLabels[level]}
    </span>
  )
}
