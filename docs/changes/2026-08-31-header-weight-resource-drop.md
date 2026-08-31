---
title: "Header weight and Resource Drop"
status: Done
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - requirements/BEHAVIOR.md
  - requirements/REQ-007.md
  - requirements/REQ-012.md
  - requirements/REQ-013.md
  - requirements/REQ-014.md
  - docs/architecture.md
  - docs/terminology.md
  - docs/changes/2026-08-31-header-weight-resource-drop.md
  - internal/authapi/server.go
  - internal/authapi/store.go
  - internal/authapi/server_test.go
  - internal/authapi/store_test.go
  - web/src/App.tsx
  - web/src/App.test.tsx
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/GameShell.tsx
  - web/src/GameShell.test.tsx
  - web/src/browser-fixture.tsx
  - web/src/styles.css
req_ref: REQ-007, REQ-012, REQ-013, REQ-014
base_branch: main
scope: "Tracks fixed header content, weight presentation, duplicate state removal, and Resource Drop removal."
---

## Problem Statement

The fixed header repeats the player name while weight remains in Map and Resource balances remain in Items. Resource Drop also permits a transfer that the current game rules no longer allow.

## Recommended Direction

Keep AP, HP, weight, and nonzero Resources in the fixed header. Remove duplicate weight and Resource balances from main tabs. Preserve Item Drop and Resource Pickup while rejecting Resource Drop.

## Key Assumptions

- Exactly 75% of the weight limit uses the green state.
- Existing ground Resource remains visible and available for Pickup.
- Resource Drop rejection applies to direct API requests as well as the frontend control.

## Acceptance Criteria

REQ-007, REQ-012, REQ-013, and REQ-014 are the sources of truth. Each task below copies its complete applicable criteria verbatim.

## MVP Scope / Not Doing

- Do not change carrying-weight calculation or the movement threshold.
- Do not remove Item Drop or Resource Pickup.
- Do not migrate or delete existing ground Resource.
- Do not add HP behavior.

## Dependency Graph

`Task 1 backend Transfer rule -> Task 2 frontend shell and tab content -> Task 3 frontend Transfer type`

Task 1 establishes the authoritative rejection. Task 2 removes the Resource Drop control and updates the shell in one compilable commit. Task 3 narrows the frontend request type after its Resource Drop caller is gone.

## Tasks

- [x] Task 1 [parallel: no]: Reject Resource Drop in `internal/authapi/server.go` and `internal/authapi/store.go`. Add API and Store tests in the same commit. Verify AP, player Resource, and ground Resource remain unchanged after rejection. Verify the rejection log contains user ID, Location, asset type, asset identifier, quantity, outcome, reason, and request ID without credentials. Add regressions for Active and Expired Item Drop, Resource Pickup, and existing ground Resource visibility.
  - REQ-013.1: 每個 Location 必須具有獨立的地面 Item 與 Resource 狀態。
  - REQ-013.2: 地面資產不得限制總重量、quantity 或堆疊數量。
  - REQ-013.3: 地面資產不得具有 owner、存取權限或預留機制。
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的有效或失效 Item Drop 至目前 Location。
  - REQ-013.6: 後端必須拒絕玩家把 Resource Drop 至任何 Location，且所有狀態保持不變。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的有效地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.9: Item 必須依 Location、Item type 與有效狀態合併 quantity。有效與失效 Item 不得合併。
  - REQ-013.10: Resource 必須依 Location 與 Resource type 合併 quantity，不得轉換成 Item。
  - REQ-013.11: Pickup 與 Drop 必須屬於 Transfer，不得視為 Action。
  - REQ-013.12: Transfer 成功或失敗時都不得消耗或恢復 AP。
  - REQ-013.13: Transfer 成功時，來源扣除與目的增加必須原子更新，且總 quantity 必須保持不變。
  - REQ-013.14: 玩家持有量或地面持有量不足，或玩家嘗試 Pickup 失效 Item 時，Transfer 必須失敗，且所有狀態保持不變。
  - REQ-013.15: 玩家不得 Pickup 或 Drop 目前 Location 以外的地面資產。
  - REQ-013.16: 多位玩家同時 Pickup 時，地面 quantity 不得低於 0，且成功取得的總量不得超過原有 quantity。
  - REQ-013.17: 地面資產與 Transfer 結果必須在重新整理、重新登入及其他玩家讀取後保持一致。
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 必須提供 Drop，Resource 不得提供 Drop。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-013.20: Backend 必須把 Transfer access 與結果寫入 stdout，並包含 user ID、Location、asset type、asset identifier、quantity、結果與 request ID。
  - REQ-013.21: Backend log 不得包含 credentials、session、OAuth 資料、cookie 或未處理的原始輸入。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Pickup 不得指定 Item 狀態。
  - REQ-013.23: Item 狀態缺少、不合法或不適用時，Transfer 必須失敗，且所有狀態保持不變。

- [x] Task 2 [parallel: no]: Update `web/src/App.tsx` and `web/src/GameShell.tsx`, with presentation support in `web/src/styles.css`. Keep Item Drop in the Items Inventory. Keep ground Item and Resource tables plus Pickup in Area. Remove only the Map weight block, Items Resource balance table, Resource Drop control, and player name header entry. Pass authoritative current and maximum weight to the fixed header. Render row 1 as AP, HP placeholder, and `Weight <current>/<max>`. Use green at or below 75%, yellow above 75% through 100%, and red above 100%. Add App and GameShell tests in the same commit for exact boundaries, retained controls and tables, authoritative state updates, and accessible safe, warning, or overweight names that do not rely only on color. `web/src/styles.css` is a presentation support file; the two source/logic files are App and GameShell.
  - REQ-007.4: 玩家必須能在固定 header 看到持有量大於 0 的 Resource 與 quantity。持有量為 0 的 Resource 不必顯示。
  - REQ-012.2: App Shell 頂端必須固定顯示兩排核心狀態，且不得隨主分頁切換。
  - REQ-012.3: 第一排必須依序顯示目前 AP、目前 HP 與 `Weight <current>/<max>`，不得顯示玩家名稱。
  - REQ-012.4: HP 尚未實作時，第一排必須顯示明確 placeholder，不得顯示虛構數值。
  - REQ-012.5: 第二排必須依固定順序顯示玩家持有量大於 0 的 Resource，且最多包含既有 8 種 Resource。
  - REQ-012.6: 兩排核心狀態都必須維持單行，不得自動換行。
  - REQ-012.7: 任一核心狀態列超過可用寬度時，玩家必須能在該列水平 swipe 查看後續資訊。
  - REQ-012.8: 核心狀態列的水平 swipe 不得造成整個頁面水平溢出，也不得阻止主內容垂直捲動。
  - REQ-012.11: `地圖` 必須顯示玩家目前 Location，以及後端回傳的可抵達 Location、Route 與 AP 成本。
  - REQ-012.12: 玩家必須能在 `地圖` 對後端回傳的 Route 執行 Move。
  - REQ-012.15: `地區` 必須顯示目前 Location 的地面 Item 與 Resource，並保留既有 Pickup 與 Drop 規則。
  - REQ-012.17: `道具` 必須顯示玩家 Inventory，以及 Active 與 Expired Item 的既有狀態與操作。
  - REQ-012.18: `道具` 不得重複顯示 header 已提供的 Resource 持有量。
  - REQ-012.22: 前端只能顯示後端依 REQ-018 回傳的 Action、target、method 與 recipe，不得自行補回不可用選項。
  - REQ-012.23: Action 完成後，頂端核心狀態與目前主分頁必須使用最新後端狀態更新，不得要求完整頁面重新載入。
  - REQ-012.24: 個別主分頁載入或操作失敗時，頂端核心狀態與底部主分頁必須保持可見。
  - REQ-012.25: `Weight` 小於或等於上限 75% 時必須顯示綠色，超過 75% 且小於或等於上限時必須顯示黃色，超過上限時必須顯示紅色。
  - REQ-012.26: `地圖` 與其他主內容不得重複顯示 header 已提供的目前重量與重量上限。
  - REQ-013.4: 玩家必須能查看目前 Location 的所有地面 Item 與 Resource quantity。
  - REQ-013.5: 玩家必須能指定正整數 quantity，將持有的有效或失效 Item Drop 至目前 Location。
  - REQ-013.6: 後端必須拒絕玩家把 Resource Drop 至任何 Location，且所有狀態保持不變。
  - REQ-013.7: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的有效地面 Item。
  - REQ-013.8: 玩家必須能指定正整數 quantity，Pickup 目前 Location 的任意地面 Resource。
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 必須提供 Drop，Resource 不得提供 Drop。
  - REQ-013.19: Transfer 完成後，前端必須顯示後端回傳的最新玩家與地面狀態。
  - REQ-014.5: Item Drop 維持不消耗 AP，且不受目前重量限制。

- [x] Task 3 [parallel: no]: Narrow `DropRequest` in `web/src/auth.ts` to Item Transfer only after Task 2 removes every Resource Drop caller. Keep `PickupRequest` compatible with Item and Resource. Add frontend contract tests in the same commit that reject Resource Drop at the type boundary and preserve Resource Pickup serialization.
  - REQ-013.6: 後端必須拒絕玩家把 Resource Drop 至任何 Location，且所有狀態保持不變。
  - REQ-013.18: 前端必須用表格顯示地面 Item 與 Resource。有效 Item 與 Resource 必須提供 Pickup。玩家持有的有效或失效 Item 必須提供 Drop，Resource 不得提供 Drop。
  - REQ-013.22: Item Transfer 必須指定 `active` 或 `expired` 堆疊。Resource Pickup 不得指定 Item 狀態。
  - REQ-013.23: Item 狀態缺少、不合法或不適用時，Transfer 必須失敗，且所有狀態保持不變。

## Task 3 Verification

- `npm test -- --run` passed: 146 tests.
- `npm run build` passed: TypeScript compilation and Vite production build.
- `DropRequest` accepts only Item transfers, while `PickupRequest` accepts Item and Resource transfers.
- Contract tests reject Resource Drop at the TypeScript boundary and preserve Resource Pickup JSON serialization.

## Task 1 Verification

- `CGO_ENABLED=0 go test ./internal/authapi -run 'TestGroundTransfer|TestResourceDrop|TestItemTransfers'` passed.
- Resource Drop is rejected by Store and API with unchanged AP, player Resource, and ground Resource state.
- Rejection logs include user ID, Location, asset type, asset identifier, quantity, outcome, reason, and request ID without session credentials.

## Review Issues

- [x] [Minor] `Store.Drop` 已先拒絕 Resource，卻保留不可達的 Resource 寫入分支。兩種策略互相衝突，後續調整 guard 會意外恢復禁用行為。

## Refactor Verification

- 移除 `Store.Drop` 中被 `ErrResourceDropNotAllowed` guard 排除的 Resource 寫入分支，行為不變。
- `CGO_ENABLED=0 go test ./...` 通過。
- `doc_health_check.py` 通過，架構與術語文件無需更新。

## Task 2 Verification

- `npm test -- --run` passed: 145 tests.
- `npm run build` passed: TypeScript compilation and Vite production build.
- Fixed header now renders AP, HP placeholder, and authoritative `Weight <current>/<max>` without player name.
- Weight boundaries use safe at 75%, warning above 75% through 100%, and overweight above 100%, with accessible state names.
- Items retains Item Drop and removes Resource balance and Resource Drop. Area retains ground Item and Resource tables with Pickup.

## Plan Review Issues

- [x] Task 2 與架構文件誤移 Item Drop。需求未授權改分頁。Item Drop 必須留在 `道具` Inventory。`地區` 只保留地面資產與 Pickup。
- [x] Task 1 的拒絕測試不完整。API 與 Store 都必須拒絕 Resource Drop。測試必須確認 AP 與兩側狀態不變。測試也必須檢查清理後的必要 log 欄位。Item Drop、Resource Pickup 與地面 Resource 必須回歸。
- [x] Task 2 的移除範圍無對應測試。只可移除玩家 Resource balance、Map weight 與 Resource Drop。`地區` 的 ground Resource 表格、quantity 與 Pickup 必須保留。Transfer 後也必須套用最新狀態。
- [x] Task 3 需要 `App.tsx` 傳入兩個重量值。該檔只列在 Task 2。計畫未定義可逐 task 編譯的介面交接。Dependency graph 也同時宣告 Task 3 依賴及平行於 Task 2。
- [x] Task 3 缺少 75% 邊界與無障礙測試。測試必須涵蓋等於及剛超過 75%。也必須涵蓋等於及超過上限。重量狀態必須提供非僅靠顏色的可存取名稱。
- [x] Task 4 沒有同 task 驗證。兩份文件也已在 plan 階段修改。計畫必須改成可執行的文件檢查。架構文件必須先修正 Item Drop 分頁。schema 與 `docs/schemas.md` 的決策可保留。
- [x] `source_paths` 遺漏已修改的需求索引與四份 REQ。保留目前 `Draft` 狀態。所有問題解決並重審後才能改狀態。
