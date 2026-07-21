# InfraView 项目状态

最后更新：2026-07-21

## 当前阶段

Mock MVP 正在隔离分支 `feature/infraview-mock-mvp` 中按子代理驱动和测试驱动方式实施。Task 1–7 已实现并通过任务级规格与质量审查。当前按用户要求暂停在 Task 8 主机详情页的 RED 阶段，等待下次会话继续。

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

## 当前任务

Task 8 已完成 RED 准备，尚未写生产页面：

- 已修改但未提交：`web/src/test/fixtures.ts`
- 已新增但未提交：`web/src/features/hosts/HostDetailPage.test.tsx`
- 已确认聚焦测试因 `./HostDetailPage` 不存在而失败，属于预期 RED。
- `HostDetailPage.tsx` 尚未创建，没有需要回滚的部分生产实现。
- 当前分支：`feature/infraview-mock-mvp`
- 当前 HEAD：`900d62f fix: canonicalize host list pagination`
- 隔离 worktree：`/root/github/InfraView/.worktrees/infraview-mock-mvp`

- [Mock MVP 实施计划](superpowers/plans/2026-07-20-infraview-mock-mvp.md)
- 持久子代理进度账本：`.superpowers/sdd/progress.md`（本地忽略文件）。

## 下一步

1. 从现有 Task 8 RED 测试继续，创建 `HostDetailPage.tsx` 并实现独立 current/history 查询。
2. 完成 Task 8 unit/typecheck/build/audit、任务提交与独立审查。
3. 完成 Task 9 单容器 Docker Compose 与冒烟验证。
4. 完成 Task 10 E2E、性能、资源占用和最终文档验收。
5. 夜莺测试环境就绪后，进入真实适配器集成阶段。

## 当前阻塞项

- Nightingale 的版本、API 地址与认证方式尚未确定。因此当前阶段不实现或猜测真实 Nightingale API，只保留适配接口与配置边界。
- 本次暂停是用户主动要求的上下文/token 检查点，不是代码或测试阻塞。

## 验证状态

- 应用代码：Task 1–7 已实现并审查通过；Task 8 只有未提交测试与 fixture，生产页面尚未创建。
- 自动化测试：截至 Task 7，前端 unit 32/32、typecheck、build 通过；全仓 Go 普通测试、race、vet、build、gofmt 检查通过；生产和全量 npm audit 均为 0 漏洞。Task 8 聚焦测试当前为预期 RED。
- Docker 镜像：尚未创建。
- 设计规格：已确认并提交。
- 实施计划：已编写并处于执行中。
- 已知非阻塞项：Task 3 缺少 current TTL 20 秒的直接服务级默认值回归；ECharts 应在最终阶段考虑懒加载并优化重复生命周期；jsdom 不执行 canvas 初始化生命周期，计划在 Task 10 Playwright 覆盖。
