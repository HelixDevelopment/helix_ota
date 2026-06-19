# QA evidence — migration 002 (staged-rollout + end-user rollback)

| Field | Value |
|---|---|
| Run id | 20260608-migration-002 |
| Feature | Migration `002_staged_rollout` — `deployment_phases`, `rollouts`, `rollback_history` (1.0.1; end-user rollback ratified INTO 1.0.1) |
| Date | 2026-06-08 |
| Evidence | [`run.log`](run.log) — real `psql` against Postgres 16 (podman) |

## What this proves (anti-bluff, Constitution §11.4)

Applied to a **real PostgreSQL 16** container (booted via podman), captured in `run.log`:

- `001 up: OK` then `002 up: OK` — 002 applies cleanly on top of the canonical schema.
- The three 1.0.1 tables are created (`deployment_phases` shown via `\dt`).
- **FK chain exercised**: users → artifacts → releases → deployments → `deployment_phases`
  insert succeeds (`phase insert OK`) — the foreign keys resolve.
- **CHECK enforced**: inserting a `kind='abort'` `rollback_history` row that carries a
  release is **rejected** — `violates check constraint "rollback_history_kind_ref_chk"`,
  proving the abort-vs-rollback reference shape is enforced at the DB layer.
- `002 down: OK` with `remaining_101 = 0` (the three tables dropped), then `001 down: OK` —
  clean reversible all-or-nothing rollback.

## Reproduce

```
podman run -d --name helix-mig002-test -e POSTGRES_USER=helix -e POSTGRES_PASSWORD=helix \
  -e POSTGRES_DB=helix_ota -p 55444:5432 docker.io/library/postgres:16-alpine
PGPASSWORD=helix psql -h 127.0.0.1 -p 55444 -U helix -d helix_ota -v ON_ERROR_STOP=1 \
  -f docs/research/main_specs/1.0.0-mvp/database/migrations/001_initial_schema.up.sql \
  -f docs/research/main_specs/1.0.0-mvp/database/migrations/002_staged_rollout.up.sql
# ... then the .down.sql files in reverse. podman rm -f helix-mig002-test
```
