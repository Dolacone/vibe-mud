<!-- last_reviewed: 2026-08-31 -->

# Changelog

## 2026-08-31

- Added install metadata and home-screen icons for standalone PWA launch.
- Added explicit Web App Manifest content type and production-container verification.
- Kept browser navigation unchanged without adding a Service Worker or offline behavior.
- Moved carrying weight into the fixed header with green, yellow, and red thresholds.
- Removed duplicate weight and Resource balances from gameplay tabs.
- Rejected Resource Drop while retaining Item Drop and Resource Pickup.

## 2026-08-30

- Replaced the authenticated development page with a mobile-first four-tab game shell.
- Added fixed player status, swipeable Resource summaries, safe-area support, and bottom navigation.
- Moved existing gameplay controls into Map, Area, Items, and Character views.

## 2026-08-29

- Removed completed construction AP from public Building and extension state.
- Removed duplicate tier suffixes from extension display names.
- Replaced public Item and Building durability seconds with rounded-up integer percentages.
- Filtered gameplay Actions, targets, methods, and recipes by authoritative backend state.
- Kept all eight Resource balances and other player state visible when options are unavailable.
- Made the frontend render only backend-returned gameplay options.
- Added configurable hand and Sawmill conversion methods with Wood Essence output.
- Added the Sawmill Package T1 recipe and Sawmill T1 Building extension.
- Added shared extension construction, owner removal, and Building durability consumption.
- Added compact conversion and Building extension controls to the frontend.
- Added atomic extension computation logs with exact Building, extension, and AP attribution.
- Added one-hour definition-backed durability for every Item type.
- Added separate Active and Expired Item stacks with weighted durability merging.
- Retained Expired Items for one day while counting them toward carrying weight.
- Allowed Expired Item Drop while rejecting Expired Item Pickup and Action inputs.
- Extended Disabled Building retention from three days to seven days.
- Added Item durability calculation and cleanup logs.

## 2026-08-28

- Changed all persistent timestamps from Unix nanoseconds to Unix seconds.
- Added idempotent migration and converted-value logging for existing timestamps.
- Added seven-day time-derived durability for completed Buildings.
- Added a shared repair Action that spends 10 AP and one Wood Resource for up to one hour.
- Added a three-day repair window before expired Buildings disappear and release their Location slot.
- Replaced the authenticated page's stacked lists and cards with compact gameplay tables.
- Added responsive table regions that contain wide content on narrow screens.
- Added unlimited public ground holdings for Item and Resource quantities at each Location.
- Added AP-free Pickup and Drop Transfers with atomic quantity updates.
- Added definition-backed Item and Resource weights with a derived player carrying total.
- Blocked movement above the 1000-unit threshold without limiting asset collection or transfers.
- Added authoritative carrying weight state, computation logs, and an overweight interface warning.

## 2026-08-27

- Replaced the generic Resource balance with separate Food, Wood, Stone, Metal, Fiber, Hide, Medicinal, and Arcane quantities.
- Updated conversion to produce Wood Resource while preserving existing Action behavior.
- Added backend-owned crafting recipes with Resource inputs, optional Item inputs, and explicit Item outputs.
- Added Wood Component crafting at every Location with atomic AP, Resource, and Inventory updates.
- Added Building Lv1 construction with Wood Component and Wood Resource inputs.
- Added shared AP contributions, persistent progress, and completed Building state.

## 2026-08-26

- Moved the production frontend into the Fly.io application for same-origin API and OAuth access.
- Replaced the Cloudflare Pages Functions proxy with browser-cached static files served by Go.
- Added persistent player locations, backend-owned Routes, and AP-consuming movement.
- Added strict Action and target validation with safe rejection logging.
- Added location-specific gathering that atomically consumes AP and collects Wood.
- Added persistent Inventory state and frontend display.
- Added persistent Resource conversion that atomically consumes AP and Wood.

## 2026-08-25

- Added Google-only SSO and SQLite-backed application sessions.
- Added the authenticated current-user JSON API.
- Added single-Machine Fly.io packaging with persistent SQLite storage.
- Added the Cloudflare Pages login interface and backend-confirmed identity display.
- Added the allow-listed Pages Functions proxy for same-origin authentication.
- Added timestamp-derived AP with a 3000 cap and one AP restored per complete minute.
- Added the authenticated `rest` action across the API and Cloudflare Pages interface.
