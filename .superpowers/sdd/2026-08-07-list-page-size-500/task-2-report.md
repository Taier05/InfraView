# Task 2 report — 七个 HTTP GET 的 500 条契约

## Status

完成。仅新增 HTTP API 回归测试；未修改任何生产 handler、Service、响应字段或错误文案。

## 覆盖范围

- 七个认证 GET 列表都覆盖 `?page=1&page_size=500`：主机、硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java。
- 成功断言验证根包络仍为 `data`/`meta`、每个接口完整 `data` key 集，以及固定边界 `total=501`、首列表长度 `500`、`page=1`、`page_size=500`、`total_pages=2`。
- 七个列表均覆盖 `page_size=499`、`501`、`0`、`-1` 与空白，确认返回安全的 `400 invalid_query` 包络。
- 测试使用现有认证 request helper；新增断言不输出响应正文、Cookie、Token、认证头或数据标识。

## TDD evidence

- RED：在 Task 1 前的产品快照 `eb08b85` 上只应用本 Task 的六个测试差异，七个 `page_size=500` 用例均如预期返回 400，故测试失败。
- 临时 Git 快照起初缺少被忽略的 `internal/httpapi/webdist`，因 `//go:embed webdist` 在编译前失败；该失败不计作 RED。随后仅复制当前 worktree 已有的忽略构建产物作为编译前提，重新运行后取得上述行为级 RED。临时 worktree 已删除。
- GREEN：在 Task 1 完成的当前 HEAD `2a891a0` 上，新增 focused 测试通过；Service 的既有白名单改动使其自然转绿。

## Verification

在 Go 1.24 一次性容器中完成：

- `go test ./internal/httpapi -run 'PageSize500|RejectsInvalidPageSize' -count=1` — PASS。
- `go test ./internal/httpapi -count=1` — PASS。
- `go test -race ./internal/httpapi -run 'PageSize500|RejectsInvalidPageSize' -count=1` — PASS。
- 对六个测试文件执行 `gofmt`，并完成 `git diff --check`。

## Files

- `internal/httpapi/api_test.go`
- `internal/httpapi/disk_handlers_test.go`
- `internal/httpapi/redis_handlers_test.go`
- `internal/httpapi/elasticsearch_handlers_test.go`
- `internal/httpapi/rabbitmq_handlers_test.go`
- `internal/httpapi/java_handlers_test.go`
- `.superpowers/sdd/2026-08-07-list-page-size-500/task-2-report.md`

## Concerns

- 没有发现 Task 2 覆盖的 `page_size` 输入会暴露额外 handler 溢出防护缺口，因此没有生产代码变更。
- 临时 RED 依赖本地已存在、Git 忽略的 `webdist` 仅为满足 Go embed 编译；这不是产品代码差异，也未提交。

## Review fix round 1

- 审查发现第一轮成功 fixture 均少于 500 条，且辅助断言会根据响应自身的 `total` 推导 `total_pages`，无法证明 500 条分页边界；现已改为每个接口独立的 501 条脱敏 fixture/provider。
- 七个成功断言现在固定验证 `total=501`、第一页列表长度为 `500`、`page_size=500`、`total_pages=2`，并分别锁定各接口完整 `data` key 集：主机、硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java 的列表字段、分页字段和筛选选项字段均被覆盖。
- 可逆 mutation RED 1：临时把 `internal/httpapi/mysql_handlers.go` 的 `total_pages` 固定为 `1`，运行 `go test ./internal/httpapi -run '^TestMySQLInstancesPageSize500$' -count=1` 以 exit 1 失败，关键断言为 `total/total_pages = 501/1, want 501/2`。
- 可逆 mutation RED 2：临时把 MySQL `available_labels` 的 JSON tag 改为 `-`，运行同一命令以 exit 1 失败，关键断言为 `JSON path "data" has 5 keys, want 6`。
- 每次 mutation 后均立即恢复生产文件；`git hash-object internal/httpapi/mysql_handlers.go` 恢复为前置 SHA `45caaaf44691985a6cfbfe307bbda607618725e5`，且 `git diff --exit-code -- internal/httpapi/mysql_handlers.go` 为 exit 0。
- 恢复后的 focused GREEN：`go test ./internal/httpapi -run 'PageSize500|RejectsInvalidPageSize' -count=1`，exit 0。
- 最终容器验证：`go test ./internal/httpapi -count=1`，exit 0；`go test -race ./internal/httpapi -run 'PageSize500|RejectsInvalidPageSize' -count=1`，exit 0；`git diff --check`，exit 0。
