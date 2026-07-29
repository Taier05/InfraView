# MySQL 表格细化验证记录

日期：2026-07-29
范围：`feature/mysql-module` 的 MySQL 表格契约、原 8080 部署与浏览器验收

## 安全与部署边界

- InfraView 始终只读；未连接生产，未修改 Nightingale、Categraf、MySQL 或私密配置。
- 仅强制重建既有 `infraview` Compose 项目的原 8080 测试服务；未创建第二个 InfraView 服务、端口、Compose 项目或预览。
- 未运行 `scripts/e2e.sh`，未执行远程命令，未推送或合并。
- 本记录不包含 `.env` 内容、Token、Cookie、认证头、Base URL、上游响应正文、真实地址/标识、资源数量或业务指标值。

## 精确技术契约

- Nightingale MySQL snapshot 固定为一次 `query-instant-batch`、14 条代码内置查询，无实例 N+1。新增查询为 `mysql_global_variables_innodb_buffer_pool_size`。
- 容量按 `BufferPoolSizeBytes` → `BufferPoolSizeBytes` → JSON `buffer_pool_size_bytes` 贯穿领域、Service 和 HTTP；字段可空，只接受非负最新有效值，缺失、非法或最新同时间戳冲突保持 `null`。
- `available_labels` 来自完整 snapshot 的非空实例名，去重后稳定排序，不受标签、状态、角色、搜索、排序或分页影响；`label` 去除首尾空白后按实例名精确匹配。服务端与页面 `search` 均只匹配实例地址或所属主机。
- 页面角色显示“读写/只读/未知”；Buffer Pool 依次支持“容量 / 使用率”“容量 / —”“— / 使用率”“—”。
- 11 列依次为实例地址、所属主机、版本 / 角色、连接、线程、QPS、慢查询、Buffer Pool、复制 / 延迟、运行时间、状态；桌面列宽为 `13/9/9/9/6/5/7/13/12/8/9%`。

## 验证结果

- `docker build --no-cache --progress=plain .`：PASS；覆盖前端全量 Vitest、typecheck、production build、Go 普通测试、Go race、Linux build 和镜像导出。
- `./scripts/e2e-safety.test.sh`：PASS。
- 固定 14 查询的一次 batch 定向 Go 契约测试：PASS。
- 旧部署在新几何断言下按预期 RED；失败来自新增单行/布局/对齐断言，不是认证、网络或页面不可达。
- 状态列首次部署暴露“圆点 + 间距”导致首文本起点偏移；修复提交 `aa54eae` 让 Host/MySQL 状态列表头预留对应偏移，并通过范围复审。
- 在 `aa54eae` 上重新完成无缓存构建和安全回归后，只重建既有原 8080；容器达到 healthy。
- 无正文 HTTP：`/healthz` 为 200 JSON，未认证受保护 API 为 401 JSON。
- 完整受跟踪 `mysql-compact-live.spec.ts`：Chromium 5/5 PASS。前四项覆盖四列总览、11 个单行表头、Host/MySQL 共享列文本起点差值 `<= 1px`、1440×900 页面/表格无水平溢出及 900px 控制区两行三列。
- 第五项受跟踪匿名用例在登录后才注册错误监听，并以布尔值或计数断言标签控件顺序、非空标签选择、URL/列表请求标签参数、首列纯地址、角色文案和 Buffer Pool 四种合法形态；认证后的 console error、page error、request failure 和 MySQL API 非成功响应均为 0。用例不返回页面文本、真实地址/标签、URL 参数值、响应正文或请求头。

## 结论

MySQL 表格细化在仅连接测试 Nightingale 的原 8080 上完成脱敏 GREEN 验收。固定查询从 13 增至 14，但仍只有一次 batch、无 N+1，产品未增加任意查询、数据库连接或运维写能力。当前仅形成本地提交，未推送、未合并，等待集成授权。
