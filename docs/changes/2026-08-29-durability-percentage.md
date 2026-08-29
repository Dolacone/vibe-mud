---
title: "Durability percentage API"
status: Draft
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths: []
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

## Tasks

