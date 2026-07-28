# InfraView Nightingale 审查修复设计规格

最后更新：2026-07-27

## 目标

修复 Nightingale 第二阶段静态审查发现的安全边界、上游协议校验、主机可见性和配置隔离问题，并补齐能阻止同类回归的测试。本轮不扩展产品功能，不改变 InfraView 只读边界，不提交、推送、合并或重启当前 8080 服务。

## HTTP 重定向安全

- Provider 必须复制注入的 `http.Client`，不得修改调用者或全局客户端。
- 复制后的客户端必须拒绝所有 HTTP 重定向，使原始 3xx 响应进入现有非 200 错误路径并映射为 `datasource.ErrUnavailable`。
- 必须保留注入客户端已有的 Transport、Jar、Timeout 和其他设置；Provider 只收紧重定向策略。
- 回归测试使用两个 `httptest.Server`：第一个返回跨主机重定向，第二个记录是否收到请求。测试必须确认第二个服务器未收到请求和 `X-User-Token`。

## Nightingale 响应协议严格性

- 通用 envelope 必须拒绝缺失 `dat` 和显式 `dat:null`，继续接受 `err:null` 或空字符串。
- Target 分页响应必须明确包含可解码的 `list` 和 `total`；缺失字段、负数、分页间变化、达到总数前的空页、超出总数、空 ident 和重复 ident 均返回 `datasource.ErrUnavailable`。
- `/query-instant-batch` 和 `/query-range-batch` 的外层结果数量必须与发送的查询数量完全相等。上游无序列必须在相应查询索引返回空数组，不能省略索引或附加结果。
- 协议形状错误必须作为失败返回，不能以空资产、空指标或零主机成功进入缓存，以便服务层保留 stale 回退能力。

## 历史指标主机可见性

- `service.Metrics` 的缓存加载路径在执行任何范围查询前，必须通过 Provider 精确验证一次主机是否存在。
- 未知主机映射为现有 `service.ErrNotFound`，HTTP 返回现有 404 契约，并且范围查询调用次数为零。
- 主机验证只能执行一次，不能在八个 MetricKey 的每次 `QueryRange` 调用中重复，避免引入按指标重复的资产查询。
- 保留现有范围结果缓存和 stale 语义；缓存命中不额外增加上游请求。

## 配置与构造边界

- 只有 `INFRAVIEW_DATA_SOURCE=nightingale` 时才校验 Nightingale Base URL、Token 和接口排除正则。Mock 模式忽略未使用的 Nightingale 专属变量。
- `nightingale.New` 自身必须执行与配置层一致的 Base URL 安全约束：只接受绝对 HTTP(S) URL，拒绝 userinfo、query 和 fragment，不能静默清理非法输入。
- Token 错误和所有上游错误继续保持脱敏，不包含 Token、认证头、上游正文、原始 PromQL 或请求 ID。

## 数值边界

- 内存总量、运行时间和 Unix 时间戳在转换为整数或 `time.Duration` 前必须验证可表示范围。
- 极端有限值、NaN 和正负无穷均按缺失数据处理，不允许溢出为负数或异常时间。
- 正常整数、带小数的合法 Unix 秒和现有真实数据映射保持兼容。

## 测试策略

所有生产代码变更必须遵循 RED → GREEN：

1. 先加入重定向安全失败测试并确认现有代码确实访问第二个服务器。
2. 加入 `dat:null`、缺失分页字段、非法分页、批量数量不匹配测试并确认失败。
3. 加入未知历史主机 404 且零范围查询测试并确认失败。
4. 加入 Mock 模式隔离、Provider Base URL 和数值边界测试并确认失败。
5. 加入数据源并发发现、失败后重试和完整 PromQL 白名单测试；如果某项现有实现已正确，测试可以直接通过，但必须记录它是补充覆盖而非修复证明。
6. 每组最小实现完成后运行对应 Docker Go 测试；最后运行仓库 Dockerfile 覆盖的普通测试、race 测试、前端测试、typecheck、production build 和生产镜像构建。

测试断言必须观察真实行为，不依赖源码文本；预期 PromQL 使用手工推导的固定字符串。夹具继续只使用保留文档地址和虚构 ident，不记录任何真实 Token 或上游响应正文。

## 文档与交付边界

- 修复完成后更新 `docs/TESTING.md`、`docs/PROJECT_STATUS.md`、`docs/TODO.md` 和 `docs/HANDOFF.md`，记录修复与最新验证证据。
- 本轮不读取或输出 `/secure/path/infraview.env` 内容，不重复 SSH、版本探测、Token 实证或真实 Nightingale 联调。
- 未经后续明确授权，不提交、推送、合并，不重建或重启当前 8080 服务。
