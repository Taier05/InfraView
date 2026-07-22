#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
project_name=${INFRAVIEW_E2E_PROJECT:-infraview-e2e-$$}
host_port=${INFRAVIEW_E2E_PORT:-18080}
username=${INFRAVIEW_E2E_USERNAME:-e2e-admin}
password=${INFRAVIEW_E2E_PASSWORD-}
if [ -z "${INFRAVIEW_E2E_PASSWORD+x}" ]; then
	password='e2e-quote-"-slash-\-password'
fi
base_url="http://127.0.0.1:$host_port"
playwright_image=${PLAYWRIGHT_IMAGE:-mcr.microsoft.com/playwright:v1.61.1-noble}
env_file=$(mktemp)
project_created=false

fail() {
	echo "e2e: FAIL: $*" >&2
	exit 1
}

cleanup() {
	status=$?
	trap - EXIT HUP INT TERM
	cd "$repo_root"
	if [ "$project_created" = true ]; then
		INFRAVIEW_ENV_FILE="$env_file" INFRAVIEW_PORT="$host_port" \
			docker compose -p "$project_name" --env-file "$env_file" down --remove-orphans >/dev/null 2>&1 || true
	fi
	rm -f "$env_file"
	exit "$status"
}
trap cleanup EXIT HUP INT TERM

printf '%s\n' "$project_name" | grep -Eq '^[a-z0-9][a-z0-9_-]*$' || \
	fail "项目名只能包含小写字母、数字、连字符和下划线：$project_name"

compose_projects=$(docker compose ls --all --format json)
if printf '%s\n' "$compose_projects" | grep -Eq '"Name"[[:space:]]*:[[:space:]]*"'"$project_name"'"'; then
	fail "项目名已存在，拒绝复用或清理：$project_name"
fi

container_resources=$(docker ps -a --filter "label=com.docker.compose.project=$project_name" --quiet)
network_resources=$(docker network ls --filter "label=com.docker.compose.project=$project_name" --quiet)
volume_resources=$(docker volume ls --filter "label=com.docker.compose.project=$project_name" --quiet)
[ -z "$container_resources$network_resources$volume_resources" ] || \
	fail "项目资源已存在，拒绝复用或清理：$project_name"

case "$username$password" in
	*"'"* | *'
'*)
		echo "e2e: FAIL: E2E 凭据不能包含单引号或换行" >&2
		exit 1
		;;
esac

printf '%s\n' \
	"INFRAVIEW_USERNAME='$username'" \
	"INFRAVIEW_PASSWORD='$password'" \
	"INFRAVIEW_PORT=$host_port" \
	'INFRAVIEW_LISTEN_ADDR=:8080' \
	'INFRAVIEW_COOKIE_SECURE=false' \
	'INFRAVIEW_SESSION_TTL=12h' \
	'INFRAVIEW_DATA_SOURCE=mock' \
	'INFRAVIEW_MOCK_HOST_COUNT=32' \
	'INFRAVIEW_REFRESH_INTERVAL=30s' \
	'INFRAVIEW_INVENTORY_TTL=60s' \
	'INFRAVIEW_CURRENT_METRICS_TTL=20s' \
	'INFRAVIEW_RANGE_TTL=60s' \
	'INFRAVIEW_HEALTH_TTL=15s' \
	'INFRAVIEW_MAX_STALE=5m' \
	'INFRAVIEW_UPSTREAM_TIMEOUT=10s' \
	'INFRAVIEW_WARNING_PERCENT=80' \
	'INFRAVIEW_CRITICAL_PERCENT=90' \
	'TZ=Asia/Hong_Kong' >"$env_file"

cd "$repo_root"
INFRAVIEW_ENV_FILE="$env_file" INFRAVIEW_PORT="$host_port" \
	docker compose -p "$project_name" --env-file "$env_file" up -d --build
project_created=true

INFRAVIEW_BASE_URL="$base_url" \
	INFRAVIEW_USERNAME="$username" \
	INFRAVIEW_PASSWORD="$password" \
	"$repo_root/scripts/smoke.sh"

if [ "${INFRAVIEW_E2E_RUN_BENCHMARK:-false}" = true ]; then
	INFRAVIEW_BASE_URL="$base_url" \
		INFRAVIEW_USERNAME="$username" \
		INFRAVIEW_PASSWORD="$password" \
		"$repo_root/scripts/benchmark.sh"
fi

if [ "${INFRAVIEW_E2E_CHECK_RESOURCES:-false}" = true ]; then
	container_id=$(
		INFRAVIEW_ENV_FILE="$env_file" INFRAVIEW_PORT="$host_port" \
			docker compose -p "$project_name" --env-file "$env_file" ps -q infraview
	)
	[ -n "$container_id" ] || {
		echo "resources: FAIL: 找不到 InfraView 容器" >&2
		exit 1
	}
	memory_usage=$(docker stats --no-stream --format '{{.MemUsage}}' "$container_id" | awk '{ print $1 }')
	memory_mib=$(awk -v raw="$memory_usage" '
		BEGIN {
			value = raw + 0
			unit = raw
			gsub(/[0-9.]/, "", unit)
			factor = 1
			if (unit == "KiB") factor = 1024
			else if (unit == "MiB") factor = 1024 * 1024
			else if (unit == "GiB") factor = 1024 * 1024 * 1024
			printf "%.3f", value * factor / (1024 * 1024)
		}')
	if ! awk -v value="$memory_mib" 'BEGIN { exit(value < 256 ? 0 : 1) }'; then
		echo "resources: FAIL: memory=${memory_mib}MiB，要求低于 256MiB" >&2
		exit 1
	fi
	image_id=$(docker inspect --format '{{.Image}}' "$container_id")
	image_bytes=$(docker image inspect --format '{{.Size}}' "$image_id")
	image_mb=$(awk -v bytes="$image_bytes" 'BEGIN { printf "%.2f", bytes / 1000000 }')
	echo "resources: PASS memory=${memory_mib}MiB image=${image_mb}MB"
fi

docker run --rm --network host --ipc=host \
	-e CI=1 \
	-e INFRAVIEW_E2E_BASE_URL="$base_url" \
	-e INFRAVIEW_E2E_USERNAME="$username" \
	-e INFRAVIEW_E2E_PASSWORD="$password" \
	-v "$repo_root:/work" \
	-w /work/web \
	"$playwright_image" \
	npx playwright test
