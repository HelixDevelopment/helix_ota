# QA evidence — rollout pgx StoragePort

| Field | Value |
|---|---|
| Run id | 20260608-rollout-pgx-storageport |
| Feature | `rollout.PostgresStore` — pgx-backed `engine.StoragePort` for the staged-rollout engine (production persistence) |
| Date | 2026-06-08 |
| Evidence | [`run.log`](run.log) — real `go test -tags integration -count=1` |

## What this proves (anti-bluff, Constitution §11.4)

- A **real PostgreSQL** is booted on-demand via the `containers` submodule on
  podman (`boot summary: started=1 failed=0`) — never a manual step, never a fake.
- The pgx `PostgresStore` passes the **same** `runStorageScenario` the in-memory
  store passes (shared helper), proving brick-port parity: create → start (with a
  persisted Load round-trip of state + the immutable 2-phase plan) → healthy
  advance → healthy complete, and an independent rollout HALTs on an error breach
  — all read back from the DB.
- Distinct host port (55445) from the store integration test (55432) so both
  integration packages boot concurrently under `go test -tags integration`.

## Reproduce

```
cd server
go test -tags integration -count=1 ./internal/rollout/ -run TestPostgresStoreScenario_Integration -v
```
Requires podman/docker + `podman compose`. Compose file: `server/deploy/postgres-rollout.compose.yml`.
