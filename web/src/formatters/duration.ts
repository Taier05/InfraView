export type DurationSeconds = number | bigint | null

const minute = 60n
const hour = 3_600n
const day = 86_400n
const year = 31_536_000n

export function formatDurationSeconds(value: DurationSeconds): string {
  const seconds = durationSeconds(value)
  if (seconds === null) return "暂无数据"
  if (seconds < minute) return "不足1分钟"

  let remaining = seconds
  const parts: string[] = []
  for (const [unit, label] of [[year, "年"], [day, "天"], [hour, "小时"], [minute, "分钟"]] as const) {
    const amount = remaining / unit
    if (amount > 0n) parts.push(`${amount}${label}`)
    remaining %= unit
  }
  return parts.join(" ")
}

function durationSeconds(value: DurationSeconds): bigint | null {
  if (value === null) return null
  if (typeof value === "bigint") return value >= 0n ? value : null
  if (!Number.isFinite(value) || value < 0) return null
  return BigInt(Math.floor(value))
}
