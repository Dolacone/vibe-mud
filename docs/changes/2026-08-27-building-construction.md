---
title: "Building construction"
status: Ready-to-review
created: 2026-08-27
doc_type: change
last_reviewed: 2026-08-27
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - requirements/BEHAVIOR.md
  - requirements/REQ-010.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-010
base_branch: main
scope: "Tracks Building Lv1 placement, shared AP contributions, completion, and persistence."
---

## Problem Statement

Players can create Inventory Items but cannot turn those Items into persistent world Buildings or cooperate on construction.

## Recommended Direction

Store backend-owned Building recipes separately from player-owned Buildings. Starting construction consumes recipe inputs and creates an in-progress Building. Players at the same Location can contribute AP until the stored requirement is met.

Authenticated state adds `building_recipes` and `buildings`. Each Building response contains `id`, owner identity, recipe identity, level, required AP, contributed AP, status, and extension slot count. `POST /api/actions/build` accepts only `{"recipe_id":"building_lv1"}`. `POST /api/actions/contribute-construction` accepts only `{"building_id":1,"ap":10}`. Both endpoints return the backend-authoritative player state.

```json
{
  "building_recipes": [{
    "id": "building_lv1",
    "display_name": "Building Lv1",
    "building_level": 1,
    "required_ap": 60,
    "extension_slot_count": 1,
    "resource_inputs": [{"resource": {"id": "wood", "display_name": "Wood"}, "quantity": 10}],
    "item_inputs": [{"item": {"id": "wood_component", "display_name": "Wood Component"}, "quantity": 1}]
  }],
  "buildings": [{
    "id": 1,
    "owner": {"id": 1, "display_name": "Player"},
    "recipe": {"id": "building_lv1", "display_name": "Building Lv1"},
    "building_level": 1,
    "required_ap": 60,
    "contributed_ap": 0,
    "status": "under_construction",
    "extension_slot_count": 1
  }]
}
```

Every collection field is a JSON array. `building_recipes.resource_inputs` and `building_recipes.item_inputs` can be empty individually, but not together. `buildings.status` is either `under_construction` or `completed`. Owner responses omit email.

## Key Assumptions

- Building recipe values are backend-owned balance data.
- The MVP Building Lv1 recipe consumes 1 Wood Component, 10 Wood Resource, and requires 60 AP.
- Resource and Item input tables each allow zero rows, but a recipe must have at least one combined input.
- The construction AP requirement is copied into each Building when construction starts.
- An in-progress Building counts toward the owner and Location uniqueness rule.
- The server consumes only the remaining AP after an oversized contribution.

## Acceptance Criteria

REQ-010 is the source of truth. The plan stage will copy each criterion into its owning task.

## MVP Scope / Not Doing

- No Building removal or demolition.
- No extension installation.
- No durability or maintenance.
- No Building effects.
- No recipe acquisition.

## Tasks

Dependency graph:

```text
Task 1: Building definitions and persistence
  -> Task 2: Start construction
      -> Task 3: Contribute construction AP
          -> Task 4: Building API
              -> Task 5: Frontend building client
                  -> Task 6: Building UI
```

### Task 1: Persist Building recipes and current-Location Buildings

- [x] Add normalized Building recipe, Resource input, Item input, and Building tables in `internal/authapi/store.go`.
- [x] Seed Building Lv1 with 1 Wood Component input, 10 Wood Resource input, 60 required AP, level 1, and one extension slot.
- [x] Load only recipes with at least one combined input.
- [x] Return every Building at the player's current Location with owner, status, progress, and snapshots.
- [x] Preserve existing player state while upgrading an existing SQLite database.
- [x] Cover definitions, empty input rejection, Location visibility, uniqueness, snapshots, and schema upgrade in `internal/authapi/store_test.go`.

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-010.1: `Building Lv1` 必須具有後端定義的 recipe。
- REQ-010.2: Recipe 必須定義 inputs 與施工所需 AP。
- REQ-010.3: 玩家必須能看到 recipe 的 inputs 與所需 AP。
- REQ-010.4: Recipe 來源不屬於本 REQ。
- REQ-010.5: 前端不能覆寫 recipe inputs 或所需 AP。
- REQ-010.7: 每位玩家在同一 Location 最多擁有一座 Building。
- REQ-010.8: 施工中的 Building 必須占用 Building 名額。
- REQ-010.11: 新 Building 必須標記為施工中。
- REQ-010.12: 新 Building 的已投入 AP 必須為 0。
- REQ-010.14: 玩家已有 Building 時，系統不得建立第二座。
- REQ-010.15: Building 必須保存建立時的所需 AP。
- REQ-010.16: Recipe 變更不能影響施工中的 Building。
- REQ-010.17: 同一 Location 的玩家都能看到施工進度。
- REQ-010.26: 已投入 AP 達到需求時，Building 必須立即完成。
- REQ-010.27: Building 進度不能超過所需 AP。
- REQ-010.28: 完成的 `Building Lv1` 具有一個空 extension slot。
- REQ-010.29: 重新登入後，玩家必須看到已保存的狀態。

### Task 2: Start Building construction atomically

- [x] Add a store operation that resolves a Location-independent backend-owned recipe, then assigns the new Building to the player's current Location.
- [x] Atomically consume every recipe input and create an in-progress owned Building.
- [x] Enforce one Building per owner and Location for completed and in-progress rows.
- [x] Preserve every player and Building value after unknown recipe, insufficient input, or occupied slot failures.
- [x] Cover Resource inputs, Item inputs, mixed inputs, exact depletion, uniqueness, rollback, and recipe snapshot behavior in `internal/authapi/store_test.go`.

Source files: `internal/authapi/store.go`

Acceptance criteria:

- REQ-010.5: 前端不能覆寫 recipe inputs 或所需 AP。
- REQ-010.6: 玩家可以在目前 Location 建造自己的 `Building Lv1`。
- REQ-010.7: 每位玩家在同一 Location 最多擁有一座 Building。
- REQ-010.8: 施工中的 Building 必須占用 Building 名額。
- REQ-010.9: 開始施工時，系統必須一次扣除所有 inputs。
- REQ-010.10: 扣除 inputs 與建立 Building 必須是原子結果。
- REQ-010.11: 新 Building 必須標記為施工中。
- REQ-010.12: 新 Building 的已投入 AP 必須為 0。
- REQ-010.13: Inputs 不足時，系統不得修改任何狀態。
- REQ-010.14: 玩家已有 Building 時，系統不得建立第二座。
- REQ-010.15: Building 必須保存建立時的所需 AP。
- REQ-010.16: Recipe 變更不能影響施工中的 Building。
- REQ-010.31: 本 REQ 不提供撤銷、拆除或 extension 安裝。
- REQ-010.32: 後端必須拒絕不存在或不允許的 target。
- REQ-010.34: 拒絕時，所有狀態必須保持不變。

### Task 3: Contribute construction AP atomically

- [x] Add a store operation that accepts a Building target and requested positive AP.
- [x] Require the contributor to be at the Building Location.
- [x] Consume the smaller of requested AP and remaining AP.
- [x] Atomically spend contributor AP, increase progress, and mark exact completion.
- [x] Reject insufficient AP, completed Building, invalid target, and remote contribution without state changes.
- [x] Cover shared contributors, oversized input, insufficient AP, completion, repeated completion, concurrency, and rollback in `internal/authapi/store_test.go`.

Source files: `internal/authapi/store.go`, `internal/authapi/store_test.go`

Acceptance criteria:

- REQ-010.17: 同一 Location 的玩家都能看到施工進度。
- REQ-010.18: 同一 Location 的玩家都能投入自己的 AP。
- REQ-010.19: 玩家可以選擇本次投入的正整數 AP。
- REQ-010.20: 不在同一 Location 的玩家不能投入 AP。
- REQ-010.21: 系統必須以剩餘所需 AP 限制實際投入量。
- REQ-010.22: 超額投入只能消耗剩餘所需 AP。
- REQ-010.23: AP 低於實際投入量時，施工必須失敗。
- REQ-010.24: AP 扣除與進度增加必須是原子結果。
- REQ-010.25: 失敗時，AP 與 Building 進度必須保持不變。
- REQ-010.26: 已投入 AP 達到需求時，Building 必須立即完成。
- REQ-010.27: Building 進度不能超過所需 AP。
- REQ-010.28: 完成的 `Building Lv1` 具有一個空 extension slot。
- REQ-010.29: 重新登入後，玩家必須看到已保存的狀態。
- REQ-010.30: 完成後不能繼續投入 AP。
- REQ-010.32: 後端必須拒絕不存在或不允許的 target。
- REQ-010.34: 拒絕時，所有狀態必須保持不變。

### Task 4: Expose Building construction through the API

- [x] Add `building_recipes` and current-Location `buildings` arrays to authenticated state in `internal/authapi/server.go`.
- [x] Return recipe `id`, `display_name`, `building_level`, `required_ap`, `extension_slot_count`, `resource_inputs`, and `item_inputs` with typed nested Item or Resource values.
- [x] Return Building numeric `id`, owner `id` and `display_name`, recipe `id` and `display_name`, `building_level`, `required_ap`, `contributed_ap`, `status`, and `extension_slot_count`.
- [x] Add `POST /api/actions/build` accepting only `{"recipe_id":"building_lv1"}`.
- [x] Add `POST /api/actions/contribute-construction` accepting only `{"building_id":1,"ap":10}` with positive integer values.
- [x] Reject missing, duplicate, malformed, unknown, remote, completed, and extra values with authoritative state.
- [x] Log success and sanitized rejection outcomes with user ID, Action, result, reason, and request ID.
- [x] Cover response contracts, whitelist validation, state preservation, and log sanitization in `internal/authapi/server_test.go`.

Source files: `internal/authapi/server.go`

Acceptance criteria:

- REQ-010.3: 玩家必須能看到 recipe 的 inputs 與所需 AP。
- REQ-010.5: 前端不能覆寫 recipe inputs 或所需 AP。
- REQ-010.6: 玩家可以在目前 Location 建造自己的 `Building Lv1`。
- REQ-010.13: Inputs 不足時，系統不得修改任何狀態。
- REQ-010.14: 玩家已有 Building 時，系統不得建立第二座。
- REQ-010.17: 同一 Location 的玩家都能看到施工進度。
- REQ-010.18: 同一 Location 的玩家都能投入自己的 AP。
- REQ-010.19: 玩家可以選擇本次投入的正整數 AP。
- REQ-010.20: 不在同一 Location 的玩家不能投入 AP。
- REQ-010.21: 系統必須以剩餘所需 AP 限制實際投入量。
- REQ-010.22: 超額投入只能消耗剩餘所需 AP。
- REQ-010.23: AP 低於實際投入量時，施工必須失敗。
- REQ-010.24: AP 扣除與進度增加必須是原子結果。
- REQ-010.25: 失敗時，AP 與 Building 進度必須保持不變。
- REQ-010.26: 已投入 AP 達到需求時，Building 必須立即完成。
- REQ-010.27: Building 進度不能超過所需 AP。
- REQ-010.28: 完成的 `Building Lv1` 具有一個空 extension slot。
- REQ-010.29: 重新登入後，玩家必須看到已保存的狀態。
- REQ-010.30: 完成後不能繼續投入 AP。
- REQ-010.31: 本 REQ 不提供撤銷、拆除或 extension 安裝。
- REQ-010.32: 後端必須拒絕不存在或不允許的 target。
- REQ-010.33: 後端必須拒絕錯誤格式與未支援欄位。
- REQ-010.34: 拒絕時，所有狀態必須保持不變。
- REQ-010.35: Log 必須記錄 user ID、Action、結果與 request ID。
- REQ-010.36: Log 不得包含 credentials、session、OAuth 或原始輸入。

### Task 5: Parse Building state and Actions in the frontend client

- [x] Parse recipe `id`, `display_name`, `building_level`, `required_ap`, `extension_slot_count`, `resource_inputs`, and `item_inputs` from the `building_recipes` array in `web/src/auth.ts`.
- [x] Parse Building numeric `id`, owner `id` and `display_name`, recipe `id` and `display_name`, `building_level`, `required_ap`, `contributed_ap`, `status`, and `extension_slot_count` from the `buildings` array.
- [x] Submit only recipe identifier for start and Building identifier with requested AP for contribution.
- [x] Apply backend-authoritative state for success and every server failure.
- [x] Reject malformed Building state and Action responses in `web/src/auth.test.ts`.

Source files: `web/src/auth.ts`, `web/src/auth.test.ts`

Acceptance criteria:

- REQ-010.3: 玩家必須能看到 recipe 的 inputs 與所需 AP。
- REQ-010.5: 前端不能覆寫 recipe inputs 或所需 AP。
- REQ-010.6: 玩家可以在目前 Location 建造自己的 `Building Lv1`。
- REQ-010.11: 新 Building 必須標記為施工中。
- REQ-010.12: 新 Building 的已投入 AP 必須為 0。
- REQ-010.17: 同一 Location 的玩家都能看到施工進度。
- REQ-010.18: 同一 Location 的玩家都能投入自己的 AP。
- REQ-010.19: 玩家可以選擇本次投入的正整數 AP。
- REQ-010.20: 不在同一 Location 的玩家不能投入 AP。
- REQ-010.21: 系統必須以剩餘所需 AP 限制實際投入量。
- REQ-010.22: 超額投入只能消耗剩餘所需 AP。
- REQ-010.25: 失敗時，AP 與 Building 進度必須保持不變。
- REQ-010.26: 已投入 AP 達到需求時，Building 必須立即完成。
- REQ-010.27: Building 進度不能超過所需 AP。
- REQ-010.28: 完成的 `Building Lv1` 具有一個空 extension slot。
- REQ-010.29: 重新登入後，玩家必須看到已保存的狀態。
- REQ-010.30: 完成後不能繼續投入 AP。
- REQ-010.32: 後端必須拒絕不存在或不允許的 target。
- REQ-010.33: 後端必須拒絕錯誤格式與未支援欄位。
- REQ-010.34: 拒絕時，所有狀態必須保持不變。

### Task 6: Display and operate Building construction

- [x] Display the Building Lv1 recipe and current-Location Buildings in `web/src/App.tsx`.
- [x] Show owner, status, contributed AP, required AP, percentage, and empty extension slot count.
- [x] Start construction by recipe identifier.
- [x] Let any same-Location player choose positive AP and contribute to an in-progress Building.
- [x] Disable duplicate pending Actions and completed Building contributions.
- [x] Apply backend-authoritative AP, Inventory, Resources, and Building state after every result.
- [x] Cover start, occupied owner slot, shared progress, oversized contribution, completion, reload, and failures in `web/src/App.test.tsx`.

Source files: `web/src/App.tsx`

Acceptance criteria:

- REQ-010.3: 玩家必須能看到 recipe 的 inputs 與所需 AP。
- REQ-010.5: 前端不能覆寫 recipe inputs 或所需 AP。
- REQ-010.6: 玩家可以在目前 Location 建造自己的 `Building Lv1`。
- REQ-010.7: 每位玩家在同一 Location 最多擁有一座 Building。
- REQ-010.8: 施工中的 Building 必須占用 Building 名額。
- REQ-010.11: 新 Building 必須標記為施工中。
- REQ-010.12: 新 Building 的已投入 AP 必須為 0。
- REQ-010.13: Inputs 不足時，系統不得修改任何狀態。
- REQ-010.14: 玩家已有 Building 時，系統不得建立第二座。
- REQ-010.17: 同一 Location 的玩家都能看到施工進度。
- REQ-010.18: 同一 Location 的玩家都能投入自己的 AP。
- REQ-010.19: 玩家可以選擇本次投入的正整數 AP。
- REQ-010.20: 不在同一 Location 的玩家不能投入 AP。
- REQ-010.21: 系統必須以剩餘所需 AP 限制實際投入量。
- REQ-010.22: 超額投入只能消耗剩餘所需 AP。
- REQ-010.23: AP 低於實際投入量時，施工必須失敗。
- REQ-010.24: AP 扣除與進度增加必須是原子結果。
- REQ-010.25: 失敗時，AP 與 Building 進度必須保持不變。
- REQ-010.26: 已投入 AP 達到需求時，Building 必須立即完成。
- REQ-010.27: Building 進度不能超過所需 AP。
- REQ-010.28: 完成的 `Building Lv1` 具有一個空 extension slot。
- REQ-010.29: 重新登入後，玩家必須看到已保存的狀態。
- REQ-010.30: 完成後不能繼續投入 AP。
- REQ-010.31: 本 REQ 不提供撤銷、拆除或 extension 安裝。
- REQ-010.32: 後端必須拒絕不存在或不允許的 target。
- REQ-010.33: 後端必須拒絕錯誤格式與未支援欄位。
- REQ-010.34: 拒絕時，所有狀態必須保持不變。

## Review Issues

- [x] [Major] Snapshot the Building display name or stop reading it from the mutable recipe. Mutable recipe names currently rename existing Buildings. This violates REQ-010.16. Extend the snapshot test to cover the displayed recipe identity.
- [x] [Major] Align `docs/schemas.md` with the implemented `buildings` table. The document specifies `owner_user_id`, `created_at`, and `completed_at`. The implementation uses `owner_id` and `status`. The documented completion invariant is absent from the schema.
- [x] [Major] Complete Task 4 API tests. Cover the exact response contract and successful contribution. Cover every rejection class and state preservation. Assert required log fields, success logs, and credential sanitization.
- [x] [Major] Reopen Task 4 API coverage. The fix does not verify the exact Building recipe or owner contract. It omits most build rejection classes, several contribution decoding classes, build success logs, rejection log fields, and meaningful credential sanitization inputs.
- [x] [Minor] Make the documented `buildings` SQL match the implementation. The document adds a nonexistent `DEFAULT 0` to `contributed_ap` and omits the implemented `DEFAULT ''` from `display_name`.
- [x] [Major] The same Task 4 API coverage issue remains. The second fix verifies only an empty `resource_inputs` array and never exercises its nested contract. It also omits the `insufficient_resource` API rejection class required by the prior review finding.
- [x] [Minor] Return a Building-target error from contribution failures. `ErrBuildingNotFound` currently says "building recipe not found", so an unknown `building_id` produces a false error message.
- [x] [Major] Log the construction computation result. An oversized request can consume less AP than requested, but the success path logs only the remaining player AP and generic Action success. Record the Building target, effective AP contribution, resulting progress, and completion outcome without logging raw input.

## Plan Review Issues

- [x] Clarify Task 2 so it resolves a Location-independent Building recipe, then assigns only the new Building to the player's current Location. The current phrase "recipe for the player's current Location" can introduce an unintended recipe Location restriction.
- [x] Define the Building endpoint paths, exact request payloads, and authenticated-state response fields in the plan. Task 4 currently delegates the API contract to implementation, so the backend and frontend tasks do not share a reviewable contract.
- [x] Define exact JSON field names and types for each `building_recipes` and `buildings` entry. The current plan omits recipe input fields and leaves "owner identity" and "recipe identity" representations ambiguous, so Tasks 4 and 5 still lack one shared response contract.
