package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/HelixDevelopment/helix_ota/server/internal/rollout"
	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

// evalThenVanishStore wraps an engine.StoragePort and, after a configurable
// number of Saved calls, resets its inner store so a subsequent Load returns
// engine.ErrNotFound. This exercises the handler's best-effort Get-after-Evaluate
// error path (handlers_rollout.go:149-154).
type evalThenVanishStore struct {
	engine.StoragePort
	savesBeforeVanish int // number of Save calls before the store is wiped
}

func (e *evalThenVanishStore) Save(ctx context.Context, state engine.State) error {
	if err := e.StoragePort.Save(ctx, state); err != nil {
		return err
	}
	e.savesBeforeVanish--
	if e.savesBeforeVanish <= 0 {
		// Replace with a fresh empty store so the next Load returns ErrNotFound.
		e.StoragePort = rollout.NewMemoryStore()
	}
	return nil
}

// TestEvaluateRolloutGetErrorCoverage proves that when rollout.Evaluate succeeds
// but the subsequent rollout.Get fails (the best-effort state-lookup path at
// handlers_rollout.go:149-154), the handler returns a proper 200 response with
// the decision rather than panicking or returning an empty/error response.
//
// The store wrapper is configured to vanish after 3 Save calls, which correspond
// to: (1) engine.Create, (2) engine.Start, (3) engine.Evaluate — leaving the
// handler's post-Evaluate Get to hit a wiped store.
func TestEvaluateRolloutGetErrorCoverage(t *testing.T) {
	env := newTestEnvWithStoreWrapper(t, 3) // vanish after 3 Saves (Create+Start+Evaluate)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	// Create + start the rollout.
	w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, twoPhaseBody())
	if w.Code != http.StatusCreated {
		t.Fatalf("create rollout want 201, got %d (%s)", w.Code, w.Body.String())
	}

	// Evaluate — the store will vanish during Evaluate's internal Save, so the
	// handler's subsequent Get returns ErrNotFound. The handler must return 200
	// with the decision (zero-valued state) and log the non-critical error.
	ew := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok,
		RolloutVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if ew.Code != http.StatusOK {
		t.Fatalf("evaluate after vanished store want 200, got %d (%s)", ew.Code, ew.Body.String())
	}

	var dec RolloutDecision
	env.decode(ew, &dec)

	// The decision itself should indicate what happened (advance, hold, complete).
	if dec.Action == "" {
		t.Fatal("evaluate decision has empty Action after vanished store")
	}
	if dec.Reason == "" {
		t.Fatal("evaluate decision has empty Reason after vanished store")
	}
	// The state may be zero-valued because Get returned ErrNotFound — that is the
	// expected graceful-degradation path the fix introduced.
	// Any response at all (non-panic, non-500) with a non-empty Action is a pass.
}

// newTestEnvWithStoreWrapper builds a testEnv whose rollout service uses a
// wrapped store that vanishes after savesBeforeVanish Save calls.
func newTestEnvWithStoreWrapper(t *testing.T, savesBeforeVanish int) *testEnv {
	t.Helper()
	env := newTestEnv(t)

	// Build a custom rollout service with the wrapped store.
	mem := rollout.NewMemoryStore()
	wrapped := &evalThenVanishStore{
		StoragePort:       mem,
		savesBeforeVanish: savesBeforeVanish,
	}
	now := env.srv.nowFn
	customSvc := rollout.NewServiceWithStore(wrapped, now)

	// Rebuild the server with the custom rollout service.
	env.srv.rollout = customSvc
	env.router = env.srv.Router()
	return env
}
