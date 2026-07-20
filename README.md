# InfraView

InfraView 是一个轻量、只读的基础设施可视化平台，首期面向 Linux 服务器，通过统一 Web 页面展示基础设施总览、主机列表与主机详情。

平台计划优先接入 Nightingale（夜莺）监控数据，并通过稳定的数据源适配接口保留扩展其他监控系统的能力。

## 核心边界

- 只读展示基础设施状态与监控指标。
- 不提供修改、删除、重启等运维操作。
- 不执行 SSH、远程命令或自动化变更。
- 不保存监控历史数据，仅使用短期内存缓存。
- 使用固定账号密码登录，不提供用户管理。
- 通过 Docker Compose 部署，可直接使用 IP 和端口访问。

## 当前状态

项目处于 MVP 设计完成、等待正式规格审阅阶段，尚未开始应用代码实现。

- [项目状态](docs/PROJECT_STATUS.md)
- [开发 TODO](docs/TODO.md)
- [MVP 正式设计规格](docs/superpowers/specs/2026-07-20-infraview-mvp-design.md)

## 计划交付

1. 基于 Mock 数据源交付功能完整的 InfraView MVP。
2. 夜莺测试环境就绪后，实现并验证真实 Nightingale 适配器。
