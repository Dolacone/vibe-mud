---
title: "Architecture"
doc_type: architecture
last_reviewed: 2026-09-01
source_paths:
  - Dockerfile
  - cmd/server
  - internal/authapi
  - web
---

# Architecture

## System Boundary

One Go process serves the prebuilt React frontend, Google-only SSO, SQLite-backed application sessions, and the JSON game API. Production browsers use one Fly.io origin for static files, `/auth/*`, and `/api/*`.

## Backend

The backend uses Go, chi, `database/sql`, and `modernc.org/sqlite`. It owns authentication, action validation, game rules, persistence, access logs, and computation logs. SQLite runs on one connection because production uses one Fly.io Machine and one attached Volume.

SQLite stores absolute times as UTC Unix seconds. Store initialization converts legacy Unix nanoseconds once while preserving second-level expiry and AP behavior.

## Frontend

The frontend uses React, TypeScript, and Vite under `web/`. The Docker build compiles it into versioned static assets before copying `web/dist` into the runtime image. The Go static handler requires revalidation for the entry document and gives versioned assets an immutable one-year browser cache.

The browser calls relative `/auth/*` and `/api/*` paths without a proxy. The frontend publishes `/manifest.webmanifest`, 180, 192, and 512 pixel PNG icons, and iOS home-screen metadata for the `Vibe MUD` name, root launch path, and `standalone` display mode. It has no Service Worker or offline runtime. Direct browser navigation remains a normal web session. Authenticated users without a Player name see only the initial naming interface. The mobile-first App Shell appears after naming and uses the dynamic viewport with top and bottom safe-area insets. It keeps a two-row player status header and four-tab bottom navigation visible around one vertically scrollable content region. Focused controls scroll into the visible content area when the software keyboard changes the viewport. The header shows AP, current HP, `Weight <current>/<max>`, and nonzero Resources in stable definition order. Weight is green through 75%, yellow above 75% through 100%, and red above 100%. Each header row scrolls horizontally without widening the page. The bottom navigation uses labeled native buttons, exposes the current page, keeps focus on the activated destination, and provides touch targets of at least 44 by 44 CSS pixels. Map contains current Location, Routes, Move, and intercepted Combat. Area contains gathering, Buildings, every Building and extension control, installation targets, ground Item and Resource holdings, Pickup, Location Monster count, Attack, and active Combat results. Items contains Inventory, Item Drop, provider-backed Convert, and Craft. Character contains the Player name, its rename control, and Rest. It omits application identity fields and unimplemented progression placeholders. Action and Transfer feedback stays in the active content region. Backend state updates the fixed header and active tab without a page reload.

## Authentication

Google login starts at `GET /auth/google/login` and returns through `GET /auth/google/callback`. The backend stores OAuth attempts, application identities, Player profiles, and hashed sessions in SQLite. Google identity remains separate from the game-owned Player name. Session and OAuth flow cookies use `Secure`.

## Game State and API

`GET /api/me` returns application identity, `player_name` as `string|null`, AP, HP, location, Location Monster count, allowed Routes, available action options, recipes, current-Location Buildings and extensions, Inventory, all eight typed Resource quantities, nonzero public ground holdings, derived carrying weight, and the movement weight threshold. `PUT /api/player/name` accepts exactly `{"player_name":"..."}`. Store validation trims the display value, applies NFKC for uniqueness, folds only ASCII A-Z, and assigns ASCII characters one length point and other Unicode characters two. HTTP 200 returns the same complete current-user shape as `GET /api/me`. Missing authentication returns HTTP 401 with `{"error":"authentication required"}`. Malformed structure returns HTTP 400 with `{"error":"invalid player name input"}`. Semantic validation returns HTTP 400 with `{"error":"invalid player name"}`. A normalized duplicate returns HTTP 409 with `{"error":"player name unavailable"}`. Unnamed players cannot use Action or Transfer endpoints, which return HTTP 409 with `{"error":"player name required"}`. Public Item and Building durability uses status plus an integer percentage from 0 to 100. The percentage rounds upward. Public responses omit durability limits, remaining durability, retention details, and timestamps. In-progress Buildings and extensions expose integer `contributed_ap` and `required_ap`. The frontend derives a whole-number construction percentage with the existing downward-rounded ratio. Completed Buildings and extensions omit both construction keys. Extension display names already include their tier for players, so installed extensions, installation definitions, and Convert provider options render each display name without another tier suffix. The backend recalculates executable Actions, targets, methods, and recipes from that snapshot. It returns provider extension IDs for local Convert methods, installation targets for extension definitions, and per-Building and per-extension Action metadata. Game actions use dedicated endpoints under `/api/actions/*`. Attack accepts an empty JSON object. Convert accepts the typed `{method_id, quantity, provider_extension_id?}` contract or the legacy empty `{}` contract. Typed requests require a positive quantity and use backend-loaded method values. The legacy request resolves the current Location's authoritative `conversion_option`. Unknown fields and trailing JSON are rejected. Combat responses include authoritative player state, the ordered attacks, result, Monster display name, and drops. Extension installation, construction contribution, and removal use `/api/actions/install-extension`, `/api/actions/contribute-extension-construction`, and `/api/actions/remove-extension`. Access and computation logs use stable identifiers, outcomes, request IDs, and sanitized values. Player name logs omit submitted and stored names. They never include credentials, sessions, OAuth data, cookies, secrets, or raw input.

Pickup and Drop are AP-free Transfers under `/api/transfers/*`, not Actions. Item Transfers submit `item_status` to distinguish Active and Expired stacks. Active and Expired Items can be dropped, but only Active Items can be picked up. Resource Pickup can move any ground Resource to the player. Resource Drop is rejected without changing state. Resource Transfers do not submit Item status. The backend derives the player's current Location and atomically moves quantity between player holdings and public ground holdings.

AP persistence stores only the timestamp when the player will reach full AP. The backend derives current AP from its clock, caps it at 3000, and advances the timestamp when an action spends AP. No scheduler updates AP values.

Movement starts at `camp`. The backend resolves a submitted target against directed Routes stored in SQLite. Players can hold more than the 1000-unit movement weight threshold, but cannot move until their derived carrying weight returns to 1000 or less. A successful move updates AP and location in one transaction. Invalid Action, target, JSON input, or overweight state leaves AP and location unchanged and produces a sanitized error outcome log.

Each Location stores one aggregate Monster count and settlement timestamp. Authoritative reads settle complete 30-minute intervals with one backend roll per interval until the configured cap. Monster type is selected only when Combat begins. Active Attack costs 30 AP. A Move interception costs no AP and leaves the player at the origin. The backend resolves the complete Combat in one transaction and returns the ordered attacks, result, drops, HP, AP, Location, and Monster count.

Gathering is available only when the current Location has a backend-owned gathering rule. The frontend submits `{}` and cannot submit an item, quantity, cost, or location. A successful gather updates AP and adds a full-durability Item to Inventory in one transaction.

Global Convert methods are available at every Location. Completed Building extensions can provide more methods to players at the same Location. The frontend submits a method identifier, quantity, and provider extension when required. The backend loads AP cost, capacity, input, Resource output, Essence probability, and Essence quantity from typed SQLite definitions. A successful Convert atomically consumes one AP work unit and Active input, adds Resource, performs one Essence roll per input, and applies extension durability cost.

Crafting recipes are available at every Location. Each backend-owned recipe defines a base AP cost, one or more Resource inputs, zero or more Item inputs, and one explicit Item output. The frontend submits only `recipe_id`. A successful craft consumes every input and adds the output Item in one transaction. The first recipe creates one Wood Component from 10 Wood Resource and 10 AP.

Building recipes are Location-independent. `POST /api/actions/build` starts a Building at the player's current Location from a submitted `recipe_id`. `POST /api/actions/contribute-construction` lets a player at that Location submit a Building ID and positive AP. The Building Lv1 recipe consumes one Wood Component and 10 Wood Resource, then creates an in-progress Building with a 60 AP requirement snapshot. A contribution consumes no more than the remaining requirement. Completion starts a seven-day durability expiry and makes the saved extension slot capacity available.

Building extension definitions name a Package Item, tier, and construction AP requirement. The Building owner installs a Package into an empty slot. Players at that Location can contribute AP until the saved requirement is complete. Completed extensions provide their declared capability to players at that Location. The owner can remove an incomplete or completed extension without recovering its Package or contributed AP.

Sawmill T1 uses one Sawmill Package T1 and requires 30 construction AP. Its capability exposes the Sawmill Wood Convert method with capacity 6 and a 60-second Building durability cost per successful use. Capacity and durability cost are read from current definitions. Installed extensions save identity, name, tier, and construction AP snapshots.

The backend derives Building durability from its UTC Unix seconds clock. It converts remaining durability to a rounded-up public percentage. Completed Buildings are Active before expiry and Disabled for seven days after expiry. Disabled Buildings report 0% durability and cannot install, construct, or use extensions. Repair restores the saved extension state. State reads, builds, construction contributions, and repairs delete Buildings beyond the retention window together with their extensions. `POST /api/actions/repair-building` accepts only a Building ID. It lets any player at the Building's Location spend 10 AP and one Wood Resource. Repair extends durability by at most one hour and clamps expiry to seven days from the repair time.

Each Item definition supplies an internal durability limit. The current test setting gives every Item one hour of Active durability. Public Active Item durability is a rounded-up percentage. Expired Items report 0% durability without a retention countdown. Inventory and ground holdings keep separate Active and Expired stacks. Active stacks merge by quantity-weighted remaining time. Expired stacks retain the latest deletion time when merged. Expired Items remain visible for one day, count toward carrying weight, can be dropped, and cannot be picked up or consumed.

See [SQLite Schemas](schemas.md) for data structures and [Behavior Index](../requirements/BEHAVIOR.md) for agreed behavior.
