#!/bin/sh
set -eu
umask 077

backup_dir=/var/lib/currents-backups
stamp=$(date -u +%Y-%m-%dT%H%M%SZ)
started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
name="currents-$stamp.dump"
archive="$backup_dir/$name"
checksum="$archive.sha256"
succeeded=0

cleanup() {
	rm -f "$archive" "$checksum"
}

record_run() {
	run_status=$1
	details=$2
	docker exec currents-db-1 psql -v ON_ERROR_STOP=1 -U appview -d appview \
		-v run_status="$run_status" -v started_at="$started_at" -v details="$details" \
		-c "INSERT INTO operations_job_run (job, status, started_at, finished_at, details) VALUES ('postgres_backup', :'run_status', :'started_at'::timestamptz, now(), :'details'::jsonb)" \
		>/dev/null 2>&1 || true
}

finish() {
	exit_status=$?
	set +e
	if [ "$succeeded" -eq 1 ]; then
		backup_bytes=$(wc -c <"$archive")
		record_run success "{\"name\":\"$name\",\"bytes\":$backup_bytes}"
	else
		record_run failed '{}'
	fi
	cleanup
	trap - EXIT
	exit "$exit_status"
}
trap finish EXIT
trap 'exit 1' HUP INT TERM

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

succeeded=1
printf 'Uploaded s3://%s/postgres/%s\n' "$BACKUPS_S3_BUCKET" "$name"
