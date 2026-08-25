<!-- last_reviewed: 2026-08-25 -->

# Repository Instructions

- Build the backend with Go, chi, `database/sql`, `modernc.org/sqlite`, and `coder/websocket`.
- Build the frontend with React, TypeScript, and Vite.
- Deploy the backend as one Docker container on a single Fly.io Machine with a Fly Volume.
- Deploy the static frontend to Cloudflare Pages.
- Use Cloudflare Pages Functions only as the same-origin proxy for `/auth/*` and `/api/*`. Keep authentication, game rules, persistence, and other application logic in the Fly.io backend.
- Use REST for initial state and game actions.
- Use WebSocket for chat and real-time event delivery.
- Persist application users, OAuth attempts, and sessions in SQLite. Do not replace SQLite with stateless cookies or another persistence method without explicit user approval.
- Update `docs/schemas.md` in the same commit as any SQLite table, column, index, constraint, initialization, or backfill change.
- Write every backend access event and backend computation result to standard output for Fly.io log collection. Include the stable application user ID, action, outcome, and request ID when available; use an explicit anonymous value before authentication. Never log credentials, OAuth codes or tokens, session tokens, cookies, secrets, or raw sensitive values.
