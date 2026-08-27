---
title: "Typed resources"
status: Ready-to-review
created: 2026-08-27
doc_type: change
last_reviewed: 2026-08-27
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
related_req_refs:
  - REQ-008
base_branch: main
scope: "Tracks the replacement of one generic Resource balance with eight typed Resource quantities."
---

## Problem Statement

The game stores one generic Resource balance, so it cannot represent Resource categories with separate quantities.

## Recommended Direction

Store Resource definitions separately from per-player quantities. Return all eight definitions with zero for missing player rows. Keep the existing `convert` Action and make its output explicitly target Wood Resource.

## Key Assumptions

- The Resource types are Food, Wood, Stone, Metal, Fiber, Hide, Medicinal, and Arcane.
- A missing player Resource row represents quantity 0.
- Existing generic Resource balances can be discarded.
- The current `convert` location, input quantity, output quantity, AP cost, and request format remain unchanged.
- The current `convert` output becomes Wood Resource.

## Acceptance Criteria

The source of truth is REQ-007. Existing `convert` behavior remains defined by REQ-008.

## MVP Scope / Not Doing

- Include typed Resource definitions, per-player quantities, full API and UI display, and Wood Resource output from `convert`.
- Exclude weight, Item expiration, batch dismantling, new raw materials, new conversions, building storage, Rare Components, and Resource consumers.

## Tasks

```text
Task 1: SQLite typed Resource state
  -> Task 2: Typed Resource API
    -> Task 3: Typed Resource client
      -> Task 4: Typed Resource UI
```

所有 task 依序執行。沒有可平行執行的 task。

### Task 1: Store typed Resource state atomically

  - [x] 在 `internal/authapi/store.go` 建立 Resource type、per-player quantity 與 typed conversion schema，並加入 8 種 Resource seed。
  - [x] 捨棄 legacy generic balance，讓缺少 player row 的 Resource 回傳 quantity 0。
  - [x] 讓 `convert` 將 AP、Wood item 與 Wood Resource quantity 放在同一 transaction 更新。
  - [x] 在 `internal/authapi/store_test.go` 驗證 schema 升級、8 種狀態、零值、持久化、成功、拒絕與 rollback。

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-007.1: 遊戲必須定義 `Food`、`Wood`、`Stone`、`Metal`、`Fiber`、`Hide`、`Medicinal` 與 `Arcane` 這 8 種 Resource。
- REQ-007.2: 系統必須為每位玩家分別保存每種 Resource 的 quantity，不能將多種 Resource 合併為單一 balance。
- REQ-007.3: 玩家尚未取得某種 Resource 時，該 Resource 的 quantity 必須為 0。
- REQ-007.5: Resource quantity 必須保存為獨立於 Inventory item quantity 的持久化玩家狀態。
- REQ-008.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Wood Resource。
- REQ-008.3: 後端必須決定允許的 Location、input item、input quantity、output Resource、output quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-008.4: 玩家不在 `camp` 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.5: 玩家沒有至少 1 個 `Wood` 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.6: 玩家 AP 少於 1 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.7: AP 扣除、`Wood` item 扣除與 Wood Resource 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-008.8: 玩家轉換最後一個 `Wood` item 後，Inventory 必須不再顯示 `Wood`。
- REQ-008.9: 玩家重複成功執行 `convert` 時，系統必須持續扣除 `Wood` item 並累加 Wood Resource quantity。
- REQ-008.10: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與所有 Resource quantity。

### Task 2: Expose typed Resource state through the API

- [x] 在 `internal/authapi/server.go` 將 player state 與 conversion option 改為 typed Resource response。
- [x] 保留 `convert` 的 `{}` request contract、錯誤狀態與安全 rejection log。
- [x] 在 `internal/authapi/server_test.go` 驗證 8 種 Resource、零值、Wood output、成功與所有 rejection response。

Source files: `internal/authapi/server.go`, `internal/authapi/server_test.go`

Acceptance criteria:

- REQ-007.1: 遊戲必須定義 `Food`、`Wood`、`Stone`、`Metal`、`Fiber`、`Hide`、`Medicinal` 與 `Arcane` 這 8 種 Resource。
- REQ-007.2: 系統必須為每位玩家分別保存每種 Resource 的 quantity，不能將多種 Resource 合併為單一 balance。
- REQ-007.3: 玩家尚未取得某種 Resource 時，該 Resource 的 quantity 必須為 0。
- REQ-007.4: 玩家必須能在前端看到全部 8 種 Resource 與各自的 quantity。
- REQ-007.5: Resource quantity 必須保存為獨立於 Inventory item quantity 的持久化玩家狀態。
- REQ-008.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-008.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Wood Resource。
- REQ-008.3: 後端必須決定允許的 Location、input item、input quantity、output Resource、output quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-008.4: 玩家不在 `camp` 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.5: 玩家沒有至少 1 個 `Wood` 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.6: 玩家 AP 少於 1 時，`convert` 必須失敗，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.7: AP 扣除、`Wood` item 扣除與 Wood Resource 增加必須是同一個原子結果，系統不能保存部分結果。
- REQ-008.8: 玩家轉換最後一個 `Wood` item 後，Inventory 必須不再顯示 `Wood`。
- REQ-008.9: 玩家重複成功執行 `convert` 時，系統必須持續扣除 `Wood` item 並累加 Wood Resource quantity。
- REQ-008.10: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與所有 Resource quantity。
- REQ-008.11: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與所有 Resource quantity。
- REQ-008.12: `convert` 輸入格式錯誤或包含未支援欄位時，後端必須拒絕 Action，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。
- REQ-008.13: 後端拒絕不合法的 `convert` 時，必須將事件以 error outcome 寫入 stdout，並包含 user ID、Action、拒絕原因與 request ID；log 不得包含 credentials、session、OAuth 資料或未處理的原始輸入。

### Task 3: Parse typed Resource state in the frontend client

- [x] 在 `web/src/auth.ts` 將單一 Resource number 改為 typed Resource list，並解析 conversion option 的 Wood output。
- [x] 保留 `convert` request 與所有 result branches。
- [x] 在 `web/src/auth.test.ts` 驗證 8 種 Resource、零值、invalid response 與 conversion response。

Source files: `web/src/auth.ts`

Acceptance criteria:

- REQ-007.1: 遊戲必須定義 `Food`、`Wood`、`Stone`、`Metal`、`Fiber`、`Hide`、`Medicinal` 與 `Arcane` 這 8 種 Resource。
- REQ-007.2: 系統必須為每位玩家分別保存每種 Resource 的 quantity，不能將多種 Resource 合併為單一 balance。
- REQ-007.3: 玩家尚未取得某種 Resource 時，該 Resource 的 quantity 必須為 0。
- REQ-007.4: 玩家必須能在前端看到全部 8 種 Resource 與各自的 quantity。
- REQ-008.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-008.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Wood Resource。
- REQ-008.3: 後端必須決定允許的 Location、input item、input quantity、output Resource、output quantity 與 AP 成本，前端不能指定或覆寫這些值。
- REQ-008.10: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與所有 Resource quantity。
- REQ-008.11: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與所有 Resource quantity。
- REQ-008.12: `convert` 輸入格式錯誤或包含未支援欄位時，後端必須拒絕 Action，且 AP、位置、Inventory 與所有 Resource quantity 都保持不變。

### Task 4: Display all typed Resources

- [x] 在 `web/src/App.tsx` 顯示 8 種 Resource 與 quantity，並顯示 `convert` 的 Wood Resource output。
- [x] 成功與失敗後都套用後端權威的 typed Resource state。
- [x] 在 `web/src/App.test.tsx` 驗證初始零值、Wood 累加、完整列表與既有 action feedback。

Source files: `web/src/App.tsx`

Acceptance criteria:

- REQ-007.1: 遊戲必須定義 `Food`、`Wood`、`Stone`、`Metal`、`Fiber`、`Hide`、`Medicinal` 與 `Arcane` 這 8 種 Resource。
- REQ-007.3: 玩家尚未取得某種 Resource 時，該 Resource 的 quantity 必須為 0。
- REQ-007.4: 玩家必須能在前端看到全部 8 種 Resource 與各自的 quantity。
- REQ-008.1: 已登入玩家位於 `camp` 時，必須能透過前端執行 `convert` Action。
- REQ-008.2: `convert` 成功時，必須消耗 1 AP 與 1 個 `Wood` item，並增加 1 Wood Resource。
- REQ-008.10: 玩家重新整理或重新登入後，系統必須顯示已保存的 Inventory 與所有 Resource quantity。
- REQ-008.11: `convert` 成功後，前端必須顯示更新後的 AP、Inventory 與所有 Resource quantity。

## Review Issues
