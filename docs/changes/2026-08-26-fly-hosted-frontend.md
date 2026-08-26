---
title: "Fly-hosted frontend"
status: Draft
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths: []
req_ref: "REQ-001, REQ-002"
base_branch: main
scope: "Tracks moving the static frontend from Cloudflare Pages to the existing Fly.io application."
---

## Problem Statement

Cloudflare Pages Functions currently proxies browser authentication and API requests to Fly.io. This extra network path produces unstable identity-loading latency and keeps the frontend and backend on different origins.

## Recommended Direction

Build the React frontend inside the Docker build, copy `web/dist` into the runtime image, and let the Go server provide static files beside `/api/*` and `/auth/*`. Serve versioned assets with immutable browser caching and require the entry document to revalidate.

## Key Assumptions

- The existing Fly.io application remains the only runtime application.
- The browser continues to use relative `/api/*` and `/auth/*` URLs.
- The Go server serves prebuilt files and does not render game pages.
- OAuth, session, SQLite, AP, and `rest` behavior remain unchanged.
- Fly.io remains responsible for TLS and process availability.

## Acceptance Criteria

The sources of truth are `REQ-001` and `REQ-002`.

## MVP Scope / Not Doing

- Include the production frontend build, same-origin routing, static asset caching, deployment documentation, and tests.
- Remove the Cloudflare Pages Functions runtime path.
- Exclude server-side rendering, a service worker, a custom domain, CDN integration, and game behavior changes.

## Tasks

## Review Issues
