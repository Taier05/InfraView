# InfraView MySQL 模块实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 InfraView 中增加基于 Nightingale v8.4.1 固定批量查询的 MySQL 健康总览卡和紧凑实例列表，同时提供确定性 Mock、完整测试和严格只读边界。

**Architecture:** 新建独立 `internal/mysql` 领域契约和 `service.MySQLService`，不扩张现有 Linux 主机 `datasource.Provider`。Nightingale 的现有 `Provider` 复用同一安全 HTTP Client 和数据源发现并额外实现 `mysql.Provider`；总览和列表共用一个缓存 snapshot，所有 MySQL PromQL 固定在代码内，一次 snapshot 只调用一次批量接口。

**Tech Stack:** Go 1.24、`net/http/httptest`、内存 TTL/singleflight 缓存、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、MSW、Playwright、Docker、Docker Compose、Nightingale v8.4.1。

## Global Constraints

- InfraView 只展示数据，不执行任何运维操作。
- 不增加 SQL 执行、实例重启、主从切换、配置修改、任意 PromQL、任意代理、SSH、远程命令或写接口。
- Nightingale v8.4.1 是主要开发与真实验证版本；v9.x 保留现有协议兼容。
- 所有 MySQL PromQL 必须由代码内置，前端、HTTP 请求和配置都不能传入查询表达式。
- 实例身份固定使用 `ident + NUL + instance + NUL + address`；API ID 使用稳定 URL 安全摘要。
- 多复制通道保持每实例一行，线程状态取最差，复制延迟取最大值。
- 复制延迟阈值固定为 `<5s` 正常、`5s <= lag < 30s` 警告、`lag >= 30s` 严重。
- 可写实例缺少复制指标不告警；只读或角色未知实例缺少复制关键数据时显示未知并按警告风险计数。
- 连接、活跃线程、QPS、慢查询、Buffer Pool 和运行时间首期只展示，不设置经验阈值。
- 不输出或提交 `.env`、Token、Cookie、认证头、Base URL、真实实例标识、地址、资源数量、指标值或上游响应正文。
- Go 格式化、测试、race 和构建使用容器；不在宿主机安装 Go 或 Node 依赖。
- 生产代码严格按 RED -> GREEN TDD 实施；每个任务只提交本任务文件。
- 执行前使用 `superpowers:using-git-worktrees` 在独立 feature worktree 中工作，不直接在 `main` 上写业务代码。
- 未获得用户单独授权前，不重建现有 InfraView 服务、不更改部署、不推送远端。

---

### Task 1: 建立 MySQL 领域契约

**Files:**
- Create: `internal/mysql/types.go`
- Create: `internal/mysql/provider.go`
- Create: `internal/mysql/mysqltest/contract.go`
- Create: `internal/mysql/contract_test.go`
- Test: `internal/mysql/contract_test.go`

**Interfaces:**
- Consumes: `context.Context` 和 `errors` 标准库。
- Produces: `mysql.Provider.MySQLSnapshot(context.Context) (mysql.Snapshot, error)`、`mysql.StableInstanceID(string, string, string) string`、`mysql.Instance`、`mysql.ReplicationChannel`、`mysql.ErrUnavailable`。

- [ ] **Step 1: 写稳定身份和 Provider 契约失败测试**

```go
func TestStableInstanceIDUsesAllIdentityLabels(t *testing.T) {
	first := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.10:3306")
	again := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.10:3306")
	changedAddress := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.11:3306")
	if first == "" || first != again || first == changedAddress {
		t.Fatalf("stable IDs do not preserve the identity contract")
	}
	if strings.Contains(first, "fixture") || strings.Contains(first, "192.0.2.") {
		t.Fatalf("stable ID exposes raw labels: %q", first)
	}
}

package mysqltest

import (
	"context"
	"testing"

	"github.com/Taier05/InfraView/internal/mysql"
)

func RunContract(t testing.TB, provider mysql.Provider) {
	t.Helper()
	snapshot, err := provider.MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatalf("MySQLSnapshot() error = %v", err)
	}
	seen := map[string]struct{}{}
	for _, instance := range snapshot.Instances {
		if instance.ID == "" || instance.Name == "" || instance.Address == "" || instance.Host == "" {
			t.Fatalf("incomplete fixture instance")
		}
		if _, exists := seen[instance.ID]; exists {
			t.Fatalf("duplicate stable ID")
		}
		seen[instance.ID] = struct{}{}
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/mysql -count=1
```

Expected: FAIL，提示 `internal/mysql` 或 `StableInstanceID` 尚不存在。

- [ ] **Step 3: 实现最小领域类型和稳定 ID**

`internal/mysql/provider.go`：

```go
package mysql

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("mysql data source: unavailable")

type Provider interface {
	MySQLSnapshot(context.Context) (Snapshot, error)
}
```

`internal/mysql/types.go`：

```go
package mysql

import (
	"crypto/sha256"
	"encoding/base64"
)

type Availability string
type Role string

const (
	AvailabilityUp      Availability = "up"
	AvailabilityDown    Availability = "down"
	AvailabilityUnknown Availability = "unknown"
	RoleWritable        Role = "writable"
	RoleReadOnly        Role = "read_only"
	RoleUnknown         Role = "unknown"
)

type ReplicationChannel struct {
	IORunning  *bool
	SQLRunning *bool
	LagSeconds *float64
}

type Instance struct {
	ID                     string
	Name                   string
	Address                string
	Host                   string
	Version                string
	Availability           Availability
	Role                   Role
	UptimeSeconds          *float64
	Connections            *float64
	MaxConnections         *float64
	ThreadsRunning         *float64
	QPS                    *float64
	SlowQueriesPerSecond   *float64
	BufferPoolUsagePercent *float64
	ReplicationChannels    []ReplicationChannel
}

type Snapshot struct {
	Instances []Instance
}

func StableInstanceID(host, name, address string) string {
	sum := sha256.Sum256([]byte(host + "\x00" + name + "\x00" + address))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
```

- [ ] **Step 4: 容器化格式化并确认 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/mysql/types.go internal/mysql/provider.go internal/mysql/mysqltest/contract.go internal/mysql/contract_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/mysql -count=1
```

Expected: PASS。

- [ ] **Step 5: 提交领域契约**

```bash
git add internal/mysql
git diff --cached --check
git commit -m "feat: 定义 MySQL 只读领域契约"
```

---

### Task 2: 实现确定性 MySQL Mock Provider

**Files:**
- Create: `internal/adapters/mock/mysql_provider.go`
- Create: `internal/adapters/mock/mysql_provider_test.go`
- Test: `internal/adapters/mock/mysql_provider_test.go`

**Interfaces:**
- Consumes: `mysql.Provider`、`mysql.Snapshot`、`mysql.Instance` 和 `mysqltest.RunContract`。
- Produces: `mock.NewMySQL() mysql.Provider`，供 Service、主程序和 E2E 使用。

- [ ] **Step 1: 写 Mock 契约和场景失败测试**

```go
func TestMySQLProviderContract(t *testing.T) {
	mysqltest.RunContract(t, mock.NewMySQL())
}

func TestMySQLProviderContainsDeterministicHealthScenarios(t *testing.T) {
	first, err := mock.NewMySQL().MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := mock.NewMySQL().MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}
	if len(first.Instances) < 6 {
		t.Fatal("mock must cover normal, warning, critical and unknown scenarios")
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/mock -run 'TestMySQLProvider' -count=1
```

Expected: FAIL，提示 `NewMySQL` 未定义。

- [ ] **Step 3: 实现完全虚构的固定 snapshot**

在 `mysql_provider.go` 中实现 `mysqlProvider`，固定返回：

- 可写、可用、无复制通道实例。
- 只读、可用、复制延迟低于 5 秒实例。
- 只读、延迟处于警告区间实例。
- 只读、延迟达到严重区间实例。
- 复制线程停止实例。
- 只读但复制数据缺失实例。
- 可用性未知及辅助指标缺失实例。

所有名称使用 `fixture-mysql-*`，主机使用 `fixture-host-*`，地址使用 RFC 5737 文档地址；构造实例时调用：

```go
func mockMySQLInstance(host, name, address string) mysql.Instance {
	return mysql.Instance{
		ID:      mysql.StableInstanceID(host, name, address),
		Host:    host,
		Name:    name,
		Address: address,
	}
}

func NewMySQL() mysql.Provider {
	return &mysqlProvider{}
}
```

- [ ] **Step 4: 格式化并确认 Mock GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/adapters/mock/mysql_provider.go internal/adapters/mock/mysql_provider_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/mock ./internal/mysql -count=1
```

Expected: PASS。

- [ ] **Step 5: 提交 Mock Provider**

```bash
git add internal/adapters/mock/mysql_provider.go internal/adapters/mock/mysql_provider_test.go
git diff --cached --check
git commit -m "feat: 增加确定性 MySQL Mock 数据"
```

---

### Task 3: 实现 MySQL 实例规范化和告警计算

**Files:**
- Create: `internal/service/mysql_types.go`
- Create: `internal/service/mysql_service.go`
- Create: `internal/service/mysql_service_test.go`
- Test: `internal/service/mysql_service_test.go`

**Interfaces:**
- Consumes: `mysql.Instance`、`mysql.Provider`、现有 `service.Level` 和 `service.Meta`。
- Produces: `service.NewMySQL(mysql.Provider, *cache.Store, service.MySQLOptions) *service.MySQLService`、`service.MySQLInstanceSummary`、`service.MySQLReplicationSummary`。

- [ ] **Step 1: 写复制边界和最终等级失败测试**

```go
func TestMySQLSummaryAggregatesReplicationChannelsAndBoundaries(t *testing.T) {
	tests := []struct {
		name string
		lag  float64
		want Level
	}{
		{name: "below warning", lag: 4.999, want: LevelNormal},
		{name: "warning boundary", lag: 5, want: LevelWarning},
		{name: "below critical", lag: 29.999, want: LevelWarning},
		{name: "critical boundary", lag: 30, want: LevelCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMySQLInstance(readOnlyInstanceWithLag(tt.lag))
			if summary.Replication.Level != tt.want {
				t.Fatalf("level = %q, want %q", summary.Replication.Level, tt.want)
			}
		})
	}
}

func TestMySQLSummaryMakesStoppedThreadCritical(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(false), SQLRunning: boolPointer(true), LagSeconds: floatPointer(2)},
	)
	summary := summarizeMySQLInstance(instance)
	if summary.Status != LevelCritical ||
		summary.Replication.State != ReplicationThreadsStopped {
		t.Fatalf("instance = %#v", summary)
	}
}
```

`mysql_service_test.go` 使用 `package service`，使聚焦领域测试可以直接调用未导出的纯函数，不提前依赖 Task 4 才实现的分页 API。

同一测试文件定义以下确定性 helper，后续 Task 4 继续复用：

```go
func boolPointer(value bool) *bool       { return &value }
func floatPointer(value float64) *float64 { return &value }

func readOnlyInstanceWithLag(lag float64) mysql.Instance {
	return instanceWithChannels(mysql.ReplicationChannel{
		IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(lag),
	})
}

func instanceWithChannels(channels ...mysql.ReplicationChannel) mysql.Instance {
	return mysql.Instance{
		ID: mysql.StableInstanceID("fixture-host-a", "fixture-mysql-a", "192.0.2.10:3306"),
		Name: "fixture-mysql-a", Address: "192.0.2.10:3306", Host: "fixture-host-a",
		Availability: mysql.AvailabilityUp, Role: mysql.RoleReadOnly,
		ReplicationChannels: channels,
	}
}

type recordingMySQLProvider struct {
	snapshot mysql.Snapshot
	err      error
	calls    int
}

func (p *recordingMySQLProvider) MySQLSnapshot(context.Context) (mysql.Snapshot, error) {
	p.calls++
	return p.snapshot, p.err
}
```

- [ ] **Step 2: 运行聚焦测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service \
  -run 'TestMySQLSummary(AggregatesReplicationChannelsAndBoundaries|MakesStoppedThreadCritical)$' \
  -count=1
```

Expected: FAIL，提示 `summarizeMySQLInstance` 或复制输出类型未定义。

- [ ] **Step 3: 定义 Service 输出类型**

`mysql_types.go` 定义：

```go
type MySQLOptions struct {
	CurrentMetricsTTL time.Duration
	MaxStale          time.Duration
	Clock             func() time.Time
}

type MySQLQuery struct {
	Search   string
	Status   Level
	Role     mysql.Role
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type MySQLReplicationState string

const (
	ReplicationNormal         MySQLReplicationState = "normal"
	ReplicationThreadsStopped MySQLReplicationState = "threads_stopped"
	ReplicationNotConfigured  MySQLReplicationState = "not_configured"
	ReplicationUnknown        MySQLReplicationState = "unknown"
)

type MySQLReplicationSummary struct {
	State      MySQLReplicationState
	LagSeconds *float64
	Level      Level
}

type MySQLInstanceSummary struct {
	ID                     string
	Name                   string
	Address                string
	Host                   string
	Version                string
	Role                   mysql.Role
	Connections            *float64
	MaxConnections         *float64
	ConnectionUsagePercent *float64
	ThreadsRunning         *float64
	QPS                    *float64
	SlowQueriesPerSecond   *float64
	BufferPoolUsagePercent *float64
	UptimeSeconds          *float64
	Replication            MySQLReplicationSummary
	Status                 Level
}
```

- [ ] **Step 4: 实现可用性、角色和复制聚合**

在 `mysql_service.go` 中实现：

```go
const mysqlSnapshotCacheKey = "service:mysql:snapshot"

type MySQLService struct {
	provider mysql.Provider
	store    *cache.Store
	options  MySQLOptions
}

func NewMySQL(provider mysql.Provider, store *cache.Store, options MySQLOptions) *MySQLService
func summarizeMySQLInstance(source mysql.Instance) MySQLInstanceSummary
func replicationSummary(role mysql.Role, channels []mysql.ReplicationChannel) MySQLReplicationSummary
func mysqlHigherLevel(left, right Level) Level
```

`mysqlHigherLevel` 使用 `critical > warning > unknown > normal`。线程停止先返回严重；其余通道取最大有效延迟。可写且无通道返回 `not_configured/normal`；只读或角色未知且数据不足返回 `unknown/unknown`。

- [ ] **Step 5: 补充缺失语义和连接使用率测试**

增加明确用例：

```go
func TestMySQLSummaryAppliesRoleAndMissingReplicationSemantics(t *testing.T) {
	writable := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleWritable, Availability: mysql.AvailabilityUp})
	if writable.Replication.State != ReplicationNotConfigured || writable.Status != LevelNormal {
		t.Fatalf("writable = %#v", writable)
	}
	readOnly := summarizeMySQLInstance(mysql.Instance{Role: mysql.RoleReadOnly, Availability: mysql.AvailabilityUp})
	if readOnly.Replication.State != ReplicationUnknown || readOnly.Status != LevelUnknown {
		t.Fatalf("readOnly = %#v", readOnly)
	}
	unknownAvailability := summarizeMySQLInstance(mysql.Instance{
		Role: mysql.RoleWritable, Availability: mysql.AvailabilityUnknown,
	})
	if unknownAvailability.Status != LevelUnknown {
		t.Fatalf("unknown availability = %#v", unknownAvailability)
	}
}

func TestMySQLSummaryCalculatesOnlyValidConnectionUsageAndMaximumLag(t *testing.T) {
	instance := instanceWithChannels(
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(1)},
		mysql.ReplicationChannel{IORunning: boolPointer(true), SQLRunning: boolPointer(true), LagSeconds: floatPointer(8)},
	)
	instance.Connections = floatPointer(25)
	instance.MaxConnections = floatPointer(100)
	summary := summarizeMySQLInstance(instance)
	if summary.ConnectionUsagePercent == nil || *summary.ConnectionUsagePercent != 25 {
		t.Fatalf("connection usage = %#v", summary.ConnectionUsagePercent)
	}
	if summary.Replication.LagSeconds == nil || *summary.Replication.LagSeconds != 8 {
		t.Fatalf("replication lag = %#v", summary.Replication.LagSeconds)
	}
	instance.MaxConnections = floatPointer(0)
	if got := summarizeMySQLInstance(instance).ConnectionUsagePercent; got != nil {
		t.Fatalf("zero maximum produced usage %#v", got)
	}
}
```

- [ ] **Step 6: 格式化并确认告警计算 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/service/mysql_types.go internal/service/mysql_service.go internal/service/mysql_service_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'TestMySQL' -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交规范化和告警计算**

```bash
git add internal/service/mysql_types.go internal/service/mysql_service.go internal/service/mysql_service_test.go
git diff --cached --check
git commit -m "feat: 计算 MySQL 实例健康状态"
```

---

### Task 4: 完成 MySQL snapshot 缓存、总览和列表查询

**Files:**
- Modify: `internal/service/mysql_types.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/mysql_service_test.go`
- Test: `internal/service/mysql_service_test.go`

**Interfaces:**
- Consumes: Task 3 的 `MySQLService` 和 `MySQLInstanceSummary`。
- Produces: `MySQLService.Overview(context.Context) (MySQLOverview, Meta, error)`、`MySQLService.Instances(context.Context, MySQLQuery) (MySQLPage, Meta, error)`。

- [ ] **Step 1: 写缓存、singleflight 和 stale 失败测试**

```go
func TestMySQLServiceSharesOneSnapshotAcrossOverviewAndList(t *testing.T) {
	provider := &recordingMySQLProvider{snapshot: fixtureMySQLSnapshot()}
	clock := &mysqlTestClock{now: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)}
	store := cache.New(clock.Now)
	svc := NewMySQL(provider, store, MySQLOptions{
		CurrentMetricsTTL: 15 * time.Second,
		MaxStale:          5 * time.Minute,
		Clock:             clock.Now,
	})
	if _, _, err := svc.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("MySQLSnapshot calls = %d, want 1", provider.calls)
	}
}

func TestMySQLServiceReturnsStaleSnapshotAfterUpstreamFailure(t *testing.T) {
	provider, clock, svc := newCachingMySQLService(fixtureMySQLSnapshot())
	if _, _, err := svc.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(16 * time.Second)
	provider.err = mysql.ErrUnavailable
	_, meta, err := svc.Overview(context.Background())
	if err != nil || !meta.Stale {
		t.Fatalf("meta = %#v, err = %v", meta, err)
	}
}

func TestMySQLServiceCachesSuccessfulEmptySnapshot(t *testing.T) {
	provider, _, svc := newCachingMySQLService(mysql.Snapshot{Instances: []mysql.Instance{}})
	overview, _, err := svc.Overview(context.Background())
	if err != nil || overview.Total != 0 || provider.calls != 1 {
		t.Fatalf("overview = %#v, calls = %d, err = %v", overview, provider.calls, err)
	}
}

func TestMySQLServiceReturnsIndependentSnapshotCopies(t *testing.T) {
	_, _, svc := newCachingMySQLService(fixtureMySQLSnapshot())
	first, _, _ := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20})
	first.Instances[0].Name = "mutated"
	second, _, _ := svc.Instances(context.Background(), MySQLQuery{Page: 1, PageSize: 20})
	if second.Instances[0].Name == "mutated" {
		t.Fatal("cached snapshot was mutated by a caller")
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'TestMySQLService' -count=1
```

Expected: FAIL，缓存读取或总览接口尚未实现。

- [ ] **Step 3: 实现 snapshot 加载和深复制**

```go
func (s *MySQLService) snapshot(ctx context.Context) (mysql.Snapshot, Meta, error) {
	result, err := s.store.GetOrLoad(
		ctx,
		mysqlSnapshotCacheKey,
		s.options.CurrentMetricsTTL,
		s.options.MaxStale,
		func(loadCtx context.Context) (any, error) {
			return s.provider.MySQLSnapshot(loadCtx)
		},
	)
	if err != nil {
		return mysql.Snapshot{}, Meta{}, err
	}
	snapshot, ok := result.Value.(mysql.Snapshot)
	if !ok {
		return mysql.Snapshot{}, Meta{}, fmt.Errorf("service: mysql cache contained %T", result.Value)
	}
	return cloneMySQLSnapshot(snapshot), resultMeta(result), nil
}
```

`cloneMySQLSnapshot` 必须复制所有数值指针和 `ReplicationChannels`。

- [ ] **Step 4: 写总览计数失败测试**

```go
func TestMySQLOverviewCountsUnknownAsWarningRisk(t *testing.T) {
	overview, _, err := newMySQLServiceWithSnapshot(alertCategoryFixtureSnapshot()).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Total == 0 ||
		overview.WarningInstances != overview.Warning+overview.Unknown ||
		overview.AffectedInstances != overview.Warning+overview.Unknown+overview.Critical {
		t.Fatalf("overview = %#v", overview)
	}
}

func TestMySQLOverviewCountsAlertCategoriesIndependently(t *testing.T) {
	svc := newMySQLServiceWithSnapshot(alertCategoryFixtureSnapshot())
	overview, _, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Alerts.Availability.Critical == 0 ||
		overview.Alerts.ReplicationThreads.Critical == 0 ||
		overview.Alerts.ReplicationLag.Warning == 0 ||
		overview.Alerts.ReplicationData.Warning == 0 {
		t.Fatalf("alerts = %#v", overview.Alerts)
	}
}
```

- [ ] **Step 5: 定义并实现总览类型**

```go
type MySQLAlertCount struct {
	Warning  int
	Critical int
}

type MySQLOverviewAlerts struct {
	Availability       MySQLAlertCount
	ReplicationThreads MySQLAlertCount
	ReplicationLag     MySQLAlertCount
	ReplicationData    MySQLAlertCount
}

type MySQLOverview struct {
	Total             int
	Normal            int
	Warning           int
	Critical          int
	Unknown           int
	AffectedInstances int
	WarningInstances  int
	CriticalInstances int
	Alerts            MySQLOverviewAlerts
}
```

`Overview` 从同一 snapshot 映射 summaries；四个状态互斥，未知同时计入 `WarningInstances` 和 `AffectedInstances`。

- [ ] **Step 6: 写列表规范化、筛选、排序和分页失败测试**

```go
func TestMySQLInstancesSearchesNameAddressAndHost(t *testing.T) {
	for _, search := range []string{"fixture-mysql", "192.0.2.", "fixture-host"} {
		page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
			Search: search, Page: 1, PageSize: 20,
		})
		if err != nil || len(page.Instances) == 0 {
			t.Fatalf("search %q returned %#v, err = %v", search, page, err)
		}
	}
}

func TestMySQLInstancesFiltersStatusAndRole(t *testing.T) {
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Status: LevelWarning, Role: mysql.RoleReadOnly, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, instance := range page.Instances {
		if instance.Status != LevelWarning || instance.Role != mysql.RoleReadOnly {
			t.Fatalf("unexpected instance %#v", instance)
		}
	}
}

func TestMySQLInstancesSortsMissingMetricsLastAndUsesStableTieBreak(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
			Sort: "qps", Order: order, Page: 1, PageSize: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		assertAvailableValuesBeforeMissing(t, page.Instances, func(item MySQLInstanceSummary) *float64 { return item.QPS })
		assertStableIDTieBreak(t, page.Instances)
	}
}

func TestMySQLInstancesAcceptsOnlySupportedPageSizesAndHandlesOverflow(t *testing.T) {
	for _, pageSize := range []int{1, 19, 21, 101} {
		_, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{Page: 1, PageSize: pageSize})
		if !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("page size %d error = %v", pageSize, err)
		}
	}
	page, _, err := fixtureMySQLService().Instances(context.Background(), MySQLQuery{
		Page: math.MaxInt, PageSize: 20,
	})
	if err != nil || len(page.Instances) != 0 {
		t.Fatalf("overflow page = %#v, err = %v", page, err)
	}
}
```

Task 4 在测试文件补充以下 helper，所有数据仍使用虚构身份：

```go
type mysqlTestClock struct{ now time.Time }

func (c *mysqlTestClock) Now() time.Time             { return c.now }
func (c *mysqlTestClock) Advance(delta time.Duration) { c.now = c.now.Add(delta) }

func newCachingMySQLService(snapshot mysql.Snapshot) (*recordingMySQLProvider, *mysqlTestClock, *MySQLService) {
	provider := &recordingMySQLProvider{snapshot: snapshot}
	clock := &mysqlTestClock{now: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)}
	svc := NewMySQL(provider, cache.New(clock.Now), MySQLOptions{
		CurrentMetricsTTL: 15 * time.Second,
		MaxStale:          5 * time.Minute,
		Clock:             clock.Now,
	})
	return provider, clock, svc
}

func newMySQLServiceWithSnapshot(snapshot mysql.Snapshot) *MySQLService {
	_, _, svc := newCachingMySQLService(snapshot)
	return svc
}

func fixtureMySQLService() *MySQLService {
	return newMySQLServiceWithSnapshot(fixtureMySQLSnapshot())
}

func assertAvailableValuesBeforeMissing(
	t *testing.T,
	items []MySQLInstanceSummary,
	value func(MySQLInstanceSummary) *float64,
) {
	t.Helper()
	seenMissing := false
	for _, item := range items {
		if value(item) == nil {
			seenMissing = true
		} else if seenMissing {
			t.Fatal("available metric sorted after a missing metric")
		}
	}
}
```

`fixtureMySQLSnapshot` 和 `alertCategoryFixtureSnapshot` 使用 Task 3 的 `instanceWithChannels` 构造固定实例；`assertStableIDTieBreak` 对相同排序值的相邻实例断言 ID 升序。

- [ ] **Step 7: 实现列表查询白名单**

```go
type MySQLPage struct {
	Instances []MySQLInstanceSummary
	Total     int
	Page      int
	PageSize  int
}

func normalizeMySQLQuery(query MySQLQuery) (MySQLQuery, error)
func sortMySQLInstances(items []MySQLInstanceSummary, field, order string)
```

默认 `sort=instance`、`order=asc`；允许 `instance`、`connections`、`threads_running`、`qps`、`slow_queries`、`buffer_pool`、`replication_lag`、`uptime`、`status`。搜索匹配 `Name`、`Address`、`Host`。

- [ ] **Step 8: 格式化并运行完整 Service 测试**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/service/mysql_types.go internal/service/mysql_service.go internal/service/mysql_service_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -count=1
```

Expected: PASS。

- [ ] **Step 9: 提交 MySQL Service**

```bash
git add internal/service/mysql_types.go internal/service/mysql_service.go internal/service/mysql_service_test.go
git diff --cached --check
git commit -m "feat: 增加 MySQL 总览和实例查询服务"
```

---

### Task 5: 实现 Nightingale MySQL 固定批量查询

**Files:**
- Create: `internal/adapters/nightingale/mysql_promql.go`
- Create: `internal/adapters/nightingale/mysql_provider.go`
- Create: `internal/adapters/nightingale/testdata/mysql-instant-batch.json`
- Modify: `internal/adapters/nightingale/provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Test: `internal/adapters/nightingale/provider_test.go`

**Interfaces:**
- Consumes: 现有 `Provider.queryInstant(context.Context, []string)` 和 `instantSeries`。
- Produces: `(*nightingale.Provider).MySQLSnapshot(context.Context) (mysql.Snapshot, error)`，使 `*nightingale.Provider` 同时满足 `datasource.Provider` 和 `mysql.Provider`。

- [ ] **Step 1: 写固定查询顺序失败测试**

```go
func TestMySQLPromQLIsFixed(t *testing.T) {
	want := []string{
		"mysql_up",
		"mysql_version_info",
		"mysql_global_status_uptime",
		"mysql_global_variables_read_only",
		"mysql_global_status_threads_connected",
		"mysql_global_variables_max_connections",
		"mysql_global_status_threads_running",
		"rate(mysql_global_status_questions[5m])",
		"rate(mysql_global_status_slow_queries[5m])",
		"mysql_global_status_buffer_pool_pages_utilization",
		"mysql_slave_status_seconds_behind_master",
		"mysql_slave_status_slave_io_running",
		"mysql_slave_status_slave_sql_running",
	}
	if got := mysqlPromQL(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mysqlPromQL() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestMySQL' -count=1
```

Expected: FAIL，提示 `mysqlPromQL` 未定义。

- [ ] **Step 3: 实现固定查询列表**

`mysql_promql.go` 仅返回 Step 1 的 13 个常量查询，不接收参数，不拼接用户输入。

- [ ] **Step 4: 添加完全脱敏的批量夹具和 happy-path 失败测试**

夹具外层 `dat` 必须严格包含 13 个查询结果。实例标签仅使用：

```json
{
  "__name__": "mysql_up",
  "ident": "fixture-host-a",
  "instance": "fixture-mysql-a",
  "address": "192.0.2.10:3306"
}
```

复制查询使用虚构 `channel_name`、`master_host=fixture-source-a` 和固定虚构 UUID。测试：

```go
func TestMySQLSnapshotUsesOneBatchAndMergesInstances(t *testing.T) {
	var batchCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			batchCalls++
			assertMySQLBatchRequest(t, r, mysqlPromQL())
			writeFixture(t, w, "mysql-instant-batch.json")
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	provider := newFixtureProvider(server)
	snapshot, err := provider.MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if batchCalls != 1 || len(snapshot.Instances) == 0 {
		t.Fatalf("calls = %d, snapshot = %#v", batchCalls, snapshot)
	}
}
```

- [ ] **Step 5: 让 Nightingale 构造器返回可共享的具体 Provider**

将：

```go
func New(option Options) datasource.Provider
```

改为：

```go
func New(option Options) *Provider
```

保留：

```go
var _ datasource.Provider = (*Provider)(nil)
var _ mysql.Provider = (*Provider)(nil)
```

现有调用方仍可把返回值赋给 `datasource.Provider`。

- [ ] **Step 6: 实现实例身份和辅助指标合并**

`mysql_provider.go` 实现：

```go
func (p *Provider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error)
func mysqlIdentity(labels map[string]string) (host, name, address, key string, ok bool)
func mysqlBinary(series instantSeries) (*bool, time.Time, bool)
func mergeMySQLScalar(target **float64, targetTime *time.Time, candidate instantSeries)
```

处理顺序：

1. 用 `mysql_up` 建立实例集合；关键标签缺失或完整键重复返回 `mysql.ErrUnavailable`。
2. `mysql_up` 非 0/1 时保留实例并标记 `AvailabilityUnknown`。
3. 其他非复制指标按完整键选择最新有效样本；同时间冲突时字段为 nil。
4. `mysql_version_info` 从 `version` 标签读取版本，冲突时留空。
5. 复制通道用 `channel_name + NUL + master_host + NUL + master_uuid` 在实例内分组，但不把这些标签写入领域输出。
6. 无法匹配已有实例的辅助序列忽略，不创建幽灵实例。

- [ ] **Step 7: 格式化并确认 Nightingale happy path GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/adapters/nightingale/mysql_promql.go \
    internal/adapters/nightingale/mysql_provider.go \
    internal/adapters/nightingale/provider.go \
    internal/adapters/nightingale/provider_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'TestMySQL' -count=1
```

Expected: PASS。

- [ ] **Step 8: 提交 Nightingale happy path**

```bash
git add internal/adapters/nightingale internal/mysql
git diff --cached --check
git commit -m "feat: 接入 Nightingale MySQL 当前指标"
```

---

### Task 6: 加固 Nightingale MySQL 契约和错误边界

**Files:**
- Modify: `internal/adapters/nightingale/mysql_provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Test: `internal/adapters/nightingale/provider_test.go`

**Interfaces:**
- Consumes: Task 5 的 `MySQLSnapshot`。
- Produces: 对缺失、重复、非法值、批量错误和敏感信息泄露的完整防御。

- [ ] **Step 1: 写关键身份和批量形状失败测试**

```go
func TestMySQLSnapshotEnforcesIdentityAndBatchShape(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([][]instantSeries) [][]instantSeries
		wantErr bool
		empty   bool
	}{
		{name: "missing identity", wantErr: true, mutate: func(result [][]instantSeries) [][]instantSeries {
			delete(result[0][0].Metric, "ident")
			return result
		}},
		{name: "duplicate full identity", wantErr: true, mutate: func(result [][]instantSeries) [][]instantSeries {
			result[0] = append(result[0], result[0][0])
			return result
		}},
		{name: "wrong outer cardinality", wantErr: true, mutate: func(result [][]instantSeries) [][]instantSeries {
			return result[:len(result)-1]
		}},
		{name: "successful empty up vector", empty: true, mutate: func(result [][]instantSeries) [][]instantSeries {
			result[0] = []instantSeries{}
			return result
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := runMySQLBatch(t, tt.mutate(mysqlBatchResultsFixture()))
			if tt.wantErr != errors.Is(err, mysql.ErrUnavailable) {
				t.Fatalf("error = %v, want unavailable=%v", err, tt.wantErr)
			}
			if tt.empty && (err != nil || len(snapshot.Instances) != 0) {
				t.Fatalf("snapshot = %#v, err = %v", snapshot, err)
			}
		})
	}
}
```

每个测试使用 `httptest.Server` 返回完全虚构 envelope，并断言 `errors.Is(err, mysql.ErrUnavailable)`；空 `mysql_up` 断言 `len(snapshot.Instances)==0` 且无错误。

- [ ] **Step 2: 写数值和冲突选择失败测试**

```go
func TestMySQLSnapshotNormalizesValuesAndConflicts(t *testing.T) {
	result := mysqlBatchResultsFixture()
	result[0][0].Value = rawInstantValue(1785200000, "2")
	result[2][0].Value = rawInstantValue(1785200000, "-1")
	result[7][0].Value = rawInstantValue(1785200000, "-1")
	result[9][0].Value = rawInstantValue(1785200000, "101")
	result[4] = append(result[4],
		instantSeries{Metric: cloneLabels(result[4][0].Metric), Value: rawInstantValue(1785200100, "20")},
		instantSeries{Metric: cloneLabels(result[4][0].Metric), Value: rawInstantValue(1785200100, "21")},
	)
	snapshot, err := runMySQLBatch(t, result)
	if err != nil {
		t.Fatal(err)
	}
	instance := snapshot.Instances[0]
	if instance.Availability != mysql.AvailabilityUnknown ||
		instance.UptimeSeconds != nil ||
		instance.QPS != nil ||
		instance.BufferPoolUsagePercent != nil ||
		instance.Connections != nil {
		t.Fatalf("instance = %#v", instance)
	}
}

func TestMySQLSnapshotKeepsReplicationChannelsSeparateAndSelectsNewestValue(t *testing.T) {
	result := mysqlBatchResultsFixture()
	result[4] = append(result[4], instantSeries{
		Metric: cloneLabels(result[4][0].Metric),
		Value: rawInstantValue(1785200100, "22"),
	})
	addSecondFixtureReplicationChannel(result)
	snapshot, err := runMySQLBatch(t, result)
	if err != nil {
		t.Fatal(err)
	}
	instance := snapshot.Instances[0]
	if instance.Connections == nil || *instance.Connections != 22 ||
		len(instance.ReplicationChannels) != 2 {
		t.Fatalf("instance = %#v", instance)
	}
}
```

- [ ] **Step 3: 实现最小严格校验**

新增局部 helper：

```go
func nonNegative(value float64) bool
func percentage(value float64) bool
func newestMySQLValue(current **float64, currentAt *time.Time, candidate float64, candidateAt time.Time) bool
```

所有非法辅助值只使字段缺失；关键身份错误和批量结构错误使整个 snapshot 不可用。

- [ ] **Step 4: 写错误脱敏和一次批量调用测试**

```go
func TestMySQLSnapshotErrorDoesNotExposeTokenBodyLabelsOrQueries(t *testing.T) {
	provider := providerReturningMySQLHTTPError(t, "fixture-secret-token", "fixture-mysql-sensitive")
	_, err := provider.MySQLSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	for _, forbidden := range []string{
		"fixture-secret-token", "fixture-mysql-sensitive", "mysql_up", "<html>",
	} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked %q", forbidden)
		}
	}
}

func TestMySQLSnapshotUsesOnlyDiscoveryAndOneInstantBatch(t *testing.T) {
	var paths []string
	provider := providerRecordingMySQLPaths(t, &paths, mysqlBatchResultsFixture())
	if _, err := provider.MySQLSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/api/n9e/datasource/brief",
		"/api/n9e/query-instant-batch",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}
```

测试 helper `runMySQLBatch` 用 `httptest.Server` 为 `/datasource/brief` 返回脱敏 fixture，并用 `writeEnvelope` 返回传入的 `[][]instantSeries`；其他路径直接 `t.Fatalf`。`rawInstantValue` 只生成固定时间戳和字符串值，`cloneLabels` 深复制 label map，避免表驱动用例互相污染。

错误字符串不得包含夹具 Token、HTML 正文、`fixture-mysql-*`、文档地址或固定查询文本。请求路径集合必须只有数据源 brief 和一次 instant batch。

- [ ] **Step 5: 运行 Nightingale 普通和 race 测试**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -count=1
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test -race ./internal/adapters/nightingale -count=1
```

Expected: 两次 PASS。

- [ ] **Step 6: 提交契约加固**

```bash
git add internal/adapters/nightingale/mysql_provider.go internal/adapters/nightingale/provider_test.go
git diff --cached --check
git commit -m "test: 加固 MySQL 指标契约边界"
```

---

### Task 7: 装配 MySQL Provider 集合和超时边界

**Files:**
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`
- Test: `cmd/infraview/main_test.go`

**Interfaces:**
- Consumes: `mock.NewMySQL()`、`mysqltest.RunContract` 和同时实现两个 Provider 接口的 `*nightingale.Provider`。
- Produces: `providerSet{Hosts datasource.Provider, MySQL mysql.Provider}` 和 `withMySQLUpstreamTimeout`，供 Task 8 创建 MySQL Service。

- [ ] **Step 1: 写装配失败测试**

```go
func TestProviderSetUsesMockForHostsAndMySQL(t *testing.T) {
	providers := dataSourceProviders(config.Config{
		DataSource: "mock", MockHostCount: 8,
	}, time.Now)
	if providers.Hosts == nil || providers.MySQL == nil {
		t.Fatalf("providers = %#v", providers)
	}
	mysqltest.RunContract(t, providers.MySQL)
}

func TestProviderSetSharesOneNightingaleProvider(t *testing.T) {
	providers := dataSourceProviders(config.Config{
		DataSource: "nightingale",
		NightingaleBaseURL: "https://n9e.example.com",
		NightingaleToken: "fixture-token",
	}, time.Now)
	hostProvider, hostOK := providers.Hosts.(*nightingale.Provider)
	mysqlProvider, mysqlOK := providers.MySQL.(*nightingale.Provider)
	if !hostOK || !mysqlOK || hostProvider != mysqlProvider {
		t.Fatalf("providers do not share one Nightingale client")
	}
}

func TestMySQLTimeoutProviderCancelsSlowSnapshot(t *testing.T) {
	provider := withMySQLUpstreamTimeout(blockingMySQLProvider{}, 10*time.Millisecond)
	start := time.Now()
	_, err := provider.MySQLSnapshot(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("error = %v, elapsed = %s", err, time.Since(start))
	}
}

type blockingMySQLProvider struct{}

func (blockingMySQLProvider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error) {
	<-ctx.Done()
	return mysql.Snapshot{}, ctx.Err()
}
```

共享测试断言 Nightingale 模式下 `Hosts` 和 `MySQL` 的底层具体指针相同；超时测试使用阻塞 Provider 并断言返回 `context.DeadlineExceeded` 或安全不可用错误。

- [ ] **Step 2: 运行测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./cmd/infraview -run 'Test(ProviderSet|MySQLTimeout)' -count=1
```

Expected: FAIL，`providerSet` 和 MySQL 依赖尚未定义。

- [ ] **Step 3: 实现 provider set 和独立超时包装**

```go
type providerSet struct {
	Hosts datasource.Provider
	MySQL mysql.Provider
}

func dataSourceProviders(cfg config.Config, clock func() time.Time) providerSet

type mysqlTimeoutProvider struct {
	provider mysql.Provider
	timeout  time.Duration
}

func (p *mysqlTimeoutProvider) MySQLSnapshot(ctx context.Context) (mysql.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.MySQLSnapshot(ctx)
}
```

Mock 模式分别构造主机和 MySQL Provider；Nightingale 模式只调用一次 `nightingale.New` 并把同一指针赋给两个接口。

- [ ] **Step 4: 让现有主机装配使用 provider set**

```go
providers := dataSourceProviders(cfg, clock)
hostProvider := withUpstreamTimeout(providers.Hosts, cfg.UpstreamTimeout)
store := cache.New(clock)
```

Task 7 只替换原 `dataSourceProvider` 调用并保持现有主机 Service 参数原样；`providers.MySQL` 在 `providerSet` 中保留，由 Task 8 完成 Service 和 HTTP 装配。

- [ ] **Step 5: 格式化并确认装配 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w cmd/infraview/main.go cmd/infraview/main_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./cmd/infraview ./internal/service ./internal/adapters/mock ./internal/adapters/nightingale -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交依赖装配**

```bash
git add cmd/infraview
git diff --cached --check
git commit -m "feat: 装配 MySQL 数据源能力"
```

---

### Task 8: 增加 MySQL 只读 HTTP API 并完成 Service 装配

**Files:**
- Create: `internal/httpapi/mysql_handlers.go`
- Modify: `internal/httpapi/api.go`
- Modify: `internal/httpapi/api_test.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`
- Test: `internal/httpapi/api_test.go`

**Interfaces:**
- Consumes: `service.NewMySQL`、`service.MySQLService.Overview`、`service.MySQLService.Instances` 和 Task 7 的 `providerSet`。
- Produces: `GET /api/v1/mysql/overview`、`GET /api/v1/mysql/instances` 和稳定 JSON view。

- [ ] **Step 1: 写路由、认证和只读方法失败测试**

```go
func TestMySQLRoutesRequireAuthentication(t *testing.T) {
	handler, _ := newMySQLAPITestHandler(t, fixtureMySQLSnapshot())
	for _, path := range []string{"/api/v1/mysql/overview", "/api/v1/mysql/instances"} {
		response := serveJSONRequest(t, handler, http.MethodGet, path, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
}

func TestMySQLOverviewAndInstancesReturnReadOnlyViews(t *testing.T) {
	handler, sessionCookie := newMySQLAPITestHandler(t, fixtureMySQLSnapshot())
	overview := serveAuthenticatedJSONRequest(t, handler, sessionCookie, http.MethodGet, "/api/v1/mysql/overview")
	if overview.Code != http.StatusOK ||
		!jsonPathIsNumber(t, overview.Body.Bytes(), "data.total") ||
		!jsonPathIsObject(t, overview.Body.Bytes(), "data.alerts.replication_lag") {
		t.Fatalf("invalid overview response")
	}
	instances := serveAuthenticatedJSONRequest(
		t, handler, sessionCookie, http.MethodGet,
		"/api/v1/mysql/instances?page=1&page_size=20",
	)
	if instances.Code != http.StatusOK ||
		!jsonPathIsArray(t, instances.Body.Bytes(), "data.instances") ||
		!jsonPathAllowsNull(t, instances.Body.Bytes(), "data.instances.0.qps") {
		t.Fatalf("invalid instances response")
	}
}

func TestMySQLRoutesRejectMutationMethods(t *testing.T) {
	handler, sessionCookie := newMySQLAPITestHandler(t, fixtureMySQLSnapshot())
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, path := range []string{"/api/v1/mysql/overview", "/api/v1/mysql/instances"} {
			response := serveAuthenticatedJSONRequest(t, handler, sessionCookie, method, path)
			if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET" {
				t.Fatalf("%s %s status=%d allow=%q", method, path, response.Code, response.Header().Get("Allow"))
			}
		}
	}
}
```

测试 helper 使用现有认证 Manager 创建会话并把 `service.NewMySQL` 注入 `Dependencies.MySQLService`；JSON path helper 只检查类型，不打印响应正文。

- [ ] **Step 2: 运行测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi -run 'TestMySQL' -count=1
```

Expected: FAIL，新路由返回 404。

- [ ] **Step 3: 定义 JSON view 和 handlers**

`mysql_handlers.go` 定义：

```go
type mysqlReplicationView struct {
	State      service.MySQLReplicationState `json:"state"`
	LagSeconds *float64                      `json:"lag_seconds"`
	Level      service.Level                 `json:"level"`
}

type mysqlInstanceView struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	Address                string               `json:"address"`
	Host                   string               `json:"host"`
	Version                string               `json:"version"`
	Role                   mysql.Role           `json:"role"`
	Connections            *float64             `json:"connections"`
	MaxConnections         *float64             `json:"max_connections"`
	ConnectionUsagePercent *float64             `json:"connection_usage_percent"`
	ThreadsRunning         *float64             `json:"threads_running"`
	QPS                    *float64             `json:"qps"`
	SlowQueriesPerSecond   *float64             `json:"slow_queries_per_second"`
	BufferPoolUsagePercent *float64             `json:"buffer_pool_usage_percent"`
	UptimeSeconds          *float64             `json:"uptime_seconds"`
	Replication            mysqlReplicationView `json:"replication"`
	Status                 service.Level        `json:"status"`
}

type mysqlInstancePageView struct {
	Instances  []mysqlInstanceView `json:"instances"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}
```

总览 view 使用 `availability`、`replication_threads`、`replication_lag`、`replication_data` 四个 `alertCountView`。

- [ ] **Step 4: 注册 GET 路由和 405 fallback**

在 `api.go` 的依赖和 server 增加：

```go
MySQLService *service.MySQLService
```

在 `cmd/infraview/main.go` 的 `buildHandler` 中完成装配，不使用省略参数：

```go
providers := dataSourceProviders(cfg, clock)
hostProvider := withUpstreamTimeout(providers.Hosts, cfg.UpstreamTimeout)
mysqlProvider := withMySQLUpstreamTimeout(providers.MySQL, cfg.UpstreamTimeout)
store := cache.New(clock)
queryService := service.New(hostProvider, store, service.Options{
	InventoryTTL:       cfg.InventoryTTL,
	CurrentMetricsTTL:  cfg.CurrentMetricsTTL,
	RangeTTL:           cfg.RangeTTL,
	HealthTTL:          cfg.HealthTTL,
	MaxStale:           cfg.MaxStale,
	WarningPercent:     cfg.WarningPercent,
	CriticalPercent:    cfg.CriticalPercent,
	NetworkWarningBPS:  cfg.NetworkWarningBPS,
	NetworkCriticalBPS: cfg.NetworkCriticalBPS,
	Clock:              clock,
})
mysqlService := service.NewMySQL(mysqlProvider, store, service.MySQLOptions{
	CurrentMetricsTTL: cfg.CurrentMetricsTTL,
	MaxStale:          cfg.MaxStale,
	Clock:             clock,
})
```

注册：

```go
mux.Handle("GET /api/v1/mysql/overview", server.requireAuthentication(http.HandlerFunc(server.mysqlOverview)))
mux.Handle("GET /api/v1/mysql/instances", server.requireAuthentication(http.HandlerFunc(server.mysqlInstances)))
mux.HandleFunc("/api/v1/mysql/overview", server.methodNotAllowed)
mux.HandleFunc("/api/v1/mysql/instances", server.methodNotAllowed)
```

- [ ] **Step 5: 实现严格参数白名单**

实例 handler 只允许 `search`、`status`、`role`、`sort`、`order`、`page`、`page_size`，并构造 `service.MySQLQuery`。总览不允许任何查询参数。

- [ ] **Step 6: 增加非法查询和错误脱敏测试**

```go
func TestMySQLInstancesRejectsUnknownOrRepeatedQueryParameters(t *testing.T) {
	handler, cookie := newMySQLAPITestHandler(t, fixtureMySQLSnapshot())
	for _, query := range []string{
		"?unknown=value",
		"?status=normal&status=warning",
		"?role=primary",
		"?sort=raw_promql",
		"?page_size=10",
	} {
		response := serveAuthenticatedJSONRequest(t, handler, cookie, http.MethodGet, "/api/v1/mysql/instances"+query)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d", query, response.Code)
		}
	}
}

func TestMySQLInstancesMapsUnavailableToSafe503(t *testing.T) {
	handler, cookie := newMySQLAPIErrorHandler(t, mysql.ErrUnavailable)
	response := serveAuthenticatedJSONRequest(t, handler, cookie, http.MethodGet, "/api/v1/mysql/instances")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, forbidden := range []string{"mysql_up", "fixture-token", "dat", "err"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("safe error leaked %q", forbidden)
		}
	}
}
```

`writeServiceError` 将 `errors.Is(err, mysql.ErrUnavailable)` 映射为与现有数据源一致的安全 503，不把底层错误写入响应或日志。

- [ ] **Step 7: 格式化并运行 HTTP 全包测试**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/httpapi/mysql_handlers.go internal/httpapi/api.go internal/httpapi/api_test.go
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi -count=1
```

Expected: PASS。

- [ ] **Step 8: 提交 HTTP API**

```bash
git add internal/httpapi cmd/infraview/main.go cmd/infraview/main_test.go
git diff --cached --check
git commit -m "feat: 提供 MySQL 只读查询 API"
```

---

### Task 9: 实现 MySQL 实例列表页面

**Files:**
- Create: `web/src/features/mysql/MySQLPage.tsx`
- Create: `web/src/features/mysql/MySQLPage.test.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`
- Test: `web/src/features/mysql/MySQLPage.test.tsx`

**Interfaces:**
- Consumes: `GET /api/v1/mysql/instances`。
- Produces: `MySQLPage`、`MySQLInstancePageResponse` 和 `/mysql` 页面主体。

- [ ] **Step 1: 定义前端 API 类型**

在 `types.ts` 增加：

```ts
export type MySQLRole = 'writable' | 'read_only' | 'unknown'
export type MySQLReplicationState =
  | 'normal'
  | 'threads_stopped'
  | 'not_configured'
  | 'unknown'

export interface MySQLInstance {
  id: string
  name: string
  address: string
  host: string
  version: string
  role: MySQLRole
  connections: number | null
  max_connections: number | null
  connection_usage_percent: number | null
  threads_running: number | null
  qps: number | null
  slow_queries_per_second: number | null
  buffer_pool_usage_percent: number | null
  uptime_seconds: number | null
  replication: {
    state: MySQLReplicationState
    lag_seconds: number | null
    level: MetricLevel
  }
  status: MetricLevel
}

export interface MySQLInstancePageData {
  instances: MySQLInstance[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type MySQLInstancePageResponse = ApiResponse<MySQLInstancePageData>
```

- [ ] **Step 2: 添加 MSW fixture 和页面失败测试**

fixture 必须包含完全虚构的正常、警告、严重、未知实例。测试：

```tsx
it('renders the eleven compact MySQL columns', async () => {
  renderMySQLPage('/mysql')
  expect(await screen.findByRole('heading', { name: 'MySQL 实例' })).toBeVisible()
  for (const heading of [
    '实例', '所属主机', '版本 / 角色', '连接使用', '活跃线程',
    'QPS', '慢查询速率', 'Buffer Pool 使用率', '复制状态 / 延迟',
    '运行时间', '状态',
  ]) {
    expect(screen.getByRole('columnheader', { name: new RegExp(heading) })).toBeVisible()
  }
})

it('writes filters sort and pagination to the URL', async () => {
  const user = userEvent.setup()
  renderMySQLPage('/mysql')
  await screen.findByText('fixture-mysql-a')
  await user.selectOptions(screen.getByRole('combobox', { name: '实例状态' }), 'warning')
  await user.selectOptions(screen.getByRole('combobox', { name: '读写属性' }), 'read_only')
  await user.selectOptions(screen.getByRole('combobox', { name: '每页数量' }), '50')
  await user.click(screen.getByRole('button', { name: /^QPS排序/ }))
  expect(window.location.search).toContain('status=warning')
  expect(window.location.search).toContain('role=read_only')
  expect(window.location.search).toContain('page_size=50')
  expect(window.location.search).toContain('sort=qps')
  expect(window.location.search).toContain('page=1')
})

it('renders missing metrics as 暂无数据', async () => {
  const fixture = mysqlInstancePageFixture()
  fixture.data.instances[0] = {
    ...fixture.data.instances[0],
    connections: null,
    max_connections: null,
    connection_usage_percent: null,
    threads_running: null,
    qps: null,
    slow_queries_per_second: null,
    buffer_pool_usage_percent: null,
    uptime_seconds: null,
  }
  vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(fixture))
  renderMySQLPage()
  const row = (await screen.findByText('fixture-mysql-a')).closest('tr')
  expect(row).not.toBeNull()
  expect(within(row!).getAllByText('暂无数据')).toHaveLength(6)
})

it('renders every replication state', async () => {
  renderMySQLPage()
  await screen.findByText('fixture-mysql-a')
  for (const label of ['正常', '线程异常', '未配置复制', '状态未知']) {
    expect(screen.getAllByText(label).length).toBeGreaterThan(0)
  }
})

it('keeps stale data visible and reports background errors', async () => {
  const stale = mysqlInstancePageFixture({ meta: { stale: true } })
  vi.mocked(globalThis.fetch)
    .mockResolvedValueOnce(jsonResponse(stale))
    .mockResolvedValueOnce(jsonResponse({
      code: 'datasource_unavailable',
      message: '数据源暂时不可用，请稍后重试',
      request_id: 'req-fixture-mysql-refresh-error',
      retryable: true,
    }, 503))
  renderMySQLPage()
  expect(await screen.findByText('fixture-mysql-a')).toBeVisible()
  expect(screen.getByText('数据已过期')).toBeVisible()
  await userEvent.setup().click(screen.getByRole('button', { name: '刷新 MySQL 实例列表' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('刷新失败')
  expect(screen.getByText('fixture-mysql-a')).toBeVisible()
})
```

- [ ] **Step 3: 运行 Vitest 确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx'
```

Expected: FAIL，`MySQLPage` 尚不存在。

- [ ] **Step 4: 实现 URL 规范化和查询**

复用主机页的 300ms 搜索防抖、20/50/100 页大小、响应页码规范化和运行时刷新周期。MySQL sort 白名单：

```ts
const sortFields = [
  'instance',
  'connections',
  'threads_running',
  'qps',
  'slow_queries',
  'buffer_pool',
  'replication_lag',
  'uptime',
  'status',
] as const
```

请求只调用 `/api/v1/mysql/instances`，参数来自规范化 URL。

- [ ] **Step 5: 实现 11 列表格和格式化**

具体格式：

- 实例：`name · address` 单行省略，`title` 保留完整组合。
- 版本/角色：`version · 可写|只读|未知`。
- 连接：`connections / max_connections (usage%)`；任一必要值缺失时显示已有值或“暂无数据”，不得补零。
- QPS、慢查询：每秒值，最多两位小数。
- Buffer Pool：一位小数百分比。
- 复制：`正常 · Ns`、`线程异常`、`未配置复制`、`状态未知`。
- 运行时间：天/小时；缺失显示“暂无数据”。
- 状态：正常、警告、严重、未知，颜色和文字同时表达。

- [ ] **Step 6: 运行页面测试和 typecheck**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx && npm run typecheck'
```

Expected: PASS。

- [ ] **Step 7: 提交 MySQL 页面**

```bash
git add web/src/features/mysql web/src/api/types.ts web/src/test/fixtures.ts web/src/test/server.ts
git diff --cached --check
git commit -m "feat: 增加 MySQL 实例列表"
```

---

### Task 10: 集成总览卡、导航、路由和样式

**Files:**
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/AppShell.tsx`
- Modify: `web/src/app/AppShell.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/app/theme.css`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/test/server.ts`
- Test: `web/src/app/AppShell.test.tsx`
- Test: `web/src/features/overview/OverviewPage.test.tsx`

**Interfaces:**
- Consumes: `MySQLPage` 和 `GET /api/v1/mysql/overview`。
- Produces: `/mysql` 路由、侧边栏“MySQL”入口和总览 MySQL 健康卡。

- [ ] **Step 1: 定义 MySQL 总览类型和 fixture**

```ts
export interface MySQLOverviewData {
  total: number
  normal: number
  warning: number
  critical: number
  unknown: number
  affected_instances: number
  warning_instances: number
  critical_instances: number
  alerts: {
    availability: AlertCount
    replication_threads: AlertCount
    replication_lag: AlertCount
    replication_data: AlertCount
  }
}

export type MySQLOverviewResponse = ApiResponse<MySQLOverviewData>
```

- [ ] **Step 2: 写总览和导航失败测试**

```tsx
it('renders a MySQL health card linking to /mysql', async () => {
  renderOverview()
  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(card).toHaveAttribute('href', '/mysql')
  expect(within(card).getByText('复制延迟')).toBeVisible()
  expect(within(card).getByText('复制数据缺失')).toBeVisible()
})

it('shows zero alert categories with normal wording', async () => {
  const fixture = mysqlOverviewFixture({
    data: {
      total: 2,
      normal: 2,
      warning: 0,
      critical: 0,
      unknown: 0,
      affected_instances: 0,
      warning_instances: 0,
      critical_instances: 0,
      alerts: {
        availability: { warning: 0, critical: 0 },
        replication_threads: { warning: 0, critical: 0 },
        replication_lag: { warning: 0, critical: 0 },
        replication_data: { warning: 0, critical: 0 },
      },
    },
  })
  mockOverviewRequests({ mysql: fixture })
  renderOverview()
  const card = await screen.findByRole('link', { name: '查看 MySQL 板块' })
  expect(card).toHaveAttribute('data-level', 'normal')
  expect(within(card).getAllByText('无异常')).toHaveLength(4)
  expect(within(card).getByText('无严重')).toBeVisible()
  expect(within(card).getByText('无警告')).toBeVisible()
})

it('adds MySQL to navigation without changing datasource count', async () => {
  renderShell()
  expect(screen.getByRole('link', { name: 'MySQL' })).toHaveAttribute('href', '/mysql')
  const connection = screen.getByLabelText('数据连接汇总')
  expect(await within(connection).findByText('1/1 正常')).toBeVisible()
  expect(within(connection).queryByText('MySQL')).not.toBeInTheDocument()
})

it('keeps Linux and MySQL card failures independent', async () => {
  mockOverviewRequests({
    host: overviewFixture(),
    mysqlError: {
      code: 'datasource_unavailable',
      message: '数据源暂时不可用，请稍后重试',
      request_id: 'req-fixture-mysql-overview-error',
      retryable: true,
    },
  })
  renderOverview()
  expect(await screen.findByRole('link', { name: '查看 Linux 主机板块' })).toBeVisible()
  expect(screen.getByRole('alert', { name: 'MySQL 板块加载失败' })).toBeVisible()
})
```

- [ ] **Step 3: 运行前端测试确认 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/overview/OverviewPage.test.tsx src/app/AppShell.test.tsx'
```

Expected: FAIL，MySQL 卡、路由和导航尚不存在。

- [ ] **Step 4: 注册路由和导航**

`App.tsx`：

```tsx
<Route path="mysql" element={<MySQLPage />} />
```

`AppShell.tsx`：

```tsx
<NavLink to="/mysql">MySQL</NavLink>
```

数据连接汇总仍只有指标来源 `Nightingale` 或 `Mock`，不增加第二个连接计数。

- [ ] **Step 5: 实现独立 MySQL overview query 和卡片**

`OverviewPage` 同时使用：

```ts
useQuery({
  queryKey: ['mysql-overview'],
  queryFn: ({ signal }) =>
    apiRequest<MySQLOverviewResponse>('/api/v1/mysql/overview', { signal }),
  refetchInterval: refreshIntervalMs,
  refetchIntervalInBackground: false,
})
```

MySQL 卡使用现有 `module-status-card` 和四个 `MetricAlert` 等价结构；整体等级为严重优先，其次警告或未知，最后正常。Linux 主机卡查询失败不能被 MySQL 卡吞掉，MySQL 查询失败显示该板块自己的错误卡或重试状态。

统一刷新控件以 `hostOverview.isFetching || mysqlOverview.isFetching` 表示刷新中，手动刷新同时调用两个 query 的 `refetch()`；“上次刷新”只在两个板块都至少成功一次后显示二者较早的 `dataUpdatedAt`，避免把部分刷新误报为整体成功。

- [ ] **Step 6: 增加紧凑列表和双卡布局样式**

在 `theme.css` 增加 `mysql-*` 类，要求：

- 11 列单行、等宽数值、左对齐。
- 实例组合文本溢出省略。
- 只有 status 和 replication 使用告警色。
- 1440x900 无横向页面溢出；表格容器必要时内部滚动。
- 不添加操作列或按钮。

- [ ] **Step 7: 运行完整前端验证**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run && npm run typecheck && npm run build'
```

Expected: Vitest、typecheck、production build 全部 PASS。

- [ ] **Step 8: 提交总览和导航**

```bash
git add web/src
git diff --cached --check
git commit -m "feat: 集成 MySQL 总览与导航"
```

---

### Task 11: 扩展 smoke 和 Chromium E2E

**Files:**
- Modify: `scripts/smoke.sh`
- Modify: `web/e2e/infraview.spec.ts`
- Modify: `docs/TESTING.md`
- Test: `scripts/e2e.sh`

**Interfaces:**
- Consumes: Mock MySQL API、总览卡和 `/mysql` 页面。
- Produces: 容器级 API smoke 和真实 Chromium MySQL 关键路径。

- [ ] **Step 1: 先写 MySQL API smoke 断言**

在 `scripts/smoke.sh` 登录后的受保护请求中增加：

```sh
mysql_overview_body=$(request_json '/api/v1/mysql/overview')
printf '%s' "$mysql_overview_body" |
	jq -e '(.data.total|type=="number") and
		(.data.alerts.availability|type=="object") and
		.meta.stale==false' >/dev/null

mysql_instances_body=$(request_json '/api/v1/mysql/instances?page=1&page_size=20')
printf '%s' "$mysql_instances_body" |
	jq -e '(.data.instances|type=="array") and
		(.data.page_size==20) and
		.meta.stale==false' >/dev/null
```

脚本不能 echo 响应正文、数量或值。

- [ ] **Step 2: 写 Chromium MySQL 用例**

在 `infraview.spec.ts` 增加独立测试：

```ts
test('shows the read-only MySQL overview and compact instance list', async ({ page }) => {
  await login(page)
  const mysqlCard = page.getByRole('link', { name: '查看 MySQL 板块' })
  await expect(mysqlCard).toBeVisible()
  await mysqlCard.click()
  await expect(page.getByRole('heading', { name: 'MySQL 实例' })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: /复制状态/ })).toBeVisible()
  await expect(page.getByText('线程异常').first()).toBeVisible()
  await expect(page.getByRole('button', { name: /重启|执行|切换|配置/ })).toHaveCount(0)
  await expect(page.locator('body')).not.toHaveCSS('overflow-x', 'scroll')
})
```

再增加 URL 恢复测试：设置角色筛选、状态筛选、排序和每页 50，刷新后断言控件与 URL 保持。

- [ ] **Step 3: 构建隔离 Mock E2E 并确认 GREEN**

```bash
INFRAVIEW_E2E_PROJECT=infraview-mysql-0728 \
INFRAVIEW_E2E_PORT=18085 \
./scripts/e2e.sh
```

Expected: Compose smoke 和全部 Chromium 用例 PASS；脚本只清理 `infraview-mysql-0728` 自己创建的资源。

- [ ] **Step 4: 更新测试文档**

在 `docs/TESTING.md` 记录 MySQL 领域、Nightingale 契约、HTTP、Vitest、Mock smoke 和 Chromium 覆盖；不得记录真实实例或指标结果。

- [ ] **Step 5: 提交 E2E**

```bash
git add scripts/smoke.sh web/e2e/infraview.spec.ts docs/TESTING.md
git diff --cached --check
git commit -m "test: 验收 MySQL 只读模块"
```

---

### Task 12: 同步文档、完整验证和真实只读验收

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/superpowers/plans/2026-07-28-mysql-module.md`

**Interfaces:**
- Consumes: 完成的 MySQL 模块和全部自动化证据。
- Produces: 可恢复交接、最终容器验证和脱敏 v8.4.1 真实只读验收结论。

- [x] **Step 1: 更新架构、设计、安全和状态文档**

统一记录：

- MySQL 是独立领域和 Service，共享 Nightingale 安全客户端。
- 两个新 GET API、总览卡、11 列实例页和 Mock。
- 固定查询、一次 batch、无 N+1、无历史/详情/写能力。
- 精确复制阈值、缺失语义和未知计数。
- 当前完成项、未完成项和恢复提示词。

不得把本计划中的“预期通过”提前写成“已经通过”。

- [x] **Step 2: 执行敏感模式和格式自审**

```bash
git diff --check
if git diff -- . |
  awk '/^\+[^+]/ {sub(/^\+/, ""); print}' |
  rg -q 'X-User-Token:[[:space:]]+[^<`]|Cookie:[[:space:]]+[^<`]|INFRAVIEW_NIGHTINGALE_TOKEN=.+|192\.168\.|[0-9a-f]{64}'; then
  echo '新增内容敏感模式检查失败' >&2
  exit 1
fi
```

Expected: 无输出且退出码 0。RFC 5737 测试地址若被规则命中，必须限定扫描排除 `internal/**/testdata` 和测试文件，而不是删除脱敏夹具。

- [x] **Step 3: 运行完整生产镜像验证**

```bash
docker build --tag infraview:mysql-module-verify .
```

Expected: 前端 Vitest、typecheck、production build、Go 普通测试、race 测试和 Go build 全部 PASS。

- [x] **Step 4: 运行全仓聚焦的独立 race 复核**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test -race ./... -count=1
```

Expected: PASS。

- [ ] **Step 5: 经用户单独授权后启动一次性真实验证容器**

此步骤不得自动执行。获得授权后：

```bash
set -euo pipefail
private_env_file=${INFRAVIEW_ENV_FILE:?必须显式提供私密环境文件}
test "$(stat -c '%a' "$private_env_file")" = 600
common_dir=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)
main_checkout=$(dirname "$common_dir")
git -C "$main_checkout" check-ignore -q "$private_env_file"
docker run --detach --rm \
  --name infraview-mysql-real-verify \
  --env-file "$private_env_file" \
  --publish 127.0.0.1:18086:8080 \
  --read-only \
  --cap-drop ALL \
  infraview:mysql-module-verify >/dev/null
```

只创建明确命名的一次性容器，不触碰现有 Compose 服务。

- [ ] **Step 6: 执行不输出正文的真实只读 API smoke**

```bash
set -euo pipefail
private_env_file=${INFRAVIEW_ENV_FILE:?必须显式提供私密环境文件}
username=$(sed -n 's/^INFRAVIEW_USERNAME=//p' "$private_env_file" | tail -n 1)
password=$(sed -n 's/^INFRAVIEW_PASSWORD=//p' "$private_env_file" | tail -n 1)
base_url=http://127.0.0.1:18086

ready=false
for attempt in $(seq 1 30); do
  if curl --silent --fail --max-time 2 "$base_url/healthz" |
    jq -e '.status=="ok"' >/dev/null; then
    ready=true
    break
  fi
  sleep 1
done
test "$ready" = true

login_payload=$(jq -cn \
  --arg username "$username" \
  --arg password "$password" \
  '{username:$username,password:$password}')
login_headers=$(printf '%s' "$login_payload" |
  curl --silent --max-time 10 \
    --dump-header - --output /dev/null \
    --header 'Content-Type: application/json' \
    --data-binary @- \
    "$base_url/api/v1/session")
login_status=$(printf '%s' "$login_headers" |
  awk 'toupper($1) ~ /^HTTP\// {code=$2} END {print code}')
cookie=$(printf '%s' "$login_headers" |
  sed -n 's/^[Ss]et-[Cc]ookie: \([^;]*\).*/\1/p' |
  tr -d '\r' |
  tail -n 1)
test "$login_status" = 204
test -n "$cookie"

mysql_overview_body=$(curl --silent --fail --max-time 15 \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/mysql/overview")
printf '%s' "$mysql_overview_body" |
  jq -e '(.data.total|type=="number") and
    (.data.alerts.availability|type=="object") and
    (.data.alerts.replication_threads|type=="object") and
    (.data.alerts.replication_lag|type=="object") and
    (.data.alerts.replication_data|type=="object") and
    .meta.stale==false' >/dev/null

mysql_instances_body=$(curl --silent --fail --max-time 15 \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/mysql/instances?page=1&page_size=20")
printf '%s' "$mysql_instances_body" |
  jq -e '(.data.instances|type=="array") and
    (.data.instances|length)>0 and
    (.data.instances[0].id|type=="string") and
    (.data.instances[0].replication|type=="object") and
    .meta.stale==false' >/dev/null

echo '真实 MySQL 只读 API 验收：通过'
```

脚本只输出固定通过文案；不得增加 `set -x`、响应 echo、Cookie echo、实例数量或指标值输出。

- [ ] **Step 7: 只读浏览器验收并清理一次性容器**

浏览器只确认：

- 总览 MySQL 卡可见并可进入 `/mysql`。
- 11 列表头、搜索、状态/角色筛选和刷新控件存在。
- 页面没有破坏性控件和横向页面溢出。
- 控制台没有应用错误。

验收后：

```bash
docker stop infraview-mysql-real-verify
```

该容器使用 `--rm`，停止后自动删除；不得删除或重启其他容器。

- [x] **Step 8: 记录实际验证结果并勾选计划**

只在相应命令真实通过后更新 `PROJECT_STATUS`、`HANDOFF`、`TODO`、`NIGHTINGALE` 和本计划复选框。记录结构性 PASS/FAIL，不记录真实数据。

- [x] **Step 9: 提交最终文档**

```bash
git add README.md docs
git diff --cached --check
git commit -m "docs: 记录 MySQL 模块实现与验证"
```

- [x] **Step 10: 最终分支核验**

```bash
git status --short
git log --oneline main..HEAD
git diff --check main...HEAD
```

Expected: 工作区干净；分支只包含 MySQL 设计、实现、测试和同步文档。未经用户明确授权，不 push、不合并、不重建现有服务。
