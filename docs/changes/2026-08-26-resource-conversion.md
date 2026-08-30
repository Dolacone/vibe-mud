---
title: "Resource conversion"
status: Done
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-007
base_branch: main
scope: "Tracks the first Inventory-to-Resource conversion loop."
---

## Problem Statement

Gathering produces persistent `Wood` items, but players cannot convert collected items into a persistent Resource balance with a later spending purpose.

## Recommended Direction

Add a backend-owned conversion rule for `camp`, consume AP and `Wood`, increment Resource, and return the complete player state from one SQLite transaction.

## Key Assumptions

- `convert` is available only at `camp`.
- Each successful `convert` costs 1 AP and 1 `Wood`.
- Each successful `convert` yields exactly 1 Resource.
- Resource is one generic persistent integer balance, not an Inventory item.
- The backend owns all conversion values.
- The only valid `convert` request payload is an empty JSON object: `{}`.

## Acceptance Criteria

The source of truth is `REQ-007`.

## MVP Scope / Not Doing

- Include one deterministic conversion rule, persistent Resource balance, atomic AP and Inventory mutation, strict input rejection, safe error logging, and frontend Resource display.
- Exclude item creation, Resource spending, buildings, trading, conversion ratios, batch conversion, bonuses, capacity, and multiple Resource types.

## Tasks

```text
Task 1: SQLite conversion and Resource state
  -> Task 2: Conversion API
    -> Task 3: Frontend conversion client
      -> Task 4: Conversion and Resource UI
```

所有 task 依序執行。沒有可平行執行的 task。

### Task 1: Store conversion and Resource state atomically

- [x] 在 `internal/authapi/store.go` 建立 conversion rule 與 Resource balance schema，加入 `camp` conversion rule seed，並初始化新舊玩家的 balance。
- [x] 將 Resource balance 與目前位置可用的 conversion option 納入玩家狀態。
- [x] 實作原子化驗證位置、Wood、AP，扣除 AP 與 Wood，累加 Resource 的 convert operation。
- [x] 在 `internal/authapi/store_test.go` 驗證預設 balance、成功、重複轉換、最後一個 Wood、所有拒絕、持久化與 rollback。

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-007.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Resource。
- REQ-007.3: 後端必須決定允許的 Location、input item、input quantity、Resource 產量與 AP 成本，前端不能指定或覆寫這些值。
- REQ-007.4: Resource 必須保存為獨立於 Inventory item quantity 的持久化玩家 balance。
- REQ-007.5: 尚未取得 Resource 的玩家必須擁有 0 Resource。
- REQ-007.6: 玩家不在 `camp` 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.7: 玩家沒有至少 1 個 `Wood` 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.8: 玩家 AP 少於 1 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.9: AP 扣除、`Wood` 扣除與 Resource 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-007.10: 玩家轉換最後一個 `Wood` 後，Inventory 必須不再顯示 `Wood`。
- REQ-007.11: 玩家重複成功執行 `convert` 時，系統必須持續扣除 `Wood` 並累加 Resource balance。
- REQ-007.12: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與 Resource balance。

### Task 2: Add conversion API and safe rejection logging

- [x] 在 `internal/authapi/server.go` 擴充 `GET /api/me`，並新增 `POST /api/actions/convert`。
- [x] `convert` request 只接受 `{}`，並拒絕 malformed JSON、unknown fields、duplicate fields 與 trailing JSON values。
- [x] 對位置錯誤、Wood 不足、AP 不足與輸入錯誤回傳後端權威狀態，並寫出安全 outcome log。
- [x] 在 `internal/authapi/server_test.go` 驗證成功、拒絕、狀態不變與 log 欄位。

Source files: `internal/authapi/server.go`

Acceptance criteria:

- REQ-007.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-007.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Resource。
- REQ-007.3: 後端必須決定允許的 Location、input item、input quantity、Resource 產量與 AP 成本，前端不能指定或覆寫這些值。
- REQ-007.4: Resource 必須保存為獨立於 Inventory item quantity 的持久化玩家 balance。
- REQ-007.5: 尚未取得 Resource 的玩家必須擁有 0 Resource。
- REQ-007.6: 玩家不在 `camp` 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.7: 玩家沒有至少 1 個 `Wood` 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.8: 玩家 AP 少於 1 時，`convert` 必須失敗，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.9: AP 扣除、`Wood` 扣除與 Resource 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-007.10: 玩家轉換最後一個 `Wood` 後，Inventory 必須不再顯示 `Wood`。
- REQ-007.11: 玩家重複成功執行 `convert` 時，系統必須持續扣除 `Wood` 並累加 Resource balance。
- REQ-007.12: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與 Resource balance。
- REQ-007.13: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與 Resource balance。
- REQ-007.14: `convert` 輸入格式錯誤或包含未支援欄位時，後端必須拒絕 Action，且 AP、位置、Inventory 與 Resource balance 都保持不變。
- REQ-007.15: 後端拒絕不合法的 `convert` 時，必須將事件以 error outcome 寫入 stdout，並包含 user ID、Action、拒絕原因與 request ID；log 不得包含 credentials、session、OAuth 資料或未處理的原始輸入。

### Task 3: Add typed frontend conversion client

- [x] 在 `web/src/auth.ts` 解析 Resource 與 backend-owned conversion option，並以 `{}` 送出不含 gameplay values 的 convert request。
- [x] 將成功、位置錯誤、Wood 不足、AP 不足、輸入錯誤與未登入 response 轉為明確 client result。
- [x] 在 `web/src/auth.test.ts` 驗證 player state、request 與所有 response 分支。

Source files: `web/src/auth.ts`

Acceptance criteria:

- REQ-007.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-007.3: 後端必須決定允許的 Location、input item、input quantity、Resource 產量與 AP 成本，前端不能指定或覆寫這些值。
- REQ-007.4: Resource 必須保存為獨立於 Inventory item quantity 的持久化玩家 balance。
- REQ-007.5: 尚未取得 Resource 的玩家必須擁有 0 Resource。
- REQ-007.12: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與 Resource balance。
- REQ-007.13: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與 Resource balance。

### Task 4: Add conversion and Resource UI

- [x] 在 `web/src/App.tsx` 顯示 Resource balance，並依後端 conversion option 顯示 convert control。
- [x] request 期間停用所有 action control。成功與失敗後都套用後端權威狀態。
- [x] 在 `web/src/App.test.tsx` 驗證預設 balance、可用 action、成功、失敗、最後一個 Wood 與重複提交防護。

Source files: `web/src/App.tsx`

Acceptance criteria:

- REQ-007.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-007.5: 尚未取得 Resource 的玩家必須擁有 0 Resource。
- REQ-007.10: 玩家轉換最後一個 `Wood` 後，Inventory 必須不再顯示 `Wood`。
- REQ-007.12: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與 Resource balance。
- REQ-007.13: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與 Resource balance。

## Review Issues

- [x] [Major] `decodeConvertRequest` 接受 `{"":1}`。空字串欄位仍是 unsupported input，因此違反 REQ-007.14 的 only-`{}` contract。加入拒絕邏輯與測試。
- [x] [Major] AP 不足時，`convert` 寫出 `outcome=insufficient_ap`，且未包含 `reason`。REQ-007.15 要求所有拒絕事件使用 `outcome=error`，並包含安全拒絕原因。改用 rejection log 並更新測試。
