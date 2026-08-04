# InfraView Elasticsearch Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 InfraView 中交付严格只读、支持多集群的 Elasticsearch 总览健康卡与 16 列节点列表。

**Architecture:** 新建独立 `internal/elasticsearch` 领域与 `ElasticsearchService`，由现有 Nightingale Provider 一次固定 26 查询 batch 生成同一份集群/节点快照。HTTP 仅提供两个认证 GET API；前端强制复用 `ModuleStatusCardShell` 与 `ListPage`，保持现有四列总览网格并让第五张 Elasticsearch 卡自然换行。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Testing Library、Playwright 1.61、Docker/Docker Compose、Nightingale v8.4.1 只读 batch API。

## Global Constraints

- 工作目录固定为 `/root/github/InfraView`，分支固定为 `main`；开始每个任务前检查并保留用户现有差异，禁止 reset、restore、checkout、clean 或覆盖。
- InfraView 始终只读；不得直连 Elasticsearch，不得执行 Elasticsearch API、命令、脚本、SSH、远程命令或任何运维写操作。
- 开发 8080 永远只连接现有测试 Nightingale；禁止连接、切换或探测生产 Nightingale，禁止创建其他 InfraView 测试端口。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实集群/节点标识、地址、资源数量、容量、指标值或上游正文。
- Nightingale 查询必须是代码内固定 26 条 MetricsQL，一次 `query-instant-batch`，无集群/节点 N+1；API 和前端不得传入指标名、PromQL、URL 或原始请求体。
- 首期只做总览 Elasticsearch 卡与 `/elasticsearch` 节点列表；不做索引列表/详情、节点详情、历史、拓扑、分片明细、慢查询、日志或追踪。
- 集群状态与节点状态严格分离；集群黄/红不得批量升级所属节点状态。
- 节点表固定 16 列；每个单元格只有一个单行值，不使用 `<br>`、副指标堆叠或省略号截断。宽度不足时只允许表格区域横向滚动。
- Elasticsearch 必须复用 `ListPage` 与 `ModuleStatusCardShell`；不得复制模块专用搜索框、下拉框、刷新区、空状态或分页结构。
- 全部开发和验证使用 Docker；宿主机不安装或运行 Go/Node 服务。
- 不运行会创建 18080 的 `scripts/e2e.sh`；只允许 `playwright test --list` 静态发现，或经单独授权后让一次性浏览器访问现有 8080。
- 实现授权、每个任务的 commit、push 与现有 8080 重建是独立授权。下方 commit 步骤仅在执行前已取得提交授权时执行；任何任务都不得自行 push。

---

## File Structure

### New Go domain and adapters

- `internal/elasticsearch/provider.go`：只读 Provider 接口和安全领域错误。
- `internal/elasticsearch/types.go`：集群、节点、角色、快照、稳定 ID 与深拷贝。
- `internal/elasticsearch/contract_test.go`：领域级稳定身份和拷贝测试。
- `internal/elasticsearch/elasticsearchtest/contract.go`：Provider 共享契约测试。
- `internal/adapters/mock/elasticsearch_provider.go`：确定性多集群 Mock。
- `internal/adapters/mock/elasticsearch_provider_test.go`：Mock 契约与状态场景。
- `internal/adapters/nightingale/elasticsearch_promql.go`：固定 26 查询及索引常量。
- `internal/adapters/nightingale/elasticsearch_provider.go`：一次 batch 解析、归并和安全校验。
- `internal/adapters/nightingale/elasticsearch_provider_test.go`：固定查询、协议、归并与错误测试。
- `internal/adapters/nightingale/testdata/elasticsearch-instant-batch.json`：完全脱敏的 26 组响应夹具。

### New service and HTTP layer

- `internal/service/elasticsearch_types.go`：查询、Overview、节点摘要、状态来源和分页类型。
- `internal/service/elasticsearch_service.go`：缓存、新鲜度、状态、汇总、筛选、排序与分页。
- `internal/service/elasticsearch_service_test.go`：Service 全部边界测试。
- `internal/httpapi/elasticsearch_handlers.go`：显式 View、两个 GET handler 与参数解析。
- `internal/httpapi/elasticsearch_handlers_test.go`：响应契约、安全错误与方法测试。

### Existing Go integration points

- `cmd/infraview/main.go`：Provider set、超时包装、Service 构造与 HTTP 依赖注入。
- `cmd/infraview/main_test.go`：Mock/Nightingale 依赖和超时测试。
- `internal/httpapi/api.go`：依赖字段、路由与 405 fallback。

### New and existing frontend files

- `web/src/features/elasticsearch/ElasticsearchPage.tsx`：共享模板上的 16 列节点页。
- `web/src/features/elasticsearch/ElasticsearchPage.test.tsx`：列、单值、控件、URL 和状态测试。
- `web/e2e/elasticsearch.spec.ts`：路由、布局、单行、内部滚动与只读静态规格。
- `web/src/api/types.ts`：Elasticsearch View 对应类型。
- `web/src/test/fixtures.ts`：与 Handler 同形状的完全脱敏夹具。
- `web/src/test/server.ts`：MSW 两个 GET handler。
- `web/src/features/overview/OverviewPage.tsx`：Elasticsearch 查询与共享卡。
- `web/src/features/overview/OverviewPage.test.tsx`：第五卡、响应解析和状态测试。
- `web/src/app/App.tsx`：`/elasticsearch` 路由。
- `web/src/app/AppShell.tsx`、`web/src/app/AppShell.test.tsx`：侧边栏入口。
- `web/src/app/theme.css`：只增加 Elasticsearch 表格宽度/滚动规则，不复制公共控件样式。

### Documentation updates after implementation

- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/DESIGN.md`
- `docs/SECURITY.md`
- `docs/TESTING.md`
- `docs/datasources/NIGHTINGALE.md`
- `docs/PROJECT_STATUS.md`
- `docs/TODO.md`
- `docs/HANDOFF.md`
- `docs/superpowers/specs/2026-08-01-elasticsearch-module-design.md`
- `docs/superpowers/plans/2026-08-01-elasticsearch-module.md`

---

### Task 1: Elasticsearch Domain Contract and Deterministic Mock

**Files:**
- Create: `internal/elasticsearch/provider.go`
- Create: `internal/elasticsearch/types.go`
- Create: `internal/elasticsearch/contract_test.go`
- Create: `internal/elasticsearch/elasticsearchtest/contract.go`
- Create: `internal/adapters/mock/elasticsearch_provider.go`
- Create: `internal/adapters/mock/elasticsearch_provider_test.go`

**Interfaces:**
- Produces: `elasticsearch.Provider` with `ElasticsearchSnapshot(context.Context) (elasticsearch.Snapshot, error)`.
- Produces: `elasticsearch.Snapshot{Clusters []Cluster, Nodes []Node}` with a deep `Clone()`.
- Produces: `StableClusterID(cluster string) string` and `StableNodeID(cluster, name string) string`.
- Produces: canonical `Health`, `Availability`, and `Role` values consumed by Tasks 2–4.

- [x] **Step 1: Write failing domain identity and clone tests**

```go
func TestStableIDsUseOnlyDomainIdentity(t *testing.T) {
	clusterID := StableClusterID("fixture-cluster-a")
	if clusterID == "" || clusterID != StableClusterID("fixture-cluster-a") {
		t.Fatal("cluster ID must be deterministic")
	}
	if clusterID == StableClusterID("fixture-cluster-b") {
		t.Fatal("different clusters must have different IDs")
	}
	nodeID := StableNodeID("fixture-cluster-a", "fixture-node-a")
	if nodeID == "" || nodeID != StableNodeID("fixture-cluster-a", "fixture-node-a") {
		t.Fatal("node ID must be deterministic")
	}
}

func TestSnapshotCloneDeepCopiesRolesAndPointers(t *testing.T) {
	original := Snapshot{Nodes: []Node{{
		ID: StableNodeID("fixture-cluster-a", "fixture-node-a"),
		Roles: []Role{RoleMaster, RoleData},
		HeapUsedBytes: int64Pointer(40),
	}}}
	clone := original.Clone()
	clone.Nodes[0].Roles[0] = RoleIngest
	*clone.Nodes[0].HeapUsedBytes = 90
	if original.Nodes[0].Roles[0] != RoleMaster || *original.Nodes[0].HeapUsedBytes != 40 {
		t.Fatal("clone mutated original")
	}
}
```

- [x] **Step 2: Run the focused test and verify RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/elasticsearch -count=1
```

Expected: FAIL because `Snapshot`, stable ID functions and roles do not exist.

- [x] **Step 3: Implement the minimal domain types and provider contract**

```go
var ErrUnavailable = errors.New("elasticsearch data source: unavailable")

type Provider interface {
	ElasticsearchSnapshot(context.Context) (Snapshot, error)
}

type Health string
const (
	HealthGreen Health = "green"
	HealthYellow Health = "yellow"
	HealthRed Health = "red"
	HealthUnknown Health = "unknown"
)

type Availability string
const (
	AvailabilityUp Availability = "up"
	AvailabilityDown Availability = "down"
	AvailabilityUnknown Availability = "unknown"
)

type Role string
const (
	RoleMaster Role = "master"
	RoleData Role = "data"
	RoleDataContent Role = "data_content"
	RoleDataHot Role = "data_hot"
	RoleDataWarm Role = "data_warm"
	RoleDataCold Role = "data_cold"
	RoleDataFrozen Role = "data_frozen"
	RoleIngest Role = "ingest"
	RoleML Role = "ml"
	RoleTransform Role = "transform"
	RoleRemoteClusterClient Role = "remote_cluster_client"
	RoleClient Role = "client"
)
```

Use these exact domain fields:

```go
type Cluster struct {
	ID                    string
	Name                  string
	Availability          Availability
	NodeStatsAvailability Availability
	Health                Health
	NumberOfNodes         *int64
	NumberOfDataNodes     *int64
	ActivePrimaryShards   *int64
	ActiveShards          *int64
	RelocatingShards      *int64
	InitializingShards    *int64
	UnassignedShards      *int64
	PendingTasks          *int64
	TaskMaxWaitingMillis  *int64
	CollectionTracked     bool
	ReportedAt            time.Time
}

type Node struct {
	ID                 string
	Name               string
	Cluster            string
	Address            string
	Roles              []Role
	HeapUsedBytes      *int64
	HeapMaxBytes       *int64
	DiskUsagePercent   *float64
	CPUUsagePercent    *float64
	IndexRate          *float64
	SearchRate         *float64
	Documents          *int64
	StoreSizeBytes     *int64
	ThreadPoolQueue    *int64
	RejectedRate       *float64
	UptimeSeconds      *int64
	DataNode           bool
	CollectionTracked  bool
	ReportedAt         time.Time
}
```

`StableClusterID` and `StableNodeID` normalize identity, hash it with SHA-256 and encode it with raw URL-safe base64. They must never include `ident`, `instance`, `url`, address or `cluster_uuid`.

- [x] **Step 4: Add the shared Provider contract and deterministic Mock**

```go
func RunContract(t *testing.T, provider elasticsearch.Provider) {
	t.Helper()
	first, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil { t.Fatalf("snapshot: %v", err) }
	second, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil { t.Fatalf("second snapshot: %v", err) }
	if len(first.Clusters) == 0 || len(first.Nodes) == 0 {
		t.Fatal("contract requires clusters and nodes")
	}
	first.Nodes[0].Roles[0] = elasticsearch.RoleIngest
	if reflect.DeepEqual(first, second) {
		t.Fatal("provider returned shared mutable state")
	}
}
```

Mock data must use only documentation ranges (`192.0.2.0/24`, `198.51.100.0/24`) and fixture names. Include multiple clusters, a multi-role data node, a dedicated master-like node, green/yellow/red cluster states, resource warning/critical cases, rejection warning and missing optional values. Do not copy any live identity or value.

- [x] **Step 5: Run Task 1 tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/elasticsearch ./internal/adapters/mock -run Elasticsearch -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1 if commit authorization is active**

```bash
git add internal/elasticsearch internal/adapters/mock/elasticsearch_provider.go internal/adapters/mock/elasticsearch_provider_test.go
git commit -m "feat: add Elasticsearch domain and mock provider"
```

---

### Task 2: Fixed 26-Query Nightingale Snapshot Provider

**Files:**
- Create: `internal/adapters/nightingale/elasticsearch_promql.go`
- Create: `internal/adapters/nightingale/elasticsearch_provider.go`
- Create: `internal/adapters/nightingale/elasticsearch_provider_test.go`
- Create: `internal/adapters/nightingale/testdata/elasticsearch-instant-batch.json`
- Test: `internal/elasticsearch/elasticsearchtest/contract.go`

**Interfaces:**
- Consumes: `elasticsearch.Provider`, `Snapshot`, `Cluster`, `Node`, stable ID and role constants from Task 1.
- Produces: `func (provider *Provider) ElasticsearchSnapshot(context.Context) (elasticsearch.Snapshot, error)`.
- Produces: `elasticsearchPromQL() []string` returning a defensive copy of exactly 26 fixed expressions.

- [x] **Step 1: Write failing fixed-query and one-batch tests**

```go
func TestElasticsearchSnapshotUsesOneFixedBatch(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/n9e/datasource/brief":
			serveFixture(t, w, "testdata/datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			assertElasticsearchQueryBody(t, r, fixedElasticsearchPromQL[:])
			serveFixture(t, w, "testdata/elasticsearch-instant-batch.json")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := newFixtureProvider(t, server.URL)
	snapshot, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil { t.Fatalf("snapshot: %v", err) }
	if len(snapshot.Clusters) == 0 || len(snapshot.Nodes) == 0 { t.Fatal("empty snapshot") }
	if diff := cmp.Diff([]string{"/api/n9e/datasource/brief", "/api/n9e/query-instant-batch"}, paths); diff != "" {
		t.Fatalf("paths mismatch (-want +got):\n%s", diff)
	}
}
```

Also add table tests for: result group count mismatch, nil inventory groups, invalid timestamp/value, non-binary up, invalid health/color code, unsupported role, same-timestamp conflict, address conflict, duplicate collectors, auxiliary series not creating identities, overflow-safe sums and upstream safe errors.

- [x] **Step 2: Run focused Provider tests and verify RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run Elasticsearch -count=1
```

Expected: FAIL because Elasticsearch PromQL and Provider methods do not exist.

- [x] **Step 3: Define exact query indexes and fixed expressions**

`elasticsearch_promql.go` uses `iota` constants in this exact order, ending with `elasticsearchQueryCount`:

```go
var fixedElasticsearchPromQL = [...]string{
	`elasticsearch_clusterinfo_up`,
	`elasticsearch_node_stats_up`,
	`elasticsearch_cluster_health_status`,
	`elasticsearch_cluster_health_number_of_nodes`,
	`elasticsearch_cluster_health_number_of_data_nodes`,
	`elasticsearch_cluster_health_active_primary_shards`,
	`elasticsearch_cluster_health_active_shards`,
	`elasticsearch_cluster_health_relocating_shards`,
	`elasticsearch_cluster_health_initializing_shards`,
	`elasticsearch_cluster_health_unassigned_shards`,
	`elasticsearch_cluster_health_number_of_pending_tasks`,
	`elasticsearch_cluster_health_task_max_waiting_in_queue_millis`,
	`elasticsearch_nodes_roles`,
	`elasticsearch_jvm_memory_used_bytes{area="heap"}`,
	`elasticsearch_jvm_memory_max_bytes{area="heap"}`,
	`max by (cluster, name, host, ident, instance, es_client_node, es_data_node, es_ingest_node, es_master_node) (100 * (1 - elasticsearch_filesystem_data_available_bytes / elasticsearch_filesystem_data_size_bytes))`,
	`elasticsearch_process_cpu_percent`,
	`rate(elasticsearch_indices_indexing_index_total[5m])`,
	`rate(elasticsearch_indices_search_query_total[5m])`,
	`elasticsearch_indices_docs`,
	`elasticsearch_indices_store_size_bytes`,
	`elasticsearch_jvm_uptime_seconds`,
	`max by (cluster, name, host, ident, instance) (elasticsearch_thread_pool_queue_count)`,
	`sum by (cluster, name, host, ident, instance) (rate(elasticsearch_thread_pool_rejected_count[5m]))`,
	`tlast_over_time(elasticsearch_clusterinfo_up[24h])`,
	`tlast_over_time(elasticsearch_jvm_uptime_seconds[24h])`,
}
```

`elasticsearchPromQL()` returns a copied slice so callers cannot mutate the fixed array.

```go
func elasticsearchPromQL() []string {
	queries := make([]string, len(fixedElasticsearchPromQL))
	copy(queries, fixedElasticsearchPromQL[:])
	return queries
}
```

- [x] **Step 4: Implement inventory-first parsing and conflict-safe merge**

Build cluster states only from query 25 and node states only from query 26. Merge current queries into existing states by normalized `cluster` or `cluster + name`; never expose or use `ident`, `instance`, `url`, `cluster_uuid` as domain identity. Use typed merge states that retain latest valid timestamp and set a conflict flag when equal timestamps carry different values.

Health accepts only `green=1`, `yellow=2`, `red=3`. Role accepts only:

```text
client, data, data_cold, data_content, data_frozen, data_hot, data_warm,
ingest, master, ml, remote_cluster_client, transform
```

Sort role output with `master`, data roles, `ingest`, `ml`, `transform`, `remote_cluster_client`, `client`. Set `DataNode=true` for `data` or any `data_*` role. JVM percent is not calculated in the Provider; retain valid used/max values for Service assessment. Return only `fmt.Errorf("%w: Nightingale Elasticsearch 当前指标不可用", elasticsearch.ErrUnavailable)` on unsafe upstream/shape errors.

- [x] **Step 5: Add a completely sanitized 26-group fixture**

The fixture envelope must be:

```json
{"dat":[[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[],[]],"err":""}
```

Replace each group with at least the series needed to represent multiple fixture clusters and nodes. Labels may use only `fixture-cluster-*`, `fixture-node-*`, documentation IPs, `fixture-ident-*` and role constants above. Include duplicate collector series with equal values, current and 24-hour inventory timestamps, valid null-producing gaps, and no live-derived values.

- [x] **Step 6: Run Provider and contract tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale ./internal/elasticsearch -run Elasticsearch -count=1
```

Expected: PASS and the test server records exactly one instant batch.

- [ ] **Step 7: Commit Task 2 if commit authorization is active**

```bash
git add internal/adapters/nightingale/elasticsearch_promql.go internal/adapters/nightingale/elasticsearch_provider.go internal/adapters/nightingale/elasticsearch_provider_test.go internal/adapters/nightingale/testdata/elasticsearch-instant-batch.json
git commit -m "feat: add Nightingale Elasticsearch snapshot provider"
```

---

### Task 3: Elasticsearch Service, Status and Server-Side Querying

**Files:**
- Create: `internal/service/elasticsearch_types.go`
- Create: `internal/service/elasticsearch_service.go`
- Create: `internal/service/elasticsearch_service_test.go`
- Reuse: `internal/service/freshness.go`
- Reuse: `internal/cache/store.go`

**Interfaces:**
- Consumes: `elasticsearch.Provider` and immutable snapshot clones from Tasks 1–2.
- Produces: `NewElasticsearch(provider elasticsearch.Provider, store *cache.Store, options ElasticsearchOptions) *ElasticsearchService`.
- Produces: `Overview(context.Context) (ElasticsearchOverview, Meta, error)`.
- Produces: `Nodes(context.Context, ElasticsearchQuery) (ElasticsearchPage, Meta, error)`.

- [x] **Step 1: Write failing status boundary tests**

```go
func TestElasticsearchNodeStatusThresholds(t *testing.T) {
	tests := []struct {
		name string
		mutate func(*elasticsearch.Node)
		level Level
		source ElasticsearchNodeStatusSource
	}{
		{"heap warning", func(n *elasticsearch.Node) { n.HeapUsedBytes = int64Ptr(75); n.HeapMaxBytes = int64Ptr(100) }, LevelWarning, ElasticsearchNodeStatusJVM},
		{"heap critical", func(n *elasticsearch.Node) { n.HeapUsedBytes = int64Ptr(85); n.HeapMaxBytes = int64Ptr(100) }, LevelCritical, ElasticsearchNodeStatusJVM},
		{"disk warning", func(n *elasticsearch.Node) { n.DiskUsagePercent = floatPtr(85) }, LevelWarning, ElasticsearchNodeStatusDisk},
		{"disk critical", func(n *elasticsearch.Node) { n.DiskUsagePercent = floatPtr(90) }, LevelCritical, ElasticsearchNodeStatusDisk},
		{"rejection warning", func(n *elasticsearch.Node) { n.RejectedRate = floatPtr(0.01) }, LevelWarning, ElasticsearchNodeStatusThreadPool},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := healthyElasticsearchNode()
			test.mutate(&node)
			got := summarizeElasticsearchNode(node, LevelNormal)
			if got.Status != test.level || got.StatusSource != test.source {
				t.Fatalf("status/source = %q/%q, want %q/%q", got.Status, got.StatusSource, test.level, test.source)
			}
		})
	}
}
```

Add cluster tests for up/down/unknown, green/yellow/red, node collector failure, same-level source priority, and explicit proof that cluster health never changes node status. Add 2/5-cycle tests for cluster and node progress, cache hit not advancing samples, stale fallback deep copy, process-baseline behavior and sample timestamp rollback.

- [x] **Step 2: Write failing query/pagination tests**

Cover combined search, exact cluster, role membership, cluster health, node status, all 16 sort fields, missing-last in both orders, deterministic ID tie-break, 20/50/100 page size, invalid values and `math.MaxInt` page offset returning `ErrInvalidQuery` instead of panicking.

```go
func TestElasticsearchNodesRejectsOverflowPageOffset(t *testing.T) {
	service := newElasticsearchServiceWithSnapshot(fixtureSnapshot())
	_, _, err := service.Nodes(context.Background(), ElasticsearchQuery{
		Sort: "node", Order: "asc", Page: math.MaxInt, PageSize: 20,
	})
	if !errors.Is(err, ErrInvalidQuery) { t.Fatalf("err = %v", err) }
}
```

- [x] **Step 3: Run focused Service tests and verify RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run Elasticsearch -count=1
```

Expected: FAIL because the Elasticsearch Service and types do not exist.

- [x] **Step 4: Implement Service types and status assessments**

```go
type ElasticsearchQuery struct {
	Search string
	Cluster string
	Role elasticsearch.Role
	ClusterHealth elasticsearch.Health
	Status Level
	Sort string
	Order string
	Page int
	PageSize int
}

type ElasticsearchNodeStatusSource string
const (
	ElasticsearchNodeStatusCollection ElasticsearchNodeStatusSource = "collection"
	ElasticsearchNodeStatusDisk ElasticsearchNodeStatusSource = "disk"
	ElasticsearchNodeStatusJVM ElasticsearchNodeStatusSource = "jvm"
	ElasticsearchNodeStatusThreadPool ElasticsearchNodeStatusSource = "thread_pool"
	ElasticsearchNodeStatusNormal ElasticsearchNodeStatusSource = "normal"
	ElasticsearchNodeStatusUnknown ElasticsearchNodeStatusSource = "unknown"
)

type ElasticsearchClusterStatusSource string
const (
	ElasticsearchClusterStatusAvailability ElasticsearchClusterStatusSource = "availability"
	ElasticsearchClusterStatusHealth ElasticsearchClusterStatusSource = "health"
	ElasticsearchClusterStatusCollection ElasticsearchClusterStatusSource = "collection"
	ElasticsearchClusterStatusNormal ElasticsearchClusterStatusSource = "normal"
	ElasticsearchClusterStatusUnknown ElasticsearchClusterStatusSource = "unknown"
)

type ElasticsearchLevelCounts struct {
	Total int
	Normal int
	Warning int
	Critical int
	Unknown int
}

type ElasticsearchAlertCount struct {
	Warning int
	Critical int
}

type ElasticsearchOverviewAlerts struct {
	ClusterHealth ElasticsearchAlertCount
	NodeResource ElasticsearchAlertCount
	UnassignedShards ElasticsearchAlertCount
	RequestRejections ElasticsearchAlertCount
}

type ElasticsearchOverview struct {
	Status Level
	Clusters ElasticsearchLevelCounts
	Nodes ElasticsearchLevelCounts
	Alerts ElasticsearchOverviewAlerts
}

type ElasticsearchNodeSummary struct {
	ID string
	Name string
	Cluster string
	Address string
	Roles []elasticsearch.Role
	ClusterHealth elasticsearch.Health
	HeapUsagePercent *float64
	DiskUsagePercent *float64
	CPUUsagePercent *float64
	IndexRate *float64
	SearchRate *float64
	Documents *int64
	StoreSizeBytes *int64
	ThreadPoolQueue *int64
	RejectedRate *float64
	UptimeSeconds *int64
	Status Level
	StatusSource ElasticsearchNodeStatusSource
	CollectionLevel Level
}

type ElasticsearchPage struct {
	Nodes []ElasticsearchNodeSummary
	AvailableClusters []string
	AvailableRoles []elasticsearch.Role
	Total int
	Page int
	PageSize int
}
```

Use exact source priority and thresholds from the spec. `ClusterHealth` counts yellow clusters as warning and red clusters as critical. `NodeResource` counts nodes whose winning source is disk or JVM. `UnassignedShards` sums valid unassigned shard counts into warning for yellow clusters and critical for red clusters without changing status. `RequestRejections` counts affected nodes as warning and always has zero critical because no critical rejection threshold is defined. CPU, queue, workload, document/store and uptime fields never change status. Data nodes missing disk and all nodes missing valid heap become unknown only when no higher issue exists.

- [x] **Step 5: Implement shared snapshot cache and two freshness trackers**

Use cache key `service:elasticsearch:snapshot`. The loader calls Provider once and stores a cloned `elasticsearch.Snapshot`; cache return and stale fallback must also clone. Observe cluster and node `ReportedAt` only on successful Provider loads, not cache hits. Derive cluster document/store/rates from valid node summaries with overflow checks.

- [x] **Step 6: Implement validation, filtering, sorting and safe pagination**

Normalize defaults to `sort=node`, `order=asc`, `page=1`, `page_size=20`. Validate exact enums and page sizes. Before offset multiplication:

```go
maxInt := int(^uint(0) >> 1)
if query.Page-1 > maxInt/query.PageSize {
	return ElasticsearchQuery{}, fmt.Errorf("%w: page offset overflows int", ErrInvalidQuery)
}
```

Build available cluster/role options from the complete snapshot before applying filters. Natural default order is cluster then node; all sorts end with stable ID.

- [x] **Step 7: Run Service tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run Elasticsearch -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 3 if commit authorization is active**

```bash
git add internal/service/elasticsearch_types.go internal/service/elasticsearch_service.go internal/service/elasticsearch_service_test.go
git commit -m "feat: add Elasticsearch status and query service"
```

---

### Task 4: HTTP Contracts and Application Dependency Injection

**Files:**
- Create: `internal/httpapi/elasticsearch_handlers.go`
- Create: `internal/httpapi/elasticsearch_handlers_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Consumes: `ElasticsearchService`, `ElasticsearchQuery`, overview/page types from Task 3.
- Produces: `GET /api/v1/elasticsearch/overview` and `GET /api/v1/elasticsearch/nodes`.
- Produces: explicit JSON View fields consumed by frontend Tasks 5–6.

- [x] **Step 1: Write failing handler contract tests**

Test authenticated success, missing service 503, Provider failure safe 503, stale success, exact allowed parameters, duplicates/empty parameters 400, invalid enum/int/page 400, POST/PUT/DELETE 405 with `Allow: GET`, and no `ident`, `instance`, `url`, `cluster_uuid`, raw labels or PromQL anywhere in encoded JSON.

```go
func TestElasticsearchNodesReturnsExplicitView(t *testing.T) {
	request := authenticatedRequest(http.MethodGet,
		"/api/v1/elasticsearch/nodes?sort=node&order=asc&page=1&page_size=20")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK { t.Fatalf("status = %d", response.Code) }
	assertJSONKeys(t, response.Body.Bytes(), []string{
		"data", "meta", "nodes", "available_clusters", "available_roles",
		"total", "page", "page_size", "total_pages",
	})
	assertNotContainsSensitiveKeys(t, response.Body.Bytes())
}
```

Assert empty slices encode as `[]`, not `null`, and `total_pages` is zero for an empty result.

- [x] **Step 2: Run handler and main tests and verify RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi ./cmd/infraview -run Elasticsearch -count=1
```

Expected: FAIL because routes, dependencies and handlers do not exist.

- [x] **Step 3: Implement explicit Views and handlers**

The node View must expose exactly:

```go
type elasticsearchNodeView struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Cluster string `json:"cluster"`
	Address string `json:"address"`
	Roles []elasticsearch.Role `json:"roles"`
	ClusterHealth elasticsearch.Health `json:"cluster_health"`
	HeapUsagePercent *float64 `json:"heap_usage_percent"`
	DiskUsagePercent *float64 `json:"disk_usage_percent"`
	CPUUsagePercent *float64 `json:"cpu_usage_percent"`
	IndexRate *float64 `json:"index_rate"`
	SearchRate *float64 `json:"search_rate"`
	Documents *int64 `json:"documents"`
	StoreSizeBytes *int64 `json:"store_size_bytes"`
	ThreadPoolQueue *int64 `json:"thread_pool_queue"`
	RejectedRate *float64 `json:"rejected_rate"`
	UptimeSeconds *int64 `json:"uptime_seconds"`
	Status service.Level `json:"status"`
	StatusSource service.ElasticsearchNodeStatusSource `json:"status_source"`
	CollectionLevel service.Level `json:"collection_level"`
}
```

Overview View must be exact and must not encode domain structs directly:

```go
type elasticsearchLevelCountsView struct {
	Total int `json:"total"`
	Normal int `json:"normal"`
	Warning int `json:"warning"`
	Critical int `json:"critical"`
	Unknown int `json:"unknown"`
}

type elasticsearchOverviewAlertsView struct {
	ClusterHealth alertCountView `json:"cluster_health"`
	NodeResource alertCountView `json:"node_resource"`
	UnassignedShards alertCountView `json:"unassigned_shards"`
	RequestRejections alertCountView `json:"request_rejections"`
}

type elasticsearchOverviewView struct {
	Status service.Level `json:"status"`
	Clusters elasticsearchLevelCountsView `json:"clusters"`
	Nodes elasticsearchLevelCountsView `json:"nodes"`
	Alerts elasticsearchOverviewAlertsView `json:"alerts"`
}
```

- [x] **Step 4: Register authenticated GET routes and 405 fallbacks**

Add `ElasticsearchService *service.ElasticsearchService` to `Dependencies` and `elasticsearchService` to `api`. Register exact GET patterns and fallback patterns in `New`; all go through `requireAuthentication` and existing response helpers.

- [x] **Step 5: Wire Mock/Nightingale providers, timeout and Service**

Extend `providerSet` with `Elasticsearch elasticsearch.Provider`. Mock mode uses `mock.NewElasticsearch(clock)`; Nightingale/default modes reuse the same `*nightingale.Provider`. Add:

```go
type elasticsearchTimeoutProvider struct {
	provider elasticsearch.Provider
	timeout time.Duration
}

func (p *elasticsearchTimeoutProvider) ElasticsearchSnapshot(ctx context.Context) (elasticsearch.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.ElasticsearchSnapshot(ctx)
}
```

Construct Service with `SnapshotTTL` and `CollectionInterval` both set to `cfg.ExpectedCollectionInterval`, `MaxStale` and `Clock`, then pass it to `httpapi.New`.

- [x] **Step 6: Run HTTP/main tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi ./cmd/infraview -run Elasticsearch -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4 if commit authorization is active**

```bash
git add internal/httpapi/elasticsearch_handlers.go internal/httpapi/elasticsearch_handlers_test.go internal/httpapi/api.go cmd/infraview/main.go cmd/infraview/main_test.go
git commit -m "feat: expose read-only Elasticsearch APIs"
```

---

### Task 5: Frontend API Contract and Overview Card

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`

**Interfaces:**
- Consumes: exact JSON Views from Task 4.
- Produces: `ElasticsearchOverviewResponse`, `ElasticsearchNodePageResponse` and fifth shared overview card.
- Produces: runtime-safe `requestElasticsearchOverview(signal)` to prevent response-shape regressions.

- [x] **Step 1: Add failing TypeScript contract and overview tests**

Add fixtures whose keys exactly match Task 4. Test malformed `data`, missing alert groups, null arrays and wrong enum values reject with `APIError("服务器响应格式无效")`; valid envelopes render a card using `ModuleStatusCardShell`.

```tsx
expect(screen.getByRole('link', { name: '查看 Elasticsearch 板块' }))
  .toHaveAttribute('href', '/elasticsearch')
expect(container.querySelectorAll('.module-status-card')).toHaveLength(5)
```

Update the desktop geometry expectation to four grid tracks with the fifth card on row two; do not compress the grid to five columns.

- [x] **Step 2: Run focused frontend tests and verify RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

Expected: FAIL because Elasticsearch types and card do not exist.

- [x] **Step 3: Add exact frontend types and fixtures**

```ts
export type ElasticsearchHealth = 'green' | 'yellow' | 'red' | 'unknown'
export type ElasticsearchRole =
  | 'master' | 'data' | 'data_content' | 'data_hot' | 'data_warm'
  | 'data_cold' | 'data_frozen' | 'ingest' | 'ml' | 'transform'
  | 'remote_cluster_client' | 'client'
export type ElasticsearchNodeStatusSource =
  | 'collection' | 'disk' | 'jvm' | 'thread_pool' | 'normal' | 'unknown'

export interface ElasticsearchNode {
  id: string
  name: string
  cluster: string
  address: string
  roles: ElasticsearchRole[]
  cluster_health: ElasticsearchHealth
  heap_usage_percent: number | null
  disk_usage_percent: number | null
  cpu_usage_percent: number | null
  index_rate: number | null
  search_rate: number | null
  documents: number | null
  store_size_bytes: number | null
  thread_pool_queue: number | null
  rejected_rate: number | null
  uptime_seconds: number | null
  status: MetricLevel
  status_source: ElasticsearchNodeStatusSource
  collection_level: MetricLevel
}

export interface ElasticsearchLevelCounts {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
}

export interface ElasticsearchOverviewData {
  status: MetricLevel
  clusters: ElasticsearchLevelCounts
  nodes: ElasticsearchLevelCounts
  alerts: {
    cluster_health: AlertCount
    node_resource: AlertCount
    unassigned_shards: AlertCount
    request_rejections: AlertCount
  }
}

export interface ElasticsearchNodePageData {
  nodes: ElasticsearchNode[]
  available_clusters: string[]
  available_roles: ElasticsearchRole[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type ElasticsearchOverviewResponse = ApiResponse<ElasticsearchOverviewData>
export type ElasticsearchNodePageResponse = ApiResponse<ElasticsearchNodePageData>
```

Node page data contains `nodes`, `available_clusters`, `available_roles`, and pagination. Overview data contains separate cluster/node level counts and the four alert groups. Keep fixture arrays non-null.

- [x] **Step 4: Implement runtime overview validation and shared card**

Follow the safe Redis request pattern but validate Elasticsearch fields directly. Do not cast `unknown` without checking objects, finite non-negative integers and exact enums. The card level is the worst cluster/node level; its four slots are 集群健康、节点资源、未分配分片、请求拒绝. Handle loading, initial error, stale data and background refresh error independently.

- [x] **Step 5: Preserve the four-track overview grid**

Keep `.overview-compact-grid` at four desktop columns. Render Elasticsearch as the fifth card after Redis so it starts the second row at desktop width. Update geometry tests to require five cards, four tracks, equal card widths and no page overflow; do not add a five-column breakpoint.

- [x] **Step 6: Run focused tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

Expected: PASS with five cards and four desktop tracks.

- [ ] **Step 7: Commit Task 5 if commit authorization is active**

```bash
git add web/src/api/types.ts web/src/test/fixtures.ts web/src/test/server.ts web/src/features/overview/OverviewPage.tsx web/src/features/overview/OverviewPage.test.tsx
git commit -m "feat: add Elasticsearch overview card"
```

---

### Task 6: Shared-Template 16-Column Elasticsearch Node Page

**Files:**
- Create: `web/src/features/elasticsearch/ElasticsearchPage.tsx`
- Create: `web/src/features/elasticsearch/ElasticsearchPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`

**Interfaces:**
- Consumes: `ElasticsearchNodePageResponse`, shared `ListPage` components and refresh runtime.
- Produces: complete `/elasticsearch` route, sidebar entry and node page with 16 single-value columns and URL state.

- [x] **Step 1: Write failing 16-column and shared-template tests**

```tsx
const expectedHeaders = [
  '节点名称', '所属集群', '节点地址', '节点角色', '集群健康',
  'JVM堆使用率', '磁盘使用率', 'CPU使用率', '索引速率', '搜索速率',
  '文档数', '存储大小', '线程池队列', '拒绝速率', '运行时间', '状态',
]
expect(screen.getAllByRole('columnheader').map((cell) => cell.textContent))
  .toEqual(expectedHeaders)
expect(container.querySelectorAll('tbody td')).toHaveLength(expectedHeaders.length)
expect(container.querySelector('tbody br')).toBeNull()
```

Also assert the page renders `ListPageHeader`, `ListPageControls`, `ListSearchField`, four `ListSelectField` controls, `ListPageSizeField`, `ListTablePanel`, and `RefreshControl` state in the same control row. Test no Elasticsearch-specific duplicate input markup/classes replace the shared components.

Add `AppShell.test.tsx` and route tests that require an “Elasticsearch” navigation link after Redis and verify `/elasticsearch` renders the real node page heading, not a temporary shell.

- [x] **Step 2: Write failing URL, formatting and response-state tests**

Cover 300ms search debounce, cluster/role/health/status filters, 16 sort fields, ascending toggle, page canonicalization, 20/50/100 page size, stale banner, initial 503, background error, empty state, refetch, roles as one line, IEC storage, percentage, rates, integer counts, and host/MySQL day-hour uptime rules. Add malformed-response cases for null arrays, missing pagination, invalid role/health/status/source enums, non-finite numbers and fields with the wrong primitive type; all must become `APIError("服务器响应格式无效")` rather than render partial data.

- [x] **Step 3: Run the page test and verify RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/elasticsearch/ElasticsearchPage.test.tsx src/app/AppShell.test.tsx'
```

Expected: FAIL because the complete node page and styles do not exist.

- [x] **Step 4: Implement the page with shared controls only**

Use `useSearchParams`, TanStack Query/Table and the existing refresh interval. Implement `requestElasticsearchNodes(signal, parameters)` to fetch `unknown`, validate the full envelope and every node/pagination/options field, then return `ElasticsearchNodePageResponse`; do not cast unchecked JSON. The request URL includes only whitelisted non-empty parameters. Render an empty role set as `未知`, otherwise render roles as one value with `roles.join(' / ')`; do not create badges on multiple lines. Format null as `暂无数据`, unavailable address as `暂无数据`, rates with `/s`, percentages with one decimal, bytes as IEC, and uptime as `x天 x小时`/`x天`/`x小时`.

Status label priority must reflect `collection_level` before `status_source`; never infer cluster-level failure as a node failure. Cluster health text is 绿色/黄色/红色/未知 and has its own column.

Import the completed `ElasticsearchPage` in `App.tsx`, register `<Route path="elasticsearch" element={<ElasticsearchPage />} />`, and add `<NavLink to="/elasticsearch">Elasticsearch</NavLink>` after Redis in `AppShell.tsx`.

- [x] **Step 5: Add only Elasticsearch table layout CSS**

```css
.elasticsearch-list-controls {
  grid-template-columns:
    minmax(14rem, 1fr) repeat(4, minmax(7rem, 9rem)) minmax(6rem, 7rem) auto;
}

.elasticsearch-table {
  width: max-content;
  min-width: 150rem;
  table-layout: auto;
}

.elasticsearch-table th,
.elasticsearch-table td {
  overflow: visible;
  text-overflow: clip;
  white-space: nowrap;
}

.elasticsearch-table th:nth-child(1),
.elasticsearch-table td:nth-child(1) { min-width: 10rem; }
.elasticsearch-table th:nth-child(2),
.elasticsearch-table td:nth-child(2) { min-width: 10rem; }
.elasticsearch-table th:nth-child(3),
.elasticsearch-table td:nth-child(3) { min-width: 11rem; }
.elasticsearch-table th:nth-child(4),
.elasticsearch-table td:nth-child(4) { min-width: 18rem; }
.elasticsearch-table th:nth-child(n+5),
.elasticsearch-table td:nth-child(n+5) { min-width: 8rem; }
```

Pass `scrollClassName="elasticsearch-table-scroll"` to `ListTablePanel`; this adds a selector to the existing `.host-table-scroll` element and does not create a second wrapper. Do not copy public input/select/refresh/pagination styles. Do not reduce font size or increase row padding/line height. Keep that shared scroll element as the only horizontal overflow owner.

- [x] **Step 6: Run page and shared-component tests and verify GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/elasticsearch/ElasticsearchPage.test.tsx src/app/AppShell.test.tsx src/components/ListPage.test.tsx src/components/ModuleStatusCardShell.test.tsx'
```

Expected: PASS.

- [ ] **Step 7: Commit Task 6 if commit authorization is active**

```bash
git add web/src/features/elasticsearch/ElasticsearchPage.tsx web/src/features/elasticsearch/ElasticsearchPage.test.tsx web/src/app/App.tsx web/src/app/AppShell.tsx web/src/app/AppShell.test.tsx web/src/app/theme.css web/src/test/fixtures.ts web/src/test/server.ts
git commit -m "feat: add Elasticsearch node list"
```

---

### Task 7: Browser Specification, Full Verification and Durable Handoff

**Files:**
- Create: `web/e2e/elasticsearch.spec.ts`
- Modify: `web/e2e/infraview.spec.ts`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/superpowers/specs/2026-08-01-elasticsearch-module-design.md`
- Modify: `docs/superpowers/plans/2026-08-01-elasticsearch-module.md`

**Interfaces:**
- Consumes: complete backend/frontend module from Tasks 1–6.
- Produces: static browser acceptance coverage, full Docker verification and a recoverable repository handoff.

- [x] **Step 1: Write the browser specification**

Add tests that authenticate to Mock when run by an authorized harness, navigate through sidebar and fifth overview card, verify 16 exact headers, exercise search/filter/sort/page URL state, and assert no destructive controls.

Geometry assertions at 1440x900 must check:

```ts
const geometry = await page.evaluate(() => ({
  documentOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  tableOverflow: document.querySelector('.elasticsearch-table-scroll')!.scrollWidth >
    document.querySelector('.elasticsearch-table-scroll')!.clientWidth,
  rowHeights: [...document.querySelectorAll('.elasticsearch-table tbody tr')]
    .map((row) => row.getBoundingClientRect().height),
  wrappedCells: [...document.querySelectorAll('.elasticsearch-table tbody td')]
    .filter((cell) => getComputedStyle(cell).whiteSpace !== 'nowrap').length,
}))
expect(geometry.documentOverflow).toBe(false)
expect(geometry.tableOverflow).toBe(true)
expect(geometry.wrappedCells).toBe(0)
expect(Math.max(...geometry.rowHeights) - Math.min(...geometry.rowHeights)).toBeLessThanOrEqual(1)
```

Assert all 16 cells have no `<br>`, `textOverflow` is `clip`, and each representative value has `scrollWidth === clientWidth` or its column expands rather than truncating.

- [x] **Step 2: Run Playwright static discovery only**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npx playwright test --list'
```

Expected: exits 0 and lists the Elasticsearch specs. Do not run `npm run e2e` or `scripts/e2e.sh`.

- [x] **Step 3: Run frontend full tests, typecheck and build**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build'
```

Expected: all Vitest files pass, TypeScript exits 0 and production build exits 0.

- [x] **Step 4: Run Go formatting, vet, ordinary/race tests and build**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'files=$(find cmd internal -type f -name "*.go"); test -z "$(gofmt -l $files)" && go vet ./... && go test ./... && go test -race ./... && CGO_ENABLED=0 go build -o /tmp/infraview ./cmd/infraview'
```

Expected: exits 0 with no unformatted files, vet issues, test failures or build failures.

- [x] **Step 5: Build the complete production image without cache**

```bash
docker build --no-cache --tag infraview:elasticsearch-verify .
```

Expected: Dockerfile frontend tests/typecheck/build and Go ordinary/race/build all pass. This does not start a service or connect upstream.

- [x] **Step 6: Run read-only and sensitive-data static scans**

```bash
test -z "$(rg -n -i -g '!**/*_test.go' 'redis-cli|elasticsearch.*(put|post|delete)|_cluster/reroute|_settings|_forcemerge|_close|_open|_delete_by_query|ssh|os/exec|exec\.Command' internal/elasticsearch internal/adapters/nightingale/elasticsearch_* internal/httpapi/elasticsearch_* web/src/features/elasticsearch || true)"
test -z "$(git diff --name-only | rg '(^|/)(\.env|.*token.*|.*secret.*|.*cookie.*|credentials?)$' || true)"
test -z "$(git diff | rg -i 'authorization:|x-user-token:|bearer [a-z0-9._-]+|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' || true)"
git diff --check
```

Expected: all commands exit 0; no write/remote execution construct, secret path/pattern or whitespace error is present. Review any hit manually before deciding whether it is a false positive; never print matched secret content.

- [x] **Step 7: Update durable documentation**

Record the exact 26-query contract, two GET APIs, multi-cluster identity, status rules, 16 single-value columns, shared-template rule, validation results and open authorization boundaries. `HANDOFF` must add the spec and this plan to the recovery list and state the actual Git status at handoff. Historical Redis/SMART records remain historical and must not be rewritten as current Elasticsearch results.

- [x] **Step 8: Review the complete implementation against the spec**

Check every section of `docs/superpowers/specs/2026-08-01-elasticsearch-module-design.md` against code/tests. In particular confirm:

```text
one 26-query batch; no N+1; no direct Elasticsearch;
cluster/node status separation; exact thresholds and source priority;
two authenticated GET APIs; explicit View and non-null arrays;
shared templates; 16 single-value single-line columns; no ellipsis;
no production Nightingale, extra port, write control or sensitive output.
```

Fix any gap with a new RED->GREEN cycle, then rerun Steps 2–6.

- [ ] **Step 9: Commit Task 7 if commit authorization is active**

```bash
git add web/e2e/elasticsearch.spec.ts web/e2e/infraview.spec.ts README.md docs/ARCHITECTURE.md docs/DESIGN.md docs/SECURITY.md docs/TESTING.md docs/datasources/NIGHTINGALE.md docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md docs/superpowers/specs/2026-08-01-elasticsearch-module-design.md docs/superpowers/plans/2026-08-01-elasticsearch-module.md
git commit -m "docs: record Elasticsearch module verification"
```

---

## Existing 8080 Acceptance Gate

Stop after Task 7 and report offline verification. Do not rebuild or restart any service unless the user explicitly authorizes this new feature deployment. If authorized, only reuse the existing `infraview` Compose project, its existing private test-Nightingale configuration and port 8080:

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env docker compose --project-name infraview up -d --build --force-recreate infraview
```

The command may reference the private file path but must never print or inspect its contents. After rebuild, verify only controlled booleans/statuses: container healthy, `/healthz` 200, only existing 8080 published, non-root user, read-only root filesystem, cap drop `ALL`, no-new-privileges, deployed assets match the current build, both Elasticsearch GET APIs have valid envelopes, write methods return 405, and Linux/disk/MySQL/Redis regressions remain healthy. Browser acceptance may use an ephemeral Playwright container against existing 8080 without publishing a port; do not output API bodies or live identifiers/values.

Push remains a separate explicit authorization after all code, documentation, review and acceptance work is complete.
