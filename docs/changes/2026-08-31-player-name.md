---
title: "Player name and tab requirements"
status: Issues-confirmed
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - docs/architecture.md
  - docs/schemas.md
  - docs/terminology.md
  - go.mod
  - go.sum
  - internal/authapi/store.go
  - internal/authapi/store_test.go
  - internal/authapi/server.go
  - internal/authapi/server_test.go
  - requirements/BEHAVIOR.md
  - requirements/REQ-002.md
  - requirements/REQ-012.md
  - requirements/REQ-020.md
  - requirements/REQ-021.md
  - requirements/REQ-022.md
  - requirements/REQ-023.md
  - requirements/REQ-024.md
  - web/src/auth.ts
  - web/src/auth.test.ts
  - web/src/App.tsx
  - web/src/App.test.tsx
req_ref: REQ-002, REQ-012, REQ-020, REQ-021, REQ-022, REQ-023, REQ-024
base_branch: main
scope: "Tracks persistent player names, forced initial naming, in-game renaming, and the split of tab-specific frontend requirements."
---

## Problem Statement

Google identity currently doubles as the visible player identity. The Character tab also exposes login identity fields and unrelated placeholders instead of one game-owned player name.

## Recommended Direction

Persist a game-owned unique Player name. Require unnamed players to choose one before entering the game. Keep renaming and Rest in Character while separating each tab from the shared App Shell requirement.

## Key Assumptions

- Existing users have not chosen a Player name and must complete initial naming after deployment.
- Name comparison uses Unicode NFKC normalization and folds ASCII `A` through `Z` to lowercase.
- Name display preserves the accepted trimmed spelling while uniqueness uses its normalized key.
- ASCII characters cost one length point. Other Unicode characters cost two.

## Acceptance Criteria

REQ-002, REQ-012, and REQ-020 through REQ-024 are the sources of truth.

## MVP Scope / Not Doing

- Do not add name history, rename cooldowns, moderation, reserved words, or rename costs.
- Do not add equipment, skills, or level behavior.
- Do not change Google OAuth identity or session persistence.

## Tasks

Dependency graph: `Task 1 storage -> Task 2 API -> Task 3 frontend contract -> Task 4 frontend flow`. Task 4 also verifies the extracted tab requirements against the existing interface.

- [x] Task 1 [parallel: no]: Add the `player_profiles` schema, existing-user initialization, Player name normalization, weighted-length validation, unique normalized keys, reads, and atomic writes in `internal/authapi/store.go`. Add Store tests in the same commit for initial unnamed profiles, persistence, mixed ASCII and Unicode length boundaries, control characters, NFKC collisions, ASCII English case collisions, non-English case distinctions, concurrent duplicate attempts, and unchanged state after rejection. Add `golang.org/x/text/unicode/norm` only for deterministic NFKC normalization. Fold only ASCII `A` through `Z` for the REQ-defined English case comparison. Keep `docs/schemas.md` and `docs/terminology.md` aligned.
  - REQ-020.1: 每個應用程式使用者必須具有獨立於 Google 顯示名稱的持久化 Player name。
  - REQ-020.3: 初次命名與後續改名必須使用相同的名稱規則。
  - REQ-020.4: 系統必須去除名稱首尾空白，並以 ASCII 字元 1 點、其他 Unicode 字元 2 點計算名稱長度。
  - REQ-020.5: 系統必須接受 1 至 16 點的名稱，使純中文字名稱最多 8 個字，純英文字名稱最多 16 個字。
  - REQ-020.6: 系統必須拒絕空白名稱、超過 16 點的名稱，以及包含控制字元的名稱。
  - REQ-020.7: Player name 必須唯一。系統必須使用 Unicode NFKC 與不分英文大小寫的結果判定重名。
  - REQ-020.10: 命名失敗時，既有 Player name、AP 與其他玩家狀態必須保持不變。
  - REQ-020.11: 重新整理或重新登入後，系統必須保留最新 Player name。

- [x] Task 2 [parallel: no]: Add `player_name` as `string|null` to the existing `GET /api/me` body and implement `PUT /api/player/name` with exact request `{"player_name":"..."}` in `internal/authapi/server.go`. A successful update returns HTTP 200 with the same complete current-user body as `GET /api/me`. Return HTTP 401 `{"error":"authentication required"}` without a session. Return HTTP 400 `{"error":"invalid player name input"}` for malformed JSON, unknown or duplicate fields, trailing JSON, missing `player_name`, or a non-string value. Return HTTP 400 `{"error":"invalid player name"}` for a semantic validation failure. Return HTTP 409 `{"error":"player name unavailable"}` for a normalized duplicate. Require a Player name before every Action or Transfer endpoint and return HTTP 409 `{"error":"player name required"}` when absent. Keep authentication, `GET /api/me`, and the naming endpoint available. Log every naming access and result with user ID, action, outcome, reason when present, and request ID. Never log the submitted or stored name. Add API tests in the same commit for exact contracts, action and transfer gating, successful state preservation across AP, Inventory, Resource, Location and other player state, rejected state preservation, post-rename `GET /api/me`, a new session read, and sanitized logs. Keep `docs/architecture.md` aligned.
  - REQ-002.5: 前端必須向後端查詢目前登入的應用程式使用者，不得從前端狀態或 Google 回應自行推定身分。
  - REQ-002.6: 有效 session 存在時，前端必須顯示已登入遊戲介面。
  - REQ-020.2: 尚未設定 Player name 的玩家進入遊戲時，必須先完成命名才能使用其他遊戲內容或操作。
  - REQ-020.3: 初次命名與後續改名必須使用相同的名稱規則。
  - REQ-020.7: Player name 必須唯一。系統必須使用 Unicode NFKC 與不分英文大小寫的結果判定重名。
  - REQ-020.8: 玩家必須能隨時修改自己的 Player name，且不消耗 AP 或其他資產。
  - REQ-020.9: 命名成功後，前端必須立即使用後端回傳的最新 Player name，不得要求重新登入或重新載入頁面。
  - REQ-020.10: 命名失敗時，既有 Player name、AP 與其他玩家狀態必須保持不變。
  - REQ-020.11: 重新整理或重新登入後，系統必須保留最新 Player name。
  - REQ-020.12: Backend 必須把初次命名與改名的 access 及結果寫入 stdout，並包含 user ID、action、outcome 與 request ID。
  - REQ-020.13: Backend log 不得包含輸入名稱、Google 身分資料、credentials、session、OAuth 資料、cookie 或 secrets。

- [x] Task 3 [parallel: no]: Extend `web/src/auth.ts` with `player_name: string|null` parsing and a typed Player name update client. Send `PUT /api/player/name` with exact JSON `{"player_name":"..."}`. Parse HTTP 200 as the same complete current-user shape used by `GET /api/me`. Map HTTP 400 `invalid player name input` and `invalid player name` to invalid outcomes. Map HTTP 409 `player name unavailable` to an unavailable outcome. Treat HTTP 401 as unauthenticated. Reject every unexpected status, error body, or success body without inventing player state. Add client contract tests in the same commit for named and unnamed identity responses, exact serialization, authoritative success, each rejection class, and invalid response rejection.
  - REQ-002.5: 前端必須向後端查詢目前登入的應用程式使用者，不得從前端狀態或 Google 回應自行推定身分。
  - REQ-002.6: 有效 session 存在時，前端必須顯示已登入遊戲介面。
  - REQ-002.7: 有效 session 不存在時，前端必須顯示未登入狀態與登入操作，不得顯示先前取得的使用者身分。
  - REQ-020.2: 尚未設定 Player name 的玩家進入遊戲時，必須先完成命名才能使用其他遊戲內容或操作。
  - REQ-020.3: 初次命名與後續改名必須使用相同的名稱規則。
  - REQ-020.9: 命名成功後，前端必須立即使用後端回傳的最新 Player name，不得要求重新登入或重新載入頁面。
  - REQ-020.10: 命名失敗時，既有 Player name、AP 與其他玩家狀態必須保持不變。

- [x] Task 4 [parallel: no]: Gate `web/src/App.tsx` on the authoritative nullable Player name. Show only the initial naming form before entry. After naming, preserve the shared App Shell and all extracted Map, Area, and Items behavior. Replace Character identity and progression tables with current Player name, a button-controlled rename form, and the existing Rest interface. Hide application user ID, Google display name, email, equipment, skills, and level. Apply successful names immediately and retain the current name after rejection. Add App tests in the same commit for initial naming, every name result, no reload, Character contents, Rest, and regressions for each extracted tab. Run the existing GameShell component tests as regression evidence for fixed status rows, swipe behavior, weight boundaries, and duplicate-state removal.
  - REQ-012.1: 已登入介面必須使用 mobile-first 的單一 App Shell。
  - REQ-012.2: App Shell 頂端必須固定顯示兩排核心狀態，且不得隨主分頁切換。
  - REQ-012.3: 第一排必須依序顯示目前 AP、目前 HP 與 `Weight <current>/<max>`，不得顯示玩家名稱。
  - REQ-012.4: HP 尚未實作時，第一排必須顯示明確 placeholder，不得顯示虛構數值。
  - REQ-012.5: 第二排必須依固定順序顯示玩家持有量大於 0 的 Resource，且最多包含既有 8 種 Resource。
  - REQ-012.6: 兩排核心狀態都必須維持單行，不得自動換行。
  - REQ-012.7: 任一核心狀態列超過可用寬度時，玩家必須能在該列水平 swipe 查看後續資訊。
  - REQ-012.8: 核心狀態列的水平 swipe 不得造成整個頁面水平溢出，也不得阻止主內容垂直捲動。
  - REQ-012.9: App Shell 底部必須固定提供 `地圖`、`地區`、`道具`、`角色` 四個主分頁。
  - REQ-012.10: 切換主分頁不得重新登入或遺失最新 authoritative player state。
  - REQ-012.22: 前端只能顯示後端依 REQ-018 回傳的 Action、target、method 與 recipe，不得自行補回不可用選項。
  - REQ-012.23: Action 完成後，頂端核心狀態與目前主分頁必須使用最新後端狀態更新，不得要求完整頁面重新載入。
  - REQ-012.24: 個別主分頁載入或操作失敗時，頂端核心狀態與底部主分頁必須保持可見。
  - REQ-012.25: `Weight` 小於或等於上限 75% 時必須顯示綠色，超過 75% 且小於或等於上限時必須顯示黃色，超過上限時必須顯示紅色。
  - REQ-012.26: `地圖` 與其他主內容不得重複顯示 header 已提供的目前重量與重量上限。
  - REQ-020.2: 尚未設定 Player name 的玩家進入遊戲時，必須先完成命名才能使用其他遊戲內容或操作。
  - REQ-020.8: 玩家必須能隨時修改自己的 Player name，且不消耗 AP 或其他資產。
  - REQ-020.9: 命名成功後，前端必須立即使用後端回傳的最新 Player name，不得要求重新登入或重新載入頁面。
  - REQ-020.10: 命名失敗時，既有 Player name、AP 與其他玩家狀態必須保持不變。
  - REQ-021.1: `地圖` 必須顯示玩家目前 Location。
  - REQ-021.2: `地圖` 必須顯示後端回傳的可抵達 Location、Route 與 AP 成本。
  - REQ-021.3: 玩家必須能在 `地圖` 對後端回傳的 Route 執行 Move。
  - REQ-022.1: `地區` 必須顯示目前 Location 的可互動 gathering option。
  - REQ-022.2: `地區` 必須顯示目前 Location 的 Buildings、extensions 與後端回傳的可用操作。
  - REQ-022.3: `地區` 必須顯示目前 Location 的地面 Item 與 Resource，並保留既有 Pickup 與 Drop 規則。
  - REQ-022.4: `地區` 必須顯示後端回傳的 Building recipes，並允許玩家建立 Building。
  - REQ-023.1: `道具` 必須顯示玩家 Inventory，以及 Active 與 Expired Item 的既有狀態與操作。
  - REQ-023.2: `道具` 不得重複顯示 header 已提供的 Resource 持有量。
  - REQ-023.3: `道具` 必須顯示後端回傳的 Convert methods 與 Craft recipes，並允許玩家執行對應操作。
  - REQ-024.1: `角色` 必須顯示後端回傳的目前 Player name。
  - REQ-024.2: `角色` 必須提供修改 Player name 的操作。
  - REQ-024.3: 既有 Rest 操作必須保留在 `角色`，直到後續 REQ 指定其他位置。
  - REQ-024.4: `角色` 不得顯示應用程式 user ID、Google 顯示名稱或 email。
- REQ-024.5: `角色` 不得顯示尚未實作的裝備、技能或等級 placeholder。

## Verification

- Task 3: `npm test` passed with 5 test files and 153 tests.
- Task 3: `npm run build` passed with TypeScript compilation and Vite production output.
- Task 4: `npm test -- --run src/App.test.tsx` passed with 63 tests.
- Task 4: `npm test -- --run src/GameShell.test.tsx` passed with 6 tests.
- Task 4: `npm test` passed with 5 test files and 158 tests.
- Task 4: `npm run build` passed with TypeScript compilation and Vite production output.

## Review Issues

- [x] [Major] `normalizePlayerName` 在 NFKC 後計算點數，違反顯示名稱的 ASCII 與 Unicode 計分規則。九個全形 ASCII 字元會正規化成九個 ASCII 字元並通過，但原輸入應計 18 點並拒絕。現有邊界測試未涵蓋會改變字元類別的 NFKC 輸入。
- [x] [Major] `requirePlayerName` 在 Player profile 查詢失敗或遺失時放行 Action 與 Transfer。命名閘門因此採 fail-open，未命名玩家可在 profile 缺漏或資料庫讀取錯誤時執行遊戲操作。
- [x] [Major] 角色改名收到 HTTP 401 時，前端只顯示 session 過期訊息。它仍保留 App Shell 與先前 Player name，違反 REQ-002.7 的未登入畫面及身分清除要求。Task 4 也缺少此結果與一般錯誤結果的 App 測試。
- [x] [Major] REQ-020.8 的 Plan Review Issue 尚未解決。API 測試只對預設空狀態執行初次命名，未設定非預設 AP、Inventory、Resource 或 Location，也未執行後續改名。測試無法偵測資產刪除或狀態重設，卻已將對應問題標為完成。
- [ ] [Major] 修正完成後，變更文件仍維持 `Issues-confirmed`。Implement 階段未依生命週期改成 `Ready-to-review`，因此不符合 review 入口條件。

## Plan Review Issues

- [x] Task 1 計畫使用 Unicode case folding，但 REQ-020.7 只指定不分英文大小寫。Unicode case folding 會讓非英文大小寫與部分字串產生額外碰撞。請改成符合 REQ 的英文大小寫比對，或回到 capture 修訂 REQ。
- [x] Task 2 與 Task 3 沒有共享可檢查的完整 API contract。請指定 `/api/me` 的 Player name 欄位、`PUT /api/player/name` 的精確 JSON request 與 success response，以及 unauthenticated、unnamed、malformed、invalid、duplicate 的 HTTP status 與 error body。同步更新 `docs/architecture.md`，否則「exact error classes」沒有判定基準。
- [x] REQ-020.11 只追蹤到 Store task，無法證明重新整理或重新登入會從後端取得最新名稱。請讓 Task 2 追蹤完整 REQ-020.11，並以改名後的 `GET /api/me` 與新 session 測試最新名稱。
- [x] REQ-020.8 要求成功改名不消耗 AP 或其他資產，但 Task 2 的測試意圖只寫「renaming」與 response state。請明確測試成功初次命名及改名前後的 AP、Inventory、Resource、Location 與其他玩家狀態不變。
- [x] Task 4 宣稱保留 shared App Shell，但只追蹤 REQ-012.1、9、10、22 至 24。請加入受此流程影響的完整 REQ-012.2 至 8、25、26，並指定對應回歸測試，或縮小保留宣稱並說明既有獨立測試如何覆蓋這些行為。
