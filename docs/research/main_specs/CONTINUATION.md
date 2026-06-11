# Helix OTA — Continuation / Resume Handoff

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-06-07 |
| Last modified | 2026-06-10T16:10:00Z |
| Status | active — resume with "continue" |
| Status summary | Single source of truth for resuming work. Captures exactly what is DONE (verified), the git state, and the prioritized NEXT steps. Everything below is committed to `main` and pushed to all 4 upstreams (GitHub, GitLab, GitFlic, GitVerse). |

## ⤴ CURRENT STATE (2026-06-10 overnight) — read `docs/RESUMPTION.md` (Rev 9) FIRST

The freshest live-state anchors are in **`docs/RESUMPTION.md`** (the §11.4.131 canonical
entry). As of HEAD **`0d27438`** the build is **GREEN across every achievable tier**, and
during the 2026-06-10 overnight autonomous session (operator away-from-keyboard) **every
achievable software tier was RE-RUN fresh on this exact HEAD and re-confirmed GREEN** (§11.4.132
risk-ordered re-validation) — full proof in **`docs/qa/STABILITY_REPORT.md`** Rev 2: Go (all tiers
+ `-race -count=1`), pgx integration (store 88.7% / rollout 71.8% with `-tags integration`),
inheritance gate, meta-test §1.1, dashboard 58/58 + typecheck, HelixQA LIVE bank 10/0 + dry-run
10/0 + self-test §1.1, podman full-lifecycle + fleet 5/5 + recall-recovery 7/7, QEMU firmware
tier, all Go submodule cores. Android bricks (`ota-android-agent` `1061015`, `ota-update-engine-bridge`
`8bb8d2f`) are byte-identical to their prior proof (pins unchanged, trees clean at pin) — NOT
re-run, to respect the §12.6 60%-memory ceiling alongside podman (honest §11.4.6/§11.4.101 call,
not a re-run claim). **No release tag** — §11.4.40 needs the on-device RK3588 tier (hardware-BLOCKED,
§11.4.112), so a tag would be a bluff. BLOCKED (hardware): T2 Cuttlefish / GSI-A-B real-apply /
T3 RK3588. DEFERRED (zero-risk): fabric scheduler P3 (unwired §11.4.124), HelixQA in-tree compile
(`../containers` layout). **UPDATE 2026-06-11: an operator-directed 6-stream parallel-hardening
round landed on top (HEAD `dead6ef`) — real coverage gains (api 87.7% / fabric 96.7% / transport
94.9%, dashboard 58→93), §11.4.65 PDF backfill (66 siblings — weasyprint IS present, the prior
"no weasyprint" deferral was stale), and an extended security suite (26/0/0 ×3); two real defects
in the agents' output were caught + fixed during conductor-verification. A SECOND parallel
round then landed (HEAD `a58f7f8`): the integration-port collision is now FIXED (flock-serialized
shared Postgres), deviceemu 76→94%, ota-protocol + ota-artifact-validator submodules → 100%
(pushed `eda12b7`/`087fa08`), a rollout halt-on-breach e2e (47/0/0), and the extended security
suite wired into the HelixQA bank (LIVE 11/0/0). Remaining follow-ups are now DIMINISHING-RETURNS
(other-submodule coverage, more e2e scenarios) — see `docs/RESUMPTION.md` §1.** Everything below
this box is prior-wave history.

## How to resume

Type **`continue`** in a new session. Read **`docs/RESUMPTION.md` Rev 8** + this file + the memory index first. All work is on `main` (latest commit pushed to all upstreams). Working branch history: foundation → research → MVP specs → submodule impls → server, each merged to `main`.

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

### Round 5 deliverables — DONE this session
- **Recall = forward-fix** (operator decision honor-AVB, 4e35c3e): `handleRecall` supersedes the current deployment + creates a NEW active deployment of the target release; the update-check anti-downgrade invariant means AVB is honored by construction. New store `UpdateDeployment` (memory+pgx, real-DB parity). Decision recorded in `rollback_ux.md` Rev 2 + `threat_model.md` §11.11 RESOLVED (f5ec504).

### Round 6 deliverables — DONE this session
- **Delta-artifact store + API** (66464d7): `delta_artifacts` (base≠target CHECK + UNIQUE pair) on memory+pgx; `POST/GET /deltas` register+lookup; real-DB parity.
- **device-TUF client-decision memo** (recommend gomobile-go-tuf/v2, ADR-0002 §4.3) + sibling.
- **Additive WIDENs — full set landed** (operator-approved): audit `?since/?until` filters + telemetry `failure_rate`/`by_state` (028e656, memory+pgx real-DB parity) + group `member_count` (4cb86d7). `spec_impl_alignment.md` Rev 3: rows 2+5+6-partial landed; as-built doc re-synced (recall now WIRED, /deltas + recall + rollbacks documented).

### Round 7 — breaking WIDENs landed (operator ruling: WIDEN-impl)
- **Group** (a91271b + 02ad2d0): `id`→`group_id`; batch member-add → 200 `{added, already_member, not_found}`. Server+tests+e2e(36/0/1)+dashboard+bank+docs.
- **Audit** (fbaefbe): `actor` → object `{user_id, subject}`. Server+test+dashboard+docs.
- **Telemetry per-device** (2a48ab5): `events`→`items`, newest-first, `?limit`/`?cursor`+`next_cursor`. Server+tests+dashboard+docs.
- **GET /members** (3b4b1d8 + e2e): `device_ids[]` → `items[]` of `{device_id, added_at}`; store gained `device_group_members.added_at` + `ListGroupMembersDetailed` (memory+pgx, real-DB parity); e2e 39/0/1.
- Each: full default suite green, all wire consumers updated in lockstep, `spec_impl_alignment.md` rows 6/8/1/4-structural + GET-members RESOLVED.

**WIDEN ruling status: COMPLETE** except two legitimately-parked items: row-4 richer telemetry fields (`duration_ms`/`bytes_transferred`) **blocked on UNVERIFIED ingest** (event source must carry them first) + the per-device telemetry filters; group/members list pagination (row 7) **deferred** (groups bounded — memo's own recommendation). Both need either ingest work or an operator nudge; not autonomously actionable now. _(UPDATE 2026-06-10: the per-device telemetry **filters** + group/members list **pagination** are now DONE — shipped `50ef5c6`; see the "Session 2026-06-10 update" below. Only row-4 richer numeric fields remain parked on ingest.)_

### Round 8 — "all of it" wave (operator: do all frontiers)
- **Server-side delta-selection** (9179d0a): ota-protocol `UpdateAvailable.Delta` (brick 7d18edc, pushed; server builds via dev `replace`); store `ReleaseByVersion` (memory+pgx, real-DB parity); update-check resolves current→base artifact→`FindDelta` → offers delta + full fallback; `TestClientUpdateOffersDelta`.
- **Device-side delta-apply** (ota-android-agent, pushed + gitlink): pure `DeltaApplyDecision` (USE_DELTA vs FULL_PAYLOAD), 11 tests, `:core` 47/0 real Gradle.
- **Operator cleanup** (DONE): `vasic-digital/containers` + `HelixConstitution` GitLab mirrors flipped **private→public** (re-read-verified); CODEOWNERS `@milos85vasic` confirmed.
- **OpenAPI** synced to all widened/new endpoints + `UpdateAvailable.delta`; **§11.4.65** corpus export coverage (46 html+pdf pairs).

### Round 9 — hardware-free QA wave (operator: "all of it", no Orange Pi tonight)
- **Stress + chaos** (§11.4.85, 5363511): race-clean in-process suite — 200 concurrent creates (0 err, p99 6.4ms), 2400 sustained reads (0 err), 60-device membership contention → no lost updates, chaos fault→500→recover→200. Evidence `docs/qa/20260608-stress-chaos/`.
- **Full-pipeline e2e + security** (e7e1a1c): `tests/e2e/pipeline_signed.sh` (real ed25519-signed artifact → upload→release→deploy→rollout→delta-bearing update; bogus-sig→422) 32/0/0 — **closes the artifact-pipeline SKIP**; `tests/security/security_probes.sh` (authn/authz/ownership/injection/trust-boundary) 37/0/0. Both re-run in-tree.
- **Performance/NFR + scaling** (7b75212): real memory-vs-pgx concurrency sweep — in-mem 20.5k RPS @ c=128 p99 30.6ms; pgx ~6.2k plateau (2-CPU container). `docs/research/main_specs/1.0.0-mvp/nfr/performance_baseline.md`.
- **Dashboard build-out + Playwright** (just landed): functional Fleet/Deployments+Recall/Groups/Audit screens on the real API; tsc+build exit 0; **Playwright 5/5** vs a live server.
- **Coverage ledger** (§11.4.25) + **HelixQA bank** now 8 challenges (incl. signed-pipeline + security).
- **Benchmarking** (§11.4.27, bbc97e4): Go `testing.B` suite — healthz 2.5µs, group-create 8.5µs, update-check 4.9µs, FindDelta 160ns/0-alloc, etc. `docs/qa/20260608-benchmarks/`.
- **DDoS/flood probe** (§11.4.27, 6d29fb8): 5,952-req burst served, responsive post-flood; surfaced the honest "no rate-limiter" finding.
- **Rate-limit FEATURE** (1b97fc7): implemented the finding's fix — in-flight cap middleware (`HELIX_MAX_INFLIGHT`, default-off) sheds 429 RATE_LIMITED; proven cap=1/300-concurrent → 244 shed / 56 served / recovers. **§11.4.27 test-type matrix now complete** for the operational surface (unit/integration/e2e/security/ddos/scaling/chaos/stress/performance/benchmarking/ui/Challenges). Full rebuild+validate sweep green (`docs/qa/20260608-full-rebuild/`).

## Session 2026-06-10 update (DONE, on `main`, pushed all 4 upstreams)

- **Per-device telemetry filters + group/members pagination** (`50ef5c6`, `feat(api): per-device telemetry filters + group/members pagination`): closes the two previously-PARKED WIDEN bits — per-device telemetry now accepts the filter params, and the group/members list is paginated. **OpenAPI synced + redocly-clean.** These move out of the "parked WIDEN bits" list (no longer deferred).
- **Dashboard client lockstep** (`b0b8ee2`, `chore(dashboard): sync API client to new pagination/filter params`): the dashboard API client updated in lockstep with the new pagination/filter params, so every wire consumer stays consistent (§11.4.92 cross-feature consistency). **HEAD is now `b0b8ee2`.**
- **Emulator-driven device testing initiative started** — tiered plan captured as FACT in `docs/design/EMULATED_DEVICE_TESTING.md`. **Tier-1 (podman `ota-device-emulator` over real `ota-protocol`) is IN PROGRESS**; Tier-2 (Cuttlefish A/B, Linux+nested-KVM-gated) and Tier-3 (real RK3588, hardware-gated) are designed, host/hardware-gated (NOT structurally impossible per §11.4.112). Extends the `containers` submodule (`pkg/boot`/`compose`/`health` + `pkg/emulator` AVD-x86_64 + `pkg/vm` qemu-aarch64) per §11.4.76.
- **Canonical §11.4.131 session-resumption file created** at `docs/RESUMPTION.md` — the fixed out-of-the-box entry point for any fresh session (SHORT + FULL variants, read-first handoff pointers, live-state anchors, PHASE/NEXT/terminal-goal, binding constraints). Point a new session at that one file.

### NEXT wave (still open — all hardware/ingest-gated)
1. **Device-side TUF** (gomobile-go-tuf/v2 per the decision memo) — gated on an arm64 `.so`-size/JNI measurement on real RK3588 hardware.
2. **Device payload-apply integration** — wire `DeltaApplyDecision` into the on-device apply path (`:android`/update_engine) — needs a real device to validate end-to-end.
3. **Emulator-driven device testing** — tiered plan now in flight (`docs/design/EMULATED_DEVICE_TESTING.md`): **Tier-1 IN PROGRESS** (podman `ota-device-emulator` speaking real `ota-protocol` to the control plane — register→update-check→telemetry→delta→rollout→recall, runnable on this macOS host now); Tier-2 Cuttlefish A/B (Linux+nested-KVM-gated); Tier-3 real RK3588 (hardware-gated).
4. **Parked WIDEN bits**: row-4 richer telemetry fields (`duration_ms`/`bytes_transferred`) — still blocked on UNVERIFIED ingest (event source must carry them first). **Telemetry per-device filters + group/members list pagination are now DONE** (shipped this session, see below) — no longer parked.

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
