# InfraView Sample Progress Freshness Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用样本推进状态替代绝对样本年龄，消除稳定链路时延导致的 Linux/MySQL 假告警，并统一主机采集状态颜色。

**Architecture:** Service 维护并发安全的进程内 `freshnessTracker`。Provider 成功加载时观察原始样本时间是否变化，页面读取本地最后推进时间计算 2/5 周期等级；Provider、API 与固定 PromQL 契约不变。

**Tech Stack:** Go 1.24、React、TypeScript、Vitest、Docker、Nightingale v8.4.1。

## Global Constraints

- InfraView 始终只读。
- 默认采集周期 `15s`，2 个周期警告、5 个周期严重。
- 不新增配置、API 字段、持久化、任意 PromQL、代理或 N+1。
- 不输出私密配置、凭据、真实资源信息、指标值或上游正文。
- 只重建既有测试 8080；不提交或推送。

---

### Task 1: 并发安全样本推进状态机

**Files:**
- Create: `internal/service/freshness.go`
- Create: `internal/service/freshness_test.go`

**Interfaces:**
- Produces: `newFreshnessTracker(clock func() time.Time, interval time.Duration) *freshnessTracker`
- Produces: `Observe(samples map[string]time.Time)`
- Produces: `Level(key string, sampleAt time.Time) Level`

- [x] **Step 1: 写失败测试**

覆盖首次旧时间正常、同一时间在 30/75 秒升级、时间推进立即正常、时间回退重建基线、未知键与零时间安全等级、并发 `Observe/Level`。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'FreshnessTracker' -count=1
```

- [x] **Step 3: 实现最小状态机**

使用 `sync.Mutex` 保护 `map[string]sampleProgress`。首次或样本时间变化时同时更新 `sampleAt/advancedAt`；等级只使用本地 `advancedAt` 与配置周期计算。

- [x] **Step 4: 运行 GREEN**

重复 Step 2，期望通过。

### Task 2: Linux 与 MySQL Service 集成

**Files:**
- Modify: `internal/service/service.go`
- Modify: `internal/service/hosts.go`
- Modify: `internal/service/overview.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/service_test.go`
- Modify: `internal/service/mysql_service_test.go`

**Interfaces:**
- Consumes: Task 1 `freshnessTracker`
- Preserves: 现有 API `collection_level` 和状态聚合

- [x] **Step 1: 写 Linux/MySQL 失败测试**

用可推进的测试 Provider 连续加载：绝对时间很旧但每轮推进时保持正常；同一时间冻结 30/75 秒分别警告/严重；恢复推进后立即正常。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'Collection.*Progress|Collection.*Freeze|Collection.*Recover' -count=1
```

- [x] **Step 3: 集成 tracker**

构造 Linux/MySQL Service 时各创建 tracker；只在缓存 loader 成功取得 Provider 数据后批量 `Observe`；列表、总览和详情按稳定 ID 调用 `Level`。

- [x] **Step 4: 运行 GREEN 与服务包全测**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -count=1
```

### Task 3: 主机采集颜色

**Files:**
- Modify: `web/src/features/hosts/HostListPage.test.tsx`
- Modify: `web/src/features/hosts/HostListPage.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Preserves: 状态文案与表格布局
- Produces: 主机采集延迟 warning 黄色、采集失联 critical 红色

- [x] **Step 1: 写失败测试**

断言“采集延迟”元素 `data-level=warning`，“采集失联”元素 `data-level=critical`，正常在线为 `data-level=normal`。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/hosts/HostListPage.test.tsx'
```

- [x] **Step 3: 实现统一等级**

`StatusText` 优先使用 `collectionLevel`，否则映射 `status`；DOM/CSS 统一从 `data-status` 改为 `data-level`。

- [x] **Step 4: 运行 GREEN**

重复 Step 2，期望通过。

### Task 4: 文档、完整验证与原 8080

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Records: 样本推进语义、重启基线、验证与部署结果

- [x] **Step 1: 同步文档**

将“绝对原始样本年龄”更新为“本地观察到的最后推进时间”，记录现场稳定链路时延根因。

- [x] **Step 2: 无缓存完整构建**

```bash
docker build --no-cache --tag infraview:sample-progress-freshness-final .
```

- [x] **Step 3: 原位重建既有测试服务**

```bash
docker compose up -d --build --force-recreate infraview
```

- [x] **Step 4: 脱敏现场验收**

确认持续推进的 Linux/MySQL 恢复正常；若再次停止测试采集器，同一原始样本时间在 2/5 周期进入警告/严重；API 非 stale。

- [x] **Step 5: 最终检查**

```bash
git diff --check
git status --short --branch
```
