#!/bin/sh
set -eu

: "${OPS_REPORT_URL:?}"
: "${OPS_REPORTING_SECRET:?}"
: "${OPS_HOST:?}"
: "${OPS_DISK_PATH:?}"

case "$OPS_HOST" in
	main|inference) ;;
	*) echo 'OPS_HOST must be main or inference' >&2; exit 2 ;;
esac

memory_total=$(awk '/^MemTotal:/ { printf "%.0f", $2 * 1024 }' /proc/meminfo)
memory_available=$(awk '/^MemAvailable:/ { printf "%.0f", $2 * 1024 }' /proc/meminfo)
load1=$(cut -d ' ' -f 1 /proc/loadavg)

set -- $(df -B1 --output=size,used,avail "$OPS_DISK_PATH" | awk 'NR == 2 { print $1, $2, $3 }')
[ "$#" -eq 3 ] || { echo "could not read disk metrics for $OPS_DISK_PATH" >&2; exit 1; }
storage_total=$1
storage_used=$2
storage_available=$3

containers=$(docker stats --no-stream --format '{{json .}}' 2>/dev/null |
	awk 'BEGIN { printf "[" } { if (NR > 1) printf ","; printf "%s", $0 } END { printf "]" }') || containers='[]'

model_version=null
model_updated_at=null
if [ -n "${OPS_MODEL_VERSION_FILE:-}" ] && [ -f "$OPS_MODEL_VERSION_FILE" ]; then
	model_version=$(tr -d '\n' <"$OPS_MODEL_VERSION_FILE")
	model_version=$(printf '"%s"' "$model_version")
	model_updated_at=$(date -u -d "@$(stat -c %Y "$OPS_MODEL_VERSION_FILE")" +%Y-%m-%dT%H:%M:%SZ)
	model_updated_at=$(printf '"%s"' "$model_updated_at")
fi

body=$(printf '{"host":"%s","memory":{"totalBytes":%s,"availableBytes":%s},"load1":%s,"storage":{"path":"%s","totalBytes":%s,"usedBytes":%s,"availableBytes":%s},"containers":%s,"modelVersion":%s,"modelUpdatedAt":%s}' \
	"$OPS_HOST" "$memory_total" "$memory_available" "$load1" "$OPS_DISK_PATH" \
	"$storage_total" "$storage_used" "$storage_available" "$containers" "$model_version" "$model_updated_at")
timestamp=$(date -u +%s)
signature=$(printf '%s\n%s' "$timestamp" "$body" |
	openssl dgst -sha256 -hmac "$OPS_REPORTING_SECRET" -binary | base64 | tr -d '\n')

curl --fail --silent --show-error --max-time 10 \
	-X POST "$OPS_REPORT_URL" \
	-H "Content-Type: application/json" \
	-H "X-Currents-Ops-Timestamp: $timestamp" \
	-H "X-Currents-Ops-Signature: sha256=$signature" \
	--data-binary "$body" >/dev/null
