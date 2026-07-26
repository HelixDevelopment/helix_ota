# Feature Specification: Pending Work Completion & Impeccable State

**Feature Directory**: `specs/002-pending-completion`

**Created**: 2026-07-26

**Status**: Draft

**Input**: "Plan now all pending work, everything unfinished or work on fixes on any existing issues!"

## User Scenarios & Testing

### User Story 1 - Project reaches impeccable completion state (Priority: P1)

An operator or project manager reviews the project and finds zero remaining software-actionable items — the workable-items database is synced with reality, all merged feature branches are cleaned up, the main branch is up to date, and any pre-existing code issues are documented or fixed.

**Why this priority**: The project has 16 items closed in code but still showing Queued in the DB, 4 commits on a feature branch not yet merged to main, and several pre-existing code issues still open. Until these are resolved, the project state is inconsistent between source and tracking.

**Independent Test**: The workable-items DB shows zero Queued items for software-fixed work, `feature/production-readiness` is merged to `main`, and `git status` shows a clean tree.

**Acceptance Scenarios**:
1. **Given** the workable-items DB shows items as Queued that have code fixes committed, **When** a DB sync is performed, **Then** those items reflect Closed/Fixed with evidence paths.
2. **Given** the feature/production-readiness branch has 4 commits, **When** main is merged into it and then it is merged back to main per §11.4.188, **Then** main carries all production-readiness improvements.
3. **Given** pre-existing build errors (unused import in wire.go), **When** fixed, **Then** `go build ./...` passes with zero errors.

---

### User Story 2 - Remaining software-only issues are resolved (Priority: P2)

A developer addresses the remaining Queued workable items that are NOT hardware-gated — including VM/emu hardening backlog items, QEMU e2e integration gaps, and any code quality issues that have been identified but not yet fixed.

**Why this priority**: These items are software-actionable and do not require physical hardware. They represent the last remaining gaps before the project is in a truly impeccable state.

**Independent Test**: All software-only Queued items in the workable-items DB transition from Queued to Closed with captured evidence.

**Acceptance Scenarios**:
1. **Given** VM/emu hardening backlog items (OTA-059), **When** addressed, **Then** the hardening backlog is reduced to zero remaining actionable items.
2. **Given** QEMU e2e integration gaps (OTA-071), **When** real QEMU integration tests are created, **Then** the test suite covers the QEMU code path with captured evidence.

---

### User Story 3 - Hardware-gated items are documented and deferred (Priority: P3)

The project documents exactly which items require physical hardware (RK3588 Orange Pi 5 Max), what the unblock conditions are, and what preparatory work can be done before hardware arrives.

**Why this priority**: Hardware-gated items block a subset of the remaining queue. Honest documentation with clear unblock conditions prevents wasted cycles and enables immediate action when hardware becomes available.

**Independent Test**: Every hardware-gated item has a documented unblock condition, estimated effort post-hardware, and any preparatory code that can be written now.

**Acceptance Scenarios**:
1. **Given** OTA-042 (RK3588 Tier-3 on-silicon validation) is hardware-blocked, **When** the unblock conditions are documented, **Then** the item description includes: what hardware is needed, what tests to run, and estimated effort.
2. **Given** OTA-038 (Linux/U-Boot ApplyPort) requires a specific target, **When** the preparatory kernel/slot-writer code is reviewed, **Then** any code that CAN be written without hardware is implemented.

## Requirements

### Functional Requirements

- **FR-001**: Workable-items database MUST be synced to reflect actual closed/fixed status for all 16 items with committed code changes.
- **FR-002**: Feature branch `feature/production-readiness` MUST be merged into `main` per §11.4.188 main→feature and feature→main merge cadence.
- **FR-003**: Pre-existing build error (unused import in `server/internal/api/wire.go`) MUST be fixed.
- **FR-004**: All software-only Queued workable items MUST be transitioned to Closed or documented as intentionally deferred.
- **FR-005**: Hardware-gated workable items MUST be updated with unblock conditions, required hardware, and estimated effort.
- **FR-006**: Any remaining constitutional compliance gaps (gates, carrier files, documentation exports) MUST be resolved.
- **FR-007**: The project `README.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `QWEN.md` MUST reflect the post-1.0.0 state with updated revision and fixed/implemented counts.

## Success Criteria

- **SC-001**: The workable-items DB Queued count drops from 22 to ≤6 (only hardware-gated items remain).
- **SC-002**: `main` branch contains all production-readiness commits and the feature branch is retired or merged.
- **SC-003**: `go build ./...` exits zero in `server/` with no unused import errors.
- **SC-004**: All 5 carrier files (CLAUDE/AGENTS/QWEN/GEMINI/GEMINI) are updated to post-1.0.0 state per §11.4.157.
- **SC-005**: The project root `README.md` reflects the current revision with updated feature counts and link table.

## Assumptions

- Hardware-gated items (OTA-038, 041, 042, 043) remain Queued until RK3588 Orange Pi 5 Max is available.
- Accounts M6-M8 (OTA-051-053) is a separate feature workstream per §11.4.167, not part of this completion round.
- HelixTrack sync (OTA-021) remains Operator-blocked until admin onboarding is performed via web UI.
- The merge to main is a fast-forward or no-conflict merge — the feature branch was created from main and main has had only doc/tooling commits since.
