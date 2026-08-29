---
title: "Sawmill product chain"
status: Draft
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths: []
req_ref: [REQ-008, REQ-010, REQ-011, REQ-014, REQ-015, REQ-016, REQ-017]
base_branch: main
scope: "Tracks configurable Convert methods, the generic Building extension lifecycle, and Sawmill T1."
---

## Problem Statement

Convert currently uses one fixed Camp-only rule. The game has no Essence output, extension construction, or Building function that improves processing capacity.

## Recommended Direction

Store balance values in typed domain definitions. Keep deterministic formulas in code. Separate Convert, Building durability, Item lifecycle, extension lifecycle, and Sawmill product behavior across their existing responsibility boundaries.

## Key Assumptions

- Each Convert execution consumes one complete AP work unit.
- Only construction AP is copied into an installed extension snapshot.
- Existing Sawmills read current capacity and Building durability cost from their definition.
- A completed extension is available to every player at the same Location until Building permissions are introduced in a separate change.
- Random Essence outcomes use an injectable deterministic source in tests.
- Balance changes use versioned SQLite migrations. The MVP has no administration interface.

## Acceptance Criteria

The confirmed requirements referenced by `req_ref` are the source of truth.

## MVP Scope / Not Doing

- No Sawmill T2 or higher tier.
- No Essence type other than Wood Essence T1.
- No supporting Resource in the Sawmill Package T1 recipe.
- No Building access mode, co-owner, user role, or ownership transfer.
- No item quality or Craft success probability.
- No generic balance key-value table.

## Tasks

## Review Issues
