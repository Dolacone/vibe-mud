---
title: "Google SSO Login"
status: Draft
created: 2026-08-25
doc_type: change
last_reviewed: 2026-08-25
source_paths: []
req_ref: REQ-001
base_branch: main
scope: "Tracks the Google-only login API from design through review."
---

## Problem Statement

The backend has no authenticated user boundary. It cannot establish an application session from a verified Google identity or return the current application user as JSON.

## Recommended Direction

Build the first backend as a Go HTTP API. Keep Google token exchange inside the API process, persist application users and sessions in SQLite, and expose only application identity data.

## Key Assumptions

- The repository has no existing Go source convention. Implementation agents must follow `AGENTS.md`, this change document, and standard Go conventions.
- Google OAuth credentials and public URLs enter through environment variables.
- The browser uses an HTTP-only application session cookie after Google login.

## Acceptance Criteria

The source of truth is `REQ-001`.

## MVP Scope / Not Doing

- Include Google SSO, application sessions, stable user mapping, callback redirection, and current-user JSON.
- Exclude a frontend login page, logout, other identity providers, roles, account deletion, Google API access, and stored Google refresh tokens.

## Tasks

## Review Issues
