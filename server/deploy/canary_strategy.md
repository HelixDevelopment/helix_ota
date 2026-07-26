# Canary Deployment Strategy — Helix OTA

| Field | Value |
|---|---|
| Document | Canary Strategy |
| Created | 2026-07-26 |
| Last modified | 2026-07-26 |
| Status | Draft |
| **Revision:** 1 |
| **Last modified:** 2026-07-26T00:00:00Z |

---

## 1. Overview

Canary deployment is the controlled, phased rollout of a new release to a subset of the device fleet before full deployment. This limits blast radius and provides early-warning signals for regressions. The Helix OTA rollout engine supports canary percentages natively via the `rollout` resource's `canary_percent` field.

---

## 2. Canary Phases

| Phase | % of Fleet | Duration | Success Criterion | Action on Failure |
|---|---|---|---|---|
| Phase 1 — Smoke | 5% | 2 hours | 0 crash-rate increase, health GREEN | Immediate recall |
| Phase 2 — Staging | 15% | 24 hours | ≤1% error rate, telemetry nominal | Recall after 1h of investigation |
| Phase 3 — Canary | 33% | 48 hours | All metrics within baseline ±10% | Recall after 2h of investigation |
| Phase 4 — Ramp | 66% | 24 hours | No new regressions vs Phase 3 | Recall with operator decision |
| Phase 5 — Full | 100% | — | Stable for 24h | Rollback procedure invoked |

---

## 3. Monitoring Signals (per-phase go/no-go)

### 3.1 Must-Pass (any failure = IMMEDIATE RECALL)

- Crash-rate (fatal + non-fatal) ≤ baseline
- Apply success rate ≥ 99%
- Server response 5xx rate ≤ 0.1%
- Device heartbeat within expected window

### 3.2 Should-Pass (triggers investigation, not automatic recall)

- P95 latency ≤ 2× baseline
- Memory usage within 20% of baseline
- Disk I/O within baseline range
- Network throughput within baseline range

### 3.3 Telemetry Sources

- Server-side: `/api/v1/telemetry` endpoint, Prometheus metrics
- Client-side: Android agent telemetry callback
- Infrastructure: PostgreSQL query stats, MinIO/S3 bucket metrics
- External: OTA server health endpoint monitoring

---

## 4. Recall Procedure

1. Operator or automation sets rollout `status` = `recalled`
2. Rollout engine recalculates deployment target to previous release
3. Devices receive `update_available` with previous release metadata
4. Affected devices download and apply the rollback release
5. Post-recall health sweep runs on recalled devices

---

## 5. Integration Points

- **Rollout Engine** (`server/internal/rollout/`): `canary_percent`, `status`, `recalled_at`
- **Deployment Handler** (`server/internal/api/handlers_deployment.go`): deployment lifecycle
- **Telemetry** (`server/internal/api/handlers_telemetry.go`): health signal ingestion
- **Health** (`server/internal/api/handlers_health.go`): server health endpoint

---

## 6. Configuration

```yaml
canary:
  enabled: true
  default_phase_durations:
    smoke: 2h
    staging: 24h
    canary: 48h
    ramp: 24h
  auto_recall_on:
    crash_rate_exceeded: true
    error_rate_threshold: 0.05
  monitoring:
    metrics_port: 9090
    health_endpoint: /health
```
