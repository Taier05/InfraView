# Nightingale 数据源接入

状态：真实只读适配器已实现。当前主要开发与验证版本为 Nightingale v8.4.1，v9.x 仅保留协议兼容；开发 8080 永远只连接测试 Nightingale。本轮已获授权原位重建现有 8080 并完成硬盘 API、Chromium 和跨两个采集周期的脱敏现场验收；未创建其他 InfraView 端口，未连接生产。`INFRAVIEW_DATA_SOURCE` 接受 `mock` 或 `nightingale`。

最后采集：2026-07-29

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
- 快照仅发送一次 `POST /api/n9e/query-instant-batch`，请求内固定包含 16 条代码内置查询：原有可用性、版本、运行时间、只读角色、连接、线程、QPS、慢查询、Buffer Pool 与复制指标，加上显式事务 TPS 和 `tlast_over_time(mysql_up[24h])` 近期实例身份/原始最后样本时间。容量固定使用 `mysql_global_variables_innodb_buffer_pool_size`，TPS 固定归并 `commit|rollback` 的 5 分钟速率。
- `BufferPoolSizeBytes` / `buffer_pool_size_bytes` 为可空字段：只接受非负最新有效样本；缺失、负数、NaN、Inf 或最新同时间戳冲突均保持 `null`，不得伪造成零。新增容量不增加第二次 batch，也不改变复制查询索引的身份规则。
- Provider 按完整实例身份与复制通道归并批量结果；没有按实例发起的 N+1 查询。批量基数、身份标签、数值范围和上游错误均由脱敏契约测试验证，错误映射为安全的 MySQL 数据源不可用状态。
- 实例列表的 `available_labels` 从完整 snapshot 的非空实例名去重并排序，独立于标签、状态、角色、搜索、排序和分页；`label` 仅按去除首尾空白后的实例名精确匹配。服务端 `search` 只匹配实例地址，`sort=instance` 按 IP 与端口自然排序。
- Linux 固定当前指标 batch 额外使用 `tlast_over_time(system_uptime[24h])` 的查询值识别主机原始样本是否推进；Target 时间和其他即时查询的外层求值时间不参与新鲜度。MySQL 使用 `tlast_over_time(mysql_up[24h])` 的查询值识别实例样本推进；Service 按本地最后观察到推进后经过的 2/5 周期判断等级，不直接使用原始时间的绝对年龄。仅近期身份查询命中的实例保留在清单且即时指标为空，显示“采集延迟/采集失联”。超过 24 小时没有 `mysql_up` 样本后才退出近期清单。TPS 只表示显式 `COMMIT` 与 `ROLLBACK`，可能低估默认 autocommit 工作负载。
- 不提供 MySQL 历史、实例详情、数据库写入或运维操作。2026-07-29 已获授权在仅连接测试 Nightingale 的原 8080 完成脱敏 MySQL API 与浏览器验收；该证据不等同于生产验收。
- 历史 MySQL 版本已通过无缓存生产镜像、Go 普通/race、E2E 隔离安全测试和原 8080 Chromium 验收。原始样本新鲜度修复已通过无缓存生产镜像、Go 普通/race 与原有测试 8080 的脱敏登录态现场验收；停采对象在 Linux、MySQL 和两张总览卡均为严重，响应非 stale。
- 样本推进修复已通过无缓存生产镜像、Go 普通/race、前端状态颜色测试及原有测试 8080 脱敏验收；持续采集对象跨越多个缓存刷新和至少两个采集周期保持正常且非 stale。冻结升级由自动化测试验证，本轮未再次停止采集器。

## 主机硬盘 SMART 只读映射

- 硬盘是独立领域与 Service；共享 Nightingale `*Provider` 额外实现 `disk.Provider`，但沿用相同受限 HTTP Client、数据源发现、认证、超时、Content-Type、`dat`/`err` envelope、8 MiB 上限和安全错误清洗。
- 每个硬盘快照精确发送一次 `POST /api/n9e/query-instant-batch`，固定 19 条代码内置即时查询。前 16 组覆盖 SMART 健康、两个温度来源、寿命、通电时间、NVMe 设备警告/备用空间、ATA 属性失败及七类错误计数；第 17 组固定为 `smart_device_command_timeout` 命令超时累计次数，第 18 组固定为 `smart_disk_capacity_bytes`，第 19 组固定为 `tlast_over_time(smart_device_health_ok[24h])` 近期设备 inventory 与原始最后样本时间。不发送硬盘范围查询，不按主机或设备 N+1。
- 原始身份在 Provider 内按同一主机归并，稳定 ID 的优先级固定为 WWN、`serial_no`、设备名，并将主机身份和身份类型一起纳入不可逆哈希。第 19 组同一 `ident + device` 的兼容 24 小时历史序列先归并，`ReportedAt` 取最大的原始样本时间，非空型号不因较新序列缺失而丢失；`serial_no` 或 WWN 只有双方都非空且值不同时才判冲突，任一侧缺失仍允许补齐。第 17 组命令超时与第 18 组容量都只归并到已由 inventory 建立、具有当前健康上报的已知设备；两者只接受有限非负最新值，缺失、非法、负数、较旧值、越界或同一最新时间冲突均保持 `null`。命令超时缺失不得猜测为零；它只进入错误摘要和该列排序，不改变设备状态、`status_source` 或总览告警。旧 inventory `capacity` 标签不读取、不校验也不回退，容量不改变设备发现、`ReportedAt` 或 freshness。辅助序列沿用相同身份规则。Provider 输出、HTTP API 与前端不包含序列号、WWN、原始标签、PromQL、数据源 ID 或上游请求信息。
- `INFRAVIEW_SMART_COLLECTION_INTERVAL` 默认 `60s`，同时作为独立快照 TTL 与 freshness 周期。Service 记录 InfraView 本地最后观察到原始样本时间推进的时刻；业务值变化、即时查询外层时间和原始样本绝对年龄都不参与。默认 120 秒未推进为警告，300 秒为严重，缓存命中不伪造推进，stale 回退继续老化。
- 温度、寿命和错误计数仅展示，不套用 InfraView 通用阈值。API 显式返回六值 `status_source`；最终等级同级时设备来源优先于采集来源，设备来源内部为 `smart_health`、`device_warning`、`attribute_failure`，仅采集等级严格更高时使用 `collection`，其余为 `normal` 或 `unknown`。
- 当前主要验证版本仍为 Nightingale v8.4.1，v9.x 仅保留脱敏协议兼容。19 组命令超时与错误摘要增量已完成脱敏离线契约及完整离线验证；本轮未启动服务或浏览器、未连接上游，也未输出私密环境、真实标识、数量、容量值、指标值或上游正文。任何生产 Nightingale 验证均未执行。

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

共享的 `*Provider` 同时实现 Linux 主机 `internal/datasource.Provider`（`Health`、`ListHosts`、`GetHost`、`GetCurrentMetrics`、`QueryRange`、`QueryAggregateRange`）、MySQL `internal/mysql.Provider`（`MySQLSnapshot`）、硬盘 `internal/disk.Provider`（`SMARTSnapshot`）、Redis `internal/redis.Provider`（`RedisSnapshot`）、Elasticsearch `internal/elasticsearch.Provider`（`ElasticsearchSnapshot`）与 RabbitMQ `internal/rabbitmq.Provider`（`RabbitMQSnapshot`）。前端和服务层不接收 Nightingale 原始请求体、任意 URL 或任意查询表达式。

## 实施顺序

1. [x] 更新版本、认证和端点证据。
2. [x] 编写独立规格与计划。
3. [x] 加入脱敏契约夹具和失败测试。
4. [x] 实现只读适配器、批量查询、超时、大小限制和错误分类。
5. [x] 验证缓存组合、分页、批量、空结果和 100 台规模。
6. [x] 更新配置、部署、安全、测试和项目状态文档。

任何真实凭据都不得进入仓库、测试夹具、错误消息或日志。

## 上游样本时间契约

API 的既有 `meta.collected_at` 统一表示“本次响应内最新有效 Nightingale 样本时间”，来源只能是已解析样本的 `Timestamp`、领域快照的 `ReportedAt` 或健康检查的 `CheckedAt`。多组数据合并时取最大非零值并输出 UTC；缓存命中或 stale 回退仍保留最近一次样本时间，不能使用缓存写入时间、HTTP 响应时间、Service Clock 或浏览器时间替代。响应中没有任何有效样本时省略 `collected_at`。该字段只表达样本时间，是否 stale 仍由 `meta.stale` 独立表达。

## 当前凭据与后续动作

真实部署必须配置专用最小只读 Token；公开仓库不记录 Token 值、绑定账号或权限。InfraView 仍只调用已确认的只读接口，但应用只读边界不能替代上游最小权限。

2026-07-28 已使用更新后的私密配置完成 v8.4.1 上游只读契约预检：认证 profile、Target 分页、默认 Prometheus 类型数据源以及即时/区间批量查询的状态、Content-Type、envelope 和外层形状均符合适配器预期。随后完成生产镜像构建、隔离 Mock smoke/Chromium 4/4、8080 重建和真实应用端只读 smoke；数据源健康且非 stale，总览、主机、单机详情和 1 小时指标范围均通过。容器保持非 root、只读根文件系统和 capabilities 全删。真实资源数量、标识、地址、值和响应正文均未输出或进入仓库。

磁盘容量、磁盘读写历史没有充分契约证据，适配器明确返回空序列。后续仍需落实每个私有环境的专用最小只读 Token，恢复入口见 `docs/HANDOFF.md`。

## Redis Cluster 只读映射

Redis Provider 固定发送 21 组即时查询且只发送一次 batch：可用性、运行时间、Cluster 标记、内存、客户端、阻塞、QPS、命中率、键数量、过期/淘汰/拒绝连接速率、复制连接/链路/通信/同步/最差延迟，以及 24 小时 inventory/角色补充。第 20 组建立实例集合和原始最后样本时间，第 21 组仅在当前角色缺失时补充 `master|slave`。键数量与复制延迟在 PromQL 中先移除会导致多序列的维度；Provider 不向 API 暴露原始身份标签或复制端身份。查询文本、顺序与数量由测试锁定，调用方不能传入 PromQL。

## Elasticsearch 只读映射

Elasticsearch Provider 复用同一个受限 Nightingale Client，恰好发送一次 `POST /api/n9e/query-instant-batch`，固定 26 条查询且顺序不可变：

```promql
elasticsearch_clusterinfo_up
elasticsearch_node_stats_up
elasticsearch_cluster_health_status
elasticsearch_cluster_health_number_of_nodes
elasticsearch_cluster_health_number_of_data_nodes
elasticsearch_cluster_health_active_primary_shards
elasticsearch_cluster_health_active_shards
elasticsearch_cluster_health_relocating_shards
elasticsearch_cluster_health_initializing_shards
elasticsearch_cluster_health_unassigned_shards
elasticsearch_cluster_health_number_of_pending_tasks
elasticsearch_cluster_health_task_max_waiting_in_queue_millis
elasticsearch_nodes_roles
elasticsearch_jvm_memory_used_bytes{area="heap"}
elasticsearch_jvm_memory_max_bytes{area="heap"}
max by (cluster, name, host, ident, instance, es_client_node, es_data_node, es_ingest_node, es_master_node) (100 * (1 - elasticsearch_filesystem_data_available_bytes / elasticsearch_filesystem_data_size_bytes))
elasticsearch_process_cpu_percent
rate(elasticsearch_indices_indexing_index_total[5m])
rate(elasticsearch_indices_search_query_total[5m])
elasticsearch_indices_docs
elasticsearch_indices_store_size_bytes
elasticsearch_jvm_uptime_seconds
max by (cluster, name, host, ident, instance) (elasticsearch_thread_pool_queue_count)
sum by (cluster, name, host, ident, instance) (rate(elasticsearch_thread_pool_rejected_count[5m]))
tlast_over_time(elasticsearch_clusterinfo_up[24h])
tlast_over_time(elasticsearch_jvm_uptime_seconds[24h])
```

第 25 组只建立最近 24 小时集群集合并提供原始最后样本时间，第 26 组只建立最近 24 小时节点集合并提供原始最后样本时间；其他指标不能创建未知身份。集群身份只使用规范化 `cluster`，节点身份只使用 `cluster + name`；`host` 仅用于地址展示，`ident`、`instance`、原始角色标志、PromQL 和数据源信息不进入领域输出或 API。多采集器序列按领域身份去重，最新同时间冲突保持缺失。

26 组返回数量必须精确一致。第 25/26 组 inventory 是身份集合硬依赖，任一为 `null` 时整个快照安全失败；第 1–24 组均为可选观测数据，单组 `null` 只令对应字段暂缺，不能把整个 Elasticsearch 快照判为不可用或 stale。此规则不改变查询文本、顺序、批次数和无 N+1 边界。

集群和节点共用一次快照、无 N+1，但状态与 freshness 分离。集群来源同级优先为 availability、health、collection；节点来源同级优先为 collection、disk、jvm、thread_pool。默认 15 秒周期下连续 2/5 周期未推进为 warning/critical。节点磁盘使用率 85%/90%、JVM 堆使用率 75%/85% 分别为 warning/critical；最近 5 分钟拒绝速率大于 0 为 warning。`elasticsearch_node_stats_up` 仅表达集群级采集状态，集群黄/红不传播为单节点异常。

HTTP 仅暴露受认证的 `GET /api/v1/elasticsearch/overview` 和 `GET /api/v1/elasticsearch/nodes`；前端为总览第五卡和共享模板上的 16 列节点页。首期不直连 Elasticsearch，不做详情、历史、拓扑、任意查询或运维写操作。本轮只使用完全脱敏夹具完成离线验证，未访问真实 Nightingale/Elasticsearch 或现有 8080。

## Java 业务服务只读映射

Java Provider 复用受限 Nightingale Client，每个共享快照恰好一次 `POST /api/n9e/query-instant-batch`，固定 11 组并按以下顺序执行：`service_health_latency_ms`、`service_health_up`、`service_port_up`、`service_process_count`、`service_process_cpu_percent`、`service_process_memory_bytes`、`service_process_memory_percent`、`service_process_port_consistent`、`service_process_start_time_seconds`、`service_process_up`、`tlast_over_time(service_process_up[24h])`。调用方不能传入指标名、PromQL、URL 或任意查询；不按服务、采集机器或指标发起 N+1。

查询 11 建立近期业务实例 inventory 并提供原始样本时间。稳定业务实例身份只使用原始 `name + server_ip`；`ident` 只在 Provider 内部关联、去重与冲突判断，绝不进入领域对外 View、HTTP 响应、页面、fixture 或日志。其他十组只归并到 inventory 中已知实例；最新合法样本优先，同一最新时间冲突保持字段缺失。`service_port_pid` 与 `service_process_pid` 不进入固定查询、领域、API 或页面。

名称映射仅作完整值精确匹配：`tikbee`→`用户端`、`rider`→`骑手端`、`mch`→`商家端`、`saas`→`管理后台端`、`mch_saas`→`商家 PC 端`；未知代码原样展示。健康、端口、进程和端口进程一致性为必需二值字段；缺失或冲突不伪造为零。状态来源为 `health|port|process|consistency|collection|normal|unknown`，等级为 `critical > warning > unknown > normal`。采集 freshness 只按查询 11 原始样本是否推进的本地观察计算：默认 15 秒预期周期，连续 2/5 周期未推进为 warning/critical；首次观察与样本时间回退重新建立基线。CPU、内存、健康延迟、进程数和运行时间仅展示，不设推测阈值。

HTTP 只提供认证的 `GET /api/v1/java/overview` 与 `GET /api/v1/java/services`，后者只接受固定搜索、名称、状态、13 个排序字段、方向和分页参数；其他方法为 405。领域与 Service 内的进程数、内存字节数和运行时间保持 nullable `int64`，后端排序不变；服务列表的 `process_count`、`memory_bytes`、`uptime_seconds` 则固定传输为规范非负十进制字符串或 `null`，上限为 `MaxInt64`，禁止符号、科学计数和非规范前导零。前端使用 BigInt 无损校验与格式化，不能转成 JavaScript `number`。页面固定 13 列，缺失值显示“暂无数据”。本地验证仅使用脱敏 Go/前端夹具和合成 Playwright route fixture；未读取私密环境、未连接真实或生产 Java 服务/Nightingale、未部署 8080，也未运行动态浏览器。任何 push、`main` 合并、8080 重建/部署和动态验收均须新的单独授权。

## RabbitMQ 只读映射

RabbitMQ Provider 复用同一个受限 Nightingale Client，恰好发送一次 `POST /api/n9e/query-instant-batch`，固定 22 条查询且顺序不可变：

```promql
rabbitmq_identity_info
rabbitmq_build_info
rabbitmq_erlang_uptime_seconds
rabbitmq_alarms_memory_used_watermark
rabbitmq_alarms_free_disk_space_watermark
rabbitmq_alarms_file_descriptor_limit
rabbitmq_unreachable_cluster_peers_count
rabbitmq_process_resident_memory_bytes
rabbitmq_resident_memory_limit_bytes
rabbitmq_disk_space_available_bytes
rabbitmq_disk_space_available_limit_bytes
rabbitmq_process_open_fds
rabbitmq_process_max_fds
rabbitmq_erlang_processes_used
rabbitmq_erlang_processes_limit
rabbitmq_connections
rabbitmq_queues
rabbitmq_queue_messages
sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_received_total[5m]))
sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_delivered_total[5m]))
tlast_over_time(rabbitmq_identity_info[24h])
tlast_over_time(rabbitmq_erlang_uptime_seconds[24h])
```

第 1 组当前身份与第 21 组近期身份共同建立具名节点，第 21 组仍要求 `rabbitmq_node`；连接指标可补充发现身份结果暂缺但仍有当前连接序列的实例。`cluster + instance` 只建立采集关联索引。Provider 扫描同一次 22 组结果，只有同一采集键出现唯一一致的显式 `rabbitmq_node` 时才确定性补全名称；缺失或冲突时节点名称保持空值，页面显示“暂无数据”，禁止使用实例地址、`ident` 或名称模式猜测。普通指标带 `rabbitmq_node` 时精确归并；缺少该标签且采集键对应多个候选时保持缺失，不把歧义值复制给所有节点。集群内部身份优先 `rabbitmq_cluster_permanent_id`，缺失时回退 `rabbitmq_cluster`，再回退采集 `cluster`；API 只返回不可逆集群/节点 ID，永久身份原文和 `ident` 不离开 Provider。第 22 组是节点原始样本时间与采集推进的唯一来源。当前 `rabbitmq_queue_*` 序列没有 queue/vhost 身份标签，只能形成节点级聚合，不能支持队列清单或伪装单队列数据。

集群不可达 peer 只影响集群通信，不传播为全部节点异常。节点资源使用率 80%/90% 为 warning/critical；磁盘可用空间不高于限制为 critical，高于限制但不足 1.2 倍为 warning；三类明确 RabbitMQ 告警为 critical；连续 2/5 个默认 15 秒周期未观察到原始 uptime 样本推进为 warning/critical。连接、队列、消息积压、发布/投递速率只展示。HTTP 仅暴露受认证的 `GET /api/v1/rabbitmq/overview` 与 `GET /api/v1/rabbitmq/nodes`，不接受指标名、PromQL、URL 或任意查询。本轮只使用脱敏夹具与合成 E2E route mock，未访问真实 Nightingale/RabbitMQ 或现有 8080。
