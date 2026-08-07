# Java 业务服务状态模块设计

日期：2026-08-05

## 目标与范围

首期采用“Java 服务总览卡 + Java 业务服务列表”。数据源仅为现有测试 Nightingale，使用已验证的 `service_` 指标。InfraView 始终只读，不直连 Java 应用，不提供命令、重启、发布、配置、诊断、日志或其他运维操作。

模块名称固定为：

- 侧边栏：`Java 服务`
- 总览卡：`Java 服务`
- 页面标题：`Java 业务服务`
- 页面路由：`/java`
- 代码领域：`internal/javaapp`

首期展示业务端、服务地址、健康检查、健康延迟、端口状态、进程状态、进程数、端口进程一致性、CPU 使用率、内存占用、内存使用率、运行时间和综合状态。PID、历史趋势、详情页、JVM 专属指标和未经确认的资源阈值不在首期范围内。

## 已验证的数据契约

只读发现已确认以下指标存在：

- `service_health_latency_ms`
- `service_health_up`
- `service_port_pid`
- `service_port_up`
- `service_process_count`
- `service_process_cpu_percent`
- `service_process_memory_bytes`
- `service_process_memory_percent`
- `service_process_pid`
- `service_process_port_consistent`
- `service_process_start_time_seconds`
- `service_process_up`

这些指标的身份标签固定为 `ident`、`name`、`server_ip`。`ident` 表示采集机器，不是业务服务身份，不进入页面或 API，也不能作为服务名称展示。`name + server_ip` 是业务实例身份。

已验证的数值语义如下：

- `service_health_up`、`service_port_up`、`service_process_port_consistent`、`service_process_up` 是二值指标，只接受 0 或 1。
- `service_process_count`、`service_process_memory_bytes`、`service_process_start_time_seconds` 是非负整数。
- `service_process_cpu_percent`、`service_process_memory_percent` 只接受 0 到 100 的有限数值。
- `service_health_latency_ms` 只接受有限非负数值。
- `tlast_over_time(service_process_up[24h])` 可提供近期服务 inventory、原始样本时间和采集推进依据。

PID 不展示，因此 `service_port_pid` 和 `service_process_pid` 不进入固定查询，也不进入领域类型或 API。

Provider 每个快照恰好发送一次 `query-instant-batch`，固定 11 组查询且顺序不可变：

1. `service_health_latency_ms`
2. `service_health_up`
3. `service_port_up`
4. `service_process_count`
5. `service_process_cpu_percent`
6. `service_process_memory_bytes`
7. `service_process_memory_percent`
8. `service_process_port_consistent`
9. `service_process_start_time_seconds`
10. `service_process_up`
11. `tlast_over_time(service_process_up[24h])`

API 和前端不能提交指标名、PromQL、URL 或任意查询。禁止按服务、采集机器或指标发起 N+1 请求。真实标签值、地址、身份、数量和样本值不进入本文档、日志或测试输出。

## 架构与组件边界

- `internal/javaapp`：领域类型、Provider 契约、等级与来源枚举、稳定身份及测试契约。
- `internal/adapters/mock`：完全脱敏的 Java 服务 Mock，覆盖正常、警告、严重和未知。
- `internal/adapters/nightingale`：11 组固定查询、严格解析、身份归并和安全错误。
- `internal/service`：`JavaService`，负责共享快照缓存、样本推进、状态计算、总览、筛选、排序和分页。
- `internal/httpapi`：显式 Java View、认证 GET 路由、参数白名单和安全错误映射。
- `web/src/features/java`：复用共享列表结构的 Java 业务服务页。
- `web/src/features/overview`：复用共享卡壳的第七张总览卡。

总览和服务列表共享同一份快照，不重复查询 Nightingale。缓存周期沿用 15 秒，后台刷新失败时可在既有全局 stale 边界内返回缓存快照。Java 模块的加载、失败、空数据和 stale 状态与其他模块隔离。

## 身份、名称与归并规则

业务实例的稳定内部键是原始 `name + server_ip`。API 的服务 ID 使用带明确分隔的内部键生成不可逆哈希，原始键不作为 ID 输出。`ident` 仅供 Provider 在单批结果中关联、去重和识别冲突，不进入领域对外 View、HTTP 响应或页面。

多个 `ident` 采集到相同 `name + server_ip` 时合并为一个业务实例：

- 查询 11 的近期结果建立 inventory；其他 10 组指标只归并到 inventory 中已知的业务实例，不额外创造身份。
- 同一字段存在多个合法序列时，原始样本时间较新的值生效。
- 同一最新时间出现不同值时，该字段保持缺失，不任意选择。
- 身份标签缺失、为空或不完整的序列不得建立业务实例。
- 不使用 `ident`、地址内容或其他标签猜测 `name`。

`name` 的完整精确映射固定为：

- `tikbee` → `用户端`
- `rider` → `骑手端`
- `mch` → `商家端`
- `saas` → `管理后台端`
- `mch_saas` → `商家 PC 端`

匹配是完整值精确匹配，不按前缀或包含关系匹配。当前已知值按中文业务端展示；未来出现未知代码时展示原始代码，以免错误归类。原始 `name` 仍用于稳定身份、搜索和 API 结构化字段。

## 数值与展示规则

- 所有数值拒绝 NaN、Inf、负值和越界值；二值指标拒绝 0、1 以外的值。
- 非负整数，特别是内存字节数，必须从原始 JSON 数值文本安全解析，不通过可能丢失精度的浮点中转。
- CPU 和内存使用率按百分比统一格式化；它们只展示，不参与状态。
- 健康延迟按毫秒展示，只展示，不设置推测阈值。
- 内存占用使用 IEC 单位格式化。
- 运行时间由当前时间减去 `service_process_start_time_seconds` 计算；缺失、非法或晚于当前时间时显示“暂无数据”。
- 进程数只展示，不参与状态。
- 字段缺失或冲突统一显示“暂无数据”，不能用零代替。

## 服务状态

综合等级固定为 `critical > warning > unknown > normal`，来源固定为：

- `health`
- `port`
- `process`
- `consistency`
- `collection`
- `normal`
- `unknown`

同一等级的来源优先级为：健康检查、端口、进程、端口进程一致性、采集。规则如下：

- `service_health_up = 0`：严重，来源为健康检查。
- `service_port_up = 0`：严重，来源为端口。
- `service_process_up = 0`：严重，来源为进程。
- `service_process_port_consistent = 0`：严重，来源为端口进程一致性。
- 上述任一必需二值字段缺失或冲突，且没有更高等级问题时：未知。
- 查询 11 的原始样本时间连续 2 个预期 15 秒周期未推进：采集警告。
- 连续 5 个预期周期未推进：采集严重。
- 首次合法样本只建立本地推进基线，不因绝对样本时间较旧直接误报。
- 样本时间正常推进时恢复正常观察；时间回退时重新建立基线，不根据回退值猜测故障。
- 缓存命中不能伪造样本推进；stale 回退期间仍按本地观察时间累计未推进周期。
- CPU、内存、健康延迟、进程数和运行时间仅展示，不设置未经确认的告警阈值。

存在多个问题时，先取最高等级，再按固定来源优先级选择状态来源，保证结果稳定。

## 总览卡

Java 服务是总览第七张独立卡，继续使用共享网格与 `ModuleStatusCardShell`，独立加载、失败、stale 和重试，不阻塞其他板块。

主汇总固定为“异常服务 x / 总服务”，异常服务为 warning、critical、unknown 之和。右侧分别展示“严重 N”和“警告/未知 N”。下方四类摘要为：

- `健康检查`：健康检查严重或未知的服务。
- `端口状态`：端口状态严重或未知的服务。
- `进程状态`：进程状态或端口进程一致性严重、未知的服务；同一服务只按较高问题计一次。
- `采集状态`：采集警告、严重或未知的服务。

某类没有问题时显示“无异常”。服务总数为零时显示“暂无 Java 服务”。模块整体等级取全部服务的最高等级。

## 服务列表与交互

侧边栏在 RabbitMQ 后增加 Java 服务。页面必须复用 `ListPage`、共享刷新控件、搜索/下拉框、表格面板、空状态和分页，不新增 Java 专用控件结构或重复样式。

固定 13 列且顺序不可变：

1. 业务端
2. 服务地址
3. 健康检查
4. 健康延迟
5. 端口状态
6. 进程状态
7. 进程数
8. 端口进程一致性
9. CPU 使用率
10. 内存占用
11. 内存使用率
12. 运行时间
13. 状态

每个单元格只显示一个值并严格单行，不使用 `<br>`、上下副指标或第二行说明。业务端和服务地址过长时显示省略号，完整内容仅通过原生 `title` 提示。

1440×900 下页面和表格均不得横向滚动；更窄视口才允许唯一表格滚动容器横向滚动。不得通过换行、隐藏列或降低状态徽标可读性达成布局目标。

交互规则：

- 搜索匹配原始业务代码、映射后的中文业务端和服务地址，300ms 防抖。
- 支持业务端筛选和正常、警告、严重、未知状态筛选；业务端选项从完整共享快照生成，不受当前页影响。
- 13 个固定字段均支持服务端升降序；缺失值无论方向均置后，并以稳定服务 ID 收口。
- 每页数量仅允许 20、50、100，默认 20；切换搜索、筛选、排序或每页数量后回到第 1 页。
- `search`、`name`、`status`、`sort`、`direction`、`page`、`page_size` 全部写入 URL，刷新与分享后恢复。

## HTTP 契约与安全边界

仅新增两个认证 GET API：

- `GET /api/v1/java/overview`
- `GET /api/v1/java/services`

总览拒绝任何查询参数。服务列表只接受已定义的搜索、业务端、状态、13 个排序字段、方向与分页参数；未知、重复、空值或非法参数返回 400。其他 HTTP 方法返回 405。上游不可用且无可用缓存时返回不含上游信息的安全 503。

使用显式 HTTP View；数组始终为 `[]` 而不是 `null`。响应不得包含 `ident`、原始标签集合、PromQL、数据源 ID、认证信息、上游 URL、Base URL 或上游正文。页面和 API 不提供详情、历史、代理、任意查询或任何写操作。

## 错误与降级

- Provider 严格校验 HTTP 状态、Content-Type、响应 envelope、非空数据、11 个结果组及顺序、响应大小和超时，并禁用重定向。
- Nightingale 401/403、重定向、非 JSON、错误 envelope、空数据、响应超过 8 MiB、超时或结果组数量不匹配统一映射为安全不可用错误。
- 身份、批量结构或协议错误使整个快照不可用；单项数值非法、越界或冲突只使对应字段缺失。
- 首次加载显示页面或卡片级加载状态；空集合显示明确空状态。
- 后台刷新失败且有缓存时，在既有全局 stale 边界内保留旧内容并显示 stale 与刷新失败提示。
- Java 总览和服务页错误相互隔离，Java 模块失败不阻塞其他模块。
- 错误、日志和测试输出不得包含 Token、Cookie、认证头、Base URL、真实标签值、地址、数量、指标值、固定查询全文或上游正文。

## 测试与验收

所有功能按 TDD 完成：

- 领域与 Mock：锁定稳定 ID、四种状态等级场景、精确名称映射、未知名称兼容、`ident` 排除和深拷贝隔离。
- Provider：锁定 11 条查询及顺序、查询副本、一次 batch、无 N+1、近期 inventory、跨 `ident` 去重、最新值、同时间冲突、身份缺失、二值/整数/百分比/时间边界、精度和响应安全。
- Service：锁定四类业务状态、来源优先级、2/5 freshness、时间推进与回退、stale 老化、资源指标不告警、总览去重、搜索、业务端筛选、13 列排序、缺失置后、稳定收口、分页和极端页码防溢出。
- HTTP：锁定两个认证 GET、显式 View、非 null 数组、参数白名单、重复/空/非法参数 400、405、503、stale 和敏感字段排除。
- 前端：锁定第七张卡、四类摘要、13 个精确表头、每行 13 个单值单行单元格、共享控制栏、URL 恢复、格式化、空/错/stale/刷新状态和无破坏性控件。
- Playwright：锁定侧边栏与总览入口、业务端映射、13 列、筛选/排序/分页 URL、1440×900 无页面或表格横向滚动、更窄视口只有表格滚动、身份省略与完整提示以及没有运维控件。

最终实施验证包括前端全测、typecheck、production build、Playwright 静态发现、Go gofmt/vet、全仓普通/race 测试、编译、只读/敏感扫描和 Docker 无缓存生产镜像。

只有取得单独授权后，才可原位重建仍连接测试 Nightingale 的现有 8080 并执行动态浏览器验收；不创建其他 InfraView 测试端口，不连接或探测生产 Nightingale/Java 服务，不输出任何现场值。

## 明确不做

首期不做 PID 列、详情页、历史趋势、JVM 专属指标、资源阈值告警、日志、链路追踪、直连 Java 应用、任意 PromQL、任意代理、诊断、发布、配置、重启、进程控制或任何运维操作。

## 2026-08-07 交付状态

本设计的 Java 服务实现、脱敏测试与本地提交均已完成。交付保持固定 11 查询、`name + server_ip` 身份、`ident` 对外排除、五项完整精确业务端映射、13 列单值单行列表和默认 15 秒周期的 2/5 freshness。静态 E2E 使用完全合成 route fixture，只执行 Playwright `--list`；动态浏览器验收、现有 8080 重建/部署、`main` 合并和 push 均未执行，且每项需要新的单独授权。
