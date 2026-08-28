---
title: "Item durability and expired retention"
status: Done
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - docs/changes/2026-08-29-item-durability.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/store_durability_overflow_test.go
  - internal/authapi/store_durability_attribution_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-015
base_branch: main
scope: "Tracks time-derived durability for stackable Items and seven-day expired retention for Items and Buildings."
---

## Problem Statement

Stackable Items remain usable forever. Players cannot distinguish active assets from expired assets, and the existing Building retention period conflicts with the agreed seven-day rule.

## Recommended Direction

Add `max_durability_seconds` to Item definitions. Rebuild legacy Inventory and ground Item holdings as Active stacks with a full definition-backed lifetime from migration time.

Persist `durability_status` and `status_expires_at` on Inventory and ground Item stacks. Active `status_expires_at` is the durability deadline. Expired `status_expires_at` is the deletion deadline. Each player or Location can hold one Active and one Expired stack per Item type.

Normalize affected holdings inside the same transaction before reads, Actions, movement, and Transfers. Convert elapsed Active stacks to Expired stacks, merge Expired quantities using the latest deletion deadline, and delete Expired stacks after retention. Include both retained statuses in carrying weight.

Return `durability_status`, nullable `durability_remaining_seconds`, and nullable `retention_remaining_seconds` on Inventory and ground Item entries. Item Transfer requests add `item_status`. Item Drop accepts `active` or `expired`; Item Pickup accepts only `active`; Resource Transfers reject `item_status`.

## Key Assumptions

- The MVP covers existing stackable Items in Inventory and on the ground.
- Active and Expired quantities use separate stacks.
- Resources remain non-expiring.
- Equipment and per-instance durability remain outside this change.
- Time calculations use UTC Unix seconds.
- Invalid Item Transfer input returns HTTP 400. Expired Pickup and insufficient quantity return HTTP 409 with authoritative state.
- Store state carries non-JSON durability calculation and cleanup metadata for safe stdout logging.

## Acceptance Criteria

## MVP Scope / Not Doing

- No Equipment or Item instances.
- No battle-based durability loss.
- No Item repair.
- No Resource expiry.

## Tasks

```text
Task 1: durability schema and lifecycle [parallel: no]
└── Task 2: Item Actions and Transfers [parallel: no]
    └── Task 3: backend durability API and logs [parallel: no]
        └── Task 4: frontend durability contract [parallel: no]
            └── Task 5: durability interface [parallel: no]
```

- [x] Task 1 [parallel: no]: Add Item durability definitions, idempotent holding-table migration, Active-to-Expired normalization, Expired cleanup, carrying-weight integration, and seven-day Building retention in `internal/authapi/store.go`. Update store tests and keep planned documentation aligned.
  - REQ-011.6: Disabled Building 必須保留 7 天，期間仍能維修。
  - REQ-011.7: Building 停用超過 7 天後必須永久消失。
  - REQ-011.8: 永久消失的 Building 不得再顯示或維修。
  - REQ-011.9: Building 永久消失後，不得繼續占用玩家在該 Location 的 Building 名額。
  - REQ-015.1: 每種 Item 必須分別定義耐久時間上限。
  - REQ-015.2: Wood Item 與 Wood Component 的耐久時間上限都必須為 7 天。
  - REQ-015.3: 既有的有效 Item 必須在此功能部署時取得完整 7 天耐久時間。
  - REQ-015.5: Item 剩餘耐久時間大於 0 時必須顯示為 Active。
  - REQ-015.6: Item 剩餘耐久時間到達 0 時必須顯示為 Expired，且不能修復或恢復為 Active。
  - REQ-015.7: Active 與 Expired Item 必須顯示為不同堆疊。
  - REQ-015.8: 玩家必須能看到每個 Item 堆疊的狀態與剩餘有效時間。
  - REQ-015.14: Expired Item 必須從失效時間起保留 7 天。
  - REQ-015.15: Expired Item 保留期結束後必須永久刪除。
  - REQ-015.16: Expired Item 仍必須計入玩家的攜帶重量。
  - REQ-015.18: 相同持有位置的同種 Expired Item 可以合併為一個堆疊。
  - REQ-015.19: Expired 堆疊合併時，結果刪除時間必須採用所有來源中最晚的刪除時間。
  - REQ-015.20: Active Item 永遠不得與 Expired Item 合併。
  - REQ-015.25: 本需求不增加 Equipment、Item instance、戰鬥耗損或耐久修復。
- [x] Task 2 [parallel: no]: Make Item-producing Actions create full-durability Active quantities, make Item-consuming Actions use only Active quantities, and preserve or merge durability during Item Transfers in `internal/authapi/store.go`. Update store tests without changing Resource Transfer behavior.
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的有效或失效 Item Drop 至目前 Location。
  - REQ-013.6: 玩家必須能指定正整數 quantity，將持有的任意 Resource Drop 至目前 Location。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的有效地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.9: Item 必須依 Location、Item type 與有效狀態合併 quantity。有效與失效 Item 不得合併。
  - REQ-013.12: Transfer 成功或失敗時都不得消耗或恢復 AP。
  - REQ-013.13: Transfer 成功時，來源扣除與目的增加必須原子更新，且總 quantity 必須保持不變。
  - REQ-013.14: 玩家持有量或地面持有量不足，或玩家嘗試 Pickup 失效 Item 時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-013.15: 玩家不得 Pickup 或 Drop 目前 Location 以外的地面資產。
  - REQ-013.16: 多位玩家同時 Pickup 時，地面 quantity 不得低於 0，且成功取得的總量不得超過原有 quantity。
  - REQ-013.17: 地面資產與 Transfer 結果必須在重新整理、重新登入及其他玩家讀取後保持一致。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Transfer 不得指定 Item 狀態。
  - REQ-013.23: Item 狀態缺少、不合法或不適用時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-015.4: Gather 與 Craft 新產生的 Item 必須取得該 Item 的完整耐久時間。
  - REQ-015.9: 相同持有位置的同種 Active Item 必須合併為一個堆疊。
  - REQ-015.10: Active 堆疊合併時，結果剩餘時間必須等於各來源 quantity 乘以剩餘時間後的總和，再除以合併後總 quantity。
  - REQ-015.11: 合併後的剩餘時間必須向下取整至完整秒數。
  - REQ-015.12: Active Item 被部分 Transfer 時，來源與移出 quantity 必須保留 Transfer 當下的相同剩餘時間。
  - REQ-015.13: Active Item Transfer 至既有 Active 堆疊時，必須依相同的數量加權公式合併。
  - REQ-015.17: Expired Item 不得用於 Convert、Craft 或 Building inputs。
  - REQ-015.18: 相同持有位置的同種 Expired Item 可以合併為一個堆疊。
  - REQ-015.19: Expired 堆疊合併時，結果刪除時間必須採用所有來源中最晚的刪除時間。
  - REQ-015.20: Active Item 永遠不得與 Expired Item 合併。
- [x] Task 3 [parallel: no]: Expose durability stack fields, validate `item_status`, return authoritative expired-Pickup conflicts, and emit safe durability calculation and cleanup logs in `internal/authapi/server.go`. Update server tests.
  - REQ-013.14: 玩家持有量或地面持有量不足，或玩家嘗試 Pickup 失效 Item 時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 與 Resource 必須提供 Drop。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-013.20: Backend 必須把 Transfer access 與結果寫入 stdout，並包含 user ID、Location、asset type、asset identifier、quantity、結果與 request ID。
  - REQ-013.21: Backend log 不得包含 credentials、session、OAuth 資料、cookie 或未處理的原始輸入。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Transfer 不得指定 Item 狀態。
  - REQ-013.23: Item 狀態缺少、不合法或不適用時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-015.5: Item 剩餘耐久時間大於 0 時必須顯示為 Active。
  - REQ-015.6: Item 剩餘耐久時間到達 0 時必須顯示為 Expired，且不能修復或恢復為 Active。
  - REQ-015.7: Active 與 Expired Item 必須顯示為不同堆疊。
  - REQ-015.8: 玩家必須能看到每個 Item 堆疊的狀態與剩餘有效時間。
  - REQ-015.21: Inventory 與地面都必須顯示尚在保留期內的 Expired Item。
  - REQ-015.23: 後端必須將 Item 耐久計算與失效清理結果寫入標準輸出。紀錄必須包含玩家 ID 或明確的 anonymous、操作、結果、request ID 與計算值。
  - REQ-015.24: Log 不得包含 credentials、session、OAuth 資料、cookie、secret 或未處理的原始輸入。
- [x] Task 4 [parallel: no]: Parse durability stack fields and submit strict Item or Resource Transfer payloads in `web/src/auth.ts`. Update client contract tests.
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 與 Resource 必須提供 Drop。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Transfer 不得指定 Item 狀態。
  - REQ-013.23: Item 狀態缺少、不合法或不適用時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-015.5: Item 剩餘耐久時間大於 0 時必須顯示為 Active。
  - REQ-015.6: Item 剩餘耐久時間到達 0 時必須顯示為 Expired，且不能修復或恢復為 Active。
  - REQ-015.7: Active 與 Expired Item 必須顯示為不同堆疊。
  - REQ-015.8: 玩家必須能看到每個 Item 堆疊的狀態與剩餘有效時間。
  - REQ-015.21: Inventory 與地面都必須顯示尚在保留期內的 Expired Item。
- [x] Task 5 [parallel: no]: Display separate Active and Expired Inventory and ground rows with the correct remaining time and available Transfer controls in `web/src/App.tsx`. Update interface tests.
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 與 Resource 必須提供 Drop。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Transfer 不得指定 Item 狀態。
  - REQ-015.5: Item 剩餘耐久時間大於 0 時必須顯示為 Active。
  - REQ-015.6: Item 剩餘耐久時間到達 0 時必須顯示為 Expired，且不能修復或恢復為 Active。
  - REQ-015.7: Active 與 Expired Item 必須顯示為不同堆疊。
  - REQ-015.8: 玩家必須能看到每個 Item 堆疊的狀態與剩餘有效時間。
  - REQ-015.21: Inventory 與地面都必須顯示尚在保留期內的 Expired Item。
  - REQ-015.22: 前端必須顯示 Expired Item 的剩餘保留時間，且不能提供會使用該 Item 的操作。

## Review Issues

- [x] [Major] `isGroundItems` rejects a valid Active and Expired pair because it requires unique Item IDs instead of unique `(item_id, durability_status)` keys. Any backend state with both ground stacks makes `getCurrentUser` and Transfer response parsing fail.
- [x] [Major] Successful Pickup, Move, Convert, Craft, Build, construction contribution, and repair operations discard cleanup metadata from normalization. Expiry and deletion results triggered by those operations never reach `logItemDurability`, which violates the stdout computation-log contract.
- [x] [Major] Item normalization processes every player's Inventory, but cleanup metadata omits the holding owner. A request by one player can log another player's cleanup with the requester's `user_id`, so the stable application user ID is incorrect.
- [x] [Major] Active-stack weighting multiplies persisted quantities by Unix deadlines in `int64` without overflow checks. Valid large quantities can wrap the weighted deadline and corrupt durability instead of applying the required floored weighted average.
- [ ] [Minor] Frontend response validation enforces unique `(item_id, durability_status)` ground stacks but does not enforce the same Inventory invariant. A malformed response with duplicate Active or duplicate Expired Inventory rows is accepted and reaches React with duplicate row keys.
