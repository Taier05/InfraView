# InfraView 项目状态

最后更新：2026-07-22

## 当前阶段

Mock MVP 的 Task 1–10 已完成，并通过完整分支最终独立审查。最终结论为 Critical 0、Important 0；功能分支已通过 merge commit 合入并推送到 `main`，当前 Mock 版本已在局域网设备部署运行。

当前正在 `feature/compact-layout` 分支制作 v0.1.1 紧凑布局预览，等待用户在浏览器中确认。第一轮已缩窄侧栏；第二轮按用户反馈保持左侧不变，只将右侧工作区提升为 dense 密度：进一步缩短顶栏和标题区、减少页面/卡片留白、降低图表和表格高度、让主机筛选同排、详情当前指标宽屏自适应多列并将历史趋势改为桌面双列。该预览不改变 API、数据源、认证或只读安全边界。

总览页采用可扩展的“板块状态卡”模式：当前只保留一张普通单列宽度的可点击 `Linux 主机` 卡，集中显示总数、在线、离线和未知状态，点击整张卡进入主机列表。总览不再展示 CPU、内存汇总卡、资源趋势或时间范围控件；后续服务、网络、存储等板块按相同尺寸逐张加入。

- 当前分支：`main`
- 主工作树：`/root/github/InfraView`
- 保留的开发 worktree：`/root/github/InfraView/.worktrees/infraview-mock-mvp`
- 合并提交：`f901023 merge: deliver InfraView Mock MVP`
- 发布标签：`v0.1.0-mock`
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

## 当前部署

- 访问地址：`http://192.168.8.200:8080`
- 数据源：确定性 Mock，32 台主机。
- Compose 服务：`infraview`，健康状态正常。
- 运行验证：smoke PASS；局域网首页可访问；内存约 `7.4 MiB`。
- 容器安全：UID/GID 10001、只读根文件系统、capabilities 全删、禁止提权、`restart: unless-stopped`。
- 登录凭据仅保存在主工作树被 Git 忽略且权限为 600 的 `.env`，文档和仓库不记录密码。

## 下一步

1. 由用户从浏览器确认紧凑布局预览；确认后再合并并发布 v0.1.1。
2. Nightingale 测试环境就绪后，按已记录证据要求进入第二阶段规格和实现。
