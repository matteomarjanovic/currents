#!/bin/sh
set -eu

root=/opt/currents-inference
models="$root/models"
key=umap_model.joblib
version_file="$models/.$key.version"
download="$models/.$key.download.$$"
version_tmp="$version_file.$$"

cleanup() {
	rm -f "$download" "$version_tmp"
}
trap cleanup EXIT HUP INT TERM

set -a
. "$root/.env.storage"
set +a
export AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$S3_SECRET_KEY"
export AWS_DEFAULT_REGION=fr-par

aws() {
	docker run --rm \
		-e AWS_ACCESS_KEY_ID \
		-e AWS_SECRET_ACCESS_KEY \
		-e AWS_DEFAULT_REGION \
		-v "$models:/models" \
		amazon/aws-cli:latest \
		--endpoint-url https://s3.fr-par.scw.cloud "$@"
}

version=$(aws s3api head-object \
	--bucket "$MODELS_S3_BUCKET" \
	--key "$key" \
	--query VersionId \
	--output text)

if [ -f "$version_file" ] && [ "$(cat "$version_file")" = "$version" ]; then
	exit 0
fi

aws s3api get-object \
	--bucket "$MODELS_S3_BUCKET" \
	--key "$key" \
	--version-id "$version" \
	"/models/$(basename "$download")" >/dev/null
test -s "$download"
mv "$download" "$models/$key"

cd /opt/currents
docker compose --env-file .env.inference \
	-f docker-compose.inference.scaleway.yml \
	exec -T inference curl --fail --request POST \
	http://localhost:8000/reload-umap

printf '%s\n' "$version" >"$version_tmp"
mv "$version_tmp" "$version_file"
