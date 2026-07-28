# InfraView 总览状态、刷新周期与数据源展示实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让总览零值呈现正常语义，当前页面与 Nightingale 当前指标按 15 秒刷新，并由后端返回真实数据源类型和运行时刷新周期。

**Architecture:** 数据源状态 API 作为运行时只读配置边界，返回 `type` 与 `refresh_interval_seconds`；`AppShell` 复用该查询并通过 `Outlet` 上下文向当前路由传递刷新周期。总览只改变展示语义，后端仅调整当前指标和界面周期默认值，资产与历史缓存保持 60 秒。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、React Router 7、TanStack Query 5、Vitest、Testing Library、Docker Compose。

## Global Constraints

- 始终保持只读展示，不增加 Nightingale 写操作、任意查询或代理能力。
- 不读取或输出私密 `.env` 内容、Token、认证头、Cookie 或上游响应正文。
- 所有行为修改先写失败测试并确认按预期失败，再写最小实现。
- `INFRAVIEW_REFRESH_INTERVAL=15s`、`INFRAVIEW_CURRENT_METRICS_TTL=15s`、`INFRAVIEW_INVENTORY_TTL=60s`、`INFRAVIEW_RANGE_TTL=60s`。
- 当前路由前台刷新，未挂载路由与浏览器后台不刷新，同一查询不重叠。
- 本轮不提交、不推送、不合并，重新部署后等待用户验收。

---

### Task 1: 总览正常零值语义

**Files:**
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`

**Interfaces:**
- Consumes: `OverviewAlerts`、`MetricLevel`、`StatusBadge`
- Produces: 零值正常化的主机与指标告警文案和等级

- [x] **Step 1: 写失败测试**

构造全部告警、离线和未知计数为零的总览响应，断言卡片显示“无严重、无警告、无离线、无未知”和四个“无异常”，并且这些元素的 `data-level` 均为 `normal`。

- [x] **Step 2: 运行定向测试确认失败**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/features/overview/OverviewPage.test.tsx`

Expected: FAIL，现有页面仍显示“严重 0”“警告 0”“离线 0”和指标零值详情。

- [x] **Step 3: 写最小实现**

为主机级状态徽标按计数计算等级和文案；`MetricAlert` 在合计为零时展示“无异常”，否则保留异常明细。

- [x] **Step 4: 重新运行定向测试**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/features/overview/OverviewPage.test.tsx`

Expected: PASS。

### Task 2: 数据源状态运行时契约

**Files:**
- Modify: `internal/httpapi/api_test.go`
- Modify: `internal/httpapi/query_handlers.go`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/app/AppShell.tsx`

**Interfaces:**
- Produces: `DataSourceStatusData.type: 'mock' | 'nightingale'`
- Produces: `DataSourceStatusData.refresh_interval_seconds: number`
- Produces: `AppOutletContext.refreshIntervalMs: number`

- [x] **Step 1: 写 Go API 失败测试**

使用 `DataSource=nightingale` 与 `RefreshInterval=15s` 构造 API，断言 `/api/v1/datasource/status` 返回 `"type":"nightingale"` 和 `"refresh_interval_seconds":15`。

- [x] **Step 2: 运行 Go 定向测试确认失败**

Run: `docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'go test ./internal/httpapi -run TestDataSourceStatusReturnsRuntimeConfiguration -count=1'`

Expected: FAIL，响应缺少两个字段。

- [x] **Step 3: 扩展后端视图**

从已经校验的 `a.config.DataSource` 和 `a.config.RefreshInterval` 构造只读响应字段，不改 service 或 Provider 契约。

- [x] **Step 4: 运行 Go 定向测试确认通过**

Run: `docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'go test ./internal/httpapi -run TestDataSourceStatusReturnsRuntimeConfiguration -count=1'`

Expected: PASS。

- [x] **Step 5: 写前端失败测试**

让 AppShell fixture 返回 Nightingale 和 15 秒，断言左下角显示 `Nightingale`，并在 14999ms 不刷新、15000ms 刷新。

- [x] **Step 6: 运行前端定向测试确认失败**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/app/AppShell.test.tsx`

Expected: FAIL，现有界面仍显示 `Mock` 且周期为 30 秒。

- [x] **Step 7: 扩展前端类型与 AppShell**

使用 API 返回的类型映射显示名称，把刷新秒数转换为毫秒，并通过 `<Outlet context={{ refreshIntervalMs }}>` 提供给当前页面；首次响应前使用 15 秒。

- [x] **Step 8: 重新运行前端定向测试**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/app/AppShell.test.tsx`

Expected: PASS。

### Task 3: 当前页面使用运行时 15 秒周期

**Files:**
- Create: `web/src/app/runtime.ts`
- Modify: `web/src/components/RefreshControl.test.tsx`
- Modify: `web/src/components/RefreshControl.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/hosts/HostListPage.test.tsx`
- Modify: `web/src/features/hosts/HostListPage.tsx`

**Interfaces:**
- Produces: `useRefreshIntervalMs(): number`
- Consumes: `RefreshControl.refreshIntervalSeconds: number`

- [x] **Step 1: 把现有轮询测试改为 15 秒期望并运行确认失败**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/components/RefreshControl.test.tsx src/features/overview/OverviewPage.test.tsx src/features/hosts/HostListPage.test.tsx`

Expected: FAIL，现有实现和文案固定为 30 秒。

- [x] **Step 2: 写最小运行时周期实现**

`useRefreshIntervalMs` 从 Outlet 上下文读取正数毫秒，否则返回 `15000`；总览和主机查询、页面说明及 `RefreshControl` 统一使用该值。

- [x] **Step 3: 重新运行定向测试**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run -- src/components/RefreshControl.test.tsx src/features/overview/OverviewPage.test.tsx src/features/hosts/HostListPage.test.tsx`

Expected: PASS。

### Task 4: 后端默认周期和文档配置

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/service/service_test.go`
- Modify: `internal/service/service.go`
- Modify: `.env.example`
- Modify: `docs/CONFIGURATION.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`

**Interfaces:**
- Produces: 默认 `RefreshInterval=15s`
- Produces: 默认 `CurrentMetricsTTL=15s`
- Preserves: `InventoryTTL=60s`、`RangeTTL=60s`

- [x] **Step 1: 修改默认值测试并确认失败**

Run: `docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'go test ./internal/config ./internal/service -run "TestLoadDefaults|TestServiceDefaultsCurrentMetricsTTLToFifteenSeconds" -count=1'`

Expected: FAIL，当前默认值仍为 30 秒和 20 秒。

- [x] **Step 2: 修改最小默认值**

只调整 `config.Load` 的刷新周期和当前指标 TTL 默认值，以及 `service.New` 的当前指标 TTL 防御性默认值；资产、范围和健康 TTL 不变。

- [x] **Step 3: 运行定向 Go 测试**

Run: `docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'go test ./internal/config ./internal/service -count=1'`

Expected: PASS。

- [x] **Step 4: 更新公开模板和项目文档**

将非私密默认周期、API 契约、总览零值语义、部署状态和剩余待办同步到仓库文档，不写入任何凭据。

### Task 5: 全量验证与重新部署

**Files:**
- Verify: repository and deployed Compose service; no planned source file modifications.

**Interfaces:**
- Consumes: Tasks 1–4 的全部行为
- Produces: 可在 `http://192.0.2.10:8080` 验收的容器

- [x] **Step 1: 前端完整验证**

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run test:run`

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run typecheck`

Run: `docker run --rm -v "$PWD/web:/app" -w /app node:22-bookworm npm run build`

- [x] **Step 2: Go 完整普通与竞态验证**

Run: `docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -l cmd internal | tee /tmp/gofmt-files; test ! -s /tmp/gofmt-files; go test ./...; go test -race ./...'`

- [x] **Step 3: 生产镜像构建**

Run: `docker build --tag infraview:overview-refresh-verify .`

- [x] **Step 4: 安全更新私密环境中的两个周期变量**

只把 `INFRAVIEW_REFRESH_INTERVAL` 和 `INFRAVIEW_CURRENT_METRICS_TTL` 更新为 `15s`，不打印文件内容，并保持权限为 `600`。

- [x] **Step 5: 重建当前 Compose 服务**

Run: `INFRAVIEW_ENV_FILE=/secure/path/infraview.env docker compose -p infraview up -d --build infraview`

- [x] **Step 6: 只读冒烟验证**

检查容器健康、登录、数据源状态、总览和主机列表；只报告状态码、真实数据源类型、刷新秒数和主机数量，不输出密码、Token、Cookie 或响应正文。

- [x] **Step 7: 最终差异检查**

Run: `git diff --check`

Run: `git status --short`
