# InfraView MySQL 模块本地验证摘要

验证日期：2026-07-29

## 结论

本轮最终修正已完成本地 RED -> GREEN 和容器化验证。证据只覆盖仓库代码、完全脱敏夹具与一次性 Mock 环境，不代表真实 Nightingale 或生产环境已经验收。

## 本轮回归修正

- Nightingale 标量与复制延迟先过滤非法候选，再按时间选择最新有效样本；非法候选不更新已选时间。
- 复制通道身份按 `channel_name`、`master_host`、`master_uuid` 三个标签键是否存在判定，允许合法空值；缺键序列忽略，通道身份不进入领域对象或 API。
- `replication_threads` 按角色独立分类：仅 writable 零通道为 normal，read_only/unknown 零通道为 unknown 并计入警告风险。
- MySQL 总览警告徽标使用 `warning_instances`，文案明确为“警告风险”；unknown-only 不再显示“无警告”。
- 任意复制状态只要 `lag_seconds` 非 null，页面都显示秒数。

## RED -> GREEN 证据

- Nightingale 聚焦测试在修复前稳定暴露“旧有效值被新非法候选清空”和“空默认通道被丢弃”；修复后聚焦测试及适配器全包通过。
- Service 精确总览测试在修复前只差 `replication_threads` 的 read_only/unknown 零通道警告计数；修复后聚焦测试及 Service 全包通过。
- 前端聚焦 Vitest 在修复前稳定暴露 unknown-only 警告徽标口径及非 normal 复制状态不显示有效延迟；修复后两个页面测试文件全部通过。

## 本地容器化验证

- Go：全仓格式检查、普通测试、race 测试和静态编译通过。
- 前端：Vitest 67/67、TypeScript typecheck 和 production build 通过。
- 无缓存生产镜像：Dockerfile 内前端测试/typecheck/build、Go 普通/race 测试和 Go build 全部通过。
- E2E 安全：同名项目、孤儿资源、部分启动失败的精确隔离/清理测试通过。
- 一次性 Mock：API smoke、资源阈值检查和 Chromium 6/6 通过。
- 清理：专用 Compose 容器、网络、卷和本轮两个验证镜像均已删除；未触碰既有 `infraview` 项目。

## 生产步骤

以下步骤未执行，也没有以本地 Mock 结果代替：

- 真实 Nightingale v8.4.1 MySQL API smoke。
- 真实 Nightingale 数据下的 Chromium 页面验收。
- 任何现有 InfraView 服务重建、部署或重启。

上述真实步骤必须取得用户单独授权，并继续遵守不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识、地址、数量、指标值或上游正文的要求。

## 只读边界

InfraView 仍只展示数据。此次修正没有增加 SQL、写接口、任意 PromQL、任意代理、数据库连接、SSH、远程命令、实例重启、主从切换或配置修改能力。
