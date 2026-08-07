# Java 展示优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Java 健康、端口、进程状态增加现有等级色，并将业务端下拉选项显示为精确中文名称且保持原始筛选代码。

**Architecture:** 只修改 Java 页面展示和测试。后端继续返回中文 `business` 与原始代码 `available_names`；前端用纯映射函数生成 option label，并用既有 `StatusBadge` 表示三个 nullable boolean 字段。

**Tech Stack:** React 19、TypeScript 5.8、TanStack Query/Table、Vitest、Docker。

## Global Constraints

- 不修改 Java Provider、固定 11 查询 batch、Service 状态计算、HTTP View 或 API 参数。
- `tikbee/rider/mch/saas/mch_saas` 仅精确映射；未知代码原样显示。
- option `value`、URL `name` 和请求参数保持原始代码。
- 正常为 `normal` 绿色，异常为 `critical` 红色，缺失为 `unknown` 灰色；不新增颜色体系。
- 页面始终只读；不得增加运维控件、写请求、任意 PromQL 或现场信息输出。
- 测试和构建仅在一次性容器执行；不得启动新端口。提交、推送、部署和重启均需单独授权。

## File map

- `web/src/features/java/JavaPage.tsx`：业务代码 label 和 nullable boolean 状态徽标。
- `web/src/features/java/JavaPage.test.tsx`：颜色、中文 option、原始请求值和未知回退。
- `web/src/app/theme.css`：仅在既有徽标无法满足表格紧凑度时增加 Java cell 包装样式。
- `docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`：Java 展示交付记录。

---

### Task 1: Java 状态色与业务端中文筛选

**Files:**
- Modify: `web/src/features/java/JavaPage.tsx:75-435`
- Modify: `web/src/app/theme.css:1250-1325`（仅在测试证明需要时）
- Test: `web/src/features/java/JavaPage.test.tsx:170-330`

**Interfaces:**
- Consumes: `JavaService.health_up|port_up|process_up` 的 `boolean|null`，`available_names: string[]`。
- Produces: `javaBusinessLabel(code: string): string`、三个 `StatusBadge` cell；不改变网络契约。

- [ ] **Step 1: 写 Java RED 测试**

用五个已知业务代码和一个未知代码构造 `available_names`，断言可见 label 与 value：

```ts
const expectedOptions = [
  ['用户端', 'tikbee'], ['骑手端', 'rider'], ['商家端', 'mch'],
  ['管理后台端', 'saas'], ['商家 PC 端', 'mch_saas'],
  ['future_business', 'future_business'],
]
for (const [label, value] of expectedOptions) {
  expect(within(select).getByRole('option', { name: label })).toHaveValue(value)
}
```

选择“骑手端”，断言 URL/请求仍为 `name=rider`。分别构造 `true`、`false`、`null`，断言三个状态 cell 内徽标文本为正常、异常、暂无数据且 `data-level` 为 `normal`、`critical`、`unknown`。

- [ ] **Step 2: 运行 Java RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/java/JavaPage.test.tsx'
```

Expected: FAIL，下拉仍显示原始代码且二值状态没有等级属性。

- [ ] **Step 3: 实现精确映射和 nullable boolean 徽标**

在 `JavaPage.tsx` 增加纯函数：

```tsx
const businessLabels: Readonly<Record<string, string>> = {
  tikbee: '用户端',
  rider: '骑手端',
  mch: '商家端',
  saas: '管理后台端',
  mch_saas: '商家 PC 端',
}

function javaBusinessLabel(code: string) {
  return businessLabels[code] ?? code
}

function BinaryStatus({ value }: { value: boolean | null }) {
  const level: MetricLevel = value === null ? 'unknown' : value ? 'normal' : 'critical'
  const label = value === null ? '暂无数据' : value ? '正常' : '异常'
  return <StatusBadge level={level} label={label} />
}
```

健康、端口、进程三列改用 `<BinaryStatus>`；端口进程一致性保持现有展示。下拉 options 改为 `{ value, label: javaBusinessLabel(value) }`，保留当前筛选值被服务端选项暂时移除时的回填逻辑。

- [ ] **Step 4: 运行 Java GREEN、相邻模块回归和 typecheck**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/java/JavaPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx && npm run typecheck'
```

Expected: PASS。

- [ ] **Step 5: 经授权后提交 Task 1**

```bash
git add web/src/features/java/JavaPage.tsx web/src/features/java/JavaPage.test.tsx web/src/app/theme.css
git diff --cached --check
git commit -m "feat: polish Java status presentation"
```

---

### Task 2: Java 展示全量验证与持久文档

**Files:**
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`

**Interfaces:**
- Consumes: Task 1 已验证页面。
- Produces: 脱敏、可恢复的交付记录。

- [ ] **Step 1: 运行前端全量门禁**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run && npm run typecheck && npm run build && npx playwright test --list'
```

Expected: 全部 PASS，不启动浏览器服务。

- [ ] **Step 2: 运行静态只读扫描**

```bash
rg -n 'fetch\(|apiRequest\(' web/src/features/java/JavaPage.tsx
rg -n 'method:\s*["'"'](POST|PUT|PATCH|DELETE)|执行命令|重启|删除|PromQL' web/src/features/java
git diff --check
```

Expected: 仅固定 GET；无破坏性能力；diff check 无输出。

- [ ] **Step 3: 更新当前状态文档**

记录三类状态色、五个精确中文 label、未知回退、原始筛选值兼容和验证结果；不记录现场值。若旧列表计划同时执行，合并写入同一组状态文档，避免重复段落。

- [ ] **Step 4: 经授权后提交 Task 2**

```bash
git add docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md
git diff --cached --check
git commit -m "docs: record Java display polish"
```
