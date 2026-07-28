# InfraView 开发交接

最后更新：2026-07-28

## 新对话恢复入口

请先阅读：

1. `docs/HANDOFF.md`（本文件）
2. `docs/PROJECT_STATUS.md`
3. `docs/TODO.md`
4. `docs/datasources/NIGHTINGALE.md`

继续开发时使用：

- 工作目录：`<仓库根目录>`
- 分支：`main`
- Nightingale 私密环境文件：`/secure/path/infraview.env`
- 当前 InfraView 访问地址：由私有部署环境提供，不进入公开仓库
- Nightingale API：由私有部署环境提供，不进入公开仓库

不要在聊天、测试夹具、日志、错误消息或 Git 中输出 Nightingale Token。仓库内没有 Token；后续 Compose 若从仓库启动，应显式设置 `INFRAVIEW_ENV_FILE=/secure/path/infraview.env`，不要复制私密文件进仓库。

## 切换 Codex 账号后的恢复提示词

在新账号的新对话中直接粘贴以下内容：

```text
继续开发 InfraView。请始终使用简体中文回复。

工作目录：<仓库根目录>
分支：main

请先完整阅读并遵循：
1. <仓库根目录>/docs/HANDOFF.md
2. <仓库根目录>/docs/PROJECT_STATUS.md
3. <仓库根目录>/docs/TODO.md
4. <仓库根目录>/docs/datasources/NIGHTINGALE.md
5. Nightingale v8.4.1 兼容历史规格与计划：
   - docs/superpowers/specs/2026-07-28-nightingale-v8-compat-design.md
   - docs/superpowers/plans/2026-07-28-nightingale-v8-compat.md

Nightingale v8.4.1 兼容功能交付基线为 18d26a6，已合并并推送到 origin/main；原 feature/nightingale-v8-compat 分支及 worktree 已清理。本交接文档提交位于该功能基线之后，实际 HEAD 以 git log -1 为准。Nightingale v8.4.1 是主要开发与真实验证版本，v9.x 保留协议兼容。InfraView 始终只读，不增加任意 PromQL、任意代理、SSH、远程命令或写接口。

生产试运行反馈：除 IO 忙碌度外，已有板块指标正常；生产环境可查询 diskio_io_time 派生利用率，但 InfraView 当前固定读取 diskio_io_util。用户决定暂不修改 InfraView，先升级生产 Categraf；升级后至少等待两个采集周期，再验证 diskio_io_util，不能主动猜测或提前替换 IO PromQL。

绝对不要输出私密环境文件内容、Nightingale Token、Cookie、认证头、真实主机标识、IP、资源数量、指标值或上游响应正文。

先只读执行 git status、git log -3 --oneline、git diff --check，确认 main 与 origin/main 状态。除非我明确授权，不要修改代码或文档、提交、推送、合并、重启服务或更改部署。

持续维护设计、架构、开发进度、TODO、部署、测试、安全和 HANDOFF 文档，确保后续新对话仍可从仓库恢复完整上下文。
```

## 当前暂停点

Nightingale v8.4.1 兼容功能已经快进合并并推送到 `origin/main`，功能交付基线为 `18d26a6`；原 `feature/nightingale-v8-compat` 分支和对应 worktree 已删除。该功能完成 v8.4.1 Target 时间字段的 RED→GREEN：`StatusTime` 优先采用有效 `beat_time`，缺失或无效时回退有效 `update_at`，两者都无效时保持零值。v8.4.1 的 profile、Target、数据源 brief、即时批量和区间批量只读契约预检通过；运行时不增加版本探测请求，v9.x 既有协议测试继续保留。

完整生产镜像构建、隔离 Mock smoke/Chromium 4/4、私密环境文件权限与 Git 忽略检查、8080 重建、真实 v8.4.1 应用端只读 smoke 和容器安全检查均已通过。应用端验收覆盖数据源状态、总览、主机清单、单机详情和 1 小时指标范围，返回均非 stale；真实资源信息和响应正文未输出或进入仓库。

生产环境试运行已确认当前版本整体可用，除 IO 忙碌度显示“暂无数据”外，已有板块指标正常。InfraView 当前固定查询 `max by (ident) (diskio_io_util{ident!=""})`；生产环境目前可通过 `diskio_io_time` 的一分钟速率派生近似利用率，两者概念相同但采样窗口和算法不同，不能视为数值完全相等。用户已决定暂不修改 InfraView，先升级生产 Categraf；升级后至少等待两个采集周期，再确认 `diskio_io_util` 是否出现以及当前页面是否在自动刷新后恢复展示。

历史 Nightingale 第二阶段已经完成实现、Docker 全量构建、脱敏 API 联调和 Chromium 页面验收。旧私有预览曾切换为真实 Nightingale，资源数量不进入公开仓库，当时容器健康且数据非 stale。

随后按用户确认方案完成状态与刷新优化：总览零值使用绿色“无…”文案；数据源状态 API 返回真实类型和运行时刷新周期；当前页面与当前指标默认每 15 秒刷新，静态资产和历史缓存仍为 60 秒。左下角已进一步改为紧凑的“数据连接”汇总，健康 Nightingale 默认显示绿色 `1/1 正常`，展开后显示“指标 / Nightingale / 健康 / 最近检查”，Mock 明确使用黄色提示；后续日志等数据源可在同一入口扩展。前端 45/45、typecheck、production build、Go 全仓普通/race、生产镜像和隔离 Mock Chromium 4/4 均通过。私有预览重建后，登录、真实类型、15 秒周期、脱敏主机样本和总览 smoke 均通过且非 stale，并重新完成真实 Nightingale 1440×900 登录浏览器默认/展开状态视觉验收。

### 2026-07-27 审查修复交接

本功能分支又完成一轮审查阻断项 TDD 修复：Provider 复制 HTTP Client 并拒绝重定向；严格要求非 null envelope `dat`、精确空 `err`、分页 `list`/`total` 与批量外层基数；Content-Type 仅接受 `application/json`；历史指标缓存加载先精确验证一次主机；Mock 模式不校验未使用的 Nightingale 设置；构造器拒绝非 HTTP(S)、userinfo、query、fragment；数值转 `int64`、`time.Duration`、指标时间和 Target `beat_time` 前验证 JSON/RFC3339 范围；数据源成功或失败 discovery flight 都只发一次请求并广播结果，失败不缓存且后续可重试。

最终修复后 Docker 相关包、全仓普通/race 测试和 `infraview:nightingale-review-fixes-verify` 生产镜像均重新通过；最终整分支审查 findings 已全部处理，唯一范围复审 PASS。随后按用户授权显式设置 `INFRAVIEW_ENV_FILE=/secure/path/infraview.env`，重建同一 `infraview` Compose 项目的 8080 服务；容器 healthy，登录、数据源状态、脱敏主机样本和总览只读 smoke 均通过。未输出 `.env`、Token、Cookie 或响应正文，未进行 SSH 或额外 Token 实证。用户已于 2026-07-28 授权直接合并并推送 `main`，该操作已完成；原功能分支和 worktree 随后已清理。若重新执行全仓 shell 验证，`golang:1.24-bookworm` 的 `sh -lc` 会因登录 PATH 缺少 `/usr/local/go/bin` 而找不到 Go；使用 `sh -c`，并独立运行 race 命令。

当前 `main` 已包含本轮 Nightingale v8.4.1 兼容源码、测试、夹具和文档。Nightingale 本身没有被 InfraView 修改或重启，InfraView 运行时仍只有只读 HTTP 查询能力。

宿主机没有安装 Go，直接执行 `go test ./...` 会得到 `go: command not found`。仓库现有 Dockerfile 会在构建阶段执行普通测试和 race 测试，后续应使用 Docker 构建完成 Go 验证，不需要在宿主机安装 Go。

## 本轮已完成的只读验证

- 私有环境文件必须被 Git 忽略并限制为 `600` 权限；公开仓库不记录实际路径、属主或内容。
- `INFRAVIEW_NIGHTINGALE_BASE_URL` 和 `INFRAVIEW_NIGHTINGALE_TOKEN` 的真实值只存在于私有部署环境，未进入仓库。
- Nightingale 凭据必须使用专用最小只读 Token；公开文档不记录当前部署账号或凭据权限。
- `X-User-Token` 认证成功；以下接口均返回 HTTP 200、JSON、`err` 为空：
  - `GET /api/n9e/self/profile`
  - `GET /api/n9e/targets?limit=100&p=1`
  - `GET /api/n9e/targets/stats`
  - `GET /api/n9e/busi-groups`
  - `GET /api/n9e/datasource/brief`
- 脱敏验证账号可读取 Target、业务组和数据源；真实数量不进入公开仓库。
- 无 Token 和错误 Token 调用受保护接口均返回 HTTP 401，Content-Type 为 `text/plain; charset=utf-8`，不能假设所有错误都有 JSON envelope。
- `POST /api/n9e/query-instant-batch` 和 `POST /api/n9e/query-range-batch` 已真实验证成功。
- 批量查询返回结构：
  - 即时：`dat[查询索引][序列]`，序列含 `metric` 和 `value=[Unix秒, 字符串值]`。
  - 区间：`dat[查询索引][序列]`，序列含 `metric` 和 `values=[[Unix秒, 字符串值], ...]`。
- 不存在主机的固定筛选查询返回 HTTP 200、`err` 为空及空序列，不是 404。
- v8.4.1 真实预检确认 Target 提供 `update_at`；v9.x 脱敏契约保留 `beat_time`。两者均按 Unix 秒处理，测试覆盖优先级、回退和非法范围。
- 下列 9 个即时查询均按脱敏 `ident` 返回对应序列，值为字符串、时间戳为整数：CPU、内存、负载、IO 忙碌度、网络发送、网络接收、运行时间、CPU 核数、内存总量。
- CPU 序列标签键为 `__name__`、`cpu`、`ident`；内存/负载/运行时间/配置指标含 `__name__`、`ident`；聚合后的 IO 和网络序列仅含 `ident`。

## 已确认实现决策

- `ListHosts` 分页读取 `/targets`，以 `ident` 作为稳定 ID。
- 状态暂按 `target_up=2 -> online`、`1 -> unknown`、`0 -> offline` 映射。
- 状态时间优先读取有效 `beat_time`，缺失或无效时回退有效 `update_at`；不按 Nightingale 版本号分支。
- CPU 核数优先读取 Target `cpu_num`；非正数映射为未知。
- 内存总容量通过 `mem_total` 批量即时查询；运行时间通过 `system_uptime` 查询。
- 当前指标必须一次调用 `/query-instant-batch` 批量获取并按 `ident` 归并，禁止按主机 N+1。
- IO 使用 `max by (ident) (diskio_io_util{ident!=""})`。
- 网络使用按主机求和的 `rate(net_bytes_sent[2m])` / `rate(net_bytes_recv[2m])`，默认排除 `lo|docker.*|veth.*|cali.*|br-.*|tunl.*`；过滤规则要可配置并安全转义，不能硬编码当前物理网卡名。
- 数据源 ID 从 `/datasource/brief` 选择默认 Prometheus/VictoriaMetrics 数据源并在 Provider 内缓存。
- 只允许代码内置的指标到 PromQL 映射；不接受前端传入 URL、PromQL 或 Nightingale 原始请求体；不使用任意代理接口。
- HTTP 客户端必须校验状态码、Content-Type、JSON envelope 和 `err`，限制响应体大小，并把 401/403、非 JSON、解析失败、上游错误统一映射为安全的领域错误，不能泄露 Token 或响应正文。

## 本轮实现与验证

- 新增 v8.4.1 兼容规格、TDD 计划和完全脱敏 Target 夹具；适配器包、完整镜像、隔离 E2E、真实部署 smoke 与容器安全验收均已通过。
- 新增第二阶段规格与计划，以及完全脱敏的 Target、数据源、即时、区间、空结果和错误夹具。
- Provider 已实现 `Health`、Target 分页、默认 Prometheus 数据源成功缓存、资产批量、当前指标批量、历史与聚合范围查询。
- HTTP 客户端校验状态码、JSON Content-Type、envelope `err` 和 8 MiB 上限；错误不包含 Token 或响应正文。
- 配置支持 `nightingale`，校验 Base URL、Token 和接口排除 RE2；主程序已安全注入 Provider。
- `docker build --tag infraview:nightingale-verify .` 通过前端、Go 普通/race 测试和构建。
- 临时 18081 容器完成真实只读 API 与页面验证后已删除。
- 私密 `.env` 的 `INFRAVIEW_DATA_SOURCE` 已改为 `nightingale`，并使用 `INFRAVIEW_ENV_FILE=/secure/path/infraview.env` 重建 8080 Compose 服务。
- 正式 8080 验证：数据源健康，脱敏主机样本的资产/当前指标有值；CPU/内存/负载/网络历史有效；1440×900 页面无溢出，登录后无页面错误。

## 新对话的第一组任务

1. 先查看 `git status`、`git log -3 --oneline` 和 `git diff --check`，确认当前位于 `main`，并核对与 `origin/main` 的同步状态。
2. 等待用户完成生产 Categraf 升级后的反馈；只读确认 `diskio_io_util` 是否在至少两个采集周期后出现。未经用户明确授权，不修改 InfraView 的 IO 查询。
3. 后续事项：每个私有部署落实专用最小只读 Token；按真实证据补充磁盘容量和磁盘读写历史指标。

## 安全边界

InfraView 始终只读展示：不提供修改、删除、重启、发布或配置下发，不执行 SSH/远程命令，不运行脚本，不自动化变更服务器、服务或配置。开发期已执行的远端检查仅用于用户授权的只读取证；运行时不得包含 SSH 客户端或远程执行能力。
