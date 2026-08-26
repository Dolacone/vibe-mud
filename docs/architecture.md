---
title: "Architecture"
doc_type: architecture
last_reviewed: 2026-08-26
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

## Frontend

The frontend uses React, TypeScript, and Vite under `web/`. The Docker build compiles it into versioned static assets before copying `web/dist` into the runtime image. The Go static handler requires revalidation for the entry document and gives versioned assets an immutable one-year browser cache.

The browser calls relative `/auth/*` and `/api/*` paths without a proxy. The frontend displays backend-authoritative identity, AP, location, Route, gathering option, conversion option, Inventory, Resource, and action results.

## Authentication

Google login starts at `GET /auth/google/login` and returns through `GET /auth/google/callback`. The backend stores OAuth attempts, application identities, and hashed sessions in SQLite. Session and OAuth flow cookies use `Secure`.

## Game State and API

`GET /api/me` returns identity, AP, location, allowed Routes, available action options, Inventory, and Resource. Game actions use `POST /api/actions/rest`, `POST /api/actions/move`, `POST /api/actions/gather`, and `POST /api/actions/convert`.

AP persistence stores only the timestamp when the player will reach full AP. The backend derives current AP from its clock, caps it at 3000, and advances the timestamp when an action spends AP. No scheduler updates AP values.

Movement starts at `camp`. The backend resolves a submitted target against directed Routes stored in SQLite. A successful move updates AP and location in one transaction. Invalid Action, target, or JSON input leaves state unchanged and produces a sanitized error outcome log.

Gathering is available only when the current Location has a backend-owned gathering rule. The frontend submits `{}` and cannot submit an item, quantity, cost, or location. A successful gather updates AP and Inventory quantity in one transaction.

Conversion is available only when the current Location has a backend-owned conversion rule. The frontend submits `{}` and cannot submit gameplay values. A successful convert consumes AP and Inventory quantity, then increases Resource in one transaction.

See [SQLite Schemas](schemas.md) for data structures and [Behavior Index](../requirements/BEHAVIOR.md) for agreed behavior.
