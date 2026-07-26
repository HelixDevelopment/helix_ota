# Helix OTA — Disaster Recovery

## Overview

This document covers full PostgreSQL restore from backup, server rebuild from
scratch, and data integrity validation after restore. References the backup
script at `server/deploy/backup.sh` and the restore procedure at
`server/deploy/restore.md`.

## PostgreSQL Restore from Backup

### Prerequisites

- A backup file from `server/deploy/backup.sh` (`.dump`, pg_dump -Fc format)
- PostgreSQL 16+ running and accessible
- `pg_restore` client (package: `postgresql-client-16`)
- Restore target environment variables: `PG_HOST`, `PG_DB`, `PG_USER`, `PGPASSWORD`

### Procedure

#### 1. Stop the OTA Server

Prevent writes during restore:

```bash
cd /opt/helix_ota/deploy
podman-compose -f system.compose.yml stop ota-server
```

#### 2. Drop and Recreate the Database

```bash
podman exec -it deploy_postgres_1 psql -U postgres -c "DROP DATABASE IF EXISTS helix_ota;"
podman exec -it deploy_postgres_1 psql -U postgres -c "CREATE DATABASE helix_ota OWNER helix;"
```

Or connect directly if PostgreSQL is on bare metal:

```sql
DROP DATABASE IF EXISTS helix_ota;
CREATE DATABASE helix_ota OWNER helix;
```

#### 3. Restore the Backup

From local file:

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

From S3/MinIO directly (without downloading first):

```bash
# Using mc (MinIO client):
mc cp "helix-backup/${S3_BUCKET}/helix_ota_backup_2026-07-26T02-00-00Z.dump" - | \
  pg_restore -h "${PG_HOST}" -U "${PG_USER}" -d "${PG_DB}" --no-password --no-owner

# Using aws s3:
aws s3 --endpoint-url "${S3_ENDPOINT}" \
  cp "s3://${S3_BUCKET}/helix_ota_backup_2026-07-26T02-00-00Z.dump" - | \
  pg_restore -h "${PG_HOST}" -U "${PG_USER}" -d "${PG_DB}" --no-password --no-owner
```

For large databases, parallelize: `pg_restore --jobs=$(nproc) ...`

#### 4. Data Integrity Verification

Run these queries to verify the restore is complete and consistent:

```sql
-- Row counts across all key tables
SELECT 'devices'           AS tbl, count(*) FROM devices           UNION ALL
SELECT 'artifacts',               count(*) FROM artifacts         UNION ALL
SELECT 'releases',                count(*) FROM releases          UNION ALL
SELECT 'deployments',             count(*) FROM deployments       UNION ALL
SELECT 'telemetry_events',        count(*) FROM telemetry_events  UNION ALL
SELECT 'projects',                count(*) FROM projects          UNION ALL
SELECT 'webhooks',                count(*) FROM webhooks          UNION ALL
SELECT 'audit_logs',              count(*) FROM audit_logs        UNION ALL
SELECT 'device_groups',           count(*) FROM device_groups     UNION ALL
SELECT 'group_members',           count(*) FROM group_members     UNION ALL
SELECT 'branches',                count(*) FROM branches          UNION ALL
SELECT 'delta_artifacts',         count(*) FROM delta_artifacts   UNION ALL
SELECT 'rollback_history',        count(*) FROM rollback_history  UNION ALL
SELECT 'project_members',         count(*) FROM project_members   UNION ALL
SELECT 'fabric_nodes',            count(*) FROM fabric_nodes      UNION ALL
SELECT 'fabric_targets',          count(*) FROM fabric_targets    UNION ALL
SELECT 'accounts',                count(*) FROM accounts
ORDER BY tbl;

-- Verify schema_migrations version
SELECT version, name, applied_at FROM schema_migrations ORDER BY version DESC LIMIT 5;

-- Verify referential integrity: every deployment references a valid release
SELECT d.deployment_id, d.release_id
FROM deployments d
LEFT JOIN releases r ON d.release_id = r.release_id
WHERE r.release_id IS NULL;

-- Verify referential integrity: every release references a valid artifact
SELECT r.release_id, r.artifact_id
FROM releases r
LEFT JOIN artifacts a ON r.artifact_id = a.artifact_id
WHERE a.artifact_id IS NULL AND r.artifact_id IS NOT NULL;

-- Verify active deployments exist (if expected)
SELECT deployment_id, release_id, status, created_at
FROM deployments WHERE status = 'active';

-- Verify device count is non-zero (if fleet was registered)
SELECT count(*) AS registered_devices FROM devices;

-- Check for orphaned telemetry events
SELECT count(*)
FROM telemetry_events te
LEFT JOIN devices d ON te.device_id = d.device_id
WHERE d.device_id IS NULL;
```

#### 5. Start the Server

```bash
cd /opt/helix_ota/deploy
podman-compose -f system.compose.yml up -d ota-server
```

#### 6. Verify Server Health

```bash
sleep 5
curl -s http://localhost:18080/healthz | jq .  # {"status":"ok"}
curl -s http://localhost:18080/readyz | jq .   # {"status":"ready"}
```

## Server Rebuild from Scratch

If the VM itself is lost, rebuild:

### 1. Provision VM

Ubuntu 24.04 LTS, ≥ 4 GB RAM, ≥ 20 GB disk, SSH access.

### 2. Follow Deployment Runbook

Follow `server/deploy/deployment_runbook.md` Steps 1-5 to install prerequisites,
clone the repo, configure environment, and start services.

### 3. Restore Data

Follow the PostgreSQL restore procedure above (Steps 1-6).

### 4. Verify OTA Functionality

```bash
# Re-obtain a bearer token
TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin@helix.example","password":"admin"}' \
  | jq -r '.access_token')

# List devices to verify data restored
curl -s http://localhost:18080/api/v1/devices \
  -H "Authorization: Bearer ${TOKEN}" | jq '.items | length'

# Verify fleet telemetry
curl -s http://localhost:18080/api/v1/telemetry/overview \
  -H "Authorization: Bearer ${TOKEN}" | jq '.event_counts'
```

## Backup Schedule Recommendation

Configure the backup script as a cron job:

```
# Daily at 02:00 UTC
0 2 * * * /opt/helix_ota/server/deploy/backup.sh >> /var/log/helix_backup.log 2>&1
```

Or as a podman-compose sidecar container (see `deploy/system.compose.yml` for
a commented cron sidecar example).

Retention policy:
- Daily backups kept for 7 days
- Weekly backups kept for 4 weeks
- Monthly backups kept for 12 months

## Manual Backup (Ad-hoc)

```bash
cd /opt/helix_ota/server/deploy
PG_HOST=localhost PG_DB=helix_ota PG_USER=helix \
  S3_BUCKET=helix-backups S3_ENDPOINT=https://s3.example.com \
  bash backup.sh
```
