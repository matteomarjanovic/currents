#!/bin/sh
set -eu

set -f
# Split the forced command into validated tokens; globbing is disabled.
# shellcheck disable=SC2086
set -- ${SSH_ORIGINAL_COMMAND:-}
[ "$#" -eq 3 ] || exit 64
[ "$1" = deploy ] || exit 64
sha=$2
services=$3
case "$sha" in *[!0-9a-f]*|'') exit 64 ;; esac
[ "${#sha}" -eq 40 ] || exit 64
case "$services" in
	appview|clustering|appview,clustering) ;;
	*) exit 64 ;;
esac

exec /usr/bin/sudo /usr/local/sbin/currents-deploy-main "$sha" "$services"
