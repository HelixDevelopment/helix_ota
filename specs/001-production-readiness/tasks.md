---

description: "Task list for Production Readiness Gap Closure — all 47 gaps across 7 phases, ~80-100 person-days, parallel multi-track execution"

---

# Tasks: Production Readiness — Gap Closure & Full Completion

**Input**: Design documents from `specs/001-production-readiness/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. All P1 stories can proceed in parallel after Foundation completes. P2/P3 stories depend on P1 completion.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize branch, audit current baseline, and prepare the gap-closure tracking infrastructure

- [x] T001 Create feature branch `feature/production-readiness` from latest `main`
- [x] T002 [P] Run and capture full baseline test suite: `go test ./... -count=1` in `server/`, `vitest run` in `dashboard/`, `npx playwright test` in `dashboard/e2e/` — store results in `qa-results/baseline-001/`
- [x] T003 [P] Rebuild CodeGraph index: `codegraph index` — capture node count in `docs/codegraph/Status.md`
- [x] T004 [P] Run constitution inheritance gate: `bash tests/test_constitution_inheritance.sh` — capture PASS evidence in `qa-results/baseline-001/constitution-gate.txt`
- [x] T005 Create gap-tracking spreadsheet at `docs/research/production_planning_20260726/gap_tracker.csv` with columns: Gap-ID, Phase, Description, Owner, Status, Evidence-Path, Closed-Date — seed with all 47 gaps from `docs/research/completion/report/2026.07.25.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database schema integrity, migration framework with rollback, and shared config defaults that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 [P] Fix SQL syntax error in `server/internal/store/schema_postgres.sql` — comma issue before `current_version TEXT` in `telemetry_events` table (G-17)
- [x] T007 [P] Add `Down()` methods to migration 001 (baseline) and migration 002 (accounts) in `server/internal/store/migrations.go` — implement safe rollback per gap G-19
- [x] T008 [P] Add `HELIX_MAX_INFLIGHT` default constant 1000 in `server/internal/config/config.go` — change from 0 (unlimited) to 1000, add doc comment (G-21)
- [x] T009 Create migration 003: add `stage_deadline TIMESTAMPTZ` and `last_evaluated_at TIMESTAMPTZ` columns to rollout table in `server/internal/store/migrations.go` (G-03)
- [x] T010 Create migration 004: add `status TEXT DEFAULT 'active'`, `suspended_at TIMESTAMPTZ`, `archived_at TIMESTAMPTZ` columns to accounts table (G-07)
- [x] T011 Create migration 005: create `branches` table (id UUID PK, project_id UUID FK, name TEXT, description TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, created_by UUID) (G-02)
- [x] T012 Create migration 006: create `webhooks` table (id UUID PK, project_id UUID FK, url TEXT, secret TEXT, events TEXT[], active BOOLEAN, last_success_at TIMESTAMPTZ, last_failure_at TIMESTAMPTZ, created_at TIMESTAMPTZ) (G-05)
- [x] T013 [P] Create migration 007: add `added_by UUID` and `added_at TIMESTAMPTZ` columns to `project_members` table (G-06)
- [x] T014 [P] Create migration 008: create `delta_metadata` table for delta update tracking (id UUID PK, artifact_id UUID FK, base_artifact_id UUID, delta_size BIGINT, algorithm TEXT, created_at TIMESTAMPTZ) (G-09, G-18)
- [x] T015 [P] Create migration 009: add `hardware_capabilities JSONB` column to `devices` table (G-18)
- [x] T016 Add `REVOKE ALL ON accounts, projects, project_members, devices, deployments FROM public;` then `GRANT SELECT, INSERT, UPDATE, DELETE ON ... TO app_user;` in migration 002 or new migration 010 — prep for RLS (G-20)
- [x] T017 Add RLS enable + policy creation for all tenant-scoped tables in migration 010: `ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;` + per-table `CREATE POLICY tenant_isolation ... USING (tenant_id = current_setting('app.tenant_id')::UUID);` (G-22)
- [x] T018 Set `app.tenant_id` session variable in connection pool middleware at query time — add to `server/internal/api/middleware.go` or store layer (G-22)
- [x] T019 Add `HELIX_ROLLOUT_POLL_INTERVAL` config parameter (default 60s) in `server/internal/config/config.go` for rollout auto-progress scheduler (G-03)
- [x] T020 [P] Document `HELIX_MAX_INFLIGHT` and `HELIX_ROLLOUT_POLL_INTERVAL` in `server/deploy/system.compose.yml` with commented defaults

**Checkpoint**: Foundation ready — all migrations deploy clean, schema parses, config defaults set. User story implementation can now begin in parallel. Capture checkpoint evidence in `qa-results/baseline-001/checkpoint-foundation.txt`.

---

## Phase 3: User Story 2 - Developer implements core OTA functionality (Priority: P1) 🎯 MVP

**Goal**: The control plane is functionally complete — ApplyPort actually writes and arms, rollouts auto-advance, all CRUD endpoints exist, branches are implemented, Android agent is wired.

**Independent Test**: `go test ./... -count=1 -v` in `server/` must PASS with all new handler tests GREEN, then `curl`-based integration test exercising every endpoint in sequence.

### Implementation for User Story 2

- [x] T0*21 [P] [US2] Implement `WriteAndArm` body in `server/internal/device/applyport.go` — replace SCAFFOLD stub with real write-to-inactive-slot + `fw_setenv` boot-target update + `boot_control` arm call (G-01)
- [x] T0*22 [P] [US2] Implement rollout auto-progress background goroutine in `server/internal/api/server.go` — starts on server init, polls every `HELIX_ROLLOUT_POLL_INTERVAL`, checks `auto_progress=true` AND `stage_deadline <= NOW()`, calls evaluation engine (G-03)
- [x] T0*23 [P] [US2] Create `server/internal/api/handlers_branches.go` — implement CreateBranch, ListBranches, GetBranch, UpdateBranch, DeleteBranch handlers matching the contract in `handlers_branches_test.go` (G-02)
- [x] T0*24 [P] [US2] Register branch routes in `server/internal/api/server.go` router setup (G-02)
- [x] T0*25 [P] [US2] Add `POST /admin/accounts` handler in `server/internal/api/handlers_accounts.go` — create new account with role assignment (G-07)
- [x] T0*26 [P] [US2] Add `PATCH /admin/accounts/:id` handler — update account name, email, role (G-07)
- [x] T0*27 [P] [US2] Add `DELETE /admin/accounts/:id` handler — soft-delete/archive account (G-07)
- [x] T0*28 [P] [US2] Add `POST /admin/accounts/:id/suspend` handler — set status=suspended, set suspended_at (G-07)
- [x] T0*29 [P] [US2] Add `POST /admin/accounts/:id/unsuspend` handler — set status=active, clear suspended_at (G-07)
- [x] T0*30 [P] [US2] Add `POST /projects/:id/members` handler in `server/internal/api/handlers_project.go` — validate user exists, add member with role, set added_by/added_at (G-06)
- [x] T0*31 [P] [US2] Add `PATCH /projects/:id/members/:userId` handler — update member role (G-06)
- [x] T0*32 [P] [US2] Add `POST /admin/accounts/:id/archive` handler — set status=archived, validate no active deployments (G-07)
- [x] T0*33 [P] [US2] Add `POST /deltas/generate` handler in `server/internal/api/handlers_delta.go` — compute delta between two artifact versions and register in delta_metadata table (G-09)
- [x] T0*34 [P] [US2] Mount fabric registry HTTP routes in `server/internal/api/server.go` — expose fabric CRUD endpoints from `server/internal/fabric/registry.go` via Gin router (G-10)
- [x] T0*35 [P] [US2] Wire `SetAccountMembership` store method to HTTP endpoint — add handler in `server/internal/api/handlers_accounts.go` or `server/internal/api/users.go` that calls the existing store method (G-08)
- [x] T0*36 [P] [US2] Wire `submodules/ota-android-agent` and `submodules/ota-update-engine-bridge` as Gradle dependencies in `clients/ota-manager/settings.gradle.kts` — include the Kotlin/KMP submodule projects at the Android agent build level (G-12, G-39, G-40). NOTE: these are Gradle/Kotlin modules, NOT Go modules — they cannot be added to `server/go.mod`.
- [x] T0*37 [P] [US2] Add `fw_env.config` round-trip verification test in `server/internal/device/applyport_test.go` — write fixture fw_env, read back, assert key=value round-trips (G-04)
- [x] T0*38 [US2] Validate all new endpoints with `curl` integration test sequence — create via `POST /admin/accounts`, list via `GET`, update via `PATCH`, suspend, unsuspend, archive, add project member, list members, update role, remove member

**Checkpoint**: Core OTA control plane is functionally complete. Every new endpoint has at least one test. The `handlers_branches_test.go` tests pass against real handlers.

---

## Phase 4: User Story 3 - Security engineer validates system hardening (Priority: P1)

**Goal**: Security posture hardened — RLS active, rate limiting defaults safe, session invalidation works, tamper detection wired, security submodule integrated, credential rotation possible, vulnerability disclosure policy published.

**Independent Test**: Multi-tenant isolation test (tenant A cannot see tenant B's data), rate-limit test (150 concurrent → 429 for 50), session invalidation test (role change → old token denied), tamper test (bad hash → reject + alert).

### Implementation for User Story 3

- [x] T0*39 [P] [US3] Wire SECURITY_TAMPER_DETECTED event in `server/internal/api/handlers_artifact.go` — when hash/signature validation fails, log event at SECURITY severity to audit log. Write event payload in CloudEvents format for future webhook delivery. (NOTE: webhook dispatch notification deferred until T049 webhook engine is complete in Phase 5; this task creates the event emission seam that T050 wires into the dispatch engine.) (G-71)
- [x] T0*40 [P] [US3] Implement session invalidation on role change in `server/internal/api/middleware.go` — add role version counter to JWT claims, increment on role change, verify at each authenticated request (G-25)
- [x] T0*41 [P] [US3] Vendor `submodules/security/` into `server/go.mod` — add `replace` directive pointing to `../submodules/security`, pin tag (G-23, G-38)
- [x] T0*42 [US3] Wire security submodule middleware in `server/internal/api/server.go` — add PII detection middleware, security headers middleware (X-Content-Type-Options, X-Frame-Options, Strict-Transport-Security, Content-Security-Policy) (G-23)
- [x] T0*43 [P] [US3] Add `HELIX_ARTIFACT_SIGNING_KEY_ROTATION_INTERVAL` config parameter and rotation mechanism — allow operator to replace signing key without downtime in `server/internal/config/config.go` + `server/internal/api/token.go` (G-29)
- [x] T0*44 [P] [US3] Create `SECURITY.md` at repo root (vulnerability disclosure policy, PGP key, contact email) (G-27)
- [x] T0*45 [P] [US3] Create `.well-known/security.txt` served at known security URL (G-27)
- [x] T0*46 [P] [US3] Add Kotlin TUF client library dependency in `clients/ota-manager/` build — integrate a Kotlin-native TUF library for client-side artifact signature verification and delegation chain validation during artifact download. NOTE: `go-tuf/v2` is Go-only and cannot be consumed by Kotlin. The server-side TUF verification is handled by T047. (G-26)
- [x] T0*47 [P] [US3] Add server-side TUF verification step to artifact validation pipeline in `server/internal/api/handlers_artifact.go` — after hash/signature check, call TUF metadata verification to confirm the artifact's delegation chain before accepting upload (G-26). Depends on the TUF Go client library being vendored.
- [x] T0*48 [P] [US3] Review and tighten Content-Security-Policy header for SPA in `dashboard/` — audit current `default-src 'self'`, add specific directives for fonts, images, scripts, restrict to production origins (G-28)

**Checkpoint**: Security hardening complete — all security middleware active, RLS enforced at DB level, session invalidation working, tamper detection wired, TUF client verifying artifacts.

---

## Phase 5: User Story 1 - Operator deploys and manages OTA system in production (Priority: P1)

**Goal**: A production deployment is reliable and observable — rate limiting enabled, webhooks deliver events, backups automated, monitoring/alerting configured, compose files hardened with TLS defaults, graceful degradation on store failures.

**Independent Test**: Deploy full stack via `podman-compose -f system.compose.yml up -d`, then verify: health endpoint returns 200, rate-limit rejects excess with 429, webhook test event delivered, backup script creates a valid dump, Prometheus targets up, Grafana dashboard loads.

### Implementation for User Story 1

- [x] T0*49 [US1] Implement webhook dispatch engine in `server/internal/api/webhook.go` — HTTP POST to configured webhook URLs with CloudEvents-format JSON envelope, HMAC-SHA256 signature header, exponential backoff retry (max 3), last_success_at/last_failure_at tracking (G-05)
- [x] T0*50 [P] [US1] Wire webhook dispatch trigger points in event-emitting handlers — rollout stage change, deployment failure, rollback, health breach, security.tamper_detected (G-05)
- [x] T0*51 [P] [US1] Add `POST /api/v1/webhooks` handler — create webhook registration (G-05)
- [x] T0*52 [P] [US1] Add `GET /api/v1/webhooks` and `DELETE /api/v1/webhooks/:id` handlers — list and delete webhooks (G-05)
- [x] T0*53 [P] [US1] Add rate limiting middleware to artifact download path in `server/internal/api/rate_limit.go` — apply `HELIX_MAX_INFLIGHT` to `/api/v1/artifacts/:id/download` (G-24)
- [x] T0*54 [P] [US1] Add TLS configuration support in `server/deploy/system.compose.yml` — document expected cert paths, add example TLS env vars (G-65)
- [x] T0*55 [P] [US1] Create automated PostgreSQL backup script at `server/deploy/backup.sh` — `pg_dump -Fc` to MinIO/S3 bucket via `mc` or direct S3 API, add to `system.compose.yml` as a sidecar/cron (G-57)
- [x] T0*56 [P] [US1] Create restore procedure documentation in `server/deploy/restore.md` — step-by-step from backup to running database (G-57)
- [x] T0*57 [P] [US1] Add Prometheus alerting rules file at `server/deploy/prometheus/alerts.yml` — alerts for: rollout failure rate >0/5m, error rate >5%/5m, PostgreSQL down, inflight >80% of max (G-60)
- [x] T0*58 [P] [US1] Create pre-configured Grafana dashboard JSON at `server/deploy/grafana/ota-dashboard.json` — panels for: request rate, error rate, rollout progress, device counts, artifact sizes, deployment latency (G-60)
- [x] T0*59 [P] [US1] Add structured log shipping config in `server/deploy/promtail.yml` or equivalent — ship logs to Loki endpoint for centralized log aggregation (G-61)
- [x] T0*60 [P] [US1] Implement graceful degradation path in `server/cmd/server/main.go` — when PostgreSQL connection fails at startup, log warning and start with in-memory store as fallback; add health endpoint degradation signal (G-72)
- [x] T0*61 [P] [US1] Extract hardcoded timeouts and limits from `server/internal/api/handlers_*.go` into `server/internal/config/config.go` — artifact upload timeout, rollout evaluation timeout, DB connection pool size (G-73)
- [x] T0*62 [P] [US1] Add `HELIX_MAX_INFLIGHT` env var reference in `server/deploy/system.compose.yml` under server service with comment explaining the default (G-64)

**Checkpoint**: Production operations baseline complete — deployable, observable, backed up, alerting wired, webhooks delivering.

---

## Phase 6: User Story 5 - Developer completes testing and quality baseline (Priority: P2)

**Goal**: Full constitutional test-type compliance — all 12 components have stress+chaos coverage, all gates have paired mutation tests, cross-ACL boundary tests exist, session management tests exist, API fuzz testing runs with zero crashes.

**Independent Test**: Run `go test -count=1 ./...` in `server/` and verify all new test suites PASS with captured evidence, run `bash tests/stress_chaos/run_all.sh` and verify all 12 components report PASS.

### Implementation for User Story 5

- [x] T0*63 [P] [US5] Add stress test scenarios for remaining 6 components (Android modules, containers, challenges, helixqa, dashboard, emulator) — follow established patterns from `tests/challenges/stress_chaos/scenarios/` for the 6 already-covered components (G-30)
- [x] T0*64 [P] [US5] Add chaos test scenarios for remaining 6 components — process-death, network-fault, input-corruption, resource-exhaustion, state-corruption per component (G-30)
- [x] T0*65 [P] [US5] Add paired §1.1 mutation tests for every pre-build gate in `tests/` — for each gate script, create a mutation entry that breaks the gate's assertion and asserts the gate reports FAIL (G-31)
- [x] T0*66 [P] [US5] Create cross-ACL boundary test suite in `server/tests/acl_boundary/` — multi-tenant scenarios where Tenant A attempts to read/write Tenant B's data via every API path (G-33)
- [x] T0*67 [P] [US5] Create session management test suite in `server/tests/session/` — test: role change invalidation, concurrent sessions, refresh token rotation, token expiry (G-34)
- [x] T0*68 [P] [US5] Add `Down()` migration rollback tests in `server/internal/store/migrations_test.go` — apply migration, roll back with Down(), apply again, assert state consistent (G-35)
- [x] T0*69 [P] [US5] Add rollout evaluation performance benchmark in `server/internal/rollout/benchmark_test.go` — measure p50/p95/p99 latency for 100/1000/10000 concurrent evaluations (G-36)
- [x] T0*70 [P] [US5] Add Go fuzz tests for all API handler input validation in `server/internal/api/fuzz/` — create fuzz targets for POST /artifacts, GET /devices/by-hardware/:hardwareId, POST /admin/accounts, POST /projects/:id/members (G-37)
- [x] T0*71 [P] [US5] Create host-rendered visual regression tests in `dashboard/hostrender/` — render every screen×state×{light,dark} theme variant to PNG, run pixelmatch diff against golden images, pass OCR/layout oracle asserting no overlap/clip/off-screen per §11.4.170 (G-32)
- [x] T0*72 [US5] Run all new test suites together via `go test -count=1 -fuzz=. -fuzztime=30s ./internal/api/fuzz/...` and `bash tests/stress_chaos/run_all.sh` — capture evidence in `qa-results/tests-quality-baseline/`

**Checkpoint**: Testing baseline complete — constitutional test-type coverage expanded, all gates mutation-tested, cross-ACL/session/fuzz test suites passing.

---

## Phase 7: User Story 6 - Documentation author validates exported documents and API spec (Priority: P3)

**Goal**: All documentation is accurate and production-ready — OpenAPI spec matches all server routes, exported documents pass visual validation, operational runbooks exist and are verified.

**Independent Test**: `redocly lint openapi.yaml` passes with zero errors, visual validation pipeline scripts exit 0 for every exported document, deployment runbook followed step-by-step on a fresh VM produces a working instance.

### Implementation for User Story 6

- [x] T0*73 [P] [US6] Rewrite `docs/research/main_specs/1.0.0-mvp/api/openapi.yaml` — dump all ~45 routes from `server/internal/api/server.go` route registrations, map each to OpenAPI path+method+request/response schema, run `redocly lint` until zero errors + zero warnings (G-48)
- [x] T0*74 [P] [US6] Create exported-document visual validation pipeline at `scripts/validate_document_exports.sh` — render each exported HTML/PDF/DOCX, convert to PNG via headless browser, run OCR, assert: no raw markup text, no overlap, no clipped content (G-19 per §11.4.168)
- [x] T0*75 [P] [US6] Wire document visual validation gate into `scripts/pre_build_verification.sh` — FAIL if any export fails visual validation (G-19 per §11.4.168)
- [x] T0*76 [P] [US6] Create deployment runbook at `server/deploy/deployment_runbook.md` — step-by-step from fresh Ubuntu 24.04 VM to running production instance: prerequisites, clone, config, compose up, verify health, upload artifact, create rollout, monitor (G-49)
- [x] T0*77 [P] [US6] Create disaster recovery document at `server/deploy/disaster_recovery.md` — full DB restore from backup, server rebuild from scratch, data validation after restore (G-50)
- [x] T0*78 [P] [US6] Create on-call runbook at `server/deploy/on_call_runbook.md` — alert response procedures for each alert type (rollout failure, error rate, PostgreSQL down, inflight) (G-51)
- [x] T0*79 [P] [US6] Create developer onboarding guide at `docs/guides/developer_onboarding.md` — new developer setup: prerequisites, clone, submodules, build, run tests, run server, common troubleshooting (G-55)
- [x] T0*80 [P] [US6] Create upgrade/rollback procedure at `server/deploy/upgrade_rollback.md` — server binary upgrade with DB migrations, rollback to previous version with migration Down(), verify after upgrade (G-56)
- [x] T0*81 [P] [US6] Create API changelog at `docs/research/main_specs/1.0.0-mvp/api/CHANGELOG.md` — list every API route with added/modified/deprecated status per version (G-53)
- [x] T0*82 [P] [US6] Add CodeGraph auto-rebuild hook at `scripts/git_hooks/post-merge` — detect `.go`/`.ts`/`.tsx`/`.kt` file changes and trigger `codegraph index` automatically (G-59)
- [x] T0*83 [P] [US6] Implement Docs Chain Phase 6 — create submodule distribution mechanism for the docs_chain engine: add `docs_chain/` as a registered git submodule at project root in `.gitmodules`, update `helix-deps.yaml` with the docs_chain dependency, and wire `scripts/export_docs.sh` to invoke docs_chain CLI for export verification (G-58)
- [x] T0*84 [US6] Verify all runbooks by following deployment runbook on a fresh VM — capture evidence screenshots and log output in `qa-results/runbook-verification/`

**Checkpoint**: Documentation complete — OpenAPI matches all routes, all exports pass visual validation, runbooks verified working, onboarding guide enables first-day productivity.

---

## Phase 8: User Story 4 - Project manager tracks production readiness progress (Priority: P2)

**Goal**: A living production readiness document with velocity tracking, ADR acceptance recorded, and a visible gap-closure dashboard.

**Independent Test**: Generate the production readiness report — verify it contains per-phase completion %. velocity metrics (items/week), estimated completion date, and risk areas with mitigations.

### Implementation for User Story 4

- [x] T0*85 [P] [US4] Create production readiness planning document at `docs/research/production_planning_20260726/ANALYSIS.md` — per-phase gap closure status, velocity metrics (items/week), estimated completion date from velocity, risk register with mitigations (G-50 per §11.4.172)
- [x] T0*86 [P] [US4] Formally accept all 5 ADRs (ADR-0001 through ADR-0005) — add `**Status**: Accepted` and `**Accepted**: 2026-07-26` to each ADR file in `docs/research/main_specs/research/adr/`, document rationale referencing the locked architecture decision (G-52, FR-021)
- [x] T0*87 [P] [US4] Regenerate `docs/RESUMPTION.md` with current session state — update git HEAD, in-flight work status, phase completion, device states, evidence paths (G-54)
- [x] T0*88 [P] [US4] Add velocity tracking mechanism — script at `scripts/track_velocity.sh` reads gap_tracker.csv, computes items/week completed, appends to velocity log in `docs/research/production_planning_20260726/velocity.tsv` (FR-020)
- [x] T0*89 [US4] Generate and review the production readiness report — verify all 47 gaps tracked, per-phase status accurate, velocity trend visible

**Checkpoint**: Production readiness tracking operational — stakeholders have data-driven visibility into completion progress.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Constitutional compliance gates, infrastructure hardening, duplicate cleanup, and final release verification.

- [x] T0*90 [P] Wire `CM-COVENANT-114-167-PROPAGATION` gate into `scripts/pre_build_verification.sh` (feature work-stream lifecycle)
- [x] T0*91 [P] Wire `CM-COVENANT-114-168-PROPAGATION` gate (exported-document visual validation — consumes T074/T075)
- [x] T0*92 [P] Wire `CM-COVENANT-114-169-PROPAGATION` gate (mandatory 13 test-type coverage)
- [x] T0*93 [P] Wire `CM-COVENANT-114-170-PROPAGATION` gate (host-rendered UI visual-proof mandate — verify dashboard hostrender tests exist and PASS)
- [x] T0*94 [P] Wire `CM-COVENANT-114-172-PROPAGATION` gate (production-readiness planning document exists and current)
- [x] T0*95 [P] Wire `CM-COVENANT-114-176-PROPAGATION` gate (multi-track work-division arbitration)
- [x] T0*96 [P] Wire `CM-COVENANT-114-184-PROPAGATION` gate (SonarQube CLI installed on PATH + install_check.sh GREEN)
- [x] T0*97 [P] Wire `CM-COVENANT-114-185-PROPAGATION` gate (manual QA gate workflow — create QA handoff checklist)
- [x] T0*98 [P] Wire `CM-COVENANT-114-186-PROPAGATION` gate (cross-document consistency gate — add doc-integrity validator check to pre-build)
- [x] T0*99 [P] Wire `CM-COVENANT-12-12-PROPAGATION` gate (RLIMIT_NPROC awareness — add thread-headroom check to pre-build)
- [x] T0*100 [P] Clean up duplicate LLMProvider submodule — consolidate `submodules/llm_provider/` and `submodules/LLMProvider/` into one canonical path (G-15)
- [x] T0*101 [P] Compile helixqa Go orchestrator in-tree — fix layout assumption from `helix-deps.yaml` and integrate orchestrator compilation into server build (G-47)
- [x] T0*102 [P] Document canary deployment strategy in `server/deploy/canary_strategy.md` — describe blue-green deploy, traffic shifting, and rollback triggers for server updates (G-63)
- [x] T0*103 [P] Audit and add code comments to complex logic in server handlers — cover `handlers_artifact.go`, `handlers_rollout.go`, `applyport.go`, `migrations.go`, `server.go` startup sequence (G-70)
- [x] T0*104 [P] Create defect-discovery protocol at `scripts/discovered_defect_workflow.sh` — when stress/chaos or any testing reveals a new defect: auto-log to `docs/Issues.md` with Status:Queued, Type:Bug, reference the test that found it, and file the reopening per §11.4.4 test-interrupt-on-discovery (covers spec edge cases section)
- [x] T0*105 [P] Run SonarQube install check: `bash constitution/scripts/sonarqube/sonarqube_install_check.sh` — capture GREEN evidence (G-65 per §11.4.184)
- [x] T0*106 [P] Run full constitution sweep: `bash tests/pre_build_verification.sh` + `bash tests/test_constitution_inheritance.sh` — capture all gates GREEN evidence
- [x] T0*107 [P] Run full meta-test mutation sweep — for every gate, assert its paired §1.1 mutation produces FAIL, then restores to PASS
- [x] T0*108 Run full-suite retest per §11.4.40 — `go test ./...` in `server/`, `vitest run` in `dashboard/`, `npx playwright test` in `dashboard/e2e/`, stress+chaos all 12 components, Challenges bank, HelixQA — all GREEN with captured evidence in `qa-results/final-suite-001/`
- [x] T0*109 Create 1.0.0 release tag per §11.4.151: `git tag -a helix_ota-1.0.0 -m "Production-ready 1.0.0 release"` — push tag to ALL upstream remotes per §2.1 (FR-023, SC-006)
- [x] T0*110 Update `docs/changelogs/helix_ota-1.0.0.md` with version changelog per §5 — user-visible changes, developer changes, known caveats
- [x] T0*111 [P] Update all five governance-carrier files (CLAUDE.md, AGENTS.md, QWEN.md, GEMINI.md, GEMINI.md) at project root — update Fixed/Status sections with post-gap-closure state, all 47 gaps closed, 1.0.0 release metadata, evidence paths, and new constitutional compliance state per §11.4.157 five-carrier lockstep

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS ALL user stories (DB schema must be correct)
- **US2 Core Functionality (Phase 3)**: Depends on Foundation — can run in parallel with US3 (US1) + US3 (US1)
- **US3 Security Hardening (Phase 4)**: Depends on Foundation — parallel with US2 + US1 after Foundation
- **US1 Production Operations (Phase 5)**: Depends on Foundation — parallel with US2 + US3 after Foundation
- **US5 Testing Baseline (Phase 6)**: Depends on US2 + US3 + US1 completion (tests exercise real endpoints and security)
- **US6 Documentation (Phase 7)**: Depends on US2 completion (OpenAPI must document the real endpoints)
- **US4 Readiness Tracking (Phase 8)**: Can start after Foundation — runs in parallel with all other phases
- **Polish (Phase 9)**: Depends on ALL user story phases completion

### User Story Dependencies

- **User Story 2 (P1)**: Can start after Foundation — No dependencies on other stories
- **User Story 3 (P1)**: Can start after Foundation — No dependencies on other stories
- **User Story 1 (P1)**: Can start after Foundation — No dependencies on other stories
- **User Story 5 (P2)**: Depends on US2 + US3 + US1 (tests need the hardened, running system)
- **User Story 6 (P3)**: Depends on US2 (OpenAPI needs the implemented endpoints)
- **User Story 4 (P2)**: Can start after Foundation — independent tracking

### Parallel Execution Model

Per the parallel multi-track execution strategy (spec clarification):

```
Track A (Main):     Foundation → US2 Core + US4 Tracking → Polish
Track B (Parallel): Foundation → US3 Security            → Polish
Track C (Parallel): Foundation → US1 Operations          → Polish
Track D (Later):    ← depends on A+B+C — US5 Testing + US6 Documentation → Polish
```

### Within Each User Story

- Models before services before endpoints
- Core implementation before integration
- Story complete before moving to next phase

### Parallel Opportunities

- All Phase 1 Setup tasks marked [P] can run in parallel
- All Phase 2 Foundational tasks marked [P] can run in parallel (migrations are additive)
- All Phase 3-5 (US2, US3, US1) tasks marked [P] can run in parallel within their phase
- Tasks in different user stories can run on different tracks simultaneously
- **⚠️ `server/internal/api/server.go` is modified by T022 (US2), T024 (US2), T034 (US2), T042 (US3), and T060 (US1) — these edits MUST be sequenced even across parallel tracks.** Designate one track as server.go owner; merge other tracks' route registrations into that track's version.
- All Phase 9 Polish tasks marked [P] can run in parallel

---

## Parallel Example: User Story 2 - Core Functionality

```bash
# Launch all independent implementation tasks in parallel:
Task: "Implement WriteAndArm body in server/internal/device/applyport.go"
Task: "Create server/internal/api/handlers_branches.go"
Task: "Add POST /admin/accounts handler in server/internal/api/handlers_accounts.go"
Task: "Add POST /projects/:id/members handler in server/internal/api/handlers_project.go"
Task: "Wire Android agent submodules into clients/ota-manager/ Gradle build"
Task: "Add POST /deltas/generate handler in server/internal/api/handlers_delta.go"
Task: "Mount fabric registry HTTP routes in server/internal/api/server.go"
```

---

## Implementation Strategy

### MVP First (User Stories 2 + 3 + 1 only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phases 3 + 4 + 5 in parallel: US2 + US3 + US1
4. **STOP and VALIDATE**: Run full-suite retest, verify all P1 stories complete
5. Proceed to P2/P3 stories (Phases 6 + 7 + 8)

### Incremental Delivery

1. Complete Setup + Foundation → Foundation ready
2. Add US2 Core + US3 Security + US1 Operations in parallel → Deploy/Demo (Production-Capable MVP!)
3. Add US5 Testing → Quality baseline complete
4. Add US6 Documentation → Documentation complete
5. Add US4 Tracking → Readiness visibility
6. Polish → Release!

### Parallel Track Strategy

With 3 tracks:

1. Tracks complete Foundation together (serial — DB schema must be applied once)
2. Once Foundation is done:
   - Track A: US2 Core Functionality + US4 Tracking
   - Track B: US3 Security Hardening
   - Track C: US1 Production Operations
3. Phase 6 + 7 + 8 proceed in parallel after their dependencies are met
4. Phase 9 Polish: All tracks come together for final verification
