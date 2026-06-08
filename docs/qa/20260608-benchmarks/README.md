# §11.4.27 benchmarking suite — measured ns/op

Go `testing.B` micro-benchmarks (real measured ns/op + B/op + allocs/op), the
benchmarking test-type §11.4.27 requires. Complements the HTTP-level loadtest
(throughput) + stress/chaos (resilience) with per-operation cost.

Run: `cd server && go test -bench=. -benchmem -run='^$' ./internal/api/ ./internal/store/`

## Captured (this host, see bench.log for the full run)
| Benchmark | ns/op | allocs/op |
|---|---|---|
| Healthz (full router GET /healthz) | ~2,489 | 32 |
| GroupCreate (POST /groups) | ~8,537 | 79 |
| GroupList (GET /groups, 20 seeded) | ~14,299 | 78 |
| ClientUpdate no-deployment (auth device → 204) | ~4,915 | 49 |
| MemoryCreateGroup | ~666 | 4 |
| MemoryFindDelta (100 seeded) | ~160 | **0** |
| MemoryListAudit (200 seeded, limit 50) | ~16,331 | 10 |

Numbers are this-host measurements (Apple Silicon), not production guarantees —
they are a baseline to catch regressions, captured per §11.4.6 (measured, not asserted).
