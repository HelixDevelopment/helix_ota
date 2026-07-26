# HelixQA Integration Status — Helix OTA

| Field | Value |
|---|---|
| Document | HelixQA Integration Status |
| Created | 2026-07-26 |
| Last modified | 2026-07-26 |
| Status | Active |
| **Revision:** 1 |
| **Last modified:** 2026-07-26T00:00:00Z |

---

## 1. Integration Overview

HelixQA (`HelixDevelopment/HelixQA`) is wired as a submodule at `submodules/helixqa/`. It provides the autonomous QA orchestration layer: test bank definitions, bank-runner execution, and read-the-screen content verification (per §11.4.158, §11.4.160).

---

## 2. Current Integration State

| Component | Status | Evidence |
|---|---|---|
| Submodule present | PASS | `submodules/helixqa/` checked out |
| Bank-runner wired | PASS | `tools/helixqa/run_bank.sh --self-test` wired into pre_build_verification.sh |
| HelixQA self-test in pre-build gate | PASS | `tests/pre_build_verification.sh` line 32 |
| OTA server test bank | PARTIAL | Basic health endpoint + auth flow tests present |
| Dashboard e2e flow evidence | PRESENT | `docs/helixqa/e2e_dashboard_flow_evidence.md` |
| Full autonomous QA sessions | PLANNED | Requires complete test bank coverage |
| Vision bridge (§11.4.160) | PLANNED | OCR/vision content verification for recordings |

---

## 3. Test Banks

### 3.1 Operational Banks

| Bank | Scope | Status | Tests |
|---|---|---|---|
| `ota-server-health` | Server health endpoints | Active | GET /health, readiness probe |
| `ota-server-auth` | Authentication flow | Active | Login, JWT validation, token refresh |
| `ota-server-api` | CRUD API endpoints | Partial | Projects, releases, rollouts |
| `ota-server-transport` | HTTP/2, HTTP/3, Brotli | Planned | Transport negotiation, compression |
| `ota-server-security` | Security headers, rate limiting | Planned | HSTS, CSP, CSRF, rate-limit enforcement |
| `ota-dashboard` | Dashboard UI | Planned | Login, project list, theme toggle |

### 3.2 Challenge Banks

| Bank | Scope | Status |
|---|---|---|
| `challenges-ota-auth` | Auth bypass / token forgery challenges | Planned |
| `challenges-ota-rollout` | Rollout state machine edge cases | Planned |
| `challenges-ota-integrity` | Artifact integrity / signature challenges | Planned |

---

## 4. Integration Points

### 4.1 Pre-Build Gate

```bash
# tests/pre_build_verification.sh (line 32)
run_gate "helixqa-bank-runner-self-test" \
  bash "${SCRIPT_DIR}/../tools/helixqa/run_bank.sh" --self-test
```

### 4.2 HelixQA Wrappers

- `tools/helixqa/run_bank.sh` — bank-runner entry point
- `submodules/helixqa/` — HelixQA engine + bank definitions
- `submodules/challenges/` — companion challenge bank submodule

### 4.3 Vision Bridge (§11.4.160)

The vision bridge is a planned integration point that feeds captured recording frames into HelixQA for automated read-the-screen content verification:

```
Recording → Frame capture (≤5s interval) → OCR/vision analysis →
  Self-validated analyzer → Per-frame PASS/FAIL → HelixQA bank verdict
```

---

## 5. Roadmap

| Milestone | Target | Status |
|---|---|---|
| 1.0.0 — Basic bank self-test wired | Done | PASS |
| 1.1.0 — Full API test bank | Planned | — |
| 1.2.0 — Dashboard UI bank | Planned | — |
| 1.3.0 — Vision bridge MVP | Planned | — |
| 1.4.0 — Full autonomous QA sessions | Planned | — |

---

## 6. Evidence Registry

| Run ID | Date | Scope | Result |
|---|---|---|---|
| helixqa-sweep-20260610T104458Z | 2026-06-10 | Full bank sweep | GREEN |
| e2e-dashboard-flow | 2026-06 | Dashboard E2E flow | RECORDED |
