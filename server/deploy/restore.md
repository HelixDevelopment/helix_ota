# Helix OTA — Database Restore Procedure

This document describes how to restore the PostgreSQL database from a backup
created by `deploy/backup.sh` (pg_dump -Fc format).

## Prerequisites

- PostgreSQL 16+ running and accessible
- The `pg_restore` client (included with `postgresql-client`)
- A backup file from `deploy/backup.sh` (`.dump` extension, -Fc custom format)
- Restore target env vars: PG_HOST, PG_DB, PG_USER, PGPASSWORD

## Step-by-Step

### 1. Stop the ota-server

Stop the control-plane service so no writes occur during restore:

```bash
# If using podman-compose:
cd deploy && podman-compose -f system.compose.yml stop ota-server

# If using docker-compose:
cd deploy && docker-compose -f system.compose.yml stop ota-server
```

### 2. Drop and recreate the database (clean restore)

Connect to PostgreSQL and reset the target database:

```sql
-- Connect as a superuser (postgres)
DROP DATABASE IF EXISTS helix_ota;
CREATE DATABASE helix_ota OWNER helix;
```

### 3. Restore the backup

```bash
pg_restore \
  -h "${PG_HOST}" \
  -p "${PG_PORT:-5432}" \
  -U "${PG_USER}" \
  -d "${PG_DB}" \
  --no-password \
  --no-owner \
  --verbose \
  helix_ota_backup_YYYY-MM-DDTHH-MM-SSZ.dump
```

To restore a backup from S3 without downloading first:

```bash
# Using mc (MinIO client):
mc cp "helix-backup/${S3_BUCKET}/helix_ota_backup_2026-07-26T02-00-00Z.dump" - | \
  pg_restore -h "${PG_HOST}" -U "${PG_USER}" -d "${PG_DB}" --no-password --no-owner

# Using aws s3:
aws s3 --endpoint-url "${S3_ENDPOINT}" \
  cp "s3://${S3_BUCKET}/helix_ota_backup_2026-07-26T02-00-00Z.dump" - | \
  pg_restore -h "${PG_HOST}" -U "${PG_USER}" -d "${PG_DB}" --no-password --no-owner
```

### 4. Verify the restore

Run verification queries to confirm data integrity:

```sql
-- Row counts for key tables
SELECT 'devices'  AS tbl, count(*) FROM helix_ota.devices
UNION ALL
SELECT 'artifacts', count(*) FROM helix_ota.artifacts
UNION ALL
SELECT 'releases',  count(*) FROM helix_ota.releases
UNION ALL
SELECT 'deployments', count(*) FROM helix_ota.deployments
UNION ALL
SELECT 'telemetry_events', count(*) FROM helix_ota.telemetry_events
UNION ALL
SELECT 'projects', count(*) FROM helix_ota.projects
UNION ALL
SELECT 'webhooks', count(*) FROM helix_ota.webhooks
UNION ALL
SELECT 'audit_logs', count(*) FROM helix_ota.audit_logs
ORDER BY tbl;

-- Check schema_migrations version matches expected
SELECT version, name, applied_at FROM helix_ota.schema_migrations ORDER BY version DESC LIMIT 5;

-- Verify at least one active deployment exists (if expected)
SELECT deployment_id, release_id, status, created_at
FROM helix_ota.deployments WHERE status = 'active';

-- Verify device count is reasonable (non-zero if fleet was registered)
SELECT count(*) AS registered_devices FROM helix_ota.devices;
```

### 5. Start the ota-server

```bash
cd deploy && podman-compose -f system.compose.yml up -d ota-server
```

### 6. Verify the server is healthy

```bash
# Wait a few seconds then check:
curl -s http://localhost:18080/healthz | jq .
# {"status":"ok"}

curl -s http://localhost:18080/readyz | jq .
# {"status":"ready"}
```

## Notes

- The `--no-owner` flag ensures restored objects are owned by the restoring user
  (typically `helix`), not the original owning role from the dump.
- For large databases, add `--jobs=N` to parallelize the restore (one job per
  CPU core is a good starting point).
- Always test the restore procedure against a non-production instance first.
