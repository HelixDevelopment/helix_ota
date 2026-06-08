package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema_postgres.sql
var postgresSchema string

// PostgresRepository is the pgx/PostgreSQL implementation of Repository, the
// production persistence target (architecture.md §4). It satisfies the exact
// same contract as MemoryRepository; the shared contract test asserts parity.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// compile-time assertion that PostgresRepository satisfies Repository.
var _ Repository = (*PostgresRepository)(nil)

// NewPostgresRepository opens a pgx pool against dsn and returns a Repository.
func NewPostgresRepository(ctx context.Context, dsn string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping postgres: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

// NewPostgresRepositoryFromPool wraps an existing pool (used by tests).
func NewPostgresRepositoryFromPool(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Migrate applies the store schema DDL (idempotent). It is the on-demand
// bring-up step for the integration test and for first-run provisioning.
func (r *PostgresRepository) Migrate(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, postgresSchema); err != nil {
		return fmt.Errorf("store: apply schema: %w", err)
	}
	return nil
}

// Close releases the pool.
func (r *PostgresRepository) Close() { r.pool.Close() }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func jsonbOf(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// --- devices ---

func (r *PostgresRepository) CreateDevice(ctx context.Context, d Device) error {
	meta, err := jsonbOf(orEmptyMap(d.Metadata))
	if err != nil {
		return err
	}
	const q = `
INSERT INTO helix_ota.devices
 (device_id, hardware_id, model, os_type, os_version, current_version, group_name,
  metadata, registered_at, last_seen, update_state, active_slot, last_error_code,
  health_ok, target_version)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (device_id) DO UPDATE SET
  hardware_id=EXCLUDED.hardware_id, model=EXCLUDED.model, os_type=EXCLUDED.os_type,
  os_version=EXCLUDED.os_version, current_version=EXCLUDED.current_version,
  group_name=EXCLUDED.group_name, metadata=EXCLUDED.metadata,
  registered_at=EXCLUDED.registered_at, last_seen=EXCLUDED.last_seen,
  update_state=EXCLUDED.update_state, active_slot=EXCLUDED.active_slot,
  last_error_code=EXCLUDED.last_error_code, health_ok=EXCLUDED.health_ok,
  target_version=EXCLUDED.target_version`
	_, err = r.pool.Exec(ctx, q,
		d.DeviceID, d.HardwareID, d.Model, string(d.OSType), d.OSVersion, d.CurrentVersion,
		d.Group, meta, d.RegisteredAt, nullTime(d.LastSeen), d.UpdateState, d.ActiveSlot,
		d.LastErrorCode, d.HealthOK, d.TargetVersion)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *PostgresRepository) GetDevice(ctx context.Context, deviceID string) (Device, error) {
	return r.scanDevice(r.pool.QueryRow(ctx, deviceSelect+` WHERE device_id=$1`, deviceID))
}

func (r *PostgresRepository) GetDeviceByHardwareID(ctx context.Context, hardwareID string) (Device, error) {
	return r.scanDevice(r.pool.QueryRow(ctx, deviceSelect+` WHERE hardware_id=$1`, hardwareID))
}

func (r *PostgresRepository) UpdateDevice(ctx context.Context, d Device) error {
	meta, err := jsonbOf(orEmptyMap(d.Metadata))
	if err != nil {
		return err
	}
	const q = `
UPDATE helix_ota.devices SET
  hardware_id=$2, model=$3, os_type=$4, os_version=$5, current_version=$6,
  group_name=$7, metadata=$8, registered_at=$9, last_seen=$10, update_state=$11,
  active_slot=$12, last_error_code=$13, health_ok=$14, target_version=$15
WHERE device_id=$1`
	ct, err := r.pool.Exec(ctx, q,
		d.DeviceID, d.HardwareID, d.Model, string(d.OSType), d.OSVersion, d.CurrentVersion,
		d.Group, meta, d.RegisteredAt, nullTime(d.LastSeen), d.UpdateState, d.ActiveSlot,
		d.LastErrorCode, d.HealthOK, d.TargetVersion)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const deviceSelect = `
SELECT device_id, hardware_id, model, os_type, os_version, current_version, group_name,
       metadata, registered_at, last_seen, update_state, active_slot, last_error_code,
       health_ok, target_version
FROM helix_ota.devices`

func (r *PostgresRepository) scanDevice(row pgx.Row) (Device, error) {
	var d Device
	var osType string
	var meta []byte
	var lastSeen *time.Time
	if err := row.Scan(&d.DeviceID, &d.HardwareID, &d.Model, &osType, &d.OSVersion,
		&d.CurrentVersion, &d.Group, &meta, &d.RegisteredAt, &lastSeen, &d.UpdateState,
		&d.ActiveSlot, &d.LastErrorCode, &d.HealthOK, &d.TargetVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Device{}, ErrNotFound
		}
		return Device{}, err
	}
	d.OSType = otaprotocol.OSType(osType)
	if lastSeen != nil {
		d.LastSeen = *lastSeen
	}
	_ = json.Unmarshal(meta, &d.Metadata)
	return d, nil
}

// --- artifacts ---

func (r *PostgresRepository) CreateArtifact(ctx context.Context, a Artifact) error {
	props, err := jsonbOf(a.PayloadProperties)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO helix_ota.artifacts
 (artifact_id, sha256, size, os_type, target_model, version, storage_ref, verified,
  uploaded_at, signature, payload_properties)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (artifact_id) DO UPDATE SET
  sha256=EXCLUDED.sha256, size=EXCLUDED.size, os_type=EXCLUDED.os_type,
  target_model=EXCLUDED.target_model, version=EXCLUDED.version,
  storage_ref=EXCLUDED.storage_ref, verified=EXCLUDED.verified,
  uploaded_at=EXCLUDED.uploaded_at, signature=EXCLUDED.signature,
  payload_properties=EXCLUDED.payload_properties`
	_, err = r.pool.Exec(ctx, q, a.ArtifactID, a.SHA256, a.Size, string(a.OSType),
		a.TargetModel, a.Version, a.StorageRef, a.Verified, a.UploadedAt, a.Signature, props)
	return err
}

func (r *PostgresRepository) GetArtifact(ctx context.Context, artifactID string) (Artifact, error) {
	const q = `
SELECT artifact_id, sha256, size, os_type, target_model, version, storage_ref,
       verified, uploaded_at, signature, payload_properties
FROM helix_ota.artifacts WHERE artifact_id=$1`
	var a Artifact
	var osType string
	var props []byte
	if err := r.pool.QueryRow(ctx, q, artifactID).Scan(&a.ArtifactID, &a.SHA256, &a.Size,
		&osType, &a.TargetModel, &a.Version, &a.StorageRef, &a.Verified, &a.UploadedAt,
		&a.Signature, &props); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Artifact{}, ErrNotFound
		}
		return Artifact{}, err
	}
	a.OSType = otaprotocol.OSType(osType)
	_ = json.Unmarshal(props, &a.PayloadProperties)
	return a, nil
}

// --- releases ---

func (r *PostgresRepository) CreateRelease(ctx context.Context, rel Release) error {
	const q = `
INSERT INTO helix_ota.releases
 (release_id, artifact_id, version, os_type, target_model, status, notes,
  min_current_version, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (release_id) DO UPDATE SET
  artifact_id=EXCLUDED.artifact_id, version=EXCLUDED.version, os_type=EXCLUDED.os_type,
  target_model=EXCLUDED.target_model, status=EXCLUDED.status, notes=EXCLUDED.notes,
  min_current_version=EXCLUDED.min_current_version, created_at=EXCLUDED.created_at`
	_, err := r.pool.Exec(ctx, q, rel.ReleaseID, rel.ArtifactID, rel.Version, string(rel.OSType),
		rel.TargetModel, rel.Status, rel.Notes, rel.MinCurrentVersion, rel.CreatedAt)
	return err
}

const releaseSelect = `
SELECT release_id, artifact_id, version, os_type, target_model, status, notes,
       min_current_version, created_at
FROM helix_ota.releases`

func scanRelease(row pgx.Row) (Release, error) {
	var rel Release
	var osType string
	if err := row.Scan(&rel.ReleaseID, &rel.ArtifactID, &rel.Version, &osType, &rel.TargetModel,
		&rel.Status, &rel.Notes, &rel.MinCurrentVersion, &rel.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Release{}, ErrNotFound
		}
		return Release{}, err
	}
	rel.OSType = otaprotocol.OSType(osType)
	return rel, nil
}

func (r *PostgresRepository) GetRelease(ctx context.Context, releaseID string) (Release, error) {
	return scanRelease(r.pool.QueryRow(ctx, releaseSelect+` WHERE release_id=$1`, releaseID))
}

// LatestRelease reduces with the validator's dotted comparator in Go, matching
// MemoryRepository's S4 monotonicity semantics exactly.
func (r *PostgresRepository) LatestRelease(ctx context.Context, os otaprotocol.OSType, targetModel string) (Release, error) {
	rows, err := r.pool.Query(ctx, releaseSelect+` WHERE os_type=$1 AND target_model=$2 ORDER BY seq`,
		string(os), targetModel)
	if err != nil {
		return Release{}, err
	}
	defer rows.Close()
	var latest Release
	found := false
	for rows.Next() {
		rel, serr := scanRelease(rows)
		if serr != nil {
			return Release{}, serr
		}
		if !found {
			latest, found = rel, true
			continue
		}
		if c, cerr := otavalidator.CompareDotted(rel.Version, latest.Version); cerr == nil && c > 0 {
			latest = rel
		}
	}
	if err := rows.Err(); err != nil {
		return Release{}, err
	}
	if !found {
		return Release{}, ErrNotFound
	}
	return latest, nil
}

// ListReleases applies the same insertion-order + offset-cursor paging as memory.
func (r *PostgresRepository) ListReleases(ctx context.Context, f ReleaseFilter) ([]Release, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := decodeCursor(f.Cursor)

	q := releaseSelect + ` WHERE ($1='' OR os_type=$1) AND ($2='' OR target_model=$2)
 AND ($3='' OR status=$3) ORDER BY seq OFFSET $4 LIMIT $5`
	rows, err := r.pool.Query(ctx, q, string(f.OSType), f.TargetModel, f.Status, start, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []Release
	for rows.Next() {
		rel, serr := scanRelease(rows)
		if serr != nil {
			return nil, "", serr
		}
		out = append(out, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = encodeCursor(start + limit)
	}
	return out, next, nil
}

// --- deployments ---

func (r *PostgresRepository) CreateDeployment(ctx context.Context, d Deployment) error {
	const q = `
INSERT INTO helix_ota.deployments
 (deployment_id, release_id, strategy, group_name, status, target_count, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (deployment_id) DO UPDATE SET
  release_id=EXCLUDED.release_id, strategy=EXCLUDED.strategy, group_name=EXCLUDED.group_name,
  status=EXCLUDED.status, target_count=EXCLUDED.target_count, created_at=EXCLUDED.created_at`
	_, err := r.pool.Exec(ctx, q, d.DeploymentID, d.ReleaseID, d.Strategy, d.Group, d.Status,
		d.TargetCount, d.CreatedAt)
	return err
}

const deploymentSelect = `
SELECT deployment_id, release_id, strategy, group_name, status, target_count, created_at
FROM helix_ota.deployments`

func scanDeployment(row pgx.Row) (Deployment, error) {
	var d Deployment
	if err := row.Scan(&d.DeploymentID, &d.ReleaseID, &d.Strategy, &d.Group, &d.Status,
		&d.TargetCount, &d.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Deployment{}, ErrNotFound
		}
		return Deployment{}, err
	}
	return d, nil
}

func (r *PostgresRepository) GetDeployment(ctx context.Context, deploymentID string) (Deployment, error) {
	return scanDeployment(r.pool.QueryRow(ctx, deploymentSelect+` WHERE deployment_id=$1`, deploymentID))
}

// ActiveDeploymentForTarget mirrors memory: an active deployment whose release
// targets os+target_model, with the same group-narrowing rule (skip only when
// both the deployment group and the query group are non-empty and differ).
func (r *PostgresRepository) ActiveDeploymentForTarget(ctx context.Context, os otaprotocol.OSType, targetModel, group string) (Deployment, error) {
	const q = `
SELECT d.deployment_id, d.release_id, d.strategy, d.group_name, d.status, d.target_count, d.created_at
FROM helix_ota.deployments d
JOIN helix_ota.releases r ON r.release_id = d.release_id
WHERE d.status = $1 AND r.os_type = $2 AND r.target_model = $3
  AND ($4 = '' OR d.group_name = '' OR d.group_name = $4)
ORDER BY d.seq
LIMIT 1`
	return scanDeployment(r.pool.QueryRow(ctx, q, string(otaprotocol.DeploymentActive),
		string(os), targetModel, group))
}

func (r *PostgresRepository) ListActiveDeployments(ctx context.Context) ([]Deployment, error) {
	rows, err := r.pool.Query(ctx, deploymentSelect+` WHERE status=$1 ORDER BY seq`,
		string(otaprotocol.DeploymentActive))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		d, serr := scanDeployment(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// --- telemetry ---

func (r *PostgresRepository) AppendTelemetry(ctx context.Context, rec TelemetryRecord) error {
	const q = `
INSERT INTO helix_ota.telemetry_events
 (device_id, deployment_id, event, version, error_code, detail, timestamp, received_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.pool.Exec(ctx, q, rec.DeviceID, rec.DeploymentID, string(rec.Event),
		rec.Version, rec.ErrorCode, rec.Detail, rec.Timestamp, rec.ReceivedAt)
	return err
}

func (r *PostgresRepository) TelemetryForDeployment(ctx context.Context, deploymentID string) ([]TelemetryRecord, error) {
	const q = `
SELECT device_id, deployment_id, event, version, error_code, detail, timestamp, received_at
FROM helix_ota.telemetry_events WHERE deployment_id=$1 ORDER BY seq`
	rows, err := r.pool.Query(ctx, q, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelemetryRecord
	for rows.Next() {
		var rec TelemetryRecord
		var event string
		if serr := rows.Scan(&rec.DeviceID, &rec.DeploymentID, &event, &rec.Version,
			&rec.ErrorCode, &rec.Detail, &rec.Timestamp, &rec.ReceivedAt); serr != nil {
			return nil, serr
		}
		rec.Event = otaprotocol.TelemetryEvent(event)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// TelemetryForDevice returns a device's event history in insertion order.
func (r *PostgresRepository) TelemetryForDevice(ctx context.Context, deviceID string) ([]TelemetryRecord, error) {
	const q = `
SELECT device_id, deployment_id, event, version, error_code, detail, timestamp, received_at
FROM helix_ota.telemetry_events WHERE device_id=$1 ORDER BY seq`
	rows, err := r.pool.Query(ctx, q, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelemetryRecord
	for rows.Next() {
		var rec TelemetryRecord
		var event string
		if serr := rows.Scan(&rec.DeviceID, &rec.DeploymentID, &event, &rec.Version,
			&rec.ErrorCode, &rec.Detail, &rec.Timestamp, &rec.ReceivedAt); serr != nil {
			return nil, serr
		}
		rec.Event = otaprotocol.TelemetryEvent(event)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// TelemetryEventCounts returns fleet-wide counts keyed by event type.
func (r *PostgresRepository) TelemetryEventCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT event, COUNT(*) FROM helix_ota.telemetry_events GROUP BY event`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int64)
	for rows.Next() {
		var event string
		var n int64
		if serr := rows.Scan(&event, &n); serr != nil {
			return nil, serr
		}
		counts[event] = n
	}
	return counts, rows.Err()
}

// --- device groups ---

func (r *PostgresRepository) CreateGroup(ctx context.Context, g Group) error {
	const q = `
INSERT INTO helix_ota.device_groups (group_id, name, description, created_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (group_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description`
	_, err := r.pool.Exec(ctx, q, g.ID, g.Name, g.Description, g.CreatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *PostgresRepository) GetGroup(ctx context.Context, groupID string) (Group, error) {
	var g Group
	err := r.pool.QueryRow(ctx,
		`SELECT group_id, name, description, created_at FROM helix_ota.device_groups WHERE group_id=$1`, groupID).
		Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

func (r *PostgresRepository) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT group_id, name, description, created_at FROM helix_ota.device_groups ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if serr := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); serr != nil {
			return nil, serr
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) UpdateGroup(ctx context.Context, g Group) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE helix_ota.device_groups SET name=$2, description=$3 WHERE group_id=$1`,
		g.ID, g.Name, g.Description)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteGroup(ctx context.Context, groupID string) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM helix_ota.device_groups WHERE group_id=$1`, groupID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) AddGroupMember(ctx context.Context, groupID, deviceID string) error {
	if _, err := r.GetGroup(ctx, groupID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO helix_ota.device_group_members (group_id, device_id) VALUES ($1,$2)
		 ON CONFLICT (group_id, device_id) DO NOTHING`, groupID, deviceID)
	return err
}

func (r *PostgresRepository) ListGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	if _, err := r.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT device_id FROM helix_ota.device_group_members WHERE group_id=$1 ORDER BY seq`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if serr := rows.Scan(&id); serr != nil {
			return nil, serr
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) RemoveGroupMember(ctx context.Context, groupID, deviceID string) error {
	if _, err := r.GetGroup(ctx, groupID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`DELETE FROM helix_ota.device_group_members WHERE group_id=$1 AND device_id=$2`, groupID, deviceID)
	return err
}

// --- audit ---

func (r *PostgresRepository) AppendAudit(ctx context.Context, e AuditEntry) error {
	details, err := jsonbOf(orEmptyMap(e.Details))
	if err != nil {
		return err
	}
	const q = `
INSERT INTO helix_ota.audit_logs
 (audit_id, user_id, actor_subject, action, resource_type, resource_id, details,
  ip_address, user_agent, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err = r.pool.Exec(ctx, q, e.ID, e.UserID, e.ActorSubject, e.Action, e.ResourceType,
		e.ResourceID, details, e.IPAddress, e.UserAgent, e.CreatedAt)
	return err
}

func (r *PostgresRepository) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := decodeCursor(f.Cursor)
	const q = `
SELECT audit_id, user_id, actor_subject, action, resource_type, resource_id, details,
       ip_address, user_agent, created_at
FROM helix_ota.audit_logs
WHERE ($1='' OR action=$1) AND ($2='' OR resource_type=$2)
ORDER BY seq OFFSET $3 LIMIT $4`
	rows, err := r.pool.Query(ctx, q, f.Action, f.ResourceType, start, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var details []byte
		if serr := rows.Scan(&e.ID, &e.UserID, &e.ActorSubject, &e.Action, &e.ResourceType,
			&e.ResourceID, &details, &e.IPAddress, &e.UserAgent, &e.CreatedAt); serr != nil {
			return nil, "", serr
		}
		_ = json.Unmarshal(details, &e.Details)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = encodeCursor(start + limit)
	}
	return out, next, nil
}

// --- idempotency ---

func (r *PostgresRepository) GetIdempotent(ctx context.Context, key string) (string, bool) {
	var resultID string
	err := r.pool.QueryRow(ctx, `SELECT result_id FROM helix_ota.idempotency_keys WHERE key=$1`, key).Scan(&resultID)
	if err != nil {
		return "", false
	}
	return resultID, true
}

func (r *PostgresRepository) PutIdempotent(ctx context.Context, key, resultID string) {
	_, _ = r.pool.Exec(ctx,
		`INSERT INTO helix_ota.idempotency_keys (key, result_id) VALUES ($1,$2)
		 ON CONFLICT (key) DO NOTHING`, key, resultID)
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// nullTime maps a zero time.Time to SQL NULL so an unset last_seen is stored as
// NULL (and round-trips back to the zero value).
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
