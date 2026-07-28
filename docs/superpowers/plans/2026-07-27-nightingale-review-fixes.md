# InfraView Nightingale 审查修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以 TDD 修复 Nightingale 第二阶段审查发现的 Token 重定向泄露、响应协议宽松、历史主机可见性、配置隔离和极端数值问题，并补齐关键契约测试。

**Architecture:** 在 Nightingale Provider 的 HTTP 边界拒绝重定向并严格校验 envelope、分页和批量响应；在 Service 范围缓存加载器中只做一次主机存在性验证；在配置层仅为 Nightingale 模式校验专属变量。保持现有 Provider 接口、缓存/stale 语义、固定 PromQL 和只读产品边界。

**Tech Stack:** Go 1.24、`net/http`、`httptest`、RE2、现有 datasource/service/cache 契约、Docker/Dockerfile。

## Global Constraints

- 工作目录固定为 `<当前功能工作树>`，分支固定为 `feature/compact-layout`。
- 不读取或输出 `/secure/path/infraview.env`、Nightingale Token、认证头值或上游响应正文。
- 只调用代码内置的只读 Nightingale 路径；不增加任意 URL、PromQL、代理、SSH、远程命令或写操作。
- 宿主机不安装 Go；所有 Go 测试、race、格式和构建验证通过 Docker 执行。
- 严格执行 RED → GREEN；每项生产行为变更前必须先看到对应测试因该缺陷而失败。
- 保留已有未提交改动；禁止 `git reset`、`checkout`、`clean`、`restore`。
- 本计划不执行 `git add`、`git commit`、push、merge、Compose 重建或 8080 服务重启。
- 完成修复后更新 `docs/TESTING.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 和 `docs/HANDOFF.md`，但不重复真实 Nightingale、SSH 或 Token 实证。

---

### Task 1: Nightingale 审查阻断项 TDD 修复

**Files:**
- Modify: `internal/adapters/nightingale/client.go`
- Modify: `internal/adapters/nightingale/provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/adapters/nightingale/promql.go`（仅当完整白名单测试暴露真实缺陷）
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/service/metrics.go`
- Modify: `internal/service/service_test.go`
- Modify: `internal/httpapi/api_test.go`
- Modify: `docs/TESTING.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`

**Interfaces:**
- Consumes: `nightingale.Options`、`datasource.Provider`、`Service.Metrics`、现有缓存 `GetOrLoad`。
- Produces: 保持所有公开签名不变；收紧 HTTP/JSON 契约和配置校验，未知历史主机稳定映射为现有 404。

- [ ] **Step 1: 写 HTTP 重定向与 envelope RED 测试**

在 `provider_test.go` 加入真实双 `httptest.Server` 测试：

```go
func TestProviderRejectsRedirectWithoutForwardingToken(t *testing.T) {
    // destination 记录请求次数和 X-User-Token。
    // upstream 对 /api/n9e/self/profile 返回 302 Location=destination.URL。
    // provider.Health 必须返回 datasource.ErrUnavailable。
    // destination 请求次数必须为 0。
}

func TestProviderRejectsNullData(t *testing.T) {
    // 返回 {"dat":null,"err":""}。
    // Health 必须返回 datasource.ErrUnavailable。
}
```

- [ ] **Step 2: 在 Docker 中验证重定向与 null 测试 RED**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestProviderRejects(RedirectWithoutForwardingToken|NullData)$' -count=1
```

Expected: FAIL；重定向目标收到请求，或 `dat:null` 被当作成功。失败必须来自缺失行为，不得来自编译、夹具或测试设置错误。

- [ ] **Step 3: 最小实现重定向拒绝与非 null envelope**

在 `nightingale.New` 中复制客户端值并覆盖重定向策略：

```go
clientCopy := *client
clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
    return http.ErrUseLastResponse
}
provider.httpClient = &clientCopy
```

在 `do` 解码 envelope 后明确拒绝 `dat:null`，继续允许 `err:null` 和空字符串；不改变现有安全错误文本。

- [ ] **Step 4: 验证 HTTP RED 测试转 GREEN**

运行 Step 2 的同一命令。Expected: PASS。

- [ ] **Step 5: 写分页和批量响应形状 RED 测试**

在 `provider_test.go` 加入表驱动用例，分别构造：

```text
targets: missing list
targets: missing total
targets: negative total
targets: total changes between pages
targets: empty page before total
targets: records exceed total
targets: empty ident
targets: duplicate ident
instant batch: fewer result groups than queries
instant batch: more result groups than queries
range batch: fewer result groups than queries
range batch: more result groups than queries
```

每个用例断言 `errors.Is(err, datasource.ErrUnavailable)`；批量空结果的合法表达仍为与查询数相同的 `[[], ...]`。

- [ ] **Step 6: 在 Docker 中验证协议形状测试 RED**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestProviderRejects(InvalidTargetPage|MismatchedBatchResultCount)$' -count=1
```

Expected: 至少缺失字段和批量数量不匹配用例 FAIL；已有防护用例可直接 PASS，但必须记录哪些分支已由实现覆盖。

- [ ] **Step 7: 最小实现严格分页 DTO 和批量基数**

使用可区分字段缺失的分页 DTO，例如：

```go
type targetPage struct {
    List  *[]targetRecord `json:"list"`
    Total *int            `json:"total"`
}
```

在分页循环解引用前要求 `List != nil && Total != nil`，保留现有负数、变化、空页、超量、空/重复 ident 防护。

`queryInstant` 与 `queryRange` 在 POST 成功后要求：

```go
if len(result) != len(queries) {
    return nil, unavailableError()
}
```

- [ ] **Step 8: 验证协议形状测试转 GREEN**

运行 Step 6 的同一命令。Expected: PASS。

- [ ] **Step 9: 写数据源发现并发、失败重试和 PromQL 白名单覆盖**

在 `provider_test.go` 加入：

```go
func TestDatasourceDiscoveryCoalescesConcurrentRequests(t *testing.T)
func TestDatasourceDiscoveryRetriesAfterFailure(t *testing.T)
func TestPromQLWhitelistUsesExactQueriesAndSkipsUnsupportedMetrics(t *testing.T)
```

并发测试使用屏障让多个查询同时进入发现路径，断言 `/datasource/brief` 只调用一次；失败重试断言第一次失败不缓存、第二次重新发现成功；PromQL 测试用手写字面量逐项覆盖 CPU、内存、负载、IO、网络发送/接收、聚合查询，并断言不支持的磁盘指标零上游请求。

- [ ] **Step 10: 运行覆盖补强测试并判断是否需要生产改动**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'Test(DatasourceDiscovery|PromQLWhitelist)' -count=1
```

Expected: 现有正确行为可以直接 PASS。若失败，先确认测试预期与设计规格一致，再做最小生产修改；不得为让错误测试通过而改变固定 PromQL。

- [ ] **Step 11: 写 Mock 配置隔离与 Provider URL RED 测试**

在 `config_test.go` 加入 Mock 模式设置非法 Nightingale URL/Token/正则仍能成功加载的用例。

在 `provider_test.go` 加入直接构造 `nightingale.New` 的 URL 表驱动测试，拒绝：

```text
ftp://n9e.example.test
https://user:pass@n9e.example.test
https://n9e.example.test?query=value
https://n9e.example.test#fragment
```

并确认合法 `http`/`https` 绝对 URL 保持可用。

- [ ] **Step 12: 在 Docker 中验证配置与 URL 测试 RED**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/config ./internal/adapters/nightingale \
  -run 'Test(MockModeIgnoresNightingaleSettings|ProviderRejectsUnsafeBaseURL)$' -count=1
```

Expected: FAIL；现有 Mock 配置无条件编译正则，Provider 构造器接受至少一种不安全 URL。

- [ ] **Step 13: 最小实现模式隔离与 Provider URL 校验**

把 Nightingale 正则编译移动到 `cfg.DataSource == "nightingale"` 分支内。

在 `nightingale.New` 中要求：

```go
baseURL.IsAbs()
baseURL.Scheme == "http" || baseURL.Scheme == "https"
baseURL.Host != ""
baseURL.User == nil
baseURL.RawQuery == ""
baseURL.Fragment == ""
```

非法输入保持 `datasource.ErrNotConfigured`，不得包含输入值或 Token。

- [ ] **Step 14: 验证配置与 URL 测试转 GREEN**

运行 Step 12 的同一命令。Expected: PASS。

- [ ] **Step 15: 写未知历史主机 RED 测试**

在 `service_test.go` 增加 Provider 记录器测试：

```go
func TestMetricsRejectsUnknownHostBeforeRangeQueries(t *testing.T) {
    // GetHost 返回 datasource.ErrNotFound。
    // Metrics 返回 service.ErrNotFound。
    // QueryRange 调用次数为 0。
}
```

在 `api_test.go` 增加：

```text
GET /api/v1/hosts/unknown-host/metrics?range=1h
=> 404 host_not_found
```

- [ ] **Step 16: 在 Docker 中验证未知主机测试 RED**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/service ./internal/httpapi \
  -run 'Test(MetricsRejectsUnknownHostBeforeRangeQueries|QueryValidationAndMissingHost)$' -count=1
```

Expected: FAIL；当前路径会执行范围查询或返回 200。

- [ ] **Step 17: 最小实现一次主机可见性验证**

在 `loadMetricRange` 开头、范围查询循环之前调用一次：

```go
if _, err := s.provider.GetHost(ctx, id); err != nil {
    return MetricRange{}, err
}
```

复用现有 `mapProviderError` 映射；不在每个 MetricKey 中重复验证，不改变缓存键、TTL 或 stale 行为。

- [ ] **Step 18: 验证未知主机测试转 GREEN**

运行 Step 16 的同一命令。Expected: PASS。

- [ ] **Step 19: 写数值边界 RED 测试**

在 `provider_test.go` 覆盖：

```text
mem_total: MaxInt64 附近不可安全表示或越界的有限值
system_uptime: 超过 time.Duration 可表示秒数的有限值
timestamp: 超出 time.Time/Unix 转换安全边界的有限值
NaN、+Inf、-Inf
合法小数 Unix 秒保持纳秒部分
```

断言非法资产/时间按缺失处理且不会产生负数；合法值映射不变。

- [ ] **Step 20: 在 Docker 中验证数值边界测试 RED**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestProviderNumericBoundaries$' -count=1
```

Expected: 至少一个越界用例 FAIL；NaN/Inf 现有防护可以直接 PASS。

- [ ] **Step 21: 最小实现安全数值转换**

为内存、duration 和 Unix 时间加入显式且可证明的上下界检查；复用小型私有 helper，避免复制边界逻辑。不得把缺失值伪造成 0。

- [ ] **Step 22: 验证数值边界测试转 GREEN**

运行 Step 20 的同一命令。Expected: PASS。

- [ ] **Step 23: 运行 Nightingale、配置、服务和 HTTP API 相关测试**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/config ./internal/service ./internal/httpapi -count=1
```

Expected: PASS，0 failures。

- [ ] **Step 24: 运行格式、全仓普通测试和 race 测试**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm \
  sh -lc 'test -z "$(gofmt -l cmd internal)" && go test ./... && go test -race ./...'
```

Expected: 格式检查无输出，普通测试和 race 测试全部 PASS。

- [ ] **Step 25: 构建生产镜像完成前后端全量验证**

Run:

```bash
docker build --tag infraview:nightingale-review-fixes-verify .
```

Expected: Dockerfile 内前端 Vitest、typecheck、production build、Go 普通/race 测试和最终镜像构建全部成功。

- [ ] **Step 26: 更新文档**

更新 `docs/TESTING.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 和 `docs/HANDOFF.md`，准确记录：

```text
修复的 Critical/Important/Minor findings
新增的失败路径测试
本轮实际执行的 Docker 命令与结果
未执行真实 Nightingale/SSH/Token/8080 重启验证
仍未提交、推送或合并
```

- [ ] **Step 27: 最终只读差异核验**

Run:

```bash
git diff --check
git status --short --branch
git diff --stat
```

Expected: `git diff --check` 无输出；状态仅包含既有 Nightingale 工作和本计划授权的修复/文档改动。
