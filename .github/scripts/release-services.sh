#!/usr/bin/env bash
set -euo pipefail

release_sha=${1:-}
force_all=${2:-}
case "$release_sha" in
	*[!0-9a-f]*|'') echo 'release SHA must be a full lowercase commit SHA' >&2; exit 2 ;;
esac
[[ ${#release_sha} -eq 40 ]]
git rev-parse --verify "$release_sha^{commit}" >/dev/null
case "$force_all" in
	''|--all) ;;
	*) echo 'unknown release service selector' >&2; exit 2 ;;
esac

appview=false
inference=false
clustering=false
if [[ "$force_all" == --all ]]; then
	appview=true
	inference=true
	clustering=true
else
	while IFS= read -r path; do
		case "$path" in
			appview/*) appview=true ;;
			inference/*|docker-compose.inference.scaleway.yml) inference=true ;;
			clustering/*) clustering=true ;;
			docker-compose.scaleway.yml) appview=true; clustering=true ;;
		esac
	done < <(git diff-tree --no-commit-id --name-only -r -m --root "$release_sha")
fi

has_services=false
if "$appview" || "$inference" || "$clustering"; then has_services=true; fi

matrix='{"include":['
add_image() {
	[[ "$matrix" == '{"include":[' ]] || matrix+=','
	matrix+="$1"
}
"$appview" && add_image '{"image":"currents-appview","context":"appview"}'
"$inference" && add_image '{"image":"currents-inference","context":"inference"}'
"$clustering" && add_image '{"image":"currents-clustering","context":"clustering"}'
matrix+=']}'

{
	printf 'release_sha=%s\n' "$release_sha"
	printf 'appview=%s\n' "$appview"
	printf 'inference=%s\n' "$inference"
	printf 'clustering=%s\n' "$clustering"
	printf 'has_services=%s\n' "$has_services"
	printf 'matrix=%s\n' "$matrix"
} >> "$GITHUB_OUTPUT"
