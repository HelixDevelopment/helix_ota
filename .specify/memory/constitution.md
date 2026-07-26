<!--
  Sync Impact Report

  Version change: 0.0.0 (template) → 1.0.0 (initial populated constitution)
  Modified principles: all five populated from template placeholders
    [PRINCIPLE_1_NAME] → I. Native A/B Safety & Integrity
    [PRINCIPLE_2_NAME] → II. Signature Trust & Credential Safety
    [PRINCIPLE_3_NAME] → III. Reuse-First, Decouple-Hard
    [PRINCIPLE_4_NAME] → IV. Constitution Inheritance & Governance Compliance
    [PRINCIPLE_5_NAME] → V. Full-Stack Anti-Bluff Quality Covenant
  Added sections: Technology Stack, Development Workflow & Quality Gates
  Removed sections: none (template had no non-placeholder sections)
  Templates requiring updates:
    - .specify/templates/plan-template.md — ✅ no changes needed (generic "Constitution Check" phrasing compatible)
    - .specify/templates/spec-template.md — ✅ no changes needed
    - .specify/templates/tasks-template.md — ✅ no changes needed
    - .specify/templates/checklist-template.md — ✅ no changes needed
  Follow-up TODOs: none — all placeholders resolved
-->

# Helix OTA Constitution

## Core Principles

### I. Native A/B Safety & Integrity

The Android OTA update path MUST preserve native A/B slot semantics
(update_engine + AVB/dm-verity + automatic boot-failure rollback). The
control plane MAY orchestrate, gate, and observe updates but MUST NOT
bypass the device's slot-verification and auto-rollback guarantees. The
update artifact MUST pass structure, hash, signature, and metadata
validation before any deploy action is accepted. The server-side
validation pipeline is OS-aware via plugins; the Android-specific path
consumes the `ota-artifact-validator` and `ota-update-engine-bridge`
submodules for slot-level operations.

### II. Signature Trust & Credential Safety

The artifact-signature verification key is sourced EXCLUSIVELY from
server configuration. A request-supplied verification key is NEVER
trusted (it would defeat signature verification). All credentials,
secrets, API tokens, and signing keys MUST follow the HelixConstitution
§11.4.10 credentials-handling mandate: no tracked-file storage,
runtime-load only from gitignored paths, per-service file separation,
pre-store leak audit (§11.4.10.A), never logged or echoed. The server
auth stack uses OAuth2/JWT with RBAC; every secret-bearing path is
excluded from version control and code-intelligence indexing.

### III. Reuse-First, Decouple-Hard

Every capability is built as an independently reusable submodule brick
with its own tests and documentation, decoupled enough to be consumed
by future projects. Before scaffolding any new module or helper, the
catalogue at `constitution/submodules-catalogue.md` (141+ repos across
vasic-digital and HelixDevelopment orgs) MUST be surveyed; when a
submodule covers >=80% of the functionality it is reused or extended
upstream, never duplicated. Six new OTA-specific submodules are
introduced per this principle: `ota-protocol`, `ota-telemetry-schema`,
`ota-artifact-validator`, `ota-rollout-engine`, `ota-update-engine-bridge`,
and `ota-android-agent`. All new reusable bricks get PUBLIC repos on
GitHub + GitLab under HelixDevelopment / vasic-digital.

### IV. Constitution Inheritance & Governance Compliance

This project is governed by the HelixConstitution submodule at
`constitution/Constitution.md` as the universal base. All clauses
there apply unless explicitly overridden. The project-specific
constitution at `docs/guides/HELIX_OTA_CONSTITUTION.md` extends the
universal base with project clauses (A/B safety, trust boundary,
reuse-first, forward-OS roadmap). The universal Act-No-Bluff
Covenant (§11.4), Mutation-Paired Gates (§1.1), Four-Layer Test
Coverage (§1), and Host-Session Safety (§12) are non-negotiable.
Every governance change propagates through all five carrier files
(CLAUDE.md, AGENTS.md, QWEN.md, GEMINI.md, GEMINI.md) in lockstep
per §11.4.157.

### V. Full-Stack Anti-Bluff Quality Covenant

The bar for shipping is not "tests pass" but "users can use the
feature." Every PASS MUST carry positive captured evidence (recorded
video, sink-side probe, OCR-verified output, runtime signature) per
HelixConstitution §11.4, §11.4.5, §11.4.69, §11.4.107. The project
spans a Go/Gin control plane, a React/TypeScript dashboard, and
Kotlin/KMP device agents — every layer is covered by the full closed
test-type set: unit (Go test, Vitest), integration (pgx/PostgreSQL,
real infra via containers submodule), e2e (Playwright, Go e2e),
full-automation (autonomous, re-runnable, deterministic across N=10
iterations), stress+chaos (sustained load, failure-injection,
categorised recovery), security (authn/authz, injection, secret-leak,
transport), DDoS/load-flood, memory (leak census over soak, peak-RSS
cap), and benchmarking (p50/p95/p99 vs baseline). Each test that
PASSes cites a rock-solid physical evidence artefact; metadata-only /
config-only / absence-of-error PASS is FORBIDDEN. Pair every gate with
a §1.1 meta-test mutation that proves the gate catches its own
negation.

## Technology Stack

The stack is locked by operator decision from the master design:

| Layer | Technology |
|---|---|
| Language/Runtime | Go (control plane, rollout engine, validators); Kotlin/KMP (Android agent); TypeScript (dashboard) |
| HTTP Framework | Gin (gin-gonic) |
| Transport | HTTP/3 (QUIC) with automatic HTTP/2 fallback; Brotli content compression, gzip fallback |
| API Surface | REST primary (/api/v1); gRPC optional/internal only |
| Persistence | PostgreSQL (relational), MinIO/S3 (artifact blobs), Redis (optional caching, only where measured need exists) |
| Observability | OpenTelemetry; Prometheus/Grafana; structured logging |
| Dashboard | React 18, React Router v6, Vite, TypeScript 5.6 |
| Dashboard Testing | Vitest (unit), Playwright (e2e + host-render), pixelmatch (visual regression), axe-core (a11y) |
| Server Dependencies | pgx/v5 (Postgres driver), Prometheus client, quic-go, ota-protocol, ota-artifact-validator, ota-rollout-engine, ota-telemetry-schema, digital.vasic.containers, digital.vasic.http3 |
| Containers | Rootless Podman via vasic-digital/containers submodule |
| Governance | HelixConstitution submodule (`constitution/`) |
| CI/CD | ALL disabled per §11.4.156 — local-only via pre-build gates + git hooks |

---

## Development Workflow & Quality Gates

The project follows the HelixConstitution Parallel Work Unit (PWU)
pipeline (§11.4.58): develop (subagent-driven, isolated worktrees per
§11.4.20/§11.4.70) → merge (via `scripts/commit_all.sh` with flock
lock, never direct `git add`/`git commit`) → rebuild+flash (via
containerized distributed build per §11.4.173 on the designated remote
build host, never bare-host) → validate (risk-ordered per §11.4.132,
most-reopened-first per §11.4.189, with captured evidence per
§11.4.108 runtime-signature on a clean target). Every change crosses
an independent code-review agent (§11.4.125/§11.4.142) and an
eight-angle impact-research pass (§11.4.145) before the pre-build
sweep. Multi-upstream push to GitHub + GitLab + GitFlic + GitVerse per
§2.1; force-push is absolutely forbidden (§11.4.113). Documentation
exports (md → html → pdf → docx) are auto-synced via the docs_chain
engine (§11.4.106).

---

## Governance

The HelixConstitution submodule at `constitution/` is the universal
source of truth. This project-specific constitution extends it; no
override of universal clauses exists. Amendments to this document
require a version bump (semantic: MAJOR for incompatible principle
changes, MINOR for additions, PATCH for clarifications) and a
corresponding update to `docs/guides/HELIX_OTA_CONSTITUTION.md` and
the root AGENTS.md. Every change produces captured evidence of
compliance—no bluff, no guessing language (§11.4.6). All PRs/reviews
verify governance compliance; complexity must be justified per the
plan-template's Complexity Tracking section. The `constitution/AGENTS.md`
and project root AGENTS.md provide runtime development guidance.

**Version**: 1.0.0 | **Ratified**: 2026-06-07 | **Last Amended**: 2026-07-26
