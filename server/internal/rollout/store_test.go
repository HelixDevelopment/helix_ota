package rollout

import (
	"context"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// fixedClock returns a controllable clock for deterministic evaluation.
func fixedClock(t *time.Time) ClockFunc { return func() time.Time { return *t } }

func twoPhasePlan() []engine.Phase {
	return []engine.Phase{
		{Percentage: 50, SuccessThreshold: 0.9, ErrorThreshold: 0.1, Duration: 0, AutoProgress: true},
		{Percentage: 100, SuccessThreshold: 0.9, ErrorThreshold: 0.1, Duration: 0, AutoProgress: true},
	}
}

// TestEngineDrivesPhasesToCompletion exercises the brick Engine over the
// in-memory StoragePort: create -> start (active, phase 0) -> a healthy verdict
// advances to phase 1 -> a healthy verdict on the final phase completes.
func TestEngineDrivesPhasesToCompletion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	eng, err := engine.New(NewMemoryStore(), fixedClock(&now))
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	st, err := eng.Create(ctx, "dep-1", twoPhasePlan())
	if err != nil || st.Status != engine.StatusPending {
		t.Fatalf("Create: status=%q err=%v", st.Status, err)
	}
	st, err = eng.Start(ctx, "dep-1")
	if err != nil || st.Status != engine.StatusActive || st.CurrentPhase != 0 {
		t.Fatalf("Start: status=%q phase=%d err=%v", st.Status, st.CurrentPhase, err)
	}

	// Healthy phase 0 -> advance to phase 1.
	dec, err := eng.Evaluate(ctx, "dep-1", engine.HealthVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if err != nil || dec.Action != engine.ActionAdvance || dec.Status != engine.StatusActive {
		t.Fatalf("Evaluate phase0: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}

	// Healthy final phase -> complete.
	dec, err = eng.Evaluate(ctx, "dep-1", engine.HealthVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if err != nil || dec.Action != engine.ActionComplete || dec.Status != engine.StatusCompleted {
		t.Fatalf("Evaluate final: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}
}

// TestEngineHaltsOnErrorBreach proves the HALT-wins safety invariant: an
// error-rate breach halts the rollout even with a passing success rate.
func TestEngineHaltsOnErrorBreach(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	eng, _ := engine.New(NewMemoryStore(), fixedClock(&now))
	if _, err := eng.Create(ctx, "dep-2", twoPhasePlan()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := eng.Start(ctx, "dep-2"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// success high AND error breached -> HALT wins.
	dec, err := eng.Evaluate(ctx, "dep-2", engine.HealthVerdict{SuccessRate: 0.99, ErrorRate: 0.5})
	if err != nil || dec.Action != engine.ActionHalt || dec.Status != engine.StatusHalted {
		t.Fatalf("Evaluate breach: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}
}

// TestMemoryStoreNotFound confirms the port returns the brick sentinel.
func TestMemoryStoreNotFound(t *testing.T) {
	_, err := NewMemoryStore().Load(context.Background(), "nope")
	if err != engine.ErrNotFound {
		t.Fatalf("want engine.ErrNotFound, got %v", err)
	}
}
