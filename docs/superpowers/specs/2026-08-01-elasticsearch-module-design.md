# InfraView Elasticsearch 首期模块设计

日期：2026-08-01
状态：设计已批准，待实施计划
范围：总览 Elasticsearch 集群健康卡与 Elasticsearch 节点列表

## 背景与证据

InfraView 当前包含 Linux 主机、主机硬盘、MySQL 与 Redis 四个只读板块。本轮新增独立 Elasticsearch 板块，数据源继续使用现有受限 Nightingale Provider；开发 8080 永远只连接测试 Nightingale。

经用户明确授权，对测试 Nightingale 完成只读脱敏契约取证：

- `elasticsearch_*` 指标同时包含集群健康与节点指标。
- 集群身份标签为 `cluster`；节点指标包含 `cluster + name + host`。
- `ident` 与 `instance` 标识采集来源，不等同于 Elasticsearch 节点地址。
- 集群健康、节点统计采集、分片、任务、角色、JVM、文件系统、CPU、索引、搜索、文档、存储、运行时间与线程池指标存在。
- 拟定的 26 条固定 MetricsQL 表达式均在测试 Nightingale 返回合法序列。
- `elasticsearch_node_stats_up` 没有节点名称标签，只能表示集群级节点统计采集器状态，不能表示单个节点可用性。

取证只输出指标名、标签键、表达式存在性和安全类型结论，没有输出私密环境文件、Token、Cookie、认证头、Base URL、真实集群/节点标识、地址、资源数量、容量、指标值或上游正文；一次性探针容器不发布端口、使用只读根文件系统、删除全部 capabilities，并在结束后自动删除。取证没有修改或重启任何服务，也没有连接、切换或探测生产 Nightingale。

公开语义依据：

- Categraf Elasticsearch 插件：<https://github.com/flashcatcloud/categraf/tree/main/inputs/elasticsearch>
- Elasticsearch 磁盘水位：<https://www.elastic.co/docs/troubleshoot/elasticsearch/fix-watermark-errors>
- Elasticsearch JVM 内存压力：<https://www.elastic.co/guide/en/elasticsearch/reference/current/high-jvm-memory-pressure.html>

## 目标与非目标

### 目标

- 总览增加 Elasticsearch 集群健康卡。
- 侧边栏增加 `/elasticsearch`，提供支持多集群的节点列表。
- 一次固定即时 batch 同时获取集群与节点快照，无集群或节点 N+1。
- 分别计算集群状态与节点状态，集群异常不批量污染节点状态。
- 支持集群、角色、集群健康、节点状态、搜索、排序与服务端分页。
- 强制复用 Redis 修复后建立的共享总览卡和列表页模板。
- 保持缓存、stale、样本推进新鲜度、安全错误与紧凑中文界面基线。

### 非目标

- 不做索引列表、索引详情、节点详情、历史趋势或拓扑图。
- 不做分片分布详情、慢查询、日志或追踪。
- 不直连 Elasticsearch，不执行 Elasticsearch API、命令、脚本、SSH 或远程操作。
- 不提供任意 PromQL、任意指标名、任意 URL、原始请求体或代理。
- 不修改 Elasticsearch、Nightingale、Categraf 或任何受监控系统。
- 不连接、切换或探测生产 Nightingale。

## 架构与数据流

采用独立业务模块与统一前端模板：

- `internal/elasticsearch`：集群、节点、角色、稳定 ID、快照和只读 Provider 契约。
- `internal/adapters/mock`：确定性、完全脱敏的多集群和多角色 Mock。
- `internal/adapters/nightingale`：共享 `*Provider` 实现固定 Elasticsearch batch 快照。
- `internal/service`：独立 `ElasticsearchService`，共享一份快照缓存并计算集群与节点状态。
- `internal/httpapi`：两个认证只读 GET API 和显式 View 类型。
- `web/src/features/elasticsearch`：总览卡与节点列表业务组件。
- `ModuleStatusCardShell`：统一总览卡结构。
- `ListPage`：统一搜索、筛选、每页数量、刷新、表格、空状态和分页结构。

数据流固定为：

```text
测试 Nightingale
  -> 一次固定 26 查询即时 batch
  -> Elasticsearch Provider
  -> 同一份集群/节点快照
  -> ElasticsearchService
  -> 两个认证只读 GET API
  -> 总览卡与节点列表
```

Elasticsearch 不创建第二个 Nightingale HTTP Client；继续复用数据源发现、认证、超时、状态码、Content-Type、`dat`/`err` envelope、响应大小限制和安全错误清洗。

## 领域与身份

Provider 接口：

```go
type Provider interface {
	ElasticsearchSnapshot(context.Context) (Snapshot, error)
}
```

快照同时包含 `Clusters` 与 `Nodes`。

- 集群领域身份为规范化后的 `cluster` 标签，稳定 ID 对该身份做不可逆哈希。
- 节点领域身份为规范化后的 `cluster + name`，稳定 ID 对二者做不可逆哈希。
- `host` 只作为节点地址展示字段，不参与稳定身份。
- `ident`、`instance`、`url`、`cluster_uuid`、原始角色标志与完整指标标签不得进入 API。
- `instance` 是 exporter/采集目标，禁止误当成 Elasticsearch 节点地址。
- 多角色节点保存去重并按固定优先级排序的角色集合；页面单行展示该角色集合。

历史 inventory 建立领域对象集合；当前或辅助指标不得单独创建未知身份。缺少有效集群名或节点名的序列忽略。相同领域身份的地址在同一最新时间冲突时保持地址未知，不能任意选择或暴露采集来源。

## 固定 Nightingale 查询

`ElasticsearchSnapshot` 恰好发送一次 `POST /api/n9e/query-instant-batch`，固定 26 条查询：

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

约束：

- 查询顺序、查询文本与恰好 26 组结果由直接测试锁定。
- 第 25 组建立最近 24 小时集群集合并提供集群原始最后样本时间。
- 第 26 组建立最近 24 小时节点集合并提供节点原始最后样本时间。
- 只允许代码内固定查询；API、前端和配置不能传入指标名或 PromQL。
- 不增加第二次 batch，不按集群、节点、角色或筛选条件查询。

## 数值与归并

- 每个字段选择最新有效样本；同一最新时间出现不同有效值时该字段保持 `null`。
- 缺失、负数（仅非负字段）、NaN、Inf、越界、解析失败或非法枚举保持 `null`。
- `clusterinfo_up`、`node_stats_up` 只接受合法二值。
- 集群健康优先使用合法 `color=green|yellow|red` 与对应有效样本；非法或冲突保持未知。
- 节点角色只接受代码白名单内的非空角色；值未启用的角色序列不纳入角色集合。
- JVM 堆使用率为有效 `used / max * 100`；上限非正或任一字段缺失时保持 `null`。
- 磁盘使用率由查询计算各数据路径使用率，并取节点最高有效值；无效分母或越界值忽略。
- 索引、搜索和拒绝速率使用 5 分钟窗口，负数或非法结果忽略。
- 集群文档量和存储量由有效节点值求和；文档量语义固定为“含副本的物理文档量”。
- 集群索引和搜索速率由有效节点速率求和。
- 数值求和必须检查整数或浮点溢出，不能静默回绕。

## 缓存与采集新鲜度

- Elasticsearch 快照 TTL 复用 `INFRAVIEW_EXPECTED_COLLECTION_INTERVAL`，默认 `15s`。
- 集群与节点分别使用独立并发安全的 freshness tracker，但共享同一 Provider 快照缓存。
- 首次非零原始时间建立正常基线；原始时间推进或回退时重置本地推进时间。
- 连续 2 个预期采集周期未推进为 warning，5 个周期为 critical。
- 只在 Provider loader 成功时观察样本；缓存命中不伪造推进。
- stale 回退期间继续基于本地时间老化；进程重启后重新建立基线。
- `elasticsearch_node_stats_up` 是集群级节点统计采集状态，不得用于单节点离线判断。
- 单节点没有显式 `up` 时只显示“采集延迟/采集失联”，不得无证据显示“节点离线”。

## 状态模型

等级固定为：

```text
critical > warning > unknown > normal
```

### 集群状态

来源固定为：

```text
availability | health | collection | normal | unknown
```

同级来源优先级：availability、health、collection。

- `clusterinfo_up=0` 为 critical；缺失为 unknown。
- 健康颜色 red 为 critical，yellow 为 warning，green 为 normal，缺失或冲突为 unknown。
- `node_stats_up=0` 表示集群节点统计采集失败，集群 collection 为 critical。
- 集群样本 2/5 周期未推进分别为 collection warning/critical。
- 未分配分片、待处理任务和最长等待时间只用于解释健康状态，不重复提升等级。

### 节点状态

来源固定为：

```text
collection | disk | jvm | thread_pool | normal | unknown
```

同级来源优先级：collection、disk、jvm、thread_pool。

- 节点样本 2/5 周期未推进分别为 collection warning/critical。
- 磁盘使用率 `>=85%` 为 warning，`>=90%` 为 critical。
- JVM 堆使用率 `>=75%` 为 warning，`>=85%` 为 critical。
- 最近 5 分钟线程池拒绝速率大于 0 为 warning，不设置固定 critical 阈值。
- CPU、线程池队列、索引/搜索速率、文档量、存储量和运行时间只展示。
- 数据节点缺失有效磁盘指标，或任意节点缺失有效 JVM 堆指标时，在没有更高等级问题的情况下为 unknown。
- 非数据节点缺少数据路径指标不提升为 unknown。
- 集群黄/红不传播为全部节点异常；节点列表独立展示集群健康。

模块总览等级取全部集群与节点中的最高等级，但异常集群与异常节点分别汇总，不能混成一个含糊数字。

## Service、API 与查询

API：

- `GET /api/v1/elasticsearch/overview`
- `GET /api/v1/elasticsearch/nodes`

Overview 返回模块状态、集群与节点各等级汇总，以及集群健康、节点资源、未分配分片、请求拒绝四类摘要。API 返回结构化字段、`stale` 和快照更新时间；不返回原始领域对象或标签。

Nodes 参数：

- `search`：只匹配节点名称或节点地址。
- `cluster`：所属集群精确匹配。
- `role`：包含指定角色即匹配。
- `cluster_health`：`green|yellow|red|unknown`。
- `status`：`normal|warning|critical|unknown`。
- `sort`：`node|cluster|address|role|cluster_health|heap|disk|cpu|index_rate|search_rate|documents|store|thread_queue|rejected_rate|uptime|status`。
- `order`：`asc|desc`。
- `page`：正整数，并在计算偏移前检查 `int` 溢出。
- `page_size`：只允许 `20|50|100`。

默认按所属集群与节点名称自然升序；数值缺失项在升降序中始终置后，并以稳定 ID 收口。角色集合按固定角色优先级排序。集群健康与节点状态按明确等级排序。

列表响应返回完整快照中的可选集群和可选角色；选项不受搜索、筛选、排序或分页影响。所有集合为空时返回 `[]`，不能返回 `null`。

所有接口只允许 GET；非法参数返回安全 400，其他方法返回 405。首次加载且上游不可用时返回安全 503；存在缓存时允许 stale 回退。错误不得泄露 URL、Token、认证头、响应正文或真实采集身份。

## 前端与固定 16 列

总览桌面增加 Elasticsearch 卡，直接复用 `ModuleStatusCardShell`。四个固定槽位为：

1. 集群健康
2. 节点资源
3. 未分配分片
4. 请求拒绝

侧边栏增加 `/elasticsearch`。节点页直接复用 `ListPage`，控制栏固定包含搜索、所属集群、节点角色、集群健康、节点状态、每页数量和现有刷新控制。

节点表固定 16 列：

1. 节点名称
2. 所属集群
3. 节点地址
4. 节点角色
5. 集群健康
6. JVM堆使用率
7. 磁盘使用率
8. CPU使用率
9. 索引速率
10. 搜索速率
11. 文档数
12. 存储大小
13. 线程池队列
14. 拒绝速率
15. 运行时间
16. 状态

展示硬规则：

- 每个单元格只显示一个值，不使用上下两行、`<br>` 或副指标堆叠。
- 所有数据单元格保持单行和现有紧凑行高。
- Elasticsearch 数据单元格禁止使用省略号截断。
- 多角色去重排序后作为一个单行文本值展示。
- 保持现有字号和密度，不为塞入 16 列缩小字体。
- 表格设置足够的最小宽度；空间不足时只允许表格区域横向滚动，页面本身不得横向溢出。
- 首次部署后根据真实 8080 浏览器验收调整列宽或删除低价值只展示列；不得以复合列或增加行高解决宽度问题。
- 不增加详情、历史、重启、切换、删除、配置或其他运维控件。

前端筛选、排序、页码和每页数量写入 URL；搜索使用 300ms 防抖。loading、error、stale、后台刷新失败和空列表继续复用既有组件与 TanStack Query 状态。

## 响应契约防回归

- 两个 API 使用现有统一 JSON envelope。
- 后端显式 View 类型与前端 TypeScript 类型逐字段对应。
- 空集合一律编码为 `[]`。
- Handler 契约测试锁定字段名、分页结构、400、405、503、stale 和敏感字段排除。
- 前端测试使用与 Handler 相同形状的完整夹具，覆盖 loading、error、stale、empty 与正常响应。
- 禁止前端依赖未经后端返回的字段，避免再次出现“服务器响应格式无效”。

## 测试与验收

严格执行 RED->GREEN：

- 领域/Mock：稳定 ID、多集群、多角色、可空字段、深拷贝与确定性脱敏数据。
- Nightingale：固定 26 查询、一次 batch、无 N+1、inventory、身份归并、冲突、非法数值、安全错误与完全脱敏夹具。
- Service：缓存、singleflight、stale、2/5 周期新鲜度、阈值边界、状态来源、筛选、排序、分页与极端页码。
- HTTP：两个 GET 契约、统一 envelope、空数组、400、405、503 和敏感字段排除。
- 前端：共享模板、总览四槽位、16 列、单值单行、URL 状态、搜索防抖、筛选、排序、分页、刷新和全部查询状态。
- 浏览器规格：导航、总览、节点页、1440x900 页面无横向溢出、表格内部滚动、紧凑行高、完整值和无破坏性控件。
- 全仓回归：Linux 主机、主机硬盘、MySQL 与 Redis 的行为和样式不得改变。

针对 Redis 初版问题建立阻断测试：

- Elasticsearch 必须使用 `ListPage` 与 `ModuleStatusCardShell`。
- 不得新增模块专用搜索框、下拉框、刷新区或分页结构。
- 16 个单元格分别只有一个值，禁止复合两行内容。
- Elasticsearch 数据单元格禁止省略号规则。
- API 夹具与后端实际响应结构一致。
- 1440x900 下控制栏样式一致、表头完整、行高紧凑、页面无横向溢出。

完整验证使用 Docker：前端全量 Vitest、typecheck、production build、Playwright 静态发现、Go 全仓普通/race 测试与编译、无缓存生产镜像和只读安全扫描。不得运行会创建额外 InfraView 端口的 E2E。

## 文档与交付边界

实施时同步 README、架构、设计、安全、测试、Nightingale、项目状态、TODO、HANDOFF、本规格和实施计划。历史状态必须与当前状态分开标注。

- 本规格提交不授权实现代码、推送或重建服务。
- 实现、现有 8080 重建、提交和推送继续遵守各自授权边界。
- 用户对“已授权修复通过验证后原位重建现有测试 8080”的长期授权不扩展到生产 Nightingale、额外端口、其他服务、提交或推送。
- InfraView 始终只读；任何生产 Nightingale 验证永久禁止。

## 验收标准

- 总览卡与节点页满足多集群首期范围。
- 集群健康和节点状态严格分离，状态阈值与来源符合本设计。
- 数据来自一次固定 26 查询 batch，无 N+1、无任意 PromQL。
- 页面强制复用共享模板，不重现 Redis 初版的控制栏和响应契约问题。
- 节点表 16 列均为单值单行，完整显示；宽度不足只在表格内部滚动。
- API/UI 不暴露原始采集身份、数据源信息或上游内容。
- Docker 全量验证通过，既有模块无回归，产品继续严格只读。
