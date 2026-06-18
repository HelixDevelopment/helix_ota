package store

import (
	"context"
	"time"
)

// Emulation test-fabric registry — in-memory implementation
// (docs/design/emulation_fabric/SCHEMA.sql). Mirrors the pgx implementation's
// behaviour so the shared contract test proves memory+pgx parity. The
// exclusive-lease invariant (§11.4.119) and the non-empty-evidence rule
// (§11.4.69) are enforced here exactly as the pgx UNIQUE partial index + CHECK
// enforce them, so neither layer can silently diverge.

func cloneStrMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// CreateFabricNode upserts a fabric node by id.
func (m *MemoryRepository) CreateFabricNode(_ context.Context, n FabricNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n.Labels = cloneStrMap(n.Labels)
	m.fabNodes[n.NodeID] = n
	return nil
}

// GetFabricNode returns a fabric node by id.
func (m *MemoryRepository) GetFabricNode(_ context.Context, nodeID string) (FabricNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.fabNodes[nodeID]
	if !ok {
		return FabricNode{}, ErrNotFound
	}
	n.Labels = cloneStrMap(n.Labels)
	return n, nil
}

// CreateFabricTarget upserts a fabric target by id (insertion order preserved).
func (m *MemoryRepository) CreateFabricTarget(_ context.Context, t FabricTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.fabTargets[t.TargetID]; !exists {
		m.fabTgtOrder = append(m.fabTgtOrder, t.TargetID)
	}
	m.fabTargets[t.TargetID] = t
	return nil
}

// GetFabricTarget returns a fabric target by id.
func (m *MemoryRepository) GetFabricTarget(_ context.Context, targetID string) (FabricTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.fabTargets[targetID]
	if !ok {
		return FabricTarget{}, ErrNotFound
	}
	return t, nil
}

// ListFabricTargets returns all targets in insertion order.
func (m *MemoryRepository) ListFabricTargets(_ context.Context) ([]FabricTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]FabricTarget, 0, len(m.fabTgtOrder))
	for _, id := range m.fabTgtOrder {
		out = append(out, m.fabTargets[id])
	}
	return out, nil
}

// AcquireFabricLease records an exclusive hold (§11.4.119). For an exclusive
// target that already has an active (ReleaseAt nil) lease it returns ErrConflict
// — the equivalent of the pgx UNIQUE partial index uq_fabric_lease_active.
func (m *MemoryRepository) AcquireFabricLease(_ context.Context, l FabricLease) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tgt, ok := m.fabTargets[l.TargetID]
	if !ok {
		return ErrNotFound
	}
	if tgt.Exclusive {
		for _, existing := range m.fabLeases {
			if existing.TargetID == l.TargetID && existing.ReleaseAt == nil {
				return ErrConflict
			}
		}
	}
	l.ReleaseAt = nil
	m.fabLeases[l.LeaseID] = l
	// Reflect the lease in the target status (best-effort, mirrors fleet view).
	tgt.Status = "leased"
	m.fabTargets[l.TargetID] = tgt
	return nil
}

// ReleaseFabricLease marks the lease released. Idempotent on an already-released
// lease; ErrNotFound for an unknown lease id.
func (m *MemoryRepository) ReleaseFabricLease(_ context.Context, leaseID string, releaseAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.fabLeases[leaseID]
	if !ok {
		return ErrNotFound
	}
	if l.ReleaseAt == nil {
		rt := releaseAt
		l.ReleaseAt = &rt
		m.fabLeases[leaseID] = l
	}
	// If no other active lease holds the target, return it to idle.
	stillHeld := false
	for _, other := range m.fabLeases {
		if other.TargetID == l.TargetID && other.ReleaseAt == nil {
			stillHeld = true
			break
		}
	}
	if !stillHeld {
		if tgt, ok := m.fabTargets[l.TargetID]; ok && tgt.Status == "leased" {
			tgt.Status = "idle"
			m.fabTargets[l.TargetID] = tgt
		}
	}
	return nil
}

// ActiveFabricLease returns the currently-held lease for a target, or ErrNotFound.
func (m *MemoryRepository) ActiveFabricLease(_ context.Context, targetID string) (FabricLease, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, l := range m.fabLeases {
		if l.TargetID == targetID && l.ReleaseAt == nil {
			return l, nil
		}
	}
	return FabricLease{}, ErrNotFound
}

// CreateFabricRun records a test run.
func (m *MemoryRepository) CreateFabricRun(_ context.Context, r FabricRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fabTargets[r.TargetID]; !ok {
		return ErrNotFound
	}
	if r.Verdict == "" {
		r.Verdict = "PENDING"
	}
	m.fabRuns[r.RunID] = r
	return nil
}

// GetFabricRun returns a run by id.
func (m *MemoryRepository) GetFabricRun(_ context.Context, runID string) (FabricRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.fabRuns[runID]
	if !ok {
		return FabricRun{}, ErrNotFound
	}
	return r, nil
}

// UpdateFabricRun overwrites a run (e.g. verdict + ended_at). ErrNotFound if unknown.
func (m *MemoryRepository) UpdateFabricRun(_ context.Context, r FabricRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fabRuns[r.RunID]; !ok {
		return ErrNotFound
	}
	m.fabRuns[r.RunID] = r
	return nil
}

// AttachFabricEvidence appends a non-empty evidence artefact (§11.4.69). A
// non-positive ByteSize is rejected with ErrEvidenceEmpty (mirrors the pgx
// byte_size > 0 CHECK); an unknown run id returns ErrNotFound.
func (m *MemoryRepository) AttachFabricEvidence(_ context.Context, e FabricEvidence) error {
	if e.ByteSize <= 0 {
		return ErrEvidenceEmpty
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fabRuns[e.RunID]; !ok {
		return ErrNotFound
	}
	m.fabEvidence[e.RunID] = append(m.fabEvidence[e.RunID], e)
	return nil
}

// ListFabricEvidence returns a run's evidence in insertion order.
func (m *MemoryRepository) ListFabricEvidence(_ context.Context, runID string) ([]FabricEvidence, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]FabricEvidence(nil), m.fabEvidence[runID]...), nil
}
