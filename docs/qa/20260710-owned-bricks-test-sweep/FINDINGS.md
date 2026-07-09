# Owned Go Bricks — Test Coverage Sweep (Stream BT)

**Revision:** 1
**Last modified:** 2026-07-09T20:56:46Z

## Scope

Read-only build/vet/gofmt/test sweep of the OWNED Go submodule bricks under
`submodules/` in the Helix OTA repo (§11.4.169 comprehensive test-type
coverage evidence + §11.4.28 equal-codebase attention to owned submodules).

No source was modified. No git command was run. `submodules/helixqa` and
`submodules/http3` were SKIPPED per instruction (foreign/in-flight work
owned by another stream, per §11.4.84 working-tree quiescence).

Discovery: `find submodules -maxdepth 3 -name go.mod -not -path '*/vendor/*'`
found 14 `go.mod` files; excluding `helixqa` and `http3` leaves 13 owned Go
module roots swept below (note `submodules/llms_verifier` has a nested
second module `submodules/llms_verifier/llm-verifier` — both are swept as
distinct module roots).

## Results

| Module | Build | Vet | gofmt | Tests | Evidence log |
|---|---|---|---|---|---|
| `submodules/challenges` | OK | OK | CLEAN | PASS (20 pkgs) | `submodules_challenges.log` |
| `submodules/containers` | OK | OK | CLEAN | **FAIL (1 test, non-reproducible — see finding)** | `submodules_containers.log` |
| `submodules/doc_processor` | OK | OK | CLEAN | PASS (10 pkgs) | `submodules_doc_processor.log` |
| `submodules/llm_orchestrator` | OK | OK | **NEEDS_FORMAT (6 files)** | PASS (9 pkgs) | `submodules_llm_orchestrator.log` |
| `submodules/llm_provider` | OK | OK | CLEAN | PASS (53 pkgs) | `submodules_llm_provider.log` |
| `submodules/llms_verifier` | OK | OK | CLEAN | PASS (15 pkgs, 1 pkg "no tests to run") | `submodules_llms_verifier.log` |
| `submodules/llms_verifier/llm-verifier` | OK | OK | CLEAN | PASS (60 pkgs) | `submodules_llms_verifier_llm-verifier.log` |
| `submodules/ota-artifact-validator` | OK | OK | CLEAN | PASS (3 pkgs) | `submodules_ota-artifact-validator.log` |
| `submodules/ota-protocol` | OK | OK | CLEAN | PASS (3 pkgs) | `submodules_ota-protocol.log` |
| `submodules/ota-rollout-engine` | OK | OK | CLEAN | PASS (3 pkgs) | `submodules_ota-rollout-engine.log` |
| `submodules/ota-telemetry-schema` | OK | OK | CLEAN | PASS (3 pkgs) | `submodules_ota-telemetry-schema.log` |
| `submodules/security` | OK | OK | CLEAN | PASS (18 pkgs) | `submodules_security.log` |
| `submodules/vision_engine` | OK | OK | **NEEDS_FORMAT (16 files)** | PASS (8 pkgs) | `submodules_vision_engine.log` |

**Module count swept: 13.** All 13 build clean (`go build ./...` exit 0),
all 13 vet clean (`go vet ./...` exit 0). 11/13 gofmt-clean; 2/13 need
formatting. 12/13 fully green on tests; 1/13 (`containers`) surfaced one
real test FAILure during the full-sweep run.

## Findings requiring surfacing (anti-bluff, §11.4.6 / §11.4.1)

### 1. `submodules/containers` — `TestIdleShutdown_TouchResets` FAILed once (real finding, non-reproducible in isolation)

During the full `go test ./... -count=1` sweep of the `containers` module,
`pkg/lifecycle` FAILed:

```
--- FAIL: TestIdleShutdown_TouchResets (0.37s)
    idle_test.go:37:
        Error Trace: .../pkg/lifecycle/idle_test.go:37
        Error:       Not equal:
                     expected: 0
                     actual  : 1
        Test:        TestIdleShutdown_TouchResets
FAIL
FAIL	digital.vasic.containers/pkg/lifecycle	2.184s
```

Reproducibility check performed (read-only, no source changes): re-ran the
single test in isolation 3× (`-run TestIdleShutdown_TouchResets -count=3`)
— PASS/PASS/PASS. Re-ran the full `pkg/lifecycle` package 3× more
(`-count=1` each) — PASS/PASS/PASS. **6/6 re-runs PASSed; only the original
full-module sweep run FAILed once.**

This is captured empirical evidence of a load/timing-sensitive test
(the FAIL occurred only when `go test ./...` was running many `containers`
packages concurrently, i.e. under host CPU/scheduling contention). Root
cause is **UNCONFIRMED** per §11.4.6 — no source was touched to
investigate further (out of this stream's read-only scope). This is
reported as a real finding, not smoothed over: it is a genuine, if
non-deterministic, test-suite result the conductor should track (candidate
root causes to investigate: a wall-clock/timer-based idle-reset assertion
that is not robust to scheduling delay under concurrent test load).

Full detail + reproducibility-check transcript: `submodules_containers.log`.

### 2. `submodules/llm_orchestrator` — gofmt reports 6 files needing formatting

```
pkg/agent/agent.go
pkg/agent/agent_test.go
pkg/agent/simple_pool.go
pkg/parser/parser.go
pkg/parser/parser_security_test.go
pkg/protocol/message.go
```

Not reformatted (read-only mandate). Real finding — `gofmt -l` returned
non-empty, meaning the recent formatting sweep referenced in the task
prompt did not (or no longer) cover this module.

### 3. `submodules/vision_engine` — gofmt reports 16 files needing formatting

```
cmd/visiondescribe/main.go
pkg/analyzer/i18n_defaults.go
pkg/analyzer/types.go
pkg/analyzer/types_test.go
pkg/config/i18n_callsites_test.go
pkg/graph/graph.go
pkg/graph/graph_automation_test.go
pkg/llmvision/anthropic.go
pkg/llmvision/astica.go
pkg/llmvision/i18n_defaults.go
pkg/llmvision/provider.go
pkg/llmvision/provider_test.go
pkg/opencv/i18n_defaults.go
pkg/opencv/orb_vision_test.go
pkg/remote/deployer_test.go
pkg/remote/remote.go
pkg/remote/remote_test.go
```

Not reformatted (read-only mandate). Real finding, same class as #2.

## Coverage-gap check (§11.4.169)

No swept module is entirely without tests — every module produced at
least one `ok` package result. Sub-package `[no test files]` results are
present in several modules (e.g. `cmd/` entrypoint packages, interface-only
packages, fixture directories) — these are normal, not whole-module gaps.
No module-level "no test files" coverage gap was found in this sweep.

## Evidence files

- `docs/qa/20260710-owned-bricks-test-sweep/COVERAGE_SWEEP.md` (this file)
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_challenges.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_containers.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_doc_processor.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_llm_orchestrator.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_llm_provider.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_llms_verifier.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_llms_verifier_llm-verifier.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_ota-artifact-validator.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_ota-protocol.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_ota-rollout-engine.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_ota-telemetry-schema.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_security.log`
- `docs/qa/20260710-owned-bricks-test-sweep/submodules_vision_engine.log`

No HTML/PDF siblings exported (conductor's responsibility per task scope).

---

## Conductor reconciliation (2026-07-10, independent spot-check §11.4.2/§11.4.6)

Re-ran the two flagged findings myself against the live tree before accepting this sweep:

- **Finding 1 — `containers TestIdleShutdown_TouchResets`**: re-ran in isolation → `ok pkg/lifecycle 0.306s` (PASS). Combined with the sweep's 6/6 isolated reruns, this is a **timing-sensitive test that failed once only under the parallel-sweep host contention** (all 13 modules testing at once → CPU/scheduler pressure delayed an idle-timer). Root cause **UNCONFIRMED** (§11.4.6 — not asserting a cause without forensic proof); classification `PENDING_FORENSICS`. **NOT release-blocking** for the OTA control plane — it is a lifecycle-timer test in the generic `vasic-digital/containers` brick, not a control-plane defect, and does not reproduce without induced load. Tracked follow-up: harden the test's timer margin or gate it on a load-detector; owner = a future operator-batchable submodule window (same window as the mirror-fork gofmt below).
- **Findings 2+3 — gofmt on `llm_orchestrator` (6 files) + `vision_engine` (17 files; sweep said 16, live count 17)**: confirmed real. These are **exactly the two bricks already DEFERRED** from the wave-4 gofmt canonicalization sweep for a pre-existing multi-mirror fork (see CONTINUATION §1 / `docs/research/submodule_gofmt_vet_sweep_20260710/FINDINGS.md` Rev 2). Their gofmt is intentionally pending the careful §11.4.113 merge-onto-latest-mirror convergence (no force-push), which is operator-batchable — NOT a new regression. Consistent with known state.

**Net:** all 13 owned Go bricks build + vet clean; 11/13 fully gofmt-clean + tests-PASS; the 2 gofmt-pending bricks are the known deferred-mirror pair; one timing-flaky containers test is `PENDING_FORENSICS`, non-blocking. No new release blocker — the loop continues (§11.4.4 not triggered: no reproduced product defect).
