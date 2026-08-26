<!-- last_reviewed: 2026-08-26 -->

# Changelog

## 2026-08-26

- Moved the production frontend into the Fly.io application for same-origin API and OAuth access.
- Replaced the Cloudflare Pages Functions proxy with browser-cached static files served by Go.
- Added persistent player locations, backend-owned Routes, and AP-consuming movement.
- Added strict Action and target validation with safe rejection logging.

## 2026-08-25

- Added Google-only SSO and SQLite-backed application sessions.
- Added the authenticated current-user JSON API.
- Added single-Machine Fly.io packaging with persistent SQLite storage.
- Added the Cloudflare Pages login interface and backend-confirmed identity display.
- Added the allow-listed Pages Functions proxy for same-origin authentication.
- Added timestamp-derived AP with a 3000 cap and one AP restored per complete minute.
- Added the authenticated `rest` action across the API and Cloudflare Pages interface.
