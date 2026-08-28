---
title: "Architecture"
doc_type: architecture
last_reviewed: 2026-08-28
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

The browser calls relative `/auth/*` and `/api/*` paths without a proxy. The frontend displays backend-authoritative identity, AP, location, Route, gathering option, conversion option, crafting recipes, Building recipes, current-Location Buildings, Inventory, typed Resources, and action results.

## Authentication

Google login starts at `GET /auth/google/login` and returns through `GET /auth/google/callback`. The backend stores OAuth attempts, application identities, and hashed sessions in SQLite. Session and OAuth flow cookies use `Secure`.

## Game State and API

`GET /api/me` returns identity, AP, location, allowed Routes, available action options, recipes, current-Location Buildings, Inventory, and all eight typed Resource quantities. Game actions use dedicated endpoints under `/api/actions/*`.

AP persistence stores only the timestamp when the player will reach full AP. The backend derives current AP from its clock, caps it at 3000, and advances the timestamp when an action spends AP. No scheduler updates AP values.

Movement starts at `camp`. The backend resolves a submitted target against directed Routes stored in SQLite. A successful move updates AP and location in one transaction. Invalid Action, target, or JSON input leaves state unchanged and produces a sanitized error outcome log.

Gathering is available only when the current Location has a backend-owned gathering rule. The frontend submits `{}` and cannot submit an item, quantity, cost, or location. A successful gather updates AP and Inventory quantity in one transaction.

Conversion is available only when the current Location has a backend-owned conversion rule. The frontend submits `{}` and cannot submit gameplay values. A successful convert consumes AP and Inventory quantity, then increases the rule's typed Resource quantity in one transaction. The current rule produces Wood Resource.

Crafting recipes are available at every Location. Each backend-owned recipe defines a base AP cost, one or more Resource inputs, zero or more Item inputs, and one explicit Item output. The frontend submits only `recipe_id`. A successful craft consumes every input and adds the output Item in one transaction. The first recipe creates one Wood Component from 10 Wood Resource and 10 AP.

Building recipes are Location-independent. `POST /api/actions/build` starts a Building at the player's current Location from a submitted `recipe_id`. `POST /api/actions/contribute-construction` lets a player at that Location submit a Building ID and positive AP. The Building Lv1 recipe consumes one Wood Component and 10 Wood Resource, then creates an in-progress Building with a 60 AP requirement snapshot. A contribution consumes no more than the remaining requirement. Completion exposes one empty extension slot and starts a seven-day durability expiry.

The backend derives Building durability from its UTC Unix seconds clock. Completed Buildings are Active before expiry and Disabled for three days after expiry. State reads, builds, construction contributions, and repairs delete Buildings beyond that window. `POST /api/actions/repair-building` accepts only a Building ID. It lets any player at the Building's Location spend 10 AP and one Wood Resource. Repair extends durability by at most one hour and clamps expiry to seven days from the repair time. Building responses expose the maximum duration plus nullable derived status and remaining seconds. Under-construction Buildings use null durability fields. Destroyed Buildings are absent.

See [SQLite Schemas](schemas.md) for data structures and [Behavior Index](../requirements/BEHAVIOR.md) for agreed behavior.
