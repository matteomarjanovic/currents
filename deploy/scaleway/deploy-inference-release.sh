#!/bin/sh
set -eu
umask 077

project_dir=/opt/currents
compose_file=docker-compose.inference.scaleway.yml
env_file=.env.inference
registry_env="$project_dir/.env.registry"
state_dir=/var/lib/currents-releases
lock_file=/run/lock/currents-deploy-inference.lock

fail() {
	printf 'deploy-inference: %s\n' "$*" >&2
	exit 1
}

release_sha=${1:-}
case "$release_sha" in
	*[!0-9a-f]*|'') fail 'release SHA must be lowercase hexadecimal' ;;
esac
[ "${#release_sha}" -eq 40 ] || fail 'release SHA must contain 40 characters'
[ "$#" -eq 1 ] || fail 'expected exactly one release SHA'
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
flock -n 9 || fail 'another inference release is active'
mkdir -p "$state_dir"
chmod 700 "$state_dir"

compose() {
	(
		cd "$project_dir"
		docker compose --env-file "$env_file" -f "$compose_file" "$@"
	)
}

container_id() {
	compose ps -q inference
}

container_running() {
	id=$(container_id)
	[ -n "$id" ] && [ "$(docker inspect --format '{{.State.Running}}' "$id" 2>/dev/null)" = true ]
}

wait_for_inference() {
	i=0
	while [ "$i" -lt 210 ]; do
		id=$(container_id)
		if [ -n "$id" ] && container_running && \
			docker exec "$id" curl -fsS --max-time 5 -o /dev/null http://127.0.0.1:8000/health; then
			return 0
		fi
		i=$((i + 1))
		sleep 2
	done
	return 1
}

image="$SCW_REGISTRY/currents-inference:$release_sha"
rollback_image=currents-inference:rollback

registry_docker pull "$image"
current_id=$(container_id)
[ -n "$current_id" ] || fail 'inference container is not running; no rollback image is available'
previous_id=$(docker inspect --format '{{.Image}}' "$current_id")
docker image tag "$previous_id" "$rollback_image"

export INFERENCE_IMAGE="$image"
compose up -d --no-deps --no-build --force-recreate inference
if wait_for_inference; then
	tmp="$state_dir/inference.current.$$"
	if [ -f "$state_dir/inference.current" ]; then
		cp "$state_dir/inference.current" "$state_dir/inference.previous"
		chmod 600 "$state_dir/inference.previous"
	fi
	printf 'sha=%s\nimage=%s\ndeployed_at=%s\nprevious_image_id=%s\n' \
		"$release_sha" "$image" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$previous_id" >"$tmp"
	chmod 600 "$tmp"
	mv "$tmp" "$state_dir/inference.current"
	logger -t currents-deploy "service=inference sha=$release_sha result=success"
	printf 'deploy-inference: inference %s is healthy\n' "$release_sha"
	exit 0
fi

printf 'deploy-inference: health failed; restoring the previous image\n' >&2
export INFERENCE_IMAGE="$rollback_image"
compose up -d --no-deps --no-build --force-recreate inference
if wait_for_inference; then
	logger -t currents-deploy "service=inference sha=$release_sha result=rolled-back"
	fail 'inference health failed; previous image restored'
fi
logger -t currents-deploy "service=inference sha=$release_sha result=rollback-failed"
fail 'inference and its automatic rollback both failed health checks'
