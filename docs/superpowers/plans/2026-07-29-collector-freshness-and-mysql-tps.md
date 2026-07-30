# InfraView Collector Freshness and MySQL TPS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Linux 主机和 MySQL 实例在采集停止后按 2/5 个采集周期进入警告/严重，并完成 MySQL TPS、地址排序、表格与总览精简。

**Architecture:** Service 使用统一采集周期计算新鲜度；Linux 复用 Target 与指标时间，MySQL Provider 在同一次固定 batch 中同时读取当前 `mysql_up` 和 24 小时实例发现结果。API 只增加派生状态与 TPS 字段，前端保持紧凑只读展示。

**Tech Stack:** Go 1.24、React 19、TypeScript、Vitest、Playwright、Docker、Nightingale v8.4.1 固定 batch API。

## Global Constraints

- 始终使用简体中文文案和文档。
- InfraView 只展示数据，不执行任何运维操作。
- Nightingale v8.4.1 是主要开发与验证版本，v9.x 保留协议兼容。
- 不增加任意 PromQL、任意代理、SSH、远程命令、MySQL 连接或写接口。
- 不输出或提交 `.env`、Token、Cookie、认证头、真实标识、IP、资源数量、指标值或上游正文。
- MySQL 只允许 16 条代码内置查询通过一次即时 batch 完成，禁止实例 N+1。
- 警告阈值固定为 2 个预期采集周期，严重阈值固定为 5 个周期；默认周期 `15s`。
- 未经单独授权，不部署、重启、提交或推送。

---

### Task 1: 统一采集新鲜度与配置

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/service/types.go`
- Modify: `internal/service/service.go`
- Modify: `internal/service/hosts.go`
- Modify: `internal/service/overview.go`
- Modify: `internal/service/service_test.go`
- Modify: `internal/adapters/nightingale/provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `cmd/infraview/main.go`

**Interfaces:**
- Produces: `Config.ExpectedCollectionInterval time.Duration`
- Produces: `service.CollectionLevel(sampleAt time.Time) Level`
- Produces: `HostSummary.CollectionLevel Level` 与 `HostDetail.CollectionLevel Level`

- [x] **Step 1: 写配置、时间戳和主机 2/5 周期失败测试**

测试必须断言：默认周期为 `15s`；非法或非正周期拒绝；全部当前指标缺失时 Timestamp 为零；年龄达到 2 周期为 warning，达到 5 周期为 critical；Host 列表与总览使用派生状态。

- [x] **Step 2: 运行聚焦 Go 测试并确认按缺失行为失败**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/config ./internal/adapters/nightingale ./internal/service -count=1
```

- [x] **Step 3: 实现最小新鲜度逻辑**

`collectionLevel` 使用 `Clock().UTC()` 与样本时间比较；零时间为 unknown，达到 `5*interval` 为 critical，达到 `2*interval` 为 warning，其余 normal。主机使用 Target 与指标时间的较新值，warning 派生为 unknown，critical 派生为 offline。

- [x] **Step 4: 格式化并确认聚焦测试 GREEN**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w cmd/infraview/main.go internal/config internal/service internal/adapters/nightingale
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/config ./internal/adapters/nightingale ./internal/service -count=1
```

### Task 2: MySQL 近期实例保留与 TPS

**Files:**
- Modify: `internal/mysql/types.go`
- Modify: `internal/adapters/nightingale/mysql_promql.go`
- Modify: `internal/adapters/nightingale/mysql_provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/adapters/nightingale/testdata/mysql-instant-batch.json`
- Modify: `internal/adapters/mock/mysql_provider.go`
- Modify: `internal/service/mysql_types.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/mysql_service_test.go`

**Interfaces:**
- Produces: `mysql.Instance.ReportedAt`, `mysql.Instance.Reporting`, `mysql.Instance.TPS`
- Produces: `MySQLInstanceSummary.TPS` 与 `MySQLInstanceSummary.CollectionLevel`
- Consumes: 固定查询索引 0 当前 `mysql_up`、11 TPS、15 近期实例发现。

- [x] **Step 1: 写 16 查询、实例保留、TPS 与新鲜度失败测试**

测试必须断言：查询顺序固定；当前实例有真实样本时间；仅近期发现的实例保留但即时指标为空；TPS 按身份归并；warning/critical 精确覆盖 2/5 周期边界。

- [x] **Step 2: 运行 Nightingale/MySQL Service 聚焦测试确认 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/service -run 'MySQL' -count=1
```

- [x] **Step 3: 实现 16 条单 batch 归并**

用当前 `mysql_up` 建立 reporting 实例，用 `max_over_time(mysql_up[24h])` 补充仅历史实例。历史实例保持 AvailabilityUnknown、Reporting=false、即时数值为空；TPS 作为非负标量归并。

- [x] **Step 4: 实现 Service 新鲜度与总览计数**

MySQL 最终状态取可用性、复制和采集新鲜度最高等级；采集失联计入可用性告警。克隆路径必须完整复制 TPS、时间和 reporting 状态。

- [x] **Step 5: 格式化并确认聚焦测试 GREEN**

运行 Task 2 Step 2 的同一命令，期望全部通过。

### Task 3: API 与紧凑页面

**Files:**
- Modify: `internal/httpapi/query_handlers.go`
- Modify: `internal/httpapi/mysql_handlers.go`
- Modify: `internal/httpapi/api_test.go`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/features/hosts/HostListPage.tsx`
- Modify: `web/src/features/hosts/HostListPage.test.tsx`
- Modify: `web/src/features/mysql/MySQLPage.tsx`
- Modify: `web/src/features/mysql/MySQLPage.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/infraview.spec.ts`

**Interfaces:**
- Produces: Host/MySQL JSON `collection_level`
- Produces: MySQL JSON `tps`
- Preserves: MySQL JSON `host` 兼容字段与 URL `sort=instance`

- [x] **Step 1: 写 API 和前端失败测试**

覆盖新增字段、主机/MySQL 采集文案、地址排序、地址唯一搜索、“QPS / TPS”展示、10 列表头和总览底部无重复状态徽标。

- [x] **Step 2: 运行聚焦测试确认 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi -count=1
docker run --rm -v "$PWD/web:/app" -w /app node:24-bookworm \
  npm test -- --run src/features/hosts/HostListPage.test.tsx \
    src/features/mysql/MySQLPage.test.tsx \
    src/features/overview/OverviewPage.test.tsx
```

- [x] **Step 3: 实现 API 映射与页面最小变更**

MySQL 搜索仅匹配 Address；`sort=instance` 比较 Address。页面删除所属主机列，把 QPS/TPS 合并显示，采集等级覆盖状态文案；总览非空卡底部只保留查看入口。

- [x] **Step 4: 确认 API 与前端聚焦测试 GREEN**

运行 Task 3 Step 2 的同一组命令，期望全部通过。

### Task 4: 文档与完整验证

**Files:**
- Modify: `.env.example`
- Modify: `README.md`
- Modify: `docs/CONFIGURATION.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Consumes: Tasks 1–3 的最终实现。
- Produces: 可恢复的配置、查询、状态与验证记录。

- [x] **Step 1: 更新配置、设计、状态、TODO 与交接文档**

记录默认采集周期、2/5 周期、16 条固定查询、24 小时发现、TPS 语义、10 列表格和未部署状态。

- [x] **Step 2: 执行完整容器验证**

```bash
docker build --no-cache --tag infraview:collector-freshness-verify .
```

构建必须包含前端 Vitest、typecheck、production build、Go 普通/race 测试和编译。

- [x] **Step 3: 执行差异与敏感模式检查**

```bash
git diff --check
git status --short
git diff --stat
```

人工检查新增内容不含真实环境信息，并核对所有需求均有测试覆盖。

- [x] **Step 4: 停在未提交、未部署状态**

向用户报告修改文件、RED→GREEN 证据、完整验证结果及未执行事项；等待部署或提交授权。
