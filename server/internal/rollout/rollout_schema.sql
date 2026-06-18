-- Rollout-engine StoragePort schema (domain-aligned mapping of the brick State).
-- The canonical migration 002 (rollouts + deployment_phases, UUID-keyed) is the
-- full-system target; this is the lean store the brick's StoragePort needs,
-- keyed by the opaque deployment id the control plane uses.
-- Idempotent: safe to apply repeatedly (the integration-test bring-up DDL).

CREATE SCHEMA IF NOT EXISTS helix_ota;
SET search_path = helix_ota, public;

CREATE TABLE IF NOT EXISTS helix_ota.rollout_states (
    deployment_id    TEXT PRIMARY KEY,
    current_phase    INT         NOT NULL DEFAULT 0,
    status           TEXT        NOT NULL,
    phase_started_at TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS helix_ota.rollout_phases (
    deployment_id     TEXT             NOT NULL,
    phase_index       INT              NOT NULL,
    percentage        INT              NOT NULL,
    success_threshold DOUBLE PRECISION NOT NULL,
    error_threshold   DOUBLE PRECISION NOT NULL,
    duration_ns       BIGINT           NOT NULL,
    auto_progress     BOOLEAN          NOT NULL,
    CONSTRAINT rollout_phases_pk PRIMARY KEY (deployment_id, phase_index)
);
