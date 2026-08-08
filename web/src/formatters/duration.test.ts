import { describe, expect, it } from "vitest"
import * as duration from "./duration"

const { formatDurationSeconds } = duration

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
    [9007199254741019n, "285616414年 264天 7小时 36分钟"],
  ] as const)("formats %s seconds as %s", (value, want) => {
    expect(formatDurationSeconds(value)).toBe(want)
  })
})

describe("formatDurationDisplay", () => {
  it("limits visible text to two non-zero units while preserving the full title", () => {
    const formatDurationDisplay = Reflect.get(duration, "formatDurationDisplay") as unknown
    expect(formatDurationDisplay).toBeTypeOf("function")

    const format = formatDurationDisplay as (value: number | bigint | null) => {
      text: string
      title: string
    }
    expect(format(null)).toEqual({ text: "暂无数据", title: "暂无数据" })
    expect(format(60)).toEqual({ text: "1分钟", title: "1分钟" })
    expect(format(90_180)).toEqual({ text: "1天 1小时", title: "1天 1小时 3分钟" })
    expect(format(43_912_800)).toEqual({ text: "1年 143天", title: "1年 143天 6小时" })
    expect(format(31_536_300)).toEqual({ text: "1年 5分钟", title: "1年 5分钟" })
    expect(format(9_007_199_254_741_019n)).toEqual({
      text: "285616414年 264天",
      title: "285616414年 264天 7小时 36分钟",
    })
  })
})
