#!/bin/sh
set -eu
umask 077

project_dir=/opt/currents
compose_file=docker-compose.scaleway.yml
env_file=.env.production.phase-a
registry_env="$project_dir/.env.registry"
state_dir=/var/lib/currents-releases
lock_file=/run/lock/currents-deploy-main.lock
backup_service=currents-postgres-backup.service

fail() {
	printf 'deploy-main: %s\n' "$*" >&2
	exit 1
}

release_sha=${1:-}
services=${2:-}
[ "$#" -eq 2 ] || fail 'expected a release SHA and service list'

case "$release_sha" in
	*[!0-9a-f]*|'') fail 'release SHA must be lowercase hexadecimal' ;;
esac
[ "${#release_sha}" -eq 40 ] || fail 'release SHA must contain 40 characters'
case "$services" in
	appview|clustering|appview,clustering) ;;
	*) fail 'services must be appview, clustering, or appview,clustering' ;;
esac

[ "$(id -u)" -eq 0 ] || fail 'must run as root'
[ -d "$project_dir" ] || fail "$project_dir is missing"
[ -r "$registry_env" ] || fail "$registry_env is missing"

set -a
# The deployment configuration is root-owned.
# shellcheck disable=SC1090
. "$registry_env"
set +a
: "${SCW_REGISTRY:?SCW_REGISTRY is missing from .env.registry}"
case "$SCW_REGISTRY" in
	rg.fr-par.scw.cloud/*) ;;
	*) fail 'SCW_REGISTRY must be an fr-par namespace endpoint' ;;
esac
case "$SCW_REGISTRY" in
	*/) fail 'SCW_REGISTRY must not end with a slash' ;;
esac

registry_config=$(mktemp -d)
cleanup_registry_config() {
	rm -f "$registry_config/config.json"
	rmdir "$registry_config" 2>/dev/null || true
}
trap cleanup_registry_config EXIT HUP INT TERM
chmod 700 "$registry_config"
IFS= read -r registry_key || fail 'registry credential is missing'
[ -n "$registry_key" ] || fail 'registry credential is missing'
printf '%s\n' "$registry_key" |
	docker --config "$registry_config" login "$SCW_REGISTRY" -u nologin --password-stdin
unset registry_key

registry_docker() {
	docker --config "$registry_config" "$@"
}

exec 9>"$lock_file"
flock -n 9 || fail 'another main-VM release is active'
mkdir -p "$state_dir"
chmod 700 "$state_dir"

compose() {
	(
		cd "$project_dir"
		docker compose --env-file "$env_file" -f "$compose_file" "$@"
	)
}

container_id() {
	compose ps -q "$1"
}

container_running() {
	id=$(container_id "$1")
	[ -n "$id" ] && [ "$(docker inspect --format '{{.State.Running}}' "$id" 2>/dev/null)" = true ]
}

migration_state() {
	db_id=$(container_id db)
	[ -n "$db_id" ] || {
		printf 'unknown\n'
		return
	}
	state=$(docker exec "$db_id" psql -U appview -d appview -Atqc \
		"SELECT CASE WHEN dirty THEN 'dirty' ELSE 'clean' END FROM schema_migrations LIMIT 1" \
		2>/dev/null || true)
	case "$state" in
		clean|dirty) printf '%s\n' "$state" ;;
		*) printf 'unknown\n' ;;
	esac
}

appview_responds() {
	id=$(container_id appview)
	[ -n "$id" ] || return 1
	docker exec "$id" wget -q -O /dev/null http://127.0.0.1:8080/api/supporter/stats >/dev/null 2>&1
}

public_appview_responds() {
	curl -fsS --connect-timeout 2 --max-time 5 -o /dev/null \
		https://api.currents.is/api/supporter/stats &&
		curl -fsS --connect-timeout 2 --max-time 5 -o /dev/null \
		https://api.currents.is/.well-known/did.json &&
		curl -fsS --connect-timeout 2 --max-time 5 -o /dev/null \
		https://api.currents.is/oauth-client-metadata.json &&
		curl -fsS --connect-timeout 2 --max-time 5 -o /dev/null \
		'https://api.currents.is/xrpc/is.currents.feed.getFeed?limit=1&personalized=0'
}

wait_for_appview() {
	i=0
	while [ "$i" -lt 60 ]; do
		if container_running appview && [ "$(migration_state)" = clean ] && \
			appview_responds && public_appview_responds; then
			return 0
		fi
		i=$((i + 1))
		sleep 2
	done
	return 1
}

wait_for_clustering() {
	i=0
	while [ "$i" -lt 5 ]; do
		container_running clustering || return 1
		i=$((i + 1))
		sleep 2
	done
}

record_release() {
	service=$1
	image=$2
	previous_id=$3
	tmp="$state_dir/$service.current.$$"
	if [ -f "$state_dir/$service.current" ]; then
		cp "$state_dir/$service.current" "$state_dir/$service.previous"
		chmod 600 "$state_dir/$service.previous"
	fi
	printf 'sha=%s\nimage=%s\ndeployed_at=%s\nprevious_image_id=%s\n' \
		"$release_sha" "$image" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$previous_id" >"$tmp"
	chmod 600 "$tmp"
	mv "$tmp" "$state_dir/$service.current"
}

prepare_rollback() {
	service=$1
	rollback_image=$2
	id=$(container_id "$service")
	[ -n "$id" ] || fail "$service container is not running; no rollback image is available"
	previous_id=$(docker inspect --format '{{.Image}}' "$id")
	docker image tag "$previous_id" "$rollback_image"
	printf '%s\n' "$previous_id"
}

deploy_appview() {
	image="$SCW_REGISTRY/currents-appview:$release_sha"
	rollback_image=currents-appview:rollback
	previous_id=$(prepare_rollback appview "$rollback_image")

	if systemctl is-active --quiet "$backup_service"; then
		fail 'a PostgreSQL backup is already active'
	fi
	printf 'deploy-main: taking a fresh database backup before appview\n'
	systemctl start "$backup_service"

	export APPVIEW_IMAGE="$image"
	compose up -d --no-deps --no-build --force-recreate appview
	if wait_for_appview; then
		record_release appview "$image" "$previous_id"
		logger -t currents-deploy "service=appview sha=$release_sha result=success"
		printf 'deploy-main: appview %s is healthy\n' "$release_sha"
		return
	fi

	if [ "$(migration_state)" = dirty ]; then
		logger -t currents-deploy "service=appview sha=$release_sha result=dirty-migration"
		fail 'schema migration is dirty; automatic rollback was refused'
	fi

	printf 'deploy-main: appview health failed; restoring the previous image\n' >&2
	export APPVIEW_IMAGE="$rollback_image"
	compose up -d --no-deps --no-build --force-recreate appview
	if wait_for_appview; then
		logger -t currents-deploy "service=appview sha=$release_sha result=rolled-back"
		fail 'appview health failed; previous image restored'
	fi
	logger -t currents-deploy "service=appview sha=$release_sha result=rollback-failed"
	fail 'appview and its automatic rollback both failed health checks'
}

deploy_clustering() {
	image="$SCW_REGISTRY/currents-clustering:$release_sha"
	rollback_image=currents-clustering:rollback
	previous_id=$(prepare_rollback clustering "$rollback_image")

	export CLUSTERING_IMAGE="$image"
	compose up -d --no-deps --no-build --force-recreate clustering
	if wait_for_clustering; then
		record_release clustering "$image" "$previous_id"
		logger -t currents-deploy "service=clustering sha=$release_sha result=success"
		printf 'deploy-main: clustering %s is running\n' "$release_sha"
		return
	fi

	printf 'deploy-main: clustering failed; restoring the previous image\n' >&2
	export CLUSTERING_IMAGE="$rollback_image"
	compose up -d --no-deps --no-build --force-recreate clustering
	if wait_for_clustering; then
		logger -t currents-deploy "service=clustering sha=$release_sha result=rolled-back"
		fail 'clustering failed; previous image restored'
	fi
	logger -t currents-deploy "service=clustering sha=$release_sha result=rollback-failed"
	fail 'clustering and its automatic rollback both failed'
}

case "$services" in
	appview) registry_docker pull "$SCW_REGISTRY/currents-appview:$release_sha" ;;
	clustering) registry_docker pull "$SCW_REGISTRY/currents-clustering:$release_sha" ;;
	appview,clustering)
		registry_docker pull "$SCW_REGISTRY/currents-appview:$release_sha"
		registry_docker pull "$SCW_REGISTRY/currents-clustering:$release_sha"
		;;
esac

case "$services" in
	appview) deploy_appview ;;
	clustering) deploy_clustering ;;
	appview,clustering)
		deploy_appview
		deploy_clustering
		;;
esac

if ! curl -fsS --connect-timeout 2 --max-time 5 -o /dev/null https://currents.is/; then
	printf 'deploy-main: warning: the independent Netlify frontend did not answer its final check\n' >&2
fi
