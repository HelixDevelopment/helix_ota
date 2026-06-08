package rollout

import (
	"context"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// Service is the control-plane facade over the brick Engine + a StoragePort. It
// exposes exactly the operations the REST layer needs (create+start, read,
// evaluate) so handlers never touch the engine/port directly.
type Service struct {
	engine *engine.Engine
	store  engine.StoragePort
}

// NewService builds a Service backed by an in-memory store and the given clock
// (the MVP default — no database required to run).
func NewService(now func() time.Time) *Service {
	return NewServiceWithStore(NewMemoryStore(), now)
}

// NewServiceWithStore builds a Service over any engine.StoragePort (e.g. the
// pgx PostgresStore for production) and the given clock.
func NewServiceWithStore(store engine.StoragePort, now func() time.Time) *Service {
	eng, _ := engine.New(store, ClockFunc(now)) // store+clock non-nil => never errors
	return &Service{engine: eng, store: store}
}

// CreateAndStart validates + persists the plan and activates the first phase.
func (s *Service) CreateAndStart(ctx context.Context, deploymentID string, phases []engine.Phase) (engine.State, error) {
	if _, err := s.engine.Create(ctx, deploymentID, phases); err != nil {
		return engine.State{}, err
	}
	return s.engine.Start(ctx, deploymentID)
}

// Get returns the current rollout state, or engine.ErrNotFound.
func (s *Service) Get(ctx context.Context, deploymentID string) (engine.State, error) {
	return s.store.Load(ctx, deploymentID)
}

// Evaluate applies a health verdict and returns the engine decision.
func (s *Service) Evaluate(ctx context.Context, deploymentID string, v engine.HealthVerdict) (engine.Decision, error) {
	return s.engine.Evaluate(ctx, deploymentID, v)
}
