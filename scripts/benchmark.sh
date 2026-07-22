#!/bin/sh
set -eu

base_url=${INFRAVIEW_BASE_URL:-http://127.0.0.1:18080}
username=${INFRAVIEW_USERNAME:-admin}
password=${INFRAVIEW_PASSWORD:-change-me-please}

cookie_file=$(mktemp)
response_file=$(mktemp)
times_file=$(mktemp)
trap 'rm -f "$cookie_file" "$response_file" "$times_file"' EXIT HUP INT TERM

fail() {
	echo "benchmark: FAIL: $*" >&2
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

curl --fail --silent --show-error --cookie "$cookie_file" \
	"$base_url/api/v1/overview?range=24h" >"$response_file" || fail "预热总览失败"

sample=1
while [ "$sample" -le 100 ]; do
	curl --fail --silent --show-error --output /dev/null \
		--write-out '%{time_total}\n' \
		--cookie "$cookie_file" \
		"$base_url/api/v1/overview?range=24h" >>"$times_file" || fail "第 $sample 次请求失败"
	sample=$((sample + 1))
done

p95=$(LC_ALL=C sort -n "$times_file" | sed -n '95p')
[ -n "$p95" ] || fail "无法计算第 95 个样本"
if ! awk -v value="$p95" 'BEGIN { exit(value < 0.200 ? 0 : 1) }'; then
	fail "p95=${p95}s，要求低于 0.200s"
fi

echo "benchmark: PASS p95=${p95}s"
