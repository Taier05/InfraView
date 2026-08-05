# InfraView 项目状态

最后更新：2026-08-05

## 当前阶段

### 2026-08-05 RabbitMQ Task 11（节点发现与名称真实性收口）

现场反馈确认，部分实例具有连接指标但当前/近期 `rabbitmq_identity_info` 没有覆盖，原 inventory-only 集合会漏节点；连接发现补齐实例后，旧回退又把实例地址错误写入“节点名称”。最终架构改为：当前与近期身份优先建立具名节点，连接指标只补充发现缺失实例；Provider 扫描同一固定批次，只有采集键对应唯一一致的显式 `rabbitmq_node` 才补全名称，缺失或冲突时名称保持空值，页面显示“暂无数据”。实例地址和 `ident` 均不再参与名称猜测。

两个 Provider 回归测试分别锁定“缺名不能冒充地址”和“其他节点级序列携带唯一标签时可确定性补全”，旧实现均得到 RED，最小修复后 GREEN；页面空名称规格同样先 RED 后 GREEN。RabbitMQ 前后端定向测试通过，随后现有 8080 原位重建；镜像内前端全量、typecheck/build、Go 普通/race/编译全部通过。容器健康、唯一 8080、错误回退源码移除及 staged/unstaged whitespace 检查通过。未访问夜莺仪表盘、未读取或输出上游正文与真实身份数据；动态登录态 Chromium 未执行。

### 2026-08-05 RabbitMQ Task 10（共享采集目标多节点与零值摘要修复完成）

现场反馈显示夜莺能展示多个 RabbitMQ 节点，而 InfraView 只保留一个。系统化排查确认 Provider 先以 `cluster + ident + instance` 作为唯一 inventory 键，导致同一采集目标下不同 `rabbitmq_node` 被较新样本覆盖或在同时间被当作冲突丢弃。Task 10 改为以“集群内部身份 + `rabbitmq_node`”稳定身份保存节点，并保留采集键到候选节点的二级索引；带节点标签的指标精确归并，无节点标签且候选不唯一时保持缺失。两条吞吐速率查询的 `sum by` 同步保留 `rabbitmq_node`，固定查询数量、顺序、单批次与只读边界不变。

后端回归先稳定得到共享采集目标场景 `nodes=1` 的 RED，最小修复后 Provider、领域、Service、HTTP RabbitMQ 定向套件均 GREEN。用户追加要求总览四个摘要框的全零文案与其他卡一致；新增前端测试先因找不到“无异常”得到 RED，随后共享 `MetricAlert` 在总风险数为零时统一显示“无异常”，非零分级明细和颜色保持不变。Go 全仓 gofmt/vet/普通/race/静态编译通过；前端全量为 14 文件/210 项，typecheck、production build 与 Playwright 4 文件/21 项静态发现通过。既有 npm lock 仍报告 1 个 moderate 与 2 个 high，本轮未改依赖、未运行 `audit fix`。

经明确授权，现有 `infraview` Compose 服务已原位重建且只保留 8080；镜像内再次通过前端 210 项、typecheck/build、Go 普通/race/编译。容器 healthy、单服务、非 root、只读根文件系统、cap drop `ALL`、禁止提权，健康接口和两个 RabbitMQ API 的未认证拒绝均通过。未创建其他端口，未读取或输出私密配置、认证信息、真实身份或上游正文；动态登录态 Chromium、commit 和 push 未执行。

### 2026-08-05 RabbitMQ Task 1–9（离线全量验证与终审完成）

已按批准规格实现独立 RabbitMQ 领域、完全合成 Mock、Nightingale 固定 22 查询 Provider、共享快照 `RabbitMQService`、两个认证 GET API、总览第六卡、侧边栏与固定 15 列节点页。Provider 每个快照恰好一次即时 batch、无集群/节点/指标 N+1；集群身份依次回退永久集群 ID、逻辑集群名和采集集群名，API 只返回不可逆身份。集群不可达 peer 与节点状态严格分离；内存/FD/Erlang 进程使用率 80%/90%、磁盘余量 `<=1`/`<1.2`、明确资源告警和 2/5 周期采集推进规则均由前序 Task 报告记录。连接、队列、消息积压和吞吐只展示。

Task 8 新增 `web/e2e/rabbitmq.spec.ts`，所有 RabbitMQ GET route 均返回合成数据，405 规格使用共享已认证 request context。Task 9 fresh 验证完成：前端 14 文件/209 项、typecheck/build、Playwright 静态发现 4 文件/21 项（RabbitMQ 4 项），Go gofmt/vet/普通/race/静态编译，安全与 whitespace 检查，以及无缓存生产镜像构建均退出 0；镜像未运行。终审 Important 的 query 7 集群聚合已按 RED→GREEN 修复并通过复审，Clone、query 21、overview validator 与四色 E2E Minor 均已闭环。npm lock 仍报告既有 1 个 moderate 与 2 个 high，本轮未修改依赖文件，也未执行 `audit fix`。动态 Chromium 1440/1100、现有 8080 访问或重建、deploy、commit、push 均未获授权且未执行；没有读取私密环境或访问真实/生产上游。

### 2026-08-01 Elasticsearch 首期模块（离线与现有 8080 现场验收完成）

Elasticsearch 功能提交 `80da061 feat: add read-only Elasticsearch monitoring` 已推送到 `origin/main`，当前正常预期为 `main...origin/main` 且工作树干净。模块已实现独立领域/Mock/Nightingale Provider/Service、两个认证 GET API、总览第五卡、侧边栏和共享模板上的 16 列节点页。Nightingale Provider 固定一次 26 查询即时 batch，同时构建多集群和节点快照，无集群/节点 N+1；集群身份为 `cluster`，节点身份为 `cluster + name`，采集身份不进入 API。

集群状态与节点状态严格分离：集群来源为 availability、health、collection、normal、unknown，节点来源为 collection、disk、jvm、thread_pool、normal、unknown；默认 15 秒周期下 2/5 周期未推进为 warning/critical，磁盘 85%/90%、JVM 堆 75%/85%、拒绝速率大于 0 的阈值已由测试锁定。集群黄/红不批量改变节点状态。HTTP 仅有 `GET /api/v1/elasticsearch/overview` 与 `GET /api/v1/elasticsearch/nodes`，空集合为 `[]`，写方法为 405。

2026-08-01 基线离线验证曾通过：Playwright 3 文件/17 项静态发现（其中 Elasticsearch 3 项）；前端 12 文件/154 项、typecheck/build；Go gofmt/vet/普通/race/编译；无缓存镜像 `infraview:elasticsearch-verify`。终审后又以 RED→GREEN 修复 health `color`、inventory 缺身份/地址归并、clusterinfo/node_stats 来源拆分和浮点求和顺序；2026-08-04 主控 fresh 全量为前端 12 文件/155 项、typecheck/build、Playwright 3 文件/17 项静态发现、Go gofmt/vet/全仓普通/race/编译、只读/敏感/whitespace 扫描和无缓存镜像 `infraview:elasticsearch-final-verify`，均退出 0。最终提交前无缓存镜像 `infraview:elasticsearch-precommit-verify` 再次通过前端 159 项、typecheck/build、Go 全仓普通/race 和编译。npm 仍报告既有 1 个 moderate 与 2 个 high，Vite 仍有第三方 `"use client"` warning；本轮未修改依赖。

经用户单独授权，现有 `infraview` Compose 服务已原位重建，继续只使用既有测试 Nightingale 配置且只发布原 8080。服务 healthy，并保持 `10001:10001`、只读根文件系统、cap drop `ALL` 与禁止提权。脱敏登录态 API 验收只输出布尔结论：测试 Nightingale 健康、Elasticsearch 总览/节点列表非 stale、节点集合非空、采集敏感字段排除、两个 API 的 POST/PUT/PATCH/DELETE 均为 405，以及 Linux/硬盘/MySQL/Redis 回归均通过。一次性 Chromium 不发布端口、不截图、不保留 trace，在 1440×900 下确认总览五卡四轨、Elasticsearch 第五卡位于第二行、16 个精确表头和单值单行数据格、代表值不截断、页面无横向溢出、宽表只在内部滚动、紧凑等高、无破坏性控件和无浏览器错误。未连接生产 Nightingale，未创建其他 InfraView 端口；功能提交已推送。

2026-08-04 追加完成表格密度与 uptime 修复：uptime 改为专用安全解析，接受 Prometheus 合法的有限非负小数/科学计数文本、向下取整且拒绝负数/NaN/Inf/越界，其他整数指标解析不变。节点角色超过两个时仅显示前两个与 `…`、`title` 保留完整列表；集群健康使用四色状态徽标。16 列使用紧凑固定布局，1440×900 页面与表格均无横向滚动，更窄视口保留内部滚动兜底。fresh 验证为前端 12 文件/157 项、typecheck/build、Playwright 17 项静态发现、Go gofmt/vet/全仓普通/race/编译和无缓存镜像 `infraview:elasticsearch-density-uptime-verify` 全部通过。原 8080 已再次原位重建；脱敏 API 验收确认 uptime 非空、两个端点写方法 405 与既有模块回归，Chromium 3/3 确认新布局/颜色/角色/运行时间契约。服务仍 healthy、唯一发布 8080 且安全基线不变；未连接生产，修复已包含在功能提交中。

2026-08-04 继续补齐总览节点汇总：Elasticsearch 卡现在与既有模块一致复用共享 `module-alert-summary`，显示“异常节点 x / 总节点”、独立“严重”徽标和紧凑的“警告/未知”徽标；异常节点只由节点 warning、critical、unknown 计数派生，集群健康仍在下方独立展示。当集群和节点总数同时为零时显示“暂无 Elasticsearch 节点”，只有一侧为零时仍保留另一侧信息。该改动不涉及后端、26 条查询、API、阈值或状态计算。Vitest 定向 RED 为 3 项失败/45 项通过，最小实现后定向 GREEN 为 48/48；fresh 前端全量为 12 文件/159 项，typecheck/build 与 Playwright 3 文件/17 项静态发现通过，无缓存镜像 `infraview:elasticsearch-overview-summary-verify` 构建通过。现有 8080 已原位重建，服务 healthy、单服务且唯一发布 8080，容器安全基线通过；一次性 Chromium 3/3 通过。该修复已包含在功能提交中并推送。

### 2026-08-01 硬盘容量独立指标与分列（离线及现有 8080 现场验收完成）

本功能按 TDD 把硬盘固定即时 batch 从 17 组扩展为 18 组：第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组 inventory 继续建立设备并提供型号与原始最后样本时间。容量只归并到已知设备，并从原始数值文本精确解析为最新非负 `int64`；缺失、非法、越界或同一最新时间冲突保持 `null`，`2^53` 以上整数不会经过 `float64` 丢失精度。旧 inventory `capacity` 标签不再读取、校验或回退，容量不参与设备发现、`ReportedAt`、freshness、状态或总览。

Service/API 已支持 `sort=capacity`，使用精确 `int64` 比较，升降序均将缺失值置后并以设备 ID 稳定收口。硬盘页已改为“主机、设备、型号、容量、SMART 健康、温度、寿命、通电时间、错误摘要、状态”十列；总览仍固定四槽位，分页仍仅 20/50/100，命令超时、失联移除和“全部”分页均未实现。完整离线验收已通过：前端 8 文件/101 项、typecheck/build、Go 格式检查、全仓普通/race 测试和编译、Playwright 13 项静态发现，以及无缓存镜像 `infraview:disk-capacity-column-verify`；镜像未运行、未映射端口、未连接上游。

经用户单独授权，本增量已原位重建现有 8080，并继续只连接测试 Nightingale。服务 healthy，保持非 root、只读根文件系统、cap drop `ALL`、禁止提权且仍只发布 8080。脱敏 API 验收确认 Nightingale、硬盘响应非 stale、至少一个容量存在、容量升降序实际有序、敏感字段排除、写方法 405、Linux/MySQL 回归及浏览器错误为零；一次性 Chromium 在 1440×900 下确认硬盘十列、型号/容量分列、IEC 容量、容量升降序 URL、页面/表格无横向溢出、无破坏性控件，以及总览仍为四个固定槽位。未创建其他端口，未连接、切换或探测生产 Nightingale。功能提交 `6300413` 已推送到 `origin/main`。

### 2026-07-30 总览四槽位与硬盘展示细化（离线与现有 8080 现场验收完成）

桌面总览恢复四个固定槽位，当前三张卡占前三位；硬盘“型号 / 容量”改为独立两行，长型号不再遮挡容量；`unsafe_shutdowns` 改为“异常断电 N 次”，仅表示累计展示，不参与状态判断。离线 Docker 前端全量回归已通过：Vitest 8 个文件、101 项测试，另有 typecheck 和 production build 均退出 0。无缓存镜像 `infraview:overview-disk-display-verify` 也退出 0，Dockerfile 内再次完成前端全量、Go 普通/race 测试和二进制编译。

用户随后授权本细化项的现有 8080 原位重建与现场验收。服务仍只发布原 8080、仍为测试 Nightingale 模式；healthz、非 root、只读根文件系统、cap drop、禁止提权均通过。登录态只读 API 验收确认数据源类型、非 stale、硬盘六值 `status_source`、敏感身份键排除和写方法 405；Linux、MySQL 与硬盘固定 API 均返回合法 JSON。一次性 Chromium 不发布端口，在 1440×900 下确认总览四轨等宽且三卡宽度匹配前三轨、第四轨自然空白，硬盘九列、型号/容量独立两行、无页面或表格横向溢出、无破坏性控件且无非预期登录后浏览器错误；错误监听覆盖首次导航，预认证会话 GET 401 需响应与同来源 console 双重核验且各恰好一次，登录成功路由切换只精确豁免 `fetch + net::ERR_ABORTED`。未读取私密环境文件、未输出凭据/API 正文/现场值，未创建额外端口，也未运行 `scripts/e2e.sh`。既有 SMART 模块和用户文档差异均保留，提交与推送仍待分别授权。

### 2026-07-30 主机硬盘 SMART 模块（共享工作区，未提交）

当前 `main` 共享工作树在保留本功能开始前 `docs/HANDOFF.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 用户差异的基础上，新增独立只读硬盘领域/Mock/Nightingale Provider/DiskService、两条 GET API、`/disks` 9 列页面、侧边栏入口和总览第三张独立卡。硬盘 Provider 精确使用一次固定 17 查询即时 batch，无主机/设备 N+1；Nightingale v8.4.1 为主要验证版本，v9.x 仅保留协议兼容。

硬盘快照 TTL 与 freshness 使用独立默认 `60s`，按 InfraView 本地最后观察到原始样本时间推进的时刻计算，默认 120/300 秒为警告/严重。温度、寿命和错误计数只展示。设备 ID 包含主机身份并按 WWN、序列号、设备名回退做不可逆哈希，API 永不返回原始身份。`status_source` 固定六值；最终等级同级时设备来源优先于采集来源，设备来源内部依次为 SMART 健康、设备警告、属性失败。

Task 1–7 的 RED→GREEN 证据分别保存在本地 SDD task report。Task 8 与终审修复后已完成静态安全扫描、前端 8 文件/99 测试、typecheck/build、Go 普通/race 全仓和无缓存完整镜像构建，全部退出 0；终审的 1 个 Important 和 2 个 Minor 均已修复，范围复审 Approved。测试 Nightingale 现场首次发现第 17 组同一 `ident + device` 的兼容历史身份序列因原始最后时间不同而被旧实现误判重复；已按 RED→GREEN 实现顺序无关的安全归并，补齐可选身份、取最新原始时间并保留非空元数据，范围复审 Approved。修复后无缓存全量构建、现有 8080 原位重建、容器安全、脱敏 API、一次性 Chromium 1440×900 和跨两个 60 秒周期推进均通过。未创建其他 InfraView 端口，未连接生产，未提交或推送。

### 2026-07-30 样本推进新鲜度修复（已交付 main）

前一版原始样本绝对年龄方案部署后，测试现场确认 Linux `system_uptime` 与 MySQL `mysql_up` 原始样本仍约每 15 秒推进，但样本时间相对 InfraView 稳定落后数十秒。绕过 InfraView 缓存后仍处于警告年龄窗口，Nightingale HTTP 时钟与 InfraView 对齐，因此稳定时间差来自采集端时间戳或采集/转发/写入链路。直接计算“当前时间减原始样本时间”会让正常采集对象长期误报。

当前实现使用并发安全的进程内样本推进状态：首次观察建立本地基线，原始时间变化时重置本地推进时刻，同一时间连续 30 秒未变化进入警告、75 秒未变化进入严重，恢复推进立即正常；时间回退重新建立基线。Provider 的 `tlast_over_time`、MySQL 16 条固定单 batch、24 小时身份与 API 契约不变。主机“采集延迟/采集失联”颜色同步改为 warning 黄色/critical 红色，与 MySQL 一致。

无缓存完整镜像已通过前端全测、TypeScript、生产构建、Go 全仓普通/race 测试和编译；隔离浏览器 E2E 最终通过。原有仅连接测试 Nightingale 的 8080 已原位重建并完成脱敏验收。冻结 2/5 周期升级及恢复推进由自动化测试覆盖。功能提交 `4956ea9` 已推送到 `origin/main`；本次仅有三份交接文档未提交。

### 2026-07-30 原始样本绝对年龄修复（已部署测试 8080，判定逻辑已被替代）

测试采集器停止后已确认旧实现会被 Nightingale Target 更新时间和即时查询外层求值时间掩盖：Linux 主机保持正常，MySQL 直到上游即时回看窗口结束才告警。该历史阶段按 TDD 修复：Linux 固定 batch 增加 `tlast_over_time(system_uptime[24h])`，MySQL 第 16 条固定查询改为 `tlast_over_time(mysql_up[24h])`；两者都只使用查询值中的原始最后样本 Unix 时间计算采集新鲜度。

Target 状态仍用于资产在线/离线判定，但不再参与采集新鲜度。默认 `15s` 周期与 2/5 周期阈值不变，即 30 秒警告、75 秒严重；MySQL 仍保持 16 条固定查询、一次 batch、24 小时近期身份和无 N+1。该绝对年龄方案及其后续样本推进替代方案均已随 `4956ea9` 交付。

### 2026-07-29 采集新鲜度与 MySQL TPS（已部署，随后发现时间口径缺陷）

该历史阶段在 `main` 工作区实现了默认 `15s` 的预期采集周期：Linux 主机与 MySQL 实例达到 2 个周期进入警告、达到 5 个周期进入严重；页面分别显示“采集延迟”和“采集失联”。Nightingale 不再为全空当前指标伪造当前时间，后续时间口径已由样本推进方案替代并交付。

MySQL 固定即时查询由 14 条扩展为 16 条，仍在一次 batch 内完成且无实例 N+1。最初使用 `max_over_time(mysql_up[24h])` 保留近期实例身份，现已由上方 2026-07-30 修复替换为兼具身份和原始样本时间的 `tlast_over_time`；新增 TPS 为显式 `COMMIT` 与 `ROLLBACK` 的每秒速率，可能低估 autocommit。清单调整为 10 列，删除“所属主机”，合并显示“QPS / TPS”，自由搜索仅匹配地址，实例地址按 IP 与端口自然排序。总览非空卡片删除底部重复状态标签。

2026-07-30 已获用户授权，使用现有受限配置原位重建既有 `infraview` 服务；仍只有原 8080，仍为测试 Nightingale 模式。后续登录态 Chromium、完整构建和隔离 E2E 验收均已完成，功能已随 `4956ea9` 推送；未连接生产。

### 2026-07-29 MySQL 模块文档与本地验收（历史）

当时的 `feature/mysql-module` 工作树实现了只读 MySQL 领域、Service、Mock、Nightingale 适配、两个 GET API、总览告警卡与 11 列实例页。该阶段使用 14 条固定查询；当前实现已演进为 16 条固定查询和 10 列实例页，仍保持一次 batch、无 N+1，且不提供历史、详情、写入、数据库连接或任意查询能力。

复制线程任一明确停止为严重；复制延迟最大有效值 5 秒起警告、30 秒起严重。缺失复制通道、线程或延迟不转换为零：仅可写且零复制通道为正常，只读或角色未知且零通道时复制线程与实例状态均为未知并计入警告风险。Nightingale 适配器先忽略非法候选再选择最新有效样本，复制通道身份按标签键存在性判定并允许合法空值。2026-07-29 已在本地完成无缓存生产镜像构建（前端 Vitest 71/71、typecheck、production build、Go 普通/race 测试与编译）、E2E 隔离安全测试，以及既有 Mock 验收。用户随后授权仅连接测试 Nightingale 的原 8080 Chromium 验收并已完成；该证据不等同于生产 MySQL 验收。

同日的 MySQL 紧凑布局验收已完成：总览桌面四列紧凑模块位已锁定，Linux/MySQL 模块等宽；MySQL 11 列和所有表头在 1440×900 下可见，页面与表格均无横向滚动。完整无缓存镜像验证和 E2E 清理边界测试已运行；只重建既有 `infraview` Compose 项目的原 8080，健康与无正文 HTTP 检查通过，原 8080 Chromium live 两项布局用例通过。该开发服务只连接测试 Nightingale，未连接生产，也没有创建其他测试端口或 Compose 项目。受跟踪的脱敏验证记录见 `docs/superpowers/reports/2026-07-29-mysql-compact-layout-verification.md`。

最终全分支审查发现的 MySQL batch `null` 语义和总览四类互斥状态已按 TDD 修复，范围化复审通过；最终提交重新完成无缓存镜像、原 8080 健康与无正文 HTTP、E2E safety 和 Chromium live 验收。后续改动已经进入 `main`。

MySQL 表格细化也已完成：`available_labels` 来自完整 snapshot 的稳定非空实例名集合，`label` 为去空白后的实例名精确筛选并可与状态、角色、搜索、排序和分页组合；服务端与页面 `search` 均只匹配实例地址或所属主机。页面使用“读写”而非“可写”，Buffer Pool 同列展示容量与使用率，缺失组合不伪造成零。桌面 11 列宽为 `13/9/9/9/6/5/7/13/12/8/9%`。

修复提交 `aa54eae` 解决状态圆点导致的状态列表头文本起点偏移后，该阶段重新完成无缓存镜像、E2E safety、固定查询定向测试及原 8080 重建。原 8080 始终只连接测试 Nightingale，未连接生产；相关分支后续已合入 `main`。

Mock MVP、紧凑布局、Nightingale 只读接入、15 秒当前页面刷新、“数据连接”汇总和 Nightingale v8.4.1 兼容均已完成并进入 `main`。v8.4.1 功能交付基线为 `18d26a6`，已推送到 `origin/main`；原 `feature/nightingale-v8-compat` 分支和 worktree 已清理。当前以 Nightingale v8.4.1 为主要开发与真实验证版本，同时保留 v9.x 已覆盖的协议兼容。

v8.4.1 上游只读契约预检已经通过；Target 在当前环境提供 `update_at` 而非 `beat_time`。适配器已按 TDD 实现“有效 `beat_time` 优先、有效 `update_at` 回退、否则零值”，运行时不探测版本，也不增加额外上游请求。完整生产镜像、隔离 Mock E2E、8080 重建、应用端真实只读 smoke 和容器安全检查均已通过。

后续新增的服务、网络、存储等板块统一沿用当前主机页交互规范：刷新按钮旁展示上次成功刷新时间和自动刷新周期，请求中显示“正在刷新…”；列表数据统一左对齐、统一字号和字重，数值采用等宽字体，异常仅通过颜色等级强调。

### 2026-07-28 Nightingale v8.4.1 兼容

当前主要真实验证版本为 v8.4.1；v9.x 保留脱敏夹具和既有协议回归，但不再声明为当前真实验证环境。`/api/n9e/versions` 可提供 v8.4.1 版本信息，InfraView 运行时不依赖该接口。认证 profile、Target 分页、默认 Prometheus 类型数据源、即时批量和区间批量接口的状态、JSON Content-Type、`dat`/`err` envelope 与外层结构已完成安全预检；没有输出 Token、认证头、响应正文、真实标识、数量或指标值。

最终独立代码审查结论为 Critical 0、Important 0、Minor 2，两个 Minor 均已处理：补充 `update_at=0` 和负数保持零时间的直接回归用例，并在最终验证后勾选实施计划 Step 7。

### 2026-07-27 Nightingale 审查修复

已完成第二阶段审查阻断项修复：Provider 拒绝重定向且不转发认证信息；拒绝 `dat:null`、缺失分页字段和批量结果数失配；主机历史读取先验证主机可见性；Mock 模式隔离 Nightingale 专属校验；构造器拒绝不安全 Base URL；内存、运行时间、指标时间和 Target `beat_time` 在越界前视为缺失；Content-Type 仅接受 `application/json`。数据源发现现在同时合并成功和失败 flight，失败不缓存，后续请求可重试，等待者可独立响应 context 取消。新增固定 PromQL 白名单、未知主机 404 和完整协议/数值/并发边界测试。

合并前安全收尾已完成：公开文档和夹具不记录真实部署情报；Nightingale 默认只允许 HTTPS，受控 HTTP 测试环境必须显式开启不安全兼容开关。完整验证与最终独立复核通过，结论为 `Critical 0，Important 0，Minor 0`。

所有新增行为按 TDD 完成，最终修复后相关包、全仓普通/race 测试和生产镜像构建已在 Docker 中重新通过；最终整分支审查的 2 个 Important 和 1 个 Minor 均已修复，唯一范围复审 PASS。随后使用受限环境文件重建私有 Compose 服务：容器 healthy，登录、数据源状态、脱敏主机样本和总览只读 smoke 均通过；公开仓库不记录实际资源数量。

### 2026-07-27 至 2026-07-28 状态、刷新与数据连接优化

总览卡片正常零值现在使用绿色明确显示“无严重”“无警告”“无离线”“无未知”和“无异常”，非零异常继续沿用严重/警告/未知等级，卡片结构不变。数据源状态 API 新增真实 `type` 与 `refresh_interval_seconds`，左下角根据后端运行时配置显示 `Nightingale` 或 `Mock`；AppShell 将同一刷新周期通过路由上下文传给当前页面，不再在多个页面硬编码周期。

默认界面刷新和当前指标缓存均调整为 15 秒，主机清单与静态资产、历史范围缓存仍为 60 秒；只有当前挂载路由轮询，浏览器后台标签页继续停止刷新，同一查询继续避免重叠。左下角已由永久展开的单数据源卡片改为紧凑“数据连接”汇总：健康 Nightingale 默认显示绿色 `1/1 正常`，展开后显示指标来源、健康结果和最近检查时间；Mock 使用黄色提示，异常、过期和状态获取失败直接显示在汇总行。后续日志等数据源可沿同一入口扩展，当前后端仍保持单数据源状态契约。上述行为按 TDD 完成，前端 Vitest 45/45、typecheck、production build、Go 全仓普通/race 测试、生产镜像构建和隔离 Mock Chromium 4/4 均通过。历史 v9 预览曾使用权限保持为 600 的私密环境文件重建，容器 healthy；当时只读 smoke 验证登录、真实 `nightingale` 类型、15 秒周期、脱敏主机样本和总览数据均成功且非 stale，并完成 1440×900 登录浏览器默认/展开状态视觉验收。

总览页已升级为可扩展的“板块告警摘要卡”模式：当前只保留一张普通单列宽度的可点击 `Linux 主机` 卡，优先显示整体正常/警告/严重状态、异常主机数、严重/警告主机数，以及 CPU、内存、IO、网络各自异常数量和等级构成；底部保留在线、离线和未知数量，点击整张卡进入主机列表。离线主机计严重、未知主机计警告，指标沿用主机列表阈值，同一主机只按最高等级计一次。总览不再展示 CPU、内存平均值卡、资源趋势或时间范围控件；后续服务、网络、存储等板块按相同尺寸逐张加入。

总览告警汇总由后端复用已经获取的全部主机当前指标在内存中计算，不增加数据源请求、不存储告警事件。总览和主机页现在共用刷新控制组件，统一展示刷新按钮、上次成功刷新时间、后端运行时下发的自动刷新周期和刷新中状态。

主机列表已进一步压缩纵向占用，当前表格为主机名、IP 地址、CPU 核数、内存容量、CPU 使用率、内存使用率、负载、IO 忙碌度、网络出/入、运行时间、状态共 11 列。CPU 核数和内存容量来自可选资产字段，使用“8 核”“32 GiB”格式，缺失时显示“暂无数据”；Mock 采用固定硬件配置档位，后续 Nightingale 接入前必须确认真实字段，禁止从使用率反推。每页数量支持 20、50、100 三档，默认 20；选择后写入 URL 并回到第 1 页。除状态列外，所有数据统一使用 `0.78rem` 字号、`500` 字重、相同正常颜色、行高和等宽字体；主机名不再使用强调色或额外加粗，指标警告/严重颜色仍按阈值覆盖。长主机名固定限制在主机名列内并显示省略号，完整值保留在悬停提示中，不能覆盖 IP 列。所有表头和值统一左对齐，身份列保持收紧后的宽度和紧凑间距。刷新按钮旁显示浏览器最近一次成功收到数据的时间和“每 15 秒自动刷新”，请求期间切换为“正在刷新…”。每个字段保持单行显示；网络列按“发送 / 接收”顺序只展示数值并自动换算单位。CPU、内存和 IO 忙碌度使用 80%/90% 警告与严重阈值，网络发送、接收分别使用可配置的 B/s 阈值着色。Mock 第一页固定包含各类警告和严重样本，便于视觉确认。IO 支持按忙碌度排序，网络支持按发送与接收总吞吐量排序，缺失值始终置后；所有可排序表头使用明确的 `⇅` 按钮，当前排序列显示强调色 `↑/↓`。主机详情页、详情测试、专用图表组件和 ECharts 前端依赖已删除。

- 当前开发分支：`main`
- 当前功能交付基线：`6300413`，已推送到 `origin/main`
- 主工作树：`/root/github/InfraView`
- 当前预览 worktree：无；原 v8.4.1 兼容 worktree 已清理
- 其他 worktree：以 `git worktree list` 的只读结果为准，不在交接中假设
- Nightingale v8.4.1 功能交付基线：`18d26a6`，已推送到 `origin/main`
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
- 稳定数据源领域契约、确定性 Mock、以 v8.4.1 为主要真实验证版本并保留 v9.x 协议兼容的只读 Nightingale Provider。
- 独立 MySQL 领域与 Service、MySQL Mock、固定 16 查询单 batch 的 Nightingale MySQL Provider、MySQL 总览/实例 GET API、总览卡与 10 列实例页。
- 共享工作树中的独立硬盘领域与 DiskService、硬盘 Mock、固定 18 查询单 batch 的 Nightingale 硬盘 Provider、硬盘总览/设备 GET API、总览卡与型号/容量分列的 10 列设备页；第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组为 inventory。
- TTL 缓存、相同请求合并、最多 5 分钟旧值降级。
- 总览、主机列表、单机详情和数据源状态只读 API。
- 深色中文 React UI、板块告警摘要卡、统一刷新控件、搜索、筛选、排序、URL 状态和分页。
- 导航使用紧凑数据连接汇总展示健康、Mock、异常、过期和请求失败，展开后显示来源与最近检查时间；当前页面支持手动及后端运行时周期驱动的非重叠刷新，默认 15 秒。
- 受保护 API 会话失效后集中清缓存并返回登录页。
- Go 同源托管 SPA；单一安全 Compose 容器直接暴露 IP:端口。
- Playwright Chromium 覆盖总览与主机清单关键路径、stale/error、退出/重定向和无破坏性控件。
- 主机详情与图表前端代码已删除，生产前端不包含 ECharts 依赖。
- 完整架构、设计、配置、部署、开发、测试、安全和 Nightingale 接入文档。

## v0.1.0-mock 历史基线验收证据

- 前端：Vitest 60/60、typecheck、build 通过；生产及全量 npm audit 均为 0。
- Go：全仓普通测试、race、格式检查和构建通过；镜像构建阶段重复执行普通/race 测试。
- Smoke：含双引号和反斜杠的账号密码 JSON 转义后 PASS。
- Playwright：真实 Chromium 3/3，通过总览和详情 4 canvas 验证。
- 缓存延迟：100 次顺序认证 overview 请求，第 95 样本 `0.003819s`，低于 `0.200s`。
- 资源：内存 `10.210 MiB`，低于 256 MiB；镜像 `16.64 MB`。
- 隔离：专用项目/18080 在成功、碰撞和部分启动失败路径均受精确清理保护；最终只保留原有 `cliproxyapi`。
- 审查：完整分支最终复审 Approved，未发现破坏性运维能力或代理信任阻断问题。

## 历史紧凑预览验证

- 前端：Vitest 45/45、typecheck、production build 通过。2026-07-27 的 npm audit 新报告 React Router RSC 模式公告 `GHSA-qwww-vcr4-c8h2`，共 2 个 High；当前 InfraView 为普通 SPA，不使用 RSC Action，但审计事项保持开放，等待兼容的上游修复版本，不执行强制降级。
- Go：全仓普通测试和 race 测试通过；总览告警汇总包含独立服务测试和 API JSON 契约验证。
- 真实浏览器：官方 Playwright Chromium 隔离环境 4/4 通过；1440×900 总览无横向或纵向溢出，告警摘要卡完整显示。主机页选择每页 50 条后 URL 保持 `page_size=50`，32 台 Mock 主机可在同页渲染且无横向溢出；默认 20 条布局下，新增 CPU 核数与内存容量后 11 列仍可完整单行显示。超长主机名浏览器用例确认文本发生截断且主机名单元格右边界不超过 IP 单元格左边界；主机名、IP、配置值和正常指标的计算后颜色、字体、字号、字重、行高一致，状态列保持独立样式。
- 部署：原开发 8080 已使用受限私密配置原位重建并完成脱敏验收；该服务永远只连接测试 Nightingale，不连接生产。
- 主机清单：IO 忙碌度、网络出入简化格式、百分比及网络阈值等级测试通过。
- Nightingale：完全脱敏夹具覆盖认证头、分页、v9 `beat_time` 优先、v8.4.1 `update_at` 回退、嵌套批量响应、状态/单位、空结果、401、非 JSON、envelope 错误、8 MiB 大小限制、PromQL 转义、无 N+1 和 100 台规模。
- 生产镜像：Dockerfile 内前端 Vitest/typecheck/build、Go 普通测试、race 测试和构建全部通过。
- 历史 v9 真实联调：数据源健康且非 stale；脱敏 Target 样本的资产、运行时间、六类当前指标和历史序列均完成验证，真实资源数量与点数不进入公开仓库。
- v8.4.1 验收记录：认证、Target、数据源、即时批量和区间批量只读契约通过；应用端数据源、总览、主机、单机详情和 1 小时指标范围 smoke 均通过且非 stale。
- 真实浏览器：官方 Chromium 在 1440×900 下总览与多行主机表正常，11 列关键内容可见，无横向/纵向溢出、无破坏性控件，登录后零页面错误。

## 安全与产品边界

平台只读展示，不提供修改、删除、重启、发布、命令、脚本、任意查询或代理；不执行远程命令或自动化变更。无业务数据库，监控历史和会话不落盘。

## 当前阻塞

当前没有已知 Nightingale v8.4.1 主机硬盘 SMART 契约阻塞；现有测试 8080 的 API、Chromium 和样本推进验收已经通过。生产试运行中，除 IO 忙碌度显示“暂无数据”外，已有板块指标正常；当前 InfraView 固定读取 `diskio_io_util`，而该生产环境目前可查询 `diskio_io_time` 派生利用率。用户决定先升级生产 Categraf，暂不修改 InfraView；因此这是一项等待采集端升级后复验的兼容观察，不是已经确认的 InfraView 缺陷。

其他开放事项是每个私有部署必须使用专用最小只读 Token；磁盘容量和磁盘读写历史缺少已确认指标契约，因此保持空序列。

## 当前开发部署与历史生产观察

- 访问地址和凭据由私有部署环境提供，不进入公开仓库。
- 开发服务：仅使用既有 8080，永远连接测试 Nightingale；不创建额外 InfraView 测试端口。
- 最近记录的开发验收：原位重建后完成健康、只读 API、样本推进和 Chromium 检查；本次交接不重新验证或重启服务。
- 生产试运行：已有板块指标整体正常；IO 忙碌度待生产 Categraf 升级并经过至少两个采集周期后复验。未修改 InfraView 的固定 IO 查询。
- 容器安全：UID/GID 10001、只读根文件系统、capabilities 全删、禁止提权、`restart: unless-stopped`。
- 登录凭据仅保存在主工作树被 Git 忽略且权限为 600 的 `.env`，文档和仓库不记录密码。

## 下一步

1. 当前硬盘功能已提交并推送；继续前确认工作树干净并等待下一项开发需求，未经新授权不再提交或推送。
2. 现有 8080 已完成本轮授权验收并继续只连接测试 Nightingale；后续不得切换、探测或连接生产 Nightingale。
3. 生产 Categraf 升级完成后，如用户要求，再只读验证 Linux `diskio_io_util`；开发 8080 不得切换到生产。
4. 后续落实专用最小只读 Token；Linux 磁盘容量/读写历史仍须按证据决定，不与本次 SMART 当前清单混淆。

### 2026-08-01 Redis Cluster 模块（已交付）

已按批准规格实现独立 Redis 领域、Mock/Nightingale Provider、Service、两个认证 GET API、总览第四卡、侧边栏和实例页。Provider 固定一次 21 查询 batch，无实例 N+1；状态覆盖可用性、采集、内存、拒绝连接与复制，并保留 unknown。上一版总览角色字段和控制栏修复曾经授权原位重建至现有 8080，健康与容器安全基线通过。

本轮按 RED→GREEN 建立共享列表页结构 `ListPage` 与共享总览卡壳 `ModuleStatusCardShell`，Redis 为首个接入者；Linux、硬盘和 MySQL 暂不改变布局或行为，后续触及时渐进迁移。Redis 实例页固定十一列：“实例地址、角色、内存上限、内存使用率、连接、QPS/命中率、key总数、复制链路、延迟、运行时间、状态”。过期/淘汰仅从页面移除，后端 21 查询、字段、状态逻辑和 `sort=evicted` 兼容不变；复制延迟只显示真实 lag。运行时间已按 RED→GREEN 纠正为主机/MySQL 的天/小时格式，并按用户新增长期授权直接原位重建至现有测试 8080。镜像内前端 11 文件/112 项、typecheck/build、Go 普通/race/编译均通过；容器健康、唯一 8080、安全基线和部署资源匹配通过。Redis 功能提交 `c3b5c7d` 已推送到 `origin/main`。

部署流程长期规则：每次用户已授权的修复通过验证后，直接原位重建既有 8080，无需再次取得部署授权。该授权仅覆盖复用现有测试 Nightingale 配置的同一 8080 服务；不覆盖生产 Nightingale、额外端口、其他服务、提交或推送。

提交前范围审查发现并按 TDD 修复 Redis 极端页码偏移溢出：旧实现对 `math.MaxInt` 页码产生负切片边界，现于查询规范化阶段返回 `ErrInvalidQuery`；正常分页行为不变。修复后镜像内前端 11 文件/112 项、typecheck/build、Go 普通/race/编译和 Playwright 14 项静态发现通过，并已自动原位重建同一测试 8080；健康、唯一端口、安全基线和部署资源匹配通过。
