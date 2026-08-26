<!-- last_reviewed: 2026-08-26 -->

# Repository Instructions

- Build the backend with Go, chi, `database/sql`, `modernc.org/sqlite`, and `coder/websocket`.
- Build the frontend with React, TypeScript, and Vite.
- Deploy the backend and prebuilt static frontend as one Docker container on a single Fly.io Machine with a Fly Volume.
- Serve `/api/*`, `/auth/*`, and the static frontend from the same Fly.io origin. Do not use Cloudflare Pages or Pages Functions in the runtime path.
- Use REST for initial state and game actions.
- Use WebSocket for chat and real-time event delivery.
- Persist application users, OAuth attempts, and sessions in SQLite. Do not replace SQLite with stateless cookies or another persistence method without explicit user approval.
- Update `docs/schemas.md` in the same commit as any SQLite table, column, index, constraint, initialization, or backfill change.
- Update `docs/terminology.md` in the same commit as any game term addition, rename, or definition change.
- Keep the root `README.md` limited to the player-facing game introduction. Put architecture, API, setup, and deployment content under `docs/`.
- Write every backend access event and backend computation result to standard output for Fly.io log collection. Include the stable application user ID, action, outcome, and request ID when available; use an explicit anonymous value before authentication. Never log credentials, OAuth codes or tokens, session tokens, cookies, secrets, or raw sensitive values.
