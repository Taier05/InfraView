export type TrendValueFormat = 'percent' | 'bytesPerSecond' | 'number'

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
