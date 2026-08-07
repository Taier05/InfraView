# 旧模块列表统一 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将主机、硬盘、MySQL、Redis 列表统一为紧凑、单行、无可见排序箭头且所有展示列均执行真实服务端排序的列表。

**Architecture:** 保持现有 API View、Provider、固定 Nightingale 查询和缓存不变，只扩展四个 Service 的排序白名单与比较器，并在现有 GET handler 中透传固定排序参数。前端拆开独立指标列，Host/Disk/MySQL 迁移到既有 `ListPage` 壳，四页共享既有表格类和各自列宽。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Docker。

## Global Constraints

- InfraView 始终只读；不得增加写 API、运维控件、任意 PromQL、代理或 N+1。
- 不改变 Nightingale 查询、缓存、freshness、API 路径或响应字段。
- 每个单元格只显示一个单行值；连接列显示 `当前连接/最大连接`，硬盘错误摘要保持一列。
- 所有展示列均支持服务端升降序；缺失值无论方向始终置后，相同值以稳定实体 ID 升序收口。
- 表头不得渲染 `↑`、`↓`、`⇅`；排序状态保留 `data-active`、`aria-label` 和 `title`。
- 主机/Redis 的 `int64` 容量和计数直接用整数比较，不得转为 `float64`。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文。
- 测试、typecheck、build 只在一次性容器执行；不得启动新端口。提交、推送、部署和重启均需单独授权。

## File map

- `internal/service/hosts.go`：主机排序白名单、精确数值比较和有效状态排序。
- `internal/service/disk_service.go`：硬盘型号、SMART、错误摘要和有效状态排序。
- `internal/service/mysql_service.go`：MySQL 拆列字段排序和旧字段兼容别名。
- `internal/service/redis_service.go`：Redis 拆列字段排序和精确整数比较。
- `internal/service/service_test.go`、`disk_service_test.go`、`mysql_service_test.go`、`redis_service_test.go`：Service RED/GREEN 契约。
- `internal/httpapi/api_test.go`、`disk_handlers_test.go`、`redis_handlers_test.go`：GET 排序白名单契约。
- `web/src/features/{hosts,disks,mysql,redis}/*Page.tsx`：精确列、排序键和共享列表壳。
- `web/src/features/{hosts,disks,mysql,redis}/*Page.test.tsx`：列、拆值、URL 和无箭头回归。
- `web/src/app/theme.css`：四页紧凑列宽、单行与唯一滚动容器。
- `docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`：交付状态和恢复入口。

---

### Task 1: 主机全部列服务端排序

**Files:**
- Modify: `internal/service/types.go:8-30`
- Modify: `internal/service/hosts.go:108-225`
- Test: `internal/service/service_test.go:233-425`
- Test: `internal/httpapi/api_test.go:832-870`

**Interfaces:**
- Consumes: `HostQuery.Sort string`、`HostSummary` 的 `CPUCores`、`MemoryTotalBytes`、收发网络指标、`Status`、`CollectionLevel`。
- Produces: 共享 `listLevelSortRank(Level) int`；排序键 `name|ip|cpu_cores|memory_total|cpu|memory|load|io|network_transmit|network_receive|uptime|status`。

- [ ] **Step 1: 写主机排序 RED 测试**

在 `service_test.go` 增加表驱动用例，分别断言精确整数、收发拆分、有效状态、双向缺失置后和 ID 收口：

```go
tests := []struct {
	field string
	wantAsc, wantDesc []string
}{
	{"cpu_cores", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
	{"memory_total", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
	{"network_transmit", []string{"id-low", "id-high", "id-missing"}, []string{"id-high", "id-low", "id-missing"}},
	{"network_receive", []string{"id-high", "id-low", "id-missing"}, []string{"id-low", "id-high", "id-missing"}},
}
```

另设 `CollectionLevel=warning`、离线、在线、未知主机，断言状态升序为正常、警告、严重、未知。HTTP 测试逐个请求 `?sort=<field>&order=asc&page=1&page_size=20` 并断言不是 400。

- [ ] **Step 2: 运行主机 RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service ./internal/httpapi -run 'Hosts.*Sort|HostList.*Sort' -count=1
```

Expected: FAIL，新增排序键被拒绝或顺序不符合契约。

- [ ] **Step 3: 实现精确比较和有效状态排序**

扩展 `normalizeHostQuery` 白名单；将旧 `network` 保留为兼容别名，但新页面只发送拆分键。实现整数比较与有效等级：

```go
func hostIntegerSortValue(host HostSummary, field string) (int64, bool, bool) {
	switch field {
	case "cpu_cores":
		if host.CPUCores == nil { return 0, false, true }
		return int64(*host.CPUCores), true, true
	case "memory_total":
		if host.MemoryTotalBytes == nil { return 0, false, true }
		return *host.MemoryTotalBytes, true, true
	default:
		return 0, false, false
	}
}

func hostStatusRank(host HostSummary) int {
	if host.CollectionLevel == LevelWarning { return 1 }
	if host.CollectionLevel == LevelCritical { return 2 }
	switch host.Status {
	case datasource.StatusOnline: return 0
	case datasource.StatusOffline: return 2
	default: return 3
	}
}

func listLevelSortRank(level Level) int {
	switch level {
	case LevelNormal: return 0
	case LevelWarning: return 1
	case LevelCritical: return 2
	default: return 3
	}
}
```

`hostMetricSortValue` 增加 `network_transmit` 和 `network_receive`；比较器必须在应用 `order` 前完成“可用值优先”，相同时使用 `ID`。

- [ ] **Step 4: 格式化并运行主机 GREEN/回归**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/types.go internal/service/hosts.go internal/service/service_test.go internal/httpapi/api_test.go && go test ./internal/service ./internal/httpapi -run "Hosts.*Sort|HostList.*Sort" -count=1 && go test -race ./internal/service -run "Hosts.*Sort" -count=1'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 1**

```bash
git add internal/service/types.go internal/service/hosts.go internal/service/service_test.go internal/httpapi/api_test.go
git diff --cached --check
git commit -m "feat: sort every host list column"
```

---

### Task 2: 硬盘全部列服务端排序

**Files:**
- Modify: `internal/service/disk_service.go:305-430`
- Test: `internal/service/disk_service_test.go:459-635`
- Test: `internal/httpapi/disk_handlers_test.go:252-310`

**Interfaces:**
- Consumes: `DiskDeviceSummary.Model`、`SMARTHealth`、`Errors`、`Status`。
- Produces: 新排序键 `model|smart|errors`，并保留现有七个键。

- [ ] **Step 1: 写硬盘排序 RED 测试**

新增 `model` 自然排序、SMART 等级、错误摘要求和、全部错误缺失置后、双向 ID 收口测试。错误合计只使用页面现有六项，不包括未展示的 `UnsafeShutdowns`：

```go
fields := []string{"model", "smart", "errors"}
for _, field := range fields {
	for _, order := range []string{"asc", "desc"} {
		page, _, err := service.Devices(context.Background(), DiskQuery{
			Sort: field, Order: order, Page: 1, PageSize: 20,
		})
		if err != nil { t.Fatalf("%s/%s: %v", field, order, err) }
		got := diskDeviceIDs(page.Devices)
		if got[len(got)-1] != "id-missing" {
			t.Fatalf("%s/%s IDs = %v, missing item is not last", field, order, got)
		}
	}
}
```

更新 HTTP 支持字段表为 `host,device,model,capacity,smart,temperature,lifetime,power_on_hours,errors,status`。

- [ ] **Step 2: 运行硬盘 RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service ./internal/httpapi -run 'Disk.*Sort|DiskDevicesAcceptsEverySupportedSort' -count=1
```

Expected: FAIL，新键被拒绝。

- [ ] **Step 3: 实现型号、SMART、错误摘要排序**

扩展查询白名单，并实现：

```go
func diskErrorSortValue(errors disk.ErrorCounters) (float64, bool) {
	values := []*float64{
		errors.PendingSectors, errors.ReallocatedSectors,
		errors.UncorrectableSectors, errors.UDMACRCErrors,
		errors.MediaIntegrityErrors, errors.ErrorLogEntries,
	}
	var total float64
	available := false
	for _, value := range values {
		if value != nil { total += *value; available = true }
	}
	return total, available
}

func diskHealthRank(value disk.Health) int {
	switch value {
	case disk.HealthHealthy: return 0
	case disk.HealthFailed: return 2
	default: return 3
	}
}
```

`model` 使用 `compareNatural` 且空白末置；`status` 改为 Task 1 产出的 `listLevelSortRank`，不再比较原始字符串；所有 tie 以 ID 收口。告警计算继续使用原 `diskLevelRank`，两者不得混用。

- [ ] **Step 4: 格式化并运行硬盘 GREEN/回归**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/disk_service.go internal/service/disk_service_test.go internal/httpapi/disk_handlers_test.go && go test ./internal/service ./internal/httpapi -run "Disk.*Sort|DiskDevicesAcceptsEverySupportedSort" -count=1 && go test -race ./internal/service -run "Disk.*Sort" -count=1'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 2**

```bash
git add internal/service/disk_service.go internal/service/disk_service_test.go internal/httpapi/disk_handlers_test.go
git diff --cached --check
git commit -m "feat: sort every disk list column"
```

---

### Task 3: MySQL 拆列字段服务端排序

**Files:**
- Modify: `internal/service/mysql_service.go:220-330`
- Test: `internal/service/mysql_service_test.go:552-625`
- Test: `internal/httpapi/api_test.go:65-380`

**Interfaces:**
- Produces: `instance|version|role|connections|threads_running|qps|tps|slow_queries|buffer_pool_size|buffer_pool_usage|replication_state|replication_lag|uptime|status`。
- Compatibility: 继续接受旧 `buffer_pool`，等价于 `buffer_pool_usage`。

- [ ] **Step 1: 写 MySQL RED 测试**

为 `version`、`role`、`tps`、`buffer_pool_size`、`buffer_pool_usage`、`replication_state` 增加升降序、缺失末置和 ID tie 测试；断言 `status` 按等级而非字符串，`buffer_pool` 旧键仍成功。

```go
sorts := []string{
	"instance", "version", "role", "connections", "threads_running",
	"qps", "tps", "slow_queries", "buffer_pool_size",
	"buffer_pool_usage", "replication_state", "replication_lag",
	"uptime", "status",
}
```

- [ ] **Step 2: 运行 MySQL RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service ./internal/httpapi -run 'MySQL.*Sort|MySQLInstances.*Sort' -count=1
```

Expected: FAIL，新键被拒绝。

- [ ] **Step 3: 实现 MySQL 拆列排序**

数值字段直接接入 `mysqlMetricSortValue`：

```go
case "tps": return metricSortValue(item.TPS)
case "buffer_pool_size": return metricSortValue(item.BufferPoolSizeBytes)
case "buffer_pool", "buffer_pool_usage": return metricSortValue(item.BufferPoolUsagePercent)
```

`version` 使用自然文本且空白末置；角色顺序 `writable, read_only, unknown`；复制状态先用 `listLevelSortRank` 比较 `Replication.Level`，再比较 `normal, not_configured, threads_stopped, unknown`；状态同样使用 `listLevelSortRank`。不得改变连接按 `ConnectionUsagePercent` 排序的既有语义。

- [ ] **Step 4: 格式化并运行 MySQL GREEN/回归**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/mysql_service.go internal/service/mysql_service_test.go internal/httpapi/api_test.go && go test ./internal/service ./internal/httpapi -run "MySQL.*Sort|MySQLInstances.*Sort" -count=1 && go test -race ./internal/service -run "MySQL.*Sort" -count=1'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 3**

```bash
git add internal/service/mysql_service.go internal/service/mysql_service_test.go internal/httpapi/api_test.go
git diff --cached --check
git commit -m "feat: sort split MySQL columns"
```

---

### Task 4: Redis 拆列字段服务端排序

**Files:**
- Modify: `internal/service/redis_service.go:170-215,399-455`
- Test: `internal/service/redis_service_test.go:168-215`
- Test: `internal/httpapi/redis_handlers_test.go:19-95`

**Interfaces:**
- Produces: `instance|role|memory_limit|memory|connections|blocked_connections|qps|hit_rate|keys|replication_link|replication_lag|uptime|status`。
- Compatibility: 保留既有 `evicted` 排序键，但页面不展示该列。

- [ ] **Step 1: 写 Redis RED 测试**

增加 `role`、`memory_limit`、`blocked_connections`、`hit_rate`、`replication_link` 双向排序测试。容量和阻塞连接使用接近 `int64` 上界的脱敏 fixture，锁定不经浮点转换；master 的复制链路“—”作为不适用值始终末置。

```go
sorts := []string{
	"instance", "role", "memory_limit", "memory", "connections",
	"blocked_connections", "qps", "hit_rate", "keys",
	"replication_link", "replication_lag", "uptime", "status",
}
```

- [ ] **Step 2: 运行 Redis RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service ./internal/httpapi -run 'Redis.*Sort|Redis.*Queries' -count=1
```

Expected: FAIL，新键被拒绝。

- [ ] **Step 3: 实现 Redis 精确排序**

为整数值增加精确比较路径：

```go
func redisIntegerSortValue(item RedisInstanceSummary, field string) (int64, bool, bool) {
	var value *int64
	switch field {
	case "memory_limit": value = item.MaxMemoryBytes
	case "blocked_connections": value = item.BlockedClients
	case "keys": value = item.Keys
	case "uptime": value = item.UptimeSeconds
	default: return 0, false, false
	}
	if value == nil { return 0, false, true }
	return *value, true, true
}
```

`hit_rate` 使用原始 `HitRate`；角色顺序 `master, slave, unknown`。复制链路对 slave 按正常、断开、未知排序，对 master 标记为不可用并末置。状态使用 Task 1 产出的 `listLevelSortRank`；告警计算继续使用原 `redisLevelRank`；tie 统一 ID。

- [ ] **Step 4: 格式化并运行 Redis GREEN/回归**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/redis_service.go internal/service/redis_service_test.go internal/httpapi/redis_handlers_test.go && go test ./internal/service ./internal/httpapi -run "Redis.*Sort|Redis.*Queries" -count=1 && go test -race ./internal/service -run "Redis.*Sort" -count=1'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 4**

```bash
git add internal/service/redis_service.go internal/service/redis_service_test.go internal/httpapi/redis_handlers_test.go
git diff --cached --check
git commit -m "feat: sort split Redis columns"
```

---

### Task 5: 主机和硬盘共享列表壳与精确列

**Files:**
- Modify: `web/src/features/hosts/HostListPage.tsx:15-520`
- Modify: `web/src/features/disks/DiskPage.tsx:15-610`
- Modify: `web/src/app/theme.css:1320-1515`
- Test: `web/src/features/hosts/HostListPage.test.tsx`
- Test: `web/src/features/disks/DiskPage.test.tsx`

**Interfaces:**
- Consumes: Task 1/2 排序键。
- Produces: 主机 12 列、硬盘 10 列，无可见箭头，复用 `ListPageHeader/Controls/TablePanel`。

- [ ] **Step 1: 写前端 RED 测试**

主机精确表头：

```ts
const headers = [
  '主机名', 'IP 地址', 'CPU 核数', '内存容量', 'CPU 使用率',
  '内存使用率', '负载', 'IO 忙碌度', '网络发送', '网络接收',
  '运行时间', '状态',
]
const sorts = [
  'name', 'ip', 'cpu_cores', 'memory_total', 'cpu', 'memory', 'load',
  'io', 'network_transmit', 'network_receive', 'uptime', 'status',
]
```

硬盘精确表头/排序键为 `主机,设备,型号,容量,SMART 健康,温度,寿命,通电时间,错误摘要,状态` 与 `host,device,model,capacity,smart,temperature,lifetime,power_on_hours,errors,status`。逐个点击表头，断言 URL 键、页码归 1、第二次点击降序；断言表头文本不含 `/[⇅↑↓]/`，每个表头内都有 button。

- [ ] **Step 2: 运行主机/硬盘 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx'
```

Expected: FAIL，旧复合列、静态表头和箭头仍存在。

- [ ] **Step 3: 实现共享壳、拆列和无箭头表头**

两页导入 `ListPageHeader`、`ListPageControls`、字段组件和 `ListTablePanel`。排序按钮固定为：

```tsx
function sortButton(field: SortField, label: string) {
  const current = sort === field ? (order === 'asc' ? '升序' : '降序') : '未排序'
  return <button className="host-sort-button" type="button"
    data-active={sort === field}
    aria-label={`${label}排序，当前${current}`}
    title={`点击按${label}排序`}
    onClick={() => changeSort(field)}>{label}</button>
}
```

主机网络两个 cell 分别读取 transmit/receive；硬盘错误摘要函数和展示保持不变。迁移控制栏时保留现有搜索防抖、筛选、采集时间、stale、错误、空状态和分页语义。

- [ ] **Step 4: 增加主机/硬盘紧凑 CSS 并运行 GREEN**

在现有 `host-table`/`disk-table` 选择器上设置 `table-layout: fixed`、百分比列宽、`white-space: nowrap`、省略和 `.host-table-scroll { overflow-x: auto }`；不得添加第二个 overflow-x owner。

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/components/ListPage.test.tsx src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx && npm run typecheck'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 5**

```bash
git add web/src/features/hosts/HostListPage.tsx web/src/features/hosts/HostListPage.test.tsx web/src/features/disks/DiskPage.tsx web/src/features/disks/DiskPage.test.tsx web/src/app/theme.css
git diff --cached --check
git commit -m "feat: unify host and disk list tables"
```

---

### Task 6: MySQL 和 Redis 独立指标列与紧凑样式

**Files:**
- Modify: `web/src/features/mysql/MySQLPage.tsx:20-650`
- Modify: `web/src/features/redis/RedisPage.tsx:25-530`
- Modify: `web/src/app/theme.css:1180-1405`
- Test: `web/src/features/mysql/MySQLPage.test.tsx`
- Test: `web/src/features/redis/RedisPage.test.tsx`

**Interfaces:**
- Consumes: Task 3/4 排序键。
- Produces: MySQL 14 列、Redis 13 列；连接保留 `当前/最大`，阻塞连接独立。

- [ ] **Step 1: 写 MySQL/Redis RED 测试**

锁定表头和排序键：

```ts
const mysqlHeaders = [
  '实例地址', '版本', '角色', '连接', '活跃线程', 'QPS', 'TPS',
  '慢查询', 'Buffer Pool 容量', 'Buffer Pool 使用率',
  '复制状态', '复制延迟', '运行时间', '状态',
]
const redisHeaders = [
  '实例地址', '角色', '内存上限', '内存使用率', '连接', '阻塞连接',
  'QPS', '命中率', 'key 总数', '复制链路', '延迟', '运行时间', '状态',
]
```

断言连接 cell 只包含 `当前/最大`，Redis 阻塞值仅出现在独立列；版本、角色、QPS、TPS、容量、使用率、复制状态、复制延迟均为独立 cell；所有表头是无箭头排序按钮。

- [ ] **Step 2: 运行 MySQL/Redis RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx'
```

Expected: FAIL，旧复合列、静态列和箭头存在。

- [ ] **Step 3: 实现 MySQL 14 列**

`sortFields` 使用 Task 3 的 14 个键。删除 `versionRole`、`qpsTps`、`bufferPool` 复合展示，改为独立 cell；连接 helper 仅返回：

```ts
function connectionCount(instance: MySQLInstance) {
  if (instance.connections === null && instance.max_connections === null) return '暂无数据'
  return `${decimal(instance.connections, 0)}/${decimal(instance.max_connections, 0)}`
}
```

复制状态继续使用现有中文和等级色，延迟独立格式化。MySQL 页面迁移到 `ListPage` 壳，保留标签、状态、角色筛选及 URL 规范化。

- [ ] **Step 4: 实现 Redis 13 列和无箭头**

`sortFields` 使用 Task 4 的 13 个展示键；保留后端 `evicted` 但从前端 URL 白名单移除。连接 helper 不再附加阻塞文本：

```ts
function connections(instance: RedisInstance) {
  if (instance.connected_clients === null && instance.max_clients === null) return '暂无数据'
  return `${decimal(instance.connected_clients, 0)}/${decimal(instance.max_clients, 0)}`
}
```

新增阻塞连接和命中率独立 cell；命中率仍将 0..1 原始比例格式化为百分比。

- [ ] **Step 5: 增加紧凑列宽并运行 GREEN/回归**

为 14/13 列分别设置合计 100% 的百分比宽度和足够 `min-width`；桌面保持紧凑，窄屏仅 `.mysql-table-scroll`/`.redis-table-scroll` 横向滚动。

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/components/ListPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx && npm run typecheck'
```

Expected: PASS。

- [ ] **Step 6: 经授权后提交 Task 6**

```bash
git add web/src/features/mysql/MySQLPage.tsx web/src/features/mysql/MySQLPage.test.tsx web/src/features/redis/RedisPage.tsx web/src/features/redis/RedisPage.test.tsx web/src/app/theme.css
git diff --cached --check
git commit -m "feat: split MySQL and Redis list columns"
```

---

### Task 7: 全量验证与持久文档

**Files:**
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`

**Interfaces:**
- Consumes: Task 1–6 的已验证交付。
- Produces: 可恢复的当前 main 状态；不包含现场值。

- [ ] **Step 1: 运行前端全量门禁**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

Expected: 全部 PASS；只执行 Playwright 静态发现，不启动服务。

- [ ] **Step 2: 运行 Go 全量门禁**

先确保前端 build 已生成 `internal/httpapi/webdist`，再运行：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'files="$(find cmd internal -type f -name "*.go")"; test -z "$(gofmt -l $files)" && go vet ./... && go test ./... -count=1 && go test -race ./... -count=1 && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/infraview ./cmd/infraview'
```

Expected: 全部 PASS。

- [ ] **Step 3: 执行静态只读与敏感扫描**

```bash
rg -n 'fetch\(|apiRequest\(' web/src/features/{hosts,disks,mysql,redis}
rg -n 'method:\s*["'"'](POST|PUT|PATCH|DELETE)|执行命令|重启|删除|故障转移|PromQL' web/src/features/{hosts,disks,mysql,redis} internal/httpapi
git diff --check
```

Expected: 仅固定 GET 请求；第二条无新增破坏性能力；diff check 无输出。

- [ ] **Step 4: 更新当前状态文档**

记录精确列数、拆列例外、排序键、验证命令和未执行的部署/推送状态；删除与当前工作树不符的旧措辞。不得写入现场地址、数量或指标值。

- [ ] **Step 5: 经授权后提交 Task 7**

```bash
git add docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md
git diff --cached --check
git commit -m "docs: record consistent list tables"
```
