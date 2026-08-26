---
title: "Fly-hosted frontend"
status: Done
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths:
  - AGENTS.md
  - CHANGELOG.md
  - README.md
  - docs/changes/2026-08-25-ap-rest.md
  - docs/changes/2026-08-25-frontend-login.md
  - docs/schemas.md
  - docs/terminology.md
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - cmd/server/main.go
  - cmd/server/main_test.go
  - cmd/server/static.go
  - cmd/server/static_test.go
  - Dockerfile
  - .dockerignore
  - scripts/test-container.sh
  - web/package.json
  - web/package-lock.json
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
- The production frontend URL is `https://vibe-mud-api.fly.dev` until a custom domain replaces it.
- `FRONTEND_URL` uses the Fly.io application origin. `GOOGLE_REDIRECT_URL` and the Google Console callback use that origin plus `/auth/google/callback`.
- Vite production assets use the `name-[A-Za-z0-9_-]{8,}.ext` filename form as their version identifier. Files outside that form must not receive immutable caching.

## Acceptance Criteria

The sources of truth are `REQ-001` and `REQ-002`.

## MVP Scope / Not Doing

- Include the production frontend build, same-origin routing, static asset caching, deployment documentation, and tests.
- Remove the Cloudflare Pages Functions runtime path.
- Exclude server-side rendering, a service worker, a custom domain, CDN integration, and game behavior changes.

## Tasks

Dependency graph: Task 1 -> Task 2 -> Task 3. These tasks run sequentially because each task changes the runtime boundary consumed by the next task.

- [x] Task 1 - Allow the API server middleware to wrap an injected frontend fallback. [parallel: no]
  - Source scope: `internal/authapi/server.go`
  - Tests: `internal/authapi/server_test.go`
  - Acceptance criteria:
    - REQ-001 condition 3: `/api/*` 必須提供 JSON API，`/auth/*` 必須提供登入流程，其他遊戲前端路徑必須提供靜態前端內容。
    - REQ-001 condition 5: Google 登入所需的導向回應不受純 JSON 回應限制。
  - Test intent: prove known API and OAuth routes preserve their JSON or redirect contracts, reserved unknown API and OAuth paths preserve JSON 404 responses, frontend paths reach the injected fallback, and the shared request ID plus access log middleware covers both route groups without logging credentials.
- [x] Task 2 - Serve the built frontend from the Go process with browser cache controls. [parallel: no]
  - Source scope: `cmd/server/main.go`, `cmd/server/static.go`
  - Tests: `cmd/server/main_test.go`, `cmd/server/static_test.go`, `web/src/App.test.tsx`, `web/src/auth.test.ts`
  - Acceptance criteria:
    - REQ-001 condition 1: Fly.io 上的單一 application 必須執行後端 HTTP server，並提供預先建置的遊戲前端靜態檔案。
    - REQ-001 condition 2: 前端與後端 API 必須使用相同 origin。
    - REQ-001 condition 3: `/api/*` 必須提供 JSON API，`/auth/*` 必須提供登入流程，其他遊戲前端路徑必須提供靜態前端內容。
    - REQ-001 condition 4: 後端不得動態產生遊戲頁面。
    - REQ-002 condition 1: 使用者必須能從 Fly.io application 提供的遊戲前端開始 Google 登入。
    - REQ-002 condition 2: 瀏覽器必須透過相同 origin 直接呼叫後端 API 與登入路徑。
    - REQ-002 condition 4: Google 登入成功後，使用者必須回到相同 origin 的遊戲前端。
    - REQ-002 condition 8: 帶有版本識別的前端靜態資源必須允許瀏覽器長期快取，且版本未變時不得重新下載內容。
    - REQ-002 condition 9: 前端入口文件必須允許瀏覽器重新驗證版本，避免發布後持續使用舊版資源。
  - Runtime configuration: require `FRONTEND_URL` to contain only the Fly application origin. Require `GOOGLE_REDIRECT_URL` to use that exact origin and the exact `/auth/google/callback` path without query or fragment. Reject startup when these values differ.
  - Test intent: prove the server reads only prebuilt files, serves the entry document at `/` and client routes, gives only assets matching `name-[A-Za-z0-9_-]{8,}.ext` the `public, max-age=31536000, immutable` policy, prevents immutable caching for unversioned files, serves the entry document with `no-cache`, revalidates unchanged files, and never sends frontend content for reserved API or OAuth paths. Prove the frontend exposes relative `/auth/google/login` and calls relative `/api/*`. Prove startup rejects a different frontend origin, callback origin, callback path, callback query, or callback fragment.
- [x] Task 3 - Build the frontend into the Fly.io image and remove the Cloudflare runtime path. [parallel: no]
  - Source scope: the retired Cloudflare Pages proxy and `scripts/test-container.sh`
  - Tests: `scripts/test-container.sh`; the proxy test is removed with the retired proxy. Run the Go suite, frontend suite, frontend typecheck, frontend production build, and Docker production build.
  - Supporting files: `Dockerfile`, `.dockerignore`, `web/package.json`, `web/package-lock.json`, the retired Wrangler configuration, `AGENTS.md`, `README.md`
  - Acceptance criteria:
    - REQ-001 condition 1: Fly.io 上的單一 application 必須執行後端 HTTP server，並提供預先建置的遊戲前端靜態檔案。
    - REQ-002 condition 3: 前端聯絡後端時不得依賴 Cloudflare Pages 或 Cloudflare Pages Functions proxy。
  - Build intent: use `.dockerignore` to exclude local `server`, `web/node_modules`, `web/dist`, and Wrangler output from the context. Prove a clean Docker build installs locked frontend dependencies, emits `web/dist`, compiles the Go server, and copies only the server plus built frontend into the runtime image. The container test must start the production image, request the entry document and one versioned asset, inspect the runtime filesystem, and prove Node.js, source files, local build output, and Cloudflare runtime dependencies are absent.

## Review Issues

無。

## Review Verification

- `go test ./...` 因 macOS linker 的 `dyld: missing LC_UUID` 環境問題失敗。
- `go test -ldflags=-linkmode=external ./...` 通過。
- `npm test`、`npm run typecheck` 與 `npm run build` 通過。
- `scripts/test-container.sh` 通過。Production image 可提供入口文件與版本化資源，且 runtime 未包含 Node.js、前端原始碼或 Cloudflare runtime 依賴。

## Plan Review Issues

- [x] Task 2 將 REQ-001 condition 2 與 REQ-002 conditions 2、4 列為驗收條件，但計畫只記錄同源設定值，沒有要求 `loadConfig` 拒絕不同 origin 的 `FRONTEND_URL` 與 `GOOGLE_REDIRECT_URL`。目前測試甚至使用兩個不同 origin。加入執行期設定驗證及測試，並要求 callback URL 為相同 origin 的 `/auth/google/callback`。否則部署仍可讓 API、登入回呼與登入後導向分屬不同 origin。
- [x] Task 2 將 REQ-002 conditions 1、2 列為驗收條件，但所列 Go 測試只能證明靜態檔案與後端路由可用。它們無法證明前端提供相對 `/auth/google/login`，或以相對 `/api/*` 聯絡後端。把 `web/src/App.test.tsx` 與 `web/src/auth.test.ts` 的既有契約納入此任務的測試範圍，或把條件與前端測試移到同一任務。
- [x] Task 2 的快取測試只要求所有 asset bytes 取得 `immutable`，沒有區分帶版本識別與未帶版本識別的資源。REQ-002 condition 8 只允許帶版本識別的資源使用長期 immutable 快取。定義可判定的版本命名規則，並測試版本化資源取得 immutable、未版本化資源不會取得 immutable。
- [x] Task 3 沒有對應測試。刪除 proxy test、執行既有套件與完成 Docker build，都無法測出 runtime image 漏拷 `web/dist`、路徑錯誤或缺少啟動檔。加入自動化容器整合測試，驗證 image 只含所需 runtime 內容，並能從 image 內的預建檔案提供入口與版本化資源。
- [x] Task 3 未包含 `.dockerignore`。目前 Docker context 會送入被 Git 忽略的 `web/node_modules`、`web/dist`、Wrangler 暫存檔與本機 `server`，因此「clean Docker build installs locked frontend dependencies」不成立。加入 `.dockerignore`，並讓容器測試證明 build 不依賴這些本機產物。
