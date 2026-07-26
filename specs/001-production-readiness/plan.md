# Implementation Plan: Production Readiness — Gap Closure & Full Completion

**Branch**: `feature/production-readiness` | **Date**: 2026-07-26 | **Spec**: specs/001-production-readiness/spec.md

**Input**: Feature specification from `specs/001-production-readiness/spec.md`

## Summary

Close all 47 gaps identified in the completion report (`docs/research/completion/report/2026.07.25.md`) across 7 phases (critical blockers, high-priority implementation, medium implementation, constitutional gates compliance, testing/quality, ADR resolution, infrastructure) to bring Helix OTA to a full production-ready state. Parallel multi-track execution targeting ~15-20 calendar days with 3 parallel streams. All 47 gaps must close before 1.0.0 ships.

## Technical Context

**Language/Version**: Go 1.26 (server/control plane), TypeScript 5.6 (dashboard), Kotlin/KMP (Android agent)

**Primary Dependencies**: Gin (gin-gonic v1.12), pgx/v5 (PostgreSQL), quic-go (HTTP/3), Prometheus client, Brotli, ota-protocol, ota-artifact-validator, ota-rollout-engine, ota-telemetry-schema, digital.vasic.containers, digital.vasic.http3, React 18, React Router v6, Vite, Playwright, Vitest, pixelmatch

**Storage**: PostgreSQL (relational — accounts, projects, devices, deployments, telemetry, audit), MinIO/S3 (artifact blobs), SQLite (workable-items DB at docs/workable_items.db), Redis (optional caching, only where measured need exists)

**Testing**: Go `testing` package (server unit/integration), Vitest + Testing Library (dashboard unit), Playwright (e2e + host-render), pixelmatch (visual regression), axe-core (a11y), Go fuzz (API input validation), custom stress+chaos suite, HelixQA + Challenges submodules

**Target Platform**: Linux server (amd64) — initial production target is Android 15 on Orange Pi 5 Max; per spec §104 the architecture supports forward-OS expansion

**Project Type**: Web service (Go/Gin REST API) + React SPA dashboard + Android OTA agent (Kotlin/KMP)

**Performance Goals**: Rollout evaluation <500ms p95, artifact validation <2s for typical 100MB image, API response <200ms p95 under 1000 concurrent requests, dashboard page load <3s at 90th percentile

**Constraints**: All 47 gaps must close before 1.0.0 release (§11.4.40 full-suite retest required); all work produces captured physical evidence per HelixConstitution (§11.4.5, §11.4.69, §11.4.107); no force-push (§11.4.113); all changes cross independent code-review (§11.4.125/§11.4.142); rootless Podman containers only (§11.4.161); existing Go codebase already at ~45 implemented routes; architecture locked per master design (no new major changes)

**Scale/Scope**: 47 gaps across 7 phases, ~80-100 person-days, 3 parallel tracks, ~15-20 calendar days

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Applicable principles from project constitution:**

- **I. Native A/B Safety & Integrity** (G-01 ApplyPort WriteAndArm must preserve slot semantics; G-12/G-39/G-40 Android agent integration must wire update_engine bridge)
- **II. Signature Trust & Credential Safety** (G-22 RLS; G-29 credential rotation; G-26 TUF supply-chain; all credentials runtime-load only)
- **III. Reuse-First, Decouple-Hard** (G-23 security submodule integration; G-38–G-47 submodule consumption; verify catalogue before any new build)
- **IV. Constitution Inheritance & Governance Compliance** (Gates G-21–G-29: all 11 unwired propagation gates are Phase 3 scope; §11.4.157 five-carrier lockstep)
- **V. Full-Stack Anti-Bluff Quality Covenant** (every gap closure carries captured physical evidence; FR-014 mutation testing for all gates; FR-015 stress+chaos all 12 components; no metadata-only/absence-of-error PASS)

**Gate evaluation:** PASS — no violations. All gaps are tracked, prioritized, and constitutionally justified. Complexity tracking not required (no unjustified violations).

**Re-check note:** Phase 1 design (data model + contracts) must not introduce gaps that violate the reuse-first or constitutional compliance principles. Re-evaluate after research is complete.

## Project Structure

### Documentation (this feature)

```text
specs/001-production-readiness/
├── spec.md              # Feature specification (/speckit.specify command output)
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── webhook-payload-schema.md
│   └── automation-interface-schema.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Created by /speckit.tasks command
```

### Source Code (repository root)

```text
# Web application (Go backend + React frontend)
server/
├── cmd/
├── internal/
│   ├── api/               # HTTP handlers (targets for G-02, G-06, G-07, G-08, G-09, G-10, G-71)
│   ├── device/             # ApplyPort (target for G-01, G-04, G-13)
│   ├── store/              # PostgreSQL + migrations (targets for G-17, G-18, G-19, G-20)
│   ├── security/           # Rate limiting, RLS, session (targets for G-21, G-22, G-25)
│   └── config/             # Defaults (target for G-21)
├── deploy/                 # Compose files (targets for G-60, G-61, G-64, G-65)
└── tests/

dashboard/
├── src/
├── e2e/
└── hostrender/

submodules/
├── ota-android-agent/      # Target for G-12, G-39
├── ota-update-engine-bridge/ # Target for G-12, G-40
├── security/               # Target for G-23, G-38
├── challenges/             # Target for G-46
└── helixqa/               # Target for G-47
```

**Structure Decision**: Existing repository layout is preserved. All gap-closure work targets specific existing files and directories identified in the completion report's file-by-file appendix. No new top-level directories are introduced.

## Complexity Tracking

> Not applicable — Constitution Check passes with zero violations.
