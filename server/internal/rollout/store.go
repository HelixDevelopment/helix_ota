// Package rollout wires the reusable ota-rollout-engine brick (the staged-rollout
// Engine: Create/Start/Evaluate over a StoragePort) into the Helix OTA control
// plane. It supplies an in-memory StoragePort and a clock adapter; the engine
// itself — phase validation, the HALT-wins safety invariant, deterministic
// cohort assignment — is the brick's, not reinvented here (§11.4.74).
package rollout

import (
	"context"
	"sync"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// MemoryStore is an in-memory, concurrency-safe engine.StoragePort. The
// production target is a pgx-backed store over migration 002's rollouts +
// deployment_phases tables; this keeps the engine seam exercisable without a DB.
type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]engine.State
}

// compile-time assertion that MemoryStore satisfies the brick port.
var _ engine.StoragePort = (*MemoryStore)(nil)

// NewMemoryStore constructs an empty in-memory rollout state store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]engine.State)}
}

// Load returns the persisted state for deploymentID, or engine.ErrNotFound.
func (m *MemoryStore) Load(_ context.Context, deploymentID string) (engine.State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.states[deploymentID]
	if !ok {
		return engine.State{}, engine.ErrNotFound
	}
	return st.Clone(), nil
}

// Save persists state keyed by state.DeploymentID.
func (m *MemoryStore) Save(_ context.Context, state engine.State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.DeploymentID] = state.Clone()
	return nil
}

// ClockFunc adapts a func() time.Time into the brick's engine.Clock, so the
// control plane shares one clock (and tests inject a deterministic one).
type ClockFunc func() time.Time

// compile-time assertion that ClockFunc satisfies engine.Clock.
var _ engine.Clock = ClockFunc(nil)

// Now returns the current instant from the wrapped function.
func (f ClockFunc) Now() time.Time { return f() }
