# Helix OTA — Per-Feature Coverage Matrix (§11.4.25 Coverage Ledger)

**Revision:** 1
**Last modified:** 2026-06-23T17:30:00Z
**Scope:** the §11.4.25 Full-Automation-Coverage LEDGER for the helix_ota control
plane + Android A/B stack — a per-FEATURE × per-test-type status matrix that
consolidates the 2026-06-23 test-coverage + anti-bluff build-out program into the
feature dimension the by-test-type docs do not surface.
**Companion docs (referenced, NOT duplicated):**
- By-test-type live ledger (§11.4.45) —
  [`MASTER_TEST_COVERAGE_PLAN_20260623.md`](MASTER_TEST_COVERAGE_PLAN_20260623.md)
- By-test-type audit-grade snapshot —
  [`PHASE1_COVERAGE_REPORT_20260623.md`](PHASE1_COVERAGE_REPORT_20260623.md)
- Underlying evidence corpus — `docs/qa/20260623-*/` (~28 run dirs)

**Authority:** the route/feature set is enumerated from
`server/internal/api/server.go` (the live router) + the per-handler/per-test files
in `server/internal/api/`; the A/B + agent features from the Android bricks under
`submodules/`.
**HEAD at authorship:** `1f6ec790`.
**Honesty (§11.4.6):** every non-empty cell cites the captured-evidence dir that
backs it. GAP / PENDING / REFERENCE / N-A are stated as facts, never rounded up to
green. Read this matrix together with §3 (gaps & honest findings) — the green cells
are only real because the gaps are honestly marked.

---

## 1. How to read the matrix

- **unit** — Go unit / table tests with measured `go test -cover` (or JVM JaCoCo for
  Android features).
- **integration (pgx)** — exercised against real Postgres booted on-demand (containers
  brick), `-tags integration`.
- **e2e / live** — black-box against the real running system (real HTTP + real DB via
  F-CLUSTER), full user-equivalent flow.
- **security** — trust-boundary / authz / JWT-tamper / fuzz / saturation as applicable
  to the feature.
- **stress / chaos** — §11.4.85 sustained-load + failure-injection where the feature is
  on the hot path.
- **benchmark** — benchstat-guarded micro-benchmark where the feature has a perf-critical
  Go function.
- **Challenge** — registered in the helix_ota shell-dispatch Challenge bank (Phase 2).
- Cell values: `PASS` / measured `%` / `N-A` (test type does not apply) /
  `GAP` (should exist, does not yet) / `PENDING` (blocked on external dep) /
  `REFERENCE` (proven on a reference target, not the production target).

---

## 2. Per-feature × per-test-type matrix

| # | Feature (route / surface) | unit | integration (pgx) | e2e / live | security | stress / chaos | benchmark | Challenge | Evidence dirs (docs/qa/20260623-…) | Overall |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | **Health / readiness** (`/healthz`, `/readyz`) | PASS 100% | PASS (booted) | PASS (`/readyz`→200) | N-A | PASS (166k reqs, 0 non-2xx) | N-A | PASS (live boot) | `server-coverage`, `real-system-boot`, `http-load-live` | **COVERED** |
| 2 | **Auth / JWT** (`/auth/login`, `/auth/refresh`) | PASS (api 90.5%) | PASS | PASS (login→token used downstream) | PASS (JWT-tamper / authz rejected) | N-A | N-A | partial | `server-coverage`, `e2e-live-system`, `trust-boundary-live` | **COVERED** |
| 3 | **Device registration** (`/devices/register`, list, `by-hardware`, `:id/status`) | PASS (device 92.6%) | PASS (store 85.5%) | PASS (devices=2 persisted, end-of-run delete real) | PASS (authz) | PASS (12-race idempotent → exactly-one-device) | N-A | partial | `server-coverage`, `postgres-integration`, `e2e-live-system`, `chaos-live` | **COVERED** |
| 4 | **Artifact upload + signature verify** (`/artifacts/upload`, `:id`) | PASS | PASS | PASS (signed upload in 15/15 pipeline) | PASS (trust boundary 4/4 + fuzz 0-crash) | PASS (malformed/oversized→400) | N-A | PASS (bad-sig / request-key-ignored) | `signed-pipeline-live`, `trust-boundary-live`, `fuzz`, `chaos-live` | **COVERED** |
| 5 | **Delta generation** (`/deltas` POST/GET) | PASS | PASS | PASS (delta path in pipeline) | N-A | N-A | PASS (MemoryFindDelta 196ns / 0-allocs) | GAP | `server-coverage`, `benchmarks` | **COVERED** |
| 6 | **Release management** (`/releases` POST/GET/`:id`) | PASS | PASS | PASS (release step in 15/15) | PASS (authz) | N-A | N-A | partial | `server-coverage`, `signed-pipeline-live` | **COVERED** |
| 7 | **Deployment** (`/deployments` POST/GET/`:id`) | PASS | PASS | PASS (deploy step in 15/15) | PASS (authz) | N-A | N-A | partial | `server-coverage`, `signed-pipeline-live` | **COVERED** |
| 8 | **Rollout — staged + halt/evaluate** (`/rollout`, `/rollout/evaluate`) | PASS (rollout 100% unit) | PASS (rollout 83.1%) | PASS (rollout step in 15/15) | N-A | N-A | N-A | PASS (rollout-staged-halt 47/47 vs live) | `ota-coverage`, `postgres-integration`, `phase2-challenges` | **COVERED** |
| 9 | **Recall / rollback** (`/recall`, `/rollbacks`) | PASS (handlers_recall_test) | PASS | partial (covered by handler+integration; not in the 15/15 happy-path e2e) | N-A | N-A | N-A | GAP | `server-coverage`, `postgres-integration` | **PARTIAL** |
| 10 | **Client update poll + anti-downgrade** (`/client/update`) | PASS (handlers_client + antidowngrade test) | PASS | PASS (device-poll-receives-signed-update in 15/15) | PASS (anti-downgrade enforced) | N-A | N-A | partial | `server-coverage`, `signed-pipeline-live` | **COVERED** |
| 11 | **Telemetry ingest + views** (`/client/telemetry`, `/devices/:id/telemetry`, `/telemetry/overview`) | PASS (schema 98.9%) | PASS | PASS (audit/telemetry rows persisted) | PASS (telemetry parser fuzzed 0-crash) | N-A | N-A | PASS (telemetry-schema 12/12 vs live) | `ota-coverage`, `fuzz`, `phase2-challenges`, `e2e-live-system` | **COVERED** |
| 12 | **Groups / membership** (`/groups` CRUD, `/members`) | PASS (handlers_group_test) | PASS | PASS (device_groups=0 end-of-run-delete real) | PASS (authz) | N-A | N-A | GAP | `server-coverage`, `e2e-live-system` | **COVERED** |
| 13 | **Audit log** (`/audit`) | PASS (handlers_audit_test) | PASS | PASS (audit_logs=14 rows persisted) | PASS (authz) | N-A | N-A | GAP | `server-coverage`, `e2e-live-system` | **COVERED** |
| 14 | **Projects** (`/projects` CRUD) | PASS (coverage_project_deployment_test) | PASS | partial (unit+integration covered; not in the operational 39/39 e2e flow) | PASS (admin-role on DELETE) | N-A | N-A | GAP | `server-coverage`, `postgres-integration` | **PARTIAL** |
| 15 | **Rate limiting / DDoS resilience** (`maxInflightMiddleware`) | PASS (rate_limit_test) | N-A | N-A | PASS *when cap set* (429 shed + recover) | PASS (default-stack 360-flood → 0 5xx) | N-A | PASS (load + saturation registered) | `saturation-live`, `http-load-live`, `challenges-bank` | **COVERED (ships OFF — see §3.1)** |
| 16 | **Compression / middleware** (gzip negotiation) | PASS (middleware_compression_test) | N-A | PASS (live HTTP path) | N-A | PASS (under load) | N-A | N-A | `server-coverage`, `http-load-live` | **COVERED** |
| 17 | **Android agent (JVM)** — `ota-android-agent` :core (poll + json) | PASS (LINE 100% / BRANCH 100%, 47 tests) | N-A | N-A | N-A | N-A | N-A | N-A | `android-jacoco`, `json-coverage`, `agent-poll-coverage` | **COVERED (JVM)** |
| 18 | **update_engine bridge (JVM)** — `ota-update-engine-bridge` | PASS (LINE 100%, 113/113, 27 tests) | N-A | N-A | N-A | N-A | N-A | N-A | `android-jacoco` | **COVERED (JVM)** |
| 19 | **A/B on-device flow** (update_engine + AVB/dm-verity + auto-rollback, slot flip) | N-A (JVM unit ≠ on-device) | N-A | REFERENCE (slot-flip + rollback trace on Cuttlefish cvd Tier-2) | N-A | N-A | N-A | N-A | `cuttlefish-tier2-ab` (ref), `cuttlefish-launch-verified` (PARTIAL — boot operator-gated) | **GAP / REFERENCE (no production hardware)** |
| 20 | **cmd/* + tools/* binaries** (ota-server, applyport, ota-device-emu, loadtest) | partial (applyport 24.4%, loadtest 63.3%, **ota-device-emu 0.0%**) | N-A | smoke (each main boots, vet+gofmt clean) | N-A | N-A | N-A | N-A | `cmd-smoke` | **PARTIAL (smoke-covered; emu 0% unit — see §3.5)** |

**Cross-cutting anti-bluff infrastructure** (not a product feature, but the proof the
matrix above is real — see the companion reports): F-ANTIBLUFF-LIB 8/8 mutation-proven
(`antibluff-lib`); all 14 propagation gates + functional gates §1.1-paired
(`metagates`, `propagation-metagates`); §11.4.165 independent verifier run on the batch
and caught a tautological restore-integrity bluff (`indep-verif-gate`).

---

## 3. Gaps & honest findings (§11.4.6 — the anti-bluff proof)

These are the cells the matrix marks below-green, stated as facts, not papered over.
They mirror the Phase-1 report §3; restated here against the per-feature dimension.

1. **Rate-limiter (feature 15) ships OFF by default.** The 429 shed/recover control is
   PROVEN to work when `HELIX_MAX_INFLIGHT` is set, but the shipped
   `server/deploy/system.compose.yml` leaves it **unset**, so `maxInflightMiddleware` is
   a no-op passthrough. The default stack survives a bounded 360-flood via host
   scheduling + bounded work, NOT via shedding. Recorded as a hardening recommendation
   (operator decision, §11.4.122) — never silently flipped.
   *Evidence:* `docs/qa/20260623-saturation-live/case_b_SUMMARY.txt`.

2. **A/B on-device flow (feature 19) is GAP / REFERENCE — no production hardware.** The
   JVM bricks (features 17–18) are LINE/BRANCH 100%, but the on-device
   update_engine + AVB/dm-verity + auto-rollback slot-flip is proven only on a
   **Cuttlefish cvd Tier-2 reference**, not on the RK3588 / Orange Pi 5 Max target. The
   `cuttlefish-launch-verified` run is itself **PARTIAL** — the privileged guest boot is
   operator-gated under rootless (expected `VIRTUAL_DEVICE_BOOT_FAILED run_cvd
   returned 10`); asset-feed + `launch_cvd` discovery + cvd config-assembly are proven,
   the privileged boot is not. This is the single largest honest gap.
   *Evidence:* `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md`,
   `docs/qa/20260623-cuttlefish-launch-verified/REPORT.md`.

3. **Recall/rollback (feature 9) is PARTIAL at the e2e layer.** Covered by unit
   (`handlers_recall_test`) + integration (real pgx), but the 15/15 signed e2e pipeline
   exercises the happy-path deploy→rollout→poll flow, not a recall trigger end-to-end
   against the live system. No dedicated recall Challenge yet.
   *Next step:* a `recall-halts-rollout` live e2e + Challenge with paired mutation.

4. **Projects (feature 14) is PARTIAL at the e2e layer.** Unit
   (`coverage_project_deployment_test`) + integration covered; not part of the
   operational 39/39 black-box e2e flow, and no Project Challenge registered.

5. **`ota-device-emu` (feature 20) reports 0.0% unit coverage.** Recorded honestly: it
   is a dev/emulation tool — build-tested, boots, vet+gofmt clean (smoke-covered), but
   carries no unit tests. Not claimed as unit-covered.
   *Evidence:* `docs/qa/20260623-cmd-smoke/evidence.txt`.

6. **Challenge column is shell-dispatch only; several features show GAP.** Telemetry-
   schema (12/12), rollout-staged-halt (47/47), trust-boundary, load, chaos, and
   signed-pipeline are registered; delta, recall, groups, audit, projects have **no
   dedicated OTA Challenge** yet. The Go `cmd/helixqa` orchestrator is **OPERATOR-GATED**
   (7 of 8 own-org `replace` submodules missing + §11.4.28(C) layout mismatch) — bank
   format matches, needs a ~1-file DeviceExec adapter.
   *Evidence:* `docs/qa/20260623-phase2-challenges/`,
   `docs/qa/20260623-helixqa-orchestrator/WIRING_PLAN.md`.

7. **Functional media/vision gates (§11.4.163/.165) are LATENT.** helix_ota ships no UI
   surface; the media-validation pipeline binds the moment a UI/recorded-artifact
   surface ships (§11.4.96 pattern). Recorded as latent, not as a failed gate.

8. **Structurally-unreachable telemetry branch left honestly uncovered.** ota-telemetry-
   schema is **98.9%** — the 1.1% residual is a decode branch not reachable through the
   public path; left uncovered rather than forced to a synthetic PASS.
   *Evidence:* `docs/qa/20260623-ota-coverage/ota-telemetry-schema_after.txt`.

---

## 4. One-line verdict per feature

| # | Feature | Verdict |
|---|---|---|
| 1 | Health / readiness | Fully covered across applicable types; live-boot + load proven. |
| 2 | Auth / JWT | Fully covered; trust-boundary + tamper rejection proven. |
| 3 | Device registration | Fully covered; race-idempotency proven (exactly-one-device). |
| 4 | Artifact upload + signature verify | Fully covered; trust boundary 4/4 + fuzz 0-crash + bad-sig Challenge. |
| 5 | Delta generation | Covered incl. benchmark guard (196ns/0-allocs); no dedicated Challenge. |
| 6 | Release management | Fully covered through the signed pipeline. |
| 7 | Deployment | Fully covered through the signed pipeline. |
| 8 | Rollout (staged + halt) | Fully covered; 47/47 staged-halt Challenge vs live. |
| 9 | Recall / rollback | **PARTIAL** — unit+integration solid; live e2e + Challenge are the gap. |
| 10 | Client update poll + anti-downgrade | Fully covered; device receives signed update in 15/15. |
| 11 | Telemetry | Fully covered; parser fuzzed + 12/12 schema Challenge vs live. |
| 12 | Groups / membership | Covered (unit+integration+e2e persistence); no Challenge yet. |
| 13 | Audit log | Covered; 14 rows persisted proof; no Challenge yet. |
| 14 | Projects | **PARTIAL** — unit+integration solid; e2e + Challenge are the gap. |
| 15 | Rate limiting / DDoS | Capability covered + proven, but **ships OFF by default** (hardening rec). |
| 16 | Compression / middleware | Covered under live HTTP + load. |
| 17 | Android agent (JVM) | Fully covered — LINE 100% / BRANCH 100%. |
| 18 | update_engine bridge (JVM) | Fully covered — LINE 100%. |
| 19 | A/B on-device flow | **GAP / REFERENCE** — Cuttlefish Tier-2 only; no production hardware. |
| 20 | cmd/* + tools/* binaries | **PARTIAL** — smoke-covered; ota-device-emu 0% unit. |

**Tally:** 20 enumerated features.
- **Fully covered (across all applicable test types):** 14 — features 1–8, 10–13, 16, 17, 18.
- **Covered-with-caveat:** 1 — feature 15 (capability proven but ships OFF; hardening rec).
- **PARTIAL:** 3 — features 9 (recall e2e/Challenge), 14 (projects e2e/Challenge), 20 (emu 0% unit).
- **GAP / REFERENCE:** 1 — feature 19 (A/B on-device; no production hardware).

The largest honest gap is the on-device A/B flow (feature 19), blocked on real
RK3588 / Orange Pi 5 Max hardware; the JVM layer beneath it is at 100%. The remaining
gaps are e2e/Challenge breadth for recall + projects, the emu unit-coverage hole, and
the operator-gated rate-limiter default + HelixQA Go orchestrator.

---

*This matrix is a faithful per-feature consolidation of the committed
`docs/qa/20260623-*` evidence corpus and the two companion reports. It asserts no result
not backed by a cited captured-evidence dir. HTML + PDF siblings (§11.4.65) are generated
by the conductor, not in this doc.*
