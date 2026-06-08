# Helix OTA — Exhaustive Requirement Analysis of `additions/initial_research_02.md`

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-08 |
| Last modified | 2026-06-08 |
| Status | active |
| Status summary | Line-by-line requirement inventory of the operator-supplied draft `additions/initial_research_02.md` (~2044 lines), mapping every extractable requirement to the current canonical corpus (`00-master/`, `1.0.0-mvp/`, `1.0.1-staged-rollout/`, `1.X-*`, `research/adr/`, `research/stacks/`) and the built Go server (`server/`). Each requirement is rated COVERED / PARTIAL / MISSING with a citation or gap, checked for conflict with the locked decisions/ADRs, and given a folding action. This is the granular successor to `research/additions_synthesis.md` (Rev 1, high-level) for draft 02 specifically. |
| Issues | HelixConstitution clause numbers (§11.4.x, §1, §7.1) are carried from corpus convention and are UNVERIFIED against the authoritative Constitution text (the Constitution file is not present in this repository). Several draft-02 factual specifics (hawkBit GitHub stats, "Mender client is C++", Android-15 optional `payload_properties` header set) remain UNVERIFIED and are flagged where they appear. Exact public surfaces of catalogue submodules (`auth`, `security`, `Storage`, `database`, `http3`, `ratelimiter`, `middleware`) are UNVERIFIED. |
| Fixed | Initial granular analysis of draft 02. |
| Fixed summary | All 15 numbered sections of draft 02 inventoried; coverage cross-referenced against corpus + built server code. |
| Continuation | Re-run if `initial_research_02.md` is revised. Feed the MISSING/PARTIAL items flagged below into the relevant phase specs (1.0.1 rollout, 1.X future-OS, ADR-0001 hawkBit un-gating). Confirm catalogue-brick public surfaces to clear the UNVERIFIED reuse claims. |
| Owner | Helix OTA spec-analysis |
| Related | [`../additions_synthesis.md`](../additions_synthesis.md); [`../../00-master/2026-06-07-helix-ota-design.md`](../../00-master/2026-06-07-helix-ota-design.md); [`../adr/adr-0001-wrapped-engine.md`](../adr/adr-0001-wrapped-engine.md) … `adr-0005`; [`../../1.0.0-mvp/`](../../1.0.0-mvp/) |

## Table of contents

1. [purpose_and_method](#1-purpose_and_method)
2. [coverage_summary](#2-coverage_summary)
3. [conflicts_with_locked_decisions](#3-conflicts_with_locked_decisions)
4. [requirement_inventory](#4-requirement_inventory)
   - [4.1 research_existing_systems](#41-research_existing_systems-draft-1)
   - [4.2 android15_ota_mechanism](#42-android15_ota_mechanism-draft-2)
   - [4.3 architecture](#43-architecture-draft-3)
   - [4.4 database_schema](#44-database_schema-draft-4)
   - [4.5 api_specifications](#45-api_specifications-draft-5)
   - [4.6 phased_rollout](#46-phased_rollout-draft-6)
   - [4.7 security_model](#47-security_model-draft-7)
   - [4.8 testing_strategy](#48-testing_strategy-draft-8)
   - [4.9 infrastructure_deployment](#49-infrastructure_deployment-draft-9)
   - [4.10 android_client](#410-android_client-draft-10)
   - [4.11 future_os_support](#411-future_os_support-draft-11)
   - [4.12 roadmap](#412-roadmap-draft-12)
   - [4.13 reusable_components](#413-reusable_components-draft-13)
   - [4.14 success_factors](#414-success_factors-draft-14)
5. [unverified_register](#5-unverified_register)
6. [sources](#6-sources)

---

## 1. purpose_and_method

Per locked decision D5 (`additions/` is authoritative input to be deeply analyzed and folded in),
this document inventories every requirement in `additions/initial_research_02.md` and maps it to
the current state of the system. Three judgements are recorded per requirement:

- **Coverage** — COVERED (a corpus file or built code already realizes it, cited), PARTIAL (some
  but not all of it is realized; gap named), or MISSING (no corpus/code realization).
- **Conflict** — whether it contradicts a locked decision or an ADR; resolution recommended as
  *defer to ADR* / *reject* / *accept-with-modification* / *none*.
- **Folding action** — where the accepted element lands.

Evidence rule (§11.4.6 no-guessing / §7.1 anti-bluff, UNVERIFIED clause numbers): every COVERED
rating cites a real file; unconfirmed claims are marked UNVERIFIED, never invented. Draft 02's
self-description ("simulated analysis", "most comprehensive ever created") is marketing and is
stripped, not folded (`additions_synthesis.md` §6).

Key finding up front: **the corpus is already mature and has folded the large majority of draft 02.**
The master design, the five ADRs, the full `1.0.0-mvp/` spec set, and a working Go server
(`server/internal/api`, `server/internal/store`) already realize draft 02's MVP-relevant content,
*re-based onto the locked stack*. The residual gaps are concentrated in the **post-MVP** material
(staged-rollout engine internals, TUF device client, future-OS adapters) which is correctly
**deferred** (outlined, not specified) — plus a handful of draft-02 items that were deliberately
**rejected** as conflicting with locked decisions (microservices, gRPC-primary, hawkBit-committed,
TUF-in-MVP).

## 2. coverage_summary

| Draft 02 section | Dominant rating | Notes |
|---|---|---|
| §1 research (hawkBit/Mender/TUF/RAUC/SWUpdate) | COVERED | Expanded into `research/stacks/*` + ADR-0001/0002; draft's verdicts treated as hypotheses only. |
| §2 Android 15 Virtual A/B + payload.bin | COVERED | `research/stacks/android15-virtual-ab.md`, `aosp-update-engine.md`, `1.0.0-mvp/client_android/update_engine_integration.md`. |
| §3 architecture (microservices) | PARTIAL / CONFLICT | Architecture COVERED as **modular monolith** (ADR-0003); microservices topology REJECTED for MVP. |
| §4 database schema | COVERED | `1.0.0-mvp/database/schema.md` + executed migration; richer-base normalization done. |
| §5 REST + gRPC API | PARTIAL / CONFLICT | REST COVERED + built; gRPC demoted to optional/internal (out of scope, ADR-0004). |
| §6 phased rollout | PARTIAL (deferred) | Outlined in `1.0.1-staged-rollout/README.md`; full engine spec + SQL not yet written. |
| §7 security (TUF in MVP) | PARTIAL / CONFLICT | MVP = signing+SHA-256+AVB (COVERED, built validator); TUF-in-MVP REJECTED, deferred to 1.0.1 (ADR-0002). |
| §8 testing | COVERED | Replaced by four-layer + mutation model (master §13); draft's 80% floor superseded. |
| §9 infra (compose + k8s) | COVERED | `1.0.0-mvp/deployment/overview.md` + compose/k8s manifests, re-based on `containers`. |
| §10 Android client (Kotlin + service) | COVERED | `1.0.0-mvp/client_android/integration_guide.md` (KMP + WorkManager); reference Kotlin reworked. |
| §11 future OS (Linux/Windows/universal) | PARTIAL (deferred) | OS-adapter seam COVERED in master §4; per-OS specs are research outlines (`1.X-*`). |
| §12 48-week roadmap | PARTIAL | Re-expressed as phase dirs; week-by-week schedule not adopted as canonical. |
| §13 reusable components | PARTIAL / CORRECTED | Real catalogue used (`submodule_reuse_map.md`); draft's guessed submodule names corrected. |
| §14 success factors (SLAs/quality gates) | PARTIAL | Captured as guarantees (master §1); numeric SLAs (99.9%, <100ms, 10k devices) UNVERIFIED/not bound. |

## 3. conflicts_with_locked_decisions

Five draft-02 positions conflict with locked decisions/ADRs. All are already resolved in the
corpus; recorded here for traceability with the recommended resolution.

| # | Draft-02 position (source) | Conflicts with | Recommended resolution | Corpus disposition |
|---|---|---|---|---|
| K1 | **Microservices** topology with 7 services + API gateway (§3.2, L209–296) | D6/locked modular-monolith-for-MVP; ADR-0003 | **defer to ADR-0003** (reject for MVP; seams stay extractable) | ADR-0003 decides modular monolith; `server/` is one binary. RESOLVED. |
| K2 | **gRPC service** as a co-equal/primary device surface (§5.2, L557–624) | D6 REST-primary; ADR-0004 §4 (C4) | **accept-with-modification**: gRPC optional/internal only | `endpoints.md` §1 declares gRPC out of scope; not built. RESOLVED. |
| K3 | **TUF in the MVP security service** (§3.2.7, §7.1, L287–294, L758–833) | Locked scope (TUF→1.0.1+); ADR-0002 (C5) | **defer to ADR-0002** (MVP = signing+SHA-256+AVB) | ADR-0002 defers device-side TUF; MVP validator built. RESOLVED. |
| K4 | **hawkBit committed** as the deployment back end (§1.1, §3.2.4, L24–44, L258–266) | D3 engine choice is research-decided; ADR-0001 (C1) | **defer to ADR-0001** (hawkBit GATED front-runner, AOSP-native fallback) | ADR-0001 keeps hawkBit gated; no `hawkbit_*` columns in MVP schema. RESOLVED. |
| K5 | **Early user/multi-version rollback** roadmapped (§12 Phase 1.4.0, L1840–1858) | Locked MVP non-goal (end-user rollback deferred) | **defer**: MVP = automatic A/B boot-failure rollback only | master §1 non-goals; `device_deployments.status='rolled_back'` covers auto path. RESOLVED. |

Secondary conflicts (non-architectural, already reconciled in `additions_synthesis.md` §5):
6 h poll cadence (§10.2 L1488) vs locked **15 min + jitter** (master D7) — **reject** draft constant;
Redis as session store (§3.2.2/§9.1) — **accept-with-modification** (optional, prefer `cache` brick);
80% coverage floor (§8.1 L860) — **reject**, superseded by four-layer + mutation with ≥90% on
safety-critical paths (synthesis §5 C8).

## 4. requirement_inventory

ReqID format `R02-<section>.<n>`. Citations: corpus paths are relative to
`docs/research/main_specs/`; server paths are relative to `server/`.

### 4.1 research_existing_systems (draft §1)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-1.1 | Eclipse hawkBit evaluated as wrap candidate (§1.1 L24) | Research | COVERED — `research/stacks/eclipse-hawkbit.md`; `research/adr/adr-0001-wrapped-engine.md` §3.1 | K4 (hawkBit gated, not committed) | None; gated in ADR-0001 |
| R02-1.2 | hawkBit GitHub stats "579 stars… last commit June 5 2026" (§1.1 L28) | Research | MISSING / UNVERIFIED — not propagated as fact anywhere (synthesis §6 flagged) | none | Reject as fact; re-check live if needed |
| R02-1.3 | Mender evaluated; "C++ client, not Go" (§1.2 L46–60) | Research | COVERED — `research/stacks/mender.md`; ADR-0001 §3.2. "C++ client" UNVERIFIED (synthesis §6) | K4/C1 | None; Mender scored in ADR-0001 |
| R02-1.4 | TUF (go-tuf/v2) as security framework (§1.3 L62) | Research | COVERED — `research/stacks/tuf-go-tuf.md`; ADR-0002 §3.2 | K3 (TUF→1.0.1) | None; deferred in ADR-0002 |
| R02-1.5 | RAUC for Linux (§1.4 L80) | Research | COVERED — `research/stacks/rauc.md`; `1.X-linux/README.md` §2 | none | None (future phase) |
| R02-1.6 | SWUpdate for embedded Linux (§1.5 L91) | Research | COVERED — `research/stacks/swupdate.md`; `1.X-linux/README.md` §2 | none | None (future phase) |
| R02-1.7 | Wrap-hawkBit integration topology (Go→REST→hawkBit→DDI→devices) (§1.1 L38) | Architecture | COVERED as gated option — ADR-0001 §3.1; landscape report §3.2 | K4 | None; un-gate only if ADR-0001 selects hawkBit |

### 4.2 android15_ota_mechanism (draft §2)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-2.1 | Virtual A/B with compression (Android 15) (§2.1 L106) | Client | COVERED — `research/stacks/android15-virtual-ab.md`; master §4 device subgraph | none | None |
| R02-2.2 | 10-step update flow poll→download→verify→applyPayload→reboot→verify→rollback (§2.2 L115) | Client | COVERED — `1.0.0-mvp/client_android/integration_guide.md` §3 duty cycle; master §5 | none | None |
| R02-2.3 | OTA `.zip` structure (payload.bin, payload_properties.txt, META-INF, caremap) (§2.3 L130) | Client | COVERED — `1.0.0-mvp/client_android/update_engine_integration.md` | none | None |
| R02-2.4 | payload.bin internal structure (metadata/manifest/signatures/blobs) (§2.4 L144) | Client | COVERED — `research/stacks/aosp-update-engine.md`; update_engine_integration.md | none | None |
| R02-2.5 | `update_engine.applyPayload(url,offset,size,props)` API + callbacks (§2.5 L161) | Client | COVERED — `research/stacks/android-update-engine-api.md`; api `endpoints.md` §12.1 returns offset/size/props | none | None |
| R02-2.6 | payload_properties FILE_HASH/FILE_SIZE/METADATA_HASH/METADATA_SIZE (§10.1 L1329) | Client/API | COVERED — `endpoints.md` §12.1 `payload_properties`; `wire.go` ArtifactUploadMetadata | none | Optional Android-15 header set UNVERIFIED |

### 4.3 architecture (draft §3)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-3.1 | Universal OS-agnostic OTA with pluggable OS adapters (§3 L18, §11.3) | Architecture | COVERED — master §4 OS-adapter seam; `1.X-*/README.md` §3 adapter contract | none | None |
| R02-3.2 | Client layer (Android/Linux/Windows clients) (§3.1 L199) | Architecture | PARTIAL — Android COVERED (`client_android/`); Linux/Windows are research outlines (`1.X-linux`, `1.X-windows`) | none | Deferred to future phases |
| R02-3.3 | API gateway (Traefik/Kong/custom) (§3.2.1 L227) | Architecture | PARTIAL — reverse proxy in `deployment/overview.md` §3.4; no separate gateway service (monolith) | K1 | Accept reverse-proxy only; reject gateway-as-service |
| R02-3.4 | Auth service (JWT/OAuth2/RBAC) (§3.2.2 L237) | Architecture | COVERED — as a module: `endpoints.md` §4; `server/internal/api/middleware.go` requireRole, `token.go`, `users.go` | K1 (module not service) | None |
| R02-3.5 | Artifact service (upload/store/validate/checksum) (§3.2.3 L247) | Architecture | COVERED — `server/internal/api/handlers_artifact.go`; `1.0.0-mvp/server/artifact_validation.md` | K1 | None |
| R02-3.6 | Deployment service wrapping hawkBit (§3.2.4 L258) | Architecture | PARTIAL — deployment module built (`handlers_deployment.go`), all-targets only; hawkBit wrap gated | K1, K4 | Defer hawkBit to ADR-0001 |
| R02-3.7 | Device management service (registration/inventory/groups) (§3.2.5 L268) | Architecture | COVERED — `server/internal/api/handlers_device.go`; `store.go` Device | K1 | Groups schema present, group endpoints deferred (see R02-5.5) |
| R02-3.8 | Monitoring service (Prometheus/Grafana/ELK) (§3.2.6 L278) | Architecture | PARTIAL — telemetry ingest COVERED (`handlers_client.go`, `server/telemetry_processing.md`); Prometheus/Grafana/ELK wiring not built | K1 | OpenTelemetry via `observability` brick (master §3) |
| R02-3.9 | Security service (TUF/signing/key mgmt/HSM/Vault) (§3.2.7 L287) | Architecture | PARTIAL — signing/verify + key mgmt COVERED (`security/signing_verification.md`, `key_management.md`); TUF/HSM/Vault deferred | K3 | Defer TUF/HSM to ADR-0002/1.0.1 |
| R02-3.10 | Data layer: PostgreSQL/Redis/MinIO/Prometheus (§3.1 L222) | Architecture | PARTIAL — PostgreSQL + MinIO COVERED (master §3, `deployment/`); Redis optional (`cache` brick); Prometheus surface only | C7 (Redis) | Accept PG+MinIO; Redis only if needed |

### 4.4 database_schema (draft §4)

All of draft §4 is COVERED by `1.0.0-mvp/database/schema.md` (12 tables, executed migration on PG
16.14). Draft 02 was explicitly adopted as the **richer base**, normalized (master §7; schema.md §1).

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-4.1 | `users` + `api_keys` (§4.1 L304–323) | Schema | COVERED — schema.md §5.1–5.2 | none | None |
| R02-4.2 | `devices` + `device_groups` + `device_group_members` (§4.1 L326–355) | Schema | COVERED — schema.md §5.3–5.4 | none | None |
| R02-4.3 | `artifacts` + `artifact_versions` (§4.1 L358–385) | Schema | COVERED — schema.md §5.5 (checksum_sha256 NOT NULL + CHECK; signature col) | none | None |
| R02-4.4 | `deployments` (§4.1 L388–401) | Schema | COVERED — schema.md §5.7 (rollout_strategy JSONB; all_at_once default) | none | None |
| R02-4.5 | `deployment_phases` (§4.1 L403–414) | Schema | MISSING (deliberately) — deferred to 1.0.1 migration `002_*` | none | schema.md §8; `1.0.1-staged-rollout` §49 |
| R02-4.6 | `device_deployments` (§4.1 L416–427) | Schema | COVERED — schema.md §5.8 (phase_id omitted at MVP) | none | None |
| R02-4.7 | `rollouts` incl. `hawkbit_rollout_id` (§4.1 L430–441) | Schema | MISSING (deliberately) — hawkBit gated; deferred | K4 | schema.md §8; un-gate with ADR-0001 |
| R02-4.8 | `audit_logs` (§4.1 L444–454) | Schema | COVERED — schema.md §5.10 | none | None |
| R02-4.9 | `update_metrics` (§4.1 L457–469) | Schema | COVERED — renamed to canonical `telemetry_events`, schema.md §5.9 | none | None (rename) |
| R02-4.10 | Performance indexes (§4.1 L472–483) | Schema | COVERED — schema.md §7 (45 indexes, draft list + query-driven additions) | none | None |

### 4.5 api_specifications (draft §5)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-5.1 | REST `/api/v1` base + auth/artifacts/deployments/devices endpoints (§5.1 L490–555) | API | COVERED — `endpoints.md`; built routes in `server/internal/api/server.go` (auth, devices, artifacts, releases, deployments, client) | none | None |
| R02-5.2 | gRPC `UpdateService` (CheckForUpdate/Download/ReportStatus/RequestRollback) (§5.2 L557–624) | API | MISSING (deliberately) — gRPC out of scope; REST equivalents exist | K2 | Reject as primary; optional/internal only |
| R02-5.3 | Auth endpoints login/logout/refresh/device-register (§5.1 L493–498) | API | PARTIAL — login/refresh/register COVERED (`server.go` L112–119); explicit `/logout` not present (refresh-rotation model) | none | Accept-with-mod: no stateful logout in MVP |
| R02-5.4 | Artifact CRUD + validate + download (§5.1 L501–509) | API | PARTIAL — upload/get COVERED; PUT/DELETE/explicit validate/download-route not built (validate is inline on upload; download is the Range-served storage path) | none | Accept-with-mod; CRUD-delete deferred |
| R02-5.5 | Deployment lifecycle start/pause/resume/rollback + status/devices (§5.1 L512–524) | API | PARTIAL — create/get COVERED; start/pause/resume/rollback/devices-list are **rollout-engine** ops deferred to 1.0.1 | K5 (rollback) | Deferred to `1.0.1-staged-rollout` |
| R02-5.6 | Device CRUD + history + check-update (§5.1 L527–535) | API | PARTIAL — register/status/check-update COVERED; device PUT/DELETE/history not built | none | Accept-with-mod; history deferred |
| R02-5.7 | Device-groups CRUD + membership (§5.1 L538–546) | API | MISSING — group tables exist (schema §5.4) but no group endpoints built | none | Deferred; needed for grouped deployments (1.0.1) |
| R02-5.8 | Monitoring endpoints (dashboard/metrics/logs/alerts) (§5.1 L549–555) | API | MISSING — telemetry ingest built; read-side dashboard/metrics/alerts endpoints not built | none | Deferred to dashboard/monitoring phase |

### 4.6 phased_rollout (draft §6)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-6.1 | Phased rollout JSON config (ordered phases: %, success/error thresholds, duration, auto_progress) (§6.1 L632–683) | Rollout | PARTIAL — design seed adopted: `1.0.1-staged-rollout/README.md` §33; master §8. Full schema/SQL not yet written | none | Specify in `1.0.1-staged-rollout` |
| R02-6.2 | Phase names Canary/Pilot/Limited/GA (§6.1 L646–676) | Rollout | PARTIAL — pattern noted (Foundries wave/canary, 1.0.1 §33); not formalized | none | Specify in 1.0.1 |
| R02-6.3 | Rollout engine Go loop (start→monitor→threshold-check→advance/pause) (§6.2 L687–749) | Rollout | PARTIAL — adopted as design reference, hardened with deterministic cohort selection + idempotent transitions (master §8; 1.0.1 §33); not implemented | none | Implement `ota-rollout-engine` in 1.0.1 |
| R02-6.4 | pause_on_error / rollback_on_critical_failure / halt-on-breach (§6.1 L678–682) | Rollout | PARTIAL — "halt wins over advance" safety invariant captured (1.0.1 §33); auto-abort wiring is 1.0.1 (§37) | none | 1.0.1 |
| R02-6.5 | Notification channels (email/slack) (§6.1 L681) | Rollout/Ops | MISSING — alerting routed to `Herald` brick (master §9) but channels not specified | none | Specify in 1.0.1 monitoring via `Herald` |

### 4.7 security_model (draft §7)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-7.1 | TUF metadata signing (Root/Targets/Snapshot/Timestamp) (§7.1 L758–798) | Security | MISSING (deliberately) — deferred; ADR-0002 §3.2 (device enforcement gated) | K3 | 1.0.1 device TUF client |
| R02-7.2 | Artifact verify: signature + SHA-256/512 hash match before apply (§7.1 L800–832) | Security | COVERED — `security/signing_verification.md` §5–6; `server/internal/api/handlers_artifact.go` validation; device re-verify (`integration_guide.md` §7) | none | None |
| R02-7.3 | Auth flow: login→JWT→Redis session→validate-per-request (§7.2 L837–851) | Security | PARTIAL — OAuth2/JWT + refresh-rotation COVERED (`endpoints.md` §4; `token.go`); Redis session store optional (`cache` brick) | C7 | Accept; Redis only if needed |
| R02-7.4 | Key management (root/targets/snapshot/timestamp keys, HSM, Vault) (§7.1 L765–770) | Security | PARTIAL — MVP signing-key custody COVERED (`security/key_management.md`); multi-role TUF keys + HSM/Vault deferred | K3 | 1.0.1 / ADR-0002 |
| R02-7.5 | Zero-trust architecture (§14.1 L1960) | Security | PARTIAL — TLS 1.3, per-request JWT verify, device-id binding (master §6); full zero-trust not formalized | none | Threat model `00-master/threat_model.md` |

### 4.8 testing_strategy (draft §8)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-8.1 | Test levels (unit/integration/e2e/load/security) (§8.1 L859–889) | Testing | COVERED — superseded by four-layer + mutation model (master §13; `tests/test_strategy.md`); built tests present (`server/internal/api/*_test.go`) | none | None |
| R02-8.2 | 80% coverage minimum (§8.1 L860) | Testing | COVERED-with-override — four-layer + mutation immunity, ≥90% floor on safety-critical paths (synthesis §5 C8) | C8 | Reject flat 80%; per-component floor |
| R02-8.3 | Example tests (upload/phased-rollout/rollback) (§8.2 L893–977) | Testing | PARTIAL — upload/auth/device/deployment tests built (`handlers_*_test.go`); phased-rollout + rollback tests are 1.0.1 | none | 1.0.1 for rollout/rollback tests |
| R02-8.4 | Load test 10,000+ concurrent devices (§8.1 L878, §14.1 L1958) | Testing | MISSING / UNVERIFIED — no load-test harness or figures; rate-limit numbers UNVERIFIED (`endpoints.md` §5) | none | Defer; set numbers from MVP load tests |

### 4.9 infrastructure_deployment (draft §9)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-9.1 | Docker Compose dev stack (§9.1 L986–1105) | Infra | COVERED — `1.0.0-mvp/deployment/overview.md` + `docker-compose.mvp.yml`, re-based on `containers` brick; hawkBit service dropped (gated) | K4 | None |
| R02-9.2 | Kubernetes manifests (deployment/service/statefulset/probes) (§9.2 L1109–1240) | Infra | COVERED — `deployment/overview.md` §6 + `kubernetes/` manifests | none | None |
| R02-9.3 | Image digest pinning / no `:latest` (implicit) | Infra | COVERED — overview.md §11 anti-bluff fixes (minio pinned, digest TBD) | none | None |
| R02-9.4 | Secrets via env/Secret refs (§9 various) | Infra | COVERED — overview.md §5 secrets handling | none | None |

### 4.10 android_client (draft §10)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-10.1 | HelixOTAClient lib (checkForUpdate/downloadAndInstall/rollback) (§10.1 L1248–1463) | Client | COVERED — reworked to KMP in `1.0.0-mvp/client_android/integration_guide.md` (register→poll→download→verify→apply→report) | none | Reference Kotlin reworked, not copied |
| R02-10.2 | Verify checksum before apply (§10.1 L1319) | Client | COVERED — integration_guide.md §7 verify-before-apply (`Security-KMP`) | none | None |
| R02-10.3 | `file://` local apply path (§10.1 L1328) | Client | COVERED — local verified-file apply chosen (ADR-0002 §4.1; integration_guide.md §7); resolves draft's file:// vs HTTPS ambiguity (synthesis §6) | none | None |
| R02-10.4 | Foreground OTAUpdateService, 6 h periodic check (§10.2 L1480–1524) | Client | PARTIAL — WorkManager PeriodicWorkRequest COVERED (integration_guide.md §6); **6 h cadence rejected** for locked 15 min + jitter | C3 | Reject 6 h constant |
| R02-10.5 | Notification channel + progress UI (§10.2 L1541–1575) | Client | PARTIAL — duty cycle covers status reporting; notification UI not specified in MVP spec | none | Optional UI detail; low priority |
| R02-10.6 | Rollback via marking slot unbootable / root (§10.1 L1371–1381) | Client | COVERED-by-native — automatic A/B boot-failure rollback (AVB/boot_control), not app-driven (master §6; `research/stacks/android-avb-rollback.md`) | K5 | Reject app-driven rollback; native path |

### 4.11 future_os_support (draft §11)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-11.1 | Linux support via RAUC/SWUpdate/pkg-mgr adapters (§11.1 L1591–1626) | Future-OS | PARTIAL (deferred) — `1.X-linux/README.md` research outline + adapter table; not specified in depth | none | Future phase |
| R02-11.2 | Windows support (Windows Update API/MSI/MSIX/GroupPolicy) (§11.2 L1628–1655) | Future-OS | PARTIAL (deferred) — `1.X-windows/README.md` outline (MSIX/MSI/WinGet/WUA) | none | Future phase |
| R02-11.3 | Universal OSAdapter interface + AdapterRegistry (§11.3 L1657–1689) | Future-OS | COVERED (seam) — master §4 OS-adapter seam; `1.X-*/README.md` §3 contract (CheckForUpdate/Download/Verify/Install/Rollback/GetCapabilities) | none | Seam defined; registry impl future |
| R02-11.4 | RTOS / macOS support (§12 Phase 2.0.0 L1868–1872) | Future-OS | PARTIAL (deferred) — `1.X-other-os/README.md` | none | Future phase |

### 4.12 roadmap (draft §12)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-12.1 | 48-week, 8-phase roadmap (1.0.0…2.0.0) (§12 L1693–1885) | Planning | PARTIAL — re-expressed as phase directories (master §11); week-by-week schedule NOT adopted as canonical | none | Keep phase dirs; drop calendar |
| R02-12.2 | Phase 1.0.0 MVP (8 weeks) scope (§12 L1695–1726) | Planning | COVERED — master §5 MVP definition matches (auth/artifact/device/deploy/telemetry/android) | none | None |
| R02-12.3 | Phase 1.0.1 phased rollout (§12 L1728–1747) | Planning | COVERED — `1.0.1-staged-rollout/` (note: corpus folds rollback+TUF into 1.0.1, draft split them later) | none | None |
| R02-12.4 | Phases 1.0.2 monitoring / 1.1.0 TUF / 1.2.0 Linux / 1.3.0 Windows / 1.4.0 rollback / 2.0.0 universal | Planning | PARTIAL — mapped to phase dirs but renumbered/regrouped vs draft | none | Corpus numbering wins |

### 4.13 reusable_components (draft §13)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-13.1 | Reuse vasic-digital bricks: containers/security/auth/config/observability/cache/storage/eventbus/recovery/streaming (§13 L1892–1932) | Reuse | COVERED-with-correction — real catalogue in `00-master/submodule_reuse_map.md`; verified names (`auth`,`security`,`database`,`Storage`,`observability`,`eventbus`,`ratelimiter`,`middleware`,`http3`,`recovery`,`cache`,`config`, etc.) | none | None; surfaces UNVERIFIED |
| R02-13.2 | Reuse HelixDevelopment: HelixConstitution/helixqa/HelixCode/LLMProvider (§13 L1934–1951) | Reuse | PARTIAL — Constitution governs corpus (UNVERIFIED clauses); helixqa/HelixCode/LLMProvider not wired | none | LLMProvider is Phase 2 per draft |
| R02-13.3 | Guessed submodules (`go-common`, `helm-charts`, `vasic-digital/secrets`) | Reuse | CORRECTED — do not exist; replaced by verified catalogue (synthesis §6) | none | Reject guessed names |
| R02-13.4 | NEW submodules (ota-protocol, ota-artifact-validator, ota-rollout-engine, ota-update-engine-bridge, ota-android-agent, ota-telemetry-schema) | Reuse | COVERED — master §10 new-repo table; `ota-protocol` already imported in `server/internal/api/wire.go` | none | None |

### 4.14 success_factors (draft §14)

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R02-14.1 | Scalability 10,000+ concurrent devices; single-board→millions (§14.1 L1958, §3 L18) | NFR | PARTIAL / UNVERIFIED — guarantee stated (master §1); jitter design serves it (integration_guide §6); no load proof | none | Validate via load tests (deferred) |
| R02-14.2 | Reliability 99.9% uptime SLA (§14.1 L1959) | NFR | MISSING / UNVERIFIED — no SLA bound or HA spec | none | Out of MVP scope; future SRE spec |
| R02-14.3 | < 100 ms API response time (§14.1 L1961) | NFR | MISSING / UNVERIFIED — no perf budget defined | none | Defer; set from load tests |
| R02-14.4 | Compatibility: Android 15 (API 35+), Linux 5.10+, Windows 10+ (§14.1 L1962) | NFR | PARTIAL — Android 15 COVERED; Linux/Windows future | none | Future phases |
| R02-14.5 | Quality gates: zero critical vulns, security scanning, integration testing (§14.2 L1964–1970) | NFR/Quality | PARTIAL — four-layer testing COVERED; CI security scanning not specified | none | Add to CI spec |
| R02-14.6 | Risk mitigation: abstract hawkBit behind interface; horizontal scaling; DR/backups (§14.3 L1972–1978) | NFR | PARTIAL — hawkBit abstraction COVERED (ADR-0001 gated behind interface); DR/backup/sharding not specified | K4 | DR/backup is future ops spec |

## 5. unverified_register

Per §7.1 / §11.4.6 (UNVERIFIED clause numbers), claims that could not be confirmed against an
authoritative source:

- **HelixConstitution clause numbers** (§1, §7.1, §11.4.6, §11.4.8, §11.4.28, §11.4.61, §11.4.74)
  — carried from corpus convention; the Constitution text is not in this repository.
- **hawkBit GitHub stats** (R02-1.2) and **"Mender client is C++"** (R02-1.3) — draft-02 specifics,
  not independently re-verified; not propagated as fact.
- **Android-15 optional `payload_properties` headers** (`SWITCH_SLOT_ON_REBOOT`, `RUN_POST_INSTALL`,
  `DISABLE_DOWNLOAD_RESUME`) (R02-2.6) — UNVERIFIED against AOSP 15 (carried from `aosp-update-engine`
  open items).
- **Catalogue-brick public surfaces** (`auth`, `security`, `Storage`, `database`, `http3`,
  `ratelimiter`, `middleware`) — reuse claims are conditional on inspection (submodule_reuse_map
  Continuation).
- **NFR numbers** (10,000+ devices, 99.9% uptime, <100 ms) (R02-14.x) — asserted in draft, not
  measured or bound anywhere in the corpus.
- **Range-over-HTTP/3** and **Brotli quality tuning** — UNVERIFIED pending the ADR-0004 §6 spike.

## 6. sources

- `additions/initial_research_02.md` (the analyzed draft; line cites above).
- `research/additions_synthesis.md` — Rev 1 high-level synthesis (this doc is its granular successor for draft 02).
- `00-master/2026-06-07-helix-ota-design.md` — master design (§1 guarantees/non-goals, §2 locked decisions, §3 stack, §4 architecture/OS-adapter seam, §5 MVP, §6 trust, §7 data model, §8 rollout, §9 telemetry, §10 reuse, §11 phasing, §13 testing).
- `research/adr/adr-0001-wrapped-engine.md` … `adr-0005-delta-updates.md` — conflict resolutions K1–K5.
- `1.0.0-mvp/api/endpoints.md`, `database/schema.md`, `security/signing_verification.md`, `security/key_management.md`, `client_android/integration_guide.md`, `client_android/update_engine_integration.md`, `server/artifact_validation.md`, `server/telemetry_processing.md`, `deployment/overview.md`, `tests/test_strategy.md`.
- `1.0.1-staged-rollout/README.md`; `1.X-linux/README.md`, `1.X-windows/README.md`, `1.X-other-os/README.md`.
- `research/stacks/*` — eclipse-hawkbit, mender, tuf-go-tuf, uptane, rauc, swupdate, android15-virtual-ab, aosp-update-engine, android-update-engine-api, android-avb-rollback.
- Built server: `server/internal/api/server.go` (routes), `wire.go` (wire types), `handlers_*.go`, `middleware.go`, `token.go`, `users.go`; `server/internal/store/store.go` (Repository).
