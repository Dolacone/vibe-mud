---
title: "Unix seconds storage"
status: Draft
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths: []
req_ref: "REQ-001, REQ-003"
base_branch: main
scope: "Tracks the internal timestamp precision change while preserving authentication and AP behavior."
---

## Problem Statement

Persistent timestamps use Unix nanoseconds despite game and authentication behavior needing only second precision. The storage change must not create a new user-facing requirement.

## Recommended Direction

Store absolute times as UTC Unix seconds. Convert legacy nanoseconds during Store initialization. Preserve the existing authentication and AP behavior from REQ-001 and REQ-003.

## Key Assumptions

- Subsecond precision has no game or authentication consumer.
- Legacy timestamps contain contemporary positive UTC values.
- Internal migration telemetry belongs in implementation review, not requirements.

## Acceptance Criteria

## MVP Scope / Not Doing

- Keep all user-facing behavior unchanged.
- Do not expose timestamps through the public API.
- Do not add a schema version table.
- Do not add Building durability in this change.

## Tasks

## Review Issues
