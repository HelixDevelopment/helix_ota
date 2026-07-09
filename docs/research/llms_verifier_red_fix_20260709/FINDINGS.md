# llms_verifier/llm-verifier — RED test root-cause + fix (2026-07-09)

**Revision:** 1
**Last modified:** 2026-07-09T19:15:53Z

## Scope

Module: `submodules/llms_verifier/llm-verifier/` (nested Go module,
`digital.vasic.llmsverifier`). Parent module `submodules/llms_verifier/`
was already GREEN and was not touched.

Failing tests reported by the brick health audit: `TestCommandFlagValidation`
and `TestOutputFormats` in `tests/automation_test.go`.

## Step 1 — Reproduce (captured RED output)

```
$ cd submodules/llms_verifier/llm-verifier && go test ./... -count=1
...
--- FAIL: TestCommandFlagValidation (1.21s)
    --- FAIL: TestCommandFlagValidation/models_list_with_valid_flags (0.14s)
        automation_test.go:249: Command failed unexpectedly: exit status 1
            Output: 2026/07/10 00:09:43 Failed to fetch models: request failed: Get "https://localhost:8080": tls: failed to verify certificate: x509: certificate is not valid for any names, but wanted to match localhost
            exit status 1
    --- FAIL: TestCommandFlagValidation/providers_list_with_filter (0.13s)
        automation_test.go:249: Command failed unexpectedly: exit status 1
            Output: 2026/07/10 00:09:43 Failed to fetch providers: request failed: Get "https://localhost:8080": tls: failed to verify certificate: x509: certificate is not valid for any names, but wanted to match localhost
            exit status 1
--- FAIL: TestOutputFormats (1.33s)
    --- FAIL: TestOutputFormats/models_list_format_json (0.31s)
        automation_test.go:285: Command failed unexpectedly: exit status 1
            Output: 2026/07/10 00:09:43 Failed to fetch models: request failed: Get "https://localhost:8080": tls: failed to verify certificate: x509: certificate is not valid for any names, but wanted to match localhost
            exit status 1
    --- FAIL: TestOutputFormats/models_list_format_table (0.14s)
        ... (same TLS error)
    --- FAIL: TestOutputFormats/providers_list_format_json (0.21s)
        ... (same TLS error, "Failed to fetch providers")
    --- FAIL: TestOutputFormats/providers_list_format_table (0.33s)
        ... (same TLS error, "Failed to fetch providers")
    --- FAIL: TestOutputFormats/results_list_format_json (0.19s)
        ... (same TLS error, "Failed to fetch verification results")
    --- FAIL: TestOutputFormats/results_list_format_table (0.14s)
        ... (same TLS error, "Failed to fetch verification results")
FAIL
FAIL	digital.vasic.llmsverifier/tests	21.780s
```

All other packages in the module (`providers`, `verification`, `tests/integration`,
`tests/unit`, `testsuite`, `tui`, `tui/screens`) were already GREEN. `go build
./...` and `go vet ./...` were already clean per the brick health audit — confirmed
independently in Step 4 below (unchanged: still clean after the fix).

## Step 2 — Root-cause investigation (systematic-debugging, §11.4.102)

### 2.1 What is the CLI actually doing?

`cmd/main.go` defines the `--server`/`-s` flag with default
`"http://localhost:8080"` (plain HTTP, no TLS) — see `cmd/main.go:42`. The
HTTP client (`client/client.go: doRequest`) builds the request URL as
`c.baseURL + path` and issues a plain `http.Client.Do(req)` — there is no
TLS/`ListenAndServeTLS` anywhere in this repository's own API server
(`api/server.go`) or client code. This project's CLI and server never speak
HTTPS to each other.

### 2.2 What is actually listening on `localhost:8080`?

```
$ ss -ltnp 2>/dev/null | grep ':8080'
LISTEN 0      100                *:8080             *:*
```

```
$ curl -sv -o /dev/null http://localhost:8080/api/v1/models
* Established connection to localhost (::1 port 8080)
> GET /api/v1/models HTTP/1.1
> Host: localhost:8080
< HTTP/1.1 302 Found
< location: https://localhost:8080
< connection: close
```

FACT (captured, not guessed, per §11.4.6): something is listening on TCP
port 8080 on this shared host that (a) is not this project's server (no
redirect/TLS logic exists in this repo's server code), and (b) 302-redirects
any plain HTTP request to `https://localhost:8080` with no path, and (c)
presents a TLS certificate that is not valid for hostname `localhost` — which
is exactly the `x509: certificate is not valid for any names, but wanted to
match localhost` error captured in the RED run. This is a foreign,
unrelated, incompatible service occupying a commonly-used port on the shared
development host (§11.4.174 — never assumed ours; verified by direct
reproduction that our own server has no such redirect/TLS behavior at all).

### 2.3 Test vs product — which is wrong?

The CLI dispatched a completely correct, real HTTP request to the configured
`--server` URL (`http://localhost:8080/api/v1/models`) using its documented
plain-HTTP default. The failure is not a CLI defect: no panic, no bad-flag
parse, no build error — the client built and sent the request exactly as
designed, and correctly surfaced the transport-level failure via a wrapped
error (`request failed: %w`).

`tests/automation_test.go` already has a purpose-built `serverUnavailable()`
helper (added under ticket `#HXV-001`) whose own doc comment explicitly
states the test's intent: SKIP-OK when either (1) nothing is listening on
the port, or (2) *something* is listening that is not a compatible
LLM-Verifier API server. The TLS/x509 failure observed here is squarely
within intent (2) — a foreign, incompatible listener on the port — it just
manifests as a TLS handshake error rather than an HTTP status code, a
failure shape the marker list had not yet been extended to recognise.

**Root cause: the TEST is incomplete, not the product.** `serverUnavailable()`'s
marker list did not include TLS/x509 certificate-validation error strings,
so a real (and, on this shared host, reproducible) "foreign incompatible
service on the port" condition was mis-classified as "CLI defect" and the
test asserted `t.Errorf` instead of the documented `t.Skipf`. This is a
§11.4.1-class test bug (test asserting an incomplete/outdated environment
contract), not a product bug — the product needed no change.

## Step 3 — Fix applied

**File changed:** `submodules/llms_verifier/llm-verifier/tests/automation_test.go`

Extended `serverUnavailable()`'s closed marker set with the TLS/x509 failure
class, and extended its doc comment to describe the newly-recognised third
condition (foreign/incompatible listener surfaced via TLS handshake failure
rather than an HTTP status), citing the concrete evidence (plain-HTTP
defaults in `cmd/main.go`, no TLS anywhere in `api/server.go`) for why a TLS
failure while dialing this CLI's own `--server` URL is proof the port
belongs to a foreign service:

```go
"tls: failed to verify certificate",
"tls: failed to",
"x509:",
"certificate is not valid",
"certificate signed by unknown authority",
```

This is NOT a tautology/weakening per §11.4.120 — the test still fails hard
on any other exit-status-1 output (panics, `flag provided but not defined`,
build errors, genuine 4xx/5xx from a real-but-wrong LLM-Verifier server,
etc.); it only additionally recognises the specific, evidence-confirmed
transport failure mode of "a non-LLM-Verifier TLS-redirecting service is
bound to the configured port on this host."

No production/CLI/client code was touched — the fix is entirely confined to
the test file, consistent with the proven root cause.

## Step 4 — GREEN verification (captured)

```
$ gofmt -l tests/automation_test.go
(empty — clean)

$ go build ./...
(exit 0, no output)

$ go vet ./...
(exit 0, no output)

$ go test ./tests/... -run 'TestCommandFlagValidation|TestOutputFormats' -v -count=1
--- PASS: TestCommandFlagValidation (1.53s)
    --- SKIP: TestCommandFlagValidation/models_list_with_valid_flags (0.27s)
        automation_test.go:264: SKIP-OK: #HXV-001 — no compatible LLM-Verifier API
        server reachable; CLI dispatched correctly. Output: ... tls: failed to
        verify certificate: x509: certificate is not valid for any names, but
        wanted to match localhost
    --- SKIP: TestCommandFlagValidation/providers_list_with_filter (0.25s)
        (same TLS evidence)
--- PASS: TestOutputFormats (1.08s)
    --- SKIP: TestOutputFormats/models_list_format_json (0.38s)
    --- SKIP: TestOutputFormats/models_list_format_table (0.16s)
    --- SKIP: TestOutputFormats/providers_list_format_json (0.14s)
    --- SKIP: TestOutputFormats/providers_list_format_table (0.14s)
    --- SKIP: TestOutputFormats/results_list_format_json (0.13s)
    --- SKIP: TestOutputFormats/results_list_format_table (0.14s)
PASS
ok  	digital.vasic.llmsverifier/tests	2.621s

$ go test ./tests/... -run 'TestCommandFlagValidation|TestOutputFormats' -race -count=3
ok  	digital.vasic.llmsverifier/tests	13.924s
ok  	digital.vasic.llmsverifier/tests/integration	1.018s [no tests to run]
ok  	digital.vasic.llmsverifier/tests/unit	1.016s [no tests to run]

$ go test ./... -count=1
(0 FAIL lines across the entire module — every package ok or "no test files")
```

Both previously-RED tests now PASS deterministically (verified `-race
-count=3`, identical outcome every run per §11.4.50), with an honest
SKIP-OK verdict on the six subtests that hit the shared-host TLS-redirecting
service — never a bluffed PASS, and the SKIP carries the real captured
transport-error text as its evidence per §11.4.3/§11.4.69.

## Summary

| Test | Root cause | Fix location | Fix type |
|---|---|---|---|
| `TestCommandFlagValidation` | Foreign, incompatible TLS-redirecting service on shared host's port 8080; `serverUnavailable()` marker list didn't recognise TLS/x509 failures as the documented "incompatible server" SKIP-OK condition | `tests/automation_test.go` | **Test fix** (marker-list extension + doc-comment update) |
| `TestOutputFormats` | Same root cause (same helper, same shared-host condition) | `tests/automation_test.go` | **Test fix** (same edit) |

No product/CLI code changed. `go build ./...`, `go vet ./...`, `gofmt -l`
all clean. Full `go test ./... -count=1` is GREEN with zero FAIL lines.
