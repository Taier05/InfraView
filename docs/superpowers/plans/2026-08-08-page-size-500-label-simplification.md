# 每页 500 条文案简化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将七个只读列表的第四个每页数量选项从“全部（最多500条）”改为普通“500 条”，保持既有分页、排序、筛选和 API 行为不变。

**Architecture:** 仅在共享 `ListPage` 中移除 500 的特殊文案分支，七页继续传入同一组 `20|50|100|500`。测试锁定四个选项的精确文本及旧文案消失；当前状态文档同步为普通 500 条分页语义。

**Tech Stack:** React 19、TypeScript、Vitest、Testing Library、Vite、Docker Compose

## Global Constraints

- InfraView 始终只读展示，不增加写 API、运维控件或任意查询能力。
- 开发与验收仅使用现有测试 8080，不创建额外端口，不访问或输出私密环境与现场数据。
- `page_size` 允许值仍严格为 `20|50|100|500`；选择 500 后回到第 1 页。
- 排序、筛选在服务端完成后再分页；超过 500 条时正常进入后续页，不截断、不前端拼接。
- 500 与其他每页数量使用同一套翻页控件规则，不新增 500 专属分支。

---

### Task 1: 共享 500 条选项与当前文档

**Files:**
- Modify: `web/src/components/ListPage.tsx:128-132`
- Test: `web/src/components/ListPage.test.tsx:75-83`
- Modify: `docs/superpowers/specs/2026-08-07-list-all-duration-and-disk-error-summary-design.md:15-22`
- Modify: `docs/superpowers/plans/2026-08-07-list-page-size-500.md:1-180`
- Modify: `docs/PROJECT_STATUS.md:7-9`
- Modify: `docs/TODO.md:5-8`
- Modify: `docs/HANDOFF.md:5-7`

**Interfaces:**
- Consumes: `ListPageSizeField.pageSizes: readonly number[]` 与七页既有 `[20, 50, 100, 500]`。
- Produces: 每个 `option` 的值仍为原始数字，显示文本统一为 `` `${size} 条` ``。

- [ ] **Step 1: 写入失败测试**

将共享控件的精确标签断言改为：

```tsx
for (const label of ["20 条", "50 条", "100 条", "500 条"]) {
  expect(screen.getByRole("option", { name: label })).toBeInTheDocument();
}
expect(screen.queryByText("全部（最多500条）")).not.toBeInTheDocument();
```

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine sh -lc 'npm ci --ignore-scripts && npm test -- --run src/components/ListPage.test.tsx'
```

Expected: FAIL，找不到名称为“500 条”的选项；旧实现仍显示“全部（最多500条）”。

- [ ] **Step 3: 写入最小实现**

将共享组件的选项内容改为：

```tsx
<option key={size} value={size}>
  {`${size} 条`}
</option>
```

不得修改七页 `pageSizes`、URL 参数、GET 请求或后端白名单。

- [ ] **Step 4: 运行聚焦 GREEN 与类型检查**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine sh -lc 'npm ci --ignore-scripts && npm test -- --run src/components/ListPage.test.tsx && npm run typecheck'
```

Expected: 聚焦测试与 `tsc --noEmit` 均 PASS。

- [ ] **Step 5: 同步当前文档**

将旧设计、旧计划、`PROJECT_STATUS.md`、`TODO.md`、`HANDOFF.md` 中面向当前状态的“全部（最多500条）”改为普通“500 条”；明确超过 500 条继续分页、无 500 专属交互。保留历史提交和验证事实，不声称已合并或推送。

- [ ] **Step 6: 运行完整前端门禁**

Run:

```bash
docker run --rm -v "$PWD/web:/app" -w /app node:22-alpine sh -lc 'npm ci --ignore-scripts && npm test -- --run && npm run typecheck && npm run build && npx playwright test --list'
git diff --check
```

Expected: Vitest、TypeScript、Vite build、Playwright discovery 和 whitespace 检查全部 PASS；产品代码中不再出现旧选项文案。

- [ ] **Step 7: 本地提交**

```bash
git add web/src/components/ListPage.tsx web/src/components/ListPage.test.tsx \
  docs/superpowers/specs/2026-08-07-list-all-duration-and-disk-error-summary-design.md \
  docs/superpowers/plans/2026-08-07-list-page-size-500.md \
  docs/PROJECT_STATUS.md docs/TODO.md docs/HANDOFF.md \
  docs/superpowers/plans/2026-08-08-page-size-500-label-simplification.md
git diff --cached --check
git commit -m "fix: simplify 500-row page option"
```

- [ ] **Step 8: 原位部署测试 8080 并核验**

使用现有私密 Compose 环境文件与 `INFRAVIEW_PORT=8080` 原位 `--build --force-recreate` 同一 `infraview` 服务；不得打印环境文件内容或创建其他端口。部署后只检查容器健康状态、唯一 8080 映射、安全属性、`/healthz` 200 与未认证 API 401。
