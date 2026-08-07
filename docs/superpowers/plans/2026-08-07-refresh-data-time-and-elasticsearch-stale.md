# Latest Data Time and Elasticsearch Stale Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用真实夜莺样本时间替代无意义的手动刷新 UI，并避免 Elasticsearch 可选指标短暂缺失触发整页 stale。

**Architecture:** 后端保留 `meta.collected_at` JSON 契约，但由各 Service 根据快照内 `Timestamp/ReportedAt` 计算最新样本时间；前端共享 `DataTime` 组件消费该字段。Elasticsearch Provider 只对固定组数和两个 inventory 组做结构硬校验，其余组缺失按可选数据处理。

**Tech Stack:** Go 1.24、React 19、TypeScript、TanStack Query、Vitest、Docker。

## Global Constraints

- InfraView 只读展示，不新增写 API、运维按钮或任意 PromQL。
- 夜莺查询保持代码内固定、一次 batch、无 N+1。
- 不连接生产或现场数据源，不读取/输出任何敏感配置或真实数据。
- 不创建端口，不运行、部署或重建 8080。
- 仅使用容器化测试和构建；不安装宿主机依赖。
- 未获单独授权前不执行 commit、push 或 deploy。

---

### Task 1: 修复 Elasticsearch 可选结果组容错

**Files:**
- Modify: `internal/adapters/nightingale/elasticsearch_provider.go`
- Test: `internal/adapters/nightingale/elasticsearch_provider_test.go`

**Interfaces:**
- Consumes: 固定 `elasticsearchQueryCount == 26` 结果组；`elasticsearchClusterInventoryQuery` 与 `elasticsearchNodeInventoryQuery`。
- Produces: `buildElasticsearchSnapshot(results [][]instantSeries) (elasticsearch.Snapshot, error)` 对可选 `nil` 组容错，对 inventory `nil` 保持安全错误。

- [ ] **Step 1: 写失败测试**

  增加表驱动测试，将第 1–24 组逐一设为 `nil`，断言快照仍成功且 inventory 身份集合不变；保留并强化两个 inventory 组为 `nil` 的失败断言。

- [ ] **Step 2: 验证 RED**

  Run: `docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run 'Elasticsearch.*Nil|Elasticsearch.*Optional' -count=1`

  Expected: FAIL，原因是现有循环拒绝任意 `nil` 组。

- [ ] **Step 3: 最小实现**

  将全组 `nil` 循环替换为仅校验两个 inventory 组；保持组数校验及后续 merge 不变，因为遍历 `nil` slice 是安全空操作。

- [ ] **Step 4: 验证 GREEN**

  重跑 Step 2 命令，并运行 `go test ./internal/adapters/nightingale -run Elasticsearch -count=1`。

### Task 2: 让 API meta 使用真实样本时间

**Files:**
- Modify: `internal/service/types.go`
- Modify: `internal/service/service.go`
- Modify: `internal/service/metrics.go`
- Modify: `internal/service/overview.go`
- Modify: `internal/service/status.go`
- Modify: `internal/service/disk_service.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/redis_service.go`
- Modify: `internal/service/elasticsearch_service.go`
- Modify: `internal/service/rabbitmq_service.go`
- Modify: `internal/service/java_service.go`
- Test: corresponding `internal/service/*_test.go`
- Test: `internal/httpapi/*_test.go`

**Interfaces:**
- Consumes: `datasource.CurrentMetrics.Timestamp`、趋势点时间和各领域 `ReportedAt`。
- Produces: `service.Meta.CollectedAt` 为最新有效样本时间；`resultMetaAt(cache.Result, time.Time) Meta`；`mergeMeta` 取最大有效时间。

- [ ] **Step 1: 写 Service RED 测试**

  使用固定且不同的缓存时钟与样本时间，断言主机和七个模块的 `Meta.CollectedAt` 等于最大样本时间而非缓存时钟；断言 stale 再取仍保留原样本时间；空快照得到零时间。

- [ ] **Step 2: 验证 RED**

  Run: `docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run 'CollectedAt|SampleTime|DataTime' -count=1`

  Expected: FAIL，现有 meta 返回缓存 `StoredAt`。

- [ ] **Step 3: 实现通用时间归并**

  让 `resultMeta` 只表达 stale；新增 `resultMetaAt` 绑定明确样本时间；新增只接受非零时间并取 UTC 最大值的 helper；让 `mergeMeta` 对 `CollectedAt` 取最大值。

- [ ] **Step 4: 接入各 Service**

  主机当前指标、趋势和状态分别从其真实时间字段设置 meta；七个模块在从缓存取出已类型校验的快照后，遍历实体 `ReportedAt` 计算最新值。Elasticsearch 同时遍历 cluster/node；未跟踪或零时间不参与。

- [ ] **Step 5: 更新 HTTP 契约测试并验证 GREEN**

  保留 JSON 字段名 `collected_at` 与 RFC3339 编码；将旧的“等于缓存时钟”断言更新为“等于 fixture 样本时间”。运行 Service 与 HTTP 定向测试。

### Task 3: 删除手动刷新 UI 并展示共享数据时间

**Files:**
- Create: `web/src/components/DataTime.tsx`
- Create: `web/src/components/DataTime.test.tsx`
- Delete: `web/src/components/RefreshControl.tsx`
- Delete: `web/src/components/RefreshControl.test.tsx`
- Modify: `web/src/components/ListPage.tsx`
- Modify: `web/src/components/ListPage.test.tsx`
- Modify: `web/src/components/ModuleStatusCardShell.tsx`
- Modify: `web/src/components/ModuleStatusCardShell.test.tsx`
- Modify: `web/src/components/StaleBanner.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Consumes: `collectedAt?: string`。
- Produces: `DataTime({ collectedAt, className? })`；可访问文本 `最新数据时间：...`；无效/缺失显示 `暂无数据`。

- [ ] **Step 1: 写共享组件 RED 测试**

  固定测试时区，断言有效 RFC3339 时间格式化为 `YYYY/MM/DD HH:mm:ss`，无效或缺失时间显示“暂无数据”；断言列表控制区不再包含刷新按钮。

- [ ] **Step 2: 验证 RED**

  Run: `docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/components/DataTime.test.tsx src/components/ListPage.test.tsx src/components/ModuleStatusCardShell.test.tsx'`

  Expected: FAIL，`DataTime` 尚不存在且列表仍渲染按钮。

- [ ] **Step 3: 最小实现与样式**

  新增 `DataTime`；`ListPageControls` 将必填 `refresh` 改为可选时间 prop；`ModuleStatusCardShell` 在 footer 展示时间；`StaleBanner` 复用同一格式化显示；删除 refresh 专用 CSS 和移动端布局规则。

- [ ] **Step 4: 验证共享组件 GREEN**

  重跑 Step 2 命令。

### Task 4: 接入七个列表页和七张总览卡

**Files:**
- Modify: `web/src/features/hosts/HostListPage.tsx`
- Modify: `web/src/features/disks/DiskPage.tsx`
- Modify: `web/src/features/mysql/MySQLPage.tsx`
- Modify: `web/src/features/redis/RedisPage.tsx`
- Modify: `web/src/features/elasticsearch/ElasticsearchPage.tsx`
- Modify: `web/src/features/rabbitmq/RabbitMQPage.tsx`
- Modify: `web/src/features/java/JavaPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Test: corresponding page test files and `web/src/test/fixtures.ts`

**Interfaces:**
- Consumes: response `meta.collected_at`。
- Produces: 所有正常页面无手动刷新按钮；列表控制区和每张总览卡展示自己的最新数据时间；自动 query interval 和错误态 `refetch` 不变。

- [ ] **Step 1: 写页面 RED 测试**

  更新/增加断言：七个列表页找不到正常态刷新按钮与旧文案，能看到各自 fixture 时间；总览没有全局刷新控制，每张卡片内部显示对应模块时间；错误面板仍有“重试”。

- [ ] **Step 2: 验证 RED**

  Run: `docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx src/features/overview/OverviewPage.test.tsx'`

  Expected: FAIL，现有页面仍渲染手动刷新或未在卡片内展示时间。

- [ ] **Step 3: 最小接入**

  移除所有 `RefreshControl` import、props 和点击处理；传入各响应的 `meta.collected_at`。删除总览统一 `allDataUpdatedAt` 和“每 N 秒自动刷新”文案，但保留所有 `refetchInterval` 与 ErrorPanel 重试回调。

- [ ] **Step 4: 验证页面 GREEN**

  重跑 Step 2 命令，随后执行前端全量测试。

### Task 5: 文档与最终验证

**Files:**
- Modify: `docs/DESIGN.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Produces: 可恢复的时间语义、Elasticsearch 容错边界、实际验证结果和未授权边界记录。

- [ ] **Step 1: 更新持久文档**

  记录 `collected_at` 的样本语义、正常态无手动刷新、错误态保留重试、Elasticsearch 可选组容错和关键 inventory 硬失败。历史记录保持历史，不改写为当前状态。

- [ ] **Step 2: Go fresh 验证**

  先安全生成 `internal/httpapi/webdist`，再在 Go 1.24 容器执行 `gofmt` 检查、`go vet ./...`、`go test ./... -count=1`、`go test -race ./... -count=1`、`go build -o /tmp/infraview ./cmd/infraview`。

- [ ] **Step 3: 前端 fresh 验证**

  在 Node 22 容器执行 `npm ci --ignore-scripts`、`npm run test:run`、`npm run typecheck`、`npm run build`；不启动浏览器或服务。

- [ ] **Step 4: 安全与工作树核对**

  运行 `git diff --check`、`git diff --cached --check`、只读/敏感字段静态扫描、`git status --short --branch`。确认仅功能分支有改动，`main` 仍干净，未提交、未推送、未部署。

- [ ] **Step 5: 停止并汇报**

  汇报实际修改文件、RED→GREEN 证据、全量验证结果和已执行的修改命令；等待用户单独授权 commit/push/deploy。
