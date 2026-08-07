import { afterEach, describe, expect, it } from "vitest"
import "./theme.css"

describe("observability table typography", () => {
  afterEach(() => {
    document.body.replaceChildren()
  })

  it.each([
    ["host-list-table", "host-status", "host-status-dot", "", "750"],
    ["disk-table", "status-badge", "status-badge-dot", "disk-capacity", ""],
    ["mysql-table mysql-table-compact", "status-badge", "status-badge-dot", "", ""],
    ["redis-table", "status-badge", "status-badge-dot", "", ""],
    ["elasticsearch-table", "status-badge", "status-badge-dot", "", ""],
    ["rabbitmq-table", "status-badge", "status-badge-dot", "", ""],
    ["java-table", "status-badge", "status-badge-dot", "", ""],
  ])("computes the exact shared tokens for %s", (tableClasses, badgeClass, dotClass, valueClass, badgeFontWeight) => {
    const value = valueClass ? `<span class="${valueClass}">容量</span>` : ""
    document.body.innerHTML = `<table class="host-table observability-table ${tableClasses}"><thead><tr><th>表头</th></tr></thead><tbody><tr><td>${value}<span class="${badgeClass}"><span class="${dotClass}"></span>正常</span></td></tr></tbody></table>`
    const header = getComputedStyle(document.querySelector("th")!)
    const cell = getComputedStyle(document.querySelector("td")!)
    const badge = getComputedStyle(document.querySelector(`.${badgeClass}`)!)
    const dot = getComputedStyle(document.querySelector(`.${dotClass}`)!)
    const valueStyle = valueClass ? getComputedStyle(document.querySelector(`.${valueClass}`)!) : null

    expect([header.paddingTop, header.paddingRight, header.paddingBottom, header.paddingLeft]).toEqual(["0.3rem", "0.18rem", "0.3rem", "0.18rem"])
    expect([header.fontSize, header.fontWeight, header.lineHeight]).toEqual(["0.62rem", "750", "1.1"])
    if (badgeFontWeight) expect(badge.fontWeight).toBe(badgeFontWeight)
    if (valueStyle) expect(valueStyle.fontSize || cell.fontSize).toBe("0.69rem")
    expect(header.letterSpacing).toBe("0")
    expect([cell.paddingTop, cell.paddingRight, cell.paddingBottom, cell.paddingLeft]).toEqual(["0.3rem", "0.18rem", "0.3rem", "0.18rem"])
    expect([cell.fontSize, cell.fontWeight, cell.lineHeight]).toEqual(["0.69rem", "500", "1.25"])
    expect([badge.fontSize, badge.gap, badge.paddingTop, badge.paddingRight, badge.paddingBottom, badge.paddingLeft]).toEqual(["0.66rem", "0.2rem", "0.14rem", "0.26rem", "0.14rem", "0.26rem"])
    expect([dot.width, dot.height]).toEqual(["0.36rem", "0.36rem"])
  })
})
