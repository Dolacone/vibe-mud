---
title: "Frontend Login"
status: Done
created: 2026-08-25
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - AGENTS.md
  - README.md
  - .gitignore
  - web/package.json
  - web/package-lock.json
  - web/tsconfig.json
  - web/tsconfig.app.json
  - web/tsconfig.node.json
  - web/vite.config.ts
  - web/src/vite-env.d.ts
  - web/src/test-setup.ts
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
  - web/src/main.tsx
  - web/src/styles.css
  - web/index.html
req_ref: REQ-002
base_branch: main
scope: "Tracks the Cloudflare Pages frontend login from design through review."
---

## Problem Statement

The deployed backend can establish an application session, but users have no Cloudflare-hosted interface that starts login or asks the backend which application user owns the current session.

## Recommended Direction

Build a single-page React 19, TypeScript, and Vite frontend under `web/`. Use a narrowly allow-listed Cloudflare Pages Function as a same-origin proxy to the Fly.io backend. Start login through `/auth/google/login` and treat `/api/me` as the only authenticated identity source.

## Key Assumptions

- The backend remains a separate Fly.io API and continues to own Google OAuth and application sessions.
- The browser calls relative `/auth/*` and `/api/*` URLs on the Pages origin. It never receives the Fly backend origin or credentials.
- The Pages Function reads server-side `BACKEND_ORIGIN`. It accepts only an absolute HTTPS origin with no credentials, path, query, or fragment, and proxies only `GET` or `OPTIONS` requests whose parsed and normalized pathname remains under `/auth/` or `/api/`.
- Allow-listed request queries are forwarded unchanged to the same backend path because the OAuth callback requires `state` and `code`. The proxy never logs query values.
- Upstream fetches use manual redirect handling. The proxy forwards request cookies and preserves every upstream `Set-Cookie` header, redirect, status, and response header. All authentication and identity decisions remain in the backend.
- The identity client treats HTTP 200 as authenticated, HTTP 401 as unauthenticated, and other failures as errors. It never caches identity in browser storage.
- Cloudflare Pages serves static assets. Pages Functions provide only the same-origin proxy and contain no authentication or game logic.
- Cloudflare Pages uses `web/` as its root directory, `npm run build` as its build command, and `dist` as its output directory.
- `BACKEND_ORIGIN` is the origin-only Fly URL. Fly `FRONTEND_URL` is the exact Pages origin. Fly `GOOGLE_REDIRECT_URL` and the Google Console authorized redirect URI are the exact Pages `/auth/google/callback` URL.
- Host-only OAuth flow and session cookies returned through the proxy belong to the Pages origin. The login callback returns through the Pages proxy before the backend redirects to the frontend root.
- Deployment uses one stable Pages project origin such as `https://<project>.pages.dev`, not an immutable per-deployment preview URL. Every browser entry, callback, cookie, redirect, and API request uses that origin.

## Acceptance Criteria

The source of truth is `REQ-002`.

## MVP Scope / Not Doing

- Include a login action, authenticated identity display, unauthenticated state, and backend failure state.
- Exclude logout, routing, game state, chat, WebSocket, account settings, and a component framework.

## Tasks

Dependency graph: Task 1 -> Task 2 -> Task 3 -> Task 4. Tasks run sequentially. Task 4 waits for the user to connect the GitHub repository to Cloudflare Pages after Tasks 1-3 finish.

- [x] Task 1 - Add the typed backend identity client and frontend foundation. [parallel: no]
  - Source scope: `web/src/auth.ts`
  - Tests: `web/src/auth.test.ts`
  - Supporting files: `web/package.json`, `web/package-lock.json`, `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/src/vite-env.d.ts`, `web/src/test-setup.ts`
  - Acceptance criteria:
    - REQ-002 condition 3: 前端必須向後端查詢目前登入的應用程式使用者，不得從前端狀態或 Google 回應自行推定身分。
  - Test intent: prove the client sends same-origin `GET /api/me` with credentials, returns only backend identity fields for HTTP 200, treats HTTP 401 as unauthenticated, rejects other failures, and does not use browser storage or a Google client SDK.
- [x] Task 2 - Add the login page and static Pages build. [parallel: no]
  - Source scope: `web/src/App.tsx`, `web/src/main.tsx`
  - Tests: `web/src/App.test.tsx`
  - Supporting files: `web/index.html`, `web/src/styles.css`, `.gitignore`, `web/package.json`, `web/package-lock.json`
  - Acceptance criteria:
    - REQ-002 condition 4: 有效 session 存在時，前端必須顯示後端回傳的應用程式使用者 ID、顯示名稱與電子郵件。
    - REQ-002 condition 5: 有效 session 不存在時，前端必須顯示未登入狀態與登入操作，不得顯示先前取得的使用者身分。
  - Test intent: prove the page loads identity through the same-origin client, exposes `/auth/google/login`, renders only current backend identity, removes prior identity when unauthenticated, and distinguishes backend errors from signed-out state.
  - Build intent: prove the production build emits static assets to `web/dist` without embedding the Fly origin or any credential.
- [x] Task 3 - Add the allow-listed Pages Function proxy. [parallel: no]
  - Source scope: the Cloudflare Pages proxy
  - Tests: the Cloudflare Pages proxy test
  - Supporting files: the Pages configuration, `web/package.json`, `web/package-lock.json`, `README.md`
  - Acceptance criteria:
    - REQ-002 condition 2: Google 登入成功後，使用者必須回到前端。
    - REQ-002 condition 3: 前端必須向後端查詢目前登入的應用程式使用者，不得從前端狀態或 Google 回應自行推定身分。
  - Test intent: prove only normalized `/auth/*` and `/api/*` paths reach an origin-only HTTPS backend; reject unsupported methods, configured origin credentials, paths, queries, fragments, encoded traversal, and paths that normalize outside the allow-list; forward allow-listed request queries and cookies without logging their values; and pass unrelated paths to static assets.
  - Integration intent: prove upstream fetch uses manual redirect handling, the proxied login endpoint preserves the external Google redirect, and the proxied callback preserves both the OAuth flow-cookie deletion and session `Set-Cookie` headers plus the backend redirect to the Pages origin.
- [x] Task 4 - Deploy and verify the stable Cloudflare Pages origin. [parallel: no]
  - Source scope: none
  - Tests: live browser login and deployed endpoint verification
  - Supporting files: `docs/changes/2026-08-25-frontend-login.md`
  - Acceptance criteria:
    - REQ-002 condition 1: 使用者必須能從部署在 Cloudflare Pages 的前端開始 Google 登入。
    - REQ-002 condition 2: Google 登入成功後，使用者必須回到前端。
  - Runtime settings: select one stable Pages project origin such as `https://<project>.pages.dev`; set Pages `BACKEND_ORIGIN` to the origin-only Fly URL; set Fly `FRONTEND_URL` to that exact Pages origin; set Fly `GOOGLE_REDIRECT_URL` and the Google Console authorized redirect URI to that origin plus `/auth/google/callback`.
  - Verification intent: after the user connects GitHub to Cloudflare Pages, verify the deployed values and record the stable Pages project URL plus durable evidence that the browser enters through that origin, starts Google login, returns through its callback, stores both host-only cookies for that host, and displays the backend-confirmed user from same-origin `/api/me`.
  - Live verification evidence: `https://vibe-mud.pages.dev/` returned HTTP 200. Same-origin `/api/me` returned HTTP 401 JSON before authentication. Same-origin `/auth/google/login` returned HTTP 302 to Google through the Pages proxy. Pages `BACKEND_ORIGIN` resolved to the Fly API origin. After Fly and Google callback settings were updated, a real browser completed Google login, returned through the Pages callback, and displayed the backend-confirmed numeric application user ID, display name, and email. No credential, token, cookie value, or personal value is recorded here.

## Review Issues

- [x] [Major] Live deployment returned a numeric JSON user ID from the backend, but the frontend identity validator accepted only string IDs. The frontend now accepts the existing numeric ID contract and rejects string IDs.
- [x] [Minor] `source_paths` omitted `.gitignore`, although Task 2 changed it to exclude frontend dependency and build directories.

## Plan Review Issues

- [x] The planned default Cloudflare Pages URL and Fly.io URL are cross-site. The existing `SameSite=Lax` API cookie will not accompany a credentialed cross-site `fetch`, so `GET /api/me` cannot identify the user on `*.pages.dev` -> `*.fly.dev`. Require sibling custom domains such as `game.<domain>` and `api.<domain>` in the plan and deployment instructions, or return to capture before changing the agreed cookie boundary. Add production-domain verification of credentialed `/api/me`.
- [x] REQ-002 condition 1 says the frontend is deployed on Cloudflare Pages, but Task 2 only builds static files and documents Git integration. Add an explicit deployment and verification step with durable evidence, or return to capture and change the criterion to deployable rather than deployed.
- [x] REQ-002 condition 2 is assigned to a frontend-only task, but its test intent proves only that the login URL is exposed. Trace this condition through the existing backend callback contract, the deployed `FRONTEND_URL`, and a verification that login returns to the Cloudflare frontend.
- [x] `VITE_API_BASE_URL` validation is underspecified. Define it as an absolute API origin with no credentials, path, query, or fragment; require HTTPS in production while allowing an explicit local-development origin. Document that Vite exposes this value publicly and that it must never contain credentials.
- [x] `README.md` still defines the production browser flow as credentialed requests from `game.<domain>` directly to `api.<domain>`, while the new plan defines relative same-origin requests through Pages Functions. Replace the obsolete topology instead of leaving both as active instructions, and state that the browser-owned host-only cookies now belong to the Pages origin.
- [x] Task 4 does not spell out the three distinct runtime values required for the proxied callback: `BACKEND_ORIGIN` must be the origin-only Fly URL, Fly `FRONTEND_URL` must be the exact Pages origin, and both Fly `GOOGLE_REDIRECT_URL` plus the Google Console authorized redirect URI must be the exact Pages `/auth/google/callback` URL. Add these settings and verify the deployed values; setting an OAuth callback to an origin, as the current README says, breaks login.
- [x] The proxy boundary does not fully define `BACKEND_ORIGIN` or path normalization. Require an HTTPS origin with no credentials, path, query, or fragment, and verify the final upstream pathname remains under `/auth/` or `/api/` after URL parsing. Add encoded traversal and origin-component tests so an allowed-looking request or configured value cannot escape the allow-list.
- [x] Task 3 says to reject queries without limiting that rule to `BACKEND_ORIGIN`, but the Google callback requires the request's `state` and `code` query parameters. State explicitly that configured origins with a query are invalid while allow-listed request queries are forwarded to the same backend path and never logged. Add a callback-query test.
- [x] The proxy plan does not cover the two response mechanics that the existing backend requires: upstream OAuth redirects must use manual redirect handling, and the callback's flow-cookie deletion plus session cookie must remain two `Set-Cookie` headers. Add tests for the external login redirect and both callback cookies instead of testing one generic redirect or cookie header.
- [x] Task 4 calls the deployment target a preview URL while README uses the production `<project>.pages.dev` origin. Select one stable Pages origin or branch alias and require the browser entry URL, Fly `FRONTEND_URL`, Google callback, OAuth flow cookie host, session cookie host, and `/api/me` request to use that same origin. Do not use an immutable per-deployment preview URL that changes after each build.
