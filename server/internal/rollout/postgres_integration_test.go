//go:build integration

// Rollout pgx StoragePort integration test: proves the PostgresStore satisfies
// the brick's StoragePort contract identically to the in-memory store, against a
// REAL PostgreSQL booted on-demand through the containers submodule (never a
// manual step, never a fake). Run with: go test -tags integration ./internal/rollout/
package rollout

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgHostPort = "55445"
	pgDSN      = "postgres://helix:helix@localhost:55445/helix_ota?sslmode=disable"
)

func TestPostgresStoreScenario_Integration(t *testing.T) {
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

	// Wait for query-readiness, drop+create the schema, run the shared scenario.
	store := mustConnect(t, ctx)
	t.Cleanup(store.Close)
	resetSchema(t, ctx)
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	runStorageScenario(t, store)
}

func mustConnect(t *testing.T, ctx context.Context) *PostgresStore {
	t.Helper()
	var lastErr error
	for i := 0; i < 60; i++ {
		store, err := NewPostgresStore(ctx, pgDSN)
		if err == nil {
			return store
		}
		lastErr = err
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for postgres: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("postgres never became query-ready: %v", lastErr)
	return nil
}

func resetSchema(t *testing.T, ctx context.Context) {
	t.Helper()
	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		t.Fatalf("reset connect: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS helix_ota CASCADE"); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
}
