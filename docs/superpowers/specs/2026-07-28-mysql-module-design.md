# InfraView MySQL 模块设计规格

最后更新：2026-07-28

## 目标

在 InfraView 中增加独立的 MySQL 只读观测模块。首期交付包括：

- 基础设施总览中的 MySQL 健康摘要卡。
- MySQL 实例列表。
- Nightingale v8.4.1 固定批量查询接入。
- 完全虚构且确定性的 MySQL Mock 数据。

MySQL 模块继续遵守 InfraView 的只读产品边界：只展示监控数据，不提供 SQL 执行、实例重启、主从切换、配置修改、用户管理、任意 PromQL、任意代理、SSH 或远程命令。

## 范围

### 首期包含

- 每个 MySQL 实例一行的当前状态列表。
- 实例搜索、状态筛选、角色筛选、排序、分页和 URL 状态。
- 可用性、复制线程、复制延迟和复制数据完整性告警。
- 连接、活跃线程、QPS、慢查询、Buffer Pool、运行时间等当前指标展示。
- 与现有刷新控件、缓存、singleflight 和 stale 降级一致的交互。
- Mock、Go、前端和 Chromium 自动化验证。
- 使用私密配置进行脱敏、只读的 Nightingale v8.4.1 验收。

### 首期不包含

- MySQL 单实例详情页。
- 历史趋势或时间范围选择。
- SQL、慢 SQL 文本、Schema、表、账号、权限或配置展示。
- 数据库告警事件系统。
- 可配置指标、动态列或动态 PromQL。
- MySQL 管理、变更或故障处置能力。

## 已确认的 Nightingale 指标契约

2026-07-28 已使用现有私密配置对 Nightingale v8.4.1 完成只读、脱敏探查。公开规格只记录指标名、标签键、单位类别和响应形状，不记录 Base URL、Token、认证头、真实实例标识、地址、数量、指标值或响应正文。

通用实例标签：

- `ident`
- `instance`
- `address`

`ident + instance` 在当前契约中不能保证唯一，`ident + instance + address` 可以唯一标识一条 `mysql_up` 实例序列。

复制指标额外包含：

- `channel_name`
- `master_host`
- `master_uuid`

三项标签键共同构成复制通道身份；只要求键存在，标签值允许为空（包括默认空 `channel_name`）。缺少任一键的复制序列忽略，不把通道身份标签暴露到领域对象或 API。

已确认首期指标：

| 领域字段 | Nightingale 指标或固定 PromQL | 单位与语义 |
| --- | --- | --- |
| 可用性 | `mysql_up` | 0/1 |
| 版本 | `mysql_version_info` 的 `version` 标签 | 字符串 |
| 运行时间 | `mysql_global_status_uptime` | 秒 |
| 读写属性 | `mysql_global_variables_read_only` | 0/1，仅解释为可写/只读 |
| 当前连接 | `mysql_global_status_threads_connected` | 连接数 |
| 最大连接 | `mysql_global_variables_max_connections` | 连接数 |
| 活跃线程 | `mysql_global_status_threads_running` | 线程数 |
| QPS | `rate(mysql_global_status_questions[5m])` | 每秒 |
| 慢查询速率 | `rate(mysql_global_status_slow_queries[5m])` | 每秒 |
| Buffer Pool 使用率 | `mysql_global_status_buffer_pool_pages_utilization` | 百分比 |
| 复制延迟 | `mysql_slave_status_seconds_behind_master` | 秒 |
| 复制 IO 线程 | `mysql_slave_status_slave_io_running` | 0/1 |
| 复制 SQL 线程 | `mysql_slave_status_slave_sql_running` | 0/1 |

二值指标已验证符合 0/1 语义；Buffer Pool 使用率已验证符合百分比范围。所有查询必须在代码内固定，不允许从 HTTP 请求、前端或配置传入 PromQL。

## 架构

采用“独立 MySQL 领域接口，共享 Nightingale 客户端”的边界。

### MySQL 领域

新增独立 MySQL 领域包，定义：

- `Instance`
- `CurrentMetrics`
- `ReplicationState`
- `Snapshot`
- `Overview`
- `AlertSummary`
- 只读 `Provider`

MySQL Provider 只暴露一个 snapshot 读取入口。现有 Linux 主机 `datasource.Provider` 不增加 MySQL 方法，避免主机、数据库及未来其他基础设施模块相互耦合。

### Nightingale 适配器

Nightingale MySQL Provider 复用现有：

- 安全 HTTP Client。
- 默认 Prometheus 类型数据源发现与缓存。
- 超时和 context 取消。
- HTTPS 默认与显式受控 HTTP 测试开关。
- 拒绝重定向。
- JSON Content-Type、HTTP 状态、`dat`/`err` envelope 和批量外层基数校验。
- 8 MiB 响应大小上限。
- 安全领域错误映射。

Provider 使用一次 `/api/n9e/query-instant-batch` 发送全部固定查询，按实例组合键归并结果。不得按实例发起 N+1 请求。

### Service

新增独立 MySQL Service，负责：

- snapshot 缓存和 singleflight。
- stale 降级。
- 指标规范化和缺失值语义。
- 多复制通道聚合。
- 实例最终等级与总览告警摘要。
- 搜索、筛选、稳定排序和分页。

总览和实例列表从同一个缓存 snapshot 派生，不重复请求 Nightingale。

### Mock

Mock MySQL Provider 实现相同领域接口，提供完全虚构、确定性的：

- 正常、警告、严重和未知实例。
- 可写和只读实例。
- 单通道和多通道复制。
- 缺失辅助指标。
- 复制线程异常及边界延迟样本。

Mock 必须明确显示为 Mock 数据，不复用任何真实标签值或响应。

## 实例身份和数据归并

实例组合键为：

```text
ident + NUL + instance + NUL + address
```

API `id` 使用该组合键生成稳定的 URL 安全摘要，不直接嵌入原始标签。该摘要只用于稳定标识，不作为访问控制或保密边界。

页面显示：

- `instance`
- `address`
- `ident`

日志、错误和测试失败消息不得输出这些字段的真实值。测试只使用 `fixture-*` 名称和 RFC 5737 文档地址。

`mysql_up` 是首期当前实例集合的基准序列：

- 成功且为空时返回有效空列表。
- 身份标签完整但值无效时保留实例，状态为未知。
- 任一序列缺少 `ident`、`instance` 或 `address` 时，整个 snapshot 判为不可用。
- 完整组合键重复时，整个 snapshot 判为不可用。

辅助指标按相同组合键合并。缺失或非法的非关键指标返回 `null`，不得转换为零。

同一实例出现多条非复制辅助指标序列时，选择时间戳最新的有效样本；负数、NaN、Inf、超出 0..100 的 Buffer Pool 使用率等非法候选必须先忽略，不能更新已选时间。最新有效时间戳相同但值或版本标签冲突时，该字段返回 `null`。复制指标保留通道维度后再按本规格聚合；每个通道的复制延迟也选择最新有效样本，非法候选不得推进时间。

## 角色语义

`mysql_global_variables_read_only` 只用于显示：

- `0`：可写。
- `1`：只读。
- 缺失或非法：未知。

不得仅凭 `read_only` 把实例声明为“主库”或“从库”。复制关系由独立复制指标表达。

## 复制通道聚合

每个实例可以包含多个复制通道。列表保持每个实例一行，并按以下规则聚合：

1. 任一通道的 IO 或 SQL 线程停止，则复制线程状态为严重。
2. 所有已知通道线程均正常，则复制线程状态正常。
3. 存在通道但线程状态缺失或非法，且没有更高等级异常时，复制线程状态未知。
4. 复制延迟取所有有效通道中的最大值。
5. 线程异常等级高于复制延迟等级。

不向前端暴露 `master_host` 或 `master_uuid`；首期不展示复制拓扑。

## 告警规则

等级为 `normal`、`warning`、`critical` 和 `unknown`。实例最终等级优先级为：

```text
critical > warning > unknown > normal
```

`unknown` 保持独立展示并可单独筛选，但在总览告警口径中按警告风险处理：

- 状态摘要中的 `normal`、`warning`、`critical`、`unknown` 互斥，合计为实例总量。
- `warning_instances` 包含最终状态为 `warning` 或 `unknown` 的实例。
- `critical_instances` 只包含最终状态为 `critical` 的实例。
- `affected_instances` 包含最终状态为 `warning`、`unknown` 或 `critical` 的实例。
- 卡片有严重实例时整体为严重；否则有警告或未知实例时整体为警告；其余为正常。

实例最终等级取以下类别中的最高等级：

### 可用性

- `mysql_up=1`：正常。
- `mysql_up=0`：严重。
- `mysql_up` 值缺失或非法：未知，并按警告计入实例摘要。

### 复制线程

- 任一已知复制通道的 IO 或 SQL 线程停止：严重。
- 所有已知复制通道线程正常：正常。
- 应有复制状态但数据不完整：未知，并按警告计入实例摘要。

### 复制延迟

- `0 <= lag < 5` 秒：正常。
- `5 <= lag < 30` 秒：警告。
- `lag >= 30` 秒：严重。
- 负数、非有限值或解析失败：缺失。

### 复制数据缺失

- 可写实例没有复制指标：显示“未配置复制”，不告警。
- 只读实例缺少复制线程状态或复制延迟：显示“状态未知”，记为警告。
- 角色未知且复制数据不足：显示“状态未知”，记为警告。

### 信息指标

当前连接、最大连接、连接使用率、活跃线程、QPS、慢查询速率、Buffer Pool 使用率和运行时间首期只展示，不参与告警。

连接使用率仅在当前连接和最大连接都有效且最大连接大于零时计算；否则返回 `null`。

## 缓存与降级

- MySQL snapshot 使用独立缓存键。
- TTL 默认与当前指标刷新周期一致。
- 相同未命中请求合并为一次 Provider 调用。
- 上游失败且旧值未超过 `MaxStale` 时返回旧 snapshot，并设置 `meta.stale=true` 和原采集时间。
- 上游成功返回空实例集合时视为有效新结果，不继续返回旧实例。
- 超过 `MaxStale` 或没有旧值时返回统一可重试错误。
- 缓存值在返回前复制，调用方不得修改共享 snapshot。

## 只读 API

### MySQL 总览

```text
GET /api/v1/mysql/overview
```

返回：

- 实例总量。
- 正常、警告、严重和未知摘要。
- 受影响实例、警告实例和严重实例。
- `availability`
- `replication_threads`
- `replication_lag`
- `replication_data`

受影响实例按最终最高等级去重；各告警类别独立计数。

### MySQL 实例列表

```text
GET /api/v1/mysql/instances
```

查询参数：

- `search`：匹配实例名、地址或所属主机。
- `status`：`normal`、`warning`、`critical`、`unknown`。
- `role`：`writable`、`read_only`、`unknown`。
- `sort`
- `order`
- `page`
- `page_size`

排序白名单：

- 实例名。
- 连接使用率。
- 活跃线程。
- QPS。
- 慢查询速率。
- Buffer Pool 使用率。
- 复制延迟。
- 运行时间。
- 状态。

分页只允许 20、50、100，默认 20。缺失值始终置后；相同值使用实例稳定 ID 保持确定顺序。

MySQL API 只注册 GET。POST、PUT、PATCH、DELETE 返回 405。不存在 SQL、restart、failover、promote、configure、proxy 或 query 路由。

## 前端设计

### 总览卡片

在 Linux 主机卡旁增加同尺寸 MySQL 卡片，点击进入 `/mysql`。

卡片展示：

- 整体状态。
- 异常实例。
- 严重和警告实例。
- 可用性异常。
- 复制线程异常。
- 复制延迟异常。
- 复制数据缺失。
- 正常、警告、严重和未知摘要。

零异常使用绿色“无异常”文案，不使用异常色强调零值。卡片不展示 QPS、连接等负载聚合值。

### MySQL 实例列表

侧边栏增加“MySQL”入口。数据连接仍表示 Nightingale 或 Mock，不把 MySQL 误称为独立数据源连接。

页面采用紧凑 11 列：

1. 实例：单行组合显示实例名和地址，超长省略并提供完整提示。
2. 所属主机。
3. 版本 / 角色。
4. 连接使用。
5. 活跃线程。
6. QPS。
7. 慢查询速率。
8. Buffer Pool 使用率。
9. 复制状态 / 延迟。
10. 运行时间。
11. 状态。

连接使用显示当前连接、最大连接和可计算的百分比。复制列显示“正常”“线程异常”“未配置复制”或“状态未知”，有有效延迟时附带秒数。

只有可用性和复制指标使用警告或严重色。信息指标不设置经验阈值，不擅自着色。

搜索、筛选、排序、页码和每页数量写入 URL。页面复用统一刷新控件，显示上次成功刷新时间、运行时刷新周期和“正在刷新…”。stale、空态、首次加载、后台刷新失败和无缓存错误沿用现有统一交互。

首期不提供实例详情链接、操作列或历史图表。

## 错误处理

以下情况使整个 snapshot 不可用：

- HTTP 状态、Content-Type 或 JSON envelope 非法。
- `err` 非空。
- `dat` 为 null 或批量外层结果数不匹配。
- `mysql_up` 结果不是预期 vector。
- `mysql_up` 关键身份标签缺失。
- 实例完整组合键重复。
- 响应超过大小限制。

辅助指标单条序列缺失、值非法或无法匹配实例时，不泄露原始序列；对应字段保持缺失。复制关键数据缺失按告警规则转为未知。

错误响应和日志不得包含 Token、Cookie、认证头、Base URL、PromQL、上游正文、实例标签值或指标值。

## 测试策略

严格执行 RED -> GREEN TDD。

### 领域与 Service

- 复制延迟 5 秒和 30 秒边界。
- 多通道最差线程状态和最大延迟。
- 可写、只读和未知角色的缺失复制语义。
- 实例最终最高等级。
- 连接使用率计算及无效分母。
- 搜索、状态/角色筛选、稳定排序、缺失值置后和分页。
- snapshot TTL、singleflight、stale、空成功结果和独立值复制。

### Nightingale 适配器

- 完全脱敏的实例、版本、当前状态和多复制通道夹具。
- 一次 snapshot 只调用一次 `/query-instant-batch`。
- 固定查询顺序和批量外层基数。
- 身份标签合并、缺失身份和重复组合键。
- 缺失辅助指标、非法数值和空结果。
- 401/403、非 JSON、HTML、envelope 错误、超时和响应大小。
- 错误不泄露上游正文或敏感标签。

### Mock

- 通过 MySQL Provider 契约测试。
- 确定性结果。
- 正常、警告、严重、未知、可写、只读和多通道样本。

### HTTP

- 两个 GET API 的 JSON 契约。
- 非法 search/status/role/sort/order/page/page_size。
- 未认证响应。
- 非 GET 方法返回 405。
- 不存在任何写接口或任意查询接口。

### 前端

- 总览 MySQL 卡及四类告警摘要。
- 11 列实例列表。
- 搜索、筛选、排序、分页和 URL 恢复。
- 空态、stale、后台刷新错误和首次错误。
- 无实例详情、SQL、重启、主从切换或配置修改控件。

### 容器与浏览器

- Docker 中运行前端 Vitest、typecheck 和 production build。
- Docker 中运行 Go 普通测试、race 测试和构建。
- 隔离 Mock Compose smoke 与 Chromium E2E。
- Chromium 验证总览进入 MySQL、列表字段、刷新、无溢出和无破坏性控件。

## 真实环境验收

完整容器验证通过后，使用被 Git 忽略且权限受限的私密配置执行 Nightingale v8.4.1 只读验收：

- MySQL snapshot API 返回结构有效。
- 总览和实例列表非 stale。
- 搜索、筛选和排序可用。
- 页面正确渲染健康及复制语义。

验收过程不得输出 `.env`、Token、Cookie、认证头、真实地址、主机或实例标识、实例数量、指标值或上游响应正文。

真实验收不修改或重启 Nightingale、Categraf 或 MySQL。未获得用户单独授权前，不重建当前 InfraView 服务、不更改部署、不提交、不推送。

## 文档更新

实施时同步更新：

- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/DESIGN.md`
- `docs/PROJECT_STATUS.md`
- `docs/TODO.md`
- `docs/TESTING.md`
- `docs/SECURITY.md`
- `docs/HANDOFF.md`
- `docs/datasources/NIGHTINGALE.md`

所有当前状态、测试结果和后续恢复入口必须与实际实现及验证证据一致。

## 验收标准

- 总览出现可点击的 MySQL 健康摘要卡。
- `/mysql` 提供已确认的紧凑 11 列实例列表。
- 每个实例一行，多复制通道正确聚合。
- 复制延迟边界及缺失语义符合本规格。
- Nightingale snapshot 只发送一次固定批量查询，无实例 N+1。
- Mock 与 Nightingale 使用相同领域契约。
- 所有 API 和 UI 保持严格只读，不暴露任意 PromQL 或上游请求。
- 容器化普通/race/前端/E2E 验证通过。
- 真实 v8.4.1 只读验收通过且没有敏感信息进入输出、日志、夹具或仓库。
