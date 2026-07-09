# Helix OTA Server — Real Build/Test Health Baseline

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z

Captured per anti-bluff/no-guessing discipline — every command below was
actually executed in `/home/milos/Factory/projects/tools_and_research/helix_ota/server`,
and every exit code / output shown is verbatim from that execution. No source
files were modified. No git add/commit/push was performed.

## 1. Go version

Command: `go version`

```
go version go1.26.4-X:nodwarf5 linux/amd64
```

## 2. `go build ./...`

Command (from `server/`):
```
go build ./... > /tmp/build_out.txt 2>&1; echo "BUILD_EXIT=$?" >> /tmp/build_out.txt
```

Output:
```
BUILD_EXIT=0
```

(No compiler output preceded the exit line — a clean build.)

**Result: BUILD OK (exit 0).**

## 3. `go vet ./...`

Command:
```
go vet ./... > /tmp/vet_out.txt 2>&1; echo "VET_EXIT=$?" >> /tmp/vet_out.txt
```

Output:
```
VET_EXIT=0
```

(No vet diagnostics preceded the exit line.)

**Result: VET OK (exit 0).**

## 4. `gofmt -l .`

Command:
```
gofmt -l . > /tmp/gofmt_out.txt 2>&1; echo "GOFMT_EXIT=$?" >> /tmp/gofmt_out.txt
```

Output:
```
GOFMT_EXIT=0
```

(No filenames listed — gofmt reported zero formatting violations. `gofmt -l`
itself always exits 0 regardless of findings; the empty file list before the
exit line is what confirms a clean tree.)

**Result: GOFMT CLEAN — no files listed.**

## 5. `timeout 300 go test ./...`

Command:
```
timeout 300 go test ./... > /tmp/test_out.txt 2>&1; echo "TEST_EXIT=$?" >> /tmp/test_out.txt
```

Full verbatim output:
```
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/applyport	0.975s
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/ota-device-emu	0.961s
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/ota-server	2.495s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	0.382s
?   	github.com/HelixDevelopment/helix_ota/server/internal/api/manager-dist	[no test files]
ok  	github.com/HelixDevelopment/helix_ota/server/internal/config	0.002s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/device	0.024s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/deviceemu	0.282s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/fabric	0.004s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/health	0.002s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/rollout	0.004s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/store	0.023s
ok  	github.com/HelixDevelopment/helix_ota/server/internal/transport	0.112s
ok  	github.com/HelixDevelopment/helix_ota/server/tests/chaos	0.026s
ok  	github.com/HelixDevelopment/helix_ota/server/tests/stress	0.010s
ok  	github.com/HelixDevelopment/helix_ota/server/tools/loadtest	1.404s
TEST_EXIT=0
```

No hang, no timeout — full suite completed in well under the 300s budget
(slowest package `cmd/ota-server` at 2.495s).

**Per-package result:**

| Package | Result | Time |
|---|---|---|
| cmd/applyport | ok | 0.975s |
| cmd/ota-device-emu | ok | 0.961s |
| cmd/ota-server | ok | 2.495s |
| internal/api | ok | 0.382s |
| internal/api/manager-dist | no test files | — |
| internal/config | ok | 0.002s |
| internal/device | ok | 0.024s |
| internal/deviceemu | ok | 0.282s |
| internal/fabric | ok | 0.004s |
| internal/health | ok | 0.002s |
| internal/rollout | ok | 0.004s |
| internal/store | ok | 0.023s |
| internal/transport | ok | 0.112s |
| tests/chaos | ok | 0.026s |
| tests/stress | ok | 0.010s |
| tools/loadtest | ok | 1.404s |

**Result: 15/15 tested packages PASS, 0 FAIL, 1 package (`internal/api/manager-dist`) has no test files** (not integration-gated — genuinely empty of `_test.go` files; a coverage gap, not a failure). No package required `-tags integration` or external infra to run — the full module tree ran under the default `go test ./...` invocation.

## 6. `timeout 300 go test -cover ./...`

Command:
```
timeout 300 go test -cover ./... > /tmp/cover_out.txt 2>&1; echo "COVER_EXIT=$?" >> /tmp/cover_out.txt
```

Full verbatim output:
```
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/applyport	1.437s	coverage: 24.4% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/ota-device-emu	1.444s	coverage: 0.0% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/cmd/ota-server	1.867s	coverage: 5.1% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	0.158s	coverage: 92.4% of statements
?   	github.com/HelixDevelopment/helix_ota/server/internal/api/manager-dist	[no test files]
ok  	github.com/HelixDevelopment/helix_ota/server/internal/config	0.002s	coverage: 100.0% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/device	0.042s	coverage: 92.6% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/deviceemu	0.265s	coverage: 97.7% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/fabric	0.004s	coverage: 100.0% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/health	0.002s	coverage: 100.0% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/rollout	0.004s	coverage: 28.2% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/store	0.005s	coverage: 47.9% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/transport	0.110s	coverage: 100.0% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/tests/chaos	0.025s	coverage: [no statements]
ok  	github.com/HelixDevelopment/helix_ota/server/tests/stress	0.010s	coverage: [no statements]
ok  	github.com/HelixDevelopment/helix_ota/server/tools/loadtest	1.383s	coverage: 63.3% of statements
COVER_EXIT=0
```

**Per-package coverage:**

| Package | Coverage |
|---|---|
| cmd/applyport | 24.4% |
| cmd/ota-device-emu | 0.0% |
| cmd/ota-server | 5.1% |
| internal/api | 92.4% |
| internal/api/manager-dist | no test files |
| internal/config | 100.0% |
| internal/device | 92.6% |
| internal/deviceemu | 97.7% |
| internal/fabric | 100.0% |
| internal/health | 100.0% |
| internal/rollout | 28.2% |
| internal/store | 47.9% |
| internal/transport | 100.0% |
| tests/chaos | no statements (test-only package) |
| tests/stress | no statements (test-only package) |
| tools/loadtest | 63.3% |

**Honest gap observations (stated as fact, not speculation):**
- `cmd/ota-device-emu` (0.0%), `cmd/ota-server` (5.1%), `cmd/applyport` (24.4%)
  are `main`-package command entry points — low coverage here is typical for
  `main.go` wiring code exercised mainly through integration/e2e paths rather
  than unit tests; this is a coverage gap, not a build/test failure.
- `internal/rollout` (28.2%) and `internal/store` (47.9%) are the two lowest
  non-main-package coverage figures and are candidates for additional unit
  test investment.
- `internal/api/manager-dist` has zero test files — flagged as a coverage gap
  requiring an honest tracked item, not a silent absence.

## Summary

| Check | Result |
|---|---|
| Go version | go1.26.4-X:nodwarf5 linux/amd64 |
| `go build ./...` | OK (exit 0) |
| `go vet ./...` | OK (exit 0) |
| `gofmt -l .` | CLEAN (no files listed) |
| `go test ./...` | PASS — 15/16 packages ok, 1 no test files, exit 0 |
| `go test -cover ./...` | exit 0; coverage ranges 0.0%–100.0% per package (see table) |

No hangs, no timeouts, no failures encountered during this baseline capture.
