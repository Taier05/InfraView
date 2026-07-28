# InfraView

InfraView 是一个轻量、只读的 Linux 基础设施可视化平台。当前版本具备固定账号登录、板块状态总览、紧凑主机清单、搜索/筛选/排序/分页、指标阈值着色、旧缓存提示和统一错误展示。

生产形态是一个非 root InfraView 容器：Go 同源提供只读 API 与 React 静态页面，无业务数据库、任务队列、SSH 客户端或远程执行器。数据源支持确定性 Mock 和 Nightingale 只读适配器；当前主要开发与真实验证版本为 Nightingale v8.4.1，v9.x 保留已覆盖的协议兼容。浏览器只接收归一化 InfraView API，不接触 Token 或任意 PromQL。

## 安全边界

- 只展示监控数据，不修改、删除、重启服务器、服务或配置。
- 不执行 SSH、远程命令、脚本、发布或自动化变更。
- 不提供任意上游 URL、任意查询表达式或通用代理。
- 除登录/退出 Web 会话外，业务 API 全部只读。
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
