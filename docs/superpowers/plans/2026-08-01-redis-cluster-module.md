# InfraView Redis Cluster Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增严格只读、Cluster 感知的 Redis 总览卡与十列实例列表，通过测试 Nightingale 的一次固定 21 查询 batch 展示节点、内存、连接、性能和复制状态。

**Architecture:** 新建独立 `internal/redis` 领域与 `RedisService`；共享 Nightingale `*Provider` 实现一次固定 batch 的 Redis 快照，Service 负责缓存、样本推进、状态、查询和分页；HTTP 暴露两条 GET API，React 新增 `/redis` 页面并占用总览第四槽位。Linux、硬盘、MySQL 与 Redis 共享基础设施但保持领域、缓存和状态独立。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Playwright、Docker、Nightingale v8.4.1 固定 batch API。

## Global Constraints

- 完整设计依据：`docs/superpowers/specs/2026-08-01-redis-cluster-module-design.md`。
- InfraView 始终只读；不得连接 Redis、执行 Redis 命令、SSH、脚本、远程操作或任何写入。
- 开发 8080 永远只连接测试 Nightingale；不得连接、切换或探测生产 Nightingale，不得创建其他 InfraView 端口。
- 不读取、输出或提交私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/指标值或上游正文。
- Redis 必须使用一次固定 21 查询即时 batch，无实例 N+1；不得增加任意 PromQL、任意 URL、代理或原始请求体入口。
- 首期只做总览卡和实例列表；不做详情、历史、槽位、拓扑图、Sentinel 语义或故障转移。
- 快照 TTL 与 freshness 复用 `INFRAVIEW_EXPECTED_COLLECTION_INTERVAL`，默认 `15s`；2 个周期警告，5 个周期严重。
- 页面固定十列；总览 Redis 占第四槽位，侧边栏位于 MySQL 之后。
- 全部开发和验证使用 Docker；不安装宿主机 Go、Node 或浏览器依赖。
- 当前 `docs/HANDOFF.md` 有意保持未提交，任何任务都不得 reset、restore、checkout、clean 或覆盖该差异。
- commit、push、现有 8080 重建/重启分别需要用户明确授权。每个任务的提交步骤未获授权时跳过，不阻塞后续 TDD。

## File Map

- `internal/redis/`：领域类型、稳定 ID、Provider 契约和契约测试。
- `internal/adapters/mock/redis_provider.go`：确定性脱敏 Redis Cluster Mock。
- `internal/adapters/nightingale/redis_promql.go`：固定 21 查询及索引。
- `internal/adapters/nightingale/redis_provider.go`：身份、角色、数值和复制归并。
- `internal/service/redis_types.go`、`redis_service.go`：缓存、freshness、状态和列表查询。
- `internal/httpapi/redis_handlers.go`：Overview/Instances View 与只读 Handler。
- `web/src/features/redis/RedisPage.tsx`：十列 Redis 页面。
- `web/src/features/overview/OverviewPage.tsx`：第四张 Redis 卡片。
- `cmd/infraview/main.go`：共享 Provider、超时包装和 Service 接线。

---

### Task 1: Redis 领域契约与确定性 Mock

**Files:**
- Create: `internal/redis/types.go`
- Create: `internal/redis/provider.go`
- Create: `internal/redis/contract_test.go`
- Create: `internal/redis/redistest/contract.go`
- Create: `internal/adapters/mock/redis_provider.go`
- Create: `internal/adapters/mock/redis_provider_test.go`

**Interfaces:**
- Produces: `redis.Provider.RedisSnapshot(context.Context) (redis.Snapshot, error)`
- Produces: `redis.StableInstanceID(ident, instance, address string) string`
- Produces: `mock.NewRedis(clock func() time.Time) redis.Provider`
- Security invariant: 对外领域类型不包含原始标签集合、复制 IP/端口、PromQL 或数据源信息

- [ ] **Step 1: 写领域和稳定 ID 失败测试**

测试锁定：相同 `ident + instance + address` 生成相同 ID；任一身份变化都改变 ID；ID 不包含原始输入；空身份返回空 ID；角色只接受 `master|slave|unknown`；所有指针字段可安全深拷贝。

核心断言：

```go
func TestStableInstanceIDUsesCompleteIdentity(t *testing.T) {
	id := redis.StableInstanceID("fixture-ident", "fixture-instance", "192.0.2.10:6379")
	if id == "" || id != redis.StableInstanceID("fixture-ident", "fixture-instance", "192.0.2.10:6379") {
		t.Fatal("stable ID is not deterministic")
	}
	if id == redis.StableInstanceID("fixture-ident", "fixture-instance-b", "192.0.2.10:6379") {
		t.Fatal("instance label is missing from identity")
	}
}
```

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/redis ./internal/adapters/mock -run 'Redis|StableInstance' -count=1
```

Expected: FAIL，因为 Redis 领域和 Mock 尚不存在。

- [ ] **Step 3: 实现最小领域类型**

`provider.go`：

```go
var ErrUnavailable = errors.New("redis data source: unavailable")

type Provider interface {
	RedisSnapshot(context.Context) (Snapshot, error)
}
```

`types.go` 固定外形：

```go
type Availability string
type Role string

const (
	AvailabilityUp Availability = "up"
	AvailabilityDown Availability = "down"
	AvailabilityUnknown Availability = "unknown"
	RoleMaster Role = "master"
	RoleSlave Role = "slave"
	RoleUnknown Role = "unknown"
)

type Replication struct {
	ConnectedReplicas       *int64
	MasterLinkUp            *bool
	MasterLastIOSecondsAgo  *float64
	MasterSyncInProgress    *bool
	WorstReplicaLagSeconds  *float64
}

type Instance struct {
	ID, Address             string
	Availability            Availability
	Role                    Role
	ClusterEnabled          *bool
	UptimeSeconds           *int64
	UsedMemoryBytes         *int64
	MaxMemoryBytes          *int64
	ConnectedClients        *int64
	MaxClients              *int64
	BlockedClients          *int64
	QPS                     *float64
	HitRate                 *float64
	Keys                    *int64
	ExpiredKeysPerSecond    *float64
	EvictedKeysPerSecond    *float64
	RejectedConnectionsRate *float64
	Replication             Replication
	CollectionTracked       bool
	ReportedAt               time.Time
}

type Snapshot struct { Instances []Instance }
```

`StableInstanceID` 使用 SHA-256 和 URL-safe Base64，不保存原始身份。

- [ ] **Step 4: 实现脱敏 Cluster Mock**

Mock 使用虚构地址和稳定时钟，覆盖 master/slave、正常/警告/严重/未知、内存有/无上限、从节点断链、主节点复制延迟、拒绝连接、字段缺失。Mock 不包含真实地址或现场数量。

- [ ] **Step 5: 格式化并运行 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'gofmt -w internal/redis internal/adapters/mock/redis_provider.go internal/adapters/mock/redis_provider_test.go && go test ./internal/redis ./internal/adapters/mock -count=1'
```

Expected: PASS。

- [ ] **Step 6: 可选提交闸门**

仅获用户明确提交授权后执行：

```bash
git add internal/redis internal/adapters/mock/redis_provider.go internal/adapters/mock/redis_provider_test.go
git diff --cached --check
git commit -m "feat: add Redis domain and mock provider"
```

---

### Task 2: Nightingale 固定 21 查询 Redis Provider

**Files:**
- Create: `internal/adapters/nightingale/redis_promql.go`
- Create: `internal/adapters/nightingale/redis_provider.go`
- Create: `internal/adapters/nightingale/redis_provider_test.go`
- Create: `internal/adapters/nightingale/testdata/redis-instant-batch.json`
- Modify: `internal/adapters/nightingale/provider.go`

**Interfaces:**
- Implements: `redis.Provider` on existing `*nightingale.Provider`
- Consumes: existing `queryInstant`, discovery cache and safe HTTP client
- Produces: one `redis.Snapshot` from exactly one fixed 21-query batch

- [ ] **Step 1: 写固定查询和单 batch RED 测试**

直接断言 `redisPromQL()` 返回新切片且顺序精确等于规格中的 21 条。HTTP 测试只允许一次 `/datasource/brief` 和一次 `/query-instant-batch`，batch 外层必须恰好 21 组。

```go
func TestRedisPromQLIsFixedAndReturnsACopy(t *testing.T) {
	got := redisPromQL()
	if len(got) != 21 { t.Fatalf("len = %d", len(got)) }
	got[0] = "changed"
	if redisPromQL()[0] != "redis_up" { t.Fatal("global query list was mutated") }
}
```

- [ ] **Step 2: 写身份、角色和字段 RED 测试**

脱敏 fixture 覆盖：

- 第 20 组建立 `ident + instance + address` 实例集合和 `ReportedAt`。
- 第 21 组在当前 INFO 缺失时补充 `master|slave`，冲突为 unknown。
- 当前 `redis_up` 只决定可用性，不覆盖原始采集时间。
- 键数量已跨 `db` 聚合，复制延迟已移除副本身份。
- int64 字段精确解析；负数、浮点、NaN/Inf、越界和同时间冲突保持 nil。
- 命中率只接受 `0..1`，二值字段只接受 `0|1`。
- 未知辅助序列忽略，完整身份冲突安全失败。

- [ ] **Step 3: 写协议与安全 RED 测试**

覆盖 401/403、重定向、非 JSON、Content-Type 错误、`dat:null`、非空 `err`、21 组基数错误、必要 inventory 组为 null、超时和 8 MiB 上限。错误必须满足 `errors.Is(err, redis.ErrUnavailable)`，且文本不含测试秘密、URL、标签或正文。

- [ ] **Step 4: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'Redis' -count=1
```

Expected: FAIL，因为 Redis Provider 尚不存在。

- [ ] **Step 5: 实现查询常量和内部状态**

`redis_promql.go` 定义 0-based 查询索引、`redisQueryCount=21` 和返回副本的 `redisPromQL()`。`redis_provider.go` 使用内部 identity state 保存原始标签，只在 finalize 时生成安全 `redis.Instance`。

核心调用固定为：

```go
func (p *Provider) RedisSnapshot(ctx context.Context) (redis.Snapshot, error) {
	if err := p.ready(); err != nil { return redis.Snapshot{}, redisUnavailableError() }
	results, err := p.queryInstant(ctx, redisPromQL())
	if err != nil || len(results) != redisQueryCount { return redis.Snapshot{}, redisUnavailableError() }
	return buildRedisSnapshot(results)
}
```

- [ ] **Step 6: 实现最新值、精确整数和安全归并**

先处理第 20 组 inventory，再处理第 21 组历史角色，最后归并 1–19 组。float 字段使用有限值状态；整数直接从原始数值文本解析为非负 int64，不能先经过 float64。相同最新时间不同值标记冲突并输出 nil。

- [ ] **Step 7: 运行 GREEN、适配器全测和 race**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'gofmt -w internal/adapters/nightingale/redis_*.go internal/adapters/nightingale/provider.go && go test ./internal/adapters/nightingale -count=1 && go test -race ./internal/adapters/nightingale -run Redis -count=1'
```

Expected: PASS，且请求路径只有 discovery 与一次 instant batch。

- [ ] **Step 8: 可选提交闸门**

```bash
git add internal/adapters/nightingale/redis_promql.go internal/adapters/nightingale/redis_provider.go internal/adapters/nightingale/redis_provider_test.go internal/adapters/nightingale/testdata/redis-instant-batch.json internal/adapters/nightingale/provider.go
git diff --cached --check
git commit -m "feat: load Redis snapshot from Nightingale"
```

仅获用户明确授权后执行。

---

### Task 3: RedisService 缓存、状态和实例查询

**Files:**
- Create: `internal/service/redis_types.go`
- Create: `internal/service/redis_service.go`
- Create: `internal/service/redis_service_test.go`
- Reuse: `internal/service/freshness.go`

**Interfaces:**
- Produces: `service.NewRedis(provider, store, RedisOptions) *RedisService`
- Produces: `RedisService.Overview(ctx) (RedisOverview, Meta, error)`
- Produces: `RedisService.Instances(ctx, RedisQuery) (RedisPage, Meta, error)`
- Uses cache key: `service:redis:snapshot`

- [ ] **Step 1: 写缓存和 freshness RED 测试**

覆盖默认 15 秒 TTL、缓存命中不重复调用 Provider、loader 成功才 Observe、首次旧时间正常、30/75 秒冻结为 warning/critical、推进/回退恢复、stale 继续老化、并发无竞态。

- [ ] **Step 2: 写状态矩阵 RED 测试**

锁定：

- `up=0` critical，缺失 unknown。
- Cluster master 副本数为零 critical；非 Cluster master 不告警。
- slave 上游断链 critical，缺失 unknown。
- lag `5–<30s` warning、`>=30s` critical。
- 内存 `85–<95%` warning、`>=95%` critical；`maxmemory<=0` 不告警。
- 拒绝连接速率大于零 warning；可选指标缺失不单独升级 unknown。
- 同级来源优先级为 availability、replication、memory、connection、collection。

```go
const (
	RedisStatusAvailability RedisStatusSource = "availability"
	RedisStatusReplication  RedisStatusSource = "replication"
	RedisStatusMemory       RedisStatusSource = "memory"
	RedisStatusConnection   RedisStatusSource = "connection"
	RedisStatusCollection   RedisStatusSource = "collection"
	RedisStatusNormal       RedisStatusSource = "normal"
	RedisStatusUnknown      RedisStatusSource = "unknown"
)
```

- [ ] **Step 3: 写 Overview、查询和排序 RED 测试**

覆盖状态/角色计数、受影响实例、availability/memory/connection/replication 摘要；地址搜索；角色/状态筛选；9 个 sort 白名单；IP/端口自然排序；数值缺失升降序都置后；稳定 ID 收口；20/50/100 分页；返回值深拷贝。

- [ ] **Step 4: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'Redis' -count=1
```

Expected: FAIL，因为 RedisService 尚不存在。

- [ ] **Step 5: 定义 Service 类型并实现快照 loader**

```go
type RedisOptions struct {
	SnapshotTTL, CollectionInterval, MaxStale time.Duration
	Clock func() time.Time
}

type RedisQuery struct {
	Search string
	Role redis.Role
	Status Level
	Sort, Order string
	Page, PageSize int
}
```

loader 成功后仅观察 `CollectionTracked` 的 `ID -> ReportedAt`；缓存值和所有指针字段在返回前深拷贝。

- [ ] **Step 6: 实现状态、Overview、筛选和排序**

默认 sort=`instance`、order=`asc`；sort 白名单严格为 `instance|memory|connections|qps|keys|evicted|replication_lag|uptime|status`。合并列排序规则按规格执行，缺失值不随 desc 翻到前面。

- [ ] **Step 7: 运行 GREEN、Service 全测和 race**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'gofmt -w internal/service/redis_*.go && go test ./internal/service -count=1 && go test -race ./internal/service -run "Redis|Freshness" -count=1'
```

Expected: PASS。

- [ ] **Step 8: 可选提交闸门**

```bash
git add internal/service/redis_types.go internal/service/redis_service.go internal/service/redis_service_test.go
git diff --cached --check
git commit -m "feat: add Redis monitoring service"
```

仅获用户明确授权后执行。

---

### Task 4: 运行时接线与两条只读 Redis API

**Files:**
- Create: `internal/httpapi/redis_handlers.go`
- Create: `internal/httpapi/redis_handlers_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `internal/httpapi/query_handlers.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Produces: `GET /api/v1/redis/overview`
- Produces: `GET /api/v1/redis/instances`
- Extends: `providerSet.Redis redis.Provider`
- Extends: `httpapi.Dependencies.RedisService *service.RedisService`
- Maps: `redis.ErrUnavailable` to safe retryable `503 redis_unavailable`

- [ ] **Step 1: 写 Provider 接线和超时 RED 测试**

覆盖 Mock 模式返回 Redis Mock；Nightingale 模式同一个 `*Provider` 同时实现四个 Provider；Redis timeout wrapper 传递 deadline；authenticated Mock API 可访问两条 Redis GET。

- [ ] **Step 2: 写 API RED 测试**

Overview 覆盖精确字段、stale meta、认证、空 query、写方法 405、安全 503。Instances 覆盖参数白名单、默认值、角色/状态、9 个 sort、分页和结构化字段；序列化文本不得包含 `replica_ip`、`replica_port`、`replica_id`、`ident`、PromQL、数据源 ID 或测试秘密。

- [ ] **Step 3: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi ./cmd/infraview -run 'Redis|AuthenticatedMockAPI|Provider' -count=1
```

Expected: FAIL，因为 API 和运行时接线尚不存在。

- [ ] **Step 4: 扩展共享 Provider 与 timeout wrapper**

`providerSet` 增加 `Redis redis.Provider`；Mock 使用 `mock.NewRedis(clock)`；Nightingale/default 将同一 provider 赋给四个字段。新增 `redisTimeoutProvider.RedisSnapshot`，模式与 MySQL/Disk wrapper 一致。

- [ ] **Step 5: 构造 RedisService 并注入 API**

```go
redisService := service.NewRedis(redisProvider, store, service.RedisOptions{
	SnapshotTTL: cfg.ExpectedCollectionInterval,
	CollectionInterval: cfg.ExpectedCollectionInterval,
	MaxStale: cfg.MaxStale,
	Clock: clock,
})
```

将其放入 `httpapi.Dependencies`，注册两条 GET 和 method fallback。

- [ ] **Step 6: 实现明确 View、参数解析和安全错误**

Handler 逐字段映射 Service summary，不直接序列化领域对象。`instances` 只接受 `search,role,status,sort,order,page,page_size`；空参数或非法值统一 400。`writeServiceError` 在通用 datasource 分支前处理 `redis.ErrUnavailable`。

- [ ] **Step 7: 运行 GREEN、HTTP/main 全测和 race**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'gofmt -w internal/httpapi/redis_*.go internal/httpapi/api.go internal/httpapi/query_handlers.go cmd/infraview && go test ./internal/httpapi ./cmd/infraview -count=1 && go test -race ./internal/httpapi ./cmd/infraview -run Redis -count=1'
```

Expected: PASS。

- [ ] **Step 8: 可选提交闸门**

```bash
git add \
  cmd/infraview/main.go \
  cmd/infraview/main_test.go \
  internal/httpapi/api.go \
  internal/httpapi/query_handlers.go \
  internal/httpapi/redis_handlers.go \
  internal/httpapi/redis_handlers_test.go
git diff --cached --check
git commit -m "feat: expose read-only Redis APIs"
```

仅获用户明确授权后执行。

---

### Task 5: `/redis` 十列实例页面

**Files:**
- Create: `web/src/features/redis/RedisPage.tsx`
- Create: `web/src/features/redis/RedisPage.test.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Consumes: `GET /api/v1/redis/instances`
- Produces: authenticated `/redis` route
- Preserves: URL-driven query state and shared runtime refresh interval

- [ ] **Step 1: 写前端类型和 fixture**

定义 `RedisRole`、`RedisStatusSource`、`RedisReplication`、`RedisInstance`、`RedisInstancePageData/Response`。Fixture 只使用保留地址段和虚构实例，不包含真实资源信息。

- [ ] **Step 2: 写十列与格式化 RED 测试**

表头严格为：

```ts
['实例地址', '角色', '内存', '连接', 'QPS / 命中率', '键数量', '过期 / 淘汰', '复制', '运行时间', '状态']
```

覆盖 IEC 内存、无上限、连接/阻塞、QPS/百分比、键数、速率、master/slave 复制文案、运行时间、缺失值和 status_source 文案。

- [ ] **Step 3: 写 URL、刷新和安全 RED 测试**

覆盖地址搜索、角色/状态筛选、9 个排序、升降序、页码纠正、20/50/100、loading/error/retry/stale、刷新保留旧数据；DOM 不包含切换、故障转移、重启、删除、命令或执行控件。

- [ ] **Step 4: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/redis/RedisPage.test.tsx'
```

Expected: FAIL，因为 Redis 页面尚不存在。

- [ ] **Step 5: 实现页面和路由**

复用 MySQL 页面模式：`useSearchParams` 是唯一 URL 状态；query key 含全部规范参数；使用 `RefreshControl`、`StaleBanner`、`ErrorPanel`；TanStack Table 只渲染固定十列。状态文案仅依据 `status_source` 和 `collection_level`。

- [ ] **Step 6: 实现十列紧凑样式**

所有表头和值单行、左对齐、数值等宽；地址/复制文本在自身列内省略并保留 title；不以横向滚动作为桌面布局方案。列宽合计 100%，最终允许按 Chromium 几何做小幅调整。

- [ ] **Step 7: 运行 GREEN、typecheck 和前端全测**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run -- src/features/redis/RedisPage.test.tsx && npm run typecheck && npm run test:run'
```

Expected: PASS。

- [ ] **Step 8: 可选提交闸门**

```bash
git add web/src/features/redis web/src/api/types.ts web/src/test/fixtures.ts web/src/app/App.tsx web/src/app/theme.css
git diff --cached --check
git commit -m "feat: add Redis instance page"
```

仅获用户明确授权后执行。

---

### Task 6: 总览第四卡、导航与浏览器规格

**Files:**
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/infraview.spec.ts`

**Interfaces:**
- Consumes: `GET /api/v1/redis/overview`
- Produces: sidebar order `总览 → 主机 → 硬盘 → MySQL → Redis`
- Produces: fourth independent Redis overview module

- [ ] **Step 1: 写导航和第四槽位 RED 测试**

断言五个导航入口和顺序；Redis 指向 `/redis`。总览四张卡顺序固定，Redis 恰好位于第四槽位，不增加占位 DOM。

- [ ] **Step 2: 写 Redis 卡 RED 测试**

覆盖空、正常、warning、critical、unknown；角色分布；受影响实例；可用性/内存/连接/复制四类摘要；loading/error/stale/retry 独立；刷新错误保留旧数据；卡片底部只有“查看 Redis”。

- [ ] **Step 3: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run -- src/app/AppShell.test.tsx src/features/overview/OverviewPage.test.tsx'
```

Expected: FAIL，因为 Redis 导航和卡片尚不存在。

- [ ] **Step 4: 实现独立 Overview query 和卡片**

Redis 使用独立 query key、请求、refetch 和错误边界；整体等级由后端计数直接决定，前端不重新聚合实例状态。桌面保持现有 `repeat(4, minmax(0, 1fr))`，现在四张卡自然填满四槽位。

- [ ] **Step 5: 更新 Playwright 规格但不运行额外端口**

新增：导航进入 `/redis`、总览卡跳转、十列表头、URL 搜索/筛选/排序、无破坏性控件、1440×900 页面及表格 `scrollWidth <= clientWidth`、四卡同一行且各占一个轨道。

- [ ] **Step 6: 运行 GREEN、前端全测/typecheck/build和静态发现**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-bookworm \
  sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

Expected: PASS；只静态发现 Playwright，不启动服务、不创建端口。

- [ ] **Step 7: 可选提交闸门**

```bash
git add \
  web/src/app/AppShell.tsx \
  web/src/app/AppShell.test.tsx \
  web/src/app/theme.css \
  web/src/features/overview/OverviewPage.tsx \
  web/src/features/overview/OverviewPage.test.tsx \
  web/src/api/types.ts \
  web/src/test/fixtures.ts \
  web/e2e/infraview.spec.ts
git diff --cached --check
git commit -m "feat: add Redis navigation and overview"
```

仅获用户明确授权后执行。

---

### Task 7: 文档、全量验证和授权边界

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/superpowers/specs/2026-08-01-redis-cluster-module-design.md`
- Modify: `docs/superpowers/plans/2026-08-01-redis-cluster-module.md`

**Interfaces:**
- Records: Redis 固定查询、Cluster 语义、状态、API/UI、验证证据和未授权事项
- Preserves: 当前有意保留的 `docs/HANDOFF.md` 差异

- [ ] **Step 1: 增量同步文档**

记录独立 Redis 模块、固定 21 查询、一次 batch、15 秒 freshness、十列、第四槽位、Cluster master/slave 语义、只读边界和实际测试结果。历史状态必须标注历史，不覆盖现有 HANDOFF 内容。

- [ ] **Step 2: 运行安全静态扫描**

```bash
rg -n 'replica_ip|replica_port|replica_id|redis-cli|exec\.Command|query-instant|PromQL' internal/redis internal/adapters internal/service internal/httpapi web/src/features/redis docs
rg -n '故障转移|主从切换|重启|删除|执行命令|写入' web/src/features/redis web/src/features/overview
```

确认原始复制身份只存在 Provider 内部、脱敏测试和安全文档；HTTP View/前端不含这些字段；没有 Redis 命令或写能力。

- [ ] **Step 3: 运行 Go 全仓格式、普通/race和编译**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  sh -c 'files=$(find cmd internal -type f -name "*.go"); test -z "$(gofmt -l $files)" && go test ./... -count=1 && go test -race ./... -count=1 && CGO_ENABLED=0 go build -o /tmp/infraview ./cmd/infraview'
```

Expected: 全部退出 0。

- [ ] **Step 4: 运行前端全测、typecheck和生产构建**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine \
  sh -c 'npm ci --ignore-scripts && npm run test:run && npm run typecheck && npm run build'
```

Expected: 全部退出 0，记录实际测试文件和测试数。

- [ ] **Step 5: 无缓存完整镜像构建**

```bash
docker build --no-cache --tag infraview:redis-cluster-module-verify .
```

Expected: Dockerfile 内前端、Go 普通/race和编译再次通过；镜像不运行、不映射端口、不连接上游。

- [ ] **Step 6: 最终工作区和敏感信息检查**

```bash
git diff --check
git status --short --branch
git diff --stat
```

人工确认没有私密文件、现场数据、截图、trace 或测试临时产物进入 Git，且原有 HANDOFF 差异被保留。

- [ ] **Step 7: 记录未授权现场步骤**

未获单独授权时明确记录以下未执行：

- 不运行会创建 18080 的 `scripts/e2e.sh`。
- 不重建或重启现有 8080。
- 不执行真实测试 Nightingale API/Chromium 现场验收。
- 永久不连接或验证生产 Nightingale。

如用户后续授权，只允许原位重建仍连接测试 Nightingale 的现有 8080，并仅输出脱敏布尔结论；不得创建其他端口。

- [ ] **Step 8: 可选最终提交闸门**

仅用户明确授权提交后，先列出并核对精确文件，再执行：

```bash
git add \
  README.md \
  docs/ARCHITECTURE.md \
  docs/DESIGN.md \
  docs/SECURITY.md \
  docs/TESTING.md \
  docs/datasources/NIGHTINGALE.md \
  docs/PROJECT_STATUS.md \
  docs/TODO.md \
  docs/HANDOFF.md \
  docs/superpowers/specs/2026-08-01-redis-cluster-module-design.md \
  docs/superpowers/plans/2026-08-01-redis-cluster-module.md \
  cmd/infraview/main.go \
  cmd/infraview/main_test.go \
  internal/redis \
  internal/adapters/mock/redis_provider.go \
  internal/adapters/mock/redis_provider_test.go \
  internal/adapters/nightingale/provider.go \
  internal/adapters/nightingale/redis_promql.go \
  internal/adapters/nightingale/redis_provider.go \
  internal/adapters/nightingale/redis_provider_test.go \
  internal/adapters/nightingale/testdata/redis-instant-batch.json \
  internal/service/redis_types.go \
  internal/service/redis_service.go \
  internal/service/redis_service_test.go \
  internal/httpapi/api.go \
  internal/httpapi/query_handlers.go \
  internal/httpapi/redis_handlers.go \
  internal/httpapi/redis_handlers_test.go \
  web/src/features/redis \
  web/src/api/types.ts \
  web/src/test/fixtures.ts \
  web/src/app/App.tsx \
  web/src/app/AppShell.tsx \
  web/src/app/AppShell.test.tsx \
  web/src/app/theme.css \
  web/src/features/overview/OverviewPage.tsx \
  web/src/features/overview/OverviewPage.test.tsx \
  web/e2e/infraview.spec.ts
git diff --cached --check
git commit -m "feat: add Redis Cluster monitoring"
```

push 是独立授权；未获授权不得执行。

---

## Definition of Done

- Redis 领域、Mock、Nightingale Provider、RedisService、API 和 React 页面均有 RED→GREEN 证据。
- Provider 精确发送一次固定 21 查询 batch，无 N+1。
- Cluster master/slave、可用性、采集、内存、连接拒绝和复制规则符合规格。
- Overview 第四卡、侧边栏和十列列表符合已批准布局。
- API/UI 不暴露原始复制身份、PromQL、数据源信息或上游内容。
- Docker Go 普通/race/编译、前端测试/typecheck/build、Playwright 静态发现和无缓存镜像均通过。
- 额外端口、现有 8080 重建、提交和推送严格遵守独立授权边界。

## 执行结果（2026-08-01）

- Task 1–6 的领域、Provider、Service、API、页面、总览与导航均已按 RED→GREEN 完成。
- Task 7 的文档、安全扫描、Go 格式/普通/race/编译、前端 9 文件/107 项、typecheck/build、Playwright 14 项静态发现和无缓存镜像构建均退出 0。
- 未运行 `scripts/e2e.sh`，未启动 InfraView，未创建端口，未连接测试或生产 Nightingale。
- Task 7 Step 8 提交闸门、push、现有 8080 原位重建与现场验收均未执行，分别等待用户明确授权。
