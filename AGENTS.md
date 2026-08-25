<!-- last_reviewed: 2026-08-25 -->

# Repository Instructions

- Build the backend with Go, chi, `database/sql`, `modernc.org/sqlite`, and `coder/websocket`.
- Build the frontend with React, TypeScript, and Vite.
- Deploy the backend as one Docker container on a single Fly.io Machine with a Fly Volume.
- Deploy the static frontend to Cloudflare Pages.
- Do not use Cloudflare Workers for the MVP.
- Use REST for initial state and game actions.
- Use WebSocket for chat and real-time event delivery.
- Persist application users, OAuth attempts, and sessions in SQLite. Do not replace SQLite with stateless cookies or another persistence method without explicit user approval.
