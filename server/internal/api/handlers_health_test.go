package api

import (
	"context"
	"net/http"
	"testing"
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
