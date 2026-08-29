---
title: "Durability percentage API"
status: Refactored
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths:
  - docs/architecture.md
  - docs/changes/2026-08-29-durability-percentage.md
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - requirements/BEHAVIOR.md
  - requirements/REQ-011.md
  - requirements/REQ-015.md
  - web/src/App.test.tsx
  - web/src/App.tsx
  - web/src/auth.test.ts
  - web/src/auth.ts
req_ref: REQ-011, REQ-015
base_branch: main
scope: "Replace frontend durability timing details with rounded-up integer percentages."
---

## Problem Statement

The player API exposes durability limits, remaining durability, and retention details in seconds. Clients need a stable percentage without receiving internal timing precision.

## Recommended Direction

Calculate durability percentages in the backend from authoritative time-based state. Return only status and percentage to clients while retaining second-based storage, computation, and logs.

## Key Assumptions

- Construction progress remains AP-based and does not use durability percentage.
- Expired Item retention and Disabled Building deletion still use exact internal timestamps.
- Transfer and mutation requests continue to identify Item status without durability timing values.

## Acceptance Criteria

REQ-011 and REQ-015 are the sources of truth. The plan will copy each affected criterion into its owning task.

## MVP Scope / Not Doing

- Do not change SQLite schema or persisted timestamps.
- Do not change durability duration, repair behavior, retention, cleanup, or logs.
- Do not expose a retention countdown in another unit.

## Dependency Graph

`Task 1 backend response contract -> Task 2 frontend parsing and display`

## Tasks

- [x] Task 1 [parallel: no]: Replace public durability timing fields with backend-calculated percentages in `internal/authapi/server.go`. Update backend tests in the same commit.
  - REQ-011.28: Building 耐久百分比必須以剩餘耐久時間除以 7 天耐久上限計算，乘以 100 後無條件進位，且不得超過 100。
  - REQ-011.29: Active Building 的耐久百分比必須介於 1 至 100，Disabled Building 必須為 0。
  - REQ-011.30: 後端傳給前端的 Building 資訊不得包含耐久上限秒數、剩餘耐久秒數、失效保留秒數或對應 timestamp。
  - REQ-015.26: Item 耐久百分比必須以剩餘耐久時間除以該 Item 的耐久時間上限計算，乘以 100 後無條件進位，且不得超過 100。
  - REQ-015.27: Active Item 的耐久百分比必須介於 1 至 100，Expired Item 必須為 0。
  - REQ-015.28: 後端傳給前端的 Item definition、Inventory 與地面 Item 資訊不得包含耐久上限秒數、剩餘耐久秒數、失效保留秒數或對應 timestamp。
- [x] Task 2 [parallel: no]: Parse and display status with integer durability percentages in `web/src/auth.ts` and `web/src/App.tsx`. Update frontend tests and verify the production build in the same commit.
  - REQ-011.4: 玩家必須能看到 Building 的耐久狀態與整數耐久百分比。
  - REQ-015.8: 玩家必須能看到每個 Item 堆疊的狀態與整數耐久百分比。
  - REQ-015.22: 前端必須將 Expired Item 顯示為 0% 耐久，且不能提供會使用該 Item 的操作。

## Plan Review Issues

- [x] `docs/architecture.md` 尚未記錄公開 API 僅回傳耐久狀態與整數百分比，且不得回傳秒數或 timestamp。Plan 階段必須先更新相關文件，不得把此更新延後到 Task 1 的實作 commit。

## Review Issues

- [x] [Major] `source_paths` 只列出 `main...HEAD` 11 個變更檔案中的 7 個。缺少 change document 與 3 個 requirements 檔案，因此中繼資料不符合實際 diff。
