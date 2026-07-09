# Server Security-Header Test-Adequacy Gap Closure — SR-review I2/I3

**Revision:** 1
**Last modified:** 2026-07-10T00:00:00Z

Scope: `server/internal/api/security_headers_test.go` only (test file). No
production (non-test) source file was modified in the final state. Closes
findings I2 and I3 from
`docs/research/security_headers_adversarial_review_20260710/REVIEW.md`, per
§11.4.134 (iterate-to-adequate) and §11.4.115 (RED-baseline / polarity
anti-bluff for every new guard test).

---

## What was closed

- **I2 — CLOSED (full closure, not the "at minimum" narrowed fallback).**
  Added `TestSecurityHeaders_TierA_OnShed429_RealRouter`, which forces a
  deterministic 429 shed **through the real `Server.Router()` chain**
  (`NewServer(Options{Config: config.Config{MaxInflight: 1, ...}}).Router()`),
  not a hand-rolled two-middleware stand-in, and asserts the Tier-A headers
  survive the shed.
- **I3 — CLOSED.** Added `TestSecurityHeaders_TierA_OnPanic500_RealRouter`,
  which drives a genuine panic through the real `Server.Router()` chain
  (`recoveryMiddleware` → … → `securityHeadersMiddleware` → … → handler) and
  asserts the Tier-A headers survive the recovered 500.

Both new tests are **anti-bluff per §11.4.115**: each was proven to FAIL when
the exact property it guards is broken, via a RED demonstration (below), then
the RED-inducing code was deleted (it was never part of the permanent suite).

---

## The seam (and why it is production-file-safe)

Neither new test required a build-tag-gated route, a panic-route hook, or any
other addition to `server.go` / `security_headers.go`. The seam is a
**test-file-only technique** built on gin's own public API and its documented
composition behaviour, verified against the real vendored gin v1.12.0 source
(`/home/milos/go/pkg/mod/github.com/gin-gonic/gin@v1.12.0/routergroup.go`):

- `Router()` calls `r.Use(recoveryMiddleware(), requestIDMiddleware(),
  securityHeadersMiddleware(s.tlsEnabled()), maxInflightMiddleware(s.cfg.MaxInflight),
  compressionMiddleware())` (`server.go:122-123`) **before** it returns the
  `*gin.Engine`. `RouterGroup.Use` (`routergroup.go:63-66`) appends those five
  middlewares to `group.Handlers`.
- Any route registered on that group **afterward** — `r.GET(path, handler)` —
  goes through `RouterGroup.handle` (`routergroup.go:86-91`), which calls
  `combineHandlers` (`routergroup.go:241-248`). `combineHandlers` concatenates
  the group's **currently-registered** `Handlers` slice with the new route's
  handler(s):

  ```go
  func (group *RouterGroup) combineHandlers(handlers HandlersChain) HandlersChain {
      finalSize := len(group.Handlers) + len(handlers)
      mergedHandlers := make(HandlersChain, finalSize)
      copy(mergedHandlers, group.Handlers)
      copy(mergedHandlers[len(group.Handlers):], handlers)
      return mergedHandlers
  }
  ```

- Therefore a route attached to the `*gin.Engine` **returned by
  `srv.Router()`**, from a test file, in the same package (`api`), inherits
  the **exact production middleware chain and order** — identical to what
  every real route (`/healthz`, `/api/v1/devices`, …) gets. It is not a copy
  or approximation of the production order; it *is* the production order,
  because `Handlers` at that point in time already contains it.
- This means a **future reorder of `r.Use(...)` in `server.go`** (e.g.
  `maxInflightMiddleware` before `securityHeadersMiddleware`, the exact
  regression I2 warns about) changes what `TestSecurityHeaders_TierA_OnShed429_RealRouter`
  and `TestSecurityHeaders_TierA_OnPanic500_RealRouter` exercise too — closing
  the gap SR-review I2 identified, with **zero bytes added to any
  non-test file**.
- `MountManagerUI`'s `/manager` group and `NoRoute` fallback (`embed.go`)
  register a distinct, non-conflicting path prefix, so attaching
  `/__red_review_i2_block` and `/__red_review_i3_panic` at the engine root
  does not collide with the SPA routes or the route tree.

No build tag, no conditional compilation, no test-only registration hook was
added to `server.go` or `security_headers.go`. The "seam" is entirely
contained in the test file already in scope
(`server/internal/api/security_headers_test.go`).

---

## New tests

### `TestSecurityHeaders_TierA_OnShed429_RealRouter` (I2)

Builds a real `Server` via `NewServer` with `Config.MaxInflight: 1` (the
`newTestEnv` helper used by most other tests in the file leaves
`MaxInflight` at its zero value, under which `maxInflightMiddleware` is a
permanent no-op — this is why no other existing test could ever trigger a
real shed against production wiring, per the SR review). Calls `srv.Router()`
to get the real engine, attaches a test-only blocking route directly to it,
drives two concurrent requests (the first holds the single in-flight slot,
the second is shed with 429), and asserts:

- the shed response status is 429
- **all seven Tier-A headers** are present with their exact expected values
  (`assertTierA`)
- `Retry-After: 1` is present

### `TestSecurityHeaders_TierA_OnPanic500_RealRouter` (I3)

Reuses the existing `newSecHeadersRouter(t, "", "")` helper (already builds
the real `srv.Router()`), attaches a test-only route that unconditionally
panics, and asserts:

- the response status is 500 (i.e. `recoveryMiddleware` genuinely recovered
  the panic through the real chain)
- all seven Tier-A headers are present with their exact expected values

---

## RED demonstration (§11.4.115 anti-bluff)

Each new test was proven non-vacuous by constructing, in a **temporary
scratch test file** (`server/internal/api/zz_red_demo_scratch_test.go` —
created, run once, then deleted; never part of the permanent suite and never
committed), the exact broken property each new test guards against, and
confirming the assertion FAILs. No production (non-test) file was ever
touched for this — the mutation lives entirely in the scratch test's own
locally-constructed `gin.Engine`, using the same unexported middleware
constructors the test file already has package-level access to.

- **I2 RED — `TestREDDEMO_I2_ShedLosesHeadersWhenMaxInflightRunsFirst`.**
  Built `r.Use(maxInflightMiddleware(1), securityHeadersMiddleware(false))` —
  the exact reversed order (`maxInflight` before `securityHeaders`) that
  SR-review I2 warns a future `server.go` reorder could introduce
  undetected. Result: **FAIL** — 7/7 Tier-A headers absent on the 429 shed,
  because `maxInflightMiddleware` aborts the request before
  `securityHeadersMiddleware` ever runs. See `red_demo_run.log`.
- **I3 RED — `TestREDDEMO_I3_PanicBeforeHeadersSetLosesHeaders`.** Built
  `r.Use(recoveryMiddleware(), <panicking middleware>, securityHeadersMiddleware(false))`
  — a panic occurring *before* `securityHeadersMiddleware` runs, while
  `recoveryMiddleware` still recovers it (proving the FAIL is specifically
  about header timing, not about recovery itself). Result: **FAIL** — 7/7
  Tier-A headers absent on the recovered 500. See `red_demo_run.log`.

Captured RED output (`red_demo_run.log`, `go test -run REDDEMO -count=1 -v
./internal/api/`):

```
=== RUN   TestREDDEMO_I2_ShedLosesHeadersWhenMaxInflightRunsFirst
    zz_red_demo_scratch_test.go:51: 429 shed (MUTATED order: maxInflight before securityHeaders): header "Cross-Origin-Opener-Policy" = "", want "same-origin"
    ... (7/7 Tier-A headers reported absent) ...
--- FAIL: TestREDDEMO_I2_ShedLosesHeadersWhenMaxInflightRunsFirst (0.00s)
=== RUN   TestREDDEMO_I3_PanicBeforeHeadersSetLosesHeaders
    zz_red_demo_scratch_test.go:72: panic -> 500 (MUTATED: panic before securityHeaders): header "Referrer-Policy" = "", want "no-referrer"
    ... (7/7 Tier-A headers reported absent) ...
--- FAIL: TestREDDEMO_I3_PanicBeforeHeadersSetLosesHeaders (0.00s)
FAIL
```

Full output preserved verbatim in `red_demo_run.log`. After capture, the
scratch file was deleted (`rm
server/internal/api/zz_red_demo_scratch_test.go`); it never touched any
non-test file and left zero trace in the working tree.

This proves both new GREEN tests are load-bearing: each one FAILs the moment
the real production property it claims to guard is broken, satisfying
§11.4.115 and the §11.4.134 "would a paired mutation make this test FAIL?"
bar.

---

## Build / vet / gofmt / test results

### `go build ./...` — exit 0, no output. See `build.log`.

### `go vet ./...` — exit 0, no output. See `vet.log`.

### `gofmt -l .` — see `gofmt.log`.

```
internal/api/memory_test.go
```

This is a **pre-existing** formatting gap in a file this work stream did not
touch (`internal/api/memory_test.go`, last modified at commit `1df9a649`,
unrelated to security headers). `gofmt -l` run scoped to the files this work
stream edited (`security_headers_test.go`, plus the unmodified
`security_headers.go` / `server.go` for completeness) is clean:

```
$ gofmt -l internal/api/security_headers_test.go internal/api/security_headers.go internal/api/server.go
(no output — all three clean)
```

Flagging `memory_test.go` here for conductor visibility rather than fixing it
silently — it is outside this work stream's scope (security-header test
seams only) per §11.4.122 (no silent changes outside the assigned scope).

### `go test -run 'SecurityHeaders' -count=1 -v ./internal/api/` — 9/9 PASS.

See `targeted_test_run.log`. All nine `TestSecurityHeaders_*` tests pass,
including the two new `_RealRouter` tests:

```
--- PASS: TestSecurityHeaders_TierA_OnEveryResponseClass (0.00s)
--- PASS: TestSecurityHeaders_TierA_OnShed429 (0.00s)
--- PASS: TestSecurityHeaders_TierA_OnShed429_RealRouter (0.00s)
--- PASS: TestSecurityHeaders_TierA_OnPanic500_RealRouter (0.00s)
--- PASS: TestSecurityHeaders_HSTS_TLSGated (0.00s)
--- PASS: TestSecurityHeaders_TierB_ScopedToAPI (0.00s)
--- PASS: TestSecurityHeaders_TierB_NotOnSPAAssets (0.00s)
--- PASS: TestSecurityHeaders_TierC_SPADocumentCSP (0.00s)
--- PASS: TestSecurityHeaders_TierC_UpgradeInsecureWhenTLS (0.00s)
PASS
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	0.010s
```

(`TestSPACSP_CompatibleWithRealBundle` does not match the `SecurityHeaders`
run filter by name; it is exercised in the full-package run below and was
unaffected by this change.)

### `go test ./internal/api/ -race -count=1 -v` — 242/242 PASS, 0 FAIL, 0 SKIP.

See `full_race_run.log` (full verbatim output, 12.847s). Summary line:

```
PASS
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	12.847s
```

`grep -c '^--- PASS'` → 242; `grep -c '^--- FAIL'` → 0; `grep -c '^--- SKIP'`
→ 0. No data race reported by `-race` across the full package, including the
two new concurrency-bearing tests (the shed test runs a real goroutine
holding the in-flight slot while the main goroutine issues the shedding
request).

---

## Files touched

- `server/internal/api/security_headers_test.go` — added
  `TestSecurityHeaders_TierA_OnShed429_RealRouter` and
  `TestSecurityHeaders_TierA_OnPanic500_RealRouter`; updated the file's top
  doc-comment and the pre-existing `TestSecurityHeaders_TierA_OnShed429`
  doc-comment to accurately describe which tests use the real router vs. the
  hand-composed minimal chain (the prior comment's blanket "these tests
  exercise the REAL wired router... not a hand-rolled stand-in" claim was
  itself inaccurate for that one test — corrected here rather than left
  standing).
- **No other file was modified in the final state.** The RED-demonstration
  scratch file (`zz_red_demo_scratch_test.go`) was created, run, and deleted;
  it does not exist in the working tree post-evidence-capture.

## What was NOT done (honest scope note)

- I2 was closed **fully** (real deterministic shed through the real router),
  not via the "at minimum, assert production middleware order" fallback the
  task description offered as a lesser outcome — the fallback was
  unnecessary because gin's own route-registration semantics already made the
  full closure straightforward with no seam in production files.
- No production (non-test) file changes are proposed or needed for I2/I3.
  Nothing here requires conductor review of a production-file seam, because
  none was added.
- The `memory_test.go` gofmt gap is pre-existing and out of scope; flagged,
  not fixed, per the assigned scope boundary.
