# QA evidence — pgx PostgreSQL Repository (containerized integration)

| Field | Value |
|---|---|
| Run id | 20260608-pgx-postgres-integration |
| Feature | `store.PostgresRepository` (pgx) implementing `store.Repository`, the production persistence seam |
| Date | 2026-06-08 |
| Evidence | [`run.log`](run.log) — real `go test -tags integration -count=1` output |

## What this proves (anti-bluff, Constitution §11.4)

- A **real PostgreSQL** is booted on-demand through the `containers` submodule
  (`digital.vasic.containers` → `pkg/boot` + `pkg/compose` + `pkg/health`) on the
  **podman** runtime — not a manual `podman`/`compose` step, not a fake/mock
  (`boot summary: started=1 … failed=0`).
- The pgx `PostgresRepository` passes the **exact same** behavioural contract
  (`runRepositoryContract`) that the in-memory repository passes — proving
  parity across both implementations (devices incl. hardware-id conflict,
  artifacts incl. payload-properties round-trip, releases incl. dotted-version
  `LatestRelease` + offset paging, deployments incl. group-narrowed
  `ActiveDeploymentForTarget`, telemetry ordering, first-write-wins idempotency).
- The booted container is torn down on test cleanup (`mgr.Shutdown`); no leftover
  containers remain after the run.

## Reproduce

```
cd server
go test -tags integration -count=1 ./internal/store/ \
  -run TestPostgresRepositoryContract_Integration -v
```

Requires a container runtime (podman or docker) with a running machine and
`podman compose` / `docker compose`. The compose file is
`server/deploy/postgres.compose.yml` (Postgres 16, host port 55432).
