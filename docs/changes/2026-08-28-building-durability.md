---
title: "Building durability and repair"
status: Draft
created: 2026-08-28
doc_type: change
last_reviewed: 2026-08-28
source_paths: []
req_ref: REQ-011
base_branch: main
scope: "Tracks Building Lv1 durability, disablement, destruction, and shared repair."
---

## Problem Statement

Completed Buildings persist without maintenance pressure. The backend needs time-derived durability and one shared repair Action.

## Recommended Direction

Store a Building durability expiry in Unix seconds. Derive Active and Disabled state from backend time. Remove Buildings after the three-day disabled window.

## Key Assumptions

- Durability begins when construction completes.
- Repair consumes the acting player's state.
- Destruction removes the Building row and releases its ownership slot.
- No background scheduler is required for user-visible state.

## Acceptance Criteria

## MVP Scope / Not Doing

- No extension installation or usage wear.
- No public or private access mode.
- No co-owner, user, or ownership transfer.
- No monster durability multiplier.
- No change to the one-Building ownership limit.

## Tasks

## Review Issues
