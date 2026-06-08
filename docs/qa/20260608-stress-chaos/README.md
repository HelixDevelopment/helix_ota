# QA evidence — stress + chaos (§11.4.85)

In-process black-box resilience suite over the real `Server.Router()`
(`server/internal/api/resilience_test.go`), run under `go test -race`.

- **Stress** — concurrent group-create (200 parallel, 0 errors), sustained reads
  (2400 requests across 16 workers, 0 errors), concurrent membership contention
  (60 distinct devices added to one group in parallel → exactly 60, no lost updates).
- **Chaos** — injected repository fault (`faultRepo`) → `GET /groups` degrades to
  500 gracefully (no panic), stays 500 under sustained fault, then **recovers** to
  200 when the fault clears.

`run.log` holds the captured per-test latency (p50/p95/p99) + error census +
the chaos recovery transition. `-race` clean ⇒ the concurrent paths are data-race-free.
Reproduce: `cd server && go test -race ./internal/api/ -run 'TestStress|TestChaos' -v`.
