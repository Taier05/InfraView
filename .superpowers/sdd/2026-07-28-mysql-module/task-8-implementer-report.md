# Task 8 Implementer Report

## 结果

- 基线：`437866239ea22e6c2a6142f8953cea8bb6553bdd`
- 新增且仅新增两个 MySQL 业务 GET 接口：
  - `GET /api/v1/mysql/overview`
  - `GET /api/v1/mysql/instances`
- 两个接口复用现有认证、中间件、安全响应和 405 fallback；`POST`、`PUT`、`PATCH`、`DELETE` 均返回 `405` 与 `Allow: GET`。
- 总览拒绝所有 query；实例仅允许 `search`、`status`、`role`、`sort`、`order`、`page`、`page_size`，拒绝未知键、重复键、非法值、malformed query 和显式空分页值。
- HTTP 层只把白名单值构造为 `service.MySQLQuery`；未接受或传递 SQL、PromQL、上游 URL。
- 使用显式 JSON view 输出总览、告警、实例、复制状态和分页字段，不直接暴露 provider 结构。
- `mysql.ErrUnavailable` 映射为脱敏、可重试的 `503 mysql_unavailable`；底层错误、fixture token、指标名和上游 envelope 标识不会进入响应。
- `buildHandler` 中 Host 与 MySQL Service 共用同一个 `cache.Store`；MySQL provider 独立复用已有上游 timeout wrapper。
- `Dependencies.MySQLService` 缺失时，MySQL GET 安全返回脱敏 503，不进入 nil pointer panic recovery。
- 未访问生产或敏感数据；未 push、部署、重启服务或执行任何运维写操作。

## 变更文件

- 新增：`internal/httpapi/mysql_handlers.go`
- 修改：`internal/httpapi/api.go`
- 修改：`internal/httpapi/query_handlers.go`
- 修改：`internal/httpapi/api_test.go`
- 修改：`cmd/infraview/main.go`
- 修改：`cmd/infraview/main_test.go`
- 新增：本报告

所有受跟踪文件修改均通过 `apply_patch` 完成；没有使用 shell 重写受跟踪源码。`gofmt` 仅对上述 Go 文件做机械格式化。

## TDD 与复核证据

1. 路由测试首次有效 RED：认证 GET、成功 view、写方法分别因 `404` 失败。
2. 装配测试 RED：`buildHandler` 的 MySQL 总览因 `404` 失败。
3. 最小 handlers、认证 GET 路由、405 fallback、view、Service 装配完成后，两组定向测试 GREEN。
4. 白名单/错误分支 RED：总览 query 得到 `200`、实例未知 query 得到 `200`、MySQL 不可用得到 `500`。
5. 首次错误脱敏 GREEN 检查发现既有错误码 `datasource_unavailable` 自身含禁词 `dat`；MySQL 分支改用 `mysql_unavailable`，同样保持安全 503、中文消息和 retryable 语义，随后定向测试 GREEN。
6. 独立 reviewer 报告两个 Important：
   - malformed query 与显式空分页可能被静默当成默认查询；
   - 未注入 MySQL Service 时路由可能进入 nil pointer recovery。
7. 两项分别新增失败测试并观察到 `200`、`500` RED；随后集中使用 `url.ParseQuery`、区分缺失/空分页，并增加 nil Service 安全 503 guard，各自定向 GREEN。
8. reviewer 的 Minor 是扩大稳定 view 断言；brief 指定的类型断言、显式 view 和全包测试已覆盖本任务，未据此扩大实现范围。

## 环境前置与风险

- 第一次 Go 测试未进入断言：新 worktree 缺少被 Git 忽略的 `internal/httpapi/webdist`，触发 `go:embed` 编译错误。
- 宿主没有 `make`，所以 `make web-copy` 在执行任何配方前以 127 失败；未安装软件。
- 随后严格按 Makefile 等价流程，在 Node 22 容器内构建前端，并在确认目标受 Git 忽略、不是符号链接、真实路径精确位于当前 worktree 后复制静态产物。生成物未纳入 Git。
- `npm ci` 报告既有依赖存在 2 个 high severity audit 项；本任务不升级依赖，未执行 `npm audit fix`。

## 实际命令（按执行顺序）

下列为与仓库状态、生成物、TDD、验证和提交直接相关的实际 shell 命令。只读源码/技能文档的 `sed`、`rg` 定位命令未逐项展开；它们未修改状态。

1. 核对基线与初始工作树：

```bash
git rev-parse HEAD
git status --short
git log --oneline --decorate -8
```

2. 首次 MySQL HTTP RED 尝试（因缺少 `webdist` 在 setup 阶段失败）：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL' -count=1
```

3. 尝试仓库标准生成目标（宿主缺少 `make`，退出 127，未执行配方）：

```bash
make web-copy
```

4. 在容器内执行等价前端构建，并带路径安全检查复制忽略产物：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -w /src/web node:22-alpine sh -c 'npm ci && npm run build'
git check-ignore -q internal/httpapi/webdist/
test ! -L internal/httpapi/webdist
mkdir -p internal/httpapi/webdist
test "$(realpath internal/httpapi/webdist)" = "$PWD/internal/httpapi/webdist"
find internal/httpapi/webdist -mindepth 1 -delete
cp -R web/dist/. internal/httpapi/webdist/
```

5. 有效 HTTP 路由 RED（预期 404）：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL' -count=1
```

6. `buildHandler` 装配 RED（预期 MySQL 总览 404）：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./cmd/infraview -run 'TestBuildHandlerWiresAuthenticatedMockAPI' -count=1
```

7. 路由/view/405 与装配首次 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi ./cmd/infraview -run 'Test(MySQL|BuildHandlerWiresAuthenticatedMockAPI)' -count=1
```

8. 白名单与脱敏错误 RED：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL(OverviewRejectsQueryParameters|InstancesRejectsUnknownOrRepeatedQueryParameters|InstancesMapsUnavailableToSafe503)' -count=1
```

9. 首次白名单/503 GREEN 尝试（发现安全错误码自身包含禁词 `dat`）：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL(OverviewRejectsQueryParameters|InstancesRejectsUnknownOrRepeatedQueryParameters|InstancesMapsUnavailableToSafe503)' -count=1
```

10. 调整 MySQL 专属安全错误码后，白名单/503 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL(OverviewRejectsQueryParameters|InstancesRejectsUnknownOrRepeatedQueryParameters|InstancesMapsUnavailableToSafe503)' -count=1
```

11. 首轮格式化：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -w internal/httpapi/mysql_handlers.go internal/httpapi/api.go internal/httpapi/query_handlers.go internal/httpapi/api_test.go cmd/infraview/main.go cmd/infraview/main_test.go
```

12. 首轮 HTTP 全包：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -count=1
```

13. 首轮 main 全包：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./cmd/infraview -count=1
```

14. review 前全仓回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./... -count=1
```

15. reviewer 两项 Important 的 RED：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL(InstancesRejectsMalformedOrEmptyPaginationParameters|RoutesFailSafelyWithoutService)' -count=1
```

16. malformed/空分页修复 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQLInstancesRejectsMalformedOrEmptyPaginationParameters' -count=1
```

17. nil Service 安全 503 修复 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQLRoutesFailSafelyWithoutService' -count=1
```

18. review 修复后格式化：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -w internal/httpapi/mysql_handlers.go internal/httpapi/query_handlers.go internal/httpapi/api_test.go
```

19. review 修复后 HTTP 全包：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -count=1
```

20. 最终全仓回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./... -count=1
```

21. 最终格式/范围/路由门检：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -l internal/httpapi/mysql_handlers.go internal/httpapi/api.go internal/httpapi/query_handlers.go internal/httpapi/api_test.go cmd/infraview/main.go cmd/infraview/main_test.go
git status --short
git diff --check
git diff --stat
rg -n '^\s*mux\.Handle\("[A-Z]+ /api/v1/mysql/' internal/httpapi/api.go
```

22. 首次暂存 Task 8 文件与报告；6 个 Go 文件已暂存，但报告受 `.gitignore` 中 `.superpowers/` 规则影响，命令退出 1：

```bash
git add internal/httpapi/mysql_handlers.go internal/httpapi/api.go internal/httpapi/query_handlers.go internal/httpapi/api_test.go cmd/infraview/main.go cmd/infraview/main_test.go .superpowers/sdd/2026-07-28-mysql-module/task-8-implementer-report.md
```

23. 仅强制暂存任务明确要求的报告文件：

```bash
git add -f .superpowers/sdd/2026-07-28-mysql-module/task-8-implementer-report.md
```

24. 提交前检查：

```bash
git diff --cached --check
git diff --cached --stat
```

25. 提交：

```bash
git commit -m "feat: 提供 MySQL 只读查询 API"
```

## 最终验证结果

- `docker ... go test ./internal/httpapi -count=1`：PASS
- `docker ... go test ./... -count=1`：PASS；所有 Go 包 0 failures
- `docker ... gofmt -l ...`：无输出
- `git diff --check`：无输出
- MySQL method-qualified 路由扫描：仅两个 `GET`

---

# Fix Round 1

## 正式审查结论处理

- 修正基线：`80f2761bb5416bbdfe411c49818c0a05c7bbc9d1`
- Important 根因：`url.ParseQuery` 将 `?key` 和 `?key=` 都解析为单个空字符串；共享 `queryParameters` 原先只校验允许键和单值数量。`search`、`status`、`role`、`sort`、`order` 的空值因此被 Service 当作未设置，返回 200 并调用 provider。
- RED 覆盖：`search`、`status`、`role`、`sort`、`order`、`page`、`page_size` 七个允许参数，各自覆盖 bare key 与显式空值，共 14 个子测试；同时断言非法请求的 provider 调用次数为 0。
- RED 证据：10 个非分页子测试均得到 `status = 200, provider calls = 1`；分页参数已由既有整数解析返回 400。
- 最小修复：共享 `queryParameters` 在解析成功、键允许、值数量等于 1 后，再要求唯一值非空。所有 14 个子测试随后返回 400 且 provider 调用保持 0。
- Minor 契约测试：
  - 精确检查 overview/instances 的 envelope、meta、data、alerts、instance、replication key 集合；
  - 检查所有叶子 JSON 类型；
  - fixture 的 connections、max_connections、connection_usage_percent、threads_running、qps、slow_queries_per_second、buffer_pool_usage_percent、uptime_seconds、replication.lag_seconds 必须保持 JSON `null`；
  - 空快照必须编码为 `instances: []`，并保持 `total=0`、`page=1`、`page_size=20`、`total_pages=0`。
- 未改变 API 设计、路由、Service 查询语义或生产装配。

## Fix Round 1 变更文件

- 修改：`internal/httpapi/query_handlers.go`
- 修改：`internal/httpapi/api_test.go`
- 追加：本报告

所有受跟踪文件修改均通过 `apply_patch` 完成；`gofmt` 仅对两个 Go 文件做机械格式化。

## Fix Round 1 实际命令（按执行顺序）

1. 核对修正基线与工作树：

```bash
git rev-parse HEAD
git status --short
```

2. 首次空/bare 参数 RED：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQLInstancesRejectsEmptyAllowedParametersBeforeProvider' -count=1
```

3. 合并状态与 provider 调用断言后的精确 RED：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQLInstancesRejectsEmptyAllowedParametersBeforeProvider' -count=1
```

4. 共享 parser 最小修复后的 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQLInstancesRejectsEmptyAllowedParametersBeforeProvider' -count=1
```

5. Important 与 Minor 聚焦契约：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL(InstancesRejectsEmptyAllowedParametersBeforeProvider|ViewsExposeCompleteSchemaAndPreserveNullMetrics|InstancesEncodeEmptySnapshotAsArray)' -count=1
```

6. 格式化：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -w internal/httpapi/api_test.go internal/httpapi/query_handlers.go
```

7. 全部 MySQL HTTP 聚焦回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestMySQL' -count=1
```

8. 全仓 Go 回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./... -count=1
```

9. 差异与范围审查：

```bash
git status --short
git diff --check
git diff --stat
git diff -- internal/httpapi/query_handlers.go internal/httpapi/api_test.go
rg -n 'parameterValues\[0\]|RejectsEmptyAllowed|provider calls|ExposeCompleteSchema|EncodeEmptySnapshot|jsonPathIsNull|assertJSONObjectKeys' internal/httpapi/query_handlers.go internal/httpapi/api_test.go
```

10. 最终格式与差异门检：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -l internal/httpapi/api_test.go internal/httpapi/query_handlers.go
git diff --check
git status --short
```

11. 精确暂存 Fix Round 1 文件；三个文件均进入 index，但 Git 仍对显式 `.superpowers` 路径输出忽略提示并以 1 退出：

```bash
git add internal/httpapi/query_handlers.go internal/httpapi/api_test.go .superpowers/sdd/2026-07-28-mysql-module/task-8-implementer-report.md
```

12. 仅强制刷新暂存任务明确要求的原报告：

```bash
git add -f .superpowers/sdd/2026-07-28-mysql-module/task-8-implementer-report.md
```

13. 提交前检查：

```bash
git diff --cached --check
git diff --cached --stat
```

14. 单独修正提交：

```bash
git commit -m "fix: 拒绝 MySQL 空查询参数"
```

## Fix Round 1 最终验证结果

- `docker ... go test ./internal/httpapi -run 'TestMySQL' -count=1`：PASS
- `docker ... go test ./... -count=1`：PASS；所有 Go 包 0 failures
- 14 个 empty/bare 子测试：全部 400，provider 调用为 0
- 完整 schema、nullable null、空快照数组契约：PASS

---

# Fix Round 2

## 复审回归与修复

- 修正基线：`ea1a3ef464557a7b393d7f03b4719560416cb795`
- 回归根因：Fix Round 1 把“唯一值必须非空”加入共享 `queryParameters`，导致所有使用该 parser 的既有 Host API 在进入 `valueOrDefault` 或 Service 默认逻辑前返回 400。
- 兼容证据：前端已有 `status=` URL 语义；overview 和 metrics 的 handler 使用 `valueOrDefault(r, "range", "24h")`，明确把空 `range` 解释为默认 24h。
- RED 回归覆盖：
  - `GET /api/v1/overview?range=` 必须保持 200；
  - `GET /api/v1/hosts?status=` 必须保持 200；
  - `GET /api/v1/hosts/{id}/metrics?range=` 必须保持 200。
- RED 证据：首个 overview 请求得到 400。
- 最小修复：
  - 共享 `queryParameters` 恢复为只校验解析错误、允许键和单值数量，不改变既有默认行为；
  - `mysqlInstances` 在解析后调用 MySQL 专属 `hasEmptyMySQLQueryParameter`，在分页解析与 Service 调用前拒绝任一空值。
- 双向 GREEN：
  - 三个 Host 显式空值请求全部 200；
  - MySQL 七参数 × bare/显式空值共 14 个子测试继续全部 400，provider 调用保持 0；
  - 完整 JSON schema、nullable `null`、空快照 `instances: []` 契约不退化。
- 未改变 Host handler、Host Service、MySQL API 设计、路由或生产装配。

## Fix Round 2 变更文件

- 修改：`internal/httpapi/query_handlers.go`
- 修改：`internal/httpapi/mysql_handlers.go`
- 修改：`internal/httpapi/api_test.go`
- 追加：本报告

所有受跟踪文件修改均通过 `apply_patch` 完成；`gofmt` 仅对三个 Go 文件做机械格式化。

## Fix Round 2 实际命令（按执行顺序）

1. 核对修正基线、工作树与既有 Host 空值调用：

```bash
git rev-parse HEAD
git status --short
sed -n '105,245p' internal/httpapi/query_handlers.go
sed -n '292,325p' internal/httpapi/query_handlers.go
sed -n '85,125p' internal/httpapi/mysql_handlers.go
rg -n 'overview\?range=|metrics\?range=|hosts\?.*status=|status=' internal/httpapi/api_test.go web/src
```

2. Host 显式空值语义 RED：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'TestExistingHostRoutesPreserveExplicitEmptyQueryDefaults' -count=1
```

3. 移动校验边界后的 Host+MySQL 双向 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'Test(ExistingHostRoutesPreserveExplicitEmptyQueryDefaults|MySQLInstancesRejectsEmptyAllowedParametersBeforeProvider)' -count=1
```

4. 格式化：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -w internal/httpapi/api_test.go internal/httpapi/query_handlers.go internal/httpapi/mysql_handlers.go
```

5. 完整 Host+MySQL 聚焦回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'Test(MySQL|ExistingHostRoutesPreserveExplicitEmptyQueryDefaults)' -count=1
```

6. 全仓 Go 回归：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./... -count=1
```

7. 差异与范围审查：

```bash
git status --short
git diff --check
git diff --stat
git diff -- internal/httpapi/query_handlers.go internal/httpapi/mysql_handlers.go internal/httpapi/api_test.go
rg -n 'PreserveExplicitEmpty|hasEmptyMySQL|parameterValues\[0\]' internal/httpapi/api_test.go internal/httpapi/mysql_handlers.go internal/httpapi/query_handlers.go
```

8. 最终格式与差异门检：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/src" -w /src golang:1.24-bookworm gofmt -l internal/httpapi/api_test.go internal/httpapi/query_handlers.go internal/httpapi/mysql_handlers.go
git diff --check
git status --short
```

9. 精确暂存 Fix Round 2 Go 文件：

```bash
git add internal/httpapi/query_handlers.go internal/httpapi/mysql_handlers.go internal/httpapi/api_test.go
```

10. 仅强制暂存任务明确要求的原报告：

```bash
git add -f .superpowers/sdd/2026-07-28-mysql-module/task-8-implementer-report.md
```

11. 提交前检查：

```bash
git diff --cached --check
git diff --cached --stat
```

12. 单独修正提交：

```bash
git commit -m "fix: 保持主机空查询默认语义"
```

## Fix Round 2 最终验证结果

- Host `range=` / `status=` 聚焦回归：PASS
- MySQL 14 个 empty/bare 子测试：全部 400，provider 调用为 0
- MySQL 完整 schema、nullable null、空快照数组契约：PASS
- `docker ... go test ./internal/httpapi -run 'Test(MySQL|ExistingHostRoutesPreserveExplicitEmptyQueryDefaults)' -count=1`：PASS
- `docker ... go test ./... -count=1`：PASS；所有 Go 包 0 failures
