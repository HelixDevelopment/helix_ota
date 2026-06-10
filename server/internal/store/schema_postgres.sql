-- Helix OTA — store-domain schema for the pgx Repository (MVP persistence seam).
--
-- This schema maps 1:1 to the store domain structs (opaque string ids, group as a
-- free label, runtime device fields). The canonical normalized 12-table schema in
-- docs/research/main_specs/1.0.0-mvp/database/migrations/001_initial_schema.up.sql
-- remains the design target; this is the leaner mapping the Repository contract
-- needs for the modular-monolith MVP (architecture.md §4).
--
-- Idempotent: safe to apply repeatedly (used as the integration-test bring-up DDL).

CREATE SCHEMA IF NOT EXISTS helix_ota;
SET search_path = helix_ota, public;

CREATE TABLE IF NOT EXISTS helix_ota.devices (
    device_id       TEXT PRIMARY KEY,
    hardware_id     TEXT        NOT NULL,
    model           TEXT        NOT NULL DEFAULT '',
    os_type         TEXT        NOT NULL,
    os_version      TEXT        NOT NULL DEFAULT '',
    current_version TEXT        NOT NULL DEFAULT '',
    group_name      TEXT        NOT NULL DEFAULT '',
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    registered_at   TIMESTAMPTZ NOT NULL,
    last_seen       TIMESTAMPTZ,
    update_state    TEXT        NOT NULL DEFAULT '',
    active_slot     TEXT        NOT NULL DEFAULT '',
    last_error_code TEXT        NOT NULL DEFAULT '',
    health_ok       BOOLEAN     NOT NULL DEFAULT FALSE,
    target_version  TEXT        NOT NULL DEFAULT '',
    CONSTRAINT devices_hardware_id_uniq UNIQUE (hardware_id)
);

CREATE TABLE IF NOT EXISTS helix_ota.artifacts (
    artifact_id        TEXT PRIMARY KEY,
    sha256             TEXT        NOT NULL,
    size               BIGINT      NOT NULL,
    os_type            TEXT        NOT NULL,
    target_model       TEXT        NOT NULL,
    version            TEXT        NOT NULL,
    storage_ref        TEXT        NOT NULL DEFAULT '',
    verified           BOOLEAN     NOT NULL DEFAULT FALSE,
    uploaded_at        TIMESTAMPTZ NOT NULL,
    signature          TEXT        NOT NULL DEFAULT '',
    payload_properties JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS helix_ota.releases (
    seq                 BIGSERIAL,
    release_id          TEXT PRIMARY KEY,
    artifact_id         TEXT        NOT NULL,
    version             TEXT        NOT NULL,
    os_type             TEXT        NOT NULL,
    target_model        TEXT        NOT NULL,
    status              TEXT        NOT NULL DEFAULT '',
    notes               TEXT        NOT NULL DEFAULT '',
    min_current_version TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_releases_target ON helix_ota.releases (os_type, target_model);

CREATE TABLE IF NOT EXISTS helix_ota.deployments (
    seq           BIGSERIAL,
    deployment_id TEXT PRIMARY KEY,
    release_id    TEXT        NOT NULL,
    strategy      TEXT        NOT NULL DEFAULT '',
    group_name    TEXT        NOT NULL DEFAULT '',
    status        TEXT        NOT NULL DEFAULT '',
    target_count  INT         NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON helix_ota.deployments (status);

CREATE TABLE IF NOT EXISTS helix_ota.telemetry_events (
    seq               BIGSERIAL PRIMARY KEY,
    device_id         TEXT        NOT NULL DEFAULT '',
    deployment_id     TEXT        NOT NULL DEFAULT '',
    event             TEXT        NOT NULL,
    version           TEXT        NOT NULL DEFAULT '',
    error_code        TEXT        NOT NULL DEFAULT '',
    detail            TEXT        NOT NULL DEFAULT '',
    timestamp         TIMESTAMPTZ NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL,
    -- Optional per-event telemetry annotations (spec_impl_alignment.md row 4).
    -- NULLABLE so a legacy event that omits them stays NULL, never a misleading 0.
    duration_ms       BIGINT,
    bytes_transferred BIGINT
);
CREATE INDEX IF NOT EXISTS idx_telemetry_deployment ON helix_ota.telemetry_events (deployment_id);
-- Additive, idempotent column adds for databases provisioned before the
-- duration_ms/bytes_transferred annotations landed. ADD COLUMN IF NOT EXISTS is a
-- no-op on a fresh schema (the columns are already in the CREATE above) and a
-- safe forward-migration on an existing one (nullable, no default => no rewrite).
ALTER TABLE helix_ota.telemetry_events ADD COLUMN IF NOT EXISTS duration_ms       BIGINT;
ALTER TABLE helix_ota.telemetry_events ADD COLUMN IF NOT EXISTS bytes_transferred BIGINT;

CREATE TABLE IF NOT EXISTS helix_ota.device_groups (
    seq         BIGSERIAL,
    group_id    TEXT PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT device_groups_name_uniq UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS helix_ota.device_group_members (
    group_id   TEXT        NOT NULL,
    device_id  TEXT        NOT NULL,
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    seq        BIGSERIAL,
    CONSTRAINT device_group_members_pk PRIMARY KEY (group_id, device_id),
    CONSTRAINT device_group_members_group_fk
        FOREIGN KEY (group_id) REFERENCES helix_ota.device_groups (group_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS helix_ota.audit_logs (
    seq           BIGSERIAL PRIMARY KEY,
    audit_id      TEXT        NOT NULL,
    user_id       TEXT        NOT NULL DEFAULT '',
    actor_subject TEXT        NOT NULL DEFAULT '',
    action        TEXT        NOT NULL,
    resource_type TEXT        NOT NULL DEFAULT '',
    resource_id   TEXT        NOT NULL DEFAULT '',
    details       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    ip_address    TEXT        NOT NULL DEFAULT '',
    user_agent    TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_action   ON helix_ota.audit_logs (action);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON helix_ota.audit_logs (resource_type, resource_id);

CREATE TABLE IF NOT EXISTS helix_ota.delta_artifacts (
    delta_id           TEXT PRIMARY KEY,
    base_artifact_id   TEXT        NOT NULL,
    target_artifact_id TEXT        NOT NULL,
    sha256             TEXT        NOT NULL DEFAULT '',
    size               BIGINT      NOT NULL DEFAULT 0,
    storage_ref        TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT delta_artifacts_base_ne_target_chk CHECK (base_artifact_id <> target_artifact_id),
    CONSTRAINT delta_artifacts_pair_uniq UNIQUE (base_artifact_id, target_artifact_id)
);
CREATE INDEX IF NOT EXISTS idx_delta_artifacts_pair ON helix_ota.delta_artifacts (base_artifact_id, target_artifact_id);

CREATE TABLE IF NOT EXISTS helix_ota.rollback_history (
    seq                  BIGSERIAL PRIMARY KEY,
    rollback_id          TEXT        NOT NULL,
    deployment_id        TEXT        NOT NULL DEFAULT '',
    kind                 TEXT        NOT NULL,
    from_release_id      TEXT        NOT NULL DEFAULT '',
    to_release_id        TEXT        NOT NULL DEFAULT '',
    recall_deployment_id TEXT        NOT NULL DEFAULT '',
    reason               TEXT        NOT NULL DEFAULT '',
    triggered_by         TEXT        NOT NULL DEFAULT '',
    details              JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL,
    CONSTRAINT rollback_history_kind_chk CHECK (kind IN ('abort','rollback')),
    CONSTRAINT rollback_history_kind_ref_chk CHECK (
        (kind = 'rollback' AND from_release_id <> '' AND to_release_id <> '')
        OR (kind = 'abort' AND from_release_id = '' AND to_release_id = '')
    )
);
CREATE INDEX IF NOT EXISTS idx_rollback_history_deployment ON helix_ota.rollback_history (deployment_id);

CREATE TABLE IF NOT EXISTS helix_ota.idempotency_keys (
    key        TEXT PRIMARY KEY,
    result_id  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
