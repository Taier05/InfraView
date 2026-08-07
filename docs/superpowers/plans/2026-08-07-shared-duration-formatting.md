# 共享运行时长格式化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将六个运行时间列和硬盘通电时间统一为精确的“年、天、小时、分钟”动态展示，同时保持原始数值排序不变。

**Architecture:** 新增无 React 依赖的前端纯函数，将 `number|bigint|null` 统一转换为中文时长；普通模块传数值秒，Java 将规范十进制字符串转为 `bigint`，硬盘将小时换算为秒。Service、API 和排序字段完全不改。

**Tech Stack:** React 19、TypeScript 5.8、Vitest、Docker Node 22。

## Global Constraints

- 1 年固定为 365 天；只展示年、天、小时、分钟并隐藏 0 单位。
- 0 至 59 秒显示“不足1分钟”；缺失或非有限/负数防御性显示“暂无数据”。
- 秒数向下取整到完整分钟；后端字段和值不变。
- Java 规范十进制字符串必须经 `BigInt` 精确处理，禁止转为 `number`。
- 所有排序继续由 Service 使用原始秒数或通电小时数完成；不得按中文文本排序。
- 不改变自动刷新、Nightingale 查询、API、缓存、状态或只读边界。
- 只在一次性 Node 22 容器测试；不得启动服务、浏览器或端口。提交、推送、部署和重启均需单独授权。

## File map

- Create `web/src/formatters/duration.ts`、`duration.test.ts`：共享精确格式化。
- Modify/Test `web/src/features/hosts/HostListPage.tsx`、`mysql/MySQLPage.tsx`、`redis/RedisPage.tsx`：数值秒。
- Modify/Test `web/src/features/elasticsearch/ElasticsearchPage.tsx`、`rabbitmq/RabbitMQPage.tsx`：可空整数秒。
- Modify/Test `web/src/features/java/JavaPage.tsx`：十进制字符串转 `bigint`。
- Modify/Test `web/src/features/disks/DiskPage.tsx`：通电小时换算。
- Modify `docs/PROJECT_STATUS.md`、`docs/TODO.md`、`docs/HANDOFF.md`：记录交付。

---

### Task 1: 精确共享格式化工具

**Files:**
- Create: `web/src/formatters/duration.ts`
- Test: `web/src/formatters/duration.test.ts`

**Interfaces:**

```ts
export type DurationSeconds = number | bigint | null
export function formatDurationSeconds(value: DurationSeconds): string
```

- [ ] **Step 1: 写工具 RED 测试**

覆盖 `null`、负数、`NaN`、`Infinity`、0、59、60、3599、3600、90180、365 天、508 天 6 小时，以及大于 `Number.MAX_SAFE_INTEGER` 的 `bigint`。精确断言“1年 143天 6小时”、隐藏 0 单位和“不足1分钟”。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/formatters/duration.test.ts'
```

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现纯函数**

普通数值先验证 `Number.isFinite(value) && value >= 0`，再 `Math.floor` 后转 `BigInt`；`bigint` 直接验证非负。小于 60 秒提前返回“不足1分钟”，否则按下列常量分解并拼接非零单位：

```ts
const minute = 60n
const hour = 3_600n
const day = 86_400n
const year = 31_536_000n
```

- [ ] **Step 4: GREEN/typecheck 与提交**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/formatters/duration.test.ts && npm run typecheck'
```

经授权后仅提交两个新文件，提交信息 `feat: add precise duration formatter`。

---

### Task 2: 主机、MySQL、Redis 使用共享时长

**Files:**
- Modify/Test: `web/src/features/hosts/HostListPage.tsx`、`HostListPage.test.tsx`
- Modify/Test: `web/src/features/mysql/MySQLPage.tsx`、`MySQLPage.test.tsx`
- Modify/Test: `web/src/features/redis/RedisPage.tsx`、`RedisPage.test.tsx`

- [ ] **Step 1: 写页面 RED**

三页分别加入 90 秒、跨天、跨年、缺失和 `title` 断言；保留点击“运行时间”后 URL 排序键仍为 `uptime`、页码回到 1 的断言。

- [ ] **Step 2: 运行 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/hosts/HostListPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx'
```

Expected: FAIL，旧函数只显示天和小时。

- [ ] **Step 3: 最小实现**

删除三份本地 `uptime`，导入 `formatDurationSeconds`。每个运行时间单元格正文与原生 `title` 使用同一个返回字符串；不得改变 Column ID、sort 参数或 API 类型。

- [ ] **Step 4: GREEN/typecheck 与提交**

重复 Step 2 命令并追加 `&& npm run typecheck`。经授权提交六个页面/测试文件，提交 `feat: format core service uptimes`。

---

### Task 3: Elasticsearch、RabbitMQ、Java 精确时长

**Files:**
- Modify/Test: `web/src/features/elasticsearch/ElasticsearchPage.tsx`、`ElasticsearchPage.test.tsx`
- Modify/Test: `web/src/features/rabbitmq/RabbitMQPage.tsx`、`RabbitMQPage.test.tsx`
- Modify/Test: `web/src/features/java/JavaPage.tsx`、`JavaPage.test.tsx`

- [ ] **Step 1: 写页面 RED**

三页加入相同分钟/跨年/缺失矩阵；Java 使用超出安全整数范围的规范字符串，精确计算期望，不允许测试先转 `number`。

- [ ] **Step 2: 验证 Java 精度防线**

先运行旧格式 RED；实现后临时把 Java 转为 `Number(value)`，超大整数测试必须再次 RED，立即恢复正确实现。

- [ ] **Step 3: 最小实现**

Elasticsearch/RabbitMQ 直接传数值或 null；Java 使用：

```ts
formatDurationSeconds(value === null ? null : BigInt(value))
```

运行时 validator 仍先拒绝非规范、负数和带符号字符串。

- [ ] **Step 4: GREEN/typecheck 与提交**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx src/formatters/duration.test.ts && npm run typecheck'
```

经授权提交六个页面/测试文件，提交 `feat: format messaging service uptimes`。

---

### Task 4: 硬盘通电时间与完整验证

**Files:**
- Modify/Test: `web/src/features/disks/DiskPage.tsx`、`DiskPage.test.tsx`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `docs/HANDOFF.md`

- [ ] 测试加入整数小时、带小数小时、跨年、0 和缺失；点击仍发送 `sort=power_on_hours`。
- [ ] 运行旧格式 RED；将非负 `power_on_hours * 3600` 传给共享函数。若上游是整数小时，禁止凭空补分钟。
- [ ] 运行 DiskPage、共享工具和七页相邻回归、typecheck。
- [ ] 更新状态文档；运行前端全量测试、typecheck、build、Playwright `--list` 和 `git diff --check`。
- [ ] 经授权提交硬盘页、测试和文档，提交 `docs: record duration formatting delivery`；不得推送、部署或重启。
