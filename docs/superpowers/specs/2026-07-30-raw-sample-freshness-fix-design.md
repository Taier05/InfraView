# InfraView 原始样本新鲜度修复设计

> 2026-07-30 补充：绝对原始样本年龄在存在稳定链路时延时会误报，本设计的 Service 判定部分已由 `2026-07-30-sample-progress-freshness-fix-design.md` 的样本推进状态机替代；Provider 的 `tlast_over_time` 契约继续保留。

最后更新：2026-07-30

## 问题与证据

关闭测试采集器后，当前指标已经缺失，但主机仍因 Nightingale Target 时间较新而保持正常；MySQL 只有在即时 `mysql_up` 退出上游回看窗口后才进入严重。现有实现使用即时查询响应外层时间戳，并对主机取 Target 时间与指标时间的较新值，无法可靠表达原始样本最后写入时间。

测试 Nightingale 已只读确认 MetricsQL `tlast_over_time` 可用，并能对主机心跳与 MySQL 可用性返回超过严重阈值的原始最后样本时间。取证未输出上游响应正文或真实资源信息。

## 修复原则

- 采集新鲜度只依据原始指标最后样本时间，不依据查询求值时间。
- Target 状态继续表达 Nightingale 的资产/在线状态，但不再参与指标采集新鲜度。
- 默认预期采集周期仍为 `15s`：2 个周期警告、5 个周期严重。
- 固定查询、一次 batch、无 N+1 和严格只读边界保持不变。

## Linux 主机

主机当前指标固定 batch 增加：

```promql
tlast_over_time(system_uptime{<固定 ident 匹配器>}[24h])
```

该查询的数值是原始最后样本 Unix 时间。Provider 只用该数值设置 `CurrentMetrics.Timestamp`；CPU、内存、负载、IO、网络查询响应的外层时间戳不再更新采集时间。

Service 只使用 `CurrentMetrics.Timestamp` 判断新鲜度。已知主机没有 24 小时心跳时间时直接视为严重，不能被 Target `StatusTime` 掩盖。Target 原始在线/离线状态仍与采集等级合并为最终主机状态。

## MySQL

把已有近期身份查询：

```promql
max_over_time(mysql_up[24h])
```

替换为：

```promql
tlast_over_time(mysql_up[24h])
```

查询仍提供最近 24 小时出现过的完整实例身份，同时其数值提供原始最后样本时间。当前 `mysql_up` 继续判断实时可用性，但不再使用响应外层时间戳设置 `ReportedAt`。

MySQL Service 无论当前 `mysql_up` 是否仍被上游回看返回，都按 `ReportedAt` 执行 2/5 周期判断。实例超过 24 小时无样本后按原设计退出近期清单。

MySQL 固定即时查询保持 16 条、一次 `query-instant-batch`，不增加按实例查询。

## API 与页面

既有 `collection_level`、主机派生状态和 MySQL 最终状态契约不变，前端无需新增字段。修复后总览、主机清单和 MySQL 清单将在同一原始时间口径下显示“采集延迟”或“采集失联”。

## 测试与验收

- Provider 测试必须构造“外层时间新鲜、原始最后样本时间过期”的夹具，并证明采集时间采用查询数值。
- 主机测试必须证明 Target 时间持续更新也不能掩盖缺失或过期心跳。
- MySQL 测试必须证明当前 `mysql_up` 仍存在时，过期原始样本也会按 2/5 周期告警。
- 验证固定 PromQL、批量基数、非法原始时间、24 小时身份、Mock 兼容和无 N+1。
- 完成无缓存 Docker 构建后，只按当前授权原位重建既有测试 Nightingale 8080，不创建其他端口。
- 不输出 `.env`、Token、Cookie、认证头、真实标识、地址、数量、指标值或上游响应正文。
