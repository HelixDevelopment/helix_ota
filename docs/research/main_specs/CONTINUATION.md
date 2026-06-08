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

### Staged rollout + more — DONE this session
- Migration 002 real-DB validated (e14942a). **Go engine** wired via the `ota-rollout-engine` brick: `server/internal/rollout` + REST `POST/GET /deployments/{id}/rollout` + `/evaluate` (create→start→advance→complete, halt-on-error-breach); 7 tests green (046ea07).
- End-user rollback reconciled INTO **1.0.1** per operator decision; 1.0.2-rollback superseded→folded (a4c2b3d).
- **G11** repo audit: all 6 ota-* PUBLIC on GitHub+GitLab (dab2f0e). **G12** NFR/load harness real measured percentiles (6547c7f).

### Round 2 deliverables — DONE this session
- **Dashboard** scaffold (G6) — Vite+React+TS, real `tsc`/`vite build` exit 0 (813a00e).
- **6 submodule READMEs** enriched + pushed to each github+gitlab (813a00e).
- **Dependabot ×4 merged + verified**: gin 1.12.0, quic-go 0.60.0 (http3 transport re-verified), actions v6 (33039b8).
- **Rollout pgx StoragePort** — real-DB tested via containers submodule (8190e92).
- **§11.4.65 exports** for 13+2 new docs (242ee2f, a288a4b). **device_tuf.md** + **rollback_ux.md** specs (1.0.1).

### Round 3 deliverables — DONE this session
- **Rollback-history store layer** (AppendRollback/ListRollbacks) memory+pgx, real-DB parity (e4165a6).
- **Recall endpoint** `POST /deployments/{id}/recall` + `GET /rollbacks` (c98dfac) — records rollback_history, validates deployment/target-release, operator/admin; 3 tests.
- **Delta-updates 1.0.3** design (ADR-0005 Option B) + **as-built endpoint reference** (server.go route table) + submodule README §11.4.65 siblings (573779a).
- **Autonomous e2e challenge** `tests/e2e/challenge_operational.sh` — real live-server run, 28 passed/0 failed/1 skip, independently re-verified (bb332a4).

### Round 4 deliverables — DONE this session
- **pgx wired into `main`** (`HELIX_DATABASE_URL`): production uses the pgx repo + rollout StoragePort; **e2e challenge PASS (28/0/1) against a real Postgres-backed server** (1cdad81, docs/qa/20260608-pgx-server-e2e/).
- **migration 004** delta_artifacts real-DB validated; **CI** e2e+loadtest jobs; **README** doc-map §10; **HelixQA bank**; **threat-model** extended (6a3e213, c-threat).

### NEXT wave (still open)
1. **Recall → actual N-1 re-deployment** (deployment-engine path; reconcile with AVB anti-downgrade — operator decision flagged in threat_model §11 item 11) (`NewServiceWithStore` exists; main wires memory). Optionally the recall→actual N-1 re-deployment (deployment-engine job).
2. **Device-side TUF implementation** (per `device_tuf.md`) — ADR-0002 spike to pick gomobile-go-tuf vs Kotlin client.
3. **Spec↔impl alignment** (per `implemented_endpoints.md` §10): decide whether to widen the leaner handlers (audit actor object, telemetry pagination/newest-first, batch group-add) to the fuller spec, or trim the spec.
4. Confirm CODEOWNERS GitHub handle; GitLab-mirror visibility (G11); delta migration 004; HelixQA bank wiring into CI.
3. Confirm CODEOWNERS GitHub handle; make `vasic-digital/containers` + `HelixConstitution` GitLab mirrors public (or document GitHub-canonical) per the G11 audit.
4. Delta-updates 1.0.3 full spec; HelixQA challenge bank entries for the new endpoints.

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
