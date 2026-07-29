# MySQL Table Refinement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修正主机与 MySQL 表头对齐，精简 MySQL 表格语义，并在不增加 Nightingale 请求次数的前提下提供实例标签筛选和 Buffer Pool 容量 / 使用率。

**Architecture:** 保持 InfraView 全链路只读。Nightingale 适配器在现有 `query-instant-batch` 中增加一个固定 PromQL；服务层从完整快照生成稳定标签选项并组合筛选；HTTP API 以向后兼容的可空字段和列表扩展响应；React 页面消费新契约，CSS 统一可排序表头起始边界，Playwright 在唯一的原 8080 服务上验证实际布局。

**Tech Stack:** Go、React、TypeScript、TanStack Query/Table、CSS、Docker、Playwright

## Global Constraints

- 全程只读访问 Nightingale；不得增加写接口、运维动作、任意 PromQL 或逐实例查询。
- MySQL 固定批量查询由 13 条增加到 14 条，但仍只发起一次 `query-instant-batch`，禁止 N+1。
- 只使用现有 `infraview` Compose 项目和 `http://192.168.8.200:8080/`；不得创建额外端口或预览服务。
- 开发与验证只连接测试 Nightingale，永不连接生产环境。
- 不输出 `.env`、Token、Cookie、认证头、上游响应正文、真实主机或实例标识、IP、资源数量和指标值。
- 所有实现先写失败测试，再写最小实现；每个任务完成后独立提交，不推送、不合并。
- 容量指标缺失或冲突不得改变实例健康状态和告警等级。

---

### Task 1: 打通 Buffer Pool 容量只读数据链路

**Files:**

- Modify: `internal/adapters/nightingale/mysql_promql.go`
- Modify: `internal/adapters/nightingale/mysql_provider.go`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/mysql/types.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/mysql_types.go`
- Modify: `internal/service/mysql_service_test.go`
- Modify: `internal/httpapi/mysql_handlers.go`
- Modify: `internal/httpapi/api_test.go`

- [ ] **Step 1: 为适配器写容量指标失败测试**

在 `provider_test.go` 的 MySQL 快照测试中补第 14 组批量结果，覆盖：

- `mysql_global_variables_innodb_buffer_pool_size` 能映射到 `BufferPoolSizeBytes`；
- 负数、`NaN`、`Inf` 被忽略；
- 采用最新有效样本；
- 相同最新时间戳出现不同值时结果为 `nil`；
- 断言批量查询数量为 14，且仍只有一次批量请求。

运行：

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/mysql ./internal/service ./internal/httpapi -count=1
```

预期：因查询和字段尚未实现而失败。

- [ ] **Step 2: 最小实现 Nightingale 容量采集**

在 `mysqlPromQL()` 的 Buffer Pool 使用率之后加入：

```go
"mysql_global_variables_innodb_buffer_pool_size",
```

在标量枚举中加入 `mysqlBufferPoolSize`，将标量合并范围扩展为查询索引 2–10，调整复制指标索引为 11–13，并让容量使用现有 `nonNegative`、最新有效值及同时间戳冲突规则。向 `mysql.Instance` 增加：

```go
BufferPoolSizeBytes *float64
```

同时更新快照克隆，确保缓存返回值不共享指针。

- [ ] **Step 3: 为服务和 HTTP 契约写失败测试**

测试 `BufferPoolSizeBytes`：

- 从领域对象映射到 `MySQLInstanceSummary`；
- API JSON 键为 `buffer_pool_size_bytes`；
- 缺失时序列化为 `null`；
- 不参与状态和告警计算。

运行：

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/mysql ./internal/service ./internal/httpapi -count=1
```

预期：因服务/API 字段尚未实现而失败。

- [ ] **Step 4: 扩展服务与 API 响应**

在服务摘要与 HTTP view 中增加：

```go
BufferPoolSizeBytes *float64 `json:"buffer_pool_size_bytes"`
```

只做透传，不加入排序、健康状态或告警逻辑。

- [ ] **Step 5: 运行后端回归并提交**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/mysql ./internal/service ./internal/httpapi -count=1
git diff --check
git status --short
git add internal/adapters/nightingale/mysql_promql.go internal/adapters/nightingale/mysql_provider.go internal/adapters/nightingale/provider_test.go internal/mysql/types.go internal/service/mysql_service.go internal/service/mysql_types.go internal/service/mysql_service_test.go internal/httpapi/mysql_handlers.go internal/httpapi/api_test.go
git diff --cached --check
git commit -m "feat: expose MySQL Buffer Pool capacity"
```

预期：测试通过，提交只包含容量链路。

---

### Task 2: 增加实例标签精确筛选

**Files:**

- Modify: `internal/service/mysql_types.go`
- Modify: `internal/service/mysql_service.go`
- Modify: `internal/service/mysql_service_test.go`
- Modify: `internal/httpapi/mysql_handlers.go`
- Modify: `internal/httpapi/api_test.go`

- [ ] **Step 1: 写服务层筛选失败测试**

覆盖：

- `available_labels` 从完整快照的 `Name` 去重并稳定升序；
- 标签列表不受状态、角色、搜索、标签、排序和分页影响；
- `label` 按实例名精确匹配；
- 同名不同地址实例全部保留；
- 标签可与状态、角色、搜索、排序和分页组合；
- 自由搜索仅匹配地址和所属主机，不再匹配实例名。

运行：

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src golang:1.24-bookworm \
  go test ./internal/adapters/nightingale ./internal/mysql ./internal/service ./internal/httpapi -count=1
```

预期：因查询字段与标签列表尚未实现而失败。

- [ ] **Step 2: 实现服务层标签契约**

扩展类型：

```go
type MySQLQuery struct {
    Label string
    // existing fields
}

type MySQLPage struct {
    AvailableLabels []string
    // existing fields
}
```

在遍历筛选前，从完整快照建立 `map[string]struct{}`，忽略纯空白名称并排序；筛选使用原始规范名称精确相等：

```go
if query.Label != "" && summary.Name != query.Label {
    continue
}
```

自由搜索只检查 `Address` 与 `Host`。对 `Label` 使用 `strings.TrimSpace` 规范化。

- [ ] **Step 3: 写 HTTP 参数与响应失败测试**

覆盖：

- `label` 是允许的查询参数并传入服务；
- 重复、空值和未知参数继续返回 `invalid_query`；
- 响应包含稳定的 `available_labels`；
- 空结果页仍返回完整标签选项。

- [ ] **Step 4: 实现 HTTP 映射**

将 `label` 加入参数白名单和 `service.MySQLQuery`，并在页面 view 增加：

```go
AvailableLabels []string `json:"available_labels"`
```

返回时复制列表，避免响应层意外共享底层切片。

- [ ] **Step 5: 运行后端回归并提交**

```bash
docker build --target backend-test --progress=plain .
git diff --check
git status --short
git add internal/service/mysql_types.go internal/service/mysql_service.go internal/service/mysql_service_test.go internal/httpapi/mysql_handlers.go internal/httpapi/api_test.go
git diff --cached --check
git commit -m "feat: add MySQL instance label filter"
```

---

### Task 3: 收紧 MySQL 前端语义与交互

**Files:**

- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`
- Modify: `web/src/features/mysql/MySQLPage.tsx`
- Modify: `web/src/features/mysql/MySQLPage.test.tsx`

- [ ] **Step 1: 更新类型和匿名测试夹具**

向 `MySQLInstance` 增加：

```ts
buffer_pool_size_bytes: number | null
```

向 `MySQLInstancePageData` 增加：

```ts
available_labels: string[]
```

所有夹具使用匿名值，不引入真实实例信息。

- [ ] **Step 2: 写前端失败测试**

覆盖：

- “实例标签”位于“实例状态”之前，选项来自 `available_labels`；
- 选择标签后 URL 与请求均包含 `label`，其他筛选保留且页码回到 1；
- 直接访问带 `label` 的 URL 可恢复选择；
- 搜索提示只提地址和所属主机；
- 第一列标题为“实例地址”，单元格只显示 `address`，完整地址保留在 `title`；
- 所有 `writable` 文案显示“读写”，包括角色筛选；
- 11 个短表头及完整 `title` 正确；
- Buffer Pool 四种缺失组合分别显示“容量 / 使用率”“容量 / —”“— / 使用率”“—”。

运行：

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx'
```

预期：新增断言失败。

- [ ] **Step 3: 实现 URL、请求与筛选控件**

增加 `label` 的 URL 读取、规范化、Query Key 和请求参数；以响应 `available_labels` 渲染下拉框：

```tsx
<label className="host-status-filter">
  <span>实例标签</span>
  <select
    value={label}
    onChange={(event) => updateParameters({ label: event.target.value })}
  >
    <option value="">全部标签</option>
    {availableLabels.map((value) => (
      <option key={value} value={value}>
        {value}
      </option>
    ))}
  </select>
</label>
```

标签值仅来自后端或当前 URL，控件不得发起额外 API 请求。

- [ ] **Step 4: 实现精简表头和单元格**

采用以下桌面表头：

```text
实例地址 | 所属主机 | 版本 / 角色 | 连接 | 线程 | QPS | 慢查询 | Buffer Pool | 复制 / 延迟 | 运行时间 | 状态
```

每个短表头通过 `title` 保留完整语义。第一列仅渲染地址；`writable` 显示为“读写”。

增加字节容量格式化和组合函数，规则固定为：

```text
容量和使用率都有 -> 容量 / 使用率
只有容量         -> 容量 / —
只有使用率       -> — / 使用率
两者都没有       -> —
```

容量使用 IEC 单位（B、KiB、MiB、GiB、TiB），不得把缺失值显示为 0。

- [ ] **Step 5: 运行前端回归并提交**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx'
git diff --check
git status --short
git add web/src/api/types.ts web/src/test/fixtures.ts web/src/features/mysql/MySQLPage.tsx web/src/features/mysql/MySQLPage.test.tsx
git diff --cached --check
git commit -m "fix: refine MySQL table semantics"
```

---

### Task 4: 修复共享列对齐并加入浏览器几何验收

**Files:**

- Modify: `web/src/app/theme.css`
- Modify: `web/e2e/mysql-compact-live.spec.ts`
- Modify: `web/src/features/hosts/HostListPage.test.tsx`
- Modify: `web/src/features/mysql/MySQLPage.test.tsx`

- [ ] **Step 1: 为结构约束写失败测试**

单元测试确认主机和 MySQL 可排序表头仍使用统一 `.host-sort-button`，MySQL 11 列均有单行标题语义和完整 `title`。

Playwright 增加匿名几何断言：

- 1440×900 下 Host 从 CPU 使用率到状态的表头与首个数据单元格左边界差值均 `<= 1px`；
- MySQL 从连接到状态的左边界差值均 `<= 1px`；
- MySQL 11 个表头的 `scrollHeight <= clientHeight + 1`，证明未换行；
- 表格容器和文档均无水平溢出。

运行前端单元测试；浏览器 RED 留到原 8080 部署前执行，禁止启动额外服务。

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx src/features/hosts/HostListPage.test.tsx'
```

- [ ] **Step 2: 实现统一起始边界与列宽**

移除可排序按钮的水平内边距，保留垂直点击区域：

```css
.host-sort-button {
  padding: 0.12rem 0;
}
```

MySQL 表头强制单行，并使用总和为 100% 的列宽：

```css
13% 9% 9% 9% 6% 5% 7% 13% 12% 8% 9%
```

桌面保持无横向滚动；`<= 1100px` 保持表格内部滚动。调整 `.mysql-list-controls` 以容纳搜索、标签、状态、角色、每页数量与刷新六个控件，并保持窄屏换行。

- [ ] **Step 3: 运行前端回归并提交**

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/npm-cache \
  -v "$PWD:/src" -w /src/web node:22-alpine \
  sh -c 'npm ci && npm run test:run -- src/features/mysql/MySQLPage.test.tsx src/features/hosts/HostListPage.test.tsx'
git diff --check
git status --short
git add web/src/app/theme.css web/e2e/mysql-compact-live.spec.ts web/src/features/hosts/HostListPage.test.tsx web/src/features/mysql/MySQLPage.test.tsx
git diff --cached --check
git commit -m "fix: align infrastructure table columns"
```

---

### Task 5: 完整验证、原 8080 部署与项目文档收口

**Files:**

- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/datasources/NIGHTINGALE.md`
- Modify: `docs/superpowers/specs/2026-07-28-mysql-module-design.md`
- Modify: `docs/superpowers/plans/2026-07-29-mysql-table-refinement.md`
- Create: `docs/superpowers/reports/2026-07-29-mysql-table-refinement-verification.md`

- [ ] **Step 1: 运行完整无缓存构建与安全回归**

```bash
docker build --no-cache --progress=plain .
./scripts/e2e-safety.test.sh
git diff --check
```

只记录通过/失败与匿名结论，不输出测试数据正文。

- [ ] **Step 2: 在原 8080 上先执行浏览器 RED**

使用已有测试凭据运行 `web/e2e/mysql-compact-live.spec.ts`，确认旧部署尚未满足至少一项新断言。不得显示凭据或页面业务数据，不得启动额外端口。

```bash
(
  INFRAVIEW_ENV_FILE=/root/github/InfraView/.env
  set -a
  . "$INFRAVIEW_ENV_FILE"
  set +a
  docker run --rm --network host --ipc=host \
    -e CI=1 \
    -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
    -e INFRAVIEW_E2E_USERNAME="$INFRAVIEW_USERNAME" \
    -e INFRAVIEW_E2E_PASSWORD="$INFRAVIEW_PASSWORD" \
    -v "$PWD:/work" \
    -v /work/web/node_modules \
    -w /work/web \
    mcr.microsoft.com/playwright:v1.61.1-noble \
    sh -c 'npm ci >/dev/null && npx playwright test e2e/mysql-compact-live.spec.ts'
)
```

- [ ] **Step 3: 仅重建现有 8080 服务**

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env INFRAVIEW_PORT=8080 docker compose -p infraview up -d --build --force-recreate
```

该命令只重建既有测试服务；执行前再次确认 Compose 项目名为 `infraview`、端口为 8080。

- [ ] **Step 4: 进行无正文健康检查与 Chromium 验收**

验证：

- `/healthz` 和受保护页面返回预期状态，只记录状态码与 Content-Type；
- 原 8080 的 Host、MySQL 对齐差值满足 `<= 1px`；
- 11 个 MySQL 表头单行；
- 1440×900 无页面和表格水平溢出；
- 标签筛选、地址展示、“读写”和 Buffer Pool 格式正确；
- 页面没有控制台错误或失败请求；
- 服务仍只连接测试 Nightingale，查询批次固定为 14。

执行：

```bash
(
  INFRAVIEW_ENV_FILE=/root/github/InfraView/.env
  set -a
  . "$INFRAVIEW_ENV_FILE"
  set +a
  docker run --rm --network host --ipc=host \
    -e CI=1 \
    -e INFRAVIEW_E2E_BASE_URL=http://127.0.0.1:8080 \
    -e INFRAVIEW_E2E_USERNAME="$INFRAVIEW_USERNAME" \
    -e INFRAVIEW_E2E_PASSWORD="$INFRAVIEW_PASSWORD" \
    -v "$PWD:/work" \
    -v /work/web/node_modules \
    -w /work/web \
    mcr.microsoft.com/playwright:v1.61.1-noble \
    sh -c 'npm ci >/dev/null && npx playwright test e2e/mysql-compact-live.spec.ts'
)
```

- [ ] **Step 5: 更新文档和计划勾选**

记录：

- 新指标名、字段契约、缺失/冲突语义和固定查询数；
- 标签筛选与搜索边界；
- UI 文案、列宽和浏览器几何验收；
- 原 8080 部署结果；
- 保持只读、未推送、未合并。

验证报告不得包含真实环境标识、资源数量和指标值。

- [ ] **Step 6: 文档检查并提交**

```bash
git diff --check
git status --short
git add docs/HANDOFF.md docs/PROJECT_STATUS.md docs/TODO.md docs/datasources/NIGHTINGALE.md docs/superpowers/specs/2026-07-28-mysql-module-design.md docs/superpowers/plans/2026-07-29-mysql-table-refinement.md docs/superpowers/reports/2026-07-29-mysql-table-refinement-verification.md
git diff --cached --check
git commit -m "docs: record MySQL table refinement verification"
git show --check --stat --oneline HEAD
git status --short --branch
```

- [ ] **Step 7: 停止并等待集成授权**

汇报本地提交、验证结果和原 8080 状态。不得自行推送、合并到 `main` 或改动生产部署。
