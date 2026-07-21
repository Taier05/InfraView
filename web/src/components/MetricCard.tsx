import { useId, type ReactNode } from 'react'

import type { MetricLevel } from '../api/types'
import { StatusBadge } from './StatusBadge'

export interface MetricCardProps {
  label: string
  value: string | number | null
  unit?: string
  level?: MetricLevel
  statusLabel?: string
  children?: ReactNode
}

export function MetricCard({
  label,
  value,
  unit,
  level,
  statusLabel,
  children,
}: MetricCardProps) {
  const labelID = useId()

  return (
    <article
      className="metric-card"
      aria-labelledby={labelID}
      data-level={level}
    >
      <div className="metric-card-heading">
        <h2 id={labelID}>{label}</h2>
        {level !== undefined && (
          <StatusBadge level={level} label={statusLabel} />
        )}
      </div>
      <p className="metric-card-value">
        {value === null ? (
          <span className="metric-card-empty">暂无数据</span>
        ) : (
          <>
            {value}
            {unit !== undefined && (
              <span className="metric-card-unit">{unit}</span>
            )}
          </>
        )}
      </p>
      {children !== undefined && (
        <div className="metric-card-details">{children}</div>
      )}
    </article>
  )
}
