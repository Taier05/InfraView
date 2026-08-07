# Java Service Status Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现只读 Java 服务总览第七卡和 13 列业务服务列表，数据仅来自测试 Nightingale 中已确认的固定 `service_` 指标。

**Architecture:** 新建独立 `internal/javaapp` 领域与 `JavaService`，Nightingale Provider 每个共享快照只发送一次固定 11 查询 batch，并以 `name + server_ip` 建立不可逆稳定身份、跨 `ident` 去重。HTTP 仅新增两个认证 GET API；前端复用 `ModuleStatusCardShell` 与 `ListPage`，总览和列表共享 15 秒缓存及 2/5 周期采集推进状态。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Testing Library、Playwright 1.61、Docker。

## Global Constraints

- 规格唯一来源：`docs/superpowers/specs/2026-08-05-java-service-status-module-design.md`。
- InfraView 始终只读；不直连 Java 应用，不提供任意 PromQL、代理、详情、历史、PID、日志或任何运维操作。
- 开发 8080 永远只连接测试 Nightingale；禁止连接或探测生产 Nightingale/Java 服务，禁止创建其他 InfraView 测试端口。
- Provider 每个快照必须恰好一次固定 11 查询 `query-instant-batch`，无服务、采集机器或指标 N+1。
- `ident` 只在 Provider 内部用于归并；页面、领域对外 View 和 API 均不得输出 `ident`。
- 稳定服务身份只能由原始 `name + server_ip` 生成不可逆哈希；不能用 `ident` 或地址猜测名称。
- `tikbee`、`rider`、`mch`、`saas`、`mch_saas` 只做完整值精确映射；未知代码展示原值。
- CPU、内存、健康延迟、进程数和运行时间只展示，不设置推测阈值。
- 新页面必须复用 `ListPage` 与 `ModuleStatusCardShell`；13 列顺序固定，每格一个单行值，1440×900 无页面或表格横向滚动。
- 真实身份、IP、数量、容量、指标值、Token、Cookie、认证头、Base URL 和上游正文不得进入测试日志、文档或提交信息。
- 所有实现与验证使用 Docker/容器化工具；不在宿主机安装依赖或运行项目服务。
- 计划中的 commit 是建议检查点；只有用户另行明确授权 commit 时才执行。push、8080 重建和动态验收始终需要各自单独授权。

---

## File Map

- `internal/javaapp/types.go`：Java 服务领域结构、稳定 ID 与深拷贝。
- `internal/javaapp/provider.go`：只读 Provider 契约与安全不可用错误。
- `internal/javaapp/javaapptest/contract.go`：Provider 共享契约 fixture。
- `internal/adapters/mock/java_provider.go`：脱敏 Java 服务 Mock。
- `internal/adapters/nightingale/java_promql.go`：固定 11 查询与索引。
- `internal/adapters/nightingale/java_provider.go`：inventory-first 解析、跨 `ident` 去重与字段归并。
- `internal/service/java_types.go`：查询、总览、服务摘要与状态来源。
- `internal/service/java_service.go`：共享缓存、freshness、状态、搜索、筛选、排序和分页。
- `internal/httpapi/java_handlers.go`：显式 View、参数白名单与两个认证 GET handler。
- `web/src/features/java/JavaPage.tsx`：共享列表模板上的 13 列页面。
- `web/src/features/overview/OverviewPage.tsx`：第七张 Java 服务总览卡。
- `web/e2e/java.spec.ts`：导航、URL、13 列、单行与 1440×900 布局契约。
- `docs/HANDOFF.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/TESTING.md`、`docs/datasources/NIGHTINGALE.md`：最终交付状态与恢复入口。

---

### Task 1: Java Domain Contract and Sanitized Mock

**Files:**
- Create: `internal/javaapp/types.go`
- Create: `internal/javaapp/provider.go`
- Create: `internal/javaapp/contract_test.go`
- Create: `internal/javaapp/javaapptest/contract.go`
- Create: `internal/adapters/mock/java_provider.go`
- Create: `internal/adapters/mock/java_provider_test.go`

**Interfaces:**
- Produces: `javaapp.Provider.JavaSnapshot(context.Context) (javaapp.Snapshot, error)`。
- Produces: `javaapp.StableServiceID(name, address string) string`。
- Produces: `javaapp.Snapshot{Services []javaapp.Service}.Clone()`。

- [ ] **Step 1: Write failing domain and Provider contract tests**

```go
func TestStableServiceIDUsesNameAndAddressWithoutExposingEither(t *testing.T) {
    first := javaapp.StableServiceID("tikbee", "fixture-address-a")
    if first == "" || first == javaapp.StableServiceID("tikbee", "fixture-address-b") {
        t.Fatal("service ID must be non-empty and address scoped")
    }
    if strings.Contains(first, "tikbee") || strings.Contains(first, "fixture-address-a") {
        t.Fatal("stable ID exposed raw identity")
    }
}

func TestSnapshotCloneDoesNotAliasPointers(t *testing.T) {
    source := javaapptest.Snapshot()
    clone := source.Clone()
    *clone.Services[0].ProcessCount = 999
    if *source.Services[0].ProcessCount == 999 {
        t.Fatal("clone aliases source")
    }
}
```

`javaapptest.RunContract` must call the Provider twice, require non-nil data for the display fields, mutate the first snapshot and prove the second snapshot remains unchanged.

- [ ] **Step 2: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/javaapp ./internal/adapters/mock -run Java -count=1
```

Expected: FAIL because `internal/javaapp` and the Java Mock do not exist.

- [ ] **Step 3: Implement minimal domain types and deterministic Mock**

```go
type Service struct {
    ID, Name, Address                         string
    HealthLatencyMilliseconds                *float64
    HealthUp, PortUp, ProcessUp, PortConsistent *bool
    ProcessCount, ProcessMemoryBytes          *int64
    ProcessCPUPercent, ProcessMemoryPercent   *float64
    ProcessStartTimeSeconds                   *int64
    CollectionTracked                        bool
    ReportedAt                               time.Time
}

type Snapshot struct {
    Services []Service
}

var ErrUnavailable = errors.New("java service data source: unavailable")

type Provider interface {
    JavaSnapshot(context.Context) (Snapshot, error)
}
```

`StableServiceID` must hash `"java-service\x00" + name + "\x00" + address` with SHA-256 and Raw URL-safe Base64 after trimming both fields. `Clone` must preserve nil versus non-nil empty slices and clone every pointer. Mock uses only `fixture-*` identities，包含：全部业务字段正常、健康检查失败、必需字段缺失，以及可由固定 `ReportedAt` 在 Service 测试中推进到采集警告的实例。

- [ ] **Step 4: Run GREEN, formatting and contract tests**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/javaapp internal/adapters/mock/java_provider.go internal/adapters/mock/java_provider_test.go && go test ./internal/javaapp ./internal/adapters/mock -run Java -count=1'
```

- [ ] **Step 5: Commit only if explicitly authorized**

```bash
git add internal/javaapp internal/adapters/mock/java_provider.go internal/adapters/mock/java_provider_test.go
git commit -m "feat: add Java service domain and mock provider"
```

---

### Task 2: Fixed Nightingale Java Provider

**Files:**
- Create: `internal/adapters/nightingale/java_promql.go`
- Create: `internal/adapters/nightingale/java_provider.go`
- Create: `internal/adapters/nightingale/java_provider_test.go`
- Create: `internal/adapters/nightingale/testdata/java-instant-batch.json`

**Interfaces:**
- Consumes: shared Nightingale `queryInstant(ctx, queries)` and `javaapp.Snapshot`。
- Produces: `func (p *Provider) JavaSnapshot(context.Context) (javaapp.Snapshot, error)`。
- Produces: private `javaPromQL() []string` returning a defensive copy。

- [ ] **Step 1: Lock the exact fixed-query contract in a failing test**

```go
func TestJavaPromQLIsFixedAndReturnsACopy(t *testing.T) {
    want := []string{
        "service_health_latency_ms",
        "service_health_up",
        "service_port_up",
        "service_process_count",
        "service_process_cpu_percent",
        "service_process_memory_bytes",
        "service_process_memory_percent",
        "service_process_port_consistent",
        "service_process_start_time_seconds",
        "service_process_up",
        "tlast_over_time(service_process_up[24h])",
    }
    got := javaPromQL()
    if !reflect.DeepEqual(got, want) { t.Fatalf("queries = %#v", got) }
    got[0] = "changed"
    if javaPromQL()[0] != want[0] { t.Fatal("fixed Java queries were mutated") }
}
```

- [ ] **Step 2: Write failing one-batch, inventory and numeric-safety tests**

Use `httptest.Server` to accept only `/api/n9e/datasource/brief` and one `/api/n9e/query-instant-batch`. Assert all 11 queries use one identical query time and any second batch fails the test. Fixtures and focused builders must cover:

- query 11 alone creates identities; current metric groups cannot create additional services;
- complete `name + server_ip` is required; missing/blank labels are ignored;
- the same identity from different `ident` values becomes one service;
- newer raw sample timestamp wins and same-latest-time differing values become nil;
- binary values accept only 0/1;
- percent accepts only finite 0..100, latency accepts finite nonnegative values;
- count, memory bytes and start time parse directly from raw JSON text into nonnegative `int64` without float precision loss;
- inventory `ReportedAt` comes only from query 11’s sample value;
- wrong group count、nil inventory、401/403、redirect、non-JSON、`dat:null`、error envelope、oversize、timeout all return `javaapp.ErrUnavailable` without leaking fixture secrets, labels, Base URL, body or query text。

- [ ] **Step 3: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run Java -count=1
```

Expected: FAIL because the fixed query list and Java Provider method do not exist.

- [ ] **Step 4: Implement inventory-first parsing and conflict-aware merge**

```go
const (
    javaHealthLatencyQuery = iota
    javaHealthUpQuery
    javaPortUpQuery
    javaProcessCountQuery
    javaProcessCPUQuery
    javaProcessMemoryBytesQuery
    javaProcessMemoryPercentQuery
    javaPortConsistentQuery
    javaProcessStartTimeQuery
    javaProcessUpQuery
    javaInventoryQuery
    javaQueryCount
)

type javaLatest[T comparable] struct {
    value *T
    timestamp time.Time
    conflict bool
}
```

Build a map keyed only by `javaapp.StableServiceID(name, serverIP)` from query 11. Store raw `name` and `server_ip` in the domain fields, never store `ident`. Merge the first 10 groups by the same business key; update `javaLatest` only for a newer sample, and clear the final field on same-time conflict. Reuse the shared client’s redirect ban、8 MiB body limit、Content-Type/envelope checks and safe error mapping.

- [ ] **Step 5: Run GREEN, race and formatting checks**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/adapters/nightingale/java_*.go && go test ./internal/adapters/nightingale -run Java -count=1 && go test -race ./internal/adapters/nightingale -run Java -count=1'
```

- [ ] **Step 6: Commit only if explicitly authorized**

```bash
git add internal/adapters/nightingale/java_promql.go internal/adapters/nightingale/java_provider.go internal/adapters/nightingale/java_provider_test.go internal/adapters/nightingale/testdata/java-instant-batch.json
git commit -m "feat: add fixed Java Nightingale provider"
```

---

### Task 3: Java Service, Status, Freshness and Query Semantics

**Files:**
- Create: `internal/service/java_types.go`
- Create: `internal/service/java_service.go`
- Create: `internal/service/java_service_test.go`

**Interfaces:**
- Produces: `NewJava(provider javaapp.Provider, store *cache.Store, options JavaOptions) *JavaService`。
- Produces: `Overview(context.Context) (JavaOverview, Meta, error)`。
- Produces: `Services(context.Context, JavaQuery) (JavaPage, Meta, error)`。

- [ ] **Step 1: Write failing business-name and status-priority tests**

```go
func TestJavaBusinessNameUsesExactMapping(t *testing.T) {
    cases := map[string]string{
        "tikbee": "用户端", "rider": "骑手端", "mch": "商家端",
        "saas": "管理后台端", "mch_saas": "商家 PC 端",
        "tikbee-extra": "tikbee-extra",
    }
    for input, want := range cases {
        if got := javaBusinessName(input); got != want { t.Fatalf("%q => %q", input, got) }
    }
}

func TestJavaStatusUsesCriticalBusinessSignalsBeforeCollection(t *testing.T) {
    now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
    item := javaapp.Service{
        ID: "fixture-id", Name: "tikbee", Address: "fixture-address-a",
        HealthUp: javaBool(true), PortUp: javaBool(true),
        ProcessUp: javaBool(true), PortConsistent: javaBool(true),
        CollectionTracked: true, ReportedAt: now,
    }
    *item.HealthUp = false
    summary := summarizeJavaService(item, LevelCritical, now)
    if summary.Status != LevelCritical || summary.StatusSource != JavaStatusHealth {
        t.Fatalf("summary = %#v", summary)
    }
}

func javaBool(value bool) *bool { return &value }
```

Add table tests for each 0-valued required signal, every required-field missing case, combinations at equal and different levels, and prove CPU/memory/latency/process-count extremes do not change a normal status.

- [ ] **Step 2: Write failing freshness, overview and list-query tests**

Use a controllable clock and a Provider returning an unchanged `ReportedAt`:

- first load only establishes baseline;
- after 2×15s without progress collection is warning; after 5×15s it is critical;
- advancing or rolling back `ReportedAt` re-establishes progress;
- a cache hit does not call Provider or fake progress;
- stale fallback keeps the previous snapshot and continues aging;
- overview counts normal/warning/critical/unknown and independently counts health、port、process/consistency、collection summaries;
- search matches raw code、Chinese business name and address only;
- `name` and status filters are exact; `AvailableNames` is built from the complete snapshot;
- all 13 sort keys support asc/desc, missing values always last, stable ID closes ties;
- page sizes only 20/50/100 and page arithmetic rejects integer overflow。

- [ ] **Step 3: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run Java -count=1
```

Expected: FAIL because `JavaService` and its query types do not exist.

- [ ] **Step 4: Implement the service types and shared snapshot state**

```go
type JavaQuery struct {
    Search, Name, Sort, Order string
    Status Level
    Page, PageSize int
}

type JavaStatusSource string
const (
    JavaStatusHealth JavaStatusSource = "health"
    JavaStatusPort JavaStatusSource = "port"
    JavaStatusProcess JavaStatusSource = "process"
    JavaStatusConsistency JavaStatusSource = "consistency"
    JavaStatusCollection JavaStatusSource = "collection"
    JavaStatusNormal JavaStatusSource = "normal"
    JavaStatusUnknown JavaStatusSource = "unknown"
)

type JavaServiceSummary struct {
    ID, Name, Business, Address string
    HealthUp *bool
    HealthLatencyMilliseconds *float64
    PortUp, ProcessUp *bool
    ProcessCount *int64
    PortConsistent *bool
    CPUUsagePercent *float64
    MemoryBytes *int64
    MemoryUsagePercent *float64
    UptimeSeconds *int64
    Status Level
    StatusSource JavaStatusSource
    CollectionLevel Level
}
```

Define `JavaLevelCounts`、`JavaAlertCount`、`JavaOverviewAlerts`、`JavaOverview` and `JavaPage{Services, AvailableNames, Total, Page, PageSize}`. Use cache key `service:java:snapshot`, default TTL/interval/max-stale 15s/15s/5m and the existing `freshnessTracker`. The 13 accepted sort keys are exactly `business,address,health,health_latency,port,process,process_count,consistency,cpu,memory,memory_percent,uptime,status`.

- [ ] **Step 5: Implement deterministic assessment, uptime, sorting and pagination**

Choose status by level `critical > warning > unknown > normal`, then source priority `health > port > process > consistency > collection`. A missing required binary field yields unknown only when no higher issue exists. Compute uptime only when start time is nonnegative and not later than `Clock().Unix()`; otherwise return nil. Every sort comparator handles nil-last independently of direction and finishes with `ID` ascending.

- [ ] **Step 6: Run GREEN and race tests**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/java_*.go && go test ./internal/service -run Java -count=1 && go test -race ./internal/service -run Java -count=1'
```

- [ ] **Step 7: Commit only if explicitly authorized**

```bash
git add internal/service/java_types.go internal/service/java_service.go internal/service/java_service_test.go
git commit -m "feat: add Java service status aggregation"
```

---

### Task 4: Authenticated Read-only HTTP API and Application Wiring

**Files:**
- Create: `internal/httpapi/java_handlers.go`
- Create: `internal/httpapi/java_handlers_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Consumes: `JavaService.Overview` and `JavaService.Services`。
- Produces: `GET /api/v1/java/overview` and `GET /api/v1/java/services`。
- Produces: `Dependencies.JavaService *service.JavaService` and Java timeout Provider wiring。

- [ ] **Step 1: Write failing HTTP contract and parameter tests**

Test both endpoints through the authenticated API server. Assert:

- unauthenticated requests return 401; authenticated GET returns envelope with non-null arrays;
- overview rejects every query parameter;
- services accepts only `search,name,status,sort,direction,page,page_size` once each;
- unknown、duplicate、empty or invalid values return 400;
- POST/PUT/PATCH/DELETE return 405;
- `service.ErrInvalidQuery` maps to 400 and Provider failure maps to safe retryable 503;
- stale metadata is preserved;
- encoded JSON excludes `ident`、raw label maps、PromQL、datasource/auth/Base URL/upstream fields。

- [ ] **Step 2: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi ./cmd/infraview -run Java -count=1
```

Expected: FAIL because routes, views and dependency wiring do not exist.

- [ ] **Step 3: Implement explicit JSON Views and strict query parsing**

```go
type javaServiceView struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Business string `json:"business"`
    Address string `json:"address"`
    HealthUp *bool `json:"health_up"`
    HealthLatencyMilliseconds *float64 `json:"health_latency_ms"`
    PortUp *bool `json:"port_up"`
    ProcessUp *bool `json:"process_up"`
    ProcessCount *int64 `json:"process_count"`
    PortConsistent *bool `json:"port_consistent"`
    CPUUsagePercent *float64 `json:"cpu_usage_percent"`
    MemoryBytes *int64 `json:"memory_bytes"`
    MemoryUsagePercent *float64 `json:"memory_usage_percent"`
    UptimeSeconds *int64 `json:"uptime_seconds"`
    Status service.Level `json:"status"`
    StatusSource service.JavaStatusSource `json:"status_source"`
    CollectionLevel service.Level `json:"collection_level"`
}
```

Overview View exposes only `status`、`services` level counts and `alerts.{health,port,process,collection}`. Services View exposes `services`、`available_names`、`total`、`page`、`page_size`、derived `total_pages`. Always allocate arrays as `[]`, never `null`.

- [ ] **Step 4: Wire routes, providers, timeout wrapper and service construction**

Add Java to `providerSet` for Mock、configured Nightingale and invalid Nightingale. Wrap it with `context.WithTimeout`, construct `service.NewJava` using the existing cache/TTL/collection/max-stale/clock values, and register:

```go
mux.Handle("GET /api/v1/java/overview", server.requireAuthentication(http.HandlerFunc(server.javaOverview)))
mux.Handle("GET /api/v1/java/services", server.requireAuthentication(http.HandlerFunc(server.javaServices)))
mux.Handle("/api/v1/java/overview", server.requireAuthentication(http.HandlerFunc(server.methodNotAllowed)))
mux.Handle("/api/v1/java/services", server.requireAuthentication(http.HandlerFunc(server.methodNotAllowed)))
```

- [ ] **Step 5: Run GREEN, race and whole-backend regression tests**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/httpapi/java_*.go internal/httpapi/api.go cmd/infraview/main.go cmd/infraview/main_test.go && go test ./internal/javaapp ./internal/adapters/mock ./internal/adapters/nightingale ./internal/service ./internal/httpapi ./cmd/infraview -run Java -count=1 && go test -race ./internal/httpapi ./cmd/infraview -run Java -count=1'
```

- [ ] **Step 6: Commit only if explicitly authorized**

```bash
git add internal/httpapi/java_handlers.go internal/httpapi/java_handlers_test.go internal/httpapi/api.go cmd/infraview/main.go cmd/infraview/main_test.go
git commit -m "feat: expose read-only Java service APIs"
```

---

### Task 5: Frontend Java API Contract and Sanitized Fixtures

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`
- Create: `web/src/test/java_contract.test.ts`

**Interfaces:**
- Produces: `JavaOverviewResponse` and `JavaServicePageResponse` TypeScript contracts。
- Produces: `javaOverviewFixture()`、`javaServicePageFixture()` and strict MSW GET handlers。

- [ ] **Step 1: Write failing transport-contract tests**

```ts
it('两个固定 GET 返回 Java envelope 且不提供写 handler', async () => {
  const [overview, services] = await Promise.all([
    apiRequest<JavaOverviewResponse>('/api/v1/java/overview'),
    apiRequest<JavaServicePageResponse>('/api/v1/java/services'),
  ])
  expect(overview.data.services.total).toBeGreaterThanOrEqual(0)
  expect(Array.isArray(services.data.services)).toBe(true)
  await expect(
    apiRequest('/api/v1/java/overview', { method: 'POST' }),
  ).rejects.toMatchObject({ status: 405 })
})
```

Add empty/stale/error/malformed-envelope cases and assert no fixture contains `ident`, credentials, real-looking addresses or upstream query text.

- [ ] **Step 2: Run frontend RED in a clean Node container**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/test/java_contract.test.ts'
```

Expected: FAIL because Java types, fixtures and handlers do not exist.

- [ ] **Step 3: Add exact TypeScript contracts and GET fixtures**

```ts
export type JavaStatusSource =
  | 'health' | 'port' | 'process' | 'consistency'
  | 'collection' | 'normal' | 'unknown'

export interface JavaService {
  id: string
  name: string
  business: string
  address: string
  health_up: boolean | null
  health_latency_ms: number | null
  port_up: boolean | null
  process_up: boolean | null
  process_count: number | null
  port_consistent: boolean | null
  cpu_usage_percent: number | null
  memory_bytes: number | null
  memory_usage_percent: number | null
  uptime_seconds: number | null
  status: MetricLevel
  status_source: JavaStatusSource
  collection_level: MetricLevel
}
```

Define level counts and alert counts including `unknown`, then `JavaOverviewData{status,services,alerts}` and `JavaServicePageData{services,available_names,total,page,page_size,total_pages}`. Fixtures use documentation-only reserved addresses such as `fixture-address-a`, never live values.

- [ ] **Step 4: Run GREEN, typecheck and existing contract regressions**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/test/java_contract.test.ts src/test/rabbitmq_contract.test.ts && npm run typecheck'
```

- [ ] **Step 5: Commit only if explicitly authorized**

```bash
git add web/src/api/types.ts web/src/test/fixtures.ts web/src/test/server.ts web/src/test/java_contract.test.ts
git commit -m "test: define Java service frontend contract"
```

---

### Task 6: Shared-template Java Business Service List

**Files:**
- Create: `web/src/features/java/JavaPage.tsx`
- Create: `web/src/features/java/JavaPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Consumes: `GET /api/v1/java/services` and the Task 5 contracts。
- Produces: authenticated `/java` route, `Java 服务` navigation item and 13-column list page。

- [ ] **Step 1: Write failing page structure, mapping and routing tests**

Assert the exact headers in order:

```ts
const exactHeaders = [
  '业务端', '服务地址', '健康检查', '健康延迟', '端口状态',
  '进程状态', '进程数', '端口进程一致性', 'CPU 使用率',
  '内存占用', '内存使用率', '运行时间', '状态',
] as const
```

Tests must require every rendered row to have exactly 13 `<td>` cells, each cell to contain no `<br>` and computed `white-space: nowrap`. Cover exact mappings for all five known codes, unknown-code passthrough, `暂无数据`, status labels and `title` for truncated identity values. Assert real App routing renders `Java 业务服务` at `/java` and navigation order places Java after RabbitMQ.

- [ ] **Step 2: Write failing URL, validation and state tests**

Cover canonical `search,name,status,sort,direction,page,page_size`, 300ms search debounce, reset-to-page-1 behavior, invalid URL normalization, retained removed filter option, 20/50/100 page sizes, all 13 sort buttons, empty/loading/error/stale/background-refresh states and strict response validation.

- [ ] **Step 3: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/features/java/JavaPage.test.tsx src/app/AppShell.test.tsx'
```

Expected: FAIL because the Java page, route and navigation item do not exist.

- [ ] **Step 4: Implement the page by composing shared primitives**

Use `ListPageHeader`、`ListPageControls`、`ListSearchField`、`ListSelectField`、`ListPageSizeField`、`ListTablePanel`、`StatusBadge` and `StaleBanner`. Do not copy shared component CSS. Request parameters and sort keys must match Task 3 exactly.

Display rules:

```ts
function binary(value: boolean | null) {
  if (value === null) return '暂无数据'
  return value ? '正常' : '异常'
}

function uptime(value: number | null) {
  if (value === null) return '暂无数据'
  const days = Math.floor(value / 86_400)
  const hours = Math.floor((value % 86_400) / 3_600)
  if (days > 0 && hours > 0) return `${days}天 ${hours}小时`
  if (days > 0) return `${days}天`
  return `${hours}小时`
}
```

Format latency as `x ms`, CPU/memory as one-decimal percent, memory bytes with IEC units and process count as localized integer. `name` is the filter/query code; `business` is the displayed first column.

- [ ] **Step 5: Add narrowly scoped 13-column density styles**

Add `.java-table`、`.java-table-scroll`、`.java-identity`、`.java-value`、`.java-status` rules patterned after RabbitMQ. Desktop column widths are fixed to `8%,10%,7%,7%,7%,7%,6%,9%,7%,8%,8%,8%,8%` in header order, exactly totaling 100%:

```css
.java-table { width: 100%; table-layout: fixed; }
.host-table.java-table th:nth-child(1) { width: 8%; }
.host-table.java-table th:nth-child(2) { width: 10%; }
.host-table.java-table th:nth-child(3),
.host-table.java-table th:nth-child(4),
.host-table.java-table th:nth-child(5),
.host-table.java-table th:nth-child(6),
.host-table.java-table th:nth-child(9) { width: 7%; }
.host-table.java-table th:nth-child(7) { width: 6%; }
.host-table.java-table th:nth-child(8) { width: 9%; }
.host-table.java-table th:nth-child(10),
.host-table.java-table th:nth-child(11),
.host-table.java-table th:nth-child(12),
.host-table.java-table th:nth-child(13) { width: 8%; }
.java-table th, .java-table td {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

Below the existing narrow breakpoint set a fixed minimum width and allow only `.java-table-scroll` to scroll.

- [ ] **Step 6: Run GREEN, frontend regression and production build**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/features/java/JavaPage.test.tsx src/app/AppShell.test.tsx src/components/ListPage.test.tsx && npm run typecheck && npm run build'
```

- [ ] **Step 7: Commit only if explicitly authorized**

```bash
git add web/src/features/java/JavaPage.tsx web/src/features/java/JavaPage.test.tsx web/src/app/App.tsx web/src/app/AppShell.tsx web/src/app/AppShell.test.tsx web/src/app/theme.css
git commit -m "feat: add Java business service list"
```

---

### Task 7: Java Overview Seventh Card

**Files:**
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/test/fixtures.ts`

**Interfaces:**
- Consumes: `GET /api/v1/java/overview` and `JavaOverviewResponse`。
- Produces: independent seventh `ModuleStatusCardShell` linked to `/java`。

- [ ] **Step 1: Write failing seventh-card and isolation tests**

Tests must require:

- `.overview-compact-grid .module-status-card` count is 7;
- card title is `Java 服务`, link/aria target is `/java`;
- main copy is `异常服务 x / 总服务` and badges are `严重 N`、`警告/未知 N`;
- four summaries are exactly `健康检查`、`端口状态`、`进程状态`、`采集状态`;
- zero summary displays `无异常`; empty total displays `暂无 Java 服务`;
- loading、503、stale and retry are isolated from the other six modules;
- malformed Java response is rejected without breaking other cards。

- [ ] **Step 2: Run focused RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

Expected: FAIL because the Java query and seventh card do not exist.

- [ ] **Step 3: Implement strict response validation, query and card**

Add `requestJavaOverview(signal)` with runtime validation for level/count consistency and safe invalid-response errors. Use its own query key and refresh lifecycle:

```ts
const javaOverview = useQuery({
  queryKey: ['java-overview'],
  queryFn: ({ signal }) => requestJavaOverview(signal),
  refetchInterval: refreshIntervalMs,
  refetchIntervalInBackground: false,
})
```

Compute affected as warning + critical + unknown. `JavaStatusCard` must only compose `ModuleStatusCardShell` and existing metric-summary primitives; do not introduce a separate card shell or new refresh mechanism.

- [ ] **Step 4: Run GREEN and all overview regressions**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run -- src/features/overview/OverviewPage.test.tsx src/components/ModuleStatusCardShell.test.tsx && npm run typecheck'
```

- [ ] **Step 5: Commit only if explicitly authorized**

```bash
git add web/src/features/overview/OverviewPage.tsx web/src/features/overview/OverviewPage.test.tsx web/src/test/fixtures.ts
git commit -m "feat: add Java service overview card"
```

---

### Task 8: End-to-end Contract, Durable Documentation and Final Verification

**Files:**
- Create: `web/e2e/java.spec.ts`
- Include: `docs/superpowers/specs/2026-08-05-java-service-status-module-design.md`
- Include: `docs/superpowers/plans/2026-08-05-java-service-status-module.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Consumes: completed backend, frontend and sanitized Java API fixtures。
- Produces: static E2E discovery, security/read-only evidence and durable recovery context。

- [x] **Step 1: Add deterministic Playwright route fixtures and acceptance cases**

`web/e2e/java.spec.ts` must intercept both Java GET endpoints with sanitized synthetic data and test:

- sidebar and seventh card reach `/java`;
- exact 13 headers and one-line 13-cell rows;
- search/name/status/sort/page/page-size URL behavior;
- five exact business mappings and unknown passthrough;
- missing-value format, stale/error/empty states and native title text;
- 1440×900 has no document or table horizontal overflow;
- a narrower viewport has horizontal overflow only on `.java-table-scroll`;
- POST to both Java API paths returns 405 through the authenticated shared request context;
- no destructive buttons or links exist。

- [x] **Step 2: Run static Playwright discovery without starting a service or port**

```bash
docker run --rm --ipc=host --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/work" -v /work/web/node_modules -w /work/web mcr.microsoft.com/playwright:v1.61.1-noble sh -c 'npm ci && npx playwright test --list e2e/java.spec.ts'
```

Expected: the Java spec and all intended tests are listed; no InfraView container or port is created.

- [x] **Step 3: Update durable documentation without live data**

Record the 11-query contract、`name + server_ip` identity、`ident` exclusion、exact business mapping、13 columns、2/5 freshness、test commands and authorization boundaries. `HANDOFF.md` must distinguish completed implementation from any still-unperformed commit/push/deploy work and provide the current recovery entry. Never copy live labels, addresses, counts, values or private configuration.

- [x] **Step 4: Run complete containerized verification without dynamic E2E**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run test:run && npm run typecheck && npm run build'
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'files="$(find cmd internal -type f -name "*.go")"; test -z "$(gofmt -l $files)" && go vet ./... && go test ./... && go test -race ./... && go build -o /tmp/infraview ./cmd/infraview'
./scripts/e2e-safety.test.sh
docker build --no-cache --tag infraview:java-verify .
git diff --check
git diff --cached --check
if rg -n '[[:blank:]]+$' internal/javaapp internal/adapters/mock/java_provider.go internal/adapters/nightingale/java_*.go internal/service/java_*.go internal/httpapi/java_*.go web/src/features/java web/e2e/java.spec.ts docs/superpowers/specs/2026-08-05-java-service-status-module-design.md docs/superpowers/plans/2026-08-05-java-service-status-module.md; then exit 1; fi
```

Do not run `make verify` or `scripts/e2e.sh`, because their default path creates a separate 18080 service, which violates this project’s current port constraint.

- [x] **Step 5: Run sensitive/read-only source scans**

```bash
rg -n 'service_port_pid|service_process_pid|ident.*json|json:.*ident|exec\(|os/exec|CommandContext|http\.Method(Post|Put|Patch|Delete)' internal/javaapp internal/adapters/mock/java_provider.go internal/adapters/nightingale/java_*.go internal/service/java_*.go internal/httpapi/java_*.go web/src/features/java web/e2e/java.spec.ts
```

Expected: PID metrics and `ident` JSON exposure have no matches; only test assertions or explicit 405 checks may mention write methods; no command execution exists.

- [ ] **Step 6: Request separate authorization for existing-8080 deployment and dynamic browser acceptance**

Do not execute any Compose or Playwright browser command until the user explicitly authorizes both the existing 8080 rebuild and dynamic acceptance. After authorization, first verify that the configured data source is the test Nightingale without printing `.env` or any private value, then rebuild only the existing project/service in place. Point Playwright at `http://127.0.0.1:8080`; do not invoke `scripts/e2e.sh`, because it owns and creates a separate Compose project.

- [x] **Step 7: Commit only if explicitly authorized; push remains separate**

```bash
git add web/e2e/java.spec.ts docs/superpowers/specs/2026-08-05-java-service-status-module-design.md docs/superpowers/plans/2026-08-05-java-service-status-module.md docs/HANDOFF.md docs/PROJECT_STATUS.md docs/TODO.md docs/TESTING.md docs/datasources/NIGHTINGALE.md
git commit -m "docs: record Java service monitoring delivery"
```

Do not run `git push` without a new, explicit push authorization.
