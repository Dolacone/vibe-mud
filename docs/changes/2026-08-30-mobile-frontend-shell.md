---
title: "Mobile frontend shell"
status: Ready-to-implement
created: 2026-08-30
doc_type: change
last_reviewed: 2026-08-30
source_paths:
  - docs/architecture.md
  - web/src/GameShell.tsx
  - web/src/GameShell.test.tsx
  - web/src/styles.css
req_ref: REQ-012
base_branch: main
scope: "Replace the development table page with a mobile-first four-tab game shell."
---

## Problem Statement

The authenticated frontend places every game system on one long table page. It does not provide stable mobile navigation or keep the most important player state visible.

## Recommended Direction

Create one mobile-first App Shell with a fixed two-row status header, a scrollable active-tab content region, and fixed bottom navigation for Map, Area, Items, and Character.

## Key Assumptions

- The existing backend API remains unchanged.
- HP, equipment, skills, and level use explicit placeholders.
- Existing Actions move between tabs without changing domain behavior.

## Acceptance Criteria

REQ-012 is the source of truth. The plan will copy every criterion into one owning task.

## MVP Scope / Not Doing

- Do not add game rules, backend fields, routes, or persistence.
- Do not add HP, equipment, skills, or level behavior.
- Do not add illustrations, a world map renderer, animations, or chat.

## Dependency Graph

`Task 1 App Shell primitives -> Task 2 gameplay tab integration`

## Tasks

- [x] Task 1 [parallel: no]: Build the fixed mobile shell, status header, and bottom navigation in `web/src/GameShell.tsx` and `web/src/styles.css`. Add component tests in the same commit. Use the stable Resource order Food, Wood, Stone, Metal, Fiber, Hide, Medicinal, Arcane. Display `HP --` until HP exists. Use `100dvh`, top and bottom safe-area insets, one middle vertical scroll region, scroll padding, and focused-control visibility when the software keyboard changes the viewport. Use semantic navigation with labeled native buttons, `aria-current="page"`, visible focus, focus retained on the activated destination, and touch targets of at least 44 by 44 CSS pixels.
  - REQ-012.1: 已登入介面必須使用 mobile-first 的單一 App Shell。
  - REQ-012.2: App Shell 頂端必須固定顯示兩排核心狀態，且不得隨主分頁切換。
  - REQ-012.3: 第一排必須依序顯示玩家名稱、目前 AP 與目前 HP。
  - REQ-012.4: HP 尚未實作時，第一排必須顯示明確 placeholder，不得顯示虛構數值。
  - REQ-012.5: 第二排必須依固定順序顯示玩家持有量大於 0 的 Resource，且最多包含既有 8 種 Resource。
  - REQ-012.6: 兩排核心狀態都必須維持單行，不得自動換行。
  - REQ-012.7: 任一核心狀態列超過可用寬度時，玩家必須能在該列水平 swipe 查看後續資訊。
  - REQ-012.8: 核心狀態列的水平 swipe 不得造成整個頁面水平溢出，也不得阻止主內容垂直捲動。
  - REQ-012.9: App Shell 底部必須固定提供 `地圖`、`地區`、`道具`、`角色` 四個主分頁。
  - REQ-012.10: 切換主分頁不得重新登入或遺失最新 authoritative player state。
- [ ] Task 2 [parallel: no]: Move existing gameplay sections into the four tabs and connect them to the shell in `web/src/App.tsx`. Add a local-only layout harness in `web/src/browser-fixture.tsx` with its root `web/browser-fixture.html`; Vite's production build must not include this non-entry HTML. Update application tests and verify the production build in the same commit. Map uses a compact Route list and owns carrying weight plus the movement threshold. Area owns gathering, Building creation, every Building install, contribution, removal, and repair control, installation targets, ground holdings, and Pickup. Items owns Inventory, all Resource balances, Item and Resource Drop, provider-backed Convert, and Craft. Character owns Rest and progression placeholders. Action and Transfer feedback renders in the active content region so failures cannot unmount the shell. Tests must cover global `available_actions`, per-Building and per-extension actions, installation targets, provider extension IDs, Routes, methods, and recipes. An authoritative success response must update the header without changing tabs. Request and domain failures must leave the header and navigation mounted. After integration, run `npm exec vite -- --host 127.0.0.1 --port 4173` from `web/` and open `http://127.0.0.1:4173/browser-fixture.html`. The harness must supply a long player name, all eight nonzero Resources, long tab content, and a quantity input without calling `/api/me`. At 320 by 568 and 390 by 844, drag each header row and verify its `scrollLeft` changes, vertically scroll the content and verify the header and navigation bounds remain fixed, and verify the document does not overflow horizontally. Focus the quantity input, reduce the viewport height to simulate a software keyboard, and verify the control remains above the bottom navigation. Record the observed evidence in this change document.
  - REQ-012.11: `地圖` 必須顯示玩家目前 Location，以及後端回傳的可抵達 Location、Route 與 AP 成本。
  - REQ-012.12: 玩家必須能在 `地圖` 對後端回傳的 Route 執行 Move。
  - REQ-012.13: `地區` 必須顯示目前 Location 的可互動 gathering option。
  - REQ-012.14: `地區` 必須顯示目前 Location 的 Buildings、extensions 與後端回傳的可用操作。
  - REQ-012.15: `地區` 必須顯示目前 Location 的地面 Item 與 Resource，並保留既有 Pickup 與 Drop 規則。
  - REQ-012.16: `地區` 必須顯示後端回傳的 Building recipes，並允許玩家建立 Building。
  - REQ-012.17: `道具` 必須顯示玩家 Inventory，以及 Active 與 Expired Item 的既有狀態與操作。
  - REQ-012.18: `道具` 必須顯示全部 8 種 Resource 的持有量。
  - REQ-012.19: `道具` 必須顯示後端回傳的 Convert methods 與 Craft recipes，並允許玩家執行對應操作。
  - REQ-012.20: `角色` 必須顯示玩家角色資訊，並為尚未實作的裝備、技能與等級提供明確 placeholder。
  - REQ-012.21: 既有 Rest 操作必須放在 `角色`，直到後續 REQ 指定其他位置。
  - REQ-012.22: 前端只能顯示後端依 REQ-018 回傳的 Action、target、method 與 recipe，不得自行補回不可用選項。
  - REQ-012.23: Action 完成後，頂端核心狀態與目前主分頁必須使用最新後端狀態更新，不得要求完整頁面重新載入。
  - REQ-012.24: 個別主分頁載入或操作失敗時，頂端核心狀態與底部主分頁必須保持可見。

## Plan Review Issues

- [x] Task 1 and the architecture document omit mobile safe-area handling, dynamic viewport sizing, and focused-input behavior when the software keyboard changes the viewport. Specify top and bottom safe-area insets and keep focused controls visible without letting fixed navigation cover them.
- [x] Task 1 relies only on component tests for REQ-012.6 through REQ-012.8, but jsdom cannot verify real horizontal swipe, vertical pan coexistence, or page overflow. Add a mobile-viewport browser verification for both header rows and the vertically scrollable content region.
- [x] Task 1 does not define keyboard and accessibility behavior for the four tabs. Specify semantic navigation or tab roles, selected-state exposure, keyboard activation, focus behavior after switching, accessible labels, and usable touch targets.
- [x] Task 2 does not map every existing gameplay surface to an owning tab. Explicitly place carrying weight and its movement threshold, Item and Resource Drop controls, action and Transfer feedback, and every Building install, contribute, remove, and repair control so the shell rewrite cannot silently remove them.
- [x] Task 2 does not define a verification matrix for backend availability and shell persistence. Cover global `available_actions`, per-Building and per-extension actions, installation targets, provider IDs, Routes, methods, and recipes; also verify that authoritative success state updates the header without changing tabs and that request or domain failures leave the shell mounted.
- [x] Task 1's real-browser verification is not executable at that stage because `GameShell` does not enter the browser-rendered `App` until Task 2, and the plan defines no standalone fixture. Move the verification after Task 2 integration or define a reachable fixture, then name the server URL, authenticated or mocked state, touch-swipe method, and software-keyboard method used to collect the evidence.
- [x] Task 2 still lacks an executable `/api/me` fixture. The repository has no browser test dependency, and the available Browser API does not expose request interception, so Vite preview will return its normal 404 response. Specify a supported mock server, proxy, fixture route, or test harness with exact startup commands and keep its files inside the task limit.
