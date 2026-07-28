# InfraView Nightingale v8.4.1 兼容设计规格

最后更新：2026-07-28

## 目标

将 Nightingale v8.4.1 设为 InfraView 后续开发和真实验证的主要版本，同时保留现有 v9 系列协议兼容能力。生产接入目标是只更换私密连接配置并执行最终只读冒烟，不再修改业务代码。

本阶段不改变 InfraView 的只读产品边界，不增加任意 PromQL、任意代理、SSH、远程命令、配置下发或其他写操作。

## 已验证的 v8.4.1 契约

测试环境已经完成以下只读验证，公开仓库只记录结构结论：

- `GET /api/n9e/versions` 返回可识别的 `v8.4.1` 版本。
- `GET /api/n9e/self/profile` 接受 `X-User-Token`，返回 HTTP 200、JSON 和空 `err`。
- `GET /api/n9e/targets?limit=100&p=1` 返回 `dat.list` 数组和数值 `dat.total`。
- Target 包含 `ident`、`host_ip`、`os`、`target_up`、`cpu_num` 和 `update_at`，不包含 v9 环境已验证的 `beat_time`。
- `GET /api/n9e/datasource/brief` 返回带正整数 `id` 和 `plugin_type=prometheus` 的候选数据源。
- `POST /api/n9e/query-instant-batch` 接受当前即时批量请求结构，并按查询顺序返回 vector 数组。
- `POST /api/n9e/query-range-batch` 接受当前范围批量请求结构，并按查询顺序返回 matrix 数组。

真实 Base URL、Token、主机标识、IP、资源数量、数据源 ID、指标值和响应正文不得进入源码、文档、测试夹具、错误或日志。

## 方案

采用能力兼容而非运行时版本分支：

1. Provider 继续调用已验证的五个核心只读 API，不增加版本探测请求。
2. Target 同时解析 `beat_time` 和 `update_at`。
3. `StatusTime` 优先使用有效的 `beat_time`；`beat_time` 缺失或非正数时回退到有效的 `update_at`。
4. 两个字段都缺失、非正数或越界时保持零时间，不伪造当前时间。
5. 其他资产、当前指标、历史指标、数据源发现和错误边界保持现有实现。

该方案让 v8.4.1 成为主要验证版本，同时避免无必要地破坏 v9 兼容性。由于版本接口在不同版本间存在路径和响应格式差异，运行时不依赖 `/version` 或 `/versions` 决定解析逻辑。

## 数据模型

`targetRecord` 新增：

```go
UpdateAt int64 `json:"update_at"`
```

状态时间选择规则：

```text
beat_time > 0 且为有效 Unix 秒  -> 使用 beat_time
否则 update_at > 0 且为有效 Unix 秒 -> 使用 update_at
否则                                -> 零时间
```

`target_up` 继续使用 `2=online`、`1=unknown`、`0=offline`；未知值映射为 unknown。

## 测试设计

严格执行 TDD：

1. 先加入一个完全脱敏的 v8.4.1 Target 分页夹具，仅包含 `update_at`，不包含 `beat_time`。
2. 先写失败测试，断言 v8.4.1 Target 的 `StatusTime` 来自 `update_at`。
3. 再写优先级测试，断言两个字段同时存在时仍优先使用 `beat_time`。
4. 覆盖 `update_at` 非正数和越界值，断言保持零时间。
5. 运行聚焦 Nightingale 包测试确认 RED，再做最小实现并确认 GREEN。
6. 运行 Go 全仓普通/race 测试、前端 Vitest、类型检查、生产构建和隔离 Chromium E2E。

测试夹具使用 `fixture-node-*`、RFC 5737 文档地址和固定虚构时间，不复制真实响应正文。

## 真实环境验收

代码和容器验证全部通过后，才允许使用被 Git 忽略且权限为 600 的私密环境文件重建当前 InfraView Compose 服务。

真实 v8.4.1 验收只执行：

- InfraView 登录。
- 数据源状态和健康检查。
- 总览与主机清单。
- 当前指标和支持的历史指标。
- 浏览器页面只读检查。

验收输出不得包含 Cookie、Token、认证头、上游正文、真实地址、主机标识、资源数量或指标值。验收失败时保留旧缓存语义和安全错误，不修改 Nightingale。

## 文档与支持范围

完成后更新：

- `README.md`
- `docs/HANDOFF.md`
- `docs/PROJECT_STATUS.md`
- `docs/TODO.md`
- `docs/datasources/NIGHTINGALE.md`
- 相关配置、部署与测试文档

支持矩阵表述：

- Nightingale v8.4.1：主要开发与真实验证版本。
- Nightingale v9.x：保留协议兼容实现，但不再作为当前真实验证环境。
- 其他版本：没有对应契约证据前不声明支持。

## 验收标准

- v8.4.1 `update_at` 正确映射到 `StatusTime`。
- v9 `beat_time` 行为不回归。
- 五个核心 API 契约测试和真实只读冒烟通过。
- 当前指标仍为单次批量查询，不出现按主机 N+1。
- 固定 PromQL、HTTPS 默认、显式 HTTP 测试开关、超时、响应大小和错误脱敏边界保持不变。
- 公开仓库不包含任何真实部署信息或凭据。
