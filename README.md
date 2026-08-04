# InfraView

InfraView 是一个轻量、只读的 Linux 基础设施可视化平台。当前工作区具备固定账号登录、Linux 主机、主机硬盘 SMART、MySQL、Redis 与 Elasticsearch 板块状态总览、紧凑清单、搜索/筛选/排序/分页、指标状态着色、旧缓存提示和统一错误展示；其中 Elasticsearch 已完成离线实现与验证，尚未重建到现有 8080，也未提交或推送。

生产形态是一个非 root InfraView 容器：Go 同源提供只读 API 与 React 静态页面，无业务数据库、任务队列、SSH 客户端或远程执行器。数据源支持确定性 Mock 和 Nightingale 只读适配器；MySQL 与硬盘 SMART 分别使用独立领域、Service 和缓存，复用 Nightingale 的受限安全客户端。当前主要开发与验证版本为 Nightingale v8.4.1，v9.x 仅保留已覆盖的协议兼容。浏览器只接收归一化 InfraView API，不接触 Token、任意 PromQL、硬盘序列号或 WWN。

Linux 主机与 MySQL 实例会按原始样本是否持续推进判断数据新鲜度：连续 2 个预期采集周期未推进显示“采集延迟”，连续 5 个周期未推进显示“采集失联”。MySQL 清单通过固定的 24 小时近期身份查询保留刚停止上报的实例，并以“QPS / TPS”展示查询吞吐与显式事务吞吐。

硬盘 SMART 使用独立默认 `60s` 周期，Nightingale Provider 只发送一次固定 18 查询即时 batch；第 17 组 `smart_disk_capacity_bytes` 是容量唯一来源，第 18 组 inventory 负责设备发现、型号和原始最后样本时间。freshness 记录的是 InfraView 本地最后观察到原始样本时间推进的时刻，不比较业务值变化，也不把原始样本绝对年龄当作停采。温度、寿命与错误计数只展示，不套用 Linux/MySQL 的通用阈值改变总体状态。

## 安全边界

- 只展示监控数据，不修改、删除、重启服务器、服务或配置。
- 不执行 SSH、远程命令、脚本、发布或自动化变更。
- 不执行 `smartctl`、`nvme-cli`、SMART 扫描/自检、修复、启停或擦除。
- 不提供任意上游 URL、任意查询表达式或通用代理。
- 除登录/退出 Web 会话外，业务 API 全部只读。
- MySQL 仅提供总览和实例清单；不提供历史、实例详情、写入或运维操作。
- 硬盘仅提供总览和设备清单；稳定 ID 使用包含主机身份的不可逆哈希，API 永不返回序列号、WWN、原始标签或数据源请求信息。
- 监控数据和会话只存在内存中，容器重启后清空。

## 快速部署

```bash
cp .env.example .env
# 必须编辑 .env，更换示例密码并确认端口、Cookie、时区与数据源配置
docker compose up -d --build
docker compose ps
```

默认可通过 `http://服务器IP:8080` 访问。`.env.example` 只能作为模板，禁止把示例凭据直接用于正式环境。HTTPS 反向代理部署必须同时启用 Secure Cookie、回环端口绑定和精确可信代理 CIDR，详见[部署说明](docs/DEPLOYMENT.md)。

## 验证

标准入口是：

```bash
make verify
```

它覆盖前端单测、类型检查、构建、依赖审计、静态资源复制、Go 格式/普通测试/race/build、镜像构建、Compose smoke、真实 Chromium E2E、缓存 P95 和资源检查。宿主没有 `make` 时参见 [测试说明](docs/TESTING.md) 的等价隔离命令。

## 文档导航

- [项目状态](docs/PROJECT_STATUS.md)
- [架构](docs/ARCHITECTURE.md)
- [产品与交互设计](docs/DESIGN.md)
- [配置](docs/CONFIGURATION.md)
- [部署、升级与回滚](docs/DEPLOYMENT.md)
- [开发](docs/DEVELOPMENT.md)
- [测试](docs/TESTING.md)
- [安全](docs/SECURITY.md)
- [Nightingale 接入与验证](docs/datasources/NIGHTINGALE.md)
- [TODO](docs/TODO.md)
- [架构决策记录](docs/decisions/0001-single-container-go-react.md)

## Redis Cluster 只读模块

当前工作区已实现独立 Redis 垂直模块：总览第四张 Redis 卡、侧边栏入口和十一列实例清单。数据仅来自 Nightingale 代码内置的 21 条固定查询，并合并为一次即时 batch；页面不接受任意 PromQL，不连接 Redis，也不提供切换、故障转移、命令执行或其他写操作。状态覆盖可用性、采集推进、内存、拒绝连接和主从复制；复制拓扑、详情和历史不在首期范围。

## Elasticsearch 只读模块

Elasticsearch 首期只提供总览第五张集群健康卡与 `/elasticsearch` 节点列表。数据来自 Nightingale 代码内置的 26 条固定查询，恰好一次即时 batch、无集群或节点 N+1；集群身份为 `cluster`，节点身份为 `cluster + name`，地址不参与稳定身份。HTTP 仅提供 `GET /api/v1/elasticsearch/overview` 与 `GET /api/v1/elasticsearch/nodes`。节点页复用共享列表模板，固定 16 个单值单行列；空间不足时只允许表格内部横向滚动。首期不含索引/节点详情、历史、拓扑、Elasticsearch 直连、任意查询或运维操作。
