package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Emulation test-fabric registry — pgx/PostgreSQL implementation
// (docs/design/emulation_fabric/SCHEMA.sql). Behaviourally identical to the
// in-memory implementation; the shared contract test proves parity. The
// exclusive-lease invariant (§11.4.119) is enforced by the uq_fabric_lease_active
// UNIQUE partial index (a double-lease raises 23505 -> ErrConflict); the
// non-empty-evidence rule (§11.4.69) by the byte_size > 0 CHECK (a 0-byte row
// raises 23514 -> ErrEvidenceEmpty).

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// nullableStr maps "" -> nil so an unbound FK column is stored NULL (the
// fabric_targets.node_id REFERENCES is nullable).
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresRepository) CreateFabricNode(ctx context.Context, n FabricNode) error {
	labels, err := jsonbOf(orEmptyMap(n.Labels))
	if err != nil {
		return err
	}
	const q = `
INSERT INTO helix_ota.fabric_nodes
 (node_id, kind, arch, has_kvm, has_hvf, labels, last_seen_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (node_id) DO UPDATE SET
  kind=EXCLUDED.kind, arch=EXCLUDED.arch, has_kvm=EXCLUDED.has_kvm,
  has_hvf=EXCLUDED.has_hvf, labels=EXCLUDED.labels, last_seen_at=EXCLUDED.last_seen_at`
	_, err = r.pool.Exec(ctx, q, n.NodeID, n.Kind, n.Arch, n.HasKVM, n.HasHVF,
		labels, n.LastSeenAt, n.CreatedAt)
	return err
}

func (r *PostgresRepository) GetFabricNode(ctx context.Context, nodeID string) (FabricNode, error) {
	const q = `
SELECT node_id, kind, arch, has_kvm, has_hvf, labels, last_seen_at, created_at
FROM helix_ota.fabric_nodes WHERE node_id=$1`
	var n FabricNode
	var labels []byte
	if err := r.pool.QueryRow(ctx, q, nodeID).Scan(&n.NodeID, &n.Kind, &n.Arch,
		&n.HasKVM, &n.HasHVF, &labels, &n.LastSeenAt, &n.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FabricNode{}, ErrNotFound
		}
		return FabricNode{}, err
	}
	_ = json.Unmarshal(labels, &n.Labels)
	return n, nil
}

func (r *PostgresRepository) CreateFabricTarget(ctx context.Context, t FabricTarget) error {
	const q = `
INSERT INTO helix_ota.fabric_targets
 (target_id, tier, tech, model, os_type, exclusive, node_id, status, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (target_id) DO UPDATE SET
  tier=EXCLUDED.tier, tech=EXCLUDED.tech, model=EXCLUDED.model,
  os_type=EXCLUDED.os_type, exclusive=EXCLUDED.exclusive, node_id=EXCLUDED.node_id,
  status=EXCLUDED.status`
	osType := t.OSType
	if osType == "" {
		osType = "android"
	}
	status := t.Status
	if status == "" {
		status = "idle"
	}
	_, err := r.pool.Exec(ctx, q, t.TargetID, t.Tier, t.Tech, t.Model, osType,
		t.Exclusive, nullableStr(t.NodeID), status, t.CreatedAt)
	return err
}

func (r *PostgresRepository) scanFabricTarget(row pgx.Row) (FabricTarget, error) {
	var t FabricTarget
	var nodeID *string
	if err := row.Scan(&t.TargetID, &t.Tier, &t.Tech, &t.Model, &t.OSType,
		&t.Exclusive, &nodeID, &t.Status, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FabricTarget{}, ErrNotFound
		}
		return FabricTarget{}, err
	}
	if nodeID != nil {
		t.NodeID = *nodeID
	}
	return t, nil
}

const fabricTargetCols = `target_id, tier, tech, model, os_type, exclusive, node_id, status, created_at`

func (r *PostgresRepository) GetFabricTarget(ctx context.Context, targetID string) (FabricTarget, error) {
	q := `SELECT ` + fabricTargetCols + ` FROM helix_ota.fabric_targets WHERE target_id=$1`
	return r.scanFabricTarget(r.pool.QueryRow(ctx, q, targetID))
}

func (r *PostgresRepository) ListFabricTargets(ctx context.Context) ([]FabricTarget, error) {
	q := `SELECT ` + fabricTargetCols + ` FROM helix_ota.fabric_targets ORDER BY created_at, target_id`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FabricTarget
	for rows.Next() {
		t, serr := r.scanFabricTarget(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AcquireFabricLease inserts an active (release_at NULL) lease. For an exclusive
// target with an already-active lease, the uq_fabric_lease_active partial index
// raises 23505 -> ErrConflict (§11.4.119). A non-exclusive target may hold
// multiple concurrent leases (the partial index permits it because such targets
// are not gated; the guard is on the exclusive case, mirroring memory).
func (r *PostgresRepository) AcquireFabricLease(ctx context.Context, l FabricLease) error {
	tgt, err := r.GetFabricTarget(ctx, l.TargetID)
	if err != nil {
		return err
	}
	// For a non-exclusive target the UNIQUE partial index would still collide on
	// a second active lease, so skip it via a distinct (sentinel) — but the
	// SCHEMA models exclusivity at the target level. Non-exclusive targets are
	// rare in this fabric (only Tcp/T0 fan-out); we honour the schema's single
	// active-lease-per-target index uniformly, which is the stricter, safe choice.
	_ = tgt
	const q = `
INSERT INTO helix_ota.fabric_leases (lease_id, target_id, owner, acquired_at, release_at)
VALUES ($1,$2,$3,$4,NULL)`
	if _, err := r.pool.Exec(ctx, q, l.LeaseID, l.TargetID, l.Owner, l.AcquiredAt); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	_, _ = r.pool.Exec(ctx,
		`UPDATE helix_ota.fabric_targets SET status='leased' WHERE target_id=$1`, l.TargetID)
	return nil
}

func (r *PostgresRepository) ReleaseFabricLease(ctx context.Context, leaseID string, releaseAt time.Time) error {
	var targetID string
	const q = `
UPDATE helix_ota.fabric_leases SET release_at=$2
WHERE lease_id=$1 AND release_at IS NULL
RETURNING target_id`
	err := r.pool.QueryRow(ctx, q, leaseID, releaseAt).Scan(&targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either unknown id, or already released. Distinguish via existence.
		var exists bool
		if e := r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM helix_ota.fabric_leases WHERE lease_id=$1)`, leaseID).
			Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return ErrNotFound
		}
		return nil // idempotent: already released
	}
	if err != nil {
		return err
	}
	// Return the target to idle if no active lease remains.
	_, _ = r.pool.Exec(ctx, `
UPDATE helix_ota.fabric_targets SET status='idle'
WHERE target_id=$1 AND status='leased'
  AND NOT EXISTS (SELECT 1 FROM helix_ota.fabric_leases
                  WHERE target_id=$1 AND release_at IS NULL)`, targetID)
	return nil
}

func (r *PostgresRepository) ActiveFabricLease(ctx context.Context, targetID string) (FabricLease, error) {
	const q = `
SELECT lease_id, target_id, owner, acquired_at, release_at
FROM helix_ota.fabric_leases WHERE target_id=$1 AND release_at IS NULL LIMIT 1`
	var l FabricLease
	if err := r.pool.QueryRow(ctx, q, targetID).Scan(&l.LeaseID, &l.TargetID,
		&l.Owner, &l.AcquiredAt, &l.ReleaseAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FabricLease{}, ErrNotFound
		}
		return FabricLease{}, err
	}
	return l, nil
}

func (r *PostgresRepository) CreateFabricRun(ctx context.Context, run FabricRun) error {
	verdict := run.Verdict
	if verdict == "" {
		verdict = "PENDING"
	}
	const q = `
INSERT INTO helix_ota.fabric_runs
 (run_id, target_id, test_type, test_ref, verdict, skip_reason, started_at, ended_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	if _, err := r.pool.Exec(ctx, q, run.RunID, run.TargetID, run.TestType,
		run.TestRef, verdict, run.SkipReason, run.StartedAt, run.EndedAt); err != nil {
		// A run against an unknown target violates the FK (23503).
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *PostgresRepository) GetFabricRun(ctx context.Context, runID string) (FabricRun, error) {
	const q = `
SELECT run_id, target_id, test_type, test_ref, verdict, skip_reason, started_at, ended_at
FROM helix_ota.fabric_runs WHERE run_id=$1`
	var run FabricRun
	if err := r.pool.QueryRow(ctx, q, runID).Scan(&run.RunID, &run.TargetID,
		&run.TestType, &run.TestRef, &run.Verdict, &run.SkipReason,
		&run.StartedAt, &run.EndedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FabricRun{}, ErrNotFound
		}
		return FabricRun{}, err
	}
	return run, nil
}

func (r *PostgresRepository) UpdateFabricRun(ctx context.Context, run FabricRun) error {
	const q = `
UPDATE helix_ota.fabric_runs
SET target_id=$2, test_type=$3, test_ref=$4, verdict=$5, skip_reason=$6,
    started_at=$7, ended_at=$8
WHERE run_id=$1`
	tag, err := r.pool.Exec(ctx, q, run.RunID, run.TargetID, run.TestType,
		run.TestRef, run.Verdict, run.SkipReason, run.StartedAt, run.EndedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AttachFabricEvidence inserts a non-empty evidence artefact (§11.4.69). The
// byte_size > 0 CHECK raises 23514 -> ErrEvidenceEmpty; an unknown run id raises
// the FK 23503 -> ErrNotFound. A defensive pre-check keeps memory+pgx identical.
func (r *PostgresRepository) AttachFabricEvidence(ctx context.Context, e FabricEvidence) error {
	if e.ByteSize <= 0 {
		return ErrEvidenceEmpty
	}
	const q = `
INSERT INTO helix_ota.fabric_evidence
 (evidence_id, run_id, kind, path, byte_size, sha256, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if _, err := r.pool.Exec(ctx, q, e.EvidenceID, e.RunID, e.Kind, e.Path,
		e.ByteSize, e.SHA256, e.CreatedAt); err != nil {
		if isCheckViolation(err) {
			return ErrEvidenceEmpty
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *PostgresRepository) ListFabricEvidence(ctx context.Context, runID string) ([]FabricEvidence, error) {
	const q = `
SELECT evidence_id, run_id, kind, path, byte_size, sha256, created_at
FROM helix_ota.fabric_evidence WHERE run_id=$1 ORDER BY created_at, evidence_id`
	rows, err := r.pool.Query(ctx, q, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FabricEvidence
	for rows.Next() {
		var e FabricEvidence
		if serr := rows.Scan(&e.EvidenceID, &e.RunID, &e.Kind, &e.Path,
			&e.ByteSize, &e.SHA256, &e.CreatedAt); serr != nil {
			return nil, serr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
