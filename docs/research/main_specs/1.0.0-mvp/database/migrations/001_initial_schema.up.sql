-- =============================================================================
-- Helix OTA — 1.0.0-MVP initial schema (UP migration)
-- Migration:    001_initial_schema
-- Direction:    up
-- Target:       PostgreSQL 14+ (uses gen_random_uuid() from pgcrypto / built-in
--               in PG13+; CREATE EXTENSION guards older installs).
-- Schema:       helix_ota
-- Scope:        1.0.0-MVP only. Staged rollout (deployment_phases, rollouts),
--               and end-user / multi-version rollback (rollback_history) are
--               DEFERRED to 1.0.1 per ADR-0002 / master design §1, §7, §8 and
--               are intentionally NOT created here.
-- Provenance:   master design 2026-06-07-helix-ota-design.md §7 (entity list);
--               additions/initial_research_02.md §4 (richer base, normalized);
--               ADR-0001 (hawkBit gated -> no hawkbit_* columns committed at MVP);
--               ADR-0002 (signing + SHA-256 + AVB at MVP, TUF deferred);
--               ADR-0003 (modular monolith — one PostgreSQL schema).
-- Note:         All DDL is transactional in PostgreSQL; runners that wrap each
--               migration in a single transaction get all-or-nothing semantics.
-- =============================================================================

BEGIN;

-- gen_random_uuid() lives in the pgcrypto extension on PostgreSQL < 13 and is
-- built into core on PostgreSQL 13+. CREATE EXTENSION IF NOT EXISTS is a no-op
-- when the function is already available in core.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- All Helix OTA control-plane state lives in a dedicated schema so it never
-- collides with a co-located hawkBit schema (ADR-0001) or other tenants.
CREATE SCHEMA IF NOT EXISTS helix_ota;

SET LOCAL search_path = helix_ota, public;

-- -----------------------------------------------------------------------------
-- users — dashboard / API operators (RBAC subjects). Identity & auth logic is
-- owned by the `auth`/`security` catalogue submodules; this table is the
-- relational projection the control plane joins against (audit, ownership).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(255) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(50)  NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT users_username_uniq UNIQUE (username),
    CONSTRAINT users_email_uniq    UNIQUE (email),
    CONSTRAINT users_role_chk      CHECK (role IN ('admin', 'operator', 'viewer'))
);

-- -----------------------------------------------------------------------------
-- api_keys — non-interactive credentials (CI build pipeline, automation).
-- Only the hash is stored; the cleartext is shown once at creation.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.api_keys (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL,
    key_hash    VARCHAR(255) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    permissions JSONB        NOT NULL DEFAULT '{}'::jsonb,
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT api_keys_key_hash_uniq UNIQUE (key_hash),
    CONSTRAINT api_keys_user_fk
        FOREIGN KEY (user_id) REFERENCES helix_ota.users (id) ON DELETE CASCADE,
    CONSTRAINT api_keys_expiry_chk
        CHECK (expires_at IS NULL OR expires_at > created_at)
);

CREATE INDEX idx_api_keys_user ON helix_ota.api_keys (user_id);

-- -----------------------------------------------------------------------------
-- devices — fleet inventory. `device_id` is the stable external identity (token
-- bound to hardware id, master §6). `os_type` is 'android' for Phase 1; the
-- column exists so the OS-adapter seam (ADR-0003 §3.1) is schema-ready.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.devices (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id       VARCHAR(255) NOT NULL,
    name            VARCHAR(255),
    os_type         VARCHAR(50)  NOT NULL,
    os_version      VARCHAR(100),
    hardware_model  VARCHAR(255),
    serial_number   VARCHAR(255),
    current_version VARCHAR(100),
    status          VARCHAR(50)  NOT NULL DEFAULT 'active',
    last_seen_at    TIMESTAMPTZ,
    metadata        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT devices_device_id_uniq UNIQUE (device_id),
    CONSTRAINT devices_os_type_chk
        CHECK (os_type IN ('android', 'linux', 'windows', 'other')),
    CONSTRAINT devices_status_chk
        CHECK (status IN ('active', 'inactive', 'blocked'))
);

CREATE INDEX idx_devices_os_type   ON helix_ota.devices (os_type);
CREATE INDEX idx_devices_status    ON helix_ota.devices (status);
CREATE INDEX idx_devices_last_seen ON helix_ota.devices (last_seen_at);

-- -----------------------------------------------------------------------------
-- device_groups — named / dynamic cohorts. `filter_criteria` holds a dynamic
-- selector; static membership is in device_group_members.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.device_groups (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    filter_criteria JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT device_groups_name_uniq UNIQUE (name)
);

-- device_group_members — static M:N membership (composite PK).
CREATE TABLE helix_ota.device_group_members (
    group_id  UUID        NOT NULL,
    device_id UUID        NOT NULL,
    added_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT device_group_members_pk PRIMARY KEY (group_id, device_id),
    CONSTRAINT device_group_members_group_fk
        FOREIGN KEY (group_id)  REFERENCES helix_ota.device_groups (id) ON DELETE CASCADE,
    CONSTRAINT device_group_members_device_fk
        FOREIGN KEY (device_id) REFERENCES helix_ota.devices (id)       ON DELETE CASCADE
);

CREATE INDEX idx_dgm_device ON helix_ota.device_group_members (device_id);

-- -----------------------------------------------------------------------------
-- artifacts — uploaded OTA blobs. `file_path` is the Storage (MinIO/S3) object
-- key, NOT a local FS path. Integrity = SHA-256 (mandatory) + SHA-512 (where
-- available) + detached signature; verified server-side on upload AND device-
-- side before apply (ADR-0002 §4.1). `artifact_type` keeps 'full' for MVP;
-- 'delta'/'incremental' are reserved for ADR-0005 (delta updates, deferred).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.artifacts (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL,
    version           VARCHAR(100) NOT NULL,
    os_type           VARCHAR(50)  NOT NULL,
    artifact_type     VARCHAR(50)  NOT NULL DEFAULT 'full',
    file_path         VARCHAR(500) NOT NULL,
    file_size         BIGINT       NOT NULL,
    checksum_sha256   CHAR(64)     NOT NULL,
    checksum_sha512   CHAR(128),
    signature         TEXT,
    metadata          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    upload_status     VARCHAR(50)  NOT NULL DEFAULT 'pending',
    validation_errors JSONB,
    uploaded_by       UUID,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT artifacts_uploaded_by_fk
        FOREIGN KEY (uploaded_by) REFERENCES helix_ota.users (id) ON DELETE SET NULL,
    CONSTRAINT artifacts_os_type_chk
        CHECK (os_type IN ('android', 'linux', 'windows', 'other')),
    CONSTRAINT artifacts_type_chk
        CHECK (artifact_type IN ('full', 'incremental', 'delta')),
    CONSTRAINT artifacts_upload_status_chk
        CHECK (upload_status IN ('pending', 'validating', 'validated', 'failed')),
    CONSTRAINT artifacts_file_size_chk
        CHECK (file_size > 0),
    -- SHA-256 hex digest is exactly 64 lowercase hex chars.
    CONSTRAINT artifacts_sha256_chk
        CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT artifacts_sha512_chk
        CHECK (checksum_sha512 IS NULL OR checksum_sha512 ~ '^[0-9a-f]{128}$'),
    -- One row per (os, version) of an artifact name.
    CONSTRAINT artifacts_name_os_version_uniq UNIQUE (name, os_type, version)
);

CREATE INDEX idx_artifacts_os_type ON helix_ota.artifacts (os_type);
CREATE INDEX idx_artifacts_version ON helix_ota.artifacts (version);
CREATE INDEX idx_artifacts_status  ON helix_ota.artifacts (upload_status);

-- -----------------------------------------------------------------------------
-- artifact_versions — version lineage / changelog per artifact. Enforces a
-- single 'latest' per artifact via a partial unique index.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.artifact_versions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id   UUID         NOT NULL,
    version       VARCHAR(100) NOT NULL,
    changelog     TEXT,
    release_notes TEXT,
    is_latest     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT artifact_versions_artifact_fk
        FOREIGN KEY (artifact_id) REFERENCES helix_ota.artifacts (id) ON DELETE CASCADE,
    CONSTRAINT artifact_versions_artifact_version_uniq UNIQUE (artifact_id, version)
);

-- At most one latest version per artifact.
CREATE UNIQUE INDEX idx_artifact_versions_one_latest
    ON helix_ota.artifact_versions (artifact_id)
    WHERE is_latest;

-- -----------------------------------------------------------------------------
-- releases — a published, deployable artifact (master §7 lists `releases` as a
-- first-class entity distinct from deployments). A release is the unit an admin
-- "publishes" before deploying; it pins one validated artifact + channel and
-- enforces version monotonicity at publish time (master §5).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.releases (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id   UUID         NOT NULL,
    version       VARCHAR(100) NOT NULL,
    channel       VARCHAR(50)  NOT NULL DEFAULT 'stable',
    status        VARCHAR(50)  NOT NULL DEFAULT 'draft',
    release_notes TEXT,
    published_at  TIMESTAMPTZ,
    published_by  UUID,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT releases_artifact_fk
        FOREIGN KEY (artifact_id) REFERENCES helix_ota.artifacts (id) ON DELETE RESTRICT,
    CONSTRAINT releases_published_by_fk
        FOREIGN KEY (published_by) REFERENCES helix_ota.users (id) ON DELETE SET NULL,
    CONSTRAINT releases_channel_chk
        CHECK (channel IN ('stable', 'beta', 'canary', 'internal')),
    CONSTRAINT releases_status_chk
        CHECK (status IN ('draft', 'published', 'superseded', 'withdrawn')),
    -- A published release must record when/by-whom it was published.
    CONSTRAINT releases_published_consistency_chk
        CHECK (
            (status = 'published' AND published_at IS NOT NULL)
            OR (status <> 'published')
        ),
    CONSTRAINT releases_channel_version_uniq UNIQUE (channel, version)
);

CREATE INDEX idx_releases_artifact ON helix_ota.releases (artifact_id);
CREATE INDEX idx_releases_status   ON helix_ota.releases (status);
CREATE INDEX idx_releases_channel  ON helix_ota.releases (channel);

-- -----------------------------------------------------------------------------
-- deployments — an instruction to deliver a release to a target set. For MVP
-- the only supported strategy is all-at-once (master §5: "staged engine lands
-- 1.0.1"). `rollout_strategy` is kept as JSONB so a 1.0.1 staged config drops
-- in without a column migration; MVP writes {"mode":"all_at_once"}.
-- DEFERRED to 1.0.1: deployment_phases, rollouts (ADR-0001 hawkBit gated).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.deployments (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    release_id       UUID         NOT NULL,
    target_type      VARCHAR(50)  NOT NULL,
    target_group_id  UUID,
    target_device_id UUID,
    rollout_strategy JSONB        NOT NULL DEFAULT '{"mode":"all_at_once"}'::jsonb,
    status           VARCHAR(50)  NOT NULL DEFAULT 'draft',
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_by       UUID,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT deployments_release_fk
        FOREIGN KEY (release_id) REFERENCES helix_ota.releases (id) ON DELETE RESTRICT,
    CONSTRAINT deployments_group_fk
        FOREIGN KEY (target_group_id) REFERENCES helix_ota.device_groups (id) ON DELETE SET NULL,
    CONSTRAINT deployments_device_fk
        FOREIGN KEY (target_device_id) REFERENCES helix_ota.devices (id) ON DELETE SET NULL,
    CONSTRAINT deployments_created_by_fk
        FOREIGN KEY (created_by) REFERENCES helix_ota.users (id) ON DELETE SET NULL,
    CONSTRAINT deployments_target_type_chk
        CHECK (target_type IN ('all', 'group', 'device')),
    CONSTRAINT deployments_status_chk
        CHECK (status IN ('draft', 'active', 'paused', 'completed', 'failed', 'cancelled')),
    -- Target reference must match target_type: 'group' needs a group, 'device'
    -- needs a device, 'all' needs neither.
    CONSTRAINT deployments_target_ref_chk
        CHECK (
            (target_type = 'all'    AND target_group_id IS NULL AND target_device_id IS NULL)
            OR (target_type = 'group'  AND target_group_id IS NOT NULL AND target_device_id IS NULL)
            OR (target_type = 'device' AND target_device_id IS NOT NULL AND target_group_id IS NULL)
        ),
    CONSTRAINT deployments_time_order_chk
        CHECK (completed_at IS NULL OR started_at IS NULL OR completed_at >= started_at)
);

CREATE INDEX idx_deployments_release ON helix_ota.deployments (release_id);
CREATE INDEX idx_deployments_status  ON helix_ota.deployments (status);

-- -----------------------------------------------------------------------------
-- device_deployments — per-device delivery state for a deployment (the join
-- that records what each device actually did). One row per (deployment, device).
-- `phase_id` is intentionally OMITTED for MVP (staged phases are 1.0.1).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.device_deployments (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID          NOT NULL,
    device_id     UUID          NOT NULL,
    status        VARCHAR(50)   NOT NULL DEFAULT 'pending',
    progress      NUMERIC(5,2)  NOT NULL DEFAULT 0.00,
    error_message TEXT,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    retry_count   INT           NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT device_deployments_deployment_fk
        FOREIGN KEY (deployment_id) REFERENCES helix_ota.deployments (id) ON DELETE CASCADE,
    CONSTRAINT device_deployments_device_fk
        FOREIGN KEY (device_id) REFERENCES helix_ota.devices (id) ON DELETE CASCADE,
    CONSTRAINT device_deployments_status_chk
        CHECK (status IN (
            'pending', 'downloading', 'installing', 'verifying',
            'success', 'failed', 'rolled_back'
        )),
    CONSTRAINT device_deployments_progress_chk
        CHECK (progress >= 0.00 AND progress <= 100.00),
    CONSTRAINT device_deployments_retry_chk
        CHECK (retry_count >= 0),
    CONSTRAINT device_deployments_uniq UNIQUE (deployment_id, device_id)
);

CREATE INDEX idx_device_deployments_status ON helix_ota.device_deployments (status);
CREATE INDEX idx_device_deployments_device ON helix_ota.device_deployments (device_id);

-- -----------------------------------------------------------------------------
-- telemetry_events — device-reported event stream (master §9). Renamed from
-- draft 02's `update_metrics` to the master's canonical `telemetry_events`.
-- This is the higher-volume, append-mostly table (ADR-0003 §3.2 trigger #1,
-- UNVERIFIED relative volume). FKs use ON DELETE SET NULL so history survives
-- device/deployment deletion.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.telemetry_events (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id         UUID,
    deployment_id     UUID,
    event_type        VARCHAR(50)  NOT NULL,
    duration_ms       BIGINT,
    bytes_transferred BIGINT,
    success           BOOLEAN,
    error_code        VARCHAR(50),
    error_message     TEXT,
    metadata          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT telemetry_events_device_fk
        FOREIGN KEY (device_id) REFERENCES helix_ota.devices (id) ON DELETE SET NULL,
    CONSTRAINT telemetry_events_deployment_fk
        FOREIGN KEY (deployment_id) REFERENCES helix_ota.deployments (id) ON DELETE SET NULL,
    -- Canonical device event vocabulary (master §9).
    CONSTRAINT telemetry_events_event_type_chk
        CHECK (event_type IN (
            'download_started', 'download_complete',
            'installing', 'installed',
            'verifying', 'success', 'failure', 'rollback'
        )),
    CONSTRAINT telemetry_events_duration_chk
        CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT telemetry_events_bytes_chk
        CHECK (bytes_transferred IS NULL OR bytes_transferred >= 0)
);

CREATE INDEX idx_telemetry_events_device     ON helix_ota.telemetry_events (device_id);
CREATE INDEX idx_telemetry_events_deployment ON helix_ota.telemetry_events (deployment_id);
CREATE INDEX idx_telemetry_events_type       ON helix_ota.telemetry_events (event_type);
CREATE INDEX idx_telemetry_events_created    ON helix_ota.telemetry_events (created_at);

-- -----------------------------------------------------------------------------
-- audit_logs — every admin/operator action (master §6 "every admin action
-- logged"). Append-only; user_id nullable so the record outlives the actor.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.audit_logs (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID,
    action        VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id   UUID,
    details       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT audit_logs_user_fk
        FOREIGN KEY (user_id) REFERENCES helix_ota.users (id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_user     ON helix_ota.audit_logs (user_id);
CREATE INDEX idx_audit_logs_action   ON helix_ota.audit_logs (action);
CREATE INDEX idx_audit_logs_resource ON helix_ota.audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_created  ON helix_ota.audit_logs (created_at);

COMMIT;
