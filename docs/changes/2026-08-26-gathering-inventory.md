---
title: "Gathering and inventory"
status: Draft
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths: []
req_ref: REQ-006
base_branch: main
scope: "Tracks the first location-specific item collection loop."
---

## Problem Statement

Movement changes the player's location, but locations do not provide a persistent item-producing action. Players also have no Inventory for collected items.

## Recommended Direction

Add a backend-owned `gather` rule for `forest_edge`, consume AP and add `Wood` in one SQLite transaction, expose Inventory through the existing player state, and render the updated state in the frontend.

## Key Assumptions

- `gather` is available only at `forest_edge`.
- Each successful `gather` costs 10 AP.
- Each successful `gather` yields exactly one `Wood` item.
- The backend owns the allowed location, item, quantity, and AP cost.
- Inventory stores item quantities and persists across sessions.

## Acceptance Criteria

The source of truth is `REQ-006`.

## MVP Scope / Not Doing

- Include one gather location, one deterministic item yield, persistent Inventory, atomic AP consumption, strict input rejection, safe error logging, and frontend Inventory display.
- Exclude tools, skills, equipment, random yield, resource conversion, item use, trading, crafting, capacity, and item loss.

## Tasks

## Review Issues
