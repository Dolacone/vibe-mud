---
title: "Mobile frontend shell"
status: Draft
created: 2026-08-30
doc_type: change
last_reviewed: 2026-08-30
source_paths: []
req_ref: REQ-012
base_branch: main
scope: "Replace the development table page with a mobile-first four-tab game shell."
---

## Problem Statement

The authenticated frontend places every game system on one long table page. It does not provide stable mobile navigation or keep the most important player state visible.

## Recommended Direction

Create one mobile-first App Shell with a fixed two-row status header, a scrollable active-tab content region, and fixed bottom navigation for Map, Area, Items, and Character.

## Key Assumptions

- The existing backend API remains unchanged.
- HP, equipment, skills, and level use explicit placeholders.
- Existing Actions move between tabs without changing domain behavior.

## Acceptance Criteria

REQ-012 is the source of truth. The plan will copy every criterion into one owning task.

## MVP Scope / Not Doing

- Do not add game rules, backend fields, routes, or persistence.
- Do not add HP, equipment, skills, or level behavior.
- Do not add illustrations, a world map renderer, animations, or chat.

## Tasks

