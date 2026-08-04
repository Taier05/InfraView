# InfraView TODO

最后更新：2026-08-01

## Mock MVP

- [x] Go 配置、固定账号、内存会话与登录限速。
- [x] 稳定数据源契约、确定性 Mock 和 Nightingale 初始占位实现。
- [x] TTL 缓存、相同请求合并、stale 降级和只读 API。
- [x] 深色中文总览、主机列表、详情和真实趋势图。
- [x] 1h/6h/24h/7d、刷新、搜索、筛选、排序、URL 状态和分页。
- [x] Go 同源 SPA、安全单容器 Docker Compose 和健康检查。
- [x] Playwright 总览/主机清单关键路径、指标列、stale/error、退出和只读控件验收。
- [x] 缓存命中 100 请求 P95 小于 200 ms。
- [x] 100 台配置边界与空闲内存低于 256 MiB。
- [x] 部署、配置、架构、设计、开发、测试、安全和上下文恢复文档。
- [x] 完成 Task 10 最终 `make verify` 等价全量验证。
- [x] 完成 Mock MVP 独立分支首轮代码审查并处理全部 Critical/Important findings。
- [x] 完成修复后的统一全量验证与最终复审（Critical 0、Important 0）。
- [x] 将总览主机卡升级为阈值告警摘要，展示异常主机和 CPU/内存/IO/网络分级数量。
- [x] 主机列表支持按 URL 保持每页 20、50、100 条并在切换时重置页码。
- [x] 主机列表增加可选 CPU 核数与内存容量配置列，Mock 提供稳定配置样本。
- [x] 统一主机列表非状态值样式，并确保超长主机名在自身列内省略且不覆盖 IP。

## Nightingale 里程碑

- [x] 完成认证准备：精确版本、基础 URL、TLS 状态、`X-User-Token` 和私密 Token 文件权限均已确认。
- [x] 只读采集主机、当前指标、历史范围、空结果和 401 错误响应证据。
- [x] 记录字段、标签、单位、时间戳、分页和批量语义，并确认 CPU 核数、内存总容量的真实字段。
- [x] 制作完全脱敏的契约夹具。
- [x] 编写独立规格/计划并以 TDD 实现 Nightingale 适配器。
- [x] 验证超时、大小限制、缓存、stale、分页、批量和 100 台规模。
- [x] 使用真实 Nightingale 构建、切换 8080 服务并完成 API 与 Chromium 验收。
- [x] 完成第二阶段审查阻断项 TDD 修复与最终复审：HTTP 重定向/响应形状/精确 Content-Type、分页与批量基数、配置隔离/Base URL、未知历史主机、指标与 `beat_time` 数值边界、成功/失败 discovery flight 合并，以及 PromQL 白名单回归覆盖；最终范围复审 PASS。
- [x] 将 Nightingale v8.4.1 设为主要开发与真实验证版本，完成只读契约预检，并以 TDD 增加 Target `update_at` 回退；v9.x 保留协议兼容。
- [x] 使用更新后的私密环境文件重建 8080 服务，完成 v8.4.1 登录、数据源、主机、当前指标和历史指标只读 smoke。
- [x] 将 v8.4.1 兼容功能快进合并并推送到 `origin/main`，清理原功能分支和 worktree。

## 已知后续改进

- [ ] 跟踪 React Router `GHSA-qwww-vcr4-c8h2` 的兼容修复版本；当前 SPA 不使用受影响的 RSC Action，禁止直接执行 npm 建议的 `audit fix --force` 强制降级。
- [x] 通过数据源状态 API 下发只读运行时刷新周期，当前页面统一使用默认 15 秒。
- [ ] 扩展缓存结果契约，可靠记录并输出 cache hit/miss 日志字段。
- [x] 数据源状态 API 返回真实数据源类型，界面按运行时配置显示 `Mock` 或 `Nightingale`。
- [x] 将左下角单数据源卡片升级为紧凑数据连接汇总，正常时收起、异常与 Mock 明确提示，并预留多数据源明细结构。
- [ ] 在每个私有部署环境中验证 Nightingale 使用专用最小只读 Token；公开仓库不记录实际账号或凭据权限。
- [ ] 生产 Categraf 升级后至少等待两个采集周期，只读确认 `diskio_io_util` 是否出现，以及 InfraView 自动刷新后 IO 忙碌度是否恢复展示。
- [ ] 若 Categraf 升级后 IO 仍缺失，只收集脱敏契约证据；未经用户明确授权，不替换或增加 IO PromQL。获得授权后按 TDD 实现必要兼容。
- [ ] 如需恢复磁盘容量/读写历史展示，先采集并确认真实指标契约；当前返回空序列，不猜测指标名。

## 明确不做

- 修改、删除、重启、发布、配置下发或任务执行。
- SSH、远程命令、脚本执行或自动化变更。
- 用户/RBAC、告警事件页面、任意查询/代理和监控历史持久化。
- MySQL 历史、实例详情、写接口、数据库连接或运维操作。
- 硬盘详情/历史、SMART 扫描/自检、修复、启停、擦除、块设备访问、`smartctl`、`nvme-cli` 或任何远程控制。

## 主机硬盘 SMART 模块

> 2026-08-01：下列 17 组/九列项目是初始模块的历史完成记录；当前容量增量见后续独立小节。

- [x] 以 RED→GREEN 实现独立硬盘领域/Mock、稳定不可逆 ID 和原始 WWN/序列号字段排除。
- [x] 以 RED→GREEN 实现 Nightingale 一次固定 17 查询即时 batch、脱敏归并、安全错误和无主机/设备 N+1。
- [x] 以 RED→GREEN 实现 DiskService 独立默认 60 秒 TTL/freshness、120/300 秒边界、恢复/回退/stale、状态聚合与列表查询。
- [x] 固化六值 `status_source` 和同级设备来源优先规则；前端不通过等级相等猜测采集来源。
- [x] 以 RED→GREEN 实现两条受认证 GET API、9 列 `/disks` 页面、侧边栏入口和独立总览卡。
- [x] Task 8 与终审修复后的无端口 Docker 静态扫描、Go 普通/race、前端 8 文件/99 测试、typecheck/build 与无缓存最终镜像构建均退出 0；持久结果见硬盘实施计划与 `docs/TESTING.md`。
- [x] 整功能终审的 1 个 Important 和 2 个 Minor 均已完成 RED→GREEN，唯一范围复审 Approved。
- [x] 修复测试 Nightingale 第 17 组兼容重复历史身份导致的 503；顺序无关归并、可选身份冲突和元数据冲突均完成 RED→GREEN及范围复审。
- [x] 经授权原位重建仅连接测试 Nightingale 的现有 8080，完成容器安全、脱敏 API、一次性 Chromium 1440×900 和跨两个 60 秒周期推进验收；未创建其他 InfraView 端口。
- [ ] 会创建 18080 的 `scripts/e2e.sh` 未执行；本轮已用不发布端口的一次性 Chromium 覆盖现有 8080。
- [ ] 提交、推送均需分别取得明确授权；任何生产 Nightingale 验证永久禁止。

## 总览四槽位与硬盘展示细化

- [x] 恢复桌面四个固定总览槽位，并保持中屏两列、窄屏一列的既有响应式规则。
- [x] 将硬盘型号与容量拆为独立两行，并把 unsafe shutdown 文案收紧为“异常断电 N 次”的累计展示语义。
- [x] 完成 Docker 离线前端全量验证（8 文件/101 测试、typecheck、production build）和无缓存镜像 `infraview:overview-disk-display-verify`；镜像内 Go 普通/race 与编译也已通过。
- [x] 在 Playwright 硬盘规格中加入首行 `.disk-model` 与 `.disk-capacity` 可见性断言，并在授权后的现有 8080 一次性 Chromium 中完成现场验证。
- [x] 经单独授权原位重建仍连接测试 Nightingale 的既有 8080，完成 healthz、非 root、只读根文件系统、cap drop、禁止提权和唯一 8080 端口检查；未读取或输出私密环境文件。
- [x] 使用安全进程凭据在现有 8080 完成不发布端口的 Chromium 验收；未创建额外端口、截图或保留 trace。

## 硬盘容量独立指标与分列

- [x] 以 RED→GREEN 将固定即时 batch 扩展为 18 组；第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组 inventory 负责设备发现、型号与原始最后样本时间，仍只有一次 batch、无 N+1。
- [x] 覆盖容量缺失、负数、小数、NaN、Inf、越界、最新值、同时间冲突、未知设备与身份冲突；旧 inventory `capacity` 标签不读取或回退，容量不影响 `ReportedAt`/freshness/状态。
- [x] 为 Service/API 增加精确 `int64` 容量排序；升降序缺失值始终最后，稳定 ID 收口。
- [x] 将硬盘页拆成型号、容量独立十列，容量可升降序；分页仍为 20/50/100，未增加命令超时、失联移除或“全部”。
- [x] 定向 Go 普通/race、前端硬盘页 14 项、typecheck/build 和 Playwright 静态发现已通过。
- [x] 完成全量离线测试、race、生产构建、无缓存镜像与最终差异检查：前端 8 文件/101 项，Go 普通/race 与编译、Playwright 13 项静态发现均退出 0。
- [x] 经单独授权原位重建仍只连接测试 Nightingale 的现有 8080，完成容器安全、脱敏 API、硬盘十列/容量排序 Chromium 和总览四槽位验收；未创建其他端口，任何生产 Nightingale 验证永久禁止。
- [x] 经明确授权完成提交与推送，硬盘 SMART 与容量增量功能基线为 `6300413`。

## MySQL 后续验收

- [x] 本地 Docker/Mock 验收：无缓存生产镜像、独立 race、E2E 隔离安全测试、smoke、资源检查和 Chromium 已完成；专用 E2E 项目资源已清理。
- [x] 完成 MySQL 紧凑布局验收：总览桌面四列紧凑模块位，MySQL 11 列在 1440×900 下无页面或表格横向滚动；仅重建测试 Nightingale 的原 8080，未创建其他测试端口。
- [x] 经用户授权，在仅连接测试 Nightingale 的原 8080 完成 MySQL Chromium 页面验收；未创建额外 InfraView 端口，未输出私密环境或真实实例/指标。
- [x] 修复最终审查发现的 batch `null` 假空快照和总览四类互斥状态问题，并完成范围化复审、完整构建及原 8080 回归。
- [x] 完成 MySQL 表格细化：新增 Buffer Pool 容量、稳定实例标签精确筛选、11 列语义收紧、状态列表头对齐和原 8080 Chromium 5/5；固定 14 条查询仍为一次 batch、无 N+1。
- [x] 实现 Linux/MySQL 采集新鲜度：默认 15 秒预期周期，2 个周期警告、5 个周期严重；MySQL 以 24 小时近期身份避免停止采集后立即消失。
- [x] 修复采集新鲜度时间口径：Linux/MySQL 均使用 `tlast_over_time` 的原始最后样本时间，Target 更新时间和即时查询外层时间不再掩盖停采。
- [x] 消除稳定链路时延误报：Linux/MySQL 按原始样本是否推进计算 2/5 周期等级，并统一主机采集延迟黄色、采集失联红色。
- [x] 增加显式事务 TPS，并将 MySQL 清单收紧为 10 列、地址专用搜索和 IP/端口自然排序；固定 16 条查询仍为一次 batch、无 N+1。
- [x] 经单独授权，仅原位重建连接测试 Nightingale 的原 8080，并完成本轮匿名 Chromium 验收；未连接生产或创建额外 InfraView 端口。
- [ ] 按真实契约证据再决定是否扩展 MySQL 指标；当前固定 16 条查询、一次 batch 和无 N+1 设计不接受前端自定义查询。

## Redis Cluster 首期模块

- [x] 以 RED→GREEN 实现独立 Redis 领域、不可逆稳定 ID、深拷贝和确定性 Mock。
- [x] 实现 Nightingale 一次固定 21 查询即时 batch、无 N+1、24 小时 inventory/角色补充、严格数值归并与安全错误。
- [x] 实现独立 RedisService 缓存/freshness、2/5 周期采集等级，以及可用性、内存、拒绝连接和主从复制阈值。
- [x] 实现两个受认证只读 GET API、严格参数白名单、405、安全 503 和显式脱敏 view model。
- [x] 实现总览第四卡、侧边栏入口、300ms 搜索防抖、URL 状态、服务端分页及完整加载/错误状态；实例页复用统一控制栏与刷新状态。
- [x] 增加 Redis 页面浏览器静态规格；不会创建额外端口的 `playwright test --list` 已通过。
- [x] 首次原位重建现有 8080 后，以 RED→GREEN 修复总览角色字段大小写契约，并统一列表控制栏、刷新状态、最终列名和复合指标可读性；修复后前端 9 文件/108 项、Go 全仓普通/race/编译及无缓存生产镜像均通过。
- [x] 经明确授权原位重建当前修复到现有 8080；健康、唯一 8080 映射和容器安全基线检查通过，未创建其他端口或项目。
- [x] 建立共享列表页与总览卡展示组件，Redis 首个接入；新增模块必须复用，Linux/硬盘/MySQL 后续触及时渐进迁移。
- [x] 将 Redis 实例页调整为十一列，拆分内存上限/使用率与复制链路/延迟；页面移除过期/淘汰列但保留后端与排序兼容。
- [x] 完成本次十一列/共享模板全量离线验证：前端 11 文件/112 项、typecheck/build、Playwright 14 项静态发现、Go 普通/race/编译和无缓存生产镜像均通过。
- [x] 经授权将十一列/共享模板原位重建至现有 8080，并通过健康、唯一端口、安全基线和新列静态资源检查；生产 Nightingale 永久禁止。
- [x] 按 RED→GREEN 将 Redis 运行时间纠正为与主机/MySQL 相同的天/小时格式。
- [x] 按长期授权将运行时间修复直接原位重建至现有测试 8080；镜像内全量验证、健康、唯一端口、安全基线和部署资源匹配通过。
- [x] 记录长期部署规则：后续每次已授权修复验证通过后自动原位重建同一测试 8080，无需再次询问；提交、推送、生产 Nightingale 和额外端口不在授权范围。
- [x] 提交前范围审查以 RED→GREEN 修复 Redis `math.MaxInt` 页码偏移溢出，查询规范化安全返回 `ErrInvalidQuery`，正常分页保持不变。
- [x] 完成审查修复后的镜像内前端/Go 全量验证、Playwright 静态发现和现有测试 8080 自动原位重建；健康、唯一端口、安全基线和部署资源匹配通过。
- [x] 经明确授权提交并推送 Redis 首期模块、共享模板及相关测试和文档；功能基线为 `c3b5c7d`，已进入 `origin/main`。

首期明确不做 Redis 拓扑图、slot 分布、实例详情、历史趋势、故障转移、主从切换、Redis 直连或命令执行。

## Elasticsearch 首期模块

- [x] 实现独立 Elasticsearch 领域、完全脱敏多集群 Mock 与不可逆集群/节点稳定 ID。
- [x] 实现 Nightingale 一次固定 26 查询即时 batch、inventory-first 归并、安全错误和无集群/节点 N+1。
- [x] 实现共享快照缓存、集群/节点独立 freshness、状态来源优先、磁盘/JVM/拒绝阈值、筛选/16 字段排序/安全分页。
- [x] 实现两个受认证 GET API、显式 View、非 null 数组、参数白名单、405 与安全 503。
- [x] 实现总览第五卡、侧边栏和复用 `ListPage` 的 16 列节点页；每格单值单行，角色超过两个时显示前两个与 `…`、完整值保留在 `title`。
- [x] 新增 Elasticsearch Playwright 静态规格，并完成 3 文件/17 项静态发现；未运行动态 E2E 或创建端口。
- [x] 完成离线全量验证：前端 12 文件/154 项、typecheck/build；Go gofmt/vet/普通/race/编译；无缓存镜像与只读/敏感静态扫描。
- [x] 终审发现的 health `color`、inventory 身份/地址归并、采集来源拆分与浮点求和顺序均完成单项 RED→GREEN。
- [x] 完成终审修复后的 provider/service 定向普通+race、Elasticsearch 页面 28 项状态回归和 gofmt 汇总。
- [x] 完成主控 fresh 最终全量：前端 12 文件/155 项、typecheck/build、Playwright 3 文件/17 项静态发现、Go gofmt/vet/全仓普通/race/编译、只读/敏感/whitespace 扫描和无缓存镜像均退出 0。
- [x] 经单独授权原位重建现有测试 Nightingale 8080；两个 GET API、写方法 405、五卡/16 列 Chromium、容器安全与既有模块回归验收均通过，未创建其他端口。
- [x] 修复 uptime 小数/科学计数文本被严格整数解析拒绝的问题，并完成角色前两个 + `…`、集群健康四色徽标及 1440×900 表格无横向滚动。
- [x] 完成密度/uptime 修复 fresh 全量（前端 12 文件/157 项、typecheck/build、Playwright 17 项静态发现、Go gofmt/vet/普通/race/编译、无缓存镜像），并原位重建原 8080；脱敏 API 与 Chromium 3/3、唯一端口和安全基线均通过。
- [x] 按 RED→GREEN 为 Elasticsearch 总览卡补齐“异常节点 x / 总节点”、严重与警告/未知徽标，以及集群和节点同时为空时的共享空状态；集群与节点继续分开统计。
- [x] 完成节点汇总修复的现有 8080 原位重建、Chromium 3/3、唯一端口和容器安全验收。
- [ ] 经明确授权提交 Elasticsearch Task1–7；push 仍需独立明确授权。

首期明确不做索引列表/详情、节点详情、历史趋势、拓扑、分片详情、慢查询、日志、追踪、Elasticsearch 直连、任意查询或任何运维操作。任何生产 Nightingale/Elasticsearch 验证永久禁止。
