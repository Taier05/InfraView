# InfraView 架构

## 总览

```text
浏览器 ──HTTP/HTTPS──> 可选 Nginx/Caddy ──> InfraView 单容器 :8080
                                             ├─ React SPA 静态资源
                                             ├─ Go 认证与只读 HTTP API
                                             ├─ 查询/聚合服务
                                             ├─ 内存 TTL/stale/singleflight 缓存
                                             └─ 数据源接口
                                                ├─ Mock（已实现）
                                                └─ Nightingale（未配置空壳）
```

应用不包含数据库、消息队列、SSH 客户端、远程执行器、配置下发或任务调度模块。

## 请求与缓存数据流

1. Go 同源返回 SPA；`index.html` 使用 `no-cache`，只有带内容指纹的 `assets/*` 使用一年 `immutable`。
2. 浏览器通过 HttpOnly 会话 Cookie 调用 `/api/v1/*`。
3. 认证中间件验证内存会话；登录失败由并发安全限速器限制。
4. 查询服务按数据类别构造缓存键：主机清单、单机身份、稳定排序后的主机 ID 集合当前指标、主机加时间范围的历史指标、总览时间范围和数据源健康。
5. 主机搜索、状态筛选、排序和分页在共享的清单/当前指标缓存读取后于内存中执行，不进入缓存键；命中有效缓存时直接返回，相同缓存未命中请求合并为一次数据源调用。
6. 上游失败且旧值未超过 `INFRAVIEW_MAX_STALE` 时返回 `stale=true` 和采集时间；否则返回统一错误。
7. 浏览器明确展示过期或错误状态，不把缺失指标转换为零。

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `cmd/infraview` | `serve` 与容器内 `healthcheck` 入口、依赖装配 |
| `internal/config` | 环境变量读取和启动前校验 |
| `internal/auth` | 固定账号、会话、登录限速 |
| `internal/cache` | TTL、旧值和请求合并 |
| `internal/datasource` | 稳定领域模型与数据源契约 |
| `internal/adapters/mock` | 最多 100 台确定性 Linux Mock 主机 |
| `internal/adapters/nightingale` | 未配置占位实现，不猜测真实 API |
| `internal/service` | 总览聚合、主机查询、范围步长、阈值与降级 |
| `internal/httpapi` | 只读路由、认证、错误、安全头、日志、SPA 托管 |
| `web/src` | React 页面、共享组件、API 客户端与深色主题 |
| `scripts` | smoke、E2E 编排和缓存延迟验收 |
| `docs` | 可恢复开发上下文和运维说明 |

## API 表面

- 会话：`POST/GET/DELETE /api/v1/session`。
- 只读查询：`GET /api/v1/overview`、`GET /api/v1/hosts`、`GET /api/v1/hosts/{id}`、`GET /api/v1/hosts/{id}/metrics`、`GET /api/v1/datasource/status`。
- 进程健康：`GET /healthz`，只反映 InfraView 进程与 HTTP 服务，不以数据源故障触发容器重启。
- command、restart、delete、patch、proxy、任意 query 等运维路由不存在，并由自动化测试持续验证。

## 前端构建

Vite 产物复制到忽略目录 `internal/httpapi/webdist` 后由 `go:embed` 嵌入。当前前端不包含主机详情页和图表运行时，主机清单使用服务端规范化的指标值与等级直接渲染。

## 状态与持久化

配置来自启动环境；会话、缓存和监控数据仅在进程内存。容器重启会使用户退出并清空缓存，但不会丢失业务数据，因为 InfraView 不拥有业务数据。真正的监控历史始终由外部数据源负责。
