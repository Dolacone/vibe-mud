#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

command -v docker >/dev/null
command -v curl >/dev/null
command -v npm >/dev/null
command -v node >/dev/null
command -v go >/dev/null

go test -count=1 -ldflags=-linkmode=external ./...
(cd web && npm test -- --run)
(cd web && npm run build)

image_tag="vibe-mud-container-test:${$}"
container_name="vibe-mud-container-test-${$}"
base_container_name="vibe-mud-base-test-${$}"
inspect_container_name="vibe-mud-inspect-test-${$}"
test_root="$(mktemp -d)"
mkdir "$test_root/data"

cleanup() {
  docker rm --force "$container_name" >/dev/null 2>&1 || true
  docker rm --force "$base_container_name" >/dev/null 2>&1 || true
  docker rm --force "$inspect_container_name" >/dev/null 2>&1 || true
  docker image rm "$image_tag" >/dev/null 2>&1 || true
  rm -rf "$test_root"
}
trap cleanup EXIT

docker build --tag "$image_tag" .
docker create --name "$base_container_name" gcr.io/distroless/static-debian12 >/dev/null
docker create --name "$inspect_container_name" "$image_tag" >/dev/null
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
manifest_file="$test_root/manifest.webmanifest"
route_file="$test_root/client-route.html"
for _ in $(seq 1 60); do
  port="$(docker port "$container_name" 8080/tcp 2>/dev/null | sed -n 's/.*://p' | head -n 1 || true)"
  if [ -n "$port" ] && curl --fail --silent --show-error --dump-header "$test_root/entry.headers" "http://127.0.0.1:${port}/" >"$entry_file"; then
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
grep -Eiq '^Content-Type: text/html' "$test_root/entry.headers"
grep -Fq '<div id="root"></div>' "$entry_file"

asset_path="$(grep --only-matching --extended-regexp '/assets/[^" ]+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+' "$entry_file" | head -n 1 || true)"
if [ -z "$asset_path" ]; then
  echo "entry document did not reference a versioned asset" >&2
  exit 1
fi
curl --fail --silent --show-error "http://127.0.0.1:${port}${asset_path}" >"$test_root/asset"
curl --fail --silent --show-error --dump-header "$test_root/manifest.headers" "http://127.0.0.1:${port}/manifest.webmanifest" >"$manifest_file"
grep -Fq 'Content-Type: application/manifest+json' "$test_root/manifest.headers"
grep -Fq '"name": "Vibe MUD"' "$manifest_file"
grep -Fq '"start_url": "/"' "$manifest_file"
grep -Fq '"scope": "/"' "$manifest_file"
grep -Fq '"display": "standalone"' "$manifest_file"
while IFS= read -r icon_path; do
  curl --fail --silent --show-error "http://127.0.0.1:${port}${icon_path}" >"$test_root/$(basename "$icon_path")"
done < <(node -e 'const m = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); for (const icon of m.icons) console.log(icon.src)' "$manifest_file")
curl --fail --silent --show-error --dump-header "$test_root/route.headers" "http://127.0.0.1:${port}/play/room" >"$route_file"
grep -Eiq '^Content-Type: text/html' "$test_root/route.headers"
cmp --silent "$entry_file" "$route_file"

if rg --quiet 'serviceWorker|navigator\.serviceWorker' web/dist; then
  echo "built frontend contains Service Worker registration" >&2
  exit 1
fi

runtime_files="$test_root/runtime-files"
base_runtime_files="$test_root/base-runtime-files"
unexpected_runtime_files="$test_root/unexpected-runtime-files"
docker export "$inspect_container_name" | tar --list --file=- | sort --unique >"$runtime_files"
docker export "$base_container_name" | tar --list --file=- | sort --unique >"$base_runtime_files"
grep -Fxq "server" "$runtime_files"
grep -Fxq "web/dist/index.html" "$runtime_files"
grep -Fxq "web/dist${asset_path}" "$runtime_files"

comm -13 "$base_runtime_files" "$runtime_files" | grep --extended-regexp --invert-match \
  '^(server|web/?|web/dist/?|web/dist/.+)$' >"$unexpected_runtime_files" || true
if [ -s "$unexpected_runtime_files" ]; then
  echo "runtime image contains unexpected files:" >&2
  cat "$unexpected_runtime_files" >&2
  exit 1
fi

echo "container test passed: entry and ${asset_path} served from the production image"
