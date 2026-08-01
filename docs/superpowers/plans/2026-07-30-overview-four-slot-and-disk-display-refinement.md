# InfraView Overview Four-Slot and Disk Display Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> 2026-08-01 替代说明：本文的型号/容量同列两行是历史展示计划。当前以 `docs/superpowers/specs/2026-08-01-disk-capacity-metric-and-column-design.md` 及对应计划为准，型号与容量已拆为两个独立列并增加容量服务端排序；总览四槽位、“异常断电”和寿命语义不变。

**Goal:** 恢复总览桌面端一行四个固定槽位，并让硬盘容量不再被长型号裁切，同时把 unsafe shutdown 改为清晰的“异常断电 N 次”累计展示。

**Architecture:** 仅调整现有 React 展示与 CSS，不改变 Nightingale 查询、硬盘 Provider、Service、API schema、状态聚合或 60 秒新鲜度。总览网格在桌面固定四列，中屏两列、窄屏一列；硬盘型号与容量在同一单元格中拆成两行；错误摘要继续由固定字段生成，正文最多两项、`title` 保留全部非零项。

**Tech Stack:** React 19、TypeScript 5.8、TanStack Table、Vitest、Testing Library、Playwright、CSS Grid、Docker。

## Global Constraints

- 完整设计依据：`docs/superpowers/specs/2026-07-30-overview-four-slot-and-disk-display-refinement-design.md`。
- InfraView 始终只读；本轮不得新增任何运维动作、任意 PromQL、上游代理、SMART 自检或本机/远程命令。
- 开发 8080 永远只连接测试 Nightingale；不得连接、切换或探测生产 Nightingale，也不得创建其他 InfraView 测试端口。
- 不读取、输出或提交私密环境、Token、Cookie、认证头、Base URL、真实标识/IP/数量/指标值或上游正文。
- 只修改前端展示和对应测试；不得改变 `capacity_bytes`、SMART 错误计数、状态来源或 freshness 契约。
- 缺失寿命必须继续显示“暂无数据”，不得改成 `-`。
- 总览当前三张卡只占桌面四个槽位的前三格；不得新增占位卡或空 DOM。
- 错误摘要中待处理扇区、重映射扇区、不可校正扇区继续保留；零值不展示，各项不得求和。
- 当前工作树包含用户及前序 SMART 模块的未提交修改；不得 reset、clean、覆盖或还原现有差异。
- 所有构建和测试使用 Docker；不得安装宿主机依赖或在宿主机启动服务。
- 本计划中的命令均为未来实施命令，创建计划时尚未执行。
- 未经用户另行明确授权，不重建或重启现有 8080，不部署，不提交，不推送，也不运行会映射 18080 的 `scripts/e2e.sh`。

---

### Task 1: 用测试锁定桌面四槽位总览几何

**Files:**
- Modify: `web/e2e/infraview.spec.ts`
- Modify: `web/src/app/theme.css`

**Behavior:**
- 1440×900 桌面视口下 `.overview-compact-grid` 必须有四个计算后网格轨道。
- Linux、主机硬盘、MySQL 三张卡仍在第一行并依次占前三个轨道。
- 单张卡宽度约为网格可用宽度的四分之一，第四槽位自然留空。
- `max-width: 1100px` 保持两列，`max-width: 480px` 保持一列。

- [ ] **Step 1: 在现有总览 E2E 中写四槽位失败断言**

在 `web/e2e/infraview.spec.ts` 的总览关键路径中，登录后、点击任何卡片前加入：

```ts
const overviewGrid = page.locator('.overview-compact-grid')
const overviewCards = overviewGrid.locator('.module-status-card')
await expect(overviewCards).toHaveCount(3)

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
expect(new Set(geometry.cardTops)).toHaveLength(1)
for (const width of geometry.cardWidths) {
  expect(width).toBeGreaterThan(geometry.gridWidth * 0.2)
  expect(width).toBeLessThan(geometry.gridWidth * 0.27)
}
```

`0.2–0.27` 仅容纳三段 gap 和像素舍入，不把测试耦合到某个绝对宽度。

- [ ] **Step 2: 在现有 8080 上运行该条 E2E，确认 RED**

仅在测试凭据已由安全进程环境提供、且不会被命令输出时运行；容器使用 host network，不发布任何端口：

```bash
docker run --rm --network host \
  -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
  -e INFRAVIEW_E2E_USERNAME \
  -e INFRAVIEW_E2E_PASSWORD \
  -v "$PWD/web:/work" -w /work \
  mcr.microsoft.com/playwright:v1.61.1-noble \
  bash -lc 'npm ci --ignore-scripts && npx playwright test e2e/infraview.spec.ts --project=chromium --grep "未登录会重定向"'
```

预期：断言得到 3 个网格轨道而失败；不得输出凭据、API 正文或页面现场数据。若安全进程环境尚未提供凭据，只记录 RED 的静态根因，不读取私密文件补取。

- [ ] **Step 3: 最小修改桌面网格**

将 `web/src/app/theme.css` 的桌面规则改为：

```css
.overview-compact-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
```

保留已有 `@media (max-width: 1100px)` 两列和 `@media (max-width: 480px)` 一列规则，不新增第四张卡、占位节点或固定卡片宽度。

- [ ] **Step 4: 运行前端静态验证**

```bash
docker run --rm \
  -v "$PWD/web:/src" -w /src node:22-alpine \
  sh -lc 'npm ci --ignore-scripts && npm run typecheck && npm run build'
```

预期：typecheck 和 production build 退出 0。几何 GREEN 留到 Task 3 在重建后的现有 8080 上验证，避免启动第二个 InfraView 端口。

---

### Task 2: 用单元测试细化型号/容量和错误摘要语义

**Files:**
- Modify: `web/src/features/disks/DiskPage.test.tsx`
- Modify: `web/src/features/disks/DiskPage.tsx`
- Modify: `web/src/app/theme.css`

**Behavior:**
- 型号与容量使用两个独立元素；长型号只裁切型号，容量保持独立可见。
- 型号完整内容保存在型号元素的 `title`；容量使用格式化值并保留自身 `title`。
- 型号或容量缺失时分别显示“暂无数据”，不得从型号推测容量。
- 缺失寿命继续显示“暂无数据”。
- unsafe shutdown 显示“异常断电 N 次”；含该项的完整提示追加“累计次数，仅展示，不参与状态判断”。
- 错误摘要正文仍最多两项，`title` 仍包含全部非零项；三个机械盘扇区计数不得被移除。

- [ ] **Step 1: 写型号/容量和寿命失败测试**

调整 `diskDevicePageFixture` 的测试副本或当前首行设备为足够长的虚构型号，并在“严格渲染九列”测试中用独立 class 查询：

```ts
const modelCapacityCell = healthyCells[2]
expect(
  within(modelCapacityCell).getByText('Atlas Enterprise NVMe Fixture Model 2TB'),
).toHaveClass('disk-model')
expect(within(modelCapacityCell).getByText('2 TiB')).toHaveClass(
  'disk-capacity',
)
expect(within(modelCapacityCell).getByText('2 TiB')).toHaveAttribute(
  'title',
  '2 TiB',
)
```

继续保留未知设备行的寿命单元格断言：

```ts
const unknownCells = within(unknownRow!).getAllByRole('cell')
expect(unknownCells[5]).toHaveTextContent('暂无数据')
```

- [ ] **Step 2: 写错误摘要失败测试**

使用脱敏 fixture 让同一设备同时具有：

```ts
pending_sectors: 2,
reallocated_sectors: 1,
uncorrectable_sectors: 3,
unsafe_shutdowns: 7,
```

断言：

```ts
expect(errorSummary).toHaveTextContent('待处理扇区 2 · 重映射扇区 1')
expect(errorSummary).not.toHaveTextContent('不可校正扇区 3')
expect(errorSummary).not.toHaveTextContent('异常断电 7 次')
expect(errorSummary.firstElementChild).toHaveAttribute(
  'title',
  '待处理扇区 2 · 重映射扇区 1 · 不可校正扇区 3 · 异常断电 7 次（累计次数，仅展示，不参与状态判断）',
)
expect(errorSummary).not.toHaveTextContent('总计')
```

另加仅 `unsafe_shutdowns` 非零的用例，锁定正文正好为“异常断电 12 次”，并断言页面不再出现“非安全关机”。

- [ ] **Step 3: 运行聚焦单元测试，确认 RED**

```bash
docker run --rm \
  -v "$PWD/web:/src" -w /src node:22-alpine \
  sh -lc 'npm ci --ignore-scripts && npm run test:run -- src/features/disks/DiskPage.test.tsx'
```

预期：因型号/容量仍是单一文本、标签仍为“非安全关机”且没有累计说明而失败。

- [ ] **Step 4: 最小实现两行型号/容量**

移除只返回合并字符串的 `modelCapacity()`，在该列 cell 中分别计算：

```tsx
const model = row.original.model.trim() || '暂无数据'
const capacity = binaryCapacity(row.original.capacity_bytes)
return (
  <span className="disk-model-capacity">
    <span className="disk-model" title={model}>
      {model}
    </span>
    <span className="disk-capacity" title={capacity}>
      {capacity}
    </span>
  </span>
)
```

将当前把 `.disk-model-capacity` 与单行省略元素合并的 CSS 拆开：

```css
.disk-model-capacity {
  display: grid;
  min-width: 0;
  gap: 0.12rem;
}

.disk-model {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.disk-capacity {
  display: block;
  min-width: 0;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 0.75rem;
}
```

使用项目已有的 muted 文本 CSS 变量；若实际变量名不同，必须复用现有等价变量，不新增近似颜色常量。

- [ ] **Step 5: 最小实现“异常断电”格式与完整提示**

扩展内部展示类型而不改 API：

```ts
type ErrorItem = {
  label: string
  value: number | null
  suffix?: string
}
```

把 unsafe 项改为：

```ts
{ label: '异常断电', value: errors.unsafe_shutdowns, suffix: ' 次' }
```

统一格式化：

```ts
function errorItemText(item: ErrorItem) {
  return `${item.label} ${item.value}${item.suffix ?? ''}`
}
```

当非零项包含 `unsafe_shutdowns` 时，仅在完整 `title` 末尾追加：

```ts
（累计次数，仅展示，不参与状态判断）
```

正文仍使用格式化后的前两项，不能把说明追加进正文，也不能改变 `status` 或 `status_source`。

- [ ] **Step 6: 运行聚焦测试，确认 GREEN**

```bash
docker run --rm \
  -v "$PWD/web:/src" -w /src node:22-alpine \
  sh -lc 'npm ci --ignore-scripts && npm run test:run -- src/features/disks/DiskPage.test.tsx'
```

预期：聚焦测试退出 0，且寿命缺失、“暂无数据”、最多两项及不求和断言继续通过。

---

### Task 3: 全量回归、授权后原位部署与现场验收

**Files:**
- Modify: `web/e2e/infraview.spec.ts`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/superpowers/specs/2026-07-30-overview-four-slot-and-disk-display-refinement-design.md`
- Modify: `docs/superpowers/plans/2026-07-30-overview-four-slot-and-disk-display-refinement.md`

- [x] **Step 1: 运行前端全量验证（2026-07-30：Docker 离线通过，Vitest 8 文件/101 测试、typecheck、build 均退出 0）**

```bash
docker run --rm \
  -v "$PWD/web:/src" -v /src/node_modules -w /src node:22-alpine \
  sh -lc 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build'
```

预期：全部 Vitest、typecheck 和 production build 退出 0。记录实际测试数量；Vite 既有警告与依赖审计项单独记录，不在本轮扩展范围。

- [x] **Step 2: 运行无缓存全仓镜像验证（2026-07-30：`infraview:overview-disk-display-verify` 构建退出 0）**

```bash
docker build --no-cache \
  --tag infraview:overview-disk-display-verify .
```

预期：镜像构建退出 0；Dockerfile 内前端全量测试/typecheck/build、Go 普通测试、Go race 测试和二进制编译全部通过。Go 代码虽未变，仍保留全仓回归证据。

- [x] **Step 3: 做工作树与文档前置检查（2026-07-30：无空白错误，既有 SMART/用户文档差异已保留）**

```bash
git diff --check
git status --short --branch
```

预期：无空白错误；既有 SMART 模块及交接文档差异均被保留，没有意外生成的截图、trace、`test-results`、凭据或环境文件。

- [x] **Step 4: 现有 8080 原位重建已获授权并执行（2026-07-30：只重建 `infraview` 项目原服务，未创建额外端口）**

这是独立授权门。若用户未明确授权，本任务在代码与离线验证完成后停止，不执行后续部署命令。

获得授权后，仅执行：

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env \
  docker compose --project-name infraview \
  up -d --build --force-recreate infraview
```

该命令只把现有 `infraview` 服务原位重建在既有 8080，不创建第二个 InfraView 端口；不得显示 `.env` 内容。

- [x] **Step 5: 验证现有 8080 健康和容器安全边界（2026-07-30：healthy、非 root、只读根文件系统、cap drop、禁止提权、唯一 8080 均通过）**

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null
docker compose --project-name infraview ps
docker inspect infraview-infraview-1 \
  --format '{{.Config.User}}|{{.HostConfig.ReadonlyRootfs}}|{{json .HostConfig.CapDrop}}|{{json .HostConfig.SecurityOpt}}|{{json .NetworkSettings.Ports}}'
```

预期：healthz 成功；服务仅暴露既有 8080；仍为非 root、只读 rootfs、cap drop、no-new-privileges。不得打印环境变量、数据源地址或认证信息。

- [x] **Step 6: 在现有 8080 运行 Chromium GREEN（2026-07-30：登录态只读 API及 1440×900 四轨/硬盘两行/无溢出验收通过）**

只在测试凭据已由安全进程环境提供时运行 Task 1 的同一条 Playwright 命令。除了四轨道断言，还在硬盘页补充：

```ts
const firstDataRow = page.getByRole('row').nth(1)
await expect(firstDataRow.locator('.disk-capacity')).toBeVisible()
await expect(firstDataRow.locator('.disk-model')).toBeVisible()
```

并保留原有九列、页面和表格无横向溢出、无破坏性控件、登录后无浏览器错误验证。不得截图、保留 trace 或输出 API 正文；验证结束删除 Playwright 临时结果目录中的本轮产物，不删除任何仓库既有文件。

- [x] **Step 7: 增量更新项目文档（2026-07-30：记录离线证据、授权后的现有 8080 重建及脱敏现场验收）**

仅在全部已执行验证有真实证据后更新：

- `docs/HANDOFF.md`：记录四槽位、两行型号/容量、“异常断电”语义、验证状态和未授权事项。
- `docs/PROJECT_STATUS.md`：记录本细化项完成度与测试结果。
- `docs/TODO.md`：只勾选实际完成项，部署未授权时保持待办。
- `docs/TESTING.md`：记录实际命令类别、测试数量、镜像标签、现有 8080 是否原位重建及 Chromium 是否执行。
- 本设计和计划：写入实际执行状态，不伪造未执行命令。

- [x] **Step 8: 最终只读复核（2026-07-30：`git diff --check` 无输出且退出 0；既有差异保留）**

```bash
git diff --check
git status --short --branch
git diff -- \
  web/src/features/disks/DiskPage.tsx \
  web/src/features/disks/DiskPage.test.tsx \
  web/src/app/theme.css \
  web/e2e/infraview.spec.ts \
  docs/HANDOFF.md \
  docs/PROJECT_STATUS.md \
  docs/TODO.md \
  docs/TESTING.md \
  docs/superpowers/specs/2026-07-30-overview-four-slot-and-disk-display-refinement-design.md \
  docs/superpowers/plans/2026-07-30-overview-four-slot-and-disk-display-refinement.md
```

预期：差异仅包含批准范围；无生产连接、后端契约、额外端口、凭据或破坏性功能变化。

本计划不包含 `git add`、commit 或 push。三者只有在用户分别明确授权后才能执行。
