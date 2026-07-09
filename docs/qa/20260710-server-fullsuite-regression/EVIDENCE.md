# Server Full-Suite Regression Evidence — 2026-07-10

**Revision:** 1
**Last modified:** 2026-07-09T20:14:32Z
**Scope:** `server/` Go buildable project (Stream FS, §11.4.119 exclusive owner)
**Authority:** §11.4.40 (full-suite retest slice), §11.4.4 (no-regression), §11.4.169 (comprehensive test types), §11.4.50 (determinism), §11.4/§11.4.6/§11.4.69 (anti-bluff, cited evidence)

## Purpose

Fresh comprehensive green regression evidence for the server on the current HEAD,
confirming this session's server changes integrate cleanly with the whole suite:

- `internal/api/memory_test.go` — memory-leak discriminator rewrite (review remediation)
- `internal/api/token_bench_test.go` — new token benchmarks

## Environment

| Field | Value |
|---|---|
| Repo | `github.com/HelixDevelopment/helix_ota/server` |
| Branch | `main` |
| HEAD SHA | `1df9a649d627e21109f4bd6b1b8a8646cb4b2f40` |
| HEAD subject | `test(server): fix memory-leak discriminator (review remediation) + security/benchmark evidence` |
| Go toolchain | `go1.26.4-X:nodwarf5 linux/amd64` |
| CPU | AMD Ryzen Threadripper 7970X 32-Cores |
| Run timestamp (UTC) | 2026-07-09T20:14:32Z |

Target files confirmed present + committed at HEAD (clean working tree for both):
`internal/api/memory_test.go` (15688 B), `internal/api/token_bench_test.go` (3202 B).

## Per-step results

| # | Step | Command | Result | Evidence log |
|---|------|---------|--------|--------------|
| 1 | HEAD | `git rev-parse HEAD` | `1df9a649…` (see above) | — |
| 2 | Build | `go build ./...` | **PASS** — exit 0, no diagnostics (0-byte log = clean) | `logs/02_go_build.log` |
| 3 | Vet | `go vet ./...` | **PASS** — exit 0, no diagnostics (0-byte log = clean) | `logs/03_go_vet.log` |
| 4 | Full suite | `go test ./... -count=1` | **PASS** — exit 0, every package `ok` (2 pkgs `[no test files]`) | `logs/04_go_test_all.log` |
| 5 | Race | `go test -race -count=1 ./internal/api/... ./internal/store/... ./internal/rollout/...` | **PASS** — exit 0, `grep -c "DATA RACE"` = 0 | `logs/05_go_test_race.log` |
| 6a | Memory tests | `go test -run 'TestMemoryGrowthClassifier\|TestMemory_SustainedAPILoadNoGrowth' -count=1 -v ./internal/api/` | **PASS** — exit 0 | `logs/06a_memory_tests.log` |
| 6b | Token benchmarks | `go test -bench=Token -benchmem -run='^$' -count=1 ./internal/api/` | **PASS** — exit 0 | `logs/06b_token_bench.log` |
| 7 | Determinism (`-count=3`) | `go test -run '…' -count=3 ./internal/api/` | **PASS** — exit 0, 1 `ok`, 0 `FAIL` | `logs/07_determinism_count3.log` |
| 7b | Determinism (3 invocations) | 3× `go test -run '…' -count=1 -v ./internal/api/` | **PASS** — 3×3 exit 0, identical PASS census, 0 `FAIL` | `logs/07b_determinism_3invocations.log` |

### Step 4 — full non-integration suite (all `ok`)

```
ok  cmd/applyport 0.924s        ok  cmd/ota-device-emu 0.924s   ok  cmd/ota-server 1.690s
ok  internal/api 6.957s         ?   internal/api/manager-dist [no test files]
ok  internal/config 0.002s      ok  internal/device 0.030s      ok  internal/deviceemu 0.181s
ok  internal/fabric 0.004s      ok  internal/health 0.002s      ok  internal/rollout 0.003s
ok  internal/store 0.004s       ok  internal/transport 0.178s
ok  tests/chaos 0.026s          ok  tests/stress 0.012s         ok  tools/loadtest 1.365s
```

### Step 5 — race detector (0 data races)

```
ok  internal/api 12.952s    ?  internal/api/manager-dist [no test files]
ok  internal/store 1.080s   ok internal/rollout 1.012s
```
`grep -c "DATA RACE"` → **0**.

## Memory-test real numbers (§11.4.169 memory + §11.4.107(10) self-validated analyzer)

| Test | Result | Real numbers |
|---|---|---|
| `TestMemory_SustainedAPILoadNoGrowth` | PASS (0.11s) | `retainedSmall=4240 retainedLarge=320 leak=false` — reason: `retainedLarge=320 <= noiseFloor=524288 (flat/below floor — retention did not scale)`, `goroutine_delta=0`. Evidence: `qa-results/memory/memory_sustained_api_load-20260709T201338Z.txt` |
| `TestMemoryGrowthClassifier` | PASS (0.00s) | table-driven classifier unit assertions |
| `TestMemoryGrowthClassifier_DetectsInjectedSteadyLeak` (golden-bad discriminator) | PASS (0.06s) | injected steady leak (4096 B/iter): `retainedSmall=8153696 retainedLarge=65497696 → leak=true` (retainedLarge > ref×3 → retention scaled with request count → **leak correctly caught**) |

The discriminator proves the rewritten classifier is **not** a PASS-bluff: it flags an
injected 4 KB/iter leak as `leak=true` (golden-bad FAILs the leak-check as it must),
while the real sustained-load workload is `leak=false` (golden-good PASSes).

## Token benchmark numbers (`internal/api`, AMD Ryzen Threadripper 7970X)

| Benchmark | iters | ns/op | B/op | allocs/op |
|---|---|---|---|---|
| `BenchmarkTokenMint-2` | 1,362,794 | 803.5 | 1312 | 14 |
| `BenchmarkTokenVerify-2` | 661,659 | 1682 | 1240 | 22 |
| `BenchmarkTokenVerifyReject-2` | 2,445,129 | 481.4 | 768 | 10 |

All benchmarks completed `PASS` / `ok`.

## Determinism (§11.4.50)

- `-count=3` single run: exit 0, 1 `ok` line, 0 `FAIL`.
- 3 separate `-count=1 -v` invocations: all exit 0; PASS census across the 3 runs —
  `TestMemory_SustainedAPILoadNoGrowth` PASS ×3, `TestMemoryGrowthClassifier`(+variant) PASS ×6,
  `FAIL` count = 0, `ok` lines = 3. Identical outcome every run.

## Out-of-scope (explicit SKIP)

- **Integration suite (`-tags integration`)** — NOT run. It requires podman + a live
  Postgres (files: `internal/store/postgres_integration_test.go`,
  `internal/rollout/postgres_integration_test.go`, `…fault…`, `…coverage…`, `faultproxy_test.go`,
  `pg_itest_lock_test.go`, etc.). Owned by a separate concern; §11.4.3 topology SKIP-with-reason,
  not a PASS-by-default. This slice covers the default (non-integration) build only.
- No git write operations performed (conductor commits). Only read-only git used.

## Verdict

**GREEN.** Build + vet clean (exit 0, no diagnostics); full non-integration suite all `ok`
(exit 0); race detector 0 data races across `internal/api` + `internal/store` + `internal/rollout`;
the session's memory-leak discriminator and token benchmarks pass on current HEAD with real
captured numbers; determinism holds across `-count=3` and 3 separate invocations. The session's
server changes integrate cleanly with the whole suite — no regressions found.
