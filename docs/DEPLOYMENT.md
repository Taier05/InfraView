# Docker Compose 部署

## 前置条件

- Linux 主机可运行 Docker Engine 与 Docker Compose v2。
- 预留一个局域网可访问端口，默认 8080。
- 使用 Nightingale 时需准备可访问的 API Base URL 和个人/专用 Token；当前主要开发与真实验证版本为 v8.4.1，v9.x 保留协议兼容，其他版本须先取得契约证据。InfraView 只调用已验证的只读接口。

## 首次部署与直接访问

```bash
cp .env.example .env
chmod 600 .env
# 编辑 .env：至少更换密码，确认端口、时区、Cookie 和数据源设置
docker compose config
docker compose up -d --build
docker compose ps
```

Nightingale 模式至少设置：

```dotenv
INFRAVIEW_DATA_SOURCE=nightingale
INFRAVIEW_NIGHTINGALE_BASE_URL=https://n9e.example.com
INFRAVIEW_NIGHTINGALE_TOKEN=仅写入私密环境文件的实际值
INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=false
```

不要把包含 Token 的 `.env` 复制进 worktree。若使用工作树 Compose，应显式执行 `INFRAVIEW_ENV_FILE=/受限路径/.env docker compose up -d --build`。

Nightingale Base URL 默认只允许 HTTPS。仅当受控测试环境确实无法提供 TLS 时，才可同时使用 `http://...` 和 `INFRAVIEW_NIGHTINGALE_ALLOW_INSECURE_HTTP=true`；该开关会允许明文发送 Token，禁止用于生产或不可信网络。

Nightingale v8.4.1 可通过 `/api/n9e/versions` 获取版本信息，但 InfraView 运行时不探测版本，也不依赖任何版本接口；适配器按已验证的响应字段能力兼容 v8.4.1 与 v9.x。

在同一局域网访问 `http://服务器IP:8080`。如设置 `INFRAVIEW_PORT=18080`，则访问 `http://服务器IP:18080`。这种纯端口写法会监听全部宿主接口，必须使用主机防火墙和网段访问控制限制来源。

健康检查：

```bash
curl --fail http://127.0.0.1:8080/healthz
docker compose ps
docker compose logs --tail=100 infraview
```

`/healthz` 只验证 InfraView 进程；数据源状态登录后由 `/api/v1/datasource/status` 展示。

## 可选 Nginx HTTPS

先在 `.env` 设置 `INFRAVIEW_PORT=127.0.0.1:8080`、`INFRAVIEW_COOKIE_SECURE=true` 和 `INFRAVIEW_TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128`，使 InfraView 只发布到宿主回环地址、信任本机代理写入的单值 `X-Real-IP`，不再通过局域网 IP 直接提供明文 HTTP。随后关闭之前为 InfraView 开放的防火墙直连端口，再配置代理：

```nginx
server {
    listen 443 ssl http2;
    server_name infra.example.com;

    ssl_certificate     /etc/nginx/tls/fullchain.pem;
    ssl_certificate_key /etc/nginx/tls/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

可另建 80 端口 server 强制跳转 HTTPS。InfraView 不要求 WebSocket。

## 可选 Caddy HTTPS

```caddyfile
infra.example.com {
    reverse_proxy 127.0.0.1:8080 {
        header_up X-Real-IP {remote_host}
    }
}
```

同样设置 `INFRAVIEW_PORT=127.0.0.1:8080`、`INFRAVIEW_COOKIE_SECURE=true` 与 `INFRAVIEW_TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128`，并关闭防火墙中的直接 HTTP 暴露。`header_up` 必须覆盖写入单值客户端 IP；不能只依赖默认 `X-Forwarded-For`。若代理通过容器网络连接 InfraView，应改填该代理网络的精确 CIDR。证书、DNS 和公网访问策略由现有 Caddy/网络策略负责。

## 升级

1. 保存当前 Git 提交或镜像标签，阅读 `docs/PROJECT_STATUS.md`。
2. 备份受限权限的 `.env`；InfraView 本身无业务数据卷。
3. 获取目标版本后执行 `docker compose build --pull`。
4. 执行 `docker compose up -d`，检查 healthy、登录、只读页面和日志。
5. 会话与缓存会因容器替换清空，这是预期行为。

## 回滚

切回已记录的上一版本提交或镜像标签，使用原 `.env` 重新构建/启动，再执行健康与 smoke 检查。因为没有 schema、数据库或业务卷，应用回滚不需要数据迁移；但真实监控数据始终应由上游系统保证。

## 停止与清理

```bash
docker compose down
```

该命令只应在 InfraView 仓库目录执行，或显式指定本项目名。不要对其他 Compose 项目执行全局清理。InfraView 没有业务数据卷；`down` 会移除容器和项目网络，内存会话/缓存不可恢复。

## 资源检查

```bash
container_id=$(docker compose ps -q infraview)
docker stats --no-stream --format '{{.Name}} {{.MemUsage}}' "$container_id"
docker image inspect --format '{{.Size}}' "$(docker inspect --format '{{.Image}}' "$container_id")"
```

Mock MVP 实测空闲内存约 10.25 MiB，目标低于 256 MiB；镜像约 16.63 MB。不同 Docker、内核和架构会有小幅差异。

## 备份含义

无需备份 InfraView 业务数据库或监控历史，因为二者不存在。需要保护的是部署配置 `.env`、反向代理证书/配置和可重建的源码/镜像版本。InfraView 不能替代 Nightingale 或其他上游监控系统的备份。
