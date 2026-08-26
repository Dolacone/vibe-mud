#!/usr/bin/env bash
set -Eeuo pipefail

# Verify that the production origin reaches authentication before game logic.
app_origin="${1:-https://vibe-mud-api.fly.dev}"
response_file="$(mktemp)"
trap 'rm -f -- "$response_file"' EXIT
status="$(curl --disable --silent --show-error --output "$response_file" --write-out '%{http_code}' \
  --request POST \
  --header "Origin: ${app_origin}" \
  --header 'Cookie:' \
  --header 'Accept: application/json' \
  "${app_origin}/api/actions/rest")"
body="$(<"$response_file")"

if [ "$status" != "401" ] || [ "$body" != '{"error":"authentication required"}' ]; then
  echo "same-origin anonymous POST returned status=${status} body=${body}; expected 401 authentication JSON" >&2
  exit 1
fi

echo "same-origin anonymous POST reached authentication"
