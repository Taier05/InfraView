# Observability Table Typography Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将主机、硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java 七个列表的表头、正文、内边距和状态徽标统一为同一套紧凑排版，同时保持现有列宽、排序、筛选、分页和单行展示不变。

**Architecture:** 在既有 `host-table` 基础上增加唯一共享排版类 `observability-table`。七页只负责挂载该类；`theme.css` 的共享规则负责字体、行高、内边距和状态徽标尺寸，模块专属规则仅保留列宽、最小宽度、滚动、截断和语义颜色。

**Tech Stack:** React 19、TypeScript 5.8、TanStack Table、CSS、Vitest、Testing Library、Node 22、Docker。

## Global Constraints

- InfraView 始终只读；不得增加写 API、运维控件、任意查询或代理能力。
- 不修改任何 Nightingale 查询、Provider、Service、API View、缓存或 freshness。
- 不改变七页的列数、列顺序、列宽、排序键、筛选、分页、采集时间、空状态或错误展示。
- 每个单元格继续只显示一个单行值；表头不得出现可见排序箭头。
- 共享精确值必须是：单元格内边距 `0.3rem 0.18rem`；表头 `0.62rem/750/1.1`；正文 `0.69rem/500/1.25`；状态徽标 `0.66rem`、间距 `0.2rem`、内边距 `0.14rem 0.26rem`、圆点 `0.36rem × 0.36rem`。
- 模块专属 CSS 不得重新声明共享字号、字重、行高、表头/正文内边距或状态徽标尺寸。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文。
- 测试、typecheck、build 只在一次性容器执行；不得启动服务、浏览器或新端口。
- 本计划不授权提交、推送、合并、部署或重启；对应动作必须取得用户单独授权。

## File map

- Create: `web/src/app/theme.test.ts`：把完整生产 CSS 注入测试 DOM，以实际 cascade/计算样式锁定共享 token 与禁止覆盖契约。
- Modify: `web/src/app/theme.css`：增加共享排版规则并删除七页专属排版覆盖。
- Modify/Test: `web/src/features/hosts/HostListPage.tsx`、`HostListPage.test.tsx`。
- Modify/Test: `web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`。
- Modify/Test: `web/src/features/mysql/MySQLPage.tsx`、`MySQLPage.test.tsx`。
- Modify/Test: `web/src/features/redis/RedisPage.tsx`、`RedisPage.test.tsx`。
- Modify/Test: `web/src/features/elasticsearch/ElasticsearchPage.tsx`、`ElasticsearchPage.test.tsx`。
- Modify/Test: `web/src/features/rabbitmq/RabbitMQPage.tsx`、`RabbitMQPage.test.tsx`。
- Modify/Test: `web/src/features/java/JavaPage.tsx`、`JavaPage.test.tsx`。

---

### Task 1: 锁定七页共享 class 契约

**Files:**
- Test: `web/src/features/hosts/HostListPage.test.tsx`
- Test: `web/src/features/disks/DiskPage.test.tsx`
- Test: `web/src/features/mysql/MySQLPage.test.tsx`
- Test: `web/src/features/redis/RedisPage.test.tsx`
- Test: `web/src/features/elasticsearch/ElasticsearchPage.test.tsx`
- Test: `web/src/features/rabbitmq/RabbitMQPage.test.tsx`
- Test: `web/src/features/java/JavaPage.test.tsx`

- [ ] **Step 1: 在每页现有列表结构测试中增加 RED 断言**

每个测试复用现有表格定位方式：主机、硬盘、MySQL、Redis 使用 `screen.getByRole("table")`；Elasticsearch、RabbitMQ、Java 可继续使用已有具名表格变量。手写断言共享 class，并保留原有模块 class 断言。例如：

```tsx
const table = screen.getByRole("table")
expect(table).toHaveClass("host-table", "host-list-table", "observability-table")
```

其余六页分别断言现有模块 class：`disk-table`、`mysql-table`、`redis-table`、`elasticsearch-table`、`rabbitmq-table`、`java-table`。不要为本次纯排版改动额外添加可访问名称，也不要从生产代码读取 class 期望值。

- [ ] **Step 2: 运行七页 RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx'
```

Expected: FAIL，且失败只来自七个表格缺少 `observability-table`。

- [ ] **Step 3: 给七个表格增加共享 class**

仅修改各页 `<table>` 的 `className`，保留原 class 和顺序语义。例如：

```tsx
<table className="host-table host-list-table observability-table">
```

RabbitMQ 多行属性写法保持可读：

```tsx
<table
  className="host-table rabbitmq-table observability-table"
  aria-label="RabbitMQ 节点列表"
>
```

- [ ] **Step 4: 运行七页 class GREEN**

重复 Step 2 命令。

Expected: PASS；原有列数、排序 URL、分页重置、无箭头和滚动容器测试也全部通过。

---

### Task 2: 建立唯一共享排版规则

**Files:**
- Create: `web/src/app/theme.test.ts`
- Modify: `web/src/app/theme.css`

- [ ] **Step 1: 写共享 CSS token RED 测试**

新增 `theme.test.ts`，把完整生产 CSS 注入测试 DOM，并通过 `getComputedStyle` 验证实际 cascade 结果；期望值全部手写，不用正则检查源码：

```ts
import { afterEach, beforeEach, describe, expect, it } from "vitest"
import themeSource from "./theme.css?raw"

describe("observability table typography", () => {
  beforeEach(() => {
    const style = document.createElement("style")
    style.dataset.testid = "theme"
    style.textContent = themeSource
    document.head.append(style)
  })

  afterEach(() => {
    document.head.querySelector('[data-testid="theme"]')?.remove()
    document.body.replaceChildren()
  })

  it.each([
    "host-list-table", "disk-table", "mysql-table", "redis-table",
    "elasticsearch-table", "rabbitmq-table", "java-table",
  ])("computes the exact shared tokens for %s", (moduleClass) => {
    document.body.innerHTML = `<table class="host-table observability-table ${moduleClass}"><thead><tr><th>表头</th></tr></thead><tbody><tr><td><span class="status-badge"><span class="status-badge-dot"></span>正常</span></td></tr></tbody></table>`
    const header = getComputedStyle(document.querySelector("th")!)
    const cell = getComputedStyle(document.querySelector("td")!)
    const badge = getComputedStyle(document.querySelector(".status-badge")!)
    const dot = getComputedStyle(document.querySelector(".status-badge-dot")!)

    expect([header.paddingTop, header.paddingRight, header.paddingBottom, header.paddingLeft]).toEqual(["0.3rem", "0.18rem", "0.3rem", "0.18rem"])
    expect([header.fontSize, header.fontWeight, header.lineHeight]).toEqual(["0.62rem", "750", "1.1"])
    expect([cell.fontSize, cell.fontWeight, cell.lineHeight]).toEqual(["0.69rem", "500", "1.25"])
    expect([badge.fontSize, badge.gap, badge.paddingTop, badge.paddingRight]).toEqual(["0.66rem", "0.2rem", "0.14rem", "0.26rem"])
    expect([dot.width, dot.height]).toEqual(["0.36rem", "0.36rem"])
  })
})
```

该矩阵使用真实模块 class 与共享 class 组合，因此模块专属选择器若在 CSS 后部重新覆盖共享排版，最终计算值会直接失败。测试不限制列宽、`min-width`、`text-overflow`、颜色或布局。

- [ ] **Step 2: 运行 CSS RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/app/theme.test.ts'
```

Expected: FAIL，共享规则尚不存在且现有模块专属排版声明仍被检测到。

- [ ] **Step 3: 在 `theme.css` 增加共享规则**

把规则放在基础 `.host-table` 之后、模块列宽规则之前，以清楚表达所有权：

```css
.host-table.observability-table th,
.host-table.observability-table td {
  padding: 0.3rem 0.18rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-table.observability-table th {
  font-size: 0.62rem;
  font-weight: 750;
  line-height: 1.1;
}

.host-table.observability-table td {
  font-size: 0.69rem;
  font-weight: 500;
  line-height: 1.25;
}

.host-table.observability-table .status-badge {
  gap: 0.2rem;
  padding: 0.14rem 0.26rem;
  font-size: 0.66rem;
}

.host-table.observability-table .status-badge-dot {
  width: 0.36rem;
  height: 0.36rem;
  flex: 0 0 auto;
}
```

`.host-status`/`.host-status-dot` 若未复用 `.status-badge`，将其标记同时使用共享 badge class，或加入等价的共享组合选择器；不得保留第二套尺寸规则。

- [ ] **Step 4: 删除模块专属排版覆盖**

从 Elasticsearch、RabbitMQ、Java、MySQL、Disk 以及 Host/Redis 相关规则中删除以下属性的页面专属声明：

- `th/td` 的 `padding`、`font-size`、`font-weight`、`line-height`；
- `status-badge` 的 `gap`、`padding`、`font-size`；
- `status-badge-dot` 的 `width`、`height`。

保留列宽、表格 `min-width`、窄屏滚动、`text-overflow`、身份/值块、颜色和 `flex` 安全属性。基础 `.status-badge` 可以继续服务总览卡等非列表场景，不要全局改小。

- [ ] **Step 5: 运行 CSS 与七页 GREEN/typecheck**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/app/theme.test.ts src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx && npm run typecheck'
```

Expected: PASS，七页原有行为契约和新共享排版契约同时成立。

- [ ] **Step 6: 经授权后创建局部提交**

```bash
git add web/src/app/theme.css web/src/app/theme.test.ts web/src/features/hosts/HostListPage.tsx web/src/features/hosts/HostListPage.test.tsx web/src/features/disks/DiskPage.tsx web/src/features/disks/DiskPage.test.tsx web/src/features/mysql/MySQLPage.tsx web/src/features/mysql/MySQLPage.test.tsx web/src/features/redis/RedisPage.tsx web/src/features/redis/RedisPage.test.tsx web/src/features/elasticsearch/ElasticsearchPage.tsx web/src/features/elasticsearch/ElasticsearchPage.test.tsx web/src/features/rabbitmq/RabbitMQPage.tsx web/src/features/rabbitmq/RabbitMQPage.test.tsx web/src/features/java/JavaPage.tsx web/src/features/java/JavaPage.test.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "style: unify observability table typography"
```

Expected: 暂存范围仅含上述 16 个文件，whitespace 检查无输出。本步骤必须在用户明确授权本地提交后执行。

---

### Task 3: 完整前端回归与交付证据

**Files:**
- Verify only; no additional product files.

- [ ] **Step 1: 运行前端全量门禁**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

Expected: 全量 Vitest、TypeScript、production build 和 Playwright 静态发现全部退出 0；不启动浏览器和 InfraView。

- [ ] **Step 2: 运行静态范围与敏感模式检查**

Run:

```bash
test "$(rg -l 'observability-table' web/src/features/{hosts,disks,mysql,redis,elasticsearch,rabbitmq,java}/*Page.tsx | wc -l)" -eq 7
rg -n 'font-size|font-weight|line-height|padding' web/src/app/theme.css
rg -n 'Token|Cookie|Authorization|X-User-Token|BaseURL|https?://' web/src/app/theme.test.ts web/src/features/{hosts,disks,mysql,redis,elasticsearch,rabbitmq,java}/*Page.test.tsx
git diff --check
git status --short
```

Expected: 七页命中数精确为 7；人工复核 CSS 命中只含共享规则或非列表控件；新增测试不含秘密或真实现场值；diff 检查无输出。`git status` 只显示本计划授权范围内的未提交变更，或在已授权提交后干净。

- [ ] **Step 3: 记录实施报告**

在执行工作流规定的报告目录记录：RED 失败名与关键断言、GREEN 命令/退出码、文件范围、未运行动态浏览器/服务、未连接上游、未提交/未推送/未部署状态。报告不得写入真实测试数据。
