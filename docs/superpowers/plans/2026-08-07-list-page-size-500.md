# 列表单页最多 500 条 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为七个只读列表增加“全部（最多500条）”，保持完整结果先排序后分页且不改变响应结构。

**Architecture:** Service 共享固定分页白名单 `20|50|100|500`，七个查询在排序完成后继续使用原分页切片。HTTP 只透传数值 `page_size`，前端共享分页控件负责 500 的中文标签，各模块继续使用自己的 URL 状态与查询缓存。

**Tech Stack:** Go 1.24、React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Docker。

## Global Constraints

- InfraView 始终只读；不得增加写 API、运维控件、任意 PromQL、代理、额外数据源请求或 N+1。
- 显式非零 `page_size` 唯一允许值为 20、50、100、500；省略参数时默认仍为 20，不支持 `all`、负数或其他非零区间值。
- Service 必须对完整筛选结果排序后再分页；前端不得本地排序或逐页拼接。
- 500 条以上继续正常分页，不截断、不创建第二套列表或虚拟滚动。
- 自动刷新仍为 15 秒；API 路径、响应包络和分页字段类型不变。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识、地址、数量、容量、指标值或上游正文。
- 测试、typecheck、build 仅在一次性容器执行；不得启动服务、浏览器或新端口。提交、推送、部署和重启均需单独授权。

## File map

- Create `internal/service/pagination.go`：共享 `validListPageSize(int) bool`。
- Modify 七个 Service 文件及对应测试：统一分页白名单和排序后分页证据。
- Test 六个 HTTP 测试文件：覆盖七个 GET 接口。
- Modify `web/src/components/ListPage.tsx` 和测试：500 的共享中文选项。
- Modify 七个列表 Page 与测试：分页常量、URL、请求和响应校验。
- Modify `docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`：记录交付和恢复状态。

---

### Task 1: Service 固定分页白名单和排序后切片

**Files:**
- Create: `internal/service/pagination.go`
- Modify: `internal/service/hosts.go`
- Modify: `internal/service/disk_service.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/redis_service.go`
- Modify: `internal/service/elasticsearch_service.go`
- Modify: `internal/service/rabbitmq_service.go`
- Modify: `internal/service/java_service.go`
- Test: `internal/service/service_test.go`
- Test: `internal/service/disk_service_test.go`
- Test: `internal/service/mysql_service_test.go`
- Test: `internal/service/redis_service_test.go`
- Test: `internal/service/elasticsearch_service_test.go`
- Test: `internal/service/rabbitmq_service_test.go`
- Test: `internal/service/java_service_test.go`

**Interfaces:**
- Produces: `func validListPageSize(value int) bool`
- Consumes: 七个 `*Query.PageSize int`；不改变任一公开查询或响应类型。

- [ ] **Step 1: 写 Service RED 测试**

每个 Service 增加表驱动断言：20、50、100、500 成功；1、19、21、499、501 及负数返回 `ErrInvalidQuery`。主机必须把旧的“1 到 100 任意值”收紧为同一白名单。`PageSize=0` 保留各模块现有内部默认语义：Elasticsearch、RabbitMQ、Java 规范化为 20，主机、硬盘、MySQL、Redis 仍返回 `ErrInvalidQuery`；所有 HTTP 接口的显式 `page_size=0` 一律返回 400。

每个列表再使用至少 501 个脱敏 fixture，按一个非默认字段降序请求 `Page=1, PageSize=500`，断言第 500 条和第 2 页第 1 条延续完整结果排序，而不是按插入顺序切片后排序。

- [ ] **Step 2: 运行 Service RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/service -run 'PageSize500|PageSizes|Pagination' -count=1
```

Expected: FAIL，500 被拒绝；主机仍错误接受非白名单值。

- [ ] **Step 3: 最小实现共享白名单**

```go
package service

func validListPageSize(value int) bool {
	switch value {
	case 20, 50, 100, 500:
		return true
	default:
		return false
	}
}
```

七个 `normalize*Query` 均调用该函数；保留各自默认值、页码校验和溢出保护。切片顺序仍为 `filter -> sort -> total -> start/end -> copy`。

- [ ] **Step 4: 运行 Service GREEN 与 race**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/service/pagination.go internal/service/hosts.go internal/service/disk_service.go internal/service/mysql_service.go internal/service/redis_service.go internal/service/elasticsearch_service.go internal/service/rabbitmq_service.go internal/service/java_service.go internal/service/service_test.go internal/service/disk_service_test.go internal/service/mysql_service_test.go internal/service/redis_service_test.go internal/service/elasticsearch_service_test.go internal/service/rabbitmq_service_test.go internal/service/java_service_test.go && go test ./internal/service -run "PageSize500|PageSizes|Pagination" -count=1 && go test -race ./internal/service -run "PageSize500|PageSizes|Pagination" -count=1'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 1**

只暂存 `pagination.go`、七个 Service 和七个对应测试，执行 `git diff --cached --check` 后提交：

```bash
git commit -m "feat: allow 500-row list pages"
```

---

### Task 2: 七个 HTTP GET 的 500 条契约

**Files:**
- Test: `internal/httpapi/api_test.go`
- Test: `internal/httpapi/disk_handlers_test.go`
- Test: `internal/httpapi/redis_handlers_test.go`
- Test: `internal/httpapi/elasticsearch_handlers_test.go`
- Test: `internal/httpapi/rabbitmq_handlers_test.go`
- Test: `internal/httpapi/java_handlers_test.go`

**Interfaces:**
- Consumes: 既有 GET 参数 `page`、`page_size`。
- Produces: 不新增字段；`page_size=500` 时响应 `page_size: 500`，`total_pages=ceil(total/500)`。

- [ ] **Step 1: 写 HTTP RED 测试**

分别请求七个列表的 `?page=1&page_size=500`，断言 200、响应包络不变和 `data.page_size == 500`。再逐一断言 `page_size=499|501|0|-1|空白` 为安全 400；使用现有认证 request helper，不输出 Cookie 或正文。

- [ ] **Step 2: 取得 RED/GREEN 证据**

在 Task 1 父提交加仅测试差异运行得到 RED，再在 Task 1 HEAD 运行得到 GREEN：

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/httpapi -run 'PageSize500|RejectsInvalidPageSize' -count=1
```

- [ ] **Step 3: 补齐必要的 handler 溢出防护**

若测试暴露某 handler 缺少 `page * page_size` 保护，只加入与 `diskDevices` 等价的 `maxInt` 校验；不得改变成功响应或安全错误文案。若无需生产修改，保持本 Task 为测试强化。

- [ ] **Step 4: 运行 HTTP race**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'gofmt -w internal/httpapi/api_test.go internal/httpapi/disk_handlers_test.go internal/httpapi/redis_handlers_test.go internal/httpapi/elasticsearch_handlers_test.go internal/httpapi/rabbitmq_handlers_test.go internal/httpapi/java_handlers_test.go && go test ./internal/httpapi -run "PageSize500|RejectsInvalidPageSize" -count=1 && go test -race ./internal/httpapi -run "PageSize500|RejectsInvalidPageSize" -count=1'
```

- [ ] **Step 5: 经授权后提交实际修改文件**

若修改了生产 handler，必须把其精确路径显式追加到 `gofmt` 和暂存命令。先用 `git diff --name-only` 收窄文件，禁止以宽泛通配符提交无关 handler；提交信息：

```bash
git commit -m "test: cover 500-row list APIs"
```

---

### Task 3: 共享分页控件与七页 URL

**Files:**
- Modify/Test: `web/src/components/ListPage.tsx`、`ListPage.test.tsx`
- Modify/Test: `web/src/features/hosts/HostListPage.tsx`、`HostListPage.test.tsx`
- Modify/Test: `web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`
- Modify/Test: `web/src/features/mysql/MySQLPage.tsx`、`MySQLPage.test.tsx`
- Modify/Test: `web/src/features/redis/RedisPage.tsx`、`RedisPage.test.tsx`
- Modify/Test: `web/src/features/elasticsearch/ElasticsearchPage.tsx`、`ElasticsearchPage.test.tsx`
- Modify/Test: `web/src/features/rabbitmq/RabbitMQPage.tsx`、`RabbitMQPage.test.tsx`
- Modify/Test: `web/src/features/java/JavaPage.tsx`、`JavaPage.test.tsx`

**Interfaces:**
- `ListPageSizeField.pageSizes` 仍为 `readonly number[]`。
- 七页 `PageSize` 联合类型自动扩展为 `20|50|100|500`。

- [ ] **Step 1: 写前端 RED 测试**

共享控件断言第四项可见名称严格为“全部（最多500条）”、值为 `500`。七页分别切换并断言 URL `page_size=500&page=1`、最后一次 GET 参数为 500、响应校验接受 500；非法 499/501 规范化为 20。至少一个页面用 `total=501,total_pages=2` 证明仍分页而非截断。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/components/ListPage.test.tsx src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx'
```

Expected: FAIL，500 选项不存在或校验拒绝响应。

- [ ] **Step 3: 最小实现**

七页统一 `const pageSizes = [20, 50, 100, 500] as const`。共享控件用 `size === 500 ? '全部（最多500条）' : \`${size} 条\`` 渲染标签，不增加页面专属分支。

- [ ] **Step 4: GREEN/typecheck 并提交**

重复 Step 2 命令并追加 `&& npm run typecheck`。经授权后仅暂存共享组件和七页的 16 个实际文件，提交 `feat: add 500-row list option`。

---

### Task 4: 文档与完整验证

- [ ] 更新 `PROJECT_STATUS/TODO/HANDOFF`，只记录分页交付，不提前宣称时长或硬盘摘要完成。
- [ ] 前端运行全量测试、typecheck、build、Playwright `--list`；Go 运行 gofmt/vet、全仓普通/race、编译；运行 `git diff --check` 与只读/敏感扫描。
- [ ] 经授权后仅提交三份状态文档，提交 `docs: record 500-row list delivery`。不得推送、部署或重启。
