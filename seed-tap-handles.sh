#!/bin/sh
set -eu

tap_url="${TAP_URL:-http://tap:2480}"
resolver_url="${HANDLE_RESOLVER_URL:-https://public.api.bsky.app}"
handles_csv="${TAP_DEV_HANDLES:-}"

if [ -z "$handles_csv" ]; then
  echo "tap-seed: no TAP_DEV_HANDLES configured" >&2
  exit 1
fi

echo "tap-seed: waiting for TAP at $tap_url"
until curl -fsS "$tap_url/health" >/dev/null; do
  sleep 1
done

dids_json=""
old_ifs=$IFS
IFS=,
for raw_handle in $handles_csv; do
  handle=$(printf '%s' "$raw_handle" | tr -d '[:space:]')
  if [ -z "$handle" ]; then
    continue
  fi

  echo "tap-seed: resolving $handle"
  response=$(curl -fsS "$resolver_url/xrpc/com.atproto.identity.resolveHandle?handle=$handle")
  did=$(printf '%s' "$response" | sed -n 's/.*"did":"\([^"]*\)".*/\1/p')
  if [ -z "$did" ]; then
    echo "tap-seed: could not resolve DID for $handle" >&2
    exit 1
  fi

  if [ -n "$dids_json" ]; then
    dids_json="$dids_json,"
  fi
  dids_json="$dids_json\"$did\""
done
IFS=$old_ifs

if [ -z "$dids_json" ]; then
  echo "tap-seed: no valid handles resolved" >&2
  exit 1
fi

payload=$(printf '{"dids":[%s]}' "$dids_json")
echo "tap-seed: seeding TAP with $handles_csv"
curl -fsS -X POST "$tap_url/repos/add" \
  -H "Content-Type: application/json" \
  -d "$payload"
echo
echo "tap-seed: done"