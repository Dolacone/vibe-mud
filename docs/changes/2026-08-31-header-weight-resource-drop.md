---
title: "Header weight and Resource Drop"
status: Draft
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - requirements/BEHAVIOR.md
  - requirements/REQ-007.md
  - requirements/REQ-012.md
  - requirements/REQ-013.md
  - requirements/REQ-014.md
req_ref: REQ-007, REQ-012, REQ-013, REQ-014
base_branch: main
scope: "Tracks fixed header content, weight presentation, duplicate state removal, and Resource Drop removal."
---

## Problem Statement

The fixed header repeats the player name while weight remains in Map and Resource balances remain in Items. Resource Drop also permits a transfer that the current game rules no longer allow.

## Recommended Direction

Keep AP, HP, weight, and nonzero Resources in the fixed header. Remove duplicate weight and Resource balances from main tabs. Preserve Item Drop and Resource Pickup while rejecting Resource Drop.

## Key Assumptions

- Exactly 75% of the weight limit uses the green state.
- Existing ground Resource remains visible and available for Pickup.
- Resource Drop rejection applies to direct API requests as well as the frontend control.

## Acceptance Criteria

## MVP Scope / Not Doing

- Do not change carrying-weight calculation or the movement threshold.
- Do not remove Item Drop or Resource Pickup.
- Do not migrate or delete existing ground Resource.
- Do not add HP behavior.

## Tasks

## Review Issues
