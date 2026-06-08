-- =============================================================================
-- Helix OTA — 1.0.3 delta / incremental updates schema (DOWN migration)
-- Migration:    004_delta_updates
-- Direction:    down
-- Target:       PostgreSQL 14+
-- Schema:       helix_ota
-- Purpose:      Reverse 004_delta_updates.up.sql exactly. Drops only the table
--               this migration created; the index drops with its owning table.
--               No 001/002 object is touched (this migration was purely
--               additive), so the down leaves the 001 + 002 schema intact.
-- =============================================================================

BEGIN;

SET LOCAL search_path = helix_ota, public;

DROP TABLE IF EXISTS helix_ota.delta_artifacts;

COMMIT;
