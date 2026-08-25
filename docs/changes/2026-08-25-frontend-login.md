---
title: "Frontend Login"
status: Draft
created: 2026-08-25
doc_type: change
last_reviewed: 2026-08-25
source_paths: []
req_ref: REQ-002
base_branch: main
scope: "Tracks the Cloudflare Pages frontend login from design through review."
---

## Problem Statement

The deployed backend can establish an application session, but users have no Cloudflare-hosted interface that starts login or asks the backend which application user owns the current session.

## Recommended Direction

Build a single-page React, TypeScript, and Vite frontend for Cloudflare Pages. Start login through the existing backend and treat its current-user API as the only authenticated identity source.

## Key Assumptions

- The backend remains a separate Fly.io API and continues to own Google OAuth and application sessions.
- The frontend sends credentialed requests to the configured backend origin.
- Cloudflare Pages serves static assets without Cloudflare Workers.

## Acceptance Criteria

The source of truth is `REQ-002`.

## MVP Scope / Not Doing

- Include a login action, authenticated identity display, unauthenticated state, and backend failure state.
- Exclude logout, routing, game state, chat, WebSocket, account settings, and a component framework.

## Tasks

## Review Issues
