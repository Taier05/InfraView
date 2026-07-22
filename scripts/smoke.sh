#!/bin/sh
set -eu

base_url=${INFRAVIEW_BASE_URL:-http://127.0.0.1:8080}
username=${INFRAVIEW_USERNAME:-admin}
password=${INFRAVIEW_PASSWORD:-change-me-please}

cookie_file=$(mktemp)
response_file=$(mktemp)
trap 'rm -f "$cookie_file" "$response_file"' EXIT HUP INT TERM

fail() {
	echo "smoke: FAIL: $*" >&2
	exit 1
}

json_escape() {
	printf '%s' "$1" | LC_ALL=C od -An -v -tu1 | awk '
		{
			for (field = 1; field <= NF; field++) {
				byte = $field + 0
				if (byte == 8) printf "\\b"
				else if (byte == 9) printf "\\t"
				else if (byte == 10) printf "\\n"
				else if (byte == 12) printf "\\f"
				else if (byte == 13) printf "\\r"
				else if (byte == 34) printf "\\\""
				else if (byte == 92) printf "\\\\"
				else if (byte < 32) printf "\\u%04x", byte
				else printf "%c", byte
			}
		}'
}

health_ready=false
health_deadline=$(($(date +%s) + 60))
while [ "$(date +%s)" -lt "$health_deadline" ]; do
	if curl --fail --silent --show-error --max-time 2 "$base_url/healthz" >"$response_file" 2>/dev/null; then
		health_ready=true
		break
	fi
	sleep 1
done
[ "$health_ready" = true ] || fail "健康检查在 60 秒内未就绪"
grep -q '"status":"ok"' "$response_file" || fail "健康检查响应不正确"

escaped_username=$(json_escape "$username")
escaped_password=$(json_escape "$password")
login_status=$(
	printf '{"username":"%s","password":"%s"}' "$escaped_username" "$escaped_password" |
		curl --silent --show-error --output "$response_file" --write-out '%{http_code}' \
			--cookie-jar "$cookie_file" \
			--header 'Content-Type: application/json' \
			--data-binary @- \
			"$base_url/api/v1/session"
)
[ "$login_status" = 204 ] || fail "登录返回 HTTP $login_status"

get_json() {
	path=$1
	marker=$2
	curl --fail --silent --show-error --cookie "$cookie_file" "$base_url$path" >"$response_file" || fail "GET $path 失败"
	grep -q "$marker" "$response_file" || fail "GET $path 响应缺少 $marker"
}

get_json "/api/v1/session" '"authenticated":true'
get_json "/api/v1/overview?range=24h" '"total"'
get_json "/api/v1/hosts?page=1&page_size=20" '"hosts"'
get_json "/api/v1/hosts/mock-host-001" '"id":"mock-host-001"'
get_json "/api/v1/hosts/mock-host-001/metrics?range=6h" '"series"'
get_json "/api/v1/datasource/status" '"healthy":true'

curl --fail --silent --show-error "$base_url/" >"$response_file" || fail "根页面访问失败"
grep -q '<div id="root"></div>' "$response_file" || fail "根页面不是 InfraView SPA"

assert_disallowed() {
	method=$1
	path=$2
	status=$(
		curl --silent --show-error --output "$response_file" --write-out '%{http_code}' \
			--cookie "$cookie_file" \
			--request "$method" \
			--header 'Content-Type: application/json' \
			--data '{}' \
			"$base_url$path"
	)
	case "$status" in
		404|405) ;;
		*) fail "$method $path 未被拒绝，返回 HTTP $status" ;;
	esac
}

assert_disallowed GET "/api/v1/command"
assert_disallowed POST "/api/v1/commands"
assert_disallowed POST "/api/v1/hosts/mock-host-001/restart"
assert_disallowed DELETE "/api/v1/hosts/mock-host-001"
assert_disallowed GET "/api/v1/proxy"
assert_disallowed POST "/api/v1/query"

echo "smoke: PASS"
