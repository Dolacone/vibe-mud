---
title: "Gathering and inventory"
status: Ready-to-review
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths:
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-006
base_branch: main
scope: "Tracks the first location-specific item collection loop."
---

## Problem Statement

Movement changes the player's location, but locations do not provide a persistent item-producing action. Players also have no Inventory for collected items.

## Recommended Direction

Add a backend-owned `gather` rule for `forest_edge`, consume AP and add `Wood` in one SQLite transaction, expose Inventory through the existing player state, and render the updated state in the frontend.

## Key Assumptions

- `gather` is available only at `forest_edge`.
- Each successful `gather` costs 10 AP.
- Each successful `gather` yields exactly one `Wood` item.
- The backend owns the allowed location, item, quantity, and AP cost.
- Inventory stores item quantities and persists across sessions.
- The only valid `gather` request payload is an empty JSON object: `{}`.

## Acceptance Criteria

The source of truth is `REQ-006`.

## MVP Scope / Not Doing

- Include one gather location, one deterministic item yield, persistent Inventory, atomic AP consumption, strict input rejection, safe error logging, and frontend Inventory display.
- Exclude tools, skills, equipment, random yield, resource conversion, item use, trading, crafting, capacity, and item loss.

## Tasks

```text
Task 1: SQLite gathering and Inventory state
  -> Task 2: Gathering API
    -> Task 3: Frontend gathering client
      -> Task 4: Gathering and Inventory UI
```

所有 task 依序執行。沒有可平行執行的 task。

### Task 1: Store gathering and Inventory state atomically

- [x] 在 `internal/authapi/store.go` 建立 item、gathering rule 與 Inventory schema，並加入 `Wood` 與 `forest_edge` gathering rule seed。
- [x] 將 Inventory 與目前位置可用的 gathering option 納入玩家狀態。
- [x] 實作原子化驗證位置、扣除 AP、累加 Inventory quantity 的 gather operation。
- [x] 在 `internal/authapi/store_test.go` 驗證空 Inventory、成功累加、位置錯誤、AP 不足、持久化與 rollback。

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-006.2: `gather` 成功時，必須消耗 10 AP，並取得 1 個 `Wood` item。
- REQ-006.3: `gather` 取得的 `Wood` 必須加入玩家的持久化 Inventory，現有 quantity 必須增加 1。
- REQ-006.4: 後端必須依玩家目前位置決定 gather 的 item、quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-006.5: 玩家不在 `forest_edge` 時，`gather` 必須失敗，且 AP、位置與 Inventory 都保持不變。
- REQ-006.6: 玩家 AP 少於 10 時，`gather` 必須失敗，且 AP、位置與 Inventory 都保持不變。
- REQ-006.7: AP 扣除與 Inventory quantity 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-006.8: 尚未取得任何 item 的玩家必須擁有空 Inventory。
- REQ-006.9: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory item 與 quantity。
- REQ-006.11: 玩家重複成功執行 `gather` 時，系統必須累加 `Wood` quantity，不能覆寫既有 quantity。

### Task 2: Add gathering API and safe rejection logging

- [x] 在 `internal/authapi/server.go` 擴充 `GET /api/me`，並新增 `POST /api/actions/gather`。
- [x] `gather` request 只接受 `{}`，並拒絕 malformed JSON、unknown fields、duplicate fields 與 trailing JSON values。
- [x] 對位置錯誤、AP 不足與輸入錯誤回傳後端權威狀態，並寫出安全 outcome log。
- [x] 在 `internal/authapi/server_test.go` 驗證成功、失敗、驗證、狀態不變與 log 欄位。

Source files: `internal/authapi/server.go`

Acceptance criteria:

- REQ-006.1: 已登入玩家位於 `forest_edge` 時，必須能透過前端執行 `gather` Action。
- REQ-006.2: `gather` 成功時，必須消耗 10 AP，並取得 1 個 `Wood` item。
- REQ-006.3: `gather` 取得的 `Wood` 必須加入玩家的持久化 Inventory，現有 quantity 必須增加 1。
- REQ-006.4: 後端必須依玩家目前位置決定 gather 的 item、quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-006.5: 玩家不在 `forest_edge` 時，`gather` 必須失敗，且 AP、位置與 Inventory 都保持不變。
- REQ-006.6: 玩家 AP 少於 10 時，`gather` 必須失敗，且 AP、位置與 Inventory 都保持不變。
- REQ-006.7: AP 扣除與 Inventory quantity 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-006.8: 尚未取得任何 item 的玩家必須擁有空 Inventory。
- REQ-006.9: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory item 與 quantity。
- REQ-006.10: `gather` 成功後，前端必須顯示更新後的 AP 與 Inventory quantity。
- REQ-006.11: 玩家重複成功執行 `gather` 時，系統必須累加 `Wood` quantity，不能覆寫既有 quantity。
- REQ-006.12: `gather` 輸入格式錯誤或包含未支援欄位時，後端必須拒絕 Action，且 AP、位置與 Inventory 都保持不變。
- REQ-006.13: 後端拒絕不合法的 `gather` 時，必須將事件以 error outcome 寫入 stdout，並包含 user ID、Action、拒絕原因與 request ID；log 不得包含 credentials、session、OAuth 資料或未處理的原始輸入。

### Task 3: Add typed frontend gathering client

- [x] 在 `web/src/auth.ts` 解析 Inventory 與 backend-owned gathering option，並以 `{}` 送出不含 gameplay values 的 gather request。
- [x] 將成功、AP 不足、位置錯誤、輸入錯誤與未登入 response 轉為明確 client result。
- [x] 在 `web/src/auth.test.ts` 驗證 player state、request 與所有 response 分支。

Source files: `web/src/auth.ts`

Acceptance criteria:

- REQ-006.1: 已登入玩家位於 `forest_edge` 時，必須能透過前端執行 `gather` Action。
- REQ-006.4: 後端必須依玩家目前位置決定 gather 的 item、quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-006.8: 尚未取得任何 item 的玩家必須擁有空 Inventory。
- REQ-006.9: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory item 與 quantity。
- REQ-006.10: `gather` 成功後，前端必須顯示更新後的 AP 與 Inventory quantity。

### Task 4: Add gathering and Inventory UI

- [x] 在 `web/src/App.tsx` 顯示 Inventory item 與 quantity，並依後端 gathering option 顯示 gather control。
- [x] request 期間停用所有 action control。成功與失敗後都套用後端權威狀態。
- [x] 在 `web/src/App.test.tsx` 驗證空 Inventory、可用 action、成功、失敗與重複提交防護。

Source files: `web/src/App.tsx`

Acceptance criteria:

- REQ-006.1: 已登入玩家位於 `forest_edge` 時，必須能透過前端執行 `gather` Action。
- REQ-006.8: 尚未取得任何 item 的玩家必須擁有空 Inventory。
- REQ-006.9: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory item 與 quantity。
- REQ-006.10: `gather` 成功後，前端必須顯示更新後的 AP 與 Inventory quantity。

## Review Issues

## Plan Review Issues

- [x] 定義唯一合法的 `gather` request payload，並讓 Task 2 與 Task 3 採用相同格式。現有 plan 只禁止 gameplay values，未說明合法格式是空 body 或 `{}`。Task 2 必須逐項驗證 malformed JSON、unknown fields、duplicate values 與 trailing values。Task 3 必須送出該合法格式。
