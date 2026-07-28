#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d)
call_log="$test_dir/docker-calls.log"
output_log="$test_dir/e2e-output.log"
trap 'rm -rf "$test_dir"' EXIT HUP INT TERM

mkdir -p "$test_dir/bin"
printf '%s\n' \
	'#!/bin/sh' \
	'printf "%s\n" "$*" >>"$DOCKER_CALL_LOG"' \
	'case "${DOCKER_SCENARIO:-existing}:$*" in' \
	'  "existing:compose ls --all --format json")' \
	'    printf "%s\n" '\''[{"Name":"infraview-existing","Status":"running(1)"}]'\''' \
	'    exit 0' \
	'    ;;' \
	'  "empty:compose ls --all --format json")' \
	'    printf "%s\n" '\''[]'\''' \
	'    exit 0' \
	'    ;;' \
	'  "resource:compose ls --all --format json")' \
	'    printf "%s\n" '\''[]'\''' \
	'    exit 0' \
	'    ;;' \
	'  resource:ps\ -a\ --filter\ label=com.docker.compose.project=infraview-orphan\ --quiet)' \
	'    printf "%s\n" orphan-container-id' \
	'    exit 0' \
	'    ;;' \
	'  full:*" up "*) exit 0 ;;' \
	'  *" up "*) exit 71 ;;' \
	'  *) exit 0 ;;' \
	'esac' >"$test_dir/bin/docker"
chmod +x "$test_dir/bin/docker"

printf '%s\n' \
	'#!/bin/sh' \
	'case "$*" in' \
	'  *"--request "*) printf "%s" 404 ;;' \
	'  *"--write-out %{http_code}"*"api/v1/session"*) printf "%s" 204 ;;' \
	'  *"/healthz"*) printf "%s" '\''{"status":"ok"}'\'' ;;' \
	'  *"api/v1/session"*) printf "%s" '\''{"authenticated":true}'\'' ;;' \
	'  *"api/v1/overview"*) printf "%s" '\''{"total":1}'\'' ;;' \
	'  *"api/v1/hosts?page"*) printf "%s" '\''{"hosts":[]}'\'' ;;' \
	'  *"mock-host-001/metrics"*) printf "%s" '\''{"series":[]}'\'' ;;' \
	'  *"mock-host-001"*) printf "%s" '\''{"id":"mock-host-001"}'\'' ;;' \
	'  *"api/v1/datasource/status"*) printf "%s" '\''{"healthy":true}'\'' ;;' \
	'  *) printf "%s" '\''<div id="root"></div>'\'' ;;' \
	'esac' >"$test_dir/bin/curl"
chmod +x "$test_dir/bin/curl"

status=0
PATH="$test_dir/bin:$PATH" \
	DOCKER_CALL_LOG="$call_log" \
	INFRAVIEW_E2E_PROJECT=infraview-existing \
	"$repo_root/scripts/e2e.sh" >"$output_log" 2>&1 || status=$?

[ "$status" -ne 0 ] || {
	echo "e2e-safety: FAIL: 同名项目存在时脚本仍返回成功" >&2
	exit 1
}
grep -q '^compose ls --all --format json$' "$call_log" || {
	echo "e2e-safety: FAIL: 启动前未查询现有 Compose 项目" >&2
	exit 1
}
if grep -Eq '(^| )up( |$)' "$call_log"; then
	echo "e2e-safety: FAIL: 同名项目存在时仍执行了 up" >&2
	exit 1
fi
if grep -Eq '(^| )down( |$)' "$call_log"; then
	echo "e2e-safety: FAIL: 同名项目存在时仍执行了 down" >&2
	exit 1
fi
grep -q '项目名已存在' "$output_log" || {
	echo "e2e-safety: FAIL: 缺少明确碰撞错误" >&2
	exit 1
}

: >"$call_log"
status=0
PATH="$test_dir/bin:$PATH" \
	DOCKER_CALL_LOG="$call_log" \
	DOCKER_SCENARIO=resource \
	INFRAVIEW_E2E_PROJECT=infraview-orphan \
	"$repo_root/scripts/e2e.sh" >"$output_log" 2>&1 || status=$?
[ "$status" -ne 0 ] || {
	echo "e2e-safety: FAIL: 同标签资源存在时脚本仍返回成功" >&2
	exit 1
}
if grep -Eq '(^| )(up|down)( |$)' "$call_log"; then
	echo "e2e-safety: FAIL: 同标签资源存在时执行了 up/down" >&2
	exit 1
fi
grep -q '项目资源已存在' "$output_log" || {
	echo "e2e-safety: FAIL: 缺少资源标签碰撞错误" >&2
	exit 1
}

first_project=
second_project=
run=1
while [ "$run" -le 2 ]; do
	: >"$call_log"
	status=0
	PATH="$test_dir/bin:$PATH" \
		DOCKER_CALL_LOG="$call_log" \
		DOCKER_SCENARIO=empty \
		"$repo_root/scripts/e2e.sh" >"$output_log" 2>&1 || status=$?
	[ "$status" -ne 0 ] || {
		echo "e2e-safety: FAIL: 假 Docker up 失败时 E2E 仍返回成功" >&2
		exit 1
	}
	project=$(awk '$1 == "compose" && $2 == "-p" { for (field = 1; field <= NF; field++) if ($field == "up") { print $3; exit } }' "$call_log")
	[ -n "$project" ] || {
		echo "e2e-safety: FAIL: 未观察到默认项目名" >&2
		exit 1
	}
	case "$project" in
		infraview-e2e-*) ;;
		*)
			echo "e2e-safety: FAIL: 默认项目名不含唯一后缀：$project" >&2
			exit 1
			;;
	esac
	down_project=$(awk '$1 == "compose" && $2 == "-p" { for (field = 1; field <= NF; field++) if ($field == "down") { print $3; exit } }' "$call_log")
	[ "$down_project" = "$project" ] || {
		echo "e2e-safety: FAIL: up 部分失败后未精确清理本次项目：up=$project down=${down_project:-none}" >&2
		exit 1
	}
	if [ "$run" -eq 1 ]; then
		first_project=$project
	else
		second_project=$project
	fi
	run=$((run + 1))
done
[ "$first_project" != "$second_project" ] || {
	echo "e2e-safety: FAIL: 连续运行复用了默认项目名 $first_project" >&2
	exit 1
}

: >"$call_log"
status=0
PATH="$test_dir/bin:$PATH" \
	DOCKER_CALL_LOG="$call_log" \
	DOCKER_SCENARIO=full \
	INFRAVIEW_E2E_PROJECT=infraview-dependency-test \
	"$repo_root/scripts/e2e.sh" >"$output_log" 2>&1 || status=$?
[ "$status" -eq 0 ] || {
	echo "e2e-safety: FAIL: 假成功场景未执行到 Playwright 容器" >&2
	exit 1
}
grep -Fq -- '-v /work/web/node_modules' "$call_log" || {
	echo "e2e-safety: FAIL: Playwright 容器未隔离 node_modules" >&2
	exit 1
}
grep -Fq -- 'sh -c npm ci && npx playwright test' "$call_log" || {
	echo "e2e-safety: FAIL: Playwright 容器未从锁文件安装依赖" >&2
	exit 1
}

grep -Fq 'proxy_set_header X-Real-IP $remote_addr;' "$repo_root/docs/DEPLOYMENT.md" || {
	echo "e2e-safety: FAIL: Nginx 示例未覆盖写入单值 X-Real-IP" >&2
	exit 1
}
grep -Fq 'header_up X-Real-IP {remote_host}' "$repo_root/docs/DEPLOYMENT.md" || {
	echo "e2e-safety: FAIL: Caddy 示例未覆盖写入单值 X-Real-IP" >&2
	exit 1
}

echo "e2e-safety: PASS"
