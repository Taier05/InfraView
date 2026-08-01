# InfraView 硬盘容量独立指标与分列设计

日期：2026-08-01
状态：已实现并完成完整离线及现有 8080 现场验收
范围：硬盘容量数据来源、容量排序及硬盘表格分列；不改变硬盘状态、采集新鲜度、分页或其他错误摘要语义

## 背景

当前硬盘列表把“型号 / 容量”放在同一列，虽然已改成两行，但型号与容量仍不能独立浏览和排序。容量目前来自第 17 组 inventory 序列的 `capacity` 标签；部分设备没有该标签，因此即使型号存在，容量仍显示“暂无数据”。

测试 Nightingale 已新增 `smart_disk_capacity_bytes`。该指标的数值表示硬盘容量字节数，本轮将它作为唯一容量来源。开发和验证仍只允许使用现有 8080 所连接的测试 Nightingale，禁止连接、切换或探测生产 Nightingale。

## 已确认范围

- 将“型号 / 容量”拆成“型号”和“容量”两列。
- 容量唯一读取 `smart_disk_capacity_bytes` 的指标值。
- 容量支持服务端升序和降序排序。
- 不接入命令超时。
- 不增加失联告警移除、人工移除或观测排除策略。
- 不增加“全部”分页；继续保持每页 20、50、100 条。
- 不改变 SMART 状态、错误摘要、总览统计或采集新鲜度规则。

## 固定 Nightingale 查询

硬盘快照仍只发送一次 `POST /api/n9e/query-instant-batch`，禁止按主机或设备 N+1。固定查询从 17 条增加为 18 条：

1. `smart_device_health_ok`
2. `smart_device_temp_c`
3. `smart_attribute_temperature_celsius`
4. `smart_attribute_percentage_used`
5. `smart_attribute_power_on_hours`
6. `smart_attribute_critical_warning`
7. `smart_attribute_available_spare`
8. `smart_attribute_available_spare_threshold`
9. `smart_attribute_value{fail=~"FAILING_NOW|In_the_past"}`
10. `smart_device_pending_sector_count`
11. `smart_device_reallocated_sectors_count`
12. `smart_device_uncorrectable_sector_count`
13. `smart_device_udma_crc_errors`
14. `smart_attribute_media_and_data_integrity_errors`
15. `smart_attribute_error_information_log_entries`
16. `smart_attribute_unsafe_shutdowns`
17. `smart_disk_capacity_bytes`
18. `tlast_over_time(smart_device_health_ok[24h])`

约束：

- 查询顺序继续作为 Provider 契约，由测试锁定。
- 外层结果必须恰好包含 18 组；第 1 组当前健康和第 18 组 inventory 仍不得为 `null`。
- 第 17 组容量允许为空或 `null`；容量不可用不能导致整个硬盘快照失败。
- PromQL 只能来自代码内固定列表，API 和前端不能传入指标名、PromQL 或原始请求体。
- 第 18 组继续负责设备发现、型号元数据和原始最后样本时间；容量指标不负责发现设备。

## 容量归并与数值规则

- 先由第 18 组 inventory 建立设备集合，再将第 17 组容量归并到已知设备。
- 容量序列沿用现有匹配规则：先按 `ident + device` 定位，再用双方均非空的 `serial_no`、`wwn` 校验身份。
- 无法匹配已知设备或身份明确冲突的容量序列忽略，不能创建新设备，也不能泄露原始身份标签。
- 只接受有限、非负、数学意义为整数且可安全转换为 `int64` 的字节值。
- 同一设备选择时间戳最新的有效容量；较旧值不能覆盖较新值。
- 同一最新时间戳存在不同有效容量时，该设备容量保持 `null`，不能任意选择。
- 容量在转为 `int64` 前必须从 Nightingale 原始数值文本精确解析，不能先经过 `float64`；缺失、负数、NaN、Inf、小数、越界、解析失败或冲突均返回 `capacity_bytes: null`。`2^53` 以上合法整数必须保持原值，同一时间不同大整数不得因浮点舍入漏判冲突。
- inventory 中旧的 `capacity` 标签不再读取、归并、冲突检查或回退；型号仍来自 inventory 的 `model` 标签。
- 容量不参与 `ReportedAt`、样本推进观察、120/300 秒 freshness、SMART 状态或总览告警。

HTTP API 继续使用现有可空字段：

```json
{
  "capacity_bytes": null
}
```

不增加新的 API 字段，也不返回 Nightingale 原始标签。

## 服务端排序

- `GET /api/v1/disks/devices` 的 `sort` 白名单新增 `capacity`。
- `sort=capacity&order=asc` 按容量字节从小到大排序。
- `sort=capacity&order=desc` 按容量字节从大到小排序。
- 两种方向都把 `null` 放在所有有效容量之后。
- 容量相同或都缺失时继续以稳定设备 ID 收口，保证刷新和翻页顺序稳定。
- 默认排序、搜索、状态筛选与每页 20/50/100 条均保持不变。

## 前端表格

硬盘表由 9 列变为 10 列：

1. 主机
2. 设备
3. 型号
4. 容量
5. SMART 健康
6. 温度
7. 寿命
8. 通电时间
9. 错误摘要
10. 状态

展示规则：

- 型号列只显示型号；缺失时显示“暂无数据”，长文本单行省略，`title` 保留完整值。
- 容量列表头使用现有排序按钮与 `⇅/↑/↓` 状态；容量按 IEC 单位格式化为 B、KiB、MiB、GiB、TiB 或 PiB。
- 容量缺失显示“暂无数据”，完整格式化结果保留在 `title`。
- 删除只为同列两行服务的 `.disk-model-capacity` 包装语义；型号和容量成为两个独立单元格。
- 桌面十列初始宽度依次为 `11/8/15/9/10/7/8/9/14/9%`，合计 100%；实现阶段只允许为通过真实几何验收做小幅调整，不能恢复合并列或引入页面横向滚动。
- 窄屏继续使用现有紧凑表格策略，不新增第二个页面或移动端卡片视图。

## 错误处理与缓存

- 容量字段级缺失或非法只影响该字段，不把快照变成 503。
- 外层 batch 基数错误、必需健康/inventory 组为 `null`、协议或安全校验失败继续映射为现有安全不可用错误。
- 快照 TTL、最大 stale、前端刷新周期和 stale 提示不变。
- 错误信息、日志和测试报告不得包含 Token、认证头、上游正文、真实标识、IP、容量值或其他现场数据。

## 测试与验收

### Provider 与领域

- 固定查询测试锁定 18 条顺序，容量为第 17 条、inventory 为第 18 条，仍只有一次 batch。
- 覆盖有效容量、缺失/null、负数、小数、NaN、Inf、越界、较新值、较旧值和同一最新时间冲突，并锁定 `2^53+1`、`MaxInt64`、大整数小数及大整数同时间冲突的精确语义。
- 覆盖可选身份标签、明确身份冲突、未知设备容量序列和旧 inventory `capacity` 标签不再回退。
- 锁定容量指标不改变设备发现、`ReportedAt` 或 freshness。

### Service 与 API

- `capacity` 加入排序白名单，升降序正确且缺失值始终最后。
- 非法排序仍返回统一 400；分页、搜索、状态筛选和默认排序回归通过。
- API schema 继续只返回可空 `capacity_bytes`，不增加原始身份或指标标签。

### 前端与 Chromium

- 单元测试锁定型号、容量为两个表头和两个独立单元格。
- 容量表头可切换升降序，并把 `sort=capacity`、正确 `order`、`page=1` 写入 URL/API 请求。
- 覆盖容量格式化、缺失文案、完整 `title` 和长型号省略。
- 硬盘表头数量从 9 更新为 10；1440×900 下十个表头和容量值可见，页面及 `.disk-table-scroll` 均无横向溢出。
- 保留无破坏性控件、登录后无非预期浏览器错误和只读写方法 405 验证。

### 全量回归

- Docker 前端 Vitest、typecheck、production build。
- Go 全仓普通测试、race 测试和二进制编译。
- 无缓存生产镜像构建。
- 只有取得用户单独授权后，才可原位重建仍连接测试 Nightingale 的现有 8080 并做脱敏 API/Chromium 验收；不得创建其他 InfraView 端口。

## 安全与非目标

- InfraView 始终严格只读。
- 不新增任意 PromQL、范围查询、Nightingale 代理、本机设备访问或运维写操作。
- 不新增 SMART 自检、扫描、修复、启停、擦除、删除或人工忽略入口。
- 不读取或输出私密环境文件、Token、Cookie、认证头、Base URL、真实标识/IP/数量/指标值或上游正文。
- 不连接、切换或探测生产 Nightingale。
- 不处理命令超时、失联移除、“全部”分页或其他模块需求。
- 未经分别明确授权，不提交、不推送、不重建或重启服务。
