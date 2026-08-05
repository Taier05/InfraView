# InfraView 开发交接

最后更新：2026-08-05

## 当前 main 恢复入口

当前工作目录为 `/root/github/InfraView`、分支为 `main`。RabbitMQ Task 1–11 已在共享工作树实现并等待已授权提交：Task 1–9 提供独立领域/Mock、固定 22 查询 Nightingale Provider、共享快照 Service、两个认证 GET API、总览第六卡、侧边栏、共享 `ListPage` 上的 15 列节点页与合成 Playwright 规格；Task 10 修复同一采集目标下多个节点被覆盖并统一总览全零摘要；Task 11 用连接指标补充发现身份结果暂缺的实例，并彻底移除实例地址/`ident` 节点名称推测。恢复时必须保留全部 staged、tracked unstaged 与未跟踪计划差异，不得 reset、restore、checkout 或 clean，并先核对 `git status --short --branch`、`git diff --check` 与 `git diff --cached --check`。

RabbitMQ 固定查询仍恰好一次 batch、无集群/节点/指标 N+1；当前与近期身份建立具名节点，连接指标只补充发现缺失实例，`cluster + instance` 只做候选索引。同批结果仅在显式 `rabbitmq_node` 唯一一致时补全名称，缺失或冲突时 API 名称为空且页面显示“暂无数据”；实例地址和 `ident` 不得冒充名称。带节点标签的指标精确归并，歧义的无节点标签指标保持缺失；两条吞吐查询保留节点维度。集群通信和节点状态严格分离，既有阈值、freshness、只读 API 和页面列不变。Task 11 前后端 RED→GREEN、定向测试及镜像内前端/Go 全量、race、编译均通过。现有 8080 已按授权原位重建，健康、唯一端口和错误回退移除检查通过；动态登录态 Chromium 未执行。禁止新建任何 InfraView 端口、连接生产 Nightingale/RabbitMQ、读取或输出私密环境内容、认证信息与上游正文。

以下 Elasticsearch 记录属于上一已交付基线，必须保留但不是当前 RabbitMQ 工作树状态：Elasticsearch 功能基线为 `80da061 feat: add read-only Elasticsearch monitoring`，此前已批准的设计提交 `6541556` 与功能提交均已推送到 `origin/main`。2026-08-04 提交前 fresh 无缓存生产镜像 `infraview:elasticsearch-precommit-verify` 已完成前端 12 文件/159 项、typecheck/build、Go 全仓普通/race 和编译，均退出 0。总览异常节点汇总版本已按授权原位重建现有 8080，Chromium 3/3、唯一端口和容器安全验收通过；继续只连接测试 Nightingale。未运行会创建 18080 的动态 E2E 栈，也未连接生产 Nightingale/Elasticsearch。

历史 Redis 功能基线为 `c3b5c7d feat: add read-only Redis monitoring`，已推送到 `origin/main`；此前交接基线 `cfe4a40` 与 SMART 功能提交 `6300413` 也均在 `origin/main`。这些 Redis/SMART 的现场验收、8080 重建和提交/推送记录只属于历史功能，不是当前 Elasticsearch 结果。继续开发前仍须先做 Git 只读检查；如实际工作区出现用户后续差异，必须保留，不得清理或回退。

2026-08-01 用户已明确授权提交并推送 Redis Cluster 首期模块与共享观测模块模板；功能提交 `c3b5c7d` 已进入 `origin/main`。最终十一列、运行时间天/小时纠错及极端页码溢出保护均已交付；现有 8080 已按长期授权原位重建，继续只连接测试 Nightingale，未连接、切换或探测生产 Nightingale，也未创建其他 InfraView 端口。

2026-08-01 容量增量已实现并完成全量离线及现有 8080 现场验收：硬盘固定即时 batch 从 17 组增至 18 组，第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组 inventory 继续负责设备发现、型号和原始最后样本时间；旧 inventory `capacity` 标签不再读取或回退。容量从原始文本精确解析为 `int64`，不经过 `float64` 丢失大整数精度。硬盘页由九列改为“型号”“容量”分列的十列，容量支持服务端升降序且缺失值始终最后。前端 8 文件/101 项、typecheck/build、Go 格式/普通/race/编译、Playwright 静态发现和无缓存镜像均通过。经用户单独授权，现有 8080 已原位重建并继续只连接测试 Nightingale；容器安全、脱敏 API、十列 Chromium 和总览四槽位验收均通过。

在新账号或新对话中直接粘贴：

```text
继续开发 InfraView。请始终使用简体中文回复。

工作目录：/root/github/InfraView
分支：main

请先完整阅读并遵循：
1. docs/HANDOFF.md
2. docs/PROJECT_STATUS.md
3. docs/TODO.md
4. docs/datasources/NIGHTINGALE.md
5. docs/superpowers/specs/2026-07-29-collector-freshness-and-mysql-tps-design.md
6. docs/superpowers/plans/2026-07-29-collector-freshness-and-mysql-tps.md
7. docs/superpowers/specs/2026-07-30-raw-sample-freshness-fix-design.md
8. docs/superpowers/plans/2026-07-30-raw-sample-freshness-fix.md
9. docs/superpowers/specs/2026-07-30-sample-progress-freshness-fix-design.md
10. docs/superpowers/plans/2026-07-30-sample-progress-freshness-fix.md
11. docs/superpowers/specs/2026-07-30-host-disk-smart-module-design.md
12. docs/superpowers/plans/2026-07-30-host-disk-smart-module.md
13. docs/superpowers/specs/2026-07-30-overview-four-slot-and-disk-display-refinement-design.md
14. docs/superpowers/plans/2026-07-30-overview-four-slot-and-disk-display-refinement.md
15. docs/superpowers/specs/2026-08-01-disk-capacity-metric-and-column-design.md
16. docs/superpowers/plans/2026-08-01-disk-capacity-metric-and-column.md
17. docs/superpowers/specs/2026-08-01-redis-cluster-module-design.md
18. docs/superpowers/plans/2026-08-01-redis-cluster-module.md
19. docs/superpowers/specs/2026-08-01-elasticsearch-module-design.md
20. docs/superpowers/plans/2026-08-01-elasticsearch-module.md
21. docs/superpowers/specs/2026-08-04-elasticsearch-table-density-and-uptime-fix-design.md
22. docs/superpowers/plans/2026-08-04-elasticsearch-table-density-and-uptime-fix.md
23. docs/superpowers/specs/2026-08-04-elasticsearch-overview-node-summary-design.md
24. docs/superpowers/plans/2026-08-04-elasticsearch-overview-node-summary.md
25. docs/superpowers/specs/2026-08-04-rabbitmq-cluster-and-node-module-design.md
26. docs/superpowers/plans/2026-08-04-rabbitmq-cluster-and-node-module.md

先只读执行：
git status --short --branch
git log -3 --oneline
git diff --check
git diff --cached --check

InfraView 始终只读。当前 RabbitMQ Task 1–11 差异是有意未提交状态，必须全部保留，不得清理。RabbitMQ 首期仍为总览第六卡与 15 列节点页，固定一次 22 查询 batch、无 N+1；连接指标补充发现身份暂缺实例，节点名称只接受唯一一致的显式标签，缺失显示“暂无数据”，并统一总览全零摘要为“无异常”。2026-08-05 fresh Go/前端验证与现有 8080 原位重建已完成，健康、唯一端口与容器安全基线通过。动态登录态 Chromium 未执行，提交和推送已获授权、尚待执行。禁止读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文，任何生产 Nightingale/RabbitMQ 验证永久禁止。

Elasticsearch 历史基线为 `80da061`，其提交前 fresh 全量、现有 8080 原位重建、Chromium 3/3、唯一端口和容器安全验收均已通过。该历史证据不能用于声明当前 RabbitMQ 已全量验证或已部署。

Redis 为独立垂直模块，固定 21 查询一次 batch、无 N+1，15 秒预期周期与 2/5 周期 freshness。共享 `ListPage` 与 `ModuleStatusCardShell`、Redis 首个接入和最终十一列已完成全量离线验证并原位重建至现有 8080。运行时间已纠正为主机/MySQL 的天/小时格式；Dockerfile 内前端/Go 全量验证、容器健康、唯一 8080、安全基线和部署资源匹配均通过。功能提交 `c3b5c7d` 已推送到 `origin/main`，未连接生产 Nightingale。

历史 2026-07-30 展示细化状态：总览已恢复桌面四槽位；当时硬盘“型号 / 容量”改为同列独立两行，unsafe shutdown 显示为“异常断电 N 次”，且仅表示累计展示、不参与状态判断。Docker 离线前端全量回归（8 文件/101 测试、typecheck、build）及无缓存全仓镜像 `infraview:overview-disk-display-verify` 已通过。用户随后授权原位重建仍连接测试 Nightingale 的现有 8080：服务 healthy，保持非 root、只读根文件系统、cap drop、禁止提权且只发布原 8080；脱敏登录态只读 API 与一次性 Chromium 1440×900 验收通过。浏览器确认总览四轨前三格占用、第四格自然空，硬盘九列、型号/容量同列两行、无横向溢出、无破坏性控件和无非预期登录后错误。这些现场证据不覆盖 2026-08-01 的 18 组/十列增量。未读取私密环境文件，未输出凭据、API 正文或现场值，未创建其他 InfraView 端口，也未运行 `scripts/e2e.sh`。提交、推送各自需要明确授权；任何生产 Nightingale 验证永久禁止。
```

## Elasticsearch 当前暂停点

- 独立 Elasticsearch 垂直模块已实现：一次固定 26 查询即时 batch 同时构建集群/节点快照，无 N+1、无 Elasticsearch 直连、无任意 PromQL。集群 ID 只使用 `cluster`，节点 ID 只使用 `cluster + name`；采集身份和原始标签不进入 API。
- 集群来源固定为 `availability|health|collection|normal|unknown`，节点来源固定为 `collection|disk|jvm|thread_pool|normal|unknown`。集群黄/红不污染节点；默认 15 秒周期连续 2/5 周期未推进为 warning/critical，磁盘 85%/90%、JVM 堆 75%/85%、拒绝速率大于 0 的阈值已覆盖。
- HTTP 仅有 `GET /api/v1/elasticsearch/overview` 与 `GET /api/v1/elasticsearch/nodes`；显式 View、非 null 数组、400/405/503、stale 和敏感字段排除均有测试。
- 总览第五卡复用 `ModuleStatusCardShell`；节点页复用 `ListPage`，固定 16 个单值单行列，无 `<br>`。角色超过两个时仅显示前两个与 `…`，完整值在 `title`；集群健康复用四色状态徽标。1440×900 页面和表格均无横向滚动，更窄视口由 `.elasticsearch-table-scroll` 兜底。
- uptime 空值根因是上游合法小数/科学计数文本被严格 `ParseInt` 拒绝；现仅对 uptime 使用有限非负浮点解析、越界检查和向下取整，其他整数指标路径不变。
- 2026-08-04 终审修复后的主控 fresh 最终全量：Playwright 静态发现 3 文件/17 项（Elasticsearch 3 项）；前端 12 文件/155 项、typecheck/build；Go gofmt/vet/全仓普通/race/编译；无缓存镜像 `infraview:elasticsearch-final-verify`；只读/敏感/whitespace 扫描均退出 0。既有 warning 为 npm 1 个 moderate、2 个 high 与 Vite 第三方 `"use client"`。
- 现有 8080 已经单独授权原位重建并完成脱敏登录态 API、1440×900 Chromium、唯一端口、容器安全和既有模块回归验收；全部只输出固定布尔结论，没有记录现场数据。任务报告位于 `.superpowers/sdd/2026-08-01-elasticsearch-module/`（本地 SDD 路径被 Git 忽略）。
- 密度/运行时间修复后的 fresh 结果为前端 12 文件/157 项、typecheck/build、Playwright 17 项静态发现、Go gofmt/vet/全仓普通/race/编译和无缓存镜像 `infraview:elasticsearch-density-uptime-verify` 全部通过。原 8080 再次原位重建，脱敏 API 确认 uptime 非空，Chromium 3/3 确认无横向滚动、角色省略、健康颜色、16 列单行和紧凑等高。
- 总览节点汇总按 RED→GREEN 补齐：主汇总为“异常节点 x / 总节点”，严重独立、warning 与 unknown 仅在紧凑徽标中合并，集群健康仍与节点分开；集群和节点同时为零时显示“暂无 Elasticsearch 节点”。定向 RED 为 3 项失败/45 项通过，GREEN 为 48/48；fresh 前端全量 12 文件/159 项、typecheck/build、Playwright 17 项静态发现及无缓存镜像 `infraview:elasticsearch-overview-summary-verify` 通过。后端、26 条查询、API 与阈值未改。
- 总览节点汇总版本已原位重建现有 8080；服务 healthy，仍为单服务、唯一发布 8080，并保持 `10001:10001`、只读根文件系统、cap drop `ALL` 与禁止提权。一次性 Chromium 3/3 通过，新增精确文本断言确认“异常节点”可见；未输出现场数量。
- 功能提交 `80da061` 已推送到 `origin/main`；提交前无缓存全量镜像 `infraview:elasticsearch-precommit-verify` 通过。未执行会创建 18080 的动态 E2E 栈或任何生产 Nightingale/Elasticsearch 验证。

## Redis Cluster 当前暂停点

- 独立 Redis 垂直模块已实现：固定 21 查询、一次即时 batch、无 N+1，独立缓存与 15 秒预期采集周期，2/5 周期采集警告/严重。
- 状态等级为 `critical > warning > unknown > normal`；同级来源优先为 availability、replication、memory、connection、collection。Cluster master 零已连接副本、slave 上游断链、复制延迟、有效 maxmemory 使用率与拒绝连接速率均按已批准阈值处理。
- API 仅有两个认证 GET；前端为总览第四卡、侧边栏第五项和固定十一列 Redis 实例页，最终列名为“实例地址、角色、内存上限、内存使用率、连接、QPS/命中率、key总数、复制链路、延迟、运行时间、状态”。内存上限为 maxmemory；复制链路主节点显示 `—`，从节点显示正常/断开/未知；延迟只使用真实 lag，缺失显示 `—`。运行时间与主机/MySQL 相同：同时有天和小时显示“x天 x小时”，整天显示“x天”，不足一天显示“x小时”。过期/淘汰仅从页面删除，后端字段、21 查询与 `sort=evicted` 兼容保留。首期无拓扑、slot、详情、历史、故障转移、Redis 直连或命令执行。
- 共享 `ListPage` 统一列表标题、标签控件、搜索框、刷新状态、表格/空状态/分页；`ModuleStatusCardShell` 统一总览卡结构。Redis 是首个接入者；新增模块必须复用，Linux/硬盘/MySQL 后续触及时渐进迁移且不主动改变行为或布局。业务阈值与状态仍由各模块计算。
- 全量前端 11 文件/112 项、typecheck/build、Playwright 2 文件/14 项静态发现、Go 格式/普通/race/编译和无缓存生产镜像均通过。十一列/共享模板已原位重建至现有 8080；未创建其他端口或项目，未读取私密环境，未连接生产 Nightingale。
- 十一列/共享模板及后续运行时间纠错均已原位重建至现有 8080；运行时间 RED 为 1 项失败/6 项通过，GREEN 为 7/7，修复后前端全量 11 文件/112 项、typecheck/build、Playwright 14 项静态发现以及镜像内 Go 普通/race/编译均通过。容器健康、唯一 8080、安全基线和部署资源匹配通过；功能提交 `c3b5c7d` 已推送。
- 提交前范围审查新增 Redis 极端页码溢出回归：`math.MaxInt` 页码旧实现会在切片处 panic，现已在查询规范化阶段安全返回 `ErrInvalidQuery`，定向 Redis Service 测试完成 RED→GREEN。修复后镜像内前端 112 项、typecheck/build、Go 普通/race/编译和 Playwright 14 项静态发现通过，并已按长期授权原位重建同一测试 8080；健康、唯一端口、安全基线和部署资源匹配通过。

## 主机硬盘 SMART 当前暂停点

- Task 1–7 的 RED→GREEN 证据保存在 `.superpowers/sdd/2026-07-30-host-disk-smart-module/task-1-report.md` 至 `task-7-report.md`。
- Task 8 与终审修复后已完成无端口、无上游连接的静态扫描、Docker Go 普通/race、前端 8 文件/99 测试、typecheck/build 和无缓存镜像构建，全部退出 0；终审的 1 个 Important 和 2 个 Minor 均已修复，范围复审 Approved。持久结果与未执行边界见已跟踪的硬盘实施计划和 `docs/TESTING.md`，当前工作区的本地 SDD report 仅作补充。
- 测试 Nightingale 现场首次发现兼容重复历史身份导致硬盘 API 503；已按 RED→GREEN 修复归并顺序和冲突边界，范围复审 Approved。修复后无缓存全量构建、现有 8080 原位重建、容器安全、硬盘 API、一次性 Chromium 1440×900 与跨两个 60 秒周期推进验收均通过。
- 未执行：会创建 18080 的 `scripts/e2e.sh` 和任何生产 Nightingale 验证。一次性 Chromium 仅访问现有 8080，未创建其他 InfraView 端口；本功能已提交并推送。
- 三份交接文档的功能前用户差异已作为基底保留；审阅时比较当前完整 diff，不能用 `git restore`、`git checkout`、`git reset` 或清理命令回退。
- 2026-08-01 容量增量已完成全量离线与现有 8080 现场验收：服务 healthy，安全配置和唯一 8080 端口符合基线；Nightingale、非 stale、容量存在、容量升降序、敏感字段排除、写方法 405、Linux/MySQL 回归及浏览器零错误均为 true；硬盘十列和总览四槽位 Chromium 用例通过。功能提交 `6300413` 已推送到 `origin/main`。

## 历史开发记录

以下段落用于追溯设计演进，其中的分支、列数、查询数和“未提交”描述只代表当时状态；当前状态一律以上方恢复入口和 `git` 只读检查为准。

### 2026-07-29 MySQL 表格细化收口

当时授权工作树为仓库内的 MySQL 功能 worktree，分支为 `feature/mysql-module`。MySQL 模块已完成独立领域/Service、Mock、Nightingale 受限适配、`GET /api/v1/mysql/overview`、`GET /api/v1/mysql/instances`、总览卡及 11 列实例页；复用 Nightingale 安全客户端，以固定 14 条查询构成一次即时 batch，禁止按实例 N+1。新增的第 14 条固定查询是 `mysql_global_variables_innodb_buffer_pool_size`，通过可空 `BufferPoolSizeBytes` / `buffer_pool_size_bytes` 契约只读下发。没有 MySQL 历史、详情、写入、数据库连接、任意 PromQL 或代理能力。

复制线程任一明确停止为严重；最大有效复制延迟 5 秒为警告、30 秒为严重。缺失值不伪造成零：仅可写且零复制通道为“未配置复制/正常”，只读或角色未知且零通道时复制线程与实例状态均为未知并纳入警告风险。任意复制状态只要存在有效延迟都显示秒数；总览警告徽标使用 `warning_instances` 并明确显示“警告风险”。

继续前读取上方 Git 跟踪文档；本地验证证据和恢复命令统一见 `docs/superpowers/reports/2026-07-29-mysql-module-verification.md`，不再依赖被 Git 忽略的 Task 12 brief/report。本地无缓存生产镜像、独立 race、E2E 安全测试、一次性 Mock smoke/资源检查与 Chromium 已通过，专用 E2E 项目资源已确认删除。真实 Nightingale v8.4.1 MySQL API smoke 和真实 Chromium 验收必须再次取得用户单独授权，绝不能读取或输出私密环境文件、Token、Cookie、认证头、真实地址、标识、数量、指标或上游正文。

2026-07-29 已完成 MySQL 紧凑布局的追加验收：桌面总览固定为四列紧凑模块位，Linux 与 MySQL 模块等宽；MySQL 保留全部 11 列，并在 1440×900 下确认页面和表格均无横向滚动。无缓存生产镜像验证、E2E 清理边界测试、原有 `infraview` 8080 的健康与无正文 HTTP 检查，以及原 8080 Chromium live 验收均已执行；Chromium 两项布局用例通过。此次只强制重建既有 `infraview` Compose 项目的 8080，未创建其他测试端口、Compose 项目或预览服务。开发 8080 永远连接测试 Nightingale，未连接生产；产品和运行时仍严格只读。受 Git 跟踪的完整脱敏证据见 `docs/superpowers/reports/2026-07-29-mysql-compact-layout-verification.md`。

同日完成 MySQL 表格细化收口：容量指标只接受非负最新有效值，缺失、非法或同一最新时间戳冲突均保持 `null`；`available_labels` 从完整 snapshot 的非空实例名去重排序，不受标签、状态、角色、搜索、排序和分页影响。`label` 去除首尾空白后按实例名精确匹配，并可与其他条件组合；服务端与页面 `search` 均只匹配实例地址或所属主机，不把实例名混入自由搜索。页面角色文案使用“读写/只读/未知”，Buffer Pool 按“容量 / 使用率”“容量 / —”“— / 使用率”“—”显示。

MySQL 表格最终列为“实例地址、所属主机、版本 / 角色、连接、线程、QPS、慢查询、Buffer Pool、复制 / 延迟、运行时间、状态”，桌面列宽依次为 `13/9/9/9/6/5/7/13/12/8/9%`。状态列表头为状态圆点预留等宽起始偏移；原 8080 Chromium 5/5 已确认 Host/MySQL 共享列文本起点差值 `<= 1px`、11 个表头单行、1440×900 无页面或表格横向溢出、900px 控制区两行三列。第 5 项受跟踪匿名用例确认标签控件顺序与筛选参数、首列纯地址、“读写”文案及 Buffer Pool 四种合法形态，认证后的 console/page/request/MySQL API 错误计数均为 0。恢复前曾在基线发现状态列 live 几何失败，修复提交 `aa54eae` 已通过范围复审并由本轮重新无缓存构建、部署和 live 验收确认。完整脱敏证据见 `docs/superpowers/reports/2026-07-29-mysql-table-refinement-verification.md`。

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

1. 先查看 `git status --short --branch`、`git log -3 --oneline`、`git diff --check` 和 `git diff --cached --check`，确认当前位于 `main`；预期为 40 个文件 staged、15 个 tracked unstaged、1 个未跟踪实施计划，历史中包含 Elasticsearch 功能基线 `80da061`。
2. 完整阅读 RabbitMQ design/plan 与 Task 1–11 记录；保留所有现有差异，不得 reset/restore/checkout/clean。若实际文件范围不同先报告，不把历史 Redis/Elasticsearch/SMART 状态覆盖到当前工作树。
3. RabbitMQ Task 11 fresh 全量和现有 8080 原位重建已经完成；动态登录态 Chromium 未执行，提交和推送已获授权、尚待执行。继续时严格按新的明确授权范围行动；禁止动态 E2E 创建额外端口、任何其他 InfraView 端口、生产 Nightingale/RabbitMQ 和私密信息输出。

## 安全边界

InfraView 始终只读展示：不提供修改、删除、重启、发布或配置下发，不执行 SSH/远程命令，不运行脚本，不自动化变更服务器、服务或配置。开发期已执行的远端检查仅用于用户授权的只读取证；运行时不得包含 SSH 客户端或远程执行能力。
