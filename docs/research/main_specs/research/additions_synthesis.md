# Helix OTA — Synthesis & Reconciliation of `additions/` Input Drafts

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-06-07 |
| Last modified | 2026-06-08 |
| Status | active |
| Status summary | Deep analysis of **all three** operator-supplied inputs in `docs/research/main_specs/additions/` — `initial_research_01.md`, `initial_research_02.md`, and the large `initial_research_03/helix_ota_big/` corpus (26 markdown design docs: SERVER_IMPLEMENTATION, ANDROID_CLIENT_DESIGN, REST_API_SPECIFICATION, VERSION_ROADMAP, ROLLBACK/DELTA/LINUX/WINDOWS/MULTI-OS research, CODE_REVIEW_AND_GAP_ANALYSIS). Extracts every reusable element, maps each to a phase/spec/code destination, and records every contradiction (between inputs, against locked decisions, against the Constitution) with a resolution rule. This document is the traceability bridge between operator input and the canonical specs. Per-input exhaustive inventories live in `additions_analysis/0{1,2,3}_analysis.md`; §8–§12 below consolidate them. |
| Issues | Inputs conflict on wrapped-engine (Mender vs hawkBit), topology (modular vs microservices), router (chi vs Gin), signing (RSA vs ed25519), transport auth (mTLS vs JWT), TUF-now vs deferred, and the staged-rollout-in-MVP question. All resolved in §5 + §10 in favor of the locked decisions / ADRs. |
| Issues summary | No input conclusion is treated as settled where it conflicts with a locked decision or an evidence-based ADR. The locked decisions win; divergent input choices are recorded and routed. |
| Fixed | Rev 2: processed addition #3 (the `helix_ota_big` corpus) + re-processed #01 (incl. its 2026-06-08 update); added the consolidated 3-input gap register (§8), conflict reconciliation (§10), future-phase catalogue (§11), and the new-work-item routing (§9). Rev 1: drafts 01+02 synthesized. |
| Fixed summary | All three additions processed per operator mandate (additions = authoritative input; nothing skipped or assumed). Every requirement is routed to a COVERED spec/code location or to a tracked gap with a destination. |
| Continuation | Implement the routed gaps in §8/§9 (MVP-critical first); write the staged-rollout engine spec (1.0.1) and the dashboard spec; fold the addition-#3 future-phase research into the versioned dirs per §11. Re-run this synthesis whenever a new `additions/` file lands. |

## Table of contents

- [§1. Purpose & method](#1-purpose--method)
- [§2. Source inventory](#2-source-inventory)
- [§3. Points of agreement (high-confidence inputs)](#3-points-of-agreement-high-confidence-inputs)
- [§4. Extracted reusable elements → destination](#4-extracted-reusable-elements--destination)
- [§5. Contradictions & resolutions](#5-contradictions--resolutions)
- [§6. Corrections of factual/guessed claims](#6-corrections-of-factualguessed-claims)
- [§7. Open questions routed to research ADRs](#7-open-questions-routed-to-research-adrs)
- [§8. Consolidated gap register (all three inputs)](#8-consolidated-gap-register-all-three-inputs)
- [§9. New work items routed to specs/phases](#9-new-work-items-routed-to-specsphases)
- [§10. Addition #3 conflict reconciliation (locked side wins)](#10-addition-3-conflict-reconciliation-locked-side-wins)
- [§11. Future-phase research catalogue → versioned dirs](#11-future-phase-research-catalogue--versioned-dirs)
- [§12. Anti-bluff / UNVERIFIED register](#12-anti-bluff--unverified-register)

## §1. Purpose & method

Per operator mandate, every file in `additions/` is authoritative input that MUST be deeply analyzed and used. This document performs that analysis for the two drafts present, applying three lenses:

1. **Extract** — what concrete, reusable material does the draft contribute (APIs, schemas, diagrams, code, sequencing)?
2. **Reconcile** — where do the drafts disagree with each other, with the locked decisions, or with the HelixConstitution?
3. **Route** — where does each accepted element land in the canonical corpus, and which contradictions are deferred to an evidence-based Architecture Decision Record (ADR)?

Governing constraints: the locked decisions (Native A/B + custom Go control plane; foundation-corpus-first; research decides the wrapped engine; pre-authorized public repo creation), the mandated stack (Go + Gin + Brotli + HTTP/3(QUIC)→HTTP/2 fallback, REST primary), and the HelixConstitution (§11.4.6 no-guessing, §11.4.8 research-before-implementation, §11.4.74 catalogue-first reuse, §1 four-layer testing).

## §2. Source inventory

| Draft | Core thesis | Strongest contributions |
|---|---|---|
| `initial_research.md` | Wrap **Mender** as the core; Go server orchestrates it; Android `UpdateEngine` client; per-phase directory tree. | OTA comparison matrix; `/api/v1` endpoint sketch; first-cut PostgreSQL schema (`releases`, `telemetry_events`, `rollback_history`); update sequence + ER + container + staged-rollout Mermaid diagrams; per-phase directory layout; multi-format export plan. |
| `initial_research_02.md` | Wrap **Eclipse hawkBit** + adopt **TUF** for supply-chain security; microservices topology; pluggable OS-adapter architecture. | Android 15 **Virtual A/B + compression** deep dive; OTA `.zip` / `payload.bin` structure; `update_engine.applyPayload` properties; richer DB schema (users, devices, groups, artifacts, deployments, phases, device_deployments, rollouts, audit_logs, update_metrics + indexes); REST + gRPC surface; phased-rollout JSON + Go rollout-engine logic; go-tuf/v2 signing/verify sketch; docker-compose + k8s manifests; Kotlin client + foreground service; OS-adapter interface; 48-week roadmap. |

## §3. Points of agreement (high-confidence inputs)

Both drafts independently converge on these — treated as high-confidence (still validated, never assumed):

- **Device-side = Android A/B via `update_engine`** with automatic boot-failure rollback (matches our locked strategy and the AOSP anti-corruption guarantee). Draft 02 correctly refines this to **Virtual A/B with compression** for Android 15.
- **Server stack = Go**, PostgreSQL for relational state, **MinIO/S3** for artifact blobs.
- **Artifact integrity = SHA-256 + signature verification**, verified server-side on upload and device-side before apply.
- **Staged rollout** with percentage phases (5/10/30/…/100) gated by success/error thresholds, with pause/halt on breach.
- **Telemetry** event stream from devices feeding dashboards.
- **Containerized** deployment; Constitution-governed; multi-format documentation export.

## §4. Extracted reusable elements → destination

| Element (source) | Accepted? | Destination in corpus |
|---|---|---|
| OTA solution comparison matrix (01 §2.1; 02 §1) | Accept as input, expand with evidence | `research/ota_landscape_report.md` + ADR-0001 (engine selection) |
| Android 15 Virtual A/B + `payload.bin`/`.zip` structure, `applyPayload` properties (02 §2) | Accept (factual, verify against AOSP docs) | `1.0.0-mvp/client_android/update_engine_integration.md` |
| `/api/v1` REST endpoints (01 §5.1; 02 §5.1) | Accept, merge, re-base on Gin + REST-primary | `1.0.0-mvp/api/openapi.yaml` + `endpoints.md` |
| gRPC service defs (02 §5.2) | Accept as **optional/internal** only | `1.0.0-mvp/api/internal_grpc.md` (REST is the mandated surface) |
| PostgreSQL schemas (01 §5.2; 02 §4) | Accept 02 as base (richer), merge 01, normalize | `1.0.0-mvp/database/` (migrations + `schema.md`) |
| Phased-rollout JSON + Go rollout-engine loop (02 §6) | Accept as design seed for the rollout engine | `1.0.1-staged-rollout/rollout_engine.md` (engine is OS-agnostic submodule) |
| go-tuf/v2 signing/verify sketch (02 §7.1) | Accept as the trust-model candidate | `research/supply_chain_trust_adr.md` (ADR-0002 Uptane/TUF), `security/` specs |
| Kotlin client + foreground service (02 §10) | Accept as reference; rework to KMP + WorkManager + jitter | `1.0.0-mvp/client_android/` code snippets |
| OS-adapter interface (02 §11.3) | Accept as the universality seam | `00-master/2026-06-07-helix-ota-design.md` (§4 architecture) + future-OS phase dirs |
| docker-compose + k8s manifests (02 §9) | Accept, re-base on the `containers` submodule | `1.0.0-mvp/deployment/` |
| Mermaid diagrams (01 §3.1/§8) | Accept | `1.0.0-mvp/architecture/diagrams/*.mmd` |
| Per-phase directory tree + export plan (01 §7/§12) | Accept, re-root under `docs/research/main_specs/` | `00-master/` + export pipeline spec |

## §5. Contradictions & resolutions

| # | Contradiction | Resolution |
|---|---|---|
| C1 | **Mender (01)** vs **hawkBit (02)** as wrapped core. | Neither is settled. Both enter ADR-0001 with AOSP-native-only as a third option. The evidence-based research report scores them; the locked strategy biases toward *minimal wrapping* (native A/B + custom Go control plane), so a full-platform adopt must clear a high bar. |
| C2 | **Microservices (02)** vs a leaner modular control plane. | MVP favors a **modular monolith** (one deployable, internal package boundaries mirroring the future services) to cut operational cost; the OS-adapter + rollout-engine seams stay extractable. Microservices revisited at scale (ADR-0003 topology). |
| C3 | **Poll cadence**: 6 h (02 service) vs X-min (01). | Locked default **15 min + jitter, configurable** (operator decision). Drafts' constants are non-binding. |
| C4 | **API surface**: REST+gRPC (both) vs mandated **Gin + REST primary, HTTP/3→HTTP/2 fallback, Brotli**. | Mandated stack wins. REST is the compatibility surface; gRPC is optional/internal only. All endpoint sketches re-based onto Gin + the `http3` submodule. |
| C5 | **TUF/Uptane timing**: 02 puts TUF in MVP security service; locked scope defers full Uptane to 1.0.1+. | MVP ships signed-artifact + SHA-256 + AVB; **TUF/Uptane is an ADR-0002 hardening item for 1.0.1+**. Signing interfaces are designed MVP-forward so Uptane drops in without rework. |
| C6 | **Rollback scope**: 02 roadmaps user-rollback/multi-version early; locked scope defers end-user rollback past MVP. | MVP = automatic A/B boot-failure rollback only. End-user/multi-version rollback = 1.0.1+ (matches operator "not for first version"). |
| C7 | **Redis** (02) as session/cache store. | Optional; prefer the `cache` submodule abstraction. Adopt Redis only if the catalogue brick doesn't meet need (§11.4.74). |
| C8 | Coverage target 80% (02) vs 90% (01). | Constitution §1 four-layer coverage + mutation immunity supersedes any single percentage; numeric target set per-component, floor ≥90% for safety-critical paths (signing, apply, rollout-gate). |

## §6. Corrections of factual/guessed claims

- **Guessed submodules** in 01 §6.2 / 02 §13 (`go-common`, `helm-charts`, `vasic-digital/secrets`) **do not exist** under those names. Use the verified catalogue: `auth`, `security`, `database`, `Storage`, `observability`, `eventbus`, `ratelimiter`, `middleware`, `http3`, `mdns`, `recovery`, `Herald`, `config`, `discovery`, `cache`, `docs_chain`/`Document`/`Formatters`, and KMP `Auth-KMP`/`Security-KMP`/`Storage-KMP`/`Config-KMP`. Secrets management → evaluate `security` + container secrets before introducing Vault.
- **hawkBit stats** in 02 §1.1 ("579 stars … last commit June 5 2026") are unverified specifics — must be re-checked live in the research report; do not propagate as fact (§11.4.6).
- **"Mender client is C++"** (02) and **"Android?*"** (01) — Mender's Android support is weak/unofficial; this materially weakens C1's Mender option and must be confirmed in ADR-0001.
- **`file://` payload to `applyPayload`** (02 §10) vs **HTTPS streaming** (02 §2.2) are two different apply paths; the spec must pick and justify (local-verified-file apply is safer for our verify-before-apply requirement).
- Draft 02's self-description as "simulated analysis" / "most comprehensive ever" is marketing, not evidence; stripped from canonical docs per §7.1 no-bluff.

## §7. Open questions routed to research ADRs

- **ADR-0001** Wrapped engine: AOSP-native-only vs hawkBit vs Mender (resolves C1).
- **ADR-0002** Supply-chain trust: plain signing vs TUF vs full Uptane, and MVP-vs-1.0.1 timing (resolves C5).
- **ADR-0003** Server topology: modular monolith vs microservices, and the scale trigger (resolves C2).
- **ADR-0004** Transport: HTTP/3(QUIC)+Brotli rollout, HTTP/2 fallback negotiation, device-network realities (anchors C4).
- **ADR-0005** Delta updates: AOSP block diffs vs custom, and phase placement.

## §8. Consolidated gap register (all three inputs)

Deduped union of every MISSING/PARTIAL requirement surfaced by the three per-input
analyses (`additions_analysis/01_analysis.md`, `02_analysis.md`, `03_analysis.md`).
Severity: **C** = MVP-critical (code-actionable now), **H** = high (next phase),
**M** = medium (catalogue/defer). Each row routes to a destination — nothing is dropped.

| ID | Gap (source) | Sev | Status vs built system | Destination / action |
|---|---|---|---|---|
| G1 | Anti-downgrade invariant on the update-check offer path (03 R16; 01) | C | MOSTLY COVERED — `handleClientUpdate` returns 204 when `current ≥ release` (so a strictly-lower target is never offered for a known, parseable version); release-creation enforces S4 monotonicity. Residual: no regression test pinning the invariant; an empty/unparseable `current` falls through to "offer". | Add regression tests proving downgrades/equal are never offered + behavior on empty/unparseable current; document the invariant in `endpoints.md §12.1`. **Implemented this pass — see §8.1.** |
| G2 | Validation-chain order hash-before-signature is verified+documented (03 R15) | C | UNVERIFIED at spec level — ordering lives in `ota-artifact-validator`; handler maps S2→422 HASH_MISMATCH before S3→422 SIGNATURE_INVALID. | Confirm S2-before-S3 in the validator brick + state it explicitly in `security/signing_verification.md`. |
| G3 | Audit logging (`audit_logs` table + write path) (01 REQ-01-60; 02; 03 R-audit) | H | PARTIAL — table planned in `database/schema.md`; no audit middleware in `server/internal/api`. | Spec an audit middleware + `AppendAudit` repo method; 1.0.1 (compliance). |
| G4 | Telemetry read/aggregation endpoints — per-device history + `/telemetry/overview` (02 R02-5.8; 03 R26) | H | PARTIAL — ingest built (`POST /client/telemetry`, schema-validated); no read side. | Spec read endpoints in `endpoints.md`; dashboard/monitoring phase. |
| G5 | Device-group CRUD + membership endpoints (02 R02-5.7; 03 R20) | H | PARTIAL — `group` accepted in `DeploymentCreate`; group tables in schema; no CRUD routes. | Spec group endpoints (1.0.1, or MVP if grouped deploys needed sooner). |
| G6 | React dashboard (upload→deploy→fleet/telemetry view) (01 REQ-01-78; 02; 03 R23) | H | MISSING — no spec, repo, or code anywhere. Largest single doc gap. | New `dashboard/` spec reusing `UI-Components-React`/`Dashboard-Analytics-React`/`Auth-Context-React`; new public repo. |
| G7 | Full staged-rollout engine: phase schema + SQL + Go engine + lifecycle ops (start/pause/resume/abort/rollback) (02 R02-6.x; 03; 01) | H | PARTIAL — design seed only (master §8, `1.0.1-staged-rollout/README.md`); `ota-rollout-engine` brick exists (halt-wins + deterministic cohorts). | Promote `1.0.1-staged-rollout` outline → full spec + migration `002_*` (deployment_phases, rollouts, rollback_history). Biggest unwritten spec body. |
| G8 | Mandatory-update / deadline semantics (03 R18) | M | MISSING — `mandatory`/`deadline` undefined everywhere. | Define in update-check contract; 1.0.1. |
| G9 | Tamper event + rollout-halt-on-HASH_MISMATCH (`SECURITY_TAMPER_DETECTED`) (03 R27) | M | MISSING. | Add to telemetry-schema + rollout-gate; 1.0.1 security hardening. |
| G10 | CI / governance: GitHub Actions, CODEOWNERS, gosec, Dependabot (01 REQ-01-75) | M | MISSING from repo. | Add CI workflows running all validators (redocly, migrations, kubeconform, go test, gradle :core:test, export render). |
| G11 | NEW `ota-*` submodule repos created PUBLIC on GitHub+GitLab (01 REQ-01-62/47) | M | UNVERIFIED — submodules wired locally; repo existence per-host not confirmed in this pass. | Verify/create per host; mirror to GitFlic/GitVerse (§4). |
| G12 | NFR targets bound + load harness: 10k devices, 99.9% SLA, <100ms (02 R02-8.4/14.2/14.3) | M | UNVERIFIED — asserted, never measured. | Build a load harness; set numbers from measured MVP load (do not assert). |
| G13 | Secrets management wired (container-native + `security`/`config`), Vault rejected (01 REQ-01-55) | M | PARTIAL — path chosen, not wired/verified. | Wire + verify in deployment spec. |
| G14 | mTLS device auth (02; 03) | M | DEFERRED by design — JWT bearer for MVP. | 1.0.1+ hardening; keep signing-forward so it drops in. |

### §8.1 Implemented this pass

- **G1** — added update-check regression tests in `server/internal/api` proving the
  anti-downgrade invariant (a strictly-lower or equal target version is never offered;
  empty/unparseable reported version behavior pinned), and documented the invariant. These
  also raise `handleClientUpdate` branch coverage toward the ≥90% safety-critical floor.

## §9. New work items routed to specs/phases

These are net-new deliverables the inputs require that are NOT yet in the canonical
roadmap as concrete specs. Each is routed; none are MVP blockers except where marked.

1. **Dashboard spec + repo** (G6) — `dashboard/` + new public repo; reuse `*-React` bricks.
2. **Staged-rollout engine full spec** (G7) — `1.0.1-staged-rollout/` promotion + migration `002_*`.
3. **Audit subsystem** (G3) — middleware + `audit_logs` write path; `security/` + `database/`.
4. **Telemetry read/analytics API** (G4) — `endpoints.md` + aggregation queries.
5. **Device-group CRUD** (G5) — `endpoints.md` + repo methods.
6. **CI/governance** (G10) — `.github/workflows/`, CODEOWNERS, gosec, Dependabot.
7. **Load/NFR harness** (G12) — `tests/load/`; bind NFR numbers from measurement.

## §10. Addition #3 conflict reconciliation (locked side wins)

Addition #3 is a **parallel, internally-divergent generation** (its own
`CODE_REVIEW_AND_GAP_ANALYSIS.md` audits its docs against *each other*, not against the
built server). Re-scored against the actual `server/` build and locked decisions:

| # | Addition-#3 position | vs | Resolution |
|---|---|---|---|
| K1 | `chi` router | LOCKED Gin (built) | **Gin** wins. |
| K2 | RSA-4096 / RSA-PSS signing (+ Vault `type=rsa-4096`) | built **ed25519 + SHA-256** / ADR-0002 | **ed25519** canonical. |
| K3 | mTLS device transport | built **JWT bearer** | **JWT** for MVP; mTLS optional hardening. |
| K4 | Module paths `vasic-digital/helix-ota-server`, `dev.helix.ota.*` (self-conflicting) | built `github.com/HelixDevelopment/helix_ota` | **HelixDevelopment/helix_ota**. |
| K5 | Vocabulary `updates`+`rollouts` | canonical `releases`+`deployments` | **releases/deployments**; staged rollouts → 1.0.1. |
| K6 | Full staged-rollout engine in MVP | locked all-targets-only MVP | engine = **1.0.1** design seed. |
| K7 | Coverage 85% | ≥90% safety-critical floor (§1/C8) | **≥90%** floor on safety paths. |
| K8 | Numbering: rollback@1.0.1, delta@1.0.2 | canonical reserves **1.0.1 = staged-rollout** | staged-rollout stays 1.0.1; rollback→**1.0.2**, delta→**1.0.3** (aligns ADR-0005). |

**Addition-#3 "CRITICAL" findings that are NOT real against the build** (artifacts of its
own multi-author generation): endpoint-path drift (C1/C3/C4), enum drift (C5/C6/C8),
`os_type` CHECK (C7), module path (C10), UUID-vs-prefixed-IDs (C11), registration body
(C2). The built server already settled canonical paths (`POST /devices/register`,
`GET /client/update`, `POST /artifacts/upload`), single-source enums from
`ota-protocol`/`ota-telemetry-schema`, opaque string IDs, and the canonical module path.

## §11. Future-phase research catalogue → versioned dirs

Addition #3 ships substantial future-phase research (markdown). Routed to the versioned
phase dirs; numbering reconciled per K8 (staged-rollout owns 1.0.1):

| Addition-#3 doc | Destination dir | Notes |
|---|---|---|
| `ROLLBACK_DESIGN.md` (server-triggered + user-initiated + multi-version) | `1.0.2-rollback/` | Auto boot-failure rollback is already MVP; this is the *operator/user-initiated* superset. |
| `DELTA_UPDATES_DESIGN.md` | `1.0.3-delta-updates/` | Aligns ADR-0005 (AOSP block diffs vs custom). |
| `LINUX_OTA_RESEARCH.md` (RAUC/SWUpdate/Mender) | `1.X-linux/` | Per-OS adapter behind the universality seam. |
| `WINDOWS_OTA_RESEARCH.md` | `1.X-windows/` | Per-OS adapter. |
| `MULTI_OS_UNIVERSAL_DESIGN.md` | `1.X-other-os/` (or `2.0.0-multi-os-universal/`) | The universal OS-adapter platform. |
| `OTA_SYSTEMS_RESEARCH.md` | merge into `research/ota_landscape_report.md` | Dedupe against existing landscape report. |
| 8 diagrams | `00-master/diagrams/` | Dedupe against existing diagram set. |

## §12. Anti-bluff / UNVERIFIED register

Carried forward from the per-input analyses; these MUST NOT be propagated as fact
(Constitution §11.4.6 no-guessing, §7.1 no-bluff):

- hawkBit/Mender repo stats and "Mender client is C++/weak Android" — confirm live in ADR-0001.
- Invented submodules (`go-common`, `helm-charts`, `vasic-digital/secrets`, `vasic-digital/auth`) do **not** exist under those names — use the verified catalogue.
- `ota-artifact-validator` exact S2-before-S3 ordering — verify in the brick (G2).
- AOSP `payload_properties` optional-header set on Android 15 / RK3588 — UNVERIFIED hardware gate.
- `Storage` brick Range-GET / pre-signed-URL behavior — UNVERIFIED until inspected.
- Addition-#3 delta-update benchmarks and NFR numbers (10k devices, 99.9%, <100ms) — UNVERIFIED, never measured.
- All HelixConstitution §11.4.x clause numbers cited corpus-wide remain UNVERIFIED except the six confirmed in `tests/test_strategy.md` §13.
