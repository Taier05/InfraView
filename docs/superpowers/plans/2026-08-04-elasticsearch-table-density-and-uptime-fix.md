# Elasticsearch Table Density and Uptime Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Elasticsearch 16 列在 1440×900 紧凑完整展示，并修复健康颜色和 uptime 空值。

**Architecture:** 保持既有 Provider→Service→API→React 契约，只为 uptime 增加专用安全数值解析；前端在节点页内增加角色摘要和健康徽标映射，并调整 Elasticsearch 专属表格 CSS。所有改变先以真实行为测试获得 RED，再最小实现为 GREEN。

**Tech Stack:** Go 1.24、React 19、TypeScript、Vitest、Playwright、Docker。

## Global Constraints

- InfraView 始终只读，不增加 Elasticsearch 直连、任意 PromQL 或运维操作。
- 现有 8080 只连接测试 Nightingale，不创建其他 InfraView 端口。
- 不读取或输出私密环境、认证信息、Base URL、现场标识、数量或指标值。
- 未获授权不得 commit/push；本计划所有 commit 步骤保持不执行。

---

### Task 1: Uptime 数值解析

**Files:**
- Modify: `internal/adapters/nightingale/elasticsearch_provider.go`
- Test: `internal/adapters/nightingale/elasticsearch_provider_test.go`

**Interfaces:**
- Consumes: `instantSeries`、`parseInstantValue`、`elasticsearchLatest[int64]`。
- Produces: uptime 专用的有限非负秒数解析，最终仍写入 `Node.UptimeSeconds *int64`。

- [x] **Step 1: 写失败测试**

加入表驱动测试，使用字面量覆盖科学计数、小数秒、负数、NaN、Inf 与越界值；合法值期望向下取整，非法值期望 `nil`。测试必须通过完整 Provider 快照行为观察结果。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm go test ./internal/adapters/nightingale -run 'Test.*Elasticsearch.*Uptime' -count=1
```

预期：科学计数或小数合法样本得到 `nil`，测试失败。

- [x] **Step 3: 最小实现**

为 `elasticsearchUptimeQuery` 调用专用 merge/parser；使用现有浮点即时值解析，验证有限、非负且不超过 `math.MaxInt64`，再向下取整转 `int64`。其他整数指标路径不变。

- [x] **Step 4: 运行 GREEN 与定向 race**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD:/src" -w /src golang:1.24-bookworm sh -c 'go test ./internal/adapters/nightingale -count=1 && go test -race ./internal/adapters/nightingale -count=1'
```

### Task 2: 角色摘要与健康颜色

**Files:**
- Modify: `web/src/features/elasticsearch/ElasticsearchPage.tsx`
- Test: `web/src/features/elasticsearch/ElasticsearchPage.test.tsx`

**Interfaces:**
- Consumes: `ElasticsearchRole[]`、`ElasticsearchHealth`、现有 `.status-badge`。
- Produces: 前两个角色摘要与完整 `title`；健康等级到 badge `data-level` 的固定映射。

- [x] **Step 1: 写失败测试**

用完整节点响应验证：多角色单元格只显示前两个与 `…`，`title` 为完整角色字符串；单角色/空角色不伪造省略号；green/yellow/red/unknown 分别映射 normal/warning/critical/unknown。

- [x] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/tmp/npm-ci.log && npm run test:run -- src/features/elasticsearch/ElasticsearchPage.test.tsx'
```

预期：旧页面仍完整显示全部角色，健康单元格没有 badge 等级，测试失败。

- [x] **Step 3: 最小实现**

新增局部纯函数生成角色摘要/完整 title 和健康等级；单元格只改变展示，不改变 API、排序或筛选。

- [x] **Step 4: 运行 GREEN**

重复 Step 2 命令，预期该文件全部通过。

### Task 3: 1440px 紧凑布局

**Files:**
- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/elasticsearch.spec.ts`
- Test: `web/src/features/elasticsearch/ElasticsearchPage.test.tsx`

**Interfaces:**
- Consumes: `.elasticsearch-table-scroll`、`.elasticsearch-table` 和固定 16 列顺序。
- Produces: 1440×900 桌面无表格横向滚动，更窄视口内部滚动兜底。

- [x] **Step 1: 更新失败断言**

把 1440×900 E2E 契约从“表格必须内部溢出”改为“页面和表格均不横向溢出”，并锁定角色省略号/title、16 格单行、代表数值不截断。

- [x] **Step 2: 用当前 8080 运行定向 Chromium RED**

使用不发布端口、不截图、不保留 trace 的一次性 Playwright 容器；仅输出固定布尔结论。预期旧 150rem 表格使桌面表格溢出断言失败。

- [x] **Step 3: 最小 CSS 实现**

改为 100% 固定布局，移除 150rem/18rem 强制宽度；为 16 列设置紧凑宽度、padding 和字体。角色及身份列单行裁剪并保留 title；窄屏 media query 设置有限 min-width 以启用内部滚动。

- [x] **Step 4: 运行前端 GREEN**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/tmp/npm-ci.log && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

### Task 4: 文档、全量验证与现有 8080

**Files:**
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/superpowers/specs/2026-08-04-elasticsearch-table-density-and-uptime-fix-design.md`
- Modify: `docs/superpowers/plans/2026-08-04-elasticsearch-table-density-and-uptime-fix.md`

- [x] **Step 1: 全仓离线验证**

运行前端全量、Go gofmt/vet/普通/race/编译、无缓存生产镜像、静态安全与 whitespace 扫描；不得运行会创建 18080 的 E2E 栈。

- [x] **Step 2: 更新文档**

记录 RED→GREEN、运行时间根因、布局契约、warning 和未执行边界，不记录现场值。

- [x] **Step 3: 原位重建现有 8080**

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env INFRAVIEW_PORT=8080 docker compose --project-name infraview up -d --build --force-recreate infraview
```

验证 healthy、唯一 8080、非 root、只读根文件系统、cap drop `ALL`、禁止提权，以及脱敏 API/Chromium/既有模块回归。

- [x] **Step 4: Git 收尾检查**

```bash
git diff --check
git status --short --branch
```

保持所有 Elasticsearch 差异未提交，等待后续 commit/push 授权。

## 执行结果

- uptime Provider 、角色/健康页面和 1440 表格契约均先取得 RED，再以最小实现转 GREEN。
- fresh 全量通过：前端 12 文件/157 项、typecheck/build、Playwright 17 项静态发现；Go gofmt/vet/全仓普通/race/编译；无缓存镜像 `infraview:elasticsearch-density-uptime-verify`。
- 原 8080 已原位重建且仍仅连接测试 Nightingale；脱敏 API、容器安全和 Chromium 3/3 均通过，未创建额外 InfraView 端口。
- 未 commit/push；依赖 warning 保持不变，未执行强制审计修复。
