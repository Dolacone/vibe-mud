---
title: "Building durability and repair"
status: Done
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths:
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - docs/schemas.md
  - docs/architecture.md
  - docs/terminology.md
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-011
base_branch: main
scope: "Tracks Building Lv1 durability, disablement, destruction, and shared repair."
---

## Problem Statement

Completed Buildings persist without maintenance pressure. The backend needs time-derived durability and one shared repair Action.

## Recommended Direction

Store a maximum durability snapshot and nullable durability expiry in Unix seconds. Derive Active and Disabled state from backend time. Lazily delete Buildings after the three-day disabled window before state reads, builds, and repairs.

The repair endpoint is `POST /api/actions/repair-building` with the exact body `{"building_id": <positive integer>}`. Every Building response includes `max_durability_seconds`. Under-construction Buildings return `durability_status: null` and `durability_remaining_seconds: null`. Completed Buildings return `durability_status: "active" | "disabled"` and a non-negative integer `durability_remaining_seconds`. Destroyed Buildings are absent.

A successful repair returns HTTP 200 and authoritative player state. Invalid JSON, fields, identifiers, or destroyed targets return HTTP 400. Remote targets, under-construction targets, insufficient AP, and insufficient Wood Resource return HTTP 409 with an error and authoritative player state.

## Key Assumptions

- Durability begins when construction completes.
- Repair consumes the acting player's state.
- Destruction removes the Building row and releases its ownership slot.
- No background scheduler is required for user-visible state.
- Existing completed Buildings receive seven days from migration time.
- Under-construction Buildings have no durability expiry.

## Acceptance Criteria

## MVP Scope / Not Doing

- No extension installation or usage wear.
- No public or private access mode.
- No co-owner, user, or ownership transfer.
- No monster durability multiplier.
- No change to the one-Building ownership limit.

## Tasks

```text
Task 1: persistence and game rules
├── Task 2: backend HTTP contract
│   └── Task 3: frontend API contract
│       └── Task 4: frontend interaction
└── Task 4: frontend interaction
```

- [x] Task 1: Persist and compute Building durability in `internal/authapi/store.go`. Add schema migration, completion initialization, lazy destruction, repair transaction, computation output, and store tests. Update `docs/schemas.md` with the exact migration and backfill in the same commit. This task blocks Tasks 2 and 4.
  - REQ-011.1: `Building Lv1` 完成時必須取得 7 天耐久時間。
  - REQ-011.2: Building 必須隨現實經過時間自然減少剩餘耐久時間。
  - REQ-011.3: 剩餘耐久時間大於 0 時，Building 必須顯示為 Active。
  - REQ-011.5: 剩餘耐久時間到達 0 時，Building 必須顯示為 Disabled。
  - REQ-011.6: Disabled Building 必須保留 3 天，期間仍能維修。
  - REQ-011.7: Building 停用超過 3 天後必須永久消失。
  - REQ-011.8: 永久消失的 Building 不得再顯示或維修。
  - REQ-011.9: Building 永久消失後，不得繼續占用玩家在該 Location 的 Building 名額。
  - REQ-011.10: 同一 Location 的所有玩家都能維修完成的 Building。
  - REQ-011.11: Building 的 owner 或權限設定不能阻止其他玩家維修。
  - REQ-011.12: 每次維修必須消耗維修玩家的 10 AP 與 1 Wood Resource。
  - REQ-011.13: 每次維修最多增加 1 小時耐久時間。
  - REQ-011.14: Active Building 必須從現有剩餘耐久時間增加 1 小時。
  - REQ-011.15: Disabled Building 必須從維修當下重新取得 1 小時耐久時間。
  - REQ-011.16: 維修後的耐久時間不能超過維修當下起算的 7 天。
  - REQ-011.17: 距離 7 天上限不足 1 小時時，維修必須扣除完整成本，並只增加至上限。
  - REQ-011.18: 維修玩家不在 Building 的 Location 時，維修必須失敗。
  - REQ-011.19: AP 或 Wood Resource 不足時，維修必須失敗。
  - REQ-011.20: 維修成功時，AP、Wood Resource 與 Building 耐久時間必須原子更新。
  - REQ-011.21: 維修失敗時，AP、Wood Resource 與 Building 耐久時間必須保持不變。
- [x] Task 2: Expose durability and repair through `internal/authapi/server.go`. Accept only `POST /api/actions/repair-building` with `{"building_id": <positive integer>}`. Return `max_durability_seconds` for every Building. Return nullable durability fields for construction, derived Active or Disabled fields for completed Buildings, and omit destroyed Buildings. Return HTTP 200 for success, HTTP 400 for invalid or missing targets, and HTTP 409 for remote, under-construction, or insufficient-cost failures. Every failure returns an error and authoritative player state. Add strict request validation and server tests. Log repair access with stable user ID, `action=repair-building`, outcome, and request ID. Log successful computation with Building ID, prior durability state, added seconds, resulting remaining seconds, AP cost, and Wood Resource cost. Never log request bodies or sensitive values. This task depends on Task 1 and blocks Task 3.
  - REQ-011.3: 剩餘耐久時間大於 0 時，Building 必須顯示為 Active。
  - REQ-011.4: 玩家必須能看到 Building 的耐久狀態與剩餘耐久時間。
  - REQ-011.5: 剩餘耐久時間到達 0 時，Building 必須顯示為 Disabled。
  - REQ-011.8: 永久消失的 Building 不得再顯示或維修。
  - REQ-011.18: 維修玩家不在 Building 的 Location 時，維修必須失敗。
  - REQ-011.19: AP 或 Wood Resource 不足時，維修必須失敗。
  - REQ-011.21: 維修失敗時，AP、Wood Resource 與 Building 耐久時間必須保持不變。
- [x] Task 3: Add the typed repair request and durability response contract in `web/src/auth.ts`, with parser and request tests. Submit only `{"building_id": <positive integer>}`. Require `max_durability_seconds` on every Building. Require null durability fields during construction and typed Active or Disabled fields after completion. Reject inconsistent combinations and parse authoritative state from HTTP 400 or 409 failures. This task depends on Task 2 and blocks Task 4.
  - REQ-011.4: 玩家必須能看到 Building 的耐久狀態與剩餘耐久時間。
  - REQ-011.22: 前端必須提供維修 Action，並在成功後顯示最新狀態。
- [x] Task 4: Render completed Building durability status and non-negative remaining seconds in `web/src/App.tsx`. Provide a repair control for completed Buildings. Apply authoritative state after success or failure and show the result. Add interaction tests. This task depends on Tasks 1 and 3.
  - REQ-011.3: 剩餘耐久時間大於 0 時，Building 必須顯示為 Active。
  - REQ-011.4: 玩家必須能看到 Building 的耐久狀態與剩餘耐久時間。
  - REQ-011.5: 剩餘耐久時間到達 0 時，Building 必須顯示為 Disabled。
  - REQ-011.22: 前端必須提供維修 Action，並在成功後顯示最新狀態。

## Review Issues

- [x] [Major] `getPlayerStateTx` 會計算每個完成 Building 的耐久狀態與剩餘秒數，但 `/api/me` 與其他回傳玩家狀態的路徑沒有記錄這些計算結果。這違反 repository 對所有 backend computation result 寫入 stdout 的規則。Log 必須包含穩定 user ID、Building ID、計算結果與 request ID，且測試必須覆蓋。
- [x] [Minor] `source_paths` 缺少本次已修改的 `docs/architecture.md` 與 `docs/terminology.md`，因此與 `git diff main...HEAD` 不一致。
- [x] [Minor] `docs/schemas.md` 將 Building recipe seed 記成明確寫入 `max_durability_seconds`，但 `NewStore` 的實際 seed 省略該欄位並依賴 default。文件必須與可執行初始化 SQL 一致。

## Plan Review Issues

- [x] Task 2、3、4 未定義 repair request 與 Building durability response 的精確 JSON contract。計畫必須指定 request body、狀態欄位、剩餘秒數欄位，以及施工中 Building 的欄位規則。
- [x] Task 1 未要求 SQLite schema 變更與 `docs/schemas.md` 在同一 commit 更新。這違反 repository instruction，且目前 schema 文件已在 plan stage 提前修改。
- [x] Task 2 只要求 sanitized logs。計畫必須明定 repair access 與 computation logs 包含穩定 user ID、action、outcome、request ID，以及不含敏感資料的維修結果。
