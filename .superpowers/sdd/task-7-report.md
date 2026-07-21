# Task 7 实施报告：主机搜索、筛选、排序与分页

## 状态

DONE_WITH_CONCERNS

Task 7 已按严格 TDD 完成。主机列表以 URL query 为唯一状态源，300ms 搜索 debounce，状态筛选、服务端排序、固定 20 条服务端分页和浏览器 history 返回均已实现。唯一非阻断顾虑是生产构建主 chunk 为 818.39 kB，Vite 给出大于 500 kB 的体积提示；本任务未做范围外代码分割，也未处理 Task 6 ECharts minor。

## 实现摘要

- 前端 Host DTO 完整匹配 Go HTTP JSON：`hostPageView -> hostView -> currentMetricsView -> metricValueView/filesystemView`。
- `/api/v1/hosts` 始终请求 `q/status/sort/order/page/page_size`，query key 包含全部 6 个参数，`page_size` 固定为 20。
- `q/status/sort/order/page` 从 `useSearchParams` 读取；搜索、筛选、排序变化均把 `page` 重置为 1。
- 搜索输入等待 300ms 后才更新 URL 和请求；详情链接只建立 `/hosts/:id`，没有实现详情内容。
- TanStack Table 只使用 `getCoreRowModel`、header/cell renderer；设置 `manualSorting/manualPagination`，没有客户端排序或分页 row model。
- 可排序列为主机、CPU、内存、负载、运行时间；前端负载发送规范字段 `sort=load`。
- 状态同时提供“在线/离线/未知”文字、颜色和 `data-status` 语义；null 指标显示“暂无数据”；百分比固定一位小数；运行时间显示中文天/小时。
- 页面没有重启、删除、执行、修改或其他写操作控件。
- 后端补齐原计划缺失的 load 排序：service 接受规范字段 `load`；HTTP 兼容 `load_1` 并归一为 `load`；缺失负载在升降序都置末，同值按 host ID 稳定排序。

## TDD RED / GREEN 证据

### 1. Go load 排序 RED

先只增加 service 与 HTTP 测试，再运行聚焦测试。

service RED：

```text
Hosts() error = service: invalid query: unsupported sort "load"
Hosts() error = service: invalid query: unsupported sort "load_1"
```

HTTP RED：

```text
sort=load status = 400, body = {"code":"invalid_query",...}
```

既有未知 `sort=password` 仍返回 400。加入最小实现后，service 升/降序、nil 末尾、稳定 tie-break，以及 HTTP `load/load_1` 两个入口全部 PASS；`go test ./internal/service ./internal/httpapi` 通过。

### 2. HostListPage RED

只创建完整 fixture 与行为测试，尚未创建生产组件，然后执行：

```bash
npm run test:run -- src/features/hosts/HostListPage.test.tsx
```

退出码 1，预期失败：

```text
Failed to resolve import "./HostListPage"
```

最小实现及测试环境同步修正后，聚焦套件 5/5 PASS。测试使用生产同构 `BrowserRouter`，完整 fetch 响应匹配真实 DTO；断言实际 URL/API/DOM 行为，没有 mock 元素断言。

### 3. 自审闭包 bug RED

自审发现 columns memo 可能持有旧 `searchParams`。先增加“筛选 offline 后排序仍保留 status”断言，当前实现按预期失败：

```text
expected '' to be 'offline'
```

移除持有旧 URL 闭包的 columns memo 后，同一聚焦测试 PASS；最终 HostList 5/5 和全量 unit 28/28 PASS。

## 最终验证与审计

最终源码状态下实际结果：

- `npm run test:run`：4 个测试文件，28/28 PASS。
- `npm run typecheck`：退出 0。
- `npm run build`：退出 0，685 modules transformed，JS 818.39 kB（gzip 271.05 kB）。
- `npm audit --omit=dev`：`found 0 vulnerabilities`。
- `npm audit`：`found 0 vulnerabilities`。
- Go gofmt 全仓检查：无未格式化文件。
- `go test ./...`：全部通过。
- `go test -race ./...`：全部通过。
- `go vet ./...`：退出 0。
- `go build -o /tmp/infraview ./cmd/infraview`：退出 0，仓库内未生成二进制。
- `git diff --check`：退出 0；无冲突标记。

构建仍显示 React Router/TanStack Query 顶层 `"use client"` 被 Rollup 忽略的既有非阻断提示，以及主 chunk 大于 500 kB 的提示；构建产物成功生成，未执行任何自动修复或依赖变更。

## 实际执行的修改/修复命令（按顺序）

源码和报告内容均通过 `apply_patch` 修改；各次作用如下：

1. `apply_patch`：增加 Go service/HTTP load 排序失败测试。
2. 尝试宿主 `gofmt -w internal/service/service_test.go internal/httpapi/api_test.go ...`；宿主没有 `gofmt`，在修改发生前退出 127。
3. `apply_patch`：增加 Go load 排序与 HTTP alias 归一实现，并把 service 降序测试统一为规范 `load`。
4. `apply_patch`：增加完整 HostPage fixture 和 HostListPage 失败测试。
5. `apply_patch`：增加前端 Host DTO、HostListPage、App 路由接线、列表样式，并修正严格一位小数测试期望。
6. `apply_patch`：把测试路由改为生产同构 BrowserRouter，并修正 searchbox 可访问角色。
7. `apply_patch`：修正测试异步等待与 fake timer 输入方式。
8. `apply_patch`：增加筛选后排序保留 status 的闭包回归断言。
9. `apply_patch`：移除 columns 陈旧闭包。
10. `apply_patch`：纯排版整理。
11. `apply_patch`：创建本报告。

实际会写文件的 shell 命令：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD":/src -w /src golang:1.24-bookworm gofmt -w internal/service/service_test.go internal/httpapi/api_test.go
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD":/src -w /src golang:1.24-bookworm gofmt -w internal/service/hosts.go internal/service/service_test.go internal/httpapi/query_handlers.go internal/httpapi/api_test.go
npm run build
```

前两条只格式化本任务 Go 文件；第三条只生成 Git 忽略的 `web/dist`。未安装/卸载软件，未重启服务，未删除数据，未执行 `npm audit fix`。

提交阶段实际执行：

```bash
git add internal/httpapi/api_test.go internal/httpapi/query_handlers.go internal/service/hosts.go internal/service/service_test.go web/src
git add -f .superpowers/sdd/task-7-report.md
git diff --cached --check
git commit -m "feat: add searchable host list"
```

## 自审

- 范围：除 Task 7 指定前端文件外，仅修改主代理明确授权的 4 个 Go load 排序文件；没有详情页或 Task 6 图表改动。
- Schema：fixture 和 TypeScript 类型包含 API 返回的全部 current metrics/file systems 字段，不是局部 mock。
- 状态：URL 是 q/status/sort/order/page 的来源；回退 history 测试验证详情返回保留完整 URL 和控件值。
- API 权限：仅 GET 查询；未增加写路由或操作控件。
- 排序：前端只发 `load`；HTTP alias 兼容集中在边界；service 内部只使用规范字段。
- 测试反模式：fetch 只作为 HTTP 边界替身；测试断言真实 URL、可访问 DOM、链接和 history，不断言 mock 本身。
- 质量：最终 unit/typecheck/build、Go test/race/vet/build、两类 audit 和 diff check 均通过。

## 顾虑与未执行建议

- 顾虑：Vite 报告主 chunk 818.39 kB 超过 500 kB。建议后续独立任务评估路由级 lazy loading/manual chunks；本任务未执行该优化，以免扩大范围或触碰 Task 6 ECharts minor。
- 未执行任何推送、PR、服务启动/重启、依赖安装/升级、审计自动修复或数据操作。
