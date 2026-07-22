# InfraView 项目状态

最后更新：2026-07-22

## 当前阶段

Mock MVP 的 Task 1–10 已完成，并通过完整分支最终独立审查。最终结论为 Critical 0、Important 0；当前分支已具备交付条件，但尚未推送或合并。

- 分支：`feature/infraview-mock-mvp`
- 隔离 worktree：`/root/github/InfraView/.worktrees/infraview-mock-mvp`
- Task 10 基线：`ee673f6 docs: record task 9 completion`
- Task 10 交付提交：`d401f0b test: verify and document InfraView MVP`。
- Task 10 隔离修复：`a2736f4 fix: isolate e2e compose projects`。
- 最终分支修复：`525f343`（部分 E2E 清理）、`17b26fe`（可信代理/重启策略）、`d88138d`（前端状态/刷新/会话失效）。
- 最终验证修复：`3c5e5c0`（测试夹具）、`9bee36e`（镜像内 Compose 规格测试）、`886a695`（代理信任与 Caddy/Nginx 对齐）。
- 实施计划：`docs/superpowers/plans/2026-07-20-infraview-mock-mvp.md`
- 本地 SDD 账本：`.superpowers/sdd/progress.md`，被 Git 忽略，不提交。

## 已完成范围

- 固定账号登录、内存会话、并发安全登录限速。
- 稳定数据源领域契约、确定性 Mock、Nightingale 未配置空壳。
- TTL 缓存、相同请求合并、最多 5 分钟旧值降级。
- 总览、主机列表、单机详情和数据源状态只读 API。
- 深色中文 React UI、四档时间范围、刷新、搜索、筛选、排序、URL 状态和分页。
- 导航展示数据源健康、异常、过期、请求失败和最近检查时间；主机列表支持手动及 30 秒非重叠刷新。
- 受保护 API 会话失效后集中清缓存并返回登录页。
- Go 同源托管 SPA；单一安全 Compose 容器直接暴露 IP:端口。
- Playwright Chromium 覆盖关键路径、真实 canvas、stale/error、退出/重定向和无破坏性控件。
- ECharts 动态加载：主 JS 约 330.78 kB，图表块约 495.23 kB，无 500 kB 构建警告。
- 完整架构、设计、配置、部署、开发、测试、安全和 Nightingale 前置证据文档。

## 最终验收证据

- 前端：Vitest 60/60、typecheck、build 通过；生产及全量 npm audit 均为 0。
- Go：全仓普通测试、race、格式检查和构建通过；镜像构建阶段重复执行普通/race 测试。
- Smoke：含双引号和反斜杠的账号密码 JSON 转义后 PASS。
- Playwright：真实 Chromium 3/3，通过总览和详情 4 canvas 验证。
- 缓存延迟：100 次顺序认证 overview 请求，第 95 样本 `0.003819s`，低于 `0.200s`。
- 资源：内存 `10.210 MiB`，低于 256 MiB；镜像 `16.64 MB`。
- 隔离：专用项目/18080 在成功、碰撞和部分启动失败路径均受精确清理保护；最终只保留原有 `cliproxyapi`。
- 审查：完整分支最终复审 Approved，未发现破坏性运维能力或代理信任阻断问题。

## 安全与产品边界

平台只读展示，不提供修改、删除、重启、发布、命令、脚本、任意查询或代理；不执行远程命令或自动化变更。无业务数据库，监控历史和会话不落盘。

## 当前阻塞

真实 Nightingale 集成缺少精确版本、认证和实际只读 API 证据。该阻塞不影响 Mock MVP；在证据就绪前禁止猜测实现。

## 下一步

1. 用户明确要求后再决定合并/推送；当前不推送、不合并。
2. Nightingale 测试环境就绪后，按已记录证据要求进入第二阶段规格和实现。
