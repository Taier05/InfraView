# InfraView Nightingale 第二阶段实施计划

> 本计划以测试驱动方式执行；宿主机不安装 Go，所有 Go 验证使用 `golang:1.24-bookworm` 或仓库 Dockerfile。

**目标：** 实现安全、批量、只读的 Nightingale Provider，通过真实环境联调后把 InfraView 当前数据源从 Mock 切换为 Nightingale。

**设计规格：** `docs/superpowers/specs/2026-07-27-nightingale-phase-2-design.md`

## Task 1：脱敏契约夹具和 HTTP 客户端失败测试

- [x] 创建 `internal/adapters/nightingale/testdata`，保存 Target 分页、数据源摘要、即时批量、区间批量、空结果、envelope 错误和非 JSON 夹具。
- [x] 测试认证头、固定路径、POST JSON Content-Type 和 Token 不出现在错误中。
- [x] 测试 HTTP 401、非 JSON、畸形 JSON、envelope `err` 和响应体大小限制。
- [x] 在 Docker 中运行 Nightingale 包测试并确认先失败。

## Task 2：主机清单、数据源发现和当前指标

- [x] 实现受限 HTTP client、JSON envelope 校验和安全领域错误。
- [x] 实现 `/targets` 分页、`ident` 稳定 ID、状态与资产字段映射。
- [x] 实现默认 Prometheus 数据源选择、并发安全成功缓存。
- [x] 用一次即时批量查询补齐内存总量和运行时间。
- [x] 用一次即时批量查询获取全部当前指标并按 `ident` 归并。
- [x] 测试 100 台单页、跨页、空结果和无 N+1。

## Task 3：历史与聚合查询

- [x] 建立领域 MetricKey 到固定 PromQL 的白名单映射。
- [x] 对主机 ID 和网络排除正则做 PromQL 字符串安全转义。
- [x] 实现 `QueryRange` 嵌套 matrix 解析和空序列语义。
- [x] 实现 `QueryAggregateRange` 单次多查询与全局聚合。
- [x] 测试时间戳、步长、单位、缺失值和最多 600 点的服务层组合。

## Task 4：配置和启动注入

- [x] 允许 `INFRAVIEW_DATA_SOURCE=nightingale`。
- [x] 校验 Base URL、Token 和接口排除 RE2；错误不得包含 Token。
- [x] 保持 Mock 模式兼容且不要求 Nightingale 变量。
- [x] 修改 `cmd/infraview/main.go`，把配置、HTTP client 和时钟安全注入 Provider。
- [x] 更新 `.env.example` 和配置文档。

## Task 5：Docker 验证与真实联调

- [x] Docker 运行 `gofmt` 检查、Go 普通测试、race 测试和构建。
- [x] 运行前端 Vitest、typecheck 和 production build。
- [x] 构建生产镜像，确认镜像阶段测试通过。
- [x] 使用 `/secure/path/infraview.env` 显式作为 Compose env file，重建并切换当前服务。
- [x] 登录后验证数据源状态、总览、脱敏主机样本、指标字段、刷新和浏览器布局。
- [x] 确认没有任意查询、写操作、Token 泄露或 N+1 请求。

## Task 6：文档与交接

- [x] 更新 `docs/datasources/NIGHTINGALE.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 和 `docs/HANDOFF.md`。
- [x] 记录真实联调结果和剩余风险，不合并 `main`。
