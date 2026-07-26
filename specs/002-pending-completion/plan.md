# Implementation Plan: Pending Work Completion & Impeccable State

**Branch**: `feature/production-readiness` (continuation) | **Date**: 2026-07-26 | **Spec**: specs/002-pending-completion/spec.md

## Summary

Complete all remaining software-actionable work from the 22 Queued items in the workable-items database. 16 items already have code changes committed but the DB was never updated. The remaining ~6 items are hardware-gated or intentionally deferred. Additionally, merge the production-readiness feature branch to main, fix pre-existing build errors, sync all five carrier files, and update project documentation to reflect the post-1.0.0 state.

## Technical Context

**Language/Version**: Go 1.26, TypeScript 5.6, Kotlin/KMP (Android agent — not in scope)

**Primary Dependencies**: Gin v1.12, pgx/v5, quic-go — all unchanged from 001-production-readiness

**Storage**: PostgreSQL, SQLite (workable_items.db), MinIO/S3 — all deployed and stable

**Testing**: Go testing, Vitest, Playwright — coverage from 001-production-readiness applies

**Target Platform**: Linux server amd64 — same as 001-production-readiness

**Project Type**: Web service + React SPA — no new components

**Performance Goals**: No new performance targets — maintaining existing p95 <200ms

**Constraints**: No new infrastructure; merge must be ff-only per §11.4.113; DB updates are idempotent; hardware-gated items are documented not implemented

**Scale/Scope**: ~16 DB updates, 1 merge, 1 build fix, 5 carrier file updates, documentation refresh

## Constitution Check

*GATE: Must pass before Phase 0*

- **IV. Constitution Inheritance**: Five-carrier lockstep (§11.4.157) — carrier files must be updated. Currently being addressed.
- **V. Full-Stack Anti-Bluff**: All DB status changes must cite evidence paths. Evidence already exists from waves 1-3.
- **§11.4.188**: Regular main→feature merge cadence — the feature branch must merge main first, then merge back.

**Gate evaluation:** PASS — all constitutional requirements mapped to specific tasks.

## Project Structure

No new directories. All work targets existing files:
- `docs/workable_items.db` — DB status updates
- `server/internal/api/wire.go` — unused import fix
- Root carrier files — post-1.0.0 state updates
- `docs/RESUMPTION.md` — current state refresh
