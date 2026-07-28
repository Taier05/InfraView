# 测试与验收

## 标准入口

```bash
make verify
```

顺序覆盖：前端 Vitest、TypeScript、两类 npm audit、Vite build/静态资源复制、Go 格式、`go test ./...`、`go test -race ./...`、Go build、生产镜像 build、E2E 项目碰撞与部分失败清理安全测试、Compose smoke、100 请求 P95、内存/镜像资源、真实 Chromium E2E。`Makefile` 使用 `.NOTPARALLEL: verify`，即使调用 `make -j verify` 也会串行执行会修改 `node_modules`、`webdist` 和验收栈的步骤。

`internal/adapters/nightingale` 另有完全脱敏契约测试，覆盖认证头、Target 分页、v9 `beat_time` 优先与 v8.4.1 `update_at` 回退、默认数据源缓存、嵌套即时/区间批量响应、状态与单位映射、空结果、401、非 JSON、envelope 错误、响应大小限制、PromQL 转义、无 N+1 和 100 台规模。

验收使用带当前进程 PID 的唯一项目名和宿主端口 `18080`。启动前会检查 Compose 项目列表以及容器、网络、卷的 project label；若发现碰撞立即失败。预检成功后 trap 只取得该唯一项目的清理责任，因此 `up` 部分创建资源后失败也会精确 teardown，同时不会执行无项目范围或针对既有项目的 `docker compose down`，不会影响其他项目。

## 浏览器覆盖

`web/e2e/infraview.spec.ts` 覆盖：

- 未登录重定向、固定账号登录、退出和退出后重定向。
- 总览板块状态卡和手动刷新。
- 主机搜索、离线筛选和主机名不可点击。
- 主机清单 9 个单行列、IO 忙碌度、网络出入速率及指标等级着色。
- 可控 stale 与 503 error 展示。
- 页面不存在重启、删除、执行、修改、发布或配置下发控件。

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
git ls-files .superpowers
docker compose ls
```

预期 whitespace 检查无输出，`.superpowers` 未被跟踪，专用验收项目已清理，其他既有 Compose 项目仍运行。

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
