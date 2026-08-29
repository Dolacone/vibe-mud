---
title: "Backend-filtered gameplay options"
status: Ready-to-review
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths:
  - internal/authapi/server.go
  - internal/authapi/store.go
  - internal/authapi/server_test.go
  - docs/architecture.md
  - web/src/App.tsx
  - web/src/App.test.tsx
  - web/src/auth.ts
  - web/src/auth.test.ts
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
