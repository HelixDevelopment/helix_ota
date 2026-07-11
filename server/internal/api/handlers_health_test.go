package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func TestHealthAndReadiness(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name string
		path string
	}{
		{"healthz", "/healthz"},
		{"readyz", "/readyz"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(http.MethodGet, tc.path, "", nil, "")
			if w.Code != http.StatusOK {
				t.Fatalf("%s: want 200, got %d (%s)", tc.path, w.Code, w.Body.String())
			}
		})
	}
}

func TestReadyzNotReady(t *testing.T) {
	env := newTestEnv(t)
	// Swap in a never-ready checker and rebuild the router.
	env.srv.health = neverReady{}
	env.router = env.srv.Router()

	w := env.do(http.MethodGet, "/readyz", "", nil, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when not ready, got %d", w.Code)
	}
	// Liveness is independent of readiness.
	if lw := env.do(http.MethodGet, "/healthz", "", nil, ""); lw.Code != http.StatusOK {
		t.Fatalf("healthz should stay 200, got %d", lw.Code)
	}
}

// neverReady is a Checker that is live but never ready.
type neverReady struct{}

func (neverReady) Live() bool                   { return true }
func (neverReady) Ready(_ context.Context) bool { return false }

// unreachableStore models a persistence store that cannot serve: every readiness
// round-trip errors, the way a down PostgreSQL surfaces a connection failure.
type unreachableStore struct{}

func (unreachableStore) ListProjects(context.Context) ([]store.Project, error) {
	return nil, errors.New("store unreachable")
}

// reachableStore models a healthy store: the readiness round-trip succeeds.
type reachableStore struct{}

func (reachableStore) ListProjects(context.Context) ([]store.Project, error) {
	return nil, nil
}

// TestReadyzReflectsStoreReadiness is the SRV-NEW-2 / OTA-063 guard: /readyz must
// consult real store health, returning 503 when the store cannot serve and 200
// only when a store round-trip succeeds — never an unconditional 200. It drives
// the production probe (NewStoreReadinessProbe) through the real handler.
//
// RED reproduction (§11.4.115): with the pre-fix always-ready probe the
// unready-store assertion fails (503 wanted, 200 returned); with the store-backed
// probe it passes. The anti-tautology polarity switch (temporarily forcing
// NewStoreReadinessProbe to `return true`) is exercised out-of-band and drives
// this same assertion RED, proving the test genuinely catches the bug.
func TestReadyzReflectsStoreReadiness(t *testing.T) {
	env := newTestEnv(t)

	// Unready store => /readyz must be 503 (an orchestrator withholds traffic).
	env.srv.health = health.New(NewStoreReadinessProbe(unreachableStore{}))
	env.router = env.srv.Router()
	if w := env.do(http.MethodGet, "/readyz", "", nil, ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unready store: want 503, got %d (%s)", w.Code, w.Body.String())
	}

	// Healthy store => /readyz is 200.
	env.srv.health = health.New(NewStoreReadinessProbe(reachableStore{}))
	env.router = env.srv.Router()
	if w := env.do(http.MethodGet, "/readyz", "", nil, ""); w.Code != http.StatusOK {
		t.Fatalf("healthy store: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// Liveness is independent of readiness: /healthz stays 200 either way.
	if lw := env.do(http.MethodGet, "/healthz", "", nil, ""); lw.Code != http.StatusOK {
		t.Fatalf("healthz should stay 200, got %d", lw.Code)
	}
}
