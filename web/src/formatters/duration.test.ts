import { describe, expect, it } from "vitest"
import { formatDurationSeconds } from "./duration"

describe("formatDurationSeconds", () => {
  it.each([
    [null, "暂无数据"],
    [-1, "暂无数据"],
    [-1n, "暂无数据"],
    [Number.NaN, "暂无数据"],
    [Number.POSITIVE_INFINITY, "暂无数据"],
    [0, "不足1分钟"],
    [59, "不足1分钟"],
    [60, "1分钟"],
    [3599, "59分钟"],
    [3600, "1小时"],
    [90180, "1天 1小时 3分钟"],
    [31536000, "1年"],
    [43912800, "1年 143天 6小时"],
    [31536300, "1年 5分钟"],
    [60.9, "1分钟"],
    [31536000000000000n, "1000000000年"],
  ] as const)("formats %s seconds as %s", (value, want) => {
    expect(formatDurationSeconds(value)).toBe(want)
  })
})
