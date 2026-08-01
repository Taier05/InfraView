# InfraView Disk Capacity Metric and Column Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用 `smart_disk_capacity_bytes` 作为硬盘容量唯一来源，把型号与容量拆成两列，并为容量提供稳定的服务端排序。

**Architecture:** Nightingale Provider 在现有单次即时 batch 中新增一条固定容量查询，先由 inventory 建立设备集合，再把容量作为独立可空字段归并；Service/API 只扩展 `capacity` 排序白名单，不改变 schema。前端把原合并单元格拆成两个单元格并复用现有 URL 排序状态，最终保持 1440×900 十列无横向溢出。

**Tech Stack:** Go 1.24、Nightingale v8.4.1 固定即时 batch、React 19、TypeScript、TanStack Query/Table、Vitest、Playwright、Docker。

## 2026-08-01 实际执行状态

| 范围 | 状态 | 证据/边界 |
| --- | --- | --- |
| Task 1 Provider | 已完成 RED→GREEN | 固定 18 组、容量唯一来源、字段级非法/冲突/身份边界；adapter 普通/race 通过 |
| Task 2 Service/API | 已完成 RED→GREEN | `sort=capacity`、精确 `int64`、缺失始终最后；service/httpapi 普通/race 通过 |
| Task 3 前端 | 已完成 RED→GREEN | 十列、型号/容量分列、容量升降序；硬盘页 14 项、typecheck/build 通过 |
| Task 4 E2E/文档 | 已完成 | Playwright 静态发现 13 项通过；文档保留历史替代说明，未运行浏览器或创建端口 |
| Task 5 全量验证 | 离线与现场均完成 | 前端 8 文件/101 项、Go 普通/race/编译、无缓存镜像通过；现有 8080 原位重建、容器安全、脱敏 API、硬盘十列/容量排序及总览四槽位 Chromium 均通过 |
| 独立代码复审 | Ready；Critical 0、Important 0、Minor 0 | 首轮 2 个 Important 与 1 个 Minor 已修复：容量改为原始文本精确解析并补 `2^53` 边界 RED→GREEN；README/ARCHITECTURE/DESIGN 当前态同步；E2E 补容量降序与脱敏格式布尔断言 |
| commit / push | 已完成 | 经用户明确授权，功能提交 `6300413` 已推送到 `origin/main` |

## Global Constraints

- 工作目录固定为 `/root/github/InfraView`，分支固定为 `main`；保留全部既有未提交 SMART 模块和用户文档差异，禁止 reset、restore、checkout 或清理。
- InfraView 始终严格只读；只允许代码内置固定 PromQL，不得增加任意查询、代理、SMART 控制或运维写操作。
- 开发 8080 永远只连接测试 Nightingale；不得连接、切换或探测生产 Nightingale，不得创建其他 InfraView 端口。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量值/指标值或上游正文。
- 固定硬盘即时 batch 从 17 条增加为 18 条，容量固定为第 17 条，inventory 固定为第 18 条；仍然只有一次 batch，禁止 N+1。
- `smart_disk_capacity_bytes` 是容量唯一来源；旧 inventory `capacity` 标签不得读取、校验、冲突判断或回退。
- 容量不参与设备发现、`ReportedAt`、freshness、SMART 状态、总览统计或错误摘要。
- 型号与容量拆成两个独立列；容量升降序都将缺失值放在最后。
- 每页数量继续只有 20、50、100；不实现命令超时、失联移除或“全部”分页。
- 所有开发和验证使用 Docker；不在宿主机安装 Go、Node 或浏览器依赖。
- 提交和推送分别需要用户明确授权；开发阶段每个任务只记录验证结果并等待复审，不自动执行 `git add`、commit 或 push。最终授权与执行结果见上方状态表。

## File Map

- `internal/adapters/nightingale/disk_promql.go`：固定 18 条硬盘 PromQL 顺序。
- `internal/adapters/nightingale/disk_provider.go`：容量状态、身份匹配、最新值/冲突归并和 inventory 容量标签移除。
- `internal/adapters/nightingale/disk_provider_test.go`：固定 batch、容量字段和安全边界的 Provider 测试。
- `internal/adapters/nightingale/testdata/disk-instant-batch.json`：完全脱敏的 18 组即时 batch 夹具。
- `internal/service/disk_service.go`、`internal/service/disk_service_test.go`：`capacity` 白名单和精确可空整数排序。
- `internal/httpapi/disk_handlers_test.go`：HTTP `sort=capacity` 白名单回归。
- `web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`：十列定义、容量排序按钮和独立单元格。
- `web/src/app/theme.css`：十列桌面宽度与省略样式。
- `web/e2e/infraview.spec.ts`：十列、容量排序 URL 与无横向溢出规格。
- `docs/datasources/NIGHTINGALE.md`、交接/状态/TODO/测试文档及本轮规格计划：持久化当前契约、验证证据和未执行边界。

---

### Task 1: Nightingale 独立容量查询与安全归并

**Files:**
- Modify: `internal/adapters/nightingale/disk_promql.go`
- Modify: `internal/adapters/nightingale/disk_provider.go`
- Modify: `internal/adapters/nightingale/disk_provider_test.go`
- Modify: `internal/adapters/nightingale/testdata/disk-instant-batch.json`

**Interfaces:**
- Consumes: `diskPromQL() []string`、`instantSeries`、`diskStateForSeries(...)`、`diskScalarState`、`disk.Device.CapacityBytes *int64`。
- Produces: `diskCapacityQuery`（第 17 组、零基索引 16）和 `diskInventoryQuery`（第 18 组、零基索引 17）；容量只来自新指标。

- [x] **Step 1: 把固定查询契约测试改为 18 条并制造 RED**

在 `wantDiskPromQL` 的 inventory 查询前加入：

```go
`smart_disk_capacity_bytes`,
`tlast_over_time(smart_device_health_ok[24h])`,
```

把测试 batch 改为 18 组：

```go
func validDiskBatch() [][]instantSeries {
	groups := make([][]instantSeries, 18)
	labels := diskLabels()
	groups[0] = []instantSeries{diskSeries(labels, 1785123100, "1")}
	groups[16] = []instantSeries{diskSeries(labels, 1785123900, "1000000000")}
	groups[17] = []instantSeries{diskSeries(labels, 1785124000, "1785123000.25")}
	return groups
}
```

把所有 inventory 测试索引从 `groups[16]` 改为 `groups[17]`；inventory 标签保留一个故意不同的脱敏 `capacity` 值，证明它不会回退。

- [x] **Step 2: 运行固定契约测试，确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestDiskPromQLIsFixedAndReturnsACopy|TestSMARTSnapshotUsesOneFixedBatch' -count=1
```

Expected: FAIL，明确显示实现仍缺少容量查询或只接受 17 组。

- [x] **Step 3: 写容量唯一来源、字段级缺失和冲突 RED 测试**

新增核心用例：

```go
func TestSMARTSnapshotUsesCapacityMetricAsOnlySource(t *testing.T) {
	groups := validDiskBatch()
	groups[17][0].Metric["capacity"] = "999999999"
	snapshot, err := runDiskBatch(t, groups)
	if err != nil { t.Fatal(err) }
	device := diskDeviceByName(t, snapshot, "/dev/fixture-a")
	if device.CapacityBytes == nil || *device.CapacityBytes != 1000000000 {
		t.Fatalf("CapacityBytes = %#v", device.CapacityBytes)
	}
}
```

以 table test 覆盖容量组为 `nil`、负数、`1.5`、`NaN`、`Inf`、超出 `int64` 和同一最新时间两个不同值，均要求 `CapacityBytes == nil` 且快照成功。另加用例验证较新有效值胜出、较旧值不覆盖、未知设备不创建、明确 serial/WWN 冲突不归并、容量时间戳不改变 `ReportedAt`。

- [x] **Step 4: 运行容量测试，确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestSMARTSnapshot.*Capacity|TestDiskPromQL' -count=1
```

Expected: FAIL；容量仍来自 inventory 或 18 组尚未实现。

- [x] **Step 5: 实现固定查询、独立容量状态和归并**

在固定列表和索引中把容量放在 inventory 前：

```go
diskUnsafeShutdownsQuery
diskCapacityQuery
diskInventoryQuery
diskQueryCount
```

给 `diskDeviceState` 增加 `capacity diskScalarState`，从 `diskInventoryState`、`buildDiskStates`、`diskInventoryMetadataConflict` 删除容量标签处理。建立 inventory 后按以下顺序归并：

```go
mergeDiskHealth(states, results[diskHealthQuery])
mergeDiskCapacity(states, results[diskCapacityQuery])
mergeDiskAuxiliary(states, results)
```

```go
func mergeDiskCapacity(states map[string]*diskDeviceState, series []instantSeries) {
	for _, candidate := range series {
		state, ok := diskStateForSeries(states, candidate)
		if !ok { continue }
		mergeDiskScalar(&state.capacity, candidate, diskNonNegativeInteger)
	}
}
```

在 `finalizeDiskDevice` 用 `finiteInt64` 转为 `CapacityBytes`。容量合并不能要求 `state.reporting`，不能修改 `ReportedAt`。

- [x] **Step 6: 更新完全脱敏 fixture 为 18 组**

在 `disk-instant-batch.json` 中把容量组放在 inventory 前。容量组只使用 `fixture-*` 标签和值；inventory 的旧 `capacity` 标签设为不同脱敏值，测试仍必须取第 17 组指标值。不得加入现场数据。

- [x] **Step 7: 运行 Provider 普通与 race 测试，确认 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c "gofmt -w internal/adapters/nightingale/disk_promql.go internal/adapters/nightingale/disk_provider.go internal/adapters/nightingale/disk_provider_test.go && go test ./internal/adapters/nightingale -count=1 && go test -race ./internal/adapters/nightingale -count=1"
```

Expected: PASS；仍只有一次即时 batch，错误不输出原始标签或正文。

- [x] **Step 8: Task 1 复审闸门**

```bash
git diff --check
git diff -- internal/adapters/nightingale/disk_promql.go internal/adapters/nightingale/disk_provider.go internal/adapters/nightingale/disk_provider_test.go internal/adapters/nightingale/testdata/disk-instant-batch.json
```

Expected: 只有容量查询、归并和脱敏测试差异。未获提交授权时不执行 commit。

---

### Task 2: Service 与 HTTP 容量排序

**Files:**
- Modify: `internal/service/disk_service.go`
- Modify: `internal/service/disk_service_test.go`
- Modify: `internal/httpapi/disk_handlers_test.go`

**Interfaces:**
- Consumes: `DiskQuery.Sort string`、`DiskDeviceSummary.CapacityBytes *int64`。
- Produces: 只读参数 `sort=capacity`；响应仍为 `capacity_bytes: number|null`。

- [x] **Step 1: 把 `capacity` 从非法查询测试移入合法白名单并制造 RED**

从非法 query table 删除 `{Sort: "capacity", Page: 1, PageSize: 20}`，把 Service 和 HTTP 的合法排序列表改为：

```go
[]string{"", "host", "device", "capacity", "temperature", "lifetime", "power_on_hours", "status"}
```

HTTP 列表不包含空字符串。

- [x] **Step 2: 增加容量缺失最后与稳定 tie-break RED 测试**

创建 `missing`、`one`、`two-a`、`two-b` 四块脱敏设备；升序期望 `[one two-a two-b missing]`，降序期望 `[two-a two-b one missing]`。两个容量相同项必须按 ID 稳定收口。

- [x] **Step 3: 运行定向测试，确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service ./internal/httpapi -run 'Disk.*CapacitySort|DiskServiceQueryValidation|DiskDevicesAcceptsEverySupportedSort' -count=1
```

Expected: FAIL，`capacity` 尚未支持或顺序错误。

- [x] **Step 4: 实现精确可空 `int64` 排序**

在 `normalizeDiskQuery` 白名单加入 `capacity`。在 `sortDiskDevices` 的浮点排序前处理容量：有效值永远排在 nil 前；两个有效值直接比较 `int64`；相等时比较 ID；只有有效值的大小关系受 `order` 反转，nil 位置不反转。不要先转成 `float64`。

核心分支：

```go
if field == "capacity" {
	left, right := items[i].CapacityBytes, items[j].CapacityBytes
	if (left == nil) != (right == nil) { return left != nil }
	comparison := 0
	if left != nil {
		switch {
		case *left < *right: comparison = -1
		case *left > *right: comparison = 1
		}
	}
	if comparison == 0 { return items[i].ID < items[j].ID }
	if order == "desc" { return comparison > 0 }
	return comparison < 0
}
```

- [x] **Step 5: 运行 Service/API 普通与 race 测试，确认 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c "gofmt -w internal/service/disk_service.go internal/service/disk_service_test.go internal/httpapi/disk_handlers_test.go && go test ./internal/service ./internal/httpapi -count=1 && go test -race ./internal/service ./internal/httpapi -count=1"
```

Expected: PASS；缺失始终最后，既有非法字段仍返回统一 400。

- [x] **Step 6: Task 2 复审闸门**

```bash
git diff --check
git diff -- internal/service/disk_service.go internal/service/disk_service_test.go internal/httpapi/disk_handlers_test.go
```

Expected: 只有容量白名单、精确比较和测试差异。未获提交授权时不执行 commit。

---

### Task 3: 前端十列与容量排序交互

**Files:**
- Modify: `web/src/features/disks/DiskPage.tsx`
- Modify: `web/src/features/disks/DiskPage.test.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Consumes: `DiskDevice.model`、`DiskDevice.capacity_bytes`、API `sort=capacity`。
- Produces: `DiskSort` 新值 `capacity`；独立 `.disk-model`/`.disk-capacity` 单元格；十列硬盘表。

- [x] **Step 1: 把九列测试改成十列并制造 RED**

表头期望改为：

```ts
['主机', '设备', '型号', '容量', 'SMART 健康', '温度', '寿命', '通电时间', '错误摘要', '状态']
```

断言每行 10 个单元格；型号索引 2、容量索引 3，后续字段索引依次后移。型号缺失测试必须确认容量独立单元格仍显示 `2 TiB`。

- [x] **Step 2: 把排序测试从六种扩展为七种并制造 RED**

把 `['容量', 'capacity']` 加到排序表中，断言首次点击产生 `sort=capacity&order=asc&page=1`，第二次点击产生 `order=desc`。

- [x] **Step 3: 运行页面测试，确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/disks/DiskPage.test.tsx'
```

Expected: FAIL，页面仍是合并列且不存在容量排序按钮。

- [x] **Step 4: 实现两个独立列**

在 `sortFields` 加入 `'capacity'`，把 `columnLabels` 的合并键换成 `model: '型号'`、`capacity: '容量'`。列定义为：

```tsx
{
  id: 'model',
  header: () => <span title="型号">型号</span>,
  cell: ({ row }) => {
    const model = row.original.model.trim() || '暂无数据'
    return <span className="disk-model" title={model}>{model}</span>
  },
},
{
  id: 'capacity',
  header: () => sortButton('capacity', '容量'),
  cell: ({ row }) => {
    const capacity = binaryCapacity(row.original.capacity_bytes)
    return <span className="disk-capacity" title={capacity}>{capacity}</span>
  },
},
```

删除 `.disk-model-capacity` 包装，不改错误摘要、状态或 freshness 文案。

- [x] **Step 5: 调整十列 CSS**

使用合计 100% 的宽度：

```css
.host-table.disk-table th:nth-child(1) { width: 11%; }
.host-table.disk-table th:nth-child(2) { width: 8%; }
.host-table.disk-table th:nth-child(3) { width: 15%; }
.host-table.disk-table th:nth-child(4) { width: 9%; }
.host-table.disk-table th:nth-child(5) { width: 10%; }
.host-table.disk-table th:nth-child(6) { width: 7%; }
.host-table.disk-table th:nth-child(7) { width: 8%; }
.host-table.disk-table th:nth-child(8) { width: 9%; }
.host-table.disk-table th:nth-child(9) { width: 14%; }
.host-table.disk-table th:nth-child(10) { width: 9%; }
```

型号和容量都保持单行省略与完整 `title`；容量继续使用 muted 色。不得通过开启横向滚动解决布局。

- [x] **Step 6: 运行定向测试、typecheck 和 build，确认 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/disks/DiskPage.test.tsx && npm run typecheck && npm run build'
```

Expected: PASS，无 TypeScript 或生产构建错误。

- [x] **Step 7: Task 3 复审闸门**

```bash
git diff --check
git diff -- web/src/features/disks/DiskPage.tsx web/src/features/disks/DiskPage.test.tsx web/src/app/theme.css
```

Expected: 十列和容量排序之外无行为变化。未获提交授权时不执行 commit。

---

### Task 4: 浏览器规格与持久文档

**Files:**
- Modify: `web/e2e/infraview.spec.ts`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/superpowers/specs/2026-07-30-host-disk-smart-module-design.md`
- Modify: `docs/superpowers/plans/2026-07-30-host-disk-smart-module.md`
- Modify: `docs/superpowers/specs/2026-07-30-overview-four-slot-and-disk-display-refinement-design.md`
- Modify: `docs/superpowers/specs/2026-08-01-disk-capacity-metric-and-column-design.md`
- Modify: `docs/superpowers/plans/2026-08-01-disk-capacity-metric-and-column.md`

**Interfaces:**
- Consumes: 十列表格 DOM、容量排序 URL、18 组契约及实际验证结果。
- Produces: 可重复浏览器规格和完整恢复入口。

- [x] **Step 1: 更新 Playwright 规格**

把用例名改成“硬盘页保留十列和 URL 状态且桌面视口无横向溢出”，表头数量改为 10，表头列表把“型号 / 容量”替换为“型号”“容量”。保留 `.disk-model`、`.disk-capacity` 独立可见性，并加入：

```ts
await page.getByRole('button', { name: '容量' }).click()
await expect(page).toHaveURL(/sort=capacity/)
await expect(page).toHaveURL(/order=asc/)
```

页面与 `.disk-table-scroll` 继续要求 `scrollWidth <= clientWidth`。

- [x] **Step 2: 静态发现 Playwright 规格，不启动新端口**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-bookworm \
  sh -c 'npm ci --ignore-scripts && npx playwright test --list'
```

Expected: 规格被发现且配置无错误；不得运行会创建 18080 的 `scripts/e2e.sh`。

- [x] **Step 3: 更新 Nightingale 与历史规格**

在 `NIGHTINGALE.md` 记录容量是第 17 组唯一来源、inventory 是第 18 组、仍一次 batch。给两份 2026-07-30 规格和旧计划追加“2026-08-01 替代说明”，明确旧 17 组、inventory 容量标签和同列两行是历史状态；不要改写历史已执行证据。

- [x] **Step 4: 更新交接、状态、待办和测试文档**

在 `HANDOFF` 恢复提示加入新规格与计划路径及当前 18 查询/十列状态。`PROJECT_STATUS`、`TODO`、`TESTING` 记录实际完成项、命令类别、测试数量、8080 是否重建、现场验证是否执行。未执行项必须写“未执行”。

- [x] **Step 5: 更新本轮规格与计划状态**

离线验证完成后把本轮规格状态改为“已实现，离线验收完成”，按真实结果勾选 Task 1–4。未获 8080 授权时现场步骤保持未勾选。

- [x] **Step 6: Task 4 复审闸门**

```bash
git diff --check
rg -n '17 组|18 组|型号 / 容量|九列|十列|smart_disk_capacity_bytes' \
  docs/HANDOFF.md docs/PROJECT_STATUS.md docs/TODO.md docs/TESTING.md \
  docs/datasources/NIGHTINGALE.md docs/superpowers/specs docs/superpowers/plans
```

Expected: 当前状态统一为 18 组、容量独立列，历史描述有替代说明。未获提交授权时不执行 commit。

---

### Task 5: 全量验证、现场授权闸门与交付

**Files:**
- Verify: 全部本轮源码、测试和文档差异
- Modify after results: `docs/HANDOFF.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/TESTING.md`、本轮规格与计划

**Interfaces:**
- Consumes: Task 1–4 GREEN 结果。
- Produces: 无缓存镜像、最终安全检查、可选的现有 8080 脱敏现场证据和提交授权边界。

- [x] **Step 1: 前端全量验证**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build'
```

Expected: 全部退出 0；记录实际测试文件数和测试数。

- [x] **Step 2: Go 全仓普通/race 与编译**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'files=$(find cmd internal -type f -name "*.go"); test -z "$(gofmt -l $files)" && go test ./... && go test -race ./... && CGO_ENABLED=0 go build -o /tmp/infraview ./cmd/infraview'
```

Expected: gofmt 无输出，测试和编译全部退出 0。

- [x] **Step 3: 无缓存镜像构建**

```bash
docker build --no-cache --tag infraview:disk-capacity-column-verify .
```

Expected: Dockerfile 内前端、Go 普通/race 和编译再次通过；镜像不运行、不映射端口、不连接上游。

- [x] **Step 4: 最终静态检查**

```bash
git diff --check
rg -n 'smart_disk_capacity_bytes|sort=capacity|capacity_bytes' internal web docs
git status --short --branch
```

Expected: 不出现私密内容或现场数据，既有差异全部保留。

- [x] **Step 5: 在现场授权闸门暂停**

若未获明确授权，记录“未部署、未重启、未执行真实 Nightingale/Chromium 验收”并跳到 Step 8。不得运行 `scripts/e2e.sh` 或创建其他端口。

- [x] **Step 6: 仅经授权原位重建现有 8080**

```bash
docker compose --project-name infraview ps --format json
docker ps --filter label=com.docker.compose.project=infraview --format '{{.Names}}|{{.Ports}}|{{.Status}}'
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env docker compose --project-name infraview up -d --build --force-recreate infraview
```

Expected: 只重建既有服务，仍只发布 8080、仍连接测试 Nightingale；不显示私密环境内容。

- [x] **Step 7: 仅经同一授权执行脱敏 API 与 Chromium 验收**

只输出状态和布尔结论，不输出正文、数量、容量值或身份。覆盖：只读 API 200 JSON/non-stale、容量字段至少存在一个非 null、`sort=capacity` 双向成功、写方法 405、十列表头、独立型号/容量、容量排序 URL、1440×900 页面/表格无溢出、无破坏性控件、无非预期浏览器错误。浏览器容器访问原 8080，不发布端口、不截图、不保存 trace。

健康与安全命令：

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null
docker inspect infraview-infraview-1 --format '{{.State.Health.Status}}|{{.Config.User}}|{{.HostConfig.ReadonlyRootfs}}|{{json .HostConfig.CapDrop}}|{{json .HostConfig.SecurityOpt}}|{{json .NetworkSettings.Ports}}'
```

Expected: healthy、非 root、只读 rootfs、cap drop `ALL`、禁止提权且只有 8080。

- [x] **Step 8: 按实际结果收口文档**

将离线和可选现场结果如实写入 Task 4 文档；未执行步骤不能写成成功。然后运行：

```bash
git diff --check
git status --short --branch
```

Expected: 无空白错误，无截图、trace 或 `web/test-results` 临时产物。

- [x] **Step 9: 提交与推送授权闸门**

未获提交授权时停止并报告未提交。获提交授权后先列出精确文件并确认不含私密/临时产物，再执行授权范围内的 `git add` 和 commit；push 仍需另一项明确授权。
