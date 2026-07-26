# Helix OTA — Upgrade and Rollback Procedure

## Overview

This document covers the server binary upgrade procedure, database schema
migration with forward/rollback verification, and post-upgrade health
checks.

## Server Binary Upgrade

### 1. Prepare the New Binary

```bash
cd /opt/helix_ota
git fetch origin
git checkout <target-tag>         # e.g., v1.1.0
git submodule update --init --recursive

cd server
go build -o ota-server ./cmd/server/
```

### 2. Stop the Running Instance

```bash
cd /opt/helix_ota/deploy
podman-compose -f system.compose.yml stop ota-server
```

### 3. Run Database Migrations (Up)

The server applies migrations automatically on startup, but explicit
verification is recommended before the upgrade:

```bash
# Connect to PostgreSQL
podman exec -it deploy_postgres_1 psql -U helix -d helix_ota

# Check current schema version
SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;
```

After confirming the current schema version, the upgrade server binary will
apply any pending `Up()` migrations on startup.

### 4. Start the New Server Binary

```bash
podman-compose -f system.compose.yml up -d ota-server
sleep 5
```

### 5. Verify Health

```bash
# Health probe
curl -s http://localhost:18080/healthz | jq .
# Expected: {"status":"ok"}

# Readiness probe
curl -s http://localhost:18080/readyz | jq .
# Expected: {"status":"ready"}

# Verify the new version is running
curl -s http://localhost:18080/healthz | jq '.version'  # if exposed
```

### 6. Post-Upgrade Verification

```bash
TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin@helix.example","password":"admin"}' \
  | jq -r '.access_token')

# Verify existing devices are still registered
curl -s http://localhost:18080/api/v1/devices \
  -H "Authorization: Bearer ${TOKEN}" | jq '.items | length'

# Verify active deployments still show
curl -s http://localhost:18080/api/v1/deployments \
  -H "Authorization: Bearer ${TOKEN}" | jq '.items[] | select(.status=="active")'

# Verify telemetry overview returns data
curl -s http://localhost:18080/api/v1/telemetry/overview \
  -H "Authorization: Bearer ${TOKEN}" | jq '.total'

# Verify audit log is accessible
curl -s "http://localhost:18080/api/v1/audit?limit=1" \
  -H "Authorization: Bearer ${TOKEN}" | jq '.items | length'
```

## Rolling Back to a Previous Version

### 1. Identify the Target Rollback Version

```bash
git log --oneline -10 -- server/
# Note the commit/tag to roll back to
```

### 2. Stop the Server

```bash
cd /opt/helix_ota/deploy
podman-compose -f system.compose.yml stop ota-server
```

### 3. Checkout the Previous Version

```bash
cd /opt/helix_ota
git checkout <previous-tag-or-commit>
git submodule update --init --recursive
```

### 4. Build and Verify the Rollback Binary

```bash
cd server
go build -o ota-server ./cmd/server/

# Verify binary version
./ota-server --version  # if supported
```

### 5. Database Migration Rollback

If the upgrade included schema migrations that need to be reversed:

```bash
# Connect to PostgreSQL
podman exec -it deploy_postgres_1 psql -U helix -d helix_ota

# Check which migrations were applied by the upgrade
SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;
```

For each migration that must be rolled back:
- Execute the `Down()` SQL from the migration file
- Remove the migration record from `schema_migrations`

Example:
```sql
BEGIN;

-- Execute Down() migration SQL (from the specific migration file)
ALTER TABLE artifacts DROP COLUMN IF EXISTS new_feature_column;

-- Remove the migration record
DELETE FROM schema_migrations WHERE version = <version_number>;

COMMIT;
```

> **Important:** Only roll back migrations that are safe to reverse
> (columns that were added can be dropped; columns that were removed and
> data that was transformed cannot be recovered by a simple Down()).

### 6. Start the Rolled-Back Server

```bash
podman-compose -f system.compose.yml up -d ota-server
sleep 5
```

### 7. Verify Rollback Success

```bash
# Health check
curl -s http://localhost:18080/healthz | jq .  # {"status":"ok"}
curl -s http://localhost:18080/readyz | jq .   # {"status":"ready"}

# Verify core functionality
TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin@helix.example","password":"admin"}' \
  | jq -r '.access_token')

# Verify we can list devices (basic CRUD check)
curl -s http://localhost:18080/api/v1/devices \
  -H "Authorization: Bearer ${TOKEN}" | jq '.items | length'
```

## Emergency Rollback Checklist

Use this when a critical issue is detected immediately after upgrade:

- [ ] Stop the server (`podman-compose stop ota-server`)
- [ ] Take a database snapshot before rollback:
  ```bash
  PG_HOST=localhost PG_DB=helix_ota PG_USER=helix \
    pg_dump -Fc -f "pre_rollback_snapshot_$(date -u +%Y%m%dT%H%M%SZ).dump"
  ```
- [ ] Checkout the previously-known-good tag/commit
- [ ] Rebuild the binary from the old commit
- [ ] If safe, roll back schema migrations (Down SQL)
- [ ] Start the old binary
- [ ] Verify health + readiness + core CRUD
- [ ] Verify active deployments are intact
- [ ] Communicate to the team

## Upgrade Path Considerations

- **Schema compatibility:** Migrations must be backward-compatible (additive
  only) when possible. Avoid `DROP COLUMN` migrations until the next version
  has been running in production for a full deployment cycle.
- **API compatibility:** The API contract must not break existing device
  clients during an upgrade. New fields must be additive; old fields cannot
  change type or semantics without a new API version path.
- **Key rotation:** If `HELIX_TOKEN_SECRET` is rotated, existing tokens will
  be invalidated. Coordinate token secret rotation with device re-enrollment
  or plan a dual-signing window.
- **Artifact signing key rotation:** The server supports a previous public key
  (`previous_artifact_public_key`) for validating artifacts signed with an
  older key during a rotation window. Set `signing_key_rotation_interval` in
  the config to enable this.
