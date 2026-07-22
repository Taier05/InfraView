export type TimeRange = '1h' | '6h' | '24h' | '7d'

const ranges: ReadonlyArray<{ value: TimeRange; label: string }> = [
  { value: '1h', label: '1小时' },
  { value: '6h', label: '6小时' },
  { value: '24h', label: '24小时' },
  { value: '7d', label: '7天' },
]

export interface TimeRangeSelectorProps {
  value: TimeRange
  onChange: (range: TimeRange) => void
}

export function TimeRangeSelector({
  value,
  onChange,
}: TimeRangeSelectorProps) {
  return (
    <div className="time-range-selector" role="group" aria-label="时间范围">
      {ranges.map((range) => (
        <button
          key={range.value}
          type="button"
          aria-pressed={range.value === value}
          onClick={() => onChange(range.value)}
        >
          {range.label}
        </button>
      ))}
    </div>
  )
}
