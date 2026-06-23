# Helix OTA — Phase-1 Test-Coverage Consolidated Report

**Revision:** 1
**Last modified:** 2026-06-23T16:00:00Z
**Scope:** consolidation of the 2026-06-23 comprehensive test-coverage + anti-bluff
build-out program (Phase 0 enablers + Phase 1 per-test-type real coverage).
**Authority:** the live program ledger is
[`MASTER_TEST_COVERAGE_PLAN_20260623.md`](MASTER_TEST_COVERAGE_PLAN_20260623.md)
(§11.4.45 — not duplicated here; this is the audit-grade snapshot of what closed).
**Honesty (§11.4.6):** every claim below cites the captured-evidence path that backs
it. Measured numbers are FACT; in-flight / pending items are marked explicitly. Green
numbers are real only because the "Honest findings & bluffs caught" section is real —
read both together.

---

## 1. Executive summary

Phase 1 of the test-coverage program is **complete** for the autonomously-reachable
test types. The Go control plane and the two Android JVM bricks are exercised against
the **real production-ready system** (real HTTP + real Postgres, booted on-demand via
the containers brick), with every PASS backed by captured physical evidence and every
functional gate paired with a §1.1 mutation that proves the gate is not a bluff.

Delivered (all committed + 4-remote synced):

- **Phase 0 enablers:** F-CLUSTER real-system boot (`system.compose.yml` ota-server +
  Postgres + `/readyz`), F-ANTIBLUFF-LIB (`ab_pass_with_evidence`), F-METAGATES
  (all 14 propagation gates + functional gates §1.1-paired).
- **Phase 1 per-type coverage:** integration (pgx), e2e/full-automation, security
  (trust-boundary + fuzz + saturation/DDoS), stress/chaos, benchmark, Android JVM,
  server unit, ota-* Go unit, cmd/* smoke.
- **Anti-bluff layer:** the §11.4.165 independent-verifier was run on the batch and
  **caught + fixed a tautological restore-integrity bluff** in the meta-test harness.

**Remaining (honest gaps):** on-device Android A/B (needs hardware), Phase 2 (HelixQA
Go orchestrator + vision against the live system; new OTA challenges), functional
media/vision gates (§11.4.163 — latent for a backend with no UI surface), and the
rate-limiter-ships-OFF hardening recommendation (operator decision, §11.4.122).

**Commit range:** `d94d84f9..HEAD` (HEAD = `86734699`), 13 commits.

---

## 2. Per-test-type coverage table

| Test type | Measured coverage / result | Evidence path | Honest gaps |
|---|---|---|---|
| **Integration (pgx/Postgres)** | store **85.5%** / rollout **83.1%** — real podman + Postgres | `docs/qa/20260623-postgres-integration/` (`coverage_func_full.txt`, `*_pkg_coverage.log`, `integration_test_run.log`) | not 100% of pgx paths; remaining branches are error-injection edges |
| **e2e / full-automation (black-box vs real DB)** | challenge_operational **39/39 PASS** (1 SKIP); DB-persistence proof `audit_logs=14, devices=2, device_groups=0` (end-of-run delete real) | `docs/qa/20260623-e2e-live-system/` (`challenge_operational_live.log`, `postgres_evidence.txt`) | — |
| **e2e signed pipeline (caller-pubkey vs live)** | **15/15 PASS** (signed upload→release→deploy→rollout→device-poll-receives-signed-update) | `docs/qa/20260623-signed-pipeline-live/SUMMARY.txt` (+ `step01..step15`) | committed this round (was IN-FLIGHT in ledger rev 2) |
| **Security — trust boundary** | **4/4 PASS** (accept-valid, reject-bad-sig, request-supplied-key ignored, metadata-key-field rejected) | `docs/qa/20260623-trust-boundary-live/SUMMARY.txt` | — |
| **Security — fuzz (`go test -fuzz`)** | 4 parsers fuzzed, **0 crashers** (~10.8M total execs across upload/telemetry/validate parsers; e.g. ParsePayloadProperties 2.69M execs → PASS) | `docs/qa/20260623-fuzz/` (`protocol_*.log`, `telemetry_*.log`, `validator_*.log`) | bounded fuzz window (≤39s/target), not exhaustive |
| **Security — saturation / DDoS** | cap-enabled flood → **429 shed + recover**; default-stack 360-flood → **0 5xx**, legit 6/6, recover 200 | `docs/qa/20260623-saturation-live/` (`case_a_*`, `case_b_SUMMARY.txt`, `EVIDENCE_INDEX.txt`) | **rate-limiter ships OFF** — see §3 honest findings |
| **Stress — HTTP load (§11.4.85)** | **p99 14.32ms @ 5,540 req/s, zero 5xx** (readyz total=166203, errors=25 non-2xx=0; p50=3.69 p90=8.54) | `docs/qa/20260623-http-load-live/SUMMARY.txt` (+ `latency_histogram.txt`) | calibrated tail ceiling 250ms; single-host profile |
| **Chaos (§11.4.85)** | **4/4 survive + recover**: postgres-kill→reconnect (no corruption), malformed/oversized→400, 12-race idempotent register→exactly-one-device, 400-conn churn→recover | `docs/qa/20260623-chaos-live/` (`case_a..case_d`, `SUMMARY.txt`) | — |
| **Benchmark** | benchstat baseline registry + negation-proven guard, **7 benches** (e.g. MemoryFindDelta 196ns/0-allocs); thresholds calibrated on measured variance | `docs/qa/20260623-benchmarks/EVIDENCE.md` + `docs/benchmarks/` (guard run/negation/exit-codes) | ns gate coarse (allocs >25%, ns >2×); tighter runner future |
| **Android JVM (instrumentation/ui via JaCoCo)** | ota-update-engine-bridge **LINE 100%** (113/113, 27 tests); ota-android-agent :core **LINE 100% / BRANCH 100%** (poll 77→100%, json 55.6→100%, 47 tests) | `docs/qa/20260623-android-jacoco/`, `docs/qa/20260623-json-coverage/`, `docs/qa/20260623-agent-poll-coverage/COVERAGE_EVIDENCE.md` | **on-device A/B pending** (needs hardware) |
| **Server unit (Go)** | device **92.6%** / api **90.5%** | `docs/qa/20260623-server-coverage/` (`device_cover_func_after.txt`, `api_cover_func_after.txt`, `coverage_evidence.md`) | — |
| **ota-* unit (Go)** | ota-protocol **100%**, ota-telemetry-schema **98.9%** (and per the ledger ota-validator/rollout 100%) | `docs/qa/20260623-ota-coverage/` (`*_after.txt`) | telemetry-schema 1.1% residual edge |
| **cmd/* + tools/* smoke** | each main boots; vet clean, gofmt clean; coverage applyport 24.4%, ota-server 5.1%, **ota-device-emu 0.0%**, loadtest 63.3% | `docs/qa/20260623-cmd-smoke/evidence.txt` | ota-device-emu 0% — see §3 (build+exec tested, no unit coverage) |
| **Anti-bluff per-assertion helper** | F-ANTIBLUFF-LIB **8/8 guard, mutation-proven** (always-pass mutant caught; no-evidence-no-PASS enforced) | `docs/qa/20260623-antibluff-lib/MUTATION_PROOF.txt` (+ `selftest_output.txt`) | — |
| **Anti-bluff gates (§1.1 meta-tests)** | meta suite **6/6** bluff-proof; **all 14 propagation gates §1.1-paired** (data-driven); semgrep fail-open FIXED; one real flake found+fixed (§11.4.50, 10/10 det) | `docs/qa/20260623-metagates/`, `docs/qa/20260623-propagation-metagates/EVIDENCE.md`, `docs/qa/20260623-indep-verif-gate/` | functional media/vision gates (§11.4.163/.165) pending — latent for backend |
| **F-CLUSTER real-system boot** | boots ota-server + Postgres, `/readyz`→200, podman healthy | `docs/qa/20260623-real-system-boot/REPORT.md` (`podman_ps_and_health.txt`, `api_smoke.txt`) | not multi-node/k8s |
| **Cuttlefish Tier-2 A/B (reference)** | slot flip + rollback trace captured on cvd (prior proven Tier-2) | `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md` | reference artifact; not the RK3588 target |

---

## 3. Honest findings & bluffs caught (the anti-bluff proof)

This section is the evidence that the green numbers above are **real** — every item is a
finding surfaced and recorded honestly (§11.4.6), not a paper-over.

1. **Rate-limiter ships OFF (HONEST CONFIG FINDING, not silently changed §11.4.122).**
   The DDoS resilience PASS is real, but the default shipped stack
   (`server/deploy/system.compose.yml`) leaves `HELIX_MAX_INFLIGHT` **unset**, so
   `maxInflightMiddleware` is a no-op passthrough. Case (a) proves the 429 shed/recover
   control **works when the cap is set**; case (b) shows the default stack survives a
   bounded 360-flood via the host scheduler + bounded work, **NOT** via shedding. This
   is recorded as a hardening recommendation, not a silent config flip.
   *Evidence:* `docs/qa/20260623-saturation-live/case_b_SUMMARY.txt`.

2. **Tautological restore-integrity bluff caught + fixed by the independent verifier
   (§11.4.165).** When the `CM-INDEPENDENT-VERIFICATION-AGENT` functional gate was added,
   the independent review found the meta-test harness could restore-and-pass without
   genuinely re-asserting integrity. The fix wired a fatal restore-integrity abort
   (`MT_RESTORE_FAILED` flag + `exit 90`) so the verifier catches a bluff **in itself**.
   Proven byte-identical restore (sha256 BEFORE == AFTER, no residue §11.4.84); meta
   suite went 5/5 → **6/6**.
   *Evidence:* `docs/qa/20260623-indep-verif-gate/README.md`, `restore_integrity.txt`,
   `summary_line.txt`.

3. **Load-harness FAIL-bluff class.** The HTTP-load harness was hardened so a
   harness-internal error cannot masquerade as a product FAIL (and vice versa) — the
   captured run reports total/rps/errors/non_2xx as independent fields with a calibrated
   tail ceiling, so a PASS is anchored to real counts (errors=25 non_2xx=0), not an
   absence-of-error inference.
   *Evidence:* `docs/qa/20260623-http-load-live/SUMMARY.txt`.

4. **Structurally-unreachable telemetry branch.** A telemetry decode branch was
   identified as not reachable through the public path; rather than forcing a synthetic
   PASS, it is left honestly uncovered (ota-telemetry-schema **98.9%**, the 1.1% being
   the unreachable edge) — coverage is reported as-measured, not rounded up.
   *Evidence:* `docs/qa/20260623-ota-coverage/ota-telemetry-schema_after.txt`.

5. **ota-device-emu 0.0% — but build + exec tested.** The `cmd/ota-device-emu` main
   reports 0.0% unit coverage. This is recorded honestly: it is a dev/emulation tool,
   smoke-covered (it builds, boots, vet-clean, gofmt-clean) but carries no unit tests.
   Not claimed as covered.
   *Evidence:* `docs/qa/20260623-cmd-smoke/evidence.txt`.

6. **Real intermittent flake found + fixed (§11.4.50).** During meta-test build-out a
   genuine intermittent flake was discovered and root-caused (not demoted to
   "transient"), then proven deterministic 10/10.
   *Evidence:* `docs/qa/20260623-metagates/`.

7. **e2e self-hosting honesty.** 4/5 e2e suites are self-hosting-by-design (the test
   must own the artifact pubkey to sign), so a caller-pubkey F-CLUSTER mode
   (`pipeline_signed_live.sh`) was drafted to exercise the signed pipeline vs the live
   system — recorded as IN-FLIGHT in the ledger until its evidence dir was committed,
   never claimed before it landed.
   *Evidence:* `docs/qa/20260623-signed-pipeline-live/SUMMARY.txt`.

---

## 4. Remaining work (honest §11.4.6)

| Item | Status | Why blocked / next step |
|---|---|---|
| On-device Android A/B (update_engine + AVB/dm-verity + auto-rollback) | **pending** | Needs real RK3588 / Orange Pi 5 Max hardware (or Cuttlefish cvd Tier-2). JVM coverage is 100%; the on-device flow is the next layer. Cuttlefish Tier-2 reference captured at `docs/qa/20260623-cuttlefish-tier2-ab/`. |
| Phase 2 — HelixQA Go orchestrator vs live system | **gap** | `cmd/helixqa` orchestrator + vision toolchain (`helixqa-text`/`recvalidate`/`recording-analyzer`) not yet wired against the live OTA system; bank-runner gates only today. |
| Phase 2 — new OTA challenges (AB-G4) | **partial** | 15 shell-dispatch banks exist; OTA-specific challenges (bad-signature-rejected, request-key-ignored, telemetry-schema, rollout-staged+halt, chaos) each need real dispatch + evidence + paired mutation. `docs/qa/20260623-challenges-bank/`. |
| Functional media/vision gates (§11.4.163) | **latent for backend** | helix_ota ships no UI surface yet; the §11.4.163 media-validation pipeline binds the moment a UI/recorded-artifact surface ships (§11.4.96 pattern). Recorded honestly as latent, not as a failed gate. |
| Rate-limiter hardening | **recommendation (operator decision)** | Recommend setting `HELIX_MAX_INFLIGHT` in the shipped `system.compose.yml` so the proven 429 shed/recover control is active by default. Operator-gated per §11.4.122 (capability change), not silently applied. |

---

## 5. Commit range

`d94d84f9..HEAD` — **13 commits** (HEAD = `86734699` "test(coverage): Phase-1 batch 4 —
saturation + cmd-smoke + §11.4.165 gate + agent 100%"; base `d94d84f9` "docs(testing):
real test-coverage audit + push-all to all upstreams"). All committed + 4-remote synced
by the conductor.

---

*This report is a faithful consolidation of the committed `docs/qa/20260623-*` evidence
corpus. It does not assert any result not backed by a cited captured-evidence path. HTML
+ PDF siblings (§11.4.65) are generated by the conductor, not in this doc.*
