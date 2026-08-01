# InfraView Host Disk SMART Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> 2026-08-01 替代说明：本文的固定 17 组、inventory 容量标签和九列表格是初始交付计划及历史执行证据。当前实现以 `docs/superpowers/specs/2026-08-01-disk-capacity-metric-and-column-design.md` 与对应实施计划为准：18 组单 batch，容量来自独立第 17 组指标，inventory 移至第 18 组，页面为型号/容量分列的十列表格。

**Goal:** 新增严格只读的主机硬盘 SMART 板块，通过测试 Nightingale 的一次固定 17 查询 batch 提供总览卡和紧凑硬盘列表，并按独立 60 秒样本推进周期判断采集新鲜度。

**Architecture:** 新建 `internal/disk` 领域与 `DiskService`，由共享 Nightingale Provider 实现硬盘只读快照；Service 负责独立缓存、新鲜度、状态聚合、查询和分页；HTTP 暴露两条固定 GET API；React 新增 `/disks` 页面和独立总览卡。Linux、MySQL 与硬盘三个板块共享基础设施但不共享领域快照或状态。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Playwright、Docker、Nightingale v8.4.1（主要验证版本，v9.x 仅保留协议兼容）。

## 2026-07-30 实际执行状态

| 范围 | 状态 | 证据/说明 |
| --- | --- | --- |
| Task 1–7 实现与 RED→GREEN | 已由前序工作者执行 | `.superpowers/sdd/2026-07-30-host-disk-smart-module/task-1-report.md` 至 `task-7-report.md`；原逐步复选框保留为计划清单，不据此伪造本任务执行记录 |
| `status_source` 用户裁定 | 已实现并纳入 Task 8 文档 | 固定六值 `smart_health|device_warning|attribute_failure|collection|normal|unknown`；同级时设备来源优先，设备内部依次为 SMART 健康、设备警告、属性失败，只有采集等级严格更高时使用 `collection` |
| Task 8 文档同步 | 已执行 | 基于现有 `HANDOFF/PROJECT_STATUS/TODO` 用户差异增量更新，未覆盖或还原 |
| Task 8、终审与现场兼容修复后的 Docker 验证 | 已执行、退出 0 | 前端 8 文件/99 测试、typecheck/build、Go 普通/race 全仓、无缓存镜像构建；持久结果见本计划与 `docs/TESTING.md` |
| E2E / Playwright | 现有 8080 一次性 Chromium 已执行、退出 0 | 不运行会创建 18080 的 `scripts/e2e.sh`；浏览器容器不发布端口、不截图、不保存 trace |
| 现有 8080 重建/重启、测试 Nightingale 现场验收 | 已授权并执行 | 只原位重建 `infraview` 项目现有 8080；API、容器安全、Chromium 和跨两个 60 秒周期推进验收通过 |
| 生产 Nightingale 验证 | 永久禁止、未执行 | 不连接、切换或探测生产 |
| `git add` / commit / push | 未执行 | 提交和推送分别需要明确授权 |

整功能终审发现默认 `sort=host` 同主机内未按设备自然排序（Important），以及 Service 极端页码溢出和磁盘总览单类 alert 合计约束（Minor）；三项均完成 RED→GREEN、范围复审结论为 Approved。

首次测试 Nightingale v8.4.1 现场验收发现第 17 组对同一 `ident + device` 返回兼容但原始最后样本时间不同的 24 小时历史身份序列，旧实现将重复主键直接判为 503。修复按主键安全归并、补齐兼容可选身份、取最新原始时间并保持非空元数据；输入顺序、跨设备 serial/WWN 冲突和同时间元数据冲突均有测试，范围复审 Approved。

## Global Constraints

- 完整设计依据：`docs/superpowers/specs/2026-07-30-host-disk-smart-module-design.md`。
- InfraView 始终只读；不得执行 `smartctl`、`nvme-cli`、SSH、SMART 自检、扫描、修复、启停、擦除或其他运维操作。
- 运行时只允许调用现有受限 Nightingale 只读客户端；不得新增任意 PromQL、任意 URL、代理、数据库连接或远程执行能力。
- 开发 8080 永远只连接测试 Nightingale；不得连接、切换或探测生产 Nightingale。
- 不读取、输出或提交私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/指标值或上游正文。
- 不在 API、UI、日志、错误或夹具中暴露 `serial_no`、`wwn`、原始标签或 PromQL。
- 硬盘快照必须恰好使用一次 `query-instant-batch` 和 17 条固定查询，不得按主机或设备 N+1。
- `INFRAVIEW_SMART_COLLECTION_INTERVAL` 默认 `60s`，同时作为硬盘快照 TTL 和 freshness 周期；2 个周期警告，5 个周期严重。
- `status_source` 固定为 `smart_health|device_warning|attribute_failure|collection|normal|unknown`；最终等级同级时设备来源优先于采集来源，设备来源内部稳定优先级为 `smart_health`、`device_warning`、`attribute_failure`，只有采集等级严格更高时才使用 `collection`。
- 当前工作树已有 `docs/HANDOFF.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 未提交修改，实施时必须保留并基于现有差异增量更新。
- 本计划中的 shell 命令均为未来实施命令，创建计划时尚未执行。
- 每个任务末尾的提交步骤仅在用户另行明确授权后执行；未授权时只保留工作树修改并继续下一检查点。
- 未经单独授权，不重建或重启现有 8080，不部署，不推送，也不运行会映射额外宿主机端口的 `scripts/e2e.sh`。

---

### Task 1: 硬盘领域契约与确定性 Mock

**Files:**
- Create: `internal/disk/types.go`
- Create: `internal/disk/provider.go`
- Create: `internal/disk/contract_test.go`
- Create: `internal/disk/disktest/contract.go`
- Create: `internal/adapters/mock/disk_provider.go`
- Create: `internal/adapters/mock/disk_provider_test.go`

**Interfaces:**
- Produces: `disk.Provider.SMARTSnapshot(context.Context) (disk.Snapshot, error)`
- Produces: `disk.StableDeviceID(hostID, wwn, serialNo, device string) string`
- Produces: `mock.NewDisk(clock func() time.Time) disk.Provider`
- Security invariant: 对外领域类型不包含序列号、WWN 或原始标签

- [ ] **Step 1: 写稳定 ID 与 Provider 契约失败测试**

覆盖：

- 相同输入生成相同不可逆 ID。
- 主机 ID 必须参与 ID。
- 身份优先级固定为 WWN、`serial_no`、`device`。
- 身份类型参与哈希，避免不同来源的相同字符串碰撞。
- 空主机或没有任何设备身份时返回空 ID。
- Provider 返回非空、唯一、稳定的设备 ID。
- 快照至少包含 ATA/NVMe、正常/警告/严重/未知、可空字段、零错误和非零错误。
- 使用反射断言 `disk.Device` 不存在 `SerialNo`、`WWN`、`Labels` 字段。

核心测试形态：

```go
func TestStableDeviceIDUsesPriorityAndHost(t *testing.T) {
	id := disk.StableDeviceID("fixture-host-a", "fixture-wwn", "fixture-serial", "/dev/sda")
	if id == "" || id != disk.StableDeviceID("fixture-host-a", "fixture-wwn", "changed", "/dev/sdb") {
		t.Fatal("WWN priority is not stable")
	}
	if id == disk.StableDeviceID("fixture-host-b", "fixture-wwn", "fixture-serial", "/dev/sda") {
		t.Fatal("host identity is missing")
	}
	for _, raw := range []string{"fixture-host", "fixture-wwn", "fixture-serial", "/dev/sda"} {
		if strings.Contains(id, raw) {
			t.Fatalf("stable ID exposes raw identity")
		}
	}
}
```

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/disk ./internal/adapters/mock -count=1
```

预期：因 `internal/disk` 和 Mock 硬盘 Provider 尚不存在而失败。

- [ ] **Step 3: 实现最小领域类型**

`internal/disk/provider.go`：

```go
package disk

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("disk data source: unavailable")

type Provider interface {
	SMARTSnapshot(context.Context) (Snapshot, error)
}
```

`internal/disk/types.go` 的固定外形：

```go
type Health string
type AttributeFailure string

const (
	HealthHealthy Health = "healthy"
	HealthFailed  Health = "failed"
	HealthUnknown Health = "unknown"

	AttributeFailureNone AttributeFailure = "none"
	AttributeFailurePast AttributeFailure = "past"
	AttributeFailureNow  AttributeFailure = "now"
)

type ErrorCounters struct {
	PendingSectors         *float64
	ReallocatedSectors     *float64
	UncorrectableSectors   *float64
	UDMACRCErrors          *float64
	MediaIntegrityErrors   *float64
	ErrorLogEntries        *float64
	UnsafeShutdowns        *float64
}

type Device struct {
	ID                              string
	HostID                          string
	Device                          string
	Model                           string
	CapacityBytes                   *int64
	SMARTHealth                     Health
	TemperatureCelsius              *float64
	LifetimeUsedPercent             *float64
	PowerOnHours                    *float64
	CriticalWarning                 *int64
	AvailableSparePercent           *float64
	AvailableSpareThresholdPercent  *float64
	AttributeFailure                AttributeFailure
	Errors                          ErrorCounters
	CollectionTracked               bool
	ReportedAt                      time.Time
}

type Snapshot struct {
	Devices []Device
}
```

`StableDeviceID` 使用：

```go
identityKind, identityValue := firstNonEmptyIdentity(wwn, serialNo, device)
sum := sha256.Sum256([]byte(hostID + "\x00" + identityKind + "\x00" + identityValue))
return base64.RawURLEncoding.EncodeToString(sum[:])
```

当 `hostID` 或所有设备身份为空时返回空字符串。不得把原始身份保存在 `Device`。

- [ ] **Step 4: 实现确定性脱敏 Mock**

Mock 使用固定虚构名称，不使用任何现场标识；每次调用按注入时钟设置非零 `ReportedAt`。至少返回：

- ATA 正常盘。
- ATA `In_the_past` 警告盘。
- ATA `FAILING_NOW` 严重盘。
- NVMe 正常盘。
- NVMe `critical_warning != 0` 严重盘。
- SMART 未知和字段部分缺失盘。

Mock 错误计数必须同时覆盖“全部为零”“部分缺失且已知为零”“存在多个非零项”“全部缺失”。

- [ ] **Step 5: 运行 GREEN 与格式化**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/disk internal/adapters/mock/disk_provider.go internal/adapters/mock/disk_provider_test.go
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/disk ./internal/adapters/mock -count=1
```

- [ ] **Step 6: 可选授权提交**

仅在用户明确授权提交后执行：

```bash
git add internal/disk internal/adapters/mock/disk_provider.go internal/adapters/mock/disk_provider_test.go
git commit -m "feat: add disk domain and mock provider"
```

---

### Task 2: Nightingale 固定 17 查询硬盘 Provider

**Files:**
- Create: `internal/adapters/nightingale/disk_promql.go`
- Create: `internal/adapters/nightingale/disk_provider.go`
- Create: `internal/adapters/nightingale/disk_provider_test.go`
- Create: `internal/adapters/nightingale/testdata/disk-instant-batch.json`
- Modify: `internal/adapters/nightingale/provider.go`

**Interfaces:**
- Implements: `disk.Provider` on the existing shared `*nightingale.Provider`
- Consumes: existing `queryInstant` safety checks, discovery cache and client
- Produces: one complete `disk.Snapshot` from exactly one fixed batch

- [ ] **Step 1: 写查询与调用次数失败测试**

直接断言查询切片长度、顺序和文本完全等于：

```go
var wantDiskPromQL = []string{
	`smart_device_health_ok`,
	`smart_device_temp_c`,
	`smart_attribute_temperature_celsius`,
	`smart_attribute_percentage_used`,
	`smart_attribute_power_on_hours`,
	`smart_attribute_critical_warning`,
	`smart_attribute_available_spare`,
	`smart_attribute_available_spare_threshold`,
	`smart_attribute_value{fail=~"FAILING_NOW|In_the_past"}`,
	`smart_device_pending_sector_count`,
	`smart_device_reallocated_sectors_count`,
	`smart_device_uncorrectable_sector_count`,
	`smart_device_udma_crc_errors`,
	`smart_attribute_media_and_data_integrity_errors`,
	`smart_attribute_error_information_log_entries`,
	`smart_attribute_unsafe_shutdowns`,
	`tlast_over_time(smart_device_health_ok[24h])`,
}
```

测试 HTTP Server 只接受一次数据源发现和一次 `/api/n9e/query-instant-batch`；检查 batch 内恰好 17 条查询，拒绝第二个 batch 或按设备请求。

- [ ] **Step 2: 写脱敏 fixture 与归并失败测试**

`disk-instant-batch.json` 只使用文档保留地址段和虚构标识，覆盖：

- 第 17 组建立 ATA/NVMe 设备全集和原始 `ReportedAt`。
- 第 17 组同一 `ident + device` 的兼容历史序列按最大原始时间合并，输入顺序不改变稳定身份或非空元数据。
- 第 1 组只标记当前报告及 SMART 自检。
- WWN、序列号、设备名优先级与相同主机下多盘。
- 辅助序列先按 `ident + device` 定位；`serial_no` 或 WWN 仅双方非空且值不同时冲突，任一侧缺失仍允许归并。
- 两个温度指标按最新有效样本归并。
- 同一最新时间冲突后字段为 `nil`。
- 未匹配辅助序列被忽略。
- `health_ok` 非 0/1、负数、NaN、Inf、越界百分比、非整数警告和非法容量只使字段缺失。
- `FAILING_NOW` 高于 `In_the_past`。
- 输出不含 WWN、序列号或原始标签。

- [ ] **Step 3: 写结构与安全失败测试**

表驱动覆盖：

- batch 外层不是 17 组。
- 第 1 或第 17 组为 `null`。
- 第 17 组身份缺失、可选身份冲突、跨设备稳定身份冲突或同时间元数据冲突。
- 第 17 组 `tlast` 不是有效 Unix 时间。
- 401/403、重定向、非 JSON、错误 Content-Type、envelope `err`、超时和超过 8 MiB。
- 上述错误都满足 `errors.Is(err, disk.ErrUnavailable)`，且错误文本不包含测试上游秘密。
- 辅助组 `null` 或单字段非法只产生缺失字段，不使完整快照失败。

- [ ] **Step 4: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -run 'Disk|SMART' -count=1
```

预期：因硬盘查询和 Provider 尚不存在而失败。

- [ ] **Step 5: 实现固定查询和主身份状态**

`disk_promql.go` 只返回新切片副本，调用方不能修改全局查询顺序：

```go
func diskPromQL() []string {
	return append([]string(nil), fixedDiskPromQL...)
}
```

`SMARTSnapshot` 必须：

1. 调用 `ready()`。
2. 仅调用一次 `queryInstant(ctx, diskPromQL())`。
3. 验证外层恰好 17 组且第 1、17 组非 `nil`。
4. 先按第 17 组 `ident + device` 归并兼容历史序列，再以合并后的 `serial_no + optional wwn` 建立内部状态和稳定 ID。
5. 从第 17 组样本值选择最大的原始最后时间，而不是使用查询响应外层时间。
6. 再归并第 1 组当前健康和第 2 至 16 组辅助字段。

内部状态可保存 WWN/序列号用于安全归并，但最终只生成不含这些字段的 `disk.Device`。

- [ ] **Step 6: 实现最新值与冲突规则**

使用字段状态：

```go
type diskScalarState struct {
	value     *float64
	timestamp time.Time
	conflict  bool
}
```

合并规则：

- 候选时间更旧：忽略。
- 候选时间更新：替换并清除旧冲突。
- 最新时间相同且值相同：保留。
- 最新时间相同但值不同：设为 `nil` 并标记冲突；该时间的后续值不能重新选中。
- 各字段先执行自己的有限值、非负、整数或百分比验证。

容量只从主身份标签解析为非负 `int64`。SMART 健康只接受 0/1。`critical_warning` 转成非负 `int64`。

- [ ] **Step 7: 实现安全错误与接口断言**

```go
func diskUnavailableError() error {
	return fmt.Errorf("%w: Nightingale SMART 当前指标不可用", disk.ErrUnavailable)
}

var _ disk.Provider = (*Provider)(nil)
```

所有上游和身份结构错误统一返回固定错误，不拼接响应、URL、标签或真实值。

- [ ] **Step 8: 运行 GREEN、全适配器回归和格式化**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/adapters/nightingale/disk_promql.go internal/adapters/nightingale/disk_provider.go internal/adapters/nightingale/disk_provider_test.go internal/adapters/nightingale/provider.go
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -count=1
```

- [ ] **Step 9: 可选授权提交**

```bash
git add internal/adapters/nightingale internal/disk
git commit -m "feat: load disk smart snapshot from nightingale"
```

仅在用户明确授权提交后执行。

---

### Task 3: DiskService 缓存、新鲜度、状态与列表查询

**Files:**
- Create: `internal/service/disk_types.go`
- Create: `internal/service/disk_service.go`
- Create: `internal/service/disk_service_test.go`
- Reuse without behavior change: `internal/service/freshness.go`
- Reuse: `internal/service/types.go`

**Interfaces:**
- Produces: `service.NewDisk(provider, store, options) *DiskService`
- Produces: `DiskService.Overview(ctx) (DiskOverview, Meta, error)`
- Produces: `DiskService.Devices(ctx, query) (DiskPage, Meta, error)`
- Uses independent cache key: `service:disk:snapshot`

- [ ] **Step 1: 写状态矩阵失败测试**

逐个设备验证：

- `health_ok=0` 为严重，`health_ok=1` 为正常，缺失为未知。
- `critical_warning != 0` 为严重。
- `available_spare <= threshold` 为严重；任一缺失不推断。
- `FAILING_NOW` 为严重，`In_the_past` 为警告。
- 温度、寿命、通电时间和错误计数不改变最终状态。
- 最终等级固定为 `critical > warning > unknown > normal`。
- 覆盖纯采集异常、device_warning critical 同级、attribute_failure warning
  同级和设备来源内部稳定优先级。
- `status_source` 精确为
  `smart_health|device_warning|attribute_failure|collection|unknown|normal`；
  设备来源同级优先，只有 collection 严格高于全部设备来源时来源为 collection。

- [ ] **Step 2: 写 60 秒样本推进失败测试**

使用可控时钟和每次强制过期的缓存测试：

- 第一次观察任意非零原始时间立即正常，不比较绝对年龄。
- 同一原始时间在 119.999 秒正常、120 秒警告、299.999 秒警告、300 秒严重。
- 样本时间向前或回退都重建正常基线。
- 缓存命中不调用 Provider、也不调用 `Observe`。
- loader 失败返回 stale 时，本地推进时间继续老化。
- loader 失败且无缓存时返回 Provider 错误。
- 并发 `Overview/Devices` 不产生竞态。

- [ ] **Step 3: 写总览、搜索、排序和分页失败测试**

覆盖：

- 总览 `total/normal/warning/critical/unknown`。
- `affected_devices` 对 warning/critical/unknown 去重。
- `warning_devices` 包含 warning 和 unknown。
- `critical_devices` 只含 critical。
- 四类 alerts 均按设备去重；未知不增加具体告警分类。
- 搜索仅匹配 host、device、model。
- 状态过滤严格接受四个等级。
- 默认按 host、device 自然升序，确保 `disk2 < disk10`。
- 支持 `host|device|temperature|lifetime|power_on_hours|status`。
- 可空数值无论升降序都排在末尾。
- 仅允许 20/50/100 页大小，先过滤排序再分页。
- 返回快照及所有指针字段为深拷贝。

- [ ] **Step 4: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -run 'Disk' -count=1
```

- [ ] **Step 5: 定义 Service 对外类型**

`disk_types.go` 固定包含：

```go
type DiskOptions struct {
	SnapshotTTL        time.Duration
	CollectionInterval time.Duration
	MaxStale           time.Duration
	Clock              func() time.Time
}

type DiskQuery struct {
	Search   string
	Status   Level
	Sort     string
	Order    string
	Page     int
	PageSize int
}

type DiskOverviewAlerts struct {
	SMARTHealth      AlertCount
	DeviceWarning    AlertCount
	AttributeFailure AlertCount
	Collection       AlertCount
}

type DiskOverview struct {
	Total, Normal, Warning, Critical, Unknown int
	AffectedDevices, WarningDevices, CriticalDevices int
	Alerts DiskOverviewAlerts
}
```

同时固定：

```go
type DiskStatusSource string

const (
	DiskStatusSourceSMARTHealth      DiskStatusSource = "smart_health"
	DiskStatusSourceDeviceWarning    DiskStatusSource = "device_warning"
	DiskStatusSourceAttributeFailure DiskStatusSource = "attribute_failure"
	DiskStatusSourceCollection       DiskStatusSource = "collection"
	DiskStatusSourceUnknown          DiskStatusSource = "unknown"
	DiskStatusSourceNormal           DiskStatusSource = "normal"
)
```

`DiskDeviceSummary` 只含 API 需要的字段：`ID`、`Host`、`Device`、`Model`、容量、SMART 健康、温度、寿命、通电时间、结构化错误、`Status`、`StatusSource`、`CollectionLevel`。不得添加原始硬件身份。

- [ ] **Step 6: 实现独立快照缓存与 freshness**

默认值：

```go
if options.SnapshotTTL <= 0 {
	options.SnapshotTTL = 60 * time.Second
}
if options.CollectionInterval <= 0 {
	options.CollectionInterval = 60 * time.Second
}
```

loader 成功取得 Provider 快照后，先收集所有 `CollectionTracked` 设备的 `ID -> ReportedAt`，再调用 `freshness.Observe`。缓存命中和失败 loader 不调用 `Observe`。克隆快照后再离开 Service。

- [ ] **Step 7: 实现状态聚合与查询规范化**

采集等级来自：

```go
collectionLevel := LevelNormal
if device.CollectionTracked {
	collectionLevel = s.freshness.Level(device.ID, device.ReportedAt)
}
```

四类状态分别计算，最终使用本地 `diskHigherLevel` 合并。来源计算固定为：

1. 全正常返回 `normal`。
2. 最终异常等级与设备来源同级时，依次选择
   `smart_health`、`device_warning`、`attribute_failure`。
3. 只有 collection 严格高于全部设备来源时返回 `collection`。
4. 无法归属的 unknown 回退为 `unknown`。

自然排序实现只比较数字/非数字片段，不新增依赖；排序相等时使用稳定 ID 兜底。数值排序先比较可用性，再按方向比较值，缺失值始终在末尾。

- [ ] **Step 8: 运行 GREEN、Service 全测和 race**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/service/disk_types.go internal/service/disk_service.go internal/service/disk_service_test.go
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/service -count=1
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test -race ./internal/service -run 'Disk|Freshness' -count=1
```

- [ ] **Step 9: 可选授权提交**

```bash
git add internal/service/disk_types.go internal/service/disk_service.go internal/service/disk_service_test.go
git commit -m "feat: add disk smart service"
```

仅在用户明确授权提交后执行。

---

### Task 4: 60 秒配置、Provider 选择与上游超时

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Produces: `Config.SMARTCollectionInterval time.Duration`
- Consumes: `INFRAVIEW_SMART_COLLECTION_INTERVAL`
- Extends: `providerSet` with `Disks disk.Provider`
- Produces: `withDiskUpstreamTimeout`

- [ ] **Step 1: 写配置 RED 测试**

在默认配置测试中断言 `60*time.Second`；在全量配置测试中传入非默认整秒值；表驱动拒绝：

- 非法时长。
- `0s`。
- 负值。
- 小于 1 秒。
- 非整秒。

错误必须只包含配置键和安全的原始配置值。

- [ ] **Step 2: 写 Provider 选择与超时 RED 测试**

覆盖：

- Mock 模式同时返回 host、MySQL、disk Provider。
- Nightingale 模式三个接口由同一个 `*nightingale.Provider` 实现，不创建第二个客户端。
- 未知模式保持现有安全不可用 Provider 行为。
- 硬盘超时包装器向下游传递 deadline，超时返回 `context.DeadlineExceeded` 或包装后的领域错误，不泄露内部信息。

- [ ] **Step 3: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/config ./cmd/infraview -run 'SMART|Disk|Provider' -count=1
```

- [ ] **Step 4: 实现配置**

在 `Config` 增加：

```go
SMARTCollectionInterval time.Duration
```

加载与校验：

```go
if cfg.SMARTCollectionInterval, err = durationValue(
	getenv,
	"INFRAVIEW_SMART_COLLECTION_INTERVAL",
	"60s",
); err != nil {
	return Config{}, err
}
if cfg.SMARTCollectionInterval < time.Second || cfg.SMARTCollectionInterval%time.Second != 0 {
	return Config{}, fmt.Errorf(
		"INFRAVIEW_SMART_COLLECTION_INTERVAL 必须是不小于 1s 的整秒时长，当前值为 %q",
		valueOrDefault(getenv, "INFRAVIEW_SMART_COLLECTION_INTERVAL", "60s"),
	)
}
```

- [ ] **Step 5: 扩展 ProviderSet 和超时包装器**

```go
type providerSet struct {
	Hosts datasource.Provider
	MySQL mysql.Provider
	Disks disk.Provider
}
```

Mock 使用 `mock.NewDisk(clock)`；Nightingale 将同一个 `provider` 同时赋给三个字段。

新增：

```go
type diskTimeoutProvider struct {
	provider disk.Provider
	timeout  time.Duration
}

func (p *diskTimeoutProvider) SMARTSnapshot(ctx context.Context) (disk.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.provider.SMARTSnapshot(ctx)
}
```

本任务暂不把 `DiskService` 传入 HTTP，保持变更可以单独编译；运行时接线在 Task 5 完成。

- [ ] **Step 6: 运行 GREEN 与格式化**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/config/config.go internal/config/config_test.go cmd/infraview/main.go cmd/infraview/main_test.go
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/config ./cmd/infraview -count=1
```

- [ ] **Step 7: 可选授权提交**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/infraview/main.go cmd/infraview/main_test.go
git commit -m "feat: configure disk smart collection"
```

仅在用户明确授权提交后执行。

---

### Task 5: 两条只读硬盘 API 与运行时接线

**Files:**
- Create: `internal/httpapi/disk_handlers.go`
- Create: `internal/httpapi/disk_handlers_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `internal/httpapi/query_handlers.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`

**Interfaces:**
- Produces: `GET /api/v1/disks/overview`
- Produces: `GET /api/v1/disks/devices`
- Extends: `httpapi.Dependencies` with `DiskService *service.DiskService`
- Maps: `disk.ErrUnavailable` to safe retryable `503 disk_unavailable`

- [ ] **Step 1: 写 overview API RED 测试**

验证：

- 未认证为 401。
- GET 返回精确字段和现有 `{data, meta}` envelope。
- stale 缓存返回 `meta.stale=true` 和 `collected_at`。
- 任意非白名单 query 为 400。
- POST/PUT/DELETE 为 405，`Allow: GET`。
- Service 缺失或 `disk.ErrUnavailable` 为安全、可重试 503。

期望 JSON 形状：

```json
{
  "data": {
    "total": 0,
    "normal": 0,
    "warning": 0,
    "critical": 0,
    "unknown": 0,
    "affected_devices": 0,
    "warning_devices": 0,
    "critical_devices": 0,
    "alerts": {
      "smart_health": {"warning": 0, "critical": 0},
      "device_warning": {"warning": 0, "critical": 0},
      "attribute_failure": {"warning": 0, "critical": 0},
      "collection": {"warning": 0, "critical": 0}
    }
  },
  "meta": {}
}
```

- [ ] **Step 2: 写 devices API RED 测试**

验证参数白名单、空参数、默认值、分页错误和所有合法排序；响应中断言：

- `devices/total/page/page_size/total_pages`。
- 每个设备仅含设计中的页面字段。
- 每个设备包含显式 `status_source`，并断言正常与 SMART unknown 的来源值。
- `capacity_bytes`、数值和错误计数正确保留 `null`。
- JSON 文本不包含 `serial_no`、`wwn`、`labels`、`promql`、上游测试秘密。

- [ ] **Step 3: 运行 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi ./cmd/infraview -run 'Disk|AuthenticatedMockAPI' -count=1
```

- [ ] **Step 4: 实现固定 View 和 Handler**

`diskDeviceView` 明确列出 JSON 字段，禁止直接序列化领域对象：

```go
type diskDeviceView struct {
	ID                      string            `json:"id"`
	Host                    string            `json:"host"`
	Device                  string            `json:"device"`
	Model                   string            `json:"model"`
	CapacityBytes           *int64            `json:"capacity_bytes"`
	SMARTHealth             disk.Health       `json:"smart_health"`
	TemperatureCelsius      *float64          `json:"temperature_celsius"`
	LifetimeUsedPercent     *float64          `json:"lifetime_used_percent"`
	PowerOnHours            *float64          `json:"power_on_hours"`
	Errors                  diskErrorsView    `json:"errors"`
	Status                  service.Level     `json:"status"`
	StatusSource            service.DiskStatusSource `json:"status_source"`
	CollectionLevel         service.Level     `json:"collection_level"`
}
```

`devices` 只接受 `search,status,sort,order,page,page_size`；默认页码 1、页大小 20。错误消息复用统一“查询参数无效”，不得回显实际 query 值。

- [ ] **Step 5: 注册只读路由与安全错误**

在 `api.go` 注册两个认证 GET 路由及 method fallback。`writeServiceError` 在 datasource 通用分支之前添加：

```go
case errors.Is(err, disk.ErrUnavailable):
	writeError(w, r, http.StatusServiceUnavailable, "disk_unavailable", "数据源暂时不可用，请稍后重试", true)
```

- [ ] **Step 6: 完成 buildHandler 接线**

```go
diskProvider := withDiskUpstreamTimeout(providers.Disks, cfg.UpstreamTimeout)
diskService := service.NewDisk(diskProvider, store, service.DiskOptions{
	SnapshotTTL:        cfg.SMARTCollectionInterval,
	CollectionInterval: cfg.SMARTCollectionInterval,
	MaxStale:           cfg.MaxStale,
	Clock:              clock,
})
```

把 `diskService` 放入 `httpapi.Dependencies`。扩展 authenticated Mock API 测试，证明两个新端点在 Mock 模式可用。

- [ ] **Step 7: 运行 GREEN、HTTP 全测和格式化**

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.24-bookworm \
  gofmt -w internal/httpapi/disk_handlers.go internal/httpapi/disk_handlers_test.go internal/httpapi/api.go internal/httpapi/query_handlers.go cmd/infraview/main.go cmd/infraview/main_test.go
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/httpapi ./cmd/infraview -count=1
```

- [ ] **Step 8: 可选授权提交**

```bash
git add internal/httpapi cmd/infraview
git commit -m "feat: expose read-only disk smart APIs"
```

仅在用户明确授权提交后执行。

---

### Task 6: `/disks` 紧凑硬盘列表页

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Create: `web/src/features/disks/DiskPage.tsx`
- Create: `web/src/features/disks/DiskPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/app/theme.css`

**Interfaces:**
- Consumes: `GET /api/v1/disks/devices`
- Produces: authenticated `/disks` route
- Preserves: URL-driven query state and shared refresh interval

- [ ] **Step 1: 定义前端类型和脱敏 fixture**

在 `api/types.ts` 增加：

```ts
export type DiskSMARTHealth = 'healthy' | 'failed' | 'unknown'

export type DiskStatusSource =
  | 'smart_health'
  | 'device_warning'
  | 'attribute_failure'
  | 'collection'
  | 'unknown'
  | 'normal'

export interface DiskErrorCounters {
  pending_sectors: number | null
  reallocated_sectors: number | null
  uncorrectable_sectors: number | null
  udma_crc_errors: number | null
  media_integrity_errors: number | null
  error_log_entries: number | null
  unsafe_shutdowns: number | null
}

export interface DiskDevice {
  id: string
  host: string
  device: string
  model: string
  capacity_bytes: number | null
  smart_health: DiskSMARTHealth
  temperature_celsius: number | null
  lifetime_used_percent: number | null
  power_on_hours: number | null
  errors: DiskErrorCounters
  status: MetricLevel
  status_source: DiskStatusSource
  collection_level: MetricLevel
}
```

Fixture 只能使用虚构主机、设备和型号，不添加序列号或 WWN。

- [ ] **Step 2: 写 9 列与格式化 RED 测试**

断言表头严格为：

1. 主机
2. 设备
3. 型号 / 容量
4. SMART 健康
5. 温度
6. 寿命
7. 通电时间
8. 错误摘要
9. 状态

覆盖二进制容量、摄氏度、“已用 X%”、紧凑天/小时、`暂无数据` 和 SMART 正常/失败/未知文案。

- [ ] **Step 3: 写错误摘要 RED 测试**

纯函数或组件固定规则：

```ts
type ErrorItem = { label: string; value: number | null }
```

- 非零项最多显示两个，`title` 包含全部已知非零项。
- 全部已知且为零：`无已报告错误`。
- 全部缺失：`暂无数据`。
- 部分缺失且已知项全零：`未发现错误 · 部分暂无`。
- 不计算或显示错误总和。

- [ ] **Step 4: 写交互和 URL RED 测试**

覆盖：

- 搜索 host/device/model。
- 状态筛选。
- 六种排序和升降序。
- page/page_size。
- query 改变时回到第一页。
- 后端总页数缩小时纠正页码。
- 首次加载、刷新中、刷新失败、retryable 重试、stale banner、空结果。
- `status_source=collection` 的纯采集 warning/critical 显示采集延迟/失联。
- device_warning critical 与 collection critical 同级时显示最终“严重”。
- attribute_failure warning 与 collection warning 同级时显示最终“警告”。
- DOM 不包含“扫描”“自检”“修复”“启停”“擦除”等操作按钮。

- [ ] **Step 5: 运行 RED**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/disks/DiskPage.test.tsx'
```

- [ ] **Step 6: 实现页面**

沿用 MySQL/Host 页面模式：

- `useSearchParams` 是筛选、排序和分页的唯一 URL 状态来源。
- `useQuery` 的 key 包含规范化后的全部参数。
- 使用 `RefreshControl`、`StaleBanner`、`ErrorPanel`、`StatusBadge`。
- TanStack Table 只渲染固定 9 列。
- 状态列禁止通过等级相等猜来源；仅 `status_source === 'collection'` 时按
  `collection_level` 显示“采集延迟”或“采集失联”，否则显示最终状态文案。
- 表头和值保持单行、左对齐、数值使用等宽字体。

- [ ] **Step 7: 接入路由和样式**

`App.tsx`：

```tsx
<Route path="disks" element={<DiskPage />} />
```

CSS 使用现有 design token 和紧凑表格结构；不得加入横向页面滚动作为默认布局方案。对窄桌面通过列宽、截断和 tooltip 保持 9 列可读。

- [ ] **Step 8: 运行 GREEN、类型检查和前端回归**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/features/disks/DiskPage.test.tsx'
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run typecheck'
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run'
```

- [ ] **Step 9: 可选授权提交**

```bash
git add web/src/api/types.ts web/src/test/fixtures.ts web/src/features/disks web/src/app/App.tsx web/src/app/theme.css
git commit -m "feat: add disk smart list page"
```

仅在用户明确授权提交后执行。

---

### Task 7: 侧边栏、总览硬盘卡与浏览器验收用例

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
- Consumes: `GET /api/v1/disks/overview`
- Produces: sidebar order `总览 → 主机 → 硬盘 → MySQL`
- Produces: independent “主机硬盘” overview module

- [ ] **Step 1: 写导航 RED 测试**

断言四个入口文本、目标和顺序固定；“硬盘”指向 `/disks`，不出现二级详情或操作入口。

- [ ] **Step 2: 写总览卡 RED 测试**

增加 `DiskOverviewData/Response` 与 fixture，覆盖：

- 空数据。
- 全部正常。
- warning。
- critical。
- unknown 被纳入警告风险。
- 四类摘要：SMART 自检、设备警告、属性失败、采集状态。
- `affected_devices` 直接使用后端去重结果，前端不重复求和。
- 卡片底部只有“查看硬盘”，不重复最终状态标签。
- 硬盘模块 loading/error/stale/retry 独立，不阻塞 Linux/MySQL。
- 刷新错误保留上一次硬盘数据。

- [ ] **Step 3: 运行 RED**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/app/AppShell.test.tsx src/features/overview/OverviewPage.test.tsx'
```

- [ ] **Step 4: 实现独立模块**

扩展：

```ts
type ModuleLabel = 'Linux 主机' | '主机硬盘' | 'MySQL'
```

硬盘卡使用独立 query key、请求和 refetch；级别：

- `critical_devices > 0`：critical。
- 否则 `warning_devices > 0`：warning。
- 否则 normal。
- `total === 0`：empty。

四类 `MetricAlert` 只消费后端 `{warning, critical}`，不从设备数量反推。

- [ ] **Step 5: 增加 Playwright 用例但不运行额外端口**

在现有 Mock E2E 文件新增：

- 侧边栏进入硬盘页。
- 总览卡进入 `/disks`。
- 9 列、搜索、状态、排序和分页 URL。
- 页面没有破坏性控件。
- 桌面视口中 `document.documentElement.scrollWidth <= clientWidth`，表格区域不横向溢出。

当前 `scripts/e2e.sh` 会默认映射宿主机 `18080`，所以本任务只编写并静态检查用例；在当前“不得创建其他 InfraView 测试端口”约束下不得执行该脚本。

- [ ] **Step 6: 运行 GREEN 与全部前端非 E2E 验证**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run -- src/app/AppShell.test.tsx src/features/overview/OverviewPage.test.tsx'
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run && npm run typecheck && npm run build'
```

- [ ] **Step 7: 可选授权提交**

```bash
git add web/src/app web/src/features/overview web/src/api/types.ts web/src/test/fixtures.ts web/e2e/infraview.spec.ts
git commit -m "feat: add disk navigation and overview"
```

仅在用户明确授权提交后执行。

---

### Task 8: 文档同步、全量 Docker 验证与授权门槛

**Files:**
- Modify: `.env.example`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/CONFIGURATION.md`
- Modify: `docs/DESIGN.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/superpowers/plans/2026-07-30-host-disk-smart-module.md`

**Interfaces:**
- Records: SMART 指标、17 查询、60 秒 freshness、只读边界、验证证据和未授权事项
- Preserves: 用户已有三份交接文档修改

- [x] **Step 1: 增加配置与架构文档**

必须写明：

- `INFRAVIEW_SMART_COLLECTION_INTERVAL=60s` 的用途、默认值和整秒约束。
- 硬盘独立领域/Service/API/页面。
- Nightingale 固定 17 查询且一次 batch。
- v8.4.1 为主要验证版本，v9.x 仅协议兼容。
- ID 优先级与不可逆哈希；API 永不返回序列号/WWN。
- 温度、寿命和错误计数只展示，不使用 InfraView 通用阈值。
- freshness 是“InfraView 本地最后观察到原始样本时间推进的时刻”，不是业务值变化，也不是原始样本绝对年龄。
- 开发 8080 只连接测试 Nightingale。

- [x] **Step 2: 更新安全、测试和交接文档**

记录：

- 严格只读与明确不做项。
- Mock、Provider、Service、API、前端测试覆盖。
- 当前实际执行过的验证及结果；未执行项明确标注，后续单独授权的现有 8080 验收按最终结果增量更新。
- `PROJECT_STATUS/TODO/HANDOFF` 基于现有未提交内容增量更新，禁止覆盖用户修改。
- `HANDOFF` 给出下一会话可复制恢复提示，并注明提交/推送/部署仍需授权。

- [x] **Step 3: 运行敏感字段与破坏性能力静态扫描**

扫描只输出文件名和代码位置，不输出真实运行时数据：

```bash
rg -n 'serial_no|wwn|smartctl|nvme-cli|exec\\.Command|query_range|query-instant' \
  internal web/src docs .env.example README.md
rg -n '扫描|自检|修复|启停|擦除|删除|执行' web/src/features/disks web/src/features/overview
```

逐项确认：

- `serial_no/wwn` 只存在于 Provider 内部归并、测试输入和安全文档，不出现在 HTTP View/前端类型。
- 没有命令执行能力。
- 硬盘 Provider 只有固定即时 batch。

- [x] **Step 4: 运行 Go 普通与 race 全测**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./... -count=1
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test -race ./... -count=1
```

- [x] **Step 5: 运行前端全测、类型检查和生产构建**

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine \
  sh -c 'npm ci >/dev/null && npm run test:run && npm run typecheck && npm run build'
```

- [x] **Step 6: 无缓存完整镜像构建**

该命令只构建镜像，不启动服务、不占用端口：

```bash
docker build --no-cache --tag infraview:host-disk-smart-verify .
```

- [x] **Step 7: 最终工作树检查**

```bash
git diff --check
git status --short --branch
git diff --stat
```

确认没有覆盖用户原有交接文档差异，没有私密文件、临时抓取或真实数据进入 Git。

- [x] **Step 8: 记录当前约束下未执行的验收**

以下项目在当前授权下不得执行：

- `scripts/e2e.sh`：会创建额外宿主机测试端口。
- `docker compose up -d --build --force-recreate infraview`：会重建并重启现有 8080。
- 测试 Nightingale 现场 API/浏览器验收：需要先把新构建部署到现有 8080。
- 任何生产 Nightingale 验证：永久禁止。

实现者不得把这些项目写成“已通过”。如用户后来明确授权现有 8080 的重建/重启，只能保持其 Nightingale 配置指向测试环境，并在执行前重新只读核对目标服务；仍不得创建其他 InfraView 端口。

- [ ] **Step 9: 可选授权提交**

仅在全部已授权验证通过、用户明确授权提交后执行：

```bash
git add .env.example README.md docs internal cmd web
git diff --cached --check
git commit -m "feat: add host disk smart module"
```

推送是独立授权；没有明确推送授权时不得执行 `git push`。

---

## Definition of Done

- `internal/disk`、Mock、Nightingale Provider、DiskService、API 和 React 页面均有 RED→GREEN 测试证据。
- Nightingale Provider 精确发送一次固定 17 查询 batch，无 N+1。
- 设备 ID 包含主机并按 WWN、序列号、设备名回退；原始身份不离开 Provider。
- 硬盘快照 TTL 和 freshness 使用独立默认 60 秒，120/300 秒边界及恢复/回退/stale 行为有测试。
- 总览和列表状态、去重、搜索、自然排序、缺失值置后、分页及 9 列展示符合设计。
- 两条 API 只允许 GET，认证、参数、安全 503、stale 和敏感字段排除有测试。
- 总览三个板块独立加载/失败/过期，侧边栏顺序正确。
- Docker Go 普通/race、前端测试/类型/构建和无缓存镜像构建通过。
- 会创建 18080 的 `scripts/e2e.sh`、提交和推送保持未执行；经后续单独授权的现有 8080 原位重建、脱敏 API、一次性 Chromium 与跨两个采集周期验收通过。
