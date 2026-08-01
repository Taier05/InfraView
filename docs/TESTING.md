# 测试与验收

## 标准入口

```bash
make verify
```

顺序覆盖：前端 Vitest、TypeScript、两类 npm audit、Vite build/静态资源复制、Go 格式、`go test ./...`、`go test -race ./...`、Go build、生产镜像 build、E2E 项目碰撞与部分失败清理安全测试、Compose smoke、100 请求 P95、内存/镜像资源、真实 Chromium E2E。`Makefile` 使用 `.NOTPARALLEL: verify`，即使调用 `make -j verify` 也会串行执行会修改 `node_modules`、`webdist` 和验收栈的步骤。

`internal/adapters/nightingale` 另有完全脱敏契约测试，覆盖认证头、Target 分页、v9 `beat_time` 优先与 v8.4.1 `update_at` 回退、默认数据源缓存、嵌套即时/区间批量响应、状态与单位映射、空结果、401、非 JSON、envelope 错误、响应大小限制、PromQL 转义、无 N+1 和 100 台规模。

验收使用带当前进程 PID 的唯一项目名和宿主端口 `18080`。启动前会检查 Compose 项目列表以及容器、网络、卷的 project label；若发现碰撞立即失败。预检成功后 trap 只取得该唯一项目的清理责任，因此 `up` 部分创建资源后失败也会精确 teardown，同时不会执行无项目范围或针对既有项目的 `docker compose down`，不会影响其他项目。

## MySQL 覆盖矩阵

- 领域与服务测试覆盖稳定实例标识、Provider 契约、可用性与复制状态聚合、基于 `tlast_over_time` 样本推进的 2/5 采集周期新鲜度、稳定绝对时间差不误报、冻结升级与恢复推进、并发状态访问、Target/外层求值时间不掩盖停采、按角色区分零复制通道、精确总览风险计数、缺失值语义、缓存与 stale 回退，以及地址搜索、自然排序和分页。
- Nightingale 契约测试全部使用脱敏 `httptest` 夹具，覆盖固定 16 条 PromQL、完整实例身份、单次 instant batch、24 小时近期实例保留、TPS、实例与复制通道合并、最新有效样本选择、冲突/非法值降级，以及错误不泄漏 Token、响应正文、标签或查询。
- HTTP 测试覆盖 `/api/v1/mysql/overview` 和 `/api/v1/mysql/instances` 的认证、完整 schema、空数组、固定查询参数、只允许 GET、写方法 405、安全 503 和未装配服务时的失败语义。
- Vitest 覆盖总览 MySQL 卡的独立加载、错误、stale、刷新状态与去重状态标签，以及实例页 10 列、QPS/TPS、采集延迟/失联、中文状态、空指标、GET 参数白名单、URL 状态、分页和后台刷新错误；主机采集延迟/失联分别锁定 warning/critical 颜色等级。
- Mock smoke 在登录前断言两个 MySQL GET API 返回 401；登录后用 `jq` 校验总览与实例 envelope，再确认写方法精确返回 405 且拒绝前后业务数据不变。脚本不打印响应正文、数量或指标值。
- Chromium 从总览 MySQL 卡进入实例页，验证 10 列、复制与实例状态、无破坏性控件和页面无横向溢出；另验证角色、状态、排序与每页数量在刷新后由 URL 恢复。

## 主机硬盘 SMART 覆盖矩阵

- 领域与 Mock 覆盖包含主机身份的不可逆稳定 ID、WWN/序列号/设备名回退、原始身份字段排除、ATA/NVMe、正常/警告/严重/未知、可空字段和错误计数形态。
- Nightingale Provider 使用完全脱敏 `httptest` 夹具，精确断言一次固定 18 查询 `query-instant-batch`、第 17 组容量唯一来源、第 18 组近期 inventory、无主机/设备 N+1、容量及其他指标最新有效样本/冲突规则、24 小时近期身份、401/403/重定向/Content-Type/envelope/8 MiB/超时安全错误和无敏感信息泄漏。
- DiskService 覆盖独立默认 60 秒 TTL/freshness、120/300 秒边界、稳定绝对时间差、冻结升级、恢复/回退、stale 老化、并发、状态聚合/去重、搜索、自然排序、精确容量排序、缺失值置后和分页。
- 状态来源契约覆盖六值 `smart_health|device_warning|attribute_failure|collection|normal|unknown`；同级时设备来源优先，设备来源内部依次为 SMART 健康、设备警告、属性失败，采集来源只有严格更高时生效。
- HTTP 测试覆盖两个 GET API 的认证、参数白名单、方法 405、安全 503、stale、schema 与序列号/WWN/原始标签排除。
- Vitest 覆盖 10 列硬盘页、型号/容量独立单元格、容量格式化与排序、错误摘要、URL 状态、后台刷新错误，以及总览硬盘卡的独立加载/失败/stale/重试和三板块隔离；Playwright 规格已加入导航、布局与无破坏性控件断言，但是否实际运行见本轮验证记录。

## 浏览器覆盖

`web/e2e/infraview.spec.ts` 覆盖：

- 未登录重定向、固定账号登录、退出和退出后重定向。
- 总览板块状态卡和手动刷新。
- 主机搜索、离线筛选和主机名不可点击。
- 主机清单 9 个单行列、IO 忙碌度、网络出入速率及指标等级着色。
- MySQL 总览导航、实例清单 10 列、复制/实例状态和紧凑布局。
- MySQL 角色/状态筛选、排序和每页数量的 URL 刷新恢复。
- 主机硬盘总览导航、10 列清单、型号/容量分列、容量排序、状态来源文案、URL 恢复和无序列号/WWN/破坏性控件。
- 可控 stale 与 503 error 展示。
- 页面不存在重启、删除、执行、切换、修改、发布或配置下发控件。

运行：

```bash
cd web
npm run e2e
```

脚本构建独立 Compose 栈，先跑 smoke，再使用固定版本 Playwright 容器。浏览器容器会在随 `--rm` 清理的匿名 `node_modules` 卷中执行锁文件 `npm ci`，因此不依赖或污染宿主/工作树依赖目录。若想在已运行服务上调试，可设置 `INFRAVIEW_E2E_BASE_URL` 后运行 `npm run e2e:run`，宿主必须自行具备 Chromium 依赖。

## 缓存延迟

```bash
INFRAVIEW_BASE_URL=http://127.0.0.1:18080 \
INFRAVIEW_USERNAME=admin \
INFRAVIEW_PASSWORD='实际密码' \
./scripts/benchmark.sh
```

脚本登录、预热总览、顺序执行 100 次认证请求，数值排序后选择第 95 个样本；`p95 >= 0.200s` 即失败。Task 10 最终验证实测 `p95=0.003212s`。这验证同机缓存命中路径，不等同于跨网络容量压测。

## 宿主无 make 的等价验证

当前开发宿主没有 `make`。可直接按 Makefile 目标顺序运行 Docker 化的 Node/Go 命令，再执行：

```bash
docker build --tag infraview:verify .
cd web
INFRAVIEW_E2E_PORT=18080 \
INFRAVIEW_E2E_RUN_BENCHMARK=true \
INFRAVIEW_E2E_CHECK_RESOURCES=true \
npm run e2e
```

Makefile 语法可在临时容器中检查；不要为此安装宿主软件。

## 手工收尾检查

```bash
git diff --check
git status --short
git status --short -- .superpowers
docker compose ls
```

预期 whitespace 检查无输出，`.superpowers` 没有新增工作区变更，专用验收项目已清理，其他既有 Compose 项目仍运行。

真实 Nightingale 联调必须使用 Git 忽略且权限受限的环境文件，只输出状态、数量和字段可用性，不输出 Token、真实响应正文或认证头。现有 Mock E2E 含固定主机 ID，真实环境应使用独立浏览器检查登录、总览、主机行数、关键列、布局溢出和登录后页面错误。

当前真实联调基线为 Nightingale v8.4.1；v9.x 仅保留脱敏契约回归。真实验证应确认 `/api/n9e/versions` 的版本证据，但 InfraView 运行时不得为了分支逻辑额外调用版本接口。

## 2026-07-27 Nightingale 审查修复验证

本轮完全使用脱敏 `httptest` 夹具和 Docker Go 镜像，未读取私密环境文件、未作真实 Nightingale/SSH 联调、未启动或重启 8080 服务。

- 重定向拒绝、`dat:null`、缺失分页字段和 instant/range 批量基数的新增测试均先得到真实 RED，最小修复后 GREEN；负数 total、变更 total、提前空页、超量、空/重复 ident 是既有 GREEN 覆盖。
- 数据源发现成功并发合并、串行失败后重试、PromQL 固定白名单及不支持磁盘指标零上游请求为覆盖补强；最终审查又发现并发失败未合并，同一失败 flight 单请求广播、等待者 context 取消和 flight 后重试均完成真实 RED→GREEN。
- Mock 模式忽略无关 Nightingale 设置、Provider Base URL 约束、未知历史主机的零范围查询/404，以及数值转换边界均完成 RED→GREEN。
- `beat_time` 复用 JSON/RFC3339 年份边界，非法极端值保持零时间且资产仍可 JSON 编码；Content-Type 精确限制为 `application/json`（允许参数），拒绝 `application/problem+json` 和 `text/*+json`，均完成真实 RED→GREEN。
- `docker run --rm -v "$PWD":/src -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale ./internal/config ./internal/service ./internal/httpapi -count=1` 通过。
- 最终修复后由主控重新运行格式检查、全仓普通测试和独立 `go test -race ./...`，全部通过；`docker build --tag infraview:nightingale-review-fixes-verify .` 成功，镜像 SHA 为 `<不记录>`。
- 最终整分支审查发现的 2 个 Important 和 1 个 Minor 已全部修复，唯一范围复审结论为 PASS。
- 简报中的 `sh -lc` 在该镜像登录 shell 中丢失 `/usr/local/go/bin`，因此无法找到 `go`/`gofmt`；使用非登录 `sh -c` 执行相同格式和普通测试语义，并另行运行独立 race 命令取得 PASS 证据。

## 2026-08-01 硬盘容量独立指标与分列离线验收

- Provider 与 Service/API 均完成真实 RED→GREEN。交付前独立复审发现容量在严格校验前先经过 `float64`，可在 `2^53` 边界误接收小数、改变合法整数或漏判同时间冲突；新增 `2^53+1`、`MaxInt64`、大整数小数和大整数同时间冲突测试均先得到 RED，随后改为从原始 JSON 数值文本精确解析到专用 `int64` 状态并转为 GREEN。adapter 普通/race、容量白名单、精确可空 `int64` 排序及 HTTP 参数契约测试均通过。
- 修复后由同一独立审查者定向复核容量精度、当前态文档和 E2E 契约，最终结论为 Ready，Critical 0、Important 0、Minor 0；复核全程只读，未连接 8080 或 Nightingale。
- 前端硬盘页 14 项定向测试通过；全量 Vitest 为 8 个文件、101 项测试通过，TypeScript 检查与 production build 均退出 0。Vite 仍仅报告既有第三方 `"use client"` 非阻断提示。
- `npx playwright test --list` 退出 0，共静态发现 13 项规格，其中硬盘规格已锁定十列表头、型号/容量独立单元格、至少一个 IEC 格式容量、容量升降序 URL 和无横向溢出要求；布尔断言不输出现场容量值。未运行浏览器，也未启动或创建端口。
- Go 格式检查无输出，全仓普通测试、race 测试和主程序编译均退出 0。`docker build --no-cache --tag infraview:disk-capacity-column-verify .` 退出 0，并在干净上下文内再次通过前端 101 项、typecheck/build、Go 普通/race 和最终二进制编译；镜像只构建未运行，未映射端口、未连接上游。
- `npm ci` 仍报告锁定依赖树中 2 个 high severity 审计项；本轮未执行 `npm audit fix --force`，未修改依赖。
- 经用户单独授权，现有 8080 已原位重建并继续只连接测试 Nightingale；服务 healthy，保持非 root、只读根文件系统、cap drop `ALL`、禁止提权且只发布 8080。脱敏登录态 API 验收仅输出布尔结果：Nightingale、硬盘 GET/non-stale、容量存在、容量升降序实际有序、敏感字段排除、写方法 405、Linux/MySQL 回归和浏览器零错误均为 true。
- 一次性 Chromium 不发布端口，trace 关闭，所有失败产物定向到容器 `/tmp` 并随容器销毁。硬盘十列用例在 1440×900 下通过，覆盖型号/容量分列、至少一个 IEC 格式容量、容量升降序 URL、页面/表格无横向溢出及无破坏性控件；原 8080 总览四槽位几何用例也通过。最初两次因筛选路径未发现测试，第三次以公开默认 E2E 凭据登录失败，均未形成产品失败；改用不落盘、不输出的应用进程凭据后通过。API 脚本首轮由页面主动请求预期 405，污染 Chrome console；改由共享会话 request API 验证后全部布尔结果为 true，产品代码未因此修改。
- 任何生产 Nightingale 验证和会创建 18080 的 `scripts/e2e.sh` 均未执行；经明确授权，功能提交 `6300413` 已推送到 `origin/main`。

## 2026-07-30 主机硬盘 SMART 当前工作区验证

本轮先完成无端口、无上游连接的 Docker 验证；后续经用户单独授权，仅原位重建仍连接测试 Nightingale 的现有 8080，并执行不发布新端口的一次性 Chromium 与脱敏现场验收。以下 fresh 结果和未执行边界是受 Git 跟踪的持久记录。

本轮最终 fresh 结果：静态扫描未发现命令执行能力，排除测试后的 HTTP View/前端生产类型没有 `serial_no`/`wwn`；前端 8 个测试文件、99 个测试通过，typecheck 与 production build 退出 0；Go 普通与 race 全仓均退出 0；终审修复后的无缓存镜像构建退出 0，并在 Dockerfile 内再次完成同一组前端/Go 测试与编译。整功能终审发现的默认硬盘排序、Service 极端页码防溢出和磁盘总览 alert 合计约束均完成 RED→GREEN，范围复审 Approved。构建未运行镜像、未映射端口、未连接上游。

测试 Nightingale 现场首次返回同一 `ident + device` 的兼容第 17 组历史序列，原始最后样本时间不同，旧实现安全返回 503。新增测试先复现 RED；修复按主键归并、对称补齐可选身份、取最大原始时间并保持非空元数据，覆盖两种输入顺序、跨设备 serial/WWN 冲突和同时间型号/容量冲突。Nightingale adapter 普通/race、范围复审及后续无缓存全量镜像均通过。

修复后的现有 8080 保持 healthy、Nightingale 模式、非 root、只读根文件系统、capabilities 全删、禁止提权且唯一发布原 8080。登录态硬盘总览/设备 API 非空且非 stale，schema、六值来源、敏感字段排除和 POST/PUT/PATCH/DELETE 405 均通过；Linux、主机与 MySQL 回归通过。一次性 Chromium 在 1440×900 下确认总览入口、九列表头、数据行、无破坏性控件、无页面或表格横向溢出及登录后无浏览器错误。跨两个 60 秒周期后设备集合稳定、响应非 stale 且采集推进状态正常。

`npm ci` 报告现有 2 个 high severity 依赖审计项，且 Vite 输出依赖包 `"use client"` 指令被忽略的非阻断警告；本轮未执行 `npm audit fix --force` 或修改依赖。

未执行会创建 18080 的 `scripts/e2e.sh`；浏览器验收改用不发布端口的一次性 Playwright 容器访问现有 8080。任何生产 Nightingale 验证、提交和推送均未执行。

## 2026-07-30 总览四槽位与硬盘展示细化离线及现有 8080 验收

- Docker 前端全量命令使用绑定 `web` 源码目录和匿名 `/src/node_modules` 卷，`npm ci --ignore-scripts` 后 Vitest 为 8 个文件、101 项测试通过，typecheck 与 production build 均退出 0。
- `npm ci` 仍报告 2 个 high severity 依赖审计项；Vite 仍报告依赖包 `"use client"` 指令被忽略。两者均为既有非阻断项，本轮未修改依赖或执行审计修复。
- `docker build --no-cache --tag infraview:overview-disk-display-verify .` 退出 0；Dockerfile 内再次通过前端全量测试、typecheck、production build、Go `go test ./...`、`go test -race ./...` 和二进制编译。镜像仅被构建，未运行、未映射端口、未连接上游。
- Playwright 规格新增首个硬盘数据行的 `.disk-capacity` 和 `.disk-model` 可见断言。授权前曾以代码内公开默认 E2E 登录凭据向既有 8080 发起一次 Chromium 尝试；登录阶段超时，未进入总览，也未形成现场 GREEN。
- 用户随后明确授权现有 8080 原位重建和现场验收。`INFRAVIEW_ENV_FILE=/root/github/InfraView/.env docker compose --project-name infraview up -d --build --force-recreate infraview` 退出 0；服务达到 healthy，仍只有原 8080，且保持 `10001:10001`、只读根文件系统、cap drop `ALL`、`no-new-privileges`。命令只让 Compose 读取既有配置，未显示其内容。
- 登录态只读 API 验收不输出正文或现场值：数据源类型为 Nightingale，Linux、MySQL、硬盘响应均为合法 JSON 且非 stale；硬盘列表非空、`status_source` 限定六值、不含 `serial_no`/`wwn`，POST/PUT/PATCH/DELETE 均返回 405。
- 一次性 Playwright 容器使用 host network 访问原 8080，不发布端口、不截图、不保留 trace。最终 1440×900 验收确认总览四轨等宽、三卡宽度分别匹配前三轨且第四轨自然空白；硬盘九列表头、数据行、型号/容量独立两行及各自完整 title 可见；页面和表格无横向溢出、无破坏性控件，登录后无非预期 console/page/request 错误。
- 首轮现场脚本因使用错误的主机/MySQL 排序参数失败；修正为页面真实白名单后，错误收集又捕获了脚本主动 405 检查和导航取消。按系统化诊断将 405 改由共享登录会话的 request API 验证，并把错误监听提前到首次导航前。预认证会话 GET 401 只有在固定会话路径的响应与同来源标准 Chrome console 文本同时出现且各恰好一次时才视为预期；登录提交阶段只精确豁免成功路由切换产生的 `fetch + net::ERR_ABORTED`。除此之外所有 console/page/request 错误仍阻断。同一完整路径退出 0，产品代码未因这些脚本问题修改。
- 未运行会创建 18080 的 `scripts/e2e.sh`，未创建额外 InfraView 端口，未连接、切换或探测生产 Nightingale，未输出凭据、Cookie、认证头、Base URL、API 正文或现场标识/数量/指标值。

## 2026-08-01 Redis Cluster 模块离线验证

- 领域/Mock：稳定 ID、深拷贝、角色与正常/警告/严重/未知场景。
- Nightingale：固定 21 查询副本、一次 batch、无 N+1、身份/数值/角色冲突与安全错误。
- Service/API：15 秒样本推进、2/5 周期 freshness、状态阈值、同级来源优先、缓存/stale、查询白名单、认证 GET、405 与安全 503。
- 前端：总览第四卡、侧边栏、固定十列、复用既有带标签控制栏、搜索防抖、刷新状态、URL 状态、分页归一化、loading/empty/stale/error、刷新失败保留旧数据及无破坏性控件。
- 首次原位重建现有 8080 后，用户现场发现总览提示响应格式无效，以及详情控制栏、列名和长指标可读性问题。RED 测试确认后端 `roles` 输出大写字段而前端契约要求小写；修复为显式小写 view model，并增加 HTTP 契约回归。详情页统一复用主机/MySQL 控制栏，最终列名按批准文案固定，内存、过期/淘汰与复制状态改为可换行结构化展示。
- 本次修复后 Go 全仓格式/普通/race/编译均退出 0；前端 9 文件/108 项测试、typecheck、production build 均退出 0；Playwright 静态发现为 2 文件/14 项。
- `docker build --no-cache --tag infraview:redis-ui-fixes-verify .` 退出 0，干净构建上下文内再次通过前端 108 项、typecheck/build、Go 普通/race 和最终编译。镜像仅构建，未运行、未映射端口、未连接上游。
- `docker build --no-cache --tag infraview:redis-cluster-module-verify .` 退出 0，Dockerfile 内再次通过前端 107 项、typecheck/build 与 Go 普通/race/编译。镜像仅构建，未运行、未映射端口、未连接上游。
- `npm ci` 仍报告既有 2 个 high severity 依赖项，Vite 仍报告依赖包 `"use client"` 指令被忽略；本轮未修改依赖，也未执行 `audit fix --force`。
- 本次修复浏览器只执行 `playwright test --list` 静态发现。随后经明确授权原位重建现有 8080，健康、唯一 8080 映射、只读根文件系统、cap drop `ALL` 和禁止提权检查通过；未创建其他端口或项目，未连接生产 Nightingale，页面现场验收由用户进行。

## 2026-08-01 Redis 十一列与共享观测模板验证

- 按 TDD 新增共享列表页与共享总览卡壳测试；两组测试均先因组件不存在而失败，再在最小实现后通过。Redis 仅迁移展示结构，查询、URL、阈值和状态计算仍留在业务模块。
- Redis 十一列测试先在旧合并列实现上得到 2 项预期失败；实现后 7/7 通过，覆盖内存上限有效/未设置/缺失、独立使用率、主/从/未知复制链路及真实 lag 存在/缺失。运行时间最终格式以后续纠错记录为准。
- 当前定向验证已通过 TypeScript 类型检查以及共享组件、Redis 页面和 Redis 总览共 4 个测试文件/42 项。
- 全量前端离线验证退出 0：Vitest 11 文件/112 项、typecheck、production build、Playwright 2 文件/14 项静态发现。Go 格式检查、全仓普通测试、race 测试与编译退出 0。
- `docker build --no-cache --tag infraview:redis-shared-template-verify .` 退出 0；Dockerfile 内再次通过前端 112 项、typecheck/build、Go 普通/race 与最终编译。镜像仅构建，未运行、未映射端口、未连接上游。
- `npm ci` 仍报告既有 2 个 high severity 依赖审计项，Vite 仍报告依赖包 `"use client"` 指令被忽略；本轮未修改依赖，也未执行 `npm audit fix --force`。
- 十一列与共享模板随后经授权原位重建至现有 8080，并通过健康、唯一端口、安全基线和新列静态资源检查；未创建其他端口，未读取私密环境文件或连接生产 Nightingale。后续运行时间纠错见下一节。

## 2026-08-01 Redis 运行时间格式纠错

- 用户验收确认 Redis 应与主机/MySQL 一致显示天/小时，而非累计纯小时。测试先将 `90000` 秒的期望由 `25小时` 改为 `1天 1小时`，得到 1 项预期失败、其余 6 项通过。
- 最小修复复制主机/MySQL 的既有日/小时分支；同一 Redis 页面测试随后 7/7 通过，并额外锁定 `null` 为“暂无数据”、`3600` 秒为“1小时”、`1800` 秒为“0小时”。
- 修复后前端全量 Vitest 11 文件/112 项、typecheck、production build 和 Playwright 2 文件/14 项静态发现均退出 0。构建仅生成离线产物，未启动服务或连接上游。
- 本次纠错随后按用户长期授权原位重建现有 8080。Dockerfile 内前端 11 文件/112 项、typecheck/build、Go 普通/race/编译全部退出 0；容器 healthy、仍只发布原 8080，并保持非 root、只读根文件系统、cap drop `ALL` 与禁止提权。部署页面引用的资源文件与本地最新 production build 完全一致；未创建其他端口、未连接生产 Nightingale。Redis 功能提交 `c3b5c7d` 已推送到 `origin/main`。
- 后续流程：每次已授权修复通过验证后自动原位重建同一测试 8080，不再逐次询问。该规则不允许读取或输出私密环境内容，不允许创建额外端口或连接生产 Nightingale。
- 提交前范围审查发现 Redis 极端合法正页码可能使 `(page-1)*page_size` 溢出并触发切片 panic。新增 `math.MaxInt` 回归先稳定复现 RED；随后在查询规范化阶段加入与硬盘模块相同的偏移溢出保护，定向 Redis Service 测试转为 GREEN。该修复不改变正常分页、查询白名单或 API 成功响应。
- 审查修复后的 Compose 重建退出 0：Dockerfile 内前端 11 文件/112 项、typecheck/build、Go 普通/race/编译全部通过；Playwright 2 文件/14 项静态发现通过。现有 8080 healthy、仅发布原端口，并保持非 root、只读根文件系统、cap drop `ALL` 和禁止提权；部署页面引用最新构建资源。
