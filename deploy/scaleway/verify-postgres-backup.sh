#!/bin/sh
set -eu
umask 077

if [ "$#" -ne 1 ]; then
	echo "usage: $0 currents-<timestamp>.dump" >&2
	exit 2
fi

name=$1
case "$name" in
	currents-*.dump) ;;
	*) echo "invalid backup name: $name" >&2; exit 2 ;;
esac
case "$name" in
	*/*) echo "backup name must not contain a path" >&2; exit 2 ;;
esac

backup_dir=/var/lib/currents-backups
archive="$backup_dir/$name"
checksum="$archive.sha256"
database="currents_restore_test_$(date -u +%Y%m%d%H%M%S)_$$"
created=0

cleanup() {
	if [ "$created" -eq 1 ]; then
		docker exec currents-db-1 \
			dropdb -U appview --force "$database" || true
	fi
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
		-v "$backup_dir:/backups" \
		amazon/aws-cli:latest \
		--endpoint-url https://s3.fr-par.scw.cloud "$@"
}

aws s3 cp "s3://$BACKUPS_S3_BUCKET/postgres/$name" \
	"/backups/$name" --no-progress
aws s3 cp "s3://$BACKUPS_S3_BUCKET/postgres/$name.sha256" \
	"/backups/$name.sha256" --no-progress

(
	cd "$backup_dir"
	sha256sum --check "$name.sha256"
)
docker exec -i currents-db-1 pg_restore --list <"$archive" >/dev/null

docker exec currents-db-1 createdb -U appview --template=template0 "$database"
created=1
docker exec -i currents-db-1 pg_restore \
	-U appview -d "$database" --no-owner --exit-on-error <"$archive"

docker exec currents-db-1 psql -U appview -d "$database" -Atc "
SELECT 'database_bytes=' || pg_database_size(current_database());
SELECT 'save_rows=' || count(*) FROM save;
SELECT 'visual_identity_rows=' || count(*) FROM visual_identity;
SELECT 'collection_rows=' || count(*) FROM collection;
SELECT 'user_rows=' || count(*) FROM \"user\";
SELECT 'schema_migration=' || version || ',dirty=' || dirty FROM schema_migrations;
SELECT 'invalid_indexes=' || count(*) FROM pg_index WHERE NOT indisvalid;
"

invalid=$(docker exec currents-db-1 psql -U appview -d "$database" -Atc \
	"SELECT count(*) FROM pg_index WHERE NOT indisvalid")
dirty=$(docker exec currents-db-1 psql -U appview -d "$database" -Atc \
	"SELECT dirty FROM schema_migrations")
test "$invalid" -eq 0
test "$dirty" = f

docker exec currents-db-1 dropdb -U appview --force "$database"
created=0
printf 'Restore verified and temporary database removed: %s\n' "$name"
