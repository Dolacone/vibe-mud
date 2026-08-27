---
title: "Typed resources"
status: Draft
created: 2026-08-27
doc_type: change
last_reviewed: 2026-08-27
source_paths: []
req_ref: REQ-007
related_req_refs:
  - REQ-008
base_branch: main
scope: "Tracks the replacement of one generic Resource balance with eight typed Resource quantities."
---

## Problem Statement

The game stores one generic Resource balance, so it cannot represent Resource categories with separate quantities.

## Recommended Direction

Store Resource definitions separately from per-player quantities. Return all eight definitions with zero for missing player rows. Keep the existing `convert` Action and make its output explicitly target Wood Resource.

## Key Assumptions

- The Resource types are Food, Wood, Stone, Metal, Fiber, Hide, Medicinal, and Arcane.
- A missing player Resource row represents quantity 0.
- Existing generic Resource balances can be discarded.
- The current `convert` location, input quantity, output quantity, AP cost, and request format remain unchanged.
- The current `convert` output becomes Wood Resource.

## Acceptance Criteria

The source of truth is REQ-007. Existing `convert` behavior remains defined by REQ-008.

## MVP Scope / Not Doing

- Include typed Resource definitions, per-player quantities, full API and UI display, and Wood Resource output from `convert`.
- Exclude weight, Item expiration, batch dismantling, new raw materials, new conversions, building storage, Rare Components, and Resource consumers.

## Tasks

## Review Issues
