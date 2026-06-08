package rollout

import (
	"context"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// runStorageScenario drives the brick Engine through a full lifecycle over the
// given StoragePort and asserts the persisted round-trip. Run against BOTH the
// in-memory store (here) and the pgx store (the integration test), proving
// behavioural parity of the two ports on identical engine behaviour.
func runStorageScenario(t *testing.T, store engine.StoragePort) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	eng, err := engine.New(store, fixedClock(&now))
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	if _, err := eng.Create(ctx, "dep-S", twoPhasePlan()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if st, err := eng.Start(ctx, "dep-S"); err != nil || st.Status != engine.StatusActive {
		t.Fatalf("Start: status=%q err=%v", st.Status, err)
	}

	// Persisted round-trip after start: state + immutable plan survive Load.
	got, err := store.Load(ctx, "dep-S")
	if err != nil || got.Status != engine.StatusActive || len(got.Phases) != 2 ||
		got.Phases[0].Percentage != 50 || got.Phases[1].Percentage != 100 {
		t.Fatalf("Load round-trip after start: %+v err=%v", got, err)
	}

	// Healthy phase 0 -> advance; healthy final -> complete.
	if dec, err := eng.Evaluate(ctx, "dep-S", engine.HealthVerdict{SuccessRate: 0.95}); err != nil || dec.Action != engine.ActionAdvance {
		t.Fatalf("Evaluate phase0: action=%q err=%v", dec.Action, err)
	}
	if dec, err := eng.Evaluate(ctx, "dep-S", engine.HealthVerdict{SuccessRate: 0.95}); err != nil || dec.Action != engine.ActionComplete {
		t.Fatalf("Evaluate final: action=%q err=%v", dec.Action, err)
	}
	if got, _ := store.Load(ctx, "dep-S"); got.Status != engine.StatusCompleted || got.CurrentPhase != 1 {
		t.Fatalf("final persisted state: %+v", got)
	}

	// Independent rollout halts on an error breach (HALT-wins).
	if _, err := eng.Create(ctx, "dep-H", twoPhasePlan()); err != nil {
		t.Fatalf("Create dep-H: %v", err)
	}
	if _, err := eng.Start(ctx, "dep-H"); err != nil {
		t.Fatalf("Start dep-H: %v", err)
	}
	if dec, err := eng.Evaluate(ctx, "dep-H", engine.HealthVerdict{SuccessRate: 0.99, ErrorRate: 0.5}); err != nil || dec.Action != engine.ActionHalt {
		t.Fatalf("Evaluate breach: action=%q err=%v", dec.Action, err)
	}
	if got, _ := store.Load(ctx, "dep-H"); got.Status != engine.StatusHalted {
		t.Fatalf("halted persisted state: %+v", got)
	}
}

// TestMemoryStoreScenario runs the shared scenario against the in-memory store.
func TestMemoryStoreScenario(t *testing.T) {
	runStorageScenario(t, NewMemoryStore())
}
