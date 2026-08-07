import { expect, test, type Page } from '@playwright/test'

const username = process.env.INFRAVIEW_E2E_USERNAME ?? 'e2e-admin'
const password =
  process.env.INFRAVIEW_E2E_PASSWORD ?? 'e2e-quote-"-slash-\\-password'

const exactHeaders = [
  '业务端',
  '服务地址',
  '健康检查',
  '健康延迟',
  '端口状态',
  '进程状态',
  '进程数',
  '端口进程一致性',
  'CPU 使用率',
  '内存占用',
  '内存使用率',
  '运行时间',
  '状态',
] as const

const javaServiceRequestKeys = [
  'search',
  'name',
  'status',
  'sort',
  'direction',
  'page',
  'page_size',
] as const

type JavaServiceRequest = Record<
  (typeof javaServiceRequestKeys)[number],
  string | null
>

const businessMappings = [
  { name: 'tikbee', business: '用户端' },
  { name: 'rider', business: '骑手端' },
  { name: 'mch', business: '商家端' },
  { name: 'saas', business: '管理后台端' },
  { name: 'mch_saas', business: '商家 PC 端' },
  {
    name: 'future_synthetic_service_code',
    business: 'future_synthetic_service_code',
  },
] as const

const overviewFixture = {
  data: {
    status: 'warning',
    services: { total: 6, normal: 4, warning: 1, critical: 0, unknown: 1 },
    alerts: {
      health: { warning: 0, critical: 0, unknown: 0 },
      port: { warning: 0, critical: 0, unknown: 0 },
      process: { warning: 0, critical: 0, unknown: 0 },
      collection: { warning: 1, critical: 0, unknown: 1 },
    },
  },
  meta: {
    request_id: 'synthetic-java-overview',
    stale: false,
    collected_at: '2026-08-05T08:00:00.000Z',
  },
}

const syntheticServices = [
  {
    id: 'synthetic-java-service-a',
    ...businessMappings[0],
    address: 'synthetic-java-address-a.fixture.invalid:18080',
    health_up: true,
    health_latency_ms: 12.5,
    port_up: true,
    process_up: true,
    process_count: '3',
    port_consistent: true,
    cpu_usage_percent: 17.5,
    memory_bytes: '1073741824',
    memory_usage_percent: 25,
    uptime_seconds: '90000',
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-java-service-b',
    ...businessMappings[1],
    address: 'synthetic-java-address-b.fixture.invalid:18080',
    health_up: true,
    health_latency_ms: 18,
    port_up: true,
    process_up: true,
    process_count: '2',
    port_consistent: true,
    cpu_usage_percent: 21,
    memory_bytes: '536870912',
    memory_usage_percent: 20,
    uptime_seconds: '86400',
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-java-service-c',
    ...businessMappings[2],
    address: 'synthetic-java-address-c.fixture.invalid:18080',
    health_up: true,
    health_latency_ms: 8,
    port_up: true,
    process_up: true,
    process_count: '4',
    port_consistent: true,
    cpu_usage_percent: 14,
    memory_bytes: '268435456',
    memory_usage_percent: 18,
    uptime_seconds: '172800',
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-java-service-d',
    ...businessMappings[3],
    address: 'synthetic-java-address-d.fixture.invalid:18080',
    health_up: true,
    health_latency_ms: 30,
    port_up: true,
    process_up: true,
    process_count: '1',
    port_consistent: true,
    cpu_usage_percent: 10,
    memory_bytes: '134217728',
    memory_usage_percent: 12,
    uptime_seconds: '43200',
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-java-service-e',
    ...businessMappings[4],
    address: 'synthetic-java-address-e.fixture.invalid:18080',
    health_up: true,
    health_latency_ms: 25,
    port_up: true,
    process_up: true,
    process_count: '2',
    port_consistent: true,
    cpu_usage_percent: 16,
    memory_bytes: '805306368',
    memory_usage_percent: 22,
    uptime_seconds: '259200',
    status: 'normal',
    status_source: 'normal',
    collection_level: 'normal',
  },
  {
    id: 'synthetic-java-service-f',
    ...businessMappings[5],
    address:
      'synthetic-java-address-with-a-deliberately-long-value.fixture.invalid:18080',
    health_up: null,
    health_latency_ms: null,
    port_up: null,
    process_up: null,
    process_count: null,
    port_consistent: null,
    cpu_usage_percent: null,
    memory_bytes: null,
    memory_usage_percent: null,
    uptime_seconds: null,
    status: 'unknown',
    status_source: 'unknown',
    collection_level: 'unknown',
  },
] as const

function servicePageFixture(
  parameters: URLSearchParams,
  services: readonly Record<string, unknown>[] = syntheticServices,
  stale = false,
) {
  const requestedName = parameters.get('name')
  const matchingServices = requestedName === null
    ? services
    : services.filter((service) => service.name === requestedName)
  const page = Number(parameters.get('page') ?? '1')
  const pageSize = Number(parameters.get('page_size') ?? '20')
  const total = matchingServices.length === 0 ? 0 : Math.max(matchingServices.length, 45)
  return {
    data: {
      services: matchingServices,
      available_names: syntheticServices.map((service) => service.name),
      total,
      page,
      page_size: pageSize,
      total_pages: total === 0 ? 0 : Math.ceil(total / pageSize),
    },
    meta: {
      request_id: 'synthetic-java-services',
      stale,
      collected_at: '2026-08-05T08:00:00.000Z',
    },
  }
}

function captureServiceRequest(url: string): JavaServiceRequest {
  const parameters = new URL(url).searchParams
  const keys = [...parameters.keys()]
  expect(keys).toEqual([...new Set(keys)])
  expect(keys.every((key) => javaServiceRequestKeys.includes(
    key as (typeof javaServiceRequestKeys)[number],
  ))).toBe(true)
  return {
    search: parameters.get('search'),
    name: parameters.get('name'),
    status: parameters.get('status'),
    sort: parameters.get('sort'),
    direction: parameters.get('direction'),
    page: parameters.get('page'),
    page_size: parameters.get('page_size'),
  }
}

async function expectLatestServiceRequest(
  capturedServiceRequests: readonly JavaServiceRequest[],
  expected: JavaServiceRequest,
) {
  await expect.poll(() => capturedServiceRequests.at(-1)).toEqual(expected)
}

async function mockJavaAPI(
  page: Page,
  options: {
    services?: readonly Record<string, unknown>[]
    stale?: boolean
    unavailable?: boolean
    capturedServiceRequests?: JavaServiceRequest[]
  } = {},
) {
  await page.route('**/api/v1/java/overview', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(overviewFixture),
    })
  })
  await page.route('**/api/v1/java/services?**', async (route) => {
    if (options.unavailable) {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'java_unavailable',
            message: '数据源暂时不可用，请稍后重试',
            retryable: true,
          },
          meta: { request_id: 'synthetic-java-unavailable' },
        }),
      })
      return
    }
    const parameters = new URL(route.request().url()).searchParams
    options.capturedServiceRequests?.push(captureServiceRequest(route.request().url()))
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        servicePageFixture(parameters, options.services, options.stale),
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

test('侧边栏与第七张总览卡均进入只读 Java 业务服务页', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockJavaAPI(page)
  await login(page)

  const navigation = page.getByRole('navigation', { name: '主导航' })
  await navigation.getByRole('link', { name: 'Java 服务', exact: true }).click()
  await expect(page).toHaveURL(/\/java/)
  await expect(page.getByRole('heading', { name: 'Java 业务服务' })).toBeVisible()

  await navigation.getByRole('link', { name: '总览', exact: true }).click()
  const cards = page.locator('.overview-compact-grid .module-status-card')
  await expect(cards).toHaveCount(7)
  const javaCard = page.getByRole('link', { name: '查看 Java 服务板块' })
  await expect(javaCard).toBeVisible()
  await expect(javaCard.getByText('异常服务', { exact: true })).toBeVisible()
  await javaCard.click()

  await expect(page).toHaveURL(/\/java/)
  await expect(page.getByRole('heading', { name: 'Java 业务服务' })).toBeVisible()
  await expectNoDestructiveControls(page)

  for (const endpoint of [
    '/api/v1/java/overview',
    '/api/v1/java/services',
  ]) {
    const response = await page.request.post(endpoint, { data: {} })
    expect(response.status()).toBe(405)
  }
})

test('固定十三列表头，并以请求契约恢复和规范 URL 状态', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  const capturedServiceRequests: JavaServiceRequest[] = []
  await mockJavaAPI(page, { capturedServiceRequests })
  await login(page)
  await page.goto(
    '/java?search=tikbee&name=rider&status=warning&sort=cpu&direction=desc&page=2&page_size=50',
  )

  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(exactHeaders.length)
  expect(await headers.allTextContents()).toEqual(exactHeaders)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: 'warning',
    sort: 'cpu',
    direction: 'desc',
    page: '2',
    page_size: '50',
  })
  await expect(
    page.getByRole('searchbox', { name: '搜索业务端、服务名称或地址' }),
  ).toHaveValue('tikbee')
  await expect(page.getByRole('combobox', { name: '业务端' })).toHaveValue('rider')
  await expect(page.getByRole('combobox', { name: '服务状态' })).toHaveValue('warning')
  await expect(page.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')

  await page.goto(
    '/java?search=%20%20&name=%20%20&status=invalid&sort=invalid&direction=invalid&page=0&page_size=30',
  )
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: null,
    name: null,
    status: null,
    sort: 'business',
    direction: 'asc',
    page: '1',
    page_size: '20',
  })

  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page).toHaveURL(/page=2/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: null,
    name: null,
    status: null,
    sort: 'business',
    direction: 'asc',
    page: '2',
    page_size: '20',
  })
  await page
    .getByRole('searchbox', { name: '搜索业务端、服务名称或地址' })
    .fill('tikbee')
  await expect(page).toHaveURL(/search=tikbee/)
  await expect(page).toHaveURL(/page=1/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: null,
    status: null,
    sort: 'business',
    direction: 'asc',
    page: '1',
    page_size: '20',
  })
  await page.getByRole('combobox', { name: '业务端' }).selectOption('rider')
  await expect(page).toHaveURL(/name=rider/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: null,
    sort: 'business',
    direction: 'asc',
    page: '1',
    page_size: '20',
  })
  await page.getByRole('combobox', { name: '服务状态' }).selectOption('warning')
  await expect(page).toHaveURL(/status=warning/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: 'warning',
    sort: 'business',
    direction: 'asc',
    page: '1',
    page_size: '20',
  })
  await page.getByRole('button', { name: /^CPU 使用率排序/ }).click()
  await expect(page).toHaveURL(/sort=cpu/)
  await expect(page).toHaveURL(/direction=asc/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: 'warning',
    sort: 'cpu',
    direction: 'asc',
    page: '1',
    page_size: '20',
  })
  await page.getByRole('button', { name: /^CPU 使用率排序/ }).click()
  await expect(page).toHaveURL(/direction=desc/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: 'warning',
    sort: 'cpu',
    direction: 'desc',
    page: '1',
    page_size: '20',
  })
  await page.getByRole('combobox', { name: '每页数量' }).selectOption('50')
  await expect(page).toHaveURL(/page_size=50/)
  await expectLatestServiceRequest(capturedServiceRequests, {
    search: 'tikbee',
    name: 'rider',
    status: 'warning',
    sort: 'cpu',
    direction: 'desc',
    page: '1',
    page_size: '50',
  })

  await page.reload()
  await expect(
    page.getByRole('searchbox', { name: '搜索业务端、服务名称或地址' }),
  ).toHaveValue('tikbee')
  await expect(page.getByRole('combobox', { name: '业务端' })).toHaveValue('rider')
  await expect(page.getByRole('combobox', { name: '服务状态' })).toHaveValue('warning')
  await expect(page.getByRole('combobox', { name: '每页数量' })).toHaveValue('50')
  await expectNoDestructiveControls(page)
})

test('route fixture 的 raw name 与业务端映射逐对保持稳定', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  const capturedServiceRequests: JavaServiceRequest[] = []
  await mockJavaAPI(page, { capturedServiceRequests })
  await login(page)
  for (const [index, mapping] of businessMappings.entries()) {
    expect(syntheticServices[index]).toMatchObject(mapping)
    await page.goto(`/java?name=${encodeURIComponent(mapping.name)}`)
    await expectLatestServiceRequest(capturedServiceRequests, {
      search: null,
      name: mapping.name,
      status: null,
      sort: 'business',
      direction: 'asc',
      page: '1',
      page_size: '20',
    })
    const row = page.locator('.java-table tbody tr').first()
    await expect(row).toBeVisible()
    await expect(row.locator('td').first()).toHaveText(mapping.business)
    await expect(row.locator('.java-identity').first()).toHaveAttribute(
      'title',
      mapping.business,
    )
  }

  const unknownRow = page.locator('.java-table tbody tr').first()
  const unknownValues = unknownRow.locator('.java-value')
  await expect(unknownValues).toHaveCount(10)
  await expect(unknownValues).toHaveText(
    Array.from({ length: 10 }, () => '暂无数据'),
  )
  await expect(unknownRow.locator('.java-identity').first()).toHaveAttribute(
    'title',
    'future_synthetic_service_code',
  )
  await expect(unknownRow.locator('.java-identity').nth(1)).toHaveAttribute(
    'title',
    'synthetic-java-address-with-a-deliberately-long-value.fixture.invalid:18080',
  )
  await expectNoDestructiveControls(page)
})

test('十三列每行单值单行，1440x900 无页面或表格横向溢出', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockJavaAPI(page)
  await login(page)
  await page.goto('/java')
  await expect(page.locator('.java-table tbody tr').first()).toBeVisible()

  const geometry = await page.evaluate(() => {
    const scroll = document.querySelector<HTMLElement>('.java-table-scroll')!
    const rows = [...document.querySelectorAll<HTMLElement>('.java-table tbody tr')]
    const cells = [...document.querySelectorAll<HTMLElement>('.java-table tbody td')]
    return {
      documentOverflow:
        document.documentElement.scrollWidth > document.documentElement.clientWidth,
      tableOverflow: scroll.scrollWidth > scroll.clientWidth,
      cellsPerRow: rows.map((row) => row.querySelectorAll('td').length),
      wrappedCells: cells.filter(
        (cell) => getComputedStyle(cell).whiteSpace !== 'nowrap',
      ).length,
      breakCells: cells.filter((cell) => cell.querySelector('br')).length,
    }
  })

  expect(geometry.documentOverflow).toBe(false)
  expect(geometry.tableOverflow).toBe(false)
  expect(geometry.cellsPerRow.every((count) => count === 13)).toBe(true)
  expect(geometry.wrappedCells).toBe(0)
  expect(geometry.breakCells).toBe(0)
  await expectNoDestructiveControls(page)
})

test('窄视口仅 Java 表格滚动容器允许横向溢出', async ({ page }) => {
  await page.setViewportSize({ width: 1000, height: 900 })
  await mockJavaAPI(page)
  await login(page)
  await page.goto('/java')

  const geometry = await page.evaluate(() => {
    const scroll = document.querySelector<HTMLElement>('.java-table-scroll')!
    const visibleHorizontalScrollContainers = [document.body, ...document.querySelectorAll<HTMLElement>('body *')]
      .filter((element) => {
        const box = element.getBoundingClientRect()
        const style = getComputedStyle(element)
        return box.width > 0 && box.height > 0 && ['auto', 'scroll'].includes(style.overflowX)
      })
      .map((element) => ({
        selector: element === scroll ? '.java-table-scroll' : element.tagName.toLowerCase(),
        scrollWidth: element.scrollWidth,
        clientWidth: element.clientWidth,
      }))
    return {
      documentHasHorizontalOverflow:
        document.documentElement.scrollWidth > document.documentElement.clientWidth,
      visibleHorizontalScrollContainers,
      overflowingHorizontalScrollContainers: visibleHorizontalScrollContainers
        .filter((container) => container.scrollWidth > container.clientWidth)
        .map((container) => container.selector),
    }
  })

  expect(geometry.documentHasHorizontalOverflow).toBe(false)
  expect(geometry.visibleHorizontalScrollContainers).toContainEqual(
    expect.objectContaining({ selector: '.java-table-scroll' }),
  )
  expect(geometry.overflowingHorizontalScrollContainers).toEqual([
    '.java-table-scroll',
  ])
  await expectNoDestructiveControls(page)
})

test('stale、错误与空集合使用独立只读状态', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockJavaAPI(page, { stale: true })
  await login(page)
  await page.goto('/java')
  await expect(page.getByRole('alert')).toContainText('数据已过期')

  await page.unroute('**/api/v1/java/services?**')
  await mockJavaAPI(page, { unavailable: true })
  await page.reload()
  await expect(
    page.getByRole('heading', { name: 'Java 业务服务列表加载失败' }),
  ).toBeVisible()

  await page.unroute('**/api/v1/java/services?**')
  await mockJavaAPI(page, { services: [] })
  await page.reload()
  await expect(page.getByText('没有符合条件的 Java 业务服务')).toBeVisible()
  await expectNoDestructiveControls(page)
})
