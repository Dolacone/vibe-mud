---
title: "Player name and tab requirements"
status: Draft
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths:
  - requirements/BEHAVIOR.md
  - requirements/REQ-002.md
  - requirements/REQ-012.md
  - requirements/REQ-020.md
  - requirements/REQ-021.md
  - requirements/REQ-022.md
  - requirements/REQ-023.md
  - requirements/REQ-024.md
req_ref: REQ-002, REQ-012, REQ-020, REQ-021, REQ-022, REQ-023, REQ-024
base_branch: main
scope: "Tracks persistent player names, forced initial naming, in-game renaming, and the split of tab-specific frontend requirements."
---

## Problem Statement

Google identity currently doubles as the visible player identity. The Character tab also exposes login identity fields and unrelated placeholders instead of one game-owned player name.

## Recommended Direction

Persist a game-owned unique Player name. Require unnamed players to choose one before entering the game. Keep renaming and Rest in Character while separating each tab from the shared App Shell requirement.

## Key Assumptions

- Existing users have not chosen a Player name and must complete initial naming after deployment.
- Name comparison uses Unicode NFKC normalization and case-insensitive matching.
- Name display preserves the accepted trimmed spelling while uniqueness uses its normalized key.
- ASCII characters cost one length point. Other Unicode characters cost two.

## Acceptance Criteria

REQ-002, REQ-012, and REQ-020 through REQ-024 are the sources of truth.

## MVP Scope / Not Doing

- Do not add name history, rename cooldowns, moderation, reserved words, or rename costs.
- Do not add equipment, skills, or level behavior.
- Do not change Google OAuth identity or session persistence.

## Tasks

## Review Issues
