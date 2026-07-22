# 测试与验收

## 标准入口

```bash
make verify
```

顺序覆盖：前端 Vitest、TypeScript、两类 npm audit、Vite build/静态资源复制、Go 格式、`go test ./...`、`go test -race ./...`、Go build、生产镜像 build、E2E 项目碰撞与部分失败清理安全测试、Compose smoke、100 请求 P95、内存/镜像资源、真实 Chromium E2E。`Makefile` 使用 `.NOTPARALLEL: verify`，即使调用 `make -j verify` 也会串行执行会修改 `node_modules`、`webdist` 和验收栈的步骤。

验收使用带当前进程 PID 的唯一项目名和宿主端口 `18080`。启动前会检查 Compose 项目列表以及容器、网络、卷的 project label；若发现碰撞立即失败。预检成功后 trap 只取得该唯一项目的清理责任，因此 `up` 部分创建资源后失败也会精确 teardown，同时不会执行无项目范围或针对既有项目的 `docker compose down`，不会影响其他项目。

## 浏览器覆盖

`web/e2e/infraview.spec.ts` 覆盖：

- 未登录重定向、固定账号登录、退出和退出后重定向。
- 总览卡片、7 天切换、手动刷新。
- 主机搜索、离线筛选和详情导航。
- 当前指标、文件系统、4 个真实 ECharts canvas。
- 可控 stale 与 503 error 展示。
- 页面不存在重启、删除、执行、修改、发布或配置下发控件。

运行：

```bash
cd web
npm run e2e
```

脚本构建独立 Compose 栈，先跑 smoke，再使用固定版本 Playwright 容器。若想在已运行服务上调试，可设置 `INFRAVIEW_E2E_BASE_URL` 后运行 `npm run e2e:run`，宿主必须自行具备 Chromium 依赖。

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
