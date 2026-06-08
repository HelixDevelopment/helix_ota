package rollout

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed rollout_schema.sql
var postgresSchema string

// PostgresStore is a pgx-backed engine.StoragePort — the production persistence
// for staged-rollout state. It satisfies the exact same brick port as
// MemoryStore; the shared scenario test asserts parity on a real database.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// compile-time assertion that PostgresStore satisfies the brick port.
var _ engine.StoragePort = (*PostgresStore)(nil)

// NewPostgresStore opens a pool and returns a StoragePort.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("rollout: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("rollout: ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// NewPostgresStoreFromPool wraps an existing pool (tests).
func NewPostgresStoreFromPool(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

// Migrate applies the rollout schema DDL (idempotent).
func (s *PostgresStore) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, postgresSchema); err != nil {
		return fmt.Errorf("rollout: apply schema: %w", err)
	}
	return nil
}

// Close releases the pool.
func (s *PostgresStore) Close() { s.pool.Close() }

// Save upserts the state row and rewrites the (immutable) phase plan atomically.
func (s *PostgresStore) Save(ctx context.Context, st engine.State) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // best-effort on the error path

	var phaseStarted any
	if !st.PhaseStartedAt.IsZero() {
		phaseStarted = st.PhaseStartedAt
	}
	if _, err = tx.Exec(ctx, `
INSERT INTO helix_ota.rollout_states (deployment_id, current_phase, status, phase_started_at, updated_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (deployment_id) DO UPDATE SET
  current_phase=EXCLUDED.current_phase, status=EXCLUDED.status,
  phase_started_at=EXCLUDED.phase_started_at, updated_at=EXCLUDED.updated_at`,
		st.DeploymentID, st.CurrentPhase, string(st.Status), phaseStarted, st.UpdatedAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM helix_ota.rollout_phases WHERE deployment_id=$1`, st.DeploymentID); err != nil {
		return err
	}
	for i, p := range st.Phases {
		if _, err = tx.Exec(ctx, `
INSERT INTO helix_ota.rollout_phases
 (deployment_id, phase_index, percentage, success_threshold, error_threshold, duration_ns, auto_progress)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			st.DeploymentID, i, p.Percentage, p.SuccessThreshold, p.ErrorThreshold,
			int64(p.Duration), p.AutoProgress); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Load reconstructs the State, or engine.ErrNotFound when absent.
func (s *PostgresStore) Load(ctx context.Context, deploymentID string) (engine.State, error) {
	var st engine.State
	st.DeploymentID = deploymentID
	var status string
	var phaseStarted *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT current_phase, status, phase_started_at, updated_at
FROM helix_ota.rollout_states WHERE deployment_id=$1`, deploymentID).
		Scan(&st.CurrentPhase, &status, &phaseStarted, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return engine.State{}, engine.ErrNotFound
		}
		return engine.State{}, err
	}
	st.Status = engine.Status(status)
	if phaseStarted != nil {
		st.PhaseStartedAt = *phaseStarted
	}

	rows, err := s.pool.Query(ctx, `
SELECT percentage, success_threshold, error_threshold, duration_ns, auto_progress
FROM helix_ota.rollout_phases WHERE deployment_id=$1 ORDER BY phase_index`, deploymentID)
	if err != nil {
		return engine.State{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var p engine.Phase
		var durNS int64
		if serr := rows.Scan(&p.Percentage, &p.SuccessThreshold, &p.ErrorThreshold, &durNS, &p.AutoProgress); serr != nil {
			return engine.State{}, serr
		}
		p.Duration = time.Duration(durNS)
		st.Phases = append(st.Phases, p)
	}
	return st, rows.Err()
}
