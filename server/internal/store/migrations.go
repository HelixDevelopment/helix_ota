package store

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migration is one ordered, forward-only schema step. Version is a strictly
// increasing positive integer; the registered set forms the schema history.
type migration struct {
	Version int64
	Name    string
	SQL     string
	DownSQL string
}

// registeredMigrations is the ordered schema history for the pgx store backend
// (SRV-NEW-1 — the versioned migration framework that replaces the previous
// unconditional whole-schema re-exec). Migration 1 ("baseline") is the full
// current schema (schema_postgres.sql, embedded as postgresSchema) — the
// STORE-1 seq forward-migration and the telemetry ADD COLUMN forward-migrations
// are folded into it (they are already idempotent and already reflected in the
// baseline CREATE statements). Future schema changes append a new numbered
// entry here (2, 3, …) instead of being tacked onto the baseline blob as
// another inline ALTER; the Accounts stream's "002/003" migrations build on
// this framework.
var registeredMigrations = []migration{
	{Version: 1, Name: "baseline", SQL: postgresSchema, DownSQL: baselineMigrationDownSQL},
	{Version: 2, Name: "accounts", SQL: accountsMigrationSQL, DownSQL: accountsMigrationDownSQL},
	{Version: 3, Name: "rollout_schema", SQL: rolloutSchemaMigrationSQL, DownSQL: rolloutSchemaMigrationDownSQL},
	{Version: 4, Name: "accounts_status", SQL: accountsStatusMigrationSQL, DownSQL: accountsStatusMigrationDownSQL},
	{Version: 5, Name: "branches", SQL: branchesMigrationSQL, DownSQL: branchesMigrationDownSQL},
	{Version: 6, Name: "webhooks", SQL: webhooksMigrationSQL, DownSQL: webhooksMigrationDownSQL},
	{Version: 7, Name: "project_members", SQL: projectMembersMigrationSQL, DownSQL: projectMembersMigrationDownSQL},
	{Version: 8, Name: "delta_metadata", SQL: deltaMetadataMigrationSQL, DownSQL: deltaMetadataMigrationDownSQL},
	{Version: 9, Name: "devices_hardware", SQL: devicesHardwareMigrationSQL, DownSQL: devicesHardwareMigrationDownSQL},
	{Version: 10, Name: "rls", SQL: rlsMigrationSQL, DownSQL: rlsMigrationDownSQL},
}

// accountsMigrationSQL is migration 2 ("accounts") — the Accounts M1 tenant
// layer ABOVE Project (design §2). It creates the accounts + account_memberships
// tables and adds the account_id scope column to projects. It is idempotent
// (IF NOT EXISTS / ADD COLUMN IF NOT EXISTS) and FULLY schema-qualified: it runs
// in its own transaction (applyOne), so it must NOT depend on the baseline's
// `SET search_path`.
//
// SCOPE (this first M1 sub-slice): projects.account_id lands NOT NULL with an
// empty-string default (empty = legacy/unscoped), which keeps the in-memory and
// pgx backends byte-identical on the empty-scope case (no NULL-vs-empty-string
// parity hazard). The design's migration 003 (a LATER M1 sub-slice) backfills
// the empty-string scope to a __default__ account, adds the composite
// (project_id, account_id) FK to every OTA
// table, the per-account UNIQUE keys, and enables Row-Level Security — none of
// which land here.
const accountsMigrationSQL = `
CREATE TABLE IF NOT EXISTS helix_ota.accounts (
    -- seq is the insertion-order counter ListAccounts orders by, the same idiom
    -- devices.seq / releases.seq use for a stable, backend-parity listing order.
    seq        BIGSERIAL,
    account_id TEXT PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    slug       TEXT        NOT NULL UNIQUE,
    status     TEXT        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','suspended','archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS helix_ota.account_memberships (
    user_id    TEXT        NOT NULL,
    account_id TEXT        NOT NULL REFERENCES helix_ota.accounts(account_id) ON DELETE CASCADE,
    role       TEXT        NOT NULL DEFAULT 'viewer'
        CHECK (role IN ('viewer','operator','admin')),
    is_owner   BOOLEAN     NOT NULL DEFAULT FALSE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, account_id)
);
CREATE INDEX IF NOT EXISTS idx_account_memberships_user    ON helix_ota.account_memberships (user_id);
CREATE INDEX IF NOT EXISTS idx_account_memberships_account ON helix_ota.account_memberships (account_id);

-- The tenant scope on projects. NOT NULL DEFAULT '' (empty = legacy/unscoped)
-- so the pgx backend matches the in-memory backend's "" default exactly; the
-- '' -> __default__ backfill + composite FK + per-account UNIQUE(account_id,name)
-- are migration 003 (a later M1 sub-slice), not this one.
ALTER TABLE helix_ota.projects ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_projects_account ON helix_ota.projects (account_id);
`

const accountsMigrationDownSQL = `
DROP INDEX IF EXISTS helix_ota.idx_projects_account;
ALTER TABLE helix_ota.projects DROP COLUMN IF EXISTS account_id;
DROP TABLE IF EXISTS helix_ota.account_memberships CASCADE;
DROP TABLE IF EXISTS helix_ota.accounts CASCADE;
`

const baselineMigrationDownSQL = `
DROP TABLE IF EXISTS helix_ota.project_access CASCADE;
DROP TABLE IF EXISTS helix_ota.projects CASCADE;
DROP TABLE IF EXISTS helix_ota.fabric_evidence CASCADE;
DROP TABLE IF EXISTS helix_ota.fabric_runs CASCADE;
DROP TABLE IF EXISTS helix_ota.fabric_leases CASCADE;
DROP TABLE IF EXISTS helix_ota.fabric_targets CASCADE;
DROP TABLE IF EXISTS helix_ota.fabric_nodes CASCADE;
DROP TABLE IF EXISTS helix_ota.idempotency_keys CASCADE;
DROP TABLE IF EXISTS helix_ota.rollback_history CASCADE;
DROP TABLE IF EXISTS helix_ota.delta_artifacts CASCADE;
DROP TABLE IF EXISTS helix_ota.audit_logs CASCADE;
DROP TABLE IF EXISTS helix_ota.device_group_members CASCADE;
DROP TABLE IF EXISTS helix_ota.device_groups CASCADE;
DROP TABLE IF EXISTS helix_ota.telemetry_events CASCADE;
DROP TABLE IF EXISTS helix_ota.deployments CASCADE;
DROP TABLE IF EXISTS helix_ota.releases CASCADE;
DROP TABLE IF EXISTS helix_ota.artifacts CASCADE;
DROP TABLE IF EXISTS helix_ota.devices CASCADE;
`

// rolloutSchemaMigrationSQL is migration 3 ("rollout_schema").
const rolloutSchemaMigrationSQL = `
ALTER TABLE helix_ota.deployments ADD COLUMN IF NOT EXISTS stage_deadline TIMESTAMPTZ;
ALTER TABLE helix_ota.deployments ADD COLUMN IF NOT EXISTS last_evaluated_at TIMESTAMPTZ;
`

const rolloutSchemaMigrationDownSQL = `
ALTER TABLE helix_ota.deployments DROP COLUMN IF EXISTS stage_deadline;
ALTER TABLE helix_ota.deployments DROP COLUMN IF EXISTS last_evaluated_at;
`

// accountsStatusMigrationSQL is migration 4 ("accounts_status").
const accountsStatusMigrationSQL = `
ALTER TABLE helix_ota.accounts ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ;
ALTER TABLE helix_ota.accounts ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
`

const accountsStatusMigrationDownSQL = `
ALTER TABLE helix_ota.accounts DROP COLUMN IF EXISTS suspended_at;
ALTER TABLE helix_ota.accounts DROP COLUMN IF EXISTS archived_at;
`

// branchesMigrationSQL is migration 5 ("branches").
const branchesMigrationSQL = `
CREATE TABLE IF NOT EXISTS helix_ota.branches (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES helix_ota.projects(project_id),
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    UNIQUE(project_id, name)
);
`

const branchesMigrationDownSQL = `
DROP TABLE IF EXISTS helix_ota.branches CASCADE;
`

// webhooksMigrationSQL is migration 6 ("webhooks").
const webhooksMigrationSQL = `
CREATE TABLE IF NOT EXISTS helix_ota.webhooks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES helix_ota.projects(project_id),
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT true,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const webhooksMigrationDownSQL = `
DROP TABLE IF EXISTS helix_ota.webhooks CASCADE;
`

// projectMembersMigrationSQL is migration 7 ("project_members").
const projectMembersMigrationSQL = `
ALTER TABLE helix_ota.project_members ADD COLUMN IF NOT EXISTS added_by UUID;
ALTER TABLE helix_ota.project_members ADD COLUMN IF NOT EXISTS added_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
`

const projectMembersMigrationDownSQL = `
ALTER TABLE helix_ota.project_members DROP COLUMN IF EXISTS added_by;
ALTER TABLE helix_ota.project_members DROP COLUMN IF EXISTS added_at;
`

// deltaMetadataMigrationSQL is migration 8 ("delta_metadata").
const deltaMetadataMigrationSQL = `
CREATE TABLE IF NOT EXISTS helix_ota.delta_metadata (
    id UUID PRIMARY KEY,
    artifact_id UUID NOT NULL,
    base_artifact_id UUID,
    delta_size BIGINT,
    algorithm TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const deltaMetadataMigrationDownSQL = `
DROP TABLE IF EXISTS helix_ota.delta_metadata CASCADE;
`

// devicesHardwareMigrationSQL is migration 9 ("devices_hardware").
const devicesHardwareMigrationSQL = `
ALTER TABLE helix_ota.devices ADD COLUMN IF NOT EXISTS hardware_capabilities JSONB DEFAULT '{}';
`

const devicesHardwareMigrationDownSQL = `
ALTER TABLE helix_ota.devices DROP COLUMN IF EXISTS hardware_capabilities;
`

// rlsMigrationSQL is migration 10 ("rls").
const rlsMigrationSQL = `
REVOKE ALL ON helix_ota.accounts, helix_ota.projects, helix_ota.project_members, helix_ota.devices, helix_ota.deployments FROM public;
GRANT SELECT, INSERT, UPDATE, DELETE ON helix_ota.accounts, helix_ota.projects, helix_ota.project_members, helix_ota.devices, helix_ota.deployments TO app_user;

ALTER TABLE helix_ota.accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.project_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.deployments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON helix_ota.accounts FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);
CREATE POLICY tenant_isolation ON helix_ota.projects FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);
CREATE POLICY tenant_isolation ON helix_ota.project_members FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);
CREATE POLICY tenant_isolation ON helix_ota.devices FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);
CREATE POLICY tenant_isolation ON helix_ota.deployments FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);
`

const rlsMigrationDownSQL = `
DROP POLICY IF EXISTS tenant_isolation ON helix_ota.accounts;
DROP POLICY IF EXISTS tenant_isolation ON helix_ota.projects;
DROP POLICY IF EXISTS tenant_isolation ON helix_ota.project_members;
DROP POLICY IF EXISTS tenant_isolation ON helix_ota.devices;
DROP POLICY IF EXISTS tenant_isolation ON helix_ota.deployments;

ALTER TABLE helix_ota.accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.projects DISABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.project_members DISABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.devices DISABLE ROW LEVEL SECURITY;
ALTER TABLE helix_ota.deployments DISABLE ROW LEVEL SECURITY;

REVOKE SELECT, INSERT, UPDATE, DELETE ON helix_ota.accounts, helix_ota.projects, helix_ota.project_members, helix_ota.devices, helix_ota.deployments FROM app_user;
`

// schemaMigrationsDDL bootstraps the applied-version ledger. It is deliberately
// NOT itself a versioned migration: the ledger must exist before we can read
// which versions have been applied (the standard chicken-and-egg bootstrap that
// goose / golang-migrate use). Idempotent — safe to run on every ApplyMigrations.
const schemaMigrationsDDL = `
CREATE SCHEMA IF NOT EXISTS helix_ota;
CREATE TABLE IF NOT EXISTS helix_ota.schema_migrations (
    version    BIGINT      PRIMARY KEY,
    name       TEXT        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// validateMigrations enforces the registry invariants: at least one migration,
// versions strictly increasing with no duplicates and no gaps, starting at 1,
// and each with a non-empty name + SQL. It returns the first violation as an
// error (§11.4.6 — a mis-ordered, gapped, or duplicate registry fails loudly at
// bring-up, never silently applies an out-of-order or partial schema history).
func validateMigrations(ms []migration) error {
	if len(ms) == 0 {
		return fmt.Errorf("store: migration registry is empty")
	}
	for i, m := range ms {
		want := int64(i + 1)
		if m.Version != want {
			return fmt.Errorf("store: migration[%d] version=%d, want %d "+
				"(versions must start at 1, increase by 1, no gaps or duplicates)", i, m.Version, want)
		}
		if m.Name == "" {
			return fmt.Errorf("store: migration %d has an empty name", m.Version)
		}
		if m.SQL == "" {
			return fmt.Errorf("store: migration %d (%s) has empty SQL", m.Version, m.Name)
		}
	}
	return nil
}

// pendingMigrations returns, in ascending version order, the registered
// migrations whose version is not present in applied. It errors if the ledger
// records a version unknown to this binary's registry — a DB migrated by a
// NEWER build must not be silently re-driven (or downgraded) by an older one
// (§11.4.6 — never guess the DB is merely "behind").
func pendingMigrations(ms []migration, applied map[int64]bool) ([]migration, error) {
	known := make(map[int64]bool, len(ms))
	for _, m := range ms {
		known[m.Version] = true
	}
	for v := range applied {
		if !known[v] {
			return nil, fmt.Errorf("store: schema_migrations records version %d "+
				"unknown to this binary (DB migrated by a newer build?)", v)
		}
	}
	var pending []migration
	for _, m := range ms {
		if !applied[m.Version] {
			pending = append(pending, m)
		}
	}
	// ms is already ascending after validateMigrations; sort defensively so the
	// apply order does not silently depend on registry declaration order.
	sort.Slice(pending, func(i, j int) bool { return pending[i].Version < pending[j].Version })
	return pending, nil
}

// migrationExecutor is the database side of the apply loop, abstracted so the
// ordering / idempotency / ledger-recording logic is unit-testable WITHOUT a
// real database (the pgx implementation needs -tags integration + live
// Postgres; a fake in-memory executor drives the same loop under plain
// `go test`).
type migrationExecutor interface {
	// appliedVersions returns the set of versions recorded in the ledger.
	appliedVersions(ctx context.Context) (map[int64]bool, error)
	// applyOne runs the migration's SQL AND records it in the ledger
	// ATOMICALLY (one transaction) — either both land or neither does.
	applyOne(ctx context.Context, m migration) error
}

// applyMigrations is the transport-independent apply loop: validate the
// registry, read the applied set, then apply only the pending migrations in
// ascending version order, each recorded exactly once. Idempotent — a
// fully-migrated DB applies nothing. Returns the versions applied THIS run (in
// order); on an applyOne failure it returns the versions applied so far plus
// the error, leaving remaining migrations for a later run (each migration is
// atomic, so the ledger never records a half-applied step).
func applyMigrations(ctx context.Context, ms []migration, ex migrationExecutor) ([]int64, error) {
	if err := validateMigrations(ms); err != nil {
		return nil, err
	}
	applied, err := ex.appliedVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: read schema_migrations ledger: %w", err)
	}
	pending, err := pendingMigrations(ms, applied)
	if err != nil {
		return nil, err
	}
	done := make([]int64, 0, len(pending))
	for _, m := range pending {
		if err := ex.applyOne(ctx, m); err != nil {
			return done, fmt.Errorf("store: apply migration %d (%s): %w", m.Version, m.Name, err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// pgxMigrationExecutor is the real (pgx / PostgreSQL) migrationExecutor.
type pgxMigrationExecutor struct {
	pool *pgxpool.Pool
}

func (e *pgxMigrationExecutor) appliedVersions(ctx context.Context) (map[int64]bool, error) {
	rows, err := e.pool.Query(ctx, "SELECT version FROM helix_ota.schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (e *pgxMigrationExecutor) applyOne(ctx context.Context, m migration) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO helix_ota.schema_migrations (version, name) VALUES ($1, $2)",
		m.Version, m.Name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ApplyMigrations brings the store schema up to date via the versioned
// migration framework: it ensures the schema_migrations ledger exists, then
// applies every registered migration not yet recorded, in ascending version
// order, one transaction per migration, recording each in the ledger.
// Idempotent — running it against an already-current DB is a no-op (it applies
// nothing). This REPLACES the previous unconditional whole-schema re-exec: a
// fresh DB ends in exactly the same schema state as before (baseline = the full
// schema_postgres.sql), and an already-current DB is now a genuine no-op gated
// by the ledger rather than a blind re-run of the whole DDL blob.
func (r *PostgresRepository) ApplyMigrations(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("store: bootstrap schema_migrations ledger: %w", err)
	}
	_, err := applyMigrations(ctx, registeredMigrations, &pgxMigrationExecutor{pool: r.pool})
	return err
}
