# RabbitMQ 集群健康与节点模块设计

日期：2026-08-04

## 目标与范围

首期采用“RabbitMQ 集群健康总览卡 + RabbitMQ 节点列表”。数据源仅为现有测试 Nightingale，指标以 `rabbitmq` 和 `erlang` 开头。InfraView 始终只读，不直连 RabbitMQ Management API，不提供命令、重启、删除、发布、切换、配置或队列操作。

首期支持多集群，展示集群通信、节点状态、资源水位、连接、队列聚合、消息积压、发布/投递速率和运行时间。消息积压仅展示，不参与状态。当前 Nightingale 中 `rabbitmq_queue_*` 序列没有 queue 或 vhost 身份标签，因此首期明确不做队列清单，也不把节点级聚合数据伪装为单队列数据。

## 已验证的数据契约

只读发现已确认：

- `rabbitmq_identity_info` 同时提供采集集群、RabbitMQ 逻辑集群、永久集群身份和 RabbitMQ 节点名称标签。
- `rabbitmq_build_info` 提供 RabbitMQ 与 Erlang 版本标签。
- 节点资源、三类 RabbitMQ 告警、不可达 peer、连接、队列、消息、吞吐和运行时间指标均存在。
- `cluster + instance` 只作为采集关联键；当前与近期身份以集群内部身份与 `rabbitmq_node` 建立具名节点，连接指标可补充发现身份结果暂缺的实例。整批固定结果中同一采集键只有唯一一致的显式 `rabbitmq_node` 时才补全名称；缺失或冲突时名称保持缺失。带 `rabbitmq_node` 的普通指标精确归并，无节点标签且关联不唯一时保持缺失。
- 真实标签值、样本值、地址、身份、数量和上游正文不进入本文档。

Provider 每个快照恰好发送一次 `query-instant-batch`，固定 22 组查询且顺序不可变：

1. `rabbitmq_identity_info`
2. `rabbitmq_build_info`
3. `rabbitmq_erlang_uptime_seconds`
4. `rabbitmq_alarms_memory_used_watermark`
5. `rabbitmq_alarms_free_disk_space_watermark`
6. `rabbitmq_alarms_file_descriptor_limit`
7. `rabbitmq_unreachable_cluster_peers_count`
8. `rabbitmq_process_resident_memory_bytes`
9. `rabbitmq_resident_memory_limit_bytes`
10. `rabbitmq_disk_space_available_bytes`
11. `rabbitmq_disk_space_available_limit_bytes`
12. `rabbitmq_process_open_fds`
13. `rabbitmq_process_max_fds`
14. `rabbitmq_erlang_processes_used`
15. `rabbitmq_erlang_processes_limit`
16. `rabbitmq_connections`
17. `rabbitmq_queues`
18. `rabbitmq_queue_messages`
19. `sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_received_total[5m]))`
20. `sum by (cluster, ident, instance, rabbitmq_node) (rate(rabbitmq_global_messages_delivered_total[5m]))`
21. `tlast_over_time(rabbitmq_identity_info[24h])`
22. `tlast_over_time(rabbitmq_erlang_uptime_seconds[24h])`

API 和前端不能提交指标名、PromQL、URL 或任意查询。禁止按集群、节点或指标发起 N+1 请求。

## 架构与组件边界

- `internal/rabbitmq`：领域类型、Provider 契约、等级与来源枚举、稳定身份及测试契约。
- `internal/adapters/mock`：完全脱敏的多集群 RabbitMQ Mock，覆盖正常、警告、严重和未知。
- `internal/adapters/nightingale`：22 组固定查询、严格解析、身份归并和安全错误。
- `internal/service`：`RabbitMQService`，负责共享快照缓存、样本推进、状态计算、总览、筛选、排序和分页。
- `internal/httpapi`：显式 RabbitMQ View、认证 GET 路由、参数白名单和安全错误映射。
- `web/src/features/rabbitmq`：复用共享列表结构的 RabbitMQ 节点页。
- `web/src/features/overview`：复用共享卡壳的第六张总览卡。

总览和节点列表共享同一份节点/集群快照，不重复查询 Nightingale。缓存周期沿用 15 秒，后台刷新失败时可在既有全局 stale 边界内返回缓存快照。

## 身份、归并与数值规则

集群内部身份优先使用 `rabbitmq_cluster_permanent_id`；缺失时依次回退 `rabbitmq_cluster` 和采集 `cluster`。API 的集群 ID 是内部身份的不可逆哈希，永久集群身份原文永不进入 API。

身份 inventory 节点必须具有 `rabbitmq_node`；连接发现的节点允许名称缺失。具名节点 ID 是“集群内部身份 + RabbitMQ 节点名称”的不可逆哈希，缺名节点使用集群内部身份与仅供内部观察的实例身份生成不可逆 ID。`ident` 不参与名称推测也不进入 API；`instance` 作为已认证页面的实例地址和采集关联键，不能冒充节点名称。

归并遵守以下规则：

- 当前与近期身份优先建立具名节点；连接指标只补充发现未被身份结果覆盖的实例。
- 普通指标先按 `cluster + instance` 找到候选节点；带 `rabbitmq_node` 时精确定位，无节点标签时仅允许唯一候选归并。同一批结果中的唯一一致节点标签可补全连接发现节点的名称，冲突标签不得任意选择。同一稳定节点出现多个合法采集序列时按原始样本时间确定最新值。
- 同一最新时间出现不同身份或不同数值时保持未知，不任意选值。
- 计数和容量拒绝负数、NaN、Inf 与越界；可整型指标不通过不安全浮点中转。
- 运行时间接受 Prometheus 有限非负小数或科学计数文本，检查越界后向下取整为秒。
- 协议和队列类型维度的速率按稳定标签键顺序求和，避免浮点求和顺序造成结果抖动。
- 比例只在分子有限非负且分母有限正数时计算，否则返回缺失。

## 集群与节点状态

集群通信与节点状态严格分离。`rabbitmq_unreachable_cluster_peers_count > 0` 使对应集群通信状态为严重，但不把该集群全部节点批量标为异常。集群通信指标缺失时为未知，明确为零时为正常。

节点等级为 `critical > warning > unknown > normal`，来源固定为：

- `alarm`
- `collection`
- `memory`
- `disk`
- `file_descriptor`
- `erlang_process`
- `normal`
- `unknown`

同一等级来源优先级为：明确告警、采集、磁盘、内存、文件描述符、Erlang 进程。规则如下：

- 内存水位、磁盘水位或文件描述符限制任一明确告警值大于零：严重。
- 常驻内存 / 内存限制达到 80%：警告；达到 90%：严重。
- 已打开 / 最大文件描述符达到 80%：警告；达到 90%：严重。
- Erlang 已用 / 最大进程数达到 80%：警告；达到 90%：严重。
- 磁盘可用空间不高于 RabbitMQ 磁盘限制：严重；高于限制但不足限制的 1.2 倍：警告。
- 当前运行时间原始样本连续 2 个预期 15 秒周期未推进：采集警告；连续 5 个周期未推进：采集严重。
- 首次合法样本只建立本地推进基线，不因原始绝对时间较旧直接误报。
- 进程重启后重新建立本地推进基线；缓存命中不伪造推进，stale 回退继续按本地观察时间老化。
- 核心告警或资源状态数据缺失且没有更高等级问题时，节点为未知，不能伪造正常。
- 连接、队列、消息积压、发布速率和投递速率只展示；缺失显示“暂无数据”，不改变状态。

## 总览卡

RabbitMQ 是总览第六张独立卡，继续使用桌面四轨网格并自然进入第二行。卡片复用 `ModuleStatusCardShell`，独立加载、失败、stale 和重试，不阻塞其他板块。

主汇总固定为“异常节点 x / 总节点”，异常节点为 warning、critical、unknown 之和。右侧分别展示“严重 N”和“警告/未知 N”。下方四类摘要为：

- `集群通信`：存在不可达 peer 的集群及未知集群。
- `资源告警`：触发三类 RabbitMQ 明确告警的节点。
- `资源压力`：内存、磁盘余量、文件描述符或 Erlang 进程达到阈值的节点。
- `采集状态`：采集警告、严重或未知节点。

集群计数与节点计数不混算。模块整体等级取集群通信和节点最高等级。当集群与节点总数同时为零时显示“暂无 RabbitMQ 节点”；只有一侧为零时仍展示另一侧信息。消息积压不进入告警摘要。

## 节点列表与交互

侧边栏在 Elasticsearch 后增加 RabbitMQ。节点页必须复用 `ListPage`、共享刷新控件、搜索/下拉框、表格面板、空状态和分页，不新增 RabbitMQ 专用控件结构或重复样式。

固定 15 列且顺序不可变：

1. 节点名称
2. 所属集群
3. 实例地址
4. 版本
5. 内存使用率
6. 磁盘余量
7. 文件描述符使用率
8. Erlang进程使用率
9. 连接
10. 队列
11. 消息积压
12. 发布速率
13. 投递速率
14. 运行时间
15. 状态

每个单元格只显示一个值并严格单行，不使用 `<br>`、上下副指标或第二行说明。节点名称、所属集群和实例地址过长时显示省略号，完整内容仅通过原生 `title` 提示。运行时间沿用统一格式：同时有天和小时显示“x天 x小时”，整天显示“x天”，不足一天显示“x小时”。

1440×900 下页面和表格均不得横向滚动；更窄视口才允许唯一表格滚动容器横向滚动。不得通过缩小状态徽标可读性、换行或隐藏列达成布局目标。

交互规则：

- 搜索只匹配节点名称和实例地址，300ms 防抖。
- 支持所属集群筛选和正常、警告、严重、未知状态筛选。
- 15 个固定字段均支持服务端升降序，缺失数值始终置后并以稳定节点 ID 收口。
- 每页数量仅允许 20、50、100，默认 20；切换筛选、搜索、排序或每页数量后回到第 1 页。
- `search`、`cluster`、`status`、`sort`、`direction`、`page`、`page_size` 全部写入 URL，刷新与分享后恢复。

## HTTP 契约与安全边界

仅新增两个认证 GET API：

- `GET /api/v1/rabbitmq/overview`
- `GET /api/v1/rabbitmq/nodes`

总览拒绝任何查询参数。节点列表只接受已定义的搜索、集群、状态、15 个排序字段、方向与分页参数；未知、重复或非法参数返回 400。其他 HTTP 方法返回 405。上游不可用且无可用缓存时返回不含上游信息的安全 503。

使用显式 HTTP View；数组始终为 `[]` 而不是 `null`。响应不得包含原始标签、永久集群身份、`ident`、PromQL、数据源 ID、认证信息、上游 URL 或正文。页面和 API 不提供详情、历史、代理、任意查询或任何写操作。

## 错误与降级

- 首次加载显示页面或卡片级加载状态。
- 空集合显示明确空状态。
- 单项指标缺失显示“暂无数据”，不以零代替。
- 后台刷新失败且有缓存时保留旧内容并显示 stale 与刷新失败提示。
- 总览和节点页错误相互隔离，RabbitMQ 失败不阻塞既有五个板块。
- Nightingale 401/403、重定向、非 JSON、错误 envelope、响应过大、超时、结果组数量不匹配或非法数值统一映射为安全不可用错误。
- 错误、日志和测试输出不得包含 Token、Cookie、认证头、Base URL、真实标签值、地址、指标值、固定查询全文或上游正文。

## 测试与验收

所有功能按 TDD 完成：

- 领域/Provider：锁定 22 条查询及顺序、一次 batch、无 N+1、身份回退、跨集群同名节点、当前/近期身份优先、连接补充发现、显式名称标签、最新值、冲突值、数值边界、响应与认证安全。
- Service：锁定集群/节点分离、告警、资源阈值、2/5 freshness、未知、恢复推进、积压只展示、筛选、自然排序、缺失置后、分页与极端页码防溢出。
- HTTP：锁定认证、GET、非 null 数组、参数白名单、400、405、503、stale 和敏感字段排除。
- 前端：锁定第六张卡、四类摘要、双空状态、15 个精确表头、每格单值单行、统一控制栏、URL 恢复、空/错/stale/刷新状态和无破坏性控件。
- Playwright：锁定侧边栏与总览入口、15 列、筛选/排序/分页 URL、1440×900 无页面或表格横向滚动、紧凑等高、身份省略与完整提示、状态颜色以及没有运维控件。

最终实施验证包括前端全测、typecheck、production build、Playwright 静态发现、Go gofmt/vet、全仓普通/race 测试、编译、只读/敏感扫描和 Docker 无缓存生产镜像。只有取得单独授权后，才可原位重建仍连接测试 Nightingale 的现有 8080；不创建其他 InfraView 端口，不连接或探测生产 Nightingale/RabbitMQ，不输出现场值。

## 明确不做

首期不做队列列表/详情、vhost/exchange/connection/channel/consumer 列表、节点详情、历史趋势、拓扑、消息追踪、日志、RabbitMQ Management API 直连、任意 PromQL、告警配置、用户权限、策略、绑定、发布/消费、清理、重启、切换或任何运维操作。
