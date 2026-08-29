---
title: "Sawmill product chain"
status: Issues-confirmed
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths:
  - CHANGELOG.md
  - docs/architecture.md
  - docs/changes/2026-08-29-item-durability.md
  - docs/changes/2026-08-29-sawmill-product-chain.md
  - docs/schemas.md
  - docs/terminology.md
  - internal/authapi/store.go
  - internal/authapi/server_test.go
  - internal/authapi/server.go
  - internal/authapi/store_durability_attribution_test.go
  - internal/authapi/store_durability_overflow_test.go
  - internal/authapi/store_test.go
  - requirements/BEHAVIOR.md
  - requirements/REQ-008.md
  - requirements/REQ-010.md
  - requirements/REQ-011.md
  - requirements/REQ-013.md
  - requirements/REQ-014.md
  - requirements/REQ-015.md
  - requirements/REQ-016.md
  - requirements/REQ-017.md
  - web/src/App.test.tsx
  - web/src/auth.ts
  - web/src/App.tsx
  - web/src/auth.test.ts
req_ref: [REQ-008, REQ-010, REQ-011, REQ-014, REQ-015, REQ-016, REQ-017]
base_branch: main
scope: "Tracks configurable Convert methods, the generic Building extension lifecycle, and Sawmill T1."
---

## Problem Statement

Convert currently uses one fixed Camp-only rule. The game has no Essence output, extension construction, or Building function that improves processing capacity.

## Recommended Direction

Store balance values in typed domain definitions. Keep deterministic formulas in code. Separate Convert, Building durability, Item lifecycle, extension lifecycle, and Sawmill product behavior across their existing responsibility boundaries.

## Key Assumptions

- Each Convert execution consumes one complete AP work unit.
- Only construction AP is copied into an installed extension snapshot.
- Existing Sawmills read current capacity and Building durability cost from their definition.
- A completed extension is available to every player at the same Location until Building permissions are introduced in a separate change.
- Random Essence outcomes use an injectable deterministic source in tests.
- Store initialization seeds missing balance rows without overwriting existing rows. Direct SQLite edits persist across restarts. Future repository defaults require a separate migration change. The MVP has no administration interface.
- `POST /api/actions/convert` accepts `method_id`, positive `quantity`, and an optional `provider_extension_id`.
- `POST /api/actions/install-extension` accepts `building_id`, `slot_index`, and `definition_id`.
- `POST /api/actions/contribute-extension-construction` accepts `extension_id` and positive `ap`.
- `POST /api/actions/remove-extension` accepts `extension_id`.
- Convert responses include the resulting player state and the Essence quantity gained by that execution.

## Acceptance Criteria

The confirmed requirements referenced by `req_ref` are the source of truth.

## MVP Scope / Not Doing

- No Sawmill T2 or higher tier.
- No Essence type other than Wood Essence T1.
- No supporting Resource in the Sawmill Package T1 recipe.
- No Building access mode, co-owner, user role, or ownership transfer.
- No item quality or Craft success probability.
- No generic balance key-value table.

## Tasks

Dependency graph: Task 1 -> Task 2 -> Task 3 -> Task 4 -> Task 5.

- [x] Task 1 [parallel: no]: Add typed SQLite definitions, migrations, seeds, and state models in `internal/authapi/store.go`. Seed only missing rows and test that Store reinitialization preserves direct balance edits. Update store tests and `docs/schemas.md` in the same commit.
  - REQ-008.2: 每個 Convert method 必須具有穩定 identifier 與顯示名稱。
  - REQ-008.3: 每個 Convert method 必須分別定義 AP 成本、input Item、單次 input quantity 上限、output Resource、每單位 input 的 Resource quantity、Essence Item、Essence 機率與 Essence quantity。
  - REQ-014.11: 每種 Item 必須分別定義單位重量，不能依 Item 類別名稱推導重量。
  - REQ-014.12: Wood Essence T1 每單位重量必須為 1。
  - REQ-014.13: Sawmill Package T1 每單位重量必須為 10。
  - REQ-015.1: 每種 Item 必須分別定義耐久時間上限。
  - REQ-015.2: 所有 Item 的耐久時間上限都必須為 1 小時。
  - REQ-016.1: 每個 Building extension definition 必須具有穩定 identifier、顯示名稱、tier、Package Item 與施工所需 AP。
  - REQ-017.1: `Sawmill Package T1` 必須具有後端定義的 Craft recipe。
  - REQ-017.2: Sawmill Package T1 recipe 必須消耗 30 AP、10 Wood Resource 與 1 個 Active Wood Essence T1，並產出 1 個 Sawmill Package T1。
  - REQ-017.3: Sawmill Package T1 recipe 不得使用其他 Resource 或其他 Essence。
  - REQ-017.4: `Sawmill T1` extension definition 必須使用 Sawmill Package T1，且施工需要 30 AP。
- [x] Task 2 [parallel: no]: Implement generic extension installation, shared construction, completion, use eligibility, removal, and parent lifecycle in `internal/authapi/store.go`. Update store tests in the same commit.
  - REQ-011.23: Disabled Building 不得安裝 extension、增加 extension 施工 progress 或提供 extension 功能。
  - REQ-011.24: Building 維修並恢復 Active 後，原有 extension 必須恢復施工或使用，不需要重新安裝。
  - REQ-011.25: Building 永久消失時，其 extension 必須一併永久消失。
  - REQ-016.2: 每個 completed Building 必須依 Building 建立時保存的 extension slot 數量提供空 slot。
  - REQ-016.3: Building owner 必須能把 definition 指定的 Active Package Item 安裝至自己目前 Location 的 completed Building 空 slot。
  - REQ-016.4: 安裝成功時，系統必須消耗 Package Item，並在指定 slot 建立 progress 0 的施工中 extension。
  - REQ-016.5: 安裝後的 extension 必須保存 definition identifier、顯示名稱、tier 與建立時的施工所需 AP。
  - REQ-016.6: Package Item 扣除與施工中 extension 建立必須原子更新。
  - REQ-016.7: Package Item 不足、Building 不符合條件、slot 已占用或 definition 不存在時，安裝必須失敗，且所有狀態保持不變。
  - REQ-016.8: 同一 Location 的所有玩家都能對施工中的 extension 投入自己的正整數 AP。
  - REQ-016.9: 系統必須以剩餘施工 AP 限制實際投入量，且只能扣除實際投入量。
  - REQ-016.10: AP 扣除與施工 progress 增加必須原子更新。
  - REQ-016.11: AP 不足、玩家不在同一 Location、extension 不在施工中或輸入不合法時，施工必須失敗，且所有狀態保持不變。
  - REQ-016.12: progress 達到施工需求時，extension 必須立即 completed。
  - REQ-016.13: Extension completed 前不得提供其功能。
  - REQ-016.15: Building owner 必須能拆除自己 Building 中施工中或 completed 的 extension。
  - REQ-016.16: 拆除 extension 時不得返還已消耗的 Package Item 或已投入 AP。
  - REQ-016.17: Extension 被拆除後，其 slot 必須恢復為空。
  - REQ-016.18: Extension、施工 progress 與 slot 狀態必須在重新整理與重新登入後保持一致。
- [x] Task 3 [parallel: no]: Replace Location-bound conversion with method selection, quantity processing, deterministic Essence rolls, Sawmill provider eligibility, and atomic parent durability use in `internal/authapi/store.go`. Update store tests in the same commit.
  - REQ-008.1: 已登入玩家必須能在任何 Location 使用徒手 Convert method。
  - REQ-008.6: 每次 Convert 必須扣除該 method 定義的一個完整 AP 成本，實際 quantity 不得改變該次 AP 成本。
  - REQ-008.7: Convert 成功時，必須依實際 quantity 扣除 Active input Item，並依每單位產量增加 output Resource。
  - REQ-008.8: 每個被 Convert 的 input Item 必須分別依該 method 定義的機率判定 Essence 結果。
  - REQ-008.9: Convert 取得的 Essence 必須加入玩家 Inventory。
  - REQ-008.10: AP、input Item、output Resource 與 Essence 結果必須原子更新。
  - REQ-008.11: AP、Active input Item 或 method 使用條件不足時，Convert 必須失敗，且所有狀態保持不變。
  - REQ-008.13: 徒手 Wood Convert method 必須消耗 30 AP，單次最多處理 3 個 Wood，每個 Wood 產出 1 Wood Resource，並各自具有 10% 機率產出 1 個 Wood Essence T1。
  - REQ-008.14: Convert 結果必須在重新整理與重新登入後保持一致。
  - REQ-011.26: 合法的 extension 使用使 Building 剩餘耐久低於 0 時，本次使用必須成功，剩餘耐久必須設為 0，Building 隨後必須成為 Disabled。
  - REQ-011.27: Extension 的功能結果與 Building 耐久變化必須原子更新。
  - REQ-015.4: 任何成功操作新產生的 Item 必須取得該 Item 的完整耐久時間。
  - REQ-016.14: 同一 Location 的所有玩家都能使用 completed extension 提供的功能。
  - REQ-017.5: Completed Sawmill T1 必須提供 Sawmill Convert method。
  - REQ-017.6: Sawmill Convert method 必須消耗 30 AP，單次最多處理 6 個 Wood，每個 Wood 產出 1 Wood Resource，並各自具有 10% 機率產出 1 個 Wood Essence T1。
  - REQ-017.7: 玩家使用 Sawmill Convert method 時，必須指定提供該 method 的 Sawmill T1。
  - REQ-017.8: 每次 Sawmill Convert 成功時，必須減少所屬 Building 的 60 秒耐久時間。
  - REQ-017.9: 已安裝的 Sawmill T1 必須使用目前 definition 的 Convert capacity 與 Building 耐久消耗秒數，不得保存這兩個平衡值的歷史快照。
- [x] Task 4 [parallel: no]: Expose strict JSON contracts, state responses, extension Action endpoints, Convert computation results, and sanitized logs in `internal/authapi/server.go`. Update server tests and `docs/architecture.md` in the same commit.
  - REQ-008.12: 後端必須拒絕不存在的 method、非正整數 quantity、超過 method 上限的 quantity、錯誤格式與未支援欄位。
  - REQ-008.15: 後端必須記錄 user ID、Action、method、quantity、Resource 產量、Essence 判定結果、結果與 request ID。
  - REQ-008.16: Log 不得包含 credentials、session、OAuth 資料、cookie、secret 或未處理的原始輸入。
  - REQ-016.20: 後端必須記錄 user ID、Building、extension、操作、AP、結果與 request ID。
  - REQ-016.21: Log 不得包含 credentials、session、OAuth 資料、cookie、secret 或未處理的原始輸入。
- [x] Task 5 [parallel: no]: Parse the expanded player state, implement typed clients, and render compact Convert and extension controls in `web/src/auth.ts` and `web/src/App.tsx`. Update frontend tests and `docs/terminology.md` in the same commit.
  - REQ-008.4: 前端必須顯示後端提供的 Convert method、AP 成本、單次 input quantity 上限、Resource 產量與 Essence 機率。
  - REQ-008.5: 玩家必須選擇後端提供的 Convert method 與不超過該 method 上限的正整數 quantity。
  - REQ-016.19: 前端必須顯示空 slot、extension 顯示名稱、tier、施工狀態、progress、所需 AP 與玩家可用操作。
  - REQ-017.10: 前端必須顯示 `Sawmill T1` 與 `Sawmill Package T1`，不得使用稱號代替 tier。
  - REQ-017.11: 前端必須顯示 Sawmill Package T1 recipe 與 Sawmill Convert method 的成本、inputs、產出、capacity 與 Essence 機率。
  - REQ-017.12: Sawmill Convert 成功後，前端必須顯示最新 AP、Inventory、Resource、Essence 結果與 Building 耐久狀態。

## Review Issues

- [x] [Major] `source_paths` 與 `main...HEAD` 不一致。Diff 還包含 `CHANGELOG.md`、另一份 change doc、兩個 durability 測試、9 份 requirements 文件與 `web/src/App.test.tsx`。
- [x] [Major] 零 Essence 的成功回應會省略 `essence_quantity`。前端要求該欄位存在，因此拒收已落盤的新狀態。
- [x] [Major] Convert 成功訊息未顯示 `essence_quantity`。玩家無法看到該次 Essence 結果。
- [x] [Major] 前端未暴露 extension definitions，也未提供安裝控制。施工中的 extension 也沒有拆除控制。Backend contract 已提供 definitions，待 Task 5 完成前端控制後勾選。
- [x] [Major] Sawmill Convert 未強制選擇 provider。無 provider 時仍可送出，後端再把 `ErrExtensionNotFound` 回成 500。
- [x] [Major] Convert 以 `methodID != "hand_wood_t1"` 判定 provider 需求。`global_conversion_methods` 的設定未參與判定。
- [x] [Major] Extension 日誌仍未使用 request metadata。AP 固定為 0，Building 與 extension 取自 mutation 後狀態的最後一筆，移除與失敗操作會記錯目標。
- [x] [Major] 超過 method capacity 的 Convert 會回傳 500。後端未把 `ErrInvalidArgument` 映射為無效 action 回應。
- [x] [Major] `install-extension` 缺少 `slot_index` 時仍使用預設值 0。無效 payload 可以消耗 Package 並安裝 extension。
- [x] [Major] 前端向非 owner 顯示拆除控制，也向 Disabled Building 顯示安裝與施工控制。這些不是玩家可用操作。
- [x] [Minor] `source_paths` 仍重複列出 `docs/terminology.md`，因此未與 `main...HEAD` 檔案清單精確唯一匹配。
- [x] [Major] 前端對 Disabled Building 隱藏拆除按鈕。Owner 因此無法拆除該 Building 的施工中或 completed extension。
- [x] [Minor] 超過 method capacity 時，後端回傳並記錄 `invalid provider extension`。拒絕原因與實際 quantity 錯誤不符。

## Plan Review Issues

- [x] `docs/schemas.md` 在建立 `wood_essence_t1` 前先 seed `conversion_methods`。已改為先 seed 所有 referenced Item，再 seed method。
- [x] `Key Assumptions` 指定 versioned SQLite migrations，但 schema 文件仍明載沒有 schema version。MVP 改為只 seed 缺少的 row，並測試 Store reinitialization 保留 direct balance edits。Future repository defaults 留給獨立 migration change。
- [x] Task 1、4、5 各自列出無法由該 task 交付的 acceptance criteria。Slot 與 snapshot criteria 已移至 generic lifecycle task。Frontend client 與 UI 已合併為兩個 source files 的單一 task。
- [x] Task 2 在 Task 3 前使用 completed Sawmill、provider eligibility 與 parent Building 耐久。Generic lifecycle 現在由 Task 2 完成，Task 3 才加入 Sawmill Convert 與 atomic durability use。
- [x] `docs/schemas.md` 保留兩個與新計畫衝突的舊敘述。Item 清單與 cascade 限制已更新。
- [x] Criterion owner 仍未分離。Convert runtime criteria 已由 Task 3 唯一承接。Completed extension capability 使用規則也已移至 Task 3。
- [x] Task 1 新增 `wood_essence_t1` 與 `sawmill_package_t1` definitions，但沒有抄入 REQ-015.1 與 REQ-015.2。兩項 Item definition criteria 已加入 Task 1。
- [x] 仍有四項 criteria 沒有唯一 task owner。REQ-016.7、REQ-016.11、REQ-016.18 與 REQ-008.5 現在各自只保留一個 owner task。
