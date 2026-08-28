---
title: "Compact table interface"
status: Draft
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths: []
req_ref: REQ-012
base_branch: main
scope: "Tracks the compact table layout for the authenticated game interface."
---

## Problem Statement

The authenticated page grows vertically with every game system. Players cannot scan current state and available Actions quickly.

## Recommended Direction

Replace repeated lists and stacked recipe cards with semantic compact tables. Keep API calls and gameplay behavior unchanged.

## Key Assumptions

- One row represents one visible game entity or Action.
- The player summary remains first.
- Narrow screens keep the page width stable.

## Acceptance Criteria

## MVP Scope / Not Doing

- No API contract change.
- No gameplay rule change.
- No new Action.
- No visual theme redesign.

## Tasks

## Review Issues
