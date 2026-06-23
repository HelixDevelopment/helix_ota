//go:build integration

// Coverage-push integration test (§11.4.69 real captured evidence, §11.4.74 reuse
// the containers brick): drives the pgx rollout StoragePort paths the shared
// scenario (scenario_test.go) does NOT reach — NewPostgresStoreFromPool (0.0%),
// Save with a SET vs ZERO PhaseStartedAt, Save OVERWRITE (the DELETE-then-reinsert
// phase-replacement branch), and Load-not-found (engine.ErrNotFound). All against
// the REAL PostgreSQL booted on-demand via the digital.vasic.containers submodule
// (port 55445, serialized via lockPgIntegration per §11.4.119). Run:
//
//	go test -tags integration ./internal/rollout/
package rollout

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
	"github.com/jackc/pgx/v5/pgxpool"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

func TestPostgresStoreCoveragePaths_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 serialize shared Postgres across integration packages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Fatalf("no container runtime (podman/docker): %v", err)
	}
	orch, err := compose.NewDefaultOrchestrator(projectDir, logging.NopLogger{})
	if err != nil {
		t.Fatalf("compose orchestrator: %v", err)
	}
	ep := endpoint.NewEndpoint().
		WithHost("localhost").WithPort(pgHostPort).
		WithHealthType("tcp").WithRequired(true).WithEnabled(true).
		WithComposeFile("postgres-rollout.compose.yml").WithServiceName("postgres").
		WithTimeout(120 * time.Second).WithRetryCount(60).
		Build()
	mgr := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{"postgres": ep},
		boot.WithRuntime(rt), boot.WithOrchestrator(orch),
		boot.WithHealthChecker(health.NewDefaultChecker()),
		boot.WithProjectDir(projectDir), boot.WithLogger(logging.NopLogger{}),
	)
	summary, err := mgr.BootAll(ctx)
	if err != nil {
		t.Fatalf("BootAll: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })
	if summary.Failed > 0 {
		t.Fatalf("boot reports %d failed service(s)", summary.Failed)
	}
	t.Logf("boot summary: started=%d failed=%d in %s", summary.Started, summary.Failed, summary.TotalDuration)

	// Build the store via the FromPool constructor (the 0.0% path).
	pool := mustPool(t, ctx)
	t.Cleanup(pool.Close)
	store := NewPostgresStoreFromPool(pool)

	resetSchema(t, ctx)
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	// --- Load not-found -> engine.ErrNotFound (the pgx.ErrNoRows branch). ---
	if _, err := store.Load(ctx, "absent"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("Load absent want engine.ErrNotFound, got %v", err)
	}

	// --- Save with ZERO PhaseStartedAt (the nil-timestamp branch) + round-trip. ---
	st := engine.State{
		DeploymentID: "dep-cov",
		Status:       engine.StatusPending,
		CurrentPhase: 0,
		// PhaseStartedAt left zero on purpose.
		UpdatedAt: now,
		Phases: []engine.Phase{
			{Percentage: 25, SuccessThreshold: 0.9, ErrorThreshold: 0.1, Duration: 0, AutoProgress: true},
			{Percentage: 50, SuccessThreshold: 0.9, ErrorThreshold: 0.1, Duration: time.Hour, AutoProgress: false},
			{Percentage: 100, SuccessThreshold: 0.95, ErrorThreshold: 0.05, Duration: 0, AutoProgress: true},
		},
	}
	if err := store.Save(ctx, st); err != nil {
		t.Fatalf("Save (zero PhaseStartedAt): %v", err)
	}
	got, err := store.Load(ctx, "dep-cov")
	if err != nil {
		t.Fatalf("Load after first Save: %v", err)
	}
	if got.Status != engine.StatusPending || got.CurrentPhase != 0 || len(got.Phases) != 3 {
		t.Fatalf("Load round-trip mismatch: %+v", got)
	}
	if !got.PhaseStartedAt.IsZero() {
		t.Fatalf("zero PhaseStartedAt must round-trip as zero, got %v", got.PhaseStartedAt)
	}
	if got.Phases[1].Duration != time.Hour || got.Phases[1].AutoProgress {
		t.Fatalf("phase[1] mismatch: %+v", got.Phases[1])
	}

	// --- Save OVERWRITE: a fewer-phase plan + SET PhaseStartedAt. The Save path
	// DELETEs the old phases and re-INSERTs; Load must reflect ONLY the new ones. ---
	st2 := engine.State{
		DeploymentID:   "dep-cov",
		Status:         engine.StatusActive,
		CurrentPhase:   1,
		PhaseStartedAt: now.Add(30 * time.Minute),
		UpdatedAt:      now.Add(time.Hour),
		Phases: []engine.Phase{
			{Percentage: 60, SuccessThreshold: 0.9, ErrorThreshold: 0.1, Duration: 0, AutoProgress: true},
			{Percentage: 100, SuccessThreshold: 0.95, ErrorThreshold: 0.05, Duration: 0, AutoProgress: true},
		},
	}
	if err := store.Save(ctx, st2); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}
	got2, err := store.Load(ctx, "dep-cov")
	if err != nil {
		t.Fatalf("Load after overwrite: %v", err)
	}
	if got2.Status != engine.StatusActive || got2.CurrentPhase != 1 {
		t.Fatalf("overwrite state mismatch: %+v", got2)
	}
	if len(got2.Phases) != 2 || got2.Phases[0].Percentage != 60 || got2.Phases[1].Percentage != 100 {
		t.Fatalf("overwrite must replace phases (want 2: 60,100), got %+v", got2.Phases)
	}
	if got2.PhaseStartedAt.IsZero() || !got2.PhaseStartedAt.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("set PhaseStartedAt must round-trip, got %v", got2.PhaseStartedAt)
	}
}

// mustPool opens a pool against the freshly-booted Postgres, retrying until ready.
func mustPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	var lastErr error
	for i := 0; i < 60; i++ {
		pool, err := pgxpool.New(ctx, pgDSN)
		if err == nil {
			if perr := pool.Ping(ctx); perr == nil {
				return pool
			} else {
				lastErr = perr
				pool.Close()
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for postgres: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("postgres never became query-ready: %v", lastErr)
	return nil
}
