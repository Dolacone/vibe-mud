---
title: "Ground asset transfers"
status: Draft
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths: []
req_ref: REQ-013
base_branch: main
scope: "Tracks AP-free transfers between player holdings and public Location ground assets."
---

## Problem Statement

Players cannot leave existing assets in the world or collect assets left by others. The game needs persistent public ground holdings without treating relocation as an AP-consuming Action.

## Recommended Direction

Persist ground Item and Resource quantities separately by Location and asset type. Use dedicated Transfer endpoints for atomic Pickup and Drop operations.

## Key Assumptions

- Ground capacity is unlimited.
- Ground holdings are public and ownerless.
- Transfers preserve total quantity and AP.
- Item durability remains outside this change.

## Acceptance Criteria

## MVP Scope / Not Doing

- No carrying weight.
- No Item durability.
- No Warehouse or Trade.
- No ground ownership, permissions, reservation, or history.
- No conversion or Building bonus change.

## Tasks

## Review Issues
