---
title: "Google SSO Login"
status: Ready-to-review
created: 2026-08-25
doc_type: change
last_reviewed: 2026-08-25
source_paths:
  - AGENTS.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - go.mod
  - go.sum
  - internal/authapi/google.go
  - cmd/server/main.go
  - internal/authapi/google_test.go
  - cmd/server/main_test.go
  - Dockerfile
  - fly.toml
req_ref: REQ-001
base_branch: main
scope: "Tracks the Google-only login API from design through review."
---

## Problem Statement

The backend has no authenticated user boundary. It cannot establish an application session from a verified Google identity or return the current application user as JSON.

## Recommended Direction

Build the first backend as a Go 1.22 HTTP API with chi. Keep Google token exchange inside the API process through `golang.org/x/oauth2` and `coreos/go-oidc`, persist application users and sessions through `database/sql` and `modernc.org/sqlite`, and expose only application identity data.

## Key Assumptions

- The repository has no existing Go source convention. Implementation agents must follow `AGENTS.md`, this change document, and standard Go conventions.
- Google OAuth credentials and public URLs enter through environment variables.
- The production topology uses `game.<domain>` for Cloudflare Pages and `api.<domain>` for Fly.io. The API allows only the configured frontend origin with credentials.
- The browser uses a host-only, `Secure`, `HttpOnly`, `SameSite=Lax` application session cookie from the API origin after Google login.
- Google OAuth attempts persist a hashed state lookup key plus the original nonce, PKCE verifier, and expiration in SQLite. The nonce and verifier remain recoverable only until atomic one-time callback consumption.
- Stable user identity is keyed by the verified OIDC issuer and subject. Email and display name are mutable profile fields.
- Google provider tokens remain transient and are never persisted or returned.
- Fly.io runs one `shared-cpu-1x` Machine with 256 MB memory and mounts a Volume at `/data` for SQLite.

## Acceptance Criteria

The source of truth is `REQ-001`.

## MVP Scope / Not Doing

- Include Google SSO, application sessions, stable user mapping, callback redirection, and current-user JSON.
- Exclude a frontend login page, logout, other identity providers, roles, account deletion, Google API access, and stored Google refresh tokens.

## Tasks

Dependency graph: Task 1 -> Task 2 -> Task 3. Tasks run sequentially.

- [x] Task 1 - Add SQLite identity and session persistence. [parallel: no]
  - Source scope: `internal/authapi/store.go`
  - Tests: `internal/authapi/store_test.go`
  - Supporting files: `go.mod`, `go.sum`
  - Acceptance criteria:
    - REQ-001 condition 7: 同一個 Google 帳號重複登入時，系統必須辨識為同一個應用程式使用者。
  - Test intent: prove issuer-plus-subject identity stability across profile changes, distinguish different subjects, atomically consume OAuth attempts once, reject expired attempts, and reject expired sessions.
- [x] Task 2 - Add the HTTP login and current-user API. [parallel: no]
  - Source scope: `internal/authapi/server.go`
  - Tests: `internal/authapi/server_test.go`
  - Supporting files: `go.mod`, `go.sum`
  - Acceptance criteria:
    - REQ-001 condition 1: 後端只提供 HTTP API，不產生遊戲前端或登入頁面。
    - REQ-001 condition 2: Google 登入所需的導向回應不受純 JSON 回應限制。
    - REQ-001 condition 4: 使用者可以從未登入狀態開始 Google 登入，並在 Google 驗證成功後建立應用程式 session。
    - REQ-001 condition 6: Google 登入完成後，後端必須將使用者導向已設定的前端位置。
    - REQ-001 condition 8: 已登入使用者可以透過 JSON 回應取得自己的應用程式使用者 ID、顯示名稱與電子郵件。
    - REQ-001 condition 9: 沒有有效 session 的使用者必須收到 HTTP 401 JSON 回應。
    - REQ-001 condition 10: 回應內容不能包含 Google authorization code、access token、refresh token、ID token、client secret 或 session secret。
  - Test intent: prove redirects are the only non-JSON responses, state replay and expiry fail before provider exchange, successful callbacks set the constrained cookie and redirect, credentialed CORS allows only the configured frontend, and identity JSON contains no provider or session secrets.
- [x] Task 3 - Connect Google OIDC and package the Fly.io API process. [parallel: no]
  - Source scope: `internal/authapi/google.go`, `cmd/server/main.go`
  - Tests: `internal/authapi/google_test.go`, `cmd/server/main_test.go`
  - Supporting files: `go.mod`, `go.sum`, `.gitignore`, `Dockerfile`, `fly.toml`
  - Acceptance criteria:
    - REQ-001 condition 3: 使用者只能透過 Google SSO 登入，不提供本機密碼或其他身分提供者。
    - REQ-001 condition 5: 後端必須驗證 Google 登入結果，驗證成功後才能建立 session。
    - REQ-001 condition 10: 回應內容不能包含 Google authorization code、access token、refresh token、ID token、client secret 或 session secret。
  - Test intent: prove PKCE and nonce enter the Google request, reject missing or invalid verified ID tokens, and reject incomplete runtime configuration before the server starts.
  - Deployment intent: build a CGo-free image, run one 256 MB shared Machine, mount `mud_data` at `/data`, and document volume creation without deploying from this change.

## Review Issues

## Plan Review Issues

- [x] Task 1 cannot satisfy REQ-001 conditions 4 or 10 within `store.go`: it neither completes a Google login nor emits an HTTP response. Task 2 has the same end-to-end mismatch for condition 4 before the Google adapter exists. Reassign criteria to independently verifiable task outcomes or restructure the tasks as vertical slices.
- [x] The plan says state, nonce, and PKCE are one-time and short-lived, but no task defines where attempts live, how they are consumed atomically, or which tests prove replay and expiry rejection. Add this lifecycle and its test intent to the task that owns REQ-001 condition 5.
- [x] Stable Google-user mapping lacks an identity-key decision. Specify mapping by the verified OIDC issuer and subject rather than mutable profile fields such as email, then make Task 1 tests prove REQ-001 condition 7.
- [x] The frontend and API deploy on different origins, but the plan defines neither credentialed CORS nor the cookie origin, `SameSite`, `Secure`, and redirect topology. Resolve this architecture so the Cloudflare Pages frontend can call `GET /api/me` without weakening the session boundary, and document the required public URLs.
- [x] Task 3 packages Docker only and omits the required single-Machine Fly.io deployment with a mounted Fly Volume for SQLite. Add the Fly configuration, persistent database path, deployment dependency, and README setup steps, or narrow the task title and record a separate required deployment task.
- [x] The OAuth-attempt design persists a hashed PKCE verifier, but the callback must recover the original verifier for the token exchange. Persist the verifier in a recoverable protected form until atomic consumption; continue hashing state for lookup and nonce for claim comparison.
- [x] Dependency-manifest scope is incomplete. Task 2 introduces chi and Task 3 introduces `golang.org/x/oauth2` plus `coreos/go-oidc`, but only Task 1 lists `go.mod` and `go.sum`. List those supporting-file updates under every task that changes them, or state that Task 1 declares all planned dependencies and explain how its commit preserves dependencies not imported until later tasks.
- [x] Task 1 cannot run its required Go tests while `go.mod` and `go.sum` belong to Task 3. Move module initialization and the SQLite dependency to Task 1, then let later tasks extend the module dependencies.
