---
title: "Building construction"
status: Draft
created: 2026-08-27
doc_type: change
last_reviewed: 2026-08-27
source_paths:
  - requirements/BEHAVIOR.md
  - requirements/REQ-010.md
req_ref: REQ-010
base_branch: main
scope: "Tracks Building Lv1 placement, shared AP contributions, completion, and persistence."
---

## Problem Statement

Players can create Inventory Items but cannot turn those Items into persistent world Buildings or cooperate on construction.

## Recommended Direction

Store backend-owned Building recipes separately from player-owned Buildings. Starting construction consumes recipe inputs and creates an in-progress Building. Players at the same Location can contribute AP until the stored requirement is met.

## Key Assumptions

- Building recipe values are backend-owned balance data.
- The construction AP requirement is copied into each Building when construction starts.
- An in-progress Building counts toward the owner and Location uniqueness rule.
- The server consumes only the remaining AP after an oversized contribution.

## Acceptance Criteria

REQ-010 is the source of truth. The plan stage will copy each criterion into its owning task.

## MVP Scope / Not Doing

- No Building removal or demolition.
- No extension installation.
- No durability or maintenance.
- No Building effects.
- No recipe acquisition.

## Tasks

## Review Issues
