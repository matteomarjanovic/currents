#!/bin/sh
set -eu
umask 077

if [ "$#" -ne 5 ]; then
	echo "usage: $0 phase-a|final production-source db-source storage-source output" >&2
	exit 2
fi

phase=$1
production_source=$2
db_source=$3
storage_source=$4
output=$5
tmp="$output.tmp.$$"

cleanup() {
	rm -f "$tmp"
}
trap cleanup EXIT HUP INT TERM

case "$phase" in
	phase-a)
		oauth_hostname=api.currents.is
		caddyfile=./Caddyfile.scaleway.phase-a
		;;
	final)
		oauth_hostname=currents.is
		caddyfile=./Caddyfile.scaleway
		;;
	*)
		echo "invalid phase: $phase" >&2
		exit 2
		;;
esac

copy_key() {
	key=$1
	file=$2
	required=$3
	count=$(grep -c "^$key=" "$file" || true)
	if [ "$count" -ne 1 ]; then
		echo "$file must contain exactly one $key assignment" >&2
		exit 1
	fi
	line=$(grep "^$key=" "$file")
	if [ "$required" -eq 1 ] && [ -z "${line#*=}" ]; then
		echo "$file contains an empty $key" >&2
		exit 1
	fi
	printf '%s\n' "$line" >>"$tmp"
}

printf '# Generated for Scaleway %s; do not commit.\n' "$phase" >"$tmp"
copy_key DB_PASSWORD "$db_source" 1
copy_key SESSION_SECRET "$production_source" 1
copy_key CLIENT_SECRET_KEY "$production_source" 1
copy_key CLIENT_SECRET_KEY_ID "$production_source" 1
copy_key TAP_ADMIN_PASSWORD "$production_source" 1
copy_key HIDDEN_DIDS "$production_source" 0
copy_key LABELER_DID "$production_source" 0
copy_key LABELER_SIGNING_KEY "$production_source" 0
copy_key POLAR_WEBHOOK_SECRET "$production_source" 0
copy_key POLAR_ACCESS_TOKEN "$production_source" 0
copy_key POLAR_SERVER "$production_source" 1
copy_key S3_ACCESS_KEY "$storage_source" 1
copy_key S3_SECRET_KEY "$storage_source" 1
copy_key MODELS_S3_BUCKET "$storage_source" 1
copy_key BACKUPS_S3_BUCKET "$storage_source" 1

printf '%s\n' \
	"OAUTH_HOSTNAME=$oauth_hostname" \
	'SERVICE_HOSTNAME=api.currents.is' \
	'FRONTEND_URL=https://currents.is' \
	'CDN_URL=https://api.currents.is' \
	'INFERENCE_URL=http://172.16.8.2:8000' \
	'ORIGIN=https://currents.is' \
	"CADDYFILE=$caddyfile" \
	'PUBLIC_POLAR_PRODUCT_MONTHLY=6717e6de-771b-46bb-a0bd-cda4456bd92e' \
	'PUBLIC_POLAR_PRODUCT_YEARLY=4b4295d5-08d4-4d06-a72f-9fc93a999519' \
	>>"$tmp"

chmod 600 "$tmp"
mv "$tmp" "$output"
