#!/bin/sh
set -eu
umask 077

backup_dir=/var/lib/currents-backups
stamp=$(date -u +%Y-%m-%dT%H%M%SZ)
name="currents-$stamp.dump"
archive="$backup_dir/$name"
checksum="$archive.sha256"

cleanup() {
	rm -f "$archive" "$checksum"
}
trap cleanup EXIT HUP INT TERM

set -a
. /opt/currents/.env.storage
set +a
export AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$S3_SECRET_KEY"
export AWS_DEFAULT_REGION=fr-par

aws() {
	docker run --rm \
		-e AWS_ACCESS_KEY_ID \
		-e AWS_SECRET_ACCESS_KEY \
		-e AWS_DEFAULT_REGION \
		-v "$backup_dir:/backups:ro" \
		amazon/aws-cli:latest \
		--endpoint-url https://s3.fr-par.scw.cloud "$@"
}

docker exec currents-db-1 \
	pg_dump -U appview -d appview --format=custom >"$archive"
test -s "$archive"
docker exec -i currents-db-1 pg_restore --list <"$archive" >/dev/null

(
	cd "$backup_dir"
	sha256sum "$name" >"$name.sha256"
)

aws s3 cp "/backups/$name" \
	"s3://$BACKUPS_S3_BUCKET/postgres/$name" --no-progress
aws s3 cp "/backups/$name.sha256" \
	"s3://$BACKUPS_S3_BUCKET/postgres/$name.sha256" --no-progress

printf 'Uploaded s3://%s/postgres/%s\n' "$BACKUPS_S3_BUCKET" "$name"
