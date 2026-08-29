---
title: "Backend-filtered gameplay options"
status: Done
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths:
  - CHANGELOG.md
  - docs/architecture.md
  - docs/changes/2026-08-29-available-gameplay-options.md
  - docs/changes/2026-08-29-sawmill-product-chain.md
  - docs/schemas.md
  - docs/terminology.md
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - requirements/BEHAVIOR.md
  - requirements/REQ-004.md
  - requirements/REQ-005.md
  - requirements/REQ-006.md
  - requirements/REQ-008.md
  - requirements/REQ-009.md
  - requirements/REQ-010.md
  - requirements/REQ-011.md
  - requirements/REQ-014.md
  - requirements/REQ-015.md
  - requirements/REQ-016.md
  - requirements/REQ-017.md
  - requirements/REQ-018.md
  - web/src/App.test.tsx
  - web/src/App.tsx
  - web/src/auth.test.ts
  - web/src/auth.ts
req_ref: REQ-018
base_branch: main
scope: "Only expose gameplay options that the authenticated player can currently execute."
---

## Problem Statement

The backend exposes unavailable Actions, targets, methods, and recipes. The frontend then duplicates eligibility rules or displays controls that can only fail.

## Recommended Direction

Calculate availability from the authoritative player state when building each response. Return complete player state, but filter executable options and attach backend-defined target metadata.

## Key Assumptions

- Transfer remains outside Action filtering.
- Existing mutation endpoints remain the final authorization boundary.
- Availability checks have no side effects.

## Acceptance Criteria

REQ-018 is the source of truth. Each criterion is copied into one task below.

## MVP Scope / Not Doing

- Do not change SQLite schema.
- Do not add Transfer filtering.
- Do not add new game Actions or eligibility rules.

## Dependency Graph

`Task 1 backend contract -> Task 2 frontend consumption`

## Tasks

- [x] Task 1 [parallel: no]: Filter backend gameplay options and expose executable target metadata in `internal/authapi/store.go` and `internal/authapi/server.go`. Update backend tests and `docs/architecture.md` in the same commit.
  - REQ-018.1: 後端回傳玩家狀態時，只能包含玩家依目前 authoritative state 可以執行的 Action、target、method 與 recipe。
  - REQ-018.2: 可執行性必須由後端依 Action 定義、玩家狀態、目前 Location、target 狀態、權限、AP、Resource inputs 與 Active Item inputs 判定。
  - REQ-018.3: 可執行性判定不得修改任何玩家或世界狀態。
  - REQ-018.4: 玩家狀態或世界狀態改變後，後端必須重新計算可回傳的遊戲選項。
  - REQ-018.6: 玩家無法執行的選項，其 identifier、顯示名稱、成本、inputs、output、機率與 capacity 都不得出現在前端可取得的回應中。
  - REQ-018.7: `rest` 必須依目前 AP 判定。
  - REQ-018.8: `move` 的 Route 必須依目前 Location、AP 與移動負重門檻判定。
  - REQ-018.9: `gather` 必須依目前 Location 與 AP 判定。
  - REQ-018.10: Convert method 必須依 AP、至少 1 個對應 Active input Item，以及 method 使用條件判定。
  - REQ-018.11: Craft recipe 必須依 AP、全部 Resource inputs 與全部 Active Item inputs 判定。
  - REQ-018.12: Building recipe 必須依全部 inputs 與玩家在目前 Location 的 Building 數量限制判定。
  - REQ-018.13: Building 維修、extension 安裝、extension 施工與 extension 拆除必須依各自既有的 target、Location、權限、狀態、AP 與 inputs 規則判定。
  - REQ-018.14: 隨機成功機率不得影響選項是否可執行。
  - REQ-018.15: 此規則不得隱藏玩家狀態。前端必須繼續取得全部 8 種 Resource、Inventory、目前 Location、Buildings 與其他既有可見狀態。
  - REQ-018.16: Pickup 與 Drop 屬於 Transfer，不屬於此 REQ 的 Action 或 recipe filtering。
  - REQ-018.17: 未回傳的 Action、target、method 或 recipe 仍必須由後端拒絕直接提交，且不得修改任何狀態。
- [x] Task 2 [parallel: no]: Consume only backend-provided Actions and targets in `web/src/auth.ts` and `web/src/App.tsx`. Update frontend tests and verify the production build in the same commit.
  - REQ-018.5: 前端只能顯示後端回傳的 Action、target、method 與 recipe，不得自行推定或補回未回傳的選項。
  - REQ-018.15: 此規則不得隱藏玩家狀態。前端必須繼續取得全部 8 種 Resource、Inventory、目前 Location、Buildings 與其他既有可見狀態。

## Review Issues

- [x] [Major] The full Go suite failed three stale tests: `TestPlayerStateResponseFiltersOptionsFromCurrentAuthoritativeState` assumed action ordering, `TestCraftAPIUsesRecipeWhitelistAndReturnsAuthoritativeState` expected consumed recipes to remain available, and `TestRestInsufficientAPReturnsConflictWithoutChangingState` expected a partial error response. Tests now assert unordered availability, post-mutation filtering, and the complete authoritative state response.
- [x] [Major] The frontend hides a backend-returned legacy Convert option when `conversion_option` is available but `conversion_methods` is empty. The backend returns `convert` for that state, while `App.tsx` renders no control unless a method exists.
- [x] [Major] A remote owned extension is omitted from the current-Location response but remains removable by direct submission. `RemoveExtension` checks ownership only, then deletes the extension without checking the player's current Location.
- [x] [Major] `source_paths` does not match `main...HEAD`. The actual 27-file diff includes the earlier sawmill change, schema and terminology documents, store tests, changelog, and requirement files beyond the declared eight paths.
- [x] [Major] A backend-returned legacy Convert option cannot execute end to end. `App.tsx` submits it through `convert(fetch)`, which sends `{}`, while `decodeConvertRequest` requires both `method_id` and `quantity`; every click returns HTTP 400 without converting. The UI test mocks `convert`, so it never exercises this HTTP contract.
- [x] [Major] Direct submission can attach an unreturned `provider_extension_id` to a global Convert method and still mutate state. Global methods return an empty `provider_extension_ids` list, but `Store.Convert` validates the provider only for non-global methods and ignores any supplied ID for a global method, violating the target rejection and no-side-effect criterion.
- [x] [Major] The full Go suite is nondeterministic and failed during this review. `TestConvertAPIUpdatesStateAndUsesBackendOwnedValues` requires an empty Inventory, but the default `essenceRoll` can create `wood_essence_t1`; the review run failed when that outcome occurred.
- [x] [Major] Legacy Convert now mutates the correct state, but its success metadata is false. The handler calculates results from the empty legacy request. The response omits method, quantity, and Resource output. The computation log records an empty `method_id`, `quantity=0`, and `resource_quantity=0` for a one-item conversion. The E2E test checks state and only the generic action log. It cannot catch this reporting regression.
- [x] [Major] The implementation fixes were submitted while this change document remained `Issues-confirmed` instead of returning to `Ready-to-review`. This violates the review-stage input gate and leaves no durable indication that all confirmed fixes were ready for independent review.
- [x] [Minor] `docs/architecture.md` says Convert accepts only `method_id`, positive `quantity`, and optional `provider_extension_id`. The repaired API also accepts the empty legacy `{}` contract. The documented request contract no longer matches the handler.
