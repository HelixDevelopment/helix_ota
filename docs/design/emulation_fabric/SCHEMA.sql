-- Helix OTA — Emulation Test-Fabric registry (proposed schema)
-- Revision: 1   Last modified: 2026-06-10T14:10:00Z
-- Status: design (proposal — NOT yet applied as a migration)
--
-- Purpose: the distributed-fabric persistence described in DESIGN.md §6. It is
-- OPTIONAL for local single-node use (T0 in-process/podman tiers need no DB) and
-- REQUIRED once distributed fan-out (P3+/P6) is wired, so the scheduler can lease
-- targets exclusively (§11.4.119) and surface fleet-of-fabric state. Reuses the
-- project's existing pgx/Postgres seam + the `helix_ota` schema + the migration
-- conventions in server/internal/store/schema_postgres.sql (idempotent, additive).
-- This file is the design source; the real migration lands in P3 (ROADMAP.md).
--
-- Anti-bluff (§11.4): a `fabric_run` is PASS only when it links >=1 non-empty
-- `fabric_evidence` row (the evidence-ledger rule mirrored from the HelixQA engine).

CREATE SCHEMA IF NOT EXISTS helix_ota;

-- A distributable execution node (a mac dev host, a Linux+KVM CI node, a LAVA
-- worker, a HIL board host). Capability flags gate which tiers it can run.
CREATE TABLE IF NOT EXISTS helix_ota.fabric_nodes (
    node_id      TEXT        PRIMARY KEY,
    kind         TEXT        NOT NULL,             -- 'dev-mac' | 'ci-linux-kvm' | 'lava-worker' | 'hil-host'
    arch         TEXT        NOT NULL,             -- 'arm64' | 'x86_64'
    has_kvm      BOOLEAN     NOT NULL DEFAULT FALSE,
    has_hvf      BOOLEAN     NOT NULL DEFAULT FALSE,
    labels       JSONB       NOT NULL DEFAULT '{}',-- arbitrary scheduler labels/taints
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL
);

-- An emulated/virtual/real target the fabric can run a job against. `tier`
-- mirrors DESIGN.md §2 (T0/T1/T2/T3/Tfw/Tcp). `exclusive` targets MUST be leased
-- to exactly one run at a time (§11.4.119).
CREATE TABLE IF NOT EXISTS helix_ota.fabric_targets (
    target_id    TEXT        PRIMARY KEY,
    tier         TEXT        NOT NULL,             -- 'T0'|'T1'|'T2'|'T3'|'Tfw'|'Tcp'
    tech         TEXT        NOT NULL,             -- 'ota-device-emulator'|'avd-arm64'|'cuttlefish'|'rk3588'|'qemu-virt'|...
    model        TEXT        NOT NULL DEFAULT '',  -- e.g. 'OrangePi5Max'
    os_type      TEXT        NOT NULL DEFAULT 'android',
    exclusive    BOOLEAN     NOT NULL DEFAULT TRUE,
    node_id      TEXT        REFERENCES helix_ota.fabric_nodes(node_id),
    status       TEXT        NOT NULL DEFAULT 'idle', -- 'idle'|'leased'|'offline'
    created_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fabric_targets_tier   ON helix_ota.fabric_targets (tier);
CREATE INDEX IF NOT EXISTS idx_fabric_targets_status ON helix_ota.fabric_targets (status);

-- Exclusive lease (§11.4.119): a UNIQUE partial index guarantees at most one
-- ACTIVE lease per exclusive target. release_at NULL == currently held.
CREATE TABLE IF NOT EXISTS helix_ota.fabric_leases (
    lease_id   TEXT        PRIMARY KEY,
    target_id  TEXT        NOT NULL REFERENCES helix_ota.fabric_targets(target_id),
    owner      TEXT        NOT NULL,               -- the run/stream holding it
    acquired_at TIMESTAMPTZ NOT NULL,
    release_at TIMESTAMPTZ                          -- NULL while held
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_fabric_lease_active
    ON helix_ota.fabric_leases (target_id) WHERE release_at IS NULL;

-- One test run on a target. verdict uses the closed §11.4.45 vocabulary.
CREATE TABLE IF NOT EXISTS helix_ota.fabric_runs (
    run_id      TEXT        PRIMARY KEY,
    target_id   TEXT        NOT NULL REFERENCES helix_ota.fabric_targets(target_id),
    test_type   TEXT        NOT NULL,              -- 'unit'|'integration'|'e2e'|'security'|'chaos'|'stress'|'challenge'|...
    test_ref    TEXT        NOT NULL,              -- script/path/bank-id dispatched
    verdict     TEXT        NOT NULL DEFAULT 'PENDING', -- 'PASS'|'FAIL'|'SKIP'|'PENDING_FORENSICS'|'OPERATOR-BLOCKED'
    skip_reason TEXT        NOT NULL DEFAULT '',    -- closed-set reason when verdict='SKIP' (§11.4.3/§11.4.69)
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_fabric_runs_verdict ON helix_ota.fabric_runs (verdict);

-- The evidence ledger (§11.4.69): a PASS run MUST link >=1 non-empty artefact.
-- byte_size is captured so a 0-byte "evidence" can never satisfy a PASS.
CREATE TABLE IF NOT EXISTS helix_ota.fabric_evidence (
    evidence_id TEXT        PRIMARY KEY,
    run_id      TEXT        NOT NULL REFERENCES helix_ota.fabric_runs(run_id),
    kind        TEXT        NOT NULL,              -- 'transcript'|'screencap'|'ab-slot'|'dm-verity'|'latency'|'sink-probe'|...
    path        TEXT        NOT NULL,              -- docs/qa/<run-id>/... (§11.4.83)
    byte_size   BIGINT      NOT NULL CHECK (byte_size > 0),  -- non-empty by construction
    sha256      TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fabric_evidence_run ON helix_ota.fabric_evidence (run_id);
