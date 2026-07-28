# InfraView Nightingale 第二阶段设计规格

最后更新：2026-07-27

## 目标

在不改变 InfraView 只读产品边界的前提下，实现 `internal/datasource.Provider` 的 Nightingale v9 适配器，并用真实 Nightingale 数据替换当前部署的 Mock 数据源。

本阶段只调用已经验证的只读接口：

- `GET /api/n9e/self/profile`
- `GET /api/n9e/targets?limit=100&p=N`
- `GET /api/n9e/datasource/brief`
- `POST /api/n9e/query-instant-batch`
- `POST /api/n9e/query-range-batch`

不使用任意代理接口，不接收来自浏览器的 URL、PromQL 或 Nightingale 原始请求体，也不实现 SSH、命令执行、配置下发或任何写操作。

## 认证与传输

- 基础地址来自 `INFRAVIEW_NIGHTINGALE_BASE_URL`，只允许绝对 `http` 或 `https` URL，禁止用户信息、查询参数和片段。
- Token 来自 `INFRAVIEW_NIGHTINGALE_TOKEN`，仅通过 `X-User-Token` 请求头发送。
- Token 不进入日志、错误、测试夹具、文档或 Git。
- 上游超时继续由 `cmd/infraview` 的 Provider 超时包装器统一施加。
- HTTP 响应体默认最多读取 8 MiB；超限、非 200、非 JSON、JSON 解析失败和 Nightingale envelope `err` 均映射为安全的 `datasource.ErrUnavailable`。

## 配置

新增字段：

| 环境变量 | 默认值 | 规则 |
| --- | --- | --- |
| `INFRAVIEW_NIGHTINGALE_BASE_URL` | 空 | Nightingale 模式必填；绝对 HTTP(S) URL；无 userinfo/query/fragment |
| `INFRAVIEW_NIGHTINGALE_TOKEN` | 空 | Nightingale 模式必填；去除首尾空白后不得为空 |
| `INFRAVIEW_NIGHTINGALE_INTERFACE_EXCLUDE_REGEX` | `lo|docker.*|veth.*|cali.*|br-.*|tunl.*` | 必须是有效 RE2 正则；仅作为 PromQL 字符串值，经安全转义后注入固定查询模板 |

Mock 模式不要求 Nightingale 配置。错误信息只能引用变量名，不能包含 Token 值。

## 数据源发现

Provider 首次需要查询指标时调用 `/datasource/brief`：

1. 只考虑 `plugin_type=prometheus` 的条目；VictoriaMetrics 在 Nightingale 中通过此插件类型提供 Prometheus 兼容查询。
2. 优先选择 `is_default=true` 的条目；同一优先级按 ID 升序选择。
3. 仅缓存成功选择的正整数 ID；失败不永久缓存，后续请求可重试。
4. 单个 Provider 实例并发发现时合并为一次上游请求。

## 主机清单映射

`ListHosts` 以 `limit=100` 分页读取 `/targets`，直到已取得 `total` 条记录。空页但尚未达到 `total`、重复/空 `ident`、非法分页数据均视为上游不可用。

| InfraView 字段 | Nightingale 字段/查询 | 映射 |
| --- | --- | --- |
| ID、Name | `ident` | 保持原值，作为稳定 ID |
| IP | `host_ip` | 保持原值 |
| OS | `os` | 保持原值 |
| Status | `target_up` | `2=online`、`1=unknown`、`0=offline`；其他值为 unknown |
| StatusTime | `beat_time` | Unix 秒转 UTC；非正数为零值 |
| CPUCores | `cpu_num` | 正整数保留；其他为未知 |
| MemoryTotalBytes | `mem_total` | 对全部 ident 批量即时查询；正有限值转 `int64` |
| Uptime | `system_uptime` | 对全部 ident 批量即时查询；非负有限秒数转 `time.Duration` |

内存总量和运行时间必须在一次 `/query-instant-batch` 中获取，禁止按主机 N+1。

`GetHost` 复用同一分页与批量映射流程，再按精确 `ident` 查找；不存在时返回 `datasource.ErrNotFound`。

## 当前指标查询

`GetCurrentMetrics` 对传入的全部主机 ID 生成安全的 `ident` 正则匹配器，并一次调用 `/query-instant-batch`。主机 ID 先通过 RE2 字面量转义，再作为 PromQL 字符串安全转义。

固定查询顺序：

1. CPU：`cpu_usage_active`，限定 `cpu="cpu-total"`。
2. 内存：`mem_used_percent`。
3. 负载：`system_load1`。
4. IO：`max by (ident) (diskio_io_util...)`。
5. 网络发送：`sum by (ident) (rate(net_bytes_sent...[2m]))`。
6. 网络接收：`sum by (ident) (rate(net_bytes_recv...[2m]))`。

网络查询固定使用 `interface!~"<排除规则>"`。配置正则只位于字符串值内，必须转义反斜杠、双引号和控制字符，不能改变 PromQL 结构。

即时响应为 `dat[查询索引][序列]`；按序列 `metric.ident` 归并。`value=[Unix秒, 字符串值]` 转为 UTC 时间和有限 `float64`。缺少 ident、空结果、NaN 或无穷值按缺失数据处理，不伪造 0。

## 历史查询

`QueryRange` 每次只接受领域 `MetricKey`，通过代码内置映射生成一个 `/query-range-batch` 查询；不支持的磁盘容量/读写指标返回对应主机的空序列，不猜测上游指标名。

已支持映射：CPU、内存、负载、IO、网络发送、网络接收。响应 `values=[[Unix秒, 字符串值], ...]` 转为 `datasource.Point`。即使上游无序列，也为每个请求主机返回空 `Series`。

`QueryAggregateRange` 使用固定上游聚合，仅支持同一调用中的领域键白名单。总览当前只请求 CPU 与内存：

- CPU：所有有效 `ident` 的平均值。
- 内存：所有有效 `ident` 的平均值。

每个键在同一次 `/query-range-batch` 中发送，结果顺序与请求键一致；无结果返回空点集。

## HTTP 与错误边界

每次请求必须：

1. 只拼接代码内置路径。
2. 设置 `Accept: application/json`；POST 同时设置 `Content-Type: application/json`。
3. 设置且只设置一次 `X-User-Token`。
4. 要求 HTTP 200。
5. 要求 Content-Type 为 `application/json` 或带参数的 JSON media type。
6. 在限定大小内读取并只解码一个 JSON 值。
7. 要求 envelope 含可解码的 `dat`，且 `err` 为 `null` 或空字符串。

401/403、HTML/text 错误、上游正文、请求 ID、Token 和原始 PromQL均不得进入对外错误。所有网络及上游协议错误统一包装 `datasource.ErrUnavailable`；精确主机缺失单独返回 `datasource.ErrNotFound`。

## 测试与验收

仓库夹具必须完全脱敏，使用保留文档地址 `192.0.2.0/24` 和虚构 ident。测试至少覆盖：

- `X-User-Token`、请求方法、路径和 JSON Content-Type。
- Target 多页读取、状态映射、CPU 核数、内存字节数、运行时间秒到 duration。
- 嵌套即时/区间响应、空结果和无 N+1。
- 默认数据源选择与成功缓存。
- 401、非 JSON、envelope `err`、畸形 JSON、响应大小上限。
- 配置模式、Base URL、Token 脱敏和接口排除正则校验。
- 主程序把配置安全注入 Provider。

先用 Docker 运行 Go 普通测试、race 测试、前端测试和生产构建。全部通过后才允许显式设置 `INFRAVIEW_ENV_FILE=/secure/path/infraview.env` 重建当前 Compose 服务并做真实只读 API/浏览器验证。
