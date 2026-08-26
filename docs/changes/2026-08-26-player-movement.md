---
title: "Player movement"
status: Issues-confirmed
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths:
  - internal/authapi/server.go
  - internal/authapi/store.go
  - internal/authapi/server_test.go
  - internal/authapi/store_test.go
  - web/src/App.tsx
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.test.tsx
req_ref: REQ-005
base_branch: main
scope: "Tracks the first persistent location-changing game action."
---

## Problem Statement

Players can spend AP through `rest`, but they have no persistent location, backend-defined Route, or movement action that changes game state.

## Recommended Direction

Add backend-owned locations and directed Routes, persist one current location per player, and expose only the currently allowed `move` targets. Consume the Route AP cost and update location as one operation.

## Key Assumptions

- `camp` is the default location.
- `camp` and `forest_edge` have one directed Route in each direction.
- Each Route costs 20 AP.
- The backend owns Action and target allowlists.
- The frontend submits only the selected target identifier.

## Acceptance Criteria

The source of truth is `REQ-005`.

## MVP Scope / Not Doing

- Include two locations, two directed Routes, persistent player location, one `move` action, frontend controls, strict input rejection, and safe error logging.
- Exclude maps, discovery, more locations, variable costs, travel time, random events, encounters, inventory, and WebSocket delivery.

## Tasks

```text
Task 1: SQLite movement state
  -> Task 2: Movement API
    -> Task 3: Frontend movement client
      -> Task 4: Movement UI
```

所有 task 依序執行。沒有可平行執行的 task。

### Task 1: Store movement state and atomic mutation

- [x] 在 `internal/authapi/store.go` 建立 location、Route 與玩家位置 schema，加入固定 seed 與既有玩家 backfill。
- [x] 實作讀取玩家狀態，以及原子化驗證 Route、扣除 AP、更新位置的 store operation。
- [x] 在 `internal/authapi/store_test.go` 驗證預設位置、Route、成功移動、AP 不足、不合法 Route、持久化與 rollback。

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-005.2: 尚無位置紀錄的玩家必須位於 `camp`。
- REQ-005.3: 後端必須保存允許的 Route、每條 Route 的起點、終點與 AP 成本。
- REQ-005.4: `camp` 與 `forest_edge` 之間必須各有一條可通行方向，每次移動成本為 20 AP。
- REQ-005.5: 已登入玩家必須能透過 `move` action，沿目前位置允許的 Route 移動至該 Route 的終點。
- REQ-005.6: `move` 成功時，系統必須扣除該 Route 的完整 AP 成本，更新目前位置，並保存兩項結果。
- REQ-005.7: 玩家 AP 不足時，`move` 必須失敗，且 AP 與目前位置都保持不變。
- REQ-005.8: 玩家要求不存在或不從目前位置出發的 Route 時，`move` 必須失敗，且 AP 與目前位置都保持不變。
- REQ-005.9: 玩家重新整理或重新登入後，系統必須顯示已保存的目前位置、允許的 Route 與目前 AP。
- REQ-005.12: `move` 只能接受目前位置允許的 Route 終點作為 target，不能接受前端自行指定的 Route 成本或起點。

### Task 2: Add movement API and safe rejection logging

- [x] 在 `internal/authapi/server.go` 擴充 `GET /api/me`，並新增 `POST /api/actions/move`。
- [x] 嚴格解析只含 `target` 的 JSON object。拒絕未知欄位、額外 JSON value、未知 Action 與不合法 target。
- [x] 使用安全 reason code，將拒絕事件寫至 stdout。不得記錄原始輸入或敏感資料。
- [x] 在 `internal/authapi/server_test.go` 驗證 response、驗證失敗、狀態不變與 log 欄位。

Source files: `internal/authapi/server.go`

Acceptance criteria:

- REQ-005.1: 玩家必須能看到目前位置、目前位置允許的 Route，以及每條 Route 的 AP 成本。
- REQ-005.5: 已登入玩家必須能透過 `move` action，沿目前位置允許的 Route 移動至該 Route 的終點。
- REQ-005.6: `move` 成功時，系統必須扣除該 Route 的完整 AP 成本，更新目前位置，並保存兩項結果。
- REQ-005.7: 玩家 AP 不足時，`move` 必須失敗，且 AP 與目前位置都保持不變。
- REQ-005.8: 玩家要求不存在或不從目前位置出發的 Route 時，`move` 必須失敗，且 AP 與目前位置都保持不變。
- REQ-005.9: 玩家重新整理或重新登入後，系統必須顯示已保存的目前位置、允許的 Route 與目前 AP。
- REQ-005.11: 後端只能執行明確允許的 Action，任何其他 action identifier 都必須被拒絕。
- REQ-005.12: `move` 只能接受目前位置允許的 Route 終點作為 target，不能接受前端自行指定的 Route 成本或起點。
- REQ-005.13: Action 輸入格式錯誤、包含未支援欄位或不符合允許值時，後端必須拒絕動作，且 AP 與目前位置都保持不變。
- REQ-005.14: 後端拒絕不合法的 action identifier、target 或輸入格式時，必須將事件以 error outcome 寫入 stdout，並包含 user ID、Action、拒絕原因與 request ID；log 不得包含 credentials、session、OAuth 資料或未處理的原始輸入。

### Task 3: Add typed frontend movement client

- [x] 在 `web/src/auth.ts` 解析目前位置與 Route，並提交只含 target identifier 的 move request。
- [x] 將成功、AP 不足、不合法輸入與未登入 response 轉為明確 client result。
- [x] 在 `web/src/auth.test.ts` 驗證 request body 與所有 response 分支。

Source files: `web/src/auth.ts`

Acceptance criteria:

- REQ-005.1: 玩家必須能看到目前位置、目前位置允許的 Route，以及每條 Route 的 AP 成本。
- REQ-005.9: 玩家重新整理或重新登入後，系統必須顯示已保存的目前位置、允許的 Route 與目前 AP。
- REQ-005.10: `move` 成功後，前端必須顯示更新後的位置、允許的 Route 與目前 AP。
- REQ-005.12: `move` 只能接受目前位置允許的 Route 終點作為 target，不能接受前端自行指定的 Route 成本或起點。

### Task 4: Add movement UI

- [x] 在 `web/src/App.tsx` 顯示目前位置、可用 Route 與 AP 成本，並提供每個允許 target 的 move control。
- [x] 移動期間停用 action control。成功後套用後端狀態，失敗後保留後端權威狀態。
- [x] 在 `web/src/App.test.tsx` 驗證首次載入、成功移動、失敗與重複提交防護。

Source files: `web/src/App.tsx`

Acceptance criteria:

- REQ-005.1: 玩家必須能看到目前位置、目前位置允許的 Route，以及每條 Route 的 AP 成本。
- REQ-005.9: 玩家重新整理或重新登入後，系統必須顯示已保存的目前位置、允許的 Route 與目前 AP。
- REQ-005.10: `move` 成功後，前端必須顯示更新後的位置、允許的 Route 與目前 AP。

## Review Issues

- [x] [Major] `accessLog` 將未知 Action 的原始 URL path 寫入 stdout。安全 rejection log 仍會伴隨 `action=POST /api/actions/<raw identifier>`，違反未處理原始輸入不得進入 log 的準則。
- [ ] [Major] `GetPlayerState` 使用三次獨立查詢讀取位置、Route 與 AP。並行 `move` 可以在查詢間提交，讓 `/api/me` 或 action response 回傳不同時間點拼成的非權威狀態。
