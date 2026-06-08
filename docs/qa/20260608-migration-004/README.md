# QA evidence — migration 004 (delta_artifacts)

| Field | Value |
|---|---|
| Run ID | 20260608-migration-004 |
| Date (UTC) | 2026-06-08T14:03Z |
| Engine | PostgreSQL 16.14 (`docker.io/library/postgres:16-alpine`, podman 5.8.2) |
| Container | `helix-mig004-test` on `127.0.0.1:55446` (removed after run) |
| Migrations under test | `004_delta_updates.{up,down}.sql` |
| Applied stack | `001_initial_schema` -> `002_staged_rollout` -> `004_delta_updates` |
| Evidence | [`run.log`](run.log) — full real psql transcript |

## What `004` adds

A new `helix_ota.delta_artifacts` table (purely additive — no `ALTER` of any 001
table; `artifacts.artifact_type` already permits `'delta'`). Per
`delta_updates_design.md` §4.1(a): a separate table holding the base->target
relationship, off the `artifacts` row.

- `base_artifact_id`, `target_artifact_id`, `delta_artifact_id` — all
  `UUID -> helix_ota.artifacts(id) ON DELETE CASCADE` (design §4.2).
  `delta_artifact_id` nullable (NULL until the job publishes the payload).
- `CHECK (base_artifact_id <> target_artifact_id)` — `delta_artifacts_base_ne_target_chk`.
- `UNIQUE (base_artifact_id, target_artifact_id)` — one relationship row per pair.
- Delta file integrity, mirroring 001's `artifacts` conventions:
  `file_path VARCHAR(500)`, `file_size BIGINT` (`> 0` when present),
  `checksum_sha256 CHAR(64)` (`~ '^[0-9a-f]{64}$'` when present).
- `status` lifecycle CHECK (`pending|generating|generated|failed|cancelled`,
  design §3.3), `savings_percent` (0..100, NULL until measured — design §7/§9
  anti-bluff), `partition_deltas`/`generation_errors` JSONB.
- Indexes for "find a delta from base X to target Y": the `UNIQUE(base,target)`
  backs the exact-pair lookup; `idx_delta_artifacts_base` and
  `idx_delta_artifacts_target` back the single-leg fan-out queries the
  selection matrix runs at update-check time; plus `idx_delta_artifacts_status`
  (servable filter) and `idx_delta_artifacts_delta` (payload back-join).

## Validation result (real captured evidence — see run.log)

| Check | Outcome |
|---|---|
| `001` up | OK |
| `002` up | OK |
| `004` up | OK (table + 4 indexes + 5 CHECKs + 3 FKs present, confirmed via `\d`) |
| Seed base + target full artifacts | OK (2 rows) |
| Insert valid delta `1.0.0 -> 1.0.1` (`generated`, 90.23%) | OK |
| "find delta from base X to target Y" indexed lookup | OK (1 row) |
| **base == target insert** | **REJECTED** — `delta_artifacts_base_ne_target_chk` |
| Duplicate `(base,target)` insert | REJECTED — `delta_artifacts_base_target_uniq` |
| Dangling base FK insert | REJECTED — `delta_artifacts_base_fk` |
| `ON DELETE CASCADE` (delete base artifact) | OK — delta row count 1 -> 0 |
| `004` down | OK — `delta_artifacts` gone; `artifacts`/`rollouts` intact |
| `002` down | OK |
| `001` down | OK — `helix_ota` schema fully removed |

No bluff: every line above is a real `psql -v ON_ERROR_STOP=1` result captured in
`run.log` against a live Postgres 16 instance. The container was removed
(`podman rm -f helix-mig004-test`) after the run.

## Reproduce

```sh
podman run -d --name helix-mig004-test -e POSTGRES_USER=helix \
  -e POSTGRES_PASSWORD=helix -e POSTGRES_DB=helix_ota \
  -p 55446:5432 docker.io/library/postgres:16-alpine
# wait for ready, then with:
#   PGPASSWORD=helix psql -h 127.0.0.1 -p 55446 -U helix -d helix_ota -v ON_ERROR_STOP=1
# apply 001 up -> 002 up -> 004 up; exercise inserts; 004 down -> 002 down -> 001 down.
podman rm -f helix-mig004-test
```
