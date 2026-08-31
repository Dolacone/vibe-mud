---
title: "PWA standalone"
status: Issues-confirmed
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - requirements/BEHAVIOR.md
  - requirements/REQ-019.md
  - docs/architecture.md
  - docs/changes/2026-08-31-pwa-standalone.md
  - web/index.html
  - web/public/manifest.webmanifest
  - web/public/icons/icon.svg
  - web/public/icons/icon-180.png
  - web/public/icons/icon-192.png
  - web/public/icons/icon-512.png
  - web/src/pwa-contract.test.ts
  - cmd/server/static.go
  - cmd/server/static_test.go
  - scripts/test-container.sh
req_ref: REQ-019
base_branch: main
scope: "Tracks standalone mobile installation and launch behavior."
---

## Problem Statement

The frontend lacks the public metadata and production assets that browsers need to offer a PWA shortcut and standalone launch.

## Recommended Direction

Add standards-based PWA metadata for iOS Safari and Android Chrome. Use standalone display mode without adding offline gameplay or a Service Worker.

## Acceptance Criteria

REQ-019 is the source of truth. The plan copies every criterion into the owning tasks below.

## MVP Scope / Not Doing

- Add install and standalone launch metadata.
- Add home-screen icons required by supported mobile browsers.
- Do not add offline gameplay, background synchronization, push notifications, or a Service Worker.
- Do not package the game for an application store.

## Tasks

### Dependency graph

`Task 1 PWA metadata and assets -> Task 2 production-container and route verification`

- [x] Task 1 [parallel: no]: Add the public PWA metadata unit in `web/index.html` and `web/public/manifest.webmanifest`. Create 180, 192, and 512 pixel PNG icons under `web/public/icons/`. Declare `name`, `short_name`, `start_url: "/"`, `scope: "/"`, `display: "standalone"`, and 192 plus 512 pixel icons with the 512 pixel icon supporting `any maskable`. Add the manifest link, `theme-color`, `apple-mobile-web-app-capable=yes`, `apple-mobile-web-app-title=Vibe MUD`, `apple-mobile-web-app-status-bar-style=black-translucent`, and the 180 pixel `apple-touch-icon` link. Add `web/src/pwa-contract.test.ts` in the same commit to parse the production files, verify metadata and icon dimensions, and assert no Service Worker registration. Update `docs/architecture.md` in the same commit. This task has two source/logic files: `web/index.html` and `web/public/manifest.webmanifest`; icons are assets and the test is not a source/logic file.
  - REQ-019.2: Manifest 必須宣告 `Vibe MUD` 名稱、遊戲圖示、根啟動路徑、根 scope 與 `standalone` 顯示模式。
  - REQ-019.3: 前端入口文件必須公開提供 iOS home-screen 名稱、圖示、啟用 web app 與狀態列 metadata。

## Validation

- `npm test -- --run src/pwa-contract.test.ts` passed: 4 tests.
- `npm run build` passed: Vite production build completed.

- [x] Task 2 [parallel: no]: Update `cmd/server/static.go` to serve `.webmanifest` responses as `application/manifest+json`, with a focused Go test in the same commit. Extend `scripts/test-container.sh` as the automated production verification. Build and run the production image, fetch `/`, `/manifest.webmanifest`, every manifest-declared icon, and a client-side frontend route through the Go static handler, then assert status, content type, and response content. Assert the built frontend contains no Service Worker registration, and run the Go and frontend test suites. Record command results in this change document. Keep Google login, API routes, existing safe-area styles, fixed status rows, and fixed bottom navigation unchanged. Do not perform or require iOS or Android device access, production deployment, OAuth account access, standalone chrome checks, or manual interaction checks. This task has two source/logic files: `cmd/server/static.go` and `scripts/test-container.sh`.
  - REQ-019.1: 前端 origin 必須公開提供 Web App Manifest，且瀏覽器可以讀取正確的 manifest content type。
  - REQ-019.4: Production container 必須提供入口文件、Manifest 與 Manifest 宣告的全部圖示。
  - REQ-019.5: 玩家直接使用瀏覽器開啟根網址或前端路徑時，遊戲必須繼續作為一般網頁運作。
  - REQ-019.6: 此 MVP 不得註冊 Service Worker，也不提供離線遊戲、背景同步或推播通知。
  - REQ-019.7: 實際 iOS Safari 與 Android Chrome 的安裝、獨立視窗、Google 登入及遊戲操作不屬於自動驗收範圍。

## Task 2 Validation

- `go test -count=1 -ldflags=-linkmode=external ./...` passed: both backend packages passed.
- `npm test -- --run` passed: 5 test files and 144 tests passed.
- `npm run build` passed: Vite production build completed.
- `bash scripts/test-container.sh` ran the Go suite, frontend suite, and build successfully, then stopped at Docker build because the Docker daemon was unavailable at `/var/run/docker.sock`.

## Review Issues

The prior draft issues are obsolete because REQ-019 now defines only automated metadata and container acceptance. The plan keeps real-device installation, standalone chrome, OAuth, and mobile interaction checks out of scope.

- [ ] [Major] `REQ-019.4` 的 production container 驗收未完成。`bash scripts/test-container.sh` 在 `docker build` 因 `/var/run/docker.sock` 無 Docker daemon 而退出 1。入口文件、Manifest、所有宣告圖示與 client-side route 尚未由 production image 驗證。

## Blocked

[Logic Conflict] Production container 驗收需要持續可用的 Docker-compatible daemon，但目前環境無法維持 Podman machine 的 socket 連線。

[Attempted Solutions] Docker Desktop 未安裝，原始 `bash scripts/test-container.sh` 首次在 Docker build 以 exit 1 結束。啟動 `podman-machine-default` 並以暫時 `DOCKER_HOST` 指向其 socket 後，`docker info` 首次成功，但 harness 在 Docker build 階段仍以 exit 1 結束，machine 隨後停止。

[Required Clarification] 提供可在完整 harness 執行期間持續運作的 Docker-compatible daemon，或確認可改用其他 production image 驗證方式。

## Plan Review Issues

- [x] Replaced the prior plan with a dependency-ordered, automatically verifiable two-task plan traced to the updated REQ-019.1 through REQ-019.7 criteria.
- [x] REQ-019.1 is assigned to Task 1, but its origin response and content type are only exercised by Task 2. The current Go static handler does not set a Manifest content type, and Go 1.22 has no built-in `.webmanifest` mapping, so `http.ServeContent` depends on runtime MIME data that the plan does not control. Assign REQ-019.1 to Task 2, explicitly set `application/manifest+json` in `cmd/server/static.go`, add the corresponding Go test in the same commit, and keep Task 2 within the two-file source/logic limit.
- [x] `source_paths` omits `requirements/REQ-019.md` and `requirements/BEHAVIOR.md`, although both differ from `main` on this branch. Add the files already changed by capture, then append implementation paths only when their tasks modify them.
