# loadtest — Helix OTA load / NFR measurement harness

A standalone, black-box HTTP load generator for the Helix OTA control plane.

**Anti-bluff (Constitution §11.4):** every number this tool prints
(p50 / p90 / p99 latency, RPS, error count) is **MEASURED** from real
request/response round-trips against a live endpoint. This harness asserts
**no** NFR target — it does not claim "the server does X req/s" or "latency is
under Y ms". It reports what it observed, and nothing else. NFR targets are
evaluated by comparing these measured numbers against the project's stated
targets **outside** this tool.

## Design constraints

- **Standard library only.** Imports nothing from
  `github.com/HelixDevelopment/helix_ota/server/internal/*`. It speaks plain
  HTTP like any external client, so it compiles and runs independently of
  concurrent server edits.
- **Black box.** No knowledge of server internals; pointed at a URL.

## Build

From `server/`:

```bash
go build ./tools/loadtest/
go vet ./tools/loadtest/
```

## Run against a running ota-server

Start the server, then:

```bash
go run ./tools/loadtest/ \
  -url http://127.0.0.1:8080 \
  -path /healthz \
  -concurrency 50 \
  -duration 10s
```

Or build a binary:

```bash
go build -o /tmp/loadtest ./tools/loadtest/
/tmp/loadtest -url http://127.0.0.1:8080 -path /healthz -concurrency 100 -duration 30s
```

### Flags

| Flag           | Default                 | Meaning                                            |
| -------------- | ----------------------- | -------------------------------------------------- |
| `-url`         | `http://127.0.0.1:8080` | Base URL of the running ota-server                 |
| `-path`        | `/healthz`              | Request path appended to `-url`                    |
| `-concurrency` | `50`                    | Number of concurrent worker goroutines             |
| `-duration`    | `10s`                   | How long to apply load (e.g. `10s`, `1m`)          |
| `-timeout`     | `30s`                   | Per-request timeout                                |
| `-selftest`    | `false`                 | Measure a throwaway in-process 200-OK server       |

## Output

- **stdout:** the measured `report` as indented JSON (machine-readable, for CI).
- **stderr:** a human-readable table plus, in `-selftest` mode, a status line.

Fields: `total_requests`, `errors` (no response received),
`non_2xx` (response received but status outside 200–299),
`requests_per_second`, and `min/mean/p50/p90/p99/max` latency in ms.

## Self-test (real evidence, zero external dependencies)

```bash
go run ./tools/loadtest/ -selftest -concurrency 20 -duration 3s
```

This spins up a throwaway in-process `httptest.Server` returning `200 OK`,
points the harness at it, and prints **real** measured percentiles. It proves
the harness compiles and produces genuine measurements end-to-end. Captured
output of a real self-test run is in [`BUILD_EVIDENCE.txt`](./BUILD_EVIDENCE.txt).

## Interpreting results vs NFR targets

This tool gives you the measured numbers. To check an NFR target (e.g. "p99
under 50 ms at 100 concurrent clients"), run the tool against a real ota-server
under that concurrency and compare the printed `p99_ms` to the target. The
comparison/judgement is intentionally **not** baked into the tool, so the tool
can never report a PASS it did not actually measure.
