# Helix OTA — Continuation / Resume Handoff

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-06-07 |
| Last modified | 2026-06-08 |
| Status | active — resume with "continue" |
| Status summary | Single source of truth for resuming work. Captures exactly what is DONE (verified), the git state, and the prioritized NEXT steps. Everything below is committed to `main` and pushed to all 4 upstreams (GitHub, GitLab, GitFlic, GitVerse). |

## How to resume

Type **`continue`** in a new session. Read this file + the memory index first. All work is on `main` (latest commit pushed to all upstreams). Working branch history: foundation → research → MVP specs → submodule impls → server, each merged to `main`.

## Locked decisions (do not re-litigate)

- Stack: **Go + Gin + Brotli + HTTP/3(QUIC)→HTTP/2 fallback, REST primary**; reuse `vasic-digital/http3`.
- Strategy: **native Android A/B (`update_engine` + AVB/dm-verity + auto-rollback) + custom Go control plane**; wrap OSS only where it adds value (hawkBit gated, ADR-0001).
- Trust: signing + SHA-256 + AVB for MVP; TUF server-side then device-side in 1.0.1 (ADR-0002).
- Topology: modular monolith for MVP (ADR-0003).
- New submodule repos auto-created PUBLIC on GitHub + GitLab (pre-authorized).
- `docs/research/main_specs/additions/` files are authoritative input — always analyze + fold in.
- Commit + push to ALL upstreams regularly. Merge to `main` when a milestone is done.

## Session 2026-06-08 update (DONE, on `main`, pushed all 4 upstreams)

- **Constitution submodule relocated** `HelixConstitution/` → **`constitution/`** (repo/URL unchanged) via `git mv`; fixed the only filesystem path refs (`tests/test_strategy.md`, `additions/initial_research_01.md`) incl. their pre-existing off-by-one link depth. Prose "HelixConstitution" clause citations (the repo NAME) intentionally left.
- **Inheritance wired**: parent `CLAUDE.md`, `AGENTS.md`, `docs/guides/HELIX_OTA_CONSTITUTION.md` inherit from the submodule; `tests/inheritance_gate.sh` (5 invariants, matches the exact §11.4 forensic-anchor heading) + `tests/test_constitution_inheritance.sh` (gate green clean + **§1.1 paired-mutation proven** via `constitution/meta_test_inheritance.sh` + recursive submodule pointer check) + `tests/pre_build_verification.sh` + installed pre-commit hook. All 6 owned `ota-*` submodules carry CLAUDE.md+AGENTS.md inheritance pointers (pushed github+gitlab; gitlinks bumped to `v0.1.0-1-g…`). Commits ee5dc7d, e11a221.
- **All 3 additions processed** (operator mandate — nothing skipped): per-input exhaustive inventories in `research/additions_analysis/0{1,2,3}_analysis.md`; `research/additions_synthesis.md` → **Rev 2** with consolidated 14-gap register (§8), new-work routing (§9), addition-#3 conflict reconciliation (§10, locks Gin/ed25519/JWT/releases+deployments/≥90%), future-phase catalogue (§11), UNVERIFIED register (§12). MVP-critical **G1 (anti-downgrade invariant)** implemented as regression tests; `handleClientUpdate` 79.3%→86.2%. Commit 629b4eb.
- **Constitution path is now `constitution/`** — update any new references accordingly.

### Server hardening — DONE this session (handoff NEXT #1, real evidence)
- **Coverage**: upload handler 71.2%→**96.6%**, client-update 79.3%→**93.1%** (≥90% safety floor). Commit 64c3bfd.
- **pgx PostgreSQL Repository** (`server/internal/store/postgres.go`): full `store.Repository` impl; shared `contract_test.go` proves parity with memory; **integration test boots real Postgres via the containers submodule on podman** (`go test -tags integration ./internal/store/`), evidence in `docs/qa/20260608-pgx-postgres-integration/`. Surfaced+fixed a real idempotency overwrite bug. Commit 96fdecb.
- **Brotli→gzip→identity** compression middleware (`server/internal/api/compression.go`). Commit b26f30e.
- **HTTP/3 (QUIC)+HTTP/2 fallback** via the new `submodules/http3` (`digital.vasic.http3`) — `server/internal/transport/`; wired into `cmd/ota-server` (TLS via HELIX_TLS_CERT/KEY → HTTPS port 8443; plain HTTP otherwise). Real h3+h2 client test; evidence in `docs/qa/20260608-http3-h2-brotli-transport/`. Commit 1469edf.

### Parallel-wave deliverables — DONE this session (commit 8efb6b8)
6 subagents closed additions gaps: **G2** validator hash-before-signature VERIFIED (FACT, file:line); **G7** staged-rollout engine spec + migration_002 design; **G6** dashboard design; **G3/G4/G5** operational endpoints spec (audit/telemetry-reads/group CRUD + proposed repo methods); **G10** CI (`.github/workflows/ci.yml` + CODEOWNERS + dependabot); **§11** future-phase folding (1.0.2-rollback, 1.0.3-delta-updates created; 1.X-linux/windows/other-os extended).

### Operational endpoints — DONE this session (G3/G4/G5, real-Postgres-verified, memory+pgx parity)
- **G3 audit** (commit 9a703bf): `auditMiddleware` records successful mutating actions (reads/failures skipped, ids never leak into the action) + `GET /audit` (admin) + `audit_logs` table.
- **G4 telemetry reads** (commit eadaa7f): `GET /devices/{id}/telemetry` (device reads only its own) + `GET /telemetry/overview` (fleet counts).
- **G5 device-group CRUD** (commit e3e1307): full `/groups` CRUD + membership; writes operator/admin, delete admin-only.
All three added `store.Repository` methods on memory + pgx, extended the shared contract, and pass the pgx integration test on real Postgres.

### NEXT wave (still open)
1. **Operator decision applied**: end-user rollback stays in **1.0.1** (with staged-rollout). Docs reconciliation (synthesis §10 K8 + 1.0.1/1.0.2 dir renumber) still pending — the agent wave for it was rate-limited; redo.
2. **Staged-rollout engine** (per `1.0.1-staged-rollout/`): migration `002_*` (deployment_phases/rollouts/rollback_history) is DONE + real-DB validated (commit e14942a, evidence `docs/qa/20260608-migration-002/`). REMAINING: the Go engine — a `StoragePort` adapter over the `ota-rollout-engine` brick + rollout REST endpoints (start/pause/resume/abort/rollback). End-user rollback included per operator decision.
3. **Dashboard repo** scaffold (G6); **G12** NFR/load harness; **G11** verify/create ota-* public repos; confirm CODEOWNERS GitHub handle. (These 5-6-agent wave items were rate-limited mid-session — redo when limits recover.)

### Carried-forward gaps register
See `additions_synthesis.md` §8/§9 (14 gaps; most now specced — implementation pending). Numbering decision: 1.0.1 = staged-rollout; rollback→1.0.2, delta→1.0.3.

## DONE (verified, on `main`, pushed)

1. **Spec corpus** (`docs/research/main_specs/`): master design + ADRs index; research = scored landscape report + 12 stack notes + 5 ADRs (ADR-0001..0005) + additions synthesis; foundation = glossary, STRIDE threat model, submodule reuse map, doc standards, requirements traceability, export pipeline; **1.0.0-mvp** full specs (api+OpenAPI, database+migrations, security, server, client_android +snippets, deployment, tests, VALIDATION_EVIDENCE); 6 Mermaid diagrams; future-phase outlines (1.0.1, 1.X-linux/windows/other-os); corpus README.
2. **Validation evidence (real)**: OpenAPI redocly-valid; **migrations applied to live PostgreSQL** (12 tables up, clean down); k8s kubeconform 5/5; compose YAML valid; all 6 diagrams render (mmdc); corpus exports = 50 HTML + 50 DOCX + SVG/PNG (`scripts/export_docs.sh`). PDF needs a LaTeX engine (not installed).
3. **6 reusable submodules** (created on GitHub `HelixDevelopment/` + GitLab `helixdevelopment1/`, scaffolded, wired under `submodules/`, **implemented v0.1.0, tagged, pushed both hosts**):
   - `ota-protocol` (Go, 99.3%), `ota-telemetry-schema` (Go, 98.9%), `ota-artifact-validator` (Go, 97.8%, ed25519+sha256), `ota-rollout-engine` (Go, 94.9%, halt-wins+deterministic cohorts)
   - `ota-update-engine-bridge` (Kotlin; :core 27 tests; :android builds an AAR)
   - `ota-android-agent` (Kotlin; :core 36 tests; :android build pending KMP/AGP-on-Gradle-9.5 alignment)
4. **Submodules wired**: HelixConstitution + containers at root; 6 ota-* under `submodules/`.
5. **Control-plane server** (`server/`, Go module `github.com/HelixDevelopment/helix_ota/server`): Gin modular monolith wiring the 4 Go modules; full /api/v1 MVP endpoints (auth, devices, artifacts upload+validate S1–S6, releases, deployments all-targets, client/update 204|200, telemetry, health); **66 httptest integration tests pass**; gofmt/vet/build clean.

## NEXT (prioritized)

1. **Server hardening to spec**: raise `internal/api` coverage to ≥90% on `handleUploadArtifact` + `handleClientUpdate` (error-path branches); add a **pgx** Repository implementation behind the existing `store.Repository` interface + testcontainers-go integration tests against real Postgres; wire `vasic-digital/http3` for HTTP/3→HTTP/2 + Brotli.
2. **ota-android-agent `:android` build**: switch its `:android` module from Kotlin MPP to plain `com.android.library` + `org.jetbrains.kotlin.android` (like the bridge, which builds), OR pin Gradle 8.x + AGP 8.7.x via a wrapper; then produce the AAR.
3. **React dashboard** (`dashboard/`): reuse `UI-Components-React`, `Dashboard-Analytics-React`, `Auth-Context-React`; secure login, upload, deploy, fleet health.
4. **1.0.1 staged-rollout**: promote the outline to full specs + implement rollout API (phases, pause/resume/abort), device-side TUF, end-user rollback; migration `002_*` (deployment_phases, rollouts, rollback_history, TUF metadata).
5. **AOSP integration**: build the agent + bridge as a system/priv-app in the Orange Pi 5 Max AOSP tree; on-device e2e (download→verify→A/B apply→reboot→post-boot; corrupt-slot→auto-rollback).
6. **Containerize** the dev stack via the `containers` submodule; CI running all validators (redocly, Postgres migrations, kubeconform, go test, gradle :core:test, export render).
7. **Per-submodule**: full README/docs/manuals + GitFlic/GitVerse mirrors + tag mirroring (§4).

## Toolchain facts (this host)

Go 1.26.2; Android SDK (platforms 31–36.1, build-tools, NDK); Gradle 9.5 + Kotlin 2.3.20; **AGP 8.5.2 + plain kotlin.android works on Gradle 9.5; Kotlin MPP does NOT** (needs Gradle 8.x). pandoc/mmdc present; no LaTeX/drawio/plantuml/graphviz/docker. Live PostgreSQL server available. `gh`+`glab` authed. API rate limits are transient — keep waves ≤6 agents, don't overlap workflows.
