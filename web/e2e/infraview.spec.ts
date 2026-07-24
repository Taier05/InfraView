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
      name: /重启|删除|执行|远程命令|修改|发布|配置下发/,
    }),
  ).toHaveCount(0)
  await expect(
    page.getByRole('link', {
      name: /重启|删除|执行|远程命令|修改|发布|配置下发/,
    }),
  ).toHaveCount(0)
}

test('未登录会重定向，登录后可完成总览和主机列表关键路径', async ({
  page,
}) => {
  await page.goto('/hosts')
  await expect(page).toHaveURL(/\/login$/)

  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()

  await expect(
    page.getByRole('heading', { name: '基础设施总览' }),
  ).toBeVisible()
  await expect(page.getByText('主机总数')).toBeVisible()
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

  await page.getByRole('link', { name: '查看 Linux 主机板块' }).click()
  await expect(
    page.getByRole('heading', { name: '主机', exact: true }),
  ).toBeVisible()
  await page.getByLabel('搜索主机名或 IP').fill('linux-017')
  await page.getByLabel('主机状态').selectOption('offline')
  await expect(page).toHaveURL(/q=linux-017/)
  await expect(page).toHaveURL(/status=offline/)
  await expect(page.getByText('linux-017')).toBeVisible()
  await expect(page.getByRole('row')).toHaveCount(2)
  await expect(page.getByRole('columnheader', { name: 'IO 忙碌度' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: '网络 出/入' })).toBeVisible()
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
