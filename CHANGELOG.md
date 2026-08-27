<!-- last_reviewed: 2026-08-27 -->

# Changelog

## 2026-08-27

- Replaced the generic Resource balance with separate Food, Wood, Stone, Metal, Fiber, Hide, Medicinal, and Arcane quantities.
- Updated conversion to produce Wood Resource while preserving existing Action behavior.

## 2026-08-26

- Moved the production frontend into the Fly.io application for same-origin API and OAuth access.
- Replaced the Cloudflare Pages Functions proxy with browser-cached static files served by Go.
- Added persistent player locations, backend-owned Routes, and AP-consuming movement.
- Added strict Action and target validation with safe rejection logging.
- Added location-specific gathering that atomically consumes AP and collects Wood.
- Added persistent Inventory state and frontend display.
- Added persistent Resource conversion that atomically consumes AP and Wood.

## 2026-08-25

- Added Google-only SSO and SQLite-backed application sessions.
- Added the authenticated current-user JSON API.
- Added single-Machine Fly.io packaging with persistent SQLite storage.
- Added the Cloudflare Pages login interface and backend-confirmed identity display.
- Added the allow-listed Pages Functions proxy for same-origin authentication.
- Added timestamp-derived AP with a 3000 cap and one AP restored per complete minute.
- Added the authenticated `rest` action across the API and Cloudflare Pages interface.
