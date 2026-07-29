# InfraView MySQL Compact Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将基础设施总览改为桌面四列紧凑模块网格，并让 MySQL 11 列实例表在 1440×900 下横向一屏完整展示。

**Architecture:** 保持现有 React 数据流、API 和领域契约不变，只增加明确的紧凑布局 class、调整桌面/响应式 CSS，并以 Vitest 锁定 DOM 契约、以 Chromium 锁定计算后尺寸和滚动行为。完成后只重建既有 8080 测试服务。

**Tech Stack:** React 19、TypeScript、TanStack Table、Vitest、Testing Library、CSS Grid、Playwright Chromium、Docker Compose

## Global Constraints

- InfraView 始终只读，不增加 SQL、任意 PromQL、详情、历史、运维按钮或写能力。
- 不改变 Nightingale 固定 13 查询、一次即时 batch、缓存、健康阈值或两个 MySQL GET API。
- 1440×900 是最低桌面验收视口；总览桌面一行至少四个模块位。
- MySQL 11 列必须全部保留；桌面不允许页面级或表格级横向滚动。
- 实例行保持单行紧凑展示；长文本可省略，但完整值必须保留在 `title`。
- 手机和窄屏允许表格容器内部滚动，不允许撑宽整个页面。
- 开发 8080 服务永远连接测试 Nightingale，不连接生产。
- 不输出私密环境文件、凭据、Cookie、认证头、响应正文、真实标识、地址、资源数量或指标值。
- 不创建其他预览端口；验证后只重建现有 `infraview` Compose 项目的 8080 服务。
- 不推送、不合并；用户完成视觉验收后再决定。

---

### Task 1: 将总览改为四列紧凑模块网格

**Files:**
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/app/theme.css`
- Create: `web/e2e/mysql-compact-live.spec.ts`

**Interfaces:**
- Consumes: 现有 `HostStatusCard`、`MySQLStatusCard` 及独立 query 状态。
- Produces: 可通过 `role="group"`、`aria-label="基础设施模块"` 和 `overview-compact-grid` 定位的模块网格。

- [x] **Step 1: 写失败的 DOM 布局契约测试**

在 `OverviewPage.test.tsx` 的总览成功场景中增加：

```tsx
const moduleGrid = screen.getByRole('group', { name: '基础设施模块' })
expect(moduleGrid).toHaveClass(
  'overview-status-grid',
  'overview-compact-grid',
)
expect(
  within(moduleGrid).getByRole('link', { name: '查看 Linux 主机板块' }),
).toBeVisible()
expect(
  within(moduleGrid).getByRole('link', { name: '查看 MySQL 板块' }),
).toBeVisible()
```

新增只依赖页面语义、不读取或输出上游数据的 live Playwright 用例。显式固定 1440×900，并在登录后增加计算后尺寸断言：

```ts
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
```

- [x] **Step 2: 运行 Vitest 和原 8080 Chromium 确认 RED**

Run:

```bash
docker run --rm \
  -v "$PWD/web:/src/web" \
  -w /src/web \
  node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/overview/OverviewPage.test.tsx'

(
  set -a
  . "$INFRAVIEW_ENV_FILE"
  set +a
  docker run --rm --network host --ipc=host \
    -e CI=1 \
    -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
    -e INFRAVIEW_E2E_USERNAME="$INFRAVIEW_USERNAME" \
    -e INFRAVIEW_E2E_PASSWORD="$INFRAVIEW_PASSWORD" \
    -v "$PWD:/work" \
    -v /work/web/node_modules \
    -w /work/web \
    mcr.microsoft.com/playwright:v1.61.1-noble \
    sh -c 'npm ci >/dev/null && npx playwright test e2e/mysql-compact-live.spec.ts'
)
```

Expected: Vitest 找不到名为“基础设施模块”的 group；当前旧版 8080 的 Chromium 尺寸断言失败。命令不得输出环境变量值、响应正文或真实数据。

- [x] **Step 3: 增加明确的紧凑网格 DOM**

将 `OverviewPage.tsx` 的模块容器改为：

```tsx
<div
  className="overview-status-grid overview-compact-grid"
  role="group"
  aria-label="基础设施模块"
>
```

- [x] **Step 4: 实现桌面四列和紧凑卡片 CSS**

在 `theme.css` 中让桌面默认使用四列：

```css
.overview-compact-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.overview-compact-grid .module-status-card {
  padding: 0.52rem;
}

.overview-compact-grid .module-alert-summary {
  gap: 0.3rem;
  margin-top: 0.4rem;
  padding: 0.32rem 0;
}

.overview-compact-grid .module-alert-total strong {
  font-size: 1.05rem;
}

.overview-compact-grid .module-metric-alert-grid {
  gap: 0.22rem;
  margin-top: 0.3rem;
}

.overview-compact-grid .module-metric-alert {
  padding: 0.28rem 0.32rem;
}
```

同步调整现有媒体查询：默认四列，`max-width: 1100px` 两列，`max-width: 480px` 一列。对应选择器使用 `.overview-compact-grid`，不得让 `.overview-status-grid` 的旧两列规则覆盖桌面四列。

- [x] **Step 5: 运行聚焦测试确认 GREEN**

Run:

```bash
docker run --rm \
  -v "$PWD/web:/src/web" \
  -w /src/web \
  node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

Expected: PASS；现有 loading、empty、error、stale 和刷新测试均保持通过。浏览器 GREEN 在 Task 3 重建同一个 8080 后确认。

- [x] **Step 6: 提交总览布局**

```bash
git add \
  web/src/features/overview/OverviewPage.tsx \
  web/src/features/overview/OverviewPage.test.tsx \
  web/src/app/theme.css \
  web/e2e/mysql-compact-live.spec.ts
git diff --cached --check
git commit -m "fix: 压缩基础设施总览模块"
```

---

### Task 2: 将 MySQL 11 列表格压缩到桌面一屏

**Files:**
- Modify: `web/src/features/mysql/MySQLPage.tsx`
- Modify: `web/src/features/mysql/MySQLPage.test.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/mysql-compact-live.spec.ts`

**Interfaces:**
- Consumes: 现有 11 列 `ColumnDef<MySQLInstance>[]` 和 `host-table` 基础样式。
- Produces: `mysql-table-scroll` 与 `mysql-table-compact` 专用布局 class；11 列字段和排序接口保持不变。

- [x] **Step 1: 写失败的紧凑表格契约测试**

在 `MySQLPage.test.tsx` 的 11 列场景增加：

```tsx
const table = screen.getByRole('table')
expect(table).toHaveClass(
  'host-table',
  'mysql-table',
  'mysql-table-compact',
)
expect(table.closest('.mysql-table-scroll')).not.toBeNull()
expect(cells[3]).toHaveTextContent('32/200 · 16.0%')
expect(cells[8].querySelector('.host-metric')).toHaveAttribute(
  'title',
  '正常 · 2s',
)
```

同时在 live Playwright 文件中增加独立用例；只读取布局尺寸和表头，不输出实例数据：

```ts
test('原 8080 的 MySQL 11 列在 1440×900 下无横向滚动', async ({
  page,
}) => {
  test.skip(!username || !password, '需要显式提供测试服务凭据')
  await login(page)
  await page.getByRole('link', { name: '查看 MySQL 板块' }).click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()

  const headers = page.getByRole('columnheader')
  await expect(headers).toHaveCount(11)
  for (const header of await headers.all()) {
    await expect(header).toBeVisible()
  }

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
```

- [x] **Step 2: 运行 Vitest 和原 8080 Chromium 确认 RED**

Run:

```bash
docker run --rm \
  -v "$PWD/web:/src/web" \
  -w /src/web \
  node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/mysql/MySQLPage.test.tsx'

(
  set -a
  . "$INFRAVIEW_ENV_FILE"
  set +a
  docker run --rm --network host --ipc=host \
    -e CI=1 \
    -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
    -e INFRAVIEW_E2E_USERNAME="$INFRAVIEW_USERNAME" \
    -e INFRAVIEW_E2E_PASSWORD="$INFRAVIEW_PASSWORD" \
    -v "$PWD:/work" \
    -v /work/web/node_modules \
    -w /work/web \
    mcr.microsoft.com/playwright:v1.61.1-noble \
    sh -c 'npm ci >/dev/null && npx playwright test e2e/mysql-compact-live.spec.ts --grep "MySQL 11 列"'
)
```

Expected: Vitest 因缺少紧凑 class、短连接格式和复制 `title` 失败；当前旧版 8080 因缺少专用 class 或表格横向溢出而失败。

- [x] **Step 3: 压缩显示格式并增加完整提示**

将连接使用率格式从：

```text
32 / 200 (16.0%)
```

改为：

```text
32/200 · 16.0%
```

`ReplicationText` 计算出的完整 `text` 同时作为 `title`：

```tsx
<span className="host-metric" data-level={level} title={text}>
  {text}
</span>
```

将表格容器和表格改为：

```tsx
<div className="host-table-scroll mysql-table-scroll">
  <table className="host-table mysql-table mysql-table-compact">
```

- [x] **Step 4: 实现桌面无滚动的专用表格 CSS**

在 `theme.css` 中替换 `104rem` 规则：

```css
.mysql-table-scroll {
  overflow-x: hidden;
}

.mysql-table-compact {
  width: 100%;
  min-width: 0;
}

.mysql-table-compact th,
.mysql-table-compact td {
  padding: 0.32rem 0.3rem;
}

.mysql-table-compact th {
  font-size: 0.68rem;
  line-height: 1.1;
  white-space: normal;
}

.mysql-table-compact td {
  font-size: 0.72rem;
  line-height: 1.15;
}
```

桌面 11 列宽度总和固定为 100%：

```css
.mysql-table-compact th:nth-child(1) { width: 14%; }
.mysql-table-compact th:nth-child(2) { width: 9%; }
.mysql-table-compact th:nth-child(3) { width: 9%; }
.mysql-table-compact th:nth-child(4) { width: 10%; }
.mysql-table-compact th:nth-child(5) { width: 7%; }
.mysql-table-compact th:nth-child(6) { width: 6%; }
.mysql-table-compact th:nth-child(7) { width: 7%; }
.mysql-table-compact th:nth-child(8) { width: 9%; }
.mysql-table-compact th:nth-child(9) { width: 12%; }
.mysql-table-compact th:nth-child(10) { width: 8%; }
.mysql-table-compact th:nth-child(11) { width: 9%; }
```

在 `max-width: 1100px` 的媒体查询中恢复窄屏内部滚动：

```css
.mysql-table-scroll {
  overflow-x: auto;
}

.mysql-table-compact {
  min-width: 68rem;
}
```

- [x] **Step 5: 运行 MySQL 与 Host 聚焦回归**

Run:

```bash
docker run --rm \
  -v "$PWD/web:/src/web" \
  -w /src/web \
  node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/mysql/MySQLPage.test.tsx src/features/hosts/HostListPage.test.tsx'
```

Expected: PASS；MySQL 11 列和 Host 表格均无回归。浏览器 GREEN 在 Task 3 重建同一个 8080 后确认。

- [x] **Step 6: 提交表格布局**

```bash
git add \
  web/src/features/mysql/MySQLPage.tsx \
  web/src/features/mysql/MySQLPage.test.tsx \
  web/src/app/theme.css \
  web/e2e/mysql-compact-live.spec.ts
git diff --cached --check
git commit -m "fix: 压缩 MySQL 实例表格"
```

---

### Task 3: 完整验证、重建原 8080 并锁定 1440×900 布局

**Files:**
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/superpowers/plans/2026-07-29-mysql-compact-layout.md`

**Interfaces:**
- Consumes: Task 1 的 `overview-compact-grid`、Task 2 的 `mysql-table-scroll`。
- Produces: 1440×900 计算后布局回归证据和只使用原 8080 测试服务的交接状态。

- [x] **Step 1: 运行完整前端和生产镜像验证**

Run:

```bash
docker build --no-cache --progress=plain .
```

Expected: 全量 Vitest、typecheck、production build、Go test、Go race、Linux build 全部通过。

- [x] **Step 2: 运行 E2E 清理边界安全测试**

Run:

```bash
./scripts/e2e-safety.test.sh
```

Expected: PASS。不得运行会绑定 18080 或其他宿主机端口的 `scripts/e2e.sh`；本计划所有实际页面测试只针对原 8080。

- [x] **Step 3: 只重建现有 8080 测试服务**

执行前由控制器在 shell 中提供 Git 忽略且权限安全的 `INFRAVIEW_ENV_FILE`，不得输出其值。

Run:

```bash
INFRAVIEW_PORT=8080 \
docker compose -p infraview up -d --build --force-recreate
```

Expected: 既有 `infraview` 容器被精确重建；没有创建其他 Compose 项目、预览服务或端口。

- [x] **Step 4: 等待健康并做无正文 HTTP 检查**

Run:

```bash
container_id=$(
  INFRAVIEW_PORT=8080 \
    docker compose -p infraview ps -q infraview
)
test -n "$container_id"

health=
attempt=0
while [ "$attempt" -lt 30 ]; do
  health=$(docker inspect --format '{{.State.Health.Status}}' "$container_id")
  [ "$health" = healthy ] && break
  attempt=$((attempt + 1))
  sleep 2
done
test "$health" = healthy

test "$(
  curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/
)" = 200
test "$(
  curl -sS -o /dev/null -w '%{http_code}' \
    http://127.0.0.1:8080/api/v1/mysql/overview
)" = 401
```

Expected: 容器达到 healthy；根页面为 200，未认证 MySQL GET 为 401。命令只以退出码表示结果，不输出响应正文。

- [x] **Step 5: 在原 8080 执行脱敏 Chromium 验收**

Run:

```bash
(
  set -a
  . "$INFRAVIEW_ENV_FILE"
  set +a
  docker run --rm --network host --ipc=host \
    -e CI=1 \
    -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
    -e INFRAVIEW_E2E_USERNAME="$INFRAVIEW_USERNAME" \
    -e INFRAVIEW_E2E_PASSWORD="$INFRAVIEW_PASSWORD" \
    -v "$PWD:/work" \
    -v /work/web/node_modules \
    -w /work/web \
    mcr.microsoft.com/playwright:v1.61.1-noble \
    sh -c 'npm ci >/dev/null && npx playwright test e2e/mysql-compact-live.spec.ts'
)
```

Expected: PASS；认证后页面来自测试 Nightingale，总览四列轨道、Linux/MySQL 同宽、MySQL 11 列和所有表头可见，页面与表格均无横向滚动。不得输出响应正文、真实标识、地址、资源数量或指标值。

- [x] **Step 6: 同步状态文档、计划复选框并提交证据**

文档必须记录：

- 总览桌面四列紧凑模块位。
- MySQL 11 列在 1440×900 下无横向滚动。
- 开发 8080 永远使用测试 Nightingale。
- 未创建其他测试端口。
- 镜像、安全测试、部署健康和 live Chromium 的实际结果。

Run:

```bash
git add \
  docs/HANDOFF.md \
  docs/PROJECT_STATUS.md \
  docs/TODO.md \
  docs/superpowers/plans/2026-07-29-mysql-compact-layout.md
git diff --cached --check
git commit -m "test: 验收 MySQL 紧凑布局"
```
