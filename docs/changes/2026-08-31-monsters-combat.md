---
title: "Location monsters and combat"
status: Draft
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - docs/changes/2026-08-31-monsters-combat.md
req_ref: REQ-005, REQ-012, REQ-018, REQ-021, REQ-022, REQ-025, REQ-026
base_branch: main
scope: "Tracks lazy Location Monster population, movement interception, automatic combat, HP recovery, drops, and the mobile interface."
---

## Problem Statement

Locations have no persistent Monster pressure. Players cannot fight, lose HP, receive Monster drops, or face risk when leaving an occupied Location.

## Recommended Direction

Settle Monster population from elapsed UTC Unix seconds when authoritative state is read. Resolve active attacks and movement interceptions in one backend transaction. Return the combat transcript and updated state to the existing mobile interface.

## Key Assumptions

- Monster type is selected only when combat starts.
- One request resolves the complete combat.
- Player HP uses one timestamp that represents recovery to full HP.
- Random generation, interception, combat damage, and drops use backend-owned randomness.

## Acceptance Criteria

REQ-005, REQ-012, REQ-018, REQ-021, REQ-022, REQ-025, and REQ-026 are the sources of truth.

## MVP Scope / Not Doing

- Do not add equipment, defense, skills, critical hits, accuracy, evasion, or player combat choices.
- Do not add background workers or scheduled Monster generation.
- Do not expose unavailable attacks or unselected Monster types.
- Do not add Monster movement, individual persistent Monster records, or combat sessions.

## Tasks

## Review Issues
