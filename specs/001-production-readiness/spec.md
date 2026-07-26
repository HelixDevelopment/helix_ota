# Feature Specification: Production Readiness — Gap Closure & Full Completion

**Feature Directory**: `specs/001-production-readiness`

**Created**: 2026-07-26

**Status**: Draft

**Input**: User description: "We MUST finish everything from reports and materials from docs/research/completion/report and bring project to full production ready state!"

## Clarifications

### Session 2026-07-26

- Q: 1.0.0 release gate — which gaps must close for 1.0.0? → A: Option C — all 47 gaps across all 7 phases must close before 1.0.0 ships.
- Q: ADR resolution approach — accept as-is or re-evaluate? → A: Option A — accept all 5 ADRs as-is with formal acceptance stamp and documented rationale.
- Q: Execution capacity strategy — single stream or parallel? → A: Option A — parallel multi-track: critical blockers + ADRs first, then high+medium implementation in parallel streams, then gates+testing+infra in a third wave; ~15-20 calendar days with 3 parallel streams.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator deploys and manages OTA system in production (Priority: P1)

An operations engineer deploys the Helix OTA server to a production environment, configures it for their fleet of devices, and manages day-to-day operations including artifact uploads, staged rollouts, rollbacks, and monitor fleet health—without encountering missing functionality, hardcoded defaults, or undocumented procedures.

**Why this priority**: The core purpose of the project is to serve production OTA updates. Without the production-readiness items resolved (alerting, webhooks, rate limiting, TLS, deployment runbook, backup strategy, monitoring stack), the system cannot be operated reliably in production. This story covers the infrastructure and operational baseline that every other story depends on.

**Independent Test**: Can be fully tested by deploying the full server stack (server + PostgreSQL + S3/MinIO) via the production compose configuration, performing a complete update lifecycle (upload artifact → create rollout → monitor → rollback), and verifying that all operational surfaces (health checks, metrics, webhooks, rate limiting, TLS, alerting, backup) function as expected.

**Acceptance Scenarios**:

1. **Given** a fresh production deployment of the Helix OTA server, **When** an operator visits the health endpoint, **Then** the server returns a healthy status and readiness check passes.
2. **Given** a production deployment with rate limiting configured, **When** an attacker sends more than the allowed concurrent requests, **Then** excess requests are gracefully rejected with a 429 status.
3. **Given** a production deployment with webhooks configured, **When** a rollout completes or fails, **Then** the configured webhook endpoint receives a notification with the deployment event payload.
4. **Given** a production deployment with automated backups configured, **When** a database failure occurs, **Then** the operator can restore from the most recent backup with zero data loss.
5. **Given** a production deployment with monitoring and alerting, **When** any key metric (error rate, rollout failure, disk usage) exceeds a threshold, **Then** the operator receives an alert through the configured channel.

---

### User Story 2 - Developer implements core OTA functionality (Priority: P1)

A developer implements the core ApplyPort `WriteAndArm` operation, the rollout auto-progress scheduler, the missing REST API endpoints (account CRUD, project member management, delta generation, fabric registry routes), and the Android agent integration—so the control plane is functionally complete for the 1.0.0 release.

**Why this priority**: Five critical implementation gaps directly block production use: the ApplyPort stub means no actual device-side write operation works, the missing auto-progress scheduler means all rollout evaluation is manual, the orphan handlers\_branches test indicates missing endpoints, the SQL syntax error prevents schema deployment, and missing account/project CRUD means fleet management is incomplete.

**Independent Test**: Can be fully tested by running the full automated server test suite (`go test ./...`) with the implementation changes, then deploying to a test environment and exercising each new endpoint and feature with real payloads, verifying correct behavior, error handling, and integration with the database.

**Acceptance Scenarios**:

1. **Given** a device with a pending update, **When** the ApplyPort `WriteAndArm` operation is triggered, **Then** the update artifact is correctly written to the device's inactive slot and armed for boot-time switching.
2. **Given** a rollout with `auto_progress` enabled and a stage duration of 1 hour, **When** the stage duration elapses, **Then** the rollout automatically progresses to the next stage without operator intervention.
3. **Given** an operator calls `POST /admin/accounts`, **When** they provide valid account data, **Then** a new account is created and returned, and the account appears in subsequent `GET /admin/accounts` responses.
4. **Given** an operator calls `POST /projects/:id/members`, **When** they provide a valid user ID and role, **Then** the user is added as a project member with the specified role.
5. **Given** the SQL schema is deployed to a fresh PostgreSQL database, **When** all migrations run, **Then** no syntax errors occur and all tables and constraints are created correctly.

---

### User Story 3 - Security engineer validates system hardening (Priority: P1)

A security engineer reviews and validates the OTA system's security posture—rate limiting, PostgreSQL Row-Level Security (RLS) for tenant isolation, session invalidation on role change, artifact tamper detection, credential rotation, security submodule integration, and vulnerability disclosure policy—ensuring the system meets production security standards.

**Why this priority**: Security gaps (no RLS, rate-limiting off by default, no session invalidation, tamper events not wired) represent real production risks including cross-tenant data exposure, DoS vulnerability, and undetected artifact tampering. These must be resolved before any production fleet deployment.

**Independent Test**: Can be fully tested by (1) running a multi-tenant scenario that attempts cross-tenant data access with RLS enabled—verifying each tenant can only see their own data, (2) sending excessive requests to verify rate limiting rejects at the configured threshold, (3) changing a user's role and confirming their existing session token no longer grants the previous permissions, and (4) uploading a tampered artifact and verifying the tamper event triggers the configured response.

**Acceptance Scenarios**:

1. **Given** PostgreSQL RLS is enabled for the accounts and projects tables, **When** User A (tenant 1) attempts to query User B's (tenant 2) data via SQL injection or an API vulnerability, **Then** RLS blocks the cross-tenant access and returns no data.
2. **Given** rate limiting is enabled with a max inflight of 1000 (production default), **When** 1500 concurrent requests are sent, **Then** the first 1000 proceed normally and the remaining 500 receive HTTP 429 responses.
3. **Given** an operator changes a user's role from `admin` to `viewer`, **When** that user's existing session token attempts an admin action, **Then** the action is denied and the token is considered invalid for elevated permissions.
4. **Given** a tampered artifact is uploaded, **When** the artifact validation pipeline detects the tamper (hash mismatch or signature failure), **Then** the upload is rejected, an audit event with `SECURITY_TAMPER_DETECTED` severity is logged, and the configured webhook/callback is triggered.
5. **Given** the security submodule is integrated, **When** the server starts, **Then** all security middleware (security headers, PII detection, encrypted storage helpers) is active and verifiable via endpoint responses.

---

### User Story 4 - Project manager tracks production readiness progress (Priority: P2)

A project manager or team lead reviews a production readiness plan and dashboard showing which gaps are resolved, which ADRs are accepted, which gates are wired, and the estimated timeline to full production readiness—enabling data-driven release decisions.

**Why this priority**: Without a living production readiness document with velocity-based timeline projections (per §11.4.172), release decisions are based on subjective judgment rather than measured progress. This story provides the visibility needed to commit to a release date.

**Independent Test**: Can be fully tested by generating the production readiness report and verifying that it includes: current gap closure percentage, per-phase completion status, measured velocity (items completed per week), estimated completion date based on velocity, and identified risk areas with mitigations.

**Acceptance Scenarios**:

1. **Given** a production readiness planning document exists, **When** any gap is closed, **Then** the document is updated with the new status, completion date, and revised timeline projection.
2. **Given** the planning document is updated, **When** viewed, **Then** it shows per-phase completion status (Critical/High/Medium/Gates/Testing/ADR/Infrastructure), velocity metrics (items per week), and estimated completion date.
3. **Given** the ADR review process is initiated, **When** all 5 outstanding ADRs are reviewed, **Then** each is formally accepted as-is with documented rationale confirming the already-locked architecture decision, and the decision is recorded in the ADR log.

---

### User Story 5 - Developer completes testing and quality baseline (Priority: P2)

A developer expands the test suite to cover the mandatory 13 test types per the HelixConstitution: stress+chaos for all 12 components, mutation testing for all gates, cross-ACL boundary tests, session management tests, and API fuzz testing—ensuring every component has anti-bluff positive-evidence coverage.

**Why this priority**: The project constitution (§11.4.169) mandates 13 closed-set test types. Currently, stress+chaos covers only 6/12 components, mutation testing exists only for the constitution inheritance gate, and cross-ACL/session/fuzz coverage is missing. Without this, the project is constitutionally non-compliant for a production release.

**Independent Test**: Can be fully tested by (1) running `go test -count=1 ./...` and verifying all tests pass with captured evidence output, (2) running the stress+chaos suite for each component and verifying categorised PASS results, (3) running the mutation test harness and verifying every gate's paired mutation produces FAIL when its assertion is broken, (4) running cross-ACL tests and verifying tenant isolation, and (5) running API fuzz tests and verifying no panics or crashes.

**Acceptance Scenarios**:

1. **Given** the full test suite is run, **When** all tests complete, **Then** every test type (unit, integration, e2e, full-automation, challenges, HelixQA, DDoS, security, stress+chaos, concurrency, race, memory, benchmarking) reports PASS with captured physical evidence per §11.4.69.
2. **Given** a gate's assertion is temporarily broken (mutation), **When** the pre-build verification runs, **Then** the gate reports FAIL, proving the gate is not a bluff (§1.1).
3. **Given** stress+chaos tests are executed for all 12 components, **When** the suite completes, **Then** each component has categorised stress (sustained load, concurrent contention, boundary) and chaos (process-death, network-fault, input-corruption, resource-exhaustion, state-corruption) results with recovery classification.

---

### User Story 6 - Documentation author validates exported documents and API spec (Priority: P3)

A technical writer or documentation maintainer validates that (a) the OpenAPI specification matches the actual server implementation, (b) every exported document (HTML/PDF/DOCX) is faithful, readable, and visually correct per §11.4.168, (c) the deployment runbook, disaster recovery procedure, and on-call runbook are complete, and (d) the developer onboarding guide enables a new developer to be productive within one day.

**Why this priority**: The OpenAPI spec documents 12 routes while the server implements ~45—a 73% documentation gap. Additionally, exported documents lack visual validation (§11.4.168), and there are no operational runbooks. Without this, operators cannot rely on the API documentation, and the project lacks critical operational documentation.

**Independent Test**: Can be fully tested by (1) running the OpenAPI spec validator against the live server and verifying all 45+ routes are documented with correct schemas, (2) generating each exported document and running the visual validation pipeline (render → OCR → verify), (3) following the deployment runbook step-by-step in a fresh environment and verifying the server is operational at the end, and (4) having a new developer follow the onboarding guide and measuring time-to-first-successful-build.

**Acceptance Scenarios**:

1. **Given** the OpenAPI specification is rewritten to cover all server routes, **When** `redocly lint` is run, **Then** the spec passes with zero errors and zero warnings.
2. **Given** an exported document (HTML or PDF or DOCX) is generated, **When** the visual validation pipeline runs, **Then** the document passes all three layers: content fidelity (no dropped data), textual correctness (no raw markup), and full visual integrity (diagrams render as images, layout intact, no overlap).
3. **Given** a new developer follows the onboarding guide, **When** they complete all setup steps, **Then** they can run the server tests and deploy the full stack within one working day.

---

### Edge Cases

- What happens when production readiness activities encounter a gap whose fix would take longer than the estimated timeline?
- How does the system handle a gap that is actually a design decision rather than an omission (e.g., no TLS in default compose)?
- What happens when multiple gaps are interdependent (e.g., RLS cannot be implemented without migration rollback support)?
- How does the project prioritize if total effort (~80-100 person-days) exceeds available capacity?
- What happens when an ADR is rejected after review, requiring a new solution design?
- How does the project handle gaps that require external dependencies (e.g., TUF client implementation)?
- What happens when stress+chaos testing reveals new defects in previously-stable components?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Operator MUST be able to deploy the Helix OTA server to production with rate limiting, TLS, and monitoring pre-configured.
- **FR-002**: Operator MUST receive automated alerts when any deployment failure, rollout phase change, or health metric breach occurs.
- **FR-003**: Operator MUST be able to restore the PostgreSQL database from an automated backup with zero data loss.
- **FR-004**: ApplyPort `WriteAndArm` MUST write the update artifact to the correct device slot and arm it for boot-time switching.
- **FR-005**: Rollout engine MUST automatically advance to the next stage when `auto_progress` is enabled and the stage duration elapses.
- **FR-006**: Server MUST expose complete account lifecycle management: create, read, update, delete, suspend, and archive accounts.
- **FR-007**: Server MUST expose complete project member lifecycle: add, list, update role, and remove members.
- **FR-008**: PostgreSQL schema MUST deploy without syntax errors across all migrations.
- **FR-009**: PostgreSQL Row-Level Security MUST be enabled for tenant isolation on all multi-tenant tables.
- **FR-010**: Rate limiting MUST be configurable and enabled by default with a sensible production default.
- **FR-011**: Session tokens MUST be invalidated when the user's role or permissions change.
- **FR-012**: Artifact tamper detection MUST trigger a security event, reject the artifact, and notify configured receivers.
- **FR-013**: Security submodule MUST be integrated providing security headers, PII detection, and encrypted storage helpers.
- **FR-014**: All gates with paired §1.1 mutations MUST have a mutation test proving the gate catches its negation.
- **FR-015**: Stress and chaos testing MUST cover all 12 project components with categorised results.
- **FR-016**: Cross-ACL boundary tests MUST verify that tenants cannot access each other's data through any API path.
- **FR-017**: API handler input validation MUST be fuzz-tested with zero crashes or panics.
- **FR-018**: OpenAPI specification MUST document all 45+ server routes with correct request/response schemas.
- **FR-019**: Every exported document MUST pass the three-layer visual validation pipeline: content, textual, and full visual integrity.
- **FR-020**: Production readiness planning document MUST exist with per-phase completion status, velocity metrics, and estimated completion date.
- **FR-021**: All 5 outstanding ADRs MUST be formally accepted as-is (documenting the already-locked architecture decisions) with documented rationale — no re-evaluation or re-design cycle is needed.
- **FR-022**: Deployment runbook, disaster recovery procedure, on-call runbook, and developer onboarding guide MUST be complete and verified.
- **FR-023**: All 47 identified gaps in the completion report MUST be resolved or explicitly deferred with documented rationale.

### Key Entities

- **Production Readiness Gap**: A single item from the completion report's 47-gap analysis, with priority, effort estimate, owner, status, and closure evidence.
- **ADR**: An Architecture Decision Record capturing a formal design decision with context, options, decision, consequences, and acceptance status.
- **Compliance Gate**: A constitutional gate (e.g., CM-COVENANT-114-168-PROPAGATION) that enforces a specific constitutional mandate and must be wired into the pre-build verification pipeline.
- **Export Document**: A project document (HTML, PDF, or DOCX) generated from Markdown source that must pass content, textual, and visual validation.
- **Mutation Test**: A paired §1.1 test that temporarily breaks a gate's assertion and verifies the gate reports FAIL, proving the gate is not a bluff.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 47 identified gaps in the completion report are closed or explicitly deferred with documented rationale within the planned timeline.
- **SC-002**: The full server test suite (`go test ./...`) and all component test suites (dashboard, emulator, submodules) pass with zero failures on a clean build.
- **SC-003**: A new operator can deploy a production-ready Helix OTA instance (server + PostgreSQL + MinIO + monitoring) in under 2 hours following the deployment runbook.
- **SC-004**: The OpenAPI specification documents 100% of server routes with zero redocly-lint errors and zero warnings.
- **SC-005**: All 12 project components have stress and chaos test coverage with categorised PASS results and captured evidence per §11.4.69.
- **SC-006**: The project ships its 1.0.0 release tag only after ALL 47 gaps across all 7 phases are closed, all constitutional gates wired and passing, and every gap closure verified with captured evidence per the HelixConstitution.
- **SC-007**: The production readiness planning document is updated at least monthly with velocity metrics and revised timeline projections per §11.4.172.

## Assumptions

- The completion report's 47-gap analysis is the authoritative gap inventory and its priority ordering (Phase 0→1→2→3→4→5→6) is the agreed execution sequence.
- Total effort to close all gaps is approximately 80-100 person-days as estimated by the report, subject to refinement during execution.
- Execution follows a parallel multi-track strategy: critical blockers + ADRs on the main track, high+medium implementation gaps in parallel sub-tracks, then gates+testing+infrastructure in a third wave — targeting ~15-20 calendar days with 3 parallel streams.
- The estimated ~80-100 person-days of work will be resourced with parallel execution capacity (existing team working across multiple tracks).
- ADRs proposed for 30+ days are accepted as-is (documenting already-locked architecture decisions); each ADR requires ~0.5 day for formal acceptance stamp and rationale, not a full re-evaluation.
- The ApplyPort `WriteAndArm` implementation follows the interfaces already defined in the codebase and targets the existing `apply_port` abstraction.
- Security submodule integration is additive (no breaking changes to existing API).
- Stress+chaos coverage expansion follows the established patterns from the 6 already-covered components.
- The current in-memory store remains as fallback; PostgreSQL is the production target.
- No new major architectural changes are introduced during gap closure (the architecture is locked per the master design).
- The TUF client implementation for device-side supply-chain hardening is in-scope for 1.0.0 and must be completed as part of Phase 1 gap closure (G-26), starting with the Android agent architecture spike then implementation.
- The multi-track ruler orchestration (§11.4.187) is a Phase 3 constitutional compliance item in-scope for 1.0.0; single-track workflow is NOT acceptable for the production deployment.
