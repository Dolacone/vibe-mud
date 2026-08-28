---
title: "Player carrying weight"
status: Ready-to-implement
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - docs/changes/2026-08-28-carrying-weight.md
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - requirements/BEHAVIOR.md
  - requirements/REQ-014.md
req_ref: REQ-014
base_branch: main
scope: "Tracks derived carrying weight and movement blocking above the weight threshold."
---

## Problem Statement

Player holdings have no weight calculation. The game cannot show how much a player carries or prevent an overloaded player from moving.

## Recommended Direction

Add a positive integer `weight_units` column to Item and Resource definitions. Existing databases gain both columns without changing player quantities. Seed Wood Item at 100, Wood Component at 10, and all Resource types at 1.

Derive carrying weight from player quantities on every authoritative state read. Return `carried_weight` and `movement_weight_threshold` in player state. Do not persist the derived total.

Check carrying weight inside the Move transaction before consuming AP. Return HTTP 409 with authoritative player state when `carried_weight` exceeds 1000. Pickup, Gather, Craft, Convert, and Drop keep their current behavior regardless of resulting weight.

## Key Assumptions

- Players can hold unlimited Item and Resource quantities.
- The weight threshold limits only Move.
- Definition weights use integer units.
- Item durability remains outside this change.

## Acceptance Criteria

## MVP Scope / Not Doing

- No Item durability.
- No Warehouse, equipment bonus, vehicle, or volume rule.
- No balancing system or batch Convert.

## Tasks

```text
Task 1: definition weights and movement rule [parallel: no]
└── Task 2: backend state and computation logs [parallel: no]
    └── Task 3: frontend state contract [parallel: no]
        └── Task 4: carrying weight interface [parallel: no]
```

- [x] Task 1 [parallel: no]: Add definition weight columns, their idempotent migration, derived player carrying weight, and atomic overweight Move rejection in `internal/authapi/store.go`. Update store tests. Keep Pickup, Gather, Craft, Convert, and Drop unrestricted by weight. Keep `docs/schemas.md`, `docs/terminology.md`, and `docs/architecture.md` aligned with the executable behavior.
  - REQ-014.1: 每位玩家的移動負重門檻為 1000 重量單位。
  - REQ-014.2: Wood Item 每單位重 100。Wood Resource 每單位重 1。Wood Component 每單位重 10。其他 Resource 每單位重 1。
  - REQ-014.3: 系統依玩家持有的每種 Item 與 Resource 數量及其單位重量，計算目前攜帶重量。系統不保存計算結果。
  - REQ-014.5: Pickup、Gather、Craft 與 Convert 可以使玩家的目前重量超過 1000。
  - REQ-014.6: Drop 維持不消耗 AP，且不受目前重量限制。
  - REQ-014.7: 如果玩家的目前重量小於或等於 1000，Move 可以依既有規則執行。
  - REQ-014.8: 如果玩家的目前重量大於 1000，Move 必須原子失敗。玩家的位置與 AP 都維持不變。
  - REQ-014.10: 本需求不增加 Item 耐久度、倉庫、裝備負重加成、載具、體積或批次 Convert。
- [ ] Task 2 [parallel: no]: Add `carried_weight` and `movement_weight_threshold` to authoritative state responses in `internal/authapi/server.go`. Return an HTTP 409 overweight Move response with current state. Log carrying weight calculations and rejections with safe computation fields. Update server tests.
  - REQ-014.3: 系統依玩家持有的每種 Item 與 Resource 數量及其單位重量，計算目前攜帶重量。系統不保存計算結果。
  - REQ-014.4: 前端顯示後端回傳的目前攜帶重量與移動負重門檻。超重時必須顯示不能移動。
  - REQ-014.8: 如果玩家的目前重量大於 1000，Move 必須原子失敗。玩家的位置與 AP 都維持不變。
  - REQ-014.9: 後端必須將重量計算與 Move 拒絕結果寫入標準輸出。紀錄必須包含玩家 ID、操作、結果、request ID 與計算值。
- [ ] Task 3 [parallel: no]: Parse and preserve authoritative `carried_weight` and `movement_weight_threshold` fields in `web/src/auth.ts`. Accept the planned overweight HTTP 409 response through the existing Move conflict path. Update client tests.
  - REQ-014.4: 前端顯示後端回傳的目前攜帶重量與移動負重門檻。超重時必須顯示不能移動。
- [ ] Task 4 [parallel: no]: Display carrying weight and the movement threshold in the compact player summary in `web/src/App.tsx`. Show an overweight movement warning and disable route controls while overweight. Update interface tests.
  - REQ-014.4: 前端顯示後端回傳的目前攜帶重量與移動負重門檻。超重時必須顯示不能移動。
  - REQ-014.7: 如果玩家的目前重量小於或等於 1000，Move 可以依既有規則執行。
  - REQ-014.8: 如果玩家的目前重量大於 1000，Move 必須原子失敗。玩家的位置與 AP 都維持不變。

## Review Issues
