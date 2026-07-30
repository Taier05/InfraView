import { expect, test, type Page } from '@playwright/test'

const username = process.env.INFRAVIEW_E2E_USERNAME ?? ''
const password = process.env.INFRAVIEW_E2E_PASSWORD ?? ''

async function login(page: Page) {
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/login')
  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()
  await expect(
    page.getByRole('heading', { name: '基础设施总览' }),
  ).toBeVisible()
}

type ColumnGeometry = {
  headerStarts: number[]
  headerTextLineCounts: number[]
  headerWhiteSpace: string[]
  cellStarts: number[]
}

async function firstRowContentStarts(
  page: Page,
  tableSelector: string,
): Promise<ColumnGeometry> {
  return page.locator(tableSelector).evaluate((table) => {
    const headerCells = Array.from(table.querySelectorAll('thead th'))
    const firstRow = table.querySelector('tbody tr')
    if (firstRow === null) {
      throw new Error('表格没有数据行，无法验证列内容起始边界')
    }
    const dataCells = Array.from(firstRow.querySelectorAll(':scope > td'))
    if (headerCells.length !== dataCells.length) {
      throw new Error('表头与首行列数不一致，无法验证列内容起始边界')
    }

    const textRange = (cell: HTMLTableCellElement) => {
      const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT)
      for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
        const range = document.createRange()
        range.selectNodeContents(node)
        const rects = Array.from(range.getClientRects()).filter(
          (rect) => rect.width > 0 && rect.height > 0,
        )
        if (rects.length > 0) {
          return { left: rects[0].left, lineCount: rects.length }
        }
      }
      throw new Error('单元格没有可测量的文本边界')
    }

    const headers = headerCells.map((cell) => ({
      ...textRange(cell),
      whiteSpace: getComputedStyle(cell).whiteSpace,
    }))
    return {
      headerStarts: headers.map((header) => header.left),
      headerTextLineCounts: headers.map((header) => header.lineCount),
      headerWhiteSpace: headers.map((header) => header.whiteSpace),
      cellStarts: dataCells.map((cell) => textRange(cell).left),
    }
  })
}

function expectAlignedContentStarts(
  geometry: ColumnGeometry,
  start: number,
  end: number,
) {
  for (let index = start; index <= end; index += 1) {
    expect(
      Math.abs(geometry.headerStarts[index] - geometry.cellStarts[index]),
    ).toBeLessThanOrEqual(1)
  }
}

function expectSingleLineHeaders(geometry: ColumnGeometry) {
  for (let index = 0; index < geometry.headerStarts.length; index += 1) {
    expect(geometry.headerWhiteSpace[index]).toBe('nowrap')
    expect(geometry.headerTextLineCounts[index]).toBe(1)
  }
}

test('原 8080 在 1440×900 下显示四列紧凑模块位', async ({ page }) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)

  const moduleGrid = page.getByRole('group', { name: '基础设施模块' })
  const hostCard = page.getByRole('link', { name: '查看 Linux 主机板块' })
  const mysqlCard = page.getByRole('link', { name: '查看 MySQL 板块' })

  const [gridBox, hostBox, mysqlBox] = await Promise.all([
    moduleGrid.boundingBox(),
    hostCard.boundingBox(),
    mysqlCard.boundingBox(),
  ])
  expect(gridBox).not.toBeNull()
  expect(hostBox).not.toBeNull()
  expect(mysqlBox).not.toBeNull()
  expect(Math.abs(hostBox!.width - mysqlBox!.width)).toBeLessThanOrEqual(1)
  expect(hostBox!.width).toBeLessThanOrEqual(gridBox!.width * 0.26)
  expect(Math.abs(hostBox!.y - mysqlBox!.y)).toBeLessThanOrEqual(1)
})

test('原 8080 的 MySQL 10 列在 1440×900 下无横向滚动', async ({
  page,
}) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)
  await page.getByRole('link', { name: '查看 MySQL 板块' }).click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()

  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(10)
  for (const header of await headers.all()) {
    await expect(header).toBeVisible()
  }

  const geometry = await firstRowContentStarts(page, '.mysql-table')
  expectSingleLineHeaders(geometry)
  expectAlignedContentStarts(geometry, 2, 9)

  const tableViewport = await page.locator('.mysql-table-scroll').evaluate(
    (element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }),
  )
  expect(tableViewport.scrollWidth).toBeLessThanOrEqual(
    tableViewport.clientWidth,
  )

  const documentViewport = await page.locator('html').evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(documentViewport.scrollWidth).toBeLessThanOrEqual(
    documentViewport.clientWidth,
  )
})

test('原 8080 的 MySQL 中等宽度控制区以两行三列布局且无文档横向溢出', async ({
  page,
}) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)
  await page.getByRole('link', { name: '查看 MySQL 板块' }).click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()
  await page.setViewportSize({ width: 900, height: 900 })

  const controlGrid = await page.locator('.mysql-list-controls').evaluate(
    (element) => ({
      columns: getComputedStyle(element).gridTemplateColumns
        .split(' ')
        .filter(Boolean).length,
      rows: getComputedStyle(element).gridTemplateRows
        .split(' ')
        .filter(Boolean).length,
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }),
  )
  expect(controlGrid.columns).toBe(3)
  expect(controlGrid.rows).toBe(2)
  expect(controlGrid.scrollWidth).toBeLessThanOrEqual(controlGrid.clientWidth)

  const documentViewport = await page.locator('html').evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(documentViewport.scrollWidth).toBeLessThanOrEqual(
    documentViewport.clientWidth,
  )
})

test('原 8080 的主机共享列在 1440×900 下从 CPU 使用率到状态对齐', async ({
  page,
}) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)
  await page.getByRole('link', { name: '查看 Linux 主机板块' }).click()
  await expect(page.getByRole('heading', { name: '主机' })).toBeVisible()

  expectAlignedContentStarts(
    await firstRowContentStarts(page, '.host-table'),
    4,
    10,
  )

  const tableViewport = await page.locator('.host-table-scroll').evaluate(
    (element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }),
  )
  expect(tableViewport.scrollWidth).toBeLessThanOrEqual(
    tableViewport.clientWidth,
  )

  const documentViewport = await page.locator('html').evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(documentViewport.scrollWidth).toBeLessThanOrEqual(
    documentViewport.clientWidth,
  )
})

test('原 8080 的 MySQL 语义筛选和认证后错误审计保持匿名只读', async ({
  page,
}) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)

  let consoleErrorCount = 0
  let pageErrorCount = 0
  let requestFailureCount = 0
  let mysqlAPIErrorCount = 0
  let sawLabeledListRequest = false
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrorCount += 1
  })
  page.on('pageerror', () => {
    pageErrorCount += 1
  })
  page.on('requestfailed', () => {
    requestFailureCount += 1
  })
  page.on('request', (request) => {
    const requestURL = new URL(request.url())
    if (
      requestURL.pathname === '/api/v1/mysql/instances' &&
      Boolean(requestURL.searchParams.get('label'))
    ) {
      sawLabeledListRequest = true
    }
  })
  page.on('response', (response) => {
    const responseURL = new URL(response.url())
    if (
      responseURL.pathname === '/api/v1/mysql/instances' &&
      !response.ok()
    ) {
      mysqlAPIErrorCount += 1
    }
  })

  await page.getByRole('link', { name: '查看 MySQL 板块' }).click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()

  const labelBeforeStatus = await page
    .locator('.mysql-list-controls')
    .evaluate((controls) => {
      const labels = Array.from(controls.querySelectorAll(':scope > label'))
      const labelIndex = labels.findIndex(
        (label) =>
          label.querySelector(':scope > span')?.textContent?.trim() ===
          '实例标签',
      )
      const statusIndex = labels.findIndex(
        (label) =>
          label.querySelector(':scope > span')?.textContent?.trim() ===
          '实例状态',
      )
      return labelIndex >= 0 && statusIndex > labelIndex
    })
  expect(labelBeforeStatus).toBe(true)

  const labelSelect = page.getByLabel('实例标签')
  await expect
    .poll(() => labelSelect.locator('option').count())
    .toBeGreaterThan(1)
  const labeledResponse = page.waitForResponse((response) => {
    const responseURL = new URL(response.url())
    return (
      responseURL.pathname === '/api/v1/mysql/instances' &&
      Boolean(responseURL.searchParams.get('label'))
    )
  })
  await labelSelect.selectOption({ index: 1 })
  await labeledResponse
  expect(
    await labelSelect.evaluate(
      (element) => (element as HTMLSelectElement).value.length > 0,
    ),
  ).toBe(true)
  expect(
    await page.evaluate(() =>
      Boolean(new URL(window.location.href).searchParams.get('label')),
    ),
  ).toBe(true)
  expect(sawLabeledListRequest).toBe(true)

  const firstAddressCell = page.locator(
    '.mysql-table tbody tr:first-child td:first-child',
  )
  await expect(firstAddressCell).toBeVisible()
  expect(
    await firstAddressCell.evaluate((cell) => {
      const text = cell.textContent?.trim() ?? ''
      const title =
        cell.querySelector('[title]')?.getAttribute('title')?.trim() ?? ''
      return text.length > 0 && title === text && !text.includes('·')
    }),
  ).toBe(true)

  expect(
    await page.getByLabel('读写属性').evaluate((element) => {
      const labels = Array.from(
        (element as HTMLSelectElement).querySelectorAll('option'),
        (option) => option.textContent?.trim() ?? '',
      )
      return labels.includes('读写') && !labels.includes('可写')
    }),
  ).toBe(true)

  expect(
    await page
      .locator('.mysql-table tbody tr td:nth-child(7)')
      .evaluateAll(
        (cells) =>
          cells.length > 0 &&
          cells.every((cell) => {
            const text = cell.textContent?.trim() ?? ''
            if (text === '—') return true
            const parts = text.split(' / ')
            if (parts.length !== 2) return false
            const [capacity, usage] = parts
            const capacityValid =
              capacity === '—' ||
              /^\d+(?:\.\d+)? (?:B|KiB|MiB|GiB|TiB)$/.test(capacity)
            const usageValid =
              usage === '—' || /^\d+(?:\.\d+)?%$/.test(usage)
            return (
              capacityValid &&
              usageValid &&
              !(capacity === '—' && usage === '—')
            )
          }),
      ),
  ).toBe(true)

  expect(consoleErrorCount).toBe(0)
  expect(pageErrorCount).toBe(0)
  expect(requestFailureCount).toBe(0)
  expect(mysqlAPIErrorCount).toBe(0)
})
