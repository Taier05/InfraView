# Elasticsearch 总览节点汇总补齐设计

日期：2026-08-04

## 问题与根因

Elasticsearch 总览 API 已返回集群和节点各自的 `total / normal / warning / critical / unknown`，但总览卡只渲染“集群健康、节点资源、未分配分片、请求拒绝”四类告警，漏掉其他模块共有的 `module-alert-summary` 汇总区。既有前端测试只锁定四类告警，没有锁定节点汇总，因此未阻止该遗漏。

## 批准展示契约

- 主汇总文案为“异常节点”，不使用“异常主机”或将集群与节点混成一个数。
- 异常节点数为 `nodes.warning + nodes.critical + nodes.unknown`，分母为 `nodes.total`。
- 右侧保持两个紧凑徽标：`critical` 独立显示“严重 N”；`warning + unknown` 合并显示“警告/未知 N”。合并只用于紧凑展示，不改变各等级原始计数或状态语义。
- 下方四类告警保持不变；“集群健康”继续独立表达集群问题，遵守已批准的“异常集群与异常节点不混算”边界。
- 当 `clusters.total === 0` 且 `nodes.total === 0` 时，卡片显示“暂无 Elasticsearch 节点”空状态；只有一边为零时仍展示正常汇总，避免遮蔽另一边数据。

## 实现与验证边界

- 仅修改总览 React 组件、对应 Vitest/Playwright 断言和持久文档。
- 复用现有 `ModuleStatusCardShell`、`module-alert-summary`、`module-alert-total`、`module-alert-levels` 和 `StatusBadge`，不新增 Elasticsearch 专用样式。
- 不修改 Nightingale 查询、Provider、Service、API 或状态阈值；不增加任意 PromQL、Elasticsearch 直连或运维操作。
- 按 TDD 先验证旧卡缺少汇总/空状态的 RED，再以最小实现取得 GREEN；全量验证后仅原位重建现有测试 Nightingale 8080。
- 不创建其他 InfraView 端口，不连接生产，不读取或输出私密环境内容和现场数值；commit/push 继续暂停。
