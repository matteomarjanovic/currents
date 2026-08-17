#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: CONFIRM_REPLACE_REHEARSAL_DB=yes $0 /opt/currents/currents-production-<timestamp>.dump" >&2
	exit 2
fi
if [ "${CONFIRM_REPLACE_REHEARSAL_DB:-}" != yes ]; then
	echo "refusing to replace the rehearsal database without CONFIRM_REPLACE_REHEARSAL_DB=yes" >&2
	exit 2
fi

dump=$1
case "$dump" in
	/opt/currents/currents-production-*.dump) ;;
	*) echo "unexpected dump path: $dump" >&2; exit 2 ;;
esac
test -f "$dump"
test -f "$dump.sha256"

cd "$(dirname "$dump")"
sha256sum --check "$(basename "$dump").sha256"
docker exec -i currents-db-1 pg_restore --list <"$dump" >/dev/null

cd /opt/currents
docker compose --env-file .env.migration \
	-f docker-compose.scaleway.yml \
	stop appview tap frontend caddy clustering
systemctl stop currents-postgres-backup.timer

docker exec currents-db-1 dropdb -U appview --force appview
docker exec currents-db-1 createdb -U appview --template=template0 appview
docker exec -i currents-db-1 pg_restore \
	-U appview -d appview --no-owner --exit-on-error <"$dump"
docker exec currents-db-1 psql -U appview -d appview -c ANALYZE >/dev/null

docker exec currents-db-1 psql -U appview -d appview -Atc "
SELECT 'database_bytes=' || pg_database_size(current_database());
SELECT 'save_rows=' || count(*) FROM save;
SELECT 'visual_identity_rows=' || count(*) FROM visual_identity;
SELECT 'collection_rows=' || count(*) FROM collection;
SELECT 'user_rows=' || count(*) FROM \"user\";
SELECT 'schema_migration=' || version || ',dirty=' || dirty FROM schema_migrations;
SELECT 'invalid_indexes=' || count(*) FROM pg_index WHERE NOT indisvalid;
SELECT 'active_import_items=' || count(*) FROM import_item WHERE status IN ('queued', 'running');
SELECT 'pds_wipe_jobs=' || count(*) FROM pds_wipe;
"

invalid=$(docker exec currents-db-1 psql -U appview -d appview -Atc \
	"SELECT count(*) FROM pg_index WHERE NOT indisvalid")
dirty=$(docker exec currents-db-1 psql -U appview -d appview -Atc \
	"SELECT dirty FROM schema_migrations")
test "$invalid" -eq 0
test "$dirty" = f

echo "Production dump restored. Services and the backup timer remain stopped for operator verification."
