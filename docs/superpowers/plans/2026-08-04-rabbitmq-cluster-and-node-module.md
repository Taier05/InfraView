# RabbitMQ Cluster and Node Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现只读 RabbitMQ 集群健康总览卡和 15 列节点列表，数据仅来自测试 Nightingale 中已确认的 `rabbitmq*` / `erlang*` 指标。

**Architecture:** 新建独立 `internal/rabbitmq` 领域与 `RabbitMQService`，Nightingale Provider 使用一次固定 22 查询 batch 生成共享集群/节点快照。HTTP 仅新增两个认证 GET API；前端强制复用 `ModuleStatusCardShell` 与 `ListPage`，集群通信与节点状态严格分离。

**Tech Stack:** Go 1.24、React 19、TypeScript、TanStack Query/Table、Vitest、Testing Library、Playwright、Docker。

## Global Constraints

- 规格唯一来源：`docs/superpowers/specs/2026-08-04-rabbitmq-cluster-and-node-module-design.md`。
- InfraView 始终只读；无 RabbitMQ Management API 直连、任意 PromQL、代理、详情、历史或运维操作。
- 只连接现有测试 Nightingale；永久禁止连接或探测生产 Nightingale/RabbitMQ。
- Provider 必须恰好一次固定 22 查询 `query-instant-batch`，无集群/节点/指标 N+1。
- 真实身份、地址、数量、容量、指标值、Token、Cookie、认证头、Base URL 和上游正文不得进入测试日志、文档或提交信息。
- 集群身份优先永久 ID、再回退逻辑集群与采集集群；原文仅在 Provider 内使用，API 只返回不可逆 ID。
- 集群通信不污染节点状态；消息积压、连接、队列和吞吐只展示。
- 新页面必须复用 `ListPage` 与 `ModuleStatusCardShell`；每格单值单行，1440×900 无页面或表格横向滚动。
- 当前开发 8080 只可在单独部署授权后原位重建；不得创建 18080 或其他 InfraView 端口。
- 下列 commit 步骤是计划中的建议检查点；只有当前用户授权明确包含 commit 时才执行。push 始终需要单独明确授权。

---

## File Map

- `internal/rabbitmq/`：领域类型、稳定身份、Provider 接口和契约测试。
- `internal/adapters/mock/rabbitmq_provider.go`：脱敏多集群快照。
- `internal/adapters/nightingale/rabbitmq_promql.go`：固定 22 查询及索引。
- `internal/adapters/nightingale/rabbitmq_provider.go`：严格解析、inventory-first 归并与快照构建。
- `internal/service/rabbitmq_types.go`：查询、总览、节点摘要和状态来源。
- `internal/service/rabbitmq_service.go`：缓存、freshness、状态、筛选、排序和分页。
- `internal/httpapi/rabbitmq_handlers.go`：显式 View、参数白名单和两个 GET handler。
- `web/src/features/rabbitmq/RabbitMQPage.tsx`：共享列表模板上的 15 列页面。
- `web/src/features/overview/OverviewPage.tsx`：第六张独立 RabbitMQ 卡。
- `web/e2e/rabbitmq.spec.ts`：导航、URL、15 列、单行和布局规格。

---

### Task 1: RabbitMQ Domain Contract and Mock Snapshot

**Files:**
- Create: `internal/rabbitmq/types.go`
- Create: `internal/rabbitmq/provider.go`
- Create: `internal/rabbitmq/contract_test.go`
- Create: `internal/rabbitmq/rabbitmqtest/contract.go`
- Create: `internal/adapters/mock/rabbitmq_provider.go`
- Create: `internal/adapters/mock/rabbitmq_provider_test.go`

**Interfaces:**
- Produces: `rabbitmq.Provider.RabbitMQSnapshot(context.Context) (rabbitmq.Snapshot, error)`。
- Produces: `StableClusterID(identity string)`、`StableNodeID(clusterIdentity, nodeName string)`。
- Produces: `Snapshot{Clusters []Cluster, Nodes []Node}.Clone()`。

- [x] **Step 1: Write failing domain contract tests**

```go
func TestStableIDsSeparateClustersAndNodes(t *testing.T) {
    first := rabbitmq.StableNodeID("cluster-a", "node-a")
    if first == "" || first == rabbitmq.StableNodeID("cluster-b", "node-a") {
        t.Fatal("node IDs must be non-empty and cluster scoped")
    }
    if strings.Contains(first, "cluster-a") || strings.Contains(first, "node-a") {
        t.Fatal("stable ID exposed raw identity")
    }
}

func TestSnapshotCloneDoesNotAliasPointers(t *testing.T) {
    snapshot := rabbitmqtest.Snapshot()
    clone := snapshot.Clone()
    *clone.Nodes[0].Connections = 999
    if *snapshot.Nodes[0].Connections == 999 { t.Fatal("clone aliases source") }
}
```

- [x] **Step 2: Run RED**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/rabbitmq ./internal/adapters/mock
```

Expected: FAIL because `internal/rabbitmq` and RabbitMQ Mock do not exist.

- [x] **Step 3: Implement focused domain types**

```go
type Cluster struct {
    ID string
    Name string
    UnreachablePeers *int64
}

type Node struct {
    ID, Name, Cluster, Address, Version string
    MemoryUsedBytes, MemoryLimitBytes *int64
    DiskAvailableBytes, DiskLimitBytes *int64
    OpenFileDescriptors, MaxFileDescriptors *int64
    ErlangProcessesUsed, ErlangProcessesLimit *int64
    Connections, Queues, Messages *int64
    PublishRate, DeliverRate *float64
    MemoryAlarm, DiskAlarm, FileDescriptorAlarm *bool
    UptimeSeconds *int64
    CollectionTracked bool
    ReportedAt time.Time
}

type Snapshot struct { Clusters []Cluster; Nodes []Node }
```

Implement SHA-256 URL-safe IDs, deep clone helpers and `ErrUnavailable`. Mock must contain at least two sanitized clusters and nodes exercising normal, warning, critical and unknown without real-looking identities or values.

- [x] **Step 4: Run GREEN and formatting check**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/rabbitmq internal/adapters/mock/rabbitmq_provider.go internal/adapters/mock/rabbitmq_provider_test.go && go test ./internal/rabbitmq ./internal/adapters/mock'
```

- [x] **Step 5: Commit if explicitly authorized**

```bash
git add internal/rabbitmq internal/adapters/mock/rabbitmq_provider.go internal/adapters/mock/rabbitmq_provider_test.go
git commit -m "feat: add RabbitMQ domain and mock provider"
```

Execution note: implementation was authorized but commit was not; Task 1 remains staged and uncommitted.

---

### Task 2: Fixed Nightingale RabbitMQ Provider

**Files:**
- Create: `internal/adapters/nightingale/rabbitmq_promql.go`
- Create: `internal/adapters/nightingale/rabbitmq_provider.go`
- Create: `internal/adapters/nightingale/rabbitmq_provider_test.go`
- Create: `internal/adapters/nightingale/testdata/rabbitmq-instant-batch.json`

**Interfaces:**
- Consumes: `rabbitmq.Snapshot` and shared Nightingale `queryInstant`.
- Produces: `func (p *Provider) RabbitMQSnapshot(context.Context) (rabbitmq.Snapshot, error)`。
- Produces: private `rabbitMQPromQL() []string` returning a defensive copy.

- [x] **Step 1: Lock the exact 22-query contract in a failing test**

```go
func TestRabbitMQPromQLContract(t *testing.T) {
    want := []string{
        "rabbitmq_identity_info", "rabbitmq_build_info",
        "rabbitmq_erlang_uptime_seconds",
        "rabbitmq_alarms_memory_used_watermark",
        "rabbitmq_alarms_free_disk_space_watermark",
        "rabbitmq_alarms_file_descriptor_limit",
        "rabbitmq_unreachable_cluster_peers_count",
        "rabbitmq_process_resident_memory_bytes",
        "rabbitmq_resident_memory_limit_bytes",
        "rabbitmq_disk_space_available_bytes",
        "rabbitmq_disk_space_available_limit_bytes",
        "rabbitmq_process_open_fds", "rabbitmq_process_max_fds",
        "rabbitmq_erlang_processes_used", "rabbitmq_erlang_processes_limit",
        "rabbitmq_connections", "rabbitmq_queues", "rabbitmq_queue_messages",
        "sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_received_total[5m]))",
        "sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_delivered_total[5m]))",
        "tlast_over_time(rabbitmq_identity_info[24h])",
        "tlast_over_time(rabbitmq_erlang_uptime_seconds[24h])",
    }
    if diff := cmp.Diff(want, rabbitMQPromQL()); diff != "" { t.Fatal(diff) }
}
```

- [x] **Step 2: Write failing HTTP fixture tests**

Use `httptest.Server` to allow exactly `/api/n9e/datasource/brief` and one `/api/n9e/query-instant-batch`. Assert the batch has 22 groups, all timestamps equal, and a second batch fails the test. Fixture cases must cover:

- permanent/logical/collection cluster fallback;
- cross-cluster same node name;
- missing `rabbitmq_node` ignored;
- protocol/queue-type rates already aggregated by the fixed query;
- newest valid sample, same-time conflict, negative/NaN/Inf/overflow rejection;
- uptime decimal/scientific notation floor;
- 401/403, redirect, non-JSON, `dat:null`, error envelope, oversized body, timeout and group-count mismatch;
- returned errors exclude fixture secret, labels, Base URL, upstream body and query text.

- [x] **Step 3: Run RED**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run RabbitMQ -count=1
```

Expected: FAIL because the fixed query list and Provider method do not exist.

- [x] **Step 4: Implement inventory-first parsing**

Define 22 `iota` indexes ending in `rabbitMQQueryCount`. Build inventory from query 21; require `rabbitmq_node`; select cluster identity in permanent/logical/collection order; hash IDs before populating domain objects. Join other series by `cluster + ident + instance`. Query 22 is the only `ReportedAt` source and sets `CollectionTracked`.

Use private helpers with explicit semantics:

```go
func rabbitMQNonNegativeInt(raw json.RawMessage) (*int64, bool)
func rabbitMQFiniteNonNegative(raw json.RawMessage) (*float64, bool)
func rabbitMQUptimeSeconds(raw json.RawMessage) (*int64, bool)
func rabbitMQAlarm(raw json.RawMessage) (*bool, bool)
func rabbitMQSampleKey(metric map[string]string) (string, bool)
```

Never expose raw permanent identity. Sort every map-derived key before snapshot emission and float aggregation.

- [x] **Step 5: Run GREEN, race and format**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/adapters/nightingale/rabbitmq_*.go && go test ./internal/adapters/nightingale -run RabbitMQ -count=1 && go test -race ./internal/adapters/nightingale -run RabbitMQ -count=1'
```

- [x] **Step 6: Commit if explicitly authorized**

```bash
git add internal/adapters/nightingale/rabbitmq_promql.go internal/adapters/nightingale/rabbitmq_provider.go internal/adapters/nightingale/rabbitmq_provider_test.go internal/adapters/nightingale/testdata/rabbitmq-instant-batch.json
git commit -m "feat: add fixed RabbitMQ Nightingale provider"
```

Execution note: implementation was authorized but commit was not; Task 2 remains staged and uncommitted. The independent rereview found no Critical or Important issues and deferred one test-coverage Minor to final review.

---

### Task 3: RabbitMQ Service, Freshness and Query Semantics

**Files:**
- Create: `internal/service/rabbitmq_types.go`
- Create: `internal/service/rabbitmq_service.go`
- Create: `internal/service/rabbitmq_service_test.go`

**Interfaces:**
- Produces: `NewRabbitMQ(provider rabbitmq.Provider, store *cache.Store, options RabbitMQOptions) *RabbitMQService`。
- Produces: `Overview(context.Context) (RabbitMQOverview, Meta, error)`。
- Produces: `Nodes(context.Context, RabbitMQQuery) (RabbitMQPage, Meta, error)`。

- [x] **Step 1: Define failing status and threshold tests**

```go
func TestRabbitMQClusterConnectivityDoesNotContaminateNodes(t *testing.T) {
    snapshot := rabbitmqtest.SnapshotWithUnreachablePeer()
    service := newRabbitMQTestService(snapshot)
    overview, _, _ := service.Overview(context.Background())
    page, _, _ := service.Nodes(context.Background(), RabbitMQQuery{})
    if overview.Clusters.Critical != 1 { t.Fatal("cluster communication must be critical") }
    if page.Nodes[0].Status != LevelNormal { t.Fatal("cluster issue contaminated node") }
}

func TestRabbitMQMessagesNeverAffectStatus(t *testing.T) {
    snapshot := rabbitmqtest.SnapshotWithDisplayOnlyMessages()
    page := mustRabbitMQPage(t, snapshot)
    if page.Nodes[0].Messages == nil || page.Nodes[0].Status != LevelNormal {
        t.Fatal("messages must display without changing status")
    }
}
```

Add table tests for exact boundaries: memory/FD/Erlang 80/90; disk margin `<=1` critical and `<1.2` warning; any explicit alarm critical; missing core state unknown; collection at 2/5 cycles; stable absolute sample time not stale; frozen samples escalate and advancing samples recover.

- [x] **Step 2: Define failing query tests**

Cover search over name/address only, exact cluster/status filters, all 15 sort names, asc/desc, missing numeric values last, stable ID tie-break, page sizes 20/50/100, invalid order/filter/sort, and `math.MaxInt` page returning `ErrInvalidQuery` without slice overflow.

- [x] **Step 3: Run RED**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run RabbitMQ -count=1
```

- [x] **Step 4: Implement service types and assessment**

```go
type RabbitMQNodeStatusSource string
const (
    RabbitMQStatusAlarm RabbitMQNodeStatusSource = "alarm"
    RabbitMQStatusCollection RabbitMQNodeStatusSource = "collection"
    RabbitMQStatusMemory RabbitMQNodeStatusSource = "memory"
    RabbitMQStatusDisk RabbitMQNodeStatusSource = "disk"
    RabbitMQStatusFileDescriptor RabbitMQNodeStatusSource = "file_descriptor"
    RabbitMQStatusErlangProcess RabbitMQNodeStatusSource = "erlang_process"
    RabbitMQStatusNormal RabbitMQNodeStatusSource = "normal"
    RabbitMQStatusUnknown RabbitMQNodeStatusSource = "unknown"
)

type RabbitMQQuery struct {
    Search, Cluster, Sort, Order string
    Status Level
    Page, PageSize int
}
```

`RabbitMQNodeSummary` contains the exact 15 display fields plus `StatusSource` and `CollectionLevel`. `RabbitMQOverview` contains module `Status`, separate cluster/node level counts, and four `RabbitMQAlertCount` groups: `ClusterConnectivity`, `ResourceAlarms`, `ResourcePressure`, `Collection`。计数类型必须保留未知，不能把 unknown 伪装成 warning：

```go
type RabbitMQAlertCount struct {
    Warning int
    Critical int
    Unknown int
}
```

Use cache key `service:rabbitmq:snapshot`, 15-second defaults and a dedicated freshness tracker keyed by stable node ID. Normalize query before loading snapshot so invalid/extreme pagination never touches a slice.

- [x] **Step 5: Run GREEN and race**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/rabbitmq_*.go && go test ./internal/service -run RabbitMQ -count=1 && go test -race ./internal/service -run RabbitMQ -count=1'
```

- [x] **Step 6: Commit if explicitly authorized**

```bash
git add internal/service/rabbitmq_types.go internal/service/rabbitmq_service.go internal/service/rabbitmq_service_test.go
git commit -m "feat: add RabbitMQ health service"
```

Execution note: implementation was authorized but commit was not; Task 3 remains staged and uncommitted. Independent rereview passed with no findings after pagination-overflow and node-sort fixes.

---

### Task 4: Read-only HTTP Views, Routes and Application Wiring

**Files:**
- Create: `internal/httpapi/rabbitmq_handlers.go`
- Create: `internal/httpapi/rabbitmq_handlers_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Produces: authenticated `GET /api/v1/rabbitmq/overview` and `GET /api/v1/rabbitmq/nodes`。
- Produces: explicit JSON snake_case views; `nodes` and `available_clusters` are non-null arrays.

- [x] **Step 1: Write failing HTTP contract tests**

Assert unauthenticated GET returns 401; authenticated valid GET returns safe envelope; overview rejects any query; nodes accepts only `search,cluster,status,sort,direction,page,page_size`; unknown/duplicate/invalid values return 400; POST/PUT/PATCH/DELETE return 405; unavailable maps to 503 without details.

Decode JSON into `map[string]any` and recursively reject keys matching:

```go
var forbidden = regexp.MustCompile(`(?i)token|cookie|authorization|base.?url|promql|query|ident|permanent|raw|label`)
```

- [x] **Step 2: Run RED**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi ./cmd/infraview -run RabbitMQ -count=1
```

- [x] **Step 3: Implement explicit views and router wiring**

```go
type rabbitMQAlertCountView struct {
    Warning int `json:"warning"`
    Critical int `json:"critical"`
    Unknown int `json:"unknown"`
}

type rabbitMQNodeView struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Cluster string `json:"cluster"`
    Address string `json:"address"`
    Version string `json:"version"`
    MemoryUsagePercent *float64 `json:"memory_usage_percent"`
    DiskAvailableBytes *int64 `json:"disk_available_bytes"`
    FileDescriptorUsagePercent *float64 `json:"file_descriptor_usage_percent"`
    ErlangProcessUsagePercent *float64 `json:"erlang_process_usage_percent"`
    Connections *int64 `json:"connections"`
    Queues *int64 `json:"queues"`
    Messages *int64 `json:"messages"`
    PublishRate *float64 `json:"publish_rate"`
    DeliverRate *float64 `json:"deliver_rate"`
    UptimeSeconds *int64 `json:"uptime_seconds"`
    Status service.Level `json:"status"`
    StatusSource service.RabbitMQNodeStatusSource `json:"status_source"`
    CollectionLevel service.Level `json:"collection_level"`
}
```

Give fields with shared Go declarations explicit JSON tags rather than relying on field names. Add optional `RabbitMQ *service.RabbitMQService` to API options, register exact routes, and inject the same Nightingale/Mock Provider chosen by `main`.

- [x] **Step 4: Run GREEN and full HTTP regression**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/httpapi/rabbitmq_*.go internal/httpapi/api.go cmd/infraview/main.go cmd/infraview/main_test.go && go test ./internal/httpapi ./cmd/infraview'
```

- [x] **Step 5: Commit if explicitly authorized**

```bash
git add internal/httpapi/rabbitmq_handlers.go internal/httpapi/rabbitmq_handlers_test.go internal/httpapi/api.go cmd/infraview/main.go cmd/infraview/main_test.go
git commit -m "feat: expose read-only RabbitMQ APIs"
```

Execution note: implementation was authorized but commit was not; Task 4 remains staged and uncommitted. Independent rereview passed after explicit HEAD rejection and targeted response-leak assertions.

---

### Task 5: Frontend API Contract and Sanitized Fixtures

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`
- Modify: `web/vite.config.ts`

**Interfaces:**
- Produces: `RabbitMQOverviewResponse`, `RabbitMQNodePageResponse`, `RabbitMQNode`, `RabbitMQNodeStatusSource`.
- Produces: Mock handlers for the two fixed GET endpoints only.

- [x] **Step 1: Add failing fixture and envelope tests**

Extend overview/page tests to import fully typed RabbitMQ fixtures. Include normal/warning/critical/unknown nodes, null display values, stale meta, empty arrays and malformed response variants. Use only synthetic names and values.

- [x] **Step 2: Define exact TypeScript contract**

```ts
export type RabbitMQNodeStatusSource =
  | 'alarm' | 'collection' | 'memory' | 'disk'
  | 'file_descriptor' | 'erlang_process' | 'normal' | 'unknown'

export interface RabbitMQNode {
  id: string
  name: string
  cluster: string
  address: string
  version: string
  memory_usage_percent: number | null
  disk_available_bytes: number | null
  file_descriptor_usage_percent: number | null
  erlang_process_usage_percent: number | null
  connections: number | null
  queues: number | null
  messages: number | null
  publish_rate: number | null
  deliver_rate: number | null
  uptime_seconds: number | null
  status: MetricLevel
  status_source: RabbitMQNodeStatusSource
  collection_level: MetricLevel
}
```

Overview has `clusters`, `nodes` level counts and alert keys `cluster_connectivity`, `resource_alarms`, `resource_pressure`, `collection`。每个 RabbitMQ alert 明确包含 `warning`、`critical`、`unknown` 三个非负整数；既有模块继续使用原 `AlertCount`，不改变其 API。

- [x] **Step 3: Run frontend contract tests**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run typecheck && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

Expected before Tasks 6–7: existing tests pass and TypeScript accepts the new contract; RabbitMQ UI behavior is not yet asserted.

- [x] **Step 4: Commit if explicitly authorized**

```bash
git add web/src/api/types.ts web/src/test/fixtures.ts web/src/test/server.ts web/vite.config.ts
git commit -m "test: add RabbitMQ frontend contracts"
```

Execution note: implementation was authorized but commit was not; Task 5 remains staged and uncommitted. A dedicated contract test was added after review and final rereview passed with no findings.

---

### Task 6: RabbitMQ Overview Card

**Files:**
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`

**Interfaces:**
- Consumes: `RabbitMQOverviewResponse`, `ModuleStatusCardShell`, `StatusBadge`, `MetricAlert`.
- Produces: independent sixth card linking to `/rabbitmq`.

- [x] **Step 1: Write failing card tests**

Assert six cards in order, shared shell, “异常节点” affected/total, severe and warning/unknown badges, the four exact labels “集群通信、资源告警、资源压力、采集状态”, and link accessible name “查看 RabbitMQ 板块”. Extend shared alert-grid rendering with an optional `unknown` count: critical wins, then warning, then unknown. When `unknown` is present, total includes it and text adds “未知 N”；when it is absent, execute the existing warning/critical branch unchanged so all five existing modules retain the same visible text.

Add tests for:

- clusters and nodes both zero -> `data-level="empty"`, “暂无 RabbitMQ 节点” and no alert grid;
- only one total zero -> normal summary remains visible;
- loading/error/stale/retry isolated from all existing cards;
- malformed enum/null array -> “服务器响应格式无效”.

- [x] **Step 2: Run RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

- [x] **Step 3: Implement minimal card and strict validator**

Use its own TanStack query key `['rabbitmq-overview']`, the shared refresh interval, and a validator that checks every level count, all three alert counts and every meta field. Add `RabbitMQ` to `ModuleLabel`. Change the local alert renderer input without changing existing API types:

```ts
type MetricAlertCount = AlertCount & { unknown?: number }
```

Treat omitted `unknown` as zero, so Linux/硬盘/MySQL/Redis/Elasticsearch behavior is unchanged. Derive:

```ts
const affectedNodes = data.nodes.warning + data.nodes.critical + data.nodes.unknown
const warningOrUnknown = data.nodes.warning + data.nodes.unknown
```

Overall level is the highest cluster/node count; do not derive node counts from cluster communication.

- [x] **Step 4: Run GREEN and overview regression**

Repeat Step 2; expected all `OverviewPage.test.tsx` tests pass.

- [x] **Step 5: Commit if explicitly authorized**

```bash
git add web/src/features/overview/OverviewPage.tsx web/src/features/overview/OverviewPage.test.tsx
git commit -m "feat: add RabbitMQ overview card"
```

Execution note: implementation was authorized but commit was not; Task 6 remains staged and uncommitted. Shared unknown-level styling was added after review and rereview passed with no findings.

---

### Task 7: Shared-template RabbitMQ Node Page and Navigation

**Files:**
- Create: `web/src/features/rabbitmq/RabbitMQPage.tsx`
- Create: `web/src/features/rabbitmq/RabbitMQPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Produces: route `/rabbitmq` and nav text `RabbitMQ` after Elasticsearch.
- Consumes: `RabbitMQNodePageResponse` and shared `ListPage` controls.

- [x] **Step 1: Write failing 15-column and shared-control tests**

Use exact headers:

```ts
const headers = [
  '节点名称', '所属集群', '实例地址', '版本', '内存使用率',
  '磁盘余量', '文件描述符使用率', 'Erlang进程使用率',
  '连接', '队列', '消息积压', '发布速率', '投递速率',
  '运行时间', '状态',
]
```

Assert every data row has 15 cells, no `<br>`, no nested secondary value, and identity cells have one clipped line plus complete `title`. Assert the page renders shared search/select/page-size/table/refresh components in one control row.

- [x] **Step 2: Write failing URL and formatting tests**

Lock sort whitelist:

```ts
const sortFields = [
  'node', 'cluster', 'address', 'version', 'memory', 'disk',
  'file_descriptors', 'erlang_processes', 'connections', 'queues',
  'messages', 'publish_rate', 'deliver_rate', 'uptime', 'status',
] as const
```

Assert 300ms search debounce, cluster/status/page size resets to page 1, refresh restores URL, invalid URL values normalize, null uses “暂无数据”, percentages/IEC/rates are single values, and uptime formats `x天 x小时`.

- [x] **Step 3: Run RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/rabbitmq/RabbitMQPage.test.tsx src/app/AppShell.test.tsx src/components/ListPage.test.tsx'
```

- [x] **Step 4: Implement the page without a parallel UI system**

Use `useSearchParams`, `useQuery`, `useReactTable` and the existing `ListPageHeader`, `ListPageControls`, `ListSearchField`, `ListSelectField`, `ListPageSizeField`, `ListTablePanel`. Add only table-specific width selectors under `.rabbitmq-table`; do not create custom search/select/refresh classes.

The status cell maps source and collection level to existing `StatusBadge`. Use `Intl.NumberFormat` for counts/rates and existing IEC/uptime formatting conventions; do not combine publish/deliver or other values into one cell.

- [x] **Step 5: Run GREEN, typecheck and build**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/rabbitmq/RabbitMQPage.test.tsx src/app/AppShell.test.tsx src/components/ListPage.test.tsx && npm run typecheck && npm run build'
```

- [x] **Step 6: Commit if explicitly authorized**

```bash
git add web/src/features/rabbitmq web/src/app/App.tsx web/src/app/AppShell.tsx web/src/app/AppShell.test.tsx web/src/app/theme.css
git commit -m "feat: add RabbitMQ node page"
```

Execution note: implementation was authorized but commit was not; Task 7 remains staged and uncommitted. Independent rereview passed; medium-width browser geometry with long synthetic cluster names remains a Task 8 acceptance check.

---

### Task 8: Browser Contract and Durable Documentation

**Files:**
- Create: `web/e2e/rabbitmq.spec.ts`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Produces: static/dynamic Playwright coverage and a recoverable `/new` handoff.

- [x] **Step 1: Add Playwright specifications**

Test navigation from sidebar and sixth card; exact 15 headers; search/cluster/status/sort/page URL; no destructive controls; 1440×900 page/table no horizontal overflow; every data cell `white-space: nowrap`, no `<br>`, compact equal row heights, identity ellipsis with title, and representative short values not clipped.

Do not assert live counts, addresses, capacities or metric values. Use exact text locators where repeated substrings exist.

- [x] **Step 2: Run static discovery only**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npx playwright test --list'
```

Expected: RabbitMQ tests are listed; no server or port is created.

- [x] **Step 3: Update durable docs**

Record the exact 22 queries, identity fallback, 15 columns, thresholds, cluster/node separation, queue-label limitation, test evidence, authorization boundaries and recovery files. Update HANDOFF current entry without deleting historical Redis/Elasticsearch evidence.

- [x] **Step 4: Validate docs and commit if explicitly authorized**

```bash
git diff --check
git add README.md docs web/e2e/rabbitmq.spec.ts
git commit -m "docs: record RabbitMQ monitoring module"
```

Execution note: Task 8 文档校验与 `git diff --check` 已完成；当前用户未授权 commit，因此未执行 commit。Task 8 文件随后由主控统一暂存，提交仍待单独明确授权。

---

### Task 9: Full Verification, Review Gates and Existing 8080

**Files:**
- Modify only if a verified defect is found; each defect requires its own RED/GREEN cycle.

**Interfaces:**
- Produces: fresh verification evidence for the complete tree.

- [x] **Step 1: Run full frontend verification**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

- [x] **Step 2: Run Go format, vet, ordinary, race and binary build**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'test -z "$(gofmt -l cmd internal)" && go vet ./... && go test ./... && go test -race ./... && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /tmp/infraview ./cmd/infraview'
```

- [x] **Step 3: Run security and whitespace checks**

Search production Go/TS for RabbitMQ command/proxy/arbitrary-query surfaces; recursively inspect HTTP JSON keys for forbidden identity fields; confirm `.env` remains ignored and unstaged; run `git diff --check` including an explicit untracked-file whitespace scan.

- [x] **Step 4: Build a no-cache production image**

```bash
docker build --no-cache --tag infraview:rabbitmq-verify .
```

The image is created but not run, publishes no port and contacts no upstream.

- [x] **Step 5: Request review and fix only evidence-backed findings**

Review the complete RabbitMQ diff against the approved spec. Any defect must first receive a focused failing test, then a minimal fix, targeted GREEN and renewed full verification. Do not treat stale test assumptions as product defects.

Execution note: 2026-08-05 fresh 验证已完成。前端 14 文件/209 项、typecheck、production build 与 Playwright 静态发现 4 文件/21 项（RabbitMQ 4 项）均退出 0；Go gofmt、vet、普通/race 测试和静态编译均退出 0；安全与 whitespace 检查通过；无缓存生产镜像构建退出 0且未运行。终审 Important 的 query 7 集群聚合已按 RED→GREEN 修复并通过复审，Clone、query 21、overview validator 与四色 E2E Minor 均已闭环。npm lock 仍报告既有 1 个 moderate 与 2 个 high，本轮未修改依赖文件，也未执行 `audit fix`。

- [x] **Step 6: Deployment authorization gate**

Only after explicit authorization, reuse the existing Compose project and existing 8080:

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env INFRAVIEW_PORT=8080 docker compose --project-name infraview up -d --build --force-recreate infraview
```

Verify healthy, single service, only 8080, `10001:10001`, read-only root filesystem, cap drop `ALL`, `no-new-privileges`, two RabbitMQ GET APIs, 405 writes, existing modules, and one-time Chromium RabbitMQ specs. Output booleans only; never output live values or bodies.

Execution note: 部署授权门已评估；当前用户未授权动态 Chromium 1440/1100、现有 8080 原位重建或 deploy，因此到门即停止，上述命令和验收均未执行。

- [x] **Step 7: Commit/push authorization gates**

Before commit: fresh full verification, `git diff --check`, exact staged file review and private-file exclusion. Commit only if explicitly authorized. Fetch and confirm `origin/main...main` is behind 0 before push; never force-push. Push only if separately explicitly authorized.

Execution note: commit/push 授权门已评估；当前用户未授权 commit 或 push，因此未执行 commit、fetch-for-push 或 push。

---

### Task 10: Preserve Multiple RabbitMQ Nodes Behind One Collection Target

**Files:**
- Modify: `internal/adapters/nightingale/rabbitmq_provider.go`
- Modify: `internal/adapters/nightingale/rabbitmq_provider_test.go`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Consumes: query 21 inventory labels, especially cluster identity and `rabbitmq_node`, plus the collection key `cluster + ident + instance`.
- Produces: one snapshot node per stable cluster/node identity even when several nodes share one collection target; ambiguous label-less metrics remain missing instead of being copied.

- [x] **Step 1: Write the focused failing regression**

Add a provider test whose query 21 inventory contains multiple different `rabbitmq_node` values with the same collection key and different original sample times. Assert that every stable node identity remains in the snapshot. Add metric series with `rabbitmq_node` to prove direct node joins, and one label-less ambiguous series to prove it does not contaminate multiple nodes.

- [x] **Step 2: Run focused RED**

```bash
go test ./internal/adapters/nightingale -run 'TestRabbitMQInventoryPreservesMultipleNodesBehindOneCollectionTarget' -count=1
```

Expected: FAIL because the current `byJoinKey` inventory map retains only one identity.

- [x] **Step 3: Implement the minimal identity-aware join**

Build inventory primarily by stable node ID, retain a secondary collection-key index, and store snapshot node state by node ID. Metrics carrying `rabbitmq_node` join through cluster identity plus node name; metrics without that label join only when their collection key maps to exactly one inventory node. Keep same-time identity/value conflicts unknown and do not clone an ambiguous metric across nodes.

- [x] **Step 4: Run focused GREEN and RabbitMQ package tests**

```bash
go test ./internal/adapters/nightingale -run 'TestRabbitMQ' -count=1
go test ./internal/rabbitmq ./internal/service ./internal/httpapi -run 'RabbitMQ' -count=1
```

- [x] **Step 5: Update durable evidence and run full verification**

Record the shared-collection-target contract and RED/GREEN evidence. Run Go formatting, vet, ordinary/race tests, frontend tests/typecheck/build, Playwright static discovery, security checks and `git diff --check`.

- [x] **Step 6: Rebuild only the existing authorized 8080 service**

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env INFRAVIEW_PORT=8080 docker compose --project-name infraview up -d --build --force-recreate infraview
```

Verify health, unique 8080 publication and security booleans without printing credentials, upstream bodies, identities, addresses, counts, capacities or metric values. Commit and push remain separately unauthorized.

Execution note: shared-target inventory and node-aware rate-query tests both completed RED→GREEN. The existing 8080 service was rebuilt in place and passed health, unique-port, non-root, read-only-rootfs, dropped-capability, no-new-privileges and unauthenticated API-boundary checks. Commit and push were not executed.

- [x] **Step 7: Normalize RabbitMQ zero-risk overview summaries**

Add a focused overview test requiring all four RabbitMQ alert boxes to show normal-level `无异常` when warning, critical and unknown are all zero. Verify RED against the old three-part zero text, then make `MetricAlert` prefer `无异常` whenever its computed total is zero. Keep nonzero two-part and three-part details unchanged and rerun the full frontend suite.

---

### Task 11: Discover Missing Instances Without Fabricating Node Names

**Files:**
- Modify: `internal/adapters/nightingale/rabbitmq_provider.go`
- Modify: `internal/adapters/nightingale/rabbitmq_provider_test.go`
- Modify: `web/src/features/rabbitmq/RabbitMQPage.tsx`
- Modify: `web/src/features/rabbitmq/RabbitMQPage.test.tsx`
- Modify: RabbitMQ durable documentation

- [x] **Step 1: Reproduce both identity failures**

Add sanitized Provider regressions proving that connection-discovered nodes must not use their instance address as a name and that a unique explicit `rabbitmq_node` elsewhere in the same fixed batch can enrich the node. Verify both fail against the old fallback.

- [x] **Step 2: Implement deterministic name enrichment**

Merge current and recent identity series, use connection series only to discover otherwise missing instances, and scan the same fixed batch for a unique consistent `rabbitmq_node` per `cluster + instance`. Reject conflicting hints. When no explicit name exists, keep `Name` empty and generate only an internal irreversible observed-node ID; never derive a name from address, `ident`, DNS or a naming pattern.

- [x] **Step 3: Make missing names explicit in the UI**

Add a page regression that first fails on a blank cell, then render `暂无数据` in the node-name column while retaining the address solely in its own column. Preserve one-value/one-line layout and title behavior.

- [x] **Step 4: Verify and rebuild the existing authorized 8080**

Run focused RED→GREEN, RabbitMQ backend and page suites, then rebuild the existing 8080. The image reruns frontend full tests/typecheck/build and Go ordinary/race/build. Verify container health, existing 8080, removed incorrect fallback and staged/unstaged whitespace without reading live bodies or private configuration.

Execution note: Task 11 completed. The user explicitly authorized commit and push after the existing 8080 rebuild; dynamic authenticated Chromium remains unexecuted.
