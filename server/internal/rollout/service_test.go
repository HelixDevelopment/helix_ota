package rollout

import (
	"context"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// TestServiceCreateStartGetEvaluate drives the control-plane Service facade
// end-to-end over its default in-memory store: NewService -> CreateAndStart
// (create + activate phase 0) -> Get (read back the active state) -> Evaluate
// (healthy verdict advances the phase). This is the seam the REST layer uses,
// so it must behave identically to the raw engine.
func TestServiceCreateStartGetEvaluate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	svc := NewService(func() time.Time { return now })

	st, err := svc.CreateAndStart(ctx, "dep-svc", twoPhasePlan())
	if err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	if st.Status != engine.StatusActive || st.CurrentPhase != 0 {
		t.Fatalf("after start: status=%q phase=%d", st.Status, st.CurrentPhase)
	}

	// Get reads the persisted active state back through the store.
	got, err := svc.Get(ctx, "dep-svc")
	if err != nil || got.Status != engine.StatusActive || len(got.Phases) != 2 {
		t.Fatalf("Get after start: %+v err=%v", got, err)
	}

	// A healthy verdict on phase 0 advances to phase 1 (stays active).
	dec, err := svc.Evaluate(ctx, "dep-svc", engine.HealthVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if err != nil || dec.Action != engine.ActionAdvance || dec.Status != engine.StatusActive {
		t.Fatalf("Evaluate phase0: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}

	// A second healthy verdict on the final phase completes the rollout.
	dec, err = svc.Evaluate(ctx, "dep-svc", engine.HealthVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if err != nil || dec.Action != engine.ActionComplete || dec.Status != engine.StatusCompleted {
		t.Fatalf("Evaluate final: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}

	// The completed state is durable through Get.
	if final, _ := svc.Get(ctx, "dep-svc"); final.Status != engine.StatusCompleted {
		t.Fatalf("Get after complete: %+v", final)
	}
}

// TestServiceEvaluateHaltsOnErrorBreach proves the Service propagates the brick's
// HALT-wins invariant: an error-rate breach halts even with a passing success rate.
func TestServiceEvaluateHaltsOnErrorBreach(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	svc := NewService(func() time.Time { return now })

	if _, err := svc.CreateAndStart(ctx, "dep-halt", twoPhasePlan()); err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	dec, err := svc.Evaluate(ctx, "dep-halt", engine.HealthVerdict{SuccessRate: 0.99, ErrorRate: 0.5})
	if err != nil || dec.Action != engine.ActionHalt || dec.Status != engine.StatusHalted {
		t.Fatalf("Evaluate breach: action=%q status=%q err=%v", dec.Action, dec.Status, err)
	}
	if got, _ := svc.Get(ctx, "dep-halt"); got.Status != engine.StatusHalted {
		t.Fatalf("halted state not persisted: %+v", got)
	}
}

// TestServiceGetNotFound covers the Service's pass-through of the store's
// ErrNotFound for an unknown rollout.
func TestServiceGetNotFound(t *testing.T) {
	svc := NewService(time.Now)
	if _, err := svc.Get(context.Background(), "never-created"); err != engine.ErrNotFound {
		t.Fatalf("want engine.ErrNotFound, got %v", err)
	}
}

// TestServiceCreateAndStartInvalidPlan covers the early-return error path of
// CreateAndStart when the engine rejects the plan (Create fails before Start).
func TestServiceCreateAndStartInvalidPlan(t *testing.T) {
	svc := NewService(time.Now)
	// An empty phase list is an invalid plan; Create must error and CreateAndStart
	// must surface that error without starting anything.
	if _, err := svc.CreateAndStart(context.Background(), "dep-bad", nil); err == nil {
		t.Fatalf("want error for empty plan, got nil")
	}
	// Nothing should have been persisted for the rejected rollout.
	if _, err := svc.Get(context.Background(), "dep-bad"); err != engine.ErrNotFound {
		t.Fatalf("rejected plan must not persist: got %v", err)
	}
}

// TestServiceWithStoreUsesProvidedStore proves NewServiceWithStore wires the
// caller-supplied StoragePort: state created through the Service is visible by
// loading directly from the same store instance.
func TestServiceWithStoreUsesProvidedStore(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	svc := NewServiceWithStore(store, func() time.Time { return now })

	if _, err := svc.CreateAndStart(ctx, "dep-shared", twoPhasePlan()); err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	// Read directly from the injected store, bypassing the Service.
	got, err := store.Load(ctx, "dep-shared")
	if err != nil || got.Status != engine.StatusActive {
		t.Fatalf("injected store missing state: %+v err=%v", got, err)
	}
}

// TestMemoryStoreSaveLoadCloneIndependence proves Save/Load deep-clone state so a
// mutation of a returned copy cannot corrupt the stored record (the anti-bluff
// guarantee behind the engine's immutable-plan contract).
func TestMemoryStoreSaveLoadCloneIndependence(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	original := engine.State{
		DeploymentID: "dep-clone",
		Status:       engine.StatusActive,
		Phases:       twoPhasePlan(),
	}
	if err := m.Save(ctx, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Mutate the caller-side struct after Save: the stored copy must not change.
	original.Status = engine.StatusHalted
	original.Phases[0].Percentage = 999

	loaded, err := m.Load(ctx, "dep-clone")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != engine.StatusActive {
		t.Fatalf("Save did not clone: stored status mutated to %q", loaded.Status)
	}
	if loaded.Phases[0].Percentage != 50 {
		t.Fatalf("Save did not clone phases: got percentage %d", loaded.Phases[0].Percentage)
	}

	// Mutating the loaded copy must not corrupt the store either.
	loaded.Phases[1].Percentage = 1
	reloaded, _ := m.Load(ctx, "dep-clone")
	if reloaded.Phases[1].Percentage != 100 {
		t.Fatalf("Load did not clone: store mutated via returned copy to %d", reloaded.Phases[1].Percentage)
	}
}

// TestClockFuncNow covers the ClockFunc adapter the control plane shares.
func TestClockFuncNow(t *testing.T) {
	want := time.Date(2026, 6, 8, 9, 30, 0, 0, time.UTC)
	var cf ClockFunc = func() time.Time { return want }
	if got := cf.Now(); !got.Equal(want) {
		t.Fatalf("ClockFunc.Now: want %v, got %v", want, got)
	}
}
