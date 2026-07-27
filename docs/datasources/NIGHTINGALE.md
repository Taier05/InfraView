# Nightingale 数据源接入

状态：测试环境证据采集中，真实适配器尚未实现。当前 `INFRAVIEW_DATA_SOURCE` 只接受 `mock`。

最后采集：2026-07-27

## 测试环境实证

| 项目 | 已验证结果 |
| --- | --- |
| Nightingale | `v9.0.0-b78db21544584381def321fb5201ac80e48ad491` |
| API 基础地址 | `http://192.168.8.211:17000`，当前为局域网 HTTP，无 TLS |
| 部署方式 | `n9e.service` 与 `categraf.service`；MySQL、Redis 使用 Docker；VictoriaMetrics 为宿主机进程 |
| Categraf | `v0.5.16`，15 秒采集周期，已有 3 台 Linux Target |
| 指标存储 | VictoriaMetrics `http://192.168.8.211:8428`，Nightingale 默认 Prometheus 类型数据源 |
| 认证能力 | `HTTP.TokenAuth.Enable=true`；个人 Token 请求头为 `X-User-Token: <token>`，不是 Bearer |
| 当前认证阻塞 | 数据库中现有个人 Token 数为 0；只有 1 个 Admin 用户，尚未提供 InfraView 专用只读 Token |
| 主机清单 | 数据库只读确认 3 台 Target：`y01`、`y02`、`y03`，均为 Categraf v0.5.16 Linux 主机 |
| 指标现状 | VictoriaMetrics 已有实际时序数据；未认证 Nightingale Target API 返回 HTTP 401 `unauthorized` |
| 官方源码匹配 | 运行二进制提交与官方 `ccfos/nightingale` 标签 `v9.0.0` 的提交 `b78db21544584381def321fb5201ac80e48ad491` 完全一致 |

本次证据采集使用用户明确授权的开发期只读 SSH，只读取服务状态、版本、脱敏配置结构和只读数据库元数据/非敏感资产字段，没有修改或重启 Nightingale 组件。该行为不改变产品边界：InfraView 运行时不得包含 SSH 客户端、远程命令或自动化变更能力。

## 已确认指标映射

| InfraView 字段 | Nightingale / Categraf 指标 | 标签与聚合 | 原始单位 |
| --- | --- | --- | --- |
| 主机 ID / 名称 | Target `ident` | 当前为 `y01`、`y02`、`y03` | 字符串 |
| IP | Target `host_ip`；`system_info` 也含 `host_ip` | 按 `ident` 映射 | IPv4 字符串 |
| CPU 使用率 | `cpu_usage_active` | 每个 `ident` 单序列 | 百分比 |
| 内存使用率 | `mem_used_percent` | 每个 `ident` 单序列 | 百分比 |
| 1 分钟负载 | `system_load1` | 每个 `ident` 单序列 | 标量 |
| IO 忙碌度 | `diskio_io_util` | 每主机存在多磁盘序列；暂定 `max by (ident)`，需用认证查询验证 Nightingale 返回结构 | 百分比 |
| 网络发送/接收 | `net_bytes_sent` / `net_bytes_recv` | 累计计数器，需 `rate()`；存在物理、Docker、Calico、veth、bridge、tunnel 接口，必须使用受控可配置过滤规则 | 字节计数器，换算 B/s |
| 运行时间 | `system_uptime` | 每个 `ident` 单序列 | 秒 |
| CPU 核数 | `system_n_cpus`；`machine_cpu_cores` 可作交叉验证 | 每个 `ident` 单序列 | 核 |
| 内存总量 | `mem_total`；`machine_memory_bytes` 可作交叉验证 | 每个 `ident` 单序列 | 字节 |
| 系统元数据 | `system_info` | 含 `ident`、`hostname`、`host_ip`、`kernel_version` | 标签 |

当前 3 台测试机的 CPU 核数均为 8，内存总量均为 `16497283072` 字节。网络物理接口当前为 `ens34`，但适配器不能硬编码该名称；建议配置允许/排除正则，默认排除 `lo`、Docker、veth、Calico、bridge 和 tunnel 接口。

## 已确认只读 API 契约

以下契约来自与运行二进制完全匹配的官方 v9.0.0 源码和内置 API 文档；未认证访问受保护路由返回 401：

- `GET /api/n9e/targets?limit=100&p=1`：分页主机清单，响应 `dat={"list":[...],"total":N}`。Target 包含 `ident`、`host_ip`、`os`、`agent_version`、`beat_time`、`target_up`、`cpu_num`、`cpu_util`、`mem_util`、`arch`、业务组和标签。
- `GET /api/n9e/targets/stats`：可见主机总数、存活/离线数和 CPU/内存分桶。
- `GET /api/n9e/busi-groups`：当前用户可见业务组，`dat` 为数组。
- `GET /api/n9e/datasource/brief`：返回已脱敏的数据源摘要；其 `id` 用作查询的 `datasource_id`，`plugin_type` 用作数据源类型。
- `POST /api/n9e/query-instant-batch`：Prometheus/VictoriaMetrics 批量即时只读查询。请求体包含 `datasource_id`、固定 PromQL 和 Unix 秒时间，`dat` 按查询顺序返回 vector 数组。
- `POST /api/n9e/query-range-batch`：Prometheus/VictoriaMetrics 批量历史只读查询。请求体包含 `datasource_id`、固定 PromQL、起止 Unix 秒和步长秒，`dat` 按查询顺序返回 matrix 数组。
- `GET /api/n9e/version`：公开版本接口，已在当前环境验证。

Nightingale API 响应外层使用 `dat`、`err`、`request_id` 字段；不能只依赖 HTTP 状态，还必须检查 `err`。错误路径会被 SPA 接管并返回 HTML，因此适配器必须同时校验 Content-Type 和 JSON 结构。

## 拟定适配策略（等待 Token 实证）

- `ListHosts`：分页调用 `/targets`，以 `ident` 作为稳定 ID 和显示主机名，映射 `host_ip`、`os`、`agent_version`；`target_up=2` 映射 online，`1` 映射 warning/unknown，`0` 映射 offline。最终状态语义需用真实响应确认。
- CPU 核数优先使用 Target 的 `cpu_num`，`-1` 映射未知；可使用 `system_n_cpus` 交叉验证。
- 内存总容量使用 `mem_total` 批量即时查询，因为 Target 只直接提供 `mem_util`，不提供总字节数。
- `GetCurrentMetrics`：对全部目标使用固定、代码内置的 PromQL 通过一次 `/query-instant-batch` 批量查询，按 `ident` 归并，禁止按主机 N+1。
- `QueryRange`：使用 `/query-range-batch`，只允许领域指标映射生成的固定查询，不接受前端或用户提交 PromQL。
- `QueryAggregateRange`：使用批量范围查询直接在上游按 `ident`/全局聚合，保持总览每指标单序列和最多 600 点约束。
- IO 使用 `max by (ident) (diskio_io_util{ident!=""})`。当前环境即时验证值正常。
- 网络使用 `rate(net_bytes_sent[2m])` / `rate(net_bytes_recv[2m])` 并执行配置化接口过滤；当前环境排除虚拟接口后得到合理 B/s。不得硬编码 `ens34`。
- 不使用 `/api/n9e/proxy/:id/*` 实现任意代理，不向 InfraView API 或页面暴露任意数据源 URL、查询表达式或原始 Nightingale 请求体。

在取得只读 Token 前，以上 API 契约虽有官方源码依据，但仍不能替代当前环境的真实认证响应、分页、权限和错误夹具。下一步必须用 `X-User-Token` 捕获完全脱敏的 GET 和只读查询响应。

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

适配器最终只实现 `internal/datasource.Provider`：`Health`、`ListHosts`、`GetHost`、`GetCurrentMetrics`、`QueryRange`、`QueryAggregateRange`。前端和服务层不接收 Nightingale 原始请求体、任意 URL 或任意查询表达式。

## 实施顺序

1. 更新本文件中的版本/认证/端点证据。
2. 编写独立规格与计划。
3. 先加入脱敏契约夹具和失败测试。
4. 实现只读适配器、批量查询、超时、大小限制和错误分类。
5. 验证 TTL、请求合并、stale、分页和 100 台规模。
6. 更新配置、部署、安全、测试和项目状态文档。

任何真实凭据都不得进入仓库、测试夹具、错误消息或日志。

## 当前需要用户完成

在 Nightingale Web 中为测试创建一个个人 Token。短期联调可以先使用现有 Admin 用户创建的个人 Token 验证接口契约；正式接入前必须换成专用 `infraview` 用户和最小只读角色。Token 不要发送到聊天或提交到 Git，应写入 InfraView 主机上权限为 600、已被 Git 忽略的 `/root/github/InfraView/.env`：

```dotenv
INFRAVIEW_NIGHTINGALE_BASE_URL=http://192.168.8.211:17000
INFRAVIEW_NIGHTINGALE_TOKEN=<个人 Token>
```

写入完成后只需告知“Token 已写入”，不要粘贴 Token 内容。
