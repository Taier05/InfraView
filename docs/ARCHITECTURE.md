# InfraView 架构

## 总览

```text
浏览器 ──HTTP/HTTPS──> 可选 Nginx/Caddy ──> InfraView 单容器 :8080
                                             ├─ React SPA 静态资源
                                             ├─ Go 认证与只读 HTTP API
                                             ├─ Linux 查询/聚合服务
                                             ├─ MySQL 查询/聚合 Service
                                             ├─ 硬盘 SMART 查询/聚合 DiskService
                                             ├─ 内存 TTL/stale/singleflight 缓存
                                             └─ 数据源接口
                                                ├─ Mock（Linux、MySQL 与硬盘）
                                                └─ Nightingale（受限只读客户端）
```

应用不包含数据库、消息队列、SSH 客户端、远程执行器、配置下发或任务调度模块。

## 请求与缓存数据流

1. Go 同源返回 SPA；`index.html` 使用 `no-cache`，只有带内容指纹的 `assets/*` 使用一年 `immutable`。
2. 浏览器通过 HttpOnly 会话 Cookie 调用 `/api/v1/*`。
3. 认证中间件验证内存会话；登录失败由并发安全限速器限制。
4. 查询服务按数据类别构造缓存键：主机清单、单机身份、稳定排序后的主机 ID 集合当前指标、主机加时间范围的历史指标、总览时间范围、MySQL 快照和数据源健康。
5. 主机搜索、状态筛选、排序和分页在共享的清单/当前指标缓存读取后于内存中执行，不进入缓存键；命中有效缓存时直接返回，相同缓存未命中请求合并为一次数据源调用。
6. 上游失败且旧值未超过 `INFRAVIEW_MAX_STALE` 时返回 `stale=true` 和采集时间；否则返回统一错误。
7. 浏览器明确展示过期或错误状态，不把缺失指标转换为零。
8. 总览服务复用同一批主机清单和当前指标，在进程内计算主机最高告警等级及 CPU、内存、IO、网络异常数量；该聚合不增加上游查询，也不存储告警事件。
9. MySQL Service 从独立的快照缓存读取实例，列表 API 的 `label` 参数按实例标签精确筛选，服务端完成地址搜索、IP/端口自然排序、其他筛选、分页、复制状态与采集新鲜度聚合；MySQL Provider 仅发送固定 16 条代码内置查询组成的一次即时 batch，并按实例身份归并，禁止按实例 N+1。
10. 采集新鲜度使用原始样本是否推进：Linux 固定 batch 通过 `tlast_over_time(system_uptime[24h])` 取得主机心跳，MySQL 通过 `tlast_over_time(mysql_up[24h])` 同时保留近期身份和最后上报时间；Service 以进程内并发安全状态记录本地最后观察到样本变化的时刻，并按 2/5 周期升级。Target 更新时间、即时查询外层求值时间和原始时间相对当前时间的固定偏差均不参与新鲜度判断。
11. DiskService 使用独立完整快照缓存。Nightingale 硬盘 Provider 精确发送一次固定 18 查询 `query-instant-batch`，第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组 inventory 建立设备集合并提供型号与原始最后样本时间；按主机与设备身份归并，无主机/设备 N+1。默认 `60s` 同时作为快照 TTL 与 freshness 周期，120 秒未观察到原始样本时间推进为警告，300 秒为严重。缓存命中不伪造推进，stale 回退时本地推进时间继续老化。
12. 硬盘稳定 ID 将主机身份与 WWN、`serial_no`、设备名中最高优先级的可用身份及其类型一起做不可逆哈希；原始身份只在 Provider 内归并，不进入领域输出、HTTP View 或前端类型。温度、寿命和错误计数只展示，不使用 InfraView 通用阈值改变最终状态。
13. 硬盘最终状态来源通过六值 `status_source` 明示：`smart_health`、`device_warning`、`attribute_failure`、`collection`、`normal`、`unknown`。等级相同时设备来源优先于采集来源，设备来源内部依次为 SMART 健康、设备警告、属性失败；只有采集等级严格更高时来源才是 `collection`。

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `cmd/infraview` | `serve` 与容器内 `healthcheck` 入口、依赖装配 |
| `internal/config` | 环境变量读取和启动前校验 |
| `internal/auth` | 固定账号、会话、登录限速 |
| `internal/cache` | TTL、旧值和请求合并 |
| `internal/datasource` | 稳定领域模型与数据源契约 |
| `internal/mysql` | MySQL 领域模型、稳定实例 ID 与 Provider 契约 |
| `internal/disk` | 硬盘领域模型、不可逆稳定设备 ID 与只读 Provider 契约 |
| `internal/adapters/mock` | 确定性 Linux、MySQL 与硬盘 Mock |
| `internal/adapters/nightingale` | 代码内置查询、受限 HTTP 校验与只读归并 |
| `internal/service` | Linux/MySQL/硬盘总览聚合、查询、阈值、新鲜度与降级 |
| `internal/httpapi` | 只读路由、认证、错误、安全头、日志、SPA 托管 |
| `web/src` | React 页面、共享组件、API 客户端与深色主题 |
| `scripts` | smoke、E2E 编排和缓存延迟验收 |
| `docs` | 可恢复开发上下文和运维说明 |

## API 表面

- 会话：`POST/GET/DELETE /api/v1/session`。
- 只读查询：`GET /api/v1/overview`、`GET /api/v1/hosts`、`GET /api/v1/hosts/{id}`、`GET /api/v1/hosts/{id}/metrics`、`GET /api/v1/datasource/status`、`GET /api/v1/mysql/overview`、`GET /api/v1/mysql/instances`、`GET /api/v1/disks/overview`、`GET /api/v1/disks/devices`。
- `GET /api/v1/overview` 的 `alerts` 字段包含受影响主机、严重/警告主机以及 CPU、内存、IO、网络分级数量；主机数按最高等级去重，指标数独立统计。
- 主机清单与单机只读响应中的 `cpu_cores`、`memory_total_bytes` 为可选资产配置字段；数据源未知时返回 `null`。这两个字段来自主机资产清单，不从使用率指标反推。
- 进程健康：`GET /healthz`，只反映 InfraView 进程与 HTTP 服务，不以数据源故障触发容器重启。
- command、restart、delete、patch、proxy、任意 query 等运维路由不存在，并由自动化测试持续验证。
- MySQL 路由只接受 GET；总览拒绝查询参数，实例清单只接受固定的搜索、状态、角色、排序与分页参数。没有 MySQL 历史、详情、写入或代理路由。
- 硬盘路由只接受 GET；总览拒绝查询参数，设备清单只接受固定搜索、状态、排序和分页参数。响应不包含序列号、WWN、原始标签、PromQL 或上游请求信息。

## 前端构建

Vite 产物复制到忽略目录 `internal/httpapi/webdist` 后由 `go:embed` 嵌入。当前前端不包含主机或硬盘详情页和图表运行时；总览分别加载 Linux、硬盘与 MySQL 可点击板块卡，任一板块失败或 stale 不阻塞其他板块。三个清单均使用服务端规范化的指标值与等级直接渲染，总览与列表共用刷新控制组件。

## 状态与持久化

配置来自启动环境；会话、缓存和监控数据仅在进程内存。容器重启会使用户退出并清空缓存，但不会丢失业务数据，因为 InfraView 不拥有业务数据。真正的监控历史始终由外部数据源负责。
