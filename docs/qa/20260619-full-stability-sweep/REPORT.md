# Full Stability Sweep Report — 2026-06-19

| Field | Value |
|---|---|
| Run date | 2026-06-19 02:15 UTC |
| Project | github.com/HelixDevelopment/helix_ota |
| Server commit | HEAD of main |
| Gate script | `tests/pre_build_verification.sh` |
| Report path | `docs/qa/20260619-full-stability-sweep/REPORT.md` |

---

## 1. Server Go Test Suite

**Command:** `cd server && go test -count=1 -v ./...`

**Result: ALL TESTED PACKAGES PASS**

| Package | Result | Time |
|---|---|---|
| `server/internal/api` | PASS | 0.384s |
| `server/internal/config` | PASS | 0.344s |
| `server/internal/deviceemu` | PASS | 1.163s |
| `server/internal/fabric` | PASS | 0.595s |
| `server/internal/health` | PASS | 0.458s |
| `server/internal/rollout` | PASS | 0.723s |
| `server/internal/store` | PASS | 0.854s |
| `server/internal/transport` | PASS | 1.227s |
| `server/internal/api/manager-dist` | (no tests) | -- |
| `server/cmd/ota-device-emu` | (no tests) | -- |
| `server/cmd/ota-server` | (no tests) | -- |
| `server/tools/loadtest` | (no tests) | -- |

**Evidence:** `/tmp/server_tests_r2.log`

---

## 2. Stress + Chaos Tests

**Command:** `bash tests/stress_chaos/run_server_stress.sh`

**Result: PASS** (exit code 0)

| Phase | Subtest | Result | Key metrics |
|---|---|---|---|
| Stress | TestStressConcurrentGroupCreate | PASS | p50=271us p95=797us p99=1.02ms |
| Stress | TestStressSustainedReads | PASS | p50=10us p95=350us p99=1.20ms |
| Stress | TestStressConcurrentMembershipNoLostUpdates | PASS | |
| Stress | TestStressConcurrentAuth | PASS | 10/10 2xx |
| Stress | TestStressConcurrentDevice | PASS | 10/10 2xx |
| Stress | TestStressConcurrentRelease | PASS | 1 created / 99 conflicts (expected) |
| Chaos | TestChaosAuthBadPayload (4 cases) | PASS | All rejected 401 |
| Chaos | TestChaosAuthHugePayload | PASS | 401 graceful degradation |
| Chaos | TestChaosConcurrentMutation | PASS | 49/50 conflicts (expected) |
| Chaos | TestChaosStoreRestart | PASS | Baseline 201 -> fault 500 -> recovered 201 |
| Chaos | TestChaosRepoFaultDegradesAndRecovers | PASS | 200 -> 500 -> 200 |

**Evidence dir:** `qa-results/stress_chaos/20260618T231559Z/`

---

## 3. Pre-Build Gates

**Command:** `bash tests/pre_build_verification.sh`

**Result: PASS** (exit code 0)

| Gate | Result | Notes |
|---|---|---|
| constitution-inheritance A (clean tree) | PASS | All 7 invariants met |
| constitution-inheritance B (sec1.1 mutation) | PASS | Mutation correctly caused FAIL |
| constitution-inheritance C (submodule pointers) | PASS | All 8 owned submodules wired |
| helixqa-bank-runner-self-test A (missing evidence) | PASS | Correctly FAILed |
| helixqa-bank-runner-self-test B (real evidence) | PASS | Correctly PASSed |
| CM-COVENANT-114-153 through 158 PROPAGATION | OK | All 6 propagation gates pass |

**Note:** First run FAILed because the constitution submodule was not checked out. After `git submodule init && git submodule update`, all gates pass clean.

**Evidence:** `/tmp/gates_r2.log`

---

## 4. Go Submodule Tests

**Command:** `for sm in ota-protocol ota-artifact-validator ota-rollout-engine ota-telemetry-schema http3; do cd submodules/$sm && go test -count=1 ./...; done`

**Result: ALL 5 SUBMODULES PASS**

| Submodule | Result | Time |
|---|---|---|
| `submodules/ota-protocol` | PASS | 0.221s |
| `submodules/ota-artifact-validator` | PASS | 0.249s |
| `submodules/ota-rollout-engine` | PASS | 2.354s |
| `submodules/ota-telemetry-schema` | PASS | 0.195s |
| `submodules/http3` | PASS | 1.111s |

---

## 5. Go Build + Vet

| Step | Exit Code | Result |
|---|---|---|
| `go build ./...` | 0 | PASS |
| `go vet ./...` | 0 | PASS |

**Note:** First run FAILed because the `containers` submodule was not checked out. After `git submodule update`, both build and vet pass.

---

## Overall Verdict

| Check | Result |
|---|---|
| Server test suite (7 packages with tests) | **PASS** |
| Build | **PASS** |
| Vet | **PASS** |
| Stress tests | **PASS** |
| Chaos tests | **PASS** |
| Pre-build gates | **PASS** |
| Go submodule tests (5 submodules) | **PASS** |
| **OVERALL** | **PASS** |

### Anomalies noted

1. **Submodule initialization required:** The constitution and containers submodules were not checked out. A fresh clone requires `git submodule update --init` before tests/gates pass.
2. **Containers dependency:** The `digital.vasic.containers` replace directive in `go.mod` blocks all packages importing `internal/deviceemu` or `internal/transport` when the submodule is missing.
3. **No tests for cmd packages:** `ota-device-emu`, `ota-server`, `manager-dist`, and `loadtest` have no test files (expected for CLI entry points).
4. **Concurrent release conflicts:** `TestStressConcurrentRelease` reports 99 conflicts from 100 concurrent version bumps -- this is the expected optimistic-concurrency behaviour.
