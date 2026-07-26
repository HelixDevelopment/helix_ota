# Manual QA Handoff Checklist — Helix OTA

| Field | Value |
|---|---|
| Document | QA Handoff Checklist |
| Created | 2026-07-26 |
| Last modified | 2026-07-26 |
| Status | Active |
| Compliance | §11.4.185 — Manual QA final confirmation mandate |
| **Revision:** 1 |
| **Last modified:** 2026-07-26T00:00:00Z |

---

## 1. Purpose

Per §11.4.185, no scope of work / release is fully completed until the project's QA team provides MANUAL testing confirmation as the FINAL step. Every automated gate is necessary but NOT sufficient. This checklist is the handoff document the agent produces when handing off a release candidate to the manual QA team.

---

## 2. QA Handoff Workflow

```
[Dev complete] --> [Automated gates ALL GREEN] --> [Agent fills in RC info below]
        --> [Agent hands off checklist to QA team] --> [QA team executes §4–§8]
        --> [QA team records findings in §9] --> [QA signs off in §10]
        --> [Agent performs post-QA actions (§11)] --> [Release tagged + pushed]
```

The QA handoff is a one-way handshake: the agent prepares the release-candidate
artifacts and the checklist, then WAITS for the QA team to complete §4 through
§10. The agent NEVER self-certifies the manual step (§11.4.185).

---

## 3. Pre-Handoff Automated Gates (ALL MUST BE GREEN)

- [ ] `bash tests/pre_build_verification.sh` — all propagation + substantive gates PASS
- [ ] `bash tests/test_constitution_inheritance.sh` — inheritance gate PASS
- [ ] `go test ./...` in `server/` — all unit/integration/e2e tests GREEN
- [ ] `bash tests/meta/run_all.sh` — all meta-test paired mutations PASS
- [ ] `bash tests/stress_chaos/run_server_stress.sh` — stress+chaos suite GREEN
- [ ] `bash tests/chaos/chaos_live.sh` — chaos injection suite GREEN
- [ ] `bash tests/stress/http_load_live.sh` — load test suite GREEN
- [ ] Dashboard hostrender visual regression tests GREEN (`npx playwright test --config=playwright.hostrender.config.ts` in `dashboard/`)
- [ ] Document export validation PASS (`bash scripts/validate_document_exports.sh docs/`)
- [ ] All Markdown exports current (HTML+PDF siblings non-stale)
- [ ] `git status` — working tree clean, nothing uncommitted

---

## 4. Release Candidate Artifacts

| Artifact | Path / SHA | Verified |
|---|---|---|
| Server binary | `server/ota-server` | [ ] |
| Tag | `helix_ota-<version>` | [ ] |
| Dashboard build | `dashboard/dist/` | [ ] |
| Deploy compose | `server/deploy/system.compose.yml` | [ ] |
| Changelog | `docs/changelogs/helix_ota-<version>.md` | [ ] |

---

## 5. Account & Authentication (CRUD)

### 5.1 Login

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 1 | POST /api/v1/auth/login with valid admin credentials | 200 + JWT in response body | [ ] | [ ] | | |
| 2 | POST /api/v1/auth/login with invalid password | 401 Unauthorized | [ ] | [ ] | | |
| 3 | POST /api/v1/auth/login with empty body | 400 / validation error | [ ] | [ ] | | |

### 5.2 Account Context (multi-tenant)

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 4 | GET /api/v1/account/projects with admin JWT | 200 + lists all projects visible to account | [ ] | [ ] | | |
| 5 | GET /api/v1/account/projects with viewer JWT | 200 + lists only projects viewer has access to | [ ] | [ ] | | |
| 6 | GET /api/v1/account/projects with no JWT | 401 | [ ] | [ ] | | |

### 5.3 Project CRUD

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 7 | POST /api/v1/projects with valid name + description | 201 Created + project ID | [ ] | [ ] | | |
| 8 | GET /api/v1/projects | 200 + list includes newly created project | [ ] | [ ] | | |
| 9 | GET /api/v1/projects/:projectId | 200 + project details | [ ] | [ ] | | |
| 10 | PATCH /api/v1/projects/:projectId (admin only) | 200 + updated fields | [ ] | [ ] | | |
| 11 | DELETE /api/v1/projects/:projectId (admin only) | 204 No Content | [ ] | [ ] | | |
| 12 | GET deleted project | 404 Not Found | [ ] | [ ] | | |

### 5.4 Project Members (RBAC)

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 13 | GET /api/v1/projects/:projectId/members | 200 + member list | [ ] | [ ] | | |
| 14 | POST /api/v1/projects/:projectId/members with new user+role | 201 + member added | [ ] | [ ] | | |
| 15 | PATCH /api/v1/projects/:projectId/members/:userId to change role | 200 + role updated | [ ] | [ ] | | |
| 16 | Verify old role token no longer works at new-role-scoped endpoint | 403 Forbidden | [ ] | [ ] | | |
| 17 | DELETE /api/v1/projects/:projectId/members/:userId | 204 + member removed | [ ] | [ ] | | |

---

## 6. Deployment Lifecycle

### 6.1 Release Management

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 18 | POST /api/v1/projects/:projectId/releases (valid artifact + version) | 201 + release created | [ ] | [ ] | | |
| 19 | POST duplicate version | 409 Conflict | [ ] | [ ] | | |
| 20 | GET /api/v1/projects/:projectId/releases | 200 + paginated list | [ ] | [ ] | | |
| 21 | GET /api/v1/projects/:projectId/releases/:releaseId | 200 + release details | [ ] | [ ] | | |

### 6.2 Deployment CRUD

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 22 | POST /api/v1/projects/:projectId/deployments (releaseId + target group) | 201 + deployment created | [ ] | [ ] | | |
| 23 | GET /api/v1/projects/:projectId/deployments | 200 + list includes new deployment | [ ] | [ ] | | |
| 24 | GET /api/v1/projects/:projectId/deployments/:deploymentId | 200 + progress stats populated | [ ] | [ ] | | |

### 6.3 Rollout Phases

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 25 | POST /api/v1/deployments/:deploymentId/rollout (canary 10%) | 200 + rollout created, phase 0 active | [ ] | [ ] | | |
| 26 | GET rollout status after healthy device telemetry | phase advances automatically | [ ] | [ ] | | |
| 27 | POST recall on active deployment | 200 + deployment recalled, devices revert | [ ] | [ ] | | |
| 28 | GET /api/v1/deployments/:deploymentId/rollbacks | 200 + rollback history | [ ] | [ ] | | |
| 29 | Verify recalled devices target previous version | 200 + device target_version matches old release | [ ] | [ ] | | |

---

## 7. Webhook Dispatch

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 30 | POST /api/v1/projects/:projectId/webhooks (valid URL + events) | 201 + webhook registered | [ ] | [ ] | | |
| 31 | GET /api/v1/projects/:projectId/webhooks | 200 + list includes new webhook | [ ] | [ ] | | |
| 32 | Trigger an event that matches webhook's event list (e.g. create deployment) | webhook fires: POST to registered URL with HMAC-SHA256 signature + CloudEvents envelope | [ ] | [ ] | | |
| 33 | Verify HMAC signature header (X-OTA-Signature) is present and valid | computed HMAC matches webhook secret | [ ] | [ ] | | |
| 34 | Trigger webhook against an endpoint that returns non-2xx | exponential backoff retry (1s → 4s → 16s); last_failure_at updated | [ ] | [ ] | | |
| 35 | DELETE /api/v1/projects/:projectId/webhooks/:id | 204 + webhook removed, no further dispatch | [ ] | [ ] | | |
| 36 | Trigger the event again after deletion | no webhook fires | [ ] | [ ] | | |

---

## 8. Backup & Restore (PostgreSQL Persistence)

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 37 | Start server with HELIX_DATABASE_URL pointing to a PostgreSQL instance | server boot logs "persistence = PostgreSQL (pgx)" | [ ] | [ ] | | |
| 38 | Verify all schema migrations run on first boot (tables created) | helix_ota schema with accounts, projects, releases, deployments, rollouts, devices, telemetry, webhooks, audit tables present | [ ] | [ ] | | |
| 39 | Create data (project + release + deployment) with PostgreSQL active | data persisted | [ ] | [ ] | | |
| 40 | Restart server (same DATABASE_URL) | all previously created data survives restart | [ ] | [ ] | | |
| 41 | Remove DATABASE_URL and restart | server falls back to in-memory store, logs "degraded" | [ ] | [ ] | | |
| 42 | Restore DATABASE_URL and restart | server detects reachable PostgreSQL, reconnects and data intact | [ ] | [ ] | | |
| 43 | Start server with unreachable DATABASE_URL | server retries 60s, then falls back to in-memory with degraded health (WARNING log entry) | [ ] | [ ] | | |

---

## 9. Monitoring & Observability

### 9.1 Health Endpoints

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 44 | GET /healthz | 200 + JSON status (reports liveness) | [ ] | [ ] | | |
| 45 | GET /readyz | 200 when store is healthy; 503 when store unreachable (postgres down, in-memory degraded) | [ ] | [ ] | | |
| 46 | Verify /healthz returns degraded=true when store is degraded | JSON field `degraded: true` | [ ] | [ ] | | |

### 9.2 Telemetry & Metrics

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 47 | GET /api/v1/telemetry/overview | 200 + aggregate counts by event type | [ ] | [ ] | | |
| 48 | GET /api/v1/devices/:deviceId/telemetry | 200 + device-specific event history with duration_ms and bytes_transferred | [ ] | [ ] | | |
| 49 | POST telemetry from a registered device | 202 + accepted/rejected counts | [ ] | [ ] | | |
| 50 | Verify telemetry events are reflected in overview | aggregation matches submitted events | [ ] | [ ] | | |

### 9.3 Audit Log

| Step | Action | Expected Result | Pass | Fail | Date | Tester |
|------|--------|-----------------|------|------|------|--------|
| 51 | Perform several mutation operations (create project, deploy, rollback) | operations visible in audit log | [ ] | [ ] | | |
| 52 | GET /api/v1/audit (admin JWT) | 200 + operations with timestamps, actor, action, entity | [ ] | [ ] | | |
| 53 | GET /api/v1/audit with viewer JWT | 403 Forbidden (admin-only) | [ ] | [ ] | | |

---

## 10. API Endpoints (Baseline)

### 10.1 Core Endpoints

- [ ] **GET /healthz** — returns 200 with service status
- [ ] **POST /api/v1/auth/login** — authenticates and returns JWT
- [ ] **GET /api/v1/projects** — lists projects (authenticated)
- [ ] **POST /api/v1/projects** — creates project (authenticated)
- [ ] **GET /api/v1/projects/:id/releases** — lists releases
- [ ] **POST /api/v1/projects/:id/releases** — creates release
- [ ] **GET /api/v1/projects/:id/rollouts** — lists rollouts
- [ ] **POST /api/v1/projects/:id/rollouts** — creates rollout
- [ ] **GET /api/v1/devices** — device fleet status
- [ ] **GET /api/v1/telemetry** — telemetry endpoint
- [ ] **POST /api/v1/artifact/upload** — artifact upload (multipart)

### 10.2 Dashboard UI

- [ ] Login page renders correctly (light + dark theme)
- [ ] Dashboard loads project list after authentication
- [ ] Project detail page shows releases and rollouts
- [ ] Device fleet grid populates with device data
- [ ] Theme toggle switches light/dark correctly
- [ ] Responsive layout works on tablet viewport width
- [ ] Error states display meaningful messages (not blank pages)

### 10.3 Transport

- [ ] HTTP/2 endpoint accessible
- [ ] HTTP/3 (QUIC) endpoint accessible
- [ ] Brotli compression active (check response Content-Encoding)
- [ ] CORS headers present and correct

### 10.4 Security

- [ ] Rate limiting active (429 after burst)
- [ ] Unauthenticated requests return 401
- [ ] Invalid JWTs rejected
- [ ] Security headers present (HSTS, CSP, X-Frame-Options, X-Content-Type-Options)
- [ ] SQL injection vectors blocked (parameterized queries)

### 10.5 Rollout Lifecycle

- [ ] Create rollout → device assignment verified
- [ ] Recall rollout → rollback confirmed
- [ ] Canary percentage → gradual device uptake
- [ ] Deployment status transitions correctly

---

## 11. Blockers / Findings

| # | Finding | Severity | Evidence | Status |
|---|---|---|---|---|
| — | — | — | — | — |

---

## 12. QA Team Sign-off

| Field | Value |
|---|---|
| QA Reviewer |  |
| Date |  |
| Verdict | [ ] APPROVED / [ ] REJECTED (see findings above) |
| Evidence path |  |
| Notes |  |

---

## 13. Post-QA Actions (agent, after QA approval)

1. Update `docs/Fixed.md` / `docs/Issues.md` with QA confirmation
2. Bump CONTINUATION.md §3 Active work state
3. Create final release tag if not already created
4. Push tag to all upstreams per §2.1
5. Archive QA evidence under `docs/qa/<run-id>/`
