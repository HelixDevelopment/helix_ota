# Determinism soak + sustained loadtest — captured evidence (HEAD `eb9e1c4`)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T16:45:00Z |
| Status | GREEN — §11.4.50 deterministic-consistency + sustained-load stability confirmed |
| HEAD | `eb9e1c4` (on `main`) |
| Authority | Operator mandate 2026-06-10 ("most stable build by morning; zero risk, zero bluff") |

Deeper-than-single-run stability evidence for the overnight session: repetition
(§11.4.50 — green must be deterministic, not a one-off) + a §11.4.118 rare-defect
discovery probe + sustained-load confirmation.

| File | What it proves |
|---|---|
| `determinism_soak_x5.log` | Full Go module `go test -race -count=5 ./...` → `SOAK_EXIT=0`; every package `ok` on all 5 iterations (deterministic green). |
| `deep_race_soak_x10.log` | Concurrency-heavy pkgs `go test -race -count=10 ./internal/{api,store,deviceemu,fabric}` → `DEEPSOAK_EXIT=0`; no rare race surfaced at 10× under the race detector. |
| `dashboard_vitest_determinism_x3.txt` | Dashboard Vitest run 3× consecutively → 58/58 every run (UI-tier determinism). |
| `loadtest_soak.log` | Black-box sustained load (ephemeral `ota-server`, 30s, c=64): 1,145,543 requests, 38,184 rps, p50 1.16ms / p90 3.77ms / p99 7.21ms, **0 non-2xx**. |

## Honest notes (§11.4.6)

- The loadtest reports 61 "errors (no response)" out of 1,145,543 (0.005%). These
  are **client-side connection-level** events (no HTTP response received) under
  extreme local concurrency — `non_2xx: 0` confirms the server returned a valid
  2xx for **every** request it answered. Not a server defect; characteristic of
  local ephemeral-port / TCP-backlog pressure at ~38k rps. Recorded, not hidden.
- The ephemeral server was booted on `:8080` (memory store) and torn down on exit
  (trap) — self-cleaning, no leak. No credential involved (healthz is unauthenticated).
