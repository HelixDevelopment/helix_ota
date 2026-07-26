package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// MemoryRepository is an in-memory, concurrency-safe Repository implementation.
// It backs the MVP skeleton and the integration tests so the api/validation
// seams are exercisable without PostgreSQL. The production target is a
// pgx-backed implementation of the same interface (architecture.md §4).
type MemoryRepository struct {
	mu sync.RWMutex

	devices     map[string]Device        // by deviceID
	devOrder    []string                 // insertion order for stable listing (ListDevices/AllDevices)
	devByHW     map[string]string        // hardwareID -> deviceID
	artifacts   map[string]Artifact      // by artifactID
	releases    map[string]Release       // by releaseID
	relOrder    []string                 // insertion order for stable listing
	deployments map[string]Deployment    // by deploymentID
	depOrder    []string                 // insertion order for stable matching (ActiveDeploymentForTarget/ListActiveDeployments)
	telemetry   []TelemetryRecord        // append-only event log
	audit       []AuditEntry             // append-only admin/operator action log
	rollbacks   []RollbackRecord         // append-only rollback/abort log
	deltas      []DeltaArtifact          // base->target delta artifacts
	groups      map[string]Group         // by groupID
	grpOrder    []string                 // insertion order for stable listing
	grpByName   map[string]string        // name -> groupID (uniqueness)
	members     map[string][]GroupMember // groupID -> ordered members (with join time)
	idem        map[string]string        // Idempotency-Key -> resultID

	// Projects (multi-project support).
	projects  map[string]Project // by projectID
	prjOrder  []string           // insertion order for stable listing
	prjByName map[string]string  // name -> projectID (uniqueness)
	// Project access control (callerID -> projectID -> role).
	prjAccess map[string]map[string]ProjectRole // callerID -> projectID -> role

	// Accounts (tenant layer above Project — Accounts M1).
	accounts   map[string]Account                      // by accountID
	accOrder   []string                                // insertion order for stable listing
	accByName  map[string]string                       // name -> accountID (uniqueness)
	accBySlug  map[string]string                       // slug -> accountID (uniqueness)
	accMembers map[string]map[string]AccountMembership // userID -> accountID -> membership

	// Emulation test-fabric registry (docs/design/emulation_fabric/SCHEMA.sql).
	fabNodes    map[string]FabricNode       // by nodeID
	fabTargets  map[string]FabricTarget     // by targetID
	fabTgtOrder []string                    // insertion order for stable listing
	fabLeases   map[string]FabricLease      // by leaseID
	fabRuns     map[string]FabricRun        // by runID
	fabEvidence map[string][]FabricEvidence // runID -> ordered evidence
	branches    map[string]Branch           // by branchID
	brnOrder    []string                    // insertion order

	webhooks map[string]Webhook // by webhookID
}

// NewMemoryRepository constructs an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		devices:     make(map[string]Device),
		devByHW:     make(map[string]string),
		artifacts:   make(map[string]Artifact),
		releases:    make(map[string]Release),
		deployments: make(map[string]Deployment),
		projects:    make(map[string]Project),
		prjByName:   make(map[string]string),
		prjAccess:   make(map[string]map[string]ProjectRole),
		accounts:    make(map[string]Account),
		accByName:   make(map[string]string),
		accBySlug:   make(map[string]string),
		accMembers:  make(map[string]map[string]AccountMembership),
		groups:      make(map[string]Group),
		grpByName:   make(map[string]string),
		members:     make(map[string][]GroupMember),
		idem:        make(map[string]string),
		fabNodes:    make(map[string]FabricNode),
		fabTargets:  make(map[string]FabricTarget),
		fabLeases:   make(map[string]FabricLease),
		fabRuns:     make(map[string]FabricRun),
		fabEvidence: make(map[string][]FabricEvidence),
		branches:    make(map[string]Branch),
		webhooks:    make(map[string]Webhook),
	}
}

// compile-time assertion that MemoryRepository satisfies Repository.
var _ Repository = (*MemoryRepository)(nil)

// CreateDevice stores a new device, rejecting a duplicate hardware_id bound to a
// different identity (endpoints.md §8.1 409 CONFLICT). The caller's Metadata map
// is cloned before storage so a later mutation of the caller's own struct cannot
// silently corrupt the stored record (the write-side half of the isolation
// guarantee memory_fabric.go's cloneStrMap already enforces for FabricNode.Labels).
func (m *MemoryRepository) CreateDevice(_ context.Context, d Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existingID, ok := m.devByHW[d.HardwareID]; ok && existingID != d.DeviceID {
		return ErrConflict
	}
	if _, exists := m.devices[d.DeviceID]; !exists {
		m.devOrder = append(m.devOrder, d.DeviceID)
	}
	d.Metadata = cloneStrMap(d.Metadata)
	m.devices[d.DeviceID] = d
	m.devByHW[d.HardwareID] = d.DeviceID
	return nil
}

// GetDevice returns a device by id. The returned Metadata map is a clone so the
// caller cannot mutate the stored record through the returned value (read-side
// isolation, symmetric with the clone CreateDevice/UpdateDevice apply on write).
func (m *MemoryRepository) GetDevice(_ context.Context, deviceID string) (Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[deviceID]
	if !ok {
		return Device{}, ErrNotFound
	}
	d.Metadata = cloneStrMap(d.Metadata)
	return d, nil
}

// GetDeviceByHardwareID returns a device by its hardware id (cloned Metadata;
// see GetDevice).
func (m *MemoryRepository) GetDeviceByHardwareID(_ context.Context, hardwareID string) (Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.devByHW[hardwareID]
	if !ok {
		return Device{}, ErrNotFound
	}
	d := m.devices[id]
	d.Metadata = cloneStrMap(d.Metadata)
	return d, nil
}

// UpdateDevice overwrites an existing device record. When HardwareID changes,
// the devByHW secondary index is re-pointed to the new value and the stale old
// mapping is removed (mirroring the rename handling UpdateGroup/UpdateProject
// already apply to their own name indexes) — otherwise GetDeviceByHardwareID
// would keep resolving the OLD hardware id to this device while the NEW
// hardware id resolved to nothing. A HardwareID collision with a different
// device is rejected with ErrConflict, matching CreateDevice's own invariant.
func (m *MemoryRepository) UpdateDevice(_ context.Context, d Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.devices[d.DeviceID]
	if !ok {
		return ErrNotFound
	}
	if old.HardwareID != d.HardwareID {
		if existingID, taken := m.devByHW[d.HardwareID]; taken && existingID != d.DeviceID {
			return ErrConflict
		}
		delete(m.devByHW, old.HardwareID)
		m.devByHW[d.HardwareID] = d.DeviceID
	}
	d.Metadata = cloneStrMap(d.Metadata)
	m.devices[d.DeviceID] = d
	return nil
}

// ListDevices returns devices matching the filter, with cursor pagination.
// Iterates devOrder (not the devices map directly) so the match set has a
// stable order across calls — Go map iteration order is randomized per range,
// so ranging over m.devices directly here made offset-cursor pagination
// silently duplicate and drop devices across pages (proven in
// memory_devices_order_test.go).
func (m *MemoryRepository) ListDevices(_ context.Context, f DeviceFilter) ([]Device, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := decodeCursor(f.Cursor)

	var matched []Device
	for _, id := range m.devOrder {
		d := m.devices[id]
		if f.OSType != "" && d.OSType != f.OSType {
			continue
		}
		if f.TargetModel != "" && d.Model != f.TargetModel {
			continue
		}
		if f.Status != "" && d.UpdateState != f.Status {
			continue
		}
		d.Metadata = cloneStrMap(d.Metadata)
		matched = append(matched, d)
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

// ReleaseByVersion returns the release for an exact os+target+version, or
// ErrNotFound. Used to resolve a device's current version to its base artifact
// for delta selection.
func (m *MemoryRepository) ReleaseByVersion(_ context.Context, os otaprotocol.OSType, targetModel, version string) (Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.relOrder {
		r := m.releases[id]
		if r.OSType == os && r.TargetModel == targetModel && r.Version == version {
			return r, nil
		}
	}
	return Release{}, ErrNotFound
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

// CreateDeployment stores a deployment in insertion order. depOrder is
// appended only for a genuinely new id (mirroring devOrder/relOrder in
// CreateDevice/CreateRelease) so ActiveDeploymentForTarget/
// ListActiveDeployments can iterate deterministically instead of ranging the
// raw map (whose iteration order Go randomizes per range).
func (m *MemoryRepository) CreateDeployment(_ context.Context, d Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.deployments[d.DeploymentID]; !exists {
		m.depOrder = append(m.depOrder, d.DeploymentID)
	}
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
// Iterates depOrder (not the deployments map directly) so that when two
// active deployments match the same target, the result is deterministic
// (the earliest-created match) across repeated calls — ranging m.deployments
// directly here made the returned deployment depend on Go's per-range
// randomized map iteration order (see the comment citing
// memory_devices_order_test.go on ListDevices, and
// memory_deployments_order_test.go for this method).
func (m *MemoryRepository) ActiveDeploymentForTarget(ctx context.Context, os otaprotocol.OSType, targetModel, group string) (Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.depOrder {
		dep, ok := m.deployments[id]
		if !ok {
			continue
		}
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

// ListActiveDeployments returns all active deployments in stable (insertion)
// order, iterating depOrder for the same determinism reason as
// ActiveDeploymentForTarget.
func (m *MemoryRepository) ListActiveDeployments(_ context.Context) ([]Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Deployment
	for _, id := range m.depOrder {
		dep, ok := m.deployments[id]
		if !ok {
			continue
		}
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

// AllDevices returns a snapshot of every registered device, in stable
// (insertion) order, each with a cloned Metadata map (see GetDevice). It backs
// the all-targets matching in the api layer (deviceLister capability); the pgx
// implementation would replace this scan with an indexed query.
func (m *MemoryRepository) AllDevices(_ context.Context) []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Device, 0, len(m.devOrder))
	for _, id := range m.devOrder {
		d := m.devices[id]
		d.Metadata = cloneStrMap(d.Metadata)
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

func (m *MemoryRepository) AddGroupMember(_ context.Context, groupID, deviceID string, addedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[groupID]; !ok {
		return ErrNotFound
	}
	for _, mem := range m.members[groupID] {
		if mem.DeviceID == deviceID {
			return nil // idempotent
		}
	}
	m.members[groupID] = append(m.members[groupID], GroupMember{DeviceID: deviceID, AddedAt: addedAt})
	return nil
}

func (m *MemoryRepository) ListGroupMembers(_ context.Context, groupID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.groups[groupID]; !ok {
		return nil, ErrNotFound
	}
	out := make([]string, 0, len(m.members[groupID]))
	for _, mem := range m.members[groupID] {
		out = append(out, mem.DeviceID)
	}
	return out, nil
}

func (m *MemoryRepository) ListGroupMembersDetailed(_ context.Context, groupID string) ([]GroupMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.groups[groupID]; !ok {
		return nil, ErrNotFound
	}
	return append([]GroupMember(nil), m.members[groupID]...), nil
}

func (m *MemoryRepository) RemoveGroupMember(_ context.Context, groupID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[groupID]; !ok {
		return ErrNotFound
	}
	for i, mem := range m.members[groupID] {
		if mem.DeviceID == deviceID {
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

// DeviceStateCounts returns the fleet device count keyed by last-known update
// state (empty/never-reported bucketed as "unknown"), for /telemetry/overview.
func (m *MemoryRepository) DeviceStateCounts(_ context.Context) (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int64)
	for _, d := range m.devices {
		state := d.UpdateState
		if state == "" {
			state = "unknown"
		}
		out[state]++
	}
	return out, nil
}

// AppendRollback appends a rollback/abort record (append-only). The Details
// map is cloned before storage so a later mutation of the caller's struct
// cannot reach back into the stored record (write-side isolation, symmetric
// with ListRollbacks' clone on read).
func (m *MemoryRepository) AppendRollback(_ context.Context, r RollbackRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.Details = cloneStrMap(r.Details)
	m.rollbacks = append(m.rollbacks, r)
	return nil
}

// ListRollbacks returns the rollback records for a deployment in insertion
// order, each with a cloned Details map so the caller cannot mutate the stored
// record through the returned value.
func (m *MemoryRepository) ListRollbacks(_ context.Context, deploymentID string) ([]RollbackRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []RollbackRecord
	for _, r := range m.rollbacks {
		if r.DeploymentID == deploymentID {
			r.Details = cloneStrMap(r.Details)
			out = append(out, r)
		}
	}
	return out, nil
}

// AppendAudit appends an admin/operator action to the audit log. The Details
// map is cloned before storage (see AppendRollback).
func (m *MemoryRepository) AppendAudit(_ context.Context, e AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.Details = cloneStrMap(e.Details)
	m.audit = append(m.audit, e)
	return nil
}

// ListAudit returns audit entries matching the filter in insertion order, with
// the same offset-cursor paging as ListReleases. Each entry's Details map is
// cloned so the caller cannot mutate the stored record through the returned
// value (see GetDevice).
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
		if !f.Since.IsZero() && e.CreatedAt.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.CreatedAt.After(f.Until) {
			continue
		}
		e.Details = cloneStrMap(e.Details)
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

// --- projects ---

func (m *MemoryRepository) CreateProject(_ context.Context, p Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.prjByName[p.Name]; ok && existing != p.ProjectID {
		return ErrConflict
	}
	if _, exists := m.projects[p.ProjectID]; !exists {
		m.prjOrder = append(m.prjOrder, p.ProjectID)
	}
	m.projects[p.ProjectID] = p
	m.prjByName[p.Name] = p.ProjectID
	return nil
}

func (m *MemoryRepository) GetProject(_ context.Context, projectID string) (Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[projectID]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

func (m *MemoryRepository) ListProjects(_ context.Context) ([]Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Project, 0, len(m.prjOrder))
	for _, id := range m.prjOrder {
		out = append(out, m.projects[id])
	}
	return out, nil
}

func (m *MemoryRepository) UpdateProject(_ context.Context, p Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.projects[p.ProjectID]
	if !ok {
		return ErrNotFound
	}
	if other, taken := m.prjByName[p.Name]; taken && other != p.ProjectID {
		return ErrConflict
	}
	if old.Name != p.Name {
		delete(m.prjByName, old.Name)
		m.prjByName[p.Name] = p.ProjectID
	}
	m.projects[p.ProjectID] = p
	return nil
}

func (m *MemoryRepository) DeleteProject(_ context.Context, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[projectID]
	if !ok {
		return ErrNotFound
	}
	delete(m.projects, projectID)
	delete(m.prjByName, p.Name)
	for i, id := range m.prjOrder {
		if id == projectID {
			m.prjOrder = append(m.prjOrder[:i], m.prjOrder[i+1:]...)
			break
		}
	}
	return nil
}

// GetProjectAccess returns the caller's role within a project, or ErrNotFound.
func (m *MemoryRepository) GetProjectAccess(_ context.Context, callerID, projectID string) (ProjectAccess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.projects[projectID]; !ok {
		return ProjectAccess{}, ErrNotFound
	}
	byCaller, ok := m.prjAccess[callerID]
	if !ok {
		return ProjectAccess{}, ErrNotFound
	}
	role, ok := byCaller[projectID]
	if !ok {
		return ProjectAccess{}, ErrNotFound
	}
	return ProjectAccess{ProjectID: projectID, CallerID: callerID, Role: role}, nil
}

// SetProjectAccess grants or updates a caller's role within a project.
func (m *MemoryRepository) SetProjectAccess(_ context.Context, access ProjectAccess) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[access.ProjectID]; !ok {
		return ErrNotFound
	}
	if m.prjAccess[access.CallerID] == nil {
		m.prjAccess[access.CallerID] = make(map[string]ProjectRole)
	}
	m.prjAccess[access.CallerID][access.ProjectID] = access.Role
	return nil
}

// ListProjectMembers returns all callers with access to a project.
func (m *MemoryRepository) ListProjectMembers(_ context.Context, projectID string) ([]ProjectAccess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.projects[projectID]; !ok {
		return nil, ErrNotFound
	}
	var result []ProjectAccess
	for callerID, byCaller := range m.prjAccess {
		if role, ok := byCaller[projectID]; ok {
			result = append(result, ProjectAccess{ProjectID: projectID, CallerID: callerID, Role: role})
		}
	}
	return result, nil
}

// RemoveProjectAccess revokes a caller's access to a project.
func (m *MemoryRepository) RemoveProjectAccess(_ context.Context, callerID, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[projectID]; !ok {
		return ErrNotFound
	}
	byCaller, ok := m.prjAccess[callerID]
	if !ok {
		return ErrNotFound
	}
	if _, ok := byCaller[projectID]; !ok {
		return ErrNotFound
	}
	delete(byCaller, projectID)
	return nil
}

// --- accounts (tenant layer above Project — Accounts M1) ---

// CreateAccount stores a new account, rejecting a duplicate name OR slug bound
// to a different account id with ErrConflict (both are UNIQUE, mirroring the pgx
// constraints).
func (m *MemoryRepository) CreateAccount(_ context.Context, a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createAccountLocked(a)
}

// createAccountLocked assumes m.mu is already held. It is shared by CreateAccount
// and CreateAccountWithOwner so the owner-membership grant happens under the SAME
// lock as the account insert — the atomic no-orphan-tenant guarantee (design §2.3).
func (m *MemoryRepository) createAccountLocked(a Account) error {
	if existing, ok := m.accByName[a.Name]; ok && existing != a.AccountID {
		return ErrConflict
	}
	if existing, ok := m.accBySlug[a.Slug]; ok && existing != a.AccountID {
		return ErrConflict
	}
	if _, exists := m.accounts[a.AccountID]; !exists {
		m.accOrder = append(m.accOrder, a.AccountID)
	}
	m.accounts[a.AccountID] = a
	m.accByName[a.Name] = a.AccountID
	m.accBySlug[a.Slug] = a.AccountID
	return nil
}

// GetAccount returns an account by id, or ErrNotFound.
func (m *MemoryRepository) GetAccount(_ context.Context, accountID string) (Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.accounts[accountID]
	if !ok {
		return Account{}, ErrNotFound
	}
	return a, nil
}

// ListAccounts returns all accounts in insertion order.
func (m *MemoryRepository) ListAccounts(_ context.Context) ([]Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Account, 0, len(m.accOrder))
	for _, id := range m.accOrder {
		out = append(out, m.accounts[id])
	}
	return out, nil
}

// CreateAccountWithOwner atomically creates the account AND its owner membership
// under one lock, so a partial write can never leave an un-administerable orphan
// tenant (design §2.3). A duplicate name/slug fails BEFORE any membership is
// written; an empty owner id is rejected.
func (m *MemoryRepository) CreateAccountWithOwner(_ context.Context, a Account, ownerUserID string, role AccountRole) error {
	if ownerUserID == "" {
		return fmt.Errorf("store: CreateAccountWithOwner requires a non-empty owner user id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.createAccountLocked(a); err != nil {
		return err
	}
	if m.accMembers[ownerUserID] == nil {
		m.accMembers[ownerUserID] = make(map[string]AccountMembership)
	}
	m.accMembers[ownerUserID][a.AccountID] = AccountMembership{
		UserID: ownerUserID, AccountID: a.AccountID, Role: role,
		IsOwner: true, GrantedAt: a.CreatedAt, GrantedBy: "",
	}
	return nil
}

// GetAccountMembership returns the user's membership in an account, or
// ErrNotFound when the user is not a member (the cross-tenant deny).
func (m *MemoryRepository) GetAccountMembership(_ context.Context, userID, accountID string) (AccountMembership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	byUser, ok := m.accMembers[userID]
	if !ok {
		return AccountMembership{}, ErrNotFound
	}
	mem, ok := byUser[accountID]
	if !ok {
		return AccountMembership{}, ErrNotFound
	}
	return mem, nil
}

// ListAccountMemberships returns every account the user belongs to, in account
// insertion order.
func (m *MemoryRepository) ListAccountMemberships(_ context.Context, userID string) ([]AccountMembership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	byUser := m.accMembers[userID]
	var out []AccountMembership
	for _, accID := range m.accOrder {
		if mem, ok := byUser[accID]; ok {
			out = append(out, mem)
		}
	}
	return out, nil
}

// SetAccountMembership grants or updates a user's membership role within an
// account (Accounts M2, design §3.5). The account MUST exist; a non-existent
// account returns ErrNotFound. Adding/updating a membership is not topic of
// this method and the M2 authZ middleware needs it.
func (m *MemoryRepository) SetAccountMembership(_ context.Context, mem AccountMembership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.accounts[mem.AccountID]; !ok {
		return ErrNotFound
	}
	if m.accMembers[mem.UserID] == nil {
		m.accMembers[mem.UserID] = make(map[string]AccountMembership)
	}
	m.accMembers[mem.UserID][mem.AccountID] = mem
	return nil
}

// ListProjectsForAccount returns only the projects owned by accountID, in
// insertion order. The `p.AccountID != accountID` predicate is the L2 tenant
// scope (design §0): the whole cross-tenant isolation guarantee on the
// RLS-less in-memory backend.
func (m *MemoryRepository) ListProjectsForAccount(_ context.Context, accountID string) ([]Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Project
	for _, id := range m.prjOrder {
		p := m.projects[id]
		if p.AccountID != accountID {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// GetProjectForAccount returns a project by id ONLY when it belongs to accountID.
// A project owned by a DIFFERENT account — or an unknown id — returns ErrNotFound
// (NOT_FOUND anti-enumeration, design §4.3), never a distinguishable error that
// would confirm the id exists in another tenant.
func (m *MemoryRepository) GetProjectForAccount(_ context.Context, accountID, projectID string) (Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[projectID]
	if !ok || p.AccountID != accountID {
		return Project{}, ErrNotFound
	}
	return p, nil
}

func (m *MemoryRepository) UpdateAccount(_ context.Context, a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.accounts[a.AccountID]
	if !ok {
		return ErrNotFound
	}
	if a.Name != existing.Name {
		if dupID, dup := m.accByName[a.Name]; dup && dupID != a.AccountID {
			return ErrConflict
		}
		delete(m.accByName, existing.Name)
		m.accByName[a.Name] = a.AccountID
	}
	if a.Slug != existing.Slug {
		if dupID, dup := m.accBySlug[a.Slug]; dup && dupID != a.AccountID {
			return ErrConflict
		}
		delete(m.accBySlug, existing.Slug)
		m.accBySlug[a.Slug] = a.AccountID
	}
	m.accounts[a.AccountID] = a
	return nil
}

func (m *MemoryRepository) CreateBranch(_ context.Context, b Branch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, bid := range m.brnOrder {
		if ex := m.branches[bid]; ex.ProjectID == b.ProjectID && ex.Name == b.Name {
			return ErrConflict
		}
	}
	m.branches[b.ID] = b
	m.brnOrder = append(m.brnOrder, b.ID)
	return nil
}

func (m *MemoryRepository) GetBranch(_ context.Context, branchID string) (Branch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.branches[branchID]
	if !ok {
		return Branch{}, ErrNotFound
	}
	return b, nil
}

func (m *MemoryRepository) ListBranches(_ context.Context, projectID string) ([]Branch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Branch
	for _, bid := range m.brnOrder {
		b := m.branches[bid]
		if b.ProjectID == projectID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (m *MemoryRepository) UpdateBranch(_ context.Context, b Branch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.branches[b.ID]
	if !ok {
		return ErrNotFound
	}
	if b.Name != existing.Name {
		for _, bid := range m.brnOrder {
			if ex := m.branches[bid]; ex.ID != b.ID && ex.ProjectID == b.ProjectID && ex.Name == b.Name {
				return ErrConflict
			}
		}
	}
	m.branches[b.ID] = b
	return nil
}

func (m *MemoryRepository) DeleteBranch(_ context.Context, branchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.branches[branchID]; !ok {
		return ErrNotFound
	}
	delete(m.branches, branchID)
	filtered := make([]string, 0, len(m.brnOrder))
	for _, bid := range m.brnOrder {
		if bid != branchID {
			filtered = append(filtered, bid)
		}
	}
	m.brnOrder = filtered
	return nil
}

func (m *MemoryRepository) CreateWebhook(_ context.Context, wh Webhook) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhooks[wh.ID] = wh
	return nil
}

func (m *MemoryRepository) GetWebhook(_ context.Context, webhookID string) (Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wh, ok := m.webhooks[webhookID]
	if !ok {
		return Webhook{}, ErrNotFound
	}
	return wh, nil
}

func (m *MemoryRepository) ListWebhooks(_ context.Context, projectID string) ([]Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Webhook
	for _, wh := range m.webhooks {
		if wh.ProjectID == projectID {
			out = append(out, wh)
		}
	}
	return out, nil
}

func (m *MemoryRepository) DeleteWebhook(_ context.Context, webhookID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.webhooks[webhookID]; !ok {
		return ErrNotFound
	}
	delete(m.webhooks, webhookID)
	return nil
}

func (m *MemoryRepository) UpdateWebhookTimestamps(_ context.Context, webhookID string, lastSuccess, lastFailure *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	wh, ok := m.webhooks[webhookID]
	if !ok {
		return ErrNotFound
	}
	wh.LastSuccessAt = lastSuccess
	wh.LastFailureAt = lastFailure
	m.webhooks[webhookID] = wh
	return nil
}
