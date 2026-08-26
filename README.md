<!-- last_reviewed: 2026-08-26 -->

# vibe-mud

This repository contains a multiplayer MUD application. One Fly.io process provides the prebuilt React frontend, Google-only SSO, SQLite-backed application sessions, and the JSON game API.

The agreed behavior is indexed in [BEHAVIOR](requirements/BEHAVIOR.md). Game terms are defined in [terminology.md](docs/terminology.md). The complete SQLite structure and lifecycle are documented in [schemas.md](docs/schemas.md). The process entrypoint is [cmd/server/main.go](cmd/server/main.go). The local compiled executable is [server](server).

The application runs as one Go process in Docker. The Docker build compiles the React frontend and copies its static output into the runtime image. Runtime configuration uses `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `FRONTEND_URL`, `DATABASE_PATH`, and `PORT`. Session and OAuth flow cookies always use `Secure`.

The login flow starts at `GET /auth/google/login`, returns through `GET /auth/google/callback`, and exposes the authenticated application identity plus server-calculated current AP at `GET /api/me`. The first game action uses `POST /api/actions/rest`.

AP persistence stores only the timestamp when the player will next reach full AP. The backend derives current AP from its clock, caps it at 3000, and moves the timestamp forward when an action spends AP. No scheduler updates AP values.

Production browsers use the Fly.io application origin for the frontend, `/auth/*`, and `/api/*`. The browser calls relative paths without a proxy. The Go server provides prebuilt static files but keeps authentication, game rules, persistence, and other application logic in backend handlers.

Fly.io runs one `shared-cpu-1x` Machine with 256 MB memory. Create the `mud_data` Volume in the selected region before deployment. The Volume mounts at `/data`, and `DATABASE_PATH` defaults to `/data/mud.db`. Keep the SQLite deployment at one Machine with these commands:

```sh
fly volumes create mud_data --app vibe-mud-api --region nrt --size 1
fly deploy --app vibe-mud-api
fly scale count 1 --app vibe-mud-api
fly machines list --app vibe-mud-api
```

The last command must show exactly one running Machine. Run `fly scale count 1` after each deployment if the count changes.

The frontend source lives under `web/`. The production Docker build runs `npm ci` and `npm run build`, then includes `web/dist` in the Fly.io runtime image. Versioned assets use long-lived immutable browser caching. The frontend entry document requires revalidation after each page load.

Use the Fly.io application origin for both `FRONTEND_URL` and the browser entry URL. Set `GOOGLE_REDIRECT_URL` and the Google Console authorized redirect URI to the same origin plus `/auth/google/callback`. For the current application, use `https://vibe-mud-api.fly.dev` and `https://vibe-mud-api.fly.dev/auth/google/callback`. Run `npm test`, `npm run typecheck`, and `npm run build` from `web/` before `fly deploy`.
