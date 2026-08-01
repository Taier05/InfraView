# InfraView 主机硬盘 SMART 模块设计

最后更新：2026-07-30

> 2026-08-01 替代说明：本文记录初始 SMART 模块的 17 组查询设计。当前容量契约已由 `docs/superpowers/specs/2026-08-01-disk-capacity-metric-and-column-design.md` 替代：固定 batch 为 18 组，第 17 组是 `smart_disk_capacity_bytes`，第 18 组才是 inventory；旧 inventory `capacity` 标签不再使用。本文其余历史证据保留，不据此判断当前实现。

## 背景与证据

InfraView 当前已经包含 Linux 主机与 MySQL 两个独立只读板块。本轮新增独立“主机硬盘”板块，数据源继续使用 Nightingale；开发、契约取证和验证只允许连接原开发 8080 所使用的测试 Nightingale，任何阶段都不得连接、切换或探测生产 Nightingale。

测试 Nightingale 的完全脱敏只读取证确认：

- 当前数据同时包含 `smart_device_*` 与 `smart_attribute_*` 指标。
- 共同身份标签包含 `ident`、`device`、`model`、`serial_no`；部分 ATA 风格序列另有 `wwn` 与 `capacity`。
- 当前指标族同时覆盖 NVMe 与 ATA 风格数据；仅凭脱敏标签和指标形状尚不能确认 SAS 覆盖。
- `smart_device_health_ok` 为二值。
- NVMe `critical_warning`、`available_spare`、`available_spare_threshold`、`percentage_used` 的值域符合对应警告、百分比和非负计数语义。
- ATA `fail` 标签值域可识别为 `-`、`FAILING_NOW` 或 `In_the_past`。
- 当前健康序列可取得非空 `ident`、`device` 与 `serial_no`；WWN 只在部分设备存在。
- `tlast_over_time(smart_device_health_ok[24h])` 保留设备身份标签，可用于近期身份和样本推进判断。
- 即时 batch 仍符合 Nightingale v8.4.1 的 `dat[查询索引][序列]` 契约；v9.x 只保留既有协议兼容。

取证没有输出或保存私密环境文件、Token、Cookie、认证头、Base URL、真实主机或硬盘标识、IP、序列号、WWN、资源数量、指标值或上游正文，也没有修改或重启任何服务。

## 目标

- 在侧边栏增加独立“硬盘”入口，并提供 `/disks` 当前硬盘健康清单。
- 在总览增加与 Linux/MySQL 等宽、同风格的“主机硬盘”卡片。
- 通过一次固定 Nightingale 即时 batch 获取全部硬盘数据，禁止按主机或硬盘 N+1。
- 以设备自身 SMART 结论、NVMe 设备警告、ATA 属性失败和设备提供的阈值派生状态。
- 使用独立 `60s` 预期采集周期和现有样本推进语义识别采集延迟/失联。
- 保持 InfraView 严格只读、安全错误映射、缓存/stale 和深色中文紧凑界面基线。

## 明确不做

- 不提供单盘详情页或历史趋势。
- 不提供 SMART 扫描、自检、启停、修复、擦除或设备控制。
- 不连接块设备，不执行 `smartctl`、`nvme-cli`、SSH、远程命令或脚本。
- 不向前端暴露任意 PromQL、任意 URL、Nightingale 原始请求体或任意代理。
- 不显示或通过 API 返回序列号、WWN 或原始标签。
- 不使用温度、寿命或错误计数的 InfraView 通用阈值改变总体状态。
- 不连接、切换或探测生产 Nightingale。
- 未经单独授权，不重建或重启既有 8080，不创建额外 InfraView 测试端口，不提交或推送。

## 总体架构

### 独立领域

新增 `internal/disk` 领域，职责仅限：

- 定义硬盘设备、SMART 快照和只读 Provider 契约。
- 生成不暴露原始硬件标识的稳定设备 ID。
- 表达可空 SMART 数据，不负责页面筛选、排序或状态文案。

接口固定为：

```go
type Provider interface {
    SMARTSnapshot(context.Context) (Snapshot, error)
}
```

共享的 Nightingale `*Provider` 同时实现现有主机、MySQL 和新增硬盘 Provider，复用同一个受限 HTTP Client、默认数据源发现缓存、认证、超时、响应大小、Content-Type、JSON envelope 与错误清洗逻辑。

### Service

新增独立 `DiskService`：

- 缓存完整硬盘快照。
- 仅在成功调用 Provider 的 loader 中观察样本推进。
- 派生 SMART 自检、设备警告、属性失败、采集状态和最终状态。
- 提供总览聚合、搜索、筛选、排序和分页。
- 克隆所有可变/可空字段，避免缓存值被调用方修改。

### HTTP API

新增只读接口：

- `GET /api/v1/disks/overview`
- `GET /api/v1/disks/devices`

仅允许 GET；其他方法返回现有统一 `405` 错误。接口复用认证、中间件、请求 ID、安全错误与 `meta.stale` 契约。

### 前端

- 新增 `/disks` 路由和硬盘列表页。
- 侧边栏顺序为“总览、主机、硬盘、MySQL”。
- 总览新增独立硬盘卡；其加载、错误、stale 和重试不阻塞 Linux/MySQL 卡片。
- 复用现有刷新控件、后端下发的页面刷新周期、URL 状态、分页和紧凑表格样式。

### Mock

新增确定性、完全脱敏的 Mock 硬盘 Provider，用于领域契约、Service、API、前端和隔离 E2E。Mock 至少覆盖：

- ATA 与 NVMe 两种标签/字段形状。
- 正常、警告、严重、未知。
- SMART 自检失败、NVMe 设备警告、ATA 当前/历史属性失败。
- 采集延迟、采集失联、恢复推进。
- 部分字段缺失、错误计数为零和存在非零错误记录。

Mock 只用于测试和隔离验收，不改变真实部署的数据源选择。

## 固定 Nightingale 查询

硬盘快照只发送一次 `POST /api/n9e/query-instant-batch`，请求内固定包含以下 17 条代码内置查询：

1. `smart_device_health_ok`
2. `smart_device_temp_c`
3. `smart_attribute_temperature_celsius`
4. `smart_attribute_percentage_used`
5. `smart_attribute_power_on_hours`
6. `smart_attribute_critical_warning`
7. `smart_attribute_available_spare`
8. `smart_attribute_available_spare_threshold`
9. `smart_attribute_value{fail=~"FAILING_NOW|In_the_past"}`
10. `smart_device_pending_sector_count`
11. `smart_device_reallocated_sectors_count`
12. `smart_device_uncorrectable_sector_count`
13. `smart_device_udma_crc_errors`
14. `smart_attribute_media_and_data_integrity_errors`
15. `smart_attribute_error_information_log_entries`
16. `smart_attribute_unsafe_shutdowns`
17. `tlast_over_time(smart_device_health_ok[24h])`

约束：

- 查询顺序是 Provider 契约的一部分，由直接测试锁定。
- 必须恰好返回 17 组结果；外层基数不匹配，或第 1、17 组任一结果为 `null` 时整次快照失败。
- 不增加第二次即时 batch，不按主机或硬盘发起查询。
- 不接受前端、API 参数或配置文件传入 PromQL。
- 第 17 条查询同时负责最近 24 小时身份和原始最后样本时间；当前健康查询只负责当前 SMART 自检。

## 设备身份

稳定 ID 始终包含主机 `ident`，设备身份按以下顺序选择：

1. 非空 `wwn`
2. 非空 `serial_no`
3. 非空 `device`

稳定 ID 使用主机 ID、身份类型和身份值生成不可逆哈希，避免把原始唯一标识暴露给 API。不能只使用 `/dev/sdX` 或 `nvmeX`，因为设备名可能随重启或硬件拓扑变化。

Provider 归并规则：

- 以第 17 组最近身份查询建立完整设备集合。
- 第 17 组允许同一 `ident + device` 出现多条 24 小时历史身份序列：兼容身份先归并，`ReportedAt` 取最大的原始样本时间，非空型号/容量按较新值优先且不因较新序列缺失而丢失；同一时间的非空元数据冲突仍使快照失败。
- 当前和辅助指标先以 `ident + device` 定位设备，再用 `serial_no` 和可选 WWN 校验身份；同一辅助身份只有在主身份与候选序列对应字段双方都非空且值不同时才冲突，任一侧缺失仍允许归并。
- 同一稳定身份出现冲突设备标签，或同一设备身份映射到多个稳定 ID 时，整次快照失败。
- 与已知设备无法匹配的辅助序列忽略，不能创建无主身份设备。
- 序列号和 WWN 可保留在 Provider 内部归并状态，但不得进入 Service 对外类型、HTTP API、前端、日志或错误消息。

## 领域数据

每块硬盘包含：

- `ID`
- `HostID`
- `Device`
- `Model`
- 可空 `CapacityBytes`
- SMART 自检状态
- 可空温度（摄氏度）
- 可空寿命已用百分比
- 可空通电小时数
- NVMe `CriticalWarning`
- 可空 `AvailableSparePercent`
- 可空 `AvailableSpareThresholdPercent`
- ATA 当前/历史失败属性状态
- 各类可空错误计数
- `CollectionTracked`
- `ReportedAt`

错误计数包括：

- 待处理扇区
- 重映射扇区
- 不可修复扇区
- UDMA CRC 错误
- NVMe 介质与数据完整性错误
- NVMe 错误日志条目
- 非安全关机

## 数值与冲突语义

- 容量仅接受可解析的非负字节数。
- 温度从 `smart_device_temp_c` 和 `smart_attribute_temperature_celsius` 中选择最新有效值。
- 寿命直接显示 `percentage_used` 的非负有限值，不擅自换算为剩余寿命等级。
- 通电时间和错误计数只接受非负有限值。
- `health_ok` 只接受 `0` 或 `1`。
- `critical_warning` 只接受非负整数。
- `available_spare` 与 `available_spare_threshold` 只接受 `0..100` 百分比。
- 同一字段选择最新有效样本；较旧样本不得覆盖新值。
- 同一最新时间戳出现不同有效值时，该字段保持 `null`，不能任意选择。
- 缺失、负数、NaN、Inf、越界、解析失败或冲突字段保持 `null`，页面显示“暂无数据”，不得伪造成零。

## 状态模型

### SMART 自检

- `health_ok=1`：正常。
- `health_ok=0`：严重。
- 缺失：未知。

### 设备警告

- NVMe `critical_warning != 0`：严重。
- `available_spare <= available_spare_threshold`：严重。
- 两个备用空间字段任一缺失时，不猜测阈值结果。

### 属性失败

- ATA `fail=FAILING_NOW`：严重。
- ATA `fail=In_the_past`：警告。
- 同一设备存在多个失败属性时只按最高等级计一次设备状态，但总览属性失败分类仍按设备去重。

### 采集状态

新增配置：

```text
INFRAVIEW_SMART_COLLECTION_INTERVAL=60s
```

- 默认值为 `60s`，非法或非正值拒绝启动。
- 硬盘快照缓存 TTL 使用该周期，避免页面每 15 秒刷新就重复查询 Nightingale。
- `DiskService` 使用独立并发安全 `freshnessTracker`。
- 第一次观察非零原始时间建立正常基线。
- 原始时间变化或回退时重置本地推进时刻。
- 连续 `2 * interval` 未推进为警告，即默认 120 秒。
- 连续 `5 * interval` 未推进为严重，即默认 300 秒。
- 只在 Provider loader 成功时调用 `Observe`；缓存命中不伪造推进。
- 上游失败并返回 stale 缓存时，本地推进时间继续老化。
- 进程重启后重新建立基线；仍停采的设备最迟在 5 个周期后重新进入严重。

### 最终状态

最终状态取 SMART 自检、设备警告、属性失败和采集状态中的最高等级：

```text
critical > warning > unknown > normal
```

- 温度、寿命和错误计数仅展示，不参与最终等级。
- 设备自身严重时，状态文案显示“严重”，不能被较低等级的“采集延迟”掩盖。
- 2026-07-30 用户裁定采用方案 1：API 为每个设备显式返回 `status_source`，
  前端不得再通过 `status === collection_level` 猜测状态来源。
- `status_source` 固定为
  `smart_health|device_warning|attribute_failure|collection|unknown|normal`。
- Service 分别计算 SMART 自检、设备警告、属性失败和采集等级，再计算最终状态。
  任一设备来源与最终状态同级时设备来源优先；设备来源内部稳定优先级为
  `smart_health`、`device_warning`、`attribute_failure`。
- 只有采集等级严格高于全部设备来源时，`status_source=collection`；
  全正常为 `normal`，无法归属的 unknown 回退为 `unknown`。
- 仅当 `status_source=collection` 时，状态列显示“采集延迟”或“采集失联”。
- 未知设备纳入总览受影响与警告风险，但不伪造成具体 SMART 告警。

## Service 查询与排序

硬盘列表查询参数：

- `search`
- `status`
- `sort`
- `order`
- `page`
- `page_size`

规则：

- `search` 只匹配主机 ID、设备名和型号。
- `status` 接受 `normal|warning|critical|unknown`。
- `sort` 接受 `host|device|temperature|lifetime|power_on_hours|status`。
- `order` 接受 `asc|desc`。
- `page_size` 只接受 `20|50|100`。
- 默认按主机、设备自然升序；设备名中的数字按自然顺序比较。
- 可空数值排序时缺失值始终置后，不因倒序移到前面。
- 过滤与排序在完整快照上执行，随后分页。

## API 契约

### `GET /api/v1/disks/overview`

返回：

- `total`
- `normal`
- `warning`
- `critical`
- `unknown`
- `affected_devices`
- `warning_devices`
- `critical_devices`
- `alerts.smart_health`
- `alerts.device_warning`
- `alerts.attribute_failure`
- `alerts.collection`

每个告警分类使用现有 `{warning, critical}` 结构。`warning_devices` 包含最终状态为 warning 或 unknown 的设备；未知设备同时计入 `affected_devices`，但不增加具体告警分类。

### `GET /api/v1/disks/devices`

返回：

- `devices`
- `total`
- `page`
- `page_size`
- `total_pages`

每个设备仅返回页面需要的字段：

- `id`
- `host`
- `device`
- `model`
- `capacity_bytes`
- `smart_health`
- `temperature_celsius`
- `lifetime_used_percent`
- `power_on_hours`
- 结构化 `errors`
- `status`
- `status_source`
- `collection_level`

不得返回 `serial_no`、`wwn`、原始标签、PromQL、数据源 ID 或上游请求信息。

## 页面设计

### 总览卡

新增与 Linux/MySQL 等宽的“主机硬盘”卡片：

- 顶部展示整体正常、警告、严重或警告/未知状态。
- 中部展示受影响硬盘占总数，以及严重/警告风险。
- 四类摘要为 SMART 自检、设备警告、属性失败、采集状态。
- 底部只保留“查看硬盘”入口，不重复状态标签。
- 空数据、首次加载、刷新失败、stale 和重试沿用现有模块规范。

### 硬盘列表

固定 9 列：

1. 主机
2. 设备
3. 型号 / 容量
4. SMART 健康
5. 温度
6. 寿命
7. 通电时间
8. 错误摘要
9. 状态

展示规则：

- 序列号和 WWN 永不显示。
- 容量使用现有二进制字节单位格式。
- 温度显示摄氏度。
- 寿命显示“已用 X%”。
- 通电时间使用紧凑天/小时格式。
- 错误计数不能相加。
- 存在非零错误项时，单元格最多展示两项，悬停提示显示全部已知非零项。
- 所有已知错误项均为零时显示“无已报告错误”。
- 所有错误项缺失时显示“暂无数据”。
- 部分指标缺失且已知项均为零时显示“未发现错误 · 部分暂无”。
- 状态文案只读取 `status_source`：来源为 `collection` 时按
  `collection_level` 显示采集延迟/失联，否则显示最终状态文案。
- 页面保持表头和值单行、左对齐、等宽数值、无横向溢出。
- 搜索、筛选、排序、页码和每页数量写入 URL。

## 缓存与错误处理

- 使用独立硬盘快照缓存键，避免与主机或 MySQL 污染。
- 默认快照 TTL 为 SMART 采集周期。
- 继续使用现有 `MaxStale` 允许返回最近一次可用快照。
- Provider 错误统一映射为 `disk.ErrUnavailable`。
- HTTP API 把数据源不可用映射为安全、可重试的 `503`，不泄露上游细节。
- 必须验证 HTTP 状态、精确 JSON Content-Type、JSON 结构、非 null `dat`、空 `err`、batch 外层基数和 8 MiB 响应上限。
- 401/403、非 JSON、重定向、超时、oversize、envelope 错误、解析失败或主身份冲突都安全失败。
- 辅助指标的非法数值只使对应字段缺失；主身份、外层形状和查询基数错误使整个快照失败。

## 安全边界

InfraView 始终只展示数据，不执行任何运维操作。

- 运行时只调用已确认的 Nightingale 只读接口。
- 不调用磁盘、SMART 或主机写接口。
- 不在日志和错误中输出 Token、Cookie、认证头、Base URL、原始响应、序列号或 WWN。
- 不记录真实主机、硬盘、IP、数量或指标值到公开仓库和测试夹具。
- 不增加 SSH 客户端、数据库连接、任意代理或远程执行能力。
- 开发 8080 永远只连接测试 Nightingale。

## 测试策略

严格执行 RED→GREEN TDD。

### 领域测试

- 稳定 ID 的 WWN、序列号和设备名回退优先级。
- 主机 ID 必须参与稳定身份。
- 原始身份不从对外类型泄露。
- 深拷贝和可空字段。

### Nightingale Provider 测试

- 精确 17 查询及固定顺序。
- 一次 batch、一次数据源发现 flight、无主机/硬盘 N+1。
- ATA 与 NVMe 脱敏标签形状。
- 主身份建立、辅助归并、未匹配辅助序列和身份冲突。
- 最新有效样本、同时间冲突、非法容量、负计数、NaN/Inf 和百分比范围。
- `health_ok`、`critical_warning`、备用空间阈值和 ATA `fail` 值域。
- 最近 24 小时身份与原始样本时间。
- 401/403、非 JSON、重定向、Content-Type、envelope、基数、超时和大小限制。

### Service 测试

- SMART 自检、设备警告、属性失败和最高等级归并。
- 默认 60 秒周期及非法配置。
- 120/300 秒精确边界。
- 稳定绝对时间差不告警、样本冻结升级、恢复推进、时间回退和并发访问。
- 缓存命中不伪造推进，stale 时推进时间继续老化。
- 搜索、状态筛选、自然排序、可空值置后和分页。
- 总览设备去重和未知风险计数。

### HTTP API 测试

- 两个 GET 接口的 JSON 契约。
- 方法、参数白名单、空参数和分页错误。
- stale 元数据与安全 503。
- 响应不包含序列号、WWN、原始标签、PromQL 或上游信息。

### 前端测试

- 总览硬盘卡的空、正常、警告、严重、未知、stale 和刷新失败状态。
- 四类摘要、异常设备去重和无重复底部状态。
- 9 列表头、格式化、错误摘要和缺失值。
- 搜索、筛选、排序、URL 状态、分页和页面纠正。
- 页面不存在扫描、测试、修复、启停或其他破坏性控件。

### E2E 与完整验证

- 使用确定性 Mock 完成导航、总览、硬盘列表、URL 状态、stale/error 和无破坏性控件验收。
- 使用 Docker 执行 Go 普通/race 测试、前端 Vitest、TypeScript、生产构建和 Go 编译。
- 最终执行无缓存完整镜像构建。
- 浏览器验证桌面紧凑布局、表头单行和页面/表格无横向溢出。
- 真实契约和最终 8080 验收只允许使用测试 Nightingale。
- 未经用户单独授权，不重建或重启既有 8080，不创建额外 InfraView 测试端口。

## 文档同步

实现时同步更新：

- `.env.example`
- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/CONFIGURATION.md`
- `docs/DESIGN.md`
- `docs/SECURITY.md`
- `docs/TESTING.md`
- `docs/datasources/NIGHTINGALE.md`
- `docs/PROJECT_STATUS.md`
- `docs/TODO.md`
- `docs/HANDOFF.md`
- 对应实施计划

## 验收标准

- 总览出现独立主机硬盘卡，侧边栏出现“硬盘”，`/disks` 可访问。
- 硬盘页按确认的 9 列展示，不暴露序列号或 WWN。
- SMART 自检、设备警告、属性失败和 60 秒采集新鲜度状态符合本设计。
- 所有数据来自一次固定 17 查询 batch，无 N+1、无任意 PromQL。
- ATA 与 NVMe 脱敏契约、缺失/冲突语义、缓存/stale 和安全错误均有自动化覆盖。
- Docker 全量验证通过。
- 产品与开发流程保持严格只读，且没有连接任何生产 Nightingale。
