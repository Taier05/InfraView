# Elasticsearch Overview Node Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Elasticsearch 总览卡补齐“异常节点 x / 总节点”汇总、严重/警告未知徽标和双空状态。

**Architecture:** 仅在现有 `ElasticsearchStatusCard` 中使用 API 已返回的 `clusters`/`nodes` 等级计数派生展示值，复用共享卡片结构与样式。不修改后端、查询、阈值或 API 契约。

**Tech Stack:** React 19、TypeScript、Vitest、Testing Library、Playwright、Docker。

## Global Constraints

- InfraView 始终只读，不增加任何运维写操作。
- 现有 8080 只连接测试 Nightingale，不创建其他 InfraView 端口。
- 不读取或输出私密环境内容、认证信息、现场标识或指标值。
- commit/push 继续暂停。

---

### Task 1: 总览卡节点汇总与空状态

**Files:**
- Modify: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/features/overview/OverviewPage.tsx`

**Interfaces:**
- Consumes: `ElasticsearchOverviewData.nodes` 与 `clusters`、`ModuleStatusCardShell`、`StatusBadge`。
- Produces: “异常节点”汇总区和双空数据卡片状态。

- [x] **Step 1: 写入汇总失败测试**

在默认完整 fixture 上断言卡片显示“异常节点”、手工推导的异常数/总数、“严重”和“警告/未知”徽标。该测试应因旧卡未渲染 `module-alert-summary` 失败。

- [x] **Step 2: 写入双空失败测试**

构造 `clusters` 与 `nodes` 五个计数全为零的完整 fixture，断言卡片为 `data-level="empty"`、显示“暂无 Elasticsearch 节点”且不显示四类告警网格。

- [x] **Step 3: 运行定向 RED**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/features/overview/OverviewPage.test.tsx'
```

预期：新增汇总和空状态断言因对应 UI 缺失而失败，旧断言保持可执行。

- [x] **Step 4: 最小实现**

在 `ElasticsearchStatusCard` 中：双 total 为零时返回共享 empty shell；否则派生 `affectedNodes = critical + warning + unknown` 和 `warningOrUnknown = warning + unknown`，渲染共享汇总 DOM，保留原四类 `MetricAlert`。

- [x] **Step 5: 运行定向 GREEN**

重复 Step 3，预期 `OverviewPage.test.tsx` 全部通过。

### Task 2: 浏览器契约、文档和 8080

**Files:**
- Modify: `web/e2e/elasticsearch.spec.ts`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`

**Interfaces:**
- Consumes: 现有 Elasticsearch 总览卡可访问名称与原 8080 Playwright 登录流程。
- Produces: 可持久的浏览器回归与交接记录。

- [x] **Step 1: 扩展 Playwright 断言**

在第一个 Elasticsearch 用例中，进入详情页前断言总览卡内“异常节点”可见；不断言或输出任何现场数值。

- [x] **Step 2: 全量验证并更新文档**

运行前端全量、typecheck/build、Playwright 静态发现、`git diff --check` 与无缓存生产镜像；记录 RED/GREEN、语义和未执行边界。

- [x] **Step 3: 原位重建并验收原 8080**

```bash
INFRAVIEW_ENV_FILE=/root/github/InfraView/.env INFRAVIEW_PORT=8080 docker compose --project-name infraview up -d --build --force-recreate infraview
```

验证容器 healthy、唯一 8080、安全基线、一次性 Chromium 中“异常节点”可见，不输出现场数值。

- [x] **Step 4: Git 收尾**

```bash
git diff --check
git status --short --branch
```

保留所有差异未提交，不 push。
