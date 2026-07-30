# InfraView 样本推进新鲜度修复设计

最后更新：2026-07-30

## 现场根因

测试 Nightingale 的 Linux `system_uptime` 与 MySQL `mysql_up` 原始样本仍约每 15 秒推进，但样本时间相对 InfraView 当前时间稳定落后数十秒。Nightingale HTTP 时钟与 InfraView 对齐，绕过 InfraView 缓存后时间差仍存在，因此问题位于采集端时间戳或采集/转发/写入链路。

现有实现直接计算“InfraView 当前时间减原始样本时间”，把稳定链路时延误判为采集停止。恢复采集后样本虽继续推进，绝对时间差仍超过 2 个周期，导致 Linux 与 MySQL 长期显示“采集延迟”。

## 目标与边界

- 采集新鲜度表达“原始样本是否继续推进”，不表达端到端时间差。
- 默认 15 秒周期不变：连续 2 个周期未推进为警告，5 个周期未推进为严重。
- Linux 与 MySQL 使用同一并发安全状态机。
- `tlast_over_time` 继续提供原始样本身份/时间；固定 batch、无 N+1、只读边界保持不变。
- 不新增配置、API 字段、持久化、任意查询或上游写入。
- 主机“采集延迟”使用黄色，“采集失联”使用红色，与 MySQL 一致。

## 样本推进状态机

Service 内新增进程内 `freshnessTracker`，按稳定资源 ID 保存：

- `sampleAt`：最近一次观察到的原始样本时间；
- `advancedAt`：InfraView 最近观察到 `sampleAt` 变化的本地时间。

每次成功从 Provider 加载新快照时调用 `Observe`：

1. 首次看到非零样本时建立基线，`advancedAt=now`，等级为正常。
2. 后续样本时间发生变化时认为采集正在推进，重置 `advancedAt=now`。
3. 样本时间未变化时保留 `advancedAt`。
4. `now-advancedAt >= 2*interval` 为警告，`>= 5*interval` 为严重。
5. Linux 已知主机没有心跳时间时直接严重；MySQL 非受跟踪的 Mock/兼容实例不应用推进判断。
6. 原始时间回退视为新的基线，避免采集端校时后持续误报。

Tracker 只在 Provider 成功加载时观察数据；读取缓存不会伪造推进。上游失败并返回 stale 缓存时，`advancedAt` 继续老化。状态只保存在内存中，InfraView 重启后重新建立基线，已停采对象最迟在 5 个周期后重新进入严重。

## 集成位置

- Linux：`Service.currentMetrics` 的成功 loader 中批量观察 `CurrentMetrics.Timestamp`；主机列表、总览和单机聚合按主机 ID 读取 tracker 等级。
- MySQL：`MySQLService.snapshot` 的成功 loader 中批量观察受跟踪实例的 `ReportedAt`；实例列表与总览按实例 ID 读取 tracker 等级。
- Provider 继续使用 `tlast_over_time(system_uptime[24h])` 和 `tlast_over_time(mysql_up[24h])`，不修改 Nightingale。

## 页面颜色

主机状态组件新增与 MySQL 相同的有效等级选择：

- `collection_level=warning` 优先使用 warning；
- `collection_level=critical` 优先使用 critical；
- 采集正常时再按主机 online/offline/unknown 映射 normal/critical/unknown。

CSS 统一使用 `data-level`，避免“采集延迟”文字与灰色 unknown 样式混用。

## 测试与验收

- Tracker 单元测试覆盖首次旧时间正常、时间不变 2/5 周期升级、时间推进恢复、时间回退重建基线和并发访问。
- Linux/MySQL Service 测试必须用多次 Provider 加载证明：稳定绝对时间差不告警，样本冻结才按 2/5 周期告警，恢复推进立即正常。
- 前端测试断言主机采集延迟为 warning、采集失联为 critical。
- 完成无缓存 Docker 构建后，只原位重建既有测试 Nightingale 8080；现场只输出脱敏等级和 stale 结论。
