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

## 2. Pre-Handoff Automated Gates (ALL MUST BE GREEN)

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

## 3. Release Candidate Artifacts

| Artifact | Path / SHA | Verified |
|---|---|---|
| Server binary | `server/ota-server` | [ ] |
| Tag | `helix_ota-<version>` | [ ] |
| Dashboard build | `dashboard/dist/` | [ ] |
| Deploy compose | `server/deploy/system.compose.yml` | [ ] |
| Changelog | `docs/changelogs/helix_ota-<version>.md` | [ ] |

---

## 4. Manual QA Test Scenarios

### 4.1 API Endpoints

- [ ] **GET /health** — returns 200 with service status
- [ ] **POST /api/v1/login** — authenticates and returns JWT
- [ ] **GET /api/v1/projects** — lists projects (authenticated)
- [ ] **POST /api/v1/projects** — creates project (authenticated)
- [ ] **GET /api/v1/projects/:id/releases** — lists releases
- [ ] **POST /api/v1/projects/:id/releases** — creates release
- [ ] **GET /api/v1/projects/:id/rollouts** — lists rollouts
- [ ] **POST /api/v1/projects/:id/rollouts** — creates rollout
- [ ] **GET /api/v1/devices** — device fleet status
- [ ] **GET /api/v1/telemetry** — telemetry endpoint
- [ ] **POST /api/v1/artifact/upload** — artifact upload (multipart)

### 4.2 Dashboard UI

- [ ] Login page renders correctly (light + dark theme)
- [ ] Dashboard loads project list after authentication
- [ ] Project detail page shows releases and rollouts
- [ ] Device fleet grid populates with device data
- [ ] Theme toggle switches light/dark correctly
- [ ] Responsive layout works on tablet viewport width
- [ ] Error states display meaningful messages (not blank pages)

### 4.3 Transport

- [ ] HTTP/2 endpoint accessible
- [ ] HTTP/3 (QUIC) endpoint accessible
- [ ] Brotli compression active (check response Content-Encoding)
- [ ] CORS headers present and correct

### 4.4 Security

- [ ] Rate limiting active (429 after burst)
- [ ] Unauthenticated requests return 401
- [ ] Invalid JWTs rejected
- [ ] Security headers present (HSTS, CSP, X-Frame-Options, X-Content-Type-Options)
- [ ] SQL injection vectors blocked (parameterized queries)

### 4.5 Rollout Lifecycle

- [ ] Create rollout → device assignment verified
- [ ] Recall rollout → rollback confirmed
- [ ] Canary percentage → gradual device uptake
- [ ] Deployment status transitions correctly

---

## 5. Blockers / Findings

| # | Finding | Severity | Evidence | Status |
|---|---|---|---|---|
| — | — | — | — | — |

---

## 6. QA Team Sign-off

| Field | Value |
|---|---|
| QA Reviewer |  |
| Date |  |
| Verdict | [ ] APPROVED / [ ] REJECTED (see findings above) |
| Evidence path |  |
| Notes |  |

---

## 7. Post-QA Actions (agent, after QA approval)

1. Update `docs/Fixed.md` / `docs/Issues.md` with QA confirmation
2. Bump CONTINUATION.md §3 Active work state
3. Create final release tag if not already created
4. Push tag to all upstreams per §2.1
5. Archive QA evidence under `docs/qa/<run-id>/`
