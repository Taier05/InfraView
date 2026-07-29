# MySQL 紧凑布局验证记录

日期：2026-07-29
范围：`feature/mysql-module` 的 MySQL 紧凑布局验收

## 安全与部署边界

- InfraView 保持只读；未连接生产、未执行远程命令、未修改上游或运行配置。
- 仅强制重建既有 `infraview` Compose 项目的原 8080 开发服务；它只连接测试 Nightingale。
- 未启动 `scripts/e2e.sh`，未创建 18080、其他 InfraView 端口、预览服务或其他 Compose 项目。
- 本记录不包含私密环境文件内容、凭据、Cookie、认证头、真实地址、标识、资源数量、指标值或响应正文。

## 实施者验收结果

- `./scripts/e2e-safety.test.sh` 通过。
- 原 8080 的既有 `infraview` 服务以强制重建方式重新创建成功。
- 服务达到 `healthy`；无正文 HTTP 检查通过：根页面状态为 200，未认证 MySQL 概览请求状态为 401。
- 原 8080 的 Chromium live 验收通过（2/2）：1440×900 下总览为四列紧凑模块位，Linux/MySQL 模块等宽；MySQL 11 列及全部表头可见，页面和表格均无横向滚动。

## 完整构建补跑结果

控制器在提交 `0f36501` 的 HEAD 上随后完整运行 `docker build --no-cache --progress=plain .`，退出码为 0。该构建覆盖前端全量 Vitest、typecheck、production build、Go 普通测试、Go race、Linux build 及镜像导出，均通过。

## 结论

MySQL 紧凑布局的 1440×900 回归证据已保存在受 Git 跟踪的文档中。开发 8080 始终使用测试 Nightingale；本次没有额外端口、预览服务或生产连接。
