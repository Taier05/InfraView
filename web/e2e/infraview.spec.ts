import { expect, test, type Page } from '@playwright/test'

const username = process.env.INFRAVIEW_E2E_USERNAME ?? 'e2e-admin'
const password =
  process.env.INFRAVIEW_E2E_PASSWORD ?? 'e2e-quote-"-slash-\\-password'

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
  await expect(
    page.getByRole('button', {
      name: /重启|删除|执行|远程命令|切换|修改|发布|配置|扫描|自检|修复|启停|擦除/,
    }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('link', {
      name: /重启|删除|执行|远程命令|切换|修改|发布|配置|扫描|自检|修复|启停|擦除/,
    }),
  ).toHaveCount(0)
}

test('未登录会重定向，登录后可完成总览和主机列表关键路径', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/hosts')
  await expect(page).toHaveURL(/\/login$/)

  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()

  await expect(
    page.getByRole('heading', { name: '基础设施总览' }),
  ).toBeVisible()
  const overviewGrid = page.locator('.overview-compact-grid')
  const overviewCards = overviewGrid.locator('.module-status-card')
  await expect(overviewCards).toHaveCount(5)

  const geometry = await overviewGrid.evaluate((grid) => {
    const cards = Array.from(
      grid.querySelectorAll<HTMLElement>('.module-status-card'),
    )
    const gridBox = grid.getBoundingClientRect()
    const cardBoxes = cards.map((card) => card.getBoundingClientRect())
    return {
      columns: getComputedStyle(grid).gridTemplateColumns.split(/\s+/).length,
      gridWidth: gridBox.width,
      cardWidths: cardBoxes.map((box) => box.width),
      cardTops: cardBoxes.map((box) => box.top),
    }
  })

  expect(geometry.columns).toBe(4)
  expect(new Set(geometry.cardTops.slice(0, 4))).toHaveLength(1)
  expect(geometry.cardTops[4]).toBeGreaterThan(geometry.cardTops[0])
  for (const width of geometry.cardWidths) {
    expect(width).toBeGreaterThan(geometry.gridWidth * 0.2)
    expect(width).toBeLessThan(geometry.gridWidth * 0.27)
  }

  const linuxCard = page.getByRole('link', { name: '查看 Linux 主机板块' })
  await expect(linuxCard.getByText('异常主机')).toBeVisible()
  await expect(
    linuxCard.getByText(/存在严重异常|存在警告|全部正常/),
  ).toBeVisible()
  await expect(page.getByText('CPU', { exact: true })).toBeVisible()
  await expect(page.getByText('内存', { exact: true })).toBeVisible()
  await expect(page.getByText('IO', { exact: true })).toBeVisible()
  await expect(page.getByText('网络', { exact: true })).toBeVisible()
  await expect(page.getByText(/上次刷新 \d{2}:\d{2}:\d{2}/)).toBeVisible()
  await expect(page.getByText(/每 15 秒自动刷新/).first()).toBeVisible()
  const connections = page.getByLabel('数据连接汇总')
  await expect(connections).toBeVisible()
  await expect(connections).toContainText('包含 Mock 数据')
  await connections.getByText('数据连接').click()
  await expect(connections.getByText('指标')).toBeVisible()
  await expect(connections.getByText('Mock', { exact: true })).toBeVisible()
  await expect(connections.getByText('健康', { exact: true })).toBeVisible()
  await expect(page.getByText('CPU 平均使用率')).toHaveCount(0)
  await expect(page.getByText('内存平均使用率')).toHaveCount(0)
  await expect(page.getByRole('heading', { name: '资源使用趋势' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '7天' })).toHaveCount(0)
  await expectNoDestructiveControls(page)

  const refreshResponse = page.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/overview?range=24h') &&
      response.status() === 200,
  )
  await page.getByRole('button', { name: '刷新' }).click()
  await refreshResponse

  await linuxCard.click()
  await expect(
    page.getByRole('heading', { name: '主机', exact: true }),
  ).toBeVisible()
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)
  await expect(
    page.getByRole('combobox', { name: '每页数量' }),
  ).toHaveValue('50')
  await page.getByLabel('搜索主机名或 IP').fill('linux-017')
  await page.getByLabel('主机状态').selectOption('offline')
  await expect(page).toHaveURL(/q=linux-017/)
  await expect(page).toHaveURL(/status=offline/)
  await expect(page).toHaveURL(/page_size=50/)
  await expect(page.getByText('linux-017')).toBeVisible()
  await expect(page.getByRole('row')).toHaveCount(2)
  await expect(page.getByText(/上次刷新 \d{2}:\d{2}:\d{2}/)).toBeVisible()
  await expect(page.getByText(/每 15 秒自动刷新/)).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'IO 忙碌度' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: '网络 出/入' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'CPU 核数' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: '内存容量' })).toBeVisible()
  await expect(
    page.getByRole('button', { name: /^CPU 使用率/ }),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: /^内存使用率/ }),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'linux-017' })).toHaveCount(0)
  await expectNoDestructiveControls(page)

  await page.getByRole('button', { name: /^IO 忙碌度/ }).click()
  await expect(page).toHaveURL(/sort=io/)
  await page.getByRole('button', { name: /^网络 出\/入/ }).click()
  await expect(page).toHaveURL(/sort=network/)

  await page.getByRole('button', { name: '退出登录' }).click()
  await expect(page).toHaveURL(/\/login$/)
  await page.goto('/hosts/mock-host-001')
  await expect(page).toHaveURL(/\/login$/)
})

test('超长主机名限制在自身列内且保留完整提示', async ({ page }) => {
  await login(page)
  const fixture = await page.evaluate(async () => {
    const response = await fetch(
      '/api/v1/hosts?q=&status=&sort=name&order=asc&page=1&page_size=20',
    )
    return response.json()
  })
  const longName =
    'production-payment-service-linux-node-with-an-extremely-long-hostname-001'
  fixture.data.hosts[0].name = longName

  await page.route('**/api/v1/hosts?**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(fixture),
    })
  })
  await page.goto('/hosts')

  const hostName = page.locator('.host-name-text', { hasText: longName })
  await expect(hostName).toHaveAttribute('title', longName)
  const clipping = await hostName.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(clipping.scrollWidth).toBeGreaterThan(clipping.clientWidth)

  const row = hostName.locator('xpath=ancestor::tr')
  const nameCellBox = await row.getByRole('cell').nth(0).boundingBox()
  const ipCellBox = await row.getByRole('cell').nth(1).boundingBox()
  expect(nameCellBox).not.toBeNull()
  expect(ipCellBox).not.toBeNull()
  expect(nameCellBox!.x + nameCellBox!.width).toBeLessThanOrEqual(ipCellBox!.x)
  await expect(row.getByRole('cell').nth(1)).toContainText('192.0.2.1')
})

test('侧边栏和总览硬盘卡都可进入只读硬盘页', async ({ page }) => {
  await login(page)

  const navigation = page.getByRole('navigation', { name: '主导航' })
  await navigation.getByRole('link', { name: '硬盘', exact: true }).click()
  await expect(page).toHaveURL(/\/disks/)
  await expect(page.getByRole('heading', { name: '硬盘设备' })).toBeVisible()

  await navigation.getByRole('link', { name: '总览', exact: true }).click()
  const diskCard = page.getByRole('link', { name: '查看主机硬盘板块' })
  await expect(diskCard).toBeVisible()
  await diskCard.click()
  await expect(page).toHaveURL(/\/disks/)
  await expect(page.getByRole('heading', { name: '硬盘设备' })).toBeVisible()
  await expectNoDestructiveControls(page)
})

test('硬盘页保留十列和 URL 状态且桌面视口无横向溢出', async ({
  page,
}) => {
  await login(page)
  const fixture = await page.evaluate(async () => {
    const response = await fetch(
      '/api/v1/disks/devices?sort=host&order=asc&page=1&page_size=20',
    )
    return response.json()
  })
  await page.route('**/api/v1/disks/devices?**', async (route) => {
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

  await page
    .getByRole('navigation', { name: '主导航' })
    .getByRole('link', { name: '硬盘', exact: true })
    .click()
  await expect(page.getByRole('heading', { name: '硬盘设备' })).toBeVisible()

  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(10)
  for (const heading of [
    '主机',
    '设备',
    '型号',
    '容量',
    'SMART 健康',
    '温度',
    '寿命',
    '通电时间',
    '错误摘要',
    '状态',
  ]) {
    await expect(headers.filter({ hasText: heading })).toHaveCount(1)
  }

  const firstDataRow = page.getByRole('row').nth(1)
  await expect(firstDataRow.locator('.disk-capacity')).toBeVisible()
  await expect(firstDataRow.locator('.disk-model')).toBeVisible()
  const hasFormattedCapacity = await page
    .locator('.disk-capacity')
    .evaluateAll((elements) =>
      elements.some((element) =>
        /^\d+(?:\.\d)? (?:B|KiB|MiB|GiB|TiB|PiB)$/.test(
          element.textContent?.trim() ?? '',
        ),
      ),
    )
  expect(hasFormattedCapacity).toBe(true)

  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page).toHaveURL(/page=2/)
  await page
    .getByRole('searchbox', { name: '搜索主机、设备或型号' })
    .fill('fixture')
  await expect(page).toHaveURL(/search=fixture/)
  await expect(page).toHaveURL(/page=1/)
  await page.getByRole('combobox', { name: '设备状态' }).selectOption('warning')
  await expect(page).toHaveURL(/status=warning/)
  await page.getByRole('button', { name: '容量' }).click()
  await expect(page).toHaveURL(/sort=capacity/)
  await expect(page).toHaveURL(/order=asc/)
  await page.getByRole('button', { name: '容量' }).click()
  await expect(page).toHaveURL(/sort=capacity/)
  await expect(page).toHaveURL(/order=desc/)
  await page.getByRole('button', { name: '温度' }).click()
  await expect(page).toHaveURL(/sort=temperature/)
  await expect(page).toHaveURL(/order=asc/)
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)

  await expectNoDestructiveControls(page)
  const viewport = await page.locator('html').evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth)
  const tableRegion = await page.locator('.disk-table-scroll').evaluate(
    (element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }),
  )
  expect(tableRegion.scrollWidth).toBeLessThanOrEqual(tableRegion.clientWidth)
})

test('可控旧数据响应会显示过期提示', async ({ page }) => {
  await login(page)
  const fresh = await page.evaluate(async () => {
    const response = await fetch('/api/v1/overview?range=24h')
    return response.json()
  })

  await page.route('**/api/v1/overview?range=24h', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ...fresh,
        meta: {
          ...fresh.meta,
          stale: true,
          collected_at: '2026-07-22T00:00:00Z',
        },
      }),
    })
  })

  await page.reload()
  await expect(page.getByRole('alert')).toContainText('数据已过期')
  await expect(page.getByText('2026-07-22T00:00:00Z')).toBeVisible()
  await expect(
    page.getByRole('link', { name: '查看 Linux 主机板块' }),
  ).toBeVisible()
})

test('可控错误响应会显示只读重试入口', async ({ page }) => {
  await login(page)
  await page.route('**/api/v1/overview?range=24h', async (route) => {
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 'datasource_unavailable',
        message: '测试数据源暂时不可用',
        request_id: 'e2e-controlled-error',
        retryable: true,
      }),
    })
  })

  await page.reload()
  await expect(page.getByRole('alert')).toContainText('无法加载总览数据')
  await expect(page.getByRole('alert')).toContainText('测试数据源暂时不可用')
  await expect(page.getByRole('button', { name: '重试' })).toBeVisible()
  await expectNoDestructiveControls(page)
})

test('shows the read-only MySQL overview and compact instance list', async ({
  page,
}) => {
  await login(page)
  const mysqlCard = page.getByRole('link', { name: '查看 MySQL 板块' })
  await expect(mysqlCard).toBeVisible()
  await expect(mysqlCard.getByText('复制线程')).toBeVisible()
  await mysqlCard.click()

  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()
  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(10)
  for (const heading of [
    '实例地址',
    '版本 / 角色',
    '连接',
    '线程',
    'QPS / TPS',
    '慢查询',
    'Buffer Pool',
    '复制 / 延迟',
    '运行时间',
    '状态',
  ]) {
    await expect(headers.filter({ hasText: heading })).toHaveCount(1)
  }

  const stoppedReplication = page
    .getByRole('row')
    .filter({ hasText: '线程异常' })
  await expect(stoppedReplication).toHaveCount(1)
  await expect(
    stoppedReplication.getByText('严重', { exact: true }),
  ).toBeVisible()
  await expectNoDestructiveControls(page)
  await expect(page.locator('body')).not.toHaveCSS('overflow-x', 'scroll')
  const viewport = await page.locator('html').evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth)
})

test('shows the read-only Redis overview and instance list', async ({ page }) => {
  await login(page)
  const redisCard = page.getByRole('link', { name: '查看 Redis 板块' })
  await expect(redisCard).toBeVisible()
  await expect(redisCard.getByText('复制', { exact: true })).toBeVisible()
  await redisCard.click()

  await expect(page.getByRole('heading', { name: 'Redis 实例' })).toBeVisible()
  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(11)
  for (const heading of [
    '实例地址',
    '角色',
    '内存上限',
    '内存使用率',
    '连接',
    'QPS/命中率',
    'key总数',
    '复制链路',
    '延迟',
    '运行时间',
    '状态',
  ]) {
    await expect(headers.filter({ hasText: heading })).toHaveCount(1)
  }
  await expect(headers.filter({ hasText: '过期/淘汰key数' })).toHaveCount(0)

  await expect(page.getByRole('combobox', { name: 'Redis 角色' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: '实例状态' })).toBeVisible()
  await expect(
    page.getByRole('button', { name: '刷新 Redis 实例列表' }),
  ).toBeVisible()
  await expect(page.getByRole('navigation').getByText('Redis')).toBeVisible()
  await expectNoDestructiveControls(page)
  await expect(page.locator('body')).not.toHaveCSS('overflow-x', 'scroll')
})

test('刷新后从 URL 恢复 MySQL 筛选、排序和每页数量', async ({ page }) => {
  await login(page)
  await page.getByRole('link', { name: '查看 MySQL 板块' }).click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()

  await page
    .getByRole('combobox', { name: '读写属性' })
    .selectOption('read_only')
  await expect(page).toHaveURL(/role=read_only/)
  await page.getByRole('combobox', { name: '实例状态' }).selectOption('critical')
  await expect(page).toHaveURL(/status=critical/)
  await page
    .getByRole('button', { name: /^复制状态 \/ 延迟排序/ })
    .click()
  await expect(page).toHaveURL(/sort=replication_lag/)
  await expect(page).toHaveURL(/order=asc/)
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)

  await page.reload()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()
  await expect(page).toHaveURL(/role=read_only/)
  await expect(page).toHaveURL(/status=critical/)
  await expect(page).toHaveURL(/sort=replication_lag/)
  await expect(page).toHaveURL(/order=asc/)
  await expect(page).toHaveURL(/page_size=50/)
  await expect(
    page.getByRole('combobox', { name: '读写属性' }),
  ).toHaveValue('read_only')
  await expect(
    page.getByRole('combobox', { name: '实例状态' }),
  ).toHaveValue('critical')
  await expect(
    page.getByRole('combobox', { name: '每页数量' }),
  ).toHaveValue('50')
  await expect(
    page.getByRole('button', { name: /^复制状态 \/ 延迟排序，当前升序$/ }),
  ).toBeVisible()
})
