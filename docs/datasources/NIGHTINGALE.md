# Nightingale 数据源接入

状态：真实只读适配器已实现。当前主要开发与真实验证版本为 Nightingale v8.4.1，v9.x 保留协议兼容；当前 8080 服务已使用更新后的私密环境文件重建并通过最终只读验收。`INFRAVIEW_DATA_SOURCE` 接受 `mock` 或 `nightingale`。

最后采集：2026-07-28

## 脱敏验证契约

| 项目 | 已验证结果 |
| --- | --- |
| Nightingale | `v8.4.1` 为当前主要真实验证版本；`v9.x` 保留脱敏契约兼容 |
| API 基础地址 | `https://n9e.example.com`；真实私有地址不进入公开仓库 |
| 部署方式 | 私有部署拓扑不进入公开仓库；适配器只依赖已确认的 Nightingale HTTP API 契约 |
| Categraf | 采集周期和节点规模属于私有部署信息；验证夹具使用虚构 Linux Target |
| 指标存储 | `https://metrics.example.com`，Nightingale 默认 Prometheus 类型数据源 |
| 认证能力 | `HTTP.TokenAuth.Enable=true`；个人 Token 请求头为 `X-User-Token: <token>`，不是 Bearer |
| 当前认证状态 | 真实账号和凭据权限不进入公开仓库；部署时必须使用专用最小只读 Token |
| 主机清单 | 脱敏夹具使用 `fixture-node-01`、`fixture-node-02`、`fixture-node-03` |
| 指标现状 | VictoriaMetrics 已有实际时序数据；未认证 Nightingale Target API 返回 HTTP 401 `unauthorized` |
| 版本与兼容策略 | v8.4.1 真实只读契约预检通过；v9.x 既有脱敏契约回归保留；其他版本在取得证据前不声明支持 |

历史 v9 证据采集使用用户明确授权的开发期只读 SSH；2026-07-28 的 v8.4.1 预检仅调用已确认的只读 HTTP API，检查状态码、Content-Type、envelope 和字段形状，不输出上游响应正文或真实资源信息。两次取证都没有修改或重启 Nightingale 组件。该行为不改变产品边界：InfraView 运行时不得包含 SSH 客户端、远程命令或自动化变更能力。

## 已确认指标映射

| InfraView 字段 | Nightingale / Categraf 指标 | 标签与聚合 | 原始单位 |
| --- | --- | --- | --- |
| 主机 ID / 名称 | Target `ident` | 脱敏夹具使用 `fixture-node-01`、`fixture-node-02`、`fixture-node-03` | 字符串 |
| IP | Target `host_ip`；`system_info` 也含 `host_ip` | 按 `ident` 映射 | IPv4 字符串 |
| CPU 使用率 | `cpu_usage_active` | 每个 `ident` 单序列 | 百分比 |
| 内存使用率 | `mem_used_percent` | 每个 `ident` 单序列 | 百分比 |
| 1 分钟负载 | `system_load1` | 每个 `ident` 单序列 | 标量 |
| IO 忙碌度 | `diskio_io_util` | 验证环境已确认每主机存在多磁盘序列并使用 `max by (ident)` 聚合；生产试运行待 Categraf 升级后复验 | 百分比 |
| 网络发送/接收 | `net_bytes_sent` / `net_bytes_recv` | 累计计数器，需 `rate()`；存在物理、Docker、Calico、veth、bridge、tunnel 接口，必须使用受控可配置过滤规则 | 字节计数器，换算 B/s |
| 运行时间 | `system_uptime` | 每个 `ident` 单序列 | 秒 |
| CPU 核数 | `system_n_cpus`；`machine_cpu_cores` 可作交叉验证 | 每个 `ident` 单序列 | 核 |
| 内存总量 | `mem_total`；`machine_memory_bytes` 可作交叉验证 | 每个 `ident` 单序列 | 字节 |
| 系统元数据 | `system_info` | 含 `ident`、`hostname`、`host_ip`、`kernel_version` | 标签 |

CPU 核数、内存总量和网络接口映射已通过脱敏夹具验证。公开仓库不记录真实节点数量、硬件规格或物理接口名；适配器不能硬编码接口名称，默认排除 `lo`、Docker、veth、Calico、bridge 和 tunnel 接口。

## 已确认只读 API 契约

以下契约来自 Nightingale v8.4.1 当前真实只读预检，以及 v9.x 既有官方源码、内置 API 文档和脱敏响应证据：

- `GET /api/n9e/targets?limit=100&p=1`：分页主机清单，响应 `dat={"list":[...],"total":N}`。共同字段包含 `ident`、`host_ip`、`os`、`agent_version`、`target_up` 和 `cpu_num`；v9.x 提供 `beat_time`，当前 v8.4.1 提供 `update_at`。InfraView 优先使用有效 `beat_time`，缺失或无效时回退有效 `update_at`，都无效时保留零时间。
- `GET /api/n9e/targets/stats`：可见主机总数、存活/离线数和 CPU/内存分桶。
- `GET /api/n9e/busi-groups`：当前用户可见业务组，`dat` 为数组。
- `GET /api/n9e/datasource/brief`：返回已脱敏的数据源摘要；其 `id` 用作查询的 `datasource_id`，`plugin_type` 用作数据源类型。
- `POST /api/n9e/query-instant-batch`：Prometheus/VictoriaMetrics 批量即时只读查询。请求体包含 `datasource_id`、固定 PromQL 和 Unix 秒时间，`dat` 按查询顺序返回 vector 数组。
- `POST /api/n9e/query-range-batch`：Prometheus/VictoriaMetrics 批量历史只读查询。请求体包含 `datasource_id`、固定 PromQL、起止 Unix 秒和步长秒，`dat` 按查询顺序返回 matrix 数组。
- `GET /api/n9e/versions`：当前 v8.4.1 环境的版本接口。`/api/n9e/version` 在该环境不是可依赖的 JSON 版本契约。InfraView 运行时不调用版本接口，也不按版本号分支。

## MySQL 只读映射

- MySQL 是独立领域；Nightingale 共享的同一个 `*Provider` 同时实现 Linux 主机 `datasource.Provider` 与 `mysql.Provider`，复用受限 HTTP 客户端和数据源发现缓存，不会增加任意代理、任意 PromQL 或数据库连接能力。
- 快照仅发送一次 `POST /api/n9e/query-instant-batch`，请求内固定包含 13 条代码内置查询：可用性、版本、运行时间、只读角色、连接数、最大连接数、运行线程、QPS、慢查询速率、Buffer Pool 使用率、复制延迟、复制 IO 线程、复制 SQL 线程。
- Provider 按完整实例身份与复制通道归并批量结果；没有按实例发起的 N+1 查询。批量基数、身份标签、数值范围和上游错误均由脱敏契约测试验证，错误映射为安全的 MySQL 数据源不可用状态。
- 不提供 MySQL 历史、实例详情、数据库写入或运维操作。真实 v8.4.1 MySQL API 与浏览器验收仍须获得单独授权；本地 Mock 和脱敏自动化覆盖不等同于生产验收。
- 本轮已通过无缓存生产镜像、独立全仓 race、E2E 隔离安全测试和一次性 Mock Chromium 验收；该证据只证明本地实现与脱敏夹具，不能替代真实 Nightingale v8.4.1 MySQL 读取契约或生产页面验收。

Nightingale API 响应外层使用 `dat`、`err`、`request_id` 字段；不能只依赖 HTTP 状态，还必须检查 `err`。错误路径会被 SPA 接管并返回 HTML，因此适配器必须同时校验 Content-Type 和 JSON 结构。

## 已确认适配策略

- `ListHosts`：分页调用 `/targets`，以 `ident` 作为稳定 ID 和显示主机名，映射 `host_ip`、`os`、`agent_version`；`target_up=2` 映射 online，`1` 映射 unknown，`0` 映射 offline。状态时间优先使用有效 `beat_time`，否则回退有效 `update_at`。脱敏 v8.4.1/v9.x Target 样本已覆盖该映射，真实资源数量和状态不进入公开仓库。
- CPU 核数优先使用 Target 的 `cpu_num`，`-1` 映射未知；可使用 `system_n_cpus` 交叉验证。
- 内存总容量使用 `mem_total` 批量即时查询，因为 Target 只直接提供 `mem_util`，不提供总字节数。
- `GetCurrentMetrics`：对全部目标使用固定、代码内置的 PromQL 通过一次 `/query-instant-batch` 批量查询，按 `ident` 归并，禁止按主机 N+1。
- `QueryRange`：使用 `/query-range-batch`，只允许领域指标映射生成的固定查询，不接受前端或用户提交 PromQL。
- `QueryAggregateRange`：使用批量范围查询直接在上游按 `ident`/全局聚合，保持总览每指标单序列和最多 600 点约束。
- IO 使用 `max by (ident) (diskio_io_util{ident!=""})`。v8.4.1 验证环境的即时查询已通过；生产试运行环境待 Categraf 升级后重新确认该指标。
- 网络使用 `rate(net_bytes_sent[2m])` / `rate(net_bytes_recv[2m])` 并执行配置化接口过滤；当前环境排除虚拟接口后得到合理 B/s。不得硬编码 任何物理接口名。
- 不使用 `/api/n9e/proxy/:id/*` 实现任意代理，不向 InfraView API 或页面暴露任意数据源 URL、查询表达式或原始 Nightingale 请求体。

真实凭据只存在于被 Git 忽略、权限受限的私有环境文件中，公开仓库不记录其路径、账号或权限。即时查询实际返回 `dat[查询索引][序列]`，其中单点值为 `[Unix秒, 字符串值]`；区间查询对应 `values=[[Unix秒, 字符串值], ...]`。无 Token 和错误 Token 均返回 HTTP 401 与 `text/plain`，因此错误解析不能假设 JSON。仓库已加入完全脱敏的分页、即时、区间、空结果和错误夹具。

## 生产试运行观察（2026-07-28）

- 当前版本在生产试运行中，除 IO 忙碌度显示“暂无数据”外，已有板块指标正常。
- InfraView 当前只使用固定查询 `max by (ident) (diskio_io_util{ident!=""})`；生产环境现有数据可通过 `max by (ident) (rate(diskio_io_time{ident!="",name!~"^(loop|ram|fd).*"}[1m]) / 10)` 派生近似 IO 利用率。
- 两种结果表达相同的“采样窗口内设备忙碌占比”概念，但采样窗口、聚合和计算路径不同，不能假设数值完全相等，也不能在缺少契约证据时自动切换 PromQL。
- 用户决定暂不修改 InfraView，先升级生产 Categraf。升级后至少等待两个采集周期，再只读确认 `diskio_io_util` 是否出现，并观察当前页面 15 秒自动刷新后是否恢复展示；通常不需要主动重启 InfraView。
- 若升级后仍缺失，后续只收集脱敏的指标名、标签形状、单位和查询状态；必须获得用户明确授权，才按 TDD 增加兼容逻辑。

## 禁止猜测

在以下证据齐全前，不编写真实请求路径、参数、字段映射或认证代码：

1. Nightingale 精确版本、部署拓扑和 API 基础地址。
2. 官方或该环境实际采用的认证方式，以及只读最小权限账号。
3. 主机清单、单机身份、当前指标、历史范围和健康状态的真实只读请求/响应。
4. 分页、批量查询、时间范围、步长、时区和时间戳格式。
5. CPU、内存、负载、IO 忙碌度百分比、网络出入速率、运行时间的名称、标签、单位和缺失语义。
6. 401/403、404、限流、超时、部分缺失和服务异常的状态码与响应体。
7. 响应大小边界、证书链和网络出口要求。

## 证据采集要求

- 只执行 GET、数据库只读查询或官方明确的只读指标查询；不创建、修改或删除任何 Nightingale 对象。
- 开发期远端检查必须获得用户逐次明确授权并保持只读；InfraView 产品本身禁止执行远程命令。不得探测写接口或把管理员凭据写入仓库。
- 保存完全脱敏夹具：删除 Token、Cookie、用户名、真实 IP/主机名、租户和业务标签。
- 记录原始单位到 InfraView 规范单位的换算及时间戳依据。
- 记录批量能力，避免按主机 N+1；总览历史必须支持聚合查询语义。

## 目标映射

共享的 `*Provider` 同时实现 Linux 主机 `internal/datasource.Provider`（`Health`、`ListHosts`、`GetHost`、`GetCurrentMetrics`、`QueryRange`、`QueryAggregateRange`）与 MySQL `internal/mysql.Provider`（`MySQLSnapshot`）。前端和服务层不接收 Nightingale 原始请求体、任意 URL 或任意查询表达式。

## 实施顺序

1. [x] 更新版本、认证和端点证据。
2. [x] 编写独立规格与计划。
3. [x] 加入脱敏契约夹具和失败测试。
4. [x] 实现只读适配器、批量查询、超时、大小限制和错误分类。
5. [x] 验证缓存组合、分页、批量、空结果和 100 台规模。
6. [x] 更新配置、部署、安全、测试和项目状态文档。

任何真实凭据都不得进入仓库、测试夹具、错误消息或日志。

## 当前凭据与后续动作

真实部署必须配置专用最小只读 Token；公开仓库不记录 Token 值、绑定账号或权限。InfraView 仍只调用已确认的只读接口，但应用只读边界不能替代上游最小权限。

2026-07-28 已使用更新后的私密配置完成 v8.4.1 上游只读契约预检：认证 profile、Target 分页、默认 Prometheus 类型数据源以及即时/区间批量查询的状态、Content-Type、envelope 和外层形状均符合适配器预期。随后完成生产镜像构建、隔离 Mock smoke/Chromium 4/4、8080 重建和真实应用端只读 smoke；数据源健康且非 stale，总览、主机、单机详情和 1 小时指标范围均通过。容器保持非 root、只读根文件系统和 capabilities 全删。真实资源数量、标识、地址、值和响应正文均未输出或进入仓库。

磁盘容量、磁盘读写历史没有充分契约证据，适配器明确返回空序列。后续仍需落实每个私有环境的专用最小只读 Token，恢复入口见 `docs/HANDOFF.md`。
