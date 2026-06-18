# Helix OTA — Analysis of Addition #3 (`initial_research_03/helix_ota_big/`)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-08 |
| Last modified | 2026-06-08 |
| Status | active |
| Status summary | Exhaustive requirement-by-requirement analysis of the large generated research/design corpus in `additions/initial_research_03/helix_ota_big/` (26 markdown files, ~36k lines). Maps every requirement to the current canonical spec corpus (`docs/research/main_specs/`) and the ACTUALLY-BUILT server (`server/`). Reconciles the addition's own `CODE_REVIEW_AND_GAP_ANALYSIS.md` against the real build, flags conflicts with locked decisions, and catalogues future-phase docs to their versioned dirs. This is Rev-1 coverage of addition #3 — the prior `additions_synthesis.md` (Rev 1) did NOT cover it. |
| Issues | The addition is a parallel, self-consistent-but-divergent generation: different module path (`vasic-digital/helix-ota-server` vs built `HelixDevelopment/helix_ota`), different HTTP framework (chi vs LOCKED Gin), different crypto (RSA-4096/RSA-PSS vs built ed25519), different domain vocabulary (`updates`+`rollouts` vs canonical `releases`+`deployments`), and mTLS device auth vs built JWT-bearer. Its internal `CODE_REVIEW` audits the addition against ITSELF, not against the built system — its findings are re-scored here against the real build. |
| Issues summary | Treated as authoritative INPUT (D5) but NOT as canonical: where it conflicts with locked decisions or the built server, the locked/built side wins; reusable material is folded forward. |
| Fixed | Addition #3 fully processed (priority docs read in full / deep; future-phase docs catalogued). |
| Fixed summary | All 26 `.md` files inventoried; binaries/exports (.pdf/.docx/.html/.tar/.zip) intentionally skipped as duplicates. |
| Continuation | Fold accepted MVP gaps (user/device-group/api-key CRUD, telemetry-event endpoint, mandatory-update behavior, anti-rollback check, tamper-event, one-active-deployment-per-group constraint) into `endpoints.md`/`schema.md`/`server/`. Route future-phase material into the versioned dirs listed in §6. Re-run when addition #4 lands. |
| Owner | Helix OTA spec-analysis |
| Related | [`additions_synthesis.md`](../additions_synthesis.md); [`../../1.0.0-mvp/api/endpoints.md`](../../1.0.0-mvp/api/endpoints.md); [`../../1.0.0-mvp/database/schema.md`](../../1.0.0-mvp/database/schema.md); [`../../1.0.0-mvp/server/architecture.md`](../../1.0.0-mvp/server/architecture.md); [`../../00-master/2026-06-07-helix-ota-design.md`](../../00-master/2026-06-07-helix-ota-design.md) |

## Table of contents

- [§1. method and evidence base](#1-method-and-evidence-base)
- [§2. document inventory](#2-document-inventory)
- [§3. requirement inventory table](#3-requirement-inventory-table)
- [§4. conflicts with locked decisions and the built server](#4-conflicts-with-locked-decisions-and-the-built-server)
- [§5. reconciling the addition's code_review against the actual build](#5-reconciling-the-additions-code_review-against-the-actual-build)
- [§6. future-phase catalogue](#6-future-phase-catalogue)
- [§7. folding actions summary](#7-folding-actions-summary)

## §1. method and evidence base

Three lenses per the operator mandate (D5; Constitution §11.4.6 no-guessing, §11.4 anti-bluff): **extract** reusable material, **reconcile** against the canonical corpus + built code, **route** each element to its destination. Evidence cited by file. Claims that could not be verified against the built code or live sources are marked **UNVERIFIED**.

**Built-server ground truth** (read this revision): `server/internal/api/server.go` (Gin router + routes), `wire.go` (wire types), `store/store.go` (Repository + domain structs), `handlers_client.go`, `handlers_deployment.go`. Key facts:

- Module: `github.com/HelixDevelopment/helix_ota/server`; depends on `HelixDevelopment/ota-protocol`, `ota-artifact-validator`, `ota-telemetry-schema`, `ota-rollout-engine`.
- HTTP framework: **Gin** (`gin-gonic`) — matches LOCKED D6.
- Persistence: **in-memory** `MemoryRepository` (`store/memory.go`); pgx/PostgreSQL is a documented future target (`store.go` header, `architecture.md §4`). No DB built yet.
- Domain vocabulary: **devices → artifacts → releases → deployments → telemetry**. Deployment strategy = `all-targets` only; staged rollout deferred to 1.0.1 (routes the cohort check through `ota-rollout-engine.InCohort` so the staged engine drops in).
- Artifact signing: **ed25519 detached signature + SHA-256** (`server.go` `ed25519.PublicKey`, `pubKey`), not RSA-4096.
- Auth: **JWT bearer** (`TokenSigner`, `authMiddleware`, RBAC roles admin/operator/viewer/device). No mTLS in the built server.
- Device endpoints: `GET /client/update` (204/200) and `POST /client/telemetry`; device acts only on its own token subject.

**Canonical corpus ground truth**: `endpoints.md` (rev 2), `schema.md`, `architecture.md`, master design. These match the built server closely (Gin, releases/deployments, all-targets MVP, opaque IDs, SHA-256+signature+AVB, staged rollout → 1.0.1, TUF → 1.0.1+).

## §2. document inventory

Base: `additions/initial_research_03/helix_ota_big/`. 26 `.md` files. Binaries/exports (.pdf/.docx/.html/.tar/.zip) intentionally not opened (duplicate renderings of the `.md`).

| # | Document (path) | Lines | What it specifies |
|---|---|---|---|
| D0 | `CODE_REVIEW_AND_GAP_ANALYSIS.md` | 539 | Independent review of the addition's OWN docs. 11 CRITICAL + 9 HIGH + 8 MEDIUM + 7 LOW gaps; all are internal contradictions (endpoint paths, enum sets, ID convention, validation order, Go module path) between the addition's own files. Reconciled in §5. |
| D1 | `README.md` | 153 | Project overview, version roadmap matrix, doc index, vasic-digital submodule list, 8 new `helix-*` submodules. Module org: `HelixDevelopment` + `vasic-digital`. RSA-4096 + SHA-256 + A/B; plugin OS-adapter vision. |
| D2 | `1.0.0-mvp/server/SERVER_IMPLEMENTATION.md` | 3968 | Full Go server design: project layout (chi router, `internal/handler|service|repository|model|validation`), `go.mod` (`github.com/vasic-digital/helix-ota-server`, chi, pgx, RSA), service interfaces (UpdateService/DeviceService/RolloutService/ArtifactService/TelemetryService/AuthService), rollout decision engine (DRAFT→RUNNING→PAUSED/HALTED→COMPLETED, deterministic SHA-256 device bucketing, dwell-time advance, auto-halt on failure-rate), validation pipeline (Structure→Hash→Signature→Compatibility, streaming SHA-256, RSA-PSS verify, worker pool), mTLS device enrollment, telemetry, background jobs, middleware, main, submodule integration. |
| D3 | `1.0.0-mvp/client/android/ANDROID_CLIENT_DESIGN.md` | 3202 | Android 15 / RK3588 client: Virtual A/B (Seamless v2), `update_engine` Binder IPC + `ApplyPayload`, A/B partition layout, OTA zip/`payload.bin`/`payload_properties.txt`/`care_map.pb` handling, resumable Range download, SHA-256+RSA verification engine, AOSP build integration (`/system/priv-app`), UX, Go client SDK (gomobile AAR). |
| D4 | `1.0.0-mvp/docs/api/REST_API_SPECIFICATION.md` | 3333 | REST surface: auth (login/refresh/logout/TOTP), device mgmt (`/devices/register`), artifact mgmt (`/artifacts/upload`, `/artifacts/{id}/download`), update check (`POST /updates/check`), rollout mgmt (`/rollouts`), telemetry/reporting, Dashboard WebSocket (underspecified), Go struct appendix, error/rate-limit appendices (clipped). |
| D5 | `1.0.0-mvp/docs/VERSION_ROADMAP.md` | 800 | Version contract MVP→Rollback→Delta→Linux→Windows→Universal. MVP feature lists (server/client/dashboard) + acceptance criteria. Conflicting endpoint paths (`POST /devices`, `GET /devices/{id}/update-check`, `POST /artifacts`). |
| D6 | `1.0.0-mvp/docs/architecture/SYSTEM_ARCHITECTURE.md` | 2795 | Full system architecture; Go structs (Device/Rollout), service decomposition, NotificationService/DashboardSession, validation chain (Hash-first). Conflicts with DB schema on field names (per D0). |
| D7 | `1.0.0-mvp/docs/database/DATABASE_SCHEMA.md` | 1691 | PostgreSQL 16 DDL: UUID v4 PKs, enums (rollout_status, device_status, upload_status, os_type CHECK), tables (devices, device_groups, artifacts, updates, rollouts, rollout_stages, telemetry partitioned, users, api_keys, audit), indexes, partitioning, migration strategy. |
| D8 | `1.0.0-mvp/docs/security/SECURITY_ARCHITECTURE.md` | 1908 | STRIDE threat model; transport (mTLS), artifact security (SHA-256 + RSA + chain), authn/authz (JWT + RBAC matrix + API keys + TOTP), device identity/cert provisioning, update-safety guarantees, server security, compliance/audit, security testing. |
| D9 | `1.0.0-mvp/docs/submodules/NEW_SUBMODULE_SPECIFICATIONS.md` | 2351 | Specs for 8 new `helix-*` submodules (server, client-sdk, android, dashboard, update-engine, artifact-validator, rollout-engine, device-identity). Module path `dev.helix.ota.*` (conflicts with D2). Container integration, release checklist, dependency matrix. |
| D10 | `1.0.0-mvp/docs/testing/TESTING_STRATEGY.md` | 2043 | Four-layer fix verification, mutation testing (≥85%), unit/integration/e2e/HIL, anti-bluff; CI/CD section clipped. |
| D11 | `1.0.0-mvp/docs/deployment/DEPLOYMENT_GUIDE.md` | 1996 | Docker/K8s deployment; env vars; monitoring; DR (referenced, partly missing). |
| D12 | `1.0.0-mvp/docs/research/OTA_SYSTEMS_RESEARCH.md` | 1964 | Survey of existing OTA solutions (overlaps canonical `research/ota_landscape_report.md` + `research/stacks/`). |
| D13 | `1.0.0-mvp/containers/docker-compose.yml` | 793 | Full stack: Traefik (TLS/ACME/mTLS forwarding), helix-ota-server (chi per comment), dashboard (React/nginx), PostgreSQL 16 (tuned), Redis 7, MinIO (SSE, versioning), **HashiCorp Vault** (transit RSA-4096 signing, kv-v2, TOTP), minio-init, vault-init, db-migrate. |
| D14 | `1.0.1-rollback/docs/ROLLBACK_DESIGN.md` | 2127 | 1.0.1 rollback: auto (boot-failure) / server-triggered / user-initiated; rollback vs downgrade; version-history mgmt; data safety; rollback API; Go + Kotlin reference impls. |
| D15 | `1.0.2-delta-updates/docs/DELTA_UPDATES_DESIGN.md` | 1969 | 1.0.2 delta/differential OTA; AOSP block diffs; `helix-delta-gen` submodule; delta selection; 60–90% bandwidth reduction; DB+API additions; benchmarks. |
| D16 | `1.1.0-linux-support/docs/LINUX_OTA_RESEARCH.md` | 2373 | 1.1.0 Linux OTA: apt/rpm-ostree/pacman/generic A/B rootfs; OSAdapter plugin interface; new submodules; migration path. |
| D17 | `1.2.0-windows-support/docs/WINDOWS_OTA_RESEARCH.md` | 2909 | 1.2.0 Windows OTA: WUA/WSUS/WUfB/Intune; Windows Service client; MSI/MSIX; Windows security (code signing); Windows adapter; submodules. |
| D18 | `2.0.0-multi-os-universal/docs/MULTI_OS_UNIVERSAL_DESIGN.md` | 1466 | 2.0.0 universal: dynamically-loaded OS-adapter plugin system; OS catalog; OS-agnostic pipeline; cross-OS dashboard + dependency coordination; plugin manifest. |
| D19–D26 | `1.0.0-mvp/docs/diagrams/*.md` (8 files) | 79–258 ea | Mermaid: high-level arch, update flow, rollout flow, client state machine, artifact validation, database ER, security layers, container arch. Overlap canonical `00-master/diagrams/`. |

## §3. requirement inventory table

Coverage is vs the union of the **canonical corpus** and the **built server**. "Built" = present in `server/`. "Corpus" = present in a spec doc. Source §s reference the addition.

| ReqID | Requirement (source) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| R01 | Device registration endpoint | API | COVERED — built `POST /devices/register` (server.go:119); corpus `endpoints.md §8.1`. Addition self-conflicts on path (D0/C1). | Addition C1 N/A vs build (build already canonical). | None. |
| R02 | Device registration body fields | API/Model | PARTIAL — built `DeviceRegistration{hardware_id,model,os,os_version,current_version,group,metadata}` (wire.go:41). Addition wants `slot_suffix`,`hardware_fingerprint`. | Addition uses RSA/mTLS-era fields. | Consider optional `slot_suffix`/`hardware_fingerprint` in metadata; not MVP-blocking. |
| R03 | Update check (device-facing) | API | COVERED — built `GET /client/update` 204/200 (handlers_client.go); corpus `endpoints.md §12.1`. Addition self-conflicts GET vs POST (D0/C3). | Build/corpus chose `GET /client/update`; addition's `POST /updates/check` rejected. | None. |
| R04 | Artifact upload + validation | API | COVERED — built `POST /artifacts/upload` (multipart, ed25519 verify); corpus `endpoints.md §9.1`, `artifact_validation.md`. | Addition path `/artifacts` (roadmap) vs `/artifacts/upload`. | None. |
| R05 | Artifact download w/ Range, identity encoding | API | COVERED — corpus `endpoints.md §3/§12.1` (ZIP_STORED, identity, Range mandatory); built returns reference URL (`artifactURL`). Byte-serving path = Storage brick. | None. | Confirm Storage-brick Range serving (UNVERIFIED, ADR-0004 §6). |
| R06 | Release create/list/get | API | COVERED — built `/releases` CRUD (server.go:125-127); corpus `endpoints.md §10`. Addition has no "release" concept (uses "update"). | Vocabulary: addition `Update` ≈ canonical `Release`. | None. |
| R07 | Deployment (all-targets) create/get | API | COVERED — built `/deployments` (handlers_deployment.go); corpus `endpoints.md §11`. | Addition uses `rollouts` (staged) at MVP. | None (staged → 1.0.1). |
| R08 | Staged/phased rollout engine (5/10/30/50/100, dwell, auto-halt) | Rollout | PARTIAL/DEFERRED — corpus defers to **1.0.1-staged-rollout**; built routes cohort via `ota-rollout-engine.InCohort`. Addition gives a full Go decision engine (SERVER_IMPL §4). | Timing: addition puts full engine in MVP; locked scope = 1.0.1. | Fold addition's decision-engine algorithm into `1.0.1-staged-rollout/` as design seed. |
| R09 | Deterministic device→cohort hashing | Rollout | COVERED — built `rollout.InCohort` seam; addition `SelectForPercentage` (SERVER_IMPL §4.3) is a compatible reference. | None. | Cross-check hashing approach when staged engine authored. |
| R10 | Telemetry ingest (batch) | API | COVERED — built `POST /client/telemetry` (batch, schema-validated); corpus `endpoints.md §12.2`, `telemetry_processing.md`. | Addition splits device-status vs telemetry-events (D0/H9). | See R26. |
| R11 | Auth: login/refresh + JWT + RBAC | Security | COVERED — built `/auth/login`,`/auth/refresh`, `TokenSigner`, roles admin/operator/viewer/device. | Addition adds logout + TOTP. | TOTP/logout optional hardening; note for 1.0.1. |
| R12 | Artifact integrity: SHA-256 + signature, server+device | Security | COVERED — built ed25519 detached sig + SHA-256; corpus `signing_verification.md`. | **CONFLICT**: addition mandates **RSA-4096 / RSA-PSS** (SERVER_IMPL §5.4, compose Vault `type=rsa-4096`). Build = ed25519. | Keep ed25519 (built+canonical). See §4. |
| R13 | A/B + AVB/dm-verity + auto boot-failure rollback | Client/Safety | COVERED — corpus master §1, `client_android/update_engine_integration.md`, `research/stacks/android-avb-rollback.md`. Addition D3 is a strong reference. | None (agrees). | Fold AOSP nano-detail from D3 into `client_android/` where deeper than current. |
| R14 | Virtual A/B + payload.bin + applyPayload headers | Client | COVERED — corpus `research/stacks/android15-virtual-ab.md`, `update_engine_integration.md`. D3 adds depth (snapuserd, COW, super_a/b). | None. | Fold D3 depth into `update_engine_integration.md` (cite UNVERIFIED AOSP specifics). |
| R15 | Validation chain order | Security/Safety | COVERED (different order) — corpus/built validate signature+hash; addition D0/C9 mandates **Hash→Signature→Structure→Compatibility**. Built uses `ota-artifact-validator`. | Addition self-conflicts (SYS_ARCH Hash-first vs SERVER_IMPL Structure-first). | Verify `ota-artifact-validator` enforces hash-before-signature; document order in `artifact_validation.md` (UNVERIFIED current order). |
| R16 | Anti-rollback / downgrade prevention | Safety | **MISSING** (MVP-critical) — addition SA2: no `target_version > device.current_version` server+client check. Built `handleClientUpdate` compares versions for steady-state (≥ ⇒ 204) but does NOT forbid a release whose version < device. Corpus `endpoints.md §2` says monotonicity enforced on release creation only. | None (gap on both sides). | Add explicit downgrade-prevention check to update-check + release validation; doc in `endpoints.md`/`artifact_validation.md`. |
| R17 | One active rollout/deployment per group | Rollout | PARTIAL — built rejects overlapping active deployment per target set (`ActiveDeploymentForTarget` → 409, handlers_deployment.go:65). Addition M6 wants DB partial-unique index. | None (build already guards; DB index pending pgx). | When pgx lands, add partial unique index per `schema.md`. |
| R18 | Mandatory update enforcement behavior | API/Client | **MISSING** — addition M7: `mandatory`/`deadline` semantics undefined. Not in built update-check (no mandatory field) nor corpus. | None. | Define mandatory/deadline behavior in `endpoints.md §12.1` + client design (1.0.1 candidate). |
| R19 | User management CRUD | API | **MISSING** — addition H1; RBAC matrix exists (D8) but no user CRUD. Built has static `UserDirectory` only (server.go:22). Corpus: not specified. | None. | Add user CRUD to `endpoints.md` (admin-only) — post-MVP unless dashboard needs it. |
| R20 | Device-group management CRUD | API | **MISSING** — addition H2. Built carries `group` as a device string field; no group entity. Corpus: deployments accept optional `group`. | None. | Add device-group CRUD when fleet segmentation lands (1.0.1). |
| R21 | API-key management CRUD | Security/API | **MISSING** — addition H3; D8 defines key format but no CRUD. Built: none. | None. | Defer; JWT-bearer covers MVP. Note for hardening. |
| R22 | Dashboard WebSocket/SSE wire protocol | API | **MISSING** — addition H4 (under-specified even in addition). Built: none. Corpus: none. | None. | Defer to dashboard design (not yet authored — see R23). |
| R23 | Dashboard Web UI design doc | Dashboard | **MISSING** — addition H5; no dashboard design at parity with client. Corpus: no dashboard design doc yet. | None. | Author `dashboard/` design when dashboard phase starts. |
| R24 | Go Client SDK design doc | Client | PARTIAL — addition H6 wants full SDK doc; corpus has `client_android/integration_guide.md`+`build_integration.md`. gomobile AAR constraints (addition L7) undocumented. | None. | Add gomobile-AAR constraints note to `client_android/`. |
| R25 | Rollout/Device model ↔ DB alignment | Model | PARTIAL — addition H7/H8 are internal addition mismatches. Built uses flat `store.*` structs (no DB yet). Canonical `schema.md` is the target. | Addition self-conflict only. | Align Go structs to `schema.md` when pgx implemented. |
| R26 | Telemetry event endpoint (rich events) distinct from device status | API | PARTIAL — addition H9. Built telemetry IS event-based (`ota-telemetry-schema`, EventDownloadStarted/Installing/Success/Failure). Per-device event history (`GET /devices/{id}/events`) + `/telemetry/overview` not built. | None. | Add read endpoints for telemetry history/overview when dashboard needs them. |
| R27 | Tampered-artifact server-side alerting (`SECURITY_TAMPER_DETECTED`) | Security | **MISSING** — addition SA3. Built: device HASH_MISMATCH not specially handled; corpus: general failure-rate only. | None. | Define tamper event type that halts rollout + alerts; route to `1.0.1-staged-rollout/` + threat model. |
| R28 | mTLS device transport | Security | NOT-BUILT — addition mandates mTLS (SECURITY §3, compose Traefik device router). Built uses **device JWT bearer**; corpus `transport_security.md` = HTTP/3→HTTP/2 + bearer. | **CONFLICT** w/ built (JWT) — see §4. | Keep JWT-bearer for MVP; mTLS optional hardening (note in `security/`). |
| R29 | Vault for signing/secrets | Deployment/Security | NOT-BUILT — addition compose runs Vault (transit RSA-4096, kv-v2, TOTP). Built: key via config/ed25519. Corpus `key_management.md` = `security` brick + container secrets first (§11.4.74). | Partial conflict (RSA-4096 in Vault). | Evaluate `security` brick + container secrets before Vault (matches synthesis §5 C correction). |
| R30 | Traefik ingress / TLS / ACME | Deployment | PARTIAL — addition compose uses Traefik. Built: plain Gin + `/healthz`,`/readyz`. Corpus `deployment/overview.md`. | None (ingress is deploy-time). | Reuse `containers` substrate; Traefik optional. |
| R31 | Redis cache/session/ratelimit | Infra | OPTIONAL — addition compose requires Redis. Built: none. Corpus/synthesis C7: prefer `cache` brick; Redis only if needed. | None. | Keep optional via `cache` brick. |
| R32 | PostgreSQL schema (devices/artifacts/releases/rollouts/telemetry/users/groups/api_keys/audit) | DB | PARTIAL — canonical `schema.md` exists; built store is in-memory (no DB). Addition D7 is a richer reference (partitioning, indexes, enums, audit). | Enum/ID conflicts are addition-internal (D0/C5–C8,C11). | When pgx implemented, mine D7 for partitioning/index/audit detail; reconcile enums to canonical wire values. |
| R33 | Mutation testing ≥85%, four-layer verification | Testing | COVERED — corpus `tests/test_strategy.md`, `VALIDATION_EVIDENCE.md`; addition D10 agrees. | Coverage % (85 vs 90) — synthesis C8 floor ≥90 for safety paths. | None. |
| R34 | Containerized via `containers` submodule | Deployment | COVERED — master §3; addition compose self-contained. | None. | Re-base addition compose onto `containers` substrate. |
| R35 | 8 new `helix-*` submodules | Submodules | PARTIAL/DIVERGENT — addition D9 names `helix-ota-server/-client-sdk/-android/-dashboard/-update-engine/-artifact-validator/-rollout-engine/-device-identity`. Built repos are `ota-protocol/ota-artifact-validator/ota-telemetry-schema/ota-rollout-engine` under `HelixDevelopment`. | **CONFLICT**: names + module paths differ; addition uses `dev.helix.ota.*` (D9) AND `vasic-digital/helix-ota-server` (D2) — self-conflict (D0/C10). | Canonical = built `HelixDevelopment/ota-*` repos. Map addition's submodule intents to existing repos in `submodule_reuse_map.md`. |
| R36 | OS-adapter seam (universality) | Architecture | COVERED — master §4 OS-adapter seam; addition D16/D18 elaborate. | None. | Fold into future-OS phase dirs (§6). |

## §4. conflicts with locked decisions and the built server

| # | Addition position | Locked / built reality | Resolution |
|---|---|---|---|
| X1 | **chi** router (`go-chi/chi/v5`, SERVER_IMPL §1–2; compose comment "Chi router") | LOCKED **Gin** (D6); built uses Gin (`server.go`). | **Gin wins.** Discard chi. The addition's handler/service/repository layering is still a useful reference for internal seams (ADR-0003 modular monolith). |
| X2 | **RSA-4096 / RSA-PSS** artifact signing (SERVER_IMPL §5.4; README; compose Vault `type=rsa-4096`) | Built **ed25519** detached signature + SHA-256 (`server.go`, `ota-artifact-validator`); corpus `signing_verification.md`. | **ed25519 wins** (built + canonical). RSA-4096 is heavier and not what's implemented. If RSA support is ever desired it must be an additive, justified ADR change — not a silent swap. |
| X3 | **mTLS** device transport + Device CA provisioning (SECURITY §3/§6; compose device-mtls router) | Built **device JWT bearer**; corpus `transport_security.md` (HTTP/3→HTTP/2 + bearer). | **JWT-bearer wins for MVP.** mTLS is a valid hardening path; record as optional in `security/` but do not block MVP. |
| X4 | Module path `github.com/vasic-digital/helix-ota-server` (D2) **and** `dev.helix.ota.*` (D9) | Built `github.com/HelixDevelopment/helix_ota/server`; deps under `HelixDevelopment/ota-*`. | **Built path wins.** Addition is internally inconsistent (its own C10). |
| X5 | Domain vocabulary **`updates` + `rollouts`** at MVP (D4/D6/D7) | Built/canonical **`releases` + `deployments`**; staged `rollouts` deferred to 1.0.1. | **releases/deployments wins.** Addition `Update`→`Release`, addition `Rollout`→ (deployment now / staged-rollout 1.0.1). |
| X6 | **Full staged-rollout decision engine in MVP** (SERVER_IMPL §4) | Locked scope: MVP = all-targets; staged → 1.0.1 (master §1 non-goals; built `strategyAllTargets`). | **Defer.** Keep the engine algorithm as a 1.0.1 design seed (high-quality reference). |
| X7 | **Vault** as signing/secret store (compose) | Corpus: `security` brick + container secrets first (§11.4.74); synthesis C7-adjacent. | **Catalogue-first.** Evaluate `security` brick before adopting Vault. |
| X8 | UUID-v4 PKs exposed; OR prefixed string IDs (D7 vs D5/D6 — addition's own C11) | Built/canonical: **opaque server-issued strings**, UUID-formatted in OpenAPI, treated opaque by clients (`endpoints.md §2`, `newRandomID`). | **Opaque-string wins.** Internal UUID vs external-id split (addition's own recommendation) already satisfied by the opaque-id convention. |
| X9 | Coverage target **85%** (D5/D10) | Synthesis C8: floor **≥90%** for safety-critical paths (signing/apply/rollout-gate). | ≥90% floor for safety paths; 85% elsewhere acceptable. |

No conflict was found with: A/B+AVB+auto-rollback, SHA-256 integrity, staged percentages (5/10/30/50/100), telemetry event stream, PostgreSQL+MinIO, containerization, REST-primary, mutation/four-layer testing — these AGREE with locked decisions and are high-confidence inputs.

## §5. reconciling the addition's code_review against the actual build

The addition's `CODE_REVIEW_AND_GAP_ANALYSIS.md` (D0) audits the **addition's own documents against each other**. It does NOT see the built server. Re-scored against the real build + canonical corpus:

**CRITICALs that are NOT real against the build (already resolved by canonical choices):**

- **C1 (device-register path), C3 (update-check method/path), C4 (artifact-upload path):** The built server already fixed canonical paths — `POST /devices/register`, `GET /client/update`, `POST /artifacts/upload`. The addition's contradictions are artifacts of its own multi-author generation; not blockers for Helix.
- **C5/C6/C8 (rollout/device/validation status enums diverge across addition docs):** Built uses typed enums from `ota-protocol`/`ota-telemetry-schema` (single source). No three-way enum drift in the real system. Real residual: ensure wire enum strings are documented (mostly done in `endpoints.md` rev 2 — telemetry 6-value enum vs 7-value update-state).
- **C7 (os_type CHECK vs API enum):** Built has no DB yet; `defaultAndroidPolicy` accepts Android only (Phase-1). When pgx lands, use application-level validation (not a brittle CHECK) — matches addition's own recommendation.
- **C10 (Go module path):** Built path is canonical (`HelixDevelopment/helix_ota`). Addition's two-path conflict is moot.
- **C11 (UUID vs prefixed string IDs):** Resolved by the opaque-id convention (X8).
- **C2 (registration body schema drift):** Built body is canonical (`DeviceRegistration`). Optional `slot_suffix`/`hardware_fingerprint` are the only real takeaways (R02).

**Findings that ARE real gaps against the current build (MVP-relevant first):**

- **SA2 / "anti-rollback protection" (→ R16): REAL, MVP-critical.** No explicit `target_version > device.current_version` downgrade block exists in the built update-check or release validation. Steady-state `≥ ⇒ 204` is not a downgrade guard. **Add server-side + (spec) client-side downgrade prevention.**
- **C9 / SA1 "validation chain order" (→ R15): REAL but indirect.** Built validation lives in `ota-artifact-validator`; the hash-before-signature ordering must be **verified** there and documented. (UNVERIFIED current order in the external module.)
- **M6 "concurrent rollout overlap" (→ R17): PARTIALLY real.** Built already 409s on overlapping active deployment per target set; the DB partial-unique index is a future-pgx hardening, not a current bug.
- **M7 "mandatory update behavior" (→ R18): REAL gap** (undefined everywhere).
- **H1/H2/H3 (user / device-group / api-key CRUD → R19/R20/R21): REAL gaps** (not built, not in corpus). Severity is post-MVP unless the dashboard needs them.
- **H4 (WebSocket protocol → R22), H5 (dashboard design → R23), H6/L7 (SDK doc + gomobile constraints → R24): REAL doc gaps** (dashboard phase not yet authored).
- **H9 (telemetry history/overview reads → R26): PARTIALLY real** — ingest is built; read/aggregate endpoints are not.
- **SA3 (tamper-detected event → R27): REAL gap.**
- **M1 (download URL signing): REAL but a documented design choice** — built returns a reference URL; pre-signed vs direct-auth must be decided in `endpoints.md`/Storage-brick (UNVERIFIED which the Storage brick provides).
- **M2–M5, L1–L6 (provisioning guide, AOSP integration guide, CI/CD doc, migration strategy, runbooks, `.env.example`, SLOs, backup/restore): REAL doc gaps**, low priority; route to ops/security/infra docs as those phases land.

**Net:** Most of the addition's 11 CRITICALs are self-referential and already settled by the canonical build. The genuinely actionable, build-relevant items are **R16 (anti-rollback — MVP-critical), R18 (mandatory-update), R15 (verify validation order), R27 (tamper event), R26 (telemetry reads), and the CRUD/doc gaps R19–R24.**

## §6. future-phase catalogue

Each future-phase doc maps to an existing versioned dir under `docs/research/main_specs/`. The addition uses `1.0.1-rollback` / `1.0.2-delta-updates` / `1.1.0-linux-support` / `1.2.0-windows-support` / `2.0.0-multi-os-universal`; the canonical corpus currently has `1.0.1-staged-rollout`, `1.X-linux`, `1.X-windows`, `1.X-other-os`. Note the **numbering divergence**: addition slots rollback at 1.0.1 and delta at 1.0.2, whereas canonical reserves **1.0.1 for staged-rollout** (locked: staged rollout is the first post-MVP increment). Recommendation: keep canonical 1.0.1 = staged-rollout; place addition's rollback/delta material as **1.0.2 / 1.0.3** (or sub-sections) so the staged-rollout engine — which both depend on — lands first.

| Addition doc | Material | Destination dir | Notes |
|---|---|---|---|
| D14 ROLLBACK_DESIGN (1.0.1) | server-triggered + user-initiated + multi-version rollback, version history, rollback API, Go/Kotlin impls | **`1.0.X-rollback/`** (new; suggest 1.0.2 after staged-rollout) | Auto boot-failure rollback is ALREADY MVP (A/B+AVB). This doc covers the *added* rollback types — matches locked "end-user/multi-version rollback deferred past MVP". |
| D15 DELTA_UPDATES (1.0.2) | AOSP block-diff deltas, `helix-delta-gen`, delta selection, DB+API additions, benchmarks | **`1.0.X-delta-updates/`** (suggest 1.0.3) | Aligns with ADR-0005 (delta updates). Fold benchmarks as UNVERIFIED until measured. |
| D16 LINUX_OTA_RESEARCH (1.1.0) | apt/rpm-ostree/pacman/generic A/B; OSAdapter interface; migration path | **`1.X-linux/`** (exists) | Strong research; reconcile OSAdapter interface with master §4 OS-adapter seam. |
| D17 WINDOWS_OTA_RESEARCH (1.2.0) | WUA/WSUS/WUfB/Intune; Windows Service client; MSI/MSIX; code-signing | **`1.X-windows/`** (exists) | "augment not replace Windows Update" thesis. |
| D18 MULTI_OS_UNIVERSAL (2.0.0) | dynamic OS-adapter plugin system; cross-OS dashboard + dependency coordination; plugin manifest | **`1.X-other-os/`** (exists) or new `2.0.0-multi-os-universal/` | Plugin/manifest design = the universality endgame; reconcile with OS-adapter seam. |
| D12 OTA_SYSTEMS_RESEARCH | OTA solution survey | merge into `research/ota_landscape_report.md` + `research/stacks/` | Likely redundant with existing landscape report; diff before folding. |
| D19–D26 diagrams | Mermaid arch/flow/state/ER/security/container | merge into `00-master/diagrams/` | De-dup against existing diagrams. |

## §7. folding actions summary

**MVP-critical (do for 1.0.0):**
1. **R16 anti-rollback / downgrade prevention** — add `target_version > device.current_version` enforcement server-side (update-check) + release validation; document in `endpoints.md §12.1` and `artifact_validation.md`. (Addition SA2.)
2. **R15 validation-order verification** — verify `ota-artifact-validator` performs hash-before-signature; document the order (currently UNVERIFIED). (Addition C9/SA1.)

**Near-term / 1.0.1 candidates:**
3. **R18 mandatory-update behavior** (M7) — define `mandatory`/`deadline` semantics.
4. **R27 tamper event** (SA3) — `SECURITY_TAMPER_DETECTED` halts rollout + alerts; into `1.0.1-staged-rollout/` + `00-master/threat_model.md`.
5. **R08 staged-rollout engine** — fold addition's decision-engine algorithm (dwell, auto-halt, deterministic bucketing) into `1.0.1-staged-rollout/` as the design seed.
6. **R17 one-active-per-group DB index** when pgx lands.

**Doc/phase work (route, don't build now):**
7. R19–R24 (user/group/api-key CRUD, WebSocket protocol, dashboard design, SDK+gomobile doc) → respective phase docs.
8. R32 — mine D7 (DATABASE_SCHEMA) for partitioning/index/audit detail when authoring the pgx implementation; reconcile enums to canonical wire strings.
9. Future-phase docs D14–D18 → versioned dirs per §6 (mind the 1.0.1 numbering divergence: staged-rollout stays 1.0.1).

**Reject (conflict with locked/built):** chi (X1), RSA-4096 swap (X2), mTLS-only MVP transport (X3), `vasic-digital`/`dev.helix.ota.*` module paths (X4), `updates`/`rollouts` MVP vocabulary (X5), Vault-first secrets (X7), exposed-UUID PKs (X8). Reuse their surrounding design ideas where additive and justified.

*End of addition #3 analysis.*
