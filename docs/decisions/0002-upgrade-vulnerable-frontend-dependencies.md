# 0002：升级存在已知漏洞的前端依赖

日期：2026-07-21

状态：已接受

## 背景

Task 5 按原实施计划安装前端依赖后，`npm audit --omit=dev` 报告 React Router DOM `7.6.2` 存在 High 风险，ECharts `5.6.0` 存在 Moderate 风险。全量审计还报告原锁定的 Vite、Vitest 和 Playwright 版本存在已修复漏洞。

继续使用原精确版本会将已知生产漏洞带入 InfraView，与平台的安全目标冲突。

## 决策

采用 Registry 已确认存在的修复版本：

- React Router DOM：`7.18.1`
- ECharts：`6.1.0`
- Vite：`7.3.6`
- Vitest：`3.2.7`
- Playwright：`1.61.1`

React、React DOM、TanStack Query、TanStack Table、TypeScript、jsdom、Testing Library、MSW 等未受本次审计结论影响的直接依赖保持原计划版本。

## 影响

- Task 5 必须重新生成锁文件并运行单元测试、类型检查、生产构建、生产依赖审计和全量依赖审计。
- Task 6 使用 ECharts 6 模块化 API 实现图表，不再以 ECharts 5 行为为准。
- 不执行 `npm audit fix --force`，避免引入未经审查的批量变更。
- 若升级后仍存在生产 High 或 Critical 风险，Task 5 不得通过审查。
