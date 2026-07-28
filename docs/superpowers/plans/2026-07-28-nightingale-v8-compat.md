# Nightingale v8.4.1 兼容实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Nightingale v8.4.1 设为 InfraView 的主要开发和真实验证版本，并用 `update_at` 回退保持 Target 状态时间兼容，同时保留 v9 `beat_time` 行为。

**Architecture:** Provider 按字段能力解析 Target，不增加运行时版本探测或新上游请求。`StatusTime` 依次选择有效的 `beat_time`、有效的 `update_at`，其余 API、固定 PromQL、缓存和错误边界保持不变。

**Tech Stack:** Go 1.24、`net/http/httptest`、JSON 契约夹具、Docker、Docker Compose、Vitest、Playwright。

## Global Constraints

- InfraView 只展示数据，不执行任何运维操作。
- 不增加任意 PromQL、任意代理、SSH、远程命令或写接口。
- 不输出或提交真实 Base URL、Token、认证头、Cookie、主机标识、IP、资源数量、指标值或上游响应正文。
- Nightingale v8.4.1 是主要开发与真实验证版本；v9.x 保留协议兼容，但不再作为当前真实验证环境。
- 所有 Go 测试、格式化、构建和运行优先通过容器完成，不在宿主机安装依赖。
- 生产代码必须遵循 RED→GREEN TDD；文档不编写只检查文本的伪测试。

---

### Task 1: Target 状态时间双字段兼容

**Files:**
- Create: `internal/adapters/nightingale/testdata/targets-v8-page-1.json`
- Modify: `internal/adapters/nightingale/provider_test.go`
- Modify: `internal/adapters/nightingale/client.go`
- Modify: `internal/adapters/nightingale/provider.go`

**Interfaces:**
- Consumes: `targetRecord`、`mapTargetRecords([]targetRecord)`、`jsonUnixTime(int64, int64)`。
- Produces: `targetRecord.UpdateAt int64` 和按 `beat_time`、`update_at` 顺序选择的 `datasource.Host.StatusTime`。

- [x] **Step 1: 添加完全脱敏的 v8.4.1 Target 夹具**

创建 `internal/adapters/nightingale/testdata/targets-v8-page-1.json`：

```json
{
  "dat": {
    "list": [
      {
        "ident": "fixture-node-01",
        "host_ip": "192.0.2.21",
        "os": "linux",
        "agent_version": "v0.0.0-fixture",
        "update_at": 1785122700,
        "target_up": 2,
        "cpu_num": 4
      }
    ],
    "total": 1
  },
  "err": "",
  "request_id": "fixture-targets-v8-page-1"
}
```

- [x] **Step 2: 写失败测试**

在 `provider_test.go` 增加：

```go
func TestListHostsUsesV8UpdateAtWhenBeatTimeMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		switch request.URL.Path {
		case "/api/n9e/targets":
			writeFixture(t, w, "targets-v8-page-1.json")
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			writeEnvelope(t, w, [][]any{
				{map[string]any{"metric": map[string]string{"ident": "fixture-node-01"}, "value": []any{"1785122700", "8589934592"}}},
				{map[string]any{"metric": map[string]string{"ident": "fixture-node-01"}, "value": []any{"1785122700", "3600"}}},
			})
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{
		BaseURL: server.URL, AllowInsecureHTTP: true,
		Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock,
	})
	hosts, err := provider.ListHosts(context.Background())
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	if len(hosts) != 1 || !hosts[0].StatusTime.Equal(time.Unix(1785122700, 0).UTC()) {
		t.Fatalf("hosts = %#v, want v8 update_at status time", hosts)
	}
}
```

再增加表驱动测试，通过 JSON 解码构造记录：

```go
func TestMapTargetRecordsSelectsCompatibleStatusTime(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{
			name: "beat_time wins when both are valid",
			raw:  `[{"ident":"fixture-node-01","beat_time":1785123000,"update_at":1785122700}]`,
			want: time.Unix(1785123000, 0).UTC(),
		},
		{
			name: "valid update_at replaces invalid beat_time",
			raw:  `[{"ident":"fixture-node-01","beat_time":253402300800,"update_at":1785122700}]`,
			want: time.Unix(1785122700, 0).UTC(),
		},
		{
			name: "invalid update_at stays missing",
			raw:  `[{"ident":"fixture-node-01","update_at":253402300800}]`,
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var records []targetRecord
			if err := json.Unmarshal([]byte(tt.raw), &records); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			hosts, _, err := mapTargetRecords(records)
			if err != nil {
				t.Fatalf("mapTargetRecords() error = %v", err)
			}
			if len(hosts) != 1 || !hosts[0].StatusTime.Equal(tt.want) {
				t.Fatalf("hosts = %#v, want status time %s", hosts, tt.want)
			}
		})
	}
}
```

- [x] **Step 3: 运行聚焦测试确认 RED**

```bash
docker run --rm \
  -e GOCACHE=/tmp/go-cache \
  -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src \
  golang:1.24-bookworm \
  go test ./internal/adapters/nightingale \
    -run 'Test(ListHostsUsesV8UpdateAtWhenBeatTimeMissing|MapTargetRecordsSelectsCompatibleStatusTime)$' \
    -count=1
```

期望：`update_at` 被现有结构忽略，至少一个用例因 `StatusTime` 为零而失败。

- [x] **Step 4: 实现最小兼容逻辑**

在 `client.go` 的 `targetRecord` 增加：

```go
UpdateAt int64 `json:"update_at"`
```

在 `provider.go` 增加：

```go
func targetStatusTime(record targetRecord) time.Time {
	for _, candidate := range []int64{record.BeatTime, record.UpdateAt} {
		if candidate <= 0 {
			continue
		}
		if statusTime, ok := jsonUnixTime(candidate, 0); ok {
			return statusTime
		}
	}
	return time.Time{}
}
```

将 `mapTargetRecords` 中只处理 `BeatTime` 的分支替换为：

```go
host.StatusTime = targetStatusTime(record)
```

- [x] **Step 5: 容器化格式化并确认 GREEN**

```bash
docker run --rm \
  -v "$PWD:/src" -w /src \
  golang:1.24-bookworm \
  gofmt -w internal/adapters/nightingale/client.go \
    internal/adapters/nightingale/provider.go \
    internal/adapters/nightingale/provider_test.go

docker run --rm \
  -e GOCACHE=/tmp/go-cache \
  -e GOMODCACHE=/tmp/go-mod \
  -v "$PWD:/src" -w /src \
  golang:1.24-bookworm \
  go test ./internal/adapters/nightingale -count=1
```

期望：Nightingale 包全部通过。

- [x] **Step 6: 提交 Target 兼容实现**

```bash
git add internal/adapters/nightingale
git diff --cached --check
git commit -m "feat: 兼容 Nightingale v8.4.1 Target 时间"
```

---

### Task 2: 将 v8.4.1 设为主要支持版本

**Files:**
- Modify: `README.md`
- Modify: `docs/DEPLOYMENT.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/TODO.md`
- Modify: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Consumes: Task 1 的双字段能力兼容和真实 v8.4.1 只读预检证据。
- Produces: 与代码一致的支持矩阵、恢复入口和验收说明。

- [ ] **Step 1: 更新支持矩阵和 Target 契约**

统一记录：

- v8.4.1 为主要开发与真实验证版本。
- v9.x 保留协议兼容，但不再作为当前真实验证环境。
- v8.4.1 使用 `/api/n9e/versions` 返回版本；运行时不依赖版本接口。
- Target 状态时间优先使用 `beat_time`，缺失或无效时回退 `update_at`。
- 其他版本没有契约证据前不声明支持。

保留原 v9 阶段规格和计划作为历史记录，不回写其既有目标。

- [ ] **Step 2: 更新项目状态和交接**

在 `PROJECT_STATUS.md`、`TODO.md`、`HANDOFF.md` 记录：

- 新分支 `feature/nightingale-v8-compat` 和当前工作树角色。
- v8.4.1 契约预检已通过。
- 当前 8080 容器在重建前仍使用旧 Token，重建后必须重新验证。
- 不记录真实地址、Token、主机数量、标识或指标值。

- [ ] **Step 3: 文档自审**

```bash
git diff --check

if git diff -- . \
  | awk '/^\+[^+]/ {sub(/^\+/, ""); print}' \
  | rg -q '192\.168\.|(/home|/root)/[^[:space:]]+/InfraView|X-User-Token:[[:space:]]+[^<`]|\b[0-9a-f]{64}\b'; then
  echo '新增文档敏感模式检查失败'
  exit 1
fi
```

期望：空白检查和新增行敏感模式检查通过。

- [ ] **Step 4: 提交支持矩阵文档**

```bash
git add README.md docs
git diff --cached --check
git commit -m "docs: 将 Nightingale v8.4.1 设为主要验证版本"
```

---

### Task 3: 完整验证、重建与真实 v8.4.1 验收

**Files:**
- Modify after verification: `docs/HANDOFF.md`
- Modify after verification: `docs/PROJECT_STATUS.md`
- Modify after verification: `docs/datasources/NIGHTINGALE.md`

**Interfaces:**
- Consumes: Task 1 的兼容实现、Task 2 的支持矩阵和私密环境文件。
- Produces: 可复现的容器验证证据、真实 v8.4.1 只读验收结论和用户预览。

- [ ] **Step 1: 构建完整验证镜像**

```bash
docker build --tag infraview:nightingale-v8-compat-verify .
```

期望：前端 45/45、类型检查、生产构建、Go 普通测试、race 测试和 Go 构建全部通过。

- [ ] **Step 2: 运行隔离 Mock E2E**

```bash
INFRAVIEW_E2E_PROJECT=infraview-v8-compat-0728 \
INFRAVIEW_E2E_PORT=18084 \
./scripts/e2e.sh
```

期望：独立 Compose smoke 和 Chromium 4/4 通过，脚本只清理自己创建的项目资源。

- [ ] **Step 3: 重建当前 InfraView 服务**

先只输出私密环境文件权限和 Git 忽略状态，不输出内容：

```bash
private_env_file=${INFRAVIEW_ENV_FILE:?必须显式提供私密环境文件}
stat -c '%a' "$private_env_file"
common_dir=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)
main_checkout=$(dirname "$common_dir")
git -C "$main_checkout" check-ignore -q "$private_env_file"
```

确认权限为 `600` 后重建：

```bash
INFRAVIEW_ENV_FILE="$private_env_file" \
  docker compose -p infraview up -d --build
```

- [ ] **Step 4: 执行安全真实 API 冒烟**

从私密环境文件读取 InfraView 登录凭据到 shell 变量；登录响应只提取 Cookie，不输出。执行：

```bash
set -euo pipefail
private_env_file=${INFRAVIEW_ENV_FILE:?必须显式提供私密环境文件}
username=$(sed -n 's/^INFRAVIEW_USERNAME=//p' "$private_env_file" | tail -n 1)
password=$(sed -n 's/^INFRAVIEW_PASSWORD=//p' "$private_env_file" | tail -n 1)
base_url='http://127.0.0.1:8080'

health=$(curl --silent --show-error --fail "$base_url/healthz")
printf '%s' "$health" | jq -e '.status=="ok"' >/dev/null

login_payload=$(jq -cn \
  --arg username "$username" \
  --arg password "$password" \
  '{username:$username,password:$password}')
login_headers=$(printf '%s' "$login_payload" |
  curl --silent --show-error \
    --dump-header - --output /dev/null \
    --header 'Content-Type: application/json' \
    --data-binary @- \
    "$base_url/api/v1/session")
login_status=$(printf '%s' "$login_headers" |
  awk 'toupper($1) ~ /^HTTP\// {code=$2} END {print code}')
cookie=$(printf '%s' "$login_headers" |
  sed -n 's/^[Ss]et-[Cc]ookie: \([^;]*\).*/\1/p' |
  tr -d '\r' |
  tail -n 1)
[ "$login_status" = 204 ] && [ -n "$cookie" ]

status_body=$(curl --silent --show-error --fail \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/datasource/status")
printf '%s' "$status_body" |
  jq -e '.data.type=="nightingale" and
    .data.healthy==true and
    .meta.stale==false' >/dev/null

overview_body=$(curl --silent --show-error --fail \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/overview?range=24h")
printf '%s' "$overview_body" |
  jq -e '(.data.total|type=="number") and .meta.stale==false' >/dev/null

hosts_body=$(curl --silent --show-error --fail \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/hosts?page=1&page_size=20")
printf '%s' "$hosts_body" |
  jq -e '(.data.hosts|type=="array") and
    (.data.hosts|length)>0 and
    .meta.stale==false' >/dev/null
host_id=$(printf '%s' "$hosts_body" | jq -r '.data.hosts[0].id')
encoded_host_id=$(jq -rn --arg value "$host_id" '$value|@uri')

host_body=$(curl --silent --show-error --fail \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/hosts/$encoded_host_id")
printf '%s' "$host_body" |
  jq -e '.data.id|type=="string"' >/dev/null

metrics_body=$(curl --silent --show-error --fail \
  --header "Cookie: $cookie" \
  "$base_url/api/v1/hosts/$encoded_host_id/metrics?range=1h")
printf '%s' "$metrics_body" |
  jq -e '(.data.series|type=="array") and .meta.stale==false' >/dev/null

echo '真实 v8.4.1 InfraView 只读冒烟：通过'
```

任何失败只由 `curl --fail` 或 `jq -e` 返回非零状态；脚本不输出 Cookie、主机 ID、数量、值或响应正文。

- [ ] **Step 5: 验证容器与只读边界**

```bash
INFRAVIEW_ENV_FILE="$private_env_file" \
  docker compose -p infraview ps

docker inspect \
  --format '{{.State.Health.Status}} {{.Config.User}} {{.HostConfig.ReadonlyRootfs}} {{json .HostConfig.CapDrop}}' \
  infraview-infraview-1
```

期望：容器 `healthy`、用户 `10001:10001`、只读根文件系统、能力全部删除。

- [ ] **Step 6: 记录真实验证结果**

只记录：

- v8.4.1 版本契约、认证、Target、数据源、即时/范围查询通过。
- InfraView 数据源、总览、主机和支持指标只读冒烟通过。
- 真实资源数量、标识、地址、值和响应正文未进入仓库。

然后执行：

```bash
git diff --check
git add docs/HANDOFF.md docs/PROJECT_STATUS.md docs/datasources/NIGHTINGALE.md
git diff --cached --check
git commit -m "docs: 记录 Nightingale v8.4.1 真实验证"
```

- [ ] **Step 7: 最终分支核验**

```bash
git status --short
git log --oneline main..HEAD
git diff --check main...HEAD
```

期望：工作区干净，分支只包含设计、兼容实现、支持矩阵和真实验证记录。
