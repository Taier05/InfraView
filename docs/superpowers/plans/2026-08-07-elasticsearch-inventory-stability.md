# Elasticsearch Inventory Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Elasticsearch 24 小时 inventory 历史候选被误判为冲突的问题，使正常新旧候选持续生成成功快照，同时保留真实 Provider 失败、stale 缓存和 2/5 周期 freshness 告警。

**Architecture:** 固定 26 查询 batch、PromQL 和 Service/HTTP 契约不变；只在 Nightingale Elasticsearch Provider 中将候选选择基准从外层查询执行时间改为 inventory 真实上报时间。非法候选局部跳过，同一节点最新时刻地址冲突仅清空地址，必需组完全无可用身份时仍返回安全 `ErrUnavailable`。

**Tech Stack:** Go 1.24、Nightingale instant batch、领域 Provider/Service/HTTP 分层、标准库 `testing`、Docker。

## Global Constraints

- InfraView 始终只读；不得新增写 API、运维控件、任意 PromQL、代理或上游写操作。
- Elasticsearch 继续恰好一次固定 26 查询 batch；不得修改查询文本、数量、顺序，不得新增 retry 或 N+1。
- 外层查询时间只验证响应形状；候选新旧只比较 inventory 真实上报时间。
- 缺少稳定领域身份或时间非法的单条候选局部跳过；不得从 `ident`、地址或采集目标猜测集群/节点名称。
- 同一节点相同最新上报时间的不同地址必须显示缺失，不能令整个快照失败，也不能按输入顺序选一个地址。
- batch 组数错误、必需组为 `nil`、集群或节点 inventory 完全无可用稳定身份时仍返回安全 `elasticsearch.ErrUnavailable`。
- 第 1–24 组可选指标为 `nil` 时继续只表示该项缺失。
- 不修改 `SnapshotTTL`、`MaxStale`、15 秒周期、2/5 freshness、前端 retry、stale 横幅或错误文案。
- fixture 只使用保留网段和脱敏名称；不得读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文。
- Go 格式、测试、race、vet、build 只在一次性容器执行；不得访问现场 Nightingale、启动服务、浏览器或新端口。
- 本计划不授权提交、推送、合并、部署或重启；对应动作必须取得用户单独授权。

## File map

- Modify: `internal/adapters/nightingale/elasticsearch_provider.go`：以真实上报时间归并 cluster/node inventory。
- Test: `internal/adapters/nightingale/elasticsearch_provider_test.go`：历史候选、顺序无关、局部非法、地址冲突和安全失败契约。
- Test: `internal/service/elasticsearch_service_test.go`：成功快照推进 freshness 与真实失败 stale 的边界。
- Test: `internal/httpapi/elasticsearch_handlers_test.go`：连续成功不 stale、真实失败仍 stale/503。
- Modify: `docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`、`docs/TESTING.md`：最终状态、验证证据和恢复入口。

---

### Task 1: 以真实上报时间选择 cluster inventory

**Files:**
- Modify: `internal/adapters/nightingale/elasticsearch_provider.go:30-170`
- Test: `internal/adapters/nightingale/elasticsearch_provider_test.go:139-218`

- [ ] **Step 1: 把旧冲突用例改成真实安全边界**

从 `TestBuildElasticsearchSnapshotRejectsUnsafeInventoryShapes` 删除“单个非法时间”和“同查询时间、不同上报时间必然整快照失败”的旧断言，保留：组数错误、必需组 `nil`、整组没有有效身份。

新增表驱动用例验证单个非法候选与合法候选并存时局部跳过；集群候选使用同一个外层查询时间、不同上报时间：

```go
func TestBuildElasticsearchClusterInventoryUsesLatestReportedTime(t *testing.T) {
  older := elasticsearchSeries(elasticsearchClusterLabels(), 1785200200, "1785200000")
  newer := elasticsearchSeries(elasticsearchClusterLabels(), 1785200200, "1785200001")
  invalid := elasticsearchSeries(elasticsearchClusterLabels(), 1785200200, "invalid")

  for _, inventory := range [][]instantSeries{
    {older, newer, invalid},
    {invalid, newer, older},
  } {
    states, err := buildElasticsearchClusterInventory(inventory)
    if err != nil {
      t.Fatal(err)
    }
    state := states["fixture-cluster-a"]
    if state == nil || state.cluster.ReportedAt.Unix() != 1785200001 {
      t.Fatalf("reported time was not selected from the latest valid inventory")
    }
  }
}
```

再增加整组只有缺失身份或非法时间时 `errors.Is(err, elasticsearch.ErrUnavailable)` 的用例。错误断言只检查安全 sentinel，不打印上游正文。

- [ ] **Step 2: 运行 cluster RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run 'Elasticsearch.*(ClusterInventory|UnsafeInventory|WithoutDomainIdentity)' -count=1
```

Expected: FAIL；当前实现把非法候选或同外层时间的不同上报时间升级为整快照不可用。

- [ ] **Step 3: 最小实现 cluster 选择器**

将 `elasticsearchClusterState.inventorySampleAt` 重命名为 `inventoryReportedAt`。保留 `elasticsearchInventoryTimes` 对两个值的解析，但丢弃返回的外层时间：

```go
_, reportedAt, ok := elasticsearchInventoryTimes(candidate.Value)
if !ok {
  continue
}

state, exists := states[cluster]
if !exists {
  id := elasticsearch.StableClusterID(cluster)
  if id == "" {
    continue
  }
  states[cluster] = &elasticsearchClusterState{
    cluster: elasticsearch.Cluster{
      ID: id, Name: cluster,
      Availability: elasticsearch.AvailabilityUnknown,
      NodeStatsAvailability: elasticsearch.AvailabilityUnknown,
      Health: elasticsearch.HealthUnknown,
      CollectionTracked: true,
      ReportedAt: reportedAt,
    },
    inventoryReportedAt: reportedAt,
  }
  continue
}
if reportedAt.After(state.inventoryReportedAt) {
  state.inventoryReportedAt = reportedAt
  state.cluster.ReportedAt = reportedAt
}
```

更早候选和相同时间等价候选都忽略。循环结束后若 `len(states) == 0`，返回 `elasticsearchUnavailableError()`；否则返回 states。

- [ ] **Step 4: 格式化并运行 cluster GREEN**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/adapters/nightingale/elasticsearch_provider.go internal/adapters/nightingale/elasticsearch_provider_test.go && go test ./internal/adapters/nightingale -run "Elasticsearch.*(ClusterInventory|UnsafeInventory|WithoutDomainIdentity)" -count=1 && go test -race ./internal/adapters/nightingale -run "Elasticsearch.*ClusterInventory" -count=1'
```

Expected: PASS；正反输入顺序、新旧候选、局部非法和全组无效边界均成立。

---

### Task 2: 以真实上报时间选择 node inventory 并局部化地址冲突

**Files:**
- Modify: `internal/adapters/nightingale/elasticsearch_provider.go:45-225`
- Test: `internal/adapters/nightingale/elasticsearch_provider_test.go:219-285`

- [ ] **Step 1: 扩展节点归并 RED 矩阵**

改造 `TestBuildElasticsearchSnapshotMergesNodeAddressByInventoryTime`，让旧、新候选具有相同外层查询时间、不同上报时间，并至少覆盖：

```go
older := elasticsearchSeries(elasticsearchNodeLabels("192.0.2.10", ""), 1785200200, "1785200000")
newer := elasticsearchSeries(elasticsearchNodeLabels("192.0.2.20", ""), 1785200200, "1785200001")
sameTimeConflict := elasticsearchSeries(elasticsearchNodeLabels("198.51.100.20", ""), 1785200200, "1785200001")
invalid := elasticsearchSeries(elasticsearchNodeLabels("203.0.113.10", ""), 1785200200, "invalid")
```

用例精确断言：

- `{older, newer}` 与 `{newer, older}` 都选择新地址和新 `ReportedAt`；
- `{newer, sameTimeConflict}`、反序和 `{newer, sameTimeConflict, newer}` 都保留节点且地址为空；
- `{invalid, newer}` 与 `{newer, invalid}` 都成功；
- 相同最新时间、相同地址安全去重；
- 整组仅缺失身份/非法时间时安全不可用。

- [ ] **Step 2: 运行 node RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run 'Elasticsearch.*(NodeInventory|MergesNodeAddress|UnsafeInventory)' -count=1
```

Expected: FAIL；当前代码按外层查询时间比较，并会把正常旧、新上报时间判成冲突。

- [ ] **Step 3: 实现顺序无关的节点选择器**

将 `inventorySampleAt` 改为 `inventoryReportedAt`，并增加不可恢复的同最新时间地址冲突标志：

```go
type elasticsearchNodeState struct {
  node elasticsearch.Node
  // existing metric maps remain unchanged
  inventoryReportedAt      time.Time
  inventoryAddressConflict bool
}
```

归并逻辑：

```go
_, reportedAt, validTimes := elasticsearchInventoryTimes(candidate.Value)
if !ok || !validTimes {
  continue
}

switch {
case reportedAt.After(state.inventoryReportedAt):
  state.inventoryReportedAt = reportedAt
  state.inventoryAddressConflict = false
  state.node.ReportedAt = reportedAt
  state.node.Address = address
case reportedAt.Equal(state.inventoryReportedAt):
  if address != state.node.Address {
    state.inventoryAddressConflict = true
    state.node.Address = ""
  }
}
```

相同时间已经发生冲突后，后续候选不得恢复地址：等时分支先检查 `inventoryAddressConflict`，为 true 时保持空地址。更早候选忽略。稳定 ID 无法生成时局部跳过；最终无节点时才返回安全 unavailable。

- [ ] **Step 4: 运行 node GREEN、全 Provider 回归与 race**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/adapters/nightingale/elasticsearch_provider.go internal/adapters/nightingale/elasticsearch_provider_test.go && go test ./internal/adapters/nightingale -run Elasticsearch -count=1 && go test -race ./internal/adapters/nightingale -run Elasticsearch -count=1'
```

Expected: PASS；固定查询、单 batch、可选 `nil`、身份安全、错误脱敏及所有既有 Elasticsearch Provider 契约不回归。

- [ ] **Step 5: 经授权后创建 Provider 提交**

```bash
git add internal/adapters/nightingale/elasticsearch_provider.go internal/adapters/nightingale/elasticsearch_provider_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "fix: stabilize Elasticsearch inventory selection"
```

Expected: 暂存范围精确为两个文件，whitespace 检查无输出。本步骤必须在用户明确授权本地提交后执行。

---

### Task 3: 锁定成功推进与真实失败边界

**Files:**
- Test: `internal/service/elasticsearch_service_test.go`
- Test: `internal/httpapi/elasticsearch_handlers_test.go`

- [ ] **Step 1: 增强 Service 成功序列测试**

在既有 freshness 测试旁新增序列用例：第一次快照建立基线；缓存过期后 Provider 返回集群/节点 `ReportedAt` 都推进的第二个成功快照；断言两个 collection level 均保持 `normal`，第二次加载不是 stale，Provider 调用数为 2。

```go
firstState, firstMeta, err := service.snapshotState(context.Background())
if err != nil || firstMeta.Stale {
  t.Fatalf("first meta = %#v, err = %v", firstMeta, err)
}
clock.Advance(2 * time.Second)
provider.snapshot.Clusters[0].ReportedAt = firstState.snapshot.Clusters[0].ReportedAt.Add(time.Second)
provider.snapshot.Nodes[0].ReportedAt = firstState.snapshot.Nodes[0].ReportedAt.Add(time.Second)
secondState, secondMeta, err := service.snapshotState(context.Background())
if err != nil || secondMeta.Stale {
  t.Fatalf("second meta = %#v, err = %v", secondMeta, err)
}
if got := service.clusterCollectionLevel(secondState.snapshot.Clusters[0], secondState.clusterAdvancedAt); got != LevelNormal {
  t.Fatalf("cluster collection level = %q, want %q", got, LevelNormal)
}
if got := service.nodeCollectionLevel(secondState.snapshot.Nodes[0], secondState.nodeAdvancedAt); got != LevelNormal {
  t.Fatalf("node collection level = %q, want %q", got, LevelNormal)
}
```

保留并明确断言现有真实失败路径：缓存存在时 stale 为 true 且 freshness 不推进；无缓存时错误为 `elasticsearch.ErrUnavailable`。

- [ ] **Step 2: 增强 HTTP 连续成功/失败测试**

在 `TestElasticsearchReturnsStaleSnapshotAfterProviderFailure` 旁增加连续成功用例：时钟越过 TTL，Provider 返回推进后的第二个成功快照，`/api/v1/elasticsearch/nodes` 仍为 200 且 `meta.stale == false`。现有真实失败用例继续断言 200 + stale；初始失败继续断言安全 503 且不泄漏 fixture 上游正文。

- [ ] **Step 3: 运行 Service/HTTP GREEN 与 race**

这些是对既有正确边界的契约强化测试。为了证明新增测试能捕获回归，先临时 mutation：让成功加载返回 `Meta{Stale: true}`，运行新增 focused 测试并确认只因预期的 stale 状态而 RED；随后立即用 `apply_patch` 恢复生产代码，再运行下列 GREEN。临时 mutation 不得进入最终 diff。若 GREEN 失败，先使用 `superpowers:systematic-debugging` 定位，不得修改 TTL、MaxStale、2/5 freshness 或 HTTP 文案来绕过。

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/elasticsearch_service_test.go internal/httpapi/elasticsearch_handlers_test.go && go test ./internal/service ./internal/httpapi -run Elasticsearch -count=1 && go test -race ./internal/service ./internal/httpapi -run Elasticsearch -count=1'
```

Expected: PASS；正常成功不产生 stale/延迟，真实失败仍如实 stale 或安全 503。

- [ ] **Step 4: 经授权后创建契约测试提交**

```bash
git add internal/service/elasticsearch_service_test.go internal/httpapi/elasticsearch_handlers_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "test: lock Elasticsearch snapshot recovery"
```

Expected: 只提交两份测试文件。本步骤必须在用户明确授权本地提交后执行。

---

### Task 4: 文档、全量验证与安全扫描

**Files:**
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/TESTING.md`

- [ ] **Step 1: 更新持久文档**

记录以下已验证事实，不记录任何现场值：

- 七页共享 `observability-table` 排版已完成（前一份计划若已执行）。
- Elasticsearch 历史 inventory 改用真实上报时间选择最新候选。
- 单条非法候选局部跳过；同最新时间地址冲突仅清空地址。
- 固定一次 26 查询 batch、缓存与 2/5 freshness 未变。
- 真实 Provider 失败仍如实 stale/503；本修复不隐藏告警、不增加 retry。
- 当前分支、提交、是否推送/部署必须按实施结束时的只读 Git/服务检查填写，不能复制旧状态。

- [ ] **Step 2: 运行 Go 全量门禁**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'files="$(find cmd internal -type f -name "*.go")"; test -z "$(gofmt -l $files)" && go vet ./... && go test ./... -count=1 && go test -race ./... -count=1 && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/infraview ./cmd/infraview'
```

Expected: gofmt 检查、vet、全仓普通/race 测试和 Linux 编译全部退出 0。

- [ ] **Step 3: 运行前端全量门禁**

若前一份排版计划已执行，Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

Expected: 全量 Vitest、typecheck、production build 和 Playwright 静态发现全部退出 0；不启动浏览器或服务。

- [ ] **Step 4: 运行固定查询、只读与敏感模式扫描**

Run:

```bash
rg -n 'elasticsearchPromQL|query-instant-batch|elasticsearchQueryCount' internal/adapters/nightingale/elasticsearch_*.go
rg -n 'POST|PUT|PATCH|DELETE|retry|Retry' internal/adapters/nightingale/elasticsearch_provider.go internal/service/elasticsearch_service.go web/src/features/elasticsearch/ElasticsearchPage.tsx
rg -n 'Token|Cookie|Authorization|X-User-Token|BaseURL|https?://' internal/adapters/nightingale/elasticsearch_provider_test.go internal/service/elasticsearch_service_test.go internal/httpapi/elasticsearch_handlers_test.go docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md docs/TESTING.md
git diff --check
git diff --cached --check
git status --short --branch
```

Expected: 人工核对仍是固定 26 查询、一次 batch、只有安全 GET 展示路径、没有新增 retry；新增 fixture/文档不含秘密或真实现场数据；两个 diff 检查无输出；工作树范围与计划一致。

- [ ] **Step 5: 写实施报告并经授权提交文档**

报告记录 RED/GREEN 的真实命令、退出码、关键输出、review finding/fix round、未连接上游、未启动端口以及当前 Git/部署状态。然后仅在用户授权本地提交后执行：

```bash
git add docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md docs/TESTING.md
git diff --cached --check
git diff --cached --stat
git commit -m "docs: record Elasticsearch inventory stability"
```

Expected: 文档提交只含四份状态文档；未获得授权时保留为未提交变更并明确汇报。

- [ ] **Step 6: 最终只读核对**

```bash
git status --short --branch
git log -6 --oneline
git diff --check
git diff --cached --check
```

Expected: 明确报告当前 HEAD、未提交范围、是否已提交；不得把未执行的 push、merge、部署或 8080 重建描述为已完成。
