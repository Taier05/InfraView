# 0003：总览趋势使用服务端聚合历史接口

日期：2026-07-21

状态：已接受

## 背景

基础设施总览需要按 1 小时、6 小时、24 小时和 7 天展示 CPU、内存趋势。现有 `QueryRange` 面向单台主机和单个指标；若总览先列出主机，再按主机查询历史，会产生主机数乘指标数的 N+1 请求。若浏览器从刷新快照自行积累历史，则范围选择无法代表真实历史，刷新或重启页面也会丢失数据。

首版规模上限为 100 台主机，每条趋势序列不超过约 600 个点。InfraView 需要让数据源适配器在最接近上游的层完成聚合，同时保持领域契约不暴露 Nightingale 查询表达式。

## 决策

- `datasource.Provider` 新增 `QueryAggregateRange(context.Context, AggregateRangeRequest) ([]Series, error)`。
- `AggregateRangeRequest` 明确包含 `Keys`、`Start`、`End`、`Step`，不复用 `RangeRequest.HostIDs` 的空值作为聚合哨兵。
- 一次请求可读取 CPU、内存等多个规范化指标键；总览每个范围只调用一次聚合接口。
- Mock 适配器对最多 100 台已配置主机的确定性点值求平均，不使用随机数。
- Nightingale 空壳在真实 API 未明确前继续返回 `ErrNotConfigured`，不猜测上游接口。
- Service 使用既有 range TTL（默认 60 秒）缓存聚合趋势、合并并发加载，并把 stale/collected_at 与清单和当前指标元数据合并。
- HTTP `/api/v1/overview` 输出规范化 `trends[].key/unit/points`；浏览器只绘制返回点并生成读屏文字摘要，不聚合主机或生成历史点。

## 影响

- 总览历史上游调用数不随主机数量增长，避免 N+1。
- 范围切换具有真实语义，并能复用服务端缓存和旧数据降级。
- Provider 的所有实现、timeout wrapper 与测试 fake 都必须实现新方法。
- 后续 Nightingale 适配器必须把多指标聚合映射到实际、受控的上游查询；在测试环境信息明确前不实现猜测版本。
- 单主机详情继续使用 `QueryRange`，两类请求保持清晰边界。
