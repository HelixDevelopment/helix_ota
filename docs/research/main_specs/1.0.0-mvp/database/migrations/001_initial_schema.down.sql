-- =============================================================================
-- Helix OTA — 1.0.0-MVP initial schema (DOWN migration)
-- Migration:    001_initial_schema
-- Direction:    down
-- Target:       PostgreSQL 14+
-- Schema:       helix_ota
-- Purpose:      Reverse 001_initial_schema.up.sql exactly. Tables are dropped in
--               strict reverse dependency order (children before parents) so the
--               FK graph never blocks a DROP, even on runners that disable the
--               CASCADE convenience. Indexes drop with their owning tables.
-- Note:         The pgcrypto EXTENSION is intentionally NOT dropped — it may be
--               shared by other schemas/migrations, and dropping a shared
--               extension is destructive beyond this migration's scope.
-- =============================================================================

BEGIN;

SET LOCAL search_path = helix_ota, public;

-- Reverse order of creation (leaf / append-only tables first).
DROP TABLE IF EXISTS helix_ota.audit_logs;
DROP TABLE IF EXISTS helix_ota.telemetry_events;
DROP TABLE IF EXISTS helix_ota.device_deployments;
DROP TABLE IF EXISTS helix_ota.deployments;
DROP TABLE IF EXISTS helix_ota.releases;
DROP TABLE IF EXISTS helix_ota.artifact_versions;
DROP TABLE IF EXISTS helix_ota.artifacts;
DROP TABLE IF EXISTS helix_ota.device_group_members;
DROP TABLE IF EXISTS helix_ota.device_groups;
DROP TABLE IF EXISTS helix_ota.devices;
DROP TABLE IF EXISTS helix_ota.api_keys;
DROP TABLE IF EXISTS helix_ota.users;

-- Drop the schema only if this migration created it and nothing else lives in
-- it. RESTRICT (the default) fails loudly if unexpected objects remain, which
-- is the desired safety behavior for a down migration.
DROP SCHEMA IF EXISTS helix_ota RESTRICT;

COMMIT;
