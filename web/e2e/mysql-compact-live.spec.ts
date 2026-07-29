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
