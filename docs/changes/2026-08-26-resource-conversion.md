---
title: "Resource conversion"
status: Draft
created: 2026-08-26
doc_type: change
last_reviewed: 2026-08-26
source_paths: []
req_ref: REQ-007
base_branch: main
scope: "Tracks the first Inventory-to-Resource conversion loop."
---

## Problem Statement

Gathering produces persistent `Wood` items, but players cannot convert collected items into a persistent Resource balance with a later spending purpose.

## Recommended Direction

Add a backend-owned conversion rule for `camp`, consume AP and `Wood`, increment Resource, and return the complete player state from one SQLite transaction.

## Key Assumptions

- `convert` is available only at `camp`.
- Each successful `convert` costs 1 AP and 1 `Wood`.
- Each successful `convert` yields exactly 1 Resource.
- Resource is one generic persistent integer balance, not an Inventory item.
- The backend owns all conversion values.
- The only valid `convert` request payload is an empty JSON object: `{}`.

## Acceptance Criteria

The source of truth is `REQ-007`.

## MVP Scope / Not Doing

- Include one deterministic conversion rule, persistent Resource balance, atomic AP and Inventory mutation, strict input rejection, safe error logging, and frontend Resource display.
- Exclude item creation, Resource spending, buildings, trading, conversion ratios, batch conversion, bonuses, capacity, and multiple Resource types.

## Tasks

## Review Issues
