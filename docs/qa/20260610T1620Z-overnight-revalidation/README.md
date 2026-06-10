# Overnight autonomous re-validation — captured evidence (HEAD `eb9e1c4`)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T16:35:00Z |
| Status | GREEN — captured stdout for the transient (non-container) tiers of the 2026-06-10 overnight sweep |
| HEAD | `eb9e1c4` (on `main`) |
| Authority | Operator mandate 2026-06-10 ("most stable build by morning; zero risk, zero bluff") |

This directory captures the real stdout of the software tiers that are otherwise
stdout-only (no container/QEMU artifact dir), so every row of
`docs/qa/STABILITY_REPORT.md` Rev 2 is artifact-verifiable (anti-bluff §11.4.5 /
review NIT-2). The container + QEMU tiers have their own dedicated evidence dirs
(`docs/qa/20260610T1617*-full-lifecycle/`, `…-fleet/`, `…-recall-recovery-container/`,
`…-qemu-fw-smoke/`).

## Row → evidence map

| STABILITY_REPORT row | Evidence file | Key result |
|---|---|---|
| Go `-race -count=1` (fresh) | `go_race_count1.log` | all 8 internal pkgs `ok` under `-race` (clean build+run; the run's `RACE_EXIT=0` was asserted at the terminal) |
| pgx PostgreSQL integration | `pgx_integration.log` | `ok …/internal/store` (real Postgres via podman, `-tags integration`) |
| Dashboard Vitest | `dashboard_vitest.log` | `Test Files 7 passed (7)` / `Tests 58 passed (58)` |
| Dashboard typecheck | `dashboard_typecheck.log` | `tsc` exit 0 (3 tsconfig projects) |
| Constitution inheritance gate | `inheritance_gate.log` | `INHERITANCE GATE: PASS` (5 invariants) |
| Constitution meta-test (§1.1) | `meta_test.log` | `CONSTITUTION INHERITANCE: PASS (gate real + mutation-proven …)` |
| HelixQA bank self-test (§1.1) | `helixqa_selftest.log` | `SELF-TEST PASS (evidence ledger catches its own negation)` |
| HelixQA bank dry-run | `helixqa_dryrun.log` | `10 passed / 0 failed / 0 skipped (mode=dry)` |
| HelixQA LIVE full-bank | `helixqa_live_bank.log` | `10 passed / 0 failed / 0 skipped (mode=live)`; `LIVE_BANK_RC=0` |
| Pre-build verifier | `pre_build_verification.log` | `PRE-BUILD VERIFICATION: PASS` |

## Notes (§11.4.6 honesty)

- The LIVE HelixQA bank ran against an ephemeral `ota-server` booted with a
  **fresh, test-only admin credential generated for the run** (never the
  operator's secret, never printed, never committed) and torn down on exit — a
  real LIVE run against a real server under our control, not a secret-guess.
- Go build/vet/gofmt clean is implied by the race + pre-build runs (both compile
  the full module); their stdout is in the logs above.
- Android tiers are NOT captured here — they were not re-run (byte-identical to
  prior proof, pins `1061015`/`8bb8d2f` unchanged; §12.6/§11.4.6 decision).
