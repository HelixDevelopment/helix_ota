# G5 Benchmark Baseline + Regression Guard — Evidence

**Date:** 2026-06-23  Machine: Apple M3 Pro, darwin/arm64, go1.26.2

- `guard_run_pass.txt` — guard vs real baseline → RESULT: PASS, exit 0 (all 7 within threshold).
- `guard_run_negation_fail.txt` — guard vs a FORGED baseline (MemoryListAudit allocs=1) → that
  benchmark reads +900% allocs/op → RESULT: FAIL (proves the guard is not a bluff gate, §11.4.6).
  Real baseline restored immediately after.
- Baseline files: docs/benchmarks/baseline/{internal_api,internal_store}.txt (raw count=6 runs).

Observed intrinsic variance (identical code, benchstat CI): ns/op 16–58%; allocs/op ±0%; B/op 0–23%.
Calibrated thresholds: allocs/op >25% FAIL (primary), ns/op >100% FAIL (secondary smoke).
