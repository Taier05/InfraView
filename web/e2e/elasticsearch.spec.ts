import { expect, test, type Page } from '@playwright/test'

const username = process.env.INFRAVIEW_E2E_USERNAME ?? 'e2e-admin'
const password =
  process.env.INFRAVIEW_E2E_PASSWORD ?? 'e2e-quote-"-slash-\\-password'

const exactHeaders = [
  '节点名称',
  '所属集群',
  '节点地址',
  '节点角色',
  '集群健康',
  'JVM堆使用率',
  '磁盘使用率',
  'CPU使用率',
  '索引速率',
  '搜索速率',
  '文档数',
  '存储大小',
  '线程池队列',
  '拒绝速率',
  '运行时间',
  '状态',
] as const

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()
  await expect(
    page.getByRole('heading', { name: '基础设施总览' }),
  ).toBeVisible()
}

async function expectNoDestructiveControls(page: Page) {
  const destructive =
    /重启|删除|执行|远程命令|切换|修改|发布|配置|下发|迁移|重分配|关闭|打开|合并|清理/
  await expect(page.getByRole('button', { name: destructive })).toHaveCount(0)
  await expect(page.getByRole('link', { name: destructive })).toHaveCount(0)
}

test('侧边栏与第五张总览卡均可进入 Elasticsearch 节点页', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await login(page)

  const navigation = page.getByRole('navigation', { name: '主导航' })
  await navigation.getByRole('link', { name: 'Elasticsearch' }).click()
  await expect(page).toHaveURL(/\/elasticsearch/)
  await expect(
    page.getByRole('heading', { name: 'Elasticsearch 节点' }),
  ).toBeVisible()

  await navigation.getByRole('link', { name: '总览', exact: true }).click()
  const cards = page.locator('.overview-compact-grid .module-status-card')
  await expect(cards).toHaveCount(5)
  const elasticsearchCard = page.getByRole('link', {
    name: '查看 Elasticsearch 板块',
  })
  await expect(elasticsearchCard).toBeVisible()
  await expect(
    elasticsearchCard.getByText('异常节点', { exact: true }),
  ).toBeVisible()
  await elasticsearchCard.click()

  await expect(page).toHaveURL(/\/elasticsearch/)
  await expect(
    page.getByRole('heading', { name: 'Elasticsearch 节点' }),
  ).toBeVisible()
  await expectNoDestructiveControls(page)
})

test('固定十六列表头并保持搜索、筛选、排序与分页 URL 状态', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await login(page)
  const fixture = await page.evaluate(async () => {
    const response = await fetch(
      '/api/v1/elasticsearch/nodes?sort=node&order=asc&page=1&page_size=20',
    )
    return response.json()
  })
  await page.route('**/api/v1/elasticsearch/nodes?**', async (route) => {
    const url = new URL(route.request().url())
    const requestedPage = Number(url.searchParams.get('page') ?? '1')
    const requestedPageSize = Number(url.searchParams.get('page_size') ?? '20')
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ...fixture,
        data: {
          ...fixture.data,
          total: 45,
          page: requestedPage,
          page_size: requestedPageSize,
          total_pages: Math.ceil(45 / requestedPageSize),
        },
      }),
    })
  })

  await page.goto('/elasticsearch')
  await expect(
    page.getByRole('heading', { name: 'Elasticsearch 节点' }),
  ).toBeVisible()
  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(exactHeaders.length)
  expect(await headers.allTextContents()).toEqual(exactHeaders)

  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page).toHaveURL(/page=2/)
  await page
    .getByRole('searchbox', { name: '搜索节点名称或地址' })
    .fill('fixture-node')
  await expect(page).toHaveURL(/search=fixture-node/)
  await expect(page).toHaveURL(/page=1/)
  await page
    .getByRole('combobox', { name: '所属集群' })
    .selectOption({ index: 1 })
  await expect(page).toHaveURL(/[?&]cluster=[^&]+/)
  await page
    .getByRole('combobox', { name: '节点角色' })
    .selectOption({ index: 1 })
  await expect(page).toHaveURL(/[?&]role=[^&]+/)
  await page
    .getByRole('combobox', { name: '集群健康' })
    .selectOption('green')
  await expect(page).toHaveURL(/cluster_health=green/)
  await page
    .getByRole('combobox', { name: '节点状态' })
    .selectOption('warning')
  await expect(page).toHaveURL(/status=warning/)
  await page.getByRole('button', { name: /^JVM堆使用率排序/ }).click()
  await expect(page).toHaveURL(/sort=heap/)
  await expect(page).toHaveURL(/order=asc/)
  await page.getByRole('button', { name: /^JVM堆使用率排序/ }).click()
  await expect(page).toHaveURL(/sort=heap/)
  await expect(page).toHaveURL(/order=desc/)
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)
  await expectNoDestructiveControls(page)
})

test('1440x900 页面和表格无横向溢出且十六列紧凑单行显示', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await login(page)
  await page.goto('/elasticsearch')
  await expect(page.locator('.elasticsearch-table tbody tr').first()).toBeVisible()

  const geometry = await page.evaluate(() => {
    const scroll = document.querySelector<HTMLElement>(
      '.elasticsearch-table-scroll',
    )!
    const rows = [
      ...document.querySelectorAll<HTMLElement>(
        '.elasticsearch-table tbody tr',
      ),
    ]
    const cells = [
      ...document.querySelectorAll<HTMLElement>(
        '.elasticsearch-table tbody td',
      ),
    ]
    const firstRowCells = [
      ...rows[0].querySelectorAll<HTMLElement>('td'),
    ]
    const headers = [
      ...document.querySelectorAll<HTMLElement>(
        '.elasticsearch-table thead th',
      ),
    ]
    const summarizedRoles = [
      ...document.querySelectorAll<HTMLElement>('.elasticsearch-role'),
    ].filter((role) => role.textContent?.includes('…') === true)
    const healthBadges = [
      ...document.querySelectorAll<HTMLElement>('.elasticsearch-health'),
    ]
    const expectedHealthLevel: Record<string, string> = {
      '绿色': 'normal',
      '黄色': 'warning',
      '红色': 'critical',
      '未知': 'unknown',
    }
    return {
      documentOverflow:
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
      tableOverflow: scroll.scrollWidth > scroll.clientWidth,
      rowHeights: rows.map((row) => row.getBoundingClientRect().height),
      cellsPerRow: rows.map((row) => row.querySelectorAll('td').length),
      wrappedCells: cells.filter(
        (cell) => getComputedStyle(cell).whiteSpace !== 'nowrap',
      ).length,
      ellipsisCells: cells.filter(
        (cell) => getComputedStyle(cell).textOverflow !== 'clip',
      ).length,
      breakCells: firstRowCells.filter((cell) => cell.querySelector('br'))
        .length,
      representativeCells: firstRowCells.map((cell) => ({
        clientWidth: cell.clientWidth,
        scrollWidth: cell.scrollWidth,
      })),
      headersFit: headers.every(
        (header) => header.scrollWidth <= header.clientWidth + 1,
      ),
      summarizedRole: summarizedRoles.some(
        (role) =>
          (role.title?.length ?? 0) > (role.textContent?.length ?? 0),
      ),
      summarizedRolesFit: summarizedRoles.every(
        (role) => role.scrollWidth <= role.clientWidth + 1,
      ),
      healthColorsValid:
        healthBadges.length > 0 &&
        healthBadges.every(
          (badge) =>
            badge.dataset.level ===
            expectedHealthLevel[badge.textContent?.trim() ?? ''],
        ),
      uptimePresent: rows.some(
        (row) => row.querySelectorAll('td')[14]?.textContent?.trim() !== '暂无数据',
      ),
    }
  })

  expect(geometry.documentOverflow).toBe(false)
  expect(geometry.tableOverflow).toBe(false)
  expect(geometry.cellsPerRow.every((count) => count === 16)).toBe(true)
  expect(geometry.wrappedCells).toBe(0)
  expect(geometry.ellipsisCells).toBe(0)
  expect(geometry.breakCells).toBe(0)
  expect(geometry.headersFit).toBe(true)
  expect(geometry.summarizedRole).toBe(true)
  expect(geometry.summarizedRolesFit).toBe(true)
  expect(geometry.healthColorsValid).toBe(true)
  expect(geometry.uptimePresent).toBe(true)
  expect(geometry.representativeCells).toHaveLength(16)
  for (const cell of geometry.representativeCells) {
    expect(cell.scrollWidth).toBeLessThanOrEqual(cell.clientWidth + 1)
  }
  expect(
    Math.max(...geometry.rowHeights) - Math.min(...geometry.rowHeights),
  ).toBeLessThanOrEqual(1)
  await expectNoDestructiveControls(page)
})
