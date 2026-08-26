---
title: "Deployment"
doc_type: runbook
last_reviewed: 2026-08-26
source_paths:
  - Dockerfile
  - fly.toml
  - cmd/server/main.go
  - scripts/test-container.sh
  - scripts/test-fly-origin.sh
  - web/package.json
---

# Deployment

## Runtime Configuration

The runtime requires `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, and `FRONTEND_URL`. `DATABASE_PATH` defaults to `/data/mud.db`, and `PORT` defaults to `8080`.

Use the Fly.io application origin for `FRONTEND_URL`. Set `GOOGLE_REDIRECT_URL` and the Google Console authorized redirect URI to the same origin plus `/auth/google/callback`. The current values are `https://vibe-mud-api.fly.dev` and `https://vibe-mud-api.fly.dev/auth/google/callback`.

## Fly.io Resources

Production uses one `shared-cpu-1x` Machine with 256 MB memory in `nrt`. The `mud_data` Volume mounts at `/data`. Keep exactly one Machine because SQLite concurrency is scoped to one process and one Volume.

Create the Volume once:

```sh
fly volumes create mud_data --app vibe-mud-api --region nrt --size 1
```

## Deployment

Run backend and frontend checks before deployment:

```sh
go test ./...
cd web
npm test
npm run typecheck
npm run build
```

Run the production container check when Docker is available:

```sh
bash scripts/test-container.sh
```

Deploy and confirm one Machine:

```sh
fly deploy --app vibe-mud-api
fly scale count 1 --app vibe-mud-api
fly machines list --app vibe-mud-api
```

## Verification

After changing Fly secrets or deploying, verify that the production origin reaches backend authentication:

```sh
bash scripts/test-fly-origin.sh
```

The anonymous same-origin action must return HTTP 401 with the authentication JSON response. This proves the browser origin reaches the API without failing origin validation.
