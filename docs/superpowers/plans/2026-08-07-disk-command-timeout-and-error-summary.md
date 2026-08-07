# 硬盘命令超时与错误摘要 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将固定指标 `smart_device_command_timeout` 安全加入现有单次 SMART batch，并把硬盘错误摘要改为非零优先、明确省略和准确缺失语义。

**Architecture:** Nightingale Provider 将固定查询从 18 条扩展为 19 条，命令超时复用 `ident+device` 和可选 serial/WWN 的既有辅助指标归并。领域、Service、HTTP 和前端逐层增加可空计数；摘要只显示非零项，状态和总览告警保持不变。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、Vitest、Docker。

## Global Constraints

- 唯一指标名为 `smart_device_command_timeout`；固定一次 19 查询 batch，无第二次请求、N+1 或任意 PromQL。
- 必须有非空 `ident+device`；可选 serial/WWN 双方非空才校验。无法匹配、身份冲突、无当前健康上报均忽略。
- 只接受有限非负值；非法、负数、较旧和同最新时间冲突按既有规则置空。
- 命令超时只展示和参与错误摘要排序，不改变设备状态、状态来源或总览告警。
- 全部缺失显示“暂无数据”；不能伪装成 0 或“未发现错误”。
- 不输出真实标签、标识、值、上游正文或任何认证信息。
- 测试仅用脱敏 fixture 和一次性容器；不得连接现场 Nightingale、启动端口或服务。提交、推送、部署和重启均需单独授权。

## File map

- Modify/Test `internal/adapters/nightingale/disk_promql.go`、`disk_provider.go`、`disk_provider_test.go`：第 19 查询与安全归并。
- Modify `internal/disk/types.go`：`ErrorCounters.CommandTimeouts *float64`。
- Modify/Test `internal/service/disk_service.go`、`disk_service_test.go`：clone 和排序。
- Modify/Test `internal/httpapi/disk_handlers.go`、`disk_handlers_test.go`：`command_timeouts` 可空 JSON。
- Modify/Test `web/src/api/types.ts`、`web/src/test/fixtures.ts`、`web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`：摘要展示。
- Modify `docs/datasources/NIGHTINGALE.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`：固定查询和交付事实。

---

### Task 1: Provider 固定第 19 查询

**Files:**
- Modify: `internal/disk/types.go`
- Modify: `internal/adapters/nightingale/disk_promql.go`
- Modify: `internal/adapters/nightingale/disk_provider.go`
- Test: `internal/adapters/nightingale/disk_provider_test.go`

**Interfaces:**

```go
type ErrorCounters struct {
	PendingSectors       *float64
	ReallocatedSectors   *float64
	UncorrectableSectors *float64
	UDMACRCErrors        *float64
	MediaIntegrityErrors *float64
	ErrorLogEntries      *float64
	CommandTimeouts      *float64
	UnsafeShutdowns      *float64
}
```

- [ ] **Step 1: 写 Provider RED**

将 `wantDiskPromQL` 精确加入 `smart_device_command_timeout`，顺序固定在异常断电之后、容量之前；`validDiskBatch` 扩为 19 组。

新增表驱动 fixture：正常值、0、负数、非法值、较旧值、同时间同值、同时间冲突、缺失 ident/device、serial/WWN 明确冲突、未知设备和没有当前 health 上报。只有匹配且有效的最新值进入 `Errors.CommandTimeouts`，序列标签不得进入领域或错误文本。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run 'DiskPromQL|SMARTSnapshot.*CommandTimeout|DiskCommandTimeout' -count=1
```

Expected: FAIL，固定查询仍为 18 条且领域字段不存在。

- [ ] **Step 3: 最小实现**

在查询枚举、`diskDeviceState`、`mergeDiskAuxiliary` 和 `finalizeDiskDevice` 增加命令超时；校验复用 `diskNonNegative`，身份和时间归并复用 `diskStateForSeries`、`mergeDiskScalar`。不得新增专属请求或宽松身份推断。

- [ ] **Step 4: GREEN/race 和 mutation**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/disk/types.go internal/adapters/nightingale/disk_promql.go internal/adapters/nightingale/disk_provider.go internal/adapters/nightingale/disk_provider_test.go && go test ./internal/adapters/nightingale -run "DiskPromQL|SMARTSnapshot.*CommandTimeout|DiskCommandTimeout" -count=1 && go test ./internal/adapters/nightingale -count=1 && go test -race ./internal/adapters/nightingale -run "Disk|SMART" -count=1'
```

临时把命令超时 validator 改为 `diskFinite`，负数专测必须 RED；立即恢复并复跑 GREEN。

- [ ] **Step 5: 经授权提交 Task 1**

仅暂存上述四文件，执行 `git diff --cached --check` 后提交 `feat: collect disk command timeouts`。

---

### Task 2: Service clone、排序和 HTTP View

**Files:**
- Modify/Test: `internal/service/disk_service.go`、`disk_service_test.go`
- Modify/Test: `internal/httpapi/disk_handlers.go`、`disk_handlers_test.go`

- [ ] **Step 1: 写 RED 测试**

Service 测试修改源或缓存克隆后的 `CommandTimeouts`，断言另一个快照不被别名污染；错误摘要排序加入 command timeout，异常断电仍明确排除。HTTP 测试断言 `errors` 精确包含 `command_timeouts`，有效值和 `null` 均正确，且没有 Go 字段名或原始 labels。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service ./internal/httpapi -run 'Disk.*CommandTimeout|Disk.*ErrorSort|DiskDevices' -count=1
```

Expected: FAIL，clone 丢字段、排序未计入或 JSON 缺键。

- [ ] **Step 3: 最小实现**

只在 `cloneDiskErrors`、`diskErrorSortValue`、`diskErrorsView`、`diskDeviceViewFrom` 增加字段；不得修改 `diskStatusAndSource`、总览计算或告警计数。

- [ ] **Step 4: GREEN/race 和 mutation**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/disk_service.go internal/service/disk_service_test.go internal/httpapi/disk_handlers.go internal/httpapi/disk_handlers_test.go && go test ./internal/service ./internal/httpapi -run "Disk.*CommandTimeout|Disk.*ErrorSort|DiskDevices" -count=1 && go test -race ./internal/service ./internal/httpapi -run "Disk.*CommandTimeout|Disk.*ErrorSort" -count=1'
```

临时从排序数组删除 command timeout，专测必须 RED；恢复后 GREEN。

- [ ] **Step 5: 经授权提交 Task 2**

仅暂存四文件并提交 `feat: expose disk command timeouts`。

---

### Task 3: 前端错误摘要准确展示

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify/Test: `web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`

**Target order:** 待处理扇区、不可修复扇区、重映射扇区、介质完整性错误、CRC错误、命令超时、错误日志、异常断电。

- [ ] **Step 1: 写 RED 测试并扩展 fixture 类型**

增加 `command_timeouts: number|null`。覆盖单项、两项、三项正文严格为前两项加 ` · …`、title 全量、全零“未发现错误”、部分缺失正文“未发现错误”且 title 提示部分暂无、全缺失“暂无数据”、异常断电累计说明、新固定优先级和中文标签。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/disks/DiskPage.test.tsx'
```

Expected: FAIL，缺字段、旧标签、无显式省略号或旧无错误文案。

- [ ] **Step 3: 最小实现**

调整 `errorItems/errorSummary`：只筛 `value > 0`；超过两项追加省略号；以 `known.length` 区分三种无非零状态；完整 title 按固定顺序。不得求和显示或改变状态徽标。

- [ ] **Step 4: GREEN/typecheck 和 mutation**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/disks/DiskPage.test.tsx && npm run typecheck'
```

临时删除省略号分支，三项测试必须 RED；恢复后 GREEN。

- [ ] **Step 5: 经授权提交 Task 3**

只提交类型、fixture、页面和测试，提交 `feat: refine disk error summaries`。

---

### Task 4: 数据源文档与完整验证

- [ ] `NIGHTINGALE.md` 将 SMART batch 精确更新为 19 条，记录命令超时身份、非负值、冲突、缺失和只展示语义。
- [ ] 更新 `PROJECT_STATUS/TODO/HANDOFF`，明确命令超时不触发状态告警且未输出现场数据。
- [ ] Go 运行 gofmt、vet、全仓普通/race、编译；前端运行全量测试、typecheck、build、Playwright `--list`；静态检查固定查询数、无写 API/N+1/敏感信息和 `git diff --check`。
- [ ] 经授权提交四份文档，提交 `docs: record disk error summary delivery`。不得推送、部署或重启。
