# 开发说明

## 运行环境

- Go 1.24 与 Node 22 通过 Docker 镜像运行，宿主无需安装 Go。
- 前端锁文件必须使用 `npm ci`。
- Playwright 使用 `mcr.microsoft.com/playwright:v1.61.1-noble`，避免宿主缺少 Chromium 动态库。
- 不安装宿主软件，不使用 Docker Socket 挂载到应用或浏览器容器。

## 常用命令

```bash
make web-test
make web-typecheck
make web-build
make go-test
make go-test-race
make go-build
cd web && npm run e2e
```

宿主没有 `make` 时，可按 `Makefile` 展开的 Docker 命令执行；完整等价顺序见 [测试说明](TESTING.md)。

## 前端开发

如本地已有 Node 22：

```bash
cd web
npm ci
npm run dev
```

Vite 将 `/api` 和 `/healthz` 代理到 `127.0.0.1:8080`，需要另行启动 Go 服务。生产验收应使用 Compose 同源形态，而不是仅依赖 Vite 开发服务器。

## 修改规则

- 功能和修复先增加失败测试，再实现最小改动，最后跑相关与全量验证。
- 新 API 默认不得增加写操作；新增数据源必须实现稳定 `datasource.Provider` 契约。
- 不直接编辑 `internal/httpapi/webdist`，它是忽略的构建产物，由 `make web-copy` 重建。
- 环境变量变化必须同步 `.env.example` 与 `docs/CONFIGURATION.md`。
- 部署、架构或安全边界变化必须同步对应文档、`PROJECT_STATUS.md` 和 `TODO.md`。
- `.superpowers/sdd` 是本地过程账本，保持 Git 未跟踪。

## Nightingale 开发入口

在测试环境版本、认证和真实只读响应证据齐全前，不修改 `internal/adapters/nightingale` 去猜 API。开始集成前先更新 [Nightingale 文档](datasources/NIGHTINGALE.md)、脱敏夹具和独立实施计划。
