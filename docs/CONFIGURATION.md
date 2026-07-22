# InfraView 配置

复制 `.env.example` 为 `.env` 后必须先更换示例凭据。配置只在容器启动时读取，页面不能修改配置。

## 应用与 Compose 变量

| 变量 | 默认/示例 | 约束与作用 |
| --- | --- | --- |
| `INFRAVIEW_USERNAME` | `admin` | 必填，固定登录用户名，不能全为空白 |
| `INFRAVIEW_PASSWORD` | `change-me-please` | 必填，至少 12 个字符；示例值禁止用于正式环境 |
| `INFRAVIEW_PORT` | `8080` | Compose 发布地址，格式为 `[宿主IP:]端口`；`8080` 监听全部宿主接口，`127.0.0.1:8080` 仅允许本机反向代理访问 |
| `INFRAVIEW_LISTEN_ADDR` | `:8080` | 容器内 Go 监听地址；Compose 期望端口为 8080 |
| `INFRAVIEW_COOKIE_SECURE` | `false` | 布尔值；HTTPS 反代必须设为 `true` |
| `INFRAVIEW_SESSION_TTL` | `12h` | 正时长；内存会话有效期，重启后仍会失效 |
| `INFRAVIEW_TRUSTED_PROXY_CIDRS` | 空 | 默认不信任转发头；反代时填写直接连接 InfraView 的代理 CIDR，多个值以英文逗号分隔；只有可信代理提供的单值合法 `X-Real-IP` 用于登录限速 |
| `INFRAVIEW_DATA_SOURCE` | `mock` | 当前版本只接受 `mock`；不得填写未实现的 `nightingale` |
| `INFRAVIEW_MOCK_HOST_COUNT` | `32` | 整数 1–100 |
| `INFRAVIEW_REFRESH_INTERVAL` | `30s` | 正时长；当前后端会校验并保留该配置，Mock MVP 浏览器刷新周期仍固定为 30 秒 |
| `INFRAVIEW_INVENTORY_TTL` | `60s` | 正时长，主机清单缓存 |
| `INFRAVIEW_CURRENT_METRICS_TTL` | `20s` | 正时长，当前指标缓存 |
| `INFRAVIEW_RANGE_TTL` | `60s` | 正时长，历史范围缓存 |
| `INFRAVIEW_HEALTH_TTL` | `15s` | 正时长，数据源健康缓存 |
| `INFRAVIEW_MAX_STALE` | `5m` | 正时长且不得超过 5 分钟，旧缓存最大可展示时间 |
| `INFRAVIEW_UPSTREAM_TIMEOUT` | `10s` | 正时长；为真实数据源调用边界预留，Mock 不发网络请求 |
| `INFRAVIEW_WARNING_PERCENT` | `80` | 0–100 有限数值，必须低于危险阈值 |
| `INFRAVIEW_CRITICAL_PERCENT` | `90` | 0–100 有限数值，必须高于警告阈值 |
| `TZ` | `Asia/Hong_Kong` | 容器系统时区；API 时间戳仍按 RFC 3339 表达 |
| `INFRAVIEW_ENV_FILE` | `.env` | Compose 工具变量，可指向专用环境文件；应用本身不读取此变量 |

Go 时长使用 `time.ParseDuration` 语法，例如 `20s`、`5m`、`12h`。错误配置会使进程拒绝启动，错误信息不会输出密码。

## 验收专用变量

`scripts/smoke.sh` 与 `scripts/benchmark.sh` 使用 `INFRAVIEW_BASE_URL` 指定待测服务，并复用 `INFRAVIEW_USERNAME`、`INFRAVIEW_PASSWORD` 登录。`scripts/e2e.sh` 支持 `INFRAVIEW_E2E_PROJECT`、`INFRAVIEW_E2E_PORT`、`INFRAVIEW_E2E_USERNAME`、`INFRAVIEW_E2E_PASSWORD`、`INFRAVIEW_E2E_RUN_BENCHMARK`、`INFRAVIEW_E2E_CHECK_RESOURCES` 和 `PLAYWRIGHT_IMAGE`。默认项目名为带进程唯一后缀的 `infraview-e2e-<PID>`、端口为 `18080`、浏览器镜像为 `mcr.microsoft.com/playwright:v1.61.1-noble`。显式项目名若已存在对应 Compose 项目或标签资源，脚本会在启动前失败，绝不复用或清理它。直接执行 `npm run e2e:run` 时可用 `INFRAVIEW_E2E_BASE_URL` 指向已运行服务。这些不是生产应用配置。

## 凭据与 `.env`

- `.env` 已被 Git 忽略；不要提交、复制到聊天或写入日志。
- 使用随机长密码，限制文件权限和读取人员。
- 正式部署前执行人工检查，确认 `.env` 不等同于 `.env.example`。
- 修改账号密码后重新创建容器；旧内存会话随重启失效。
