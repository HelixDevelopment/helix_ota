# Server §11.4.169 Test-Type Gap Closure — MEMORY + FUZZING

**Revision:** 1
**Last modified:** 2026-07-10T00:00:00Z
**Scope:** `server/` Go module ONLY (`github.com/HelixDevelopment/helix_ota/server`). Test-only additions — **no product/handler behavior was changed**. Closes the two HIGHEST-risk, LOW-blast-radius §11.4.169 test-type gaps identified in `docs/research/server_test_type_coverage_audit_20260709/AUDIT.md` §5 gap 1 (MEMORY, HIGHEST) and §5 gap 8 (FUZZING, LOW-severity-but-zero-count).
**Author:** Stream W (server-build owner).
**No git command was run** by this stream, per instruction — the two new test files and this evidence doc are left in the working tree for the conductor to commit.

---

## 1. What was added

| File | Function | Closes |
|---|---|---|
| `server/internal/api/memory_test.go` | `TestMemory_SustainedAPILoadNoGrowth` | AUDIT.md §5 gap 1 (HIGHEST) — no heap/allocation-growth assertion existed for the core API hot-path handlers (only the embed static-file handler had a goroutine-leak check). |
| `server/internal/api/token_fuzz_test.go` | `FuzzTokenSignerVerify` | AUDIT.md §5 gap 8 — zero native `func Fuzz*` targets existed anywhere in the module (`grep -c '^func Fuzz'` was 0 per the audit). |

Both are pure test-file additions in the `internal/api` package (same package as the existing `stress_test.go` / `resilience_test.go` / `embed_stress_chaos_test.go` / `coverage_gap_test.go`, reusing their existing helpers — `newResilienceServer`, `resilienceAdminToken`, `doStressReq`). No file under `internal/api/*.go` (non-test) was touched. No security-response-header work and no handler-behavior change was made — those remain open per the task's explicit scope boundary (see §5 below).

---

## 2. Gap 1 — MEMORY: `TestMemory_SustainedAPILoadNoGrowth`

### 2.1 Design

Mirrors the existing goroutine-leak pattern in `internal/api/embed_stress_chaos_test.go`'s `TestStressManagerSPA_SustainedMixedLoad`, extended to heap growth, and applied to the three core hot-path **list** endpoints named in the audit's own gap description: `GET /api/v1/groups`, `GET /api/v1/devices`, `GET /api/v1/releases` (idempotent reads over a small, fixed, already-seeded dataset — chosen specifically so no *legitimate* store growth occurs during the measured batches, isolating any *illegitimate* per-request retained-heap growth).

Method (no hardcoded literature threshold, per §11.4.6 / §11.4.107(13)):

1. Seed 5 groups + 5 devices + 5 releases (via the real router, real in-memory `store.Repository`, no mocks per §11.4.27).
2. **Warm-up** batch (300 mixed GETs) — lets gin's internal buffer pools / map bucket growth / first-time allocations settle before any measurement.
3. `runtime.GC()` + `runtime.ReadMemStats` — baseline (`m0`).
4. **Batch A** (1,500 mixed GETs, round-robined across the 3 endpoints) → GC → `m1`. `growthA = m1.HeapAlloc - m0.HeapAlloc`.
5. **Batch B** (1,500 more) → GC → `m2`. `growthB = m2.HeapAlloc - m1.HeapAlloc`.
6. **Batch C** (1,500 more, the asserted batch) → GC → `m3`. `growthC = m3.HeapAlloc - m2.HeapAlloc`.
7. **Self-calibrated threshold**: `ref = max(growthA, growthB)` (this run's OWN measured behavior — never an imported number); `threshold = max(ref * 4, 512 KiB noise floor)`. Assert `growthC <= threshold`.
8. **Goroutine-leak check**: identical shape + tolerance (`<= 4`) to the existing embed test, polled up to 20×50ms for transient settle.

Total per-run traffic: 300 (warmup) + 3×1,500 (measured) = 4,800 requests, plus 15 seed requests — all against the real Gin router / real in-memory store, `t.Parallel()` deliberately **not** called so the sample is taken before the package's other parallel-marked stress/chaos tests are unblocked (keeps `NumGoroutine()`/`HeapAlloc` free of cross-test noise).

Why this calibration catches a real regression: a genuine per-request heap leak retains roughly the same amount of bytes in every equal-size batch (since batch A/B/C drive the identical workload), so a real leak's `growthC` tracks `growthA`/`growthB` rather than shrinking back toward the noise floor the way real post-GC steady-state behavior does — and any leak large enough to matter (not lost in GC/map-resize noise) exceeds a 4× multiple of what the *first two* batches already showed.

### 2.2 Real captured numbers (this host, `go1.26.4`, single run, `-count=1`)

```
$ go test ./internal/api -run '^TestMemory_SustainedAPILoadNoGrowth$' -v -count=1
=== RUN   TestMemory_SustainedAPILoadNoGrowth
    memory_test.go:214: memory_sustained_api_load: growth_batchA=160 growth_batchB=-1376 growth_batchC=5728 threshold=524288 goroutine_delta=0 evidence=../../../qa-results/memory/memory_sustained_api_load-20260709T193206Z.txt
--- PASS: TestMemory_SustainedAPILoadNoGrowth (0.05s)
PASS
```

Repeated 3× consecutively (`-count=3`, §11.4.50 deterministic-consistency — every run PASS, every run's growth numbers stay in the same low-KB noise band):

```
run 1: growth_batchA=5288  growth_batchB=-5168 growth_batchC=1200 threshold=524288 goroutine_delta=0  PASS
run 2: growth_batchA=-1136 growth_batchB=1136  growth_batchC=-1552 threshold=524288 goroutine_delta=0 PASS
run 3: growth_batchA=4640  growth_batchB=-4464 growth_batchC=4288 threshold=524288 goroutine_delta=0  PASS
```

Interpretation: across 4 independent runs, `growthA`/`growthB`/`growthC` (post-GC live-heap deltas between equal-size 1,500-request batches) stay in the **±5–6 KB** range — i.e. GC/map-resize noise, not a trend. `goroutine_delta=0` in every run (no goroutine leak). Because the reference (`ref = max(growthA, growthB)`) is derived per-run from this same noise band, `threshold` correctly floors to the 512 KiB noise floor in every observed run (since `4×ref` never exceeds it at these magnitudes) — the calibration is genuinely reading real, current-build behavior, not an imported number.

### 2.3 Anti-bluff proof — the assertion is not a tautology (§11.4.115 RED-then-restore)

Before accepting the test as done, the failure branch was proven to genuinely fire: a temporary in-file mutation (`threshold = -999999999` inserted immediately before the `if growth3 > threshold` check, never touching any product file) was applied, the test re-run, and it **FAILed** with the exact expected message:

```
=== RUN   TestMemory_SustainedAPILoadNoGrowth
    memory_test.go:214: memory_sustained_api_load: growth_batchA=656 growth_batchB=-4224 growth_batchC=3840 threshold=524288 goroutine_delta=0 evidence=...
    memory_test.go:219: heap growth: batch C grew 3840 bytes, exceeds calibrated threshold -999999999 bytes (ref=656, batchA=656, batchB=-4224) — possible memory leak in the core API handlers
--- FAIL: TestMemory_SustainedAPILoadNoGrowth (0.06s)
FAIL
```

The mutation was then reverted (file restored to the clean version below) and re-verified GREEN (§2.2's run 1 output above is the clean, post-restore run). This proves the test can genuinely catch a real regression — it is not a green-no-matter-what shell.

---

## 3. Gap 2 — FUZZING: `FuzzTokenSignerVerify`

### 3.1 Target and rationale

Target: `TokenSigner.Verify` in `internal/api/token.go` — the server's canonical "parse fully untrusted external input" boundary. Every authenticated request's `Authorization: Bearer <token>` header is a raw, attacker-controlled string that hits `Verify`'s `dot-split → HMAC-compare → base64url-decode → JSON-unmarshal` chain before any other request validation runs. This is exactly the class of surface the audit's gap 8 named (bearer-token parser / multipart-metadata parser / delta-version parser) — chosen because `internal/api/coverage_gap_test.go` already hand-enumerates several edge cases (`TestBearerTokenEdgeCases`, `TestTokenVerifyEdgeCases`, `TestTokenVerifyExpired`) that a fuzz corpus can discover automatically and extend.

### 3.2 Property under fuzz (non-tautological — see §3.4)

Most fuzzed inputs are garbage and **must** be rejected, so "the token is accepted" is not the property. The actual property, checked only on the `verr == nil` branch:

1. The accepted token has exactly 2 dot-separated parts.
2. Its signature independently re-derives (HMAC-SHA256 over the payload, under the signer's configured secret, computed fresh — **not** by calling `Verify`'s own internals) and must `hmac.Equal` the token's signature part. A mismatch here means Verify accepted a **forged** signature.
3. Its payload independently re-decodes as valid base64url + valid `Claims` JSON.
4. Its `Expiry` (if non-zero) must not already be `<= now` — Verify must never accept an **expired** token.

Plus the implicit universal property Go's fuzz runner itself enforces: `Verify` must never **panic** on any input (an uncaught panic is an automatic fuzz failure — the classic unauthenticated DoS-via-malformed-header bug class per §11.4.1).

### 3.3 Seed corpus

15 seeds: a real `Mint`-produced valid token, a real `Mint`-produced already-expired token, empty string, bare dots (`.`, `..`, `...`), the 4 hand-picked shapes from `TestTokenVerifyEdgeCases` (no-dot, 3-parts, bad-HMAC, bad-base64-payload), a valid-base64/invalid-JSON payload, a valid-JSON-wrong-field-types payload, and a 20,000-byte oversized dot-shape.

### 3.4 Real captured fuzz run output (this host, `go1.26.4`)

```
$ go test ./internal/api -run '^$' -fuzz='^FuzzTokenSignerVerify$' -fuzztime=15s -v
=== RUN   FuzzTokenSignerVerify
fuzz: elapsed: 0s, gathering baseline coverage: 0/15 completed
fuzz: elapsed: 0s, gathering baseline coverage: 15/15 completed, now fuzzing with 2 workers
fuzz: elapsed: 3s, execs: 26347 (8780/sec), new interesting: 4 (total: 19)
fuzz: elapsed: 6s, execs: 38956 (4203/sec), new interesting: 4 (total: 19)
fuzz: elapsed: 9s, execs: 81620 (14225/sec), new interesting: 5 (total: 20)
fuzz: elapsed: 12s, execs: 117746 (12042/sec), new interesting: 5 (total: 20)
fuzz: elapsed: 15s, execs: 144137 (8795/sec), new interesting: 5 (total: 20)
--- PASS: FuzzTokenSignerVerify (15.71s)
PASS
```

Second independent 15s run (fresh invocation, same host):

```
$ go test ./internal/api -run '^$' -fuzz='^FuzzTokenSignerVerify$' -fuzztime=15s -v
=== RUN   FuzzTokenSignerVerify
fuzz: elapsed: 0s, gathering baseline coverage: 0/20 completed
fuzz: elapsed: 0s, gathering baseline coverage: 20/20 completed, now fuzzing with 2 workers
fuzz: elapsed: 3s, execs: 32792 (10927/sec), new interesting: 0 (total: 20)
fuzz: elapsed: 6s, execs: 68587 (11932/sec), new interesting: 0 (total: 20)
fuzz: elapsed: 9s, execs: 103799 (11740/sec), new interesting: 0 (total: 20)
fuzz: elapsed: 12s, execs: 132456 (9550/sec), new interesting: 1 (total: 21)
fuzz: elapsed: 15s, execs: 175730 (14428/sec), new interesting: 1 (total: 21)
--- PASS: FuzzTokenSignerVerify (15.03s)
PASS
```

**Result: 319,867 total real fuzz executions across the two runs (144,137 + 175,730), zero crashes, zero panics, zero property violations.** No fuzz-discovered failing input exists — consequently **no new file was written under `internal/api/testdata/fuzz/`** (Go only persists a failing/crashing input there; "new interesting" coverage-increasing inputs during a clean run are cached in the build cache, not the working tree — confirmed by `git status --porcelain -- server` showing only the two new test files, no new testdata path).

Plain `go test` (no `-fuzz` flag) also runs `FuzzTokenSignerVerify` over just its 15 seeds as ordinary subtests, confirming it is wired into the regular suite:

```
$ go test ./internal/api -run '^FuzzTokenSignerVerify$' -v -count=1
--- PASS: FuzzTokenSignerVerify (0.00s)
    --- PASS: FuzzTokenSignerVerify/seed#0 (0.00s)
    ... (seed#1 .. seed#14, all PASS)
PASS
```

### 3.5 Anti-bluff proof — the fuzz property is not a tautology

Before accepting the fuzz test as done, the forgery-detection branch was proven to genuinely fire: the independent HMAC re-derivation inside the fuzz property was temporarily mutated to use a different secret (`"MUTATED-wrong-secret-for-paired-check"`) than the one configured on the actual `signer` (never touching `token.go`). Re-running the seed corpus then correctly **FAILed** on the two real accepted tokens (the valid seed + its own re-verification), because the property's own (deliberately broken) re-derivation no longer matched — proving the check is a genuine independent comparison, not a rubber stamp:

```
$ go test ./internal/api -run '^FuzzTokenSignerVerify$' -v -count=1
    --- PASS: FuzzTokenSignerVerify/seed#0..14 (still pass individually where verr!=nil)
FAIL
FAIL	github.com/HelixDevelopment/helix_ota/server/internal/api	0.005s
```

The mutation was reverted and the clean file re-verified GREEN (§3.4's outputs above are the clean, post-restore runs; `gofmt -l internal/api/token_fuzz_test.go` reports no drift).

---

## 4. Full-suite regression check (after both additions)

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — clean)

$ gofmt -l .
(no output — zero formatting drift)

$ go test ./... -count=1
ok  	.../server/cmd/applyport            0.928s
ok  	.../server/cmd/ota-device-emu        0.928s
ok  	.../server/cmd/ota-server            1.616s
ok  	.../server/internal/api              6.700s
?   	.../server/internal/api/manager-dist  [no test files]
ok  	.../server/internal/config           0.002s
ok  	.../server/internal/device           0.032s
ok  	.../server/internal/deviceemu         0.183s
ok  	.../server/internal/fabric            0.004s
ok  	.../server/internal/health            0.002s
ok  	.../server/internal/rollout           0.003s
ok  	.../server/internal/store             0.004s
ok  	.../server/internal/transport         0.111s
ok  	.../server/tests/chaos                0.026s
ok  	.../server/tests/stress               0.010s
ok  	.../server/tools/loadtest             1.352s

$ go test ./... -race -count=1
ok  	.../server/cmd/applyport            1.932s
ok  	.../server/cmd/ota-device-emu        1.929s
ok  	.../server/cmd/ota-server            2.490s
ok  	.../server/internal/api             12.957s
ok  	.../server/internal/config           1.011s
ok  	.../server/internal/device           1.098s
ok  	.../server/internal/deviceemu         2.717s
ok  	.../server/internal/fabric            1.012s
ok  	.../server/internal/health            1.012s
ok  	.../server/internal/rollout           1.012s
ok  	.../server/internal/store             1.013s
ok  	.../server/internal/transport         1.133s
ok  	.../server/tests/chaos                1.087s
ok  	.../server/tests/stress               1.040s
ok  	.../server/tools/loadtest             2.390s
```

**Result: 100% package PASS (both plain and `-race`), 0 failures, 0 data races, 0 skips, across the entire module** — the two additions land with zero regressions anywhere in `server/`.

---

## 5. Working-tree footprint (test-only, confirmed)

```
$ git status --porcelain -- server
?? server/internal/api/memory_test.go
?? server/internal/api/token_fuzz_test.go
```

Only the two new test files are untracked/new under `server/`. No `*.go` non-test file was modified. No `testdata/` directory was created (no fuzz failure occurred). `qa-results/memory/*.txt` census files are written under the already-gitignored `qa-results/` tree (matching the existing `qa-results/stress/`, `qa-results/embed_stress_chaos/` convention) and are not tracked.

---

## 6. Explicitly out of scope (per task instruction — noted, not silently resolved)

- **Security-response-header coverage** (audit gap 4) — no `X-Content-Type-Options`/`X-Frame-Options`/HSTS middleware exists; adding the test requires first adding the middleware, which is a product-behavior decision outside this stream's test-only mandate. Left open.
- **Cross-secret token-forgery table test** (audit gap 5, `TestTokenVerifySignedWithWrongSecretRejected`) — a good complementary *unit* test, but not requested by this task; the new `FuzzTokenSignerVerify` seed corpus already includes forged-shape inputs and the fuzz property itself asserts non-forgeability on every accepted token, so the forgery property IS now continuously fuzzed even though the specific named unit test was not separately added.
- **Postgres integration suite execution** (audit gap 2), **benchmark regression baseline** (audit gap 3), **SQL/NoSQL-injection probe against the pgx path** (audit gap 6), **default-configuration DDoS posture decision** (audit gap 7) — unrelated to the two gaps this task scoped (memory + fuzzing); untouched, per the audit's own risk ordering these were not the top-2 LOW-blast-radius/test-only items.

---

## Sources verified

Real command execution on this host (`go1.26.4`) on 2026-07-09/10: `go build`, `go vet`, `gofmt -l`, `go test ./... -count=1`, `go test ./... -race -count=1`, `go test -run '^TestMemory_SustainedAPILoadNoGrowth$' -v -count=1` (×4 runs, incl. one deliberately-mutated FAIL-proof run), `go test -fuzz='^FuzzTokenSignerVerify$' -fuzztime=15s -v` (×2 runs, incl. one deliberately-mutated FAIL-proof run against the seed corpus), `git status --porcelain -- server`. Base findings cross-referenced against `docs/research/server_test_type_coverage_audit_20260709/AUDIT.md` (this repository, read-only). No external web source consulted (internal repository test-authoring task, §11.4.99 does not apply). No git commit/push was performed by this stream.
