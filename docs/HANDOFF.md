# InfraView 开发交接

最后更新：2026-07-27

## 新对话恢复入口

请先阅读：

1. `docs/HANDOFF.md`（本文件）
2. `docs/PROJECT_STATUS.md`
3. `docs/TODO.md`
4. `docs/datasources/NIGHTINGALE.md`

继续开发时使用：

- 工作目录：`/root/github/InfraView/.worktrees/compact-layout`
- 分支：`feature/compact-layout`
- Nightingale 私密环境文件：`/root/github/InfraView/.env`
- 当前 InfraView 访问地址：`http://192.168.8.200:8080`
- Nightingale API：`http://192.168.8.211:17000`

不要在聊天、测试夹具、日志、错误消息或 Git 中输出 Nightingale Token。工作树内没有 Token；后续 Compose 若从当前 worktree 启动，应显式设置 `INFRAVIEW_ENV_FILE=/root/github/InfraView/.env`，不要复制私密文件进工作树。

## 当前暂停点

本轮已经完成真实 Nightingale 的只读认证与 API 契约验证，但尚未开始修改 Nightingale 适配器源码，也尚未创建测试夹具。用户因长对话提示决定在此暂停，并在新对话继续。

暂停时工作树无未提交源码修改；当前线上/预览容器仍使用 Mock 数据源和 32 台主机，没有切换 Nightingale，也没有重启或修改 Nightingale。

宿主机没有安装 Go，直接执行 `go test ./...` 会得到 `go: command not found`。仓库现有 Dockerfile 会在构建阶段执行普通测试和 race 测试，后续应使用 Docker 构建完成 Go 验证，不需要在宿主机安装 Go。

## 本轮已完成的只读验证

- `/root/github/InfraView/.env` 权限为 `600`，属主/属组均为 root。
- `INFRAVIEW_NIGHTINGALE_BASE_URL` 和 `INFRAVIEW_NIGHTINGALE_TOKEN` 均存在；未输出变量值。
- 用户确认测试和现阶段正式方案都暂时使用 Nightingale root 用户的个人 Token。
- `X-User-Token` 认证成功；以下接口均返回 HTTP 200、JSON、`err` 为空：
  - `GET /api/n9e/self/profile`
  - `GET /api/n9e/targets?limit=100&p=1`
  - `GET /api/n9e/targets/stats`
  - `GET /api/n9e/busi-groups`
  - `GET /api/n9e/datasource/brief`
- 当前账号可见 3 台 Target、1 个业务组、1 个数据源。
- 无 Token 和错误 Token 调用受保护接口均返回 HTTP 401，Content-Type 为 `text/plain; charset=utf-8`，不能假设所有错误都有 JSON envelope。
- `POST /api/n9e/query-instant-batch` 和 `POST /api/n9e/query-range-batch` 已真实验证成功。
- 批量查询返回结构：
  - 即时：`dat[查询索引][序列]`，序列含 `metric` 和 `value=[Unix秒, 字符串值]`。
  - 区间：`dat[查询索引][序列]`，序列含 `metric` 和 `values=[[Unix秒, 字符串值], ...]`。
- 不存在主机的固定筛选查询返回 HTTP 200、`err` 为空及空序列，不是 404。
- 当前 3 台 Target 的 `target_up` 都为 `2`；`cpu_num` 和 `beat_time` 都是整数，`beat_time` 是 Unix 秒。
- 下列 9 个即时查询均各返回 3 条序列、覆盖 3 个 `ident`，值为字符串、时间戳为整数：CPU、内存、负载、IO 忙碌度、网络发送、网络接收、运行时间、CPU 核数、内存总量。
- CPU 序列标签键为 `__name__`、`cpu`、`ident`；内存/负载/运行时间/配置指标含 `__name__`、`ident`；聚合后的 IO 和网络序列仅含 `ident`。

## 已确认实现决策

- `ListHosts` 分页读取 `/targets`，以 `ident` 作为稳定 ID。
- 状态暂按 `target_up=2 -> online`、`1 -> unknown`、`0 -> offline` 映射。
- CPU 核数优先读取 Target `cpu_num`；非正数映射为未知。
- 内存总容量通过 `mem_total` 批量即时查询；运行时间通过 `system_uptime` 查询。
- 当前指标必须一次调用 `/query-instant-batch` 批量获取并按 `ident` 归并，禁止按主机 N+1。
- IO 使用 `max by (ident) (diskio_io_util{ident!=""})`。
- 网络使用按主机求和的 `rate(net_bytes_sent[2m])` / `rate(net_bytes_recv[2m])`，默认排除 `lo|docker.*|veth.*|cali.*|br-.*|tunl.*`；过滤规则要可配置并安全转义，不能硬编码当前物理网卡名。
- 数据源 ID 从 `/datasource/brief` 选择默认 Prometheus/VictoriaMetrics 数据源并在 Provider 内缓存。
- 只允许代码内置的指标到 PromQL 映射；不接受前端传入 URL、PromQL 或 Nightingale 原始请求体；不使用任意代理接口。
- HTTP 客户端必须校验状态码、Content-Type、JSON envelope 和 `err`，限制响应体大小，并把 401/403、非 JSON、解析失败、上游错误统一映射为安全的领域错误，不能泄露 Token 或响应正文。

## 新对话的第一组任务

1. 再次确认 `git status` 干净，不要重复做 SSH/版本探测。
2. 编写 Nightingale 第二阶段规格与实施计划。
3. 先添加完全脱敏的 `internal/adapters/nightingale/testdata` 契约夹具和失败测试：认证头、分页、嵌套批量响应、状态/单位映射、空结果、401、非 JSON、envelope `err`、响应大小限制、无 N+1。
4. 以 TDD 实现 `internal/adapters/nightingale/provider.go` 及必要客户端代码。
5. 扩展 `internal/config`：允许 `INFRAVIEW_DATA_SOURCE=nightingale`，校验 Base URL、Token 和网络接口排除规则；错误信息不得包含 Token。
6. 修改 `cmd/infraview/main.go`，把配置安全注入 Nightingale Provider。
7. 通过 Docker 构建运行 Go/前端测试；随后使用真实 `.env` 构建并切换 InfraView 数据源，验证 API 和浏览器页面。
8. 只有在真实联调通过后才更新当前部署状态；不要合并 `main`，除非用户另行确认。

## 安全边界

InfraView 始终只读展示：不提供修改、删除、重启、发布或配置下发，不执行 SSH/远程命令，不运行脚本，不自动化变更服务器、服务或配置。开发期已执行的远端检查仅用于用户授权的只读取证；运行时不得包含 SSH 客户端或远程执行能力。
