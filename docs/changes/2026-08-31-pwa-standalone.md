---
title: "PWA standalone"
status: Draft
created: 2026-08-31
doc_type: change
last_reviewed: 2026-08-31
source_paths: []
req_ref: REQ-019
base_branch: main
scope: "Tracks standalone mobile installation and launch behavior."
---

## Problem Statement

The mobile frontend opens as a normal browser shortcut with browser chrome. Players need an installable home-screen entry that launches the existing game in a standalone window.

## Recommended Direction

Add standards-based PWA metadata for iOS Safari and Android Chrome. Use standalone display mode without adding offline gameplay or a Service Worker.

## Key Assumptions

- Fly.io continues to serve the frontend and backend from one HTTPS origin.
- The existing mobile App Shell remains the installed experience.
- Google OAuth continues to use the existing same-origin callback.

## Acceptance Criteria

## MVP Scope / Not Doing

- Add install and standalone launch metadata.
- Add home-screen icons required by supported mobile browsers.
- Do not add offline gameplay, background synchronization, push notifications, or a Service Worker.
- Do not package the game for an application store.

## Tasks

## Review Issues
