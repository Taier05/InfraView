# Task 2 report — 七个 HTTP GET 的 500 条契约

## Status

完成。仅新增 HTTP API 回归测试；未修改任何生产 handler、Service、响应字段或错误文案。

## 覆盖范围

- 七个认证 GET 列表都覆盖 `?page=1&page_size=500`：主机、硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java。
- 成功断言验证根包络仍为 `data`/`meta`、`page=1`、`page_size=500`，并以响应 `total` 计算验证 `total_pages=ceil(total/500)`。
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
