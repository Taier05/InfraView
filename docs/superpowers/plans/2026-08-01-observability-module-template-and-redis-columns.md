# InfraView Observability Module Template and Redis Columns Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立共享观测列表页与总览卡模板，并把 Redis 实例页调整为语义准确的 11 列。

**Architecture:** 共享组件只负责 DOM 结构、可访问标签和既有公共 class，不引用业务类型。Redis 页面继续负责查询、URL 状态和指标格式化；Redis 总览继续负责等级计算，只把卡片骨架交给共享组件。后端 API、固定 21 查询与状态逻辑不变。

**Tech Stack:** React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Testing Library、Playwright、Docker。

## Global Constraints

- InfraView 始终只读，不新增运维操作、任意查询或代理能力。
- Redis 保持固定 21 查询、一次即时 batch、无实例 N+1。
- 不修改 Redis API、状态阈值、缓存、freshness、分页或排序白名单。
- 页面固定 11 列：实例地址、角色、内存上限、内存使用率、连接、QPS/命中率、key总数、复制链路、延迟、运行时间、状态。
- 过期/淘汰列从页面删除，但后端字段、查询和 `sort=evicted` 兼容保留。
- 开发 8080 只能连接既有测试 Nightingale且不得创建其他端口；初始计划阶段禁止重建，后续已按用户授权完成原位重建。当前长期规则为每次已授权修复验证通过后自动原位重建同一测试 8080。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文。
- 全部开发与验证使用 Docker。未经明确授权不提交、不推送、不部署、不重启。

---

### Task 1: 共享列表页展示组件

**Files:**
- Create: `web/src/components/ListPage.tsx`
- Create: `web/src/components/ListPage.test.tsx`
- Modify: `web/src/features/redis/RedisPage.tsx`

**Interfaces:**
- Produces: `ListPageHeader`、`ListPageControls`、`ListSearchField`、`ListSelectField`、`ListPageSizeField`、`ListTablePanel`。
- Consumes: 既有 `RefreshControl` 与 `host-*` 公共 class。

- [x] **Step 1: 写失败测试**

真实渲染组件并断言：可见搜索标签关联 `type="search"`；选择框标签关联；每页选项为 20/50/100 条；刷新按钮与刷新状态位于 `.host-list-controls`；表格、空状态和分页位于同一个 `.host-table-panel`。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/components/ListPage.test.tsx
```

Expected: FAIL，因为组件不存在。

- [x] **Step 3: 实现最小组件并迁移 Redis**

`ListPageControls` 始终把 `RefreshControl` 放在 children 之后；字段组件生成标准 label/input/select；`ListTablePanel` 生成滚动区、空状态与分页区。保留 Redis 搜索防抖、URL、Query 和 Table 逻辑。

- [x] **Step 4: 运行 GREEN**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/components/ListPage.test.tsx src/features/redis/RedisPage.test.tsx
```

Expected: PASS。

- [x] **Step 5: 提交（按边界跳过）**

只有获得明确授权后才执行 `git add` 与 `git commit -m "refactor: add shared observability list template"`；当前跳过。

---

### Task 2: 共享总览模块卡壳

**Files:**
- Create: `web/src/components/ModuleStatusCardShell.tsx`
- Create: `web/src/components/ModuleStatusCardShell.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Test: `web/src/features/overview/OverviewPage.test.tsx`

**Interfaces:**
- Produces: `ModuleStatusCardShell`，props 为 `to`、`ariaLabel`、`category`、`title`、`level`、`levelLabel`、`actionLabel`、`emptyState?`、`children?`。
- Consumes: 既有 `.module-status-*` class；不计算业务等级。

- [x] **Step 1: 写失败测试**

锁定 normal 与 empty 渲染：链接、可访问标签、类别、标题、等级、children 和底部入口正确；empty 显示空状态且不渲染指标 children。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/components/ModuleStatusCardShell.test.tsx
```

Expected: FAIL，因为组件不存在。

- [x] **Step 3: 实现卡壳并仅迁移 RedisStatusCard**

组件只生成 Link、heading、empty/children 和 footer。Redis 继续计算等级与告警摘要；Linux、硬盘、MySQL 本轮不迁移。

- [x] **Step 4: 运行 GREEN**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/components/ModuleStatusCardShell.test.tsx src/features/overview/OverviewPage.test.tsx
```

Expected: PASS，Redis normal/warning/critical/empty 与只读入口保持不变。

- [x] **Step 5: 提交（按边界跳过）**

只有获得明确授权后才执行 `git add` 与 `git commit -m "refactor: add shared overview module card shell"`；当前跳过。

---

### Task 3: Redis 11 列与指标格式

**Files:**
- Modify: `web/src/features/redis/RedisPage.tsx`
- Modify: `web/src/features/redis/RedisPage.test.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/infraview.spec.ts`

**Interfaces:**
- Consumes: `max_memory_bytes`、`memory_usage_percent`、`role`、`master_link_up`、`worst_replica_lag_seconds`、`uptime_seconds`。
- Produces: 11 列 UI，以及 `memoryLimit`、`replicationLink`、`replicationLag`、`uptime` 展示函数。

- [x] **Step 1: 写批准语义的失败测试**

精确锁定 11 列。覆盖：有效/零/null 上限；独立使用率；主节点链路 `—`；从节点正常/断开/未知；lag 存在/缺失；90000 秒显示 25 小时；页面无过期/淘汰表头。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/features/redis/RedisPage.test.tsx
```

Expected: FAIL，旧页面仍是合并列和天/小时格式。

- [x] **Step 3: 实现最小 11 列与 72rem 布局**

删除页面 `evicted` column；拆分 memory 和 replication column；保留 `sortFields` 的 `evicted` 兼容。延迟不读取 `master_last_io_seconds_ago`；运行时间最终按 Task 5 更正为主机/MySQL 的天/小时格式。为 11 列分配明确宽度，只允许表格区域横向滚动。

- [x] **Step 4: 同步 Playwright 规格并运行 GREEN**

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace/web node:22-alpine npm test -- --run src/features/redis/RedisPage.test.tsx src/features/overview/OverviewPage.test.tsx
```

Expected: PASS；浏览器静态规格断言 11 列、不存在过期/淘汰列，并保留刷新与只读边界。

- [x] **Step 5: 提交（按边界跳过）**

只有获得明确授权后才执行 `git add` 与 `git commit -m "feat: refine Redis memory and replication columns"`；当前跳过。

---

### Task 4: 规则落库与全量验证

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: 两份 Redis/模板规格与本计划

**Interfaces:**
- Produces: 后续模块必须复用列表页与总览卡模板的持久恢复规则。

- [x] **Step 1: 更新架构、设计、状态和交接文档**

记录共享组件职责、业务/展示边界、Redis 首个接入者、旧模块渐进迁移、11 列语义、验证结果和未执行边界。不得记录现场值。

- [x] **Step 2: 前端全量离线验证**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build && npm run e2e:run -- --list'
```

Expected: 全部退出 0，不启动服务或端口。

- [x] **Step 3: Go 全仓离线回归**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'unformatted=$(gofmt -l $(find cmd internal -type f -name "*.go")); test -z "$unformatted" && go test ./... && go test -race ./... && go build -o /tmp/infraview ./cmd/infraview'
```

Expected: 全部退出 0。

- [x] **Step 4: 无缓存生产镜像**

```bash
docker build --no-cache --tag infraview:redis-shared-template-verify .
```

Expected: 构建退出 0；镜像不运行、不映射端口、不连接上游。

- [ ] **Step 5: 最终差异检查**

```bash
git diff --check
git status --short --branch
```

Expected: main 仍跟踪 origin/main，只保留有意的 Redis、模板和文档差异。

- [x] **Step 6: 提交、推送和现有 8080 重建边界**

十一列/共享模板及后续运行时间纠错均已获授权并原位重建现有测试 8080；用户随后明确授权提交和推送，功能提交 `c3b5c7d` 已进入 `origin/main`。后续已授权修复验证通过后自动重建同一测试 8080，未来提交和推送仍需明确授权。

---

### Task 5: Redis 运行时间与主机/MySQL 对齐

**Files:**
- Modify: `web/src/features/redis/RedisPage.test.tsx`
- Modify: `web/src/features/redis/RedisPage.tsx`
- Modify: `docs/DESIGN.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/superpowers/specs/2026-08-01-redis-cluster-module-design.md`
- Modify: `docs/superpowers/specs/2026-08-01-observability-module-template-and-redis-columns-design.md`

**Interfaces:**
- Consumes: `uptime_seconds: number | null`。
- Produces: 与主机/MySQL 相同的 `x天 x小时`、`x天`、`x小时` 或“暂无数据”。

- [x] **Step 1: 写失败测试并运行 RED**

将 Redis 夹具的 `90000` 秒期望从 `25小时` 改为手工推导的 `1天 1小时`；保留 `3600` 秒为 `1小时`、`1800` 秒为 `0小时`、`null` 为“暂无数据”。运行 Redis 页面测试，预期只因旧累计小时实现得到 `25小时` 而失败。

- [x] **Step 2: 最小修复并运行 GREEN**

Redis `uptime` 使用与主机/MySQL 相同的日/小时分支：天数为 `Math.floor(seconds / 86400)`，小时为剩余秒数除以 `3600` 后向下取整。运行 Redis 页面测试和前端全量测试。

- [x] **Step 3: 同步持久文档并检查差异**

把“累计整小时”更正为“与主机/MySQL 相同的天/小时格式”，记录 RED→GREEN 与部署结果；运行 `git diff --check`。本步骤按当时边界仅原位重建同一测试 8080；用户随后另行授权提交和推送，最终随功能提交 `c3b5c7d` 交付。
