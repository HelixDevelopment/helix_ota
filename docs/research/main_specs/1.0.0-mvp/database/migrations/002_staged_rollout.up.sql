-- =============================================================================
-- Helix OTA — 1.0.1 staged-rollout schema (UP migration)
-- Migration:    002_staged_rollout
-- Direction:    up
-- Target:       PostgreSQL 14+ (same as 001)
-- Schema:       helix_ota (extends migration 001)
-- Scope:        1.0.1 staged rollout + end-user/operator rollback (the operator
--               ratified end-user rollback INTO 1.0.1 alongside staged rollout).
--               Adds the three tables deferred from MVP: deployment_phases,
--               rollouts, rollback_history. Backs the StoragePort in
--               1.0.1-staged-rollout/rollout_engine.md §6 + migration_002_design.md.
-- Provenance:   migration_002_design.md §3 (tables/columns/constraints/indexes).
--               TUF metadata is intentionally NOT created here (deferred).
-- Note:         Purely additive — no ALTER on any 001 table. The MVP
--               deployments.rollout_strategy JSONB already carries the strategy
--               marker ({"mode":"staged"}); no column migration needed (§4).
-- =============================================================================

BEGIN;

SET LOCAL search_path = helix_ota, public;

-- -----------------------------------------------------------------------------
-- deployment_phases — the ordered percentage plan for a staged deployment; one
-- row per phase, mirroring the ota-rollout-engine brick `Phase`. Phases are
-- immutable after rollout start (no UPDATE path; brick is the authoritative
-- validator of the cross-row strict-increase / final-100 invariants).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.deployment_phases (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id     UUID         NOT NULL,
    phase_index       INT          NOT NULL,
    percentage        INT          NOT NULL,
    success_threshold NUMERIC(4,3) NOT NULL,
    error_threshold   NUMERIC(4,3) NOT NULL,
    duration_seconds  BIGINT       NOT NULL,
    auto_progress     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT deployment_phases_deployment_fk
        FOREIGN KEY (deployment_id) REFERENCES helix_ota.deployments (id) ON DELETE CASCADE,
    CONSTRAINT deployment_phases_index_chk    CHECK (phase_index >= 0),
    CONSTRAINT deployment_phases_pct_chk      CHECK (percentage > 0 AND percentage <= 100),
    CONSTRAINT deployment_phases_success_chk  CHECK (success_threshold >= 0 AND success_threshold <= 1),
    CONSTRAINT deployment_phases_error_chk    CHECK (error_threshold   >= 0 AND error_threshold   <= 1),
    CONSTRAINT deployment_phases_duration_chk CHECK (duration_seconds  >= 0),
    CONSTRAINT deployment_phases_idx_uniq     UNIQUE (deployment_id, phase_index),
    CONSTRAINT deployment_phases_pct_uniq     UNIQUE (deployment_id, percentage)
);

CREATE INDEX idx_deployment_phases_deployment
    ON helix_ota.deployment_phases (deployment_id, phase_index);

-- -----------------------------------------------------------------------------
-- rollouts — one row per staged deployment: the brick engine cursor + the
-- control-plane status overlay. `status` carries the brick set PLUS the
-- control-plane-only overlays (paused/aborted/rolled_back); the partial CHECK
-- enforces the reconciliation rule (status equals engine_status unless it is an
-- overlay) so a single-writer discipline is also defended at the DB layer.
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.rollouts (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id    UUID         NOT NULL,
    engine_status    VARCHAR(20)  NOT NULL DEFAULT 'pending',
    status           VARCHAR(20)  NOT NULL DEFAULT 'pending',
    current_phase    INT          NOT NULL DEFAULT 0,
    phase_started_at TIMESTAMPTZ,
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT rollouts_deployment_fk
        FOREIGN KEY (deployment_id) REFERENCES helix_ota.deployments (id) ON DELETE CASCADE,
    CONSTRAINT rollouts_deployment_uniq UNIQUE (deployment_id),
    CONSTRAINT rollouts_engine_status_chk
        CHECK (engine_status IN ('pending','active','halted','held','completed')),
    CONSTRAINT rollouts_status_chk
        CHECK (status IN ('pending','active','halted','held','completed','paused','aborted','rolled_back')),
    CONSTRAINT rollouts_current_phase_chk CHECK (current_phase >= 0),
    CONSTRAINT rollouts_time_order_chk
        CHECK (completed_at IS NULL OR started_at IS NULL OR completed_at >= started_at),
    -- Reconciliation: control-plane status mirrors the engine status unless it
    -- is a control-plane-only overlay.
    CONSTRAINT rollouts_status_reconcile_chk
        CHECK (status = engine_status OR status IN ('paused','aborted','rolled_back'))
);

CREATE INDEX idx_rollouts_status        ON helix_ota.rollouts (status);
CREATE INDEX idx_rollouts_engine_status ON helix_ota.rollouts (engine_status);

-- -----------------------------------------------------------------------------
-- rollback_history — append-only audit of every abort / server-driven recall
-- (rollback). FKs ON DELETE SET NULL so history outlives the referenced
-- deployments/releases (mirrors 001's telemetry_events/audit_logs survivability).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.rollback_history (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id        UUID,
    kind                 VARCHAR(20)  NOT NULL,
    from_release_id      UUID,
    to_release_id        UUID,
    recall_deployment_id UUID,
    reason               TEXT,
    triggered_by         UUID,
    details              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT rollback_history_kind_chk CHECK (kind IN ('abort','rollback')),
    CONSTRAINT rollback_history_deployment_fk
        FOREIGN KEY (deployment_id)        REFERENCES helix_ota.deployments (id) ON DELETE SET NULL,
    CONSTRAINT rollback_history_from_release_fk
        FOREIGN KEY (from_release_id)      REFERENCES helix_ota.releases (id)    ON DELETE SET NULL,
    CONSTRAINT rollback_history_to_release_fk
        FOREIGN KEY (to_release_id)        REFERENCES helix_ota.releases (id)    ON DELETE SET NULL,
    CONSTRAINT rollback_history_recall_fk
        FOREIGN KEY (recall_deployment_id) REFERENCES helix_ota.deployments (id) ON DELETE SET NULL,
    CONSTRAINT rollback_history_user_fk
        FOREIGN KEY (triggered_by)         REFERENCES helix_ota.users (id)       ON DELETE SET NULL,
    -- A 'rollback' records both from/to releases; an 'abort' records neither.
    CONSTRAINT rollback_history_kind_ref_chk CHECK (
        (kind = 'rollback' AND from_release_id IS NOT NULL AND to_release_id IS NOT NULL)
        OR (kind = 'abort' AND from_release_id IS NULL AND to_release_id IS NULL)
    )
);

CREATE INDEX idx_rollback_history_deployment ON helix_ota.rollback_history (deployment_id);
CREATE INDEX idx_rollback_history_kind       ON helix_ota.rollback_history (kind);
CREATE INDEX idx_rollback_history_created    ON helix_ota.rollback_history (created_at);

COMMIT;
