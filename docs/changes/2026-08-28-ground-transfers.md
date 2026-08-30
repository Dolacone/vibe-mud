---
title: "Ground asset transfers"
status: Done
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - docs/changes/2026-08-28-ground-transfers.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
  - requirements/BEHAVIOR.md
  - requirements/REQ-013.md
req_ref: REQ-013
base_branch: main
scope: "Tracks AP-free transfers between player holdings and public Location ground assets."
---

## Problem Statement

Players cannot leave existing assets in the world or collect assets left by others. The game needs persistent public ground holdings without treating relocation as an AP-consuming Action.

## Recommended Direction

Persist ground Item and Resource quantities separately by Location and asset type. Use dedicated Transfer endpoints for atomic Pickup and Drop operations.

Expose `POST /api/transfers/drop` and `POST /api/transfers/pickup`. Both accept only `{"asset_type":"item"|"resource","asset_id":"<identifier>","quantity":<positive integer>}`. The backend derives Location from the authenticated player. Every player-state response includes nonzero `ground_items` and `ground_resources` arrays for that Location. A ground Item entry is `{"item":{"id":"<identifier>","display_name":"<name>"},"quantity":<positive integer>}`. A ground Resource entry is `{"resource":{"id":"<identifier>","display_name":"<name>"},"quantity":<positive integer>}`.

Successful Transfers return HTTP 200 with authoritative state. For authenticated handled failures, malformed or unknown assets return HTTP 400 and insufficient source quantity returns HTTP 409. These responses include an error and authoritative state without changing AP or quantities. Authentication failures return HTTP 401 without player state. Unexpected internal failures return HTTP 500 without claiming authoritative state.

## Key Assumptions

- Ground capacity is unlimited.
- Ground holdings are public and ownerless.
- Transfers preserve total quantity and AP.
- Item durability remains outside this change.

## Acceptance Criteria

## MVP Scope / Not Doing

- No carrying weight.
- No Item durability.
- No Warehouse or Trade.
- No ground ownership, permissions, reservation, or history.
- No conversion or Building bonus change.

## Tasks

```text
Task 1: persistence and Transfer rules [parallel: no]
└── Task 2: backend Transfer API [parallel: no]
    └── Task 3: frontend Transfer contract [parallel: no]
        └── Task 4: ground Transfer interface [parallel: no]
```

- [x] Task 1 [parallel: no]: Add public Location ground persistence and atomic Item or Resource Transfer rules in `internal/authapi/store.go`. Update store tests. Update `docs/schemas.md` with exact executable schema, migration behavior, zero-row cleanup, and transaction behavior in the same commit.
  - REQ-013.1: 每個 Location 必須具有獨立的地面 Item 與 Resource 狀態。
  - REQ-013.2: 地面資產不得限制總重量、quantity 或堆疊數量。
  - REQ-013.3: 地面資產不得具有 owner、存取權限或預留機制。
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的任意 Item Drop 至目前 Location。
  - REQ-013.6: 玩家必須能指定正整數 quantity，將持有的任意 Resource Drop 至目前 Location。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.9: Item 必須依 Location 與 Item type 合併 quantity。
  - REQ-013.10: Resource 必須依 Location 與 Resource type 合併 quantity，不得轉換成 Item。
  - REQ-013.11: Pickup 與 Drop 必須屬於 Transfer，不得視為 Action。
  - REQ-013.12: Transfer 成功或失敗時都不得消耗或恢復 AP。
  - REQ-013.13: Transfer 成功時，來源扣除與目的增加必須原子更新，且總 quantity 必須保持不變。
  - REQ-013.14: 玩家持有量或地面持有量不足時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-013.15: 玩家不得 Pickup 或 Drop 目前 Location 以外的地面資產。
  - REQ-013.16: 多位玩家同時 Pickup 時，地面 quantity 不得低於 0，且成功取得的總量不得超過原有 quantity。
  - REQ-013.17: 地面資產與 Transfer 結果必須在重新整理、重新登入及其他玩家讀取後保持一致。
- [x] Task 2 [parallel: no]: Add strict Pickup and Drop HTTP handlers in `internal/authapi/server.go`. Return typed ground holdings in every player state. Implement the planned request and status contract. Add sanitized Transfer access and computation logs. Update server tests.
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的任意 Item Drop 至目前 Location。
  - REQ-013.6: 玩家必須能指定正整數 quantity，將持有的任意 Resource Drop 至目前 Location。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.11: Pickup 與 Drop 必須屬於 Transfer，不得視為 Action。
  - REQ-013.12: Transfer 成功或失敗時都不得消耗或恢復 AP。
  - REQ-013.13: Transfer 成功時，來源扣除與目的增加必須原子更新，且總 quantity 必須保持不變。
  - REQ-013.14: 玩家持有量或地面持有量不足時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-013.15: 玩家不得 Pickup 或 Drop 目前 Location 以外的地面資產。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-013.20: Backend 必須把 Transfer access 與結果寫入 stdout，並包含 user ID、Location、asset type、asset identifier、quantity、結果與 request ID。
  - REQ-013.21: Backend log 不得包含 credentials、session、OAuth 資料、cookie 或未處理的原始輸入。
- [x] Task 3 [parallel: no]: Add typed ground holdings, Pickup, and Drop client contracts in `web/src/auth.ts`. Submit only the planned asset fields. Parse authoritative state from success and failure responses. Update client tests.
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的任意 Item Drop 至目前 Location。
  - REQ-013.6: 玩家必須能指定正整數 quantity，將持有的任意 Resource Drop 至目前 Location。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.10: Resource 必須依 Location 與 Resource type 合併 quantity，不得轉換成 Item。
  - REQ-013.11: Pickup 與 Drop 必須屬於 Transfer，不得視為 Action。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
- [x] Task 4 [parallel: no]: Add Ground Items and Ground Resources tables plus quantity controls in `web/src/App.tsx`. Add Drop controls to Inventory and Resources. Preserve compact table behavior and update interaction tests.
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的任意 Item Drop 至目前 Location。
  - REQ-013.6: 玩家必須能指定正整數 quantity，將持有的任意 Resource Drop 至目前 Location。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource，並提供對應的 Pickup 與 Drop Transfer。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。

## Review Issues

- [ ] [Minor] `source_paths` 未列出 diff 中修改的 `docs/architecture.md`、`docs/terminology.md`、`requirements/BEHAVIOR.md` 與 `requirements/REQ-013.md`，因此 frontmatter 不是完整的實際變更清單。
- [ ] [Minor] Transfer AP 測試只使用已達 `maxAP` 的玩家，也未檢查 `player_ap.full_timestamp`。新增非滿 AP 情境並直接檢查 timestamp，才能防止「恢復 AP」的回歸被上限遮蔽。
- [ ] [Minor] Concurrent Pickup 測試以同一位玩家送出兩個請求，未覆蓋 acceptance criterion 指定的多位玩家競爭同一地面 quantity。
- [ ] [Minor] Backend 測試未觸發 Transfer 的 unexpected internal failure，因此沒有鎖定 HTTP 500 不含 authoritative state 的 contract。

## Plan Review Issues

- [x] Update `docs/terminology.md` so Action excludes AP-free Transfer and Item or Resource definitions allow Location ground holdings. Add REQ-013 to the affected Item, Resource, and Location index entries.
- [x] Update `docs/schemas.md` relationship map with both ground table foreign keys. State that existing databases gain empty ground tables through idempotent Store initialization.
- [x] Define the exact JSON entry shapes for `ground_items` and `ground_resources`. Limit the authoritative-state failure contract to authenticated handled Transfer failures, because authentication and internal failures cannot always return player state.

- [x] Remove the duplicate unchecked copies of the three resolved Plan Review Issues so the checklist reflects their resolved state.
- [x] Update the `Location` definition and its corresponding-behavior links in `docs/terminology.md` for public ground holdings. The index links `Location` to REQ-013, but the definition still describes only player presence and links only REQ-005 and REQ-010.
