# InfraView 采集新鲜度与 MySQL TPS 优化设计

> 2026-07-30 补充：本设计最初采用的查询外层时间口径会被 Nightingale 求值时间掩盖，已由 `2026-07-30-raw-sample-freshness-fix-design.md` 的 `tlast_over_time` 原始样本时间方案替代。

最后更新：2026-07-29

## 目标

修复采集器停止后 Linux 主机与 MySQL 实例仍显示正常、MySQL 实例随后直接消失的问题，并完成总览去重、实例地址排序、隐藏所属主机和增加 TPS。

InfraView 继续保持只读：只通过 Nightingale 固定查询读取指标，不连接 MySQL，不提供任意 PromQL、代理、写接口或运维操作。

## 采集新鲜度

- 新增 `INFRAVIEW_EXPECTED_COLLECTION_INTERVAL`，默认 `15s`，表示预期采集周期。
- 最新样本或 Target 心跳年龄达到 2 个采集周期时标记为警告。
- 年龄达到 5 个采集周期时标记为严重。
- Linux 主机优先使用最新有效指标时间与 Target `StatusTime` 中较新的时间判断采集新鲜度。
- Nightingale 当前指标完全缺失时保持零时间，不再伪造当前时间。
- 主机采集警告映射为 `unknown`，采集严重映射为 `offline`；页面状态分别显示“采集延迟”和“采集失联”。

## MySQL 实例保留

MySQL 固定即时 batch 增加：

```promql
max_over_time(mysql_up[24h])
```

该查询只用于发现最近 24 小时出现过的实例身份。当前 `mysql_up` 仍用于实时可用性和样本时间：

- 当前样本存在且新鲜：按 `mysql_up` 正常判断。
- 当前样本存在但达到 2/5 个采集周期：采集状态升级为警告/严重。
- 仅在 24 小时发现查询中存在：保留实例，清空即时指标并标记“采集失联”。
- 超过 24 小时没有任何 `mysql_up` 样本：实例退出近期清单。

这样可以跨 InfraView 重启恢复近期实例，同时避免把已永久移除的实例无限保留。

## TPS

Categraf 的 `mysql_global_status_commands_total` 使用 `command` 标签归并 `Com_*` 计数。TPS 使用固定查询：

```promql
sum by (ident, instance, address) (
  rate(mysql_global_status_commands_total{command=~"commit|rollback"}[5m])
)
```

TPS 表示显式 `COMMIT` 与 `ROLLBACK` 的每秒速率；默认 autocommit 工作负载可能被低估。页面合并展示为“QPS / TPS”，排序继续明确按 QPS 执行。

加入 TPS 与近期实例发现后，MySQL 固定查询由 14 条增加为 16 条，仍通过一次 `query-instant-batch` 完成，不增加按实例 N+1。

## 页面调整

- 总览 Linux/MySQL 卡片删除底部状态分解标签，只保留上方异常严重度和底部查看入口。
- MySQL 第一列按展示的实例地址排序，不再按隐藏实例标签排序；保留 `sort=instance` 兼容已有 URL。
- 删除 MySQL 表格“所属主机”列。
- 自由搜索只匹配实例地址；`host` 字段暂时保留在 API 中用于兼容和内部关联。
- MySQL 表格使用 10 列紧凑布局，“QPS / TPS”同列显示。

## 测试与安全边界

- 严格执行 RED→GREEN TDD。
- Nightingale 契约测试验证固定 16 查询、单 batch、实例保留和样本时间。
- Service 测试覆盖 2/5 周期边界、主机与 MySQL 总览计数、地址排序和仅地址搜索。
- API/前端测试覆盖新增字段、失联文案、总览去重和 10 列布局。
- 使用 Docker 完成 Go 普通/race、前端 Vitest、类型检查和生产构建。
- 未经单独授权，不重建 8080、不部署、不提交、不推送。
