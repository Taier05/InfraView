# InfraView Raw Sample Freshness Fix Implementation Plan

> 2026-07-30 补充：本计划完成的 Provider 原始时间契约继续保留；直接比较绝对样本年龄的 Service 逻辑已由 `2026-07-30-sample-progress-freshness-fix.md` 替代。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用原始最后样本时间修复主机假正常和 MySQL 延迟告警，同时保持一次固定 batch 与只读边界。

**Architecture:** Nightingale Provider 通过固定 `tlast_over_time(...[24h])` 查询取得原始最后样本 Unix 时间；Linux `CurrentMetrics.Timestamp` 与 MySQL `ReportedAt` 只由该值设置。Service 不再用 Target 时间或当前查询外层时间掩盖停采。

**Tech Stack:** Go 1.24、MetricsQL、Nightingale v8.4.1、Docker。

## Global Constraints

- InfraView 始终只读。
- 默认采集周期 `15s`，2 个周期警告、5 个周期严重。
- MySQL 保持固定 16 条即时查询、一次 batch、无 N+1。
- 不增加前端任意查询、上游代理、数据库连接或写接口。
- 不输出私密配置、凭据、真实资源信息、指标值或上游正文。
- 未经单独授权不提交或推送；当前授权包含修复后原位重建既有测试 8080。

---

### Task 1: Linux 原始心跳时间

**Files:**
- Modify: `internal/adapters/nightingale/promql.go`
- Modify: `internal/adapters/nightingale/provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/service/service.go`
- Modify: `internal/service/service_test.go`

**Interfaces:**
- Produces: `currentPromQL` 最后一条固定查询返回原始 `system_uptime` 最后样本时间。
- Produces: `datasource.CurrentMetrics.Timestamp` 只表示原始采样时间。

- [x] **Step 1: 写 Provider 与 Service 失败测试**

测试锁定：固定 batch 新增 `tlast_over_time(system_uptime{...}[24h])`；外层时间新鲜但查询数值过期时 `CurrentMetrics.Timestamp` 使用数值；Target 时间新鲜且心跳缺失/过期时最终状态仍为严重。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/service -run 'CurrentMetrics|Collection' -count=1
```

- [x] **Step 3: 实现 Provider 和 Service 最小修复**

`GetCurrentMetrics` 对心跳索引使用 `parseUnixTime(sample.Value[1])`；其他查询只映射指标值。`hostCollectionLevel` 对零心跳返回 `LevelCritical`，否则按原始样本时间计算，不再调用 `latestTime`。

- [x] **Step 4: 格式化并确认 GREEN**

运行 Step 2 同一测试命令，期望通过。

### Task 2: MySQL 原始最后样本时间

**Files:**
- Modify: `internal/adapters/nightingale/mysql_promql.go`
- Modify: `internal/adapters/nightingale/mysql_provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/mysql_service_test.go`

**Interfaces:**
- Preserves: 16 条固定即时查询和索引 15 的近期身份职责。
- Produces: `mysql.Instance.ReportedAt` 为 `tlast_over_time(mysql_up[24h])` 的数值时间。

- [x] **Step 1: 写 MySQL 失败测试**

测试锁定：索引 15 查询为 `tlast_over_time(mysql_up[24h])`；外层时间新鲜但数值过期时 `ReportedAt` 使用数值；当前 `mysql_up` 仍存在也按过期 `ReportedAt` 进入警告/严重；非法原始时间安全失败。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/service -run 'MySQL' -count=1
```

- [x] **Step 3: 实现 MySQL 最小修复**

Provider 在近期身份循环中解析 `series.Value[1]` 为原始时间并设置 `ReportedAt`；当前 `mysql_up` 只设置 `Reporting` 与可用性。Service 对所有受跟踪实例统一使用 `ReportedAt` 计算采集等级。

- [x] **Step 4: 格式化并确认 GREEN**

运行 Step 2 同一测试命令，期望通过。

### Task 3: 文档、完整验证与测试 8080

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Records: 原始样本时间口径、MetricsQL 依赖、验证结果与部署状态。

- [x] **Step 1: 同步文档**

记录主机不再使用 Target 时间、MySQL 查询数不变、`tlast_over_time` 语义与现场根因。

- [x] **Step 2: 无缓存完整构建**

```bash
docker build --no-cache --tag infraview:raw-sample-freshness-verify .
```

- [x] **Step 3: 原位重建既有测试服务**

```bash
docker compose up -d --build --force-recreate infraview
```

只允许既有原 8080，重建前后确认服务数量、端口、Nightingale 模式和健康状态，不输出配置或响应正文。

- [x] **Step 4: 现场只读验收**

保持当前测试采集器关闭，登录应用 API 后只输出脱敏状态结论：目标主机和对应 MySQL 均为严重，总览均出现严重；响应非 stale。

- [x] **Step 5: 最终检查**

```bash
git diff --check
git status --short --branch
```

停在未提交、未推送状态。
