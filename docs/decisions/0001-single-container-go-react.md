# ADR 0001：Go 同源托管 React 的单容器架构

日期：2026-07-20

状态：已采纳

## 背景

InfraView 需要轻量 Docker Compose 部署、固定账号登录、直接 IP:端口访问，并保持严格只读。首版不需要业务数据库或前端独立扩缩容。

## 决策

使用 React/TypeScript/Vite 构建前端，由 Go 在构建时嵌入静态产物，并由同一进程提供 SPA、认证和只读 API。Compose 恰好运行一个 InfraView 服务；Nginx/Caddy 仅作为可选外部 HTTPS 反向代理。

## 理由

- 单一来源避免 CORS、双服务发现和额外 Cookie 配置。
- 最终镜像无需 Node，资源和攻击面较小。
- Go 标准库适合认证、缓存、超时和静态托管。
- React 适合总览、表格、筛选和图表交互。
- 数据源接口隔离页面与 Nightingale 版本差异。

## 后果

- 每次前端更新都要重建 Go 二进制/镜像。
- 多副本会话不共享；MVP 明确为单容器。
- 静态资源需正确区分 index no-cache 与指纹 immutable。
- 图表模块采用动态加载控制首屏主包。
- 若未来确需多副本或独立 CDN，应新建 ADR，而不是暗中引入数据库或状态服务。
