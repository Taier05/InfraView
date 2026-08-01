# InfraView Redis Cluster 模块设计

日期：2026-08-01
状态：设计已批准，待实施计划
范围：总览 Redis 卡片与 Redis 实例列表；不包含详情、历史趋势或拓扑图

## 背景与证据

InfraView 当前包含 Linux 主机、主机硬盘和 MySQL 三个只读板块。本轮新增独立 Redis 板块，数据源继续使用现有受限 Nightingale Provider；开发 8080 永远只连接测试 Nightingale。

经用户授权，对测试 Nightingale 完成了只读脱敏契约取证：

- Redis 实例身份标签为 `ident + instance + address`。
- 当前环境启用 Redis Cluster，并存在 `master`、`slave` 两类角色。
- 可用性、运行时间、Cluster、内存、连接、QPS、命中率、键空间、过期/淘汰、拒绝连接和复制指标均存在。
- 主节点侧存在已连接副本数和逐副本复制延迟；从节点侧存在上游链路、最近通信时间和同步状态。
- `redis_up`、`redis_master_link_status`、`redis_slave_read_only` 为二值；`redis_keyspace_hitrate` 为 `0..1` 比例。

取证只输出指标名、标签键和安全类型结论，没有输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP、资源数量、指标值或上游正文，也没有修改或重启服务。

官方语义依据：

- Categraf Redis 插件：<https://github.com/flashcatcloud/categraf/tree/main/inputs/redis>
- Categraf Redis 采集源码：<https://raw.githubusercontent.com/flashcatcloud/categraf/main/inputs/redis/redis.go>
- Categraf Redis 告警规则：<https://raw.githubusercontent.com/flashcatcloud/categraf/main/inputs/redis/alerts.json>

## 目标与非目标

### 目标

- 总览第四个固定槽位增加独立 Redis 卡片。
- 侧边栏增加 `/redis`，提供 Cluster 感知的节点级实例列表。
- 支持单机与 `master/slave`；未知角色安全显示为未知。
- 通过一次固定即时 batch 获取完整快照，无实例 N+1。
- 派生可用性、采集、内存、连接拒绝和复制状态。
- 保持缓存、stale、安全错误、URL 状态和紧凑中文界面基线。

### 非目标

- 不做实例详情、历史趋势、槽位分布、拓扑图或主从配对图。
- 不实现 Sentinel 专属语义、故障转移、主从切换或 Cluster 管理。
- 不连接 Redis，不执行 `redis-cli`、命令、脚本、SSH 或远程操作。
- 不提供任意 PromQL、任意 URL、原始请求体或代理。
- 不连接、切换或探测生产 Nightingale。

## 架构

采用独立 Redis 垂直模块：

- `internal/redis`：领域类型、稳定 ID、只读 Provider 契约和安全领域错误。
- `internal/adapters/mock`：确定性脱敏 Redis Cluster Mock。
- `internal/adapters/nightingale`：共享 `*Provider` 实现 Redis 固定 batch 快照。
- `internal/service`：独立 `RedisService`，负责缓存、样本推进、状态、查询和分页。
- `internal/httpapi`：两条认证 GET API 和明确 View 类型。
- `web/src/features/redis`：Redis 实例页；总览与导航按独立模块接入。

数据流固定为：

```text
Nightingale -> 固定即时 batch -> Redis Provider -> RedisService -> GET API -> React 页面
```

共享 Nightingale HTTP Client、数据源发现、认证、超时、Content-Type、`dat`/`err` envelope、响应大小限制和错误清洗；Redis 不创建第二个上游客户端。

## 领域与身份

Provider 接口：

```go
type Provider interface {
	RedisSnapshot(context.Context) (Snapshot, error)
}
```

稳定实例 ID 对 `ident + instance + address` 做不可逆哈希。领域对象可保留展示所需的 `Address`，但不得包含原始标签集合、数据源 ID、PromQL、Token 或上游请求信息。

角色固定为：

```text
master | slave | unknown
```

角色优先从当前 INFO 指标的 `replica_role` 读取；当前 INFO 缺失时，可由 24 小时角色样本补充。角色冲突或非法值保持 `unknown`，不得任意选择。

## 固定 Nightingale 查询

`RedisSnapshot` 恰好发送一次 `POST /api/n9e/query-instant-batch`，固定 21 条查询：

```promql
redis_up
redis_uptime_in_seconds
redis_cluster_enabled
redis_used_memory
redis_maxmemory
redis_connected_clients
redis_maxclients
redis_blocked_clients
redis_instantaneous_ops_per_sec
redis_keyspace_hitrate
sum by (ident, instance, address, replica_role) (redis_keyspace_keys)
rate(redis_expired_keys[5m])
rate(redis_evicted_keys[5m])
rate(redis_rejected_connections[5m])
redis_connected_slaves
redis_master_link_status
redis_master_last_io_seconds_ago
redis_master_sync_in_progress
max by (ident, instance, address, replica_role) (redis_replication_lag)
tlast_over_time(redis_up[24h])
tlast_over_time(redis_uptime_in_seconds[24h])
```

约束：

- 查询顺序和恰好 21 组结果由直接测试锁定。
- 第 20 组建立最近 24 小时实例集合并提供原始 `redis_up` 最后样本时间。
- 第 21 组只补充历史角色，不负责采集新鲜度。
- 当前 `redis_up` 判断 Redis 可用性；样本推进使用第 20 组原始时间。
- 键数量在查询侧跨 `db` 聚合；API 不暴露 DB 明细。
- 复制延迟在查询侧按主节点身份取最差副本；API 不暴露 `replica_ip`、`replica_port` 或 `replica_id`。
- 不增加第二次 batch，不按实例查询，不接受前端或配置传入 PromQL。

## 数值与归并

- 当前和辅助序列按 `ident + instance + address` 归并。
- 未知辅助序列不得创建实例；身份冲突使快照安全失败。
- 同一字段选择最新有效样本；同一最新时间不同有效值保持 `null`。
- 缺失、负数（仅非负字段）、NaN、Inf、越界或解析失败保持 `null`，不得伪造成零。
- `redis_up`、Cluster、链路和同步字段只接受合法二值。
- 命中率只接受 `0..1`；页面转换为百分比。
- 内存单位为字节；`maxmemory <= 0` 表示未配置有效上限，使用率为 `null` 且不产生内存告警。
- 复制 `lag` 单位为秒，表达副本最近 ACK/通信延迟，不宣称为严格的数据落后时间。

## 缓存与采集新鲜度

- Redis 快照 TTL 复用 `INFRAVIEW_EXPECTED_COLLECTION_INTERVAL`，默认 `15s`。
- 使用独立并发安全 `freshnessTracker`。
- 首次非零样本建立正常基线；原始时间推进或回退时重置本地推进时间。
- 连续 2 个周期未推进为 warning，5 个周期为 critical。
- 只在 Provider loader 成功时观察样本；缓存命中不伪造推进，stale 回退期间继续老化。
- 进程重启后重新建立本地基线。

## 状态模型

最终等级：

```text
critical > warning > unknown > normal
```

状态来源固定为：

```text
availability | replication | memory | connection | collection | normal | unknown
```

同级来源优先级：可用性、复制、内存、连接、采集。

规则：

- `redis_up=0` 为 critical；缺失为 unknown。
- 采集 2/5 周期未推进分别为 warning/critical。
- Cluster master 的已连接副本数为零时为 critical；非 Cluster 单机不因此告警。
- slave 的 `master_link_status=0` 为 critical；链路字段缺失为 unknown。
- 主节点最差复制延迟 `5–<30s` 为 warning，`>=30s` 为 critical。
- 配置有效 `maxmemory` 时，内存使用率 `85–<95%` 为 warning，`>=95%` 为 critical。
- 最近 5 分钟拒绝连接速率大于零为 warning。
- `maxmemory > 0` 但已用内存缺失时，内存等级为 unknown；`maxmemory <= 0` 不产生告警。
- 拒绝连接等仅用于告警的可选指标缺失时保持字段为空，不单独把实例升级为 unknown。
- QPS、命中率、键数量、阻塞连接、过期/淘汰速率、运行时间、最近通信时间和同步进行中不直接改变状态；其中过期/淘汰速率在后续 11 列页面中不再展示，但保留数据契约。

## Service 查询与 API

API：

- `GET /api/v1/redis/overview`
- `GET /api/v1/redis/instances`

Overview 返回状态计数、角色分布、受影响实例及四类摘要：可用性、内存、连接、复制。采集异常归入可用性摘要。

Instances 参数：

- `search`：只匹配实例地址。
- `role`：`master|slave|unknown`。
- `status`：`normal|warning|critical|unknown`。
- `sort`：`instance|memory|connections|qps|keys|evicted|replication_lag|uptime|status`。
- `order`：`asc|desc`。
- 默认按实例地址的 IP/端口自然升序；数值缺失项在升降序中始终置后，并以稳定 ID 收口。
- `QPS/命中率` 按 QPS 排序。后端 `sort=evicted` 为既有契约兼容保留，后续 11 列页面不再提供过期/淘汰列或对应排序按钮。
- `page`、`page_size`。
- `page_size` 仅允许 `20|50|100`。

列表初始实现为十列；2026-08-01 后续列调整以 `2026-08-01-observability-module-template-and-redis-columns-design.md` 为当前覆盖规格：

1. 实例地址
2. 角色
3. 内存上限
4. 内存使用率
5. 连接
6. QPS/命中率
7. key总数
8. 复制链路
9. 延迟
10. 运行时间
11. 状态

页面不再展示过期/淘汰列；固定查询和 API 字段保留。主节点复制链路显示 `—`，从节点显示上游链路状态；延迟只展示真实 `redis_replication_lag`，不以最近通信时间回退。运行时间复用主机/MySQL 的天/小时格式。API 返回结构化字段、`status`、`status_source` 和 `collection_level`，前端不猜测状态来源。

所有接口只允许 GET；其他方法返回 405。数据源不可用映射为安全、可重试的 503，不泄露上游细节。

## 前端

- 总览桌面顺序为 Linux 主机、主机硬盘、MySQL、Redis；Redis 占第四槽位。
- 侧边栏顺序为总览、主机、硬盘、MySQL、Redis。
- Redis 卡片和页面独立处理 loading、error、stale、刷新与重试。
- 实例页复用主机/硬盘/MySQL 的带标签搜索、筛选、每页数量和刷新状态控制栏。
- 角色显示“主节点 / 从节点 / 未知”。
- 状态文案依据 `status_source` 显示节点离线、复制异常、内存风险、连接拒绝或采集异常。
- 桌面优先在 72rem 表格内完整展示；容器不足时只允许表格区域横向滚动，文档本身不得横向溢出。复合指标允许自然换行；不存在故障转移、切换、重启或其他破坏性控件。

## 测试与验收

严格 RED→GREEN：

- 领域/Mock：稳定 ID、角色、可空字段、深拷贝和 Cluster 样本。
- Nightingale：固定 21 查询、一次 batch、无 N+1、角色补充、身份/数值冲突、安全错误和脱敏夹具。
- Service：缓存/stale、样本推进、阈值边界、角色特定复制规则、状态来源、搜索/筛选/排序/分页。
- HTTP：两个 GET 契约、参数白名单、405、安全 503、stale 和敏感字段排除。
- 前端：总览第四槽位、当前 11 列、共享模板、格式化、URL 状态、状态来源、loading/error/stale 和无破坏性控件。
- 浏览器规格：导航、总览、列表、1440×900 几何和零横向溢出。

完整验证使用 Docker：Go 普通/race/编译、前端 Vitest/typecheck/build、Playwright 静态发现和无缓存镜像构建。不得运行会创建额外 InfraView 端口的 E2E。用户已长期授权每次已授权修复验证通过后自动原位重建同一测试 8080；提交和推送仍分别等待明确授权。

## 文档同步

实施时同步 README、架构、配置、设计、安全、测试、Nightingale、项目状态、TODO、HANDOFF、本规格和实施计划。历史状态必须明确标注，不能覆盖当前有意保留的 `docs/HANDOFF.md` 差异。

## 验收标准

- Redis 卡片与 `/redis` 当前 11 列列表符合覆盖规格批准范围。
- Cluster master/slave、可用性、采集、内存、连接拒绝和复制状态符合本设计。
- 数据来自一次固定 21 查询 batch，无 N+1、无任意 PromQL。
- API/UI 不暴露原始复制身份、数据源信息或上游内容。
- Docker 全量验证通过，产品继续严格只读。

## 当前实施状态（2026-08-01）

本规格的领域、Mock、固定 21 查询 Nightingale Provider、RedisService、两个 GET API、总览第四卡、侧边栏和初始十列列表均已实现。首次原位重建现有 8080 后，现场发现总览角色字段大小写契约错误及列表控制栏、列名和长指标可读性问题；上一版修复曾通过前端 9 文件/108 项、Go 全仓普通/race/编译、typecheck/build 与 Playwright 2 文件/14 项静态发现，并经授权原位重建现有 8080。

当前未提交工作区已按覆盖规格完成 11 列与共享模板调整：共享列表页、共享总览卡壳及 Redis 首个接入均按 RED→GREEN 实现；Redis 十一列语义测试 7/7，全量前端 11 文件/112 项、typecheck/build、Playwright 14 项静态发现、Go 格式/普通/race/编译和无缓存生产镜像均通过。后端固定 21 查询、API、状态逻辑和兼容排序未改变。十一列/共享模板及运行时间天/小时纠错均已原位重建至现有测试 8080。提交和推送仍分别等待明确授权，生产 Nightingale 永久禁止。
