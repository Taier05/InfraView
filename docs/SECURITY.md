# 安全说明

## 已实现控制

- 固定账号密码，服务端恒定时间比较；密码至少 12 字符且不写日志。
- 密码学随机内存会话；Cookie 为 HttpOnly、SameSite=Strict、限定路径，HTTPS 可启用 Secure。
- 登录按来源 IP 限速；默认不信任转发头，仅允许显式可信代理的单值合法 `X-Real-IP` 参与限速；请求 ID、统一错误和敏感字段日志测试。
- 默认同源，无 CORS 通用开放；设置 CSP、禁止 iframe、MIME 嗅探和不安全 referrer。
- 业务 API 只读；command/restart/delete/patch/proxy/query 路由测试为 404/405。
- 运行容器 UID/GID 10001、只读根文件系统、`/tmp` noexec/nosuid/nodev、capabilities 全删、禁止提权。
- 生产 InfraView 应用容器不挂 Docker Socket，不使用特权模式、宿主 PID/host network 或业务写卷。
- 静态路径拒绝 dotfile、路径穿越和缺失资源 SPA fallback；只有指纹资源可 immutable。
- Nightingale 客户端只拼接代码内置路径和 PromQL 白名单，默认只允许 HTTPS；HTTP 必须通过测试环境专用开关显式选择。客户端校验 HTTP 状态、JSON Content-Type、envelope `err` 和 8 MiB 响应上限；401、非 JSON 与上游正文统一转换为不含 Token 的领域错误。
- MySQL 复用上述 Nightingale 安全客户端，只允许代码固定的 16 条即时查询并合并为一次 batch；页面和 API 不接受 MySQL PromQL、上游 URL、查询体或认证信息。MySQL 仅有两个受认证 GET 路由，写方法由测试验证为 405。
- 硬盘 SMART 复用同一安全客户端，仅允许代码固定的 18 条即时查询组成一次 batch，无主机/设备 N+1；不提供范围查询、任意 PromQL、任意上游 URL、代理或原始请求体。硬盘仅有两个受认证 GET 路由，其他方法由测试拒绝。
- Elasticsearch 复用同一 Nightingale 安全客户端，仅允许代码固定的 26 条即时查询组成一次 batch，无集群/节点 N+1；不直连 Elasticsearch，不接受任意 PromQL、指标名、URL、代理或上游请求体。仅有两个受认证 GET 路由，显式 View 排除 `ident`、`instance`、原始标签、数据源信息和上游正文，其他方法为 405。
- 原始 WWN、序列号与标签只允许在 Nightingale Provider 内部归并；稳定设备 ID 包含主机身份并按 WWN、序列号、设备名回退后做不可逆哈希。领域输出、HTTP View、前端类型、日志和错误不得包含原始身份。
- 产品不包含 `smartctl`、`nvme-cli`、块设备访问、SMART 扫描/自检、修复、启停、擦除、SSH、命令执行或远程控制能力。温度、寿命和错误计数仅展示，不驱动通用阈值告警或自动操作。

## 信任边界与限制

- E2E 的短生命周期 Playwright 浏览器容器是生产网络规则的明确例外：它使用 host network 访问专用 `127.0.0.1:18080` 验收端口，但不挂 Docker Socket、不运行 InfraView 服务、不进入生产 Compose 文件，并在测试结束后删除。

- 固定账号意味着所有使用者共享身份，无用户审计、RBAC、MFA 或密码找回。
- 直接 HTTP 访问会明文传输凭据和会话；不可信网络必须使用 Nginx/Caddy HTTPS，并设置 Secure Cookie。
- `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=true` 会允许通过 HTTP 明文发送上游 Token，只能用于隔离测试环境；生产必须保持关闭并使用 HTTPS。
- 会话和限速只在单进程内存，不适合多副本共享状态。
- 可信代理 CIDR 必须只包含直接连接 InfraView 的代理；范围过宽会允许该网段伪造限速来源。
- `/healthz` 不证明数据源健康；需查看页面或数据源状态 API。
- Mock 数据不代表真实基础设施；部署时必须根据 `INFRAVIEW_DATA_SOURCE` 明确区分演示与真实数据。
- InfraView 的只读边界不替代上游最小权限；Nightingale 必须使用专用最小只读 Token，公开仓库不记录实际部署账号或凭据权限。
- 开发 8080 只能连接测试 Nightingale，不得切换、探测或连接生产 Nightingale；现场 API/浏览器验收、部署或服务重启都需要单独明确授权。

## 凭据处理

`.env` 保持 Git 忽略和最小文件权限。smoke/benchmark 使用按字节 JSON 转义，支持双引号、反斜杠和控制字符，不依赖 `jq`。测试和日志不得输出密码、Cookie、Token、认证头或上游密钥。

## 漏洞与依赖

锁定前端依赖版本，完整验证执行生产依赖和全量 `npm audit`。基础镜像版本固定；升级前应重新跑普通测试、race、镜像、E2E、延迟和资源验收。

## 事件响应

怀疑密码泄露时：更换 `.env` 凭据、重新创建容器使所有内存会话失效、检查反向代理与 InfraView 日志，并确认仓库历史没有 `.env`。InfraView 不存监控历史，因此事件证据主要位于外部日志系统与反向代理。

## Redis 模块边界

Redis 复用 Nightingale 安全客户端，只允许代码固定的 21 条即时查询组成一次 batch，无实例 N+1。API 仅有两个受认证 GET 路由；显式 view model 排除原始标签、PromQL、数据源信息和复制端身份，写方法为 405。运行时不连接 Redis，不包含 Redis 命令、主从切换、故障转移或执行能力。

## Elasticsearch 模块边界

Elasticsearch 首期只展示集群健康和节点清单，不含索引/节点详情、历史、拓扑或控制能力。稳定集群 ID 只使用 `cluster`，稳定节点 ID 只使用 `cluster + name`；地址仅展示，采集身份不得进入 API。产品不包含 Elasticsearch 客户端、写 API、`_cluster/reroute`、设置修改、索引开关、forcemerge、delete-by-query、命令执行或远程控制。本轮只完成离线验证，未访问真实 Nightingale/Elasticsearch，未重建或访问现有 8080。
