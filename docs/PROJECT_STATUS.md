# InfraView 项目状态

最后更新：2026-07-22

## 当前阶段

Mock MVP 正在隔离分支 `feature/infraview-mock-mvp` 中按子代理驱动和测试驱动方式实施。Task 1–9 已实现并通过任务级规格与质量审查。当前正在进行 Task 10：浏览器 E2E、缓存延迟与资源验证、完整部署和维护文档。

## 已确认范围

- 页面：基础设施总览、Linux 主机列表、单机详情。
- 数据源：首期提供 Mock；真实 Nightingale 适配器在测试环境就绪后实现。
- 指标：在线状态、CPU、内存、系统负载、磁盘容量、磁盘 I/O、网络流量、运行时长。
- 视觉：现代深色中文运维控制台。
- 刷新：每 30 秒自动刷新，支持 1 小时、6 小时、24 小时、7 天。
- 主机规模：首版 100 台以内。
- 登录：Docker Compose 环境变量配置固定账号密码。
- 部署：单 InfraView 容器，可直接通过 IP 和端口访问；反向代理可选。
- 存储：无业务数据库，不保存监控历史数据，仅使用短期内存缓存。
- 安全边界：不执行远程命令，不提供基础设施修改、删除或重启能力。

## 技术方向

- 后端：Go。
- 前端：React + TypeScript。
- 交付：前端静态资源由 Go 服务提供，生产环境运行单一 InfraView 容器。
- 数据源：领域接口 + Mock/Nightingale 适配器。
- 资源目标：单容器空闲内存尽量低于 256 MB。

## 已完成

- 仓库现状调研。
- MVP 需求澄清。
- 技术路线对比与选择。
- 总体架构确认。
- 页面、后端模块和 API 边界确认。
- 数据流、缓存与降级策略确认。
- 安全、错误处理和可观测性确认。
- 测试与验收标准确认。
- 文档体系与两阶段交付范围确认。
- Task 1：Go 配置、健康检查与 Docker 化开发命令。
- Task 2：规范化数据模型、确定性 Mock 与 Nightingale 未配置空壳。
- Task 3：TTL 缓存、请求合并、旧数据降级与只读查询服务。
- Task 4：固定账号登录、内存会话、登录限速与只读 HTTP API。
- Task 5：React/Vite 基础、深色中文主题、API 客户端、登录与会话加固。
- Task 6：基础设施总览、四档真实聚合趋势、共享展示组件、30 秒刷新与 stale/error 展示。
- Task 7：主机名/IP 搜索、状态筛选、服务端排序、URL 状态与分页规范化。
- Task 8：主机详情、当前指标、文件系统容量、四类历史趋势、独立查询错误隔离与缓存保留。
- Task 9：Go 同源托管 SPA、安全单容器 Docker Compose、冒烟测试与运行时资源验证。

## 当前任务

Task 10 正在启动：

- 目标：在真实浏览器覆盖登录、总览、主机列表、详情、刷新和图表渲染。
- 性能：验证缓存 API P95、容器内存与镜像体积并记录可复现方法。
- 文档：完成部署、配置、架构、安全边界、开发、测试、故障排查、进度和 TODO。
- 收尾：处理 Task 9 审查 minor，执行完整分支最终审查。
- 当前分支：`feature/infraview-mock-mvp`
- Task 9 完成 HEAD：`952a6a5 feat: add secure single-container deployment`
- 隔离 worktree：`/root/github/InfraView/.worktrees/infraview-mock-mvp`

- [Mock MVP 实施计划](superpowers/plans/2026-07-20-infraview-mock-mvp.md)
- 持久子代理进度账本：`.superpowers/sdd/progress.md`（本地忽略文件）。

## 下一步

1. 完成 Task 10 E2E、缓存 API 延迟、资源占用和最终文档验收。
2. 对完整 Mock MVP 分支进行最终代码审查并处理发现项。
3. 夜莺测试环境就绪后，进入真实适配器集成阶段。

## 当前阻塞项

- Nightingale 的版本、API 地址与认证方式尚未确定。因此当前阶段不实现或猜测真实 Nightingale API，只保留适配接口与配置边界。
- 当前无 Task 10 代码阻塞；Nightingale 测试环境仍不属于 Mock MVP 验收前置条件。

## 验证状态

- 应用代码：Task 1–9 已实现并通过独立规格与质量审查。
- 自动化测试：截至 Task 9，前端 unit 50/50、typecheck、build 通过；全仓 Go 普通及 race 测试通过；生产和全量 npm audit 均为 0 漏洞。
- Docker 镜像：多阶段镜像已构建并通过 Compose 冒烟；约 16.4 MB，运行内存约 9.43 MiB，健康检查和安全约束生效。
- 设计规格：已确认并提交。
- 实施计划：已编写并处于执行中。
- 已知非阻塞项：Task 3 缺少 current TTL 20 秒的直接服务级默认值回归；ECharts 应在最终阶段考虑懒加载并优化重复生命周期；jsdom 不执行 canvas 初始化生命周期，计划在 Task 10 Playwright 覆盖；当前生产主 JS chunk 约 826 kB，超过 Vite 500 kB 警告阈值；静态资源 immutable 应显式限制为指纹文件；smoke 登录 JSON 需支持特殊字符凭据；本地忽略的 `.env` 当前仍为示例凭据，正式部署前必须修改。
