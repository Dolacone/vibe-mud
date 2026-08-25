<!-- last_reviewed: 2026-08-25 -->

# vibe-mud

This repository contains the API backend for a multiplayer MUD. The first delivery adds Google-only SSO, SQLite-backed application sessions, and a JSON current-user response.

The agreed behavior is documented in [REQ-001](requirements/REQ-001.md) and [BEHAVIOR](requirements/BEHAVIOR.md). The process entrypoint is [cmd/server/main.go](cmd/server/main.go). The local compiled executable is [server](server).

The API runs as one Go process in Docker. Runtime configuration uses `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `FRONTEND_URL`, `DATABASE_PATH`, and `PORT`. Session and OAuth flow cookies always use `Secure`.

The planned login flow starts at `GET /auth/google/login`, returns through `GET /auth/google/callback`, and exposes the authenticated application identity at `GET /api/me`.

Production uses `game.<domain>` for Cloudflare Pages and `api.<domain>` for Fly.io. The frontend calls the API with credentials. The API allows only `FRONTEND_URL`, sets a host-only session cookie for the API origin, and redirects successful Google callbacks to `FRONTEND_URL`.

Fly.io runs one `shared-cpu-1x` Machine with 256 MB memory. Create the `mud_data` Volume in the selected region before deployment. The Volume mounts at `/data`, and `DATABASE_PATH` defaults to `/data/mud.db`. Keep the SQLite deployment at one Machine with these commands:

```sh
fly volumes create mud_data --app vibe-mud-api --region nrt --size 1
fly deploy --app vibe-mud-api
fly scale count 1 --app vibe-mud-api
fly machines list --app vibe-mud-api
```

The last command must show exactly one running Machine. Run `fly scale count 1` after each deployment if the count changes.
