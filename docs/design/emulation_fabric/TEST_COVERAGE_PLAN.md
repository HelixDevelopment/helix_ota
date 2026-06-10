# Helix OTA — Emulation Test-Fabric — Test-Coverage Plan

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T14:10:00Z |
| Status | active — plan |
| Status summary | How EVERY supported test type (§11.4.27) maps onto the fabric tiers in [`DESIGN.md`](DESIGN.md), the anti-bluff captured-evidence shape per type (§11.4.5/§11.4.69/§11.4.107), and how 100%-feature×flow×edge-case coverage is MEASURED and ratcheted (§11.4.50) — never asserted. |
| Related | DESIGN.md; ROADMAP.md; SCHEMA.sql; `docs/research/main_specs/1.0.0-mvp/qa/coverage_ledger.md` |

## 1. Test-type × tier × evidence matrix

| Test type | Primary tier(s) | Anti-bluff captured evidence (the PASS must cite) |
|---|---|---|
| unit | host (Go/Kotlin) | `go test`/Gradle transcript; mocks permitted ONLY here (§11.4.27) |
| integration | T0 + `pkg/fabric` registry | real-Postgres parity (`-tags integration`, podman-booted PG, 0 skips); request/response transcript |
| e2e | T0/T1/T2/T3 | full register→update-check→telemetry→delta→rollout→recall transcript + server-side history cross-check |
| full-automation | T1/T2/T3 | self-driving (no human in loop, §11.4.98); ADB/`update_engine_client` state + UI capture |
| security | Tcp + T0 | authz/ownership/injection/trust-boundary probes scored on real HTTP status (e.g. recall+telemetry probes 28/0) |
| ddos / scaling | Tcp (Firecracker/Kata) | measured req throughput + shed/served counts + recovery (§11.4.85) |
| chaos | T2 + Tcp + `pkg/fabric` | fault-injection (corrupt-slot→auto-rollback; lease loss; PG SIGKILL) + recovery trace (§11.4.85) |
| stress | T0 fleet + registry | N-device/N-lease sustained load, p50/p95/p99 latency, 0 lost updates |
| performance / benchmarking | T0 + Tcp | `testing.B` ns/op + alloc; memory-vs-pgx percentile sweep |
| ui / ux | T1 (AVD) | uiautomator/ADB-driven journey + screen capture; §11.4.107 liveness oracle where pixels matter |
| Challenges (HelixQA) | all tiers | bank challenge dispatching to the real test, scored on a non-empty evidence ledger (`run_bank.sh` + canonical engine) |
| autonomous QA session (§11.4.27) | all tiers | HelixQA session driving every registered bank with captured per-check evidence |

## 2. Real A/B fidelity boundary (FACT, §11.4.6)

Real `update_engine` A/B + AVB/dm-verity + auto-rollback evidence is produced ONLY by **T2
(Cuttlefish)** and **T3 (real RK3588)**. T0 reports the lifecycle but fakes the flash; T1 (stock
AVD) runs Android userspace but real-A/B-apply under AVD is `UNCONFIRMED:` pending the P1 GSI-A/B
empirical test. The coverage ledger records this boundary so no row claims A/B fidelity it lacks.

## 3. 100% coverage — measured, not claimed

- **Inventory.** Every feature × flow × use-case × edge-case is a row in the §11.4.25 coverage
  ledger, each cross-referenced to its test(s), tier(s), and the six §11.4.25 invariants.
- **Measurement.** Go: `go test -coverprofile` per package (≥90% safety floor already enforced
  on the critical handlers). Fabric/e2e: the ledger row is COVERED only when its real test exists
  AND passed with captured evidence in the last cycle — a declared-but-unrun row is a §11.4 bluff
  (the exact HelixQA-bank gap the `run_bank.sh` runner closed).
- **Ratchet (§11.4.50).** The covered/total ratio is a gate that only moves up; a regression in
  the ratio FAILs the release sweep. Edge-case discovery (§11.4.118) feeds new rows.
- **No skip-as-pass.** A tier that cannot run on a host SKIPs-with-reason (§11.4.3) and the ledger
  shows the gap honestly; it never counts a SKIP as coverage.

## 4. Per-change obligation (§11.4.146 / §11.4.135)

Every fix/feature on the fabric: reproduce-first RED on the broken artifact → same-test GREEN on
the fix → extend across the functionality's case-space (valid/invalid/boundary/concurrent/chaos/
topology) → register a permanent regression guard → add the HelixQA challenge → update the ledger
with measured coverage. Independent-agent review (§11.4.125/§11.4.142) confirms before merge.
