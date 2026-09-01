---
title: "Location monsters and combat"
status: Issues-confirmed
created: 2026-08-31
doc_type: change
last_reviewed: 2026-09-01
source_paths:
  - docs/architecture.md
  - docs/changes/2026-08-31-monsters-combat.md
  - docs/schemas.md
  - docs/terminology.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server_test.go
  - internal/authapi/server.go
  - web/src/auth.ts
  - web/src/App.test.tsx
  - web/src/GameShell.tsx
  - web/src/auth.test.ts
req_ref: REQ-005, REQ-012, REQ-018, REQ-021, REQ-022, REQ-025, REQ-026
base_branch: main
scope: "Tracks lazy Location Monster population, movement interception, automatic combat, HP recovery, drops, and the mobile interface."
---

## Problem Statement

Locations have no persistent Monster pressure. Players cannot fight, lose HP, receive Monster drops, or face risk when leaving an occupied Location.

## Recommended Direction

Settle each complete Monster interval with one backend random roll until the Location reaches its configured cap. Resolve attacks and movement interceptions in one backend transaction. Return the combat transcript and updated state to the existing mobile interface.

## Key Assumptions

- Monster type is selected only when combat starts.
- One request resolves the complete combat.
- Player HP uses one timestamp that represents recovery to full HP.
- Random generation, interception, combat damage, and drops use backend-owned randomness.

## Acceptance Criteria

REQ-005, REQ-012, REQ-018, REQ-021, REQ-022, REQ-025, and REQ-026 are the sources of truth.

## MVP Scope / Not Doing

- Do not add equipment, defense, skills, critical hits, accuracy, evasion, or player combat choices.
- Do not add background workers or scheduled Monster generation.
- Do not expose unavailable attacks or unselected Monster types.
- Do not add Monster movement, individual persistent Monster records, or combat sessions.

## Tasks

Dependency graph: `Task 1 -> Task 2 -> Task 3 -> Task 4 -> Task 5`.

- [x] Task 1 [parallel: no]: Add only the SQLite definitions and Store state needed for player HP, Monster types, drops, Location rules, encounter weights, and aggregate Location populations in `internal/authapi/store.go`. Seed camp, forest_edge, Forest Rat, Rat Tail, and full HP for existing and new players. Add Store tests and update `docs/schemas.md` and `docs/terminology.md`.
  - REQ-025.1: 每個 Location 必須定義生成間隔秒數、單次生成機率、Monster 數量上限與單隻攔截機率。
  - REQ-025.2: 每個會生成 Monster 的 Location 必須用正整數外部鍵引用一種以上 Monster type，並為每種 type 定義正整數 encounter weight。
  - REQ-025.3: 系統必須依 Location 保存尚未抽選 type 的 Monster 總數與最近一次結算時間。
  - REQ-025.12: `camp` 必須使用 1800 秒生成間隔、0% 生成機率、0 數量上限與 0% 單隻攔截率。
  - REQ-025.13: `forest_edge` 必須使用 1800 秒生成間隔、50% 生成機率、10 數量上限與 10% 單隻攔截率。
  - REQ-025.14: `forest_edge` 必須只抽選 Forest Rat，encounter weight 為 1。
  - REQ-025.16: 重新整理、重新登入或重啟 backend 後，系統必須保留 Location Monster 總數與結算時間。
  - REQ-026.1: 玩家 HP 上限、HP 恢復間隔與空手攻擊力必須由 database definition 提供，不得寫死於戰鬥流程。
  - REQ-026.2: MVP 玩家 HP 上限必須為 100，每 60 秒恢復 1 HP，空手攻擊力必須為 3。
  - REQ-026.3: 玩家 HP 狀態只能保存恢復至滿 HP 的 UTC Unix seconds timestamp，不得保存目前 HP 數值。
  - REQ-026.4: 系統必須依 backend current timestamp 計算目前 HP，且非戰鬥狀態的 HP 必須介於 1 至 100。
  - REQ-026.5: 既有玩家與新玩家第一次取得 HP 狀態時必須具有 100 HP。
  - REQ-026.14: Forest Rat 必須使用正整數 type ID，並定義 10 HP 與 2 攻擊力。
  - REQ-026.16: Forest Rat 必須具有 Rat Tail 掉落規則，每次勝利有 50% 機率產生 1 個 Rat Tail Item。
  - REQ-026.17: Rat Tail 必須是單位重量 1 的 Item，並依 REQ-015 取得完整耐久時間及合併至正確 Inventory 堆疊。

- [x] Task 2 [parallel: no]: Add lazy Location Monster settlement and executable Attack filtering in `internal/authapi/store.go`. For each complete interval, use one standard backend random roll until the configured cap, then advance the saved settlement timestamp across every complete interval. Return the settlement values required by REQ-025.17 to the server. Add Store tests for elapsed intervals, caps, downtime, persistence, filtering, and computation values.
  - REQ-018.1: 後端回傳玩家狀態時，只能包含玩家依目前 authoritative state 可以執行的 Action、target、method 與 recipe。
  - REQ-018.2: 可執行性必須由後端依 Action 定義、玩家狀態、目前 Location、target 狀態、權限、AP、Resource inputs 與 Active Item inputs 判定。
  - REQ-018.3: 可執行性判定不得修改任何玩家或世界狀態。
  - REQ-018.4: 玩家狀態或世界狀態改變後，後端必須重新計算可回傳的遊戲選項。
  - REQ-018.18: `attack` 必須依目前 Location 的 Monster 數量與玩家 AP 判定。
  - REQ-018.19: `move` 的 Route 不得因目前 Location 具有 Monster 而隱藏，攔截必須在玩家提交 `move` 後判定。
  - REQ-018.20: `attack` 不可執行時，其 identifier、AP 成本與 Monster 資訊不得出現在可執行選項中。
  - REQ-018.21: 目前 Location 的 Monster 總數屬於世界狀態，不得因 `attack` 不可執行而隱藏。
  - REQ-025.4: 每經過一個完整生成間隔，系統必須獨立抽選一次是否增加 1 隻 Monster。
  - REQ-025.5: 生成成功時，系統必須把該 Location 的 Monster 總數增加 1，但不得超過數量上限。
  - REQ-025.6: Monster 總數已達上限時，系統仍必須結算已經過的完整間隔，不得在數量下降後補抽先前略過的間隔。
  - REQ-025.7: 後端必須在讀取 authoritative Location 狀態、合法 `attack` 通過 AP 檢查後判斷 Monster 數量，或合法 `move` 判斷攔截前，依目前 UTC Unix seconds 結算並保存所有完整生成間隔。
  - REQ-025.8: Fly Machine 停止或沒有 request 的時間仍必須依 timestamp 納入下次結算，不得依賴背景排程。
  - REQ-025.9: Monster 生成時不得預先抽選 type。系統只能在戰鬥開始時依 encounter weight 抽選本次 Monster type。
  - REQ-025.15: 多個 request 同時結算、攻擊或攔截時，Location Monster 總數不得超過上限或低於 0，同一隻 Monster 不得產生多次勝利掉落。
  - REQ-025.16: 重新整理、重新登入或重啟 backend 後，系統必須保留 Location Monster 總數與結算時間。

- [x] Task 3 [parallel: no]: Add automatic combat, active Attack, and Move interception to `internal/authapi/store.go`. Use standard backend random rolls for interception, encounter selection, damage, and drops. Keep combat state changes in the existing SQLite transaction. Return the interception values required by REQ-025.17 and combat values required by REQ-026.29 to the server. Add Store tests for both outcomes, AP, HP, drops, interception, concurrent final-Monster attacks, and computation values.
  - REQ-005.6: `move` 未被 Monster 攔截時，系統必須扣除該 Route 的完整 AP 成本，更新目前位置，並保存兩項結果。
  - REQ-005.15: 玩家從具有 Monster 的目前 Location 執行 `move` 時，系統必須在扣除 AP 或更新位置前計算攔截結果。
  - REQ-005.16: `move` 被攔截時，系統不得扣除 Route AP，且不得更新玩家位置。
  - REQ-005.17: `move` 被攔截時，玩家必須立即依 REQ-026 進入戰鬥。
  - REQ-005.18: 攔截戰鬥結束後，玩家必須停留在原本 Location，不得自動繼續移動。
  - REQ-005.19: `move` 回應必須包含攔截戰鬥結果與最新 authoritative player state。
  - REQ-025.10: 目前 Location 具有 `N` 隻 Monster 時，總攔截率必須為 `1 - (1 - 單隻攔截率)^N`。
  - REQ-025.11: 攔截只能由玩家嘗試離開具有 Monster 的目前 Location 觸發，目的地的 Monster 不得攔截本次移動。
  - REQ-025.15: 多個 request 同時結算、攻擊或攔截時，Location Monster 總數不得超過上限或低於 0，同一隻 Monster 不得產生多次勝利掉落。
  - REQ-026.6: 玩家主動執行 `attack` 必須消耗 30 AP。攔截觸發的戰鬥不得消耗 AP。
  - REQ-026.7: 主動 `attack` 只能在目前 Location 至少具有 1 隻 Monster 且玩家具有 30 AP 時執行。
  - REQ-026.8: 戰鬥開始時，系統必須依目前 Location 的 encounter weight 抽選一種 Monster type，並載入該 type 的 HP、攻擊力與掉落規則。
  - REQ-026.9: 戰鬥必須在單次 request 內由 backend 自動結算，不得等待玩家逐回合輸入。
  - REQ-026.10: 每回合必須由玩家先攻擊一次。Monster HP 歸零時，本回合必須立即結束，Monster 不得反擊。
  - REQ-026.11: 玩家攻擊後 Monster 仍存活時，Monster 必須攻擊玩家一次。玩家 HP 歸零時，戰鬥必須立即結束。
  - REQ-026.12: 每次攻擊傷害必須在 `ceil(攻擊力 × 0.5)` 至 `floor(攻擊力 × 1.5)` 之間均勻抽選整數，且傷害最低為 1。
  - REQ-026.13: 玩家與 Monster 必須使用相同的傷害公式，且戰鬥不得套用防禦、減傷、命中、閃避、暴擊或技能。
  - REQ-026.15: Monster HP 歸零時，玩家必須獲勝。系統必須把目前 Location 的 Monster 總數減少 1，並分別依每項掉落機率抽選掉落物。
  - REQ-026.18: 玩家 HP 歸零時，Monster 必須獲勝。系統必須把玩家 HP 設為 1，不得減少 Monster 總數或產生掉落物。
  - REQ-026.19: 主動 `attack` 開始後，無論勝負都必須扣除完整 30 AP。攔截戰鬥無論勝負都不得修改 AP。
  - REQ-026.20: 戰鬥結束時，玩家 HP、AP、Monster 總數與掉落物必須在同一 transaction 內保存。
  - REQ-026.21: 主動 `attack` 因輸入不合法、AP 不足或 Monster 數量以外的 Action 條件不符而失敗時，後端必須在 Location Monster 結算前拒絕，且所有玩家與世界狀態都保持不變。
  - REQ-026.22: 合法 `attack` 具有 30 AP 時，後端必須先保存 Location Monster 時間結算；結算後 Monster 數量為 0 時必須拒絕戰鬥，不扣除 AP，也不修改 HP、Inventory 或戰鬥造成的 Monster 數量。
  - REQ-026.23: 多位玩家同時攻擊最後一隻 Monster 時，最多只能有一位玩家開始戰鬥並取得勝利掉落。

- [x] Task 4 [parallel: no]: Add the dedicated Attack endpoint, combat response, and required access/computation logs in `internal/authapi/server.go`. Extend `web/src/auth.ts` to send empty Attack input and parse Attack or intercepted Move results. Add server and frontend client tests for exact request, response, failure, state, and log behavior.
  - REQ-018.5: 前端只能顯示後端回傳的 Action、target、method 與 recipe，不得自行推定或補回未回傳的選項。
  - REQ-018.6: 玩家無法執行的選項，其 identifier、顯示名稱、成本、inputs、output、機率與 capacity 都不得出現在前端可取得的回應中。
  - REQ-018.17: 未回傳的 Action、target、method 或 recipe 仍必須由後端拒絕直接提交，且不得修改任何狀態。
  - REQ-025.17: Backend 必須把每次 Monster 生成與攔截計算結果寫入 stdout，並包含 Location、間隔數、機率、結算前後數量、outcome 與 request ID。
  - REQ-025.18: Backend log 不得包含 credentials、session、OAuth 資料、cookie、secrets 或未處理的原始輸入。
  - REQ-026.21: 主動 `attack` 因輸入不合法、AP 不足或 Monster 數量以外的 Action 條件不符而失敗時，後端必須在 Location Monster 結算前拒絕，且所有玩家與世界狀態都保持不變。
  - REQ-026.22: 合法 `attack` 具有 30 AP 時，後端必須先保存 Location Monster 時間結算；結算後 Monster 數量為 0 時必須拒絕戰鬥，不扣除 AP，也不修改 HP、Inventory 或戰鬥造成的 Monster 數量。
  - REQ-026.24: 戰鬥回應必須依發生順序列出每次攻擊的攻擊者、傷害與目標剩餘 HP。
  - REQ-026.25: 戰鬥回應必須包含 Monster 顯示名稱、勝負結果、掉落物，以及最新玩家 HP、AP、Location 與 Monster 總數。
  - REQ-026.29: Backend 必須把每次 type 抽選、傷害、掉落、HP、AP 與 Monster 數量計算結果寫入 stdout，並包含 user ID、Location、action、outcome 與 request ID。
  - REQ-026.30: Backend log 不得包含 credentials、session、OAuth 資料、cookie、secrets 或未處理的原始輸入。
  - REQ-026.31: `attack` 必須屬於 Action，並透過專用 endpoint 接受空 JSON object。前端不得指定 Monster type、AP 成本、傷害、掉落或戰鬥結果。
  - REQ-026.32: Backend 必須把 `attack` 與攔截戰鬥的 success 或 failure access 結果寫入 stdout，並包含 user ID、Location、action、outcome、拒絕原因與 request ID。

- [x] Task 5 [parallel: no]: Display authoritative HP in `web/src/GameShell.tsx`. Display Monster count, backend-gated Attack, and one combat transcript in `web/src/App.tsx`. Apply returned state without reload. Add component tests for Attack and intercepted Move.
  - REQ-012.3: 第一排必須依序顯示目前 AP、目前 HP 與 `Weight <current>/<max>`，不得顯示玩家名稱。
  - REQ-012.4: 第一排必須顯示後端回傳的目前 HP，不得顯示 placeholder 或前端推算值。
  - REQ-012.22: 前端只能顯示後端依 REQ-018 回傳的 Action、target、method 與 recipe，不得自行補回不可用選項。
  - REQ-012.23: Action 完成後，頂端核心狀態與目前主分頁必須使用最新後端狀態更新，不得要求完整頁面重新載入。
  - REQ-012.24: 個別主分頁載入或操作失敗時，頂端核心狀態與底部主分頁必須保持可見。
  - REQ-021.4: `move` 被 Monster 攔截時，`地圖` 必須顯示戰鬥過程與結果，並保持玩家位於原本 Location。
  - REQ-022.5: `地區` 必須顯示目前 Location 的 Monster 總數，不得在戰鬥前顯示尚未抽選的 Monster 種類。
  - REQ-022.6: 後端回傳 `attack` 時，`地區` 必須提供主動攻擊 Monster 的操作。
  - REQ-022.7: 主動攻擊完成後，`地區` 必須顯示戰鬥過程、結果、掉落物與最新 Monster 總數。
  - REQ-026.26: 主動攻擊完成後，前端必須顯示戰鬥過程與結果，不得要求重新載入頁面。
  - REQ-026.27: 移動遭到攔截時，前端必須顯示相同格式的戰鬥過程與結果，不得要求重新載入頁面。
  - REQ-026.28: 重新整理、重新登入或重啟 backend 後，系統必須顯示依保存 timestamp 計算的目前 HP，以及已保存的 Monster 與掉落結果。

## Plan Review Issues

- [x] [Trace] Task 3 未逐字納入本次修改的 REQ-005.6：`move` 未被 Monster 攔截時，系統必須扣除 Route 的完整 AP 成本、更新位置並保存兩項結果。補上這條準則，讓未攔截分支具有完整 trace。
- [x] [Docs] `docs/terminology.md` 的 Action 定義仍宣稱玩家執行 Action 就會消耗 AP。這與 REQ-005.16、REQ-026.6 及 REQ-026.19 的攔截分支不符。修正文義，但不得擴增 REQ 外的行為。
- [x] [Task separation] Task 4 依 REQ-025.17 與 REQ-026.29 記錄每次計算，但它只修改 `server.go` 與 `auth.ts`。Task 2 與 Task 3 未要求 `store.go` 提供 interval、機率、前後數量、type、傷害、掉落、HP 與 AP 的計算結果。加入最小資料交接，避免 Task 4 需要未規劃的跨 task 修改。
- [x] [Docs follow-up] `docs/terminology.md` 的 Action 定義已修正，但同一條目的差異說明仍寫「Action 消耗 AP」。這仍與 REQ-005.16、REQ-026.6 及 REQ-026.19 的攔截分支不符。
- [x] [Task separation follow-up] Task 3 只交接 REQ-026.29 的 Combat 計算值，未交接 REQ-025.17 要求的攔截機率與 outcome。Task 4 無法只修改 `server.go` 與 `auth.ts` 就記錄每次攔截計算。

## Review Issues

- [ ] [Major] REQ-018.18 要求 `attack` 依 Monster 數量與 AP 判定，但 `filterAvailableGameplayOptions` 從未呼叫 `canAttack` 或加入 `attack`。所有真實 player state 都會漏掉 `attack`，因此 REQ-022.6 的 `地區` 操作永遠不會出現。把判定接入 response filtering，並用 server response 測試取代目前只測未使用 helper 的測試。
- [x] [Major] REQ-025.17 要求記錄每次 Monster 生成計算，但 `Attack` 與 `MoveWithCombat` 先取得 settlement computation，再以 `getPlayerStateTx` 覆寫它。無 Monster 的 Attack 更直接回傳沒有 computation 的 state。`logMonsterSettlement` 隨後記錄第二次零間隔結算或捏造的 0 機率結果，遺失實際 interval、機率、前後數量與 outcome。保留並逐筆記錄本次 request 的實際結算，包含成功移動時的 origin 與 destination 結算。
- [x] [Major] REQ-025.10 要求總攔截率等於 `1 - (1 - 單隻攔截率)^N`，但 `combinedInterceptionChanceBPS` 向上取整後用一次 basis-point roll。5 隻 Monster、單隻 10% 時，要求值是 40.951%，實作值是 40.96%。刪除改變機率的向上取整，讓實際抽選符合公式。
- [x] [Major] REQ-026.12 要求傷害均勻抽選整數，但 `damageRoll` 把 10000 個 roll 直接切成傷害區間。MVP 攻擊力 3 的三個結果分別取得 3334、3333、3333 個 roll，並不均勻。改用無偏差的整數抽選。
- [ ] [Major] REQ-026.32 要求每個 `attack` failure access log 都包含 user ID、Location、action、outcome、拒絕原因與 request ID。未登入或 session 無效時，`requirePlayerName` 仍交給 `attack` 回傳 401，但沒有設定 Location 與拒絕原因。補齊 `unknown` Location 與 authentication failure reason，並加入 access log 測試。
- [ ] [Major] REQ-012.4 要求顯示後端回傳的 HP，不得使用 placeholder 或前端推算值。`GameShellPlayer.hp` 卻是 optional，缺值時會顯示前端捏造的 `0`。刪除 fallback，並把 HP 改成 required state。
- [ ] [Major] REQ-026.22 與 REQ-026.31 只需要單一 no-Monster error 與空 object validation。`ErrNoMonsters` alias 和 `attackReasonNonEmpty` 沒有任何 caller，也不服務 copied criterion 或既有 convention。刪除這兩個 dead paths。
- [ ] [Major] REQ-026.25 只要求 Combat response 包含最新狀態，但 `docs/architecture.md` 把既有敘述擴張成所有 Action responses 都包含 authoritative state。Attack 的 insufficient-AP response 實際只有 error。刪除這項 REQ 外的廣泛承諾，或把敘述縮限至已要求且已實作的 response。
- [ ] [Minor] `source_paths` 漏列實際修改的 `web/src/App.tsx` 與 `web/src/GameShell.test.tsx`。
- [ ] [Minor] `docs/schemas.md` 的 `player_combat_definitions.id` 漏掉實際 schema 的 `CHECK (id > 0)`。
- [ ] [Minor] REQ-005.19 要求 intercepted Move response 包含 Combat 與最新 state，但 server tests 沒有呼叫 intercepted Move endpoint。現有 Store 與 frontend 測試無法在 handler 遺漏 `combat` 時失敗。
