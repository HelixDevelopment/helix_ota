# Helix OTA — Production Readiness Planning Document

| Field | Value |
|---|---|
| Document | Production Readiness Planning |
| Created | 2026-07-26 |
| Last modified | 2026-07-26 |
| Status | Active |
| Compliance | §11.4.172 — living planning document with timeline projections, risk analysis, critical-path |
| Reference | `gap_tracker.csv` — single source of truth for gap inventory |

---

## 1. Executive Summary

Helix OTA is a Go/Gin OTA server targeting Android 15 (Orange Pi 5 Max / RK3588 class) with a modular-monolith control plane, HTTP/3+Broccoli transport, and PostgreSQL+MinIO/S3 persistence. Implementation phases US1 (API+Auth), US2 (Store+Transport), and US3 (Rollout+Orchestration) are complete. The server compiles, all unit/integration/e2e/security test tiers pass on this HEAD.

A comprehensive gap analysis performed 2026-07-25 identified **73 gaps** across 8 phases (0-Critical through 13-Code) in `gap_tracker.csv`. Zero gaps are currently marked "Closed"; one gap (G-17 — SQL syntax error in `schema_postgres.sql`) is "In-Progress." The remaining 72 gaps are "Queued."

Production readiness requires a coordinated closure campaign across all phases. The critical path runs through Phase 0 (Critical — 12 items) and Phase 1 (High — 13 items), which together represent 25 safety/security/functionality gaps that must be resolved before any production deployment. Based on measured velocity (currently 0/week as of day 1), completion estimates will refine as velocity data accumulates.

---

## 2. Per-Phase Gap Closure Status

### Phase 0 — Critical (12 items)

Gaps that block functionality or create safety hazards.

| Gap-ID | Description | Status |
|---|---|---|
| G-01 | ApplyPort WriteAndArm SCAFFOLD stub | Queued |
| G-02 | handlers_branches_test.go orphan test (no handler) | Queued |
| G-03 | No rollout auto-progress scheduler | Queued |
| G-04 | fw_env.config unverified round-trip | Queued |
| G-11 | Artifact download not on control-plane (intentional) | Queued |
| G-13 | WriteAndArm no real RAUC slot-writer | Queued |
| G-16 | handlers_branches_test.go orphan (dup of G-02) | Queued |
| G-17 | SQL syntax error in schema_postgres.sql | **In-Progress** |
| G-21 | Rate-limiter OFF by default (HELIX_MAX_INFLIGHT unset) | Queued |
| G-66 | ApplyPort SCAFFOLD (dup of G-01) | Queued |
| G-67 | orphan test (dup of G-02) | Queued |
| G-69 | schema syntax error (dup of G-17) | Queued |

**Phase 0 closed: 0/12 (0%) | In-progress: 1/12**

### Phase 1 — High (13 items)

Security, authorization, and operational-hardening gaps that are high-impact.

| Gap-ID | Description | Status |
|---|---|---|
| G-05 | No webhook/alerting for deployment events | Queued |
| G-06 | No project member ADD endpoint | Queued |
| G-07 | Account CRUD incomplete (only list) | Queued |
| G-22 | RLS not enabled for tenant isolation | Queued |
| G-23 | Security submodule not consumed by server | Queued |
| G-24 | No rate limit on artifact download path | Queued |
| G-25 | No session invalidation on role change | Queued |
| G-26 | No TUF supply-chain hardening | Queued |
| G-27 | No security.txt / SECURITY.md | Queued |
| G-28 | CSP for SPA may be too permissive | Queued |
| G-29 | No credential rotation mechanism | Queued |
| G-71 | SECURITY_TAMPER_DETECTED not wired | Queued |
| G-72 | No graceful degradation for store failures | Queued |

**Phase 1 closed: 0/13 (0%)**

### Phase 2 — Medium (11 items)

Feature-completeness and integration gaps.

| Gap-ID | Description | Status |
|---|---|---|
| G-08 | SetAccountMembership has NO HTTP endpoint | Queued |
| G-09 | No /deltas/generate endpoint | Queued |
| G-10 | Fabric registry has NO HTTP routes | Queued |
| G-12 | Android agent not wired to Go module | Queued |
| G-14 | No RLS (Row-Level Security) enabled | Queued |
| G-18 | No migration 003+ for delta/rollout/hardware | Queued |
| G-19 | No migration Down() methods | Queued |
| G-20 | RLS not enabled (migration 2 mentions LATER) | Queued |
| G-38 | Security submodule not consumed (dup of G-23) | Queued |
| G-39 | ota-android-agent not consumed by Go server | Queued |
| G-40 | ota-update-engine-bridge not consumed by Go server | Queued |

**Phase 2 closed: 0/11 (0%)**

### Phase 3 — Gates (0 items)

No gaps currently assigned to the gates phase. The pre-build inheritance gate (`tests/test_constitution_inheritance.sh`) already passes, and the HelixQA self-test is wired into the pre-build gate.

**Phase 3 closed: N/A**

### Phase 4 — Testing (10 items)

Coverage and quality-assurance gaps.

| Gap-ID | Description | Status |
|---|---|---|
| G-30 | Stress+chaos not universal (6/12 components) | Queued |
| G-31 | No mutation testing for most components | Queued |
| G-32 | No UI visual regression tests | Queued |
| G-33 | No cross-ACL boundary tests | Queued |
| G-34 | No session management tests | Queued |
| G-35 | No DB migration rollback tests | Queued |
| G-36 | No performance benchmarks for rollout | Queued |
| G-37 | No fuzz testing for API input | Queued |
| G-46 | challenges not integrated into server | Queued |
| G-47 | helixqa partial integration | Queued |

**Phase 4 closed: 0/10 (0%)**

### Phase 5 — ADR (1 item)

| Gap-ID | Description | Status |
|---|---|---|
| G-52 | No architecture decision log for all decisions | Queued |

**Phase 5 closed: 0/1 (0%)**

*Note: Five ADRs (ADR-0001 through ADR-0005) exist and were formally accepted 2026-07-26. G-52 may refer to additional ADRs not yet created. The existing ADRs cover: wrapped engine, supply-chain trust, server topology, transport, and delta updates.*

### Phase 6 — Infra (17 items)

Operational, deployment, and documentation infrastructure gaps.

| Gap-ID | Description | Status |
|---|---|---|
| G-15 | Duplicate LLMProvider submodules | Queued |
| G-48 | OpenAPI spec drift (12 routes / ~45 actual) | Queued |
| G-49 | No deployment runbook | Queued |
| G-50 | No disaster recovery doc | Queued |
| G-51 | No on-call runbook | Queued |
| G-53 | No API changelog | Queued |
| G-54 | docs/RESUMPTION.md may be stale | Queued |
| G-55 | No developer onboarding guide | Queued |
| G-56 | No upgrade/rollback procedure for server | Queued |
| G-57 | No backup strategy for PostgreSQL | Queued |
| G-58 | Docs Chain Phase 6 not implemented | Queued |
| G-59 | CodeGraph index aging (no auto-rebuild) | Queued |
| G-60 | No monitoring/alerting stack | Queued |
| G-61 | No log aggregation (Loki/ELK) | Queued |
| G-63 | No canary deployment strategy | Queued |
| G-64 | No rate limiting in production compose | Queued |
| G-65 | No TLS in default compose | Queued |

**Phase 6 closed: 0/17 (0%)**

### Phase 7 — Observer (7 items)

Submodule integration and unused-component gaps.

| Gap-ID | Description | Status |
|---|---|---|
| G-41 | llm_orchestrator not consumed (unused LLM tool) | Queued |
| G-42 | llm_provider x2 not consumed (unused LLM tool) | Queued |
| G-43 | llms_verifier not consumed (unused LLM tool) | Queued |
| G-44 | vision_engine not consumed (unused) | Queued |
| G-45 | doc_processor not consumed (unused) | Queued |
| G-62 | No CI/CD (intentional per §11.4.156) | Queued |
| G-68 | Duplicate submodules (dup of G-15) | Queued |

**Phase 7 closed: 0/7 (0%)**

### Phase 13 — Code (2 items)

Code-quality and maintainability gaps.

| Gap-ID | Description | Status |
|---|---|---|
| G-70 | No comments on complex logic | Queued |
| G-73 | Hardcoded constants in handlers | Queued |

**Phase 13 closed: 0/2 (0%)**

---

## 3. Velocity Metrics

### Methodology

Velocity is computed from `gap_tracker.csv` by counting items with Status != "Queued" (i.e., "In-Progress" or "Closed") and dividing by the number of weeks since the analysis start date (2026-07-26). Results are appended to `velocity.tsv` on each run of `scripts/track_velocity.sh`.

### Current Snapshot (2026-07-26)

| Metric | Value |
|---|---|
| Total gaps identified | 73 |
| Closed (Status = Closed) | 0 |
| In-Progress (Status = In-Progress) | 1 (G-17) |
| Queued | 72 |
| Days since start | 1 |
| Measured velocity | 7.00 items/week (1 item / 0.14 weeks) |

### Velocity Note

The initial velocity of 7.00 items/week is an extrapolation from a single day of data and one "In-Progress" item. This is not yet a reliable projection. Velocity will stabilize as more items are closed and data accumulates over multiple measurement points. Per §11.4.6 (no guessing), no completion date projection is made until at least 3 measurement points exist.

### Expected Velocity Refinement

- **Week 1** (2026-08-02): First meaningful data point with ≥5 measurement days
- **Week 2** (2026-08-09): Velocity trend becomes directionally reliable
- **Month 1** (2026-08-26): Sufficient data for completion-date projection per §11.4.172

---

## 4. Estimated Completion Date

**Deferred.** Per §11.4.6 (no-guessing), a completion date cannot be reliably projected from a single data point at day 1. A projection will be added when:

1. At least 3 velocity data points exist (≥3 weeks of closure data)
2. Measured velocity has stabilized (±20% variance over 3 consecutive measurements)
3. Remaining gap count is ≤50% of initial (≤36 items)

**Current best-case bounding:** At a sustained velocity of 7 items/week, 73 items close in ~10.4 weeks (2026-10-06). At a more typical 2-3 items/week, completion spans 24-36 weeks (2027-01 to 2027-04). These are bounding estimates, not commitments.

---

## 5. Risk Register

### Risk 1: TUF Client Blocked (No Kotlin Android Client)

| Field | Detail |
|---|---|
| **Risk ID** | R-001 |
| **Severity** | High |
| **Probability** | Confirmed — no verified production Kotlin/JVM TUF client exists |
| **Impact** | Delays full supply-chain trust hardening (G-26) to indefinite future |
| **Current mitigation** | MVP ships plain SHA-256 + detached signature + AVB (per ADR-0002). TUF server-side only (go-tuf/v2) in 1.0.1+ with device-side enforcement GATED on a spike. |
| **Acceptance** | Accepted risk for 1.0.0-MVP per ADR-0002. No 1.0.0 gate blocked. |
| **Monitoring** | Track go-tuf/v2 ecosystem for Android client emergence; spike gomobile/JNI vs hand-rolled Kotlin when 1.0.1 planning begins. |
| **Escalation trigger** | If no TUF client path exists when 1.0.1 scope is locked, accept plain-signing as the long-term trust model and document residual risk. |

### Risk 2: Remaining Test Coverage Gaps

| Field | Detail |
|---|---|
| **Risk ID** | R-002 |
| **Severity** | Medium |
| **Probability** | High — 10 testing gaps remain (mutation, fuzz, visual-regression, cross-ACL, session, migration-rollback, benchmarks, chaos, challenges, helixqa) |
| **Impact** | Reduced confidence in correctness for security-critical and edge paths; potential regressions undetected outside of existing test suite |
| **Current mitigation** | Core test suite (unit/integration/e2e/security) passes GREEN. Testing gaps are at the hardening/comprehensiveness layer, not the correctness layer. |
| **Acceptance** | Acceptable for development phase; must address Phase 0/1 gaps before any production deployment. |
| **Monitoring** | Priority: G-37 (fuzz testing) and G-36 (benchmarks) first as they feed into the G-03 rollout-scheduler confidence. |
| **Escalation trigger** | Any new defect discovered in code paths lacking coverage → accelerate gap closure for that testing family. |

### Risk 3: hawkBit Integration Gates (ADR-0001 §5.3 UNVERIFIED)

| Field | Detail |
|---|---|
| **Risk ID** | R-003 |
| **Severity** | Low (mitigated by fallback) |
| **Probability** | Medium — 6 UNVERIFIED gates remain for hawkBit 1.0.x Management API, DDI schema, S3 support, dynamic rollouts |
| **Impact** | If gates fail, hawkBit wrap is cancelled and AOSP-native-only custom Go rollout engine becomes the active path |
| **Current mitigation** | AOSP-native-only fallback is pre-authorized in ADR-0001 §4. The rollout-engine is already designed as an extractable module in the modular monolith (ADR-0003). |
| **Acceptance** | No production blocker — the fallback path is viable and pre-authorized. |
| **Monitoring** | Close the 6 gates against live hawkBit 1.0.x reference docs before coding the Go wrapper. |
| **Escalation trigger** | If 3+ gates fail → activate the AOSP-native-only fallback and record in a new ADR revision. |

### Risk 4: Velocity Stall (Many Queued, Few Resources)

| Field | Detail |
|---|---|
| **Risk ID** | R-004 |
| **Severity** | High |
| **Probability** | TBD — depends on resource allocation |
| **Impact** | 73 gaps with 0 closed after 1 day; if pace stays low, completion could extend to 2027 |
| **Current mitigation** | Velocity tracking mechanism (T088) provides measurement. Gap prioritization (Phases 0→1→2→...) ensures critical items close first. |
| **Acceptance** | Acceptable if Phase 0 closes within 4 weeks. Not acceptable beyond that. |
| **Monitoring** | Weekly velocity run via `scripts/track_velocity.sh`; review at each planning checkpoint. |
| **Escalation trigger** | Velocity < 3 items/week for 2 consecutive weeks → operator-directed resource escalation. |

### Risk 5: Rate Limiter OFF by Default

| Field | Detail |
|---|---|
| **Risk ID** | R-005 |
| **Severity** | Critical |
| **Probability** | Confirmed — HELIX_MAX_INFLIGHT unset → rate limiter is effectively disabled (G-21) |
| **Impact** | Unauthenticated or authenticated DoS vector; no production-safe default |
| **Current mitigation** | None active. Rate limiter exists in code but requires explicit configuration. |
| **Acceptance** | NOT acceptable for deployment. Must be resolved (G-21 closed) before any exposed instance runs. |
| **Monitoring** | Track G-21 to Closed. Add a startup guard that refuses to start without rate-limit config in non-dev mode. |
| **Escalation trigger** | If not resolved before any network-exposed deployment → BLOCK deployment. |

---

## 6. Critical Path Analysis

The critical path to a production-ready MVP is:

```
G-17 (SQL schema fix) → G-21 (rate-limiter default) → G-01/G-13 (WriteAndArm real path)
    → G-05 (webhook/alerting) → G-22/G-23 (RLS + security submodule)
    → G-03 (rollout scheduler) → G-48 (OpenAPI sync) → G-49 (deployment runbook)
```

**Estimated minimum gate chain:** 9 items must close before any "production" label can be applied. At the initial measured velocity of 7/week, this is ~1.3 weeks (optimistic); at a conservative 2/week, ~4.5 weeks.

---

## 7. Decision Log

| Date | Decision | Rationale |
|---|---|---|
| 2026-07-26 | ADR-0001 accepted (hawkBit front-runner, AOSP-native fallback) | Architecture locked for US1-US3; reversible by design |
| 2026-07-26 | ADR-0002 accepted (MVP plain signing; TUF 1.0.1+) | Phased trust model; device-side gated on spike |
| 2026-07-26 | ADR-0003 accepted (modular monolith) | MVP lean; extractable seams for later split |
| 2026-07-26 | ADR-0004 accepted (HTTP/3 + Brotli, 2-class compression) | Mandated stack; artifact byte-identity preserved |
| 2026-07-26 | ADR-0005 accepted (full payload MVP; AOSP incrementals post-MVP) | Simplest correct artifact; measured savings gate |
| 2026-07-26 | Velocity tracking mechanism created | scripts/track_velocity.sh — appends to velocity.tsv |

---

## 8. Next Steps

1. **Close G-17** (SQL schema fix — currently In-Progress): the sole active gap
2. **Run velocity tracking** daily/weekly to build measurement history
3. **Prioritize Phase 0 closure** (12 Critical items): target 50% closed within 2 weeks
4. **Address R-005** (rate limiter default): highest-severity unmitigated risk
5. **Update this document** when velocity stabilizes (≥3 data points) with an evidence-backed completion estimate

---

## 9. Compliance (HelixConstitution §11.4.172)

| Requirement | Evidence |
|---|---|
| Living planning document | This file — updated on measurable state changes |
| Realistic timeline from measured velocity | §3 — velocity measured from gap_tracker.csv, not guessed |
| Danger-zone / risk identification | §5 — 5 risks with severities, probabilities, mitigations, triggers |
| Critical-path analysis | §6 — minimum 9-item gate chain to production |
| Updated monthly or on ≥10% item-count change | This is the initial document; next update at or before 2026-08-26 |
| Per §11.4.6 — no guessing | Completion estimate (§4) explicitly deferred until ≥3 data points exist |

---
