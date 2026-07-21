# Task 6 实施报告

## 状态

完成。已实现基础设施总览页面及共享 `MetricCard`、`StatusBadge`、`TimeRangeSelector`、`TrendChart`、`StaleBanner`、`ErrorPanel`，并接入应用首页。

最终提交将在本报告写入后创建；提交信息为 `feat: add infrastructure overview`，准确提交哈希见任务最终交付。

## 契约对齐

实现前逐字段读取并核对：

- `internal/service/types.go` 的 `Overview`、`MetricValue`、`Level`、`Meta`。
- `internal/service/overview.go` 的总览聚合与服务端阈值计算。
- `internal/httpapi/query_handlers.go` 的 `overviewView`、`metricValueView` 及 JSON tag。
- `internal/httpapi/respond.go` 的统一 `{data,meta}` 与错误响应。

前端严格使用实际 JSON：

- `data.total|online|offline|unknown`。
- `data.cpu_average|memory_average = {value: number|null, level}`。
- `level = normal|warning|critical|unknown`。
- `meta.request_id|stale|collected_at?`。

浏览器未重算 80/90 阈值。测试特意让 `73.5` 对应服务端 `critical`、`42` 对应服务端 `warning`，页面仍展示“危险/警告”，证明权威 level 直接来自服务端。

## TDD RED / GREEN

### 第一轮 RED

先创建完整 overview 夹具和页面行为测试，再执行：

```bash
npm run test:run -- src/features/overview/OverviewPage.test.tsx
```

结果按预期失败：Vite 无法解析尚不存在的 `../../components/TrendChart`；失败原因是功能缺失，不是测试拼写或环境错误。

初始测试覆盖：

- 四张主指标卡。
- 默认 `24小时` 与 `1小时/6小时/24小时/7天` 四范围。
- 范围切换取消旧请求。
- 每 30 秒刷新，pending 请求不重叠。
- stale 提示和精确 `collected_at`。
- `null` 展示“暂无数据”而不是 0。
- retryable 中文错误和重试；不可重试错误不显示按钮。
- 状态文字与颜色语义同时存在。
- TrendChart 的真实 DOM 读屏摘要，不断言图表 mock。

### GREEN 与测试环境诊断

最小实现后，30 秒无重叠和读屏摘要先通过，其余 MSW 请求落入 network error。实际捕获的底层异常是：

```text
TypeError: RequestInit: Expected signal ("AbortSignal {}") to be an instance of AbortSignal.
```

根因是当前 Node undici 不接受 jsdom realm 的 `AbortSignal`，不是生产 URL、DTO 或取消逻辑错误。最终 overview 测试在 fetch 边界使用完整响应 stub，并直接监听实际传入的 `init.signal`：范围由 `1h` 切到 `6h` 时断言旧 `1h` signal 收到 abort。没有移除生产 signal，也没有 mock ECharts 组件或断言 mock 元素。

聚焦 GREEN：

```bash
npm run test:run -- src/features/overview/OverviewPage.test.tsx
```

当时 9/9 通过。

### 第二轮 RED / GREEN：唯一真实控制

自审发现 Task 5 壳层仍有未接线范围下拉框，包含无效 `30d` 且缺少 `6h`，与 Overview 重复。先新增应用级测试，再执行：

```bash
npm run test:run -- src/auth/LoginPage.test.tsx -t "四档时间范围"
```

RED 明确找到不应存在的 combobox。随后新增手动刷新行为测试并执行：

```bash
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "手动刷新"
```

RED 明确找不到“刷新”按钮。

最小修复把四档范围和刷新按钮放入 Overview 的 `总览控制` group；刷新调用当前 query 的 `refetch()`；AppShell 删除静态范围和全局刷新占位。随后实际执行：

```bash
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "手动刷新"
npm run test:run -- src/auth/LoginPage.test.tsx -t "四档时间范围"
```

两项均 GREEN。最终应用只有一套可访问、功能真实的范围/刷新控件。

## 实现摘要

- Query key 为 `['overview', range]`。
- 默认范围 `24h`；四个按钮使用 `aria-pressed`。
- `refetchInterval: 30000`，`refetchIntervalInBackground: false`。
- query function 将 TanStack Query signal 传入 `apiRequest`，范围变化会取消失去观察者的旧请求。
- 手动刷新只重取当前范围；请求进行时按钮禁用。
- 四卡为主机总数、在线主机、CPU 平均使用率、内存平均使用率；总数卡同时显示在线/离线/未知计数。
- `null` 值显示“暂无数据”；百分比最多保留一位小数。
- stale banner 使用 `<time dateTime>` 展示服务端原始精确采集时间。
- ErrorPanel 仅对 retryable 错误显示“重试”。
- 所有状态除颜色外均显示“在线/离线/未知/正常/警告/危险”等文字。
- 保持既有低饱和深色中文视觉基线，并补充 4/2/1 列响应式卡片布局。

## ECharts 6.1.0 兼容性

实际检查 `echarts/core`、`echarts/charts`、`echarts/components`、`echarts/renderers` 导出后确认：ECharts 6.1.0 没有独立 `TimeAxisComponent`；时间轴由坐标轴核心能力通过 `xAxis.type = 'time'` 使用。

最终只模块化注册：

```ts
use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])
```

没有导入完整 `echarts` 包、没有注册 SVG/Bar/Pie/DataZoom/Aria 等未使用模块，也没有降级。读屏信息由 `.sr-only` 真实 DOM 摘要提供；canvas 本身 `aria-hidden`，用户无需观察颜色或画布即可理解摘要。

## 完整验证与审计

最终实际执行：

```bash
npm run test:run
npm run typecheck
npm run build
npm audit --omit=dev
npm audit
```

结果：

- unit：3 个测试文件，22/22 通过，输出干净。
- typecheck：`tsc --noEmit` 退出 0。
- build：Vite 7.3.6 成功，95 个模块转换，`dist/index.html` 生成；JS 266.20 kB（gzip 85.06 kB）。
- 生产审计：`found 0 vulnerabilities`。
- 全量审计：`found 0 vulnerabilities`。

构建仍显示 React Router/TanStack Query 依赖顶层 `"use client"` 被 Vite/Rollup 忽略的既有非阻断提示；构建退出 0 且产物成功。

## 实际执行命令记录

源码与报告修改全部通过 `apply_patch` 完成；没有使用 shell 重定向、`cat` 或脚本写文件。与实现、诊断、验证直接相关的 shell 命令按执行顺序如下（重复验证如实保留）：

```bash
npm run test:run -- src/features/overview/OverviewPage.test.tsx
npm run test:run -- src/features/overview/OverviewPage.test.tsx
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "展示四张" --reporter=verbose
npm run test:run -- src/auth/LoginPage.test.tsx -t "登录后进入" --reporter=verbose
node --input-type=module - <<'NODE' # 使用 setupServer+实际 handlers 诊断 overview 匹配
node --input-type=module - <<'NODE' # 比较 string/URL/Request 三种 fetch 输入
node --input-type=module - <<'NODE' # 使用 handlers+wildcard 复核请求 URL
npm run test:run -- src/features/overview/OverviewPage.test.tsx
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "展示四张" --reporter=verbose
npm run test:run -- src/features/overview/OverviewPage.test.tsx
npm run test:run
npm run typecheck
npm run test:run
npm run build
npm audit --omit=dev
npm audit
npm run test:run -- src/auth/LoginPage.test.tsx -t "四档时间范围"
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "手动刷新"
npm run test:run -- src/features/overview/OverviewPage.test.tsx -t "手动刷新"
npm run test:run -- src/auth/LoginPage.test.tsx -t "四档时间范围"
git status --short
git diff --stat
git diff --check
git diff -- web/src/api/types.ts web/src/components web/src/features/overview web/src/app/App.tsx web/src/app/AppShell.tsx web/src/app/theme.css web/src/auth/LoginPage.test.tsx web/src/test/fixtures.ts
rg -n "^(<<<<<<<|=======|>>>>>>>)" web/src
rg -n "from 'echarts|from \"echarts" web/src
npm run test:run
npm run typecheck
npm run build
npm audit --omit=dev
npm audit
```

命令前的只读 schema/文档检查较多，未逐条重复在本节；它们没有修改工作区。诊断命令仅打印运行时信息，没有写文件。

## 文件

- `.superpowers/sdd/task-6-report.md`
- `web/src/api/types.ts`
- `web/src/app/App.tsx`
- `web/src/app/AppShell.tsx`
- `web/src/app/theme.css`
- `web/src/auth/LoginPage.test.tsx`
- `web/src/components/ErrorPanel.tsx`
- `web/src/components/MetricCard.tsx`
- `web/src/components/StaleBanner.tsx`
- `web/src/components/StatusBadge.tsx`
- `web/src/components/TimeRangeSelector.tsx`
- `web/src/components/TrendChart.tsx`
- `web/src/features/overview/OverviewPage.tsx`
- `web/src/features/overview/OverviewPage.test.tsx`
- `web/src/test/fixtures.ts`

## 提交与自审

首次执行：

```bash
git diff --check && git add web/src .superpowers/sdd/task-6-report.md && git diff --cached --check && git diff --cached --name-status && git diff --cached --stat && git diff --cached -- web/src/components/TrendChart.tsx web/src/features/overview/OverviewPage.tsx web/src/features/overview/OverviewPage.test.tsx web/src/app/AppShell.tsx
```

`git diff --check` 通过，`web/src` 已暂存；但 `.superpowers` 被 `.gitignore` 忽略，普通 `git add` 拒绝报告文件，后续检查与提交没有执行。随后用 `git status --short` 和 `git diff --cached --name-status` 确认只有 Task 6 的 `web/src` 文件已暂存。

由于用户明确要求生成并交付这个单一报告文件，最终仅对该精确路径使用 `-f`，没有强制加入整个忽略目录。实际继续按顺序执行：

```bash
git add -f .superpowers/sdd/task-6-report.md
git diff --cached --check
git diff --cached --name-status
git diff --cached --stat
git diff --cached -- web/src/components/TrendChart.tsx web/src/features/overview/OverviewPage.tsx web/src/features/overview/OverviewPage.test.tsx web/src/app/AppShell.tsx
git add web/src/features/overview/OverviewPage.test.tsx
git add -f .superpowers/sdd/task-6-report.md
git diff --cached --check
git commit -m "feat: add infrastructure overview"
git status --short
```

关键 diff 审查时发现测试 helper 末尾有一行无意义视觉空白，使用 `apply_patch` 清理后重新暂存测试和更新后的报告。检查目标：暂存区只包含本报告和 Task 6 的 `web/src` 文件；无空白错误、冲突标记、构建产物、依赖目录或越界的主机/Docker 实现。

## 顾虑与未执行事项

- `/api/v1/overview` 当前只返回当前聚合快照，不返回历史序列；因此 TrendChart 作为已验证的共享组件提供给后续历史指标页面使用，没有在总览中伪造浏览器历史数据。
- jsdom/MSW 的跨 realm AbortSignal 不兼容只影响当前 Node 测试边界；生产浏览器取消语义保留并由 fetch stub 行为测试覆盖。
- 未运行 Playwright E2E：Task 6 简报要求 unit/typecheck/build/audit，没有新增 E2E 文件。
- 未执行 `npm audit fix`、`npm audit fix --force`、依赖安装/卸载/降级、服务重启、Docker、主机列表/详情或数据删除。
