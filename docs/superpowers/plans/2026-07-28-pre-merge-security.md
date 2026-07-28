# InfraView 合并前安全修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在公开仓库提交前移除真实部署情报，并让 Nightingale Token 默认只能通过 HTTPS 发送，现有 HTTP 测试环境必须显式选择不安全模式。

**Architecture:** 配置层新增默认关闭的 `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP`，HTTP Base URL 只有显式开启时才通过；Provider 的 `Options` 同样携带该布尔值，避免绕过配置层直接构造出明文认证客户端。公开文档只保留文档地址、虚构 Target 和通用部署描述，真实运行值继续留在被 Git 忽略的私有环境中。

**Tech Stack:** Go 1.24、`net/url`、现有配置加载器、Nightingale Provider、Docker。

## Global Constraints

- 不输出或提交真实 Token、认证头、Cookie、上游响应正文或私有环境文件内容。
- 默认仅允许 HTTPS Nightingale Base URL。
- HTTP 兼容仅用于明确选择的受控测试环境，必须显式配置。
- Mock 模式继续忽略 Nightingale 专属配置。
- 不改变 InfraView 只读 API、固定 PromQL、缓存和刷新周期。

---

### Task 1: HTTP 显式不安全开关

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/adapters/nightingale/provider.go`
- Modify: `cmd/infraview/main.go`
- Modify: `cmd/infraview/main_test.go`
- Modify: `.env.example`

- [x] **Step 1: 写配置与 Provider 的失败测试**

新增用例断言：Nightingale HTTP URL 默认返回安全配置错误；设置 `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=true` 后配置加载成功；直接构造 Provider 时 HTTP 默认不可用，只有 `Options.AllowInsecureHTTP=true` 才可用于 `httptest`。

- [x] **Step 2: 运行聚焦测试确认 RED**

```bash
docker run --rm -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/config ./internal/adapters/nightingale ./cmd/infraview
```

- [x] **Step 3: 实现双层最小保护**

在 `Config` 和 `nightingale.Options` 增加 `AllowInsecureHTTP bool`；配置层使用现有布尔解析器加载开关，Nightingale 模式下遇到 HTTP 且开关为 false 时返回不包含 URL/Token 的固定错误；Provider 构造器执行同一 scheme 检查；主程序传递该字段。

- [x] **Step 4: 更新受影响测试构造**

所有基于 `httptest.Server` 的 Provider 测试显式设置 `AllowInsecureHTTP: true`，HTTPS 和非法 URL 测试保持默认关闭，以证明默认安全边界。

- [x] **Step 5: 运行聚焦测试确认 GREEN**

重复 Step 2 命令，期望三个包全部通过。

### Task 2: 公开文档脱敏与部署兼容

**Files:**
- Modify: `.env.example`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: 其他命中真实部署情报或 HTTP 配置说明的跟踪文档
- Modify: `/secure/path/infraview.env`（被 Git 忽略，只增加显式布尔开关，不读取或输出内容）

- [x] **Step 1: 机械替换公开部署情报**

将真实内网 IP、端口、精确部署版本、Target 标识、物理网卡和绝对私密路径替换为 RFC 5737 文档地址、通用名称或环境变量占位；保留字段契约和验证结论。

- [x] **Step 2: 补充 HTTPS 默认与 HTTP 显式风险说明**

`.env.example` 默认设置 `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=false`；配置、安全、部署和 Nightingale 文档说明只有受控测试环境才能显式开启，生产应使用 HTTPS 和最小只读 Token。

- [x] **Step 3: 更新当前私有部署并重建**

不输出私有文件内容；只设置 `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=true`，保持权限 600，随后重建现有 `infraview` Compose 服务。

- [x] **Step 4: 验证**

运行完整前端测试、Go 普通/race 测试、类型检查、生产构建、`git diff --check`、敏感信息文件名扫描、容器健康和真实 Nightingale 只读冒烟。修复后再次执行独立阻断项复审。
