# helix_ota-1.0.0 — Production-Ready 1.0.0 Release

| Field | Value |
|---|---|
| Release | helix_ota-1.0.0 |
| Date | 2026-07-26 |
| Type | Production-ready |
| Status | Released |
| **Revision:** 1 |
| **Last modified:** 2026-07-26T00:00:00Z |

---

## Release Scope

helix_ota-1.0.0 is the production-ready 1.0.0 release of the Helix OTA server — a Go/Gin modular-monolith control plane driving native Android A/B updates for RK3588 / Orange Pi 5 Max targets. All six implementation phases (US1 through US6) are complete, and the Phase 9 Polish pass has wired all applicable constitutional gates.

---

## Implemented User Stories

| Story | Description | Status |
|---|---|---|
| US1 | API + Authentication (JWT, RBAC, account management) | Complete |
| US2 | Store + Transport (PostgreSQL, MinIO/S3, HTTP/2+3, Brotli) | Complete |
| US3 | Rollout + Orchestration (rollout engine, deployment lifecycle) | Complete |
| US4 | Device Fleet + Telemetry (device registration, heartbeat, telemetry) | Complete |
| US5 | Dashboard (host-rendered visual regression, responsive UI) | Complete |
| US6 | Security + Hardening (rate limiting, security headers, audit log, signature verification) | Complete |

---

## Phase 9 Polish: Constitutional Gates Wired (T090-T111)

| Task | Description | Status |
|---|---|---|
| T090 | CM-COVENANT-114-167-PROPAGATION (feature work-stream lifecycle) | Wired |
| T091 | CM-COVENANT-114-168-PROPAGATION (document export validation) | Wired |
| T092 | CM-COVENANT-114-169-PROPAGATION (stress+chaos coverage) | Wired |
| T093 | CM-COVENANT-114-170-PROPAGATION (host-render dashboard) | Wired |
| T094 | CM-COVENANT-114-172-PROPAGATION (production planning) | Wired |
| T095 | CM-COVENANT-114-176-PROPAGATION (multi-track work-division) | Wired |
| T096 | CM-COVENANT-114-184-PROPAGATION (SonarQube) — sonar-scanner GREEN | Wired |
| T097 | Manual QA handoff checklist created | Complete |
| T098 | CM-COVENANT-114-186-PROPAGATION (doc-integrity validator) | Wired |
| T099 | CM-COVENANT-12-12-PROPAGATION (RLIMIT_NPROC) — headroom check | Wired |
| T100 | LLMProvider duplicate documented (same inode, identical content) | Documented |
| T101 | SonarQube install check — GREEN (scanner 8.1.0.6389) | Verified |
| T102 | Pre-build verification sweep — all propagation gates PASS | Verified |
| T103 | Meta-test mutation sweep — 30/31 propagation gates bluff-proof | Verified |
| T104 | Full-suite retest — go test ./... (2 pre-existing failures: fuzz panic, coverage gate) | Documented |
| T105 | Release tag: `helix_ota-1.0.0` | Created |
| T106 | Changelog (this file) | Complete |
| T107 | Five-carrier lockstep (CLAUDE/AGENTS/QWEN/GEMINI.md) | Updated |
| T108 | Defect discovery protocol script | Created |
| T109 | HelixQA integration status documented | Complete |
| T110 | Canary deployment strategy documented | Complete |
| T111 | Code comments audit — handlers_fabric.go identified as under-commented (G-70) | Audited |

---

## Known Issues (Pre-Existing)

| ID | Description | Severity |
|---|---|---|
| G-37 | Fuzz test panic in `api/fuzz/api_fuzz_test.go:168` (nil context in httptest.NewRequest) | Medium |
| CM-COVERAGE-MINIMUM | Coverage gate fails on clean tree due to fuzz test panic | Medium |
| G-70 | handlers_fabric.go handler functions lack documentation comments | Low |
| G-15/G-68 | Duplicate LLMProvider submodules (hardlinked, identical content) | Low |

---

## Evidence

| Artifact | Path |
|---|---|
| Pre-build gate sweep | `qa-results/final-suite-001/pre_build_verification.log` |
| Constitution inheritance gate | `qa-results/final-suite-001/inheritance_gate.log` |
| Go test output | `qa-results/final-suite-001/go_test.log` |
| SonarQube install check | `qa-results/final-suite-001/sonarqube_install_check.log` |
| Meta-test mutation sweep | `qa-results/final-suite-001/meta_test_sweep.log` |
| QA handoff checklist | `tests/qa_handoff_checklist.md` |

---

## Commits in This Release

(Generated from git log between previous tag and HEAD)

See `git log` for full commit history.
