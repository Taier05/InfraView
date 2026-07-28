# InfraView 数据连接汇总实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将侧边栏永久展开的单数据源卡片改为紧凑的“数据连接”汇总入口，保留当前 Nightingale/Mock 状态可见性，并为后续日志等多数据源明细预留展示结构。

**Architecture:** 保持现有 `/api/v1/datasource/status` 和 15 秒刷新契约不变，只在 `AppShell` 的展示边界把当前响应呈现为一条“指标”连接。使用原生 `details/summary` 提供可访问的渐进展开：汇总行始终展示整体状态，详情区展示类型、健康结果和检查时间。

**Tech Stack:** React 19、TypeScript 5.8、TanStack Query、Testing Library、Vitest、Playwright、CSS。

## Global Constraints

- 始终保持只读展示，不增加数据源写操作、任意查询或代理能力。
- 健康 Nightingale 汇总显示 `1/1 正常`；Mock 明确显示 `包含 Mock 数据`。
- 状态过期、数据源异常和首次请求失败必须在未展开时即可辨识。
- 详情使用“指标”类别描述当前连接，不假装日志数据源已经接入。
- 继续由现有状态接口提供页面刷新周期，查询周期和后台标签页行为不变。
- 不读取或输出 `/secure/path/infraview.env`、Nightingale Token、认证信息或上游响应正文。

---

### Task 1: 数据连接汇总交互与样式

**Files:**
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/infraview.spec.ts`

**Interfaces:**
- Consumes: `DataSourceStatusResponse`、`datasourceState`、`refreshIntervalMilliseconds(...)`。
- Produces: 侧边栏 `details[aria-label="数据连接汇总"]`，汇总状态文案和当前“指标”连接详情。

- [x] **Step 1: 写健康与展开行为的失败测试**

在 `AppShell.test.tsx` 中将旧卡片断言改为：健康 Nightingale 时能看到 `数据连接` 和 `1/1 正常`，`details` 默认没有 `open` 属性；点击其 `summary` 后能看到 `指标`、`Nightingale`、`健康` 和带原时间值的 `time`。

- [x] **Step 2: 写异常和 Mock 汇总语义的失败测试**

分别断言过期状态的汇总文案为 `状态过期`、首次请求失败为 `状态获取失败`；新增 Mock 用例，断言未展开的汇总行显示 `包含 Mock 数据`。

- [x] **Step 3: 运行测试并确认 RED**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/app/AppShell.test.tsx
```

Expected: FAIL；现有组件仍暴露 `数据源状态`，也没有 `数据连接` 汇总和可展开详情。

- [x] **Step 4: 实现最小汇总结构**

在 `AppShell.tsx` 中保留现有查询和刷新周期计算，把状态展示替换为：

```tsx
<details
  className="source-status"
  aria-label="数据连接汇总"
  data-state={datasourceState}
  data-mode={datasourceData?.data.type ?? 'unknown'}
>
  <summary>
    <span className="status-dot" aria-hidden="true" />
    <span className="connection-summary-title">数据连接</span>
    <span className="connection-summary-state">{connectionSummary}</span>
  </summary>
  <div className="connection-details">
    <div className="connection-row">
      <span>指标</span>
      <strong>{datasourceName}</strong>
      <span>{datasourceLabel}</span>
    </div>
    {/* 有响应时继续展示旧结果提示和最近检查时间。 */}
  </div>
</details>
```

汇总映射固定为：加载中 `正在检查`、请求错误 `状态获取失败`、过期 `状态过期`、健康 Mock `包含 Mock 数据`、健康 Nightingale `1/1 正常`、不健康 `1 个连接异常`。

- [x] **Step 5: 实现紧凑和状态化样式**

让 `summary` 成为单行布局；健康为绿色、Mock 和过期为黄色、异常和请求失败为红色。详情区通过顶部分隔线展示连接行和检查时间，并保持移动端现有布局兼容。

- [x] **Step 6: 运行聚焦测试并确认 GREEN**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/app/AppShell.test.tsx
```

Expected: AppShell 全部测试通过。

- [x] **Step 7: 补充端到端关键路径断言**

隔离端到端环境固定使用 Mock：登录后断言 `数据连接汇总` 可见且包含 `包含 Mock 数据`；点击“数据连接”后断言 `指标`、`Mock` 和 `健康` 明细可见。真实部署后的冒烟检查再验证 Nightingale。

- [x] **Step 8: 执行前端全量验证**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run
docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run typecheck
docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run build
git diff --check
```

Expected: 单元测试、类型检查、生产构建全部通过，`git diff --check` 无输出。

- [x] **Step 9: 构建部署并做只读冒烟检查**

使用现有 Compose 配置构建并更新 InfraView 服务；检查容器健康、登录和数据源状态，只报告状态码、数据源类型和页面汇总，不输出密码、Token、Cookie 或响应正文。
