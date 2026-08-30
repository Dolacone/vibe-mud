---
title: "Completed construction visibility"
status: Ready-to-implement
created: 2026-08-30
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - docs/architecture.md
  - internal/authapi/server.go
  - internal/authapi/server_test.go
req_ref: REQ-010, REQ-016
base_branch: main
scope: "Remove obsolete construction values from completed Buildings and extensions."
---

## Problem Statement

Completed Buildings and extensions still expose and display final construction AP values. Those values no longer help the player act and make the Building table harder to scan.

## Recommended Direction

Keep construction values only while an entity is under construction. Omit those fields after completion and render extension display names without a duplicate tier suffix.

## Key Assumptions

- Internal construction snapshots remain stored for history and domain logic.
- Lifecycle status remains public because clients must distinguish construction from completion.
- In-progress entities retain contributed AP, required AP, and percentage.

## Acceptance Criteria

REQ-010 and REQ-016 are the sources of truth. The plan will copy each affected criterion into its owning task.

## MVP Scope / Not Doing

- Do not change construction costs, contribution rules, completion, persistence, or logs.
- Do not remove construction information from in-progress entities.
- Do not redesign the Building table.

## Dependency Graph

`Task 1 backend response contract -> Task 2 frontend parsing and display`

## Tasks

- [x] Task 1 [parallel: no]: Emit conditional construction fields for public Building and extension responses in `internal/authapi/server.go`. Update backend tests in the same commit. In-progress responses must retain integer `contributed_ap` and `required_ap`. Completed responses must omit both keys rather than return `null`. Response shaping must not clear stored snapshots or alter logs.
  - REQ-010.37: Building 完成後，後端傳給前端的 Building 資訊不得包含已投入 AP、所需 AP 或施工進度。
  - REQ-016.22: 施工中的 extension 必須向前端提供已投入 AP、所需 AP 與進度百分比。
  - REQ-016.23: Extension 完成後，後端傳給前端的 extension 資訊不得包含已投入 AP、所需 AP 或施工進度。
- [ ] Task 2 [parallel: no]: Parse conditional construction values and hide completed construction details in `web/src/auth.ts` and `web/src/App.tsx`. Update frontend tests and verify the production build in the same commit. In-progress Building and extension percentages must use the existing downward-rounded ratio. Completed entities must not calculate or display construction progress. Installed extensions, installation definitions, and Convert provider options must not append tier to the player-facing display name.
  - REQ-010.17: 同一 Location 的玩家都能看到施工中 Building 的已投入 AP、所需 AP 與進度百分比。
  - REQ-010.38: Building 完成後，前端不得顯示已完成的施工數值。
  - REQ-016.19: 前端必須顯示空 slot、extension 顯示名稱、目前狀態與玩家可用操作，且不得在顯示名稱後重複附加 tier。
  - REQ-016.24: Extension 完成後，前端不得顯示已完成的施工數值。

## Plan Review Issues

- [x] Task 1 未定義條件式 JSON contract，且把後端供應責任 REQ-016.22 放在前端 Task 2。將 REQ-016.22 移至 Task 1。明定施工中 Building 與 extension 必須保留整數 `contributed_ap`、`required_ap`，完成後必須省略兩個 key 而非回傳 `null`。後端測試必須同時驗證保留與省略。回應整形不得清除內部 snapshots 或改變既有 log。
- [x] Task 2 未定義施工進度百分比。現有 Building 使用向下取整，extension 未顯示百分比。明定沿用既有 Building 算式，且施工中 Building 與 extension 都顯示百分比。完成後不得計算或顯示施工進度。
- [x] Task 2 未列出重複 tier 的所有玩家可見路徑。`App.tsx` 目前在已安裝 extension、extension definition 與 Convert provider 選項後再次附加 tier。計畫與測試必須覆蓋所有路徑，避免只修正 Building 表格。
- [x] `docs/architecture.md` 已修改，但 `last_reviewed` 仍是 `2026-08-29`。更新為 `2026-08-30`，並記錄施工百分比由前端依條件式 AP 欄位計算。
