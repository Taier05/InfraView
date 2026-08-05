import { expect, test, type Page } from '@playwright/test'

const username = process.env.INFRAVIEW_E2E_USERNAME ?? 'e2e-admin'
const password =
  process.env.INFRAVIEW_E2E_PASSWORD ?? 'e2e-quote-"-slash-\\-password'

const exactHeaders = [
  '节点名称',
  '所属集群',
  '实例地址',
  '版本',
  '内存使用率',
  '磁盘余量',
  '文件描述符使用率',
  'Erlang进程使用率',
  '连接',
  '队列',
  '消息积压',
  '发布速率',
  '投递速率',
  '运行时间',
  '状态',
] as const

const overviewFixture = {
  data: {
    status: 'critical',
    clusters: { total: 2, normal: 1, warning: 1, critical: 0, unknown: 0 },
    nodes: { total: 4, normal: 1, warning: 1, critical: 1, unknown: 1 },
    alerts: {
      cluster_connectivity: { warning: 1, critical: 0, unknown: 0 },
      resource_alarms: { warning: 0, critical: 0, unknown: 0 },
      resource_pressure: { warning: 1, critical: 0, unknown: 0 },
      collection: { warning: 0, critical: 0, unknown: 0 },
    },
  },
  meta: {
    request_id: 'synthetic-rabbitmq-overview',
    stale: false,
    collected_at: '2026-08-04T08:00:00.000Z',
  },
}

const syntheticNodes = [
  {
    id: 'synthetic-rabbitmq-node-a',
    name: 'rabbit-a',
    cluster: 'cluster-a',
    address: 'rabbit-a.fixture.invalid:15692',
    version: '4.0',
    memory_usage_percent: 48.5,
    disk_available_bytes: 8_589_934_592,
    file_descriptor_usage_percent: 24,
    erlang_process_usage_percent: 31.5,
    connections: 16,
    queues: 8,
    messages: 42,
    publish_rate: 12.5,
    deliver_rate: 11.75,
    uptime_seconds: 90_000,
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-rabbitmq-node-b',
    name: 'rabbit-b',
    cluster: 'cluster-b',
    address: 'rabbit-b.fixture.invalid:15692',
    version: '4.0',
    memory_usage_percent: 84,
    disk_available_bytes: 4_294_967_296,
    file_descriptor_usage_percent: 44,
    erlang_process_usage_percent: 39,
    connections: 23,
    queues: 11,
    messages: 64,
    publish_rate: 9.25,
    deliver_rate: 9,
    uptime_seconds: 172_800,
    status: 'warning',
    status_source: 'memory',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-rabbitmq-node-c',
    name: 'rabbit-c',
    cluster: 'cluster-c',
    address: 'rabbit-c.fixture.invalid:15692',
    version: '4.0',
    memory_usage_percent: 93,
    disk_available_bytes: 2_147_483_648,
    file_descriptor_usage_percent: 91,
    erlang_process_usage_percent: 88,
    connections: 31,
    queues: 17,
    messages: 128,
    publish_rate: 7.5,
    deliver_rate: 6.75,
    uptime_seconds: 259_200,
    status: 'critical',
    status_source: 'alarm',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-rabbitmq-node-d',
    name: 'rabbit-d',
    cluster: 'cluster-d',
    address: 'rabbit-d.fixture.invalid:15692',
    version: '4.0',
    memory_usage_percent: null,
    disk_available_bytes: null,
    file_descriptor_usage_percent: null,
    erlang_process_usage_percent: null,
    connections: null,
    queues: null,
    messages: null,
    publish_rate: null,
    deliver_rate: null,
    uptime_seconds: null,
    status: 'unknown',
    status_source: 'unknown',
    collection_level: 'unknown',
  },
] as const

function nodePageFixture(
  parameters: URLSearchParams,
  nodes: readonly Record<string, unknown>[] = syntheticNodes,
  availableClusters = ['cluster-a', 'cluster-b', 'cluster-c', 'cluster-d'],
) {
  const page = Number(parameters.get('page') ?? '1')
  const pageSize = Number(parameters.get('page_size') ?? '20')
  const total = Math.max(nodes.length, 45)
  return {
    data: {
      nodes,
      available_clusters: availableClusters,
      total,
      page,
      page_size: pageSize,
      total_pages: Math.ceil(total / pageSize),
    },
    meta: {
      request_id: 'synthetic-rabbitmq-nodes',
      stale: false,
      collected_at: '2026-08-04T08:00:00.000Z',
    },
  }
}

async function mockRabbitMQAPI(
  page: Page,
  options: {
    nodes?: readonly Record<string, unknown>[]
    availableClusters?: string[]
  } = {},
) {
  await page.route('**/api/v1/rabbitmq/overview', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(overviewFixture),
    })
  })
  await page.route('**/api/v1/rabbitmq/nodes?**', async (route) => {
    const parameters = new URL(route.request().url()).searchParams
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        nodePageFixture(
          parameters,
          options.nodes,
          options.availableClusters,
        ),
      ),
    })
  })
}

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
    /重启|删除|执行|远程命令|切换|修改|发布|配置|下发|迁移|故障转移|清理|关闭|打开/
  await expect(page.getByRole('button', { name: destructive })).toHaveCount(0)
  await expect(page.getByRole('link', { name: destructive })).toHaveCount(0)
}

test('侧边栏与第六张总览卡均进入只读 RabbitMQ 节点页', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockRabbitMQAPI(page)
  await login(page)

  const navigation = page.getByRole('navigation', { name: '主导航' })
  await navigation.getByRole('link', { name: 'RabbitMQ', exact: true }).click()
  await expect(page).toHaveURL(/\/rabbitmq/)
  await expect(page.getByRole('heading', { name: 'RabbitMQ 节点' })).toBeVisible()

  await navigation.getByRole('link', { name: '总览', exact: true }).click()
  const cards = page.locator('.overview-compact-grid .module-status-card')
  await expect(cards).toHaveCount(6)
  const rabbitMQCard = page.getByRole('link', { name: '查看 RabbitMQ 板块' })
  await expect(rabbitMQCard).toBeVisible()
  await expect(rabbitMQCard.getByText('异常节点', { exact: true })).toBeVisible()
  await rabbitMQCard.click()

  await expect(page).toHaveURL(/\/rabbitmq/)
  await expect(page.getByRole('heading', { name: 'RabbitMQ 节点' })).toBeVisible()
  await expectNoDestructiveControls(page)

  for (const endpoint of [
    '/api/v1/rabbitmq/overview',
    '/api/v1/rabbitmq/nodes',
  ]) {
    const response = await page.request.post(endpoint, { data: {} })
    expect(response.status()).toBe(405)
  }
})

test('固定十五列表头并恢复搜索、集群、状态、排序与分页 URL', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockRabbitMQAPI(page)
  await login(page)
  await page.goto('/rabbitmq')

  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(exactHeaders.length)
  expect(await headers.allTextContents()).toEqual(exactHeaders)

  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page).toHaveURL(/page=2/)
  await page
    .getByRole('searchbox', { name: '搜索节点名称或地址' })
    .fill('rabbit-a')
  await expect(page).toHaveURL(/search=rabbit-a/)
  await expect(page).toHaveURL(/page=1/)
  await page.getByRole('combobox', { name: '所属集群' }).selectOption('cluster-b')
  await expect(page).toHaveURL(/cluster=cluster-b/)
  await page.getByRole('combobox', { name: '节点状态' }).selectOption('warning')
  await expect(page).toHaveURL(/status=warning/)
  await page.getByRole('button', { name: /^内存使用率排序/ }).click()
  await expect(page).toHaveURL(/sort=memory/)
  await expect(page).toHaveURL(/direction=asc/)
  await page.getByRole('button', { name: /^内存使用率排序/ }).click()
  await expect(page).toHaveURL(/direction=desc/)
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)

  await page.reload()
  await expect(page.getByRole('searchbox', { name: '搜索节点名称或地址' })).toHaveValue('rabbit-a')
  await expect(page.getByRole('combobox', { name: '所属集群' })).toHaveValue('cluster-b')
  await expect(page.getByRole('combobox', { name: '节点状态' })).toHaveValue('warning')
  await expect(page.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')
  await expectNoDestructiveControls(page)
})

test('1440x900 十五列紧凑等高单行且身份省略保留完整提示', async ({
  page,
}) => {
  const longName =
    'synthetic-rabbitmq-node-name-that-is-intentionally-long-for-ellipsis'
  const longCluster =
    'synthetic-rabbitmq-cluster-name-that-is-intentionally-long-for-ellipsis'
  const longAddress =
    'synthetic-rabbitmq-node-address-that-is-intentionally-long.fixture.invalid:15692'
  const nodes = [
    {
      ...syntheticNodes[0],
      name: longName,
      cluster: longCluster,
      address: longAddress,
    },
    syntheticNodes[1],
    syntheticNodes[2],
    syntheticNodes[3],
  ]
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockRabbitMQAPI(page, {
    nodes,
    availableClusters: [longCluster, 'cluster-b', 'cluster-c', 'cluster-d'],
  })
  await login(page)
  await page.goto('/rabbitmq')
  await expect(page.locator('.rabbitmq-table tbody tr').first()).toBeVisible()

  const geometry = await page.evaluate(() => {
    const scroll = document.querySelector<HTMLElement>('.rabbitmq-table-scroll')!
    const rows = [...document.querySelectorAll<HTMLElement>('.rabbitmq-table tbody tr')]
    const cells = [...document.querySelectorAll<HTMLElement>('.rabbitmq-table tbody td')]
    const firstRowCells = [...rows[0].querySelectorAll<HTMLElement>('td')]
    const identities = [...rows[0].querySelectorAll<HTMLElement>('.rabbitmq-identity')]
    const shortRowValues = [...rows[1].querySelectorAll<HTMLElement>('.rabbitmq-value')]
    const statusBadges = [
      ...document.querySelectorAll<HTMLElement>(
        '.rabbitmq-table .status-badge',
      ),
    ]
    return {
      documentOverflow:
        document.documentElement.scrollWidth > document.documentElement.clientWidth,
      tableOverflow: scroll.scrollWidth > scroll.clientWidth,
      rowHeights: rows.map((row) => row.getBoundingClientRect().height),
      cellsPerRow: rows.map((row) => row.querySelectorAll('td').length),
      wrappedCells: cells.filter(
        (cell) => getComputedStyle(cell).whiteSpace !== 'nowrap',
      ).length,
      breakCells: cells.filter((cell) => cell.querySelector('br')).length,
      clippedIdentities: identities.filter(
        (identity) =>
          identity.scrollWidth > identity.clientWidth &&
          identity.title === identity.textContent?.trim(),
      ).length,
      shortValuesFit: shortRowValues.every(
        (value) => value.scrollWidth <= value.clientWidth + 1,
      ),
      firstRowCellsFit: firstRowCells.every(
        (cell) => cell.scrollWidth <= cell.clientWidth + 1,
      ),
      statusLevels: statusBadges.map((badge) => badge.dataset.level),
      statusStyles: statusBadges.map((badge) => {
        const style = getComputedStyle(badge)
        return {
          color: style.color,
          backgroundColor: style.backgroundColor,
        }
      }),
    }
  })

  expect(geometry.documentOverflow).toBe(false)
  expect(geometry.tableOverflow).toBe(false)
  expect(geometry.cellsPerRow.every((count) => count === 15)).toBe(true)
  expect(geometry.wrappedCells).toBe(0)
  expect(geometry.breakCells).toBe(0)
  expect(geometry.clippedIdentities).toBe(3)
  expect(geometry.shortValuesFit).toBe(true)
  expect(geometry.firstRowCellsFit).toBe(true)
  expect(geometry.statusLevels).toEqual([
    'normal',
    'warning',
    'critical',
    'unknown',
  ])
  expect(
    new Set(
      geometry.statusStyles.map(
        ({ color, backgroundColor }) => `${color}|${backgroundColor}`,
      ),
    ),
  ).toHaveSize(4)
  expect(
    Math.max(...geometry.rowHeights) - Math.min(...geometry.rowHeights),
  ).toBeLessThanOrEqual(1)
  await expectNoDestructiveControls(page)
})

test('1100px 附近超长集群不撑破控制栏且仅表格区域可横向滚动', async ({
  page,
}) => {
  const longCluster =
    'synthetic-rabbitmq-cluster-with-a-deliberately-long-name-for-medium-width-layout'
  await page.setViewportSize({ width: 1100, height: 900 })
  await mockRabbitMQAPI(page, {
    nodes: [{ ...syntheticNodes[0], cluster: longCluster }],
    availableClusters: [longCluster],
  })
  await login(page)
  await page.goto(`/rabbitmq?cluster=${encodeURIComponent(longCluster)}`)
  await expect(page.getByRole('combobox', { name: '所属集群' })).toHaveValue(longCluster)

  const geometry = await page.evaluate(() => {
    const controls = document
      .querySelector<HTMLElement>('.rabbitmq-table')!
      .closest('section')!
      .querySelector<HTMLElement>('.host-list-controls')!
    const scroll = document.querySelector<HTMLElement>('.rabbitmq-table-scroll')!
    const controlBox = controls.getBoundingClientRect()
    return {
      documentOverflow:
        document.documentElement.scrollWidth > document.documentElement.clientWidth,
      controlsOverflow: controls.scrollWidth > controls.clientWidth,
      controlsOutsideViewport:
        controlBox.left < 0 || controlBox.right > document.documentElement.clientWidth,
      tableIsOnlyOverflowRegion: scroll.scrollWidth > scroll.clientWidth,
    }
  })

  expect(geometry.documentOverflow).toBe(false)
  expect(geometry.controlsOverflow).toBe(false)
  expect(geometry.controlsOutsideViewport).toBe(false)
  expect(geometry.tableIsOnlyOverflowRegion).toBe(true)
  await expectNoDestructiveControls(page)
})
