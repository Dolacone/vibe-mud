---
title: "Completed construction visibility"
status: Draft
created: 2026-08-30
doc_type: change
last_reviewed: 2026-08-30
source_paths: []
req_ref: REQ-010, REQ-016
base_branch: main
scope: "Remove obsolete construction values from completed Buildings and extensions."
---

## Problem Statement

Completed Buildings and extensions still expose and display final construction AP values. Those values no longer help the player act and make the Building table harder to scan.

## Recommended Direction

Keep construction values only while an entity is under construction. Omit those fields after completion and render extension display names without a duplicate tier suffix.

## Key Assumptions

- Internal construction snapshots remain stored for history and domain logic.
- Lifecycle status remains public because clients must distinguish construction from completion.
- In-progress entities retain contributed AP, required AP, and percentage.

## Acceptance Criteria

REQ-010 and REQ-016 are the sources of truth. The plan will copy each affected criterion into its owning task.

## MVP Scope / Not Doing

- Do not change construction costs, contribution rules, completion, persistence, or logs.
- Do not remove construction information from in-progress entities.
- Do not redesign the Building table.

## Tasks

