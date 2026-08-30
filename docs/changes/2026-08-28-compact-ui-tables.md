---
title: "Compact table interface"
status: Done
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - docs/architecture.md
  - requirements/BEHAVIOR.md
  - requirements/REQ-012.md
  - web/src/App.tsx
  - web/src/styles.css
  - web/src/App.test.tsx
  - web/vite.config.ts
req_ref: REQ-012
base_branch: main
scope: "Tracks the compact table layout for the authenticated game interface."
---

## Problem Statement

The authenticated page grows vertically with every game system. Players cannot scan current state and available Actions quickly.

## Recommended Direction

Replace repeated lists and stacked recipe cards with semantic compact tables. Keep API calls and gameplay behavior unchanged.

Use one table per named gameplay section. Give every table an accessible name. Use scoped column headers for gameplay tables and scoped row headers for the two-column summary. Put one entity or Action in each body row. Put controls in the last column with row-specific accessible names when an Action appears more than once.

Wrap every table in `.table-scroll`. The wrapper uses `max-width: 100%` and `overflow-x: auto`. Tables use full available width and a content-driven minimum width. The page container uses `box-sizing: border-box` and `width: 100%`. A computed-style test verifies these constraints. A 320 px browser check verifies `document.documentElement.scrollWidth <= window.innerWidth` while wide tables scroll inside their wrappers.

## Key Assumptions

- One row represents one visible game entity or Action.
- The player summary remains first.
- Narrow screens keep the page width stable.

## Acceptance Criteria

## MVP Scope / Not Doing

- No API contract change.
- No gameplay rule change.
- No new Action.
- No visual theme redesign.

## Tasks

```text
Task 1 [parallel: no]
└── Dependencies: none
```

- [x] Task 1 [parallel: no]: Replace the authenticated layout in `web/src/App.tsx` with semantic summary, Action, Resource, Inventory, Route, Gather, Convert, Craft, Building recipe, and Building tables. Give each table an accessible name. Use `scope="col"` headers for gameplay tables and `scope="row"` headers for the summary. Keep repeated controls distinguishable by entity-specific accessible names. Add compact responsive table styling in `web/src/styles.css` with the documented `.table-scroll`, table, and page width constraints. Update `web/src/App.test.tsx` to verify table names, header scopes, row contents, empty states, controls, preserved results, wrappers, and computed responsive styles. Run a 320 px browser check for page-level overflow and wrapper scrolling.
  - REQ-012.1: 已登入頁面必須在頂部表格顯示玩家身分、目前 AP 與目前 Location。
  - REQ-012.2: Resources 與 Inventory 必須各自使用緊湊表格顯示。
  - REQ-012.3: Available Routes、Gather、Convert 與 Craft 必須各自使用緊湊表格顯示。
  - REQ-012.4: Available Building recipes 與目前 Location 的 Buildings 必須各自使用緊湊表格顯示。
  - REQ-012.5: 每個 Resource、Item、Route、Action、recipe 或 Building 必須占用對應表格的一列。
  - REQ-012.6: 每列必須並排顯示該項目的主要資訊與可用 Action。
  - REQ-012.7: Recipe 的成本、inputs 與 output 必須顯示在同一列，不得再以巢狀清單增加頁面長度。
  - REQ-012.8: 表格化後必須保留目前所有玩家資訊、遊戲狀態、Action 與 Action 結果。
  - REQ-012.9: 沒有資料或可用 Action 時，對應區段必須保留明確的空狀態。
  - REQ-012.10: 窄螢幕顯示表格時，頁面不得產生水平方向溢出。
  - REQ-012.11: 本次變更不得改變 API contract 或遊戲規則。
  - Evidence: `npm test -- --run` passed 86 tests in 2 files, `npm run build` passed, and the 320 px browser check measured document width 320 px, body width 320 px, 10 tables, 10 wrappers, and wrapper scroll widths up to 1386 px while the page remained within the viewport.

## Review Issues

- [x] [Minor] Add `docs/architecture.md` to `source_paths`; `main...HEAD` modifies this implementation document, but the metadata omits it.
- [ ] [Minor] Add a test with multiple routes, recipes, or Buildings that asserts distinct accessible control names; the resolved plan issue requires coverage when multiple rows expose the same Action, while current tests only exercise one row per Action type.

## Plan Review Issues

- [x] Add an explicit dependency graph and `[parallel: no]` marker for Task 1. A one-task code block does not state the required dependency or parallel-execution decision.
- [x] Define the semantic table contract and its tests. Every gameplay table needs an accessible name, column headers must use scoped `th` cells, the summary table needs row or column header relationships, and controls must retain distinct accessible names when multiple rows expose the same Action.
- [x] Make the narrow-screen strategy verifiable. Specify the wrapper and table width constraints that keep the document within the viewport, then add a deterministic test or browser check that proves table content scrolls inside its wrapper without causing page-level horizontal overflow.
