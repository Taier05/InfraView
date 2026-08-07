# 最新数据时间与 Elasticsearch stale 稳定性设计

日期：2026-08-07

## 目标

- 删除正常页面中的手动刷新按钮和“上次刷新 / 每 15 秒自动刷新”文案，后台自动刷新保持不变。
- 页面改为展示来自夜莺样本的 `最新数据时间：YYYY/MM/DD HH:mm:ss`。
- 修复 Elasticsearch 任一非关键指标结果组短暂为 `null` 时整页错误回退旧缓存的问题。
- 保持 InfraView 严格只读、固定查询、单 batch、无 N+1。

## 时间语义

继续使用现有响应字段 `meta.collected_at`，但纠正其语义：它必须是当前响应快照实际采用的最新上游样本时间，不再使用 InfraView 缓存写入时间、HTTP 响应时间或浏览器时间。

- 主机当前数据：取本次快照所有有效 `CurrentMetrics.Timestamp` 的最大值。
- 主机趋势：取所有有效趋势点时间的最大值。
- 硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java：取快照内所有有效 `ReportedAt` 的最大值。
- Elasticsearch 同时比较集群和节点时间。
- 没有任何有效样本时间时省略 `collected_at`，前端显示“最新数据时间：暂无数据”。
- stale 缓存返回原快照及其原始样本时间，不把失败发生时间伪装成数据时间。
- 多来源 meta 合并时取最新有效样本时间；stale 状态仍使用逻辑或合并。

“最新数据时间”只说明该模块快照中最新一条数据的时间，不代表所有实体同一时刻采集；逐实体采集停滞继续由现有 `collection_level` 表达。

## 前端行为

- 删除 `RefreshControl` 及所有正常态手动刷新入口。
- 新增共享 `DataTime` 组件，统一解析、格式化和展示 `meta.collected_at`。
- 列表页控制区最后展示模块数据时间；后台 `refetchInterval` 不变。
- 总览不再显示统一刷新控制和自动刷新文案，每张模块卡片展示自己的数据时间。
- 首次加载失败、已有数据后台刷新失败时，现有 `ErrorPanel` 的“重试”按钮保留。
- stale 横幅继续存在，但展示缓存快照真实样本时间，不再展示缓存写入时间。
- 时间无效或缺失时不猜测，显示“暂无数据”。

## Elasticsearch 修复

`ElasticsearchSnapshot` 继续一次提交固定 26 条查询，并严格要求返回恰好 26 个结果组。

- 第 25 组集群 inventory 和第 26 组节点 inventory 是建立领域身份的关键组；任一为 `null` 仍判定数据源不可用。
- 第 1–24 组是当前状态或可选指标；某组为 `null` 时按空结果处理，对应字段保持 `unknown` 或 `null`，不能让整份快照失败。
- HTTP、认证、响应 envelope、结果组数量、inventory 时间或身份冲突等安全错误仍判定不可用，并允许缓存 stale 兜底。
- 不改变任何查询文本、查询顺序、batch 数量或身份规则。

## 验证

- TDD 证明可选 Elasticsearch 组为 `nil` 时快照成功，关键 inventory 为 `nil` 时仍失败。
- Service 测试证明 `collected_at` 来自样本、stale 时保持原值、空快照不伪造时间。
- 前端测试证明正常页面没有“刷新”按钮及旧文案，列表和七张总览卡分别展示格式化数据时间，错误态仍可重试。
- 容器内执行前端全量测试、typecheck、build；Go 格式、全量普通测试、race 测试和编译。

## 安全边界

- 不访问生产 Nightingale、RabbitMQ 或其他现场上游。
- 不读取或输出环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/容量/指标值或上游正文。
- 不新增端口，不启动或重建 8080。
- 未获得单独授权前不提交、不推送、不部署。
