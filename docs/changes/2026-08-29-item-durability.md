---
title: "Item durability and expired retention"
status: Draft
created: 2026-08-29
doc_type: change
last_reviewed: 2026-08-29
source_paths: []
req_ref: REQ-015
base_branch: main
scope: "Tracks time-derived durability for stackable Items and seven-day expired retention for Items and Buildings."
---

## Problem Statement

Stackable Items remain usable forever. Players cannot distinguish active assets from expired assets, and the existing Building retention period conflicts with the agreed seven-day rule.

## Recommended Direction

Store durability limits on Item definitions. Persist separate Active and Expired holding stacks with expiry and deletion timestamps. Derive display state from Unix seconds, merge compatible stacks by the agreed formulas, and preserve timestamps during Transfers.

## Key Assumptions

- The MVP covers existing stackable Items in Inventory and on the ground.
- Active and Expired quantities use separate stacks.
- Resources remain non-expiring.
- Equipment and per-instance durability remain outside this change.

## Acceptance Criteria

## MVP Scope / Not Doing

- No Equipment or Item instances.
- No battle-based durability loss.
- No Item repair.
- No Resource expiry.

## Tasks

## Review Issues
