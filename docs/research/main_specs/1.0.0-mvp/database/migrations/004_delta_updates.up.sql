-- =============================================================================
-- Helix OTA — 1.0.3 delta / incremental updates schema (UP migration)
-- Migration:    004_delta_updates
-- Direction:    up
-- Target:       PostgreSQL 14+ (same as 001 / 002)
-- Schema:       helix_ota (extends migration 001; independent of 002)
-- Scope:        1.0.3 delta updates — the base->target relationship the MVP
--               schema could not express. The MVP artifacts table already
--               permits artifact_type IN ('full','incremental','delta')
--               (001 lines 144, 159-160), so a delta payload is storable as an
--               artifacts row today; this migration adds the per-pair
--               *relationship* + generation lifecycle, not the type value.
-- Provenance:   delta_updates_design.md §4 / §4.1(a) (separate delta_artifacts
--               table) / §4.2 (reconcile source DDL to the canonical UUID +
--               helix_ota model) / §3.2-§3.3 (generation pipeline + status
--               machine). README.md §5 numbering: 1.0.1=002_*, 1.0.2=003_*
--               (reserved/unused), delta=004_*.
-- Note:         Purely additive — no ALTER on any 001 table. ADR-0005 is
--               Proposed; this DDL backs that design and validates up/down
--               against live Postgres ahead of binding.
-- =============================================================================

BEGIN;

SET LOCAL search_path = helix_ota, public;

-- -----------------------------------------------------------------------------
-- delta_artifacts — the base->target relationship for a generated delta.
--
-- A delta encodes the binary difference between a *base* (source) full artifact
-- a device currently runs and a *target* full artifact it is moving to. The
-- delta payload itself is stored as its own artifacts row (artifact_type =
-- 'delta'), and `delta_artifact_id` points at it once generation publishes
-- (NULL while PENDING / GENERATING / FAILED).
--
--   * base_artifact_id   -> the source full artifact (device's current image)
--   * target_artifact_id -> the target full artifact (the upgrade destination)
--   * delta_artifact_id  -> the generated delta payload (an artifacts row)
--
-- All three FKs are UUID -> helix_ota.artifacts(id) ON DELETE CASCADE (design
-- §4.2): a delta is meaningless once any leg is gone, so the relationship row
-- is removed with it. The delta file's own integrity columns (file_path /
-- file_size / checksum_sha256) mirror the artifacts conventions (001 §artifacts)
-- so this row is self-describing for serving even before the published delta
-- artifacts row is joined. The PENDING/GENERATING/... lifecycle lives here, off
-- the artifacts row (design §3.3 / §4.1(a)).
-- -----------------------------------------------------------------------------
CREATE TABLE helix_ota.delta_artifacts (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    base_artifact_id   UUID         NOT NULL,
    target_artifact_id UUID         NOT NULL,
    delta_artifact_id  UUID,
    status             VARCHAR(50)  NOT NULL DEFAULT 'pending',
    -- The delta file (the generated incremental OTA zip). Populated on publish;
    -- NULL while the job has not produced a verified payload yet.
    file_path          VARCHAR(500),
    file_size          BIGINT,
    checksum_sha256    CHAR(64),
    -- UNVERIFIED measured savings (design §7 / §9 U1): NULL until measured on
    -- real hardware; never a propagated source projection.
    savings_percent    NUMERIC(5,2),
    -- Per-partition op stats (SOURCE_COPY / SOURCE_BSDIFF / PUFFDIFF sizes etc.)
    -- and the documented delta hints (design §4.2). JSONB, not new columns.
    partition_deltas   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    generation_errors  JSONB,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT delta_artifacts_base_fk
        FOREIGN KEY (base_artifact_id)   REFERENCES helix_ota.artifacts (id) ON DELETE CASCADE,
    CONSTRAINT delta_artifacts_target_fk
        FOREIGN KEY (target_artifact_id) REFERENCES helix_ota.artifacts (id) ON DELETE CASCADE,
    CONSTRAINT delta_artifacts_delta_fk
        FOREIGN KEY (delta_artifact_id)  REFERENCES helix_ota.artifacts (id) ON DELETE CASCADE,
    -- Generation status machine (design §3.3):
    -- PENDING -> GENERATING -> GENERATED | FAILED | CANCELLED.
    CONSTRAINT delta_artifacts_status_chk
        CHECK (status IN ('pending', 'generating', 'generated', 'failed', 'cancelled')),
    -- A delta from a build to itself is nonsensical (design §3.2 stage 1 also
    -- rejects cross-OS / downgrade pairs at the service layer; base != target is
    -- the structural invariant defended here at the DB layer).
    CONSTRAINT delta_artifacts_base_ne_target_chk
        CHECK (base_artifact_id <> target_artifact_id),
    -- Exactly one delta-relationship row per (base, target) pair.
    CONSTRAINT delta_artifacts_base_target_uniq
        UNIQUE (base_artifact_id, target_artifact_id),
    -- Same integrity shape as artifacts: SHA-256 is 64 lowercase hex chars when
    -- present; file_size strictly positive when present (NULL while unpublished).
    CONSTRAINT delta_artifacts_sha256_chk
        CHECK (checksum_sha256 IS NULL OR checksum_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT delta_artifacts_file_size_chk
        CHECK (file_size IS NULL OR file_size > 0),
    CONSTRAINT delta_artifacts_savings_chk
        CHECK (savings_percent IS NULL OR (savings_percent >= 0 AND savings_percent <= 100))
);

-- "Find a delta from base X to target Y": the UNIQUE(base, target) above already
-- backs the exact-pair lookup. These add the two single-leg fan-out queries the
-- selection matrix runs at update-check time (design §6.1):
--   * all deltas FROM a given base (which targets can this device jump to)
--   * all deltas TO a given target (which sources can be served a delta)
CREATE INDEX idx_delta_artifacts_base   ON helix_ota.delta_artifacts (base_artifact_id);
CREATE INDEX idx_delta_artifacts_target ON helix_ota.delta_artifacts (target_artifact_id);
-- Selection at update-check filters to servable (status='generated') pairs.
CREATE INDEX idx_delta_artifacts_status ON helix_ota.delta_artifacts (status);
-- Join from the published delta payload back to its relationship row.
CREATE INDEX idx_delta_artifacts_delta  ON helix_ota.delta_artifacts (delta_artifact_id);

COMMIT;
