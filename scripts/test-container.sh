#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

command -v docker >/dev/null
command -v curl >/dev/null

image_tag="vibe-mud-container-test:${$}"
container_name="vibe-mud-container-test-${$}"
test_root="$(mktemp -d)"
mkdir "$test_root/data"

cleanup() {
  docker rm --force "$container_name" >/dev/null 2>&1 || true
  docker image rm "$image_tag" >/dev/null 2>&1 || true
  rm -rf "$test_root"
}
trap cleanup EXIT

docker build --tag "$image_tag" .
docker run --detach \
  --name "$container_name" \
  --publish 127.0.0.1::8080 \
  --volume "$test_root/data:/data" \
  --env DATABASE_PATH=/data/mud.db \
  --env FRONTEND_URL=https://vibe-mud-api.fly.dev \
  --env GOOGLE_CLIENT_ID=placeholder-client-id \
  --env GOOGLE_CLIENT_SECRET=placeholder-client-secret \
  --env GOOGLE_REDIRECT_URL=https://vibe-mud-api.fly.dev/auth/google/callback \
  --env PORT=8080 \
  "$image_tag" >/dev/null

port=""
entry_file="$test_root/index.html"
for _ in $(seq 1 60); do
  port="$(docker port "$container_name" 8080/tcp 2>/dev/null | sed -n 's/.*://p' | head -n 1 || true)"
  if [ -n "$port" ] && curl --fail --silent --show-error "http://127.0.0.1:${port}/" >"$entry_file"; then
    break
  fi
  if ! docker ps --quiet --filter "name=^/${container_name}$" | grep -q .; then
    docker logs "$container_name" >&2
    exit 1
  fi
  sleep 1
done

if [ ! -s "$entry_file" ]; then
  docker logs "$container_name" >&2
  exit 1
fi

asset_path="$(grep --only-matching --extended-regexp '/assets/[^" ]+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+' "$entry_file" | head -n 1 || true)"
if [ -z "$asset_path" ]; then
  echo "entry document did not reference a versioned asset" >&2
  exit 1
fi
curl --fail --silent --show-error "http://127.0.0.1:${port}${asset_path}" >"$test_root/asset"

runtime_files="$test_root/runtime-files"
docker export "$container_name" | tar --list --file=- >"$runtime_files"
grep -Fxq "server" "$runtime_files"
grep -Fxq "web/dist/index.html" "$runtime_files"
grep -Fxq "web/dist${asset_path}" "$runtime_files"

if grep --extended-regexp --quiet \
  '(^|/)(node|npm|npx|node_modules|\.wrangler|wrangler|cloudflare|workerd)(/|$)|(^|/)web/(src|functions|package(-lock)?\.json|wrangler\.jsonc)(/|$)' \
  "$runtime_files"; then
  echo "runtime image contains frontend tooling, source, or Cloudflare files" >&2
  exit 1
fi

echo "container test passed: entry and ${asset_path} served from the production image"
