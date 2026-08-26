---
title: "Player movement"
status: Draft
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths: []
req_ref: REQ-005
base_branch: main
scope: "Tracks the first persistent location-changing game action."
---

## Problem Statement

Players can spend AP through `rest`, but they have no persistent location, backend-defined Route, or movement action that changes game state.

## Recommended Direction

Add backend-owned locations and directed Routes, persist one current location per player, and expose only the currently allowed `move` targets. Consume the Route AP cost and update location as one operation.

## Key Assumptions

- `camp` is the default location.
- `camp` and `forest_edge` have one directed Route in each direction.
- Each Route costs 20 AP.
- The backend owns Action and target allowlists.
- The frontend submits only the selected target identifier.

## Acceptance Criteria

The source of truth is `REQ-005`.

## MVP Scope / Not Doing

- Include two locations, two directed Routes, persistent player location, one `move` action, frontend controls, strict input rejection, and safe error logging.
- Exclude maps, discovery, more locations, variable costs, travel time, random events, encounters, inventory, and WebSocket delivery.

## Tasks

## Review Issues
