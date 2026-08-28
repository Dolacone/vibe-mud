---
title: "Player carrying weight"
status: Draft
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths: []
req_ref: REQ-014
base_branch: main
scope: "Tracks derived carrying weight and movement blocking above the weight threshold."
---

## Problem Statement

Player holdings have no weight calculation. The game cannot show how much a player carries or prevent an overloaded player from moving.

## Recommended Direction

Store one integer unit weight on each Item and Resource definition. Derive current carrying weight from player quantities. Return the derived value with authoritative player state and block Move while it exceeds 1000.

## Key Assumptions

- Players can hold unlimited Item and Resource quantities.
- The weight threshold limits only Move.
- Definition weights use integer units.
- Item durability remains outside this change.

## Acceptance Criteria

## MVP Scope / Not Doing

- No Item durability.
- No Warehouse, equipment bonus, vehicle, or volume rule.
- No balancing system or batch Convert.

## Tasks

## Review Issues
