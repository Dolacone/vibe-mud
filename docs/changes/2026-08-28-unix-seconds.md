---
title: "Unix seconds storage"
status: Ready-to-implement
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths:
  - CHANGELOG.md
  - docs/architecture.md
  - docs/schemas.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server_test.go
req_ref: "REQ-001, REQ-003"
base_branch: main
scope: "Tracks the internal timestamp precision change while preserving authentication and AP behavior."
---

## Problem Statement

Persistent timestamps use Unix nanoseconds despite game and authentication behavior needing only second precision. The storage change must not create a new user-facing requirement.

## Recommended Direction

Store absolute times as UTC Unix seconds. Convert legacy nanoseconds during Store initialization. Preserve the existing authentication and AP behavior from REQ-001 and REQ-003.

## Key Assumptions

- Subsecond precision has no game or authentication consumer.
- Legacy timestamps contain contemporary positive UTC values.
- Internal migration telemetry belongs in implementation review, not requirements.

## Acceptance Criteria

## MVP Scope / Not Doing

- Keep all user-facing behavior unchanged.
- Do not expose timestamps through the public API.
- Do not add a schema version table.
- Do not add Building durability in this change.

## Tasks

### Dependency graph

```text
Task 1: Timestamp storage, migration, regression tests, and documentation
```

### Task 1: Timestamp storage, migration, regression tests, and documentation

- [ ] Replace new timestamp persistence and decoding with UTC Unix seconds in `internal/authapi/store.go`.
- [ ] Convert every legacy absolute timestamp column from nanoseconds to seconds during Store initialization.
- [ ] Keep migration idempotent for values already stored as seconds.
- [ ] Log the number of converted timestamp values as `converted_values`.
- [ ] Cover new writes, migration, idempotency, AP boundaries, session expiry, and OAuth expiry with tests.
- [ ] Align architecture, schema, and changelog documentation with the implementation.

Acceptance criteria:

- REQ-001.7: 使用者可以從未登入狀態開始 Google 登入，並在 Google 驗證成功後建立應用程式 session。
- REQ-001.8: 後端必須驗證 Google 登入結果，驗證成功後才能建立 session。
- REQ-001.10: 同一個 Google 帳號重複登入時，系統必須辨識為同一個應用程式使用者。
- REQ-001.11: 已登入使用者可以透過 JSON 回應取得自己的應用程式使用者 ID、顯示名稱與電子郵件。
- REQ-001.12: 沒有有效 session 的使用者必須收到 HTTP 401 JSON 回應。
- REQ-003.1: 新玩家首次登入時擁有 3000 AP。
- REQ-003.2: 玩家每經過完整一分鐘恢復 1 AP。
- REQ-003.3: 未滿一分鐘的時間不恢復 AP。
- REQ-003.4: 玩家登入時，系統依目前時間計算並回傳目前 AP。
- REQ-003.5: 目前 AP 不得低於 0，也不得超過 3000。
- REQ-003.6: 玩家超過恢復至滿值所需的時間後，目前 AP 仍為 3000。

## Plan Review Issues

- [x] [Major] Add REQ-001.11 verbatim to Task 1 acceptance criteria. The timestamp change modifies session decoding, but the current criteria never require a migrated valid session to return the user's application identity.

## Review Issues
