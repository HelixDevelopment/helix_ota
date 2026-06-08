package store

import (
	"context"
	"sync"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// MemoryRepository is an in-memory, concurrency-safe Repository implementation.
// It backs the MVP skeleton and the integration tests so the api/validation
// seams are exercisable without PostgreSQL. The production target is a
// pgx-backed implementation of the same interface (architecture.md §4).
type MemoryRepository struct {
	mu sync.RWMutex

	devices     map[string]Device     // by deviceID
	devByHW     map[string]string     // hardwareID -> deviceID
	artifacts   map[string]Artifact   // by artifactID
	releases    map[string]Release    // by releaseID
	relOrder    []string              // insertion order for stable listing
	deployments map[string]Deployment // by deploymentID
	telemetry   []TelemetryRecord     // append-only event log
	audit       []AuditEntry          // append-only admin/operator action log
	rollbacks   []RollbackRecord      // append-only rollback/abort log
	deltas      []DeltaArtifact       // base->target delta artifacts
	groups      map[string]Group      // by groupID
	grpOrder    []string              // insertion order for stable listing
	grpByName   map[string]string     // name -> groupID (uniqueness)
	members     map[string][]string   // groupID -> ordered device ids
	idem        map[string]string     // Idempotency-Key -> resultID
}

// NewMemoryRepository constructs an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		devices:     make(map[string]Device),
		devByHW:     make(map[string]string),
		artifacts:   make(map[string]Artifact),
		releases:    make(map[string]Release),
		deployments: make(map[string]Deployment),
		groups:      make(map[string]Group),
		grpByName:   make(map[string]string),
		members:     make(map[string][]string),
		idem:        make(map[string]string),
	}
}

// compile-time assertion that MemoryRepository satisfies Repository.
var _ Repository = (*MemoryRepository)(nil)

// CreateDevice stores a new device, rejecting a duplicate hardware_id bound to a
// different identity (endpoints.md §8.1 409 CONFLICT).
func (m *MemoryRepository) CreateDevice(_ context.Context, d Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existingID, ok := m.devByHW[d.HardwareID]; ok && existingID != d.DeviceID {
		return ErrConflict
	}
	m.devices[d.DeviceID] = d
	m.devByHW[d.HardwareID] = d.DeviceID
	return nil
}

// GetDevice returns a device by id.
func (m *MemoryRepository) GetDevice(_ context.Context, deviceID string) (Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[deviceID]
	if !ok {
		return Device{}, ErrNotFound
	}
	return d, nil
}

// GetDeviceByHardwareID returns a device by its hardware id.
func (m *MemoryRepository) GetDeviceByHardwareID(_ context.Context, hardwareID string) (Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.devByHW[hardwareID]
	if !ok {
		return Device{}, ErrNotFound
	}
	return m.devices[id], nil
}

// UpdateDevice overwrites an existing device record.
func (m *MemoryRepository) UpdateDevice(_ context.Context, d Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.devices[d.DeviceID]; !ok {
		return ErrNotFound
	}
	m.devices[d.DeviceID] = d
	return nil
}

// CreateArtifact stores a verified artifact record.
func (m *MemoryRepository) CreateArtifact(_ context.Context, a Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.artifacts[a.ArtifactID] = a
	return nil
}

// GetArtifact returns an artifact by id.
func (m *MemoryRepository) GetArtifact(_ context.Context, artifactID string) (Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.artifacts[artifactID]
	if !ok {
		return Artifact{}, ErrNotFound
	}
	return a, nil
}

// CreateRelease stores a release in insertion order.
func (m *MemoryRepository) CreateRelease(_ context.Context, r Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.releases[r.ReleaseID]; !exists {
		m.relOrder = append(m.relOrder, r.ReleaseID)
	}
	m.releases[r.ReleaseID] = r
	return nil
}

// GetRelease returns a release by id.
func (m *MemoryRepository) GetRelease(_ context.Context, releaseID string) (Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.releases[releaseID]
	if !ok {
		return Release{}, ErrNotFound
	}
	return r, nil
}

// LatestRelease returns the highest-versioned published release for the given
// os+target_model, or ErrNotFound when none exists. "Highest" uses the
// validator's dotted-numeric comparator so monotonicity matches S4.
func (m *MemoryRepository) LatestRelease(_ context.Context, os otaprotocol.OSType, targetModel string) (Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest Release
	found := false
	for _, id := range m.relOrder {
		r := m.releases[id]
		if r.OSType != os || r.TargetModel != targetModel {
			continue
		}
		if !found {
			latest, found = r, true
			continue
		}
		if c, err := otavalidator.CompareDotted(r.Version, latest.Version); err == nil && c > 0 {
			latest = r
		}
	}
	if !found {
		return Release{}, ErrNotFound
	}
	return latest, nil
}

// ListReleases returns releases matching the filter, in insertion order, with a
// simple offset cursor. The next cursor is empty when the page is the last.
func (m *MemoryRepository) ListReleases(_ context.Context, f ReleaseFilter) ([]Release, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := decodeCursor(f.Cursor)

	var matched []Release
	for _, id := range m.relOrder {
		r := m.releases[id]
		if f.OSType != "" && r.OSType != f.OSType {
			continue
		}
		if f.TargetModel != "" && r.TargetModel != f.TargetModel {
			continue
		}
		if f.Status != "" && r.Status != f.Status {
			continue
		}
		matched = append(matched, r)
	}

	if start > len(matched) {
		start = len(matched)
	}
	end := start + limit
	next := ""
	if end < len(matched) {
		next = encodeCursor(end)
	} else {
		end = len(matched)
	}
	return matched[start:end], next, nil
}

// CreateDeployment stores a deployment.
func (m *MemoryRepository) CreateDeployment(_ context.Context, d Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deployments[d.DeploymentID] = d
	return nil
}

// UpdateDeployment overwrites an existing deployment (e.g. to supersede it on a
// recall). Returns ErrNotFound if the deployment is unknown.
func (m *MemoryRepository) UpdateDeployment(_ context.Context, d Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.deployments[d.DeploymentID]; !ok {
		return ErrNotFound
	}
	m.deployments[d.DeploymentID] = d
	return nil
}

// GetDeployment returns a deployment by id.
func (m *MemoryRepository) GetDeployment(_ context.Context, deploymentID string) (Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.deployments[deploymentID]
	if !ok {
		return Deployment{}, ErrNotFound
	}
	return d, nil
}

// ActiveDeploymentForTarget returns an active deployment whose release targets
// the given os+target_model (optionally narrowed to a group), or ErrNotFound.
func (m *MemoryRepository) ActiveDeploymentForTarget(ctx context.Context, os otaprotocol.OSType, targetModel, group string) (Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, dep := range m.deployments {
		if dep.Status != string(otaprotocol.DeploymentActive) {
			continue
		}
		rel, ok := m.releases[dep.ReleaseID]
		if !ok {
			continue
		}
		if rel.OSType != os || rel.TargetModel != targetModel {
			continue
		}
		if group != "" && dep.Group != "" && dep.Group != group {
			continue
		}
		return dep, nil
	}
	return Deployment{}, ErrNotFound
}

// ListActiveDeployments returns all active deployments.
func (m *MemoryRepository) ListActiveDeployments(_ context.Context) ([]Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Deployment
	for _, dep := range m.deployments {
		if dep.Status == string(otaprotocol.DeploymentActive) {
			out = append(out, dep)
		}
	}
	return out, nil
}

// AppendTelemetry appends an event to the log.
func (m *MemoryRepository) AppendTelemetry(_ context.Context, rec TelemetryRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.telemetry = append(m.telemetry, rec)
	return nil
}

// TelemetryForDeployment returns all events for a deployment id.
func (m *MemoryRepository) TelemetryForDeployment(_ context.Context, deploymentID string) ([]TelemetryRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []TelemetryRecord
	for _, rec := range m.telemetry {
		if rec.DeploymentID == deploymentID {
			out = append(out, rec)
		}
	}
	return out, nil
}

// AllDevices returns a snapshot of every registered device. It backs the
// all-targets matching in the api layer (deviceLister capability); the pgx
// implementation would replace this scan with an indexed query.
func (m *MemoryRepository) AllDevices(_ context.Context) []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, d)
	}
	return out
}

// GetIdempotent returns a stored result id for an Idempotency-Key.
// TelemetryForDevice returns a device's event history in insertion order.
func (m *MemoryRepository) TelemetryForDevice(_ context.Context, deviceID string) ([]TelemetryRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []TelemetryRecord
	for _, rec := range m.telemetry {
		if rec.DeviceID == deviceID {
			out = append(out, rec)
		}
	}
	return out, nil
}

// TelemetryEventCounts returns fleet-wide counts keyed by event type.
func (m *MemoryRepository) TelemetryEventCounts(_ context.Context) (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := make(map[string]int64)
	for _, rec := range m.telemetry {
		counts[string(rec.Event)]++
	}
	return counts, nil
}

// --- device groups ---

func (m *MemoryRepository) CreateGroup(_ context.Context, g Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.grpByName[g.Name]; ok && existing != g.ID {
		return ErrConflict
	}
	if _, exists := m.groups[g.ID]; !exists {
		m.grpOrder = append(m.grpOrder, g.ID)
	}
	m.groups[g.ID] = g
	m.grpByName[g.Name] = g.ID
	return nil
}

func (m *MemoryRepository) GetGroup(_ context.Context, groupID string) (Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.groups[groupID]
	if !ok {
		return Group{}, ErrNotFound
	}
	return g, nil
}

func (m *MemoryRepository) ListGroups(_ context.Context) ([]Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Group, 0, len(m.grpOrder))
	for _, id := range m.grpOrder {
		out = append(out, m.groups[id])
	}
	return out, nil
}

func (m *MemoryRepository) UpdateGroup(_ context.Context, g Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.groups[g.ID]
	if !ok {
		return ErrNotFound
	}
	if other, taken := m.grpByName[g.Name]; taken && other != g.ID {
		return ErrConflict
	}
	if old.Name != g.Name {
		delete(m.grpByName, old.Name)
		m.grpByName[g.Name] = g.ID
	}
	m.groups[g.ID] = g
	return nil
}

func (m *MemoryRepository) DeleteGroup(_ context.Context, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return ErrNotFound
	}
	delete(m.groups, groupID)
	delete(m.grpByName, g.Name)
	delete(m.members, groupID)
	for i, id := range m.grpOrder {
		if id == groupID {
			m.grpOrder = append(m.grpOrder[:i], m.grpOrder[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MemoryRepository) AddGroupMember(_ context.Context, groupID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[groupID]; !ok {
		return ErrNotFound
	}
	for _, id := range m.members[groupID] {
		if id == deviceID {
			return nil // idempotent
		}
	}
	m.members[groupID] = append(m.members[groupID], deviceID)
	return nil
}

func (m *MemoryRepository) ListGroupMembers(_ context.Context, groupID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.groups[groupID]; !ok {
		return nil, ErrNotFound
	}
	return append([]string(nil), m.members[groupID]...), nil
}

func (m *MemoryRepository) RemoveGroupMember(_ context.Context, groupID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[groupID]; !ok {
		return ErrNotFound
	}
	for i, id := range m.members[groupID] {
		if id == deviceID {
			m.members[groupID] = append(m.members[groupID][:i], m.members[groupID][i+1:]...)
			break
		}
	}
	return nil // idempotent
}

// CreateDelta stores a base->target delta artifact, rejecting a duplicate
// (base,target) pair with ErrConflict.
func (m *MemoryRepository) CreateDelta(_ context.Context, d DeltaArtifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.deltas {
		if existing.BaseArtifactID == d.BaseArtifactID && existing.TargetArtifactID == d.TargetArtifactID {
			return ErrConflict
		}
	}
	m.deltas = append(m.deltas, d)
	return nil
}

// FindDelta returns the delta from baseArtifactID to targetArtifactID, or ErrNotFound.
func (m *MemoryRepository) FindDelta(_ context.Context, baseArtifactID, targetArtifactID string) (DeltaArtifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.deltas {
		if d.BaseArtifactID == baseArtifactID && d.TargetArtifactID == targetArtifactID {
			return d, nil
		}
	}
	return DeltaArtifact{}, ErrNotFound
}

// AppendRollback appends a rollback/abort record (append-only).
func (m *MemoryRepository) AppendRollback(_ context.Context, r RollbackRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollbacks = append(m.rollbacks, r)
	return nil
}

// ListRollbacks returns the rollback records for a deployment in insertion order.
func (m *MemoryRepository) ListRollbacks(_ context.Context, deploymentID string) ([]RollbackRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []RollbackRecord
	for _, r := range m.rollbacks {
		if r.DeploymentID == deploymentID {
			out = append(out, r)
		}
	}
	return out, nil
}

// AppendAudit appends an admin/operator action to the audit log.
func (m *MemoryRepository) AppendAudit(_ context.Context, e AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = append(m.audit, e)
	return nil
}

// ListAudit returns audit entries matching the filter in insertion order, with
// the same offset-cursor paging as ListReleases.
func (m *MemoryRepository) ListAudit(_ context.Context, f AuditFilter) ([]AuditEntry, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := decodeCursor(f.Cursor)
	var matched []AuditEntry
	for _, e := range m.audit {
		if f.Action != "" && e.Action != f.Action {
			continue
		}
		if f.ResourceType != "" && e.ResourceType != f.ResourceType {
			continue
		}
		matched = append(matched, e)
	}
	if start > len(matched) {
		start = len(matched)
	}
	end := start + limit
	next := ""
	if end < len(matched) {
		next = encodeCursor(end)
	} else {
		end = len(matched)
	}
	return matched[start:end], next, nil
}

func (m *MemoryRepository) GetIdempotent(_ context.Context, key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.idem[key]
	return v, ok
}

// PutIdempotent records an Idempotency-Key -> result id mapping.
// PutIdempotent records the result for an Idempotency-Key. It is first-write-wins:
// a replayed request must return the ORIGINAL result, so an existing key is never
// overwritten (matches the pgx repository's ON CONFLICT DO NOTHING).
func (m *MemoryRepository) PutIdempotent(_ context.Context, key, resultID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.idem[key]; !exists {
		m.idem[key] = resultID
	}
}
