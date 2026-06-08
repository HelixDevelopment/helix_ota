-- =============================================================================
-- Helix OTA — 1.0.1 staged-rollout schema (DOWN migration)
-- Migration:    002_staged_rollout
-- Direction:    down
-- Reverses 002_staged_rollout.up.sql. Drops in reverse dependency order; the
-- tables only reference 001 tables (deployments/releases/users), so no 001
-- object is affected. Transactional all-or-nothing.
-- =============================================================================

BEGIN;

SET LOCAL search_path = helix_ota, public;

DROP TABLE IF EXISTS helix_ota.rollback_history;
DROP TABLE IF EXISTS helix_ota.rollouts;
DROP TABLE IF EXISTS helix_ota.deployment_phases;

COMMIT;
